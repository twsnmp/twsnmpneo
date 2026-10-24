package polling

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/robertkrimen/otto"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	"github.com/twsnmp/twsnmpneo/backend/internal/extractor"
)

// SyslogPoller inspects ingested Syslog messages with regex, grok, or sigma rules.
type SyslogPoller struct {
	store    datastore.DataStore
	logStore *parquet.Store
}

func NewSyslogPoller(store datastore.DataStore, logStore *parquet.Store) *SyslogPoller {
	return &SyslogPoller{
		store:    store,
		logStore: logStore,
	}
}

func (p *SyslogPoller) Poll(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	start := time.Now()
	mode := strings.ToLower(pe.Mode)
	switch mode {
	case "", "count":
		return p.pollCount(ctx, pe, node, start)
	case "pri":
		return p.pollPri(ctx, pe, node, start)
	case "stats":
		return p.pollStats(ctx, pe, node, start)
	case "sigma":
		return p.pollSigma(ctx, pe, node, start)
	case "twagent":
		return p.pollTwAgent(ctx, pe, node, start)
	default:
		return &Result{
			State:   StateUnknown,
			Message: fmt.Sprintf("unsupported syslog mode: %s", pe.Mode),
			Fields:  map[string]interface{}{"error": fmt.Sprintf("unsupported syslog mode: %s", pe.Mode)},
		}, nil
	}
}

func (p *SyslogPoller) pollCount(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt, start time.Time) (*Result, error) {
	st := time.Now().Add(-time.Duration(pe.PollInt) * time.Second).UnixNano()
	if pe.Result != nil {
		if lt, ok := pe.Result["lastTime"].(float64); ok && int64(lt) > 0 {
			st = int64(lt)
		}
	}
	et := time.Now().UnixNano()

	host := pe.Params
	if host == "" && node != nil {
		host = node.IP
	}

	var reg *regexp.Regexp
	if pe.Filter != "" {
		r, err := regexp.Compile(pe.Filter)
		if err != nil {
			return &Result{State: StateUnknown, Message: fmt.Sprintf("invalid filter regex: %v", err)}, nil
		}
		reg = r
	}

	logs := p.querySyslog(ctx, st, et, host)
	fields := map[string]interface{}{}
	count := 0
	lastMsg := ""
	var threatCount, compCount, fimCount, resourceCount float64
	var lastRule, lastOp, lastPath, lastResource string

	for _, rec := range logs {
		msg := rec.Log
		if reg != nil && !reg.MatchString(msg) {
			continue
		}
		count++
		lastMsg = msg
		if sd, tag := parseTwAgentSyslogRecord(msg); len(sd) > 0 || strings.HasPrefix(tag, "twagentneo") {
			if strings.EqualFold(sd["threat"], "YES") {
				threatCount++
			}
			if strings.EqualFold(sd["comp"], "YES") {
				compCount++
			}
			if r := sd["rule"]; r != "" {
				lastRule = r
			}
			if op := sd["op"]; op != "" || tag == "twagentneo-fim" {
				fimCount++
				if op != "" {
					lastOp = op
				}
				if pt := sd["path"]; pt != "" {
					lastPath = pt
				}
			}
			if res := sd["resource"]; res != "" || tag == "twagentneo-resource" {
				resourceCount++
				if res != "" {
					lastResource = res
				}
			}
		}
	}

	fields["count"] = float64(count)
	fields["threatCount"] = threatCount
	fields["compCount"] = compCount
	fields["fimCount"] = fimCount
	fields["resourceCount"] = resourceCount
	if lastRule != "" {
		fields["lastRule"] = lastRule
	}
	if lastOp != "" {
		fields["lastOp"] = lastOp
	}
	if lastPath != "" {
		fields["lastPath"] = lastPath
	}
	if lastResource != "" {
		fields["lastResource"] = lastResource
	}
	fields["lastTime"] = float64(et)
	fields["rtt"] = float64(time.Since(start).Nanoseconds())

	// Grok / body extraction if extractor is configured
	vm := otto.New()
	SetupOttoVM(pe, vm, fields)
	extractor.RegisterBodyHelpers(lastMsg, vm)

	if pe.Extractor != "" && lastMsg != "" {
		_ = extractor.ApplyExtractor(pe.Extractor, lastMsg, vm, fields)
	}

	_ = vm.Set("count", float64(count))
	_ = vm.Set("interval", float64(pe.PollInt))
	for k, v := range fields {
		_ = vm.Set(k, v)
	}

	if pe.Script == "" {
		state := StateNormal
		if count > 0 && pe.Level != "off" {
			state = failureState(pe.Level)
		}
		return &Result{
			State:   state,
			RTT:     time.Since(start),
			Message: fmt.Sprintf("syslog count: %d matching logs", count),
			Fields:  fields,
		}, nil
	}

	val, err := vm.Run(pe.Script)
	if err != nil {
		fields["error"] = err.Error()
		return &Result{State: StateUnknown, Message: fmt.Sprintf("script error: %v", err), Fields: fields}, nil
	}
	pass, _ := val.ToBoolean()
	if pass {
		return &Result{State: StateNormal, RTT: time.Since(start), Message: fmt.Sprintf("syslog count: %d, script passed", count), Fields: fields}, nil
	}
	return &Result{State: failureState(pe.Level), RTT: time.Since(start), Message: fmt.Sprintf("syslog count: %d, threshold breached", count), Fields: fields}, nil
}

func (p *SyslogPoller) pollPri(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt, start time.Time) (*Result, error) {
	st := time.Now().Add(-time.Duration(pe.PollInt) * time.Second).UnixNano()
	if pe.Result != nil {
		if lt, ok := pe.Result["lastTime"].(float64); ok && int64(lt) > 0 {
			st = int64(lt)
		}
	}
	et := time.Now().UnixNano()

	host := pe.Params
	if host == "" && node != nil {
		host = node.IP
	}

	var reg *regexp.Regexp
	if pe.Filter != "" {
		if r, err := regexp.Compile(pe.Filter); err == nil {
			reg = r
		}
	}

	logs := p.querySyslog(ctx, st, et, host)
	fields := map[string]interface{}{}
	count := 0
	priMap := make(map[int]int)

	type miniSyslog struct {
		Facility int    `json:"facility"`
		Severity int    `json:"severity"`
		Tag      string `json:"tag"`
		Message  string `json:"message"`
	}

	for _, rec := range logs {
		var sl miniSyslog
		_ = json.Unmarshal([]byte(rec.Log), &sl)
		msg := sl.Tag + " " + sl.Message
		if reg != nil && !reg.MatchString(msg) {
			continue
		}
		count++
		pri := sl.Facility*8 + sl.Severity
		priMap[pri]++
	}

	fields["count"] = float64(count)
	fields["lastTime"] = float64(et)
	fields["rtt"] = float64(time.Since(start).Nanoseconds())
	for pri, c := range priMap {
		fields[fmt.Sprintf("pri_%d", pri)] = float64(c)
	}

	vm := otto.New()
	SetupOttoVM(pe, vm, fields)
	_ = vm.Set("count", float64(count))
	_ = vm.Set("interval", float64(pe.PollInt))
	for k, v := range fields {
		_ = vm.Set(k, v)
	}

	if pe.Script == "" {
		return &Result{
			State:   StateNormal,
			RTT:     time.Since(start),
			Message: fmt.Sprintf("syslog pri: %d logs", count),
			Fields:  fields,
		}, nil
	}

	val, err := vm.Run(pe.Script)
	if err != nil {
		fields["error"] = err.Error()
		return &Result{State: StateUnknown, Message: fmt.Sprintf("script error: %v", err), Fields: fields}, nil
	}
	pass, _ := val.ToBoolean()
	if pass {
		return &Result{State: StateNormal, RTT: time.Since(start), Message: "syslog pri script passed", Fields: fields}, nil
	}
	return &Result{State: failureState(pe.Level), RTT: time.Since(start), Message: "syslog pri threshold breached", Fields: fields}, nil
}

func (p *SyslogPoller) pollStats(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt, start time.Time) (*Result, error) {
	st := time.Now().Add(-time.Duration(pe.PollInt) * time.Second).UnixNano()
	if pe.Result != nil {
		if lt, ok := pe.Result["lastTime"].(float64); ok && int64(lt) > 0 {
			st = int64(lt)
		}
	}
	et := time.Now().UnixNano()

	host := pe.Params
	if host == "" && node != nil {
		host = node.IP
	}

	logs := p.querySyslog(ctx, st, et, host)
	fields := map[string]interface{}{}
	fields["count"] = float64(len(logs))
	fields["lastTime"] = float64(et)
	fields["rtt"] = float64(time.Since(start).Nanoseconds())

	vm := otto.New()
	SetupOttoVM(pe, vm, fields)
	_ = vm.Set("count", float64(len(logs)))
	_ = vm.Set("interval", float64(pe.PollInt))
	for k, v := range fields {
		_ = vm.Set(k, v)
	}

	if pe.Script == "" {
		return &Result{
			State:   StateNormal,
			RTT:     time.Since(start),
			Message: fmt.Sprintf("syslog stats: %d logs", len(logs)),
			Fields:  fields,
		}, nil
	}

	val, err := vm.Run(pe.Script)
	if err != nil {
		fields["error"] = err.Error()
		return &Result{State: StateUnknown, Message: fmt.Sprintf("stats script error: %v", err), Fields: fields}, nil
	}
	pass, _ := val.ToBoolean()
	if pass {
		return &Result{State: StateNormal, RTT: time.Since(start), Message: "syslog stats script passed", Fields: fields}, nil
	}
	return &Result{State: failureState(pe.Level), RTT: time.Since(start), Message: "syslog stats threshold breached", Fields: fields}, nil
}

func (p *SyslogPoller) pollSigma(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt, start time.Time) (*Result, error) {
	st := time.Now().Add(-time.Duration(pe.PollInt) * time.Second).UnixNano()
	if pe.Result != nil {
		if lt, ok := pe.Result["lastTime"].(float64); ok && int64(lt) > 0 {
			st = int64(lt)
		}
	}
	et := time.Now().UnixNano()

	host := pe.Params
	if host == "" && node != nil {
		host = node.IP
	}

	logs := p.querySyslog(ctx, st, et, host)

	// Scan logs and analyze threat levels
	var critical, high, medium, low, info, compliance, hitLogs, threatCount, fimCount float64
	scanned := float64(len(logs))

	for _, rec := range logs {
		sd, tag := parseTwAgentSyslogRecord(rec.Log)
		if len(sd) > 0 || strings.HasPrefix(tag, "twagentneo") {
			if strings.EqualFold(sd["comp"], "YES") {
				compliance++
			}
			if sd["op"] != "" || tag == "twagentneo-fim" {
				fimCount++
				high++
				hitLogs++
				continue
			}
			if strings.EqualFold(sd["threat"], "NO") {
				// Filtered out by tensai AI Judge as benign false positive
				info++
				continue
			}
			if strings.EqualFold(sd["threat"], "YES") {
				threatCount++
				critical++
				hitLogs++
				continue
			}
		}

		upper := strings.ToUpper(rec.Log)
		matched := false
		if strings.Contains(upper, "EMERG") || strings.Contains(upper, "ALERT") || strings.Contains(upper, "CRIT") || strings.Contains(upper, "FATAL") {
			critical++
			matched = true
		} else if strings.Contains(upper, "ERROR") || strings.Contains(upper, "ERR") || strings.Contains(upper, "FAIL") || strings.Contains(upper, "ATTACK") {
			high++
			matched = true
		} else if strings.Contains(upper, "WARN") || strings.Contains(upper, "DENIED") {
			medium++
			matched = true
		} else if strings.Contains(upper, "NOTICE") {
			low++
			matched = true
		} else {
			info++
		}
		if strings.Contains(upper, "PCI") || strings.Contains(upper, "HIPAA") || strings.Contains(upper, "AUDIT") {
			compliance++
		}
		if matched {
			hitLogs++
		}
	}

	totalDetections := critical + high + medium + low

	fields := map[string]interface{}{
		"count":       totalDetections,
		"hitLogs":     hitLogs,
		"critical":    critical,
		"high":        high,
		"medium":      medium,
		"low":         low,
		"info":        info,
		"compliance":  compliance,
		"threatCount": threatCount,
		"fimCount":    fimCount,
		"scanned":     scanned,
		"lastTime":    float64(et),
		"rtt":         float64(time.Since(start).Nanoseconds()),
	}

	vm := otto.New()
	SetupOttoVM(pe, vm, fields)
	for k, v := range fields {
		_ = vm.Set(k, v)
	}

	if pe.Script == "" {
		state := StateNormal
		if totalDetections > 0 {
			state = failureState(pe.Level)
		}
		return &Result{
			State:   state,
			RTT:     time.Since(start),
			Message: fmt.Sprintf("sigma detections: crit=%.0f high=%.0f med=%.0f", critical, high, medium),
			Fields:  fields,
		}, nil
	}

	val, err := vm.Run(pe.Script)
	if err != nil {
		fields["error"] = err.Error()
		return &Result{State: StateUnknown, Message: fmt.Sprintf("sigma script error: %v", err), Fields: fields}, nil
	}
	pass, _ := val.ToBoolean()
	if pass {
		return &Result{State: StateNormal, RTT: time.Since(start), Message: fmt.Sprintf("sigma ok: crit=%.0f high=%.0f", critical, high), Fields: fields}, nil
	}
	return &Result{State: failureState(pe.Level), RTT: time.Since(start), Message: fmt.Sprintf("sigma threat detected: crit=%.0f high=%.0f", critical, high), Fields: fields}, nil
}

func (p *SyslogPoller) pollTwAgent(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt, start time.Time) (*Result, error) {
	st := time.Now().Add(-time.Duration(pe.PollInt) * time.Second).UnixNano()
	if pe.Result != nil {
		if lt, ok := pe.Result["lastTime"].(float64); ok && int64(lt) > 0 {
			st = int64(lt)
		}
	}
	et := time.Now().UnixNano()

	host := pe.Params
	if host == "" && node != nil {
		host = node.IP
	}

	var reg *regexp.Regexp
	if pe.Filter != "" {
		if r, err := regexp.Compile(pe.Filter); err == nil {
			reg = r
		}
	}

	logs := p.querySyslog(ctx, st, et, host)
	var count, threatCount, compCount, fimCount, resourceCount float64
	var lastRule, lastOp, lastPath, lastResource string

	for _, rec := range logs {
		if reg != nil && !reg.MatchString(rec.Log) {
			continue
		}
		sd, tag := parseTwAgentSyslogRecord(rec.Log)
		if len(sd) == 0 && !strings.HasPrefix(tag, "twagentneo") {
			continue
		}
		count++
		if strings.EqualFold(sd["threat"], "YES") {
			threatCount++
		}
		if strings.EqualFold(sd["comp"], "YES") {
			compCount++
		}
		if r := sd["rule"]; r != "" {
			lastRule = r
		}
		if op := sd["op"]; op != "" || tag == "twagentneo-fim" {
			fimCount++
			if op != "" {
				lastOp = op
			}
			if pt := sd["path"]; pt != "" {
				lastPath = pt
			}
		}
		if res := sd["resource"]; res != "" || tag == "twagentneo-resource" {
			resourceCount++
			if res != "" {
				lastResource = res
			}
		}
	}

	fields := map[string]interface{}{
		"count":         count,
		"threatCount":   threatCount,
		"compCount":     compCount,
		"fimCount":      fimCount,
		"resourceCount": resourceCount,
		"lastRule":      lastRule,
		"lastOp":        lastOp,
		"lastPath":      lastPath,
		"lastResource":  lastResource,
		"lastTime":      float64(et),
		"rtt":           float64(time.Since(start).Nanoseconds()),
	}

	vm := otto.New()
	SetupOttoVM(pe, vm, fields)
	for k, v := range fields {
		_ = vm.Set(k, v)
	}

	if pe.Script == "" {
		state := StateNormal
		if (threatCount > 0 || compCount > 0 || fimCount > 0 || resourceCount > 0) && pe.Level != "off" {
			state = failureState(pe.Level)
		}
		return &Result{
			State:   state,
			RTT:     time.Since(start),
			Message: fmt.Sprintf("twagent syslog: threat=%.0f comp=%.0f fim=%.0f res=%.0f", threatCount, compCount, fimCount, resourceCount),
			Fields:  fields,
		}, nil
	}

	val, err := vm.Run(pe.Script)
	if err != nil {
		fields["error"] = err.Error()
		return &Result{State: StateUnknown, Message: fmt.Sprintf("twagent syslog script error: %v", err), Fields: fields}, nil
	}
	pass, _ := val.ToBoolean()
	if pass {
		return &Result{State: StateNormal, RTT: time.Since(start), Message: fmt.Sprintf("twagent syslog ok: threat=%.0f fim=%.0f", threatCount, fimCount), Fields: fields}, nil
	}
	return &Result{State: failureState(pe.Level), RTT: time.Since(start), Message: fmt.Sprintf("twagent syslog alert: threat=%.0f comp=%.0f fim=%.0f", threatCount, compCount, fimCount), Fields: fields}, nil
}

var sdKVRegex = regexp.MustCompile(`([a-zA-Z0-9_]+)="((?:\\.|[^"\\])*)"`)

func parseTwAgentSyslogRecord(rawLog string) (map[string]string, string) {
	sdMap := make(map[string]string)
	var tag string

	if strings.HasPrefix(strings.TrimSpace(rawLog), "{") {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(rawLog), &m); err == nil {
			if t, ok := m["app_name"].(string); ok && t != "" {
				tag = t
			} else if t, ok := m["tag"].(string); ok && t != "" {
				tag = t
			}
			for _, field := range []string{"structured_data", "content", "message"} {
				if s, ok := m[field].(string); ok && strings.Contains(s, "twsnmpneo@17862") {
					extractSDPairs(s, sdMap)
				}
			}
		}
	}

	if len(sdMap) == 0 && strings.Contains(rawLog, "twsnmpneo@17862") {
		extractSDPairs(rawLog, sdMap)
	}
	return sdMap, strings.ToLower(strings.TrimSpace(tag))
}

func extractSDPairs(s string, out map[string]string) {
	idx := strings.Index(s, "[twsnmpneo@17862")
	if idx < 0 {
		return
	}
	sub := s[idx:]
	matches := sdKVRegex.FindAllStringSubmatch(sub, -1)
	for _, m := range matches {
		if len(m) == 3 {
			val := strings.ReplaceAll(m[2], `\"`, `"`)
			val = strings.ReplaceAll(val, `\\`, `\`)
			out[m[1]] = val
		}
	}
}

func (p *SyslogPoller) querySyslog(ctx context.Context, st, et int64, host string) []*parquet.ParquetLogRecord {
	if p.logStore == nil {
		return nil
	}
	logs, err := p.logStore.Query(ctx, parquet.LogFilter{
		Type:      "syslog",
		StartTime: st,
		EndTime:   et,
		Src:       host,
		Limit:     5000,
	})
	if err != nil {
		return nil
	}
	return logs
}
