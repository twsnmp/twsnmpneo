package tensai

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	tensaimod "github.com/mattn/tensai"
	"github.com/mattn/tensai/gpu"
	"golang.org/x/sys/cpu"
)

// AccelerationType represents the hardware backend in use
type AccelerationType string

const (
	AccelGPU  AccelerationType = "GPU"
	AccelSIMD AccelerationType = "SIMD (AVX2)"
	AccelCPU  AccelerationType = "CPU (Portable)"
)

var (
	libDirMu      sync.RWMutex
	customLibDir  string
	gpuDevice     *gpu.Device
	gpuInitErr    error
	gpuInitOnce   sync.Once
)

// SetCustomLibDir sets the datastore-specific library directory where wgpu-native library is located.
func SetCustomLibDir(dir string) {
	libDirMu.Lock()
	customLibDir = dir
	libDirMu.Unlock()
}

// GetCustomLibDir returns the current custom library directory.
func GetCustomLibDir() string {
	libDirMu.RLock()
	defer libDirMu.RUnlock()
	return customLibDir
}

// InitWGPULibrary searches for the wgpu-native library in the configured datastore lib dir and standard locations.
func InitWGPULibrary(libDirs ...string) {
	libName := "libwgpu_native.so"
	switch runtime.GOOS {
	case "darwin":
		libName = "libwgpu_native.dylib"
	case "windows":
		libName = "wgpu_native.dll"
	}

	candidates := make([]string, 0, 8)

	for _, d := range libDirs {
		if d != "" {
			candidates = append(candidates, filepath.Join(d, libName))
			if runtime.GOOS == "windows" {
				candidates = append(candidates, filepath.Join(d, "libwgpu_native.dll"))
			}
		}
	}

	if cDir := GetCustomLibDir(); cDir != "" {
		candidates = append(candidates, filepath.Join(cDir, libName))
		if runtime.GOOS == "windows" {
			candidates = append(candidates, filepath.Join(cDir, "libwgpu_native.dll"))
		}
	}

	// Next to executable
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), libName))
		if runtime.GOOS == "windows" {
			candidates = append(candidates, filepath.Join(filepath.Dir(exe), "libwgpu_native.dll"))
		}
	}

	// Current working directory
	candidates = append(candidates, libName)
	if runtime.GOOS == "windows" {
		candidates = append(candidates, "libwgpu_native.dll")
	}

	// macOS homebrew paths
	if runtime.GOOS == "darwin" {
		candidates = append(candidates, "/opt/homebrew/lib/"+libName, "/usr/local/lib/"+libName)
	}

	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			if abs, err := filepath.Abs(path); err == nil {
				_ = os.Setenv("TENSAI_WGPU_LIB", abs)
				return
			}
		}
	}
}

// GetGPUDevice tries to open and initialize the WebGPU device (cached).
// It also registers the GPU device as a global tensai accelerator.
func GetGPUDevice() (*gpu.Device, error) {
	InitWGPULibrary()
	gpuInitOnce.Do(func() {
		dev, err := gpu.Open(gpu.HighPerformance)
		if err != nil {
			// Fallback: try Default power preference
			dev, err = gpu.Open(gpu.Default)
		}
		if err == nil && dev != nil {
			gpuDevice = dev
			// Enable global tensai accelerator for matrix multiplications
			tensaimod.UseAccelerator(dev)
		} else {
			gpuInitErr = err
		}
	})
	return gpuDevice, gpuInitErr
}

// ResetGPUDevice allows re-initializing the GPU device after downloading library.
func ResetGPUDevice() {
	gpuInitOnce = sync.Once{}
	gpuDevice = nil
	gpuInitErr = nil
}

// HasAVX2 reports whether the current CPU supports AVX2 instructions
func HasAVX2() bool {
	return runtime.GOARCH == "amd64" && cpu.X86.HasAVX2
}

// DetectAcceleration returns the active acceleration type and details string.
func DetectAcceleration() (AccelerationType, string) {
	return DetectAccelerationWithOptions(false)
}

// DetectAccelerationWithOptions returns the active acceleration type with optional GPU bypass.
func DetectAccelerationWithOptions(noGPU bool) (AccelerationType, string) {
	if !noGPU {
		dev, err := GetGPUDevice()
		if err == nil && dev != nil {
			name := dev.Name()
			if name == "" {
				name = "WebGPU Adapter"
			}
			backend := gpu.Backend()
			if backend != "" {
				return AccelGPU, fmt.Sprintf("%s (%s)", name, backend)
			}
			return AccelGPU, name
		}
		if err != nil && strings.Contains(err.Error(), "built without wgpu support") {
			return AccelCPU, fmt.Sprintf("Pure Go (%s/%s) [binary built without -tags wgpu24]", runtime.GOOS, runtime.GOARCH)
		}
	}

	if HasAVX2() {
		return AccelSIMD, "AVX2 8-lane FMA Vectorized"
	}

	return AccelCPU, fmt.Sprintf("Pure Go (%s/%s)", runtime.GOOS, runtime.GOARCH)
}
