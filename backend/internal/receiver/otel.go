package receiver

import (
	"compress/gzip"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// OTelConfig holds configuration for the OpenTelemetry receiver.
type OTelConfig struct {
	Port      int
	Store     datastore.DataStore
	LogStore  *parquet.Store
	Retention int
	From      string
}

// OTelServer receives OpenTelemetry OTLP Protobuf/JSON data over HTTP.
type OTelServer struct {
	port      int
	store     datastore.DataStore
	logStore  *parquet.Store
	retention int
	from      string
	server    *http.Server
	mu        sync.Mutex

	activeTraces sync.Map // traceID (string) -> *datastore.OTelTraceEnt
	lastSave     int64
	allowedIPs   map[string]bool
}

// NewOTelServer creates a new OpenTelemetry receiver instance.
func NewOTelServer(cfg OTelConfig) *OTelServer {
	ret := cfg.Retention
	if ret <= 0 {
		ret = 24
	}

	allowed := make(map[string]bool)
	if cfg.From != "" {
		for _, f := range strings.Split(cfg.From, ",") {
			f = strings.TrimSpace(f)
			if f != "" {
				allowed[f] = true
			}
		}
	}

	return &OTelServer{
		port:       cfg.Port,
		store:      cfg.Store,
		logStore:   cfg.LogStore,
		retention:  ret,
		from:       cfg.From,
		allowedIPs: allowed,
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
	mux.HandleFunc("/v1/metrics", func(w http.ResponseWriter, r *http.Request) {
		s.handleRequest(w, r, "metrics")
	})
	mux.HandleFunc("/v1/traces", func(w http.ResponseWriter, r *http.Request) {
		s.handleRequest(w, r, "traces")
	})
	mux.HandleFunc("/v1/logs", func(w http.ResponseWriter, r *http.Request) {
		s.handleRequest(w, r, "logs")
	})

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

	// Background ticker for trace persistence and retention cleanup
	go s.traceFlushLoop(ctx)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.server.Shutdown(shutdownCtx)
		s.flushTraces(context.Background())
	}()

	if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *OTelServer) handleRequest(w http.ResponseWriter, r *http.Request, kind string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	srcIP, _, _ := net.SplitHostPort(r.RemoteAddr)
	if srcIP == "" {
		srcIP = r.RemoteAddr
	}

	if len(s.allowedIPs) > 0 && !s.allowedIPs[srcIP] {
		http.Error(w, "forbidden", http.StatusForbidden)
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

	body, err := io.ReadAll(io.LimitReader(reader, 20*1024*1024))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	contentType := r.Header.Get("Content-Type")
	isJSON := strings.Contains(contentType, "application/json") || (len(body) > 0 && body[0] == '{')

	switch kind {
	case "metrics":
		s.processMetrics(r.Context(), srcIP, body, isJSON)
	case "traces":
		s.processTraces(r.Context(), srcIP, body, isJSON)
	case "logs":
		s.processLogs(r.Context(), srcIP, body, isJSON)
	}

	if !isJSON && (strings.Contains(contentType, "protobuf") || strings.Contains(r.Header.Get("Accept"), "protobuf")) {
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}
}

func (s *OTelServer) traceFlushLoop(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	cleanTicker := time.NewTicker(10 * time.Minute)
	defer cleanTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.flushTraces(ctx)
		case <-cleanTicker.C:
			if s.store != nil {
				_ = s.store.CleanOldOTelData(ctx, s.retention)
			}
		}
	}
}

func (s *OTelServer) flushTraces(ctx context.Context) {
	if s.store == nil {
		return
	}

	var saveList []*datastore.OTelTraceEnt
	var delIDs []string
	cutoff := time.Now().Add(-time.Hour * time.Duration(s.retention+1)).UnixNano()

	s.activeTraces.Range(func(key, val any) bool {
		tid := key.(string)
		t, ok := val.(*datastore.OTelTraceEnt)
		if !ok {
			return true
		}
		if t.Last < cutoff {
			delIDs = append(delIDs, tid)
		} else if t.Last > t.SavedLast {
			t.SavedLast = t.Last
			saveList = append(saveList, t)
		}
		return true
	})

	if len(saveList) > 0 {
		_ = s.store.SaveOTelTraces(ctx, saveList)
	}
	for _, tid := range delIDs {
		s.activeTraces.Delete(tid)
	}
}

func (s *OTelServer) processMetrics(ctx context.Context, srcIP string, body []byte, isJSON bool) {
	var req colmetricspb.ExportMetricsServiceRequest
	var err error
	if isJSON {
		opts := protojson.UnmarshalOptions{DiscardUnknown: true}
		err = opts.Unmarshal(body, &req)
	} else {
		err = proto.Unmarshal(body, &req)
	}
	if err != nil {
		slog.Debug("OTel metric unmarshal error", "error", err)
		return
	}

	now := time.Now().UnixNano()
	for _, rm := range req.GetResourceMetrics() {
		service := "unknown"
		host := srcIP
		for _, attr := range rm.GetResource().GetAttributes() {
			if attr.GetKey() == "service.name" {
				service = formatAnyValue(attr.GetValue())
			}
			if attr.GetKey() == "host.name" {
				host = formatAnyValue(attr.GetValue())
			}
		}

		for _, sm := range rm.GetScopeMetrics() {
			scope := sm.GetScope().GetName()
			for _, m := range sm.GetMetrics() {
				mName := m.GetName()
				if mName == "" {
					continue
				}
				mType := determineMetricType(m)

				var metric *datastore.OTelMetricEnt
				if s.store != nil {
					metric, _ = s.store.GetOTelMetric(ctx, host, service, scope, mName)
				}
				if metric != nil {
					metric.Count++
					metric.Last = now
				} else {
					metric = &datastore.OTelMetricEnt{
						Host:        host,
						Service:     service,
						Scope:       scope,
						Name:        mName,
						Type:        mType,
						Description: m.GetDescription(),
						Unit:        m.GetUnit(),
						Count:       1,
						First:       now,
						Last:        now,
						DataPoints:  make([]*datastore.OTelMetricDataPointEnt, 0),
					}
				}

				addMetricDataPoints(metric, m)

				if s.store != nil {
					_ = s.store.SaveOTelMetric(ctx, metric)
				}
			}
		}
	}
}

func determineMetricType(m *metricspb.Metric) string {
	switch m.GetData().(type) {
	case *metricspb.Metric_Gauge:
		return "Gauge"
	case *metricspb.Metric_Sum:
		return "Sum"
	case *metricspb.Metric_Histogram:
		return "Histogram"
	case *metricspb.Metric_ExponentialHistogram:
		return "ExponentialHistogram"
	case *metricspb.Metric_Summary:
		return "Summary"
	default:
		return "Unknown"
	}
}

func addMetricDataPoints(metric *datastore.OTelMetricEnt, m *metricspb.Metric) {
	switch data := m.GetData().(type) {
	case *metricspb.Metric_Histogram:
		for l, h := range data.Histogram.GetDataPoints() {
			dp := &datastore.OTelMetricDataPointEnt{
				Start:          int64(h.GetStartTimeUnixNano()),
				Time:           int64(h.GetTimeUnixNano()),
				Attributes:     formatAttributes(h.GetAttributes()),
				Count:          h.GetCount(),
				BucketCounts:   h.GetBucketCounts(),
				ExplicitBounds: h.GetExplicitBounds(),
				Sum:            h.GetSum(),
				Min:            h.GetMin(),
				Max:            h.GetMax(),
				Index:          l,
			}
			metric.DataPoints = append(metric.DataPoints, dp)
		}
	case *metricspb.Metric_Sum:
		for l, s := range data.Sum.GetDataPoints() {
			dp := &datastore.OTelMetricDataPointEnt{
				Start:      int64(s.GetStartTimeUnixNano()),
				Time:       int64(s.GetTimeUnixNano()),
				Attributes: formatAttributes(s.GetAttributes()),
				Sum:        extractNumberValue(s),
				Index:      l,
			}
			metric.DataPoints = append(metric.DataPoints, dp)
		}
	case *metricspb.Metric_Gauge:
		for l, g := range data.Gauge.GetDataPoints() {
			dp := &datastore.OTelMetricDataPointEnt{
				Start:      int64(g.GetStartTimeUnixNano()),
				Time:       int64(g.GetTimeUnixNano()),
				Attributes: formatAttributes(g.GetAttributes()),
				Gauge:      extractNumberValue(g),
				Index:      l,
			}
			metric.DataPoints = append(metric.DataPoints, dp)
		}
	case *metricspb.Metric_ExponentialHistogram:
		for l, eh := range data.ExponentialHistogram.GetDataPoints() {
			pCounts := make([]uint64, 0)
			if eh.GetPositive() != nil {
				pCounts = eh.GetPositive().GetBucketCounts()
			}
			nCounts := make([]uint64, 0)
			if eh.GetNegative() != nil {
				nCounts = eh.GetNegative().GetBucketCounts()
			}
			dp := &datastore.OTelMetricDataPointEnt{
				Start:         int64(eh.GetStartTimeUnixNano()),
				Time:          int64(eh.GetTimeUnixNano()),
				Attributes:    formatAttributes(eh.GetAttributes()),
				Count:         eh.GetCount(),
				Positive:      pCounts,
				Negative:      nCounts,
				Scale:         int64(eh.GetScale()),
				ZeroCount:     int64(eh.GetZeroCount()),
				ZeroThreshold: eh.GetZeroThreshold(),
				Sum:           eh.GetSum(),
				Min:           eh.GetMin(),
				Max:           eh.GetMax(),
				Index:         l,
			}
			metric.DataPoints = append(metric.DataPoints, dp)
		}
	}

	if len(metric.DataPoints) > 1000 {
		metric.DataPoints = metric.DataPoints[len(metric.DataPoints)-1000:]
	}
}

func extractNumberValue(dp *metricspb.NumberDataPoint) float64 {
	switch v := dp.GetValue().(type) {
	case *metricspb.NumberDataPoint_AsDouble:
		return v.AsDouble
	case *metricspb.NumberDataPoint_AsInt:
		return float64(v.AsInt)
	}
	return 0
}

func (s *OTelServer) processTraces(_ context.Context, srcIP string, body []byte, isJSON bool) {
	var req coltracepb.ExportTraceServiceRequest
	var err error
	if isJSON {
		opts := protojson.UnmarshalOptions{DiscardUnknown: true}
		err = opts.Unmarshal(body, &req)
	} else {
		err = proto.Unmarshal(body, &req)
	}
	if err != nil {
		slog.Debug("OTel trace unmarshal error", "error", err)
		return
	}

	for _, rs := range req.GetResourceSpans() {
		service := "unknown"
		host := srcIP
		for _, attr := range rs.GetResource().GetAttributes() {
			if attr.GetKey() == "service.name" {
				service = formatAnyValue(attr.GetValue())
			}
			if attr.GetKey() == "host.name" {
				host = formatAnyValue(attr.GetValue())
			}
		}

		for _, ss := range rs.GetScopeSpans() {
			scope := ss.GetScope().GetName()
			for _, sp := range ss.GetSpans() {
				tid := hex.EncodeToString(sp.GetTraceId())
				spID := hex.EncodeToString(sp.GetSpanId())
				parentID := ""
				if len(sp.GetParentSpanId()) > 0 {
					parentID = hex.EncodeToString(sp.GetParentSpanId())
				}
				st := int64(sp.GetStartTimeUnixNano())
				et := int64(sp.GetEndTimeUnixNano())
				dur := float64(et-st) / 1e9

				var trace *datastore.OTelTraceEnt
				if v, ok := s.activeTraces.Load(tid); ok {
					trace = v.(*datastore.OTelTraceEnt)
				} else {
					bkt := time.Now().Format("2006-01-02T15:04")
					if st > 0 {
						bkt = time.Unix(0, st).Format("2006-01-02T15:04")
					}
					trace = &datastore.OTelTraceEnt{
						Bucket:  bkt,
						TraceID: tid,
						Spans:   make([]datastore.OTelTraceSpanEnt, 0),
					}
					s.activeTraces.Store(tid, trace)
				}

				trace.Spans = append(trace.Spans, datastore.OTelTraceSpanEnt{
					SpanID:       spID,
					ParentSpanID: parentID,
					Host:         host,
					Service:      service,
					Scope:        scope,
					Name:         sp.GetName(),
					Start:        st,
					End:          et,
					Dur:          dur,
					Attributes:   formatAttributes(sp.GetAttributes()),
				})

				if trace.Start == 0 || trace.Start > st {
					trace.Start = st
				}
				if trace.End < et {
					trace.End = et
				}
				if trace.Dur < dur {
					trace.Dur = dur
				}
				trace.Last = time.Now().UnixNano()
			}
		}
	}
}

func (s *OTelServer) processLogs(_ context.Context, srcIP string, body []byte, isJSON bool) {
	var req collogspb.ExportLogsServiceRequest
	var err error
	if isJSON {
		opts := protojson.UnmarshalOptions{DiscardUnknown: true}
		err = opts.Unmarshal(body, &req)
	} else {
		err = proto.Unmarshal(body, &req)
	}
	if err != nil {
		slog.Debug("OTel log unmarshal error", "error", err)
		return
	}

	for _, rl := range req.GetResourceLogs() {
		service := "unknown"
		host := srcIP
		for _, attr := range rl.GetResource().GetAttributes() {
			if attr.GetKey() == "service.name" {
				service = formatAnyValue(attr.GetValue())
			}
			if attr.GetKey() == "host.name" {
				host = formatAnyValue(attr.GetValue())
			}
		}

		for _, sl := range rl.GetScopeLogs() {
			scope := sl.GetScope().GetName()
			for _, lr := range sl.GetLogRecords() {
				t := int64(lr.GetTimeUnixNano())
				if t == 0 {
					t = int64(lr.GetObservedTimeUnixNano())
				}
				if t == 0 {
					t = time.Now().UnixNano()
				}
				tid := ""
				if len(lr.GetTraceId()) > 0 {
					tid = hex.EncodeToString(lr.GetTraceId())
				}
				spID := ""
				if len(lr.GetSpanId()) > 0 {
					spID = hex.EncodeToString(lr.GetSpanId())
				}

				bodyText := formatAnyValue(lr.GetBody())
				sevNum := int(lr.GetSeverityNumber())
				sevText := lr.GetSeverityText()
				if sevText == "" {
					sevText = lr.GetSeverityNumber().String()
				}
				sevLevel := getOTelLogSeverity(sevNum)

				attrMap := make(map[string]string)
				for _, kv := range lr.GetAttributes() {
					attrMap[kv.GetKey()] = formatAnyValue(kv.GetValue())
				}

				ent := datastore.OTelLogEnt{
					Time:         t,
					Host:         host,
					Service:      service,
					Scope:        scope,
					TraceID:      tid,
					SpanID:       spID,
					Severity:     sevLevel,
					SeverityText: sevText,
					Message:      bodyText,
					Attributes:   attrMap,
				}

				if s.logStore != nil {
					rawJSON, err := json.Marshal(ent)
					if err == nil {
						_ = s.logStore.WriteLog(&parquet.ParquetLogRecord{
							Time: t,
							Type: "otel",
							Src:  host,
							Log:  string(rawJSON),
						})
					}
				}
			}
		}
	}
}

func getOTelLogSeverity(s int) int {
	switch {
	case s <= 8:
		return 7 // Debug
	case s <= 12:
		return 6 // Info
	case s <= 16:
		return 4 // Warn
	case s <= 20:
		return 3 // Error
	case s <= 24:
		return 2 // Crit
	default:
		return 6 // Default Info
	}
}

func formatAttributes(attrs []*commonpb.KeyValue) []string {
	res := make([]string, 0, len(attrs))
	for _, kv := range attrs {
		res = append(res, fmt.Sprintf("%s=%s", kv.GetKey(), formatAnyValue(kv.GetValue())))
	}
	sort.Strings(res)
	return res
}

func formatAnyValue(v *commonpb.AnyValue) string {
	if v == nil {
		return ""
	}
	switch val := v.GetValue().(type) {
	case *commonpb.AnyValue_StringValue:
		return val.StringValue
	case *commonpb.AnyValue_BoolValue:
		return fmt.Sprintf("%t", val.BoolValue)
	case *commonpb.AnyValue_IntValue:
		return fmt.Sprintf("%d", val.IntValue)
	case *commonpb.AnyValue_DoubleValue:
		return fmt.Sprintf("%f", val.DoubleValue)
	case *commonpb.AnyValue_ArrayValue:
		items := make([]string, 0)
		for _, item := range val.ArrayValue.GetValues() {
			items = append(items, formatAnyValue(item))
		}
		return fmt.Sprintf("[%s]", strings.Join(items, ","))
	case *commonpb.AnyValue_KvlistValue:
		return strings.Join(formatAttributes(val.KvlistValue.GetValues()), " ")
	}
	return ""
}
