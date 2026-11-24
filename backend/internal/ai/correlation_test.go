package ai

import (
	"math"
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
