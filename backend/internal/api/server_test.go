package api_test

import (
	"context"
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

	// 3. Pollings CRUD: POST, GET, DELETE/:id
	pollPayload := `{"id":"p-api-1","node_id":"n-api-1","name":"Ping Check","type":"ping","state":"normal"}`
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
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Ping Check") {
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
