package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/ai"
	"github.com/twsnmp/twsnmpneo/backend/internal/api"
	"github.com/twsnmp/twsnmpneo/backend/internal/certmgr"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/bbolt"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	"github.com/twsnmp/twsnmpneo/backend/internal/i18n"
	"github.com/twsnmp/twsnmpneo/backend/internal/importer"
	"github.com/twsnmp/twsnmpneo/backend/internal/mib"
	"github.com/twsnmp/twsnmpneo/backend/internal/monitor"
	"github.com/twsnmp/twsnmpneo/backend/internal/notify"
	"github.com/twsnmp/twsnmpneo/backend/internal/pki"
	"github.com/twsnmp/twsnmpneo/backend/internal/polling"
	"github.com/twsnmp/twsnmpneo/backend/internal/receiver"
	"github.com/twsnmp/twsnmpneo/backend/internal/service"
	"github.com/twsnmp/twsnmpneo/backend/internal/topology"
	"github.com/twsnmp/twsnmpneo/backend/internal/updater"
)

var (
	version = "v0.2.0"
	commit  = ""
	date    = "unknown"
)

func resolveCommit() string {
	if commit != "" && commit != "none" {
		return commit
	}
	// 1. Try Go build info (vcs.revision automatically embedded by Go 1.18+)
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" && s.Value != "" {
				rev := s.Value
				if len(rev) > 7 {
					return rev[:7]
				}
				return rev
			}
		}
	}
	// 2. Try git CLI fallback for dev/debug live-reload environments
	if out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output(); err == nil {
		c := strings.TrimSpace(string(out))
		if c != "" {
			return c
		}
	}
	return "dev"
}

type serverConfig struct {
	dataDir         string
	lang            string
	port            int
	acmeURL         string
	syslogUDP       int
	syslogTCP       int
	trapPort        int
	netflowPort     int
	sflowPort       int
	otelPort        int
	mqttPort        int
	shutdownTimeout int
	debug           bool
	tlsFlag         bool
	certPath        string
	keyPath         string
	acmeDomains     string
	acmeCertURL     string
	acmeEmail       string
	acmeInsecure    bool
	acmeRenewDays   int
	httpPort        int
	hostName        string
	ipAddr          string
	explicitFlags   map[string]bool
}

func handleServiceCommand(action string, remainingArgs []string) {
	switch strings.ToLower(action) {
	case "install":
		var autoStart bool
		var delayedStart bool
		var serviceArgs []string

		for i := 0; i < len(remainingArgs); i++ {
			arg := remainingArgs[i]
			if arg == "-auto" || arg == "--auto" {
				autoStart = true
			} else if arg == "-delayed" || arg == "--delayed" {
				delayedStart = true
			} else {
				serviceArgs = append(serviceArgs, arg)
			}
		}

		err := service.Install(service.InstallOptions{
			AutoStart:    autoStart,
			DelayedStart: delayedStart,
			Arguments:    serviceArgs,
		})
		if err != nil {
			fmt.Printf("Failed to install service: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Service %q installed successfully.\n", service.ServiceName)
		if delayedStart {
			fmt.Println("Startup type: Automatic (Delayed Start)")
		} else if autoStart {
			fmt.Println("Startup type: Automatic")
		} else {
			fmt.Println("Startup type: Manual")
		}
		os.Exit(0)

	case "remove", "uninstall":
		if err := service.Remove(); err != nil {
			fmt.Printf("Failed to remove service: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Service %q removed successfully.\n", service.ServiceName)
		os.Exit(0)

	case "start":
		if err := service.Start(); err != nil {
			fmt.Printf("Failed to start service: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Service %q started successfully.\n", service.ServiceName)
		os.Exit(0)

	case "stop":
		if err := service.Stop(); err != nil {
			fmt.Printf("Failed to stop service: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Service %q stopped successfully.\n", service.ServiceName)
		os.Exit(0)

	case "status":
		st, err := service.Status()
		if err != nil {
			fmt.Printf("Failed to query service status: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Service %q status: %s\n", service.ServiceName, st)
		os.Exit(0)

	default:
		fmt.Printf("Unknown service command %q. Available commands: install, remove, start, stop, status\n", action)
		os.Exit(1)
	}
}

type serviceRunner struct {
	cfg    serverConfig
	cancel context.CancelFunc
	done   chan struct{}
	err    error
}

func (s *serviceRunner) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.done = make(chan struct{})
	go func() {
		defer close(s.done)
		s.err = runServer(ctx, s.cfg)
	}()
	return nil
}

func (s *serviceRunner) Stop() error {
	if s.cancel != nil {
		s.cancel()
	}
	<-s.done
	return s.err
}

func handleUpdateCommand(args []string, debugFlag bool) {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	checkOnly := fs.Bool("check", false, "Check for available updates without applying")
	shortCheck := fs.Bool("c", false, "Check for available updates without applying (shorthand)")
	targetVer := fs.String("version", "", "Update to specified release tag (e.g. 'v0.2.0')")
	yesFlag := fs.Bool("yes", false, "Update without confirmation (ideal for CI)")
	shortYes := fs.Bool("y", false, "Update without confirmation (shorthand)")
	debug := fs.Bool("debug", false, "Enable debug mode check")
	_ = fs.Parse(args)

	isCheck := *checkOnly || *shortCheck
	isYes := *yesFlag || *shortYes
	isDebug := debugFlag || *debug

	env := updater.DetectEnvironment(version, commit, isDebug)
	fmt.Printf("TWSNMP NEO %s (commit: %s, date: %s)\n", version, commit, date)
	fmt.Printf("Environment: %s\n\n", env.Type)

	switch env.Type {
	case updater.EnvDebug:
		fmt.Println("【注意】デバッグモード（開発環境）で動作しています。")
		fmt.Println("デバッグビルド環境ではセルフアップデートは無効です。ソースコードからビルドしてください。")
		os.Exit(0)

	case updater.EnvHomebrew:
		fmt.Println("【Homebrew 環境を検出】")
		fmt.Println("twsnmpneo は Homebrew 経由でインストールされています。")
		fmt.Println("アップデートを実行するには、以下のコマンドを実行してください:")
		fmt.Println()
		fmt.Println("  brew upgrade twsnmp/tap/twsnmpneo")
		fmt.Println()
		os.Exit(0)

	case updater.EnvContainer:
		fmt.Println("【Docker コンテナ環境を検出】")
		fmt.Println("コンテナ環境ではバイナリのセルフアップデートは行えません。")
		fmt.Println("コンテナイメージを更新してください:")
		fmt.Println()
		fmt.Println("  docker compose pull && docker compose up -d")
		fmt.Println()
		os.Exit(0)
	}

	// Standalone binary / curl installer:
	upd := updater.New(updater.Config{
		CurrentVersion: version,
		Commit:         commit,
		Debug:          false,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	fmt.Println("GitHub Releases から最新バージョンを確認中...")
	info, err := upd.CheckUpdate(ctx, true)
	if err != nil {
		fmt.Printf("アップデート確認エラー: %v\n", err)
		os.Exit(1)
	}

	if !info.HasUpdate && *targetVer == "" {
		fmt.Printf("現在のバージョン %s は最新です。\n", version)
		os.Exit(0)
	}

	fmt.Printf("\n新しいバージョンが利用可能です: %s (現在: %s)\n", info.LatestVersion, version)
	if info.ReleaseTitle != "" {
		fmt.Printf("タイトル: %s\n", info.ReleaseTitle)
	}
	if info.ReleaseBody != "" {
		fmt.Printf("リリースノート:\n%s\n\n", info.ReleaseBody)
	}

	if isCheck {
		fmt.Println("アップデートを適用するには 'twsnmpneo update' を実行してください。")
		os.Exit(0)
	}

	if !isYes {
		fmt.Printf("%s へアップデートを実行しますか？ [y/N]: ", info.LatestVersion)
		var answer string
		_, _ = fmt.Scanln(&answer)
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("アップデートを中止しました。")
			os.Exit(0)
		}
	}

	fmt.Printf("%s へのアップデートを開始します...\n", info.LatestVersion)
	if err := upd.RunUpdateDirect(ctx, *targetVer); err != nil {
		fmt.Printf("アップデート失敗: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("TWSNMP NEO が %s へ正常にアップデートされました！\n", info.LatestVersion)
	os.Exit(0)
}

func main() {
	commit = resolveCommit()

	// Check if invoked as a subcommand: "twsnmpneo service <action> [args...]"
	if len(os.Args) >= 2 && strings.ToLower(os.Args[1]) == "service" {
		if len(os.Args) < 3 {
			fmt.Println("Usage: twsnmpneo service <install|remove|start|stop|status> [options]")
			os.Exit(1)
		}
		action := os.Args[2]
		remaining := os.Args[3:]
		handleServiceCommand(action, remaining)
		return
	}

	// Check if invoked as a subcommand: "twsnmpneo update [options]"
	if len(os.Args) >= 2 && strings.ToLower(os.Args[1]) == "update" {
		handleUpdateCommand(os.Args[2:], false)
		return
	}

	// If invoked as a Windows Service by SCM:
	// Adjust current working directory to executable directory to avoid System32 path resolution
	if service.IsWindowsService() {
		if exe, err := os.Executable(); err == nil {
			_ = os.Chdir(filepath.Dir(exe))
		}
	}

	fs := flag.NewFlagSet("twsnmpneo", flag.ExitOnError)
	var (
		dataDir         = fs.String("datadir", "./data", "Directory to store database and log files")
		lang            = fs.String("lang", "en", "Language (en or ja)")
		port            = fs.Int("port", 8080, "Web interface port")
		acmeURL         = fs.String("acme-url", "", "Override the PKI ACME HTTPS base URL and enable the listener")
		syslogUDP       = fs.Int("syslog-udp", 0, "Syslog UDP port (0 to disable or default)")
		syslogTCP       = fs.Int("syslog-tcp", 0, "Syslog TCP port (0 to disable)")
		trapPort        = fs.Int("trap-port", 0, "SNMP TRAP UDP port (0 to disable or default 162)")
		netflowPort     = fs.Int("netflow-port", 0, "NetFlow UDP port (0 to disable or default 2055)")
		sflowPort       = fs.Int("sflow-port", 0, "sFlow UDP port (0 to disable or default 6343)")
		otelPort        = fs.Int("otel-port", 0, "OpenTelemetry OTLP HTTP port (0 to disable or default 4318)")
		mqttPort        = fs.Int("mqtt-port", 0, "MQTT broker port (0 to disable or default 1883)")
		shutdownTimeout = fs.Int("shutdown-timeout", 5, "Graceful shutdown fallback timeout in seconds")
		debug           = fs.Bool("debug", false, "Enable debug logging")
		verFlag         = fs.Bool("version", false, "Show version and exit")
		rollbackVersion = fs.String("rollback", "", "Rollback binary to previous backup (.bak) or specified GitHub version (e.g. 'v0.1.0')")
		updateFlag      = fs.Bool("update", false, "Update twsnmpneo to the latest release")
		checkUpdateFlag = fs.Bool("check-update", false, "Check for available twsnmpneo updates and environment info")

		// Service management flags
		serviceCmd     = fs.String("service", "", "Service action for Windows SCM or Linux systemd (install, remove, start, stop, status)")
		serviceAuto    = fs.Bool("service-auto", false, "Set service startup type to automatic when installing")
		serviceDelayed = fs.Bool("service-delayed", false, "Set service startup type to automatic (delayed) when installing")

		// TLS / ACME server certificate flags
		tlsFlag       = fs.Bool("tls", false, "Enable TLS (HTTPS) for Web interface")
		certPath      = fs.String("cert", "", "Path to server certificate file (PEM)")
		keyPath       = fs.String("key", "", "Path to server private key file (PEM)")
		acmeDomains   = fs.String("acme-domain", "", "Domains / SANs for ACME auto-update server cert")
		acmeCertURL   = fs.String("acme-cert-url", "", "ACME Directory URL for server cert (default Let's Encrypt)")
		acmeEmail     = fs.String("acme-email", "", "Contact email for ACME account registration")
		acmeInsecure  = fs.Bool("acme-insecure", false, "Skip server certificate verification for ACME")
		acmeRenewDays = fs.Int("acme-renew-days", 30, "Days before certificate expiry to trigger ACME renewal")
		httpPort      = fs.Int("http-port", 80, "HTTP port for ACME challenge & HTTPS redirect (0 to disable)")
		hostName      = fs.String("host", "", "Host name for self-signed certificate SAN")
		ipAddr        = fs.String("ip", "", "IP address for self-signed certificate SAN")

		// Migration flags
		importFC       = fs.String("import-fc", "", "Path to legacy TWSNMP FC data directory or twsnmpfc.db to import and compact")
		importEvents   = fs.Bool("import-events", true, "Import event logs during TWSNMP FC migration")
		forceImport    = fs.Bool("force", false, "Force overwrite of target database during migration")
		runAfterImport = fs.Bool("run-after-import", false, "Start TWSNMP NEO server after completing migration")
	)

	// Environment variable overrides (TWSNMP_<NAME> and legacy TWSNMPFC_<NAME>)
	fs.VisitAll(func(f *flag.Flag) {
		nameUpper := strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
		if s := os.Getenv("TWSNMP_" + nameUpper); s != "" {
			_ = f.Value.Set(s)
			return
		}
		// Legacy TWSNMP FC environment variables
		legacyName := strings.ToUpper(strings.ReplaceAll(f.Name, "-", ""))
		if s := os.Getenv("TWSNMPFC_" + legacyName); s != "" {
			_ = f.Value.Set(s)
			return
		}
	})
	// Legacy FC specific mappings
	if s := os.Getenv("TWSNMPFC_AUTOCERTURL"); s != "" && *acmeCertURL == "" {
		*acmeCertURL = s
	}
	if s := os.Getenv("TWSNMPFC_AUTOCERTEMAIL"); s != "" && *acmeEmail == "" {
		*acmeEmail = s
	}
	if s := os.Getenv("TWSNMPFC_AUTOCERTSANS"); s != "" && *acmeDomains == "" {
		*acmeDomains = s
	}
	if s := os.Getenv("TWSNMPFC_AUTOCERTINSECURE"); s == "true" {
		*acmeInsecure = true
	}

	_ = fs.Parse(os.Args[1:])
	explicitFlags := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		explicitFlags[f.Name] = true
	})

	// Handle flag-based service management: -service <action>
	if *serviceCmd != "" {
		var extraArgs []string
		if *serviceAuto {
			extraArgs = append(extraArgs, "-auto")
		}
		if *serviceDelayed {
			extraArgs = append(extraArgs, "-delayed")
		}
		// Forward any configuration flags passed alongside
		for _, arg := range os.Args[1:] {
			if strings.HasPrefix(arg, "-service") || strings.HasPrefix(arg, "--service") {
				continue
			}
			extraArgs = append(extraArgs, arg)
		}
		handleServiceCommand(*serviceCmd, extraArgs)
		return
	}

	if explicitFlags["lang"] {
		i18n.SetLang(*lang)
	}

	if *verFlag {
		fmt.Printf("twsnmpneo %s (commit: %s, date: %s)\n", version, commit, date)
		os.Exit(0)
	}

	if *updateFlag || *checkUpdateFlag {
		var updateArgs []string
		if *checkUpdateFlag {
			updateArgs = append(updateArgs, "-check")
		}
		handleUpdateCommand(updateArgs, *debug)
		return
	}

	if explicitFlags["rollback"] || *rollbackVersion != "" {
		target := *rollbackVersion
		if target == "" {
			target = "previous"
		}
		u := updater.New(updater.Config{
			CurrentVersion: version,
			Commit:         commit,
			Debug:          *debug,
		})
		ctx := context.Background()
		if err := u.Rollback(ctx, target); err != nil {
			fmt.Printf("Rollback failed: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Setup logger
	logLevel := slog.LevelInfo
	if *debug {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	// Handle TWSNMP FC migration mode if -import-fc is specified
	if *importFC != "" {
		fmt.Println("============================================================")
		fmt.Println("  TWSNMP FC -> TWSNMP NEO Migration & Compaction")
		fmt.Println("============================================================")
		fmt.Printf("Source Path:      %s\n", *importFC)
		fmt.Printf("Destination Path: %s\n\n", *dataDir)

		ctx := context.Background()
		report, err := importer.RunFCMigration(ctx, *importFC, *dataDir, importer.Options{
			ImportEventLogs: *importEvents,
			Force:           *forceImport,
		})
		if err != nil {
			fmt.Printf("Error during migration: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("[Database Compaction & Extraction]")
		fmt.Printf("  Source DB Size:     %.2f MB (%s)\n", float64(report.SourceDBSize)/(1024*1024), filepath.Base(report.SourceDB))
		fmt.Printf("  Target DB Size:     %.2f MB (%s)\n", float64(report.DestDBSize)/(1024*1024), filepath.Base(report.DestDB))
		if report.SizeReductionPct > 0 {
			fmt.Printf("  Size Reduced:       %.2f%%\n", report.SizeReductionPct)
		}

		fmt.Println("\n[Imported Entities]")
		fmt.Printf("  Nodes:              %d\n", report.NodesImported)
		fmt.Printf("  Lines:              %d\n", report.LinesImported)
		fmt.Printf("  Networks:           %d\n", report.NwsImported)
		fmt.Printf("  Draw Items:         %d\n", report.ItemsImported)
		fmt.Printf("  Pollings:           %d\n", report.PollsImported)
		fmt.Printf("  Users:              %d\n", report.UsersImported)
		if *importEvents {
			fmt.Printf("  Event Logs:         %d\n", report.EventsImported)
		}
		fmt.Println("  System Configs:     Migrated (mapConf, notifyConf, locConf, backImage, customIcons)")

		if len(report.FilesCopied) > 0 {
			fmt.Println("\n[Asset Files Copied]")
			for _, f := range report.FilesCopied {
				fmt.Printf("  - %s\n", f)
			}
		}

		if len(report.Warnings) > 0 {
			fmt.Println("\n[Warnings]")
			for _, w := range report.Warnings {
				fmt.Printf("  ! %s\n", w)
			}
		}

		fmt.Println("\nStatus: Migration Completed Successfully.")
		fmt.Println("============================================================")

		if !*runAfterImport {
			os.Exit(0)
		}
		fmt.Println("\nStarting TWSNMP NEO server...")
	}

	cfg := serverConfig{
		dataDir:         *dataDir,
		lang:            *lang,
		port:            *port,
		acmeURL:         *acmeURL,
		syslogUDP:       *syslogUDP,
		syslogTCP:       *syslogTCP,
		trapPort:        *trapPort,
		netflowPort:     *netflowPort,
		sflowPort:       *sflowPort,
		otelPort:        *otelPort,
		mqttPort:        *mqttPort,
		shutdownTimeout: *shutdownTimeout,
		debug:           *debug,
		tlsFlag:         *tlsFlag,
		certPath:        *certPath,
		keyPath:         *keyPath,
		acmeDomains:     *acmeDomains,
		acmeCertURL:     *acmeCertURL,
		acmeEmail:       *acmeEmail,
		acmeInsecure:    *acmeInsecure,
		acmeRenewDays:   *acmeRenewDays,
		httpPort:        *httpPort,
		hostName:        *hostName,
		ipAddr:          *ipAddr,
		explicitFlags:   explicitFlags,
	}

	// If running under Windows Service Control Manager, dispatch to service runner
	if service.IsWindowsService() {
		slog.Info("Starting TWSNMP NEO as Windows Service", "service", service.ServiceName)
		runner := &serviceRunner{
			cfg: cfg,
		}
		if err := service.Run(service.ServiceName, runner); err != nil {
			slog.Error("Service execution failed", "error", err)
			os.Exit(1)
		}
		return
	}

	// Normal standalone console execution
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := runServer(ctx, cfg); err != nil {
		slog.Error("Server error", "error", err)
		os.Exit(1)
	}
}

func runServer(ctx context.Context, cfg serverConfig) error {
	slog.Info("Starting TWSNMP NEO",
		"version", version,
		"datadir", cfg.dataDir,
		"port", cfg.port,
		"debug", cfg.debug,
	)

	// Ensure data directory exists
	if err := os.MkdirAll(cfg.dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory %s: %w", cfg.dataDir, err)
	}

	// Initialize bbolt datastore
	dbPath := filepath.Join(cfg.dataDir, "twsnmpneo.db")
	store, err := bbolt.New(dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize bbolt datastore: %w", err)
	}
	defer store.Close()

	// Initialize parquet log store
	pqDir := filepath.Join(cfg.dataDir, "logs")
	pqStore, err := parquet.New(parquet.Config{
		Dir:            pqDir,
		BufferSize:     5000,
		BufferInterval: 10 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize parquet log store: %w", err)
	}
	defer pqStore.Close()

	// Initialize MIB subsystem
	if err := mib.Init(cfg.dataDir); err != nil {
		slog.Warn("Failed to initialize MIB subsystem", "error", err)
	}

	// Initialize GeoIP subsystem
	if err := datastore.InitGeoIP(cfg.dataDir); err != nil {
		slog.Warn("Failed to initialize GeoIP database", "error", err)
	}

	// Initialize Private PKI
	pkiMgr, err := pki.New(store)
	if err != nil {
		return fmt.Errorf("failed to initialize Private PKI: %w", err)
	}
	defer pkiMgr.Stop()

	// Record system startup event log
	_ = store.AddEventLog(ctx, &datastore.EventLogEnt{
		Time:  time.Now().UnixNano(),
		Type:  "system",
		Level: "info",
		Event: fmt.Sprintf(i18n.Trans("TWSNMP NEO %s started (Web port: %d)"), version, cfg.port),
	})

	// Ensure default Ping polling exists for all nodes with IP
	existingNodes, _ := store.ListNodes(ctx)
	existingPolls, _ := store.ListPollings(ctx)
	pollMap := make(map[string]bool)
	for _, p := range existingPolls {
		pollMap[p.NodeID] = true
	}
	for _, n := range existingNodes {
		if n.IP != "" && !pollMap[n.ID] {
			pID := datastore.GenerateID()
			_ = store.SavePolling(ctx, &datastore.PollingEnt{
				ID:       pID,
				Name:     "Ping",
				NodeID:   n.ID,
				Type:     "ping",
				PollInt:  60,
				Timeout:  1,
				Retry:    1,
				LogMode:  datastore.LogModeOnChange,
				State:    "unknown",
				NextTime: time.Now().UnixNano(),
			})
			_ = store.AddEventLog(ctx, &datastore.EventLogEnt{
				Time:     time.Now().UnixNano(),
				Type:     "system",
				Level:    "info",
				NodeName: n.Name,
				NodeID:   n.ID,
				Event:    fmt.Sprintf(i18n.Trans("Start polling node %s (%s)"), n.Name, n.IP),
			})
		}
	}

	var wg sync.WaitGroup
	_ = pkiMgr.Start(ctx, &wg)

	// Initialize Polling Manager
	pollMgr := polling.NewManager(polling.Config{
		Store:        store,
		LogStore:     pqStore,
		WorkerCount:  10,
		PollInterval: 1 * time.Second,
	})
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := pollMgr.Start(ctx); err != nil {
			slog.Error("Polling manager error", "error", err)
		}
	}()

	// Start Network background port manager
	topology.StartNetworkBackend(ctx, store)

	// Start AI Anomaly Detection background service
	anomalySvc := ai.NewAnomalyService(store, pqStore)
	go anomalySvc.Start(ctx)

	// Resolve receiver ports from CLI flags and MapConf
	mapConf, _ := store.GetMapConf(ctx)

	sUDP := cfg.syslogUDP
	if !cfg.explicitFlags["syslog-udp"] && (mapConf == nil || mapConf.EnableSyslogd) {
		sUDP = 514
	}
	sTCP := cfg.syslogTCP
	if !cfg.explicitFlags["syslog-tcp"] && (mapConf == nil || mapConf.EnableSyslogd) {
		sTCP = 514
	}
	tPort := cfg.trapPort
	if !cfg.explicitFlags["trap-port"] && (mapConf == nil || mapConf.EnableTrapd) {
		tPort = 162
	}
	nfPort := cfg.netflowPort
	if !cfg.explicitFlags["netflow-port"] && (mapConf == nil || mapConf.EnableNetflowd) {
		nfPort = 2055
	}
	sfPort := cfg.sflowPort
	if !cfg.explicitFlags["sflow-port"] && (mapConf == nil || mapConf.EnableSFlowd) {
		sfPort = 6343
	}
	oPort := cfg.otelPort
	if !cfg.explicitFlags["otel-port"] && (mapConf == nil || mapConf.EnableOTel) {
		oPort = 4318
	}
	mPort := cfg.mqttPort
	if !cfg.explicitFlags["mqtt-port"] && (mapConf == nil || mapConf.EnableMqtt) {
		mPort = 1883
	}

	slog.Info("Protocol receiver ports configured",
		"syslog_udp", sUDP,
		"syslog_tcp", sTCP,
		"trap", tPort,
		"netflow", nfPort,
		"sflow", sfPort,
		"otel", oPort,
		"mqtt", mPort,
	)

	mqttToSyslog := false
	enableArpWatch := true
	arpWatchRange := ""
	arpTimeout := 60
	otelRetention := 24
	otelFrom := ""
	if mapConf != nil {
		mqttToSyslog = mapConf.MqttToSyslog
		enableArpWatch = mapConf.EnableArpWatch
		arpWatchRange = mapConf.ArpWatchRange
		arpTimeout = mapConf.ArpTimeout
		if mapConf.OTelRetention > 0 {
			otelRetention = mapConf.OTelRetention
		}
		otelFrom = mapConf.OTelFrom
	}

	// Initialize Protocol Receivers
	recvMgr := receiver.NewManager(receiver.Config{
		Store:          store,
		LogStore:       pqStore,
		SyslogUDP:      sUDP,
		SyslogTCP:      sTCP,
		TrapPort:       tPort,
		NetFlowPort:    nfPort,
		SFlowPort:      sfPort,
		OTelPort:       oPort,
		OTelRetention:  otelRetention,
		OTelFrom:       otelFrom,
		MQTTPort:       mPort,
		MqttToSyslog:   mqttToSyslog,
		EnableArpWatch: enableArpWatch,
		ArpWatchRange:  arpWatchRange,
		ArpTimeout:     arpTimeout,
	})
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := recvMgr.Start(ctx); err != nil {
			slog.Error("Receiver manager error", "error", err)
		}
	}()

	// Initialize System Resource Monitor
	sysMon := monitor.New(monitor.Config{
		DataDir:  cfg.dataDir,
		Store:    store,
		Interval: 1 * time.Minute,
	})
	sysMon.Start(ctx)

	// Start Notify Manager
	notify.Start(ctx, store, sysMon)

	// Initialize MCP Server (configured via MapConf)
	mcpServer := ai.NewMCPServer(ai.MCPConfig{
		Store:    store,
		LogStore: pqStore,
		Monitor:  sysMon,
		Version:  version,
	})

	// Receiver info map for UI
	receiversInfo := map[string]any{
		"syslog":  map[string]any{"port": fmt.Sprintf("UDP :%d / TCP :%d", sUDP, sTCP), "status": "running"},
		"trap":    map[string]any{"port": fmt.Sprintf("UDP :%d (v1/v2c/v3)", tPort), "status": "running"},
		"netflow": map[string]any{"port": fmt.Sprintf("UDP :%d", nfPort), "status": "running"},
		"sflow":   map[string]any{"port": fmt.Sprintf("UDP :%d", sfPort), "status": "running"},
		"otel":    map[string]any{"port": fmt.Sprintf("HTTP :%d (OTLP)", oPort), "status": "running"},
		"mqtt":    map[string]any{"port": fmt.Sprintf("TCP :%d", mPort), "status": "running"},
		"mcp":     map[string]any{"port": "Streamable HTTP /api/mcp", "status": "running"},
		"arp":     map[string]any{"status": "running", "range": arpWatchRange},
	}

	// Resolve TLS and ACME server certificate
	defaultCert := filepath.Join(cfg.dataDir, "cert.pem")
	defaultKey := filepath.Join(cfg.dataDir, "key.pem")
	hasCustomCert := cfg.certPath != "" && cfg.keyPath != "" && fileExists(cfg.certPath) && fileExists(cfg.keyPath)
	hasDefaultCert := fileExists(defaultCert) && fileExists(defaultKey)
	useTLS := cfg.tlsFlag || cfg.acmeDomains != "" || hasCustomCert || hasDefaultCert

	certMgr := certmgr.New(certmgr.Config{
		DataDir:       cfg.dataDir,
		CertPath:      cfg.certPath,
		KeyPath:       cfg.keyPath,
		Host:          cfg.hostName,
		IP:            cfg.ipAddr,
		ACMEDomains:   cfg.acmeDomains,
		ACMEURL:       cfg.acmeCertURL,
		ACMEEmail:     cfg.acmeEmail,
		ACMEInsecure:  cfg.acmeInsecure,
		ACMERenewDays: cfg.acmeRenewDays,
		HTTPPort:      cfg.httpPort,
		TLSPort:       0,
		EnableTLS:     useTLS,
	}, store)

	if useTLS {
		if err := certMgr.LoadOrResolve(ctx); err != nil {
			return fmt.Errorf("failed to initialize TLS server certificate: %w", err)
		}
	}

	// Initialize Self-Updater
	upd := updater.New(updater.Config{
		CurrentVersion: version,
		Commit:         commit,
		Debug:          cfg.debug,
		OnShutdown: func() {
			if store != nil {
				_ = store.Close()
			}
			if pqStore != nil {
				_ = pqStore.Close()
			}
		},
	})

	// Initialize Web/API server
	server, err := api.NewServer(api.Config{
		Port:           cfg.port,
		Debug:          cfg.debug,
		Version:        version,
		Commit:         commit,
		DataDir:        cfg.dataDir,
		Store:          store,
		LogStore:       pqStore,
		MCPServer:      mcpServer,
		PKI:            pkiMgr,
		ACMEBaseURL:    cfg.acmeURL,
		ArpManager:     recvMgr,
		Monitor:        sysMon,
		PollingManager: pollMgr,
		Receivers:      receiversInfo,
		CertManager:    certMgr,
		Updater:        upd,
		HTTPPort:       cfg.httpPort,
		UseTLS:         useTLS,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize API server: %w", err)
	}

	// Start server in background
	serverErr := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := server.Start(ctx); err != nil {
			serverErr <- err
		}
	}()

	// Trigger initial ACME request if needed once listeners are active
	if useTLS {
		certMgr.InitialACME(ctx)
		certMgr.StartAutoRenewal(ctx, 12*time.Hour)
	}

	select {
	case err := <-serverErr:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		slog.Info("Shutdown signal received")
	}

	slog.Info("Shutting down TWSNMP NEO gracefully...")
	timeout := time.Duration(cfg.shutdownTimeout) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	shutdownDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(shutdownDone)
	}()

	select {
	case <-shutdownDone:
		slog.Info("All services stopped gracefully.")
	case <-time.After(timeout):
		slog.Warn("Graceful shutdown timed out, forcing exit.", "timeout", timeout)
	}

	slog.Info("TWSNMP NEO stopped.")
	return nil
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
