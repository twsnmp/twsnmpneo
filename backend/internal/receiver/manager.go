package receiver

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	"github.com/twsnmp/twsnmpneo/backend/internal/logreport"
)

// Config holds options for all receivers managed together.
type Config struct {
	Store          datastore.DataStore
	LogStore       *parquet.Store
	Reporter       logreport.Reporter
	SyslogUDP      int
	SyslogTCP      int
	TrapPort       int
	NetFlowPort    int
	SFlowPort      int
	OTelPort       int
	OTelRetention  int
	OTelFrom       string
	MQTTPort       int
	MqttToSyslog   bool
	EnableArpWatch bool
	ArpWatchRange  string
	ArpTimeout     int
}

// Manager controls the lifecycle of all embedded protocol receivers.
type Manager struct {
	cfg      Config
	reporter logreport.Reporter
	syslog   *SyslogServer
	trap     *TrapServer
	netflow  *NetFlowServer
	sflow    *SFlowServer
	otel     *OTelServer
	mqtt     *MQTTServer
	arpWatch *ArpWatchServer
}

// NewManager creates an instance of the receiver manager.
func NewManager(cfg Config) *Manager {
	rep := cfg.Reporter
	if rep == nil && cfg.Store != nil {
		rep = logreport.NewEngine(cfg.Store)
	}

	return &Manager{
		cfg:      cfg,
		reporter: rep,
		syslog: NewSyslogServer(SyslogConfig{
			UDPPort:  cfg.SyslogUDP,
			TCPPort:  cfg.SyslogTCP,
			Store:    cfg.Store,
			LogStore: cfg.LogStore,
			Reporter: rep,
		}),
		trap: NewTrapServer(TrapConfig{
			Port:     cfg.TrapPort,
			Store:    cfg.Store,
			LogStore: cfg.LogStore,
			Reporter: rep,
		}),
		netflow: NewNetFlowServer(NetFlowConfig{
			Port:     cfg.NetFlowPort,
			Store:    cfg.Store,
			LogStore: cfg.LogStore,
			Reporter: rep,
		}),
		sflow: NewSFlowServer(SFlowConfig{
			Port:     cfg.SFlowPort,
			Store:    cfg.Store,
			LogStore: cfg.LogStore,
			Reporter: rep,
		}),
		otel: NewOTelServer(OTelConfig{
			Port:      cfg.OTelPort,
			Store:     cfg.Store,
			LogStore:  cfg.LogStore,
			Retention: cfg.OTelRetention,
			From:      cfg.OTelFrom,
		}),
		mqtt: NewMQTTServer(MQTTConfig{
			Port:         cfg.MQTTPort,
			Store:        cfg.Store,
			LogStore:     cfg.LogStore,
			MqttToSyslog: cfg.MqttToSyslog,
		}),
		arpWatch: NewArpWatchServer(ArpWatchConfig{
			Store:    cfg.Store,
			LogStore: cfg.LogStore,
			Enabled:  cfg.EnableArpWatch,
			Range:    cfg.ArpWatchRange,
			Timeout:  cfg.ArpTimeout,
		}),
	}
}

// Start launches all enabled receivers and waits until ctx is canceled.
func (m *Manager) Start(ctx context.Context) error {
	slog.Info("Starting Protocol Receivers...")

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = m.syslog.Start(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = m.trap.Start(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = m.netflow.Start(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = m.sflow.Start(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = m.otel.Start(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = m.mqtt.Start(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = m.arpWatch.Start(ctx)
	}()

	if m.reporter != nil {
		_ = m.reporter.Start(ctx)
	}

	// Periodic Parquet log rotation & datastore retention cleaner
	go m.startRetentionCleaner(ctx)

	// Periodic sensor state & rate evaluator (every 1 minute)
	go m.startSensorEvaluator(ctx)

	<-ctx.Done()
	slog.Info("Stopping Protocol Receivers...")
	if m.reporter != nil {
		m.reporter.Stop()
	}
	wg.Wait()
	return nil
}

func (m *Manager) startSensorEvaluator(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	if m.cfg.Store != nil {
		m.cfg.Store.EvaluateSensorStates(ctx)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if m.cfg.Store != nil {
				m.cfg.Store.EvaluateSensorStates(ctx)
			}
		}
	}
}

func (m *Manager) startRetentionCleaner(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	m.cleanOldData(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.cleanOldData(ctx)
		}
	}
}

func (m *Manager) cleanOldData(ctx context.Context) {
	logDays := 14
	if m.cfg.Store != nil {
		if conf, err := m.cfg.Store.GetMapConf(ctx); err == nil && conf != nil && conf.LogDays > 0 {
			logDays = conf.LogDays
		}
	}

	if m.cfg.LogStore != nil {
		if rotated, err := m.cfg.LogStore.Rotate(logDays); err == nil && rotated > 0 {
			slog.Info("Rotated old parquet log files", "deletedFiles", rotated, "retentionDays", logDays)
		}
	}

	if m.cfg.Store != nil {
		_ = m.cfg.Store.CleanOldMqttStats(ctx, logDays)
	}
}

// GetArpTable returns all known ARP table entries from the ARP watch engine.
func (m *Manager) GetArpTable() []*datastore.ArpEnt {
	if m.arpWatch != nil {
		return m.arpWatch.GetArpTable()
	}
	return nil
}

// DeleteArpEntries removes given IPs from the in-memory ARP table.
func (m *Manager) DeleteArpEntries(ips []string) {
	if m.arpWatch != nil {
		m.arpWatch.DeleteEntries(ips)
	}
}

// ResetArpTable clears the in-memory ARP table.
func (m *Manager) ResetArpTable() {
	if m.arpWatch != nil {
		m.arpWatch.ResetTable()
	}
}

// GetReceiverStatus returns a map describing ports and running statuses of all managed receivers.
func (m *Manager) GetReceiverStatus(mcpEnabled bool) map[string]any {
	result := make(map[string]any)

	// Syslog
	if m.cfg.SyslogUDP > 0 && m.cfg.SyslogTCP > 0 {
		result["syslog"] = map[string]any{
			"port":   fmt.Sprintf("UDP :%d / TCP :%d", m.cfg.SyslogUDP, m.cfg.SyslogTCP),
			"status": "running",
		}
	} else if m.cfg.SyslogUDP > 0 {
		result["syslog"] = map[string]any{
			"port":   fmt.Sprintf("UDP :%d", m.cfg.SyslogUDP),
			"status": "running",
		}
	} else if m.cfg.SyslogTCP > 0 {
		result["syslog"] = map[string]any{
			"port":   fmt.Sprintf("TCP :%d", m.cfg.SyslogTCP),
			"status": "running",
		}
	} else {
		result["syslog"] = map[string]any{
			"port":   "-",
			"status": "stopped",
		}
	}

	// SNMP Trap
	if m.cfg.TrapPort > 0 {
		result["trap"] = map[string]any{
			"port":   fmt.Sprintf("UDP :%d (v1/v2c/v3)", m.cfg.TrapPort),
			"status": "running",
		}
	} else {
		result["trap"] = map[string]any{
			"port":   "-",
			"status": "stopped",
		}
	}

	// NetFlow
	if m.cfg.NetFlowPort > 0 {
		result["netflow"] = map[string]any{
			"port":   fmt.Sprintf("UDP :%d", m.cfg.NetFlowPort),
			"status": "running",
		}
	} else {
		result["netflow"] = map[string]any{
			"port":   "-",
			"status": "stopped",
		}
	}

	// sFlow
	if m.cfg.SFlowPort > 0 {
		result["sflow"] = map[string]any{
			"port":   fmt.Sprintf("UDP :%d", m.cfg.SFlowPort),
			"status": "running",
		}
	} else {
		result["sflow"] = map[string]any{
			"port":   "-",
			"status": "stopped",
		}
	}

	// OpenTelemetry
	if m.cfg.OTelPort > 0 {
		result["otel"] = map[string]any{
			"port":   fmt.Sprintf("HTTP :%d (OTLP)", m.cfg.OTelPort),
			"status": "running",
		}
	} else {
		result["otel"] = map[string]any{
			"port":   "-",
			"status": "stopped",
		}
	}

	// MQTT Broker
	if m.cfg.MQTTPort > 0 {
		result["mqtt"] = map[string]any{
			"port":   fmt.Sprintf("TCP :%d", m.cfg.MQTTPort),
			"status": "running",
		}
	} else {
		result["mqtt"] = map[string]any{
			"port":   "-",
			"status": "stopped",
		}
	}

	// MCP Server
	if mcpEnabled {
		result["mcp"] = map[string]any{
			"port":   "Streamable HTTP /api/mcp",
			"status": "running",
		}
	} else {
		result["mcp"] = map[string]any{
			"port":   "-",
			"status": "disabled",
		}
	}

	// ARP Watch
	if m.cfg.EnableArpWatch {
		result["arp"] = map[string]any{
			"status": "running",
			"range":  m.cfg.ArpWatchRange,
		}
	} else {
		result["arp"] = map[string]any{
			"status": "disabled",
			"range":  "",
		}
	}

	return result
}

