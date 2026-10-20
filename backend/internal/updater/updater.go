package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/semver"
)

// GitHubRelease represents the structure returned by GitHub Releases API
type GitHubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	HTMLURL     string        `json:"html_url"`
	PublishedAt time.Time     `json:"published_at"`
	Assets      []GitHubAsset `json:"assets"`
}

// GitHubAsset represents a binary asset attached to a release
type GitHubAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// UpdateInfo holds details about available updates
type UpdateInfo struct {
	CurrentVersion string    `json:"current_version"`
	LatestVersion  string    `json:"latest_version"`
	HasUpdate      bool      `json:"has_update"`
	ReleaseTitle   string    `json:"release_title"`
	ReleaseBody    string    `json:"release_body"`
	ReleaseURL     string    `json:"release_url"`
	PublishedAt    time.Time `json:"published_at"`
	IsContainer    bool      `json:"is_container"`
	IsHomebrew     bool      `json:"is_homebrew"`
	IsDebug        bool      `json:"is_debug"`
	Environment    string    `json:"environment"`
	CanSelfUpdate  bool      `json:"can_self_update"`
	UpdateGuide    string    `json:"update_guide"`
	AssetName      string    `json:"asset_name"`
	AssetURL       string    `json:"asset_url"`
	AssetSize      int64     `json:"asset_size"`
}

// UpdateStatus represents the live status of an in-progress self-update
type UpdateStatus struct {
	State    string `json:"state"` // "idle", "checking", "downloading", "extracting", "restarting", "completed", "error"
	Progress int    `json:"progress"`
	Message  string `json:"message"`
	Error    string `json:"error,omitempty"`
}

// Config holds configuration options for the Updater
type Config struct {
	CurrentVersion string
	Commit         string
	Debug          bool
	RepoOwner      string
	RepoName       string
	OnShutdown     func()
}

// Updater handles checking for GitHub releases, downloading, and hot-updating the executable
type Updater struct {
	currentVersion string
	commit         string
	debug          bool
	repoOwner      string
	repoName       string
	onShutdown     func()

	mu         sync.RWMutex
	status     UpdateStatus
	cachedInfo *UpdateInfo
	cachedETag string
	cacheTime  time.Time
	cacheTTL   time.Duration

	httpClient *http.Client
}

// New creates and initializes a new Updater instance
func New(cfg Config) *Updater {
	owner := cfg.RepoOwner
	if owner == "" {
		owner = "twsnmp"
	}
	repo := cfg.RepoName
	if repo == "" {
		repo = "twsnmpneo"
	}
	ver := cfg.CurrentVersion
	if ver == "" {
		ver = "dev"
	}

	return &Updater{
		currentVersion: ver,
		commit:         cfg.Commit,
		debug:          cfg.Debug,
		repoOwner:      owner,
		repoName:       repo,
		onShutdown:     cfg.OnShutdown,
		cacheTTL:       1 * time.Hour,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		status: UpdateStatus{
			State:   "idle",
			Message: "Ready",
		},
	}
}

// IsContainer returns true if the current binary is running inside a Docker or containerized environment
func IsContainer() bool {
	// 1. Check for standard container flag files
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	if _, err := os.Stat("/run/.containerenv"); err == nil {
		return true
	}

	// 2. Check environment variables
	if os.Getenv("container") == "docker" || os.Getenv("container") == "podman" {
		return true
	}

	// 3. Inspect cgroup on Linux
	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
			content := string(data)
			if strings.Contains(content, "docker") || strings.Contains(content, "kubepods") || strings.Contains(content, "containerd") {
				return true
			}
		}
	}

	return false
}

// GetStatus returns the current live update status
func (u *Updater) GetStatus() UpdateStatus {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.status
}

func (u *Updater) setStatus(state string, progress int, msg string, errStr string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.status = UpdateStatus{
		State:    state,
		Progress: progress,
		Message:  msg,
		Error:    errStr,
	}
}

// CheckUpdate checks GitHub Releases API for the latest release with ETag caching and rate-limit protection
func (u *Updater) CheckUpdate(ctx context.Context, force bool) (*UpdateInfo, error) {
	u.mu.RLock()
	if !force && u.cachedInfo != nil && time.Since(u.cacheTime) < u.cacheTTL {
		info := *u.cachedInfo
		u.mu.RUnlock()
		return &info, nil
	}
	cachedETag := u.cachedETag
	cachedInfo := u.cachedInfo
	u.mu.RUnlock()

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", u.repoOwner, u.repoName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create update check request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "twsnmpneo-updater/"+u.currentVersion)
	if cachedETag != "" && !force {
		req.Header.Set("If-None-Match", cachedETag)
	}
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := u.httpClient.Do(req)
	if err != nil {
		// If network fails but we have cached info, gracefully return cached info
		if cachedInfo != nil {
			slog.Warn("GitHub API check failed; returning cached release info", "error", err)
			return cachedInfo, nil
		}
		return nil, fmt.Errorf("failed to contact GitHub API: %w", err)
	}
	defer resp.Body.Close()

	// 1. Handle 304 Not Modified: Release hasn't changed, does not consume GitHub rate limit
	if resp.StatusCode == http.StatusNotModified && cachedInfo != nil {
		slog.Debug("GitHub release not modified (304); rate limit preserved")
		u.mu.Lock()
		u.cacheTime = time.Now()
		u.mu.Unlock()
		return cachedInfo, nil
	}

	// 2. Handle 403 Rate Limit Exceeded
	if resp.StatusCode == http.StatusForbidden {
		remaining := resp.Header.Get("X-RateLimit-Remaining")
		if remaining == "0" || strings.Contains(resp.Header.Get("X-RateLimit-Reset"), "") {
			resetUnix := resp.Header.Get("X-RateLimit-Reset")
			resetTime := "soon"
			if sec, pErr := strconv.ParseInt(resetUnix, 10, 64); pErr == nil {
				resetTime = time.Unix(sec, 0).Format("15:04:05")
			}
			if cachedInfo != nil {
				slog.Warn("GitHub API rate limit exceeded; fallback to cached release info", "reset", resetTime)
				return cachedInfo, nil
			}
			return nil, fmt.Errorf("GitHub API rate limit reached (resets at %s). Set GITHUB_TOKEN or check again later.", resetTime)
		}
		return nil, fmt.Errorf("GitHub API returned status: %d", resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status: %d", resp.StatusCode)
	}

	var rel GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("failed to parse GitHub release response: %w", err)
	}

	info := u.buildUpdateInfo(&rel)
	newETag := resp.Header.Get("ETag")

	u.mu.Lock()
	u.cachedInfo = info
	u.cachedETag = newETag
	u.cacheTime = time.Now()
	u.mu.Unlock()

	return info, nil
}

// buildUpdateInfo constructs an UpdateInfo struct by comparing versions and finding compatible assets
func (u *Updater) buildUpdateInfo(rel *GitHubRelease) *UpdateInfo {
	latest := rel.TagName
	if !strings.HasPrefix(latest, "v") {
		latest = "v" + latest
	}

	cur := u.currentVersion
	if cur != "" && !strings.HasPrefix(cur, "v") && cur != "dev" && cur != "unknown" {
		cur = "v" + cur
	}

	hasUpdate := false
	if cur == "dev" || cur == "unknown" || cur == "" {
		hasUpdate = true
	} else if semver.IsValid(latest) && semver.IsValid(cur) {
		hasUpdate = semver.Compare(latest, cur) > 0
	} else if latest != cur {
		hasUpdate = true
	}

	env := DetectEnvironment(u.currentVersion, u.commit, u.debug)

	// Find matching binary asset for runtime.GOOS and runtime.GOARCH
	var matchedAsset *GitHubAsset
	osName := runtime.GOOS
	archName := runtime.GOARCH

	for i := range rel.Assets {
		name := strings.ToLower(rel.Assets[i].Name)
		if strings.Contains(name, osName) && strings.Contains(name, archName) {
			matchedAsset = &rel.Assets[i]
			break
		}
	}

	info := &UpdateInfo{
		CurrentVersion: cur,
		LatestVersion:  latest,
		HasUpdate:      hasUpdate,
		ReleaseTitle:   rel.Name,
		ReleaseBody:    rel.Body,
		ReleaseURL:     rel.HTMLURL,
		PublishedAt:    rel.PublishedAt,
		IsContainer:    env.IsContainer,
		IsHomebrew:     env.IsHomebrew,
		IsDebug:        env.IsDebug,
		Environment:    string(env.Type),
		CanSelfUpdate:  env.CanSelfUpdate,
		UpdateGuide:    env.UpdateGuide,
	}

	if matchedAsset != nil {
		info.AssetName = matchedAsset.Name
		info.AssetURL = matchedAsset.BrowserDownloadURL
		info.AssetSize = matchedAsset.Size
	}

	return info
}

// GetEnvironment returns the detected execution environment
func (u *Updater) GetEnvironment() EnvironmentInfo {
	return DetectEnvironment(u.currentVersion, u.commit, u.debug)
}

// ApplyUpdate triggers the background download, validation, binary swap, and clean restart
func (u *Updater) ApplyUpdate(ctx context.Context) error {
	env := DetectEnvironment(u.currentVersion, u.commit, u.debug)
	if env.IsDebug {
		return errors.New("cannot perform self-update in debug mode; self-update is disabled")
	}
	if env.IsHomebrew {
		return errors.New("twsnmpneo is installed via Homebrew; please run 'brew upgrade twsnmp/tap/twsnmpneo' to update")
	}
	if env.IsContainer {
		return errors.New("cannot perform in-place self-update within a container; please pull the latest container image instead")
	}
	if !env.CanSelfUpdate {
		return errors.New("self-update is not supported in this environment: " + env.UpdateGuide)
	}

	u.mu.Lock()
	if u.status.State == "downloading" || u.status.State == "extracting" || u.status.State == "restarting" {
		u.mu.Unlock()
		return errors.New("update is already in progress")
	}
	u.status = UpdateStatus{
		State:    "checking",
		Progress: 5,
		Message:  "Checking latest release...",
	}
	u.mu.Unlock()

	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		if err := u.runUpdatePipeline(bgCtx, ""); err != nil {
			slog.Error("Self-update pipeline failed", "error", err)
			u.setStatus("error", 0, "Update failed", err.Error())
		}
	}()

	return nil
}

// RunUpdateDirect executes the update pipeline synchronously
func (u *Updater) RunUpdateDirect(ctx context.Context, specificTag string) error {
	env := DetectEnvironment(u.currentVersion, u.commit, u.debug)
	if env.IsDebug {
		return errors.New("cannot perform self-update in debug mode; self-update is disabled")
	}
	if env.IsHomebrew {
		return errors.New("twsnmpneo is installed via Homebrew; please run 'brew upgrade twsnmp/tap/twsnmpneo' to update")
	}
	if env.IsContainer {
		return errors.New("cannot perform in-place self-update within a container; please pull the latest container image instead")
	}
	if !env.CanSelfUpdate {
		return errors.New("self-update is not supported in this environment: " + env.UpdateGuide)
	}

	return u.runUpdatePipeline(ctx, specificTag)
}

// runUpdatePipeline downloads, extracts, swaps the binary, and launches the new process
func (u *Updater) runUpdatePipeline(ctx context.Context, specificTag string) error {
	u.setStatus("checking", 10, "Fetching release information...", "")

	var rel *GitHubRelease
	var err error
	if specificTag != "" {
		rel, err = u.fetchReleaseByTag(ctx, specificTag)
	} else {
		rel, err = u.fetchLatestRelease(ctx)
	}
	if err != nil {
		return fmt.Errorf("fetch release error: %w", err)
	}

	info := u.buildUpdateInfo(rel)
	if info.AssetURL == "" {
		return fmt.Errorf("no compatible release asset found for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	// 1. Download asset to temporary directory
	u.setStatus("downloading", 20, fmt.Sprintf("Downloading %s...", info.AssetName), "")
	tmpDir, err := os.MkdirTemp("", "twsnmpneo-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, info.AssetName)
	if err := u.downloadFile(ctx, info.AssetURL, archivePath, func(percent int) {
		u.setStatus("downloading", 20+int(float64(percent)*0.5), fmt.Sprintf("Downloading... %d%%", percent), "")
	}); err != nil {
		return fmt.Errorf("download error: %w", err)
	}

	// 2. Extract executable
	u.setStatus("extracting", 75, "Extracting binary...", "")
	extractedBin, err := extractExecutable(archivePath, tmpDir)
	if err != nil {
		return fmt.Errorf("extraction error: %w", err)
	}

	// Make sure the extracted binary is executable
	if err := os.Chmod(extractedBin, 0755); err != nil {
		return fmt.Errorf("failed to chmod binary: %w", err)
	}

	// 3. Locate current executable
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to locate current executable: %w", err)
	}
	currentExe, err = filepath.EvalSymlinks(currentExe)
	if err != nil {
		return fmt.Errorf("failed to resolve symlink for executable: %w", err)
	}

	// 4. Backup current binary
	u.setStatus("extracting", 85, "Backing up current binary...", "")
	bakPath := currentExe + ".bak"
	_ = os.Remove(bakPath) // Remove any older backup
	if err := copyFile(currentExe, bakPath); err != nil {
		slog.Warn("Failed to create .bak copy, attempting rename backup", "error", err)
	}

	// 5. Replace current binary
	u.setStatus("extracting", 90, "Installing new binary...", "")
	if err := swapBinary(extractedBin, currentExe, bakPath); err != nil {
		return fmt.Errorf("failed to swap binary: %w", err)
	}

	// 6. Spawn new process & graceful exit
	u.setStatus("restarting", 95, "Restarting TWSNMP NEO...", "")
	time.Sleep(500 * time.Millisecond) // Allow UI to poll status

	if err := restartProcess(currentExe); err != nil {
		return fmt.Errorf("failed to launch updated process: %w", err)
	}

	if u.onShutdown != nil {
		u.onShutdown()
	}

	os.Exit(0)
	return nil
}

// Rollback restores a previous binary from backup or fetches a specified version
func (u *Updater) Rollback(ctx context.Context, targetVersion string) error {
	if IsContainer() {
		return errors.New("cannot perform rollback within a container; please start the desired container image tag instead (e.g. ghcr.io/twsnmp/twsnmpneo:" + targetVersion + ")")
	}

	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to locate current executable: %w", err)
	}
	currentExe, err = filepath.EvalSymlinks(currentExe)
	if err != nil {
		return fmt.Errorf("failed to resolve executable symlinks: %w", err)
	}

	bakPath := currentExe + ".bak"

	// If no version or "previous" is specified, restore from .bak
	if targetVersion == "" || targetVersion == "previous" {
		if _, err := os.Stat(bakPath); err != nil {
			return fmt.Errorf("backup binary not found at %s: %w", bakPath, err)
		}

		fmt.Printf("Restoring previous binary from %s to %s...\n", bakPath, currentExe)
		if err := swapBinary(bakPath, currentExe, currentExe+".tmp"); err != nil {
			return fmt.Errorf("failed to restore backup: %w", err)
		}
		_ = os.Remove(currentExe + ".tmp")
		fmt.Println("Rollback to previous version completed successfully.")
		return nil
	}

	// Target version specified: download that release from GitHub
	fmt.Printf("Downloading target version %s for rollback...\n", targetVersion)
	if !strings.HasPrefix(targetVersion, "v") {
		targetVersion = "v" + targetVersion
	}

	rel, err := u.fetchReleaseByTag(ctx, targetVersion)
	if err != nil {
		return fmt.Errorf("failed to fetch release %s: %w", targetVersion, err)
	}

	info := u.buildUpdateInfo(rel)
	if info.AssetURL == "" {
		return fmt.Errorf("no matching asset found for %s/%s in release %s", runtime.GOOS, runtime.GOARCH, targetVersion)
	}

	tmpDir, err := os.MkdirTemp("", "twsnmpneo-rollback-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, info.AssetName)
	if err := u.downloadFile(ctx, info.AssetURL, archivePath, func(percent int) {
		fmt.Printf("\rDownloading %s: %d%%", info.AssetName, percent)
	}); err != nil {
		fmt.Println()
		return fmt.Errorf("download failed: %w", err)
	}
	fmt.Println("\nExtracting...")

	extractedBin, err := extractExecutable(archivePath, tmpDir)
	if err != nil {
		return fmt.Errorf("extraction error: %w", err)
	}

	_ = os.Chmod(extractedBin, 0755)

	// Backup current binary to .bak
	_ = copyFile(currentExe, bakPath)

	if err := swapBinary(extractedBin, currentExe, bakPath); err != nil {
		return fmt.Errorf("failed to swap binary: %w", err)
	}

	fmt.Printf("Successfully rolled back TWSNMP NEO to %s.\n", targetVersion)
	return nil
}

func (u *Updater) fetchLatestRelease(ctx context.Context) (*GitHubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", u.repoOwner, u.repoName)
	return u.doReleaseRequest(ctx, url)
}

func (u *Updater) fetchReleaseByTag(ctx context.Context, tag string) (*GitHubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/%s", u.repoOwner, u.repoName, tag)
	return u.doReleaseRequest(ctx, url)
}

func (u *Updater) doReleaseRequest(ctx context.Context, url string) (*GitHubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "twsnmpneo-updater/"+u.currentVersion)
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		resetTime := resp.Header.Get("X-RateLimit-Reset")
		return nil, fmt.Errorf("GitHub API rate limit exceeded (reset at %s). Please set GITHUB_TOKEN or try again later", resetTime)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var rel GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

func (u *Updater) downloadFile(ctx context.Context, url, destPath string, onProgress func(percent int)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "twsnmpneo-updater/"+u.currentVersion)
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("asset download returned status %d", resp.StatusCode)
	}

	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	totalSize := resp.ContentLength
	var downloaded int64
	buf := make([]byte, 32*1024)
	lastReport := time.Now()

	for {
		n, rErr := resp.Body.Read(buf)
		if n > 0 {
			if _, wErr := out.Write(buf[:n]); wErr != nil {
				return wErr
			}
			downloaded += int64(n)
			if totalSize > 0 && onProgress != nil && time.Since(lastReport) > 100*time.Millisecond {
				percent := int((float64(downloaded) / float64(totalSize)) * 100)
				if percent > 100 {
					percent = 100
				}
				onProgress(percent)
				lastReport = time.Now()
			}
		}
		if rErr != nil {
			if rErr == io.EOF {
				break
			}
			return rErr
		}
	}

	if onProgress != nil {
		onProgress(100)
	}
	return nil
}

// extractExecutable extracts the binary executable from .tar.gz or .zip
func extractExecutable(archivePath, destDir string) (string, error) {
	expectedName := "twsnmpneo"
	if runtime.GOOS == "windows" {
		expectedName = "twsnmpneo.exe"
	}

	if strings.HasSuffix(archivePath, ".tar.gz") || strings.HasSuffix(archivePath, ".tgz") {
		return extractFromTarGz(archivePath, destDir, expectedName)
	}
	if strings.HasSuffix(archivePath, ".zip") {
		return extractFromZip(archivePath, destDir, expectedName)
	}

	return "", fmt.Errorf("unsupported archive format: %s", filepath.Base(archivePath))
}

func extractFromTarGz(archivePath, destDir, targetName string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		baseName := filepath.Base(header.Name)
		if baseName == targetName && header.Typeflag == tar.TypeReg {
			dest := filepath.Join(destDir, targetName)
			out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return "", err
			}
			out.Close()
			return dest, nil
		}
	}

	return "", fmt.Errorf("target executable '%s' not found inside archive", targetName)
}

func extractFromZip(archivePath, destDir, targetName string) (string, error) {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer zr.Close()

	for _, file := range zr.File {
		baseName := filepath.Base(file.Name)
		if baseName == targetName && !file.FileInfo().IsDir() {
			rc, err := file.Open()
			if err != nil {
				return "", err
			}
			dest := filepath.Join(destDir, targetName)
			out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				rc.Close()
				return "", err
			}
			if _, err := io.Copy(out, rc); err != nil {
				rc.Close()
				out.Close()
				return "", err
			}
			rc.Close()
			out.Close()
			return dest, nil
		}
	}

	return "", fmt.Errorf("target executable '%s' not found inside zip", targetName)
}

// swapBinary replaces currentExe with newBinary, preserving backup
func swapBinary(newBinary, currentExe, bakPath string) error {
	// 1. Create backup if bakPath is provided and currentExe exists
	if bakPath != "" {
		if _, err := os.Stat(currentExe); err == nil {
			_ = os.Remove(bakPath)
			if err := copyFile(currentExe, bakPath); err != nil {
				slog.Warn("Failed to copy executable to backup path", "error", err)
			}
		}
	}

	if runtime.GOOS == "windows" {
		// On Windows: rename current -> bak, then move new -> current
		_ = os.Remove(bakPath)
		if err := os.Rename(currentExe, bakPath); err != nil {
			return fmt.Errorf("failed to move current executable to backup: %w", err)
		}
		if err := copyFile(newBinary, currentExe); err != nil {
			// Rollback rename if copy fails
			_ = os.Rename(bakPath, currentExe)
			return fmt.Errorf("failed to install new executable: %w", err)
		}
		return nil
	}

	// On Unix / macOS:
	// Move new binary into same directory as currentExe to guarantee same filesystem for atomic rename
	tempDest := filepath.Join(filepath.Dir(currentExe), ".twsnmpneo.update.tmp")
	_ = os.Remove(tempDest)

	if err := copyFile(newBinary, tempDest); err != nil {
		return fmt.Errorf("failed to stage new binary: %w", err)
	}
	if err := os.Chmod(tempDest, 0755); err != nil {
		_ = os.Remove(tempDest)
		return fmt.Errorf("failed to chmod staged binary: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempDest, currentExe); err != nil {
		_ = os.Remove(tempDest)
		return fmt.Errorf("failed to replace binary atomically: %w", err)
	}

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// restartProcess launches the new executable with the same arguments and environment
func restartProcess(exePath string) error {
	return restart(exePath)
}

// Helper buffer for testing release JSON parsing
func parseReleaseTest(data []byte) (*GitHubRelease, error) {
	var rel GitHubRelease
	if err := json.NewDecoder(bytes.NewReader(data)).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}
