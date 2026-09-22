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
	OTelPort     int
	MQTTPort     int
	MqttToSyslog bool
}

// Manager controls the lifecycle of all embedded protocol receivers.
type Manager struct {
	syslog  *SyslogServer
	trap    *TrapServer
	netflow *NetFlowServer
	otel    *OTelServer
	mqtt    *MQTTServer
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
		otel: NewOTelServer(OTelConfig{
			Port:     cfg.OTelPort,
			LogStore: cfg.LogStore,
		}),
		mqtt: NewMQTTServer(MQTTConfig{
			Port:         cfg.MQTTPort,
			LogStore:     cfg.LogStore,
			MqttToSyslog: cfg.MqttToSyslog,
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
		_ = m.otel.Start(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = m.mqtt.Start(ctx)
	}()

	<-ctx.Done()
	slog.Info("Stopping Protocol Receivers...")
	wg.Wait()
	return nil
}
