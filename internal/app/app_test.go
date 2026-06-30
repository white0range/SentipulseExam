package app

import (
	"testing"

	"sentipulseexam/internal/model"
)

func TestDiffSnapshots(t *testing.T) {
	previous := map[string]pluginSnapshot{
		"alpha": {
			Version: "1.0.0",
			Status:  model.PluginStatusEnabled,
			Runtime: "go",
			Enabled: true,
		},
	}

	current := map[string]pluginSnapshot{
		"alpha": {
			Version: "1.1.0",
			Status:  model.PluginStatusBlocked,
			Runtime: "go",
			Enabled: true,
		},
		"beta": {
			Version: "1.0.0",
			Status:  model.PluginStatusEnabled,
			Runtime: "python",
			Enabled: true,
		},
	}

	changes := diffSnapshots(previous, current)
	if len(changes) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(changes))
	}
}
