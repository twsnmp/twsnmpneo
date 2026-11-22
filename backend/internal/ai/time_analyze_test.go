package ai

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

func TestAnalyzePollingTimeSeries(t *testing.T) {
	ctx := context.Background()
	p := &datastore.PollingEnt{
		ID:      "poll-stl-1",
		Name:    "PING STL Test",
		Type:    "ping",
		PollInt: 60,
	}

	// Empty and single-entry logs return safe empty structures
	emptyRes := AnalyzePollingTimeSeries(ctx, p, nil)
	if len(emptyRes.TimeH) != 0 || len(emptyRes.TimePX2) != 0 {
		t.Fatalf("expected empty time axes for nil logs, got TimeH=%d TimePX2=%d", len(emptyRes.TimeH), len(emptyRes.TimePX2))
	}

	// Generate 72 hours of periodic data (24h seasonality + trend) at 5-minute intervals
	start := time.Now().Add(-72 * time.Hour).Truncate(time.Hour)
	var logs []*datastore.PollingLogEnt
	for m := 0; m < 72*60; m += 5 {
		ts := start.Add(time.Duration(m) * time.Minute)
		hourFloat := float64(m) / 60.0
		// Diurnal sine wave (period 24h) + gentle upward trend (stored in nanoseconds like ping rtt)
		rttMs := 20.0 + 10.0*math.Sin(2*math.Pi*hourFloat/24.0) + 0.05*hourFloat
		logs = append(logs, &datastore.PollingLogEnt{
			Time:      ts.UnixNano(),
			PollingID: p.ID,
			State:     "normal",
			Result: map[string]interface{}{
				"rtt":      rttMs * 1e6,
				"lastTime": ts.UnixNano(),
			},
		})
	}

	res := AnalyzePollingTimeSeries(ctx, p, logs)
	if res.PX2 < 60 {
		t.Fatalf("expected PX2 >= 60, got %d", res.PX2)
	}
	if len(res.TimeH) < 48 {
		t.Fatalf("expected >= 48 hourly bins, got %d", len(res.TimeH))
	}
	if len(res.TimePX2) == 0 || len(res.TimePX2) > 1501 {
		t.Fatalf("expected bounded PX2 grid (1..1501), got %d", len(res.TimePX2))
	}

	stlH, ok := res.StlMapH["rtt"]
	if !ok {
		t.Fatalf("expected StlMapH['rtt'] to be present")
	}
	if len(stlH.Seasonal) != len(res.TimeH) || len(stlH.Trend) != len(res.TimeH) || len(stlH.Resid) != len(res.TimeH) {
		t.Fatalf("STL H component length mismatch: time=%d seasonal=%d trend=%d resid=%d",
			len(res.TimeH), len(stlH.Seasonal), len(stlH.Trend), len(stlH.Resid))
	}
	for i := range stlH.Seasonal {
		if math.IsNaN(stlH.Seasonal[i]) || math.IsNaN(stlH.Trend[i]) || math.IsNaN(stlH.Resid[i]) {
			t.Fatalf("found NaN in STL H at index %d", i)
		}
	}

	stlPX2, ok := res.StlMapPX2["rtt"]
	if !ok || len(stlPX2.Seasonal) != len(res.TimePX2) {
		t.Fatalf("expected valid StlMapPX2['rtt']")
	}

	fftH := res.FFTH["rtt"]
	if len(fftH) == 0 {
		t.Fatalf("expected non-empty FFTH['rtt']")
	}
	// Dominant peak should be close to 3 cycles in 72 hours (24h period => freq ~ 3/72 = 0.0417 cycles/hour, i.e. period ~ 24h)
	var peakFreq, peakAmp float64
	for _, pt := range fftH {
		if math.IsNaN(pt[0]) || math.IsNaN(pt[1]) {
			t.Fatalf("NaN in FFTH output: %+v", pt)
		}
		if pt[0] > 0 && pt[1] > peakAmp {
			peakAmp = pt[1]
			peakFreq = pt[0]
		}
	}
	if peakAmp <= 0 {
		t.Fatalf("expected positive FFT peak amplitude, got %f", peakAmp)
	}
	// Period in hours = (1 / freqHz) / 3600
	if peakFreq > 0 {
		periodHours := (1.0 / peakFreq) / 3600.0
		if periodHours < 18.0 || periodHours > 30.0 {
			t.Errorf("expected dominant period near 24h, got %.2fh (freq=%.7f Hz)", periodHours, peakFreq)
		}
	}

	if len(res.FFTPX2["rtt"]) == 0 {
		t.Fatalf("expected non-empty FFTPX2['rtt']")
	}
	if _, hasLastTime := res.StlMapH["lastTime"]; hasLastTime {
		t.Errorf("expected lastTime metadata field to be excluded from numeric STL keys")
	}
}
