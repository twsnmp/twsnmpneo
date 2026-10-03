package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/ai"
	"github.com/twsnmp/twsnmpneo/backend/internal/api"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/bbolt"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	"github.com/twsnmp/twsnmpneo/backend/internal/polling"
)

func setupTestAPIEnv(t *testing.T) (datastore.DataStore, *parquet.Store, *ai.MCPServer, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "twsnmpneo-api-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}

	bStore, err := bbolt.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("create bbolt store: %v", err)
	}

	pqStore, err := parquet.New(parquet.Config{
		Dir:            filepath.Join(dir, "logs"),
		BufferSize:     2,
		BufferInterval: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("create parquet store: %v", err)
	}

	mcpSvr := ai.NewMCPServer(ai.MCPConfig{
		Store:    bStore,
		LogStore: pqStore,
		Version:  "v0.1.0-api-test",
	})

	cleanup := func() {
		_ = bStore.Close()
		_ = pqStore.Close()
		_ = os.RemoveAll(dir)
	}
	return bStore, pqStore, mcpSvr, cleanup
}

func TestAPIServer_Endpoints(t *testing.T) {
	bStore, pqStore, mcpSvr, cleanup := setupTestAPIEnv(t)
	defer cleanup()

	srv, err := api.NewServer(api.Config{
		Port:      9099,
		Debug:     true,
		Version:   "v0.1.0-test",
		Store:     bStore,
		LogStore:  pqStore,
		MCPServer: mcpSvr,
	})
	if err != nil {
		t.Fatalf("create api server failed: %v", err)
	}

	e := srv.GetEcho()

	// 1. GET /api/health
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("health returned status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "v0.1.0-test") {
		t.Errorf("expected version in health response: %s", rec.Body.String())
	}

	// 2. Nodes CRUD: POST, GET, GET/:id, DELETE/:id
	nodePayload := `{"id":"n-api-1","name":"Core Router","ip":"192.168.1.1","state":"normal"}`
	req = httptest.NewRequest(http.MethodPost, "/api/nodes", strings.NewReader(nodePayload))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("post node returned %d: %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/nodes", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Core Router") {
		t.Errorf("get nodes failed: code %d, body: %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/nodes/n-api-1", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "192.168.1.1") {
		t.Errorf("get node by id failed: code %d, body: %s", rec.Code, rec.Body.String())
	}

	// 2.5 Test node creation with addr_mode="host" and no auto-created ping
	hostNodePayload := `{"id":"n-host-1","name":"localhost","addr_mode":"host"}`
	req = httptest.NewRequest(http.MethodPost, "/api/nodes", strings.NewReader(hostNodePayload))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("post host node returned %d: %s", rec.Code, rec.Body.String())
	}
	var createdHostNode datastore.NodeEnt
	if err := json.Unmarshal(rec.Body.Bytes(), &createdHostNode); err != nil {
		t.Fatalf("unmarshal created host node: %v", err)
	}
	if createdHostNode.IP == "" {
		t.Errorf("expected localhost to resolve to IP, got empty")
	}
	// Verify no automatic ping was created
	allPolls, _ := bStore.ListPollings(context.Background())
	for _, p := range allPolls {
		if p.NodeID == "n-host-1" {
			t.Errorf("expected no auto-created polling for node n-host-1, but found %s", p.Name)
		}
	}


	// 3. Pollings CRUD: POST, GET, DELETE/:id
	pollPayload := `{"ID":"p-api-1","NodeID":"n-api-1","Name":"Ping Check","Type":"ping","Mode":"smoke","Params":"count=5,size=128","Filter":"filter","Extractor":"extractor","Script":"loss < 100","Level":"warn","PollInt":45,"Timeout":2,"Retry":3,"LogMode":2,"FailAction":"failure action","RepairAction":"repair action","AIMode":"zscore","VectorCols":"rtt,loss","MqttURL":"tcp://localhost:1883","MqttTopic":"polling","MqttCols":"state,rtt","State":"normal"}`
	req = httptest.NewRequest(http.MethodPost, "/api/pollings", strings.NewReader(pollPayload))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("post polling returned %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/pollings", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Ping Check") ||
		!strings.Contains(rec.Body.String(), `"Mode":"smoke"`) ||
		!strings.Contains(rec.Body.String(), `"Params":"count=5,size=128"`) ||
		!strings.Contains(rec.Body.String(), `"Filter":"filter"`) ||
		!strings.Contains(rec.Body.String(), `"Extractor":"extractor"`) ||
		!strings.Contains(rec.Body.String(), `"Script":"loss \u003c 100"`) ||
		!strings.Contains(rec.Body.String(), `"Level":"warn"`) ||
		!strings.Contains(rec.Body.String(), `"PollInt":45`) ||
		!strings.Contains(rec.Body.String(), `"Timeout":2`) ||
		!strings.Contains(rec.Body.String(), `"Retry":3`) ||
		!strings.Contains(rec.Body.String(), `"LogMode":2`) ||
		!strings.Contains(rec.Body.String(), `"FailAction":"failure action"`) ||
		!strings.Contains(rec.Body.String(), `"RepairAction":"repair action"`) ||
		!strings.Contains(rec.Body.String(), `"AIMode":"zscore"`) ||
		!strings.Contains(rec.Body.String(), `"VectorCols":"rtt,loss"`) ||
		!strings.Contains(rec.Body.String(), `"MqttURL":"tcp://localhost:1883"`) ||
		!strings.Contains(rec.Body.String(), `"MqttTopic":"polling"`) ||
		!strings.Contains(rec.Body.String(), `"MqttCols":"state,rtt"`) {
		t.Errorf("get pollings failed: code %d", rec.Code)
	}

	// 4. Lines: POST, GET
	linePayload := `{"id":"l-api-1","node_id1":"n-api-1","node_id2":"n-api-2","state":"normal"}`
	req = httptest.NewRequest(http.MethodPost, "/api/lines", strings.NewReader(linePayload))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("post line returned %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/lines", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "l-api-1") {
		t.Errorf("get lines failed: code %d", rec.Code)
	}

	// 5. Networks: POST, GET
	netPayload := `{"id":"net-api-1","name":"LAN 1","ip":"192.168.1.0/24"}`
	req = httptest.NewRequest(http.MethodPost, "/api/networks", strings.NewReader(netPayload))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("post network returned %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/networks", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "LAN 1") {
		t.Errorf("get networks failed: code %d", rec.Code)
	}

	// 5.5 DrawItems: POST, GET, COPY, DELETE
	itemPayload := `{"id":"di-test-1","type":11,"text":"KPI Metric","x":150,"y":220,"w":220,"h":84,"color":"#00d2ffff"}`
	req = httptest.NewRequest(http.MethodPost, "/api/drawitems", strings.NewReader(itemPayload))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("post drawitem returned %d", rec.Code)
	}

	// Test Copy: POST /api/drawitems/di-test-1/copy
	req = httptest.NewRequest(http.MethodPost, "/api/drawitems/di-test-1/copy", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("copy drawitem returned %d: %s", rec.Code, rec.Body.String())
	}
	var copiedItem datastore.DrawItemEnt
	if err := json.Unmarshal(rec.Body.Bytes(), &copiedItem); err != nil {
		t.Fatalf("unmarshal copied item failed: %v", err)
	}
	if copiedItem.ID == "di-test-1" || copiedItem.ID == "" {
		t.Errorf("expected new ID for copied item, got %s", copiedItem.ID)
	}
	if copiedItem.X != 250 || copiedItem.Y != 220 {
		t.Errorf("expected copied coords (250, 220), got (%d, %d)", copiedItem.X, copiedItem.Y)
	}
	if copiedItem.Text != "KPI Metric" || copiedItem.Type != 11 {
		t.Errorf("expected preserved text and type, got text=%s type=%d", copiedItem.Text, copiedItem.Type)
	}

	// Verify both items present in GET /api/drawitems
	req = httptest.NewRequest(http.MethodGet, "/api/drawitems", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), copiedItem.ID) {
		t.Errorf("expected copied item in list, got %s", rec.Body.String())
	}

	// Delete copied item
	req = httptest.NewRequest(http.MethodDelete, "/api/drawitems/"+copiedItem.ID, nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("delete drawitem returned %d", rec.Code)
	}

	// 6. Map Conf: GET, POST
	confPayload := `{"MapName":"Test Network Map","LLMProvider":"local","LLMModel":"tensai-1"}`
	req = httptest.NewRequest(http.MethodPost, "/api/map/conf", strings.NewReader(confPayload))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("post map conf returned %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/map/conf", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Test Network Map") {
		t.Errorf("get map conf failed: code %d", rec.Code)
	}

	// 7. Event Logs & Parquet Query
	_ = bStore.AddEventLog(context.Background(), &datastore.EventLogEnt{
		Time:  time.Now().UnixNano(),
		Type:  "system",
		Level: "info",
		Event: "Server Started",
	})
	req = httptest.NewRequest(http.MethodGet, "/api/logs/events?level=info&limit=100", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Server Started") {
		t.Errorf("get event logs failed: code %d", rec.Code)
	}

	_ = pqStore.WriteLog(&parquet.ParquetLogRecord{
		Time: time.Now().UnixNano(),
		Type: "syslog",
		Src:  "192.168.1.1",
		Log:  `{"msg":"interface up"}`,
	})
	_ = pqStore.Flush()

	req = httptest.NewRequest(http.MethodGet, "/api/logs/query?type=syslog&limit=50", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "interface up") {
		t.Errorf("query logs failed: code %d, body: %s", rec.Code, rec.Body.String())
	}

	// Test DELETE /api/logs/events
	delEvReq := httptest.NewRequest(http.MethodDelete, "/api/logs/events", nil)
	delEvRec := httptest.NewRecorder()
	e.ServeHTTP(delEvRec, delEvReq)
	if delEvRec.Code != http.StatusOK {
		t.Errorf("delete event logs returned %d", delEvRec.Code)
	}

	// Test DELETE /api/logs/query?type=syslog
	delPqReq := httptest.NewRequest(http.MethodDelete, "/api/logs/query?type=syslog", nil)
	delPqRec := httptest.NewRecorder()
	e.ServeHTTP(delPqRec, delPqReq)
	if delPqRec.Code != http.StatusOK {
		t.Errorf("delete parquet logs returned %d", delPqRec.Code)
	}

	// 8. AI Ask & Diagnose (using local tensai provider configured above)
	askPayload := `{"prompt":"Check link status","system":"system instructions"}`
	req = httptest.NewRequest(http.MethodPost, "/api/ai/ask", strings.NewReader(askPayload))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Local Model Response") {
		t.Errorf("ai ask failed: code %d, body: %s", rec.Code, rec.Body.String())
	}

	diagPayload := `{"alert_event":"High Memory Usage","node_context":"Router 10.0.0.1"}`
	req = httptest.NewRequest(http.MethodPost, "/api/ai/diagnose", strings.NewReader(diagPayload))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Local Model Response") {
		t.Errorf("ai diagnose failed: code %d, body: %s", rec.Code, rec.Body.String())
	}

	// 8b. AI Anomaly Report Endpoints
	req = httptest.NewRequest(http.MethodGet, "/api/ai/list", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("ai list failed: code %d, body: %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/ai/result/p-api-1", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("ai result failed: code %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/ai/result/p-api-1", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("ai delete result failed: code %d", rec.Code)
	}

	// 9. MCP Endpoint
	req = httptest.NewRequest(http.MethodGet, "/api/mcp", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("mcp endpoint returned %d", rec.Code)
	}

	// 10. Delete Polling and Node
	req = httptest.NewRequest(http.MethodDelete, "/api/pollings/p-api-1", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("delete polling returned %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/nodes/n-api-1", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("delete node returned %d", rec.Code)
	}

	// 11. Not found and bad requests
	req = httptest.NewRequest(http.MethodGet, "/api/nodes/non-existent-id", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for non existent node, got %d", rec.Code)
	}

	badReqs := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/nodes"},
		{http.MethodPost, "/api/pollings"},
		{http.MethodPost, "/api/lines"},
		{http.MethodPost, "/api/networks"},
		{http.MethodPost, "/api/map/conf"},
		{http.MethodPost, "/api/ai/ask"},
		{http.MethodPost, "/api/ai/diagnose"},
	}
	for _, br := range badReqs {
		r := httptest.NewRequest(br.method, br.path, strings.NewReader(`{invalid json`))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		e.ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for %s %s, got %d", br.method, br.path, w.Code)
		}
	}

	// 12. Static file / SPA routing
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	// Could be 200 or 404 depending on whether static assets are present
	if rec.Code != http.StatusOK && rec.Code != http.StatusNotFound {
		t.Errorf("unexpected status for root path: %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/dashboard/map", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK && rec.Code != http.StatusNotFound {
		t.Errorf("unexpected status for SPA path: %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/pki/not-registered", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound || strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
		t.Errorf("unexpected response for unknown API path: status=%d content-type=%q", rec.Code, rec.Header().Get("Content-Type"))
	}

	// 10. Test GeoIP endpoints
	req = httptest.NewRequest(http.MethodDelete, "/api/conf/geoip", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for DELETE /api/conf/geoip, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/map/conf", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for GET /api/map/conf, got %d", rec.Code)
	}

	// 11. Test ARP endpoints (GET, DELETE single, DELETE all)
	_ = bStore.SaveArpTable(context.Background(), []*datastore.ArpEnt{
		{IP: "192.168.1.100", MAC: "00:11:22:33:44:55"},
		{IP: "192.168.1.101", MAC: "00:11:22:33:44:66"},
	})
	req = httptest.NewRequest(http.MethodGet, "/api/arp", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for GET /api/arp, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/arp?ip=192.168.1.100", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for DELETE /api/arp?ip=..., got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/arp?all=true", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for DELETE /api/arp?all=true, got %d", rec.Code)
	}
}

func TestAPIServer_StartShutdown(t *testing.T) {
	srv, err := api.NewServer(api.Config{
		Port:    19098,
		Version: "v0.1.0-test",
	})
	if err != nil {
		t.Fatalf("new server failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	// Wait briefly for server to bind
	time.Sleep(100 * time.Millisecond)

	// Cancel context to trigger shutdown
	cancel()

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			t.Fatalf("unexpected shutdown error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server shutdown timed out")
	}
}

func TestAPIServer_OTelEndpoints(t *testing.T) {
	bStore, pqStore, mcpSvr, cleanup := setupTestAPIEnv(t)
	defer cleanup()

	srv, err := api.NewServer(api.Config{
		Port:      9099,
		Version:   "v0.1.0-api-test",
		Store:     bStore,
		LogStore:  pqStore,
		MCPServer: mcpSvr,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	e := srv.GetEcho()
	ctx := context.Background()

	// Pre-populate metric and trace
	_ = bStore.SaveOTelMetric(ctx, &datastore.OTelMetricEnt{
		Host:    "host-1",
		Service: "order-service",
		Scope:   "orders",
		Name:    "order.count",
		Type:    "Sum",
		Count:   10,
		DataPoints: []*datastore.OTelMetricDataPointEnt{
			{Time: time.Now().UnixNano(), Sum: 100},
		},
	})
	_ = bStore.SaveOTelTraces(ctx, []*datastore.OTelTraceEnt{
		{
			Bucket:  "2026-09-23T10:00",
			TraceID: "trace-12345",
			Start:   time.Now().UnixNano(),
			End:     time.Now().UnixNano() + 1000000,
			Dur:     0.001,
			Spans: []datastore.OTelTraceSpanEnt{
				{
					SpanID:  "span-1",
					Host:    "host-1",
					Service: "order-service",
					Name:    "process-order",
				},
			},
		},
	})

	// Test GET /api/otel/metrics
	req := httptest.NewRequest(http.MethodGet, "/api/otel/metrics", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "order.count") {
		t.Errorf("GET /api/otel/metrics failed: code %d, body %s", rec.Code, rec.Body.String())
	}

	// Test GET /api/otel/metrics/detail
	req = httptest.NewRequest(http.MethodGet, "/api/otel/metrics/detail?host=host-1&service=order-service&scope=orders&name=order.count", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "order.count") {
		t.Errorf("GET /api/otel/metrics/detail failed: code %d, body %s", rec.Code, rec.Body.String())
	}

	// Test GET /api/otel/traces/buckets
	req = httptest.NewRequest(http.MethodGet, "/api/otel/traces/buckets", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "2026-09-23T10:00") {
		t.Errorf("GET /api/otel/traces/buckets failed: code %d, body %s", rec.Code, rec.Body.String())
	}

	// Test GET /api/otel/traces
	req = httptest.NewRequest(http.MethodGet, "/api/otel/traces?bucket=2026-09-23T10:00", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "trace-12345") {
		t.Errorf("GET /api/otel/traces failed: code %d, body %s", rec.Code, rec.Body.String())
	}

	// Test GET /api/otel/traces/detail
	req = httptest.NewRequest(http.MethodGet, "/api/otel/traces/detail?bucket=2026-09-23T10:00&traceId=trace-12345", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "trace-12345") {
		t.Errorf("GET /api/otel/traces/detail failed: code %d, body %s", rec.Code, rec.Body.String())
	}

	// Test POST /api/otel/traces/dag
	req = httptest.NewRequest(http.MethodPost, "/api/otel/traces/dag", strings.NewReader(`{"buckets":["2026-09-23T10:00"]}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "order-service") {
		t.Errorf("POST /api/otel/traces/dag failed: code %d, body %s", rec.Code, rec.Body.String())
	}

	// Test DELETE /api/otel/all
	req = httptest.NewRequest(http.MethodDelete, "/api/otel/all", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("DELETE /api/otel/all failed: code %d", rec.Code)
	}
}

func TestAPIServer_MqttEndpoints(t *testing.T) {
	bStore, pqStore, mcpSvr, cleanup := setupTestAPIEnv(t)
	defer cleanup()
	ctx := context.Background()

	// Seed MQTT stat and log
	stat := &datastore.MqttStatEnt{
		ID:       "stat-api-1",
		ClientID: "test-client",
		Topic:    "devices/temp",
		Remote:   "192.168.1.100",
		Count:    10,
		Bytes:    2048,
		First:    time.Now().UnixNano(),
		Last:     time.Now().UnixNano(),
		Value:    "25.2",
	}
	_ = bStore.SaveMqttStat(ctx, stat)

	_ = pqStore.WriteLog(&parquet.ParquetLogRecord{
		Time: time.Now().UnixNano(),
		Type: "mqtt",
		Src:  "192.168.1.100",
		Log:  `{"topic":"devices/temp","clientID":"test-client","remote":"192.168.1.100","payload":"25.2"}`,
	})
	_ = pqStore.Flush()

	srv, err := api.NewServer(api.Config{
		Port:      9099,
		Store:     bStore,
		LogStore:  pqStore,
		MCPServer: mcpSvr,
	})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	e := srv.GetEcho()

	// 1. GET /api/mqtt/stats
	req := httptest.NewRequest(http.MethodGet, "/api/mqtt/stats", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /api/mqtt/stats failed: code %d, body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "devices/temp") {
		t.Errorf("expected devices/temp in response, got %s", rec.Body.String())
	}

	// 2. GET /api/logs/query?type=mqtt
	req = httptest.NewRequest(http.MethodGet, "/api/logs/query?type=mqtt", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /api/logs/query?type=mqtt failed: code %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "test-client") {
		t.Errorf("expected test-client in mqtt logs query response, got %s", rec.Body.String())
	}

	// 3. DELETE /api/mqtt/stats?id=stat-api-1
	req = httptest.NewRequest(http.MethodDelete, "/api/mqtt/stats?id=stat-api-1", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("DELETE /api/mqtt/stats failed: code %d", rec.Code)
	}

	// 4. DELETE /api/mqtt/stats/all
	_ = bStore.SaveMqttStat(ctx, stat)
	req = httptest.NewRequest(http.MethodDelete, "/api/mqtt/stats/all", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("DELETE /api/mqtt/stats/all failed: code %d", rec.Code)
	}
}

func TestIPAMAPI(t *testing.T) {
	ctx := context.Background()
	bStore, _, _, cleanup := setupTestAPIEnv(t)
	defer cleanup()

	// Configure multiple ranges in MapConf
	mapConf := &datastore.MapConfEnt{
		ArpWatchRange: "192.168.1.0/24, 10.0.0.0/16, 172.16.1.10-172.16.1.20",
	}
	_ = bStore.SaveMapConf(ctx, mapConf)

	// Save test nodes and ARP entries
	_ = bStore.SaveNode(ctx, &datastore.NodeEnt{
		ID:   "n1",
		Name: "Node1",
		IP:   "192.168.1.15",
	})
	_ = bStore.SaveNode(ctx, &datastore.NodeEnt{
		ID:   "n2",
		Name: "Node2",
		IP:   "10.0.5.20",
	})
	_ = bStore.SaveArpTable(ctx, []*datastore.ArpEnt{
		{IP: "192.168.1.50", MAC: "00:11:22:33:44:55", LastTime: time.Now().Unix()},
		{IP: "172.16.1.15", MAC: "aa:bb:cc:dd:ee:ff", LastTime: time.Now().Unix()},
	})

	srv, err := api.NewServer(api.Config{
		Port:    9099,
		Version: "v0.1.0-test",
		Store:   bStore,
	})
	if err != nil {
		t.Fatalf("create api server failed: %v", err)
	}
	e := srv.GetEcho()

	req := httptest.NewRequest(http.MethodGet, "/api/ipam", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp api.IPAMReportResp
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v, body: %s", err, rec.Body.String())
	}

	if resp.TotalRanges != 3 {
		t.Errorf("expected 3 ranges, got %d", resp.TotalRanges)
	}

	// Range 1: 192.168.1.0/24
	r1 := resp.Ranges[0]
	if r1.Size != 254 {
		t.Errorf("expected 254 size for /24, got %d", r1.Size)
	}
	if r1.Used != 2 { // 192.168.1.15 and 192.168.1.50
		t.Errorf("expected 2 used for range 1, got %d", r1.Used)
	}

	// Range 2: 10.0.0.0/16
	r2 := resp.Ranges[1]
	if r2.Size != 65534 {
		t.Errorf("expected 65534 size for /16, got %d", r2.Size)
	}
	if r2.Used != 1 { // 10.0.5.20
		t.Errorf("expected 1 used for range 2, got %d", r2.Used)
	}
	if len(r2.Subnets) == 0 {
		t.Errorf("expected /24 subnets for /16 range, got 0")
	}

	// Range 3: 172.16.1.10-172.16.1.20 (11 IPs)
	r3 := resp.Ranges[2]
	if r3.Size != 11 {
		t.Errorf("expected 11 size for range 3, got %d", r3.Size)
	}
	if r3.Used != 1 { // 172.16.1.15
		t.Errorf("expected 1 used for range 3, got %d", r3.Used)
	}
}

func TestAPIServer_MapAndLayoutEndpoints(t *testing.T) {
	bStore, pqStore, mcpSvr, cleanup := setupTestAPIEnv(t)
	defer cleanup()
	ctx := context.Background()

	pollMgr := polling.NewManager(polling.Config{
		Store:    bStore,
		LogStore: pqStore,
	})

	srv, err := api.NewServer(api.Config{
		Store:          bStore,
		LogStore:       pqStore,
		MCPServer:      mcpSvr,
		PollingManager: pollMgr,
	})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	e := srv.GetEcho()

	// 1. Test the all-node and per-node polling triggers.
	req := httptest.NewRequest(http.MethodPost, "/api/polling/check-all", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("POST /api/polling/check-all failed: code %d", rec.Code)
	}

	// 2. Seed a node and test node-scoped polling and map position updates.
	_ = bStore.SaveNode(ctx, &datastore.NodeEnt{
		ID:   "pos-node-1",
		Name: "Node Pos",
		X:    10,
		Y:    10,
	})
	_ = bStore.SaveNode(ctx, &datastore.NodeEnt{ID: "other-node", Name: "Other Node"})
	_ = bStore.SavePolling(ctx, &datastore.PollingEnt{
		ID: "node-poll-active", NodeID: "pos-node-1", Type: "unsupported-test", Level: "warn",
	})
	_ = bStore.SavePolling(ctx, &datastore.PollingEnt{
		ID: "node-poll-disabled", NodeID: "pos-node-1", Type: "unsupported-test", Level: "off",
	})
	_ = bStore.SavePolling(ctx, &datastore.PollingEnt{
		ID: "other-poll-active", NodeID: "other-node", Type: "unsupported-test", Level: "warn",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/polling/check/pos-node-1", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("POST /api/polling/check/pos-node-1 failed: code %d, body=%s", rec.Code, rec.Body.String())
	} else {
		var body struct {
			Count int `json:"count"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode per-node polling response: %v", err)
		}
		if body.Count != 1 {
			t.Errorf("per-node polling count = %d, want 1", body.Count)
		}
	}
	req = httptest.NewRequest(http.MethodPost, "/api/polling/check/unknown-node", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("POST /api/polling/check/unknown-node returned %d, want %d", rec.Code, http.StatusNotFound)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/tools/snmp", strings.NewReader(`{"node_id":"pos-node-1","oid":".1.3.6.1.2.1.1","mode":"invalid"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("POST /api/tools/snmp with invalid mode returned %d, want %d", rec.Code, http.StatusBadRequest)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/tools/gnmi/capabilities", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("POST /api/tools/gnmi/capabilities without node_id returned %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if err := bStore.SaveNetwork(ctx, &datastore.NetworkEnt{
		ID: "unmanaged-check", Name: "Unmanaged check", Unmanaged: true,
		Ports: []datastore.PortEnt{{ID: "p1", Name: "Port 1", State: "down"}},
	}); err != nil {
		t.Fatalf("save unmanaged network: %v", err)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/networks/unmanaged-check/check", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/networks/unmanaged-check/check returned %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var checkedNetwork datastore.NetworkEnt
	if err := json.Unmarshal(rec.Body.Bytes(), &checkedNetwork); err != nil {
		t.Fatalf("decode checked network: %v", err)
	}
	if checkedNetwork.Error == "" || checkedNetwork.Ports[0].State != "unknown" {
		t.Errorf("no-IP unmanaged network check = error %q, port state %q; want saved error and unknown port", checkedNetwork.Error, checkedNetwork.Ports[0].State)
	}
	posReqBody := `[{"ID":"pos-node-1","X":150,"Y":250}]`
	req = httptest.NewRequest(http.MethodPost, "/api/nodes/positions", strings.NewReader(posReqBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("POST /api/nodes/positions failed: code %d", rec.Code)
	}
	updatedNode, _ := bStore.GetNode(ctx, "pos-node-1")
	if updatedNode.X != 150 || updatedNode.Y != 250 {
		t.Errorf("expected node coords (150, 250), got (%d, %d)", updatedNode.X, updatedNode.Y)
	}

	// 3. Test AutoLayout endpoints
	layoutReqBody := `{"mode":1}`
	req = httptest.NewRequest(http.MethodPost, "/api/map/autolayout", strings.NewReader(layoutReqBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("POST /api/map/autolayout failed: code %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/map/autolayout/undo", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "true") {
		t.Errorf("GET /api/map/autolayout/undo failed: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/map/autolayout/undo", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("POST /api/map/autolayout/undo failed: code %d", rec.Code)
	}

	// 4. Test BackImage endpoints
	backReqBody := `{"X":10,"Y":20,"Width":800,"Height":600,"Path":"/test.png"}`
	req = httptest.NewRequest(http.MethodPost, "/api/map/backimage", strings.NewReader(backReqBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("POST /api/map/backimage failed: code %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/map/backimage", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "/test.png") {
		t.Errorf("GET /api/map/backimage failed: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/map/backimage", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("DELETE /api/map/backimage failed: code %d", rec.Code)
	}

	// 5. Test Map Import
	importReqBody := `{
		"nodes":[{"ID":"imp-node-1","Name":"Imp Node","X":50,"Y":50}],
		"lines":[{"ID":"imp-line-1","NodeID1":"imp-node-1","NodeID2":"pos-node-1"}],
		"networks":[{"ID":"imp-net-1","Name":"Imp Net","X":100,"Y":100}],
		"drawItems":[{"ID":"imp-item-1","Text":"Test Text","X":20,"Y":20}]
	}`
	req = httptest.NewRequest(http.MethodPost, "/api/map/import", strings.NewReader(importReqBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Errorf("POST /api/map/import failed: %s", rec.Body.String())
	}
}

func TestAPIServer_PollingDrawItems(t *testing.T) {
	bStore, pqStore, mcpSvr, cleanup := setupTestAPIEnv(t)
	defer cleanup()

	ctx := context.Background()
	// Add test polling with rtt in nanoseconds (e.g. 5,230,000 ns = 5.23 ms)
	p := &datastore.PollingEnt{
		ID:     "poll-kpi-1",
		NodeID: "node-1",
		Name:   "Ping Gateway",
		Type:   "ping",
		State:  "normal",
		Result: map[string]interface{}{
			"rtt": float64(5230000), // nanoseconds
			"ttl": float64(64),
		},
	}
	if err := bStore.SavePolling(ctx, p); err != nil {
		t.Fatalf("save polling: %v", err)
	}

	srv, err := api.NewServer(api.Config{
		Port:      19099,
		Store:     bStore,
		LogStore:  pqStore,
		MCPServer: mcpSvr,
	})
	if err != nil {
		t.Fatalf("create api server: %v", err)
	}
	e := srv.GetEcho()

	// 1. Create KPI Card (Type 11) for this polling
	kpiPayload := `{
		"Type": 11,
		"X": 100,
		"Y": 100,
		"PollingID": "poll-kpi-1",
		"VarName": "rtt",
		"Scale": 0.000001,
		"Format": "%.2f ms"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/drawitems", strings.NewReader(kpiPayload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST KPI drawitem failed: %d %s", rec.Code, rec.Body.String())
	}
	var createdKPI datastore.DrawItemEnt
	if err := json.Unmarshal(rec.Body.Bytes(), &createdKPI); err != nil {
		t.Fatalf("unmarshal created KPI: %v", err)
	}
	if createdKPI.Value < 5.0 || createdKPI.Value > 6.0 {
		t.Errorf("expected KPI Value ~5.23, got %f", createdKPI.Value)
	}
	if !strings.Contains(createdKPI.FormattedText, "5.23 ms") {
		t.Errorf("expected FormattedText '5.23 ms', got '%s'", createdKPI.FormattedText)
	}

	// 2. Create Polling Text (Type 4) with empty text
	ptPayload := `{
		"Type": 4,
		"X": 150,
		"Y": 150,
		"PollingID": "poll-kpi-1",
		"VarName": "rtt"
	}`
	req = httptest.NewRequest(http.MethodPost, "/api/drawitems", strings.NewReader(ptPayload))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST PollingText drawitem failed: %d %s", rec.Code, rec.Body.String())
	}
	var createdPT datastore.DrawItemEnt
	if err := json.Unmarshal(rec.Body.Bytes(), &createdPT); err != nil {
		t.Fatalf("unmarshal created PT: %v", err)
	}
	if createdPT.Text == "" {
		t.Errorf("expected non-empty Text for PollingText, got empty")
	}
	if !strings.Contains(createdPT.FormattedText, "5.23 ms") {
		t.Errorf("expected FormattedText '5.23 ms', got '%s'", createdPT.FormattedText)
	}

	// 3. Verify GET /api/drawitems returns both items properly formatted
	req = httptest.NewRequest(http.MethodGet, "/api/drawitems", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/drawitems failed: %d", rec.Code)
	}
	var list []*datastore.DrawItemEnt
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("unmarshal draw items list: %v", err)
	}
	if len(list) < 2 {
		t.Fatalf("expected at least 2 draw items, got %d", len(list))
	}
	for _, it := range list {
		if it.Type == datastore.DrawItemTypePollingKPI {
			if it.Value == 0.0 {
				t.Errorf("KPI card value should not be 0.0")
			}
			if it.FormattedText == "" || it.FormattedText == "0.0" {
				t.Errorf("KPI card formattedText should not be empty or 0.0, got '%s'", it.FormattedText)
			}
		}
		if it.Type == datastore.DrawItemTypePollingText {
			if it.Text == "" {
				t.Errorf("PollingText Text should not be empty")
			}
		}
	}
}

func TestResolveNameToOID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "system", want: ".1.3.6.1.2.1.1"},
		{input: "sysDescr", want: ".1.3.6.1.2.1.1.1"},
		{input: "sysDescr.0", want: ".1.3.6.1.2.1.1.1.0"},
		{input: "interfaces", want: ".1.3.6.1.2.1.2"},
		{input: ".1.3.6.1.2.1.1", want: ".1.3.6.1.2.1.1"},
		{input: "1.3.6.1.2.1.1", want: ".1.3.6.1.2.1.1"},
		{input: ".1", want: ".1.3"},
		{input: "unknown_mib_symbol_xyz", want: ""},
		{input: "", want: ""},
	}
	for _, tt := range tests {
		got := api.ResolveNameToOID(tt.input)
		if got != tt.want {
			t.Errorf("ResolveNameToOID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestAPIServer_MIBModuleEndpoints(t *testing.T) {
	bStore, pqStore, mcpSvr, cleanup := setupTestAPIEnv(t)
	defer cleanup()

	tempDataDir, err := os.MkdirTemp("", "twsnmpneo-mib-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDataDir)

	srv, err := api.NewServer(api.Config{
		Port:      9101,
		Debug:     true,
		Version:   "v0.1.0-test",
		Store:     bStore,
		LogStore:  pqStore,
		MCPServer: mcpSvr,
		DataDir:   tempDataDir,
	})
	if err != nil {
		t.Fatalf("create api server failed: %v", err)
	}
	e := srv.GetEcho()

	// 1. GET /api/mib/modules
	req := httptest.NewRequest(http.MethodGet, "/api/mib/modules", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/mib/modules failed: %d", rec.Code)
	}

	// 2. POST /api/mib/reload
	req = httptest.NewRequest(http.MethodPost, "/api/mib/reload", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/mib/reload failed: %d", rec.Code)
	}

	// 3. DELETE /api/mib/modules without valid file returns error
	delPayload := `{"file":"invalid.txt"}`
	req = httptest.NewRequest(http.MethodDelete, "/api/mib/modules", strings.NewReader(delPayload))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Errorf("expected error deleting non-existent file, got 200")
	}
}
