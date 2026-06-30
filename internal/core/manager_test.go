package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"sentipulseexam/internal/model"
)

func TestManagerExecuteWithSamplePlugins(t *testing.T) {
	pluginsDir := copySamplePlugins(t)
	manager := NewManager(pluginsDir)

	if err := manager.Load(); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	summary, err := manager.Execute(context.Background(), sampleInput(), false, "req-sample")
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if len(summary.Results) != 2 {
		t.Fatalf("expected 2 enabled plugin results, got %d", len(summary.Results))
	}

	byName := resultsByName(summary.Results)
	for _, name := range []string{"wordcount", "insight-summary"} {
		result, ok := byName[name]
		if !ok {
			t.Fatalf("expected result for plugin %q", name)
		}
		if result.Status != model.ExecutionStatusSuccess {
			t.Fatalf("expected success result for %q, got %s (%s)", name, result.Status, result.Error)
		}
	}

	insight := byName["insight-summary"]
	if len(insight.Dependencies) != 1 {
		t.Fatalf("expected insight-summary to have 1 active dependency, got %d", len(insight.Dependencies))
	}
	if _, exists := insight.Result["summary"]; !exists {
		t.Fatalf("expected insight-summary to produce summary output")
	}
}

func TestManagerEnableAndDisable(t *testing.T) {
	pluginsDir := copySamplePlugins(t)
	manager := NewManager(pluginsDir)

	if err := manager.Load(); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if err := manager.Enable("keyword-tagger"); err != nil {
		t.Fatalf("Enable returned error: %v", err)
	}

	summary, err := manager.Execute(context.Background(), sampleInput(), false, "req-enable")
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if len(summary.Results) != 3 {
		t.Fatalf("expected 3 enabled plugin results after enabling, got %d", len(summary.Results))
	}

	keywordResult := resultsByName(summary.Results)["keyword-tagger"]
	if keywordResult.Status != model.ExecutionStatusSuccess {
		t.Fatalf("expected keyword-tagger success, got %s (%s)", keywordResult.Status, keywordResult.Error)
	}

	insightResult := resultsByName(summary.Results)["insight-summary"]
	if len(insightResult.Dependencies) != 2 {
		t.Fatalf("expected insight-summary to consume both active dependencies after enabling keyword-tagger, got %d", len(insightResult.Dependencies))
	}

	if err := manager.Disable("keyword-tagger"); err != nil {
		t.Fatalf("Disable returned error: %v", err)
	}

	descriptors := manager.List()
	for _, descriptor := range descriptors {
		if descriptor.Manifest.Name == "keyword-tagger" && descriptor.Status != model.PluginStatusDisabled {
			t.Fatalf("expected keyword-tagger to be disabled, got %s", descriptor.Status)
		}
	}
}

func TestManagerSkipsDependentPluginWhenDependencyFails(t *testing.T) {
	workspace := createWorkspace(t)
	writePlugin(t, workspace, "broken", `{
  "name": "broken",
  "version": "1.0.0",
  "runtime": "go",
  "enabled": true,
  "command": ["go", "run", "."],
  "work_dir": ".",
  "timeout_ms": 15000,
  "on_failure": "disable"
}`, `package main

import (
	"errors"

	"sentipulseexam/pkg/sdk"
)

func main() {
	sdk.Run("broken", "1.0.0", func(data map[string]interface{}) (map[string]interface{}, error) {
		return nil, errors.New("forced failure")
	})
}
`)

	writePlugin(t, workspace, "downstream", `{
  "name": "downstream",
  "version": "1.0.0",
  "runtime": "go",
  "enabled": true,
  "command": ["go", "run", "."],
  "work_dir": ".",
  "timeout_ms": 15000,
  "dependencies": [
    {
      "name": "broken",
      "version": ">=1.0.0"
    }
  ]
}`, `package main

import "sentipulseexam/pkg/sdk"

func main() {
	sdk.Run("downstream", "1.0.0", func(data map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{"ok": true}, nil
	})
}
`)

	manager := NewManager(filepath.Join(workspace, "plugins"))
	if err := manager.Load(); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	summary, err := manager.Execute(context.Background(), map[string]interface{}{"text": "hello"}, false, "req-failure")
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	byName := resultsByName(summary.Results)
	if byName["broken"].Status != model.ExecutionStatusFailed {
		t.Fatalf("expected broken plugin to fail, got %s", byName["broken"].Status)
	}
	if byName["downstream"].Status != model.ExecutionStatusSkipped {
		t.Fatalf("expected dependent plugin to be skipped, got %s", byName["downstream"].Status)
	}

	manager = NewManager(filepath.Join(workspace, "plugins"))
	if err := manager.Load(); err != nil {
		t.Fatalf("Load returned error after auto-disable: %v", err)
	}

	descriptors := manager.List()
	for _, descriptor := range descriptors {
		if descriptor.Manifest.Name == "broken" && descriptor.Status != model.PluginStatusDisabled {
			t.Fatalf("expected broken plugin to be auto-disabled, got %s", descriptor.Status)
		}
	}
}

func resultsByName(results []model.PluginExecution) map[string]model.PluginExecution {
	index := make(map[string]model.PluginExecution, len(results))
	for _, result := range results {
		index[result.Name] = result
	}
	return index
}

func copySamplePlugins(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("Abs failed: %v", err)
	}

	workspace := createWorkspace(t)
	copyDir(t, filepath.Join(root, "plugins"), filepath.Join(workspace, "plugins"))
	return filepath.Join(workspace, "plugins")
}

func createWorkspace(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("Abs failed: %v", err)
	}

	workspace := t.TempDir()
	copyFile(t, filepath.Join(root, "go.mod"), filepath.Join(workspace, "go.mod"))
	copyDir(t, filepath.Join(root, "pkg"), filepath.Join(workspace, "pkg"))
	return workspace
}

func writePlugin(t *testing.T, workspace, name, manifest, source string) {
	t.Helper()

	pluginDir := filepath.Join(workspace, "plugins", name)
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile plugin.json failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("WriteFile main.go failed: %v", err)
	}
}

func copyDir(t *testing.T, source, destination string) {
	t.Helper()

	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(source, entry.Name())
		dstPath := filepath.Join(destination, entry.Name())

		if entry.IsDir() {
			copyDir(t, srcPath, dstPath)
			continue
		}

		data, err := os.ReadFile(srcPath)
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}
		if err := os.WriteFile(dstPath, data, 0o644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
	}
}

func copyFile(t *testing.T, source, destination string) {
	t.Helper()

	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(destination, data, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
}

func sampleInput() map[string]interface{} {
	return map[string]interface{}{
		"text": "OpenAI builds reliable tools. The experience feels great overall, although a few bugs still show up.",
		"keywords": []interface{}{
			"OpenAI",
			"reliable",
			"bugs",
		},
	}
}
