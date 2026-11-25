---
title: TWSNMP NEO MCPサーバー仕様書
layout: default
---

[English (英語版はこちら)](./mcp.html) | [マニュアルTOP](./index_ja.html)

# TWSNMP NEO MCPサーバー仕様書

本ドキュメントは、TWSNMP NEO に内蔵された **Model Context Protocol (MCP)** サーバーの仕様、提供ツール一覧、および AI エージェント（Cursor, Claude Desktop, Antigravity など）からの接続・活用方法を解説します。

---

## 1. 概要

MCP (Model Context Protocol) は、LLM（大規模言語モデル）や AI エージェントが外部ツールやデータソースと安全に対話するための標準オープンプロトコルです。
TWSNMP NEO の内蔵 MCP サーバーを利用することで、AI はネットワークトポロジーの把握、ノード状態の確認、リアルタイム Ping 実行、SNMP 走査、Syslog や TRAP ログの横断検索、証明書有効期限チェックなどを自律的に実行できます。

---

## 2. トランスポートとエンドポイント

TWSNMP NEO の MCP サーバーは、最新の **Streamable HTTP** トランスポートを採用しています。

- **エンドポイント**: `http://<TWSNMP_NEO_HOST>:<PORT>/api/mcp`
- **プロトコル**: Streamable HTTP (JSON-RPC 2.0 / Server-Sent Events 連動)
- **ポート番号**: Web UI と同一の HTTP/HTTPS ポート（デフォルト: `8080`）

### セキュリティ制御
- **MCP 有効化**: Web UI のマップ設定で MCP を有効化する必要があります（無効時は `/api/mcp` が `404` を返却）。
1. **IP / CIDR ホワイトリスト (`-mcpFrom` / `TWSNMP_MCPFROM` / マップ設定)**:
   - 接続元 IP アドレスや CIDR レンジをカンマ区切りで制限できます（例: `127.0.0.1, 192.168.1.50, 10.0.0.0/24`）。
   - ループバックアドレス（`127.0.0.1`, `::1`, `localhost`）は常に許可され、未許可 IP からのアクセスは `401 Unauthorized` となります。
2. **認証モード (`-mcpMode auth|noauth` / `TWSNMP_MCPMODE` / マップ設定)**:
   - `auth` モード時: リクエストヘッダーに `Authorization: Bearer <API_TOKEN>`（Web UI または `POST /api/login` で取得する JWT トークン）が必要です。
   - `noauth` モード時: IP/CIDR 許可リストに基づく接続制御のみを行い、Bearer トークンなしで接続できます（ローカル完結運用向け）。
   - 起動時に明示された CLI フラグ（`-mcpFrom`, `-mcpMode`）や環境変数（`TWSNMP_MCPFROM`, `TWSNMP_MCPMODE`）は、初期読み込み時の保存済みマップ設定より優先されます。

---

## 3. 公開ツール (Tools) 一覧

TWSNMP NEO MCP サーバーは、以下の 44 種類の高機能ツールおよび 32 種類のプロンプトテンプレートを提供します：

### 3.1 マップ・トポロジー操作

| ツール名 | 説明 | 主要パラメータ |
|---|---|---|
| `get_node_list` | ノード一覧の取得 | `state_filter`, `name_filter`, `ip_filter` |
| `get_node` | 特定ノードの詳細情報と紐づくポーリング一覧の取得 | `id` (必須: ノードID、名前、または IP) |
| `add_node` | マップへのノード新規登録（ノードのみ作成し PING ポーリングは自動追加しません。監視は `add_polling` で別途追加してください） | `name` (必須), `ip` (必須), `icon`, `description`, `x`, `y` |
| `update_node` | 既存ノードの設定更新 | `id` (必須), `name`, `ip`, `icon`, `description`, `x`, `y` |
| `delete_node` | ノードの削除（紐づくポーリング・接続ラインもカスケード削除） | `id` (必須: ノードIDまたは一意なノード名) |
| `get_network_list` | SW-HUB / ネットワーク一覧取得 | `name_filter`, `ip_filter` |
| `get_line_list` | 接続ライン一覧取得 | `node_filter`, `state_filter` |
| `add_line` | 2ノードまたは `NET:<network_id>` コンテナ間の接続ライン作成 | `node_id1` (必須), `node_id2` (必須), `polling_id1`, `polling_id2`, `polling_id`, `width`, `port`, `info` |
| `delete_line` | 接続ラインの削除 | `id` (必須) |
| `get_system_status` | システム状態、メモリ/Goroutine、`uptime`/`uptime_sec`、登録件数、設定済み受信機状態の取得 | なし |
| `diagnose_node` | 指定ノードに対する Ping/SNMP/HTTP リアルタイム診断 | `node_id` (必須) |

### 3.2 監視 & ポーリング操作

| ツール名 | 説明 | 主要パラメータ |
|---|---|---|
| `get_polling_list` | ポーリング定義一覧の取得 | `node_id`, `type_filter`, `name_filter`, `node_name_filter`, `state_filter`, `level_filter` |
| `get_polling` | ポーリングの詳細設定と最新結果の取得 | `id` (必須) |
| `add_polling` | 既存ノードへの新規ポーリング追加 | `node_id` (必須), `name` (必須), `type` (必須), `mode`, `params`, `filter`, `extractor`, `script`, `level`, `poll_int`, `timeout`, `retry`, `log_mode`, `ai_mode`, `fail_action`, `repair_action` |
| `update_polling` | ポーリング設定の更新と即時再チェック予約 | `id` (必須), `name`, `type`, `mode`, `params`, `filter`, `extractor`, `script`, `level`, `poll_int`, `timeout`, `retry`, `log_mode`, `ai_mode`, `fail_action`, `repair_action`, `reset_state` |
| `delete_polling` | ポーリングの削除（連動接続ラインもクリーンアップ） | `id` (必須) |
| `check_polling` | 特定ポーリングの即時実行、またはノード別・全体の非正常ポーリング再チェック予約 | `polling_id`, `node_id`, `all` |
| `get_polling_log` | ポーリングの実行ログ履歴取得 | `id` (必須), `limit` (取得件数) |
| `get_polling_log_data` | ポーリング数値メトリクスの CSV 形式取得 | `id` (必須), `limit` |
| `do_ping` | ターゲットへのリアルタイム Ping 実行 | `target` (必須), `size`, `count`, `timeout` |
| `snmpwalk` | 指定ノードに対する SNMP Walk 走査 (v1/v2c/v3) | `target` (必須), `node_id`, `root_oid`, `snmp_mode`, `community`, `user`, `password` |
| `snmpset` | 指定ノードに対する SNMP SET 設定 (v1/v2c/v3) | `target` (必須), `mib_object_name` (必須), `type` (`integer`/`string`, 必須), `value` (必須), `node_id`, `snmp_mode`, `community`, `user`, `password` |
| `get_mib_tree` | ロード済み MIB ツリー構造の取得 | なし |

### 3.3 ログ検索 & トラフィック調査 (Apache Parquet)

| ツール名 | 説明 | 主要パラメータ |
|---|---|---|
| `search_event_log` | イベントログの条件検索 | `node_filter`, `type_filter`, `level_filter`, `event_filter`, `start_time`, `end_time`, `limit` |
| `add_event_log` | イベントログへのメッセージ手動登録 | `level` (必須), `event` (必須), `type`, `node_id` |
| `search_syslog` | Syslog の高速横断検索 | `host_filter`, `tag_filter`, `level_filter`, `message_filter`, `start_time`, `end_time`, `limit` |
| `get_syslog_summary` | Syslog のホスト別・タグ別・重大度別集計 | `summary_type` (`host`/`tag`/`severity`, 必須), `host_filter`, `tag_filter`, `level_filter`, `message_filter`, `start_time`, `end_time` |
| `search_snmp_trap_log` | SNMP TRAP ログの検索 | `from_filter`, `type_filter`, `message_filter`, `start_time`, `end_time`, `limit` |
| `search_netflow` | NetFlow / IPFIX フローログの検索 | `src_filter`, `dst_filter`, `protocol_filter`, `port_filter`, `start_time`, `end_time`, `limit` |

### 3.4 レポート & アドレス帳・OTel・AI

| ツール名 | 説明 | 主要パラメータ |
|---|---|---|
| `get_ip_address_list` | 検出済み IP アドレス一覧と利用状況 | なし |
| `get_ip_address_info` | 特定 IP アドレスの履歴・ノード照会 | `ip` (必須) |
| `get_mac_address_list` | 検出済み MAC アドレス一覧 (OUIベンダー) | なし |
| `get_mac_address_info` | 特定 MAC アドレスのノード・ベンダー照会 | `mac` (必須) |
| `get_sensor_list` | リモート監視センサー一覧の取得 (`twWifiScan`, `twBlueScan`, `twpcap`, `twwinlog`, `twagentneo`) | `type_filter`, `state_filter` |
| `get_wifi_ap_list` | Wi-Fi アクセスポイント・電波状況 | なし |
| `get_bluetooth_device_list` | Bluetooth ビーコン・デバイス一覧 | `public_address_only` |
| `get_server_certificate_list` | TLS/SSL 証明書の有効期限・検証状態 | なし |
| `get_resource_monitor_list` | サーバ・ホストのリソース利用履歴 (CPU/メモリ/Swap/Disk/Load) | なし |
| `get_report_summary` | 指定レポートカテゴリの記録レコード取得 | `report_type` (必須: `wifiAP`, `blueDevice`, `envMonitor`, `dnsq`, `winEventID`, `twAgentAlert`, `twAgentInfo` 等), `limit` |
| `get_ai_anomaly_list` | AI 異常検知スコア一覧の取得 | `node_filter`, `polling_filter`, `min_score`, `limit` |
| `analyze_polling_correlation` | 2つのポーリング間の相関分析（統計手法 & `tensai` ML） | `polling_id_a`, `polling_id_b`, `algorithm`, `hours` |
| `discover_polling_correlations` | ノード・ポーリング間の相関ペア自動発見 | `base_polling_id`, `algorithm`, `hours`, `min_score`, `limit` |
| `get_otel_metrics` | OpenTelemetry メトリクス一覧・値の取得 | `name_filter`, `service_filter`, `host_filter`, `limit` |
| `get_otel_traces` | OpenTelemetry 分散トレース情報の取得 | `service_filter`, `error_only`, `limit` |

### 3.5 内蔵 MCP プロンプト (32 種類)

- **トポロジー・ノード**: `get_node_list`, `get_node`, `add_node`, `update_node`, `delete_node`, `get_network_list`, `get_line_list`, `add_line`, `delete_line`
- **ポーリング・診断**: `get_polling_list`, `get_polling`, `add_polling`, `update_polling`, `delete_polling`, `check_polling`, `get_polling_log`, `do_ping`, `snmpwalk`, `get_mib_tree`
- **ログ・フロー・AI異常検知**: `search_event_log`, `add_event_log`, `search_syslog`, `get_syslog_summary`, `search_snmp_trap_log`, `search_netflow`, `get_ai_anomaly_list`, `polling_correlation_analysis`
- **台帳・レポート**: `get_ip_address_list`, `get_ip_address_info`, `get_mac_address_info`, `get_resource_monitor_list`, `get_server_certificate_list`

---

## 4. AI クライアント設定例

### Google Antigravity (`~/.gemini/config/mcp_config.json`)

```json
{
  "mcpServers": {
    "twsnmpneo": {
      "url": "http://127.0.0.1:8080/api/mcp",
      "headers": {
        "Authorization": "Bearer YOUR_API_TOKEN"
      }
    }
  }
}
```

- `-mcpMode noauth` でローカル（ループバック）運用する場合は `headers` ブロックを省略できます。
- 設定変更やサーバー再起動後にツール定義を更新したい場合は、Antigravity 上の **Additional Options (`...`) > MCP Servers** から状態確認・リフレッシュしてください。

### Cursor (`~/.cursor/mcp.json` または プロジェクト `.cursor/mcp.json`)

```json
{
  "mcpServers": {
    "twsnmpneo": {
      "url": "http://127.0.0.1:8080/api/mcp",
      "headers": {
        "Authorization": "Bearer YOUR_API_TOKEN"
      }
    }
  }
}
```

### Claude Desktop (`claude_desktop_config.json`)

```json
{
  "mcpServers": {
    "twsnmpneo": {
      "url": "http://127.0.0.1:8080/api/mcp"
    }
  }
}
```

---

## 5. 代表的なユースケース

1. **「ノード `app-01`（192.168.1.50）を登録し、PING ポーリングを追加してすぐ疎通確認して」**
   - AI が `add_node` でノードを登録し、`add_polling`（`type: "ping"`）で監視を追加したうえで、`check_polling` を呼び出して即座に疎通結果を返します。
2. **「AI 異常スコアが高い監視項目と直近1時間の不審な NetFlow 通信を調べて」**
   - AI が `get_ai_anomaly_list` と `search_netflow`、あわせて `search_event_log` を実行して異常発生箇所と通信傾向を特定します。
3. **「有効期限が近い SSL 証明書をリストアップして」**
   - AI が `get_server_certificate_list` を取得し、更新が必要なサーバー一覧をレポートします。
