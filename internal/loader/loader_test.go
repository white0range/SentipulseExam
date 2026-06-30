package loader

import (
	"os"
	"path/filepath"
	"testing"

	"sentipulseexam/internal/model"
)

func TestLoadPlugins(t *testing.T) {
	root := t.TempDir()

	mustWriteFile(t, filepath.Join(root, "alpha", "plugin.json"), `{
  "name": "alpha",
  "version": "1.0.0",
  "enabled": true,
  "command": ["go", "run", "."],
  "work_dir": "."
}`)

	mustWriteFile(t, filepath.Join(root, "beta", "plugin.json"), `{
  "name": "beta",
  "version": "1.0.0",
  "enabled": false,
  "command": ["go", "run", "."],
  "work_dir": "."
}`)

	mustWriteFile(t, filepath.Join(root, "broken", "plugin.json"), `{
  "name": "",
  "version": "1.0.0",
  "enabled": true,
  "command": ["go", "run", "."]
}`)

	registry, err := LoadPlugins(root)
	if err != nil {
		t.Fatalf("LoadPlugins returned error: %v", err)
	}

	plugins := registry.List()
	if len(plugins) != 3 {
		t.Fatalf("expected 3 plugins, got %d", len(plugins))
	}

	if got := len(registry.Enabled()); got != 1 {
		t.Fatalf("expected 1 enabled plugin, got %d", got)
	}
}

func TestUpdatePluginEnabled(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "alpha", "plugin.json"), `{
  "name": "alpha",
  "version": "1.0.0",
  "enabled": false,
  "command": ["go", "run", "."],
  "work_dir": "."
}`)

	if err := UpdatePluginEnabled(root, "alpha", true); err != nil {
		t.Fatalf("UpdatePluginEnabled returned error: %v", err)
	}

	registry, err := LoadPlugins(root)
	if err != nil {
		t.Fatalf("LoadPlugins returned error: %v", err)
	}

	if got := len(registry.Enabled()); got != 1 {
		t.Fatalf("expected 1 enabled plugin after update, got %d", got)
	}
}

func TestLoadPluginsBlocksInvalidDependencies(t *testing.T) {
	root := t.TempDir()

	mustWriteFile(t, filepath.Join(root, "base", "plugin.json"), `{
  "name": "base",
  "version": "1.0.0",
  "enabled": true,
  "command": ["go", "run", "."],
  "work_dir": "."
}`)

	mustWriteFile(t, filepath.Join(root, "dependent", "plugin.json"), `{
  "name": "dependent",
  "version": "1.0.0",
  "enabled": true,
  "command": ["go", "run", "."],
  "work_dir": ".",
  "dependencies": [
    {
      "name": "base",
      "version": ">=2.0.0"
    }
  ]
}`)

	registry, err := LoadPlugins(root)
	if err != nil {
		t.Fatalf("LoadPlugins returned error: %v", err)
	}

	plugins := registry.List()
	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(plugins))
	}

	for _, plugin := range plugins {
		if plugin.Manifest.Name == "dependent" && plugin.Status != model.PluginStatusBlocked {
			t.Fatalf("expected dependent plugin to be blocked, got %s", plugin.Status)
		}
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
}
