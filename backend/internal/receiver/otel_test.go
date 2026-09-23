package receiver

import (
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/bbolt"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

func TestOTel_MetricsAndTracesIngestion(t *testing.T) {
	dir, err := os.MkdirTemp("", "twsnmpneo-otel-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	store, err := bbolt.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("create bbolt store: %v", err)
	}
	defer store.Close()

	pqStore, err := parquet.New(parquet.Config{
		Dir:            filepath.Join(dir, "logs"),
		BufferSize:     1,
		BufferInterval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("create parquet store: %v", err)
	}
	defer pqStore.Close()

	srv := NewOTelServer(OTelConfig{
		Port:      0, // test in-memory handlers
		Store:     store,
		LogStore:  pqStore,
		Retention: 24,
	})

	ctx := context.Background()

	// 1. Test Metrics Ingestion
	metricReq := &colmetricspb.ExportMetricsServiceRequest{
		ResourceMetrics: []*metricspb.ResourceMetrics{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key:   "service.name",
							Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "dice-service"}},
						},
						{
							Key:   "host.name",
							Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "app-node-1"}},
						},
					},
				},
				ScopeMetrics: []*metricspb.ScopeMetrics{
					{
						Scope: &commonpb.InstrumentationScope{Name: "dice.scope"},
						Metrics: []*metricspb.Metric{
							{
								Name: "dice.rolls",
								Data: &metricspb.Metric_Sum{
									Sum: &metricspb.Sum{
										DataPoints: []*metricspb.NumberDataPoint{
											{
												TimeUnixNano: uint64(time.Now().UnixNano()),
												Value:        &metricspb.NumberDataPoint_AsInt{AsInt: 42},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	mBytes, _ := proto.Marshal(metricReq)

	// Gzip compress metrics
	var gzBuf bytes.Buffer
	gw := gzip.NewWriter(&gzBuf)
	_, _ = gw.Write(mBytes)
	_ = gw.Close()

	httpReq := httptest.NewRequest(http.MethodPost, "/v1/metrics", &gzBuf)
	httpReq.Header.Set("Content-Type", "application/x-protobuf")
	httpReq.Header.Set("Content-Encoding", "gzip")
	httpReq.RemoteAddr = "10.0.0.1:12345"
	rec := httptest.NewRecorder()

	srv.handleRequest(rec, httpReq, "metrics")
	if rec.Code != http.StatusOK {
		t.Fatalf("metric request failed with code %d", rec.Code)
	}

	metrics, err := store.ListOTelMetrics(ctx)
	if err != nil || len(metrics) == 0 {
		t.Fatalf("expected metrics in store, got %v (len %d)", err, len(metrics))
	}
	if metrics[0].Name != "dice.rolls" || metrics[0].Service != "dice-service" {
		t.Errorf("unexpected metric: %+v", metrics[0])
	}
	if len(metrics[0].DataPoints) != 1 || metrics[0].DataPoints[0].Sum != 42 {
		t.Errorf("unexpected data points: %+v", metrics[0].DataPoints)
	}

	// 2. Test Traces Ingestion
	traceID := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	spanID := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	nowNano := uint64(time.Now().UnixNano())

	traceReq := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key:   "service.name",
							Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "web-frontend"}},
						},
						{
							Key:   "host.name",
							Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "web-1"}},
						},
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Scope: &commonpb.InstrumentationScope{Name: "http.client"},
						Spans: []*tracepb.Span{
							{
								TraceId:           traceID,
								SpanId:            spanID,
								Name:              "GET /roll",
								StartTimeUnixNano: nowNano,
								EndTimeUnixNano:   nowNano + 50*1000*1000, // 50ms
							},
						},
					},
				},
			},
		},
	}
	tBytes, _ := proto.Marshal(traceReq)

	traceHttpReq := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewReader(tBytes))
	traceHttpReq.Header.Set("Content-Type", "application/x-protobuf")
	traceHttpReq.RemoteAddr = "10.0.0.1:12345"
	traceRec := httptest.NewRecorder()

	srv.handleRequest(traceRec, traceHttpReq, "traces")
	if traceRec.Code != http.StatusOK {
		t.Fatalf("trace request failed with code %d", traceRec.Code)
	}

	// Flush traces to store
	srv.flushTraces(ctx)

	buckets, err := store.GetOTelTraceBuckets(ctx)
	if err != nil || len(buckets) == 0 {
		t.Fatalf("expected trace buckets, got err %v (len %d)", err, len(buckets))
	}

	traces, err := store.ListOTelTraces(ctx, buckets)
	if err != nil || len(traces) == 0 {
		t.Fatalf("expected traces in store, got %v (len %d)", err, len(traces))
	}
	if traces[0].Services != "web-frontend" {
		t.Errorf("expected service 'web-frontend', got %q", traces[0].Services)
	}

	// 3. Test Logs Ingestion
	logReq := &collogspb.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key:   "service.name",
							Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "dice-service"}},
						},
					},
				},
				ScopeLogs: []*logspb.ScopeLogs{
					{
						Scope: &commonpb.InstrumentationScope{Name: "dice.logger"},
						LogRecords: []*logspb.LogRecord{
							{
								TimeUnixNano:   nowNano,
								SeverityNumber: logspb.SeverityNumber_SEVERITY_NUMBER_INFO,
								SeverityText:   "INFO",
								Body: &commonpb.AnyValue{
									Value: &commonpb.AnyValue_StringValue{StringValue: "User rolled 6"},
								},
							},
						},
					},
				},
			},
		},
	}
	lBytes, _ := proto.Marshal(logReq)

	logHttpReq := httptest.NewRequest(http.MethodPost, "/v1/logs", bytes.NewReader(lBytes))
	logHttpReq.Header.Set("Content-Type", "application/x-protobuf")
	logHttpReq.RemoteAddr = "10.0.0.1:12345"
	logRec := httptest.NewRecorder()

	srv.handleRequest(logRec, logHttpReq, "logs")
	if logRec.Code != http.StatusOK {
		t.Fatalf("log request failed with code %d", logRec.Code)
	}

	_ = pqStore.Flush()

	pqLogs, err := pqStore.Query(ctx, parquet.LogFilter{
		Type:  "otel",
		Limit: 10,
	})
	if err != nil || len(pqLogs) == 0 {
		t.Fatalf("expected parquet otel logs, got err %v (len %d)", err, len(pqLogs))
	}
}
