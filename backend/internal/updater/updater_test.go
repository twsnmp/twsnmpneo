package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestBuildUpdateInfo(t *testing.T) {
	rel := &GitHubRelease{
		TagName:     "v0.2.0",
		Name:        "Release v0.2.0",
		Body:        "## Changes\n- New feature",
		HTMLURL:     "https://github.com/twsnmp/twsnmpneo/releases/tag/v0.2.0",
		PublishedAt: time.Now(),
		Assets: []GitHubAsset{
			{
				Name:               "twsnmpneo-v0.2.0-linux-amd64.tar.gz",
				Size:               1234567,
				BrowserDownloadURL: "https://example.com/linux-amd64.tar.gz",
			},
			{
				Name:               "twsnmpneo-v0.2.0-darwin-arm64.tar.gz",
				Size:               1234567,
				BrowserDownloadURL: "https://example.com/darwin-arm64.tar.gz",
			},
			{
				Name:               "twsnmpneo-v0.2.0-windows-amd64.zip",
				Size:               1234567,
				BrowserDownloadURL: "https://example.com/windows-amd64.zip",
			},
		},
	}

	tests := []struct {
		name        string
		curVersion  string
		wantUpdate  bool
	}{
		{"older version", "v0.1.0", true},
		{"same version", "v0.2.0", false},
		{"newer version", "v0.3.0", false},
		{"dev version", "dev", true},
		{"unknown version", "unknown", true},
		{"without v prefix", "0.1.0", true},
		{"same version without v prefix", "0.2.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := New(Config{
				CurrentVersion: tt.curVersion,
				RepoOwner:      "twsnmp",
				RepoName:       "twsnmpneo",
			})

			info := u.buildUpdateInfo(rel)
			if info.HasUpdate != tt.wantUpdate {
				t.Errorf("buildUpdateInfo() HasUpdate = %v, want %v", info.HasUpdate, tt.wantUpdate)
			}
			if info.LatestVersion != "v0.2.0" {
				t.Errorf("LatestVersion = %s, want v0.2.0", info.LatestVersion)
			}
			if tt.curVersion == "0.2.0" && info.CurrentVersion != "v0.2.0" {
				t.Errorf("CurrentVersion = %s, want v0.2.0", info.CurrentVersion)
			}

			// Check asset match for current platform
			if (runtime.GOOS == "linux" && runtime.GOARCH == "amd64") ||
				(runtime.GOOS == "darwin" && runtime.GOARCH == "arm64") ||
				(runtime.GOOS == "windows" && runtime.GOARCH == "amd64") {
				if info.AssetURL == "" {
					t.Errorf("Expected matched AssetURL for %s/%s, got empty", runtime.GOOS, runtime.GOARCH)
				}
			}
		})
	}
}

func TestExtractFromTarGz(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-targz-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, "test.tar.gz")
	targetName := "twsnmpneo"

	// Create a dummy tar.gz with target executable
	tarFile, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gzw := gzip.NewWriter(tarFile)
	tw := tar.NewWriter(gzw)

	content := []byte("#!/bin/sh\necho test\n")
	hdr := &tar.Header{
		Name:     "dist/" + targetName,
		Mode:     0755,
		Size:     int64(len(content)),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gzw.Close()
	tarFile.Close()

	// Extract
	extracted, err := extractExecutable(archivePath, tmpDir)
	if err != nil {
		t.Fatalf("extractExecutable failed: %v", err)
	}

	data, err := os.ReadFile(extracted)
	if err != nil {
		t.Fatalf("failed to read extracted file: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("extracted content mismatch: got %q, want %q", string(data), string(content))
	}
}

func TestExtractFromZip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-zip-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, "test.zip")
	targetName := "twsnmpneo"
	if runtime.GOOS == "windows" {
		targetName = "twsnmpneo.exe"
	}

	zipFile, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zipFile)

	content := []byte("dummy binary payload")
	w, err := zw.Create("dist/" + targetName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(content); err != nil {
		t.Fatal(err)
	}
	zw.Close()
	zipFile.Close()

	// Extract
	extracted, err := extractExecutable(archivePath, tmpDir)
	if err != nil {
		t.Fatalf("extractExecutable failed: %v", err)
	}

	data, err := os.ReadFile(extracted)
	if err != nil {
		t.Fatalf("failed to read extracted file: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("extracted content mismatch: got %q, want %q", string(data), string(content))
	}
}

func TestSwapBinary(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-swap-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	currentExe := filepath.Join(tmpDir, "twsnmpneo")
	newBinary := filepath.Join(tmpDir, "new_twsnmpneo")
	bakPath := currentExe + ".bak"

	if err := os.WriteFile(currentExe, []byte("v1"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newBinary, []byte("v2"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := swapBinary(newBinary, currentExe, bakPath); err != nil {
		t.Fatalf("swapBinary failed: %v", err)
	}

	curData, err := os.ReadFile(currentExe)
	if err != nil {
		t.Fatal(err)
	}
	if string(curData) != "v2" {
		t.Errorf("currentExe content = %s, want v2", string(curData))
	}

	bakData, err := os.ReadFile(bakPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(bakData) != "v1" {
		t.Errorf("bakPath content = %s, want v1", string(bakData))
	}
}

func TestCheckUpdate_ETagAndRateLimit(t *testing.T) {
	callCount := 0
	etag := `W/"mock-etag-123"`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		// Check If-None-Match header
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		if callCount > 2 {
			// Simulate 403 Rate Limit
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", "1800000000")
			w.WriteHeader(http.StatusForbidden)
			return
		}

		// First response with ETag
		w.Header().Set("ETag", etag)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(GitHubRelease{
			TagName: "v0.5.0",
			Name:    "v0.5.0",
		})
	}))
	defer ts.Close()

	u := New(Config{
		CurrentVersion: "v0.1.0",
		RepoOwner:      "mock",
		RepoName:       "mock",
	})
	u.httpClient = ts.Client()

	// 1. First fetch: should return 200 and save ETag
	ctx := context.Background()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL, nil)
	resp, err := u.httpClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var rel GitHubRelease
	_ = json.NewDecoder(resp.Body).Decode(&rel)
	resp.Body.Close()

	u.cachedInfo = u.buildUpdateInfo(&rel)
	u.cachedETag = resp.Header.Get("ETag")
	u.cacheTime = time.Now().Add(-2 * time.Hour) // Expire TTL to test ETag call

	if u.cachedETag != etag {
		t.Errorf("cachedETag = %s, want %s", u.cachedETag, etag)
	}

	// 2. Second fetch with ETag: should send If-None-Match and receive 304
	req2, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL, nil)
	req2.Header.Set("If-None-Match", u.cachedETag)
	resp2, err := u.httpClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotModified {
		t.Errorf("expected 304 Not Modified, got %d", resp2.StatusCode)
	}
}

