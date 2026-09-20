package parquet_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
)

func setupParquetTest(t *testing.T) (*parquet.Store, string, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "twsnmpneo-parquet-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}

	store, err := parquet.New(parquet.Config{
		Dir:            dir,
		BufferSize:     5,
		BufferInterval: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("create parquet store: %v", err)
	}

	cleanup := func() {
		_ = store.Close()
		_ = os.RemoveAll(dir)
	}
	return store, dir, cleanup
}

func TestStore_WriteAndQuery(t *testing.T) {
	store, _, cleanup := setupParquetTest(t)
	defer cleanup()
	ctx := context.Background()

	now := time.Now().UnixNano()

	// Write 10 logs across two types
	for i := 0; i < 6; i++ {
		err := store.WriteLog(&parquet.ParquetLogRecord{
			Time: now + int64(i*1000),
			Type: "syslog",
			Src:  "192.168.1.1",
			Log:  fmt.Sprintf(`{"msg":"syslog message %d","src":"192.168.1.1"}`, i),
		})
		if err != nil {
			t.Fatalf("write syslog failed: %v", err)
		}
	}
	for i := 0; i < 4; i++ {
		err := store.WriteLog(&parquet.ParquetLogRecord{
			Time: now + int64(10000+i*1000),
			Type: "trap",
			Src:  "192.168.1.2",
			Log:  fmt.Sprintf(`{"msg":"trap event %d","src":"192.168.1.2"}`, i),
		})
		if err != nil {
			t.Fatalf("write trap failed: %v", err)
		}
	}

	// 1. Query all
	records, err := store.Query(ctx, parquet.LogFilter{
		Limit: 20,
	})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(records) != 10 {
		t.Fatalf("expected 10 records, got %d", len(records))
	}

	// 2. Query filtered by type
	syslogs, err := store.Query(ctx, parquet.LogFilter{
		Type:  "syslog",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("query syslog failed: %v", err)
	}
	if len(syslogs) != 6 {
		t.Fatalf("expected 6 syslogs, got %d", len(syslogs))
	}

	// 3. Query filtered by keyword
	filtered, err := store.Query(ctx, parquet.LogFilter{
		Filter: "trap event 2",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("query keyword failed: %v", err)
	}
	if len(filtered) != 1 {
		t.Fatalf("expected 1 match, got %d", len(filtered))
	}

	// 4. Query filtered by source
	srcFiltered, err := store.Query(ctx, parquet.LogFilter{
		Src:   "192.168.1.2",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("query src failed: %v", err)
	}
	if len(srcFiltered) != 4 {
		t.Fatalf("expected 4 trap matches, got %d", len(srcFiltered))
	}
}

func TestStore_Rotate(t *testing.T) {
	store, dir, cleanup := setupParquetTest(t)
	defer cleanup()

	// Write a record
	_ = store.WriteLog(&parquet.ParquetLogRecord{
		Type: "syslog",
		Log:  "rotation test log",
	})
	_ = store.Flush()

	// Age the file manually to test rotation
	typeDir := filepath.Join(dir, "syslog")
	entries, _ := os.ReadDir(typeDir)
	if len(entries) == 0 {
		t.Fatal("expected parquet file to exist")
	}
	filePath := filepath.Join(typeDir, entries[0].Name())
	oldTime := time.Now().AddDate(0, 0, -30)
	_ = os.Chtimes(filePath, oldTime, oldTime)

	// Rotate older than 14 days
	deleted, err := store.Rotate(14)
	if err != nil {
		t.Fatalf("rotate failed: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 file deleted, got %d", deleted)
	}
}

func TestStore_ErrorCases(t *testing.T) {
	// Empty dir
	_, err := parquet.New(parquet.Config{Dir: ""})
	if err == nil {
		t.Fatal("expected error on empty dir")
	}

	store, _, cleanup := setupParquetTest(t)
	defer cleanup()

	// Nil record
	if err := store.WriteLog(nil); err == nil {
		t.Fatal("expected error on nil record")
	}

	// Context cancel
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = store.Query(ctx, parquet.LogFilter{})
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
}
