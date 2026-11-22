package ai

import (
	"context"
	"encoding/json"
	"math"
	"math/cmplx"
	"sort"
	"strings"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"gonum.org/v1/gonum/dsp/fourier"
)

// STLSeries holds Seasonal, Trend, and Residual components from STL decomposition.
type STLSeries struct {
	Seasonal []float64 `json:"Seasonal"`
	Trend    []float64 `json:"Trend"`
	Resid    []float64 `json:"Resid"`
}

// TimeAnalyzeResult contains STL decomposition and FFT spectrum analysis results
// at both hourly (H) and sub-hourly (PX2) resolutions.
type TimeAnalyzeResult struct {
	PX2       int64                   `json:"PX2"`
	TimeH     []int64                 `json:"TimeH"`
	TimePX2   []int64                 `json:"TimePX2"`
	StlMapH   map[string]*STLSeries   `json:"StlMapH"`
	StlMapPX2 map[string]*STLSeries   `json:"StlMapPX2"`
	FFTH      map[string][][2]float64 `json:"FFTH"`
	FFTPX2    map[string][][2]float64 `json:"FFTPX2"`
}

// AnalyzePollingTimeSeries performs STL decomposition and FFT frequency analysis
// on chronological polling logs for a given PollingEnt.
func AnalyzePollingTimeSeries(ctx context.Context, p *datastore.PollingEnt, logs []*datastore.PollingLogEnt) *TimeAnalyzeResult {
	res := &TimeAnalyzeResult{
		PX2:       120,
		TimeH:     []int64{},
		TimePX2:   []int64{},
		StlMapH:   make(map[string]*STLSeries),
		StlMapPX2: make(map[string]*STLSeries),
		FFTH:      make(map[string][][2]float64),
		FFTPX2:    make(map[string][][2]float64),
	}
	if len(logs) < 2 {
		return res
	}

	sortedLogs := make([]*datastore.PollingLogEnt, len(logs))
	copy(sortedLogs, logs)
	sort.SliceStable(sortedLogs, func(i, j int) bool {
		return sortedLogs[i].Time < sortedLogs[j].Time
	})

	keys := collectNumericResultKeys(sortedLogs)
	if len(keys) == 0 {
		return res
	}

	minSec := sortedLogs[0].Time / 1e9
	maxSec := sortedLogs[len(sortedLogs)-1].Time / 1e9
	if maxSec <= minSec {
		return res
	}

	// Determine PX2 resolution (default 2x PollInt, min 60s, scaled to keep <= 1500 grid points)
	var px2 int64 = 120
	if p != nil && p.PollInt > 0 {
		px2 = int64(p.PollInt * 2)
	}
	if px2 < 60 {
		px2 = 60
	}
	for (maxSec-minSec)/px2 > 1500 {
		px2 *= 2
	}
	res.PX2 = px2

	// 1. Hourly resolution (3600s) — cap to newest 4096 hours (~170 days) for bounded computation
	const hourStep int64 = 3600
	hourLogs := sortedLogs
	if (maxSec-minSec)/hourStep > 4096 {
		cutoff := (maxSec - 4096*hourStep) * 1e9
		idx := sort.Search(len(sortedLogs), func(i int) bool {
			return sortedLogs[i].Time >= cutoff
		})
		if idx < len(sortedLogs) {
			hourLogs = sortedLogs[idx:]
		}
	}

	timeH, seriesH := resampleLogs(hourLogs, keys, hourStep)
	res.TimeH = timeH
	if len(timeH) >= 4 {
		periodH := 24
		if len(timeH) < periodH*2 {
			periodH = len(timeH) / 2
			if periodH < 2 {
				periodH = 2
			}
		}
		for _, k := range keys {
			if ctx.Err() != nil {
				break
			}
			vals := seriesH[k]
			if stl := decomposeSTL(vals, periodH); stl != nil {
				res.StlMapH[k] = stl
			}
			if fft := computeFFT(vals, float64(hourStep)); len(fft) > 0 {
				res.FFTH[k] = fft
			}
		}
	}

	// 2. PX2 sub-hourly resolution
	timePX2, seriesPX2 := resampleLogs(sortedLogs, keys, px2)
	res.TimePX2 = timePX2
	if len(timePX2) >= 4 {
		// Nominal daily or hourly cycle depending on sample interval
		periodPX2 := int(3600 / px2)
		if periodPX2 < 6 {
			periodPX2 = 12
		}
		if len(timePX2) < periodPX2*2 {
			periodPX2 = len(timePX2) / 2
			if periodPX2 < 2 {
				periodPX2 = 2
			}
		}
		for _, k := range keys {
			if ctx.Err() != nil {
				break
			}
			vals := seriesPX2[k]
			if stl := decomposeSTL(vals, periodPX2); stl != nil {
				res.StlMapPX2[k] = stl
			}
			if fft := computeFFT(vals, float64(px2)); len(fft) > 0 {
				res.FFTPX2[k] = fft
			}
		}
	}

	return res
}

func collectNumericResultKeys(logs []*datastore.PollingLogEnt) []string {
	seen := make(map[string]bool)
	for _, l := range logs {
		if l == nil || len(l.Result) == 0 {
			continue
		}
		for k, v := range l.Result {
			if k == "lastTime" || strings.HasPrefix(k, "_") || seen[k] {
				continue
			}
			if _, ok := toNumericValue(v); ok {
				seen[k] = true
			}
		}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func toNumericValue(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		if math.IsNaN(val) || math.IsInf(val, 0) {
			return 0, false
		}
		return val, true
	case float32:
		f := float64(val)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, false
		}
		return f, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	case uint64:
		return float64(val), true
	case uint:
		return float64(val), true
	case json.Number:
		f, err := val.Float64()
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, false
		}
		return f, true
	case bool:
		if val {
			return 1.0, true
		}
		return 0.0, true
	default:
		return 0, false
	}
}

func resampleLogs(logs []*datastore.PollingLogEnt, keys []string, stepSec int64) ([]int64, map[string][]float64) {
	if len(logs) == 0 || stepSec <= 0 {
		return nil, nil
	}
	startBucket := (logs[0].Time / 1e9 / stepSec) * stepSec
	endBucket := (logs[len(logs)-1].Time / 1e9 / stepSec) * stepSec
	if endBucket < startBucket {
		return nil, nil
	}

	numBuckets := int((endBucket-startBucket)/stepSec) + 1
	times := make([]int64, numBuckets)
	for i := 0; i < numBuckets; i++ {
		times[i] = startBucket + int64(i)*stepSec
	}

	sums := make(map[string][]float64, len(keys))
	counts := make(map[string][]int, len(keys))
	for _, k := range keys {
		sums[k] = make([]float64, numBuckets)
		counts[k] = make([]int, numBuckets)
	}

	for _, l := range logs {
		if l == nil || len(l.Result) == 0 {
			continue
		}
		b := int(((l.Time / 1e9) - startBucket) / stepSec)
		if b < 0 || b >= numBuckets {
			continue
		}
		for _, k := range keys {
			if raw, ok := l.Result[k]; ok {
				if fv, okNum := toNumericValue(raw); okNum {
					sums[k][b] += fv
					counts[k][b]++
				}
			}
		}
	}

	out := make(map[string][]float64, len(keys))
	for _, k := range keys {
		arr := make([]float64, numBuckets)
		has := make([]bool, numBuckets)
		firstIdx := -1
		for i := 0; i < numBuckets; i++ {
			if counts[k][i] > 0 {
				arr[i] = sums[k][i] / float64(counts[k][i])
				has[i] = true
				if firstIdx == -1 {
					firstIdx = i
				}
			}
		}
		if firstIdx == -1 {
			// No observations for this key; fill zeros
			out[k] = arr
			continue
		}
		// Fill leading gap
		for i := 0; i < firstIdx; i++ {
			arr[i] = arr[firstIdx]
		}
		// Linear interpolation across interior gaps & carry forward trailing gap
		prevIdx := firstIdx
		for i := firstIdx + 1; i < numBuckets; i++ {
			if has[i] {
				gap := i - prevIdx
				if gap > 1 {
					startVal := arr[prevIdx]
					diff := arr[i] - startVal
					for j := 1; j < gap; j++ {
						arr[prevIdx+j] = startVal + diff*float64(j)/float64(gap)
					}
				}
				prevIdx = i
			}
		}
		for i := prevIdx + 1; i < numBuckets; i++ {
			arr[i] = arr[prevIdx]
		}
		out[k] = arr
	}

	return times, out
}

// decomposeSTL performs Seasonal-Trend decomposition using LOESS (Cleveland et al. 1990).
func decomposeSTL(values []float64, period int) *STLSeries {
	n := len(values)
	if n < 4 || period < 2 {
		return nil
	}
	if period > n/2 {
		period = n / 2
	}

	trend := make([]float64, n)
	seasonal := make([]float64, n)
	detrended := make([]float64, n)
	deseasonalized := make([]float64, n)
	resid := make([]float64, n)
	weights := make([]float64, n)
	for i := range weights {
		weights[i] = 1.0
	}

	// Trend window size (odd, >= 5, roughly 1.5x period)
	trendWindow := int(math.Ceil(1.5 * float64(period)))
	if trendWindow < 5 {
		trendWindow = 5
	}
	if trendWindow%2 == 0 {
		trendWindow++
	}
	if trendWindow > n {
		trendWindow = n
		if trendWindow%2 == 0 && trendWindow > 3 {
			trendWindow--
		}
	}

	// Seasonal subseries smoothing window
	subLen := (n + period - 1) / period
	seasonalWindow := subLen / 2
	if seasonalWindow < 3 {
		seasonalWindow = 3
	}
	if seasonalWindow%2 == 0 {
		seasonalWindow++
	}

	const outerIters = 2
	const innerIters = 2

	for outer := 0; outer < outerIters; outer++ {
		for inner := 0; inner < innerIters; inner++ {
			// Step 1: Detrend
			for i := 0; i < n; i++ {
				detrended[i] = values[i] - trend[i]
			}

			// Step 2: Cycle-subseries LOESS smoothing
			rawSeasonal := make([]float64, n)
			for m := 0; m < period; m++ {
				var subVals, subWeights []float64
				var indices []int
				for idx := m; idx < n; idx += period {
					subVals = append(subVals, detrended[idx])
					subWeights = append(subWeights, weights[idx])
					indices = append(indices, idx)
				}
				if len(subVals) == 0 {
					continue
				}
				smoothed := loessSmooth(subVals, subWeights, seasonalWindow)
				for j, idx := range indices {
					rawSeasonal[idx] = smoothed[j]
				}
			}

			// Step 3: Low-pass filter on seasonal to extract residual trend & ensure zero-mean seasonality
			lowPass := movingAvg(rawSeasonal, period)
			lowPass = movingAvg(lowPass, period)
			lowPass = movingAvg(lowPass, 3)

			// Center seasonal around zero per cycle
			var sumS float64
			for i := 0; i < n; i++ {
				seasonal[i] = rawSeasonal[i] - lowPass[i]
				sumS += seasonal[i]
			}
			meanS := sumS / float64(n)
			for i := 0; i < n; i++ {
				seasonal[i] -= meanS
			}

			// Step 4: Deseasonalize & smooth trend with LOESS
			for i := 0; i < n; i++ {
				deseasonalized[i] = values[i] - seasonal[i]
			}
			trend = loessSmooth(deseasonalized, weights, trendWindow)
		}

		// Robustness weights update using bisquare weights
		absResids := make([]float64, n)
		for i := 0; i < n; i++ {
			r := values[i] - trend[i] - seasonal[i]
			resid[i] = r
			absResids[i] = math.Abs(r)
		}
		sort.Float64s(absResids)
		medianAbs := absResids[n/2]
		h := 6.0 * medianAbs
		if h <= 1e-12 {
			for i := range weights {
				weights[i] = 1.0
			}
		} else {
			for i := 0; i < n; i++ {
				u := math.Abs(resid[i]) / h
				if u >= 1.0 {
					weights[i] = 0.0
				} else {
					t := 1.0 - u*u
					weights[i] = t * t
				}
			}
		}
	}

	for i := 0; i < n; i++ {
		resid[i] = values[i] - trend[i] - seasonal[i]
	}

	return &STLSeries{
		Seasonal: seasonal,
		Trend:    trend,
		Resid:    resid,
	}
}

func movingAvg(vals []float64, window int) []float64 {
	n := len(vals)
	out := make([]float64, n)
	if n == 0 || window <= 1 {
		copy(out, vals)
		return out
	}
	if window > n {
		window = n
	}
	halfLeft := (window - 1) / 2
	halfRight := window - 1 - halfLeft
	for i := 0; i < n; i++ {
		start := i - halfLeft
		if start < 0 {
			start = 0
		}
		end := i + halfRight
		if end >= n {
			end = n - 1
		}
		var sum float64
		for j := start; j <= end; j++ {
			sum += vals[j]
		}
		out[i] = sum / float64(end-start+1)
	}
	return out
}

// loessSmooth performs 1D locally weighted linear regression smoothing.
func loessSmooth(vals, extWeights []float64, span int) []float64 {
	n := len(vals)
	out := make([]float64, n)
	if n == 0 {
		return out
	}
	if n <= 2 {
		copy(out, vals)
		return out
	}
	if span < 3 {
		span = 3
	}
	if span > n {
		span = n
	}

	half := span / 2
	for i := 0; i < n; i++ {
		start := i - half
		end := i + half
		if start < 0 {
			end += -start
			start = 0
		}
		if end >= n {
			start -= end - (n - 1)
			if start < 0 {
				start = 0
			}
			end = n - 1
		}

		maxDist := float64(maxInt(i-start, end-i))
		if maxDist <= 0 {
			maxDist = 1
		}

		var sumW, sumWX, sumWY, sumWXX, sumWXY float64
		for j := start; j <= end; j++ {
			d := math.Abs(float64(j-i)) / (maxDist + 1e-9)
			if d >= 1.0 {
				continue
			}
			// Tricube kernel
			t := 1.0 - d*d*d
			w := t * t * t
			if extWeights != nil && j < len(extWeights) {
				w *= extWeights[j]
			}
			if w <= 0 {
				continue
			}
			x := float64(j - i)
			y := vals[j]
			sumW += w
			sumWX += w * x
			sumWY += w * y
			sumWXX += w * x * x
			sumWXY += w * x * y
		}

		if sumW <= 1e-12 {
			out[i] = vals[i]
			continue
		}
		meanX := sumWX / sumW
		meanY := sumWY / sumW
		denom := sumWXX - sumWX*meanX
		if math.Abs(denom) <= 1e-12 {
			out[i] = meanY
		} else {
			slope := (sumWXY - sumWX*meanY) / denom
			// Evaluated at x = 0 (center point i)
			out[i] = meanY - slope*meanX
		}
	}
	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// computeFFT calculates single-sided amplitude spectrum [freqHz, amplitude] using Hann window.
func computeFFT(values []float64, stepSec float64) [][2]float64 {
	n := len(values)
	if n < 4 || stepSec <= 0 {
		return nil
	}

	var mean float64
	for _, v := range values {
		mean += v
	}
	mean /= float64(n)

	windowed := make([]float64, n)
	var winSum float64
	for i, v := range values {
		w := 0.5 * (1.0 - math.Cos(2.0*math.Pi*float64(i)/float64(n-1)))
		windowed[i] = (v - mean) * w
		winSum += w
	}
	if winSum <= 1e-12 {
		winSum = float64(n)
	}

	fft := fourier.NewFFT(n)
	coeffs := fft.Coefficients(nil, windowed)

	half := n / 2
	out := make([][2]float64, 0, half)
	for k := 1; k <= half; k++ {
		freqHz := fft.Freq(k) / stepSec
		if freqHz <= 0 || math.IsNaN(freqHz) || math.IsInf(freqHz, 0) {
			continue
		}
		amp := 2.0 * cmplx.Abs(coeffs[k]) / winSum
		if math.IsNaN(amp) || math.IsInf(amp, 0) {
			continue
		}
		out = append(out, [2]float64{freqHz, amp})
	}
	return out
}
