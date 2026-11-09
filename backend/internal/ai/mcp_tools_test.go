package ai

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/bbolt"
)

func TestMCP_OTelTools(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	bStore, err := bbolt.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create bbolt store: %v", err)
	}
	defer bStore.Close()

	ctx := context.Background()

	// 1. Add mock OTel metric
	m := &datastore.OTelMetricEnt{
		Host:        "app-01",
		Service:     "order-api",
		Scope:       "http",
		Name:        "http_requests_total",
		Type:        "Counter",
		Description: "Total HTTP requests",
		Count:       42,
		First:       time.Now().UnixNano(),
		Last:        time.Now().UnixNano(),
		DataPoints: []*datastore.OTelMetricDataPointEnt{
			{
				Time:  time.Now().UnixNano(),
				Gauge: 12.3,
				Sum:   456.7,
			},
		},
	}
	if err := bStore.SaveOTelMetric(ctx, m); err != nil {
		t.Fatalf("save metric failed: %v", err)
	}

	// 2. Add mock OTel trace
	now := time.Now()
	bucket := now.Format("2006-01-02T15:04")
	tr := &datastore.OTelTraceEnt{
		Bucket:  bucket,
		TraceID: "mcp-trace-01",
		Start:   now.Add(-5 * time.Second).UnixNano(),
		End:     now.UnixNano(),
		Dur:     0.25,
		Spans: []datastore.OTelTraceSpanEnt{
			{
				SpanID:     "sp1",
				Service:    "order-api",
				Name:       "/orders",
				Start:      now.Add(-5 * time.Second).UnixNano(),
				End:        now.UnixNano(),
				Dur:        0.25,
				Attributes: []string{"http.status_code=500"},
			},
		},
	}
	if err := bStore.SaveOTelTraces(ctx, []*datastore.OTelTraceEnt{tr}); err != nil {
		t.Fatalf("save trace failed: %v", err)
	}

	mcpSrv := NewMCPServer(MCPConfig{
		Store:   bStore,
		Version: "test-v1",
	})

	// Test getOTelMetrics
	res, _, err := mcpSrv.getOTelMetrics(ctx, nil, mcpGetOTelMetricsParams{
		ServiceFilter: "order-api",
	})
	if err != nil {
		t.Fatalf("getOTelMetrics failed: %v", err)
	}
	if len(res.Content) == 0 {
		t.Fatalf("expected content from getOTelMetrics")
	}

	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %#v", res.Content[0])
	}

	var metricsList []mcpOTelMetricEnt
	if err := json.Unmarshal([]byte(tc.Text), &metricsList); err != nil {
		t.Fatalf("unmarshal getOTelMetrics failed: %v", err)
	}
	if len(metricsList) != 1 || metricsList[0].Name != "http_requests_total" {
		t.Errorf("unexpected metrics list: %+v", metricsList)
	}

	// Test getOTelTraces
	res, _, err = mcpSrv.getOTelTraces(ctx, nil, mcpGetOTelTracesParams{
		ErrorOnly: true,
	})
	if err != nil {
		t.Fatalf("getOTelTraces failed: %v", err)
	}
	if len(res.Content) == 0 {
		t.Fatalf("expected content from getOTelTraces")
	}

	tc, ok = res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %#v", res.Content[0])
	}

	var traceList []mcpOTelTraceEnt
	if err := json.Unmarshal([]byte(tc.Text), &traceList); err != nil {
		t.Fatalf("unmarshal getOTelTraces failed: %v", err)
	}
	if len(traceList) != 1 || traceList[0].TraceID != "mcp-trace-01" {
		t.Errorf("unexpected trace list: %+v", traceList)
	}
}

func TestMCP_ReportSummaryAndSensors(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	bStore, err := bbolt.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create bbolt store: %v", err)
	}
	defer bStore.Close()

	ctx := context.Background()

	// Save a mock report item
	reportData := map[string]any{
		"host":  "sensor-01",
		"ssid":  "Office-Guest",
		"bssid": "00:11:22:33:44:55",
	}
	reportBytes, _ := json.Marshal(reportData)
	if err := bStore.SaveLogReportData(ctx, "wifiAP", map[string][]byte{"00:11:22:33:44:55": reportBytes}); err != nil {
		t.Fatalf("save report data failed: %v", err)
	}

	// Save a mock sensor
	sn := &datastore.SensorEnt{
		ID:        "sn-01",
		Host:      "sensor-01",
		Type:      "twWifiScan",
		State:     "normal",
		Total:     100,
		Send:      95,
		FirstTime: time.Now().UnixNano(),
		LastTime:  time.Now().UnixNano(),
	}
	if err := bStore.SaveSensor(ctx, sn); err != nil {
		t.Fatalf("save sensor failed: %v", err)
	}

	mcpSrv := NewMCPServer(MCPConfig{
		Store:   bStore,
		Version: "test-v1",
	})

	// Test getReportSummary
	res, _, err := mcpSrv.getReportSummary(ctx, nil, mcpGetReportSummaryParams{
		ReportType: "wifiAP",
	})
	if err != nil {
		t.Fatalf("getReportSummary failed: %v", err)
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent")
	}
	var reportList []map[string]any
	if err := json.Unmarshal([]byte(tc.Text), &reportList); err != nil {
		t.Fatalf("unmarshal report summary failed: %v", err)
	}
	if len(reportList) != 1 || reportList[0]["ssid"] != "Office-Guest" {
		t.Errorf("unexpected report list: %+v", reportList)
	}

	// Test getSensorList with filter
	res, _, err = mcpSrv.getSensorList(ctx, nil, mcpGetSensorListParams{
		TypeFilter: "twWifi.*",
	})
	if err != nil {
		t.Fatalf("getSensorList failed: %v", err)
	}
	tc, ok = res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent")
	}
	var sensorList []mcpSensorEnt
	if err := json.Unmarshal([]byte(tc.Text), &sensorList); err != nil {
		t.Fatalf("unmarshal sensor list failed: %v", err)
	}
	if len(sensorList) != 1 || sensorList[0].Host != "sensor-01" {
		t.Errorf("unexpected sensor list: %+v", sensorList)
	}
}
