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

		// 7. eventlog
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
}
