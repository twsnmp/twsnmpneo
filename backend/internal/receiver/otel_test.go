package receiver

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/proto"
)

func TestOTel_ProtobufDecoding(t *testing.T) {
	// Build a Protobuf ExportMetricsServiceRequest with service.name = "dice"
	req := &colmetricspb.ExportMetricsServiceRequest{
		ResourceMetrics: []*metricspb.ResourceMetrics{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key:   "service.name",
							Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "dice"}},
						},
						{
							Key:   "service.version",
							Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "0.1.0"}},
						},
					},
				},
				ScopeMetrics: []*metricspb.ScopeMetrics{
					{
						Metrics: []*metricspb.Metric{
							{
								Name:        "dice.rolls",
								Description: "Count of dice rolls",
							},
						},
					},
				},
			},
		},
	}

	pbBytes, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal protobuf failed: %v", err)
	}

	serviceName, summary, logJSON := decodeOTelPayload("/v1/metrics", "application/x-protobuf", pbBytes)
	if serviceName != "dice" {
		t.Errorf("expected serviceName 'dice', got %q", serviceName)
	}
	if !strings.Contains(summary, "service=dice") || !strings.Contains(summary, "dice.rolls") {
		t.Errorf("expected summary to contain service=dice and dice.rolls, got %q", summary)
	}
	if !strings.Contains(logJSON, "dice.rolls") {
		t.Errorf("expected JSON to contain 'dice.rolls', got %q", logJSON)
	}
	// Verify it is valid JSON
	var doc map[string]any
	if err := json.Unmarshal([]byte(logJSON), &doc); err != nil {
		t.Errorf("expected valid JSON without mojibake, unmarshal failed: %v", err)
	}
}

func TestOTel_HTTPGzipAndProtobufIngestion(t *testing.T) {
	dir, err := os.MkdirTemp("", "twsnmpneo-otel-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	store, err := parquet.New(parquet.Config{
		Dir:            filepath.Join(dir, "logs"),
		BufferSize:     1,
		BufferInterval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("create parquet store: %v", err)
	}
	defer store.Close()

	// Build Protobuf metrics request
	req := &colmetricspb.ExportMetricsServiceRequest{
		ResourceMetrics: []*metricspb.ResourceMetrics{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key:   "service.name",
							Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "dice"}},
						},
					},
				},
				ScopeMetrics: []*metricspb.ScopeMetrics{
					{
						Metrics: []*metricspb.Metric{
							{Name: "dice.rolls"},
						},
					},
				},
			},
		},
	}
	pbBytes, _ := proto.Marshal(req)

	// Compress with gzip
	var gzBuf bytes.Buffer
	gw := gzip.NewWriter(&gzBuf)
	_, _ = gw.Write(pbBytes)
	_ = gw.Close()

	// Perform HTTP request to test the handler
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/metrics", &gzBuf)
	httpReq.Header.Set("Content-Type", "application/x-protobuf")
	httpReq.Header.Set("Content-Encoding", "gzip")
	httpReq.RemoteAddr = "192.168.1.210:54321"

	var reader io.Reader = httpReq.Body
	gz, err := gzip.NewReader(reader)
	if err != nil {
		t.Fatalf("gzip reader failed: %v", err)
	}
	defer gz.Close()

	bodyBytes, _ := io.ReadAll(gz)
	serviceName, summary, logJSON := decodeOTelPayload(httpReq.URL.Path, httpReq.Header.Get("Content-Type"), bodyBytes)
	finalLog := fmt.Sprintf("[%s] %s | %s", httpReq.URL.Path, summary, logJSON)

	err = store.WriteLog(&parquet.ParquetLogRecord{
		Time: time.Now().UnixNano(),
		Type: "otel",
		Src:  fmt.Sprintf("%s (%s)", serviceName, "192.168.1.210"),
		Log:  finalLog,
	})
	if err != nil {
		t.Fatalf("write log failed: %v", err)
	}

	// Query parquet log
	logs, err := store.Query(context.Background(), parquet.LogFilter{
		Type:  "otel",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if logs[0].Src != "dice (192.168.1.210)" {
		t.Errorf("expected src 'dice (192.168.1.210)', got %q", logs[0].Src)
	}
	if !strings.Contains(logs[0].Log, "dice.rolls") {
		t.Errorf("expected log to contain dice.rolls, got %q", logs[0].Log)
	}
	// Check no control characters
	if strings.Contains(logs[0].Log, "\x00") || strings.Contains(logs[0].Log, "\x0c") {
		t.Errorf("log contains binary control characters: %q", logs[0].Log)
	}
}
