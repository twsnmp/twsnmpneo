package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/ai"
	"github.com/twsnmp/twsnmpneo/backend/internal/api"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/bbolt"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	"github.com/twsnmp/twsnmpneo/backend/internal/mib"
	"github.com/twsnmp/twsnmpneo/backend/internal/pki"
	"github.com/twsnmp/twsnmpneo/backend/internal/polling"
	"github.com/twsnmp/twsnmpneo/backend/internal/receiver"
)

var (
	version = "v0.1.0-dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var (
		dataDir     = flag.String("datadir", "./data", "Directory to store database and log files")
		port        = flag.Int("port", 8080, "Web interface port")
		syslogUDP   = flag.Int("syslog-udp", 0, "Syslog UDP port (0 to disable or default)")
		syslogTCP   = flag.Int("syslog-tcp", 0, "Syslog TCP port (0 to disable)")
		trapPort    = flag.Int("trap-port", 0, "SNMP TRAP UDP port (0 to disable or default 162)")
		netflowPort = flag.Int("netflow-port", 0, "NetFlow UDP port (0 to disable or default 2055)")
		sflowPort   = flag.Int("sflow-port", 0, "sFlow UDP port (0 to disable or default 6343)")
		otelPort    = flag.Int("otel-port", 0, "OpenTelemetry OTLP HTTP port (0 to disable or default 4318)")
		mqttPort    = flag.Int("mqtt-port", 0, "MQTT broker port (0 to disable or default 1883)")
		debug       = flag.Bool("debug", false, "Enable debug logging")
		verFlag     = flag.Bool("version", false, "Show version and exit")
	)
	flag.Parse()

	explicitFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		explicitFlags[f.Name] = true
	})

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
	pkiDir := filepath.Join(*dataDir, "pki")
	pkiMgr, err := pki.New(pki.Config{
		DataDir: pkiDir,
	})
	if err != nil {
		slog.Error("Failed to initialize Private PKI", "error", err)
		os.Exit(1)
	}
	_ = pkiMgr

	// Setup context with graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Record system startup event log
	_ = store.AddEventLog(ctx, &datastore.EventLogEnt{
		Time:  time.Now().UnixNano(),
		Type:  "system",
		Level: "info",
		Event: fmt.Sprintf("TWSNMP NEO %s サービスを起動しました (Webポート: %d)", version, *port),
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
				Event:    fmt.Sprintf("ノード %s (%s) の Ping ポーリングを開始しました", n.Name, n.IP),
			})
		}
	}

	// Initialize Polling Manager
	pollMgr := polling.NewManager(polling.Config{
		Store:        store,
		LogStore:     pqStore,
		WorkerCount:  10,
		PollInterval: 1 * time.Second,
	})
	go func() {
		if err := pollMgr.Start(ctx); err != nil {
			slog.Error("Polling manager error", "error", err)
		}
	}()

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

	mqttToSyslog := false
	enableArpWatch := true
	arpWatchRange := ""
	arpTimeout := 60
	if mapConf != nil {
		mqttToSyslog = mapConf.MqttToSyslog
		enableArpWatch = mapConf.EnableArpWatch
		arpWatchRange = mapConf.ArpWatchRange
		arpTimeout = mapConf.ArpTimeout
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
		MQTTPort:       mPort,
		MqttToSyslog:   mqttToSyslog,
		EnableArpWatch: enableArpWatch,
		ArpWatchRange:  arpWatchRange,
		ArpTimeout:     arpTimeout,
	})
	go func() {
		if err := recvMgr.Start(ctx); err != nil {
			slog.Error("Receiver manager error", "error", err)
		}
	}()

	// Initialize MCP Server
	mcpServer := ai.NewMCPServer(ai.MCPConfig{
		Store:    store,
		LogStore: pqStore,
		Version:  version,
	})

	// Initialize Web/API server
	server, err := api.NewServer(api.Config{
		Port:      *port,
		Debug:     *debug,
		Version:   version,
		DataDir:   *dataDir,
		Store:      store,
		LogStore:   pqStore,
		MCPServer:  mcpServer,
		ArpManager: recvMgr,
	})
	if err != nil {
		slog.Error("Failed to initialize API server", "error", err)
		os.Exit(1)
	}

	// Start server in background
	serverErr := make(chan error, 1)
	go func() {
		if err := server.Start(ctx); err != nil {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		slog.Error("Server error", "error", err)
		os.Exit(1)
	case <-ctx.Done():
		slog.Info("Shutdown signal received")
	}

	slog.Info("Shutting down TWSNMP NEO gracefully...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	<-shutdownCtx.Done()
	slog.Info("TWSNMP NEO stopped.")
}
