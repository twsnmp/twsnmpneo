//go:build linux

package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinuxIsWindowsService(t *testing.T) {
	if IsWindowsService() {
		t.Errorf("expected IsWindowsService() to be false on Linux")
	}
}

func TestFormatServiceArgs(t *testing.T) {
	tmpDir := t.TempDir()
	relDir := "relative/dir"

	args := []string{
		"-port", "8080",
		"-datadir", relDir,
		"-datadir=" + relDir,
		"-debug",
	}

	formatted := formatServiceArgs(args)

	expectedAbs, _ := filepath.Abs(relDir)

	if formatted[1] != "8080" {
		t.Errorf("expected port 8080, got %s", formatted[1])
	}
	if formatted[3] != expectedAbs {
		t.Errorf("expected abs path %s, got %s", expectedAbs, formatted[3])
	}
	if formatted[4] != "-datadir="+expectedAbs {
		t.Errorf("expected abs path %s, got %s", "-datadir="+expectedAbs, formatted[4])
	}
	_ = tmpDir
}

func TestGenerateUnitContent(t *testing.T) {
	execPath := "/usr/local/bin/twsnmpneo"
	args := []string{"-port", "8080", "-datadir", "/var/lib/twsnmpneo", "-title", "My Test"}
	workDir := "/usr/local/bin"

	content := generateUnitContent(execPath, args, workDir)

	if !strings.Contains(content, "[Unit]") {
		t.Errorf("missing [Unit] section")
	}
	if !strings.Contains(content, "[Service]") {
		t.Errorf("missing [Service] section")
	}
	if !strings.Contains(content, "[Install]") {
		t.Errorf("missing [Install] section")
	}
	if !strings.Contains(content, "ExecStart=/usr/local/bin/twsnmpneo -port 8080 -datadir /var/lib/twsnmpneo -title \"My Test\"") {
		t.Errorf("ExecStart line mismatch: %s", content)
	}
	if !strings.Contains(content, "WorkingDirectory=/usr/local/bin") {
		t.Errorf("WorkingDirectory mismatch: %s", content)
	}
	if !strings.Contains(content, "LimitNOFILE=65536") {
		t.Errorf("missing LimitNOFILE")
	}
	if !strings.Contains(content, "Restart=always") {
		t.Errorf("missing Restart=always")
	}
}

func TestLinuxServiceLifecycleMocked(t *testing.T) {
	tmpUnitDir := t.TempDir()
	origUnitDir := systemdUnitDir
	systemdUnitDir = tmpUnitDir
	defer func() { systemdUnitDir = origUnitDir }()

	var executedCommands [][]string
	origRunCmd := runCmd
	runCmd = func(name string, args ...string) (string, error) {
		cmdLine := append([]string{name}, args...)
		executedCommands = append(executedCommands, cmdLine)

		if len(args) >= 2 && args[0] == "is-active" {
			return "active", nil
		}
		if len(args) >= 2 && args[0] == "is-enabled" {
			return "enabled", nil
		}
		return "", nil
	}
	defer func() { runCmd = origRunCmd }()

	// Status before install -> Not Installed
	st, err := Status()
	if err != nil {
		t.Fatalf("Status() unexpected error: %v", err)
	}
	if st != "Not Installed" {
		t.Errorf("expected Not Installed, got %s", st)
	}

	// Fake root for testing
	origGeteuid := os.Geteuid()
	_ = origGeteuid

	// Note: If running non-root in test environment, Install/Remove/Start/Stop check checkRoot().
	// We verify that non-root check triggers when euid != 0.
	if os.Geteuid() != 0 {
		if err := Install(InstallOptions{AutoStart: true}); err == nil {
			t.Errorf("expected root check failure on Install()")
		}
		if err := Remove(); err == nil {
			t.Errorf("expected root check failure on Remove()")
		}
		if err := Start(); err == nil {
			t.Errorf("expected root check failure on Start()")
		}
		if err := Stop(); err == nil {
			t.Errorf("expected root check failure on Stop()")
		}
	} else {
		// Running as root (or container): test full install & lifecycle
		opts := InstallOptions{
			AutoStart: true,
			Arguments: []string{"-port", "8080"},
		}
		if err := Install(opts); err != nil {
			t.Fatalf("Install() error: %v", err)
		}

		unitFile := filepath.Join(tmpUnitDir, ServiceName+".service")
		if _, err := os.Stat(unitFile); err != nil {
			t.Fatalf("expected unit file to exist: %v", err)
		}

		// Re-installing should fail
		if err := Install(opts); err == nil {
			t.Errorf("expected error installing already existing service")
		}

		st, err = Status()
		if err != nil {
			t.Fatalf("Status() error: %v", err)
		}
		if st != "Running (Automatic)" {
			t.Errorf("expected Running (Automatic), got %s", st)
		}

		if err := Start(); err != nil {
			t.Errorf("Start() error: %v", err)
		}
		if err := Stop(); err != nil {
			t.Errorf("Stop() error: %v", err)
		}
		if err := Remove(); err != nil {
			t.Errorf("Remove() error: %v", err)
		}

		if _, err := os.Stat(unitFile); !os.IsNotExist(err) {
			t.Errorf("expected unit file to be deleted")
		}
	}
}

type dummyHandler struct {
	started bool
	stopped bool
}

func (d *dummyHandler) Start() error {
	d.started = true
	return nil
}

func (d *dummyHandler) Stop() error {
	d.stopped = true
	return nil
}

func TestLinuxRun(t *testing.T) {
	handler := &dummyHandler{}
	if err := Run("test", handler); err != nil {
		t.Errorf("Run() error: %v", err)
	}
	if !handler.started {
		t.Errorf("expected handler.started to be true")
	}
}
