package polling

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/robertkrimen/otto"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/mib"
)

// SNMPPoller queries SNMP agents supporting 9 modes (sysUpTime, ifOperStatus, hrSystemDate,
// count, process, stats, traffic, script, and multi-OID get with delta/ps).
type SNMPPoller struct{}

func NewSNMPPoller() *SNMPPoller {
	return &SNMPPoller{}
}

func (p *SNMPPoller) Poll(_ context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	if node == nil || node.IP == "" {
		return &Result{
			State:   StateHigh,
			Message: "missing node IP address",
		}, nil
	}

	timeoutSec := pe.Timeout
	if timeoutSec <= 0 {
		timeoutSec = 2
	}
	agent := BuildSNMPAgent(node, timeoutSec, pe.Retry)

	start := time.Now()
	if err := agent.Connect(); err != nil {
		return &Result{
			State:   StateHigh,
			RTT:     time.Since(start),
			Message: fmt.Sprintf("snmp connect to %s failed: %v", node.IP, err),
			Fields: map[string]interface{}{
				"error": err.Error(),
			},
		}, nil
	}
	defer agent.Conn.Close()

	mode := pe.Mode
	switch mode {
	case "sysUpTime":
		return p.pollSysUpTime(pe, agent, start)
	case "ifOperStatus":
		return p.pollIF(pe, agent, start)
	case "hrSystemDate":
		return p.pollSystemDate(pe, agent, start)
	case "count":
		return p.pollCount(pe, agent, start)
	case "process":
		return p.pollProcess(pe, agent, start)
	case "stats":
		return p.pollStats(pe, agent, start)
	case "traffic":
		return p.pollTraffic(pe, agent, start)
	case "script":
		return p.pollScript(pe, agent, start)
	default:
		return p.pollGet(pe, agent, start)
	}
}

// 1. sysUpTime mode
func (p *SNMPPoller) pollSysUpTime(pe *datastore.PollingEnt, agent *gosnmp.GoSNMP, start time.Time) (*Result, error) {
	oid := mib.NameToOID("sysUpTime.0")
	result, err := agent.Get([]string{oid})
	rtt := time.Since(start)
	if err != nil {
		return &Result{
			State:   failureState(pe.Level),
			RTT:     rtt,
			Message: fmt.Sprintf("snmp get sysUpTime failed: %v", err),
			Fields:  map[string]interface{}{"error": err.Error(), "rtt": float64(rtt.Nanoseconds())},
		}, nil
	}

	var uptime int64
	for _, variable := range result.Variables {
		if variable.Name == oid {
			uptime = int64(gosnmp.ToBigInt(variable.Value).Uint64())
			break
		}
	}
	if uptime == 0 {
		return &Result{
			State:   failureState(pe.Level),
			RTT:     rtt,
			Message: "sysUpTime is 0",
			Fields:  map[string]interface{}{"rtt": float64(rtt.Nanoseconds())},
		}, nil
	}

	fields := map[string]interface{}{
		"sysUpTime": float64(uptime),
		"rtt":       float64(rtt.Nanoseconds()),
	}

	if pe.Result != nil {
		if v, ok := pe.Result["sysUpTime"]; ok {
			if lastUptime, ok := v.(float64); ok {
				diff := float64(uptime) - lastUptime
				fields["deltaSysUpTime"] = diff
				if lastUptime < float64(uptime) {
					return &Result{
						State:   StateNormal,
						RTT:     rtt,
						Message: fmt.Sprintf("sysUpTime ok: %d (delta: %0.f)", uptime, diff),
						Fields:  fields,
					}, nil
				}
				// Reboot detected
				return &Result{
					State:   failureState(pe.Level),
					RTT:     rtt,
					Message: fmt.Sprintf("system reboot detected: last=%0.f curr=%d", lastUptime, uptime),
					Fields:  fields,
				}, nil
			}
		}
	}

	fields["deltaSysUpTime"] = 0.0
	return &Result{
		State:   StateUnknown,
		RTT:     rtt,
		Message: fmt.Sprintf("sysUpTime initial sample: %d", uptime),
		Fields:  fields,
	}, nil
}

// 2. ifOperStatus mode
func (p *SNMPPoller) pollIF(pe *datastore.PollingEnt, agent *gosnmp.GoSNMP, start time.Time) (*Result, error) {
	if pe.Params == "" {
		return &Result{State: StateHigh, Message: "missing ifIndex in Params"}, nil
	}

	operOID := mib.NameToOID("ifOperStatus." + pe.Params)
	adminOID := mib.NameToOID("ifAdminStatus." + pe.Params)
	result, err := agent.Get([]string{operOID, adminOID})
	rtt := time.Since(start)
	if err != nil {
		return &Result{
			State:   failureState(pe.Level),
			RTT:     rtt,
			Message: fmt.Sprintf("snmp if status failed: %v", err),
			Fields:  map[string]interface{}{"error": err.Error(), "rtt": float64(rtt.Nanoseconds())},
		}, nil
	}

	var oper, admin int64
	for _, variable := range result.Variables {
		name := mib.OIDToName(variable.Name)
		if strings.HasPrefix(name, "ifOperStatus") {
			oper = gosnmp.ToBigInt(variable.Value).Int64()
		} else if strings.HasPrefix(name, "ifAdminStatus") {
			admin = gosnmp.ToBigInt(variable.Value).Int64()
		}
	}

	fields := map[string]interface{}{
		"ifOperStatus":  float64(oper),
		"ifAdminStatus": float64(admin),
		"rtt":           float64(rtt.Nanoseconds()),
	}

	if oper == 1 || admin == 2 {
		return &Result{
			State:   StateNormal,
			RTT:     rtt,
			Message: fmt.Sprintf("if #%s ok: oper=%d admin=%d", pe.Params, oper, admin),
			Fields:  fields,
		}, nil
	} else if oper == 2 && admin == 1 {
		return &Result{
			State:   failureState(pe.Level),
			RTT:     rtt,
			Message: fmt.Sprintf("if #%s DOWN (oper=2 admin=1)", pe.Params),
			Fields:  fields,
		}, nil
	}

	return &Result{
		State:   StateUnknown,
		RTT:     rtt,
		Message: fmt.Sprintf("if #%s status: oper=%d admin=%d", pe.Params, oper, admin),
		Fields:  fields,
	}, nil
}

// 3. hrSystemDate mode
func (p *SNMPPoller) pollSystemDate(pe *datastore.PollingEnt, agent *gosnmp.GoSNMP, start time.Time) (*Result, error) {
	oid := mib.NameToOID("hrSystemDate.0")
	result, err := agent.Get([]string{oid})
	rtt := time.Since(start)
	if err != nil {
		return &Result{
			State:   failureState(pe.Level),
			RTT:     rtt,
			Message: fmt.Sprintf("snmp get hrSystemDate failed: %v", err),
			Fields:  map[string]interface{}{"error": err.Error(), "rtt": float64(rtt.Nanoseconds())},
		}, nil
	}

	var diff float64
	var ts string
	for _, variable := range result.Variables {
		if variable.Name == oid {
			if v, ok := variable.Value.([]uint8); ok && len(v) >= 8 {
				direction := byte('+')
				if len(v) >= 9 {
					direction = v[8]
				}
				offsetH, offsetM := 0, 0
				if len(v) >= 11 {
					offsetH = int(v[9])
					offsetM = int(v[10])
				}
				ts = fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02d%c%02d:%02d",
					(int(v[0])*256 + int(v[1])), v[2], v[3], v[4], v[5], v[6], direction, offsetH, offsetM)
				t, err := time.Parse(time.RFC3339, ts)
				if err == nil {
					diffSeconds := t.Unix() - time.Now().Unix()
					if diffSeconds < 0 {
						diffSeconds = -diffSeconds
					}
					diff = float64(diffSeconds)
				}
			}
			break
		}
	}

	fields := map[string]interface{}{
		"hrSystemDate": ts,
		"diff":         diff,
		"rtt":          float64(rtt.Nanoseconds()),
	}

	if pe.Script == "" {
		return &Result{
			State:   StateNormal,
			RTT:     rtt,
			Message: fmt.Sprintf("hrSystemDate: %s (diff: %.0fs)", ts, diff),
			Fields:  fields,
		}, nil
	}

	vm := otto.New()
	SetupOttoVM(pe, vm, fields)
	_ = vm.Set("diff", diff)
	_ = vm.Set("hrSystemDate", ts)
	_ = vm.Set("rtt", float64(rtt.Nanoseconds()))

	val, err := vm.Run(pe.Script)
	if err != nil {
		fields["error"] = err.Error()
		return &Result{
			State:   StateHigh,
			RTT:     rtt,
			Message: fmt.Sprintf("hrSystemDate script error: %v", err),
			Fields:  fields,
		}, nil
	}
	pass, _ := val.ToBoolean()
	if pass {
		return &Result{
			State:   StateNormal,
			RTT:     rtt,
			Message: fmt.Sprintf("hrSystemDate script passed: diff=%.0fs", diff),
			Fields:  fields,
		}, nil
	}
	return &Result{
		State:   failureState(pe.Level),
		RTT:     rtt,
		Message: fmt.Sprintf("hrSystemDate threshold exceeded: diff=%.0fs", diff),
		Fields:  fields,
	}, nil
}

// 4. count mode
func (p *SNMPPoller) pollCount(pe *datastore.PollingEnt, agent *gosnmp.GoSNMP, start time.Time) (*Result, error) {
	oid := mib.NameToOID(pe.Params)
	if oid == "" {
		oid = pe.Params
	}

	var regexFilter *regexp.Regexp
	if pe.Filter != "" {
		regexFilter, _ = regexp.Compile(pe.Filter)
	}

	count := 0
	err := agent.Walk(oid, func(variable gosnmp.SnmpPDU) error {
		name := mib.OIDToName(variable.Name)
		s := mib.GetMIBValueString(name, &variable, false)
		if regexFilter != nil && !regexFilter.MatchString(s) {
			return nil
		}
		count++
		return nil
	})
	rtt := time.Since(start)

	fields := map[string]interface{}{
		"count": float64(count),
		"rtt":   float64(rtt.Nanoseconds()),
	}

	if err != nil {
		fields["error"] = err.Error()
		return &Result{
			State:   failureState(pe.Level),
			RTT:     rtt,
			Message: fmt.Sprintf("snmp walk count error: %v", err),
			Fields:  fields,
		}, nil
	}

	if pe.Script == "" {
		return &Result{
			State:   StateNormal,
			RTT:     rtt,
			Message: fmt.Sprintf("snmp count: %d", count),
			Fields:  fields,
		}, nil
	}

	vm := otto.New()
	SetupOttoVM(pe, vm, fields)
	_ = vm.Set("count", float64(count))
	_ = vm.Set("rtt", float64(rtt.Nanoseconds()))

	val, err := vm.Run(pe.Script)
	if err != nil {
		fields["error"] = err.Error()
		return &Result{
			State:   StateHigh,
			RTT:     rtt,
			Message: fmt.Sprintf("count script error: %v", err),
			Fields:  fields,
		}, nil
	}
	pass, _ := val.ToBoolean()
	if pass {
		return &Result{State: StateNormal, RTT: rtt, Message: fmt.Sprintf("count passed: %d", count), Fields: fields}, nil
	}
	return &Result{State: failureState(pe.Level), RTT: rtt, Message: fmt.Sprintf("count failed: %d", count), Fields: fields}, nil
}

// 5. process mode
func (p *SNMPPoller) pollProcess(pe *datastore.PollingEnt, agent *gosnmp.GoSNMP, start time.Time) (*Result, error) {
	oid := mib.NameToOID("hrSWRunName")
	var regexFilter *regexp.Regexp
	if pe.Filter != "" {
		regexFilter, _ = regexp.Compile(pe.Filter)
	}

	lastPidSum := 0.0
	if pe.Result != nil {
		if v, ok := pe.Result["pidSum"].(float64); ok {
			lastPidSum = v
		}
	}

	pidSum := 0.0
	count := 0
	err := agent.Walk(oid, func(variable gosnmp.SnmpPDU) error {
		if variable.Type != gosnmp.OctetString {
			return nil
		}
		name := mib.OIDToName(variable.Name)
		parts := strings.SplitN(name, ".", 2)
		if len(parts) != 2 || parts[0] != "hrSWRunName" {
			return nil
		}
		pid, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil
		}
		s := string(variable.Value.([]byte))
		if regexFilter != nil && !regexFilter.MatchString(s) {
			return nil
		}
		pidSum += float64(pid)
		count++
		return nil
	})
	rtt := time.Since(start)

	changed := 0
	if lastPidSum != 0 && pidSum != lastPidSum {
		changed = 1
	}

	fields := map[string]interface{}{
		"count":   float64(count),
		"pidSum":  pidSum,
		"changed": float64(changed),
		"rtt":     float64(rtt.Nanoseconds()),
	}

	if err != nil {
		fields["error"] = err.Error()
		return &Result{
			State:   failureState(pe.Level),
			RTT:     rtt,
			Message: fmt.Sprintf("snmp process walk error: %v", err),
			Fields:  fields,
		}, nil
	}

	if pe.Script == "" {
		if changed == 1 {
			return &Result{
				State:   failureState(pe.Level),
				RTT:     rtt,
				Message: fmt.Sprintf("process list changed (count=%d)", count),
				Fields:  fields,
			}, nil
		}
		return &Result{
			State:   StateNormal,
			RTT:     rtt,
			Message: fmt.Sprintf("process count: %d", count),
			Fields:  fields,
		}, nil
	}

	vm := otto.New()
	SetupOttoVM(pe, vm, fields)
	_ = vm.Set("count", float64(count))
	_ = vm.Set("changed", float64(changed))
	_ = vm.Set("pidSum", pidSum)
	_ = vm.Set("rtt", float64(rtt.Nanoseconds()))

	val, err := vm.Run(pe.Script)
	if err != nil {
		fields["error"] = err.Error()
		return &Result{State: StateHigh, RTT: rtt, Message: fmt.Sprintf("process script error: %v", err), Fields: fields}, nil
	}
	pass, _ := val.ToBoolean()
	if pass {
		return &Result{State: StateNormal, RTT: rtt, Message: fmt.Sprintf("process script passed: count=%d", count), Fields: fields}, nil
	}
	return &Result{State: failureState(pe.Level), RTT: rtt, Message: fmt.Sprintf("process script failed: count=%d", count), Fields: fields}, nil
}

// 6. stats mode
func (p *SNMPPoller) pollStats(pe *datastore.PollingEnt, agent *gosnmp.GoSNMP, start time.Time) (*Result, error) {
	oid := mib.NameToOID(pe.Params)
	if oid == "" {
		oid = pe.Params
	}

	count := uint64(0)
	sum := uint64(0)
	err := agent.Walk(oid, func(variable gosnmp.SnmpPDU) error {
		switch variable.Type {
		case gosnmp.Counter32, gosnmp.Counter64, gosnmp.Integer, gosnmp.Uinteger32, gosnmp.Gauge32:
			sum += gosnmp.ToBigInt(variable.Value).Uint64()
			count++
		}
		return nil
	})
	rtt := time.Since(start)

	if err != nil || count == 0 {
		msg := "no data"
		if err != nil {
			msg = err.Error()
		}
		return &Result{
			State:   failureState(pe.Level),
			RTT:     rtt,
			Message: fmt.Sprintf("snmp stats error: %s", msg),
			Fields:  map[string]interface{}{"error": msg, "rtt": float64(rtt.Nanoseconds())},
		}, nil
	}

	avg := float64(sum) / float64(count)
	fields := map[string]interface{}{
		"count": float64(count),
		"sum":   float64(sum),
		"avg":   avg,
		"rtt":   float64(rtt.Nanoseconds()),
	}

	if pe.Script == "" {
		return &Result{
			State:   StateNormal,
			RTT:     rtt,
			Message: fmt.Sprintf("stats ok: count=%d avg=%.2f sum=%d", count, avg, sum),
			Fields:  fields,
		}, nil
	}

	vm := otto.New()
	SetupOttoVM(pe, vm, fields)
	_ = vm.Set("count", float64(count))
	_ = vm.Set("sum", float64(sum))
	_ = vm.Set("avg", avg)
	_ = vm.Set("rtt", float64(rtt.Nanoseconds()))

	val, err := vm.Run(pe.Script)
	if err != nil {
		fields["error"] = err.Error()
		return &Result{State: StateHigh, RTT: rtt, Message: fmt.Sprintf("stats script error: %v", err), Fields: fields}, nil
	}
	pass, _ := val.ToBoolean()
	if pass {
		return &Result{State: StateNormal, RTT: rtt, Message: fmt.Sprintf("stats script passed: avg=%.2f", avg), Fields: fields}, nil
	}
	return &Result{State: failureState(pe.Level), RTT: rtt, Message: fmt.Sprintf("stats script failed: avg=%.2f", avg), Fields: fields}, nil
}

// 7. traffic mode
func (p *SNMPPoller) pollTraffic(pe *datastore.PollingEnt, agent *gosnmp.GoSNMP, start time.Time) (*Result, error) {
	if pe.Params == "" {
		return &Result{State: StateHigh, Message: "missing ifIndex in Params"}, nil
	}

	oids := []string{
		mib.NameToOID("ifInOctets." + pe.Params),
		mib.NameToOID("ifInUcastPkts." + pe.Params),
		mib.NameToOID("ifInNUcastPkts." + pe.Params),
		mib.NameToOID("ifInDiscards." + pe.Params),
		mib.NameToOID("ifInErrors." + pe.Params),
		mib.NameToOID("ifOutOctets." + pe.Params),
		mib.NameToOID("ifOutUcastPkts." + pe.Params),
		mib.NameToOID("ifOutNUcastPkts." + pe.Params),
		mib.NameToOID("sysUpTime.0"),
	}

	result, err := agent.Get(oids)
	rtt := time.Since(start)
	if err != nil {
		return &Result{
			State:   failureState(pe.Level),
			RTT:     rtt,
			Message: fmt.Sprintf("snmp traffic get failed: %v", err),
			Fields:  map[string]interface{}{"error": err.Error(), "rtt": float64(rtt.Nanoseconds())},
		}, nil
	}

	lr := make(map[string]interface{})
	for _, variable := range result.Variables {
		name := mib.OIDToName(variable.Name)
		if name == "sysUpTime.0" {
			lr["sysUpTime"] = float64(gosnmp.ToBigInt(variable.Value).Uint64())
			continue
		}
		lr[name] = float64(gosnmp.ToBigInt(variable.Value).Uint64())
	}

	// Try ifXTable 64-bit HC counters
	hcOids := []string{
		mib.NameToOID("ifHCInOctets." + pe.Params),
		mib.NameToOID("ifHCInUcastPkts." + pe.Params),
		mib.NameToOID("ifHCOutOctets." + pe.Params),
		mib.NameToOID("ifHCOutUcastPkts." + pe.Params),
	}
	if hcResult, err := agent.Get(hcOids); err == nil && len(hcResult.Variables) > 0 {
		for _, variable := range hcResult.Variables {
			name := mib.OIDToName(variable.Name)
			if strings.HasPrefix(name, "ifHCInOctets") {
				lr["ifInOctets."+pe.Params] = float64(gosnmp.ToBigInt(variable.Value).Uint64())
			} else if strings.HasPrefix(name, "ifHCInUcastPkts") {
				lr["ifInUcastPkts."+pe.Params] = float64(gosnmp.ToBigInt(variable.Value).Uint64())
			} else if strings.HasPrefix(name, "ifHCOutOctets") {
				lr["ifOutOctets."+pe.Params] = float64(gosnmp.ToBigInt(variable.Value).Uint64())
			} else if strings.HasPrefix(name, "ifHCOutUcastPkts") {
				lr["ifOutUcastPkts."+pe.Params] = float64(gosnmp.ToBigInt(variable.Value).Uint64())
			}
		}
	}

	lr["rtt"] = float64(rtt.Nanoseconds())
	now := float64(time.Now().UnixNano())

	if pe.Result == nil || pe.Result["lastTime"] == nil {
		lr["lastTime"] = now
		return &Result{
			State:   StateUnknown,
			RTT:     rtt,
			Message: fmt.Sprintf("initial traffic baseline sampled for if #%s", pe.Params),
			Fields:  lr,
		}, nil
	}

	lr["lastTime"] = now
	for k, v := range lr {
		if vf, ok := v.(float64); ok {
			if vo, ok := pe.Result[k].(float64); ok {
				lr[k+"_Delta"] = vf - vo
			}
		}
	}

	diff := 100.0 // default fallback 1s in 10ms ticks
	if v, ok := lr["sysUpTime_Delta"].(float64); ok && v > 0 {
		diff = v
	}

	calcPS := func(key string) float64 {
		if d, ok := lr[key+"_Delta"].(float64); ok && d >= 0 {
			return (d * 100.0) / diff
		}
		return 0
	}

	inOctetsPS := calcPS("ifInOctets." + pe.Params)
	outOctetsPS := calcPS("ifOutOctets." + pe.Params)
	inPktsPS := calcPS("ifInUcastPkts." + pe.Params)
	outPktsPS := calcPS("ifOutUcastPkts." + pe.Params)

	bps := inOctetsPS * 8
	obps := outOctetsPS * 8
	pps := inPktsPS
	opps := outPktsPS

	lr["bps"] = bps
	lr["obps"] = obps
	lr["pps"] = pps
	lr["opps"] = opps
	lr["bytes"] = inOctetsPS
	lr["outBytes"] = outOctetsPS

	if pe.Script == "" {
		return &Result{
			State:   StateNormal,
			RTT:     rtt,
			Message: fmt.Sprintf("traffic if#%s: in=%.1f bps out=%.1f bps", pe.Params, bps, obps),
			Fields:  lr,
		}, nil
	}

	vm := otto.New()
	SetupOttoVM(pe, vm, lr)
	for k, v := range lr {
		_ = vm.Set(k, v)
	}

	val, err := vm.Run(pe.Script)
	if err != nil {
		lr["error"] = err.Error()
		return &Result{State: StateHigh, RTT: rtt, Message: fmt.Sprintf("traffic script error: %v", err), Fields: lr}, nil
	}
	pass, _ := val.ToBoolean()
	if pass {
		return &Result{State: StateNormal, RTT: rtt, Message: fmt.Sprintf("traffic passed: in=%.1f bps", bps), Fields: lr}, nil
	}
	return &Result{State: failureState(pe.Level), RTT: rtt, Message: fmt.Sprintf("traffic threshold exceeded: in=%.1f bps", bps), Fields: lr}, nil
}

// 8. script mode (provides snmpGet)
func (p *SNMPPoller) pollScript(pe *datastore.PollingEnt, agent *gosnmp.GoSNMP, start time.Time) (*Result, error) {
	if pe.Script == "" {
		return &Result{State: StateHigh, Message: "missing Script in script mode"}, nil
	}

	rtt := time.Since(start)
	fields := map[string]interface{}{
		"rtt": float64(rtt.Nanoseconds()),
	}

	vm := otto.New()
	SetupOttoVM(pe, vm, fields)
	_ = vm.Set("rtt", float64(rtt.Nanoseconds()))
	_ = vm.Set("snmpGet", func(call otto.FunctionCall) otto.Value {
		if call.Argument(0).IsString() {
			name := call.Argument(0).String()
			oid := mib.NameToOID(name)
			if oid == "" {
				oid = name
			}
			res, err := agent.Get([]string{oid})
			if err == nil && len(res.Variables) > 0 {
				v := res.Variables[0]
				s := mib.GetMIBValueString(name, &v, false)
				if s == "" {
					s = formatSNMPValue(v)
				}
				if r, err := otto.ToValue(s); err == nil {
					return r
				}
			}
		}
		return otto.UndefinedValue()
	})

	val, err := vm.Run(pe.Script)
	if err != nil {
		fields["error"] = err.Error()
		return &Result{
			State:   StateHigh,
			RTT:     rtt,
			Message: fmt.Sprintf("snmp script error: %v", err),
			Fields:  fields,
		}, nil
	}

	pass, _ := val.ToBoolean()
	if pass {
		return &Result{State: StateNormal, RTT: rtt, Message: "snmp script passed", Fields: fields}, nil
	}
	return &Result{State: failureState(pe.Level), RTT: rtt, Message: "snmp script returned false", Fields: fields}, nil
}

// 9. default (multi-OID get, delta, ps, filter)
func (p *SNMPPoller) pollGet(pe *datastore.PollingEnt, agent *gosnmp.GoSNMP, start time.Time) (*Result, error) {
	param := pe.Params
	if param == "" {
		param = "sysDescr.0"
	}

	names := strings.Split(param, ",")
	oids := make([]string, 0, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		oid := mib.NameToOID(n)
		if oid == "" {
			oid = n
		}
		oids = append(oids, oid)
	}

	if pe.Mode == "ps" {
		oids = append(oids, mib.NameToOID("sysUpTime.0"))
	}

	result, err := agent.Get(oids)
	rtt := time.Since(start)
	if err != nil {
		return &Result{
			State:   StateHigh,
			RTT:     rtt,
			Message: fmt.Sprintf("snmp get failed: %v", err),
			Fields:  map[string]interface{}{"error": err.Error(), "rtt": float64(rtt.Nanoseconds())},
		}, nil
	}

	if len(result.Variables) == 0 {
		return &Result{
			State:   StateWarn,
			RTT:     rtt,
			Message: "no variables returned",
			Fields:  map[string]interface{}{"rtt": float64(rtt.Nanoseconds())},
		}, nil
	}

	lr := make(map[string]interface{})
	for _, variable := range result.Variables {
		name := mib.OIDToName(variable.Name)
		valStr := mib.GetMIBValueString(name, &variable, false)
		if valStr == "" {
			valStr = formatSNMPValue(variable)
		}
		lr[name] = valStr
		// Also provide numeric representation if convertible
		if n, err := strconv.ParseFloat(valStr, 64); err == nil {
			lr[name+"_num"] = n
		}
	}
	lr["rtt"] = float64(rtt.Nanoseconds())

	// Backward compatibility: set "val" if single variable
	if len(result.Variables) == 1 {
		name := mib.OIDToName(result.Variables[0].Name)
		lr["val"] = lr[name]
	}

	// Filter string match check
	if pe.Filter != "" {
		matched := false
		for _, v := range lr {
			if s, ok := v.(string); ok && strings.Contains(s, pe.Filter) {
				matched = true
				break
			}
		}
		if !matched {
			return &Result{
				State:   StateWarn,
				RTT:     rtt,
				Message: fmt.Sprintf("snmp value does not match filter '%s'", pe.Filter),
				Fields:  lr,
			}, nil
		}
	}

	// Script evaluation if provided
	if pe.Script != "" {
		vm := otto.New()
		SetupOttoVM(pe, vm, lr)
		for k, v := range lr {
			_ = vm.Set(k, v)
		}
		val, err := vm.Run(pe.Script)
		if err != nil {
			lr["error"] = err.Error()
			return &Result{State: StateHigh, RTT: rtt, Message: fmt.Sprintf("snmp script error: %v", err), Fields: lr}, nil
		}
		pass, _ := val.ToBoolean()
		if !pass {
			return &Result{State: failureState(pe.Level), RTT: rtt, Message: "snmp script returned false", Fields: lr}, nil
		}
	}

	return &Result{
		State:   StateNormal,
		RTT:     rtt,
		Message: fmt.Sprintf("snmp get ok, rtt=%v", rtt),
		Fields:  lr,
	}, nil
}

func formatSNMPValue(v gosnmp.SnmpPDU) string {
	switch v.Type {
	case gosnmp.OctetString:
		return string(v.Value.([]byte))
	case gosnmp.Integer, gosnmp.Counter32, gosnmp.Gauge32, gosnmp.TimeTicks, gosnmp.Counter64:
		return fmt.Sprintf("%v", v.Value)
	case gosnmp.ObjectIdentifier:
		return fmt.Sprintf("%v", v.Value)
	default:
		return strconv.FormatInt(gosnmp.ToBigInt(v.Value).Int64(), 10)
	}
}

// BuildSNMPAgent creates a configured GoSNMP client for a node.
func BuildSNMPAgent(node *datastore.NodeEnt, timeoutSec int, retries int) *gosnmp.GoSNMP {
	port := uint16(node.SnmpPort)
	if port == 0 {
		port = 161
	}
	if timeoutSec <= 0 {
		timeoutSec = 2
	}

	agent := &gosnmp.GoSNMP{
		Target:    node.IP,
		Port:      port,
		Transport: "udp",
		Community: node.Community,
		Version:   gosnmp.Version2c,
		Timeout:   time.Duration(timeoutSec) * time.Second,
		Retries:   retries,
		MaxOids:   gosnmp.MaxOids,
	}
	if agent.Community == "" {
		agent.Community = "public"
	}

	switch node.SnmpMode {
	case "v1":
		agent.Version = gosnmp.Version1
	case "v3auth":
		agent.Version = gosnmp.Version3
		agent.SecurityModel = gosnmp.UserSecurityModel
		agent.MsgFlags = gosnmp.AuthNoPriv
		agent.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName:                 node.User,
			AuthenticationProtocol:   gosnmp.SHA,
			AuthenticationPassphrase: node.Password,
		}
	case "v3authpriv":
		agent.Version = gosnmp.Version3
		agent.SecurityModel = gosnmp.UserSecurityModel
		agent.MsgFlags = gosnmp.AuthPriv
		agent.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName:                 node.User,
			AuthenticationProtocol:   gosnmp.SHA,
			AuthenticationPassphrase: node.Password,
			PrivacyProtocol:          gosnmp.AES,
			PrivacyPassphrase:        node.Password,
		}
	case "v3authprivex":
		agent.Version = gosnmp.Version3
		agent.SecurityModel = gosnmp.UserSecurityModel
		agent.MsgFlags = gosnmp.AuthPriv
		agent.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName:                 node.User,
			AuthenticationProtocol:   gosnmp.SHA256,
			AuthenticationPassphrase: node.Password,
			PrivacyProtocol:          gosnmp.AES256,
			PrivacyPassphrase:        node.Password,
		}
	case "v3sha256aes128":
		agent.Version = gosnmp.Version3
		agent.SecurityModel = gosnmp.UserSecurityModel
		agent.MsgFlags = gosnmp.AuthPriv
		agent.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName:                 node.User,
			AuthenticationProtocol:   gosnmp.SHA256,
			AuthenticationPassphrase: node.Password,
			PrivacyProtocol:          gosnmp.AES,
			PrivacyPassphrase:        node.Password,
		}
	case "v3sha512aes256":
		agent.Version = gosnmp.Version3
		agent.SecurityModel = gosnmp.UserSecurityModel
		agent.MsgFlags = gosnmp.AuthPriv
		agent.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName:                 node.User,
			AuthenticationProtocol:   gosnmp.SHA512,
			AuthenticationPassphrase: node.Password,
			PrivacyProtocol:          gosnmp.AES256,
			PrivacyPassphrase:        node.Password,
		}
	}
	return agent
}
