package monitor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMonitor(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "twsnmpneo_monitor_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create dummy db file
	dummyDB := filepath.Join(tempDir, "twsnmpneo.db")
	if err := os.WriteFile(dummyDB, []byte("bbolt dummy database content"), 0600); err != nil {
		t.Fatalf("failed to create dummy db: %v", err)
	}

	mon := New(Config{
		DataDir:  tempDir,
		Store:    nil,
		Interval: 100 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mon.Start(ctx)
	time.Sleep(250 * time.Millisecond)

	data := mon.GetData()
	if len(data) == 0 {
		t.Fatalf("expected monitor data to have at least 1 record, got 0")
	}

	latest := data[len(data)-1]
	if latest.Time <= 0 {
		t.Errorf("invalid timestamp: %d", latest.Time)
	}
	if latest.DBSize <= 0 {
		t.Errorf("expected positive DBSize, got %d", latest.DBSize)
	}

	backupFile, size, err := mon.Backup()
	if err != nil {
		t.Fatalf("failed to backup: %v", err)
	}
	if size <= 0 || backupFile == "" {
		t.Errorf("invalid backup result: file=%s, size=%d", backupFile, size)
	}
}
