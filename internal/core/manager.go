package core

import (
	"context"
	"fmt"
	"time"

	"sentipulseexam/internal/executor"
	"sentipulseexam/internal/loader"
	"sentipulseexam/internal/model"
	"sentipulseexam/internal/registry"
	"sentipulseexam/pkg/protocol"
)

type Manager struct {
	pluginsDir string
	registry   *registry.Registry
}

func NewManager(pluginsDir string) *Manager {
	return &Manager{pluginsDir: pluginsDir}
}

func (m *Manager) Load() error {
	loaded, err := loader.LoadPlugins(m.pluginsDir)
	if err != nil {
		return err
	}
	m.registry = loaded
	return nil
}

func (m *Manager) List() []model.PluginDescriptor {
	if m.registry == nil {
		return nil
	}
	return m.registry.List()
}

func (m *Manager) Enable(name string) error {
	if err := loader.UpdatePluginEnabled(m.pluginsDir, name, true); err != nil {
		return err
	}
	return m.Load()
}

func (m *Manager) Disable(name string) error {
	if err := loader.UpdatePluginEnabled(m.pluginsDir, name, false); err != nil {
		return err
	}
	return m.Load()
}

func (m *Manager) Execute(ctx context.Context, data map[string]interface{}, parallel bool, requestID string) (model.ExecutionSummary, error) {
	if m.registry == nil {
		return model.ExecutionSummary{}, fmt.Errorf("plugins are not loaded")
	}

	enabled := m.registry.Enabled()
	plan, err := buildExecutionPlan(enabled)
	if err != nil {
		return model.ExecutionSummary{}, err
	}

	if requestID == "" {
		requestID = fmt.Sprintf("req-%d", time.Now().UnixNano())
	}

	summary := model.ExecutionSummary{
		RequestID: requestID,
		StartedAt: time.Now().UTC().Format(time.RFC3339),
		Parallel:  parallel,
		Results:   make([]model.PluginExecution, 0, len(enabled)),
	}

	baseRequest := protocol.ExecuteRequest{
		RequestID: requestID,
		Data:      data,
	}

	enabledIndex := make(map[string]model.PluginDescriptor, len(enabled))
	for _, descriptor := range enabled {
		enabledIndex[descriptor.Manifest.Name] = descriptor
	}

	resultIndex := make(map[string]model.PluginExecution, len(enabled))

	for _, level := range plan {
		if parallel {
			levelResults := m.executeParallelLevel(ctx, level, baseRequest, enabledIndex, resultIndex)
			for _, result := range levelResults {
				resultIndex[result.Name] = result
				summary.Results = append(summary.Results, result)
			}
			continue
		}

		for _, descriptor := range level {
			result := m.executeOne(ctx, descriptor, baseRequest, enabledIndex, resultIndex)
			resultIndex[result.Name] = result
			summary.Results = append(summary.Results, result)
		}
	}

	return summary, nil
}

func (m *Manager) executeParallelLevel(
	ctx context.Context,
	level []model.PluginDescriptor,
	baseRequest protocol.ExecuteRequest,
	enabledIndex map[string]model.PluginDescriptor,
	resultIndex map[string]model.PluginExecution,
) []model.PluginExecution {
	type indexedResult struct {
		index  int
		result model.PluginExecution
	}

	results := make(chan indexedResult, len(level))
	for index, descriptor := range level {
		go func(i int, plugin model.PluginDescriptor) {
			results <- indexedResult{
				index:  i,
				result: m.executeOne(ctx, plugin, baseRequest, enabledIndex, resultIndex),
			}
		}(index, descriptor)
	}

	levelResults := make([]model.PluginExecution, len(level))
	for range level {
		item := <-results
		levelResults[item.index] = item.result
	}
	close(results)
	return levelResults
}

func (m *Manager) executeOne(
	ctx context.Context,
	descriptor model.PluginDescriptor,
	baseRequest protocol.ExecuteRequest,
	enabledIndex map[string]model.PluginDescriptor,
	resultIndex map[string]model.PluginExecution,
) model.PluginExecution {
	dependencyNames := activeDependencyNames(descriptor, enabledIndex)
	request, warnings, skipped := buildRequestForPlugin(baseRequest, descriptor, dependencyNames, resultIndex)
	if skipped != nil {
		return *skipped
	}

	result := executor.Execute(ctx, descriptor, request)
	result.Dependencies = dependencyNames
	result.Warnings = append(result.Warnings, warnings...)

	if descriptor.Manifest.OnFailure == "disable" &&
		(result.Status == model.ExecutionStatusFailed || result.Status == model.ExecutionStatusTimeout) {
		if err := loader.UpdatePluginEnabled(m.pluginsDir, descriptor.Manifest.Name, false); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to auto-disable plugin: %v", err))
		} else {
			result.Warnings = append(result.Warnings, "plugin auto-disabled after failure")
		}
	}

	return result
}

func buildExecutionPlan(enabled []model.PluginDescriptor) ([][]model.PluginDescriptor, error) {
	enabledIndex := make(map[string]model.PluginDescriptor, len(enabled))
	indegree := make(map[string]int, len(enabled))
	dependents := make(map[string][]string, len(enabled))
	remaining := make(map[string]struct{}, len(enabled))

	for _, descriptor := range enabled {
		name := descriptor.Manifest.Name
		enabledIndex[name] = descriptor
		indegree[name] = 0
		remaining[name] = struct{}{}
	}

	for _, descriptor := range enabled {
		for _, dependencyName := range activeDependencyNames(descriptor, enabledIndex) {
			indegree[descriptor.Manifest.Name]++
			dependents[dependencyName] = append(dependents[dependencyName], descriptor.Manifest.Name)
		}
	}

	plan := make([][]model.PluginDescriptor, 0)
	processed := 0

	for processed < len(enabled) {
		level := make([]model.PluginDescriptor, 0)
		for _, descriptor := range enabled {
			name := descriptor.Manifest.Name
			if _, exists := remaining[name]; exists && indegree[name] == 0 {
				level = append(level, descriptor)
			}
		}

		if len(level) == 0 {
			return nil, fmt.Errorf("execution plan contains a cycle among enabled plugins")
		}

		plan = append(plan, level)
		for _, descriptor := range level {
			name := descriptor.Manifest.Name
			delete(remaining, name)
			processed++
			for _, dependent := range dependents[name] {
				indegree[dependent]--
			}
		}
	}

	return plan, nil
}

func buildRequestForPlugin(
	baseRequest protocol.ExecuteRequest,
	descriptor model.PluginDescriptor,
	dependencyNames []string,
	resultIndex map[string]model.PluginExecution,
) (protocol.ExecuteRequest, []string, *model.PluginExecution) {
	request := protocol.ExecuteRequest{
		RequestID: baseRequest.RequestID,
		Data:      baseRequest.Data,
		Context: protocol.ExecuteContext{
			DependencyResults: make(map[string]protocol.DependencyExecution),
		},
	}

	warnings := make([]string, 0)
	for _, dependency := range descriptor.Manifest.Dependencies {
		result, exists := resultIndex[dependency.Name]
		if !exists {
			continue
		}

		request.Context.DependencyResults[dependency.Name] = protocol.DependencyExecution{
			Status:  string(result.Status),
			Version: result.Version,
			Result:  result.Result,
			Error:   result.Error,
		}

		if !dependency.MustSucceed() || result.Status == model.ExecutionStatusSuccess {
			continue
		}

		if descriptor.Manifest.OnDependencyFailure == "continue" {
			warnings = append(warnings, fmt.Sprintf("dependency %q completed with status %s", dependency.Name, result.Status))
			continue
		}

		skipped := model.PluginExecution{
			Name:         descriptor.Manifest.Name,
			Version:      descriptor.Manifest.Version,
			Status:       model.ExecutionStatusSkipped,
			Dependencies: dependencyNames,
			Error:        fmt.Sprintf("skipped because dependency %q completed with status %s", dependency.Name, result.Status),
		}
		return request, warnings, &skipped
	}

	if len(request.Context.DependencyResults) == 0 {
		request.Context.DependencyResults = nil
	}

	return request, warnings, nil
}

func activeDependencyNames(descriptor model.PluginDescriptor, enabledIndex map[string]model.PluginDescriptor) []string {
	names := make([]string, 0, len(descriptor.Manifest.Dependencies))
	for _, dependency := range descriptor.Manifest.Dependencies {
		target, exists := enabledIndex[dependency.Name]
		if !exists || target.Status != model.PluginStatusEnabled {
			continue
		}
		names = append(names, dependency.Name)
	}
	return names
}
