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
1. **IP アドレスホワイトリスト (`-mcpFrom` / マップ設定)**:
   - 接続元 IP アドレスをカンマ区切りで制限できます（例: `127.0.0.1, 192.168.1.50`）。
   - 空欄または未設定時はローカルホストのみが許可されます。
2. **認証モード (`-mcpMode auth|noauth` / マップ設定)**:
   - `auth` モード時: リクエストヘッダーに `Authorization: Bearer <MCP_TOKEN>` または TWSNMP NEO の API トークンが必要です。
   - `noauth` モード時: IP 許可リストに基づく接続制御のみを行います。

---

## 3. 公開ツール (Tools) 一覧

TWSNMP NEO MCP サーバーは、以下の 22 種類以上の高機能ツールを提供します：

### 3.1 マップ・トポロジー操作

| ツール名 | 説明 | 主要パラメータ |
|---|---|---|
| `get_node_list` | ノード一覧の取得 | `state_filter`, `name_filter`, `ip_filter` |
| `get_node` | 特定ノードの詳細情報取得 | `id` (ノードIDまたは名前) |
| `add_node` | マップへのノード新規登録 | `name`, `ip`, `icon`, `description`, `position` |
| `update_node` | 既存ノードの設定更新 | `id`, `name`, `ip`, `icon`, `description`, `position` |
| `delete_node` | ノードの削除（連動ポーリングも削除） | `id` |
| `get_network_list` | SW-HUB / ネットワーク一覧取得 | `name_filter`, `ip_filter` |
| `get_line_list` | 接続ライン一覧取得 | `node_filter`, `state_filter` |

### 3.2 監視 & ポーリング操作

| ツール名 | 説明 | 主要パラメータ |
|---|---|---|
| `get_polling_list` | ポーリング定義一覧の取得 | `type_filter`, `name_filter`, `node_name_filter`, `state_filter` |
| `get_polling_log` | ポーリングの実行ログ履歴取得 | `id` (必須), `limit` (取得件数) |
| `get_polling_log_data`| ポーリング数値メトリクスの取得 | `id` (必須), `limit` |
| `do_ping` | ターゲットへのリアルタイム Ping 実行 | `target` (必須), `size`, `count`, `timeout` |
| `snmpwalk` | 指定ノードに対する SNMP Walk 走査 | `target` (必須), `oid` |
| `get_mib_tree` | ロード済み MIB ツリー構造の取得 | なし |

### 3.3 ログ検索 & 調査 (Apache Parquet)

| ツール名 | 説明 | 主要パラメータ |
|---|---|---|
| `search_event_log` | イベントログの条件検索 | `node_filter`, `level_filter`, `event_filter`, `start_time`, `end_time` |
| `add_event_log` | イベントログへのメッセージ手動登録 | `level`, `event`, `node_id` |
| `search_syslog` | Syslog の高速横断検索 | `host_filter`, `tag_filter`, `level_filter`, `message_filter`, `start_time` |
| `get_syslog_summary` | Syslog のホスト別・タグ別集計 | `summary_type` (`host`/`tag`/`severity`), `host_filter` |
| `search_snmp_trap_log`| SNMP TRAP ログの検索 | `from_filter`, `type_filter`, `message_filter` |

### 3.4 レポート & アドレス帳

| ツール名 | 説明 | 主要パラメータ |
|---|---|---|
| `get_ip_address_list` | 検出済み IP アドレス一覧と利用状況 | `ip_filter`, `mac_filter` |
| `get_ip_address_info` | 特定 IP アドレスの履歴・ノード照会 | `ip` (必須) |
| `get_mac_address_list`| 検出済み MAC アドレス一覧 (OUIベンダー)| `mac_filter`, `vendor_filter` |
| `get_sensor_list` | 環境センサー・IoT デバイスの測定値 | `type_filter`, `name_filter` |
| `get_wifi_ap_list` | Wi-Fi アクセスポイント・電波状況 | `ssid_filter`, `bssid_filter` |
| `get_bluetooth_device_list` | Bluetooth ビーコン・デバイス一覧 | `name_filter`, `address_filter` |
| `get_server_certificate_list` | TLS/SSL 証明書の有効期限・検証 | `host_filter`, `status_filter` |
| `get_resource_monitor_list` | サーバ・ホストのリソース利用状況 | `host_filter` |

---

## 4. AI クライアント設定例

### Cursor (`~/.cursor/mcp.json` または プロジェクト `.cursor/mcp.json`)

```json
{
  "mcpServers": {
    "twsnmpneo": {
      "url": "http://127.0.0.1:8080/api/mcp",
      "headers": {
        "Authorization": "Bearer YOUR_MCP_TOKEN"
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

1. **「最近1時間でエラーが出ているサーバーはあるか調べて」**
   - AI が `search_event_log` や `search_syslog` を呼び出し、発生しているエラーログを集約して原因を分析します。
2. **「192.168.1.100 と疎通が取れるか Ping して」**
   - AI が `do_ping` を実行し、パケットロス率や平均レイテンシを即座に回答します。
3. **「有効期限が30日以内の SSL 証明書をリストアップして」**
   - AI が `get_server_certificate_list` を取得し、更新が必要なサーバー一覧をレポートします。
