package polling

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
)

// Config defines options for the Polling Manager.
type Config struct {
	Store        datastore.DataStore
	LogStore     *parquet.Store
	WorkerCount  int
	PollInterval time.Duration
}

// Manager orchestrates and executes polling tasks periodically.
type Manager struct {
	store        datastore.DataStore
	logStore     *parquet.Store
	workerCount  int
	pollInterval time.Duration
	pollers      map[string]Poller
	mu           sync.RWMutex
	running      bool
}

// NewManager creates an initialized PollingManager with default pollers.
func NewManager(cfg Config) *Manager {
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 10
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 1 * time.Second
	}

	m := &Manager{
		store:        cfg.Store,
		logStore:     cfg.LogStore,
		workerCount:  cfg.WorkerCount,
		pollInterval: cfg.PollInterval,
		pollers:      make(map[string]Poller),
	}

	// Register core pollers
	m.RegisterPoller("ping", &PingPoller{})
	m.RegisterPoller("http", NewHTTPPoller())
	m.RegisterPoller("https", NewHTTPPoller())
	m.RegisterPoller("tcp", &TCPPoller{})
	m.RegisterPoller("dns", &DNSPoller{})
	m.RegisterPoller("ntp", &NTPPoller{})
	m.RegisterPoller("snmp", &SNMPPoller{})

	return m
}

// RegisterPoller registers a custom or mock poller implementation.
func (m *Manager) RegisterPoller(pollType string, p Poller) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pollers[pollType] = p
}

// Start launches the polling loop until the context is canceled.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	m.running = true
	m.mu.Unlock()

	slog.Info("Starting Polling Manager", "workers", m.workerCount)
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Stopping Polling Manager...")
			m.mu.Lock()
			m.running = false
			m.mu.Unlock()
			return nil
		case <-ticker.C:
			m.checkAndSchedule(ctx)
		}
	}
}

// ExecuteOne polls a single task immediately, useful for on-demand checks and tests.
func (m *Manager) ExecuteOne(ctx context.Context, p *datastore.PollingEnt) (*Result, error) {
	m.mu.RLock()
	poller, ok := m.pollers[p.Type]
	m.mu.RUnlock()

	if !ok {
		return &Result{
			State:   StateUnknown,
			Message: fmt.Sprintf("unsupported polling type '%s'", p.Type),
		}, nil
	}

	var node *datastore.NodeEnt
	if m.store != nil && p.NodeID != "" {
		node, _ = m.store.GetNode(ctx, p.NodeID)
	}

	res, err := poller.Poll(ctx, p, node)
	if err != nil {
		res = &Result{
			State:   StateHigh,
			Message: err.Error(),
		}
	}

	now := time.Now().UnixNano()
	oldState := p.State
	p.State = res.State
	p.LastTime = now
	p.Result = map[string]interface{}{
		"state":   res.State,
		"rtt":     res.RTT.Milliseconds(),
		"message": res.Message,
	}
	for k, v := range res.Fields {
		p.Result[k] = v
	}

	// Schedule next run
	pollInt := p.PollInt
	if pollInt <= 0 {
		pollInt = 60
	}
	p.NextTime = now + int64(pollInt)*int64(time.Second)

	// Save polling state
	if m.store != nil {
		_ = m.store.SavePolling(ctx, p)

		// Record state change event log
		if oldState != "" && oldState != res.State {
			nodeName := p.NodeID
			if node != nil {
				nodeName = node.Name
			}
			level := "info"
			if res.State == StateWarn {
				level = "warn"
			} else if res.State == StateHigh {
				level = "high"
			}
			_ = m.store.AddEventLog(ctx, &datastore.EventLogEnt{
				Time:      now,
				Type:      "polling",
				Level:     level,
				NodeName:  nodeName,
				NodeID:    p.NodeID,
				Event:     fmt.Sprintf("Polling %s state changed: %s -> %s (%s)", p.Name, oldState, res.State, res.Message),
				LastLevel: oldState,
			})
		}
	}

	// Record to Parquet log store if enabled
	if m.logStore != nil && p.LogMode != datastore.LogModeNone {
		shouldLog := true
		if p.LogMode == datastore.LogModeOnChange && oldState == res.State {
			shouldLog = false
		}
		if shouldLog {
			logData, _ := json.Marshal(p.Result)
			_ = m.logStore.WriteLog(&parquet.ParquetLogRecord{
				Time: now,
				Type: "polling",
				Src:  p.NodeID,
				Log:  string(logData),
			})
		}
	}

	return res, nil
}

func (m *Manager) checkAndSchedule(ctx context.Context) {
	if m.store == nil {
		return
	}
	pollings, err := m.store.ListPollings(ctx)
	if err != nil || len(pollings) == 0 {
		return
	}

	now := time.Now().UnixNano()
	for _, p := range pollings {
		if p.NextTime <= now {
			go func(task *datastore.PollingEnt) {
				_, _ = m.ExecuteOne(ctx, task)
			}(p)
		}
	}
}
