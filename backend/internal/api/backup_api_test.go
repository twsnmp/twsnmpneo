package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/api"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/bbolt"
	"github.com/twsnmp/twsnmpneo/backend/internal/monitor"
)

func TestBackupAPI(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "twsnmpneo.db")
	bDir := filepath.Join(dir, "backup")

	store, err := bbolt.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	mon := monitor.New(monitor.Config{
		DataDir:   dir,
		BackupDir: bDir,
		Store:     store,
		Interval:  1 * time.Minute,
	})

	server, err := api.NewServer(api.Config{
		Store:   store,
		Monitor: mon,
		Version: "v0.1.0-test",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	e := newAuthedEcho(t, server)

	// 1. GET /api/system/backup
	req := httptest.NewRequest(http.MethodGet, "/api/system/backup", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/system/backup returned status %d: %s", rec.Code, rec.Body.String())
	}
	var status datastore.BackupStatusEnt
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("unmarshal status: %v", err)
	}
	if status.Config.ScheduleTime != "03:00" {
		t.Errorf("expected default schedule time 03:00, got %s", status.Config.ScheduleTime)
	}

	// 2. POST /api/system/backup (update config)
	newConf := datastore.BackupConfEnt{
		Mode:         "daily",
		ScheduleTime: "04:15",
		ConfigOnly:   true,
		Generation:   5,
	}
	body, _ := json.Marshal(newConf)
	req = httptest.NewRequest(http.MethodPost, "/api/system/backup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/system/backup returned %d: %s", rec.Code, rec.Body.String())
	}

	// 3. POST /api/system/backup/run (run backup)
	runReq := `{"config_only": true}`
	req = httptest.NewRequest(http.MethodPost, "/api/system/backup/run", bytes.NewReader([]byte(runReq)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, runReqBody(runReq))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/system/backup/run returned %d: %s", rec.Code, rec.Body.String())
	}
	var runResp struct {
		File       string `json:"file"`
		Size       int64  `json:"size"`
		ConfigOnly bool   `json:"config_only"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &runResp); err != nil {
		t.Fatalf("unmarshal runResp: %v", err)
	}
	if runResp.File == "" || runResp.Size <= 0 || !runResp.ConfigOnly {
		t.Errorf("unexpected runResp: %+v", runResp)
	}

	// Verify file on disk
	filePath := filepath.Join(bDir, runResp.File)
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("backup file %s not found on disk: %v", filePath, err)
	}

	// 4. GET /api/system/backup/download/:name
	req = httptest.NewRequest(http.MethodGet, "/api/system/backup/download/"+runResp.File, nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("download backup returned %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() <= 0 {
		t.Errorf("downloaded backup file is empty")
	}

	// 5. DELETE /api/system/backup/:name
	req = httptest.NewRequest(http.MethodDelete, "/api/system/backup/"+runResp.File, nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete backup returned %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Errorf("expected backup file to be deleted from disk, but it still exists")
	}
}

func runReqBody(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/system/backup/run", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	return req
}
