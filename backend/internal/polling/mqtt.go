package polling

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/robertkrimen/otto"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	"github.com/twsnmp/twsnmpneo/backend/internal/extractor"
)

// MQTTPoller tests MQTT broker connectivity, embedded broker topic stats, and payload evaluation.
type MQTTPoller struct {
	store    datastore.DataStore
	logStore *parquet.Store
}

func NewMQTTPoller() *MQTTPoller {
	return &MQTTPoller{}
}

func NewMQTTPollerWithStore(store datastore.DataStore, logStore *parquet.Store) *MQTTPoller {
	return &MQTTPoller{
		store:    store,
		logStore: logStore,
	}
}

func (p *MQTTPoller) Poll(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	mode := strings.ToLower(strings.TrimSpace(pe.Mode))
	switch mode {
	case "", "connect", "subscribe", "stats", "twagent":
	default:
		return &Result{
			State:   StateUnknown,
			Message: fmt.Sprintf("unsupported mqtt mode: %s", pe.Mode),
			Fields:  map[string]interface{}{"error": fmt.Sprintf("unsupported mqtt mode: %s", pe.Mode)},
		}, nil
	}
	start := time.Now()

	topicStr := strings.TrimSpace(pe.MqttTopic)
	if topicStr == "" && pe.Filter != "" {
		topicStr = strings.TrimSpace(pe.Filter)
	}
	if topicStr == "" && pe.Result != nil {
		if t, ok := pe.Result["topic"].(string); ok {
			topicStr = t
		}
	}

	rawURL := strings.TrimSpace(pe.MqttURL)
	if rawURL == "" && (mode == "" || mode == "connect") {
		rawURL = strings.TrimSpace(pe.Params)
	}
	isLocalBroker := rawURL == "" || strings.EqualFold(rawURL, "local") || strings.EqualFold(rawURL, "internal")

	// Try looking up topic in embedded MQTT broker stats first when store is available
	var matchedStat *datastore.MqttStatEnt
	if p.store != nil && (topicStr != "" || isLocalBroker || mode == "subscribe" || mode == "stats" || mode == "twagent") {
		matchedStat = p.findMatchingStat(ctx, topicStr, pe.Params, node)
	}

	var rtt time.Duration
	// Dial external broker if configured and either mode is connect or no embedded stat matched
	if !isLocalBroker && (mode == "" || mode == "connect" || matchedStat == nil) {
		dialURL := rawURL
		if !strings.Contains(dialURL, "://") {
			dialURL = "tcp://" + dialURL
		}
		u, err := url.Parse(dialURL)
		if err != nil {
			return &Result{State: StateUnknown, Message: fmt.Sprintf("invalid mqtt url: %v", err)}, nil
		}

		hostPort := u.Host
		if !strings.Contains(hostPort, ":") {
			if u.Scheme == "ssl" || u.Scheme == "tls" || u.Scheme == "tcps" {
				hostPort += ":8883"
			} else {
				hostPort += ":1883"
			}
		}

		timeout := time.Duration(pe.Timeout) * time.Second
		if timeout <= 0 {
			timeout = 5 * time.Second
		}

		var conn net.Conn
		var dialErr error
		if u.Scheme == "ssl" || u.Scheme == "tls" || u.Scheme == "tcps" {
			dialer := &tls.Dialer{
				NetDialer: &net.Dialer{Timeout: timeout},
				Config:    &tls.Config{InsecureSkipVerify: true},
			}
			conn, dialErr = dialer.DialContext(ctx, "tcp", hostPort)
		} else {
			dialer := &net.Dialer{Timeout: timeout}
			conn, dialErr = dialer.DialContext(ctx, "tcp", hostPort)
		}

		rtt = time.Since(start)
		if dialErr != nil {
			return &Result{
				State:   failureState(pe.Level),
				RTT:     rtt,
				Message: fmt.Sprintf("mqtt connection failed: %v", dialErr),
				Fields:  map[string]interface{}{"rtt": float64(rtt.Nanoseconds()), "error": dialErr.Error()},
			}, nil
		}
		_ = conn.Close()
	} else if mode == "" || mode == "connect" {
		// Default connect mode with node IP when no embedded topic is targeted
		if matchedStat == nil && topicStr == "" && node != nil && node.IP != "" {
			timeout := time.Duration(pe.Timeout) * time.Second
			if timeout <= 0 {
				timeout = 5 * time.Second
			}
			dialer := &net.Dialer{Timeout: timeout}
			conn, dialErr := dialer.DialContext(ctx, "tcp", net.JoinHostPort(node.IP, "1883"))
			rtt = time.Since(start)
			if dialErr != nil {
				return &Result{
					State:   failureState(pe.Level),
					RTT:     rtt,
					Message: fmt.Sprintf("mqtt connection failed: %v", dialErr),
					Fields:  map[string]interface{}{"rtt": float64(rtt.Nanoseconds()), "error": dialErr.Error()},
				}, nil
			}
			_ = conn.Close()
		}
	}

	payloadStr := ""
	var statCount, statBytes int64
	var statLast int64
	if matchedStat != nil {
		topicStr = matchedStat.Topic
		payloadStr = matchedStat.Value
		statCount = int64(matchedStat.Count)
		statBytes = matchedStat.Bytes
		statLast = matchedStat.Last
	} else if pe.Result != nil {
		if pl, ok := pe.Result["payload"].(string); ok {
			payloadStr = pl
		}
	}

	if rtt == 0 {
		rtt = time.Since(start)
	}
	nowNano := time.Now().UnixNano()
	ageSec := 0.0
	if statLast > 0 && nowNano > statLast {
		ageSec = float64(nowNano-statLast) / 1e9
	}

	fields := map[string]interface{}{
		"rtt":      float64(rtt.Nanoseconds()),
		"lastTime": float64(nowNano),
		"topic":    topicStr,
		"payload":  payloadStr,
		"count":    float64(statCount),
		"bytes":    float64(statBytes),
		"ageSec":   ageSec,
	}

	// Parse JSON payload fields (including twagentneo resource, interface, status, and alert schemas)
	extractMQTTJSONFields(payloadStr, pe.Params, fields)

	// If querying alert/fim/sca topics or mode=="twagent", aggregate recent Parquet mqtt logs
	if p.logStore != nil && (mode == "twagent" || strings.HasSuffix(topicStr, "/alert") || strings.HasSuffix(topicStr, "/fim") || strings.HasSuffix(topicStr, "/sca")) {
		p.aggregateMQTTAlertLogs(ctx, pe, node, topicStr, nowNano, fields)
	}

	vm := otto.New()
	SetupOttoVM(pe, vm, fields)
	extractor.RegisterBodyHelpers(payloadStr, vm)
	for k, v := range fields {
		_ = vm.Set(k, v)
	}

	if pe.Extractor != "" && payloadStr != "" {
		_ = extractor.ApplyExtractor(pe.Extractor, payloadStr, vm, fields)
		for k, v := range fields {
			_ = vm.Set(k, v)
		}
	}

	// Filter only desired columns if MqttCols is specified
	if pe.MqttCols != "" {
		for _, col := range strings.Split(pe.MqttCols, ",") {
			col = strings.TrimSpace(col)
			if col != "" {
				if val, err := vm.Get(col); err == nil && val.IsDefined() {
					if f, err := val.ToFloat(); err == nil {
						fields[col] = f
					} else if s, err := val.ToString(); err == nil {
						fields[col] = s
					}
				}
			}
		}
	}

	if pe.Script == "" {
		// Default evaluation when no script is provided
		if (mode == "subscribe" || mode == "stats" || mode == "twagent") && matchedStat == nil && payloadStr == "" {
			return &Result{
				State:   StateUnknown,
				RTT:     rtt,
				Message: "mqtt topic not received yet",
				Fields:  fields,
			}, nil
		}
		if st, ok := fields["status"].(string); ok && strings.EqualFold(st, "offline") {
			return &Result{
				State:   failureState(pe.Level),
				RTT:     rtt,
				Message: fmt.Sprintf("mqtt agent status: %s", st),
				Fields:  fields,
			}, nil
		}
		if tc, ok := fields["threatCount"].(float64); ok && tc > 0 && pe.Level != "off" {
			return &Result{
				State:   failureState(pe.Level),
				RTT:     rtt,
				Message: fmt.Sprintf("mqtt ai threat detected: %.0f", tc),
				Fields:  fields,
			}, nil
		}
		return &Result{
			State:   StateNormal,
			RTT:     rtt,
			Message: fmt.Sprintf("mqtt ok: topic=%s", topicStr),
			Fields:  fields,
		}, nil
	}

	val, err := vm.Run(pe.Script)
	if err != nil {
		fields["error"] = err.Error()
		return &Result{State: StateUnknown, Message: fmt.Sprintf("mqtt script error: %v", err), Fields: fields}, nil
	}
	pass, _ := val.ToBoolean()
	if pass {
		return &Result{State: StateNormal, RTT: rtt, Message: "mqtt script passed", Fields: fields}, nil
	}
	return &Result{State: failureState(pe.Level), RTT: rtt, Message: "mqtt threshold breached", Fields: fields}, nil
}

// findMatchingStat locates the most recent MqttStatEnt matching topic filter and node.
func (p *MQTTPoller) findMatchingStat(ctx context.Context, topicFilter, params string, node *datastore.NodeEnt) *datastore.MqttStatEnt {
	if p.store == nil {
		return nil
	}
	stats, err := p.store.ListMqttStats(ctx)
	if err != nil || len(stats) == 0 {
		return nil
	}

	var bestNodeMatch *datastore.MqttStatEnt
	var bestAnyMatch *datastore.MqttStatEnt

	for _, st := range stats {
		if topicFilter != "" && !MatchMQTTTopic(topicFilter, st.Topic) {
			continue
		}
		nodeMatched := false
		if node != nil {
			if node.IP != "" && st.Remote == node.IP {
				nodeMatched = true
			} else if node.Name != "" && (strings.EqualFold(st.ClientID, node.Name) || strings.Contains(strings.ToLower(st.Topic), strings.ToLower(node.Name))) {
				nodeMatched = true
			}
		}
		if params != "" && !strings.Contains(params, "://") {
			if strings.EqualFold(st.ClientID, params) || strings.Contains(strings.ToLower(st.Topic), strings.ToLower(params)) {
				nodeMatched = true
			}
		}

		if nodeMatched {
			if bestNodeMatch == nil || st.Last > bestNodeMatch.Last {
				bestNodeMatch = st
			}
		}
		if bestAnyMatch == nil || st.Last > bestAnyMatch.Last {
			bestAnyMatch = st
		}
	}

	if bestNodeMatch != nil {
		return bestNodeMatch
	}
	return bestAnyMatch
}

// MatchMQTTTopic checks if topic matches pattern (supports MQTT + and # wildcards, or substring match).
func MatchMQTTTopic(pattern, topic string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" || pattern == "#" {
		return true
	}
	if pattern == topic {
		return true
	}
	if !strings.ContainsAny(pattern, "+#/") {
		return strings.Contains(strings.ToLower(topic), strings.ToLower(pattern))
	}
	patParts := strings.Split(pattern, "/")
	topParts := strings.Split(topic, "/")
	for i := 0; i < len(patParts); i++ {
		if patParts[i] == "#" {
			return true
		}
		if i >= len(topParts) {
			return false
		}
		if patParts[i] != "+" && patParts[i] != topParts[i] {
			return false
		}
	}
	return len(patParts) == len(topParts)
}

// extractMQTTJSONFields parses twagentneo and generic JSON payloads into polling fields.
func extractMQTTJSONFields(payloadStr, paramFilter string, fields map[string]interface{}) {
	trimmed := strings.TrimSpace(payloadStr)
	if trimmed == "" {
		return
	}

	// Case 1: JSON Object (metrics, status, alert, fim)
	if strings.HasPrefix(trimmed, "{") {
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(trimmed), &obj); err != nil {
			return
		}
		for k, v := range obj {
			switch tv := v.(type) {
			case float64, bool, string:
				fields[k] = tv
			}
		}
		if cpu, ok := obj["cpu_percent"].(float64); ok {
			fields["cpu"] = cpu
		}
		if mem, ok := obj["mem_percent"].(float64); ok {
			fields["mem"] = mem
		}
		if load1, ok := obj["load_avg_1"].(float64); ok {
			fields["load"] = load1
		}
		if rx, ok := obj["rx_mbps"].(float64); ok {
			fields["bps"] = rx * 1e6
		}
		if tx, ok := obj["tx_mbps"].(float64); ok {
			fields["obps"] = tx * 1e6
		}

		// Disks array in twagentneo metrics
		if disksRaw, ok := obj["disks"].([]interface{}); ok {
			var maxDiskPct, totalReadBps, totalWriteBps float64
			for _, dItem := range disksRaw {
				if dm, ok := dItem.(map[string]interface{}); ok {
					if pct, ok := dm["used_percent"].(float64); ok && pct > maxDiskPct {
						maxDiskPct = pct
					}
					if r, ok := dm["read_bps"].(float64); ok {
						totalReadBps += r
					}
					if w, ok := dm["write_bps"].(float64); ok {
						totalWriteBps += w
					}
				}
			}
			fields["disk_used_percent"] = maxDiskPct
			fields["disk"] = maxDiskPct
			fields["read_bps"] = totalReadBps
			fields["write_bps"] = totalWriteBps
		}

		// Interfaces array in twagentneo metrics
		if ifacesRaw, ok := obj["interfaces"].([]interface{}); ok {
			var rxMbps, txMbps, bytesRecv, bytesSent, errIn, errOut, dropIn, dropOut float64
			for _, ifItem := range ifacesRaw {
				if im, ok := ifItem.(map[string]interface{}); ok {
					name, _ := im["name"].(string)
					if isLoopbackIfName(name) {
						continue
					}
					if v, ok := im["rx_mbps"].(float64); ok {
						rxMbps += v
					}
					if v, ok := im["tx_mbps"].(float64); ok {
						txMbps += v
					}
					if v, ok := im["bytes_recv"].(float64); ok {
						bytesRecv += v
					}
					if v, ok := im["bytes_sent"].(float64); ok {
						bytesSent += v
					}
					if v, ok := im["err_in"].(float64); ok {
						errIn += v
					}
					if v, ok := im["err_out"].(float64); ok {
						errOut += v
					}
					if v, ok := im["drop_in"].(float64); ok {
						dropIn += v
					}
					if v, ok := im["drop_out"].(float64); ok {
						dropOut += v
					}
				}
			}
			fields["rx_mbps"] = rxMbps
			fields["tx_mbps"] = txMbps
			fields["bps"] = rxMbps * 1e6
			fields["obps"] = txMbps * 1e6
			fields["bytes"] = bytesRecv
			fields["outBytes"] = bytesSent
			fields["errors"] = errIn + errOut
			fields["drops"] = dropIn + dropOut
		}

		// AI Judge nested object in twagentneo alert
		if judgeRaw, ok := obj["ai_judge"].(map[string]interface{}); ok {
			isThreat, _ := judgeRaw["is_threat"].(bool)
			compViolated, _ := judgeRaw["compliance_violated"].(bool)
			conf, _ := judgeRaw["confidence"].(float64)
			sumJa, _ := judgeRaw["summary_ja"].(string)
			actJa, _ := judgeRaw["action_ja"].(string)
			fields["is_threat"] = isThreat
			fields["compliance_violated"] = compViolated
			fields["confidence"] = conf
			fields["summary_ja"] = sumJa
			fields["action_ja"] = actJa
			if isThreat {
				fields["threat"] = float64(1)
			} else {
				fields["threat"] = float64(0)
			}
		}

		// SCA fields if present
		if rID, ok := obj["rule_id"].(string); ok && rID != "" {
			fields["rule_id"] = rID
		}
		if fp, ok := obj["file_path"].(string); ok && fp != "" {
			fields["file_path"] = fp
		}
		if pr, ok := obj["proof"].(string); ok && pr != "" {
			fields["proof"] = pr
		}
		if rem, ok := obj["remediation"].(string); ok && rem != "" {
			fields["remediation"] = rem
		}
		if sev, ok := obj["severity"].(string); ok && sev != "" {
			fields["severity"] = sev
		}
		return
	}

	// Case 2: JSON Array (twagentneo interface / network / tcp / arp topic)
	if strings.HasPrefix(trimmed, "[") {
		var arr []map[string]interface{}
		if err := json.Unmarshal([]byte(trimmed), &arr); err != nil {
			return
		}
		fields["itemCount"] = float64(len(arr))

		var rxMbps, txMbps, rxPps, txPps, rxErrPps, txErrPps, rxDropPps, txDropPps float64
		hasIfMetrics := false
		ifFilter := strings.TrimSpace(paramFilter)
		if strings.Contains(ifFilter, "://") {
			ifFilter = ""
		}

		for _, item := range arr {
			name, _ := item["name"].(string)
			if ifFilter != "" && !strings.EqualFold(name, ifFilter) {
				continue
			}
			if ifFilter == "" && isLoopbackIfName(name) {
				continue
			}
			if _, ok := item["rx_mbps"]; ok {
				hasIfMetrics = true
			}
			if v, ok := item["rx_mbps"].(float64); ok {
				rxMbps += v
			}
			if v, ok := item["tx_mbps"].(float64); ok {
				txMbps += v
			}
			if v, ok := item["rx_pps"].(float64); ok {
				rxPps += v
			}
			if v, ok := item["tx_pps"].(float64); ok {
				txPps += v
			}
			if v, ok := item["rx_error_pps"].(float64); ok {
				rxErrPps += v
			}
			if v, ok := item["tx_error_pps"].(float64); ok {
				txErrPps += v
			}
			if v, ok := item["rx_drop_pps"].(float64); ok {
				rxDropPps += v
			}
			if v, ok := item["tx_drop_pps"].(float64); ok {
				txDropPps += v
			}
		}

		if hasIfMetrics {
			fields["rx_mbps"] = rxMbps
			fields["tx_mbps"] = txMbps
			fields["rx_pps"] = rxPps
			fields["tx_pps"] = txPps
			fields["rx_error_pps"] = rxErrPps
			fields["tx_error_pps"] = txErrPps
			fields["rx_drop_pps"] = rxDropPps
			fields["tx_drop_pps"] = txDropPps
			fields["bps"] = rxMbps * 1e6
			fields["obps"] = txMbps * 1e6
			fields["pps"] = rxPps
			fields["opps"] = txPps
			fields["errors"] = rxErrPps + txErrPps
		}
	}
}

func (p *MQTTPoller) aggregateMQTTAlertLogs(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt, topicFilter string, et int64, fields map[string]interface{}) {
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
	src := ""
	if node != nil && node.IP != "" {
		src = node.IP
	}
	logs, err := p.logStore.Query(ctx, parquet.LogFilter{
		Type:      "mqtt",
		StartTime: st,
		EndTime:   et,
		Src:       src,
		Limit:     5000,
	})
	if err != nil {
		return
	}

	var alertCount, threatCount, compCount, fimCount, scaCount float64
	for _, rec := range logs {
		var mEnt datastore.MqttLogEnt
		if err := json.Unmarshal([]byte(rec.Log), &mEnt); err != nil {
			continue
		}
		if topicFilter != "" && !MatchMQTTTopic(topicFilter, mEnt.Topic) {
			continue
		}
		var payloadObj map[string]interface{}
		if err := json.Unmarshal([]byte(mEnt.Payload), &payloadObj); err != nil {
			continue
		}
		alertCount++
		if _, hasOp := payloadObj["op"]; hasOp {
			fimCount++
		}
		if _, hasFilePath := payloadObj["file_path"]; hasFilePath || strings.HasSuffix(mEnt.Topic, "/sca") {
			scaCount++
		}
		if judgeRaw, ok := payloadObj["ai_judge"].(map[string]interface{}); ok {
			if t, _ := judgeRaw["is_threat"].(bool); t {
				threatCount++
			}
			if c, _ := judgeRaw["compliance_violated"].(bool); c {
				compCount++
			}
		} else if lvl, ok := payloadObj["level"].(string); ok {
			l := strings.ToLower(lvl)
			if l == "critical" || l == "high" {
				threatCount++
			}
		}
	}

	fields["alertCount"] = alertCount
	fields["threatCount"] = threatCount
	fields["compCount"] = compCount
	fields["fimCount"] = fimCount
	fields["scaCount"] = scaCount
}

func isLoopbackIfName(name string) bool {
	l := strings.ToLower(strings.TrimSpace(name))
	return l == "lo" || strings.HasPrefix(l, "lo0") || strings.Contains(l, "loopback")
}
