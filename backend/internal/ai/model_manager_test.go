package ai

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadManagerDatadir(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "twsnmpneo_ai_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mgr := GetDownloadManager(tempDir)
	if mgr.ModelDir() != filepath.Join(tempDir, "models") {
		t.Errorf("expected modelDir %s, got %s", filepath.Join(tempDir, "models"), mgr.ModelDir())
	}
	if mgr.LibDir() != filepath.Join(tempDir, "lib") {
		t.Errorf("expected libDir %s, got %s", filepath.Join(tempDir, "lib"), mgr.LibDir())
	}

	// Test ListModels on empty directory
	models, err := mgr.ListModels()
	if err != nil {
		t.Fatalf("failed to list models: %v", err)
	}
	if len(models) != 0 {
		t.Errorf("expected 0 models, got %d", len(models))
	}

	// Create a dummy model file
	dummyModel := filepath.Join(mgr.ModelDir(), "qwen2.5-0.5b-instruct-q8_0.gguf")
	if err := os.WriteFile(dummyModel, []byte("GGUF_TEST_HEADER"), 0644); err != nil {
		t.Fatalf("failed to write dummy model: %v", err)
	}

	models, err = mgr.ListModels()
	if err != nil {
		t.Fatalf("failed to list models: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}
	if models[0].Name != "qwen2.5-0.5b-instruct-q8_0.gguf" {
		t.Errorf("unexpected model name: %s", models[0].Name)
	}

	// Test FindModel by preset name
	foundPath, err := mgr.FindModel("qwen2.5-0.5b")
	if err != nil {
		t.Fatalf("failed to find model by preset name: %v", err)
	}
	if foundPath != dummyModel {
		t.Errorf("expected %s, got %s", dummyModel, foundPath)
	}

	// Test DeleteModel
	if err := mgr.DeleteModel("qwen2.5-0.5b"); err != nil {
		t.Fatalf("failed to delete model: %v", err)
	}

	models, err = mgr.ListModels()
	if err != nil {
		t.Fatalf("failed to list models: %v", err)
	}
	if len(models) != 0 {
		t.Errorf("expected 0 models after deletion, got %d", len(models))
	}

	// Test Hardware Status
	status := mgr.GetHardwareStatus()
	if status.ModelDir != mgr.ModelDir() {
		t.Errorf("expected status modelDir %s, got %s", mgr.ModelDir(), status.ModelDir)
	}
	if status.LibDir != mgr.LibDir() {
		t.Errorf("expected status libDir %s, got %s", mgr.LibDir(), status.LibDir)
	}
}
