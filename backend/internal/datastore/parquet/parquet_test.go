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

	// 5. DeleteLogs by type
	if err := store.DeleteLogs(ctx, "trap"); err != nil {
		t.Fatalf("delete trap logs failed: %v", err)
	}
	afterDel, err := store.Query(ctx, parquet.LogFilter{Type: "trap"})
	if err != nil || len(afterDel) != 0 {
		t.Fatalf("expected 0 trap records after delete, got %d (err=%v)", len(afterDel), err)
	}
	// syslog should still remain
	sysRemaining, err := store.Query(ctx, parquet.LogFilter{Type: "syslog"})
	if err != nil || len(sysRemaining) != 6 {
		t.Fatalf("expected 6 syslog records remaining, got %d (err=%v)", len(sysRemaining), err)
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

func TestStore_QuerySkipOldFilesByStartTime(t *testing.T) {
	store, dir, cleanup := setupParquetTest(t)
	defer cleanup()
	ctx := context.Background()

	now := time.Now()
	oldTime := now.Add(-48 * time.Hour)

	// 1. Write an old log and flush to its own file
	if err := store.WriteLog(&parquet.ParquetLogRecord{
		Time: oldTime.UnixNano(),
		Type: "syslog",
		Src:  "192.168.1.10",
		Log:  "old syslog message",
	}); err != nil {
		t.Fatalf("write old log failed: %v", err)
	}
	if err := store.Flush(); err != nil {
		t.Fatalf("flush old log failed: %v", err)
	}

	// Age the flushed file's ModTime to 48 hours ago
	typeDir := filepath.Join(dir, "syslog")
	entries, err := os.ReadDir(typeDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected 1 parquet file, got %d (err=%v)", len(entries), err)
	}
	oldFilePath := filepath.Join(typeDir, entries[0].Name())
	if err := os.Chtimes(oldFilePath, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes failed: %v", err)
	}

	// Sleep briefly so the next flush gets a newer filename
	time.Sleep(2 * time.Millisecond)

	// 2. Write a recent log and flush
	if err := store.WriteLog(&parquet.ParquetLogRecord{
		Time: now.UnixNano(),
		Type: "syslog",
		Src:  "192.168.1.10",
		Log:  "recent syslog message",
	}); err != nil {
		t.Fatalf("write recent log failed: %v", err)
	}
	if err := store.Flush(); err != nil {
		t.Fatalf("flush recent log failed: %v", err)
	}

	// Query with StartTime = 24 hours ago (should only return the recent record and skip the old file)
	start24h := now.Add(-24 * time.Hour).UnixNano()
	records, err := store.Query(ctx, parquet.LogFilter{
		Type:      "syslog",
		StartTime: start24h,
		Limit:     100,
	})
	if err != nil {
		t.Fatalf("query with StartTime failed: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 recent record, got %d", len(records))
	}
	if records[0].Log != "recent syslog message" {
		t.Fatalf("unexpected log content: %s", records[0].Log)
	}
}

func TestStore_GetTotalSize(t *testing.T) {
	store, _, cleanup := setupParquetTest(t)
	defer cleanup()

	// Initial size before flush should be 0
	if size := store.GetTotalSize(); size != 0 {
		t.Fatalf("expected initial size 0, got %d", size)
	}

	// Nil store check
	var nilStore *parquet.Store
	if size := nilStore.GetTotalSize(); size != 0 {
		t.Fatalf("expected nil store size 0, got %d", size)
	}

	// Write and flush logs
	for i := 0; i < 5; i++ {
		_ = store.WriteLog(&parquet.ParquetLogRecord{
			Time: time.Now().UnixNano(),
			Type: "syslog",
			Src:  "192.168.1.1",
			Log:  fmt.Sprintf(`{"msg":"test syslog %d"}`, i),
		})
	}
	if err := store.Flush(); err != nil {
		t.Fatalf("flush failed: %v", err)
	}

	// Size should now be greater than 0
	totalSize := store.GetTotalSize()
	if totalSize <= 0 {
		t.Fatalf("expected positive total size after flush, got %d", totalSize)
	}
}

func TestStore_GetRecentPollingLog(t *testing.T) {
	store, _, cleanup := setupParquetTest(t)
	defer cleanup()
	ctx := context.Background()

	// 1. Check non-existent polling ID -> returns empty, no error, fast
	logs, err := store.GetRecentPollingLog(ctx, "non-existent-poll", 50)
	if err != nil {
		t.Fatalf("unexpected error for non-existent poll: %v", err)
	}
	if len(logs) != 0 {
		t.Fatalf("expected 0 logs, got %d", len(logs))
	}

	// 2. Write 3 polling logs to disk (flushed)
	baseTime := time.Now().Add(-10 * time.Minute).UnixNano()
	for i := 0; i < 3; i++ {
		_ = store.WriteLog(&parquet.ParquetLogRecord{
			Time: baseTime + int64(i*1000000000),
			Type: "polling",
			Src:  "poll-1",
			Log:  fmt.Sprintf(`{"State":"normal","Result":{"rtt":%d.5}}`, i+1),
		})
	}
	if err := store.Flush(); err != nil {
		t.Fatalf("flush failed: %v", err)
	}

	// 3. Write 2 more polling logs into in-memory buffer (un-flushed!)
	for i := 3; i < 5; i++ {
		_ = store.WriteLog(&parquet.ParquetLogRecord{
			Time: baseTime + int64(i*1000000000),
			Type: "polling",
			Src:  "poll-1",
			Log:  fmt.Sprintf(`{"State":"normal","Result":{"rtt":%d.5}}`, i+1),
		})
	}

	// 4. GetRecentPollingLog should fetch all 5 logs in chronological order!
	logs, err = store.GetRecentPollingLog(ctx, "poll-1", 10)
	if err != nil {
		t.Fatalf("failed to get recent polling log: %v", err)
	}
	if len(logs) != 5 {
		t.Fatalf("expected 5 logs, got %d", len(logs))
	}

	// Verify chronological order (oldest to newest)
	for i := 0; i < len(logs); i++ {
		expectedRTT := float64(i+1) + 0.5
		rttVal, ok := logs[i].Result["rtt"].(float64)
		if !ok || rttVal != expectedRTT {
			t.Errorf("log[%d] rtt mismatch: got %v, expected %f", i, logs[i].Result["rtt"], expectedRTT)
		}
	}
}

func TestStore_CountRecentLogs(t *testing.T) {
	store, _, cleanup := setupParquetTest(t)
	defer cleanup()
	ctx := context.Background()

	// Write 5 syslog and 3 trap records
	now := time.Now().UnixNano()
	for i := 0; i < 5; i++ {
		_ = store.WriteLog(&parquet.ParquetLogRecord{
			Time: now - int64(i*1000000000),
			Type: "syslog",
			Src:  "192.168.1.1",
			Log:  `{"message":"test syslog"}`,
		})
	}
	for i := 0; i < 3; i++ {
		_ = store.WriteLog(&parquet.ParquetLogRecord{
			Time: now - int64(i*1000000000),
			Type: "trap",
			Src:  "192.168.1.1",
			Log:  `{"message":"test trap"}`,
		})
	}

	// 1. In-memory count test
	counts := store.CountRecentLogs(ctx, 24*time.Hour, "syslog", "trap")
	if counts["syslog"] != 5 {
		t.Errorf("expected 5 in-memory syslog, got %d", counts["syslog"])
	}
	if counts["trap"] != 3 {
		t.Errorf("expected 3 in-memory trap, got %d", counts["trap"])
	}

	// 2. Flush to disk and test disk counting
	if err := store.Flush(); err != nil {
		t.Fatalf("flush failed: %v", err)
	}
	counts = store.CountRecentLogs(ctx, 24*time.Hour, "syslog", "trap")
	if counts["syslog"] != 5 {
		t.Errorf("expected 5 disk syslog, got %d", counts["syslog"])
	}
	if counts["trap"] != 3 {
		t.Errorf("expected 3 disk trap, got %d", counts["trap"])
	}
}

func TestStore_Compact(t *testing.T) {
	store, dir, cleanup := setupParquetTest(t)
	defer cleanup()
	ctx := context.Background()

	// Create 3 separate small parquet files and rename them to yesterday's timestamp
	yesterday := time.Now().Add(-25 * time.Hour)
	for i := 0; i < 3; i++ {
		recTime := yesterday.Add(time.Duration(i) * time.Minute)
		if err := store.WriteLog(&parquet.ParquetLogRecord{
			Time: recTime.UnixNano(),
			Type: "syslog",
			Src:  "192.168.1.50",
			Log:  fmt.Sprintf(`{"msg":"compact test %d"}`, i),
		}); err != nil {
			t.Fatalf("write failed: %v", err)
		}
		if err := store.Flush(); err != nil {
			t.Fatalf("flush failed: %v", err)
		}
	}

	typeDir := filepath.Join(dir, "syslog")
	entries, err := os.ReadDir(typeDir)
	if err != nil || len(entries) != 3 {
		t.Fatalf("expected 3 parquet files before compact, got %d (err=%v)", len(entries), err)
	}

	// Rename files to yesterday's timestamp so Compact() treats them as a completed past day bucket
	for i, e := range entries {
		oldPath := filepath.Join(typeDir, e.Name())
		ts := yesterday.Add(time.Duration(i) * time.Minute).Format("20060102150405")
		newPath := filepath.Join(typeDir, fmt.Sprintf("syslog_%s_000%d.parquet", ts, i))
		if err := os.Rename(oldPath, newPath); err != nil {
			t.Fatalf("rename failed: %v", err)
		}
		_ = os.Chtimes(newPath, yesterday, yesterday)
	}

	compacted, err := store.Compact()
	if err != nil {
		t.Fatalf("Compact failed: %v", err)
	}
	if compacted != 3 {
		t.Fatalf("expected 3 files compacted, got %d", compacted)
	}

	afterEntries, err := os.ReadDir(typeDir)
	if err != nil || len(afterEntries) != 1 {
		t.Fatalf("expected 1 compacted parquet file, got %d (err=%v)", len(afterEntries), err)
	}

	// Verify Query still returns all 3 records in descending chronological order
	records, err := store.Query(ctx, parquet.LogFilter{
		Type:  "syslog",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("Query after compact failed: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 records after compact, got %d", len(records))
	}
	if records[0].Time < records[1].Time || records[1].Time < records[2].Time {
		t.Errorf("expected descending time order after compact: %d, %d, %d", records[0].Time, records[1].Time, records[2].Time)
	}
}

func TestStore_FlushSpeed(t *testing.T) {
	dir, err := os.MkdirTemp("", "twsnmpneo-parquet-speed-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	store, err := parquet.New(parquet.Config{
		Dir:            dir,
		BufferSize:     10000,
		BufferInterval: 1 * time.Hour,
	})
	if err != nil {
		t.Fatalf("create parquet store: %v", err)
	}
	defer store.Close()

	now := time.Now().UnixNano()
	for i := 0; i < 1000; i++ {
		_ = store.WriteLog(&parquet.ParquetLogRecord{
			Time: now + int64(i),
			Type: "syslog",
			Src:  "192.168.1.1",
			Log:  fmt.Sprintf(`{"msg":"syslog message %d with extra details and some length to simulate real payload"}`, i),
		})
	}
	for i := 0; i < 800; i++ {
		_ = store.WriteLog(&parquet.ParquetLogRecord{
			Time: now + int64(i),
			Type: "netflow",
			Src:  "192.168.1.2",
			Log:  fmt.Sprintf(`{"msg":"netflow packet %d with flow headers and bytes counts"}`, i),
		})
	}
	for i := 0; i < 150; i++ {
		_ = store.WriteLog(&parquet.ParquetLogRecord{
			Time: now + int64(i),
			Type: "sflow",
			Src:  "192.168.1.3",
			Log:  fmt.Sprintf(`{"msg":"sflow counter sample %d"}`, i),
		})
	}

	st := time.Now()
	err = store.Flush()
	dur := time.Since(st)
	if err != nil {
		t.Fatalf("Flush error: %v", err)
	}
	t.Logf("Flush 1950 records in 1 shot took: %v", dur)
}

func TestStore_CountByType(t *testing.T) {
	dir, err := os.MkdirTemp("", "twsnmpneo-parquet-countbytype-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	store, err := parquet.New(parquet.Config{
		Dir:            dir,
		BufferSize:     100,
		BufferInterval: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("create parquet store: %v", err)
	}
	defer store.Close()

	now := time.Now().UnixNano()
	for i := 0; i < 50; i++ {
		_ = store.WriteLog(&parquet.ParquetLogRecord{
			Time: now + int64(i),
			Type: "sflow",
			Src:  "192.168.1.10",
			Log:  `{"sample":"flow"}`,
		})
	}
	for i := 0; i < 30; i++ {
		_ = store.WriteLog(&parquet.ParquetLogRecord{
			Time: now + int64(i),
			Type: "sflowCounter",
			Src:  "192.168.1.10",
			Log:  `{"sample":"counter"}`,
		})
	}
	for i := 0; i < 120; i++ {
		_ = store.WriteLog(&parquet.ParquetLogRecord{
			Time: now + int64(i),
			Type: "netflow",
			Src:  "192.168.1.20",
			Log:  `{"sample":"netflow"}`,
		})
	}
	for i := 0; i < 15; i++ {
		_ = store.WriteLog(&parquet.ParquetLogRecord{
			Time: now + int64(i),
			Type: "arplog",
			Src:  "192.168.1.30",
			Log:  `{"sample":"arp"}`,
		})
	}

	// 1. Flush to disk
	if err := store.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	// Add some in-memory buffered records
	_ = store.WriteLog(&parquet.ParquetLogRecord{
		Time: now + 1000,
		Type: "sflow",
		Src:  "192.168.1.10",
		Log:  `{"sample":"flow_mem"}`,
	})

	counts, err := store.CountByType(context.Background())
	if err != nil {
		t.Fatalf("CountByType: %v", err)
	}

	if counts["sflow"] != 51 {
		t.Errorf("expected sflow 51 (50 on disk + 1 in memory), got %d", counts["sflow"])
	}
	if counts["sflowCounter"] != 30 {
		t.Errorf("expected sflowCounter 30, got %d", counts["sflowCounter"])
	}
	if counts["netflow"] != 120 {
		t.Errorf("expected netflow 120, got %d", counts["netflow"])
	}
	if counts["arplog"] != 15 {
		t.Errorf("expected arplog 15, got %d", counts["arplog"])
	}

	// Close store1 before reopening
	_ = store.Close()

	// 2. Test reopening store (simulating cold boot)
	store2, err := parquet.New(parquet.Config{
		Dir:            dir,
		BufferSize:     100,
		BufferInterval: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer store2.Close()

	counts2, err := store2.CountByType(context.Background())
	if err != nil {
		t.Fatalf("CountByType on store2: %v", err)
	}
	// 50 was on disk, plus 1 that was flushed when store.Close() was called = 51
	if counts2["sflow"] != 51 {
		t.Errorf("expected sflow 51 from disk, got %d", counts2["sflow"])
	}
	if counts2["sflowCounter"] != 30 {
		t.Errorf("expected sflowCounter 30 from disk, got %d", counts2["sflowCounter"])
	}
	if counts2["netflow"] != 120 {
		t.Errorf("expected netflow 120 from disk, got %d", counts2["netflow"])
	}

	// 3. Test DeleteLogs
	if err := store2.DeleteLogs(context.Background(), "sflow"); err != nil {
		t.Fatalf("delete sflow: %v", err)
	}
	counts3, err := store2.CountByType(context.Background())
	if err != nil {
		t.Fatalf("CountByType after delete: %v", err)
	}
	if counts3["sflow"] != 0 {
		t.Errorf("expected sflow 0 after delete, got %d", counts3["sflow"])
	}
	if counts3["netflow"] != 120 {
		t.Errorf("expected netflow 120 retained, got %d", counts3["netflow"])
	}
}

func TestStore_QueryPollingLogs(t *testing.T) {
	store, _, cleanup := setupParquetTest(t)
	defer cleanup()

	baseTime := time.Now().Add(-2 * time.Hour).UnixNano()
	// Write 6 records for poll-1 (5+ flushes to disk, 1+ stays in buffer)
	for i := 0; i < 6; i++ {
		_ = store.WriteLog(&parquet.ParquetLogRecord{
			Time: baseTime + int64(i)*int64(10*time.Minute),
			Type: "polling",
			Src:  "poll-1",
			Log:  fmt.Sprintf(`{"State":"normal","Result":{"rtt":%d}}`, (i+1)*1000000),
		})
	}
	// Flush first batch, then write 2 more in memory buffer
	if err := store.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	for i := 6; i < 8; i++ {
		_ = store.WriteLog(&parquet.ParquetLogRecord{
			Time: baseTime + int64(i)*int64(10*time.Minute),
			Type: "polling",
			Src:  "poll-1",
			Log:  fmt.Sprintf(`{"State":"warn","Result":{"rtt":%d}}`, (i+1)*1000000),
		})
	}
	// Write logs for a different polling ID that must not leak into poll-1 results
	_ = store.WriteLog(&parquet.ParquetLogRecord{
		Time: baseTime + int64(3*time.Minute),
		Type: "polling",
		Src:  "poll-other",
		Log:  `{"State":"normal","Result":{"rtt":999000000}}`,
	})

	ctx := context.Background()

	// 1. All-time query combines disk + buffer in chronological order
	logs, truncated, err := store.QueryPollingLogs(ctx, "poll-1", 0, 0, 100)
	if err != nil {
		t.Fatalf("QueryPollingLogs all: %v", err)
	}
	if truncated {
		t.Errorf("expected truncated=false, got true")
	}
	if len(logs) != 8 {
		t.Fatalf("expected 8 logs for poll-1, got %d", len(logs))
	}
	for i := 1; i < len(logs); i++ {
		if logs[i].Time < logs[i-1].Time {
			t.Fatalf("logs not in chronological order at index %d", i)
		}
	}
	if logs[7].State != "warn" {
		t.Errorf("expected latest log state warn, got %s", logs[7].State)
	}

	// 2. Time range filter
	startNs := baseTime + int64(2*10*time.Minute)
	endNs := baseTime + int64(5*10*time.Minute)
	ranged, _, err := store.QueryPollingLogs(ctx, "poll-1", startNs, endNs, 100)
	if err != nil {
		t.Fatalf("QueryPollingLogs ranged: %v", err)
	}
	if len(ranged) != 4 {
		t.Fatalf("expected 4 ranged logs, got %d", len(ranged))
	}

	// 3. Limit + truncation keeps most recent entries in ascending order
	limited, trunc, err := store.QueryPollingLogs(ctx, "poll-1", 0, 0, 3)
	if err != nil {
		t.Fatalf("QueryPollingLogs limited: %v", err)
	}
	if !trunc {
		t.Errorf("expected truncated=true when limit=3")
	}
	if len(limited) != 3 {
		t.Fatalf("expected 3 logs, got %d", len(limited))
	}
	if limited[0].Time != baseTime+int64(5*10*time.Minute) {
		t.Errorf("expected oldest kept entry at index 5, got %v", limited[0].Time)
	}
}

func TestStore_DeletePollingLogs(t *testing.T) {
	store, _, cleanup := setupParquetTest(t)
	defer cleanup()

	now := time.Now().UnixNano()
	// Mixed batch flushed together in the same file
	for i := 0; i < 3; i++ {
		_ = store.WriteLog(&parquet.ParquetLogRecord{
			Time: now + int64(i)*1000,
			Type: "polling",
			Src:  "poll-del",
			Log:  `{"State":"normal","Result":{"rtt":1000000}}`,
		})
		_ = store.WriteLog(&parquet.ParquetLogRecord{
			Time: now + int64(i)*1000,
			Type: "polling",
			Src:  "poll-keep",
			Log:  `{"State":"normal","Result":{"rtt":2000000}}`,
		})
	}
	if err := store.Flush(); err != nil {
		t.Fatalf("flush mixed: %v", err)
	}

	// File containing ONLY poll-del
	for i := 0; i < 3; i++ {
		_ = store.WriteLog(&parquet.ParquetLogRecord{
			Time: now + int64(10+i)*1000,
			Type: "polling",
			Src:  "poll-del",
			Log:  `{"State":"high","Result":{"rtt":5000000}}`,
		})
	}
	if err := store.Flush(); err != nil {
		t.Fatalf("flush poll-del only: %v", err)
	}

	// Unflushed in-memory buffer for both pollings
	_ = store.WriteLog(&parquet.ParquetLogRecord{
		Time: now + 99999,
		Type: "polling",
		Src:  "poll-del",
		Log:  `{"State":"normal","Result":{"rtt":1500000}}`,
	})
	_ = store.WriteLog(&parquet.ParquetLogRecord{
		Time: now + 99999,
		Type: "polling",
		Src:  "poll-keep",
		Log:  `{"State":"normal","Result":{"rtt":2500000}}`,
	})

	ctx := context.Background()
	if err := store.DeletePollingLogs(ctx, "poll-del"); err != nil {
		t.Fatalf("DeletePollingLogs: %v", err)
	}

	delLogs, _, err := store.QueryPollingLogs(ctx, "poll-del", 0, 0, 100)
	if err != nil {
		t.Fatalf("query poll-del after delete: %v", err)
	}
	if len(delLogs) != 0 {
		t.Errorf("expected 0 logs remaining for poll-del, got %d", len(delLogs))
	}

	keepLogs, _, err := store.QueryPollingLogs(ctx, "poll-keep", 0, 0, 100)
	if err != nil {
		t.Fatalf("query poll-keep after delete: %v", err)
	}
	if len(keepLogs) != 4 {
		t.Errorf("expected 4 logs retained for poll-keep (3 disk + 1 buffer), got %d", len(keepLogs))
	}
}
