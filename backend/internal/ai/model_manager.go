package ai

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/twsnmp/twsnmpneo/backend/internal/ai/tensai"
)

// WGPUVersion specifies the target wgpu-native release version
const WGPUVersion = "v24.0.0.1"

// ModelInfo represents information about a local model file or directory
type ModelInfo struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	SizeHuman string    `json:"size_human"`
	ModTime   time.Time `json:"mod_time"`
	Type      string    `json:"type"` // "gguf", "directory"
}

// PresetModelInfo holds metadata for a recommended preset model
type PresetModelInfo struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Size        string `json:"size"`
	Params      string `json:"params"`
}

// AIHardwareStatus contains hardware acceleration details and library paths
type AIHardwareStatus struct {
	Acceleration string `json:"acceleration"`
	Detail       string `json:"detail"`
	ModelDir     string `json:"model_dir"`
	LibDir       string `json:"lib_dir"`
	WGPULibPath  string `json:"wgpu_lib_path"`
	HasGPULib    bool   `json:"has_gpu_lib"`
}

// DownloadProgressInfo tracks current download statistics
type DownloadProgressInfo struct {
	Downloading     bool   `json:"downloading"`
	Target          string `json:"target,omitempty"`
	Downloaded      int64  `json:"downloaded"`
	Total           int64  `json:"total"`
	Percent         int    `json:"percent"`
	DownloadedHuman string `json:"downloaded_human"`
	TotalHuman      string `json:"total_human"`
	Error           string `json:"error,omitempty"`
}

// AIDownloadStatus provides active status for both model and GPU library downloads
type AIDownloadStatus struct {
	Model DownloadProgressInfo `json:"model"`
	GPU   DownloadProgressInfo `json:"gpu"`
}

// PresetModels defines recommended lightweight models
var PresetModels = map[string]string{
	"qwen2.5-0.5b":       "https://huggingface.co/Qwen/Qwen2.5-0.5B-Instruct-GGUF/resolve/main/qwen2.5-0.5b-instruct-q8_0.gguf",
	"qwen2.5-1.5b":       "https://huggingface.co/Qwen/Qwen2.5-1.5B-Instruct-GGUF/resolve/main/qwen2.5-1.5b-instruct-q4_k_m.gguf",
	"qwen2.5-coder-0.5b": "https://huggingface.co/Qwen/Qwen2.5-Coder-0.5B-Instruct-GGUF/resolve/main/qwen2.5-coder-0.5b-instruct-q8_0.gguf",
	"qwen2.5-coder-1.5b": "https://huggingface.co/Qwen/Qwen2.5-Coder-1.5B-Instruct-GGUF/resolve/main/qwen2.5-coder-1.5b-instruct-q4_k_m.gguf",
	"smollm2-360m":       "https://huggingface.co/HuggingFaceTB/SmolLM2-360M-Instruct-GGUF/resolve/main/smollm2-360m-instruct-q8_0.gguf",
	"smollm2-1.7b":       "https://huggingface.co/HuggingFaceTB/SmolLM2-1.7B-Instruct-GGUF/resolve/main/smollm2-1.7b-instruct-q4_k_m.gguf",
	"llama-3.2-1b":       "https://huggingface.co/unsloth/Llama-3.2-1B-Instruct-GGUF/resolve/main/Llama-3.2-1B-Instruct-Q4_K_M.gguf",
	"deepseek-r1-1.5b":   "https://huggingface.co/unsloth/DeepSeek-R1-Distill-Qwen-1.5B-GGUF/resolve/main/DeepSeek-R1-Distill-Qwen-1.5B-Q4_K_M.gguf",
	"tinyllama":          "https://huggingface.co/TheBloke/TinyLlama-1.1B-Chat-v1.0-GGUF/resolve/main/tinyllama-1.1b-chat-v1.0.Q4_K_M.gguf",
}

// PresetModelMetadata provides UI-friendly information for preset models
var PresetModelMetadata = []PresetModelInfo{
	{Name: "qwen2.5-0.5b", URL: PresetModels["qwen2.5-0.5b"], Description: "Qwen 2.5 0.5B Instruct (Q8_0, Recommended Fast)", Size: "~500 MB", Params: "0.5B"},
	{Name: "qwen2.5-1.5b", URL: PresetModels["qwen2.5-1.5b"], Description: "Qwen 2.5 1.5B Instruct (Q4_K_M, Balanced)", Size: "~980 MB", Params: "1.5B"},
	{Name: "qwen2.5-coder-0.5b", URL: PresetModels["qwen2.5-coder-0.5b"], Description: "Qwen 2.5 Coder 0.5B Instruct (Q8_0)", Size: "~500 MB", Params: "0.5B"},
	{Name: "qwen2.5-coder-1.5b", URL: PresetModels["qwen2.5-coder-1.5b"], Description: "Qwen 2.5 Coder 1.5B Instruct (Q4_K_M)", Size: "~980 MB", Params: "1.5B"},
	{Name: "smollm2-360m", URL: PresetModels["smollm2-360m"], Description: "SmolLM2 360M Instruct (Q8_0, Ultra-lightweight)", Size: "~380 MB", Params: "360M"},
	{Name: "smollm2-1.7b", URL: PresetModels["smollm2-1.7b"], Description: "SmolLM2 1.7B Instruct (Q4_K_M)", Size: "~1.1 GB", Params: "1.7B"},
	{Name: "llama-3.2-1b", URL: PresetModels["llama-3.2-1b"], Description: "Llama 3.2 1B Instruct (Q4_K_M)", Size: "~750 MB", Params: "1B"},
	{Name: "deepseek-r1-1.5b", URL: PresetModels["deepseek-r1-1.5b"], Description: "DeepSeek R1 Distill Qwen 1.5B (Q4_K_M)", Size: "~1.1 GB", Params: "1.5B"},
	{Name: "tinyllama", URL: PresetModels["tinyllama"], Description: "TinyLlama 1.1B Chat (Q4_K_M)", Size: "~670 MB", Params: "1.1B"},
}

// DownloadProgress callback
type DownloadProgress func(downloaded, total int64)

// DownloadManager manages background model and GPU library downloads per datastore
type DownloadManager struct {
	dataDir string

	modelMu        sync.Mutex
	modelCancelCtx context.CancelFunc
	modelStatus    DownloadProgressInfo

	gpuMu        sync.Mutex
	gpuCancelCtx context.CancelFunc
	gpuStatus    DownloadProgressInfo
}

var (
	defaultMgrMu sync.Mutex
	defaultMgr   *DownloadManager
)

// GetDownloadManager returns or initializes the singleton DownloadManager for the datadir
func GetDownloadManager(dataDir string) *DownloadManager {
	defaultMgrMu.Lock()
	defer defaultMgrMu.Unlock()
	if defaultMgr == nil || defaultMgr.dataDir != dataDir {
		defaultMgr = &DownloadManager{
			dataDir: dataDir,
		}
	}
	return defaultMgr
}

// ModelDir returns the model directory inside the selected dataDir
func (m *DownloadManager) ModelDir() string {
	return filepath.Join(m.dataDir, "models")
}

// LibDir returns the library directory inside the selected dataDir
func (m *DownloadManager) LibDir() string {
	return filepath.Join(m.dataDir, "lib")
}

// GetHardwareStatus inspects current acceleration and library files
func (m *DownloadManager) GetHardwareStatus() AIHardwareStatus {
	tensai.SetCustomLibDir(m.LibDir())
	tensai.InitWGPULibrary(m.LibDir())
	accType, accDetail := tensai.DetectAcceleration()

	wgpuLib := os.Getenv("TENSAI_WGPU_LIB")
	hasLib := wgpuLib != "" && fileExists(wgpuLib)

	return AIHardwareStatus{
		Acceleration: string(accType),
		Detail:       accDetail,
		ModelDir:     m.ModelDir(),
		LibDir:       m.LibDir(),
		WGPULibPath:  wgpuLib,
		HasGPULib:    hasLib,
	}
}

// ListModels returns local models in the datastore's models folder
func (m *DownloadManager) ListModels() ([]ModelInfo, error) {
	modelDir := m.ModelDir()
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(modelDir)
	if err != nil {
		return nil, err
	}

	var list []ModelInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		name := entry.Name()
		fullPath := filepath.Join(modelDir, name)

		if entry.IsDir() {
			cfgPath := filepath.Join(fullPath, "config.json")
			if _, err := os.Stat(cfgPath); err == nil {
				var dirSize int64
				_ = filepath.Walk(fullPath, func(_ string, fi os.FileInfo, _ error) error {
					if fi != nil && !fi.IsDir() {
						dirSize += fi.Size()
					}
					return nil
				})
				list = append(list, ModelInfo{
					Name:      name,
					Path:      fullPath,
					Size:      dirSize,
					SizeHuman: humanize.Bytes(uint64(dirSize)),
					ModTime:   info.ModTime(),
					Type:      "directory",
				})
			}
		} else if strings.HasSuffix(strings.ToLower(name), ".gguf") {
			list = append(list, ModelInfo{
				Name:      name,
				Path:      fullPath,
				Size:      info.Size(),
				SizeHuman: humanize.Bytes(uint64(info.Size())),
				ModTime:   info.ModTime(),
				Type:      "gguf",
			})
		}
	}

	return list, nil
}

// FindModel locates a model by name, filename, preset name in the models folder
func (m *DownloadManager) FindModel(name string) (string, error) {
	primaryDir := m.ModelDir()

	if name != "" {
		if fi, err := os.Stat(name); err == nil {
			if fi.IsDir() {
				if _, err := os.Stat(filepath.Join(name, "config.json")); err == nil {
					return name, nil
				}
			} else if strings.HasSuffix(strings.ToLower(name), ".gguf") {
				return name, nil
			}
		}

		p := filepath.Join(primaryDir, name)
		if fi, err := os.Stat(p); err == nil {
			if !fi.IsDir() || fileExists(filepath.Join(p, "config.json")) {
				return p, nil
			}
		}
		if !strings.HasSuffix(strings.ToLower(p), ".gguf") {
			pGguf := p + ".gguf"
			if _, err := os.Stat(pGguf); err == nil {
				return pGguf, nil
			}
		}

		// Check if name is a known preset
		if presetURL, ok := PresetModels[strings.ToLower(name)]; ok {
			filename := filepath.Base(presetURL)
			if idx := strings.Index(filename, "?"); idx != -1 {
				filename = filename[:idx]
			}
			pPreset := filepath.Join(primaryDir, filename)
			if _, err := os.Stat(pPreset); err == nil {
				return pPreset, nil
			}
		}
	}

	models, err := m.ListModels()
	if err != nil {
		return "", err
	}
	if len(models) == 0 {
		return "", fmt.Errorf("no models found in %s", primaryDir)
	}
	if name == "" {
		return models[0].Path, nil
	}

	// 1. Exact match (by filename or filename without extension)
	for _, mdl := range models {
		if strings.EqualFold(mdl.Name, name) || strings.EqualFold(strings.TrimSuffix(mdl.Name, ".gguf"), name) {
			return mdl.Path, nil
		}
	}

	// 2. Preset match against list of models
	if presetURL, ok := PresetModels[strings.ToLower(name)]; ok {
		presetFile := filepath.Base(presetURL)
		if idx := strings.Index(presetFile, "?"); idx != -1 {
			presetFile = presetFile[:idx]
		}
		presetBase := strings.TrimSuffix(presetFile, ".gguf")
		for _, mdl := range models {
			if strings.EqualFold(mdl.Name, presetFile) || strings.EqualFold(strings.TrimSuffix(mdl.Name, ".gguf"), presetBase) {
				return mdl.Path, nil
			}
		}
	}

	// 3. Prefix or contains match
	nameLower := strings.ToLower(name)
	for _, mdl := range models {
		mBase := strings.ToLower(strings.TrimSuffix(mdl.Name, ".gguf"))
		if strings.HasPrefix(mBase, nameLower) || strings.Contains(mBase, nameLower) {
			return mdl.Path, nil
		}
	}

	return "", fmt.Errorf("model %q not found in %s", name, primaryDir)
}

// DeleteModel deletes a model from the datastore's models folder
func (m *DownloadManager) DeleteModel(name string) error {
	p, err := m.FindModel(name)
	if err != nil {
		targetPath := filepath.Join(m.ModelDir(), name)
		if _, statErr := os.Stat(targetPath); statErr == nil {
			return os.RemoveAll(targetPath)
		}
		return err
	}
	return os.RemoveAll(p)
}

// StartModelDownload starts downloading a model in the background
func (m *DownloadManager) StartModelDownload(target string) error {
	m.modelMu.Lock()
	if m.modelStatus.Downloading {
		m.modelMu.Unlock()
		return fmt.Errorf("another model download is already in progress")
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.modelCancelCtx = cancel
	m.modelStatus = DownloadProgressInfo{
		Downloading:     true,
		Target:          target,
		Downloaded:      0,
		Total:           0,
		Percent:         0,
		DownloadedHuman: "0 B",
		TotalHuman:      "0 B",
	}
	m.modelMu.Unlock()

	go func() {
		defer func() {
			m.modelMu.Lock()
			m.modelCancelCtx = nil
			m.modelStatus.Downloading = false
			m.modelMu.Unlock()
		}()

		progress := func(downloaded, total int64) {
			pct := 0
			if total > 0 {
				pct = int(float64(downloaded) / float64(total) * 100)
			}
			m.modelMu.Lock()
			m.modelStatus.Downloaded = downloaded
			m.modelStatus.Total = total
			m.modelStatus.Percent = pct
			m.modelStatus.DownloadedHuman = humanize.Bytes(uint64(downloaded))
			m.modelStatus.TotalHuman = humanize.Bytes(uint64(total))
			m.modelMu.Unlock()
		}

		_, err := downloadModelFile(ctx, m.ModelDir(), target, progress)
		m.modelMu.Lock()
		if err != nil && err != context.Canceled {
			m.modelStatus.Error = err.Error()
		} else {
			m.modelStatus.Error = ""
		}
		m.modelMu.Unlock()
	}()

	return nil
}

// CancelModelDownload cancels ongoing model download
func (m *DownloadManager) CancelModelDownload() error {
	m.modelMu.Lock()
	defer m.modelMu.Unlock()
	if m.modelCancelCtx != nil {
		m.modelCancelCtx()
		m.modelCancelCtx = nil
		m.modelStatus.Downloading = false
		m.modelStatus.Error = "download cancelled"
		return nil
	}
	return fmt.Errorf("no download in progress")
}

// StartGPUDownload starts downloading the wgpu-native library in the background
func (m *DownloadManager) StartGPUDownload() error {
	m.gpuMu.Lock()
	if m.gpuStatus.Downloading {
		m.gpuMu.Unlock()
		return fmt.Errorf("gpu library download is already in progress")
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.gpuCancelCtx = cancel
	m.gpuStatus = DownloadProgressInfo{
		Downloading:     true,
		Target:          "wgpu-native",
		Downloaded:      0,
		Total:           0,
		Percent:         0,
		DownloadedHuman: "0 B",
		TotalHuman:      "0 B",
	}
	m.gpuMu.Unlock()

	go func() {
		defer func() {
			m.gpuMu.Lock()
			m.gpuCancelCtx = nil
			m.gpuStatus.Downloading = false
			m.gpuMu.Unlock()
		}()

		progress := func(downloaded, total int64) {
			pct := 0
			if total > 0 {
				pct = int(float64(downloaded) / float64(total) * 100)
			}
			m.gpuMu.Lock()
			m.gpuStatus.Downloaded = downloaded
			m.gpuStatus.Total = total
			m.gpuStatus.Percent = pct
			m.gpuStatus.DownloadedHuman = humanize.Bytes(uint64(downloaded))
			m.gpuStatus.TotalHuman = humanize.Bytes(uint64(total))
			m.gpuMu.Unlock()
		}

		path, err := downloadWGPULib(ctx, m.LibDir(), progress)
		m.gpuMu.Lock()
		if err != nil && err != context.Canceled {
			m.gpuStatus.Error = err.Error()
		} else if err == nil {
			_ = os.Setenv("TENSAI_WGPU_LIB", path)
			tensai.SetCustomLibDir(m.LibDir())
			tensai.ResetGPUDevice()
			m.gpuStatus.Error = ""
		}
		m.gpuMu.Unlock()
	}()

	return nil
}

// CancelGPUDownload cancels ongoing GPU library download
func (m *DownloadManager) CancelGPUDownload() error {
	m.gpuMu.Lock()
	defer m.gpuMu.Unlock()
	if m.gpuCancelCtx != nil {
		m.gpuCancelCtx()
		m.gpuCancelCtx = nil
		m.gpuStatus.Downloading = false
		m.gpuStatus.Error = "download cancelled"
		return nil
	}
	return fmt.Errorf("no download in progress")
}

// GetStatus returns the current status of model and GPU downloads
func (m *DownloadManager) GetStatus() AIDownloadStatus {
	m.modelMu.Lock()
	mStatus := m.modelStatus
	m.modelMu.Unlock()

	m.gpuMu.Lock()
	gStatus := m.gpuStatus
	m.gpuMu.Unlock()

	return AIDownloadStatus{
		Model: mStatus,
		GPU:   gStatus,
	}
}

// Internal helper for downloading model
func downloadModelFile(ctx context.Context, modelDir, target string, progress DownloadProgress) (string, error) {
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return "", err
	}

	url := target
	var filename string

	if presetURL, ok := PresetModels[strings.ToLower(target)]; ok {
		url = presetURL
		filename = filepath.Base(presetURL)
	} else if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		url = target
		filename = filepath.Base(target)
		if idx := strings.Index(filename, "?"); idx != -1 {
			filename = filename[:idx]
		}
	} else if strings.Contains(target, "/") {
		parts := strings.Split(target, "/")
		if len(parts) >= 3 && strings.HasSuffix(parts[len(parts)-1], ".gguf") {
			repo := strings.Join(parts[:len(parts)-1], "/")
			file := parts[len(parts)-1]
			url = fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", repo, file)
			filename = file
		} else {
			repo := target
			file := strings.ToLower(parts[len(parts)-1]) + "-q8_0.gguf"
			url = fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", repo, file)
			filename = file
		}
	} else {
		var presetNames []string
		for k := range PresetModels {
			presetNames = append(presetNames, k)
		}
		return "", fmt.Errorf("unknown model preset or invalid URL: %s. Available presets: %s",
			target, strings.Join(presetNames, ", "))
	}

	if filename == "" {
		filename = "model.gguf"
	}
	destPath := filepath.Join(modelDir, filename)
	tmpPath := destPath + ".tmp"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download model: HTTP %s (%s)", resp.Status, url)
	}

	out, err := os.Create(tmpPath)
	if err != nil {
		return "", err
	}
	defer func() {
		out.Close()
		_ = os.Remove(tmpPath)
	}()

	totalSize := resp.ContentLength
	var downloaded int64

	buf := make([]byte, 64*1024)
	lastUpdate := time.Now()
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := out.Write(buf[:n]); writeErr != nil {
				return "", writeErr
			}
			downloaded += int64(n)
			if progress != nil && time.Since(lastUpdate) > 100*time.Millisecond {
				progress(downloaded, totalSize)
				lastUpdate = time.Now()
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return "", readErr
		}
	}

	if progress != nil {
		progress(downloaded, totalSize)
	}

	out.Close()
	if err := os.Rename(tmpPath, destPath); err != nil {
		return "", err
	}

	return destPath, nil
}

// GetWGPUAssetInfo returns the download URL and expected library filename for current OS/Arch
func GetWGPUAssetInfo() (string, string, error) {
	baseURL := fmt.Sprintf("https://github.com/gfx-rs/wgpu-native/releases/download/%s", WGPUVersion)

	var zipName string
	var libFileName string

	switch runtime.GOOS {
	case "darwin":
		libFileName = "libwgpu_native.dylib"
		switch runtime.GOARCH {
		case "arm64":
			zipName = "wgpu-macos-aarch64-release.zip"
		case "amd64":
			zipName = "wgpu-macos-x86_64-release.zip"
		default:
			return "", "", fmt.Errorf("unsupported macOS architecture: %s", runtime.GOARCH)
		}
	case "linux":
		libFileName = "libwgpu_native.so"
		switch runtime.GOARCH {
		case "amd64":
			zipName = "wgpu-linux-x86_64-release.zip"
		case "arm64":
			zipName = "wgpu-linux-aarch64-release.zip"
		default:
			return "", "", fmt.Errorf("unsupported Linux architecture: %s", runtime.GOARCH)
		}
	case "windows":
		libFileName = "wgpu_native.dll"
		switch runtime.GOARCH {
		case "amd64":
			zipName = "wgpu-windows-x86_64-msvc-release.zip"
		default:
			return "", "", fmt.Errorf("unsupported Windows architecture: %s", runtime.GOARCH)
		}
	default:
		return "", "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	url := fmt.Sprintf("%s/%s", baseURL, zipName)
	return url, libFileName, nil
}

func downloadWGPULib(ctx context.Context, libDir string, progress DownloadProgress) (string, error) {
	if err := os.MkdirAll(libDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create library directory: %w", err)
	}

	downloadURL, libFileName, err := GetWGPUAssetInfo()
	if err != nil {
		return "", err
	}

	destPath := filepath.Join(libDir, libFileName)

	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "twsnmpneo-downloader/1.0")

	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with HTTP status: %s", resp.Status)
	}

	totalSize := resp.ContentLength
	var downloaded int64
	var zipBuf bytes.Buffer

	buf := make([]byte, 32*1024)
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		n, err := resp.Body.Read(buf)
		if n > 0 {
			zipBuf.Write(buf[:n])
			downloaded += int64(n)
			if progress != nil {
				progress(downloaded, totalSize)
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf("error reading response body: %w", err)
		}
	}

	reader, err := zip.NewReader(bytes.NewReader(zipBuf.Bytes()), int64(zipBuf.Len()))
	if err != nil {
		return "", fmt.Errorf("failed to read zip archive: %w", err)
	}

	var foundFile *zip.File
	for _, f := range reader.File {
		base := filepath.Base(f.Name)
		if strings.EqualFold(base, libFileName) {
			foundFile = f
			break
		}
	}

	if foundFile == nil {
		return "", fmt.Errorf("could not find %s inside the downloaded archive", libFileName)
	}

	rc, err := foundFile.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open file inside zip: %w", err)
	}
	defer rc.Close()

	outFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create destination file: %w", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, rc); err != nil {
		return "", fmt.Errorf("failed to write library file: %w", err)
	}

	return destPath, nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
