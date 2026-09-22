package receiver

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// OTelConfig holds configuration for the OpenTelemetry receiver.
type OTelConfig struct {
	Port     int
	LogStore *parquet.Store
}

// OTelServer receives OpenTelemetry OTLP Protobuf/JSON data over HTTP.
type OTelServer struct {
	port     int
	logStore *parquet.Store
	server   *http.Server
	mu       sync.Mutex
}

// NewOTelServer creates a new OpenTelemetry receiver instance.
func NewOTelServer(cfg OTelConfig) *OTelServer {
	return &OTelServer{
		port:     cfg.Port,
		logStore: cfg.LogStore,
	}
}

// Start launches the OpenTelemetry HTTP server and blocks until ctx is canceled.
func (s *OTelServer) Start(ctx context.Context) error {
	if s.port <= 0 {
		return nil
	}

	addr := fmt.Sprintf(":%d", s.port)
	slog.Info("Starting OpenTelemetry receiver", "addr", addr)

	mux := http.NewServeMux()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var reader io.Reader = r.Body
		if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "gzip decode error", http.StatusBadRequest)
				return
			}
			defer gz.Close()
			reader = gz
		}

		body, err := io.ReadAll(io.LimitReader(reader, 10*1024*1024))
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		srcIP, _, _ := net.SplitHostPort(r.RemoteAddr)
		if srcIP == "" {
			srcIP = r.RemoteAddr
		}

		path := r.URL.Path
		contentType := r.Header.Get("Content-Type")

		serviceName, summary, logJSON := decodeOTelPayload(path, contentType, body)

		src := srcIP
		if serviceName != "" && serviceName != "unknown" {
			src = fmt.Sprintf("%s (%s)", serviceName, srcIP)
		}

		finalLog := fmt.Sprintf("[%s] %s", path, logJSON)
		if summary != "" {
			if logJSON != "" {
				finalLog = fmt.Sprintf("[%s] %s | %s", path, summary, logJSON)
			} else {
				finalLog = fmt.Sprintf("[%s] %s", path, summary)
			}
		}

		if s.logStore != nil {
			_ = s.logStore.WriteLog(&parquet.ParquetLogRecord{
				Time: time.Now().UnixNano(),
				Type: "otel",
				Src:  src,
				Log:  finalLog,
			})
		}

		isJSON := strings.Contains(contentType, "application/json") || strings.Contains(r.Header.Get("Accept"), "application/json")
		if !isJSON && (strings.Contains(contentType, "protobuf") || strings.Contains(r.Header.Get("Accept"), "protobuf")) {
			w.Header().Set("Content-Type", "application/x-protobuf")
			w.WriteHeader(http.StatusOK)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}
	})

	mux.HandleFunc("/v1/logs", handler)
	mux.HandleFunc("/v1/metrics", handler)
	mux.HandleFunc("/v1/traces", handler)

	s.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Warn("Failed to start OpenTelemetry receiver", "addr", addr, "error", err)
		return fmt.Errorf("listen otel: %w", err)
	}
	defer ln.Close()

	slog.Info("Started OpenTelemetry receiver", "addr", addr)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.server.Shutdown(shutdownCtx)
	}()

	if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func decodeOTelPayload(path string, contentType string, body []byte) (string, string, string) {
	isJSON := strings.Contains(contentType, "application/json") || (len(body) > 0 && body[0] == '{')

	if isJSON {
		serviceName, summary := parseOTelJSON(body)
		return serviceName, summary, string(body)
	}

	// Try Protobuf decoding
	m := protojson.MarshalOptions{
		UseProtoNames:   true,
		EmitUnpopulated: false,
	}

	switch path {
	case "/v1/metrics":
		var req colmetricspb.ExportMetricsServiceRequest
		if err := proto.Unmarshal(body, &req); err == nil {
			serviceName, summary := summarizeMetrics(&req)
			if jb, err := m.Marshal(&req); err == nil {
				return serviceName, summary, string(jb)
			}
			return serviceName, summary, ""
		}
	case "/v1/logs":
		var req collogspb.ExportLogsServiceRequest
		if err := proto.Unmarshal(body, &req); err == nil {
			serviceName, summary := summarizeLogs(&req)
			if jb, err := m.Marshal(&req); err == nil {
				return serviceName, summary, string(jb)
			}
			return serviceName, summary, ""
		}
	case "/v1/traces":
		var req coltracepb.ExportTraceServiceRequest
		if err := proto.Unmarshal(body, &req); err == nil {
			serviceName, summary := summarizeTraces(&req)
			if jb, err := m.Marshal(&req); err == nil {
				return serviceName, summary, string(jb)
			}
			return serviceName, summary, ""
		}
	}

	// Fallback: If protobuf decoding fails, sanitize non-printable bytes
	var sb strings.Builder
	for _, b := range body {
		if (b >= 32 && b <= 126) || b == '\n' || b == '\r' || b == '\t' {
			sb.WriteByte(b)
		} else {
			sb.WriteByte(' ')
		}
	}
	cleanText := strings.TrimSpace(sb.String())
	return "", "", cleanText
}

func summarizeMetrics(req *colmetricspb.ExportMetricsServiceRequest) (string, string) {
	service := "unknown"
	metricNames := make([]string, 0)
	for _, rm := range req.GetResourceMetrics() {
		for _, attr := range rm.GetResource().GetAttributes() {
			if attr.GetKey() == "service.name" {
				service = attr.GetValue().GetStringValue()
			}
		}
		for _, sm := range rm.GetScopeMetrics() {
			for _, m := range sm.GetMetrics() {
				if m.GetName() != "" {
					metricNames = append(metricNames, m.GetName())
				}
			}
		}
	}
	if len(metricNames) > 5 {
		metricNames = append(metricNames[:5], fmt.Sprintf("+%d more", len(metricNames)-5))
	}
	summary := fmt.Sprintf("service=%s metrics=[%s]", service, strings.Join(metricNames, ", "))
	return service, summary
}

func summarizeLogs(req *collogspb.ExportLogsServiceRequest) (string, string) {
	service := "unknown"
	logCount := 0
	sampleLog := ""
	for _, rl := range req.GetResourceLogs() {
		for _, attr := range rl.GetResource().GetAttributes() {
			if attr.GetKey() == "service.name" {
				service = attr.GetValue().GetStringValue()
			}
		}
		for _, sl := range rl.GetScopeLogs() {
			for _, lr := range sl.GetLogRecords() {
				logCount++
				if sampleLog == "" {
					bodyText := lr.GetBody().GetStringValue()
					if bodyText == "" {
						bodyText = lr.GetBody().String()
					}
					sampleLog = fmt.Sprintf("severity=%s msg=%q", lr.GetSeverityText(), bodyText)
				}
			}
		}
	}
	summary := fmt.Sprintf("service=%s count=%d %s", service, logCount, sampleLog)
	return service, summary
}

func summarizeTraces(req *coltracepb.ExportTraceServiceRequest) (string, string) {
	service := "unknown"
	spanNames := make([]string, 0)
	for _, rs := range req.GetResourceSpans() {
		for _, attr := range rs.GetResource().GetAttributes() {
			if attr.GetKey() == "service.name" {
				service = attr.GetValue().GetStringValue()
			}
		}
		for _, ss := range rs.GetScopeSpans() {
			for _, sp := range ss.GetSpans() {
				if sp.GetName() != "" {
					spanNames = append(spanNames, sp.GetName())
				}
			}
		}
	}
	if len(spanNames) > 5 {
		spanNames = append(spanNames[:5], fmt.Sprintf("+%d more", len(spanNames)-5))
	}
	summary := fmt.Sprintf("service=%s spans=[%s]", service, strings.Join(spanNames, ", "))
	return service, summary
}

func parseOTelJSON(body []byte) (string, string) {
	var doc struct {
		ResourceMetrics []struct {
			Resource struct {
				Attributes []struct {
					Key   string `json:"key"`
					Value struct {
						StringValue string `json:"stringValue"`
					} `json:"value"`
				} `json:"attributes"`
			} `json:"resource"`
			ScopeMetrics []struct {
				Metrics []struct {
					Name string `json:"name"`
				} `json:"metrics"`
			} `json:"scopeMetrics"`
		} `json:"resourceMetrics"`
		ResourceLogs []struct {
			Resource struct {
				Attributes []struct {
					Key   string `json:"key"`
					Value struct {
						StringValue string `json:"stringValue"`
					} `json:"value"`
				} `json:"attributes"`
			} `json:"resource"`
		} `json:"resourceLogs"`
		ResourceSpans []struct {
			Resource struct {
				Attributes []struct {
					Key   string `json:"key"`
					Value struct {
						StringValue string `json:"stringValue"`
					} `json:"value"`
				} `json:"attributes"`
			} `json:"resource"`
			ScopeSpans []struct {
				Spans []struct {
					Name string `json:"name"`
				} `json:"spans"`
			} `json:"scopeSpans"`
		} `json:"resourceSpans"`
	}

	if err := json.Unmarshal(body, &doc); err != nil {
		return "", ""
	}

	service := ""
	for _, rm := range doc.ResourceMetrics {
		for _, a := range rm.Resource.Attributes {
			if a.Key == "service.name" {
				service = a.Value.StringValue
			}
		}
	}
	if service == "" {
		for _, rl := range doc.ResourceLogs {
			for _, a := range rl.Resource.Attributes {
				if a.Key == "service.name" {
					service = a.Value.StringValue
				}
			}
		}
	}
	if service == "" {
		for _, rs := range doc.ResourceSpans {
			for _, a := range rs.Resource.Attributes {
				if a.Key == "service.name" {
					service = a.Value.StringValue
				}
			}
		}
	}

	summary := ""
	if service != "" {
		summary = fmt.Sprintf("service=%s", service)
	}
	return service, summary
}
