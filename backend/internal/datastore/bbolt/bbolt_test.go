package bbolt_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/bbolt"
)

func setupTestStore(t *testing.T) (*bbolt.Store, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "twsnmpneo-bbolt-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, "test.db")
	store, err := bbolt.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	cleanup := func() {
		_ = store.Close()
		_ = os.RemoveAll(dir)
	}
	return store, cleanup
}

func TestStore_NodeOperations(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	// 1. Save and Get
	node := &datastore.NodeEnt{
		Name:     "Test Router",
		IP:       "192.168.1.1",
		SnmpMode: "v2c",
	}
	if err := store.SaveNode(ctx, node); err != nil {
		t.Fatalf("save node failed: %v", err)
	}
	if node.ID == "" {
		t.Fatal("expected node.ID to be populated")
	}

	got, err := store.GetNode(ctx, node.ID)
	if err != nil {
		t.Fatalf("get node failed: %v", err)
	}
	if got.Name != "Test Router" || got.IP != "192.168.1.1" {
		t.Errorf("unexpected node data: %+v", got)
	}

	// 2. List
	nodes, err := store.ListNodes(ctx)
	if err != nil {
		t.Fatalf("list nodes failed: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}

	// 3. Delete
	if err := store.DeleteNode(ctx, node.ID); err != nil {
		t.Fatalf("delete node failed: %v", err)
	}
	_, err = store.GetNode(ctx, node.ID)
	if err != datastore.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestStore_LineOperations(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	line := &datastore.LineEnt{
		NodeID1: "n1",
		NodeID2: "n2",
		Width:   2,
	}
	if err := store.SaveLine(ctx, line); err != nil {
		t.Fatalf("save line failed: %v", err)
	}

	got, err := store.GetLine(ctx, line.ID)
	if err != nil {
		t.Fatalf("get line failed: %v", err)
	}
	if got.NodeID1 != "n1" || got.Width != 2 {
		t.Errorf("unexpected line data: %+v", got)
	}

	lines, err := store.ListLines(ctx)
	if err != nil || len(lines) != 1 {
		t.Fatalf("list lines failed: %v, count=%d", err, len(lines))
	}

	if err := store.DeleteLine(ctx, line.ID); err != nil {
		t.Fatalf("delete line failed: %v", err)
	}
	_, err = store.GetLine(ctx, line.ID)
	if err != datastore.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestStore_NetworkOperations(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	nw := &datastore.NetworkEnt{
		Name: "Core Switch",
		IP:   "10.0.0.1",
	}
	if err := store.SaveNetwork(ctx, nw); err != nil {
		t.Fatalf("save network failed: %v", err)
	}

	got, err := store.GetNetwork(ctx, nw.ID)
	if err != nil {
		t.Fatalf("get network failed: %v", err)
	}
	if got.Name != "Core Switch" {
		t.Errorf("unexpected network data: %+v", got)
	}

	list, err := store.ListNetworks(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list networks failed: %v, count=%d", err, len(list))
	}

	if err := store.DeleteNetwork(ctx, nw.ID); err != nil {
		t.Fatalf("delete network failed: %v", err)
	}
	_, err = store.GetNetwork(ctx, nw.ID)
	if err != datastore.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestStore_DrawItemOperations(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	item := &datastore.DrawItemEnt{
		Type: datastore.DrawItemTypeRect,
		Text: "Server Rack A",
	}
	if err := store.SaveDrawItem(ctx, item); err != nil {
		t.Fatalf("save draw item failed: %v", err)
	}

	got, err := store.GetDrawItem(ctx, item.ID)
	if err != nil {
		t.Fatalf("get draw item failed: %v", err)
	}
	if got.Text != "Server Rack A" {
		t.Errorf("unexpected draw item data: %+v", got)
	}

	items, err := store.ListDrawItems(ctx)
	if err != nil || len(items) != 1 {
		t.Fatalf("list draw items failed: %v, count=%d", err, len(items))
	}

	if err := store.DeleteDrawItem(ctx, item.ID); err != nil {
		t.Fatalf("delete draw item failed: %v", err)
	}
	_, err = store.GetDrawItem(ctx, item.ID)
	if err != datastore.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestStore_PollingOperations(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	p := &datastore.PollingEnt{
		Name:    "Ping",
		NodeID:  "node-1",
		Type:    "ping",
		PollInt: 60,
	}
	if err := store.SavePolling(ctx, p); err != nil {
		t.Fatalf("save polling failed: %v", err)
	}

	got, err := store.GetPolling(ctx, p.ID)
	if err != nil {
		t.Fatalf("get polling failed: %v", err)
	}
	if got.Name != "Ping" || got.Type != "ping" {
		t.Errorf("unexpected polling data: %+v", got)
	}

	list, err := store.ListPollings(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list pollings failed: %v, count=%d", err, len(list))
	}

	if err := store.DeletePolling(ctx, p.ID); err != nil {
		t.Fatalf("delete polling failed: %v", err)
	}
	_, err = store.GetPolling(ctx, p.ID)
	if err != datastore.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestStore_Configs(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	// MapConf
	mapConf, err := store.GetMapConf(ctx)
	if err != nil {
		t.Fatalf("get default map conf failed: %v", err)
	}
	if mapConf.MapName != "TWSNMP NEO" {
		t.Errorf("unexpected default map name: %s", mapConf.MapName)
	}
	mapConf.MapName = "Custom Map"
	if err := store.SaveMapConf(ctx, mapConf); err != nil {
		t.Fatalf("save map conf failed: %v", err)
	}
	gotMap, _ := store.GetMapConf(ctx)
	if gotMap.MapName != "Custom Map" {
		t.Errorf("expected Custom Map, got %s", gotMap.MapName)
	}

	// NotifyConf
	notifyConf := &datastore.NotifyConfEnt{
		Provider: "smtp",
		Subject:  "Alert Notification",
	}
	if err := store.SaveNotifyConf(ctx, notifyConf); err != nil {
		t.Fatalf("save notify conf failed: %v", err)
	}
	gotNotify, _ := store.GetNotifyConf(ctx)
	if gotNotify.Subject != "Alert Notification" {
		t.Errorf("expected Alert Notification, got %s", gotNotify.Subject)
	}

	// LocConf
	locConf := &datastore.LocConfEnt{
		Center: "139.69,35.68",
		Zoom:   12,
	}
	if err := store.SaveLocConf(ctx, locConf); err != nil {
		t.Fatalf("save loc conf failed: %v", err)
	}
	gotLoc, _ := store.GetLocConf(ctx)
	if gotLoc.Center != "139.69,35.68" {
		t.Errorf("expected 139.69,35.68, got %s", gotLoc.Center)
	}
}

func TestStore_EventLogs(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	ev1 := &datastore.EventLogEnt{
		Time:     time.Now().UnixNano() - 1000,
		Type:     "node",
		Level:    "warn",
		NodeName: "Node A",
		Event:    "High CPU",
	}
	ev2 := &datastore.EventLogEnt{
		Time:     time.Now().UnixNano(),
		Type:     "node",
		Level:    "high",
		NodeName: "Node B",
		Event:    "Ping Timeout",
	}
	if err := store.AddEventLog(ctx, ev1); err != nil {
		t.Fatalf("add ev1 failed: %v", err)
	}
	if err := store.AddEventLog(ctx, ev2); err != nil {
		t.Fatalf("add ev2 failed: %v", err)
	}

	logs, err := store.ListEventLogs(ctx, 10)
	if err != nil {
		t.Fatalf("list event logs failed: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(logs))
	}
	// Descending order check (latest first)
	if logs[0].Event != "Ping Timeout" {
		t.Errorf("expected first log to be Ping Timeout, got %s", logs[0].Event)
	}

	// Query with level filter
	qHigh, err := store.QueryEventLogs(ctx, datastore.EventLogFilter{Level: "high"})
	if err != nil || len(qHigh) != 1 || qHigh[0].Event != "Ping Timeout" {
		t.Fatalf("query high level logs failed: %v, count=%d", err, len(qHigh))
	}

	// Query with keyword filter
	qCPU, err := store.QueryEventLogs(ctx, datastore.EventLogFilter{Filter: "CPU"})
	if err != nil || len(qCPU) != 1 || qCPU[0].Event != "High CPU" {
		t.Fatalf("query keyword filter logs failed: %v, count=%d", err, len(qCPU))
	}

	// Delete all event logs
	if err := store.DeleteEventLogs(ctx); err != nil {
		t.Fatalf("delete event logs failed: %v", err)
	}
	delLogs, err := store.ListEventLogs(ctx, 10)
	if err != nil || len(delLogs) != 0 {
		t.Fatalf("expected 0 logs after delete, got %d (err=%v)", len(delLogs), err)
	}
}

func TestStore_PersistenceReload(t *testing.T) {
	dir, err := os.MkdirTemp("", "twsnmpneo-persist-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)
	dbPath := filepath.Join(dir, "persist.db")
	ctx := context.Background()

	// 1. Open and write
	store1, err := bbolt.New(dbPath)
	if err != nil {
		t.Fatalf("open store1 failed: %v", err)
	}
	_ = store1.SaveNode(ctx, &datastore.NodeEnt{ID: "n1", Name: "Server 1"})
	_ = store1.Close()

	// 2. Re-open and verify
	store2, err := bbolt.New(dbPath)
	if err != nil {
		t.Fatalf("open store2 failed: %v", err)
	}
	defer store2.Close()

	n, err := store2.GetNode(ctx, "n1")
	if err != nil {
		t.Fatalf("get node after reload failed: %v", err)
	}
	if n.Name != "Server 1" {
		t.Errorf("expected Server 1, got %s", n.Name)
	}
}

func TestStore_ErrorCases(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	// nil params
	if err := store.SaveNode(ctx, nil); err != datastore.ErrInvalidParams {
		t.Errorf("expected ErrInvalidParams, got %v", err)
	}
	if err := store.DeleteNode(ctx, ""); err != datastore.ErrInvalidID {
		t.Errorf("expected ErrInvalidID, got %v", err)
	}
	if err := store.SaveLine(ctx, nil); err != datastore.ErrInvalidParams {
		t.Errorf("expected ErrInvalidParams, got %v", err)
	}
	if err := store.DeleteLine(ctx, ""); err != datastore.ErrInvalidID {
		t.Errorf("expected ErrInvalidID, got %v", err)
	}
	if err := store.SaveNetwork(ctx, nil); err != datastore.ErrInvalidParams {
		t.Errorf("expected ErrInvalidParams, got %v", err)
	}
	if err := store.DeleteNetwork(ctx, ""); err != datastore.ErrInvalidID {
		t.Errorf("expected ErrInvalidID, got %v", err)
	}
	if err := store.SaveDrawItem(ctx, nil); err != datastore.ErrInvalidParams {
		t.Errorf("expected ErrInvalidParams, got %v", err)
	}
	if err := store.DeleteDrawItem(ctx, ""); err != datastore.ErrInvalidID {
		t.Errorf("expected ErrInvalidID, got %v", err)
	}
	if err := store.SavePolling(ctx, nil); err != datastore.ErrInvalidParams {
		t.Errorf("expected ErrInvalidParams, got %v", err)
	}
	if err := store.DeletePolling(ctx, ""); err != datastore.ErrInvalidID {
		t.Errorf("expected ErrInvalidID, got %v", err)
	}
	if err := store.SaveMapConf(ctx, nil); err != datastore.ErrInvalidParams {
		t.Errorf("expected ErrInvalidParams, got %v", err)
	}
	if err := store.SaveNotifyConf(ctx, nil); err != datastore.ErrInvalidParams {
		t.Errorf("expected ErrInvalidParams, got %v", err)
	}
	if err := store.SaveLocConf(ctx, nil); err != datastore.ErrInvalidParams {
		t.Errorf("expected ErrInvalidParams, got %v", err)
	}
	if err := store.AddEventLog(ctx, nil); err != datastore.ErrInvalidParams {
		t.Errorf("expected ErrInvalidParams, got %v", err)
	}

	// not found cases
	if _, err := store.GetLine(ctx, "nonexistent"); err != datastore.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
	if _, err := store.GetNetwork(ctx, "nonexistent"); err != datastore.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
	if _, err := store.GetDrawItem(ctx, "nonexistent"); err != datastore.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
	if _, err := store.GetPolling(ctx, "nonexistent"); err != datastore.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestStore_DeleteNodeCascade(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	// Create Node 1 and Node 2
	n1 := &datastore.NodeEnt{Name: "Node1", IP: "192.168.1.1"}
	if err := store.SaveNode(ctx, n1); err != nil {
		t.Fatalf("save node 1 failed: %v", err)
	}
	n2 := &datastore.NodeEnt{Name: "Node2", IP: "192.168.1.2"}
	if err := store.SaveNode(ctx, n2); err != nil {
		t.Fatalf("save node 2 failed: %v", err)
	}

	// Create Polling on Node 1
	p1 := &datastore.PollingEnt{Name: "Ping1", NodeID: n1.ID, Type: "ping"}
	if err := store.SavePolling(ctx, p1); err != nil {
		t.Fatalf("save polling failed: %v", err)
	}

	// Create Line between Node 1 and Node 2 referencing Polling 1
	l1 := &datastore.LineEnt{NodeID1: n1.ID, NodeID2: n2.ID, PollingID: p1.ID}
	if err := store.SaveLine(ctx, l1); err != nil {
		t.Fatalf("save line failed: %v", err)
	}

	// Verify they all exist
	if _, err := store.GetNode(ctx, n1.ID); err != nil {
		t.Fatalf("node 1 not found")
	}
	if _, err := store.GetPolling(ctx, p1.ID); err != nil {
		t.Fatalf("polling 1 not found")
	}
	if _, err := store.GetLine(ctx, l1.ID); err != nil {
		t.Fatalf("line 1 not found")
	}

	// Delete Node 1
	if err := store.DeleteNode(ctx, n1.ID); err != nil {
		t.Fatalf("delete node 1 failed: %v", err)
	}

	// Verify Node 1 is deleted
	if _, err := store.GetNode(ctx, n1.ID); err != datastore.ErrNotFound {
		t.Errorf("expected node 1 to be deleted, got %v", err)
	}

	// Verify Polling 1 was cascade deleted
	if _, err := store.GetPolling(ctx, p1.ID); err != datastore.ErrNotFound {
		t.Errorf("expected polling 1 to be cascade deleted, got %v", err)
	}

	// Verify Line 1 was cascade deleted
	if _, err := store.GetLine(ctx, l1.ID); err != datastore.ErrNotFound {
		t.Errorf("expected line 1 to be cascade deleted, got %v", err)
	}

	// Verify Node 2 still exists
	if _, err := store.GetNode(ctx, n2.ID); err != nil {
		t.Errorf("expected node 2 to still exist, got %v", err)
	}
}

func TestStore_CleanupOrphansOnLoad(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "orphan_test.db")
	ctx := context.Background()

	// 1. First store instance: save nodes, polling, and line
	s1, err := bbolt.New(dbPath)
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}

	n1 := &datastore.NodeEnt{Name: "Node1", IP: "10.0.0.1"}
	_ = s1.SaveNode(ctx, n1)
	n2 := &datastore.NodeEnt{Name: "Node2", IP: "10.0.0.2"}
	_ = s1.SaveNode(ctx, n2)

	p := &datastore.PollingEnt{Name: "TempPoll", NodeID: n1.ID, Type: "ping"}
	_ = s1.SavePolling(ctx, p)

	l := &datastore.LineEnt{NodeID1: n1.ID, NodeID2: n2.ID}
	_ = s1.SaveLine(ctx, l)

	// Delete Node 1 with cascade
	if err := s1.DeleteNode(ctx, n1.ID); err != nil {
		t.Fatalf("delete node failed: %v", err)
	}
	_ = s1.Close()

	// 2. Second store instance: opening should confirm no orphaned polling or line
	s2, err := bbolt.New(dbPath)
	if err != nil {
		t.Fatalf("reopen store failed: %v", err)
	}
	defer s2.Close()

	if _, err := s2.GetPolling(ctx, p.ID); err != datastore.ErrNotFound {
		t.Errorf("expected polling to remain deleted on reload, got %v", err)
	}
	if _, err := s2.GetLine(ctx, l.ID); err != datastore.ErrNotFound {
		t.Errorf("expected line to remain deleted on reload, got %v", err)
	}
}

