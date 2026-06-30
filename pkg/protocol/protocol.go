package protocol

type PluginDependency struct {
	Name           string `json:"name"`
	Version        string `json:"version,omitempty"`
	Optional       bool   `json:"optional,omitempty"`
	RequireSuccess *bool  `json:"require_success,omitempty"`
}

func (d PluginDependency) MustSucceed() bool {
	return d.RequireSuccess == nil || *d.RequireSuccess
}

type PluginManifest struct {
	Name                string             `json:"name"`
	Version             string             `json:"version"`
	Description         string             `json:"description,omitempty"`
	Runtime             string             `json:"runtime,omitempty"`
	Tags                []string           `json:"tags,omitempty"`
	Enabled             bool               `json:"enabled"`
	Command             []string           `json:"command"`
	WorkDir             string             `json:"work_dir,omitempty"`
	TimeoutMS           int                `json:"timeout_ms,omitempty"`
	Dependencies        []PluginDependency `json:"dependencies,omitempty"`
	OnFailure           string             `json:"on_failure,omitempty"`
	OnDependencyFailure string             `json:"on_dependency_failure,omitempty"`
}

type DependencyExecution struct {
	Status  string                 `json:"status"`
	Version string                 `json:"version,omitempty"`
	Result  map[string]interface{} `json:"result,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

type ExecuteContext struct {
	DependencyResults map[string]DependencyExecution `json:"dependency_results,omitempty"`
}

type ExecuteRequest struct {
	RequestID string                 `json:"request_id"`
	Data      map[string]interface{} `json:"data"`
	Context   ExecuteContext         `json:"context,omitempty"`
}

type ExecuteResponse struct {
	PluginName string                 `json:"plugin_name"`
	Version    string                 `json:"version"`
	Success    bool                   `json:"success"`
	Result     map[string]interface{} `json:"result,omitempty"`
	Error      string                 `json:"error,omitempty"`
}
