package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
)

// PingReportItem represents a single ping probe observation.
type PingReportItem struct {
	Time      int64   `json:"time"` // in ns or ms
	Stat      int     `json:"stat"` // 1: OK, 2: Timeout, 3: Warn, 4: GW
	Size      int     `json:"size"`
	SendTTL   int     `json:"send_ttl"`
	RecvTTL   int     `json:"recv_ttl"`
	RecvSrc   string  `json:"recv_src"`
	Loc       string  `json:"loc"`
	TimeStamp int64   `json:"timestamp"`
	RTT       float64 `json:"rtt"` // optional ms
}

// ExplainPing analyzes Ping, SmokePing, TraceRoute, and MTR results.
func (c *LLMClient) ExplainPing(ctx context.Context, targetIP string, results []PingReportItem, mode, tab, lang string) (string, error) {
	if len(results) == 0 {
		return "", fmt.Errorf("no ping test results available")
	}

	isJa := strings.HasPrefix(strings.ToLower(lang), "ja")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Ping Test Report: Target=%s (Mode: %s, Tab: %s)\n\n", targetIP, mode, tab))
	sb.WriteString(fmt.Sprintf("Total Ping Attempts: %d\n", len(results)))

	var successCount, timeoutCount, warnCount, gwCount int
	var minRTT, maxRTT, totalRTT int64
	minRTT = -1

	type sizeStat struct {
		count   int
		totalMs float64
	}
	sizeMap := make(map[int]*sizeStat)
	ttlMap := make(map[int]int)

	for _, r := range results {
		switch r.Stat {
		case 1:
			successCount++
		case 2:
			timeoutCount++
		case 3:
			warnCount++
		case 4:
			gwCount++
		default:
			if r.Time > 0 {
				successCount++
			} else {
				timeoutCount++
			}
		}

		timeNs := r.Time
		if timeNs > 0 && timeNs < 100000 {
			// In case frontend passed milliseconds instead of nanoseconds
			timeNs = int64(float64(timeNs) * 1000000.0)
		}

		if r.Stat == 1 || r.Stat == 4 || timeNs > 0 {
			rttMs := float64(timeNs) / 1000000.0
			if minRTT == -1 || timeNs < minRTT {
				minRTT = timeNs
			}
			if timeNs > maxRTT {
				maxRTT = timeNs
			}
			totalRTT += timeNs

			sz := r.Size
			if sz <= 0 {
				sz = 64
			}
			st, ok := sizeMap[sz]
			if !ok {
				st = &sizeStat{}
				sizeMap[sz] = st
			}
			st.count++
			st.totalMs += rttMs
		}

		if r.RecvTTL > 0 {
			ttlMap[r.RecvTTL]++
		}
	}

	lossRate := float64(timeoutCount) / float64(len(results)) * 100
	sb.WriteString(fmt.Sprintf("Status Breakdown: Success=%d, Timeout=%d, Warn=%d, GW=%d (Packet Loss: %.1f%%)\n",
		successCount, timeoutCount, warnCount, gwCount, lossRate))

	validCount := successCount + gwCount
	if validCount > 0 {
		avgRTTMs := (float64(totalRTT) / float64(validCount)) / 1000000.0
		minRTTMs := float64(minRTT) / 1000000.0
		maxRTTMs := float64(maxRTT) / 1000000.0
		sb.WriteString(fmt.Sprintf("RTT Latency: Min=%.3f ms, Avg=%.3f ms, Max=%.3f ms\n", minRTTMs, avgRTTMs, maxRTTMs))
	}

	if len(sizeMap) > 1 {
		sb.WriteString("\n## Packet Size vs Avg RTT:\n")
		for sz, data := range sizeMap {
			if data.count > 0 {
				avg := data.totalMs / float64(data.count)
				sb.WriteString(fmt.Sprintf("- Size %d bytes: %d samples, Avg RTT = %.3f ms\n", sz, data.count, avg))
			}
		}
	}

	system := "You are a network diagnostic and Ping/Smokeping performance analysis expert. Analyze the provided Ping report data and explain network responsiveness, packet loss rate, jitter/latency trends, potential network issues, and recommended actions in English using clean markdown format (tables, bold text, bullet points)."
	if isJa {
		system = "あなたはネットワーク診断およびPing/Smokeping品質解析の専門家です。提示されたPingテストデータを分析し、ネットワーク応答性、パケットロス率、ジッター・レイテンシの傾向、経路やパケットサイズによる影響、および考えられる問題点と推奨対策について、日本語のマークダウン形式（表、太字、箇条書き）で分かりやすく解説してください。"
	}

	return c.GenerateAnswer(ctx, system, sb.String())
}

// ExplainReport provides comprehensive domain analysis across system reports.
func (c *LLMClient) ExplainReport(ctx context.Context, store datastore.DataStore, logStore *parquet.Store, reportType string, data map[string]interface{}, tab, lang string) (string, error) {
	isJa := strings.HasPrefix(strings.ToLower(lang), "ja")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s Report (Tab: %s)\n\n", strings.ToUpper(reportType), tab))

	var system string

	switch strings.ToLower(reportType) {
	case "address", "ipam":
		system = "You are an IPAM and network architecture expert. Analyze the address management data and explain IP usage efficiency, duplicate IPs, APIPA (169.254.x.x) risks, MAC address changes, and subnet recommendations."
		if isJa {
			system = "あなたはIPAMおよびネットワークの専門家です。提示されたIPアドレス/ARP管理データを分析し、IPアドレス使用効率、重複IP、APIPA (169.254.x.x) のリスク、MACアドレス変更の検知状況、および推奨対策について、日本語のマークダウン形式（表、太字、箇条書き）で分かりやすく解説してください。"
		}
		if data != nil {
			b, _ := json.MarshalIndent(data, "", "  ")
			sb.WriteString(string(b))
		}

	case "cert", "cert_monitor":
		system = "You are an expert in TLS/SSL security and server certificate management. Analyze the certificate report, highlight expired or expiring certificates (within 30 days), verification errors, and provide clear step-by-step renewal and server binding guidance."
		if isJa {
			system = "あなたはTLS/SSLセキュリティおよびサーバー証明書管理の専門家です。提示されたサーバー証明書監視レポートを分析し、期限切れや30日以内に期限を迎える証明書、検証エラーの状況を整理し、証明書更新・ACME自動化およびWebサーバーバインド手順を日本語のマークダウン形式（表、太字、箇条書き）で解説してください。"
		}
		if data != nil {
			b, _ := json.MarshalIndent(data, "", "  ")
			sb.WriteString(string(b))
		}

	case "syslog":
		system = "You are a Syslog, cyber threat detection, and server operations expert. Analyze the Syslog data and explain message severity trends, top logging hosts, threat/anomaly detection results (including Sigma rules), and recommended actions."
		if isJa {
			system = "あなたはSyslog、サイバー脅威検知、およびサーバー運用の専門家です。提示されたSyslogデータを分析し、重大度別の発生傾向、主要送信ホスト、Sigmaルールによる脅威検知結果やコンプライアンス監査違反、および運用上の推奨対策について、日本語のマークダウン形式（表、太字、箇条書き）で解説してください。"
		}
		if data != nil {
			b, _ := json.MarshalIndent(data, "", "  ")
			sb.WriteString(string(b))
		}

	case "netflow", "sflow":
		system = "You are a network traffic flow analyst. Analyze the flow data and explain traffic volume trends, top communication pairs, bandwidth consumption, and anomalous traffic patterns."
		if isJa {
			system = "あなたはネットワークトラフィックフローのアナリストです。提示されたフローデータを分析し、トラフィック量、主要通信ペア、帯域占有傾向、および異常な通信パターンについて、日本語のマークダウン形式（表、太字、箇条書き）で解説してください。"
		}
		if data != nil {
			b, _ := json.MarshalIndent(data, "", "  ")
			sb.WriteString(string(b))
		}

	case "mqtt":
		system = "You are an MQTT broker and IoT operations expert. Analyze the MQTT statistics data and explain client message volumes, topic hierarchy distribution, connection issues, and anomalies."
		if isJa {
			system = "あなたはMQTTブローカーおよびIoT運用の専門家です。提示されたMQTT統計データを分析し、クライアント毎のメッセージ量、トピック階層の集中度、接続状態・切断エラー、および不審なクライアント動作について、日本語のマークダウン形式（表、太字、箇条書き）で解説してください。"
		}
		if data != nil {
			b, _ := json.MarshalIndent(data, "", "  ")
			sb.WriteString(string(b))
		}

	default:
		system = "You are a network reliability and monitoring analytics expert. Analyze the provided operational data and summarize overall health, bottlenecks, and recommended improvements."
		if isJa {
			system = "あなたはネットワーク信頼性および運用監視分析の専門家です。提示された運用集計データを分析し、全体的な健全性評価、ボトルネック、および推奨される改善アクションを日本語のマークダウン形式で解説してください。"
		}
		if data != nil {
			b, _ := json.MarshalIndent(data, "", "  ")
			sb.WriteString(string(b))
		}
	}

	return c.GenerateAnswer(ctx, system, sb.String())
}
