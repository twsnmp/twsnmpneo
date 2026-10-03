package ai

import (
	"math"
	"testing"
)

func TestMakeDeviationScore(t *testing.T) {
	req := &AIReq{
		PollingID: "test-poll",
		Df: AIDataFrame{
			Time: []int64{100, 200, 300},
			Data: map[string][]float64{
				"v": {10, 20, 30},
			},
		},
	}
	r := []float64{1.0, 2.0, 3.0}
	res := makeDeviationScore(req, r)
	if len(res.ScoreData) != 3 {
		t.Fatalf("expected 3 items, got %d", len(res.ScoreData))
	}

	if math.Abs(res.ScoreData[0][1]-37.75) > 0.1 {
		t.Errorf("expected ~37.75, got %f", res.ScoreData[0][1])
	}
	if math.Abs(res.ScoreData[1][1]-50.0) > 0.1 {
		t.Errorf("expected ~50.0, got %f", res.ScoreData[1][1])
	}
	if math.Abs(res.ScoreData[2][1]-62.25) > 0.1 {
		t.Errorf("expected ~62.25, got %f", res.ScoreData[2][1])
	}

	// Constant value check
	rConst := []float64{2.0, 2.0, 2.0}
	resConst := makeDeviationScore(req, rConst)
	for _, score := range resConst.ScoreData {
		if score[1] != 50.0 {
			t.Errorf("expected constant score to be 50.0, got %f", score[1])
		}
	}
}

func TestCalcHotelling(t *testing.T) {
	times := make([]int64, 20)
	v1 := make([]float64, 20)
	v2 := make([]float64, 20)
	for i := 0; i < 20; i++ {
		times[i] = int64(1000 + i*10)
		v1[i] = float64(i)
		v2[i] = float64(i * 2)
	}
	// Introduce one outlier at index 10
	v1[10] = 100.0

	req := &AIReq{
		PollingID: "test-hotelling",
		Df: AIDataFrame{
			Time: times,
			Data: map[string][]float64{
				"v1": v1,
				"v2": v2,
			},
		},
	}

	res := calcHotelling(req)
	if len(res.ScoreData) != 20 {
		t.Fatalf("expected 20 items, got %d", len(res.ScoreData))
	}
	maxScore := 0.0
	maxIdx := -1
	for i, sd := range res.ScoreData {
		if sd[1] > maxScore {
			maxScore = sd[1]
			maxIdx = i
		}
	}
	if maxIdx != 10 {
		t.Errorf("expected outlier at index 10 to have max score, but got index %d with score %f", maxIdx, maxScore)
	}
}

func TestAlgorithmsSmoke(t *testing.T) {
	times := make([]int64, 15)
	v1 := make([]float64, 15)
	for i := 0; i < 15; i++ {
		times[i] = int64(1000 + i*10)
		v1[i] = float64(i % 5)
	}
	v1[7] = 25.0 // outlier

	req := &AIReq{
		PollingID: "test-smoke",
		Df: AIDataFrame{
			Time: times,
			Data: map[string][]float64{
				"v1": v1,
			},
		},
	}

	modes := []string{"iforest", "zscore", "knn", "mahalanobis", "hotelling", "lof", "autoencoder", "lstm"}
	for _, m := range modes {
		res := CalcAIScore(req, m)
		if len(res.ScoreData) != 15 {
			t.Errorf("mode %s expected 15 score data points, got %d", m, len(res.ScoreData))
		}
	}
}
