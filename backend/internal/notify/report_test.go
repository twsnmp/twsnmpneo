package notify

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
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

func TestManager_GetLLMSummary(t *testing.T) {
	store, cleanup, err := setupTestDB(t)
	if err != nil {
		t.Fatalf("setupTestDB err=%v", err)
	}
	defer cleanup()

	mgr := &Manager{store: store}
	info := []reportInfoEnt{
		{Name: "Test", Value: "1"},
	}

	// 1. LLMSummary disabled -> empty string
	summary := mgr.getLLMSummary(context.Background(), &info)
	if summary != "" {
		t.Fatalf("expected empty summary when LLMSummary disabled, got %q", summary)
	}

	// 2. Cancelled context -> empty string
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	summary = mgr.getLLMSummary(cancelledCtx, &info)
	if summary != "" {
		t.Fatalf("expected empty summary with cancelled context, got %q", summary)
	}
}

func TestBuildLLMReportPrompts_AggregationAndLimit(t *testing.T) {
	store, cleanup, err := setupTestDB(t)
	if err != nil {
		t.Fatalf("setupTestDB err=%v", err)
	}
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UnixNano()

	// 1. Add identical events multiple times to test grouping
	for i := 0; i < 5; i++ {
		_ = store.AddEventLog(ctx, &datastore.EventLogEnt{
			Time:     now - int64(i)*int64(time.Minute),
			Level:    "warn",
			Type:     "polling",
			NodeName: "Node-1",
			Event:    "Ping timeout",
		})
	}

	// 2. Add distinct events exceeding 30 to test limit and omission message
	for i := 2; i <= 35; i++ {
		_ = store.AddEventLog(ctx, &datastore.EventLogEnt{
			Time:     now - int64(i)*int64(time.Minute),
			Level:    "high",
			Type:     "snmptrap",
			NodeName: fmt.Sprintf("Node-%d", i),
			Event:    fmt.Sprintf("Link down on port %d", i),
		})
	}

	info := []reportInfoEnt{
		{Name: "DB Size", Value: "10 MB"},
	}

	prompts := buildLLMReportPrompts(store, &info)

	// Check grouping: Ping timeout should have count=5
	foundGrouped := false
	for _, p := range prompts {
		if strings.Contains(p, "Ping timeout") && strings.Contains(p, "(count=5)") {
			foundGrouped = true
			break
		}
	}
	if !foundGrouped {
		t.Fatalf("expected grouped Ping timeout with count=5 in prompts, got: %v", prompts)
	}

	// Check omission: 1 group (5 logs) + 34 distinct logs = 39 logs, 35 unique events.
	// Max unique is 30, so 5 unique events (5 logs) should be omitted.
	foundOmitted := false
	for _, p := range prompts {
		if strings.Contains(p, "omitted") || strings.Contains(p, "省略") {
			foundOmitted = true
			break
		}
	}
	if !foundOmitted {
		t.Fatalf("expected omitted notice in prompts, got: %v", prompts)
	}
}

func TestGenerateSummaryWithRetry_TemporaryErrorThenSuccess(t *testing.T) {
	attempts := 0
	var slept []time.Duration
	resp, err := generateSummaryWithRetry(
		context.Background(),
		func(ctx context.Context) (string, error) {
			attempts++
			if attempts < 3 {
				return "", errors.New("gemini API error 503 (UNAVAILABLE): model overloaded")
			}
			return "Summary OK", nil
		},
		[]time.Duration{time.Second, 2 * time.Second, 3 * time.Second},
		func(ctx context.Context, d time.Duration) error {
			slept = append(slept, d)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "Summary OK" {
		t.Fatalf("unexpected response: %q", resp)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
	if len(slept) != 2 || slept[0] != time.Second || slept[1] != 2*time.Second {
		t.Fatalf("unexpected sleep durations: %v", slept)
	}
}

func TestGenerateSummaryWithRetry_NonTemporaryErrorNoRetry(t *testing.T) {
	attempts := 0
	_, err := generateSummaryWithRetry(
		context.Background(),
		func(ctx context.Context) (string, error) {
			attempts++
			return "", errors.New("gemini API error 400 (INVALID_ARGUMENT): invalid key")
		},
		[]time.Duration{time.Second, 2 * time.Second},
		func(ctx context.Context, d time.Duration) error {
			t.Fatalf("sleep should not be called on non-temporary error")
			return nil
		},
	)
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Fatalf("expected 400 error, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

func TestGenerateSummaryWithRetry_ExhaustedRetries(t *testing.T) {
	attempts := 0
	_, err := generateSummaryWithRetry(
		context.Background(),
		func(ctx context.Context) (string, error) {
			attempts++
			return "", errors.New("429 too many requests")
		},
		[]time.Duration{time.Millisecond, 2 * time.Millisecond},
		func(ctx context.Context, d time.Duration) error {
			return nil
		},
	)
	if err == nil || !strings.Contains(err.Error(), "429") {
		t.Fatalf("expected 429 error, got %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

