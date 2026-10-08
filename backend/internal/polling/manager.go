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
	"github.com/twsnmp/twsnmpneo/backend/internal/i18n"
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
	inFlight     sync.Map // pollingID -> struct{}
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
	m.RegisterPoller("ping", NewPingPoller())
	m.RegisterPoller("http", NewHTTPPoller())
	m.RegisterPoller("https", NewHTTPPoller())
	m.RegisterPoller("tcp", NewTCPPoller())
	m.RegisterPoller("tls", NewTCPPoller())
	m.RegisterPoller("dns", NewDNSPoller())
	m.RegisterPoller("ntp", NewNTPPoller())
	m.RegisterPoller("snmp", NewSNMPPoller())
	m.RegisterPoller("stun", NewSTUNPoller())
	m.RegisterPoller("twsnmp", NewTWSNMPPoller())
	m.RegisterPoller("monitor", NewMonitorPoller(nil))
	m.RegisterPoller("cmd", NewCmdPoller())
	m.RegisterPoller("command", NewCmdPoller())
	m.RegisterPoller("ssh", NewSSHPoller())
	m.RegisterPoller("syslog", NewSyslogPoller(cfg.Store, cfg.LogStore))
	m.RegisterPoller("trap", NewSnmpTrapPoller(cfg.Store, cfg.LogStore))
	m.RegisterPoller("snmptrap", NewSnmpTrapPoller(cfg.Store, cfg.LogStore))
	m.RegisterPoller("arplog", NewArpLogPoller(cfg.Store, cfg.LogStore))
	m.RegisterPoller("netflow", NewNetFlowPoller(cfg.Store, cfg.LogStore))
	m.RegisterPoller("gnmi", NewGNMIPoller())
	m.RegisterPoller("mqtt", NewMQTTPoller())
	m.RegisterPoller("email", NewEMailPoller())
	m.RegisterPoller("pihole", NewPiHolePoller())
	m.RegisterPoller("lxi", NewLXIPoller())
	m.RegisterPoller("twlogeye", NewTwLogEyePoller())
	m.RegisterPoller("report", NewCmdPoller()) // Placeholder for report type

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

func clonePolling(p *datastore.PollingEnt) *datastore.PollingEnt {
	if p == nil {
		return nil
	}
	cp := *p
	if p.Result != nil {
		cp.Result = make(map[string]interface{}, len(p.Result))
		for k, v := range p.Result {
			cp.Result[k] = v
		}
	}
	return &cp
}

// ExecuteOne polls a single task immediately, useful for on-demand checks and tests.
func (m *Manager) ExecuteOne(ctx context.Context, orig *datastore.PollingEnt) (*Result, error) {
	if orig == nil {
		return nil, fmt.Errorf("polling is nil")
	}
	if orig.ID != "" {
		if _, busy := m.inFlight.LoadOrStore(orig.ID, struct{}{}); busy {
			return &Result{
				State:   orig.State,
				Message: "already running",
			}, nil
		}
		defer m.inFlight.Delete(orig.ID)
	}

	// Defensive copy to prevent concurrent map/struct race conditions
	p := clonePolling(orig)

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
		failState := StateHigh
		if p.Level != "" {
			failState = p.Level
		}
		res = &Result{
			State:   failState,
			Message: err.Error(),
		}
	}

	// Dynamic state override via setLevel(...) from JavaScript
	if res.Fields != nil {
		if lvl, ok := res.Fields["_level"].(string); ok && lvl != "" {
			res.State = lvl
			delete(res.Fields, "_level")
		}
	}

	now := time.Now().UnixNano()
	oldState := p.State
	if oldState == "" {
		oldState = StateUnknown
	}

	sendEvent := false
	var downtimeSec int64 = 0

	switch res.State {
	case StateNormal:
		if p.Result != nil {
			delete(p.Result, "error")
		}
		if oldState != StateNormal && oldState != StateRepair {
			if oldState == StateUnknown ||
				p.Type == "syslog" || p.Type == "trap" || p.Type == "snmptrap" || p.Type == "arplog" {
				p.State = StateNormal
			} else {
				p.State = StateRepair
				if p.FailTime > 0 {
					downtimeSec = (now - p.FailTime) / (1000 * 1000 * 1000)
					if downtimeSec < 0 {
						downtimeSec = 0
					}
					p.FailTime = 0
				}
			}
			sendEvent = true
		} else {
			p.State = oldState
		}
	case StateUnknown:
		if oldState != StateUnknown {
			p.State = StateUnknown
			sendEvent = true
		}
	default:
		// Failure states (warn, low, high, etc.)
		if oldState != res.State {
			if oldState == StateNormal || oldState == StateRepair || oldState == StateUnknown || p.FailTime == 0 {
				p.FailTime = now
			}
			p.State = res.State
			sendEvent = true
		}
	}

	p.LastTime = now
	resMap := map[string]interface{}{
		"state":   p.State,
		"rtt":     float64(res.RTT.Nanoseconds()),
		"message": res.Message,
	}
	for k, v := range res.Fields {
		resMap[k] = v
	}
	p.Result = resMap

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
		if sendEvent {
			nodeName := p.NodeID
			if node != nil {
				nodeName = node.Name
			}
			dispOld := oldState
			if dispOld == "" {
				dispOld = "unknown"
			}
			eventMsg := fmt.Sprintf(i18n.Trans("Polling %s: %s -> %s (%s)"), p.Name, dispOld, p.State, res.Message)
			if p.State == StateRepair && downtimeSec > 0 {
				eventMsg += fmt.Sprintf(" [%s: %s]", i18n.Trans("Downtime"), formatDowntime(downtimeSec))
			}
			_ = m.store.AddEventLog(ctx, &datastore.EventLogEnt{
				Time:      now,
				Type:      "polling",
				Level:     p.State,
				NodeName:  nodeName,
				NodeID:    p.NodeID,
				Event:     eventMsg,
				LastLevel: oldState,
				Downtime:  downtimeSec,
			})

			// Execute configured action in background
			go doAction(context.Background(), p, m.store)
		}

		// Update node state by aggregating all active pollings
		if node != nil {
			m.updateNodeState(ctx, node)
		}
	}

	// Record to Parquet log store if enabled
	if m.logStore != nil && (p.LogMode == datastore.LogModeAlways || p.LogMode == datastore.LogModeAI || (p.LogMode == datastore.LogModeOnChange && oldState != p.State)) {
		payload := map[string]interface{}{
			"State":  p.State,
			"Result": p.Result,
		}
		logData, _ := json.Marshal(payload)
		_ = m.logStore.WriteLog(&parquet.ParquetLogRecord{
			Time:      now,
			Timestamp: now,
			Type:      "polling",
			Src:       p.ID,
			Log:       string(logData),
		})
	}

	if orig != nil {
		orig.State = p.State
		orig.FailTime = p.FailTime
		orig.LastTime = p.LastTime
		orig.NextTime = p.NextTime
		orig.Result = p.Result
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
		if p.Level == "off" {
			continue
		}
		if p.NextTime <= now {
			if _, busy := m.inFlight.Load(p.ID); busy {
				continue
			}
			go func(task *datastore.PollingEnt) {
				_, _ = m.ExecuteOne(ctx, task)
			}(p)
		}
	}
}

// CheckAll resets all pollings in non-normal state to 'unknown' and queues them to execute immediately.
func (m *Manager) CheckAll(ctx context.Context) int {
	if m.store == nil {
		return 0
	}
	pollings, err := m.store.ListPollings(ctx)
	if err != nil || len(pollings) == 0 {
		return 0
	}
	count := 0
	now := time.Now().UnixNano()
	for _, p := range pollings {
		if p.Level == "off" || p.State == StateNormal {
			continue
		}
		p.State = StateUnknown
		p.NextTime = 0
		_ = m.store.SavePolling(ctx, p)

		node, _ := m.store.GetNode(ctx, p.NodeID)
		nodeName := p.NodeID
		if node != nil {
			nodeName = node.Name
			if node.State != StateUnknown {
				node.State = StateUnknown
				_ = m.store.SaveNode(ctx, node)
			}
		}

		_ = m.store.AddEventLog(ctx, &datastore.EventLogEnt{
			Time:     now,
			Type:     "user",
			Level:    "info",
			NodeID:   p.NodeID,
			NodeName: nodeName,
			Event:    i18n.Trans("re check polling:") + p.Name,
		})

		if _, busy := m.inFlight.Load(p.ID); busy {
			continue
		}
		count++
		go func(task *datastore.PollingEnt) {
			_, _ = m.ExecuteOne(context.Background(), task)
		}(p)
	}
	return count
}

// CheckNode resets all non-normal pollings of a node to 'unknown' and queues them to execute immediately.
func (m *Manager) CheckNode(ctx context.Context, nodeID string) (int, error) {
	if m.store == nil {
		return 0, fmt.Errorf("polling manager has no datastore")
	}
	if nodeID == "" {
		return 0, fmt.Errorf("node id is required")
	}
	node, err := m.store.GetNode(ctx, nodeID)
	if err != nil || node == nil {
		return 0, fmt.Errorf("node not found")
	}

	node.State = StateUnknown
	_ = m.store.SaveNode(ctx, node)

	pollings, err := m.store.ListPollings(ctx)
	if err != nil {
		return 0, err
	}
	count := 0
	now := time.Now().UnixNano()
	for _, p := range pollings {
		if p.NodeID != nodeID || p.Level == "off" || p.State == StateNormal {
			continue
		}
		p.State = StateUnknown
		p.NextTime = 0
		_ = m.store.SavePolling(ctx, p)

		_ = m.store.AddEventLog(ctx, &datastore.EventLogEnt{
			Time:     now,
			Type:     "user",
			Level:    "info",
			NodeID:   node.ID,
			NodeName: node.Name,
			Event:    i18n.Trans("re check polling:") + p.Name,
		})

		if _, busy := m.inFlight.Load(p.ID); busy {
			continue
		}
		count++
		go func(task *datastore.PollingEnt) {
			_, _ = m.ExecuteOne(context.Background(), task)
		}(p)
	}

	if count == 0 {
		m.updateNodeState(ctx, node)
	}

	return count, nil
}

func (m *Manager) updateNodeState(ctx context.Context, node *datastore.NodeEnt) {
	if m.store == nil || node == nil {
		return
	}
	pollings, err := m.store.ListPollings(ctx)
	if err != nil {
		return
	}

	newState := StateUnknown
	hasActivePolling := false

	for _, p := range pollings {
		if p.NodeID != node.ID || p.Level == "off" {
			continue
		}
		hasActivePolling = true
		s := p.State
		if s == StateHigh {
			newState = StateHigh
			break
		}
		if s == StateLow {
			newState = StateLow
			continue
		}
		if newState == StateLow {
			continue
		}
		if s == StateWarn {
			newState = StateWarn
			continue
		}
		if newState == StateWarn {
			continue
		}
		if s == StateRepair {
			if !node.AutoAck {
				newState = StateRepair
				continue
			} else {
				p.State = StateNormal
				s = StateNormal
				_ = m.store.SavePolling(ctx, p)
			}
		}
		if newState != StateUnknown {
			continue
		}
		if s == "info" {
			s = StateNormal
		}
		newState = s
	}

	if !hasActivePolling {
		if node.State == "" || node.State == StateHigh || node.State == StateLow || node.State == StateWarn || node.State == StateRepair {
			newState = StateNormal
		} else {
			newState = node.State
		}
	}

	if node.State != newState {
		node.State = newState
		_ = m.store.SaveNode(ctx, node)
	}
}

// ClearRepairPollings resets all pollings in 'repair' state to 'unknown' and schedules them immediately.
func (m *Manager) ClearRepairPollings(ctx context.Context) int {
	if m.store == nil {
		return 0
	}
	pollings, err := m.store.ListPollings(ctx)
	if err != nil {
		return 0
	}
	count := 0
	for _, p := range pollings {
		if p.State == StateRepair {
			p.State = StateUnknown
			p.NextTime = 0
			_ = m.store.SavePolling(ctx, p)
			count++
		}
	}
	return count
}

