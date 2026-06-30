package loader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sentipulseexam/internal/model"
	"sentipulseexam/internal/registry"
	"sentipulseexam/internal/version"
	"sentipulseexam/pkg/protocol"
)

const (
	manifestFileName = "plugin.json"
	defaultTimeoutMS = 3000
)

func LoadPlugins(pluginsDir string) (*registry.Registry, error) {
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return nil, err
	}

	descriptors := make([]model.PluginDescriptor, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginDir := filepath.Join(pluginsDir, entry.Name())
		manifestPath := filepath.Join(pluginDir, manifestFileName)
		if _, err := os.Stat(manifestPath); err != nil {
			continue
		}

		descriptors = append(descriptors, loadPluginDescriptor(pluginDir, manifestPath))
	}

	descriptors = validateDependencies(descriptors)

	loaded := registry.New()
	for _, descriptor := range descriptors {
		if err := loaded.Add(descriptor); err != nil {
			descriptor.Status = model.PluginStatusError
			descriptor.LoadError = err.Error()
			descriptor.Manifest.Name = fmt.Sprintf("%s@%s", descriptor.DisplayName(), filepath.Base(descriptor.Directory))
			descriptor.StatusReason = descriptor.LoadError
			_ = loaded.Add(descriptor)
		}
	}
	return loaded, nil
}

func UpdatePluginEnabled(pluginsDir, pluginName string, enabled bool) error {
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginDir := filepath.Join(pluginsDir, entry.Name())
		manifestPath := filepath.Join(pluginDir, manifestFileName)
		if _, err := os.Stat(manifestPath); err != nil {
			continue
		}

		manifest, err := readManifest(manifestPath)
		if err != nil {
			continue
		}

		if manifest.Name != pluginName {
			continue
		}

		manifest.Enabled = enabled
		return writeManifest(manifestPath, manifest)
	}

	return fmt.Errorf("plugin %q not found", pluginName)
}

func loadPluginDescriptor(pluginDir, manifestPath string) model.PluginDescriptor {
	manifest, err := readManifest(manifestPath)
	if err != nil {
		return model.PluginDescriptor{
			Manifest: protocol.PluginManifest{
				Name: filepath.Base(pluginDir),
			},
			Directory:    pluginDir,
			ManifestPath: manifestPath,
			WorkDir:      pluginDir,
			Status:       model.PluginStatusError,
			StatusReason: err.Error(),
			LoadError:    err.Error(),
		}
	}

	descriptor := model.PluginDescriptor{
		Manifest:     manifest,
		Directory:    pluginDir,
		ManifestPath: manifestPath,
		WorkDir:      pluginDir,
	}

	if validationErr := normalizeManifest(&descriptor); validationErr != nil {
		descriptor.Status = model.PluginStatusError
		descriptor.StatusReason = validationErr.Error()
		descriptor.LoadError = validationErr.Error()
		return descriptor
	}

	if descriptor.Manifest.Enabled {
		descriptor.Status = model.PluginStatusEnabled
	} else {
		descriptor.Status = model.PluginStatusDisabled
	}
	descriptor.StatusReason = string(descriptor.Status)

	return descriptor
}

func normalizeManifest(descriptor *model.PluginDescriptor) error {
	if descriptor.Manifest.Name == "" {
		return fmt.Errorf("manifest is missing plugin name")
	}
	if descriptor.Manifest.Version == "" {
		return fmt.Errorf("plugin %q is missing version", descriptor.Manifest.Name)
	}
	if len(descriptor.Manifest.Command) == 0 || descriptor.Manifest.Command[0] == "" {
		return fmt.Errorf("plugin %q is missing command", descriptor.Manifest.Name)
	}
	if descriptor.Manifest.TimeoutMS <= 0 {
		descriptor.Manifest.TimeoutMS = defaultTimeoutMS
	}
	if descriptor.Manifest.Runtime == "" {
		descriptor.Manifest.Runtime = inferRuntime(descriptor.Manifest.Command)
	}
	if descriptor.Manifest.OnFailure == "" {
		descriptor.Manifest.OnFailure = "continue"
	}
	if descriptor.Manifest.OnDependencyFailure == "" {
		descriptor.Manifest.OnDependencyFailure = "skip"
	}
	if descriptor.Manifest.OnFailure != "continue" && descriptor.Manifest.OnFailure != "disable" {
		return fmt.Errorf("plugin %q has invalid on_failure policy %q", descriptor.Manifest.Name, descriptor.Manifest.OnFailure)
	}
	if descriptor.Manifest.OnDependencyFailure != "skip" && descriptor.Manifest.OnDependencyFailure != "continue" {
		return fmt.Errorf("plugin %q has invalid on_dependency_failure policy %q", descriptor.Manifest.Name, descriptor.Manifest.OnDependencyFailure)
	}

	workDir := descriptor.Manifest.WorkDir
	if workDir == "" {
		workDir = "."
	}
	if !filepath.IsAbs(workDir) {
		workDir = filepath.Join(descriptor.Directory, workDir)
	}

	info, err := os.Stat(workDir)
	if err != nil {
		return fmt.Errorf("plugin %q work_dir invalid: %w", descriptor.Manifest.Name, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("plugin %q work_dir must be a directory", descriptor.Manifest.Name)
	}

	descriptor.WorkDir = workDir
	return nil
}

func validateDependencies(descriptors []model.PluginDescriptor) []model.PluginDescriptor {
	index := make(map[string]int, len(descriptors))
	for i, descriptor := range descriptors {
		if descriptor.Manifest.Name != "" {
			index[descriptor.Manifest.Name] = i
		}
	}

	for i := range descriptors {
		descriptor := &descriptors[i]
		if descriptor.Status != model.PluginStatusEnabled {
			continue
		}

		reasons := make([]string, 0)
		for _, dependency := range descriptor.Manifest.Dependencies {
			if dependency.Name == "" {
				reasons = append(reasons, "dependency name must not be empty")
				continue
			}

			dependencyIndex, exists := index[dependency.Name]
			if !exists {
				if !dependency.Optional {
					reasons = append(reasons, fmt.Sprintf("missing dependency %q", dependency.Name))
				}
				continue
			}

			target := descriptors[dependencyIndex]
			if target.Status == model.PluginStatusError {
				if !dependency.Optional {
					reasons = append(reasons, fmt.Sprintf("dependency %q is invalid", dependency.Name))
				}
				continue
			}
			if !target.Manifest.Enabled && !dependency.Optional {
				reasons = append(reasons, fmt.Sprintf("dependency %q is disabled", dependency.Name))
			}
			if dependency.Version != "" {
				matched, err := version.Satisfies(target.Manifest.Version, dependency.Version)
				if err != nil {
					reasons = append(reasons, fmt.Sprintf("dependency %q has invalid version constraint %q", dependency.Name, dependency.Version))
					continue
				}
				if !matched && !dependency.Optional {
					reasons = append(reasons, fmt.Sprintf("dependency %q version %s does not satisfy %s", dependency.Name, target.Manifest.Version, dependency.Version))
				}
			}
		}

		if len(reasons) > 0 {
			descriptor.Status = model.PluginStatusBlocked
			descriptor.StatusReason = strings.Join(reasons, "; ")
		}
	}

	for _, cycle := range detectCycles(descriptors, index) {
		for _, pluginName := range cycle {
			descriptor := &descriptors[index[pluginName]]
			if descriptor.Status == model.PluginStatusEnabled {
				descriptor.Status = model.PluginStatusBlocked
				descriptor.StatusReason = fmt.Sprintf("cyclic dependency detected: %s", strings.Join(cycle, " -> "))
			}
		}
	}

	return descriptors
}

func detectCycles(descriptors []model.PluginDescriptor, index map[string]int) [][]string {
	const (
		stateNew = iota
		stateActive
		stateDone
	)

	state := make(map[string]int, len(index))
	stack := make([]string, 0)
	cycles := make([][]string, 0)
	seenCycles := make(map[string]struct{})

	var visit func(string)
	visit = func(name string) {
		state[name] = stateActive
		stack = append(stack, name)

		descriptor := descriptors[index[name]]
		for _, dependency := range descriptor.Manifest.Dependencies {
			dependencyIndex, exists := index[dependency.Name]
			if !exists {
				continue
			}

			target := descriptors[dependencyIndex]
			if target.Status != model.PluginStatusEnabled && target.Status != model.PluginStatusDisabled {
				continue
			}

			switch state[dependency.Name] {
			case stateNew:
				visit(dependency.Name)
			case stateActive:
				cycle := extractCycle(stack, dependency.Name)
				key := strings.Join(cycle, "|")
				if _, exists := seenCycles[key]; !exists {
					seenCycles[key] = struct{}{}
					cycles = append(cycles, cycle)
				}
			}
		}

		stack = stack[:len(stack)-1]
		state[name] = stateDone
	}

	names := make([]string, 0, len(index))
	for name := range index {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		if state[name] == stateNew {
			visit(name)
		}
	}

	return cycles
}

func extractCycle(stack []string, repeated string) []string {
	start := 0
	for i, item := range stack {
		if item == repeated {
			start = i
			break
		}
	}

	cycle := append([]string{}, stack[start:]...)
	cycle = append(cycle, repeated)
	return cycle
}

func inferRuntime(command []string) string {
	if len(command) == 0 {
		return "unknown"
	}

	switch filepath.Base(strings.ToLower(command[0])) {
	case "go", "go.exe":
		return "go"
	case "python", "python.exe", "python3", "python3.exe":
		return "python"
	case "node", "node.exe":
		return "node"
	default:
		return "command"
	}
}

func readManifest(path string) (protocol.PluginManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return protocol.PluginManifest{}, err
	}

	var manifest protocol.PluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return protocol.PluginManifest{}, err
	}
	return manifest, nil
}

func writeManifest(path string, manifest protocol.PluginManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
