package model

import "sentipulseexam/pkg/protocol"

type PluginStatus string

const (
	PluginStatusEnabled  PluginStatus = "enabled"
	PluginStatusDisabled PluginStatus = "disabled"
	PluginStatusBlocked  PluginStatus = "blocked"
	PluginStatusError    PluginStatus = "error"
)

type ExecutionStatus string

const (
	ExecutionStatusSuccess ExecutionStatus = "success"
	ExecutionStatusFailed  ExecutionStatus = "failed"
	ExecutionStatusSkipped ExecutionStatus = "skipped"
	ExecutionStatusTimeout ExecutionStatus = "timeout"
)

type PluginDescriptor struct {
	Manifest     protocol.PluginManifest `json:"manifest"`
	Directory    string                  `json:"directory"`
	ManifestPath string                  `json:"manifest_path"`
	WorkDir      string                  `json:"work_dir"`
	Status       PluginStatus            `json:"status"`
	StatusReason string                  `json:"status_reason,omitempty"`
	LoadError    string                  `json:"load_error,omitempty"`
}

func (d PluginDescriptor) DisplayName() string {
	if d.Manifest.Name != "" {
		return d.Manifest.Name
	}
	return d.Directory
}

type PluginExecution struct {
	Name         string                 `json:"name"`
	Version      string                 `json:"version"`
	Status       ExecutionStatus        `json:"status"`
	DurationMS   int64                  `json:"duration_ms"`
	Dependencies []string               `json:"dependencies,omitempty"`
	Result       map[string]interface{} `json:"result,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Warnings     []string               `json:"warnings,omitempty"`
}

type ExecutionSummary struct {
	RequestID string            `json:"request_id"`
	StartedAt string            `json:"started_at"`
	Parallel  bool              `json:"parallel"`
	Results   []PluginExecution `json:"results"`
}
