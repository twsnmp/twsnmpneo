package updater

import (
	"os"
	"path/filepath"
	"strings"
)

// EnvironmentType represents how twsnmpneo is executed and managed
type EnvironmentType string

const (
	EnvHomebrew  EnvironmentType = "homebrew"
	EnvCurl      EnvironmentType = "curl"
	EnvDebug     EnvironmentType = "debug"
	EnvContainer EnvironmentType = "container"
)

// EnvironmentInfo holds detected environment characteristics and update instructions
type EnvironmentInfo struct {
	Type          EnvironmentType `json:"type"`
	IsHomebrew    bool            `json:"is_homebrew"`
	IsDebug       bool            `json:"is_debug"`
	IsContainer   bool            `json:"is_container"`
	CanSelfUpdate bool            `json:"can_self_update"`
	UpdateGuide   string          `json:"update_guide"`
	Description   string          `json:"description"`
}

// IsHomebrew returns true if the current executable is located within a Homebrew or Linuxbrew hierarchy
func IsHomebrew() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	realExe, err := filepath.EvalSymlinks(exe)
	if err != nil {
		realExe = exe
	}

	cleanReal := filepath.ToSlash(strings.ToLower(realExe))
	cleanExe := filepath.ToSlash(strings.ToLower(exe))

	for _, p := range []string{cleanExe, cleanReal} {
		if strings.Contains(p, "/cellar/") ||
			strings.Contains(p, "/opt/homebrew/") ||
			strings.Contains(p, "/home/linuxbrew/") ||
			strings.Contains(p, "/.linuxbrew/") {
			return true
		}
	}

	if prefix := os.Getenv("HOMEBREW_PREFIX"); prefix != "" {
		cleanPrefix := filepath.ToSlash(strings.ToLower(prefix))
		if strings.HasPrefix(cleanReal, cleanPrefix) || strings.HasPrefix(cleanExe, cleanPrefix) {
			return true
		}
	}

	return false
}

// IsDebugEnv checks whether the process is running in debug or local dev build mode
func IsDebugEnv(version, commit string, debugFlag bool) bool {
	if debugFlag {
		return true
	}
	if os.Getenv("TWSNMP_DEBUG") == "true" || os.Getenv("DEBUG") == "1" {
		return true
	}
	if version == "dev" || version == "unknown" || commit == "dev" || commit == "" {
		return true
	}

	exe, err := os.Executable()
	if err == nil {
		clean := filepath.ToSlash(strings.ToLower(exe))
		// Ignore go test runner binaries
		if !strings.HasSuffix(clean, ".test") && !strings.Contains(clean, ".test") {
			if strings.Contains(clean, "/tmp/go-build") || strings.Contains(clean, "\\temp\\go-build") {
				return true
			}
		}
	}

	return false
}

// DetectEnvironment inspects the runtime environment and executable location
func DetectEnvironment(version, commit string, debugFlag bool) EnvironmentInfo {
	// 1. Check Debug mode first
	if IsDebugEnv(version, commit, debugFlag) {
		return EnvironmentInfo{
			Type:          EnvDebug,
			IsDebug:       true,
			CanSelfUpdate: false,
			UpdateGuide:   "デバッグモードで動作しています。セルフアップデートは無効です。",
			Description:   "Running in debug / development mode.",
		}
	}

	// 2. Check Container (Docker / Podman / Kubernetes)
	if IsContainer() {
		return EnvironmentInfo{
			Type:          EnvContainer,
			IsContainer:   true,
			CanSelfUpdate: false,
			UpdateGuide:   "docker compose pull && docker compose up -d",
			Description:   "Running inside a Docker / container environment.",
		}
	}

	// 3. Check Homebrew (macOS / Linuxbrew)
	if IsHomebrew() {
		return EnvironmentInfo{
			Type:          EnvHomebrew,
			IsHomebrew:    true,
			CanSelfUpdate: false,
			UpdateGuide:   "brew upgrade twsnmp/tap/twsnmpneo",
			Description:   "Installed via Homebrew. Please run 'brew upgrade twsnmp/tap/twsnmpneo'.",
		}
	}

	// 4. Standalone binary (curl installer / direct binary download)
	return EnvironmentInfo{
		Type:          EnvCurl,
		CanSelfUpdate: true,
		UpdateGuide:   "twsnmpneo update",
		Description:   "Installed as a standalone binary (curl / direct download).",
	}
}
