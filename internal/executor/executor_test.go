package executor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"sentipulseexam/internal/model"
	"sentipulseexam/pkg/protocol"
)

func TestExecuteSuccess(t *testing.T) {
	workDir := mustWorkDir(t)
	descriptor := model.PluginDescriptor{
		Manifest: protocol.PluginManifest{
			Name:      "success",
			Version:   "1.0.0",
			Command:   []string{"go", "run", "./testdata/success"},
			TimeoutMS: 15000,
		},
		WorkDir: workDir,
	}

	result := Execute(context.Background(), descriptor, protocol.ExecuteRequest{
		RequestID: "req-success",
		Data: map[string]interface{}{
			"text": "hello",
		},
	})

	if result.Status != model.ExecutionStatusSuccess {
		t.Fatalf("expected success, got %s (%s)", result.Status, result.Error)
	}
	if result.Result["echo"] != "hello" {
		t.Fatalf("expected echoed text, got %#v", result.Result["echo"])
	}
}

func TestExecuteFailure(t *testing.T) {
	workDir := mustWorkDir(t)
	descriptor := model.PluginDescriptor{
		Manifest: protocol.PluginManifest{
			Name:      "failure",
			Version:   "1.0.0",
			Command:   []string{"go", "run", "./testdata/failure"},
			TimeoutMS: 15000,
		},
		WorkDir: workDir,
	}

	result := Execute(context.Background(), descriptor, protocol.ExecuteRequest{
		RequestID: "req-failure",
		Data: map[string]interface{}{
			"text": "hello",
		},
	})

	if result.Status != model.ExecutionStatusFailed {
		t.Fatalf("expected failed status, got %s", result.Status)
	}
	if result.Error == "" {
		t.Fatalf("expected error message for failure")
	}
}

func TestExecuteTimeout(t *testing.T) {
	workDir := mustWorkDir(t)
	descriptor := model.PluginDescriptor{
		Manifest: protocol.PluginManifest{
			Name:      "timeout",
			Version:   "1.0.0",
			Command:   []string{"go", "run", "./testdata/slow"},
			TimeoutMS: 50,
		},
		WorkDir: workDir,
	}

	result := Execute(context.Background(), descriptor, protocol.ExecuteRequest{
		RequestID: "req-timeout",
		Data: map[string]interface{}{
			"text": "hello",
		},
	})

	if result.Status != model.ExecutionStatusTimeout {
		t.Fatalf("expected timeout status, got %s (%s)", result.Status, result.Error)
	}
}

func mustWorkDir(t *testing.T) string {
	t.Helper()

	workDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	return filepath.Clean(workDir)
}
