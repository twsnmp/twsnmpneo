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
	"github.com/twsnmp/twsnmpneo/backend/internal/topology"
)

var (
	version = "v0.1.0"
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

func main() {
	commit = resolveCommit()
	var (
		dataDir     = flag.String("datadir", "./data", "Directory to store database and log files")
		lang        = flag.String("lang", "en", "Language (en or ja)")
		port        = flag.Int("port", 8080, "Web interface port")
		acmeURL     = flag.String("acme-url", "", "Override the PKI ACME HTTPS base URL and enable the listener")
		syslogUDP   = flag.Int("syslog-udp", 0, "Syslog UDP port (0 to disable or default)")
		syslogTCP   = flag.Int("syslog-tcp", 0, "Syslog TCP port (0 to disable)")
		trapPort    = flag.Int("trap-port", 0, "SNMP TRAP UDP port (0 to disable or default 162)")
		netflowPort = flag.Int("netflow-port", 0, "NetFlow UDP port (0 to disable or default 2055)")
		sflowPort   = flag.Int("sflow-port", 0, "sFlow UDP port (0 to disable or default 6343)")
		otelPort    = flag.Int("otel-port", 0, "OpenTelemetry OTLP HTTP port (0 to disable or default 4318)")
		mqttPort        = flag.Int("mqtt-port", 0, "MQTT broker port (0 to disable or default 1883)")
		shutdownTimeout = flag.Int("shutdown-timeout", 5, "Graceful shutdown fallback timeout in seconds")
		debug           = flag.Bool("debug", false, "Enable debug logging")
		verFlag         = flag.Bool("version", false, "Show version and exit")

		// TLS / ACME server certificate flags
		tlsFlag       = flag.Bool("tls", false, "Enable TLS (HTTPS) for Web interface")
		certPath      = flag.String("cert", "", "Path to server certificate file (PEM)")
		keyPath       = flag.String("key", "", "Path to server private key file (PEM)")
		acmeDomains   = flag.String("acme-domain", "", "Domains / SANs for ACME auto-update server cert")
		acmeCertURL   = flag.String("acme-cert-url", "", "ACME Directory URL for server cert (default Let's Encrypt)")
		acmeEmail     = flag.String("acme-email", "", "Contact email for ACME account registration")
		acmeInsecure  = flag.Bool("acme-insecure", false, "Skip server certificate verification for ACME")
		acmeRenewDays = flag.Int("acme-renew-days", 30, "Days before certificate expiry to trigger ACME renewal")
		httpPort      = flag.Int("http-port", 80, "HTTP port for ACME challenge & HTTPS redirect (0 to disable)")
		hostName      = flag.String("host", "", "Host name for self-signed certificate SAN")
		ipAddr        = flag.String("ip", "", "IP address for self-signed certificate SAN")

		// Migration flags
		importFC       = flag.String("import-fc", "", "Path to legacy TWSNMP FC data directory or twsnmpfc.db to import and compact")
		importEvents   = flag.Bool("import-events", true, "Import event logs during TWSNMP FC migration")
		forceImport    = flag.Bool("force", false, "Force overwrite of target database during migration")
		runAfterImport = flag.Bool("run-after-import", false, "Start TWSNMP NEO server after completing migration")
	)

	// Environment variable overrides (TWSNMP_<NAME> and legacy TWSNMPFC_<NAME>)
	flag.VisitAll(func(f *flag.Flag) {
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

	flag.Parse()
	explicitFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		explicitFlags[f.Name] = true
	})
	if explicitFlags["lang"] {
		i18n.SetLang(*lang)
	}

	if *verFlag {
		fmt.Printf("twsnmpneo %s (commit: %s, date: %s)\n", version, commit, date)
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

	slog.Info("Starting TWSNMP NEO",
		"version", version,
		"datadir", *dataDir,
		"port", *port,
		"debug", *debug,
	)

	// Ensure data directory exists
	if err := os.MkdirAll(*dataDir, 0755); err != nil {
		slog.Error("Failed to create data directory", "path", *dataDir, "error", err)
		os.Exit(1)
	}

	// Initialize bbolt datastore
	dbPath := filepath.Join(*dataDir, "twsnmpneo.db")
	store, err := bbolt.New(dbPath)
	if err != nil {
		slog.Error("Failed to initialize bbolt datastore", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	// Initialize parquet log store
	pqDir := filepath.Join(*dataDir, "logs")
	pqStore, err := parquet.New(parquet.Config{
		Dir:            pqDir,
		BufferSize:     5000,
		BufferInterval: 10 * time.Second,
	})
	if err != nil {
		slog.Error("Failed to initialize parquet log store", "error", err)
		os.Exit(1)
	}
	defer pqStore.Close()

	// Initialize MIB subsystem
	if err := mib.Init(*dataDir); err != nil {
		slog.Warn("Failed to initialize MIB subsystem", "error", err)
	}

	// Initialize GeoIP subsystem
	if err := datastore.InitGeoIP(*dataDir); err != nil {
		slog.Warn("Failed to initialize GeoIP database", "error", err)
	}

	// Initialize Private PKI
	pkiMgr, err := pki.New(store)
	if err != nil {
		slog.Error("Failed to initialize Private PKI", "error", err)
		os.Exit(1)
	}
	defer pkiMgr.Stop()

	// Setup context with graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Record system startup event log
	_ = store.AddEventLog(ctx, &datastore.EventLogEnt{
		Time:  time.Now().UnixNano(),
		Type:  "system",
		Level: "info",
		Event: fmt.Sprintf(i18n.Trans("TWSNMP NEO %s started (Web port: %d)"), version, *port),
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

	sUDP := *syslogUDP
	if !explicitFlags["syslog-udp"] && (mapConf == nil || mapConf.EnableSyslogd) {
		sUDP = 514
	}
	sTCP := *syslogTCP
	if !explicitFlags["syslog-tcp"] && (mapConf == nil || mapConf.EnableSyslogd) {
		sTCP = 514
	}
	tPort := *trapPort
	if !explicitFlags["trap-port"] && (mapConf == nil || mapConf.EnableTrapd) {
		tPort = 162
	}
	nfPort := *netflowPort
	if !explicitFlags["netflow-port"] && (mapConf == nil || mapConf.EnableNetflowd) {
		nfPort = 2055
	}
	sfPort := *sflowPort
	if !explicitFlags["sflow-port"] && (mapConf == nil || mapConf.EnableSFlowd) {
		sfPort = 6343
	}
	oPort := *otelPort
	if !explicitFlags["otel-port"] && (mapConf == nil || mapConf.EnableOTel) {
		oPort = 4318
	}
	mPort := *mqttPort
	if !explicitFlags["mqtt-port"] && (mapConf == nil || mapConf.EnableMqtt) {
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
		DataDir:  *dataDir,
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
	defaultCert := filepath.Join(*dataDir, "cert.pem")
	defaultKey := filepath.Join(*dataDir, "key.pem")
	hasCustomCert := *certPath != "" && *keyPath != "" && fileExists(*certPath) && fileExists(*keyPath)
	hasDefaultCert := fileExists(defaultCert) && fileExists(defaultKey)
	useTLS := *tlsFlag || *acmeDomains != "" || hasCustomCert || hasDefaultCert

	certMgr := certmgr.New(certmgr.Config{
		DataDir:       *dataDir,
		CertPath:      *certPath,
		KeyPath:       *keyPath,
		Host:          *hostName,
		IP:            *ipAddr,
		ACMEDomains:   *acmeDomains,
		ACMEURL:       *acmeCertURL,
		ACMEEmail:     *acmeEmail,
		ACMEInsecure:  *acmeInsecure,
		ACMERenewDays: *acmeRenewDays,
		HTTPPort:      *httpPort,
		TLSPort:       0,
		EnableTLS:     useTLS,
	}, store)

	if useTLS {
		if err := certMgr.LoadOrResolve(ctx); err != nil {
			slog.Error("Failed to initialize TLS server certificate", "error", err)
			os.Exit(1)
		}
	}

	// Initialize Web/API server
	server, err := api.NewServer(api.Config{
		Port:           *port,
		Debug:          *debug,
		Version:        version,
		Commit:         commit,
		DataDir:        *dataDir,
		Store:          store,
		LogStore:       pqStore,
		MCPServer:      mcpServer,
		PKI:            pkiMgr,
		ACMEBaseURL:    *acmeURL,
		ArpManager:     recvMgr,
		Monitor:        sysMon,
		PollingManager: pollMgr,
		Receivers:      receiversInfo,
		CertManager:    certMgr,
		HTTPPort:       *httpPort,
		UseTLS:         useTLS,
	})
	if err != nil {
		slog.Error("Failed to initialize API server", "error", err)
		os.Exit(1)
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
		slog.Error("Server error", "error", err)
		os.Exit(1)
	case <-ctx.Done():
		slog.Info("Shutdown signal received")
	}

	slog.Info("Shutting down TWSNMP NEO gracefully...")
	timeout := time.Duration(*shutdownTimeout) * time.Second
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
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

