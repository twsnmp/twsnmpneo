package bbolt

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

func TestBackupAndRestore(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	backupDir := filepath.Join(tmpDir, "backup")

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// 1. Test BackupConf default, save and get
	conf, err := store.GetBackupConf(ctx)
	if err != nil {
		t.Fatalf("GetBackupConf failed: %v", err)
	}
	if conf.ScheduleTime != "03:00" {
		t.Errorf("expected default ScheduleTime 03:00, got %s", conf.ScheduleTime)
	}

	conf.Mode = "daily"
	conf.ScheduleTime = "04:30"
	conf.ConfigOnly = true
	conf.Generation = 3
	if err := store.SaveBackupConf(ctx, conf); err != nil {
		t.Fatalf("SaveBackupConf failed: %v", err)
	}

	conf2, err := store.GetBackupConf(ctx)
	if err != nil {
		t.Fatalf("GetBackupConf failed: %v", err)
	}
	if conf2.Mode != "daily" || conf2.ScheduleTime != "04:30" || !conf2.ConfigOnly || conf2.Generation != 3 {
		t.Errorf("loaded config mismatch: %+v", conf2)
	}

	// 2. Add some test data
	node := &datastore.NodeEnt{
		ID:    "node1",
		Name:  "TestNode",
		IP:    "192.168.1.1",
		State: "normal",
	}
	if err := store.SaveNode(ctx, node); err != nil {
		t.Fatalf("SaveNode failed: %v", err)
	}
	_ = store.AddEventLog(ctx, &datastore.EventLogEnt{
		Time:  time.Now().UnixNano(),
		Type:  "system",
		Level: "info",
		Event: "Test event",
	})

	// 3. Test Full Backup
	fullBackupFile := filepath.Join(backupDir, "twsnmpneo.db.20261030010000")
	fullSize, err := store.BackupDB(ctx, fullBackupFile, false)
	if err != nil {
		t.Fatalf("Full BackupDB failed: %v", err)
	}
	if fullSize <= 0 {
		t.Errorf("expected positive full backup size, got %d", fullSize)
	}

	// 4. Test ConfigOnly Backup
	configBackupFile := filepath.Join(backupDir, "twsnmpneo.db.config.20261030020000")
	configSize, err := store.BackupDB(ctx, configBackupFile, true)
	if err != nil {
		t.Fatalf("ConfigOnly BackupDB failed: %v", err)
	}
	if configSize <= 0 {
		t.Errorf("expected positive config-only backup size, got %d", configSize)
	}

	// 5. Test ListBackups
	list, err := store.ListBackups(backupDir)
	if err != nil {
		t.Fatalf("ListBackups failed: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 backup files, got %d", len(list))
	}

	// 6. Test Rotation
	// Create more dummy backup files to test rotation
	for i := 1; i <= 5; i++ {
		name := filepath.Join(backupDir, "twsnmpneo.db.old_"+string(rune('a'+i)))
		_ = os.WriteFile(name, []byte("dummy"), 0600)
		time.Sleep(10 * time.Millisecond)
	}
	// Total now: 2 + 5 = 7 files. With maxGenerations = 2 (keep 3), 4 should be deleted.
	if err := store.RotateBackups(backupDir, 2); err != nil {
		t.Fatalf("RotateBackups failed: %v", err)
	}
	listAfter, err := store.ListBackups(backupDir)
	if err != nil {
		t.Fatalf("ListBackups after rotation failed: %v", err)
	}
	if len(listAfter) != 3 {
		t.Errorf("expected 3 backups after rotation, got %d", len(listAfter))
	}

	// 7. Test DeleteBackup
	firstFile := listAfter[0].Name
	if err := store.DeleteBackup(backupDir, firstFile); err != nil {
		t.Fatalf("DeleteBackup failed: %v", err)
	}
	listAfterDel, _ := store.ListBackups(backupDir)
	if len(listAfterDel) != 2 {
		t.Errorf("expected 2 backups after delete, got %d", len(listAfterDel))
	}

	// 8. Test RestoreDB
	// Save a new backup to restore from later
	restoreSourceFile := filepath.Join(backupDir, "twsnmpneo.db.restore_target")
	if _, err := store.BackupDB(ctx, restoreSourceFile, false); err != nil {
		t.Fatalf("BackupDB for restore target failed: %v", err)
	}

	// Add node2 before restore
	_ = store.SaveNode(ctx, &datastore.NodeEnt{ID: "node2", Name: "Node2"})
	n2, _ := store.GetNode(ctx, "node2")
	if n2 == nil {
		t.Fatal("expected node2 to exist")
	}

	// Restore from restoreSourceFile (which did NOT have node2)
	if err := store.RestoreDB(restoreSourceFile); err != nil {
		t.Fatalf("RestoreDB failed: %v", err)
	}
	n1After, err := store.GetNode(ctx, "node1")
	if err != nil || n1After == nil {
		t.Fatalf("expected node1 to exist after restore: %v", err)
	}
	n2After, _ := store.GetNode(ctx, "node2")
	if n2After != nil {
		t.Errorf("expected node2 to NOT exist after restoring from older backup")
	}
}
