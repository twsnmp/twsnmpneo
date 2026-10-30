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
