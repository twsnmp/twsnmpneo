package receiver

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	"github.com/twsnmp/twsnmpneo/backend/internal/i18n"
	"github.com/twsnmp/twsnmpneo/backend/internal/logreport"
)

// Config holds options for all receivers managed together.
type Config struct {
	Store              datastore.DataStore
	LogStore           *parquet.Store
	Reporter           logreport.Reporter
	SyslogUDP          int
	SyslogTCP          int
	TrapPort           int
	NetFlowPort        int
	SFlowPort          int
	OTelPort           int
	OTelRetention      int
	OTelFrom           string
	MQTTPort           int
	MqttToSyslog       bool
	EnableSyslogd      bool
	EnableTrapd        bool
	EnableNetflowd     bool
	EnableSFlowd       bool
	EnableArpWatch     bool
	EnableOTel         bool
	EnableMqtt         bool
	HasExplicitEnables bool
	ArpWatchRange      string
	ArpTimeout         int
}

type receiverRunner struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// Manager controls the lifecycle of all embedded protocol receivers.
type Manager struct {
	cfg             Config
	reporter        logreport.Reporter
	syslog          *SyslogServer
	trap            *TrapServer
	netflow         *NetFlowServer
	sflow           *SFlowServer
	otel            *OTelServer
	mqtt            *MQTTServer
	arpWatch        *ArpWatchServer

	mu              sync.Mutex
	rootCtx         context.Context
	baseSyslogUDP   int
	baseSyslogTCP   int
	baseTrapPort    int
	baseNetFlowPort int
	baseSFlowPort   int
	baseOTelPort    int
	baseMQTTPort    int

	syslogRunner  *receiverRunner
	trapRunner    *receiverRunner
	netflowRunner *receiverRunner
	sflowRunner   *receiverRunner
	otelRunner    *receiverRunner
	mqttRunner    *receiverRunner
	arpRunner     *receiverRunner
}

// NewManager creates an instance of the receiver manager.
func NewManager(cfg Config) *Manager {
	rep := cfg.Reporter
	if rep == nil && cfg.Store != nil {
		rep = logreport.NewEngine(cfg.Store)
	}

	baseSUDP := cfg.SyslogUDP
	baseSTCP := cfg.SyslogTCP
	if baseSUDP <= 0 && baseSTCP <= 0 && (cfg.HasExplicitEnables || cfg.EnableSyslogd) {
		baseSUDP = 514
		baseSTCP = 514
	}
	baseTPort := cfg.TrapPort
	if baseTPort <= 0 && (cfg.HasExplicitEnables || cfg.EnableTrapd) {
		baseTPort = 162
	}
	baseNFPort := cfg.NetFlowPort
	if baseNFPort <= 0 && (cfg.HasExplicitEnables || cfg.EnableNetflowd) {
		baseNFPort = 2055
	}
	baseSFPort := cfg.SFlowPort
	if baseSFPort <= 0 && (cfg.HasExplicitEnables || cfg.EnableSFlowd) {
		baseSFPort = 6343
	}
	baseOPort := cfg.OTelPort
	if baseOPort <= 0 && (cfg.HasExplicitEnables || cfg.EnableOTel) {
		baseOPort = 4318
	}
	baseMPort := cfg.MQTTPort
	if baseMPort <= 0 && (cfg.HasExplicitEnables || cfg.EnableMqtt) {
		baseMPort = 1883
	}

	// Backward compatibility: if HasExplicitEnables is false, infer enable from port > 0
	if !cfg.HasExplicitEnables {
		if cfg.SyslogUDP > 0 || cfg.SyslogTCP > 0 {
			cfg.EnableSyslogd = true
			if cfg.SyslogUDP > 0 {
				baseSUDP = cfg.SyslogUDP
			}
			if cfg.SyslogTCP > 0 {
				baseSTCP = cfg.SyslogTCP
			}
		}
		if cfg.TrapPort > 0 {
			cfg.EnableTrapd = true
			baseTPort = cfg.TrapPort
		}
		if cfg.NetFlowPort > 0 {
			cfg.EnableNetflowd = true
			baseNFPort = cfg.NetFlowPort
		}
		if cfg.SFlowPort > 0 {
			cfg.EnableSFlowd = true
			baseSFPort = cfg.SFlowPort
		}
		if cfg.OTelPort > 0 {
			cfg.EnableOTel = true
			baseOPort = cfg.OTelPort
		}
		if cfg.MQTTPort > 0 {
			cfg.EnableMqtt = true
			baseMPort = cfg.MQTTPort
		}
	}

	arpWatch := NewArpWatchServer(ArpWatchConfig{
		Store:    cfg.Store,
		LogStore: cfg.LogStore,
		Enabled:  cfg.EnableArpWatch,
		Range:    cfg.ArpWatchRange,
		Timeout:  cfg.ArpTimeout,
	})

	return &Manager{
		cfg:             cfg,
		reporter:        rep,
		arpWatch:        arpWatch,
		baseSyslogUDP:   baseSUDP,
		baseSyslogTCP:   baseSTCP,
		baseTrapPort:    baseTPort,
		baseNetFlowPort: baseNFPort,
		baseSFlowPort:   baseSFPort,
		baseOTelPort:    baseOPort,
		baseMQTTPort:    baseMPort,
	}
}

// Start launches all enabled receivers and blocks until ctx is canceled.
func (m *Manager) Start(ctx context.Context) error {
	slog.Info("Starting Protocol Receivers...")

	m.mu.Lock()
	m.rootCtx = ctx

	if m.reporter != nil {
		_ = m.reporter.Start(ctx)
	}

	if m.cfg.EnableSyslogd {
		m.startSyslogLocked()
	}
	if m.cfg.EnableTrapd {
		m.startTrapLocked()
	}
	if m.cfg.EnableNetflowd {
		m.startNetFlowLocked()
	}
	if m.cfg.EnableSFlowd {
		m.startSFlowLocked()
	}
	if m.cfg.EnableOTel {
		m.startOTelLocked()
	}
	if m.cfg.EnableMqtt {
		m.startMQTTLocked()
	}
	if m.cfg.EnableArpWatch {
		m.startArpWatchLocked()
	}
	m.mu.Unlock()

	// Periodic Parquet log rotation & datastore retention cleaner
	go m.startRetentionCleaner(ctx)

	// Periodic sensor state & rate evaluator (every 1 minute)
	go m.startSensorEvaluator(ctx)

	<-ctx.Done()
	slog.Info("Stopping Protocol Receivers...")
	if m.reporter != nil {
		m.reporter.Stop()
	}

	m.mu.Lock()
	m.stopSyslogLocked()
	m.stopTrapLocked()
	m.stopNetFlowLocked()
	m.stopSFlowLocked()
	m.stopOTelLocked()
	m.stopMQTTLocked()
	m.stopArpWatchLocked()
	m.mu.Unlock()

	return nil
}

// UpdateConf synchronizes the active running receivers with the new configuration.
func (m *Manager) UpdateConf(conf *datastore.MapConfEnt) {
	if conf == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cfg.EnableSyslogd = conf.EnableSyslogd
	m.cfg.EnableTrapd = conf.EnableTrapd
	m.cfg.EnableNetflowd = conf.EnableNetflowd
	m.cfg.EnableSFlowd = conf.EnableSFlowd
	m.cfg.EnableArpWatch = conf.EnableArpWatch
	m.cfg.EnableOTel = conf.EnableOTel
	m.cfg.EnableMqtt = conf.EnableMqtt
	m.cfg.ArpWatchRange = conf.ArpWatchRange
	m.cfg.ArpTimeout = conf.ArpTimeout
	m.cfg.MqttToSyslog = conf.MqttToSyslog
	if conf.OTelRetention > 0 {
		m.cfg.OTelRetention = conf.OTelRetention
	}
	m.cfg.OTelFrom = conf.OTelFrom

	if m.rootCtx == nil || m.rootCtx.Err() != nil {
		return
	}

	// Syslog
	if conf.EnableSyslogd {
		if m.syslogRunner == nil {
			m.startSyslogLocked()
		}
	} else {
		if m.syslogRunner != nil {
			m.stopSyslogLocked()
		}
	}

	// Trap
	if conf.EnableTrapd {
		if m.trapRunner == nil {
			m.startTrapLocked()
		}
	} else {
		if m.trapRunner != nil {
			m.stopTrapLocked()
		}
	}

	// NetFlow
	if conf.EnableNetflowd {
		if m.netflowRunner == nil {
			m.startNetFlowLocked()
		}
	} else {
		if m.netflowRunner != nil {
			m.stopNetFlowLocked()
		}
	}

	// sFlow
	if conf.EnableSFlowd {
		if m.sflowRunner == nil {
			m.startSFlowLocked()
		}
	} else {
		if m.sflowRunner != nil {
			m.stopSFlowLocked()
		}
	}

	// OTel
	if conf.EnableOTel {
		if m.otelRunner == nil {
			m.startOTelLocked()
		}
	} else {
		if m.otelRunner != nil {
			m.stopOTelLocked()
		}
	}

	// MQTT
	if conf.EnableMqtt {
		if m.mqttRunner == nil {
			m.startMQTTLocked()
		} else if m.mqtt != nil {
			m.mqtt.SetMqttToSyslog(conf.MqttToSyslog)
		}
	} else {
		if m.mqttRunner != nil {
			m.stopMQTTLocked()
		}
	}

	// ARP Watch
	if conf.EnableArpWatch {
		if m.arpRunner == nil {
			m.startArpWatchLocked()
		} else if m.arpWatch != nil && (m.arpWatch.cfg.Range != conf.ArpWatchRange || m.arpWatch.cfg.Timeout != conf.ArpTimeout) {
			m.stopArpWatchLocked()
			m.startArpWatchLocked()
		}
	} else {
		if m.arpRunner != nil {
			m.stopArpWatchLocked()
		}
	}
}

func (m *Manager) logReceiverEvent(event string) {
	if m.cfg.Store != nil && m.rootCtx != nil && m.rootCtx.Err() == nil {
		_ = m.cfg.Store.AddEventLog(m.rootCtx, &datastore.EventLogEnt{
			Time:  time.Now().UnixNano(),
			Type:  "system",
			Level: "info",
			Event: i18n.Trans(event),
		})
	}
}

func (m *Manager) startSyslogLocked() {
	if m.syslogRunner != nil || m.rootCtx == nil || m.rootCtx.Err() != nil {
		return
	}
	udpPort := m.baseSyslogUDP
	tcpPort := m.baseSyslogTCP
	if udpPort <= 0 && tcpPort <= 0 {
		return
	}
	m.syslog = NewSyslogServer(SyslogConfig{
		UDPPort:  udpPort,
		TCPPort:  tcpPort,
		Store:    m.cfg.Store,
		LogStore: m.cfg.LogStore,
		Reporter: m.reporter,
	})
	childCtx, cancel := context.WithCancel(m.rootCtx)
	done := make(chan struct{})
	m.syslogRunner = &receiverRunner{cancel: cancel, done: done}
	go func(srv *SyslogServer) {
		defer close(done)
		_ = srv.Start(childCtx)
	}(m.syslog)
	m.logReceiverEvent("Started Syslog receiver")
}

func (m *Manager) stopSyslogLocked() {
	if m.syslogRunner == nil {
		return
	}
	r := m.syslogRunner
	m.syslogRunner = nil
	r.cancel()
	select {
	case <-r.done:
	case <-time.After(1 * time.Second):
	}
	m.logReceiverEvent("Stopped Syslog receiver")
}

func (m *Manager) startTrapLocked() {
	if m.trapRunner != nil || m.rootCtx == nil || m.rootCtx.Err() != nil {
		return
	}
	if m.baseTrapPort <= 0 {
		return
	}
	m.trap = NewTrapServer(TrapConfig{
		Port:     m.baseTrapPort,
		Store:    m.cfg.Store,
		LogStore: m.cfg.LogStore,
		Reporter: m.reporter,
	})
	childCtx, cancel := context.WithCancel(m.rootCtx)
	done := make(chan struct{})
	m.trapRunner = &receiverRunner{cancel: cancel, done: done}
	go func(srv *TrapServer) {
		defer close(done)
		_ = srv.Start(childCtx)
	}(m.trap)
	m.logReceiverEvent("Started SNMP TRAP receiver")
}

func (m *Manager) stopTrapLocked() {
	if m.trapRunner == nil {
		return
	}
	r := m.trapRunner
	m.trapRunner = nil
	r.cancel()
	select {
	case <-r.done:
	case <-time.After(1 * time.Second):
	}
	m.logReceiverEvent("Stopped SNMP TRAP receiver")
}

func (m *Manager) startNetFlowLocked() {
	if m.netflowRunner != nil || m.rootCtx == nil || m.rootCtx.Err() != nil {
		return
	}
	if m.baseNetFlowPort <= 0 {
		return
	}
	m.netflow = NewNetFlowServer(NetFlowConfig{
		Port:     m.baseNetFlowPort,
		Store:    m.cfg.Store,
		LogStore: m.cfg.LogStore,
		Reporter: m.reporter,
	})
	childCtx, cancel := context.WithCancel(m.rootCtx)
	done := make(chan struct{})
	m.netflowRunner = &receiverRunner{cancel: cancel, done: done}
	go func(srv *NetFlowServer) {
		defer close(done)
		_ = srv.Start(childCtx)
	}(m.netflow)
	m.logReceiverEvent("Started NetFlow receiver")
}

func (m *Manager) stopNetFlowLocked() {
	if m.netflowRunner == nil {
		return
	}
	r := m.netflowRunner
	m.netflowRunner = nil
	r.cancel()
	select {
	case <-r.done:
	case <-time.After(1 * time.Second):
	}
	m.logReceiverEvent("Stopped NetFlow receiver")
}

func (m *Manager) startSFlowLocked() {
	if m.sflowRunner != nil || m.rootCtx == nil || m.rootCtx.Err() != nil {
		return
	}
	if m.baseSFlowPort <= 0 {
		return
	}
	m.sflow = NewSFlowServer(SFlowConfig{
		Port:     m.baseSFlowPort,
		Store:    m.cfg.Store,
		LogStore: m.cfg.LogStore,
		Reporter: m.reporter,
	})
	childCtx, cancel := context.WithCancel(m.rootCtx)
	done := make(chan struct{})
	m.sflowRunner = &receiverRunner{cancel: cancel, done: done}
	go func(srv *SFlowServer) {
		defer close(done)
		_ = srv.Start(childCtx)
	}(m.sflow)
	m.logReceiverEvent("Started sFlow receiver")
}

func (m *Manager) stopSFlowLocked() {
	if m.sflowRunner == nil {
		return
	}
	r := m.sflowRunner
	m.sflowRunner = nil
	r.cancel()
	select {
	case <-r.done:
	case <-time.After(1 * time.Second):
	}
	m.logReceiverEvent("Stopped sFlow receiver")
}

func (m *Manager) startOTelLocked() {
	if m.otelRunner != nil || m.rootCtx == nil || m.rootCtx.Err() != nil {
		return
	}
	if m.baseOTelPort <= 0 {
		return
	}
	retention := m.cfg.OTelRetention
	if retention <= 0 {
		retention = 24
	}
	m.otel = NewOTelServer(OTelConfig{
		Port:      m.baseOTelPort,
		Store:     m.cfg.Store,
		LogStore:  m.cfg.LogStore,
		Retention: retention,
		From:      m.cfg.OTelFrom,
	})
	childCtx, cancel := context.WithCancel(m.rootCtx)
	done := make(chan struct{})
	m.otelRunner = &receiverRunner{cancel: cancel, done: done}
	go func(srv *OTelServer) {
		defer close(done)
		_ = srv.Start(childCtx)
	}(m.otel)
	m.logReceiverEvent("Started OpenTelemetry receiver")
}

func (m *Manager) stopOTelLocked() {
	if m.otelRunner == nil {
		return
	}
	r := m.otelRunner
	m.otelRunner = nil
	r.cancel()
	select {
	case <-r.done:
	case <-time.After(1 * time.Second):
	}
	m.logReceiverEvent("Stopped OpenTelemetry receiver")
}

func (m *Manager) startMQTTLocked() {
	if m.mqttRunner != nil || m.rootCtx == nil || m.rootCtx.Err() != nil {
		return
	}
	if m.baseMQTTPort <= 0 {
		return
	}
	m.mqtt = NewMQTTServer(MQTTConfig{
		Port:         m.baseMQTTPort,
		Store:        m.cfg.Store,
		LogStore:     m.cfg.LogStore,
		Reporter:     m.reporter,
		ArpWatch:     m.arpWatch,
		MqttToSyslog: m.cfg.MqttToSyslog,
	})
	childCtx, cancel := context.WithCancel(m.rootCtx)
	done := make(chan struct{})
	m.mqttRunner = &receiverRunner{cancel: cancel, done: done}
	go func(srv *MQTTServer) {
		defer close(done)
		_ = srv.Start(childCtx)
	}(m.mqtt)
	m.logReceiverEvent("Started MQTT broker")
}

func (m *Manager) stopMQTTLocked() {
	if m.mqttRunner == nil {
		return
	}
	r := m.mqttRunner
	m.mqttRunner = nil
	r.cancel()
	select {
	case <-r.done:
	case <-time.After(1 * time.Second):
	}
	m.logReceiverEvent("Stopped MQTT broker")
}

func (m *Manager) startArpWatchLocked() {
	if m.arpRunner != nil || m.rootCtx == nil || m.rootCtx.Err() != nil {
		return
	}
	m.arpWatch = NewArpWatchServer(ArpWatchConfig{
		Store:    m.cfg.Store,
		LogStore: m.cfg.LogStore,
		Enabled:  true,
		Range:    m.cfg.ArpWatchRange,
		Timeout:  m.cfg.ArpTimeout,
	})
	if m.mqtt != nil {
		m.mqtt.SetArpWatch(m.arpWatch)
	}
	childCtx, cancel := context.WithCancel(m.rootCtx)
	done := make(chan struct{})
	m.arpRunner = &receiverRunner{cancel: cancel, done: done}
	go func(srv *ArpWatchServer) {
		defer close(done)
		_ = srv.Start(childCtx)
	}(m.arpWatch)
}

func (m *Manager) stopArpWatchLocked() {
	if m.arpRunner == nil {
		return
	}
	r := m.arpRunner
	m.arpRunner = nil
	r.cancel()
	select {
	case <-r.done:
	case <-time.After(1 * time.Second):
	}
	if m.arpWatch != nil {
		m.arpWatch.cfg.Enabled = false
	}
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
	eventDays := 30
	parquetDays := 14

	if m.cfg.Store != nil {
		if conf, err := m.cfg.Store.GetMapConf(ctx); err == nil && conf != nil {
			if conf.EventLogDays > 0 {
				eventDays = conf.EventLogDays
			} else if conf.LogDays > 0 {
				eventDays = conf.LogDays
			}
			if conf.ParquetLogDays > 0 {
				parquetDays = conf.ParquetLogDays
			} else if conf.LogDays > 0 {
				parquetDays = conf.LogDays
			}
		}
	}

	if m.cfg.Store != nil {
		if rotated, err := m.cfg.Store.CleanOldEventLogs(ctx, eventDays); err == nil && rotated > 0 {
			slog.Info("Rotated old bbolt event logs", "deleted", rotated, "retentionDays", eventDays)
		}
		_ = m.cfg.Store.CleanOldMqttStats(ctx, eventDays)
	}

	if m.cfg.LogStore != nil {
		if rotated, err := m.cfg.LogStore.Rotate(parquetDays); err == nil && rotated > 0 {
			slog.Info("Rotated old parquet log files", "deletedFiles", rotated, "retentionDays", parquetDays)
		}
		if compacted, err := m.cfg.LogStore.Compact(); err == nil && compacted > 0 {
			slog.Info("Compacted small parquet log files", "mergedFiles", compacted)
		}
	}
}

// GetArpTable returns all known ARP table entries from the ARP watch engine.
func (m *Manager) GetArpTable() []*datastore.ArpEnt {
	m.mu.Lock()
	aw := m.arpWatch
	m.mu.Unlock()
	if aw != nil {
		return aw.GetArpTable()
	}
	return nil
}

// DeleteArpEntries removes given IPs from the in-memory ARP table.
func (m *Manager) DeleteArpEntries(ips []string) {
	m.mu.Lock()
	aw := m.arpWatch
	m.mu.Unlock()
	if aw != nil {
		aw.DeleteEntries(ips)
	}
}

// ResetArpTable clears the in-memory ARP table.
func (m *Manager) ResetArpTable() {
	m.mu.Lock()
	aw := m.arpWatch
	m.mu.Unlock()
	if aw != nil {
		aw.ResetTable()
	}
}

// GetReceiverStatus returns a map describing ports and running statuses of all managed receivers.
func (m *Manager) GetReceiverStatus(mcpEnabled bool) map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make(map[string]any)

	// Syslog
	if m.syslogRunner != nil {
		var portStr string
		if m.baseSyslogUDP > 0 && m.baseSyslogTCP > 0 {
			portStr = fmt.Sprintf("UDP :%d / TCP :%d", m.baseSyslogUDP, m.baseSyslogTCP)
		} else if m.baseSyslogUDP > 0 {
			portStr = fmt.Sprintf("UDP :%d", m.baseSyslogUDP)
		} else {
			portStr = fmt.Sprintf("TCP :%d", m.baseSyslogTCP)
		}
		result["syslog"] = map[string]any{
			"port":   portStr,
			"status": "running",
		}
	} else {
		result["syslog"] = map[string]any{
			"port":   "-",
			"status": "stopped",
		}
	}

	// SNMP Trap
	if m.trapRunner != nil {
		result["trap"] = map[string]any{
			"port":   fmt.Sprintf("UDP :%d (v1/v2c/v3)", m.baseTrapPort),
			"status": "running",
		}
	} else {
		result["trap"] = map[string]any{
			"port":   "-",
			"status": "stopped",
		}
	}

	// NetFlow
	if m.netflowRunner != nil {
		result["netflow"] = map[string]any{
			"port":   fmt.Sprintf("UDP :%d", m.baseNetFlowPort),
			"status": "running",
		}
	} else {
		result["netflow"] = map[string]any{
			"port":   "-",
			"status": "stopped",
		}
	}

	// sFlow
	if m.sflowRunner != nil {
		result["sflow"] = map[string]any{
			"port":   fmt.Sprintf("UDP :%d", m.baseSFlowPort),
			"status": "running",
		}
	} else {
		result["sflow"] = map[string]any{
			"port":   "-",
			"status": "stopped",
		}
	}

	// OpenTelemetry
	if m.otelRunner != nil {
		result["otel"] = map[string]any{
			"port":   fmt.Sprintf("HTTP :%d (OTLP)", m.baseOTelPort),
			"status": "running",
		}
	} else {
		result["otel"] = map[string]any{
			"port":   "-",
			"status": "stopped",
		}
	}

	// MQTT Broker
	if m.mqttRunner != nil {
		result["mqtt"] = map[string]any{
			"port":   fmt.Sprintf("TCP :%d", m.baseMQTTPort),
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
	if m.arpRunner != nil {
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
