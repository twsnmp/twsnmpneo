package datastore

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
)

var (
	ErrNotFound       = errors.New("record not found")
	ErrAlreadyExists  = errors.New("record already exists")
	ErrInvalidID      = errors.New("invalid id")
	ErrInvalidParams  = errors.New("invalid parameters")
	ErrDBNotOpen      = errors.New("database not open")
)

// GenerateID generates a random 16-hex character ID matching twsnmpfk.
func GenerateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// DataStore is the interface for managing topology and configuration persistence.
type DataStore interface {
	// Lifecycle
	Close() error

	// Nodes
	GetNode(ctx context.Context, id string) (*NodeEnt, error)
	ListNodes(ctx context.Context) ([]*NodeEnt, error)
	SaveNode(ctx context.Context, node *NodeEnt) error
	DeleteNode(ctx context.Context, id string) error

	// Lines
	GetLine(ctx context.Context, id string) (*LineEnt, error)
	ListLines(ctx context.Context) ([]*LineEnt, error)
	SaveLine(ctx context.Context, line *LineEnt) error
	DeleteLine(ctx context.Context, id string) error

	// Networks
	GetNetwork(ctx context.Context, id string) (*NetworkEnt, error)
	ListNetworks(ctx context.Context) ([]*NetworkEnt, error)
	SaveNetwork(ctx context.Context, nw *NetworkEnt) error
	DeleteNetwork(ctx context.Context, id string) error

	// DrawItems
	GetDrawItem(ctx context.Context, id string) (*DrawItemEnt, error)
	ListDrawItems(ctx context.Context) ([]*DrawItemEnt, error)
	SaveDrawItem(ctx context.Context, item *DrawItemEnt) error
	DeleteDrawItem(ctx context.Context, id string) error

	// Pollings
	GetPolling(ctx context.Context, id string) (*PollingEnt, error)
	ListPollings(ctx context.Context) ([]*PollingEnt, error)
	SavePolling(ctx context.Context, p *PollingEnt) error
	DeletePolling(ctx context.Context, id string) error

	// Configurations
	GetMapConf(ctx context.Context) (*MapConfEnt, error)
	SaveMapConf(ctx context.Context, conf *MapConfEnt) error
	GetNotifyConf(ctx context.Context) (*NotifyConfEnt, error)
	SaveNotifyConf(ctx context.Context, conf *NotifyConfEnt) error
	GetLocConf(ctx context.Context) (*LocConfEnt, error)
	SaveLocConf(ctx context.Context, conf *LocConfEnt) error

	// Event Logs
	AddEventLog(ctx context.Context, event *EventLogEnt) error
	ListEventLogs(ctx context.Context, limit int) ([]*EventLogEnt, error)
	QueryEventLogs(ctx context.Context, filter EventLogFilter) ([]*EventLogEnt, error)
	DeleteEventLogs(ctx context.Context) error
	CountEventLogs(ctx context.Context) (int64, error)

	// ARP Table
	SaveArpTable(ctx context.Context, entries []*ArpEnt) error
	LoadArpTable(ctx context.Context) ([]*ArpEnt, error)
	DeleteArpEntries(ctx context.Context, ips []string) error
	ResetArpTable(ctx context.Context) error

	// OpenTelemetry (OTel)
	ListOTelMetrics(ctx context.Context) ([]*OTelMetricEnt, error)
	GetOTelMetric(ctx context.Context, host, service, scope, name string) (*OTelMetricEnt, error)
	SaveOTelMetric(ctx context.Context, m *OTelMetricEnt) error
	DeleteOTelMetric(ctx context.Context, host, service, scope, name string) error
	GetOTelTraceBuckets(ctx context.Context) ([]string, error)
	ListOTelTraces(ctx context.Context, buckets []string) ([]*OTelTraceSummaryEnt, error)
	GetOTelTrace(ctx context.Context, bucket, traceID string) (*OTelTraceEnt, error)
	SaveOTelTraces(ctx context.Context, traces []*OTelTraceEnt) error
	GetOTelTraceDAG(ctx context.Context, buckets []string) (*OTelTraceDAGEnt, error)
	DeleteAllOTelData(ctx context.Context) error
	CleanOldOTelData(ctx context.Context, retentionHours int) error

	// MQTT Stats
	ListMqttStats(ctx context.Context) ([]*MqttStatEnt, error)
	SaveMqttStat(ctx context.Context, s *MqttStatEnt) error
	SaveMqttStats(ctx context.Context, stats []*MqttStatEnt) error
	DeleteMqttStats(ctx context.Context, ids []string) error
	DeleteAllMqttStats(ctx context.Context) error
	CleanOldMqttStats(ctx context.Context, days int) error
}
