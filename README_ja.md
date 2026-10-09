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
* **[English Manual (英語マニュアル)](https://twsnmp.github.io/twsnmpneo/)**
* **[MCP サーバー仕様・連携ガイド](https://twsnmp.github.io/twsnmpneo/mcp_ja.html)**
* **[AI プロンプト・診断ガイド](https://twsnmp.github.io/twsnmpneo/prompt_ja.html)**

---

## 主な特徴と機能

### 1. モダンWeb & 3D ビジュアライゼーション
- **p5.js マップキャンバス**: ノードアイコン（MDIフォント/カスタム画像）、SW-HUBネットワークポートパネル（LED点灯・折り返し表示）、トラフィックに応じた線の太さ・パケットフローアニメーション、全11種の描画アイテム（KPIカード、スパークライン、ゲージ、形状、画像）。
- **3D 仮想ハードウェアパネル（`vpanel`）**: WebGLによる3Dスイッチングハブ描画。RJ45ポートテクスチャ、Link UP/DOWN緑LED、1Gbps+橙LED、POWER青LED、マウスによる3D回転・自動回転表示。
- **Apache ECharts 分析チャート**: レスポンス推移、トラフィック量、IPAMヒートマップ、各種ログの時系列・ヒストグラム分析。

### 2. Apache Parquet による大容量・超高速ログストレージ
- **Syslog、SNMP TRAP、NetFlow v5/v9/IPFIX、sFlow v5、ARP Watch、OpenTelemetry、MQTT、ポーリング時系列ログ**を列指向圧縮フォーマット（Parquet）で効率的に保存。
- 1万件単位の高速クエリ、複合条件フィルタ、容量・日数指定による安全な自動ローテーション。

### 3. ネイティブ AI & 内蔵 MCP (Model Context Protocol) サーバー
- **Streamable HTTP MCP サーバー (`/api/mcp`)**: 22種以上の運用・診断ツールと19種のプロンプトテンプレートをAIエージェント（Cursor, Claude Desktop, Antigravity など）に提供。
- **マルチLLMエンジン (`tensai`)**: Gemini, OpenAI, Claude, ローカル Ollama への接続に対応。
- **ワンクリック AI アシスト**: イベントログの原因分析、自然言語によるポーリング作成アシスタント、ノード健全性診断、各レポートのAI解説。

### 4. 充実したプロトコルレシーバー & 診断機能
- **Syslog**: UDP, TCP, TLS (RFC 3164 / RFC 5424)
- **SNMP TRAP**: v1, v2c, v3（MIB OID名前解決対応）
- **フロー監視**: NetFlow v5/v9/IPFIX, sFlow v5（Flow/Counterサンプル）, Fumble異常通信検知
- **ARP監視**: IP-MAC対応関係の変更・競合・新規検知
- **OpenTelemetry**: OTel OTLP gRPC/HTTP レシーバー
- **MQTT ブローカー & クライアント**: IoTセンサー監視・ポーリング対応
- **内蔵 PKI 認証局**: Root CA構築、ブラウザ内CSR作成、証明書発行、CRL / OCSP / SCEP / ACME サーバー
- **診断ツール**: リアルタイムPing (ICMP/UDP)、Smokeping、MTR（AI解説付き）、MIBブラウザ / MIBツリー、gNMI、Wake-on-LAN (WOL)

---

## クイックスタート

### Docker による起動

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

ブラウザで `http://localhost:8080` にアクセスしてください。

### Docker Compose による起動

`docker-compose.yml` を作成します：

```yaml
services:
  twsnmpneo:
    image: ghcr.io/twsnmp/twsnmpneo:latest
    container_name: twsnmpneo
    restart: always
    ports:
      - "8080:8080"
      - "514:514/udp"
      - "162:162/udp"
      - "2055:2055/udp"
      - "6343:6343/udp"
    volumes:
      - ./data:/data
```

コンテナを起動します：

```bash
docker compose up -d
```

### スタンドアロンバイナリでの起動

[GitHub Releases](https://github.com/twsnmp/twsnmpneo/releases) からお使いのOSに合ったバイナリをダウンロードします。

#### Linux環境での注意点
Linuxで一般ユーザーとして実行する場合、Rawソケット（ICMP Ping）や1024未満の特権ポート（514のSyslog、162のTRAPなど）の利用に Linux Capabilities が必要です：

```bash
sudo setcap 'cap_net_bind_service,cap_net_raw+ep' ./twsnmpneo
./twsnmpneo -datastore ./data -port 8080
```

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
