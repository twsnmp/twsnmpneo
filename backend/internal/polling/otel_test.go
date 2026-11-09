package polling

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/bbolt"
)

func TestParseOTelParams(t *testing.T) {
	// Key-value pairs
	params1 := "service=frontend, name=http_requests_total, host=web01, scope=http"
	pMap1 := parseOTelParams(params1)
	if pMap1["service"] != "frontend" || pMap1["name"] != "http_requests_total" || pMap1["host"] != "web01" || pMap1["scope"] != "http" {
		t.Errorf("parseOTelParams failed for KV: %+v", pMap1)
	}

	// Positional format: host,service,scope,name
	params2 := "server1,auth-service,grpc,user_login"
	pMap2 := parseOTelParams(params2)
	if pMap2["host"] != "server1" || pMap2["service"] != "auth-service" || pMap2["scope"] != "grpc" || pMap2["name"] != "user_login" {
		t.Errorf("parseOTelParams failed for positional: %+v", pMap2)
	}
}

func TestIsOTelSpanError(t *testing.T) {
	span1 := datastore.OTelTraceSpanEnt{
		Attributes: []string{"http.status_code=500", "http.method=GET"},
	}
	if !isOTelSpanError(span1) {
		t.Errorf("expected span1 to be error (500)")
	}

	span2 := datastore.OTelTraceSpanEnt{
		Attributes: []string{"http.status_code=200", "http.method=GET"},
	}
	if isOTelSpanError(span2) {
		t.Errorf("expected span2 to be normal (200)")
	}

	span3 := datastore.OTelTraceSpanEnt{
		Attributes: []string{"status_code=STATUS_CODE_ERROR"},
	}
	if !isOTelSpanError(span3) {
		t.Errorf("expected span3 to be error (STATUS_CODE_ERROR)")
	}

	span4 := datastore.OTelTraceSpanEnt{
		Attributes: []string{"error=true"},
	}
	if !isOTelSpanError(span4) {
		t.Errorf("expected span4 to be error (error=true)")
	}

	span5 := datastore.OTelTraceSpanEnt{
		Attributes: []string{"rpc.grpc.status_code=2"},
	}
	if !isOTelSpanError(span5) {
		t.Errorf("expected span5 to be error (grpc error code 2)")
	}
}

func TestOTelPoller_Metric(t *testing.T) {
	store, err := bbolt.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("failed to create bbolt store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	poller := NewOTelPoller(store)

	// Save test metric
	m := &datastore.OTelMetricEnt{
		Host:    "web-01",
		Service: "web-app",
		Scope:   "metrics",
		Name:    "jvm_memory_used",
		Type:    "Gauge",
		First:   time.Now().UnixNano(),
		Last:    time.Now().UnixNano(),
		Count:   10,
		DataPoints: []*datastore.OTelMetricDataPointEnt{
			{
				Time:  time.Now().UnixNano(),
				Gauge: 85.5,
				Count: 1,
			},
		},
	}
	if err := store.SaveOTelMetric(ctx, m); err != nil {
		t.Fatalf("save metric failed: %v", err)
	}

	// 1. Script fails threshold
	pe := &datastore.PollingEnt{
		ID:      "test-otel-metric",
		Type:    "otel",
		Mode:    "metric",
		Params:  "service=web-app,name=jvm_memory_used",
		Script:  "gauge < 80.0",
		Level:   "warn",
		PollInt: 60,
		Result:  make(map[string]any),
	}

	res, err := poller.Poll(ctx, pe, nil)
	if err != nil {
		t.Fatalf("poll failed: %v", err)
	}
	if res.State != "warn" {
		t.Errorf("expected state warn for gauge=85.5 with script 'gauge < 80.0', got %s", res.State)
	}

	// 2. Script passes threshold
	pe.Script = "gauge < 90.0"
	res, err = poller.Poll(ctx, pe, nil)
	if err != nil {
		t.Fatalf("poll failed: %v", err)
	}
	if res.State != StateNormal {
		t.Errorf("expected state normal for gauge=85.5 with script 'gauge < 90.0', got %s", res.State)
	}

	// 3. Metric not found
	pe.Params = "service=nonexistent,name=jvm"
	res, err = poller.Poll(ctx, pe, nil)
	if err != nil {
		t.Fatalf("poll failed: %v", err)
	}
	if res.State != "warn" {
		t.Errorf("expected state warn for nonexistent metric, got %s", res.State)
	}
}

func TestOTelPoller_Trace(t *testing.T) {
	store, err := bbolt.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("failed to create bbolt store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	poller := NewOTelPoller(store)

	now := time.Now()
	bucket := now.Format("2006-01-02T15:04")

	trace := &datastore.OTelTraceEnt{
		Bucket:  bucket,
		TraceID: "trace-001",
		Start:   now.Add(-10 * time.Second).UnixNano(),
		End:     now.UnixNano(),
		Dur:     0.5,
		Spans: []datastore.OTelTraceSpanEnt{
			{
				SpanID:     "s1",
				Service:    "payment",
				Name:       "/checkout",
				Start:      now.Add(-10 * time.Second).UnixNano(),
				End:        now.Add(-9 * time.Second).UnixNano(),
				Dur:        0.2,
				Attributes: []string{"http.status_code=200"},
			},
			{
				SpanID:     "s2",
				Service:    "payment",
				Name:       "/checkout",
				Start:      now.Add(-9 * time.Second).UnixNano(),
				End:        now.Add(-8 * time.Second).UnixNano(),
				Dur:        1.2,
				Attributes: []string{"http.status_code=500"}, // Error span
			},
		},
	}

	if err := store.SaveOTelTraces(ctx, []*datastore.OTelTraceEnt{trace}); err != nil {
		t.Fatalf("save traces failed: %v", err)
	}

	pe := &datastore.PollingEnt{
		ID:      "test-otel-trace",
		Type:    "otel",
		Mode:    "trace",
		Params:  "service=payment",
		Script:  "errorRate < 10.0 && p95Dur < 1.0",
		Level:   "high",
		PollInt: 60,
		Result:  make(map[string]any),
	}

	// errorRate is 50%, p95Dur is 1.2 -> should fail script
	res, err := poller.Poll(ctx, pe, nil)
	if err != nil {
		t.Fatalf("poll failed: %v", err)
	}
	if res.State != "high" {
		t.Errorf("expected state high for errorRate=50%%, got %s", res.State)
	}

	// Script that tolerates higher error rate
	pe.Script = "errorRate <= 50.0 && p95Dur <= 2.0"
	res, err = poller.Poll(ctx, pe, nil)
	if err != nil {
		t.Fatalf("poll failed: %v", err)
	}
	if res.State != StateNormal {
		t.Errorf("expected state normal for relaxed script, got %s", res.State)
	}
}

func TestOTelPoller_Heartbeat(t *testing.T) {
	store, err := bbolt.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("failed to create bbolt store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	poller := NewOTelPoller(store)

	// Save fresh metric
	m := &datastore.OTelMetricEnt{
		Host:    "web-01",
		Service: "order-service",
		Scope:   "metrics",
		Name:    "active_orders",
		Last:    time.Now().UnixNano(),
		Count:   5,
	}
	if err := store.SaveOTelMetric(ctx, m); err != nil {
		t.Fatalf("save metric failed: %v", err)
	}

	pe := &datastore.PollingEnt{
		ID:      "test-otel-hb",
		Type:    "otel",
		Mode:    "heartbeat",
		State:   "unknown",
		Params:  "service=order-service",
		Filter:  "10", // 10 seconds threshold
		Level:   "high",
		PollInt: 60,
		Result:  make(map[string]any),
	}

	// Should be normal because Last is fresh
	res, err := poller.Poll(ctx, pe, nil)
	if err != nil {
		t.Fatalf("poll failed: %v", err)
	}
	if res.State != StateNormal {
		t.Errorf("expected state normal for fresh heartbeat, got %s", res.State)
	}

	// Update Last to 20 seconds ago -> should exceed 10s threshold
	m.Last = time.Now().Add(-20 * time.Second).UnixNano()
	_ = store.SaveOTelMetric(ctx, m)

	res, err = poller.Poll(ctx, pe, nil)
	if err != nil {
		t.Fatalf("poll failed: %v", err)
	}
	if res.State != "high" {
		t.Errorf("expected state high for stale heartbeat, got %s", res.State)
	}
}
