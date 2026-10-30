package notify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	"github.com/twsnmp/twsnmpneo/backend/internal/i18n"
)

func TestFormatNodeCountByStateIncludesEveryState(t *testing.T) {
	got := formatNodeCountByState(1, 2, 3, 4, 5, 6)
	want := fmt.Sprintf(i18n.Trans("High=%d,Low=%d,Warn=%d,Normal=%d,Repair=%d,Other=%d"), 1, 2, 3, 4, 5, 6)
	if got != want || strings.Contains(got, "%!") {
		t.Fatalf("formatNodeCountByState() = %q, want %q", got, want)
	}
}

func TestReportInfoIncludesDBSizeAndLogSize(t *testing.T) {
	store, cleanup, err := setupTestDB(t)
	if err != nil {
		t.Fatalf("setupTestDB err=%v", err)
	}
	defer cleanup()

	mgr := &Manager{store: store}

	// 1. Japanese
	i18n.SetLang("ja")
	infoJa := mgr.getReportInfo()
	foundDBSizeJa := false
	foundLogSizeJa := false
	for _, item := range infoJa {
		if item.Name == "DBサイズ" {
			foundDBSizeJa = true
		}
		if item.Name == "ログ保存サイズ" {
			foundLogSizeJa = true
		}
	}
	if !foundDBSizeJa {
		t.Errorf("expected DBサイズ in Japanese report info, got: %+v", infoJa)
	}
	if !foundLogSizeJa {
		t.Errorf("expected ログ保存サイズ in Japanese report info, got: %+v", infoJa)
	}

	// 2. English
	i18n.SetLang("en")
	infoEn := mgr.getReportInfo()
	foundDBSizeEn := false
	foundLogSizeEn := false
	for _, item := range infoEn {
		if item.Name == "DB Size" {
			foundDBSizeEn = true
		}
		if item.Name == "Log Size" {
			foundLogSizeEn = true
		}
	}
	if !foundDBSizeEn {
		t.Errorf("expected DB Size in English report info, got: %+v", infoEn)
	}
	if !foundLogSizeEn {
		t.Errorf("expected Log Size in English report info, got: %+v", infoEn)
	}
}

func TestManager_GetLogSizeWithParquetStore(t *testing.T) {
	store, cleanup, err := setupTestDB(t)
	if err != nil {
		t.Fatalf("setupTestDB err=%v", err)
	}
	defer cleanup()

	dir, err := os.MkdirTemp("", "twsnmpneo-notify-log-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	pqStore, err := parquet.New(parquet.Config{
		Dir:            dir,
		BufferSize:     2,
		BufferInterval: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("create parquet store: %v", err)
	}
	defer pqStore.Close()

	mgr := &Manager{store: store, logStore: pqStore}
	if size := mgr.getLogSize(); size != 0 {
		t.Fatalf("expected initial log size 0, got %d", size)
	}

	_ = pqStore.WriteLog(&parquet.ParquetLogRecord{
		Time: time.Now().UnixNano(),
		Type: "syslog",
		Src:  "192.168.1.1",
		Log:  "test log entry",
	})
	if err := pqStore.Flush(); err != nil {
		t.Fatalf("flush failed: %v", err)
	}

	size := mgr.getLogSize()
	if size <= 0 {
		t.Fatalf("expected positive log size after write, got %d", size)
	}
}

func TestManager_GetLogSizeFallbackToDataDir(t *testing.T) {
	store, cleanup, err := setupTestDB(t)
	if err != nil {
		t.Fatalf("setupTestDB err=%v", err)
	}
	defer cleanup()

	mgr := &Manager{store: store}
	dataDir := store.GetDataDir()
	logDir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("mkdir logs err=%v", err)
	}
	dummyFile := filepath.Join(logDir, "test.parquet")
	dummyData := []byte("fake parquet payload for test")
	if err := os.WriteFile(dummyFile, dummyData, 0644); err != nil {
		t.Fatalf("write dummy file err=%v", err)
	}

	size := mgr.getLogSize()
	if size != int64(len(dummyData)) {
		t.Fatalf("expected log size %d, got %d", len(dummyData), size)
	}
}
