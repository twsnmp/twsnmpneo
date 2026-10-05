package datastore

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"

	"golang.org/x/oauth2"
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
	SaveNodes(ctx context.Context, nodes []*NodeEnt) error
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
	GetBackImage(ctx context.Context) (*BackImageEnt, error)
	SaveBackImage(ctx context.Context, bi *BackImageEnt) error
	GetDiscoverConf(ctx context.Context) (*DiscoverConfEnt, error)
	SaveDiscoverConf(ctx context.Context, conf *DiscoverConfEnt) error

	// Notify OAuth2 Token
	GetNotifyOAuth2Token() *oauth2.Token
	SaveNotifyOAuth2Token(token *oauth2.Token)
	DeleteNotifyOAuth2Token()
	HasValidNotifyOAuth2Token(n *NotifyConfEnt) bool

	// Mail Templates
	LoadMailTemplate(t string) string

	// Event Logs
	AddEventLog(ctx context.Context, event *EventLogEnt) error
	ListEventLogs(ctx context.Context, limit int) ([]*EventLogEnt, error)
	QueryEventLogs(ctx context.Context, filter EventLogFilter) ([]*EventLogEnt, error)
	DeleteEventLogs(ctx context.Context) error
	CountEventLogs(ctx context.Context) (int64, error)

	// ForEach iterators (in-memory cache, no error return, callback returns false to stop)
	ForEachNodes(fn func(*NodeEnt) bool)
	ForEachNetworks(fn func(*NetworkEnt) bool)
	ForEachLines(fn func(*LineEnt) bool)
	ForEachPollings(fn func(*PollingEnt) bool)
	// ForEachLastEventLog iterates event logs in reverse chronological order
	ForEachLastEventLog(fn func(*EventLogEnt) bool)

	// AI Results & Config
	GetAIResult(pollingID string) (*AIResultEnt, error)
	SaveAIResult(ctx context.Context, result *AIResultEnt) error
	DeleteAIResult(ctx context.Context, id string) error
	GetAIConf(ctx context.Context) (*AIConfEnt, error)
	SaveAIConf(ctx context.Context, conf *AIConfEnt) error
	GetDBSize() int64

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
	ListOTelTraces(ctx context.Context, buckets []string, limit int) ([]*OTelTraceSummaryEnt, error)
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

	// Certificate Monitors (External TLS Monitor)
	ListCertMonitors(ctx context.Context) ([]*CertMonitorEnt, error)
	GetCertMonitor(ctx context.Context, id string) (*CertMonitorEnt, error)
	SaveCertMonitor(ctx context.Context, c *CertMonitorEnt) error
	DeleteCertMonitor(ctx context.Context, id string) error

	// Sensors (Remote Sensors Reporting to TWSNMP)
	ListSensors(ctx context.Context) ([]*SensorEnt, error)
	GetSensor(ctx context.Context, id string) (*SensorEnt, error)
	SaveSensor(ctx context.Context, s *SensorEnt) error
	DeleteSensors(ctx context.Context, ids []string) error
	DeleteAllSensors(ctx context.Context) error
	ToggleSensorIgnore(ctx context.Context, id string) error
	UpdateSensor(host, sensorType, param string, count int64)
	CheckSensorStats(host, sensorType, param string, count, send, total int64, ps float64)
	CheckSensorMonitor(host, sensorType, param string, cpu, mem, load, txSpeed, rxSpeed float64, sent, recv, proc int64)
	EvaluateSensorStates(ctx context.Context)

	// Log-derived reports (twWifiScan, twBlueScan, twpcap, twwinlog).
	// Entities are stored as JSON per report kind and keyed by entity ID.
	// GetLogReportData returns (nil, nil) when the entity does not exist.
	GetLogReportData(ctx context.Context, kind, id string) ([]byte, error)
	ListLogReportData(ctx context.Context, kind string) (map[string][]byte, error)
	SaveLogReportData(ctx context.Context, kind string, items map[string][]byte) error
	DeleteLogReportData(ctx context.Context, kind string, ids []string) error
	// ResetLogReportData removes every entity of the kind. Kind "" removes all kinds.
	ResetLogReportData(ctx context.Context, kind string) error
}

// AIResultEnt holds AI anomaly detection results for a polling.
type AIResultEnt struct {
	PollingID string      `json:"PollingID"`
	ScoreData [][]float64 `json:"ScoreData"`
	LastTime  int64       `json:"LastTime"`
}

// AIListEnt represents an entry in the AI anomaly detection list report.
type AIListEnt struct {
	ID       string  `json:"ID"`
	Node     string  `json:"Node"`
	Polling  string  `json:"Polling"`
	Score    float64 `json:"Score"`
	Count    int     `json:"Count"`
	LastTime int64   `json:"LastTime"`
}

// AIConfEnt defines anomaly detection threshold settings.
type AIConfEnt struct {
	HighThreshold float64 `json:"HighThreshold"`
	LowThreshold  float64 `json:"LowThreshold"`
	WarnThreshold float64 `json:"WarnThreshold"`
}

