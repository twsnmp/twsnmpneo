package ai

import (
	"math"
	"math/rand"
	"testing"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

func TestPearsonCorr(t *testing.T) {
	x := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	y := []float64{2, 4, 6, 8, 10, 12, 14, 16, 18, 20}
	r, p := pearsonCorr(x, y)
	if math.Abs(r-1.0) > 1e-6 {
		t.Fatalf("expected r=1.0, got %f", r)
	}
	if p >= 0.001 {
		t.Fatalf("expected p < 0.001, got %f", p)
	}

	yNeg := []float64{20, 18, 16, 14, 12, 10, 8, 6, 4, 2}
	rNeg, pNeg := pearsonCorr(x, yNeg)
	if math.Abs(rNeg+1.0) > 1e-6 {
		t.Fatalf("expected r=-1.0, got %f", rNeg)
	}
	if pNeg >= 0.001 {
		t.Fatalf("expected p < 0.001, got %f", pNeg)
	}
}

func TestSpearmanAndKendall(t *testing.T) {
	// Exponential monotonic relationship: Spearman/Kendall = 1.0, Pearson < 1.0
	x := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	y := make([]float64, len(x))
	for i, v := range x {
		y[i] = math.Exp(v)
	}
	r, _ := pearsonCorr(x, y)
	rho, rhoP := spearmanCorr(x, y)
	tau, tauP := kendallTauB(x, y)

	if r >= 0.99 {
		t.Fatalf("expected pearson < 0.99 for exp, got %f", r)
	}
	if math.Abs(rho-1.0) > 1e-6 {
		t.Fatalf("expected spearman=1.0, got %f", rho)
	}
	if math.Abs(tau-1.0) > 1e-6 {
		t.Fatalf("expected kendall=1.0, got %f", tau)
	}
	if rhoP >= 0.001 || tauP >= 0.005 {
		t.Fatalf("expected significant p-values, got rhoP=%f tauP=%f", rhoP, tauP)
	}
}

func TestCrossCorrLag(t *testing.T) {
	n := 40
	x := make([]float64, n)
	y := make([]float64, n)
	for i := 0; i < n; i++ {
		x[i] = math.Sin(float64(i) * 0.4)
	}
	// y[t+3] = x[t] -> x leads y by +3 buckets
	for i := 3; i < n; i++ {
		y[i] = x[i-3]
	}
	bestScore, _, bestLag, profile := crossCorrLag(x, y, 6, 60)
	if bestLag != 3 {
		t.Fatalf("expected bestLag=3, got %d", bestLag)
	}
	if math.Abs(bestScore-1.0) > 0.05 {
		t.Fatalf("expected bestScore~1.0, got %f", bestScore)
	}
	if len(profile) != 13 {
		t.Fatalf("expected 13 lag points, got %d", len(profile))
	}
}

func TestDistanceCorrMatrixNonlinear(t *testing.T) {
	// y = x^2 symmetric around 0 -> Pearson ~ 0, DistanceCorr >> 0
	n := 41
	x := make([]float64, n)
	y := make([]float64, n)
	for i := 0; i < n; i++ {
		v := float64(i - 20)
		x[i] = v
		y[i] = v * v
	}
	r, _ := pearsonCorr(x, y)
	dcor, p := distanceCorrMatrix([][]float64{x}, [][]float64{y}, true)
	if math.Abs(r) > 0.05 {
		t.Fatalf("expected pearson ~ 0 for y=x^2, got %f", r)
	}
	if dcor < 0.45 {
		t.Fatalf("expected dcor >= 0.45 for y=x^2, got %f", dcor)
	}
	if p > 0.05 {
		t.Fatalf("expected dcor p <= 0.05, got %f", p)
	}
}

func TestBucketSeriesAlignmentAndState(t *testing.T) {
	baseSec := int64(1700000000)
	var logs1 []*datastore.PollingLogEnt
	var logs2 []*datastore.PollingLogEnt
	for i := 0; i < 10; i++ {
		ts1 := (baseSec + int64(i*60) + 5) * 1_000_000_000
		ts2 := (baseSec + int64(i*60) + 25) * 1_000_000_000
		logs1 = append(logs1, &datastore.PollingLogEnt{
			Time:  ts1,
			State: "normal",
			Result: map[string]interface{}{
				"rtt": float64(i + 1),
			},
		})
		state2 := "normal"
		if i%2 == 1 {
			state2 = "high"
		}
		logs2 = append(logs2, &datastore.PollingLogEnt{
			Time:  ts2,
			State: state2,
			Result: map[string]interface{}{
				"cpu": float64((i + 1) * 10),
			},
		})
	}
	bs1 := buildBucketSeriesFromLogs(logs1, []string{"rtt"}, 60, true)
	bs2 := buildBucketSeriesFromLogs(logs2, []string{"cpu"}, 60, true)
	ts, mat1, mat2 := alignTwoBucketSeries(bs1, bs2)
	if len(ts) != 10 {
		t.Fatalf("expected 10 aligned buckets, got %d", len(ts))
	}
	if len(mat1) != 2 || len(mat2) != 2 {
		t.Fatalf("expected 2 vars each (metric + state), got %d and %d", len(mat1), len(mat2))
	}
	r, _ := pearsonCorr(mat1[0], mat2[0])
	if math.Abs(r-1.0) > 1e-6 {
		t.Fatalf("expected aligned r=1.0, got %f", r)
	}
}

func TestMLCorrelationAlgorithms(t *testing.T) {
	// 1. Nonlinear + lagged relationship: y[t] = sin(x[t-1]) + 0.35*x[t]^2
	n := 120
	x := make([]float64, n)
	y := make([]float64, n)
	noise := make([]float64, n)
	rng := rand.New(rand.NewSource(123))
	for i := 0; i < n; i++ {
		x[i] = math.Sin(float64(i)*0.25)*2.0 + rng.NormFloat64()*0.1
		noise[i] = rng.NormFloat64()
	}
	for i := 1; i < n; i++ {
		y[i] = math.Sin(x[i-1]) + 0.35*x[i]*x[i]
	}

	mlpScore, _, imps := calcMLPCorrelation([]string{"x"}, [][]float64{x}, []string{"y"}, [][]float64{y}, 3)
	if mlpScore < 0.7 {
		t.Fatalf("expected high MLP score for nonlinear lagged relation, got %v", mlpScore)
	}
	if len(imps) == 0 {
		t.Fatalf("expected non-empty feature importances from MLP")
	}

	mlpNoise, _, _ := calcMLPCorrelation([]string{"x"}, [][]float64{x}, []string{"noise"}, [][]float64{noise}, 3)
	if mlpNoise > 0.45 {
		t.Fatalf("expected low MLP score for independent noise, got %v", mlpNoise)
	}

	// 2. LSTM sequential accumulation: y[t] = 0.5*x[t] + 0.35*x[t-1] + 0.25*x[t-2]
	ySeq := make([]float64, n)
	for i := 2; i < n; i++ {
		ySeq[i] = 0.5*x[i] + 0.35*x[i-1] + 0.25*x[i-2]
	}
	lstmScore, _ := calcLSTMCorrelation([]string{"x"}, [][]float64{x}, []string{"ySeq"}, [][]float64{ySeq})
	if lstmScore < 0.65 {
		t.Fatalf("expected high LSTM score for sequential accumulation, got %v", lstmScore)
	}
	lstmNoise, _ := calcLSTMCorrelation([]string{"x"}, [][]float64{x}, []string{"noise"}, [][]float64{noise})
	if lstmNoise > 0.45 {
		t.Fatalf("expected low LSTM score for independent noise, got %v", lstmNoise)
	}

	// 3. kNN Regime classification: distinct regimes of Y driven by X
	yReg := make([]float64, n)
	for i := 0; i < n; i++ {
		ax := math.Abs(x[i])
		if ax < 0.7 {
			yReg[i] = 10.0 + rng.NormFloat64()*0.5
		} else if ax < 1.5 {
			yReg[i] = 50.0 + rng.NormFloat64()*0.5
		} else {
			yReg[i] = 90.0 + rng.NormFloat64()*0.5
		}
	}
	knnScore, _, reg := calcKNNRegimeCorrelation([]string{"x"}, [][]float64{x}, []string{"yReg"}, [][]float64{yReg})
	if knnScore < 0.6 || reg == nil || len(reg.Confusion) == 0 {
		t.Fatalf("expected high kNN regime score and confusion matrix, got score=%v reg=%+v", knnScore, reg)
	}
	knnNoise, _, _ := calcKNNRegimeCorrelation([]string{"x"}, [][]float64{x}, []string{"noise"}, [][]float64{noise})
	if knnNoise > 0.4 {
		t.Fatalf("expected low kNN score for independent noise, got %v", knnNoise)
	}

	// 4. Multi-column polling where 1 column is correlated and 2 columns are independent noise
	noise2 := make([]float64, n)
	noise3 := make([]float64, n)
	for i := 0; i < n; i++ {
		noise2[i] = rng.NormFloat64()
		noise3[i] = rng.NormFloat64()
	}
	multiMLP, _, _ := calcMLPCorrelation(
		[]string{"x", "n1"}, [][]float64{x, noise2},
		[]string{"y", "n2", "n3"}, [][]float64{y, noise, noise3},
		3,
	)
	if multiMLP < 0.35 {
		t.Fatalf("expected multi-column MLP score >= 0.35 even with noisy extra columns, got %v", multiMLP)
	}
}
