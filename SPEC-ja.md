# TWSNMP NEO (twsnmpneo) 開発仕様書 (SPEC.md)

本仕様書は、コンテナ型ネットワーク管理ソフトウェア「TWSNMP FC」の後継エディションである **「TWSNMP NEO」** の設計および実装仕様を定義するものである。
Google Antigravity 2.0 は本仕様書を単一の信頼できる情報源（Single Source of Truth）として厳格に参照し、実装・テスト・ビルド設定を自律的に進めること。

---

## 1. プロジェクト概要と目標

* **プロジェクト名称**: TWSNMP NEO
* **コンテナ名 / リポジトリ名**: `twsnmpneo`
* **主目的**:
  1. **技術スタックの全面刷新**: 6年が経過したTWSNMP FCのアーキテクチャ・依存関係を刷新し、長期保守性を確立する。
  2. **TWSNMP FKの技術資産継承**: デスクトップ版「TWSNMP FK」で実証済みのGo実装、監視ロジック、各種内蔵サーバー、Apache Parquetストレージ、AI統合をコンテナ/サーバー版へ統合・昇華する。
  3. **表示互換性の維持（マップ・パネル・グラフ）**:
     * **p5.js** によるマップ・機器パネルの描画処理をそのまま移植し、TWSNMP FC/FKとの描画互換性・操作感を完全維持する。
     * **Apache ECharts** による時系列メトリクス・ログ・トラフィックのグラフ表示を継続採用する。
     * 機器アイコン（ビルトインアイコンセット、カスタムアイコン仕様）を互換維持する。
  4. **シームレスな移行（インポート）**: 既存のTWSNMP FCユーザーが設定（ノード・マップ・ポーリング等）および過去ログを安全かつ容易に移行できるインポートパイプラインを提供する。
  5. **AI連携 & MCP (Model Context Protocol) サーバー統合**: `tensai` パッケージを通じた複数LLM連携に加え、標準でMCPサーバーを搭載し、外部AIエージェントからの自律的なネットワーク分析・操作を可能にする。
  6. **テスト駆動と高信頼性**: 移植コードを含めた徹底的なユニットテスト・モックテストを実施し、不具合やエッジケースを洗い出しながらリファクタリングする。

---

## 2. 技術スタック & 動作環境

### 2.1 バックエンド (Go)
* **言語バージョン**: Go 最新安定版 (1.23+)
* **データストレージ**:
  * **bbolt**: システム設定、マップ情報、ノード・ライン管理、ポーリング定義、認証情報、イベントログ
  * **Apache Parquet**: Syslog、SNMP TRAP、NetFlow/IPFIX、ポーリング時系列ログ（高速圧縮・列指向検索・安全なローテーション/削除）
* **AI連携**:
  * `tensai` (Goパッケージ): 複数LLMプロバイダ（Gemini, OpenAI, Claude, Ollama等）の抽象化
  * **MCP サーバー機能**: JSON-RPC / SSE / Stdio によるMCPプロトコル準拠のツール公開
* **内蔵サーバー/プロトコル**:
  * MQTTブローカー、OpenTelemetry (OTel) レシーバー、プライベートPKI（自律証明書管理）
  * Syslog (UDP/TCP/TLS), SNMP TRAP (v1/v2c/v3), NetFlow/IPFIX, ARP Watch

### 2.2 フロントエンド (SPA)
* **言語/フレームワーク**: Svelte 5 (Runesベース) + Vite + TypeScript
* **マップ & パネル描画**: **p5.js**（TWSNMP FC/FKのCanvas描画ロジック・スケッチを移植）
* **グラフ & チャート描画**: **Apache ECharts**（メトリクス、レスポンスタイム、トラフィック、イベント推移）
* **スタイリング/UI**: Tailwind CSS + `shadcn-svelte` (Bits UI) + Lucide Icons
* **提供方式**: ビルド後の静的アセットをGoバイナリに `embed.FS` でバンドル（シングルバイナリ提供）

### 2.3 開発・CI/CD環境
* **ローカル環境管理 / タスクランナー**: `mise` (`mise.toml`)
* **コンテナ実行基盤**: Docker / Podman (Linux amd64, arm64)
* **CI/CD / 配布**: GitHub Actions
  * バイナリ配布: 各OS向け圧縮アーカイブ（Linux, macOS, Windows）
  * コンテナ配布: GitHub Packages (GHCR: `ghcr.io/<owner>/twsnmpneo`)

---

## 3. ディレクトリ構成

```text
twsnmpneo/
├── .github/
│   └── workflows/
│       ├── test.yml              # PR / Push時のユニットテスト & Lint
│       └── release.yml           # Tag push時のクロスビルド・アーカイブ & GHCR公開
├── backend/
│   ├── cmd/
│   │   └── twsnmpneo/
│   │       └── main.go           # エントリポイント
│   ├── internal/
│   │   ├── ai/                   # tensaiラッパー & MCPサーバー実装
│   │   ├── api/                  # REST / WebSocket / SSE ハンドラー
│   │   ├── datastore/
│   │   │   ├── bbolt/            # 設定・ノード・イベントDB
│   │   │   └── parquet/          # 各種ログストレージ & ローテーション
│   │   ├── importer/             # 旧TWSNMP FCデータインポート・変換ロジック
│   │   ├── pki/                  # 内蔵PKI / CA・証明書管理
│   │   ├── polling/              # ポーリングエンジン (Ping, SNMP, HTTP, Script等)
│   │   ├── receiver/             # Syslog, TRAP, NetFlow, OTel, MQTTサーバー
│   │   └── notify/               # LINE, Slack, Webhook, メール等の通知処理
│   └── go.mod
├── frontend/
│   ├── src/
│   │   ├── lib/
│   │   │   ├── components/       # shadcn-svelte UIコンポーネント
│   │   │   ├── map/              # p5.js によるネットワークマップ・パネル描画
│   │   │   ├── charts/           # Apache ECharts コンポーネントラッパー
│   │   │   ├── mcp/              # MCP / AIチャットインターフェース
│   │   │   └── stores/           # Svelte 5 Runes ($state, $derived) による状態管理
│   │   ├── static/
│   │   │   └── icons/            # 互換機器アイコンセット
│   │   ├── App.svelte
│   │   └── main.ts
│   ├── package.json
│   ├── vite.config.ts
│   └── svelte.config.js
├── Dockerfile                    # Multi-stage build (Node build -> Go build -> Final)
├── docker-compose.yml
├── mise.toml                     # 開発ツール・タスクランナー定義
├── SPEC.md                       # 本仕様書
└── README.md
```

---

## 4. 機能要件

### 4.1 マップ・機器パネル・グラフィック仕様 (p5.js & ECharts)
* **p5.js によるマップ・パネル描画**:
  * TWSNMP FC/FKの `p5.js` スケッチコードを Svelte 5 コンポーネント（`frontend/src/lib/map/`）へ移植。
  * ノードの配置、ライン描画、アニメーション（パケットフローや障害点滅表示）、ズーム・パン操作、背景画像サポートを従来と同一挙動で実現する。
  * 機器パネル（ノード状態パネル、ポート接続状況、ラックマウント風ビュー等）の描画ロジックも互換移植する。
* **アイコン仕様の完全互換**:
  * TWSNMP FC/FK標準のビルトインアイコン（ルーター、スイッチ、サーバー、PC、クラウド、センサー等）をそのまま `frontend/src/static/icons/` に保持。
  * ユーザー定義のカスタムアイコンの登録・参照パス、アイコンIDの互換性を維持する。
* **Apache ECharts によるグラフ描画**:
  * ポーリングレスポンス時間推移、CPU/メモリ使用率、トラフィックグラフ、Syslog/TRAP発生頻度ヒストグラムを Apache ECharts で実装。
  * ダークモード/ライトモード対応のテーマカラー定義、時間軸ズーム（DataZoom）、ツールチップ表示をサポート。

### 4.2 データ移行（TWSNMP FC インポート）
* **入力**: 旧TWSNMP FCのデータベースファイル（bboltバックアップ）またはエクスポートアーカイブ。
* **処理フロー**:
  1. 旧データスキーマのバリデーションと読み込み。
  2. ノード・ライン・マップ座標・アイコン設定・ポーリング設定・ユーザーアカウントのTWSNMP NEO新スキーマへのマッピングと整合性検査。
  3. イベントログおよび過去ログのインポート（オプション選択可能）。
  4. 変換結果サマリー（成功件数、スキップ項目、警告）をレポート出力。

### 4.3 ログ管理（Apache Parquet）
* Syslog、SNMP TRAP、NetFlow、ポーリング時系列ログはメモリバッファリング後、定期的にParquet形式でファイル出力。
* 列指向フォーマットを活かし、日時範囲、ノードIP、ログレベル、タグ等による高速フィルタリングAPIを提供。
* 設定された保持期間（日単位）やディスク容量制限に基づき、Parquetファイル単位での安全・低負荷な自動削除を実施。

### 4.4 AI & MCP (Model Context Protocol) 統合
* **`tensai` によるマルチLLM連携**:
  * 設定画面からGemini、OpenAI、Claude、OllamaのAPIキー/エンドポイントを設定可能。
  * 障害ログ分析、アラート原因の推論、復旧アドバイス、ポーリング正規表現・スクリプトの自動生成支援。
* **内蔵MCPサーバー**:
  * トランスポート: SSE (HTTP) および Stdio。
  * 提供するMCP Tool群:
    * `get_system_status`: NEO全体のヘルスチェックと稼働サマリー
    * `list_nodes`: ノード一覧とステータス取得
    * `get_node_detail`: 特定ノードの詳細、MIB情報、ポーリング一覧
    * `get_active_alerts`: 現在発生中の障害・アラート一覧
    * `query_logs`: Parquetストアからのログ条件検索

### 4.5 内蔵サーバー・プロトコル機能
* TWSNMP FKの成熟した実装をベースに移植：
  * **Syslog / TRAP / NetFlow**: パケット受信・デコード・Parquet保存・アラート判定。
  * **OpenTelemetry Receiver**: OTel形式のメトリクス/ログ受信。
  * **MQTTブローカー**: デバイス連携、MQTT Subscribeによるポーリング判定。
  * **プライベートPKI**: Web UI用TLS証明書、内部認証用クライアント証明書の自動発行・更新。

---

## 5. テスト方針 & コード移植プロセス

TWSNMP FK等からの移植コードに対しては、以下のプロセスを徹底する。

1. **テストファースト・移植**:
   * 対象モジュール（特にポーリング判定、パケットデコーダー、データ変換処理）ごとに、FKの既存ロジックに対するユニットテスト（正常系・異常系・境界値）を先に作成する。
2. **リファクタリングと改善**:
   * テストを実行しながらコードをNEOのパッケージ構成へ移植し、不要な依存の排除、エラーハンドリングの改善、並行処理（goroutine/channel）のリーク防止を実施する。
3. **カバレッジ目標**:
   * `internal/polling`、`internal/datastore`、`internal/importer` などの主要コアロジックにおいて、C1カバレッジ（分岐網羅）80%以上を達成する。
4. **モックの活用**:
   * SNMPエージェント、Syslog送信元、LLM APIなどの外部依存はインターフェース化し、ユニットテスト内でモック（またはフェイクサーバー）を使用して自律実行可能にする。

---

## 6. ローカル環境構築 & `mise` 設定 (`mise.toml`)

ローカル開発環境のセットアップおよびビルド・テスト実行は `mise` で一元管理する。

```toml
[tools]
go = "1.23"
node = "22"
pnpm = "latest"

[tasks.dev-frontend]
description = "フロントエンド開発サーバーを起動 (Vite)"
dir = "frontend"
run = "pnpm dev"

[tasks.build-frontend]
description = "フロントエンドをビルドしてGoのembedディレクトリへ配置"
dir = "frontend"
run = "pnpm build"

[tasks.test-backend]
description = "バックエンドの全ユニットテストを実行 (レースコンディション検知付き)"
dir = "backend"
run = "go test -race -v ./..."

[tasks.build-debug]
description = "フロントエンドをバンドルしたデバッグ用Goバイナリを作成"
depends = ["build-frontend"]
dir = "backend"
run = "go build -tags dev -o ../bin/twsnmpneo ./cmd/twsnmpneo"

[tasks.run-debug]
description = "デバッグ版TWSNMP NEOをローカルで起動"
depends = ["build-debug"]
run = "./bin/twsnmpneo --datadir ./data --debug"
```

---

## 7. CI/CD & リリース仕様 (GitHub Actions)

### 7.1 CI パイプライン (`test.yml`)
* PR作成時および main ブランチへの Push 時に発火。
* `jdx/mise-action` を利用してツールチェイン（Go, Node, pnpm）を復元。
* フロントエンドの Lint & TypeCheck & Build。
* バックエンドの `go test -race -cover ./...` および `golangci-lint` を実行。

### 7.2 リリース パイプライン (`release.yml`)
* Git タグ（`v*.*.*`）プッシュ時にトリガー。
* **バイナリクロスビルド & アーカイブ**:
  * ターゲット: Linux (amd64, arm64), macOS (darwin/amd64, darwin/arm64), Windows (amd64)
  * 各バイナリを `tar.gz`（Windowsは `zip`）に圧縮し、GitHub Release のアセットとして添付。
* **コンテナイメージビルド & 配布**:
  * `docker/build-push-action` を使用したマルチアーキテクチャビルド（`linux/amd64`, `linux/arm64`）。
  * GitHub Container Registry (`ghcr.io/${{ github.repository_owner }}/twsnmpneo`) に自動プッシュ。
  * タグ: `latest`、`vX.Y.Z`、`vX.Y`

---

## 8. Google Antigravity 2.0 への実装指示

1. **ステップ1**: `mise.toml`、フロントエンドの初期スケルトン（Svelte 5 + Tailwind + shadcn-svelte + p5.js + Apache ECharts）、およびGoバックエンドの基本骨格を作成する。
2. **ステップ2**: `internal/datastore`（bbolt & Parquet）と `internal/importer`（FCインポート）を実装し、ユニットテストで旧FCデータの変換精度を担保する。
3. **ステップ3**: TWSNMP FKからポーリングエンジン、各種レシーバー、PKI、MQTT等のコアを単体テストを追加しながら移植・リファクタリングする。
4. **ステップ4**: `tensai` によるLLM連携機能およびMCPサーバーエンドポイントを実装する。
5. **ステップ5**: p5.jsスケッチとアイコンセットを組み込んだマップ・パネル描画コンポーネント、EChartsコンポーネント、管理Web UI画面を構築し、Goの `embed.FS` で結合する。
6. **ステップ6**: Multi-stage Dockerfile および GitHub Actions ワークフロー（`test.yml`, `release.yml`）を完成させ、ローカル `mise run test-backend` および `mise run run-debug` で動作確認を行う。

