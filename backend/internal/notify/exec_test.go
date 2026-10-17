package notify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecNotifyCmdWithDir_RejectsPath(t *testing.T) {
	dir := t.TempDir()
	cases := []string{
		"/bin/echo $level",
		"./script.sh $level",
		"../script.sh $level",
		"sub/script.sh $level",
	}
	for _, tc := range cases {
		err := ExecNotifyCmdWithDir(dir, tc, 1)
		if err == nil {
			t.Errorf("expected error for command with path %q, got nil", tc)
		} else if !strings.Contains(err.Error(), "has path") {
			t.Errorf("expected 'has path' error for %q, got %v", tc, err)
		}
	}
}

func TestExecNotifyCmdWithDir_ExecutesInDataDirCmd(t *testing.T) {
	dir := t.TempDir()
	cmdDir := filepath.Join(dir, "cmd")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatalf("mkdir cmd: %v", err)
	}
	outFile := filepath.Join(dir, "out.txt")
	scriptPath := filepath.Join(cmdDir, "test_notify.sh")
	scriptContent := "#!/bin/sh\necho \"level=$1\" > \"" + outFile + "\"\n"
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	if err := ExecNotifyCmdWithDir(dir, "test_notify.sh $level", 2); err != nil {
		t.Fatalf("ExecNotifyCmdWithDir failed: %v", err)
	}

	b, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if strings.TrimSpace(string(b)) != "level=2" {
		t.Errorf("expected 'level=2', got %q", strings.TrimSpace(string(b)))
	}
}
