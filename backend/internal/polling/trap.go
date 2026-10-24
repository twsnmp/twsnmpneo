package polling

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/robertkrimen/otto"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	"github.com/twsnmp/twsnmpneo/backend/internal/extractor"
)

// SnmpTrapPoller monitors received SNMP TRAP packets.
type SnmpTrapPoller struct {
	store    datastore.DataStore
	logStore *parquet.Store
}

func NewSnmpTrapPoller(store datastore.DataStore, logStore *parquet.Store) *SnmpTrapPoller {
	return &SnmpTrapPoller{
		store:    store,
		logStore: logStore,
	}
}

func (p *SnmpTrapPoller) Poll(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	mode := strings.ToLower(strings.TrimSpace(pe.Mode))
	switch mode {
	case "", "count", "stats", "twagent":
	default:
		return &Result{
			State:   StateUnknown,
			Message: fmt.Sprintf("unsupported trap mode: %s", pe.Mode),
			Fields:  map[string]interface{}{"error": fmt.Sprintf("unsupported trap mode: %s", pe.Mode)},
		}, nil
	}
	start := time.Now()
	pollSec := pe.PollInt
	if pollSec <= 0 {
		pollSec = 60
	}
	st := time.Now().Add(-time.Duration(pollSec) * time.Second).UnixNano()
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
			return &Result{State: StateUnknown, Message: fmt.Sprintf("invalid trap filter regex: %v", err)}, nil
		}
		reg = r
	}

	var logs []*parquet.ParquetLogRecord
	if p.logStore != nil {
		l, _ := p.logStore.Query(ctx, parquet.LogFilter{
			Type:      "trap",
			StartTime: st,
			EndTime:   et,
			Src:       host,
			Limit:     5000,
		})
		logs = l
	}

	count := 0
	typeMap := make(map[string]int)
	var threatCount, compCount, fimCount, resourceCount, heartbeatCount float64
	lastMsg := ""
	lastVars := make(map[string]string)
	lastTrapType := ""

	type trapRecJSON struct {
		FromAddress string `json:"FromAddress"`
		TrapType    string `json:"TrapType"`
		Variables   string `json:"Variables"`
		Enterprise  string `json:"Enterprise"`
	}

	for _, rec := range logs {
		msg := rec.Log
		if reg != nil && !reg.MatchString(msg) {
			continue
		}
		count++
		typeMap[rec.Src]++
		lastMsg = msg

		var tr trapRecJSON
		if err := json.Unmarshal([]byte(msg), &tr); err == nil {
			if tr.TrapType != "" {
				lastTrapType = tr.TrapType
			}
			vbMap := ParseTrapVariables(tr.Variables)
			for k, v := range vbMap {
				lastVars[k] = v
			}

			ttLower := strings.ToLower(tr.TrapType)
			catLower := strings.ToLower(vbMap["twagentNeoTrapCategory"])
			judgeStr := vbMap["twagentNeoTrapAIJudge"]

			if strings.Contains(ttLower, "twagentneofim") || strings.HasSuffix(ttLower, "17862.3.2.0.3") || catLower == "fim" {
				fimCount++
			} else if strings.Contains(ttLower, "twagentneoresource") || strings.HasSuffix(ttLower, "17862.3.2.0.1") || catLower == "resource" {
				resourceCount++
			} else if strings.Contains(ttLower, "twagentneoheartbeat") || strings.HasSuffix(ttLower, "17862.3.2.0.4") || catLower == "heartbeat" {
				heartbeatCount++
			} else if strings.Contains(ttLower, "twagentneosecurity") || strings.HasSuffix(ttLower, "17862.3.2.0.2") || judgeStr != "" {
				judgeUpper := strings.ToUpper(judgeStr)
				if strings.Contains(judgeUpper, "THREAT:YES") || (strings.HasPrefix(judgeUpper, "THREAT") && !strings.Contains(judgeUpper, "THREAT:NO")) || (judgeStr == "" && !strings.Contains(judgeUpper, "THREAT:NO")) {
					threatCount++
				}
				if strings.Contains(judgeUpper, "COMPVIOLATION:YES") || strings.Contains(judgeUpper, "PCI-DSS") || strings.Contains(judgeUpper, "CIS") || strings.Contains(judgeUpper, "HIPAA") || strings.Contains(judgeUpper, "GDPR") {
					compCount++
				}
			}
		}
	}

	fields := map[string]interface{}{
		"count":          float64(count),
		"lastTime":       float64(et),
		"rtt":            float64(time.Since(start).Nanoseconds()),
		"sources":        float64(len(typeMap)),
		"threatCount":    threatCount,
		"compCount":      compCount,
		"fimCount":       fimCount,
		"resourceCount":  resourceCount,
		"heartbeatCount": heartbeatCount,
		"trapType":       lastTrapType,
	}

	// Populate parsed TWAGENT-NEO-MIB varbind fields
	for k, v := range lastVars {
		fields[k] = v
	}
	if v, ok := lastVars["twagentNeoHostname"]; ok {
		fields["hostname"] = v
	}
	if v, ok := lastVars["twagentNeoVersion"]; ok {
		fields["version"] = v
	}
	if v, ok := lastVars["twagentNeoOS"]; ok {
		fields["os"] = v
	}
	if v, ok := lastVars["twagentNeoTrapSeverity"]; ok {
		fields["severity"] = v
		if idx := strings.Index(v, "("); idx > 0 {
			if n, err := strconv.ParseFloat(v[:idx], 64); err == nil {
				fields["severityNum"] = n
			}
		} else if n, err := strconv.ParseFloat(v, 64); err == nil {
			fields["severityNum"] = n
		}
	}
	if v, ok := lastVars["twagentNeoTrapCategory"]; ok {
		fields["category"] = v
	}
	if v, ok := lastVars["twagentNeoTrapRuleTitle"]; ok {
		fields["ruleTitle"] = v
	}
	if v, ok := lastVars["twagentNeoTrapAIJudge"]; ok {
		fields["aiJudge"] = v
	}
	if v, ok := lastVars["twagentNeoTrapAISummary"]; ok {
		fields["aiSummary"] = v
	}
	if v, ok := lastVars["twagentNeoTrapAIAction"]; ok {
		fields["aiAction"] = v
	}

	vm := otto.New()
	SetupOttoVM(pe, vm, fields)
	extractor.RegisterBodyHelpers(lastMsg, vm)
	_ = vm.Set("count", float64(count))
	_ = vm.Set("interval", float64(pe.PollInt))
	for k, v := range fields {
		_ = vm.Set(k, v)
	}

	if pe.Extractor != "" && lastMsg != "" {
		_ = extractor.ApplyExtractor(pe.Extractor, lastMsg, vm, fields)
		for k, v := range fields {
			_ = vm.Set(k, v)
		}
	}

	if pe.Script == "" {
		state := StateNormal
		if mode == "twagent" {
			filterLower := strings.ToLower(pe.Filter)
			if strings.Contains(filterLower, "heartbeat") || strings.Contains(filterLower, "17862.3.2.0.4") {
				if heartbeatCount == 0 && pe.Level != "off" {
					state = failureState(pe.Level)
				}
			} else if (threatCount > 0 || compCount > 0 || fimCount > 0 || resourceCount > 0) && pe.Level != "off" {
				state = failureState(pe.Level)
			}
			return &Result{
				State:   state,
				RTT:     time.Since(start),
				Message: fmt.Sprintf("twagent trap: threat=%.0f comp=%.0f fim=%.0f res=%.0f hb=%.0f", threatCount, compCount, fimCount, resourceCount, heartbeatCount),
				Fields:  fields,
			}, nil
		}

		if count > 0 && pe.Level != "off" {
			state = failureState(pe.Level)
		}
		return &Result{
			State:   state,
			RTT:     time.Since(start),
			Message: fmt.Sprintf("snmptrap count: %d matching traps", count),
			Fields:  fields,
		}, nil
	}

	val, err := vm.Run(pe.Script)
	if err != nil {
		fields["error"] = err.Error()
		return &Result{State: StateUnknown, Message: fmt.Sprintf("trap script error: %v", err), Fields: fields}, nil
	}
	pass, _ := val.ToBoolean()
	if pass {
		return &Result{State: StateNormal, RTT: time.Since(start), Message: fmt.Sprintf("trap count: %d, script passed", count), Fields: fields}, nil
	}
	return &Result{State: failureState(pe.Level), RTT: time.Since(start), Message: fmt.Sprintf("trap count: %d, threshold breached", count), Fields: fields}, nil
}

var trapVarKeyRegex = regexp.MustCompile(`(?:^|\s)([a-zA-Z0-9_]+(?:\.[0-9]+)*)=`)

// ParseTrapVariables extracts key=value pairs from a decoded Trap Variables string,
// preserving multi-word values and normalizing trailing ".0" instance suffixes.
func ParseTrapVariables(vars string) map[string]string {
	res := make(map[string]string)
	vars = strings.TrimSpace(vars)
	if vars == "" {
		return res
	}
	matches := trapVarKeyRegex.FindAllStringSubmatchIndex(vars, -1)
	if len(matches) == 0 {
		return res
	}
	for i, m := range matches {
		key := vars[m[2]:m[3]]
		valStart := m[1]
		valEnd := len(vars)
		if i+1 < len(matches) {
			valEnd = matches[i+1][0]
		}
		val := strings.TrimRight(strings.TrimSpace(vars[valStart:valEnd]), ",")
		res[key] = val
		baseKey := strings.TrimSuffix(key, ".0")
		if baseKey != key {
			res[baseKey] = val
		}
	}
	return res
}
