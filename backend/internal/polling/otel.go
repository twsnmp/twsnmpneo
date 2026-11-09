package polling

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/robertkrimen/otto"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

// OTelPoller evaluates OpenTelemetry metrics and distributed traces directly from datastore.
type OTelPoller struct {
	store datastore.DataStore
}

// NewOTelPoller creates an initialized OTelPoller.
func NewOTelPoller(store datastore.DataStore) *OTelPoller {
	return &OTelPoller{store: store}
}

// Poll executes an OpenTelemetry check based on mode ("metric", "trace", "heartbeat", "last").
func (p *OTelPoller) Poll(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	if p.store == nil {
		return &Result{
			State:   StateUnknown,
			Message: "datastore is nil",
			Fields:  map[string]interface{}{"error": "datastore is nil"},
		}, nil
	}

	mode := strings.ToLower(pe.Mode)
	switch mode {
	case "trace":
		return p.pollTrace(ctx, pe, node)
	case "heartbeat", "last":
		return p.pollHeartbeat(ctx, pe, node)
	default: // "metric", ""
		return p.pollMetric(ctx, pe, node)
	}
}

func parseOTelParams(params string) map[string]string {
	ret := make(map[string]string)
	if params == "" {
		return ret
	}
	parts := strings.Split(params, ",")
	isKV := false
	for _, p := range parts {
		kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
		if len(kv) == 2 {
			ret[strings.ToLower(strings.TrimSpace(kv[0]))] = strings.TrimSpace(kv[1])
			isKV = true
		}
	}
	if !isKV {
		// Positional format: host,service,scope,name
		fields := []string{"host", "service", "scope", "name"}
		for i, p := range parts {
			if i < len(fields) {
				ret[fields[i]] = strings.TrimSpace(p)
			}
		}
	}
	return ret
}

func (p *OTelPoller) pollMetric(ctx context.Context, pe *datastore.PollingEnt, _ *datastore.NodeEnt) (*Result, error) {
	pMap := parseOTelParams(pe.Params)
	host := pMap["host"]
	service := pMap["service"]
	scope := pMap["scope"]
	name := pMap["name"]

	var targetMetric *datastore.OTelMetricEnt
	if host != "" && service != "" && scope != "" && name != "" {
		targetMetric, _ = p.store.GetOTelMetric(ctx, host, service, scope, name)
	}
	if targetMetric == nil {
		p.store.ForEachOTelMetric(func(m *datastore.OTelMetricEnt) bool {
			if host != "" && m.Host != host {
				return true
			}
			if service != "" && m.Service != service {
				return true
			}
			if scope != "" && m.Scope != scope {
				return true
			}
			if name != "" && m.Name != name {
				return true
			}
			targetMetric = m
			return false
		})
	}

	if targetMetric == nil {
		return &Result{
			State:   failureState(pe.Level),
			Message: "metric not found",
			Fields:  map[string]interface{}{"error": "metric not found"},
		}, nil
	}

	now := time.Now().UnixNano()
	var gaugeVal, sumVal, minVal, maxVal, countVal float64
	var dpTime int64

	if len(targetMetric.DataPoints) > 0 {
		dp := targetMetric.DataPoints[len(targetMetric.DataPoints)-1]
		dpTime = dp.Time
		gaugeVal = dp.Gauge
		sumVal = dp.Sum
		minVal = dp.Min
		maxVal = dp.Max
		if dp.Count > 0 {
			countVal = float64(dp.Count)
		} else {
			countVal = float64(targetMetric.Count)
		}
	} else {
		dpTime = targetMetric.Last
		countVal = float64(targetMetric.Count)
	}

	elapsedSec := float64(now-dpTime) / 1e9
	if elapsedSec < 0 {
		elapsedSec = 0
	}

	fields := map[string]interface{}{
		"gauge":    gaugeVal,
		"sum":      sumVal,
		"min":      minVal,
		"max":      maxVal,
		"count":    countVal,
		"lastTime": float64(dpTime),
		"elapsed":  elapsedSec,
		"interval": float64(pe.PollInt),
	}

	if pe.Script == "" {
		return &Result{
			State:   StateNormal,
			Message: fmt.Sprintf("metric %s:%s active (gauge=%.2f, count=%.0f)", targetMetric.Service, targetMetric.Name, gaugeVal, countVal),
			Fields:  fields,
		}, nil
	}

	vm := otto.New()
	SetupOttoVM(pe, vm, fields)
	_ = vm.Set("interval", float64(pe.PollInt))

	val, err := vm.Run(pe.Script)
	if err != nil {
		fields["error"] = err.Error()
		return &Result{
			State:   failureState(pe.Level),
			Message: fmt.Sprintf("invalid script err=%v", err),
			Fields:  fields,
		}, nil
	}
	if ok, _ := val.ToBoolean(); ok {
		return &Result{
			State:   StateNormal,
			Message: "metric script evaluated to true",
			Fields:  fields,
		}, nil
	}
	return &Result{
		State:   failureState(pe.Level),
		Message: "metric script evaluated to false",
		Fields:  fields,
	}, nil
}

func (p *OTelPoller) pollTrace(ctx context.Context, pe *datastore.PollingEnt, _ *datastore.NodeEnt) (*Result, error) {
	pMap := parseOTelParams(pe.Params)
	serviceFilter := pMap["service"]
	scopeFilter := pMap["scope"]
	nameFilter := pMap["name"]

	st := time.Now().Add(-time.Second * time.Duration(pe.PollInt)).UnixNano()
	if pe.Result != nil {
		if v, ok := pe.Result["lastTime"]; ok {
			if vf, ok := v.(float64); ok && int64(vf) > 0 {
				st = int64(vf)
			}
		}
	}
	et := time.Now().UnixNano()

	buckets, err := p.store.GetOTelTraceBuckets(ctx)
	if err != nil {
		return &Result{
			State:   StateUnknown,
			Message: fmt.Sprintf("failed to get trace buckets: %v", err),
			Fields:  map[string]interface{}{"error": err.Error()},
		}, nil
	}

	reqBuckets := make(map[string]bool)
	for t := st; t <= et; t += int64(time.Minute) {
		reqBuckets[time.Unix(0, t).Format("2006-01-02T15:04")] = true
	}
	reqBuckets[time.Unix(0, et).Format("2006-01-02T15:04")] = true

	var totalSpans int
	var errorSpans int
	var durList []float64
	var maxDur float64
	var durSum float64

	for _, b := range buckets {
		if !reqBuckets[b] {
			continue
		}
		_ = p.store.ForEachOTelTrace(b, func(t *datastore.OTelTraceEnt) bool {
			if t.End < st || t.Start > et {
				return true
			}
			for _, span := range t.Spans {
				if span.End < st || span.Start > et {
					continue
				}
				if serviceFilter != "" && span.Service != serviceFilter {
					continue
				}
				if scopeFilter != "" && span.Scope != scopeFilter {
					continue
				}
				if nameFilter != "" && span.Name != nameFilter {
					continue
				}

				totalSpans++
				durList = append(durList, span.Dur)
				durSum += span.Dur
				if span.Dur > maxDur {
					maxDur = span.Dur
				}

				if isOTelSpanError(span) {
					errorSpans++
				}
			}
			return true
		})
	}

	var errorRate float64
	var avgDur float64
	var p95Dur float64

	if totalSpans > 0 {
		errorRate = (float64(errorSpans) / float64(totalSpans)) * 100.0
		avgDur = durSum / float64(totalSpans)
		sort.Float64s(durList)
		idx95 := int(math.Ceil(float64(totalSpans)*0.95)) - 1
		if idx95 >= 0 && idx95 < len(durList) {
			p95Dur = durList[idx95]
		}
	}

	fields := map[string]interface{}{
		"count":     float64(totalSpans),
		"error":     float64(errorSpans),
		"errorRate": errorRate,
		"avgDur":    avgDur,
		"maxDur":    maxDur,
		"p95Dur":    p95Dur,
		"lastTime":  float64(et),
		"interval":  float64(pe.PollInt),
	}

	if pe.Script == "" {
		if errorSpans > 0 {
			return &Result{
				State:   failureState(pe.Level),
				Message: fmt.Sprintf("%d trace error spans detected (errorRate=%.1f%%)", errorSpans, errorRate),
				Fields:  fields,
			}, nil
		}
		return &Result{
			State:   StateNormal,
			Message: fmt.Sprintf("%d spans processed without error (p95=%.3fs)", totalSpans, p95Dur),
			Fields:  fields,
		}, nil
	}

	vm := otto.New()
	SetupOttoVM(pe, vm, fields)
	_ = vm.Set("interval", float64(pe.PollInt))

	val, err := vm.Run(pe.Script)
	if err != nil {
		fields["error"] = err.Error()
		return &Result{
			State:   failureState(pe.Level),
			Message: fmt.Sprintf("invalid script err=%v", err),
			Fields:  fields,
		}, nil
	}
	if ok, _ := val.ToBoolean(); ok {
		return &Result{
			State:   StateNormal,
			Message: fmt.Sprintf("trace script evaluated to true (count=%d, error=%d, p95=%.3fs)", totalSpans, errorSpans, p95Dur),
			Fields:  fields,
		}, nil
	}
	return &Result{
		State:   failureState(pe.Level),
		Message: fmt.Sprintf("trace script evaluated to false (count=%d, error=%d, errorRate=%.1f%%, p95=%.3fs)", totalSpans, errorSpans, errorRate, p95Dur),
		Fields:  fields,
	}, nil
}

func isOTelSpanError(span datastore.OTelTraceSpanEnt) bool {
	for _, attr := range span.Attributes {
		lower := strings.ToLower(attr)
		if strings.Contains(lower, "error=true") ||
			strings.Contains(lower, "status_code=status_code_error") ||
			strings.Contains(lower, "status.code=status_code_error") ||
			strings.Contains(lower, "status.code=error") {
			return true
		}
		if strings.HasPrefix(lower, "http.status_code=") || strings.HasPrefix(lower, "http.response.status_code=") {
			parts := strings.Split(lower, "=")
			if len(parts) == 2 {
				if code, err := strconv.Atoi(parts[1]); err == nil && code >= 400 {
					return true
				}
			}
		}
		if strings.HasPrefix(lower, "rpc.grpc.status_code=") {
			parts := strings.Split(lower, "=")
			if len(parts) == 2 {
				if code, err := strconv.Atoi(parts[1]); err == nil && code != 0 {
					return true
				}
			}
		}
	}
	return false
}

func (p *OTelPoller) pollHeartbeat(ctx context.Context, pe *datastore.PollingEnt, _ *datastore.NodeEnt) (*Result, error) {
	pMap := parseOTelParams(pe.Params)
	serviceFilter := pMap["service"]
	hostFilter := pMap["host"]

	var latestTime int64
	now := time.Now().UnixNano()

	// Check metrics
	p.store.ForEachOTelMetric(func(m *datastore.OTelMetricEnt) bool {
		if serviceFilter != "" && m.Service != serviceFilter {
			return true
		}
		if hostFilter != "" && m.Host != hostFilter {
			return true
		}
		if m.Last > latestTime {
			latestTime = m.Last
		}
		return true
	})

	// Check traces from recent buckets
	buckets, _ := p.store.GetOTelTraceBuckets(ctx)
	if len(buckets) > 0 {
		for i := len(buckets) - 1; i >= 0 && i >= len(buckets)-3; i-- {
			_ = p.store.ForEachOTelTrace(buckets[i], func(t *datastore.OTelTraceEnt) bool {
				for _, span := range t.Spans {
					if serviceFilter != "" && span.Service != serviceFilter {
						continue
					}
					if hostFilter != "" && span.Host != hostFilter {
						continue
					}
					if span.End > latestTime {
						latestTime = span.End
					}
				}
				return true
			})
		}
	}

	if latestTime == 0 {
		return &Result{
			State:   failureState(pe.Level),
			Message: "no otel telemetry received",
			Fields:  map[string]interface{}{"error": "no otel telemetry received"},
		}, nil
	}

	elapsedSec := float64(now-latestTime) / 1e9
	if elapsedSec < 0 {
		elapsedSec = 0
	}

	thresholdSec := float64(pe.PollInt * 2)
	if pe.Filter != "" {
		if v, err := strconv.ParseFloat(pe.Filter, 64); err == nil && v > 0 {
			thresholdSec = v
		}
	}

	fields := map[string]interface{}{
		"elapsed":   elapsedSec,
		"threshold": thresholdSec,
		"lastTime":  float64(latestTime),
		"interval":  float64(pe.PollInt),
	}

	if pe.Script == "" {
		if elapsedSec > thresholdSec {
			return &Result{
				State:   failureState(pe.Level),
				Message: fmt.Sprintf("otel telemetry timeout: elapsed %.1fs > threshold %.1fs", elapsedSec, thresholdSec),
				Fields:  fields,
			}, nil
		}
		return &Result{
			State:   StateNormal,
			Message: fmt.Sprintf("otel telemetry active: elapsed %.1fs <= threshold %.1fs", elapsedSec, thresholdSec),
			Fields:  fields,
		}, nil
	}

	vm := otto.New()
	SetupOttoVM(pe, vm, fields)
	_ = vm.Set("threshold", thresholdSec)
	_ = vm.Set("interval", float64(pe.PollInt))

	val, err := vm.Run(pe.Script)
	if err != nil {
		fields["error"] = err.Error()
		return &Result{
			State:   failureState(pe.Level),
			Message: fmt.Sprintf("invalid script err=%v", err),
			Fields:  fields,
		}, nil
	}
	if ok, _ := val.ToBoolean(); ok {
		return &Result{
			State:   StateNormal,
			Message: fmt.Sprintf("heartbeat script evaluated to true (elapsed=%.1fs)", elapsedSec),
			Fields:  fields,
		}, nil
	}
	return &Result{
		State:   failureState(pe.Level),
		Message: fmt.Sprintf("heartbeat script evaluated to false (elapsed=%.1fs > threshold=%.1fs)", elapsedSec, thresholdSec),
		Fields:  fields,
	}, nil
}
