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
	case "device", "devices", "lan":
		system = "You are an enterprise network asset management, Zero Trust, and network access control (NAC) expert. Analyze the provided LAN device inventory (managed nodes vs unmanaged/rogue ARP devices, duplicate IPs/MACs, 169.254.x.x APIPA addresses, MAC address churning/spoofing, and vendor distribution). Highlight critical security and operational risks, rogue shadow IT devices, and provide step-by-step remediation in clean markdown format (tables, bold text, bullet points)."
		if isJa {
			system = "あなたは社内ネットワーク資産管理、ゼロトラスト、および端末セキュリティ（NAC/IT資産管理）の専門家です。提示されたLANデバイス（ARP/ノード台帳）データを分析し、未管理端末（野良端末/シャドーIT）、IP/MACアドレスの重複・競合、DHCP構成障害（169.254.x.x APIPA）、急速なMACアドレス変化（MACスプーフィング疑惑）、仮想マシンおよびベンダー分布の傾向についてセキュリティと運用観点から整理し、推奨される隔離・台帳登録・DHCP設定等の対策手順を日本語のマークダウン形式（表、太字、箇条書き）で分かりやすく解説してください。"
		}
		if data != nil {
			b, _ := json.MarshalIndent(data, "", "  ")
			sb.WriteString(string(b))
		}

	case "arp":
		system = "You are an ARP security and Layer-2 network operations expert. Analyze the ARP log data and detect potential ARP spoofing/poisoning, gateway conflicts, IP flapping, unauthorized MAC migrations, and IP turnover anomalies."
		if isJa {
			system = "あなたはARPセキュリティおよびレイヤー2ネットワーク運用の専門家です。提示されたARP監視ログデータを分析し、ARPスプーフィング/キャッシュポイズニングの疑惑、デフォルトゲートウェイのIP競合、同一IPに対する頻繁なMAC変更（IPフラッピング）、不正な端末すり替え、および推奨されるポートセキュリティ・ダイナミックARPインスペクション(DAI)等の対策を日本語のマークダウン形式で解説してください。"
		}
		if data != nil {
			b, _ := json.MarshalIndent(data, "", "  ")
			sb.WriteString(string(b))
		}

	case "trap":
		system = "You are an SNMP Trap and network infrastructure incident response expert. Analyze the received SNMP trap events, identify link flapping/storms, authentication failures (potential brute-force), spontaneous equipment reboots (coldStart/warmStart), and critical vendor-specific hardware alarms."
		if isJa {
			system = "あなたはSNMP TRAPおよびネットワークインフラ障害対応の専門家です。提示されたSNMP TRAP集計データを分析し、LinkDown/LinkUpストーム（ポートフラッピング）、認証失敗(authFailure)の急増による攻撃疑惑、ハードウェアの予期せぬ再起動(coldStart/warmStart)、および機器ベンダー固有の重大アラームを整理し、復旧手順と根本原因調査の指針を日本語のマークダウン形式で解説してください。"
		}
		if data != nil {
			b, _ := json.MarshalIndent(data, "", "  ")
			sb.WriteString(string(b))
		}

	case "polling":
		system = "You are a network service reliability and SLA monitoring expert. Analyze the polling performance data, identify degraded services, response time spikes, chronic timeout patterns, and recommend polling interval/threshold tuning."
		if isJa {
			system = "あなたはネットワークサービス稼働率およびSLA監視の専門家です。提示されたポーリング稼働率・応答時間データを分析し、稼働率低下ノード、応答遅延スパイクの発生箇所、頻繁にタイムアウトする監視項目、およびポーリング間隔・タイムアウト閾値の最適化提案を日本語のマークダウン形式で解説してください。"
		}
		if data != nil {
			b, _ := json.MarshalIndent(data, "", "  ")
			sb.WriteString(string(b))
		}

	case "event":
		system = "You are an IT systems incident manager and root cause analysis (RCA) expert. Analyze the system event log history, correlate cascading node failures, identify incident triggers, and summarize operational impacts."
		if isJa {
			system = "あなたはITシステムインシデント管理および根本原因分析(RCA)の専門家です。提示されたイベントログデータを分析し、重要障害イベントの時系列パターン、連鎖的ノードダウン（カスケード障害）の波及経路、障害の引き金となった初発事象、および再発防止策を日本語のマークダウン形式で解説してください。"
		}
		if data != nil {
			b, _ := json.MarshalIndent(data, "", "  ")
			sb.WriteString(string(b))
		}

	case "winlog":
		system = "You are a Windows security audit and Active Directory operations expert. Analyze the Windows event logs, identify brute-force login attempts (Event 4625), unauthorized privilege changes, service terminations, and critical system anomalies."
		if isJa {
			system = "あなたはWindowsセキュリティ監査およびActive Directory運用の専門家です。提示されたWindowsイベントログを分析し、ログオン失敗多発(Event ID 4625)による不正侵入・パスワード総当たり攻撃の兆候、ユーザー権限変更、重要サービスの異常停止、および推奨されるアカウントロック・監査ポリシー改善策を日本語のマークダウン形式で解説してください。"
		}
		if data != nil {
			b, _ := json.MarshalIndent(data, "", "  ")
			sb.WriteString(string(b))
		}

	case "sensor", "env":
		system = "You are an IoT sensor and datacenter environment monitoring expert. Analyze temperature, humidity, power consumption, and motion sensor telemetry. Identify overheating hotspots, power surge risks, and equipment operating threshold breaches."
		if isJa {
			system = "あなたはIoTセンサーおよびデータセンター環境監視の専門家です。提示された温湿度・電力・人感センサーの時系列データを分析し、局所的な温度上昇ホットスポット、空調効率、電力消費スパイクや過負荷リスク、センサー電池残量の低下を整理し、環境改善アクションを日本語のマークダウン形式で解説してください。"
		}
		if data != nil {
			b, _ := json.MarshalIndent(data, "", "  ")
			sb.WriteString(string(b))
		}

	case "wifi", "bluetooth":
		system = "You are a wireless security and RF optimization analyst. Analyze Wi-Fi APs and Bluetooth beacon data, detect rogue access points, insecure open/WEP encryption, channel congestion, and unapproved wireless transmitters."
		if isJa {
			system = "あなたは無線LANセキュリティおよび電波最適化のアナリストです。提示されたWi-FiアクセスポイントおよびBluetooth端末データを分析し、社内ポリシー違反の不正AP(Rogue AP)や野良テザリング、暗号化なし(Open)や脆弱な設定、電波干渉・チャンネル重複、および電波強度(RSSI)の異常を日本語のマークダウン形式で解説してください。"
		}
		if data != nil {
			b, _ := json.MarshalIndent(data, "", "  ")
			sb.WriteString(string(b))
		}

	case "pcap":
		system = "You are a deep packet inspection and network protocol analyst. Analyze packet capture metrics, broadcast/multicast storm ratios, TCP retransmission rates, DNS response errors, and communication anomalies."
		if isJa {
			system = "あなたはパケット解析およびネットワークプロトコルの専門家です。提示されたパケットキャプチャ集計データを分析し、ブロードキャストストーム比率、TCP再送やRSTセッションの多発、DNS解決エラー、および帯域圧迫フローの要因を日本語のマークダウン形式で解説してください。"
		}
		if data != nil {
			b, _ := json.MarshalIndent(data, "", "  ")
			sb.WriteString(string(b))
		}

	case "anomaly", "ai":
		system = "You are an AI/ML anomaly detection specialist for network operations. Correlate machine learning anomaly scores with node and polling telemetry, prioritize root-cause investigations, and suggest model tuning."
		if isJa {
			system = "あなたはAI異常検知および機械学習モデル運用の専門家です。提示された異常検知スコア（Isolation Forest等）と各種監視メトリクスの相関を分析し、突発的なスコア上昇の要因、優先して対処すべきノード・ポーリングのトリアージ、および誤検知防止のための学習データ再調整の指針を日本語のマークダウン形式で解説してください。"
		}
		if data != nil {
			b, _ := json.MarshalIndent(data, "", "  ")
			sb.WriteString(string(b))
		}

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
		system = "You are an MQTT broker and IoT operations expert. Analyze the MQTT statistics data and provide a comprehensive operational report: evaluate client message volume distribution, identify hot/concentrated topic hierarchies, analyze state health (Normal/Warn/Low ratios), detect communication anomalies or flapping clients, and recommend topic design and broker optimizations in clean markdown format."
		if isJa {
			system = "あなたはMQTTブローカーおよびIoT運用の専門家です。提示されたMQTT統計データ（クライアント・トピック・状態分布・ヒートマップ・ツリーマップ等）を分析し、①クライアント毎のトラフィック集中度と不審な接続傾向、②トピック階層の利用バランスと設計の妥当性、③異常状態（Warn/Low）の発生原因と影響、④IoTセンサー・デバイスの通信異常の兆候、⑤運用の健全化に向けた推奨対策を、日本語のマークダウン形式（表、太字、箇条書き）で分かりやすく解説してください。"
		}
		if data != nil {
			b, _ := json.MarshalIndent(data, "", "  ")
			sb.WriteString(string(b))
		}

	case "otel_metric", "otel_metrics":
		system = "You are an OpenTelemetry metrics and cloud-native observability expert. Analyze the time-series metric data points (Gauge, Sum, Histogram), detect value spikes, rate anomalies, percentile regressions (P95/P99 latency), threshold breaches, and explain potential infrastructure bottlenecks, application regressions, and remediation steps in clean markdown format."
		if isJa {
			system = "あなたはOpenTelemetryメトリクスおよびオブザーバビリティの専門家です。提示されたメトリクスデータ（時系列推移、Gauge/Sum/Histogramの分布、属性別データポイント）を分析し、①メトリック値の急激なスパイクや異常変動、②P95/P99等のパーセンタイル悪化やスループット低下の兆候、③ホスト・サービス・スコープ別の負荷偏り、④システムリソース枯渇やアプリケーション不具合の推定原因、⑤推奨される調査・チューニング手順を、日本語のマークダウン形式（表、太字、箇条書き）で分かりやすく解説してください。"
		}
		if data != nil {
			b, _ := json.MarshalIndent(data, "", "  ")
			sb.WriteString(string(b))
		}

	case "otel_trace", "otel_traces", "trace":
		system = "You are a distributed tracing, APM, and latency optimization expert. Analyze the OpenTelemetry trace waterfall and span hierarchy. Identify the critical path and latency bottlenecks, pinpoint slow database queries, external HTTP API calls, lock contention, or cascading error propagation, and provide actionable latency reduction recommendations in clean markdown format."
		if isJa {
			system = "あなたは分散トレーシング、APM、およびシステムレイテンシ最適化の専門家です。提示されたOpenTelemetryトレースデータ（全体の所要時間、スパンツリー階層、各スパンのDurationと比率、エラー・ステータス属性）を分析し、①トレース全体の遅延を引き起こしているクリティカルパスおよびボトルネック（DBスロークエリ、外部API呼び出し、同期ブロッキング等）、②エラーやステータスコード異常の発生箇所、③サービス間の依存関係における遅延連鎖、④具体的なパフォーマンス改善およびレイテンシ短縮のための推奨アクションを、日本語のマークダウン形式（表、太字、箇条書き）で分かりやすく解説してください。"
		}
		if data != nil {
			b, _ := json.MarshalIndent(data, "", "  ")
			sb.WriteString(string(b))
		}

	case "otel":
		system = "You are a full-stack OpenTelemetry observability expert. Analyze the overall OpenTelemetry telemetry summary (metrics volume, trace durations, service dependencies, error logs), assess microservice health, and recommend optimizations in clean markdown format."
		if isJa {
			system = "あなたはOpenTelemetry統合オブザーバビリティの専門家です。提示されたテレメトリ概要（メトリクス総数、トレース遅延傾向、サービス間依存DAG、エラーログ発生状況）を総合的に分析し、システム全体の健全性評価、主要ボトルネック、および信頼性向上のための推奨対策を、日本語のマークダウン形式（表、太字、箇条書き）で分かりやすく解説してください。"
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
