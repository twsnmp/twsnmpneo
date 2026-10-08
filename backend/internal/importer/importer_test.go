package importer_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/bbolt"
	"github.com/twsnmp/twsnmpneo/backend/internal/importer"
	bolt "go.etcd.io/bbolt"
)

func createMockFCDB(t *testing.T, dir string) string {
	t.Helper()
	dbPath := filepath.Join(dir, "twsnmpfc.db")
	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		t.Fatalf("create mock fc db: %v", err)
	}
	defer db.Close()

	err = db.Update(func(tx *bolt.Tx) error {
		// 1. config
		cb, _ := tx.CreateBucket([]byte("config"))
		mapConf := datastore.MapConfEnt{
			MapName: "Imported FC Map",
			PollInt: 30,
		}
		data, _ := json.Marshal(mapConf)
		_ = cb.Put([]byte("mapConf"), data)

		notifyConf := datastore.NotifyConfEnt{
			MailServer: "smtp.example.com",
			MailTo:     "admin@example.com",
		}
		data, _ = json.Marshal(notifyConf)
		_ = cb.Put([]byte("notifyConf"), data)

		locConf := datastore.LocConfEnt{
			Center:   "35.6895,139.6917",
			Zoom:     14,
			Style:    "osm",
			IconSize: 32,
		}
		data, _ = json.Marshal(locConf)
		_ = cb.Put([]byte("locConf"), data)

		customIcons := []*datastore.IconEnt{
			{ID: "icon1", Name: "server-custom.png"},
		}
		data, _ = json.Marshal(customIcons)
		_ = cb.Put([]byte("customIcons"), data)

		// 2. nodes
		nb, _ := tx.CreateBucket([]byte("nodes"))
		node1 := datastore.NodeEnt{
			ID:   "fc-node-1",
			Name: "FC Core Router",
			IP:   "10.0.0.1",
		}
		data, _ = json.Marshal(node1)
		_ = nb.Put([]byte("fc-node-1"), data)
		// Corrupted node entry to test warnings
		_ = nb.Put([]byte("corrupt-node"), []byte("{bad-json}"))

		// 3. lines
		lb, _ := tx.CreateBucket([]byte("lines"))
		line1 := datastore.LineEnt{
			ID:      "fc-line-1",
			NodeID1: "fc-node-1",
			NodeID2: "fc-node-2",
		}
		data, _ = json.Marshal(line1)
		_ = lb.Put([]byte("fc-line-1"), data)
		_ = lb.Put([]byte("corrupt-line"), []byte("{bad-json}"))

		// 4. networks
		nwb, _ := tx.CreateBucket([]byte("networks"))
		nw1 := datastore.NetworkEnt{
			ID:   "fc-nw-1",
			Name: "DMZ Network",
		}
		data, _ = json.Marshal(nw1)
		_ = nwb.Put([]byte("fc-nw-1"), data)
		_ = nwb.Put([]byte("corrupt-nw"), []byte("{bad-json}"))

		// 5. items
		ib, _ := tx.CreateBucket([]byte("items"))
		item1 := datastore.DrawItemEnt{
			ID:   "fc-item-1",
			Text: "Main DC",
		}
		data, _ = json.Marshal(item1)
		_ = ib.Put([]byte("fc-item-1"), data)
		_ = ib.Put([]byte("corrupt-item"), []byte("{bad-json}"))

		// 6. pollings
		pb, _ := tx.CreateBucket([]byte("pollings"))
		poll1 := datastore.PollingEnt{
			ID:     "fc-poll-1",
			Name:   "Ping Core",
			NodeID: "fc-node-1",
			Type:   "ping",
		}
		data, _ = json.Marshal(poll1)
		_ = pb.Put([]byte("fc-poll-1"), data)
		_ = pb.Put([]byte("corrupt-poll"), []byte("{bad-json}"))

		// 7. users
		ub, _ := tx.CreateBucket([]byte("users"))
		user1 := datastore.UserEnt{
			User: "fcadmin",
			Name: "FC Admin",
			Role: "admin",
		}
		data, _ = json.Marshal(user1)
		_ = ub.Put([]byte("fcadmin"), data)

		// 8. eventlog
		eb, _ := tx.CreateBucket([]byte("eventlog"))
		ev1 := datastore.EventLogEnt{
			Time:  time.Now().UnixNano(),
			Level: "warn",
			Event: "Link Down",
		}
		data, _ = json.Marshal(ev1)
		_ = eb.Put([]byte("ev1"), data)

		return nil
	})
	if err != nil {
		t.Fatalf("populate mock fc db: %v", err)
	}

	return dbPath
}

func createMockFCDataDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "twsnmpneo-mock-fc-dir-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}

	createMockFCDB(t, dir)

	// Mock icons/
	iconsDir := filepath.Join(dir, "icons")
	_ = os.MkdirAll(iconsDir, 0755)
	_ = os.WriteFile(filepath.Join(iconsDir, "custom.png"), []byte("mock-png"), 0644)
	_ = os.WriteFile(filepath.Join(iconsDir, "switch.svg"), []byte("<svg></svg>"), 0644)

	// Mock geoip.mmdb
	_ = os.WriteFile(filepath.Join(dir, "geoip.mmdb"), []byte("mock-geoip-data"), 0644)

	// Mock extmibs/
	extMibsDir := filepath.Join(dir, "extmibs")
	_ = os.MkdirAll(extMibsDir, 0755)
	_ = os.WriteFile(filepath.Join(extMibsDir, "VENDOR-MIB.txt"), []byte("VENDOR-MIB DEFINITIONS"), 0644)

	// Mock mib.txt
	_ = os.WriteFile(filepath.Join(dir, "mib.txt"), []byte("MY-MIB DEFINITIONS"), 0644)

	// Mock cmd/
	cmdDir := filepath.Join(dir, "cmd")
	_ = os.MkdirAll(cmdDir, 0755)
	_ = os.WriteFile(filepath.Join(cmdDir, "check_status.sh"), []byte("#!/bin/sh\nexit 0\n"), 0755)

	return dir
}

func TestImportFCDatabase(t *testing.T) {
	dir, err := os.MkdirTemp("", "twsnmpneo-importer-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	fcDB := createMockFCDB(t, dir)
	destDBPath := filepath.Join(dir, "neo.db")
	destStore, err := bbolt.New(destDBPath)
	if err != nil {
		t.Fatalf("create neo store: %v", err)
	}
	defer destStore.Close()

	ctx := context.Background()
	report, err := importer.ImportFCDatabase(ctx, fcDB, destStore, importer.Options{
		ImportEventLogs: true,
	})
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}

	// Verify imported counts
	if report.NodesImported != 1 {
		t.Errorf("expected 1 node imported, got %d", report.NodesImported)
	}
	if report.LinesImported != 1 {
		t.Errorf("expected 1 line imported, got %d", report.LinesImported)
	}
	if report.NwsImported != 1 {
		t.Errorf("expected 1 network imported, got %d", report.NwsImported)
	}
	if report.ItemsImported != 1 {
		t.Errorf("expected 1 item imported, got %d", report.ItemsImported)
	}
	if report.PollsImported != 1 {
		t.Errorf("expected 1 polling imported, got %d", report.PollsImported)
	}
	if report.UsersImported != 1 {
		t.Errorf("expected 1 user imported, got %d", report.UsersImported)
	}
	if report.EventsImported != 1 {
		t.Errorf("expected 1 event imported, got %d", report.EventsImported)
	}
	if report.SkippedEntries != 5 {
		t.Errorf("expected 5 skipped corrupted entries, got %d", report.SkippedEntries)
	}
	if len(report.Warnings) != 5 {
		t.Errorf("expected 5 warnings, got %d", len(report.Warnings))
	}

	// Verify data in destination store
	node, err := destStore.GetNode(ctx, "fc-node-1")
	if err != nil || node.Name != "FC Core Router" {
		t.Errorf("node not correctly imported: %+v, err: %v", node, err)
	}

	mapConf, err := destStore.GetMapConf(ctx)
	if err != nil || mapConf.MapName != "Imported FC Map" {
		t.Errorf("map config not correctly imported: %+v, err: %v", mapConf, err)
	}

	notifyConf, err := destStore.GetNotifyConf(ctx)
	if err != nil || notifyConf.MailServer != "smtp.example.com" {
		t.Errorf("notify config not correctly imported: %+v, err: %v", notifyConf, err)
	}

	user, err := destStore.GetUser(ctx, "fcadmin")
	if err != nil || user.Name != "FC Admin" {
		t.Errorf("user not correctly imported: %+v, err: %v", user, err)
	}
}

func TestRunFCMigration(t *testing.T) {
	srcDir := createMockFCDataDir(t)
	defer os.RemoveAll(srcDir)

	destDir, err := os.MkdirTemp("", "twsnmpneo-migration-dest-*")
	if err != nil {
		t.Fatalf("create temp dest dir: %v", err)
	}
	defer os.RemoveAll(destDir)

	ctx := context.Background()
	report, err := importer.RunFCMigration(ctx, srcDir, destDir, importer.Options{
		ImportEventLogs: true,
	})
	if err != nil {
		t.Fatalf("RunFCMigration failed: %v", err)
	}

	if report.NodesImported != 1 {
		t.Errorf("expected 1 node imported, got %d", report.NodesImported)
	}
	if report.UsersImported != 1 {
		t.Errorf("expected 1 user imported, got %d", report.UsersImported)
	}
	if len(report.FilesCopied) == 0 {
		t.Errorf("expected copied files in report, got none")
	}

	// Verify asset files were copied
	if _, err := os.Stat(filepath.Join(destDir, "images", "custom.png")); err != nil {
		t.Errorf("images/custom.png not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "images", "switch.svg")); err != nil {
		t.Errorf("images/switch.svg not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "geoip.mmdb")); err != nil {
		t.Errorf("geoip.mmdb not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "extmibs", "VENDOR-MIB.txt")); err != nil {
		t.Errorf("extmibs/VENDOR-MIB.txt not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "mib.txt")); err != nil {
		t.Errorf("mib.txt not copied: %v", err)
	}
	if info, err := os.Stat(filepath.Join(destDir, "cmd", "check_status.sh")); err != nil {
		t.Errorf("cmd/check_status.sh not copied: %v", err)
	} else if info.Mode()&0111 == 0 {
		t.Errorf("cmd/check_status.sh did not preserve execute permission: %v", info.Mode())
	}

	// Test overwriting existing destination with and without Force
	_, err = importer.RunFCMigration(ctx, srcDir, destDir, importer.Options{})
	if err == nil {
		t.Error("expected error when target DB already exists and Force is false")
	}

	_, err = importer.RunFCMigration(ctx, srcDir, destDir, importer.Options{Force: true})
	if err != nil {
		t.Errorf("expected success when Force is true, got: %v", err)
	}
}

func TestImportFCDatabase_ErrorCases(t *testing.T) {
	ctx := context.Background()

	// 1. nil destination
	_, err := importer.ImportFCDatabase(ctx, "any.db", nil, importer.Options{})
	if err != datastore.ErrInvalidParams {
		t.Errorf("expected ErrInvalidParams on nil dest, got %v", err)
	}

	// 2. Non-existent file
	dir, err := os.MkdirTemp("", "twsnmpneo-importer-err-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	destStore, _ := bbolt.New(filepath.Join(dir, "neo.db"))
	defer destStore.Close()

	_, err = importer.ImportFCDatabase(ctx, filepath.Join(dir, "nonexistent.db"), destStore, importer.Options{})
	if err == nil {
		t.Error("expected error on nonexistent source db")
	}

	// 3. RunFCMigration with empty paths
	_, err = importer.RunFCMigration(ctx, "", "", importer.Options{})
	if err == nil {
		t.Error("expected error on empty paths")
	}
}
