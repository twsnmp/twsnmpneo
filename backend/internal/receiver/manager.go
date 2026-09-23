package receiver

import (
	"context"
	"log/slog"
	"sync"

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
	OTelPort     int
	MQTTPort     int
	MqttToSyslog bool
	EnableArpWatch bool
	ArpWatchRange  string
	ArpTimeout     int
}

// Manager controls the lifecycle of all embedded protocol receivers.
type Manager struct {
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
		syslog: NewSyslogServer(SyslogConfig{
			UDPPort:  cfg.SyslogUDP,
			TCPPort:  cfg.SyslogTCP,
			LogStore: cfg.LogStore,
		}),
		trap: NewTrapServer(TrapConfig{
			Port:     cfg.TrapPort,
			Store:    cfg.Store,
			LogStore: cfg.LogStore,
		}),
		netflow: NewNetFlowServer(NetFlowConfig{
			Port:     cfg.NetFlowPort,
			LogStore: cfg.LogStore,
		}),
		sflow: NewSFlowServer(SFlowConfig{
			Port:     cfg.SFlowPort,
			LogStore: cfg.LogStore,
		}),
		otel: NewOTelServer(OTelConfig{
			Port:     cfg.OTelPort,
			LogStore: cfg.LogStore,
		}),
		mqtt: NewMQTTServer(MQTTConfig{
			Port:         cfg.MQTTPort,
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

	<-ctx.Done()
	slog.Info("Stopping Protocol Receivers...")
	wg.Wait()
	return nil
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

