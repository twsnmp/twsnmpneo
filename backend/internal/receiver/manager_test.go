package receiver

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/bbolt"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
)

func TestManager_CleanOldData(t *testing.T) {
	dir, err := os.MkdirTemp("", "twsnmpneo-manager-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	bStore, err := bbolt.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("create bbolt store: %v", err)
	}
	defer bStore.Close()

	pqStore, err := parquet.New(parquet.Config{
		Dir:            filepath.Join(dir, "logs"),
		BufferSize:     1,
		BufferInterval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("create parquet store: %v", err)
	}
	defer pqStore.Close()

	ctx := context.Background()

	// Configure different retentions: 20 days for EventLog, 5 days for Parquet
	conf := &datastore.MapConfEnt{
		EventLogDays:   20,
		ParquetLogDays: 5,
		LogDays:        14,
	}
	if err := bStore.SaveMapConf(ctx, conf); err != nil {
		t.Fatalf("save map conf: %v", err)
	}

	// 1. Add event logs to bbolt (one 30 days ago, one 2 days ago)
	oldEvent := &datastore.EventLogEnt{
		Time:     time.Now().AddDate(0, 0, -30).UnixNano(),
		Type:     "system",
		Level:    "warn",
		NodeName: "Node-Old",
		Event:    "Old event",
	}
	recentEvent := &datastore.EventLogEnt{
		Time:     time.Now().AddDate(0, 0, -2).UnixNano(),
		Type:     "system",
		Level:    "info",
		NodeName: "Node-Recent",
		Event:    "Recent event",
	}
	_ = bStore.AddEventLog(ctx, oldEvent)
	_ = bStore.AddEventLog(ctx, recentEvent)

	// 2. Add log to parquet and simulate old file
	_ = pqStore.WriteLog(&parquet.ParquetLogRecord{
		Type: "syslog",
		Src:  "192.168.1.1",
		Log:  "test syslog message",
		Time: time.Now().UnixNano(),
	})
	_ = pqStore.Flush()

	// Create an artificial old parquet file in logs/syslog
	syslogDir := filepath.Join(dir, "logs", "syslog")
	oldFile := filepath.Join(syslogDir, "syslog_old.parquet")
	if err := os.WriteFile(oldFile, []byte("fake-parquet-content"), 0644); err != nil {
		t.Fatalf("write old parquet file: %v", err)
	}
	// Set mtime to 10 days ago (older than 5 days ParquetLogDays)
	oldTime := time.Now().AddDate(0, 0, -10)
	_ = os.Chtimes(oldFile, oldTime, oldTime)

	// Create manager and execute cleanOldData
	mgr := NewManager(Config{
		Store:    bStore,
		LogStore: pqStore,
	})

	mgr.cleanOldData(ctx)

	// Verify bbolt event logs: 30-day old event should be gone, 2-day old retained
	events, err := bStore.ListEventLogs(ctx, 10)
	if err != nil {
		t.Fatalf("list event logs: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 remaining event log, got %d", len(events))
	}
	if events[0].Event != "Recent event" {
		t.Errorf("expected recent event, got: %s", events[0].Event)
	}

	// Verify old parquet file was deleted
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Errorf("expected old parquet file to be deleted, but it still exists")
	}
}

func TestManager_UpdateConf(t *testing.T) {
	dir, err := os.MkdirTemp("", "twsnmpneo-updateconf-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	bStore, err := bbolt.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("create bbolt store: %v", err)
	}
	defer bStore.Close()

	pqStore, err := parquet.New(parquet.Config{
		Dir:            filepath.Join(dir, "logs"),
		BufferSize:     1,
		BufferInterval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("create parquet store: %v", err)
	}
	defer pqStore.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initial configuration: Syslog and Trap enabled; NetFlow, sFlow, ARP, OTel, MQTT disabled
	mgr := NewManager(Config{
		Store:              bStore,
		LogStore:           pqStore,
		SyslogUDP:          60100,
		SyslogTCP:          0,
		TrapPort:           60101,
		NetFlowPort:        60102,
		SFlowPort:          60103,
		OTelPort:           60104,
		MQTTPort:           60105,
		EnableSyslogd:      true,
		EnableTrapd:        true,
		EnableNetflowd:     false,
		EnableSFlowd:       false,
		EnableArpWatch:     false,
		EnableOTel:         false,
		EnableMqtt:         false,
		HasExplicitEnables: true,
	})

	errCh := make(chan error, 1)
	go func() {
		errCh <- mgr.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Verify initial status
	st1 := mgr.GetReceiverStatus(false)
	syslogSt, _ := st1["syslog"].(map[string]any)
	if syslogSt["status"] != "running" {
		t.Fatalf("expected syslog running initially, got: %v", syslogSt)
	}
	trapSt, _ := st1["trap"].(map[string]any)
	if trapSt["status"] != "running" {
		t.Fatalf("expected trap running initially, got: %v", trapSt)
	}
	netflowSt, _ := st1["netflow"].(map[string]any)
	if netflowSt["status"] != "stopped" {
		t.Fatalf("expected netflow stopped initially, got: %v", netflowSt)
	}
	sflowSt, _ := st1["sflow"].(map[string]any)
	if sflowSt["status"] != "stopped" {
		t.Fatalf("expected sflow stopped initially, got: %v", sflowSt)
	}
	arpSt, _ := st1["arp"].(map[string]any)
	if arpSt["status"] != "disabled" {
		t.Fatalf("expected arp disabled initially, got: %v", arpSt)
	}

	// Dynamically update config: turn off Syslog, Trap; turn on NetFlow, sFlow
	mgr.UpdateConf(&datastore.MapConfEnt{
		EnableSyslogd:  false,
		EnableTrapd:    false,
		EnableNetflowd: true,
		EnableSFlowd:   true,
		EnableArpWatch: false,
		EnableOTel:     false,
		EnableMqtt:     false,
	})

	time.Sleep(100 * time.Millisecond)

	// Verify updated status
	st2 := mgr.GetReceiverStatus(false)
	syslogSt2, _ := st2["syslog"].(map[string]any)
	if syslogSt2["status"] != "stopped" {
		t.Errorf("expected syslog stopped after update, got: %v", syslogSt2)
	}
	trapSt2, _ := st2["trap"].(map[string]any)
	if trapSt2["status"] != "stopped" {
		t.Errorf("expected trap stopped after update, got: %v", trapSt2)
	}
	netflowSt2, _ := st2["netflow"].(map[string]any)
	if netflowSt2["status"] != "running" {
		t.Errorf("expected netflow running after update, got: %v", netflowSt2)
	}
	sflowSt2, _ := st2["sflow"].(map[string]any)
	if sflowSt2["status"] != "running" {
		t.Errorf("expected sflow running after update, got: %v", sflowSt2)
	}
	arpSt2, _ := st2["arp"].(map[string]any)
	if arpSt2["status"] != "disabled" {
		t.Errorf("expected arp disabled after update, got: %v", arpSt2)
	}

	// Stop manager
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("manager start error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("manager did not shut down in time")
	}

	// Verify event logs were recorded for the receiver stop/start events
	events, err := bStore.ListEventLogs(context.Background(), 100)
	if err != nil {
		t.Fatalf("list event logs: %v", err)
	}
	hasStopSyslog := false
	hasStartNetflow := false
	for _, e := range events {
		if e.Event == "Stopped Syslog receiver" || e.Event == "Syslog受信を停止しました" {
			hasStopSyslog = true
		}
		if e.Event == "Started NetFlow receiver" || e.Event == "NetFlow受信を開始しました" {
			hasStartNetflow = true
		}
	}
	if !hasStopSyslog {
		t.Errorf("expected Stop Syslog event log, got: %v", events)
	}
	if !hasStartNetflow {
		t.Errorf("expected Start NetFlow event log, got: %v", events)
	}
}
