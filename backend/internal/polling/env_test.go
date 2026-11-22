package polling

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/bbolt"
	"github.com/twsnmp/twsnmpneo/backend/internal/logreport"
)

func newEnvTestStore(t *testing.T) datastore.DataStore {
	t.Helper()
	st, err := bbolt.New(filepath.Join(t.TempDir(), "env_test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestEnvHelpers(t *testing.T) {
	// Tokyo Station (35.6812, 139.7671) to Shinjuku Station (35.6896, 139.7006) ~ 6.1 km
	dist := calcDistanceKm(35.6812, 139.7671, 35.6896, 139.7006)
	if dist < 5.5 || dist > 6.8 {
		t.Fatalf("unexpected distance Tokyo-Shinjuku: %.2f km", dist)
	}

	if lat, lon, ok := parseLocLngLat("139.75,35.68"); !ok || lat != 35.68 || lon != 139.75 {
		t.Fatalf("parseLocLngLat lng,lat failed: lat=%v lon=%v ok=%v", lat, lon, ok)
	}
	if lat, lon, ok := parseLocLngLat("35.68,139.75"); !ok || lat != 35.68 || lon != 139.75 {
		t.Fatalf("parseLocLngLat lat,lon auto-fix failed: lat=%v lon=%v ok=%v", lat, lon, ok)
	}
	if _, _, ok := parseLocLngLat(""); ok {
		t.Fatal("empty loc should return ok=false")
	}

	if p2pScaleToFloat(45) != 5.0 || p2pScaleToFloat(50) != 5.5 || p2pScaleToFloat(70) != 7.0 {
		t.Fatal("p2pScaleToFloat mismatch")
	}
	if jmaScaleStrToFloat("5-") != 5.0 || jmaScaleStrToFloat("6+") != 6.5 {
		t.Fatal("jmaScaleStrToFloat mismatch")
	}
	if mmiToJMAIntensity(5.8) != 5.0 || mmiToJMAIntensity(2.0) != 1.0 {
		t.Fatal("mmiToJMAIntensity mismatch")
	}

	score, cls := calcSolarFlareClass(2.5e-5)
	if cls != "M2.5" || score < 3.3 || score > 3.5 {
		t.Fatalf("calcSolarFlareClass(2.5e-5) = (%v, %s)", score, cls)
	}
	scoreX, clsX := calcSolarFlareClass(1.5e-4)
	if clsX != "X1.5" || scoreX < 4.1 {
		t.Fatalf("calcSolarFlareClass(1.5e-4) = (%v, %s)", scoreX, clsX)
	}
	if calcGeomagneticGScale(6.2) != 2 || calcGeomagneticGScale(4.9) != 0 {
		t.Fatal("calcGeomagneticGScale mismatch")
	}

	if lat, lon, depth, ok := parseISO6709("+35.6+140.1-50000/"); !ok || lat != 35.6 || lon != 140.1 || depth != 50.0 {
		t.Fatalf("parseISO6709 failed: lat=%v lon=%v depth=%v ok=%v", lat, lon, depth, ok)
	}
}

func TestEnvPollerQuake(t *testing.T) {
	ClearEnvAPICache()
	ctx := context.Background()
	st := newEnvTestStore(t)
	ep := NewEnvPoller(st)

	nowStr := time.Now().In(jstZone).Format("2006/01/02 15:04:05")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload := []map[string]any{
			{
				"id":   "eq-1",
				"code": 551,
				"time": nowStr,
				"earthquake": map[string]any{
					"time":     nowStr,
					"maxScale": 45, // 5弱
					"hypocenter": map[string]any{
						"name":      "千葉県北西部",
						"latitude":  35.60,
						"longitude": 140.10,
						"depth":     70.0,
						"magnitude": 5.4,
					},
				},
				"points": []map[string]any{
					{"pref": "東京都", "addr": "東京千代田区", "scale": 40},
					{"pref": "千葉県", "addr": "千葉中央区", "scale": 45},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	node := &datastore.NodeEnt{ID: "node-tokyo", Name: "Tokyo-DC", Loc: "139.75,35.68"}
	pe := &datastore.PollingEnt{
		ID:      "poll-quake-1",
		NodeID:  node.ID,
		Name:    "地震監視",
		Type:    "env",
		Mode:    "quake",
		Params:  fmt.Sprintf("source=p2p,pref=東京都,url=%s", srv.URL),
		Script:  `if (intensity >= 5.0) { setLevel("high"); false; } else if (intensity >= 4.0) { setLevel("warn"); false; } else { true; }`,
		Level:   "high",
		PollInt: 60,
		Timeout: 5,
	}

	res, err := ep.Poll(ctx, pe, node)
	if err != nil {
		t.Fatalf("Poll error: %v", err)
	}
	if res.State != StateWarn {
		t.Fatalf("expected StateWarn for Tokyo scale 40, got %s (msg=%s)", res.State, res.Message)
	}
	if res.Fields["intensity"] != 4.0 || res.Fields["max_intensity"] != 5.0 {
		t.Fatalf("unexpected intensity fields: %+v", res.Fields)
	}

	// Verify logreport EarthEnv was recorded
	m, err := st.ListLogReportData(ctx, logreport.KindEarthEnv)
	if err != nil || len(m) != 1 {
		t.Fatalf("expected 1 earthEnv report entry, got %d (err=%v)", len(m), err)
	}
}

func TestEnvPollerQuakeUSGS(t *testing.T) {
	ClearEnvAPICache()
	ctx := context.Background()
	st := newEnvTestStore(t)
	ep := NewEnvPoller(st)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload := map[string]any{
			"type": "FeatureCollection",
			"features": []map[string]any{
				{
					"id": "us1",
					"properties": map[string]any{
						"mag":     5.6,
						"place":   "10km S of San Francisco, CA",
						"time":    time.Now().Add(-5 * time.Minute).UnixMilli(),
						"mmi":     5.8,
						"tsunami": 1,
					},
					"geometry": map[string]any{
						"coordinates": []float64{-122.42, 37.70, 12.0},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	node := &datastore.NodeEnt{ID: "node-sf", Name: "SF-DC", Loc: "-122.4194,37.7749"}
	pe := &datastore.PollingEnt{
		ID:     "poll-quake-usgs",
		NodeID: node.ID,
		Name:   "SF Quake",
		Type:   "env",
		Mode:   "quake",
		Params: fmt.Sprintf("source=auto,radius=100,url=%s", srv.URL),
		Level:  "high",
	}

	res, err := ep.Poll(ctx, pe, node)
	if err != nil {
		t.Fatalf("Poll error: %v", err)
	}
	if res.State != StateHigh {
		t.Fatalf("expected StateHigh for MMI 5.8 (JMA 5.0), got %s", res.State)
	}
	if res.Fields["source"] != "usgs" || res.Fields["tsunami"] != 1.0 {
		t.Fatalf("unexpected USGS fields: %+v", res.Fields)
	}
}

func TestEnvPollerLiden(t *testing.T) {
	ClearEnvAPICache()
	ctx := context.Background()
	st := newEnvTestStore(t)
	ep := NewEnvPoller(st)

	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/times.json" {
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"basetime":  "20261120000000",
					"validtime": "20261120000500",
					"elements":  []string{"liden"},
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type": "FeatureCollection",
			"features": []map[string]any{
				{
					"type":       "Feature",
					"geometry":   map[string]any{"type": "Point", "coordinates": []float64{139.76, 35.69}}, // ~1.4 km from 139.75,35.68
					"properties": map[string]any{"type": 1},                                                 // CG
				},
				{
					"type":       "Feature",
					"geometry":   map[string]any{"type": "Point", "coordinates": []float64{139.90, 35.75}}, // ~15.6 km
					"properties": map[string]any{"type": 0},                                                 // IC
				},
			},
		})
	}))
	defer srv.Close()
	srvURL = srv.URL

	node := &datastore.NodeEnt{ID: "node-tokyo", Name: "Tokyo-DC", Loc: "139.75,35.68"}
	pe := &datastore.PollingEnt{
		ID:     "poll-liden-1",
		NodeID: node.ID,
		Name:   "落雷監視",
		Type:   "env",
		Mode:   "liden",
		Params: fmt.Sprintf("radius=20,warn_radius=10,url=%s/times.json,data_url_fmt=%s/data/%%s/%%s.geojson", srvURL, srvURL),
		Level:  "high",
	}

	res, err := ep.Poll(ctx, pe, node)
	if err != nil {
		t.Fatalf("Poll error: %v", err)
	}
	if res.State != StateHigh {
		t.Fatalf("expected StateHigh, got %s", res.State)
	}
	if res.Fields["strike_count"] != 2.0 || res.Fields["close_count"] != 1.0 || res.Fields["cg_count"] != 1.0 || res.Fields["ic_count"] != 1.0 {
		t.Fatalf("unexpected liden counts: %+v", res.Fields)
	}
}

func TestEnvPollerSpace(t *testing.T) {
	ClearEnvAPICache()
	ctx := context.Background()
	st := newEnvTestStore(t)
	ep := NewEnvPoller(st)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/xray.json" {
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"time_tag": "2026-11-20T00:00:00Z", "flux": 3.2e-5, "energy": "0.1-0.8nm"},
			})
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"time_tag": "2026-11-20T00:00:00Z", "kp_index": 5, "estimated_kp": 5.33},
		})
	}))
	defer srv.Close()

	pe := &datastore.PollingEnt{
		ID:     "poll-space-1",
		NodeID: "node-1",
		Name:   "宇宙天気",
		Type:   "env",
		Mode:   "space",
		Params: fmt.Sprintf("xray_url=%s/xray.json,kp_url=%s/kp.json", srv.URL, srv.URL),
		Level:  "warn",
	}

	res, err := ep.Poll(ctx, pe, nil)
	if err != nil {
		t.Fatalf("Poll error: %v", err)
	}
	if res.State != StateWarn {
		t.Fatalf("expected StateWarn for M3.2 / Kp 5.33, got %s", res.State)
	}
	if res.Fields["flare_class"] != "M3.2" || res.Fields["g_scale"] != 1.0 {
		t.Fatalf("unexpected space fields: %+v", res.Fields)
	}
}

func TestEnvPollerWeatherAndSyncEnvMonitor(t *testing.T) {
	ClearEnvAPICache()
	ctx := context.Background()
	st := newEnvTestStore(t)
	ep := NewEnvPoller(st)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"latitude":  35.68,
			"longitude": 139.75,
			"current": map[string]any{
				"time":                 "2026-11-20T09:00",
				"temperature_2m":       28.5,
				"relative_humidity_2m": 72.0,
				"surface_pressure":     998.0,
				"pressure_msl":         1000.0,
				"wind_speed_10m":       10.0,
				"wind_gusts_10m":       18.0,
				"precipitation":        2.5,
				"weather_code":         95,
				"cape":                 1400.0,
			},
			"hourly": map[string]any{
				"time":             []string{"t-5", "t-4", "t-3", "t-2", "t-1", "t0"},
				"surface_pressure": []float64{1006.0, 1005.0, 1004.0, 1002.0, 1000.0, 998.0},
				"cape":             []float64{500, 600, 800, 1000, 1200, 1400},
			},
		})
	}))
	defer srv.Close()

	node := &datastore.NodeEnt{ID: "node-tokyo", Name: "Tokyo-DC", Loc: "139.75,35.68"}
	pe := &datastore.PollingEnt{
		ID:     "poll-weather-1",
		NodeID: node.ID,
		Name:   "気象監視",
		Type:   "env",
		Mode:   "weather",
		Params: fmt.Sprintf("url=%s", srv.URL),
		Level:  "warn",
	}

	res, err := ep.Poll(ctx, pe, node)
	if err != nil {
		t.Fatalf("Poll error: %v", err)
	}
	if res.State != StateWarn {
		t.Fatalf("expected StateWarn, got %s", res.State)
	}
	// 998.0 - 1004.0 (index 5-3=2) = -6.0
	if res.Fields["pressure_diff_3h"] != -6.0 || res.Fields["temp"] != 28.5 {
		t.Fatalf("unexpected weather fields: %+v", res.Fields)
	}

	envMap, err := st.ListLogReportData(ctx, logreport.KindEnvMonitor)
	if err != nil || len(envMap) != 1 {
		t.Fatalf("expected 1 mirrored KindEnvMonitor entry, got %d (err=%v)", len(envMap), err)
	}
}

func TestEnvAPICacheSingleflightAndStale(t *testing.T) {
	cache := newEnvAPICache()
	cache.hostInterval = 0

	var hits atomic.Int32
	var fail atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if fail.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		time.Sleep(30 * time.Millisecond)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	// Concurrent requests should collapse to 1 HTTP call via singleflight
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body, _, _, err := cache.fetchWithCache(context.Background(), "sf-key", srv.URL, 50*time.Millisecond, 5)
			if err != nil || string(body) != `{"ok":true}` {
				t.Errorf("unexpected fetch result: body=%s err=%v", string(body), err)
			}
		}()
	}
	wg.Wait()
	if hits.Load() != 1 {
		t.Fatalf("expected 1 HTTP hit via singleflight, got %d", hits.Load())
	}

	// Expire TTL and force server 503 -> should return stale cache without error
	time.Sleep(70 * time.Millisecond)
	fail.Store(true)
	body, fromCache, _, err := cache.fetchWithCache(context.Background(), "sf-key", srv.URL, 50*time.Millisecond, 5)
	if err != nil || !fromCache || string(body) != `{"ok":true}` {
		t.Fatalf("expected stale-if-error fallback, got fromCache=%v body=%s err=%v", fromCache, string(body), err)
	}
}
