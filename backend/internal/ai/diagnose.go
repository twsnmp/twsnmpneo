package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	"github.com/twsnmp/twsnmpneo/backend/internal/diagnose"
)

// DiagnoseAlertParams defines parameters for contextual alert diagnosis.
type DiagnoseAlertParams struct {
	EventLogID  string `json:"event_log_id"`
	AlertEvent  string `json:"alert_event"`
	NodeID      string `json:"node_id"`
	NodeContext string `json:"node_context"`
	Type        string `json:"type"`
	Level       string `json:"level"`
	Time        int64  `json:"time"`
	Lang        string `json:"lang"`
}

// DiagnoseAlertContext provides rich, context-aware event and alert diagnosis.
func (c *LLMClient) DiagnoseAlertContext(ctx context.Context, store datastore.DataStore, logStore *parquet.Store, params DiagnoseAlertParams) (string, error) {
	isJa := strings.HasPrefix(strings.ToLower(params.Lang), "ja")

	var sb strings.Builder
	sb.WriteString("# Target Event Log\n")

	eventTime := params.Time
	if eventTime <= 0 {
		eventTime = time.Now().UnixNano()
	}
	tStr := time.Unix(0, eventTime).Format(time.RFC3339)
	sb.WriteString(fmt.Sprintf("- Time: %s\n", tStr))
	if params.Level != "" {
		sb.WriteString(fmt.Sprintf("- Level: %s\n", params.Level))
	}
	if params.Type != "" {
		sb.WriteString(fmt.Sprintf("- Type: %s\n", params.Type))
	}
	if params.NodeID != "" {
		sb.WriteString(fmt.Sprintf("- Node ID: %s\n", params.NodeID))
	}
	if params.NodeContext != "" {
		sb.WriteString(fmt.Sprintf("- Node Context: %s\n", params.NodeContext))
	}
	sb.WriteString(fmt.Sprintf("- Event Message: %s\n", params.AlertEvent))

	// Find node entity if possible
	var targetNode *datastore.NodeEnt
	if store != nil {
		if params.NodeID != "" {
			if n, err := store.GetNode(ctx, params.NodeID); err == nil && n != nil {
				targetNode = n
			}
		}
		if targetNode == nil {
			store.ForEachNodes(func(n *datastore.NodeEnt) bool {
				if (n.Name != "" && strings.Contains(params.AlertEvent, n.Name)) ||
					(n.IP != "" && strings.Contains(params.AlertEvent, n.IP)) ||
					(params.NodeContext != "" && strings.Contains(params.NodeContext, n.Name)) {
					targetNode = n
					return false
				}
				return true
			})
		}
	}

	var systemPrompt string
	switch strings.ToLower(params.Type) {
	case "user":
		if isJa {
			systemPrompt = "あなたはシステム運用およびセキュリティ管理の専門家です。\n" +
				"ユーザーによって実行された操作・設定変更イベントログを分析し、\n" +
				"1. 実行された操作の目的・内容の要約\n" +
				"2. システムや設定、対象ノードへの影響評価\n" +
				"3. 運用・セキュリティ上の確認事項や注意点\n" +
				"を日本語で分かりやすく解説してください。必ずマークダウン形式（表、太字、箇条書き）で回答してください。"
		} else {
			systemPrompt = "You are an expert in system operations and security management.\n" +
				"Please analyze the user operation / configuration change event log provided and explain:\n" +
				"1. Summary of the user action and its intent\n" +
				"2. Evaluation of impact on the system, settings, or target components\n" +
				"3. Operational or security precautions to verify\n" +
				"Answer in English using clean markdown format (tables, bold text, bullet points)."
		}
		if targetNode != nil {
			sb.WriteString("\n# Associated Node Information\n")
			appendNodeSummary(&sb, targetNode)
		}

	case "system":
		if isJa {
			systemPrompt = "あなたはシステムインフラおよびリソース管理の専門家です。\n" +
				"TWSNMP NEO本体やシステム全般に関するイベントログ（リソース警告やシステム状態等）を分析し、\n" +
				"1. 発生したシステムイベント・警告の要約と原因\n" +
				"2. システム全体への影響とリスク評価\n" +
				"3. 推奨される対応策（リソース解放、設定見直し、メンテナンス等）\n" +
				"を日本語で分かりやすく解説してください。必ずマークダウン形式（表、太字、箇条書き）で回答してください。"
		} else {
			systemPrompt = "You are an expert in system infrastructure and resource management.\n" +
				"Please analyze the system-level event log (resource alerts, system state, etc.) and explain:\n" +
				"1. Summary and cause of the system event / alert\n" +
				"2. Risk assessment and impact on overall system operation\n" +
				"3. Recommended remediation steps (resource cleanup, configuration tuning, etc.)\n" +
				"Answer in English using clean markdown format (tables, bold text, bullet points)."
		}

		if store != nil {
			sb.WriteString("\n# Recent System Event Logs\n")
			sysCount := 0
			store.ForEachLastEventLog(func(el *datastore.EventLogEnt) bool {
				if el.Type == "system" && el.Time != eventTime {
					tS := time.Unix(0, el.Time).Format(time.RFC3339)
					sb.WriteString(fmt.Sprintf("- %s [Level: %s] %s\n", tS, el.Level, el.Event))
					sysCount++
					if sysCount >= 10 {
						return false
					}
				}
				return true
			})
			if sysCount == 0 {
				sb.WriteString("No other recent system event logs found.\n")
			}
		}

	case "cert", "cert_monitor":
		if isJa {
			systemPrompt = "あなたはTLS/SSLセキュリティおよびサーバー証明書管理の専門家です。\n" +
				"提示されたサーバー証明書監視（Cert Monitor）のイベントログおよび登録されているサーバー証明書の情報を総合的に分析し、\n" +
				"1. 発生した証明書イベント・警告の要約と原因（有効期限切れ、接続エラー、検証失敗等）\n" +
				"2. 影響範囲およびセキュリティ上のリスク評価\n" +
				"3. 推奨される証明書更新・設定修正・対策手順\n" +
				"を日本語で分かりやすく解説してください。必ずマークダウン形式（表、太字、箇条書き）で回答してください。"
		} else {
			systemPrompt = "You are an expert in TLS/SSL security and server certificate management.\n" +
				"Please analyze the provided server certificate monitor event log and certificate status details:\n" +
				"1. Summary and cause of the certificate event / alert (expiration, connection failure, validation error, etc.)\n" +
				"2. Security risk assessment and scope of impact\n" +
				"3. Recommended action plan for certificate renewal or configuration fixes\n" +
				"Answer in English using clean markdown format (tables, bold text, bullet points)."
		}

		if store != nil {
			sb.WriteString("\n# Server Certificate Monitor Details\n")
			certs, _ := store.ListCertMonitors(ctx)
			if len(certs) == 0 {
				sb.WriteString("No server certificate monitors configured.\n")
			} else {
				for _, cEnt := range certs {
					matchedTag := ""
					if strings.Contains(params.AlertEvent, cEnt.Target) {
						matchedTag = " [Target Match]"
					}
					tNotAfter := time.Unix(cEnt.NotAfter, 0).Format(time.RFC3339)
					sb.WriteString(fmt.Sprintf("## Certificate Monitor: %s:%d (State: %s)%s\n", cEnt.Target, cEnt.Port, cEnt.State, matchedTag))
					if cEnt.Subject != "" {
						sb.WriteString(fmt.Sprintf("- Subject: %s\n", cEnt.Subject))
					}
					if cEnt.Issuer != "" {
						sb.WriteString(fmt.Sprintf("- Issuer: %s\n", cEnt.Issuer))
					}
					if cEnt.NotAfter > 0 {
						sb.WriteString(fmt.Sprintf("- Valid Until: %s\n", tNotAfter))
					}
					if cEnt.Error != "" {
						sb.WriteString(fmt.Sprintf("- Error: %s\n", cEnt.Error))
					}
				}
			}
		}

	default:
		if isJa {
			systemPrompt = "あなたはネットワーク運用、ログ解析、およびシステムトラブルシューティングの専門家です。\n" +
				"提示された対象イベントログの内容、関連ノード情報、関連ポーリング状態・ログ、およびイベント発生時刻前後の関連ログ（Syslog, SNMP Trap）を総合的に分析し、\n" +
				"1. イベントの概要および想定される発生原因\n" +
				"2. 関連コンポーネント・ポーリング・前後ログの状況分析\n" +
				"3. 推奨される確認事項・対策アクション\n" +
				"を日本語で分かりやすく診断してください。必ずマークダウン形式（表、太字、箇条書き）で回答してください。"
		} else {
			systemPrompt = "You are an expert in network operations, log analysis, and system troubleshooting.\n" +
				"Please analyze the provided event log, related node details, polling statuses, polling logs, and nearby Syslog/SNMP Trap logs.\n" +
				"Investigate and explain:\n" +
				"1. Summary & Estimated Root Cause of the Event\n" +
				"2. Analysis of Related Components, Pollings, and Nearby Logs\n" +
				"3. Recommended Action Plan & Remediation Steps\n" +
				"Answer in English using clean markdown format (tables, bold text, bullet points)."
		}

		if targetNode != nil {
			sb.WriteString("\n# Related Node Information\n")
			appendNodeSummary(&sb, targetNode)

			if store != nil {
				sb.WriteString("\n# Related Polling Information & Recent Logs\n")
				pollings, _ := store.ListPollings(ctx)
				var nodePollings []*datastore.PollingEnt
				for _, p := range pollings {
					if p.NodeID == targetNode.ID {
						nodePollings = append(nodePollings, p)
					}
				}

				if len(nodePollings) == 0 {
					sb.WriteString("No pollings configured for this node.\n")
				} else {
					for _, p := range nodePollings {
						matchedTag := ""
						if strings.EqualFold(p.Type, params.Type) {
							matchedTag = " [Target Type Match]"
						}
						sb.WriteString(fmt.Sprintf("## Polling: %s (Type: %s, Mode: %s, State: %s)%s\n", p.Name, p.Type, p.Mode, p.State, matchedTag))
						sb.WriteString(fmt.Sprintf("- Interval: %d sec, Level: %s\n", p.PollInt, p.Level))
						if p.Params != "" {
							sb.WriteString(fmt.Sprintf("- Params: %s\n", p.Params))
						}
						if p.Filter != "" {
							sb.WriteString(fmt.Sprintf("- Filter: %s\n", p.Filter))
						}

						if logStore != nil {
							logs, err := logStore.GetAllPollingLog(ctx, p.ID)
							if err == nil && len(logs) > 0 {
								sb.WriteString("- Recent Polling Logs:\n")
								count := 0
								for i := len(logs) - 1; i >= 0 && count < 5; i-- {
									l := logs[i]
									ltStr := time.Unix(0, l.Time).Format(time.RFC3339)
									sb.WriteString(fmt.Sprintf("  - %s | State: %s | Result: %s\n", ltStr, l.State, formatPollingResultForLLM(l.Result)))
									count++
								}
							}
						}
					}
				}
			}

			// Query nearby logs (+-30m)
			if logStore != nil && targetNode.IP != "" {
				st := eventTime - 30*int64(time.Minute)
				et := eventTime + 30*int64(time.Minute)

				sb.WriteString("\n## Related Syslogs (near event time)\n")
				syslogs, err := logStore.Query(ctx, parquet.LogFilter{
					Type:      "syslog",
					StartTime: st,
					EndTime:   et,
					Src:       targetNode.IP,
					Limit:     20,
				})
				if err == nil && len(syslogs) > 0 {
					for _, sl := range syslogs {
						tS := time.Unix(0, sl.Time).Format(time.RFC3339)
						sb.WriteString(fmt.Sprintf("- %s | %s\n", tS, sl.Log))
					}
				} else {
					sb.WriteString("No related Syslog entries found near event time.\n")
				}

				sb.WriteString("\n## Related SNMP Traps (near event time)\n")
				traps, err := logStore.Query(ctx, parquet.LogFilter{
					Type:      "trap",
					StartTime: st,
					EndTime:   et,
					Src:       targetNode.IP,
					Limit:     20,
				})
				if err == nil && len(traps) > 0 {
					for _, tr := range traps {
						tS := time.Unix(0, tr.Time).Format(time.RFC3339)
						sb.WriteString(fmt.Sprintf("- %s | %s\n", tS, tr.Log))
					}
				} else {
					sb.WriteString("No related SNMP Traps found near event time.\n")
				}
			}
		} else {
			sb.WriteString("\nNo associated node information found.\n")
		}
	}

	return c.GenerateAnswer(ctx, systemPrompt, sb.String())
}

// DiagnoseNode generates an extensive AI health and risk diagnosis for a node.
func (c *LLMClient) DiagnoseNode(ctx context.Context, store datastore.DataStore, logStore *parquet.Store, nodeID, lang string, probeResult *diagnose.NodeDiagnoseResult) (string, error) {
	if store == nil {
		return "", fmt.Errorf("datastore not available")
	}
	node, err := store.GetNode(ctx, nodeID)
	if err != nil || node == nil {
		return "", fmt.Errorf("node not found: %s", nodeID)
	}

	isJa := strings.HasPrefix(strings.ToLower(lang), "ja")

	var sb strings.Builder
	sb.WriteString("# Node Information\n")
	appendNodeSummary(&sb, node)

	// Active probe diagnostics (Ping, SNMP, Web)
	if probeResult != nil {
		appendProbeSummary(&sb, probeResult)
	}

	sb.WriteString("\n# Polling Status & Current Measurements\n")
	pollings, _ := store.ListPollings(ctx)
	var nodePollings []*datastore.PollingEnt
	for _, p := range pollings {
		if p.NodeID == node.ID {
			nodePollings = append(nodePollings, p)
		}
	}

	if len(nodePollings) == 0 {
		sb.WriteString("No pollings configured for this node.\n")
	} else {
		for _, p := range nodePollings {
			sb.WriteString(fmt.Sprintf("## Polling: %s (Type: %s, State: %s)\n", p.Name, p.Type, p.State))
			sb.WriteString(fmt.Sprintf("- Poll Interval: %d sec, Level: %s\n", p.PollInt, p.Level))
			if p.Params != "" {
				sb.WriteString(fmt.Sprintf("- Params: %s\n", p.Params))
			}
			if p.LastTime > 0 {
				ltStr := time.Unix(0, p.LastTime).Format(time.RFC3339)
				sb.WriteString(fmt.Sprintf("- Latest Result (%s): %s\n", ltStr, formatPollingResultForLLM(p.Result)))
			}
			if p.FailTime > 0 {
				ftStr := time.Unix(0, p.FailTime).Format(time.RFC3339)
				sb.WriteString(fmt.Sprintf("- Failed Since: %s\n", ftStr))
			}
		}
	}

	// Related logs over the last 24 hours (compact & deduplicated)
	now := time.Now().UnixNano()
	st24 := now - 24*int64(time.Hour)

	if logStore != nil && node.IP != "" {
		sb.WriteString("\n# Recent Syslogs (Last 24 Hours, Anomalies & Compact Summary)\n")
		syslogs, err := logStore.Query(ctx, parquet.LogFilter{
			Type:      "syslog",
			StartTime: st24,
			EndTime:   now,
			Src:       node.IP,
			Limit:     25,
		})
		if err == nil {
			appendCompactLogs(&sb, "Syslog", syslogs, 6)
		} else {
			sb.WriteString("No recent Syslog entries found.\n")
		}

		sb.WriteString("\n# Recent SNMP Traps (Last 24 Hours, Anomalies & Compact Summary)\n")
		traps, err := logStore.Query(ctx, parquet.LogFilter{
			Type:      "trap",
			StartTime: st24,
			EndTime:   now,
			Src:       node.IP,
			Limit:     25,
		})
		if err == nil {
			appendCompactLogs(&sb, "SNMP Trap", traps, 6)
		} else {
			sb.WriteString("No recent SNMP Traps found.\n")
		}

		sb.WriteString("\n# Recent ARP Logs (Last 24 Hours)\n")
		arplogs, err := logStore.Query(ctx, parquet.LogFilter{
			Type:      "arplog",
			StartTime: st24,
			EndTime:   now,
			Src:       node.IP,
			Limit:     15,
		})
		if err == nil {
			appendCompactLogs(&sb, "ARP Log", arplogs, 4)
		} else {
			sb.WriteString("No recent ARP logs found.\n")
		}
	}

	var systemPrompt string
	if isJa {
		systemPrompt = "あなたはネットワークおよびシステム管理の専門家です。\n" +
			"提示されたノードの基本情報、リアルタイム診断プローブ結果、ポーリング稼働状態、および関連ログを総合的に分析し、以下の構成で簡潔に診断結果を回答してください。\n\n" +
			"### 1. ノード稼働・健全性状態\n" +
			"（プローブやポーリング状況を踏まえた現在の稼働状態を2〜3行で要約）\n\n" +
			"### 2. 検出された問題点・リスク要因\n" +
			"（Ping/SNMP/Webやログから判明した異常・注意点を箇条書きで簡潔に）\n\n" +
			"### 3. 推奨される対処アクション\n" +
			"（優先度の高い復旧・設定確認手順を具体的な箇条書きで）\n\n" +
			"※回答は無駄な前置きや長大な解説を省き、管理者向けに要点のみを整理したマークダウン形式で出力してください。"
	} else {
		systemPrompt = "You are a network and system administration expert.\n" +
			"Analyze the provided node information, real-time connectivity probes, polling status, and relevant logs.\n" +
			"Provide a concise diagnostic report in Markdown using the following structure:\n\n" +
			"### 1. Node Health & Status Summary\n" +
			"(2-3 lines summarizing current health based on live probes)\n\n" +
			"### 2. Detected Issues & Risk Factors\n" +
			"(Concise bullet points highlighting anomalies, timeouts, or log errors)\n\n" +
			"### 3. Recommended Actions\n" +
			"(Prioritized, concrete troubleshooting or configuration steps)\n\n" +
			"Keep explanations concise, actionable, and avoid unnecessary preamble."
	}

	return c.GenerateAnswer(ctx, systemPrompt, sb.String())
}

func appendProbeSummary(sb *strings.Builder, pr *diagnose.NodeDiagnoseResult) {
	if pr == nil {
		return
	}
	sb.WriteString("\n# Active Realtime Probe Diagnostics\n")
	sb.WriteString(fmt.Sprintf("- Overall Health Status: %s\n", pr.Status))
	if pr.Summary != "" {
		sb.WriteString(fmt.Sprintf("- Probe Summary: %s\n", pr.Summary))
	}
	// Ping
	if pr.Ping.Success {
		sb.WriteString(fmt.Sprintf("- ICMP/Ping: SUCCESS (Sent: %d, Recv: %d, Loss: %.1f%%, Avg RTT: %.2f ms)\n", pr.Ping.Sent, pr.Ping.Received, pr.Ping.Loss, pr.Ping.AvgRTT))
	} else {
		sb.WriteString(fmt.Sprintf("- ICMP/Ping: FAILED (Sent: %d, Recv: %d, Loss: %.1f%%, Error: %s)\n", pr.Ping.Sent, pr.Ping.Received, pr.Ping.Loss, pr.Ping.Error))
	}
	// SNMP
	if !pr.SNMP.Configured {
		sb.WriteString("- SNMP: Not configured on this node\n")
	} else if pr.SNMP.Success {
		sb.WriteString(fmt.Sprintf("- SNMP: SUCCESS (RTT: %.2f ms, UpTime: %s, Descr: %s)\n", pr.SNMP.RTT, pr.SNMP.SysUpTime, pr.SNMP.SysDescr))
	} else {
		sb.WriteString(fmt.Sprintf("- SNMP: FAILED (RTT: %.2f ms, Error: %s)\n", pr.SNMP.RTT, pr.SNMP.Error))
	}
	// Web
	if pr.Web.HTTP != nil {
		if pr.Web.HTTP.Success {
			sb.WriteString(fmt.Sprintf("- Web HTTP (80): SUCCESS (Status: %d, RTT: %.2f ms)\n", pr.Web.HTTP.StatusCode, pr.Web.HTTP.RTT))
		} else {
			sb.WriteString(fmt.Sprintf("- Web HTTP (80): FAILED (Status: %d, Error: %s)\n", pr.Web.HTTP.StatusCode, pr.Web.HTTP.Error))
		}
	}
	if pr.Web.HTTPS != nil {
		if pr.Web.HTTPS.Success {
			sb.WriteString(fmt.Sprintf("- Web HTTPS (443): SUCCESS (Status: %d, RTT: %.2f ms, CertIssuer: %s, RemainingDays: %d days)\n", pr.Web.HTTPS.StatusCode, pr.Web.HTTPS.RTT, pr.Web.HTTPS.CertIssuer, pr.Web.HTTPS.RemainingDays))
		} else {
			sb.WriteString(fmt.Sprintf("- Web HTTPS (443): FAILED (Status: %d, Error: %s)\n", pr.Web.HTTPS.StatusCode, pr.Web.HTTPS.Error))
		}
	}
}

func appendCompactLogs(sb *strings.Builder, category string, records []*parquet.ParquetLogRecord, maxCount int) {
	if len(records) == 0 {
		sb.WriteString(fmt.Sprintf("No recent %s entries found.\n", category))
		return
	}

	type logAgg struct {
		lastTime int64
		sample   string
		count    int
		isError  bool
	}

	msgMap := make(map[string]*logAgg)
	order := make([]string, 0)

	for _, r := range records {
		cleaned := strings.TrimSpace(r.Log)
		if len(cleaned) > 200 {
			cleaned = cleaned[:200] + "..."
		}
		lower := strings.ToLower(cleaned)
		isErr := strings.Contains(lower, "err") || strings.Contains(lower, "fail") ||
			strings.Contains(lower, "warn") || strings.Contains(lower, "crit") ||
			strings.Contains(lower, "down") || strings.Contains(lower, "timeout")

		if agg, exists := msgMap[cleaned]; exists {
			agg.count++
			if r.Time > agg.lastTime {
				agg.lastTime = r.Time
			}
		} else {
			msgMap[cleaned] = &logAgg{
				lastTime: r.Time,
				sample:   cleaned,
				count:    1,
				isError:  isErr,
			}
			order = append(order, cleaned)
		}
	}

	sort.Slice(order, func(i, j int) bool {
		ai := msgMap[order[i]]
		aj := msgMap[order[j]]
		if ai.isError != aj.isError {
			return ai.isError
		}
		return ai.lastTime > aj.lastTime
	})

	outputCount := 0
	for _, key := range order {
		if outputCount >= maxCount {
			remaining := len(records) - outputCount
			if remaining > 0 {
				sb.WriteString(fmt.Sprintf("- ... and %d more %s entries\n", remaining, category))
			}
			break
		}
		agg := msgMap[key]
		tS := time.Unix(0, agg.lastTime).Format(time.RFC3339)
		if agg.count > 1 {
			sb.WriteString(fmt.Sprintf("- %s | %s (repeated %d times)\n", tS, agg.sample, agg.count))
		} else {
			sb.WriteString(fmt.Sprintf("- %s | %s\n", tS, agg.sample))
		}
		outputCount++
	}
}

func appendNodeSummary(sb *strings.Builder, n *datastore.NodeEnt) {
	sb.WriteString(fmt.Sprintf("- Name: %s\n", n.Name))
	sb.WriteString(fmt.Sprintf("- State: %s\n", n.State))
	sb.WriteString(fmt.Sprintf("- IP Address: %s\n", n.IP))
	sb.WriteString(fmt.Sprintf("- MAC Address: %s\n", n.MAC))
	sb.WriteString(fmt.Sprintf("- Vendor: %s\n", n.Vendor))
	if n.Descr != "" {
		sb.WriteString(fmt.Sprintf("- Description: %s\n", n.Descr))
	}
	if n.Loc != "" {
		sb.WriteString(fmt.Sprintf("- Location: %s\n", n.Loc))
	}
	if n.URL != "" {
		sb.WriteString(fmt.Sprintf("- URL: %s\n", n.URL))
	}
	sb.WriteString(fmt.Sprintf("- SNMP Mode: %s\n", n.SnmpMode))
	if n.SnmpPort > 0 {
		sb.WriteString(fmt.Sprintf("- SNMP Port: %d\n", n.SnmpPort))
	}
	if n.AddrMode != "" {
		sb.WriteString(fmt.Sprintf("- Addr Mode: %s\n", n.AddrMode))
	}
}

func formatPollingResultForLLM(res map[string]interface{}) string {
	if len(res) == 0 {
		return "{}"
	}
	formatted := make([]string, 0, len(res))
	for k, v := range res {
		vf, isNum := toFloat64(v)
		if isNum {
			switch k {
			case "rtt", "response_time":
				ms := vf / 1000000.0
				formatted = append(formatted, fmt.Sprintf("%s: %.2f ms (%v ns)", k, ms, v))
			case "speed":
				formatted = append(formatted, fmt.Sprintf("%s: %.2f Mbps", k, vf))
			case "days":
				formatted = append(formatted, fmt.Sprintf("%s: %.0f days", k, vf))
			default:
				formatted = append(formatted, fmt.Sprintf("%s: %v", k, v))
			}
		} else {
			formatted = append(formatted, fmt.Sprintf("%s: %v", k, v))
		}
	}
	return "{" + strings.Join(formatted, ", ") + "}"
}

func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int64:
		return float64(val), true
	case int:
		return float64(val), true
	case json.Number:
		f, err := val.Float64()
		return f, err == nil
	}
	return 0, false
}
