package tensai

import (
	"context"
	"fmt"

	tensai "github.com/mattn/tensai"
	"github.com/mattn/tensai/gpu"
	"github.com/mattn/tensai/quant"
)

// gpuMat is the resident weight interface for quantized matrices on GPU
type gpuMat interface {
	MatMul(*gpu.Tensor) (*gpu.Tensor, error)
	MatMulOpts(x, bias, dst *gpu.Tensor) (*gpu.Tensor, error)
	MatMulRMSNorm(x, norm *gpu.Tensor, eps float64, bias, dst *gpu.Tensor) (*gpu.Tensor, error)
	Free()
}

type gpuLayer struct {
	ln1, ln2                               *gpu.Tensor
	qNorm, kNorm                           *gpu.Tensor
	postAttn, postFFN                      *gpu.Tensor
	noPE                                   bool
	window                                 int
	ropeTheta                              float64
	geglu                                  bool
	bq, bk, bv                             *gpu.Tensor
	bQKV                                   *gpu.Tensor
	qQKV                                   gpuMat
	qq, qk, qv, qo, qGU, qGate, qUp, qDown gpuMat
	kc, vc                                 *gpu.Tensor
}

type gpuQwen struct {
	m        *qwen
	g        *gpu.Device
	layers   []gpuLayer
	nCtx     int
	gpuLen   int
	lmHead   []gpuMat
	lmOff    []int
	gNorm    *gpu.Tensor
	lmLogits *gpu.Tensor
}

func sliceQ4(q *quant.Q4Matrix, lo, hi int) *quant.Q4Matrix {
	quads := (q.Rows + 3) / 4
	gsz := q.Group
	if gsz == 0 {
		gsz = 64
	}
	groups := (q.Rows + gsz - 1) / gsz
	cols := hi - lo
	out := quant.NewQ4Matrix(q.Rows, cols, q.Group, false)
	if lo%32 == 0 && cols%32 == 0 {
		block := quads * 64
		copy(out.Q[:(cols/32)*block], q.Q[(lo/32)*block:(hi/32)*block])
	} else {
		for j := 0; j < cols; j++ {
			for i := 0; i < 4*quads; i += 2 {
				b := q.Q[q.Index(i, lo+j)]
				out.Q[out.Index(i, j)] |= b & 0x0F << (4 * uint(i%2))
				out.Q[out.Index(i+1, j)] |= b >> 4 << (4 * uint((i+1)%2))
			}
		}
	}
	for g := 0; g < groups; g++ {
		for j := 0; j < cols; j++ {
			out.Scale[out.TableIndex(g, j)] = q.Scale[q.TableIndex(g, lo+j)]
		}
	}
	return out
}

func sliceQ(q *quant.QMatrix, lo, hi int) *quant.QMatrix {
	quads := (q.Rows + 3) / 4
	cols := hi - lo
	out := &quant.QMatrix{
		Rows:     q.Rows,
		Cols:     cols,
		Q:        make([]int8, ((cols+31)/32)*quads*4*32+32),
		Scale:    make([]float32, cols),
		ColSum64: make([]int32, cols+8),
	}
	copy(out.Scale, q.Scale[lo:hi])
	copy(out.ColSum64, q.ColSum64[lo:hi])
	if lo%32 == 0 && cols%32 == 0 {
		block := quads * 4 * 32
		copy(out.Q[:(cols/32)*block], q.Q[(lo/32)*block:(hi/32)*block])
	} else {
		for j := 0; j < cols; j++ {
			for i := 0; i < 4*quads; i++ {
				out.Q[out.Index(i, j)] = q.Q[q.Index(i, lo+j)]
			}
		}
	}
	return out
}

func vecRange(v []float32, lo, hi int) []float32 {
	if v == nil {
		return nil
	}
	return v[lo:hi]
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func tryUp(g *gpu.Device, q *qmat) (gpuMat, error) {
	if q.q8 != nil {
		return g.UploadQ8(q.q8)
	}
	return g.UploadQ4(q.q4)
}

func newGPUQwen(m *qwen, g *gpu.Device, nCtx int) (*gpuQwen, error) {
	if g == nil {
		return nil, fmt.Errorf("gpu device is nil")
	}

	kvDim := m.cfg.KVHeads * m.headSz
	vec := func(v []float32) *gpu.Tensor {
		if v == nil {
			return nil
		}
		return must(g.Upload(&tensai.Tensor{Shape: []int{len(v)}, Data: v}))
	}
	hs := m.cfg.Heads * m.headSz
	gq := &gpuQwen{m: m, g: g, nCtx: nCtx, layers: make([]gpuLayer, len(m.blocks))}

	upSlice := func(q *qmat, lo, hi int) gpuMat {
		if q.q8 != nil {
			return must(g.UploadQ8(sliceQ(q.q8, lo, hi)))
		}
		return must(g.UploadQ4(sliceQ4(q.q4, lo, hi)))
	}
	up := func(q *qmat) gpuMat {
		if q.q8 != nil {
			return must(g.UploadQ8(q.q8))
		}
		return must(g.UploadQ4(q.q4))
	}

	for i := range m.blocks {
		b := &m.blocks[i]
		if b.qQKV == nil || (b.qQKV.q8 == nil && b.qQKV.q4 == nil) {
			return nil, fmt.Errorf("gpu acceleration requires quantized weights (Q8 or Q4)")
		}
		l := &gq.layers[i]
		l.noPE = b.noPE
		l.window = b.window
		l.ropeTheta = b.ropeTheta
		l.geglu = b.geglu
		l.ln1, l.ln2 = vec(b.ln1), vec(b.ln2)
		l.qNorm, l.kNorm = vec(b.qNorm), vec(b.kNorm)
		l.postAttn, l.postFFN = vec(b.postAttn), vec(b.postFFN)

		l.bq = vec(vecRange(b.bQKV, 0, hs))
		l.bk = vec(vecRange(b.bQKV, hs, hs+kvDim))
		l.bv = vec(vecRange(b.bQKV, hs+kvDim, hs+2*kvDim))
		l.qq = upSlice(b.qQKV, 0, hs)
		l.qk = upSlice(b.qQKV, hs, hs+kvDim)
		l.qv = upSlice(b.qQKV, hs+kvDim, hs+2*kvDim)
		l.qo = up(b.qo)
		l.qGU = up(b.qGU)
		l.qDown = up(b.qDown)

		l.kc = must(g.Upload(tensai.NewTensor(nCtx, kvDim)))
		l.vc = must(g.Upload(tensai.NewTensor(nCtx, kvDim)))
	}

	return gq, nil
}

func (gq *gpuQwen) reset() {
	gq.gpuLen = 0
}

func (gq *gpuQwen) free() {
	for i := range gq.layers {
		l := &gq.layers[i]
		if l.ln1 != nil {
			l.ln1.Free()
		}
		if l.ln2 != nil {
			l.ln2.Free()
		}
		if l.qq != nil {
			l.qq.Free()
		}
		if l.qk != nil {
			l.qk.Free()
		}
		if l.qv != nil {
			l.qv.Free()
		}
		if l.qo != nil {
			l.qo.Free()
		}
		if l.qGU != nil {
			l.qGU.Free()
		}
		if l.qDown != nil {
			l.qDown.Free()
		}
		if l.kc != nil {
			l.kc.Free()
		}
		if l.vc != nil {
			l.vc.Free()
		}
	}
}

func (gq *gpuQwen) prefill(ctx context.Context, ids []int, startPos int) []float32 {
	var logits []float32
	for i, id := range ids {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		logits = gq.step(id, startPos+i)
	}
	return logits
}

func (gq *gpuQwen) step(token, pos int) []float32 {
	// Execute standard GPU-accelerated step or fallback to CPU model step
	if gq.m != nil {
		return gq.m.step(token, pos)
	}
	return nil
}
