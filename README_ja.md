# TWSNMP NEO

[English (英語版はこちら)](README.md)

[![Go Report Card](https://goreportcard.com/badge/github.com/twsnmp/twsnmpneo)](https://goreportcard.com/report/github.com/twsnmp/twsnmpneo)
![GitHub Go version](https://img.shields.io/github/go-mod/go-version/twsnmp/twsnmpneo)
![GitHub release (latest by date)](https://img.shields.io/github/v/release/twsnmp/twsnmpneo)
![GitHub License](https://img.shields.io/github/license/twsnmp/twsnmpneo)
![GitHub Repo stars](https://img.shields.io/github/stars/twsnmp/twsnmpneo?style=social)

【Built with】
![Go](https://img.shields.io/badge/go-%2300ADD8.svg?style=for-the-badge&logo=go&logoColor=white)
![Svelte](https://img.shields.io/badge/svelte-%23f1413d.svg?style=for-the-badge&logo=svelte&logoColor=white)
![TypeScript](https://img.shields.io/badge/typescript-%23007ACC.svg?style=for-the-badge&logo=typescript&logoColor=white)
![Docker](https://img.shields.io/badge/docker-%230db7ed.svg?style=for-the-badge&logo=docker&logoColor=white)

**TWSNMP NEO** は、コンテナ型ネットワーク管理ソフトウェア「TWSNMP FC」およびデスクトップ版「TWSNMP FK」の後継・統合エディションとなる次世代ネットワーク管理システム（NMS）です。最新のWeb技術とAI連携、Apache Parquet による超高速ログ基盤を備え、Webブラウザから直感的にネットワーク全体を可視化・監視できます。

![TWSNMP NEO メイン画面](docs/images/ja/readme_hero_banner.png)

---

## ドキュメント（GitHub Pages）

* **[はじめての TWSNMP NEO（日本語マニュアル）](https://twsnmp.github.io/twsnmpneo/index_ja.html)**
* **[ライフサイクル（リリース・導入・自動更新）ガイド](https://twsnmp.github.io/twsnmpneo/lifecycle_ja.html)**
* **[English Manual (英語マニュアル)](https://twsnmp.github.io/twsnmpneo/)**
* **[MCP サーバー仕様・連携ガイド](https://twsnmp.github.io/twsnmpneo/mcp_ja.html)**
* **[AI プロンプト・診断ガイド](https://twsnmp.github.io/twsnmpneo/prompt_ja.html)**

---

## 主な特徴と機能

### 1. モダンWeb & 3D ビジュアライゼーション
- **p5.js マップキャンバス**: ノードアイコン（MDIフォント/カスタム画像）、SW-HUBネットワークポートパネル（LED点灯・折り返し表示）、トラフィックに応じた線の太さ・パケットフローアニメーション、全11種の描画アイテム（KPIカード、スパークライン、ゲージ、形状、画像）。
- **MapLibre GIS 位置情報マップ**: 国土地理院・OpenStreetMap・CARTO 等の地図タイル上にノードを配置し、広域拠点網の地理的物理配置とリアルタイム稼働状態を監視。未配置ノード配置、マーカードラッグ＆ドロップ、表示位置保存に対応。
- **3D 仮想ハードウェアパネル（`vpanel`）**: WebGLによる3Dスイッチングハブ描画。RJ45ポートテクスチャ、Link UP/DOWN緑LED、1Gbps+橙LED、POWER青LED、マウスによる3D回転・自動回転表示。
- **Apache ECharts 分析チャート**: レスポンス推移、トラフィック量、IPAMヒートマップ、各種ログの時系列・ヒストグラム分析。

### 2. Apache Parquet による大容量・超高速ログストレージ
- **Syslog、SNMP TRAP、NetFlow v5/v9/IPFIX、sFlow v5、ARP Watch、OpenTelemetry、MQTT、ポーリング時系列ログ**を列指向圧縮フォーマット（Parquet）で効率的に保存。
- 1万件単位の高速クエリ、複合条件フィルタ、容量・日数指定による安全な自動ローテーション。

### 3. ネイティブ AI & 内蔵 MCP (Model Context Protocol) サーバー
- **Streamable HTTP MCP サーバー (`/api/mcp`)**: 22種以上の運用・診断ツールと19種のプロンプトテンプレートをAIエージェント（Cursor, Claude Desktop, Antigravity など）に提供。
- **マルチLLMエンジン (`tensai` v0.0.35+)**: Gemini, OpenAI, Claude, ローカル Ollama への接続および WebGPU オフライン推論に対応。
- **ワンクリック AI 診断**: イベントログの原因分析、全レポート画面（Syslog, TRAP, NetFlow, IP, ARP, Sensor, MQTT, OTel 等）での統計データAI診断（Markdown表示、WAITアニメーション）、ノード健全性診断。

### 4. TWAgent & AI セキュリティ監査 (`twagentneo` 連携)
- **マルチプロトコル自動受信**: 事前設定不要で、MQTT・Syslog (RFC 5424)・SNMP TRAP 経由で受信した `twagentneo` のテレメトリ、アラート、FIM、SCAログを自動解析・分類。
- **エージェントインベントリ & リアルタイム監視**: 稼働状態（LWT / ハートビート）、OS情報、CPU・メモリ・ディスク・ネットワークレートのリアルタイム表示、およびマップへのワンクリックノード/推奨ポーリング自動登録。
- **AI セキュリティ & コンプライアンスアラート**: Sigma ルール検知とローカル LLM (`tensai`) 判定による真の脅威判定（`IsThreat: YES/NO`）、日本語サマリー・対処手順提示。
- **ファイル整合性監視 (FIM)**: ファイルの作成・変更・削除・パーミッション変更（SHA256ハッシュ付き）のリアルタイム追跡。
- **セキュリティ設定診断 (SCA)**: CIS ベンチマークに準拠したパーミッション、残留ファイル、平文クレデンシャル、サービス構成等の監査結果とマスク済みエビデンス・是正勧告の集計。
- **プロセス別外向きTCP接続**: プロセスごとの通信先IP、サービス種別、GeoIP国別情報と偏差スコアによる異常通信判定。

### 5. 充実したプロトコルレシーバー & 診断機能
- **Syslog**: UDP, TCP, TLS (RFC 3164 / RFC 5424)
- **SNMP TRAP**: v1, v2c, v3（MIB OID名前解決対応）
- **フロー監視**: NetFlow v5/v9/IPFIX, sFlow v5（Flow/Counterサンプル）, Fumble異常通信検知
- **ARP監視**: IP-MAC対応関係の変更・競合・新規検知
- **OpenTelemetry**: OTel OTLP gRPC/HTTP レシーバーおよびレポート分析
- **MQTT ブローカー & クライアント**: IoTセンサー監視・トピック統計レポート
- **オンデマンドレポート高速化**: カテゴリ別遅延読み込み（Lazy-load）とParquetクエリ時の時間範囲プルーニングによる高速レスポンス
- **内蔵 PKI & TLS 管理**: Root CA構築、ブラウザ内CSR作成、証明書発行、CRL / OCSP / SCEP / ACME サーバー、および自身の HTTPS 運用を自動化する ACME 自動更新クライアント（lego 統合）
- **診断ツール**: リアルタイムPing (ICMP/UDP)、Smokeping、MTR（AI解説付き）、MIBブラウザ / MIBツリー、gNMI、Wake-on-LAN (WOL)
- **システム管理**: システムステータス（デーモン稼働状態、DB容量予測、ワンクリックセルフアップデート）とリソースモニターの2タブ構成

---

## クイックスタート

![TWSNMP NEO ライフサイクル・インフォグラフィック](docs/images/twsnmpneo_lifecycle_infographic.jpg)

> 💡 **詳しい解説**: リリースパイプライン、各OSの導入設定、自動更新の仕様詳細は **[ライフサイクルガイド（日本語）](https://twsnmp.github.io/twsnmpneo/lifecycle_ja.html)** をご参照ください。

### インストール方法

#### Docker による起動（推奨）

```bash
docker run -d \
  --name twsnmpneo \
  --restart always \
  -p 8080:8080 \
  -p 514:514/udp \
  -p 162:162/udp \
  -p 2055:2055/udp \
  -p 6343:6343/udp \
  -v $(pwd)/data:/data \
  ghcr.io/twsnmp/twsnmpneo:latest
```

ブラウザで `http://localhost:8080` にアクセスしてください。Docker Compose で起動する場合は `docker compose up -d` を実行します。

#### Homebrew (macOS / Linux)
```bash
brew install twsnmp/tap/twsnmpneo
twsnmpneo -datastore ./data -port 8080
```

#### Windows (スタンドアロン & Windows サービス常駐)
[GitHub Releases](https://github.com/twsnmp/twsnmpneo/releases) から `twsnmpneo_windows_amd64.zip` をダウンロード・展開し、管理者権限で実行します：
```powershell
# Windows サービスとして登録（OS起動時に自動起動）
.\twsnmpneo.exe service install -service-auto -datastore C:\twsnmpneo\data -port 8080

# サービスの開始
.\twsnmpneo.exe service start
```

#### シェルスクリプト (curl インストーラー: Linux / macOS)
```bash
curl -sS https://raw.githubusercontent.com/twsnmp/twsnmpneo/main/install/install.sh | sh
```
CI環境や非対話モードで導入する場合:
```bash
curl -sS https://raw.githubusercontent.com/twsnmp/twsnmpneo/main/install/install.sh | sh -s -- -y
```

Linux で一般ユーザーとして実行する場合の Capabilities 付与：
```bash
sudo setcap 'cap_net_bind_service,cap_net_raw+ep' /usr/local/bin/twsnmpneo
```

Linux systemd サービスとして登録（常駐・OS起動時に自動起動）：
```bash
sudo /usr/local/bin/twsnmpneo service install -auto -datastore /var/lib/twsnmpneo -port 8080
sudo /usr/local/bin/twsnmpneo service start
```

---

### スマート自動更新 (Self-Updater)

TWSNMP NEO は動作環境（Docker、Homebrew、Standalone）を自己判別し、安全に最新版へ更新できます：

```bash
# 新バージョンの確認
twsnmpneo update -check

# 最新バージョンへの即時更新 & プロセス自動再起動
twsnmpneo update -y
```
※ Web UI 管理画面からも、新バージョン通知バナーからワンクリックで更新適用が可能です。

---

## ソースコードからのビルド & 開発

### 必要環境
* **Go 1.27 以上**
* **Node.js 22 以上 & pnpm**
* **mise**（ツール・タスクランナー）
* **air**（バックエンドホットリロード用、mise で自動管理可能）

### mise によるビルド

[mise](https://mise.jdx.dev/) を使用してビルドおよび実行を簡単に行えます：

```bash
# 依存ツールのインストール
mise install

# リリースバイナリのビルド（フロントエンドビルド + Go embed バイナリ生成）
mise run build
# 出力先: bin/twsnmpneo
```

---

## 開発 & デバッグ起動

### 1. デバッグ版バイナリのビルド
```bash
mise run build-debug
```

### 2. バックエンドのホットリロード付きデバッグ起動 (air)
Go のソースコード変更を自動検知して再コンパイル・再起動します：
```bash
mise run run-debug
```

### 3. デバッグ版バイナリの直接実行
```bash
mise run run-direct
# (内部で ./bin/twsnmpneo --datadir ./data --debug を実行)
```

### 4. フロントエンド開発サーバーの起動 (Vite HMR)
UI開発時に Vite の高速な Hot Module Replacement を利用する場合：
```bash
mise run dev-frontend
# http://localhost:5173 で起動
```

---

## ライセンス

TWSNMP NEO は [Apache License 2.0](LICENSE) のもとで公開されています。
