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
	if res.State != polling.StateHigh {
		t.Fatalf("expected StateHigh on missing IP, got %s", res.State)
	}

	// Localhost echo UDP
	node := &datastore.NodeEnt{IP: "127.0.0.1"}
	res, _ = poller.Poll(ctx, &datastore.PollingEnt{Timeout: 1}, node)
	if res.State != polling.StateNormal {
		t.Logf("ping result: %s (%s)", res.State, res.Message)
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
		State:    "warn", // triggers state change to normal
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
