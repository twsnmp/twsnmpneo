package ai

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

type mcpOTelMetricEnt struct {
	Host        string  `json:"host"`
	Service     string  `json:"service"`
	Scope       string  `json:"scope"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Unit        string  `json:"unit"`
	Count       int     `json:"count"`
	First       string  `json:"first"`
	Last        string  `json:"last"`
	LastGauge   float64 `json:"last_gauge,omitempty"`
	LastSum     float64 `json:"last_sum,omitempty"`
	LastMin     float64 `json:"last_min,omitempty"`
	LastMax     float64 `json:"last_max,omitempty"`
}

type mcpGetOTelMetricsParams struct {
	NameFilter    string `json:"name_filter,omitempty" jsonschema:"Metric name regex filter. If blank, all metrics are returned."`
	ServiceFilter string `json:"service_filter,omitempty" jsonschema:"Service name regex filter. If blank, all metrics are returned."`
	HostFilter    string `json:"host_filter,omitempty" jsonschema:"Host name or IP regex filter. If blank, all metrics are returned."`
	Limit         int    `json:"limit,omitempty" jsonschema:"Maximum number of metrics to return (1-200, default 50)."`
}

type mcpOTelTraceEnt struct {
	TraceID    string   `json:"trace_id"`
	Bucket     string   `json:"bucket"`
	Start      string   `json:"start"`
	End        string   `json:"end"`
	Dur        float64  `json:"dur_sec"`
	NumSpans   int      `json:"num_spans"`
	NumErrors  int      `json:"num_errors"`
	Services   string   `json:"services"`
	Hosts      string   `json:"hosts"`
	ErrorSpans []string `json:"error_spans,omitempty"`
}

type mcpGetOTelTracesParams struct {
	ServiceFilter string `json:"service_filter,omitempty" jsonschema:"Service name regex filter. If blank, all traces are returned."`
	ErrorOnly     bool   `json:"error_only,omitempty" jsonschema:"If true, returns only traces containing errors."`
	Limit         int    `json:"limit,omitempty" jsonschema:"Maximum number of traces to return (1-200, default 50)."`
}

func (s *MCPServer) registerOTelTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_otel_metrics",
		Description: "get OpenTelemetry metrics list or details from TWSNMP NEO",
	}, s.getOTelMetrics)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_otel_traces",
		Description: "get OpenTelemetry trace list or error traces from TWSNMP NEO",
	}, s.getOTelTraces)
}

func (s *MCPServer) getOTelMetrics(ctx context.Context, req *mcp.CallToolRequest, args mcpGetOTelMetricsParams) (*mcp.CallToolResult, any, error) {
	if s.store == nil {
		return jsonResult([]mcpOTelMetricEnt{})
	}
	limit := args.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	nameF := makeRegexFilter(args.NameFilter)
	svcF := makeRegexFilter(args.ServiceFilter)
	hostF := makeRegexFilter(args.HostFilter)

	metrics, err := s.store.ListOTelMetrics(ctx)
	if err != nil {
		return textResult(fmt.Sprintf("failed to get otel metrics: %v", err)), nil, nil
	}

	list := make([]mcpOTelMetricEnt, 0, limit)
	for _, m := range metrics {
		if nameF != nil && !nameF.MatchString(m.Name) {
			continue
		}
		if svcF != nil && !svcF.MatchString(m.Service) {
			continue
		}
		if hostF != nil && !hostF.MatchString(m.Host) {
			continue
		}

		ent := mcpOTelMetricEnt{
			Host:        m.Host,
			Service:     m.Service,
			Scope:       m.Scope,
			Name:        m.Name,
			Type:        m.Type,
			Description: m.Description,
			Unit:        m.Unit,
			Count:       m.Count,
		}
		if m.First > 0 {
			ent.First = time.Unix(0, m.First).Format(time.RFC3339Nano)
		}
		if m.Last > 0 {
			ent.Last = time.Unix(0, m.Last).Format(time.RFC3339Nano)
		}
		if len(m.DataPoints) > 0 {
			dp := m.DataPoints[len(m.DataPoints)-1]
			ent.LastGauge = dp.Gauge
			ent.LastSum = dp.Sum
			ent.LastMin = dp.Min
			ent.LastMax = dp.Max
		}
		list = append(list, ent)
		if len(list) >= limit {
			break
		}
	}
	return jsonResult(list)
}

func (s *MCPServer) getOTelTraces(ctx context.Context, req *mcp.CallToolRequest, args mcpGetOTelTracesParams) (*mcp.CallToolResult, any, error) {
	if s.store == nil {
		return jsonResult([]mcpOTelTraceEnt{})
	}
	limit := args.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	svcF := makeRegexFilter(args.ServiceFilter)

	buckets, err := s.store.GetOTelTraceBuckets(ctx)
	if err != nil {
		return textResult(fmt.Sprintf("failed to get trace buckets: %v", err)), nil, nil
	}

	list := make([]mcpOTelTraceEnt, 0, limit)
	for i := len(buckets) - 1; i >= 0 && len(list) < limit; i-- {
		_ = s.store.ForEachOTelTrace(buckets[i], func(t *datastore.OTelTraceEnt) bool {
			services := make(map[string]bool)
			hosts := make(map[string]bool)
			errorSpans := []string{}
			hasServiceMatch := (svcF == nil)

			for _, span := range t.Spans {
				services[span.Service] = true
				hosts[span.Host] = true
				if svcF != nil && svcF.MatchString(span.Service) {
					hasServiceMatch = true
				}
				isErr := false
				for _, attr := range span.Attributes {
					lower := strings.ToLower(attr)
					if strings.Contains(lower, "error=true") ||
						strings.Contains(lower, "status_code=status_code_error") ||
						strings.Contains(lower, "status.code=status_code_error") ||
						strings.Contains(lower, "status.code=error") {
						isErr = true
						break
					}
					if strings.HasPrefix(lower, "http.status_code=") || strings.HasPrefix(lower, "http.response.status_code=") {
						parts := strings.Split(lower, "=")
						if len(parts) == 2 {
							if code, err := strconv.Atoi(parts[1]); err == nil && code >= 400 {
								isErr = true
								break
							}
						}
					}
					if strings.HasPrefix(lower, "rpc.grpc.status_code=") {
						parts := strings.Split(lower, "=")
						if len(parts) == 2 {
							if code, err := strconv.Atoi(parts[1]); err == nil && code != 0 {
								isErr = true
								break
							}
						}
					}
				}
				if isErr {
					errorSpans = append(errorSpans, fmt.Sprintf("%s:%s", span.Service, span.Name))
				}
			}

			if !hasServiceMatch {
				return true
			}
			if args.ErrorOnly && len(errorSpans) == 0 {
				return true
			}

			svcList := make([]string, 0, len(services))
			for s := range services {
				svcList = append(svcList, s)
			}
			hostList := make([]string, 0, len(hosts))
			for h := range hosts {
				hostList = append(hostList, h)
			}

			stStr := ""
			etStr := ""
			if t.Start > 0 {
				stStr = time.Unix(0, t.Start).Format(time.RFC3339Nano)
			}
			if t.End > 0 {
				etStr = time.Unix(0, t.End).Format(time.RFC3339Nano)
			}

			list = append(list, mcpOTelTraceEnt{
				TraceID:    t.TraceID,
				Bucket:     t.Bucket,
				Start:      stStr,
				End:        etStr,
				Dur:        t.Dur,
				NumSpans:   len(t.Spans),
				NumErrors:  len(errorSpans),
				Services:   strings.Join(svcList, ", "),
				Hosts:      strings.Join(hostList, ", "),
				ErrorSpans: errorSpans,
			})
			return len(list) < limit
		})
	}

	return jsonResult(list)
}
