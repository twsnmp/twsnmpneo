package receiver

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
)

// Config holds options for all receivers managed together.
type Config struct {
	Store        datastore.DataStore
	LogStore     *parquet.Store
	SyslogUDP    int
	SyslogTCP    int
	TrapPort     int
	NetFlowPort  int
	SFlowPort    int
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
	return &Manager{
		cfg: cfg,
		syslog: NewSyslogServer(SyslogConfig{
			UDPPort:  cfg.SyslogUDP,
			TCPPort:  cfg.SyslogTCP,
			Store:    cfg.Store,
			LogStore: cfg.LogStore,
		}),
		trap: NewTrapServer(TrapConfig{
			Port:     cfg.TrapPort,
			Store:    cfg.Store,
			LogStore: cfg.LogStore,
		}),
		netflow: NewNetFlowServer(NetFlowConfig{
			Port:     cfg.NetFlowPort,
			Store:    cfg.Store,
			LogStore: cfg.LogStore,
		}),
		sflow: NewSFlowServer(SFlowConfig{
			Port:     cfg.SFlowPort,
			Store:    cfg.Store,
			LogStore: cfg.LogStore,
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

	// Periodic Parquet log rotation & datastore retention cleaner
	go m.startRetentionCleaner(ctx)

	// Periodic sensor state & rate evaluator (every 1 minute)
	go m.startSensorEvaluator(ctx)

	<-ctx.Done()
	slog.Info("Stopping Protocol Receivers...")
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

