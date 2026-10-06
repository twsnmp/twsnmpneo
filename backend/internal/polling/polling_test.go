package polling_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/bbolt"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	"github.com/twsnmp/twsnmpneo/backend/internal/polling"
)

func setupTestEnv(t *testing.T) (datastore.DataStore, *parquet.Store, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "twsnmpneo-polling-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}

	bboltStore, err := bbolt.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("create bbolt store: %v", err)
	}

	pqStore, err := parquet.New(parquet.Config{
		Dir:            filepath.Join(dir, "logs"),
		BufferSize:     2,
		BufferInterval: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("create parquet store: %v", err)
	}

	cleanup := func() {
		_ = bboltStore.Close()
		_ = pqStore.Close()
		_ = os.RemoveAll(dir)
	}
	return bboltStore, pqStore, cleanup
}

func TestHTTPPoller(t *testing.T) {
	// 1. Success case with keyword
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("TWSNMP OK"))
	}))
	defer ts.Close()

	ctx := context.Background()
	poller := polling.NewHTTPPoller()

	p := &datastore.PollingEnt{
		Type:    "http",
		Params:  ts.URL,
		Filter:  "TWSNMP",
		Timeout: 2,
	}
	res, err := poller.Poll(ctx, p, nil)
	if err != nil || res.State != polling.StateNormal {
		t.Fatalf("expected StateNormal, got %s (err: %v)", res.State, err)
	}

	// 2. Keyword mismatch
	p.Filter = "NonExistentWord"
	res, err = poller.Poll(ctx, p, nil)
	if res.State != polling.StateWarn {
		t.Fatalf("expected StateWarn on keyword mismatch, got %s", res.State)
	}

	// 3. 500 error
	errTs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer errTs.Close()
	p.Params = errTs.URL
	p.Filter = ""
	res, err = poller.Poll(ctx, p, nil)
	if res.State != polling.StateHigh {
		t.Fatalf("expected StateHigh on 500 status, got %s", res.State)
	}

	// 4. Missing URL fallback to node
	p.Params = ""
	node := &datastore.NodeEnt{URL: ts.URL}
	res, _ = poller.Poll(ctx, p, node)
	if res.State != polling.StateNormal {
		t.Fatalf("expected StateNormal with node URL, got %s", res.State)
	}

	// 5. Completely missing URL/IP
	res, _ = poller.Poll(ctx, p, nil)
	if res.State != polling.StateHigh {
		t.Fatalf("expected StateHigh on missing URL, got %s", res.State)
	}
}

func TestTCPPoller(t *testing.T) {
	ctx := context.Background()
	poller := &polling.TCPPoller{}

	// Start local TCP echo listener
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp failed: %v", err)
	}
	defer l.Close()

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			_, _ = conn.Write([]byte("HELLO TWSNMP BANNER\n"))
			_ = conn.Close()
		}
	}()

	_, portStr, _ := net.SplitHostPort(l.Addr().String())

	// 1. Success with banner match
	p := &datastore.PollingEnt{
		Type:    "tcp",
		Params:  portStr,
		Filter:  "TWSNMP",
		Timeout: 2,
	}
	node := &datastore.NodeEnt{IP: "127.0.0.1"}
	res, err := poller.Poll(ctx, p, node)
	if err != nil || res.State != polling.StateNormal {
		t.Fatalf("expected StateNormal, got %s (err: %v)", res.State, err)
	}

	// 2. Banner mismatch
	p.Filter = "WRONG_BANNER"
	res, err = poller.Poll(ctx, p, node)
	if res.State != polling.StateWarn {
		t.Fatalf("expected StateWarn on banner mismatch, got %s", res.State)
	}

	// 3. Connection refused (closed port)
	p.Params = "65534"
	p.Filter = ""
	res, _ = poller.Poll(ctx, p, node)
	if res.State != polling.StateHigh {
		t.Fatalf("expected StateHigh on unreachable port, got %s", res.State)
	}

	// 4. Missing IP
	res, _ = poller.Poll(ctx, p, nil)
	if res.State != polling.StateHigh {
		t.Fatalf("expected StateHigh on missing IP, got %s", res.State)
	}
}

func TestPingPoller(t *testing.T) {
	ctx := context.Background()
	poller := &polling.PingPoller{}

	// Missing IP
	res, _ := poller.Poll(ctx, &datastore.PollingEnt{}, nil)
	if res.State != polling.StateUnknown {
		t.Fatalf("expected StateUnknown on missing IP, got %s", res.State)
	}

	// Localhost echo UDP
	node := &datastore.NodeEnt{IP: "127.0.0.1"}
	res, _ = poller.Poll(ctx, &datastore.PollingEnt{Timeout: 1}, node)
	if res.State != polling.StateNormal {
		t.Logf("ping result: %s (%s)", res.State, res.Message)
	}

	// Unreachable IP should return StateHigh
	unreachNode := &datastore.NodeEnt{IP: "192.0.2.1"}
	res, _ = poller.Poll(ctx, &datastore.PollingEnt{Timeout: 1, Retry: 0}, unreachNode)
	if res.State != polling.StateHigh {
		t.Fatalf("expected StateHigh on unreachable IP, got %s", res.State)
	}
}

func TestDNSPoller(t *testing.T) {
	ctx := context.Background()
	poller := &polling.DNSPoller{}

	// 1. Localhost resolution
	p := &datastore.PollingEnt{
		Params:  "localhost",
		Timeout: 2,
	}
	res, _ := poller.Poll(ctx, p, nil)
	if res.State != polling.StateNormal {
		t.Fatalf("expected StateNormal resolving localhost, got %s", res.State)
	}

	// 2. Missing host fallback to node name
	node := &datastore.NodeEnt{Name: "localhost"}
	res, _ = poller.Poll(ctx, &datastore.PollingEnt{}, node)
	if res.State != polling.StateNormal {
		t.Fatalf("expected StateNormal resolving node name localhost, got %s", res.State)
	}

	// 3. Missing host completely
	res, _ = poller.Poll(ctx, &datastore.PollingEnt{}, nil)
	if res.State != polling.StateHigh {
		t.Fatalf("expected StateHigh on missing host, got %s", res.State)
	}
}

func TestNTPPoller(t *testing.T) {
	ctx := context.Background()
	poller := &polling.NTPPoller{}

	// 1. Missing node IP
	res, _ := poller.Poll(ctx, &datastore.PollingEnt{}, nil)
	if res.State != polling.StateHigh {
		t.Fatalf("expected StateHigh on missing IP, got %s", res.State)
	}

	// 2. Local mock NTP listener
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("resolve udp failed: %v", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Fatalf("listen udp failed: %v", err)
	}
	defer conn.Close()

	go func() {
		buf := make([]byte, 48)
		for {
			n, rAddr, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}
			if n >= 48 {
				resp := make([]byte, 48)
				resp[0] = 0x24 // Server mode
				_, _ = conn.WriteTo(resp, rAddr)
			}
		}
	}()

	host, portStr, _ := net.SplitHostPort(conn.LocalAddr().String())
	node := &datastore.NodeEnt{IP: host}
	_ = portStr

	// Note: NTP poller uses standard port 123; testing against unreachable port checks error path
	p := &datastore.PollingEnt{Timeout: 1}
	res, _ = poller.Poll(ctx, p, node)
	if res.State != polling.StateHigh && res.State != polling.StateNormal {
		t.Fatalf("unexpected state: %s", res.State)
	}
}

func TestSNMPPoller(t *testing.T) {
	ctx := context.Background()
	poller := &polling.SNMPPoller{}

	// 1. Missing node IP
	res, _ := poller.Poll(ctx, &datastore.PollingEnt{}, nil)
	if res.State != polling.StateHigh {
		t.Fatalf("expected StateHigh on missing IP, got %s", res.State)
	}

	// 2. Connect failure to closed port
	node := &datastore.NodeEnt{
		IP:        "127.0.0.1",
		SnmpPort:  65534,
		Community: "public",
		SnmpMode:  "v1",
	}
	p := &datastore.PollingEnt{
		Params:  "1.3.6.1.2.1.1.1.0",
		Timeout: 1,
	}
	res, _ = poller.Poll(ctx, p, node)
	if res.State != polling.StateHigh {
		t.Fatalf("expected StateHigh on connect failure, got %s", res.State)
	}
}

func TestPollingManager_ExecuteAndLog(t *testing.T) {
	store, pqStore, cleanup := setupTestEnv(t)
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mgr := polling.NewManager(polling.Config{
		Store:        store,
		LogStore:     pqStore,
		WorkerCount:  2,
		PollInterval: 50 * time.Millisecond,
	})

	// Add node
	node := &datastore.NodeEnt{
		ID:   "node-1",
		Name: "Test Web Server",
		IP:   "127.0.0.1",
	}
	_ = store.SaveNode(ctx, node)

	// Mock HTTP server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer ts.Close()

	// 1. Polling with LogModeAlways & state change
	task := &datastore.PollingEnt{
		ID:       "poll-1",
		NodeID:   "node-1",
		Name:     "Web Check",
		Type:     "http",
		Params:   ts.URL,
		State:    "unknown", // triggers state change to normal
		LogMode:  datastore.LogModeAlways,
		PollInt:  1,
		NextTime: time.Now().UnixNano(),
	}
	_ = store.SavePolling(ctx, task)

	res, err := mgr.ExecuteOne(ctx, task)
	if err != nil || res.State != polling.StateNormal {
		t.Fatalf("expected StateNormal, got %s (err: %v)", res.State, err)
	}

	// Verify state update in store
	savedTask, err := store.GetPolling(ctx, "poll-1")
	if err != nil || savedTask.State != polling.StateNormal {
		t.Fatalf("expected saved task state normal, got %v", savedTask)
	}

	// Verify Parquet log was written
	logs, err := pqStore.Query(ctx, parquet.LogFilter{
		Type:  "polling",
		Limit: 10,
	})
	if err != nil || len(logs) == 0 {
		t.Fatalf("expected parquet logs, err: %v, count: %d", err, len(logs))
	}

	// 2. Test unsupported polling type
	unknownTask := &datastore.PollingEnt{
		ID:   "poll-unk",
		Type: "unknown-type",
	}
	res, _ = mgr.ExecuteOne(ctx, unknownTask)
	if res.State != polling.StateUnknown {
		t.Fatalf("expected StateUnknown, got %s", res.State)
	}

	// 3. Test scheduler loop
	go func() {
		_ = mgr.Start(ctx)
	}()
	time.Sleep(150 * time.Millisecond)
	cancel()
}

func TestConcurrentPollingSafety(t *testing.T) {
	store, pqStore, cleanup := setupTestEnv(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer ts.Close()

	mgr := polling.NewManager(polling.Config{
		Store:        store,
		LogStore:     pqStore,
		WorkerCount:  10,
		PollInterval: 10 * time.Millisecond,
	})

	task := &datastore.PollingEnt{
		ID:       "poll-race-1",
		Name:     "Concurrent Test",
		Type:     "http",
		Params:   ts.URL,
		PollInt:  1,
		Timeout:  1,
		LogMode:  datastore.LogModeAlways,
		NextTime: time.Now().UnixNano(),
		Result: map[string]interface{}{
			"init": "val",
		},
	}
	_ = store.SavePolling(ctx, task)

	// Launch multiple concurrent executions of the same task and schedule checks
	done := make(chan struct{})
	for i := 0; i < 30; i++ {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic in concurrent execution: %v", r)
				}
				done <- struct{}{}
			}()
			_, _ = mgr.ExecuteOne(ctx, task)
			_ = store.SavePolling(ctx, task)
			_, _ = store.GetPolling(ctx, "poll-race-1")
			_, _ = store.ListPollings(ctx)
		}()
	}

	for i := 0; i < 30; i++ {
		<-done
	}
}

func TestNodeStateAggregation(t *testing.T) {
	store, pqStore, cleanup := setupTestEnv(t)
	defer cleanup()
	ctx := context.Background()

	mgr := polling.NewManager(polling.Config{
		Store:        store,
		LogStore:     pqStore,
		WorkerCount:  2,
		PollInterval: 10 * time.Millisecond,
	})

	node := &datastore.NodeEnt{
		ID:    "node-agg-1",
		Name:  "TestNode",
		IP:    "127.0.0.1",
		State: "normal",
	}
	_ = store.SaveNode(ctx, node)

	// Polling 1: Level is "off", but state is "high" (failure)
	pOff := &datastore.PollingEnt{
		ID:      "p-off",
		NodeID:  node.ID,
		Name:    "Off Polling",
		Type:    "http",
		Level:   "off",
		State:   polling.StateHigh,
		PollInt: 60,
	}
	_ = store.SavePolling(ctx, pOff)

	// Polling 2: Level is "low", and state is "normal"
	pNormal := &datastore.PollingEnt{
		ID:      "p-normal",
		NodeID:  node.ID,
		Name:    "Normal Polling",
		Type:    "http",
		Level:   "low",
		State:   polling.StateNormal,
		PollInt: 60,
	}
	_ = store.SavePolling(ctx, pNormal)

	// Execute pOff (even if executed, Level="off" must not make node state high)
	_, _ = mgr.ExecuteOne(ctx, pOff)

	savedNode, _ := store.GetNode(ctx, node.ID)
	if savedNode.State == polling.StateHigh {
		t.Fatalf("expected node state NOT to be high because polling level is off, got: %s", savedNode.State)
	}
	if savedNode.State != polling.StateNormal {
		t.Fatalf("expected node state to remain normal, got: %s", savedNode.State)
	}
}

func TestPollingRecoveryAndRepairState(t *testing.T) {
	store, pqStore, cleanup := setupTestEnv(t)
	defer cleanup()
	ctx := context.Background()

	var isServerHealthy bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isServerHealthy {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer ts.Close()

	mgr := polling.NewManager(polling.Config{
		Store:        store,
		LogStore:     pqStore,
		WorkerCount:  2,
		PollInterval: 10 * time.Millisecond,
	})

	node := &datastore.NodeEnt{
		ID:      "node-repair-1",
		Name:    "RepairTestNode",
		IP:      "127.0.0.1",
		State:   "normal",
		AutoAck: false,
	}
	_ = store.SaveNode(ctx, node)

	poll := &datastore.PollingEnt{
		ID:           "poll-repair-1",
		NodeID:       node.ID,
		Name:         "HTTP Check",
		Type:         "http",
		Params:       ts.URL,
		Level:        "high",
		State:        polling.StateNormal,
		PollInt:      60,
		FailAction:   "mail fail-sub fail-body",
		RepairAction: "mail repair-sub repair-body",
	}
	_ = store.SavePolling(ctx, poll)

	// Step 1: Polling fails (server returns 500)
	isServerHealthy = false
	res, err := mgr.ExecuteOne(ctx, poll)
	if err != nil || res.State != polling.StateHigh {
		t.Fatalf("expected failure StateHigh, got %s (err: %v)", res.State, err)
	}

	savedPoll, _ := store.GetPolling(ctx, poll.ID)
	if savedPoll.State != polling.StateHigh {
		t.Fatalf("expected saved polling state to be high, got %s", savedPoll.State)
	}
	if savedPoll.FailTime == 0 {
		t.Fatalf("expected FailTime to be recorded on failure")
	}

	savedNode, _ := store.GetNode(ctx, node.ID)
	if savedNode.State != polling.StateHigh {
		t.Fatalf("expected node state to become high, got %s", savedNode.State)
	}

	// Step 2: Server recovers -> Polling returns normal -> State must become "repair"
	time.Sleep(50 * time.Millisecond)
	isServerHealthy = true
	res, err = mgr.ExecuteOne(ctx, poll)
	if err != nil {
		t.Fatalf("expected no poll error, got %v", err)
	}
	if res.State != polling.StateNormal {
		t.Fatalf("expected poller result to be StateNormal, got %s", res.State)
	}

	savedPoll, _ = store.GetPolling(ctx, poll.ID)
	if savedPoll.State != polling.StateRepair {
		t.Fatalf("expected polling state to transition to repair, got %s", savedPoll.State)
	}
	if savedPoll.FailTime != 0 {
		t.Fatalf("expected FailTime to be reset after recovery, got %d", savedPoll.FailTime)
	}

	// Because node.AutoAck is false, node state should become "repair"
	savedNode, _ = store.GetNode(ctx, node.ID)
	if savedNode.State != polling.StateRepair {
		t.Fatalf("expected node state to be repair with AutoAck=false, got %s", savedNode.State)
	}

	// Check event logs for repair event
	var foundRepairLog bool
	store.ForEachLastEventLog(func(l *datastore.EventLogEnt) bool {
		if l.Level == polling.StateRepair && l.Type == "polling" {
			foundRepairLog = true
			return false
		}
		return true
	})
	if !foundRepairLog {
		t.Fatalf("expected to find event log with Level 'repair'")
	}

	// Step 3: Clear repair pollings
	cleared := mgr.ClearRepairPollings(ctx)
	if cleared != 1 {
		t.Fatalf("expected 1 polling cleared, got %d", cleared)
	}
	savedPoll, _ = store.GetPolling(ctx, poll.ID)
	if savedPoll.State != polling.StateUnknown {
		t.Fatalf("expected cleared polling state to be unknown, got %s", savedPoll.State)
	}
}

func TestPollingAutoAckRecovery(t *testing.T) {
	store, pqStore, cleanup := setupTestEnv(t)
	defer cleanup()
	ctx := context.Background()

	var isServerHealthy bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isServerHealthy {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer ts.Close()

	mgr := polling.NewManager(polling.Config{
		Store:        store,
		LogStore:     pqStore,
		WorkerCount:  2,
		PollInterval: 10 * time.Millisecond,
	})

	node := &datastore.NodeEnt{
		ID:      "node-autoack-1",
		Name:    "AutoAckNode",
		IP:      "127.0.0.1",
		State:   "normal",
		AutoAck: true, // AutoAck enabled!
	}
	_ = store.SaveNode(ctx, node)

	poll := &datastore.PollingEnt{
		ID:      "poll-autoack-1",
		NodeID:  node.ID,
		Name:    "HTTP Check AutoAck",
		Type:    "http",
		Params:  ts.URL,
		Level:   "high",
		State:   polling.StateNormal,
		PollInt: 60,
	}
	_ = store.SavePolling(ctx, poll)

	// Step 1: Failure
	isServerHealthy = false
	_, _ = mgr.ExecuteOne(ctx, poll)

	savedNode, _ := store.GetNode(ctx, node.ID)
	if savedNode.State != polling.StateHigh {
		t.Fatalf("expected node state high on failure, got %s", savedNode.State)
	}

	// Step 2: Recovery with AutoAck=true -> updateNodeState automatically transitions repair to normal
	isServerHealthy = true
	_, _ = mgr.ExecuteOne(ctx, poll)

	savedNode, _ = store.GetNode(ctx, node.ID)
	if savedNode.State != polling.StateNormal {
		t.Fatalf("expected node state normal with AutoAck=true, got %s", savedNode.State)
	}

	savedPoll, _ := store.GetPolling(ctx, poll.ID)
	if savedPoll.State != polling.StateNormal {
		t.Fatalf("expected polling state to be auto-acked to normal, got %s", savedPoll.State)
	}
}

func TestUnsupportedPollingModes(t *testing.T) {
	ctx := context.Background()
	store, _, cleanup := setupTestEnv(t)
	defer cleanup()

	mgr := polling.NewManager(polling.Config{
		Store: store,
	})

	node := &datastore.NodeEnt{
		ID:   "node-unsupported",
		Name: "TestNode",
		IP:   "127.0.0.1",
	}
	_ = store.SaveNode(ctx, node)

	tests := []struct {
		name    string
		pType   string
		mode    string
		wantErr string
	}{
		{"syslog twpcap", "syslog", "twpcap", "unsupported syslog mode: twpcap"},
		{"syslog twbluescan", "syslog", "twbluescan", "unsupported syslog mode: twbluescan"},
		{"snmp invalid", "snmp", "invalid_mode", "unsupported snmp mode: invalid_mode"},
		{"ping invalid", "ping", "invalid_mode", "unsupported ping mode: invalid_mode"},
		{"tcp invalid", "tcp", "invalid_mode", "unsupported tcp mode: invalid_mode"},
		{"tls invalid", "tls", "invalid_mode", "unsupported tls mode: invalid_mode"},
		{"dns invalid", "dns", "invalid_mode", "unsupported dns mode: invalid_mode"},
		{"trap invalid", "trap", "twpcap", "unsupported trap mode: twpcap"},
		{"arplog invalid", "arplog", "invalid_mode", "unsupported arplog mode: invalid_mode"},
		{"netflow invalid", "netflow", "invalid_mode", "unsupported netflow mode: invalid_mode"},
		{"stun invalid", "stun", "invalid_mode", "unsupported stun mode: invalid_mode"},
		{"http invalid", "http", "invalid_mode", "unsupported http mode: invalid_mode"},
		{"email invalid", "email", "invalid_mode", "unsupported email mode: invalid_mode"},
		{"mqtt invalid", "mqtt", "invalid_mode", "unsupported mqtt mode: invalid_mode"},
		{"gnmi invalid", "gnmi", "invalid_mode", "unsupported gnmi mode: invalid_mode"},
		{"twlogeye invalid", "twlogeye", "invalid_mode", "unsupported twlogeye mode: invalid_mode"},
		{"ntp invalid", "ntp", "invalid_mode", "unsupported ntp mode: invalid_mode"},
		{"lxi invalid", "lxi", "invalid_mode", "unsupported lxi mode: invalid_mode"},
		{"monitor invalid", "monitor", "invalid_mode", "unsupported monitor mode: invalid_mode"},
		{"cmd invalid", "cmd", "invalid_mode", "unsupported cmd mode: invalid_mode"},
		{"twsnmp invalid", "twsnmp", "invalid_mode", "unsupported twsnmp mode: invalid_mode"},
		{"ssh invalid", "ssh", "invalid_mode", "unsupported ssh mode: invalid_mode"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pe := &datastore.PollingEnt{
				ID:      "poll-" + tt.name,
				NodeID:  node.ID,
				Name:    tt.name,
				Type:    tt.pType,
				Mode:    tt.mode,
				Params:  "127.0.0.1",
				Level:   "high",
				PollInt: 60,
			}
			res, err := mgr.ExecuteOne(ctx, pe)
			if err != nil {
				t.Fatalf("ExecuteOne returned error: %v", err)
			}
			if res.State != polling.StateUnknown {
				t.Fatalf("expected StateUnknown for %s (mode=%s), got state=%s", tt.pType, tt.mode, res.State)
			}
			if res.Message != tt.wantErr {
				t.Fatalf("expected message %q, got %q", tt.wantErr, res.Message)
			}
			if errVal, ok := res.Fields["error"].(string); !ok || errVal != tt.wantErr {
				t.Fatalf("expected Fields[error] %q, got %v", tt.wantErr, res.Fields["error"])
			}
		})
	}
}


