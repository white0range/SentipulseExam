package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"time"

	"sentipulseexam/internal/config"
	"sentipulseexam/internal/core"
	"sentipulseexam/internal/model"
	"sentipulseexam/pkg/protocol"
)

func Run(args []string) int {
	cfg, err := config.Parse(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, config.Usage())
		return 1
	}

	manager := core.NewManager(cfg.PluginsDir)
	if err := manager.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "load plugins: %v\n", err)
		return 1
	}

	switch cfg.Mode {
	case config.ModeList:
		printPluginList(manager.List())
		return 0
	case config.ModeEnable:
		if err := manager.Enable(cfg.PluginName); err != nil {
			fmt.Fprintf(os.Stderr, "enable plugin: %v\n", err)
			return 1
		}
		fmt.Printf("plugin %q enabled\n", cfg.PluginName)
		return 0
	case config.ModeDisable:
		if err := manager.Disable(cfg.PluginName); err != nil {
			fmt.Fprintf(os.Stderr, "disable plugin: %v\n", err)
			return 1
		}
		fmt.Printf("plugin %q disabled\n", cfg.PluginName)
		return 0
	case config.ModeRun:
		data, err := readInput(cfg.InputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read input: %v\n", err)
			return 1
		}

		summary, err := manager.Execute(context.Background(), data, cfg.Parallel, cfg.RequestID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "execute plugins: %v\n", err)
			return 1
		}

		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(summary); err != nil {
			fmt.Fprintf(os.Stderr, "encode summary: %v\n", err)
			return 1
		}
		return 0
	case config.ModeWatch:
		return runWatchMode(cfg, manager)
	default:
		fmt.Fprintf(os.Stderr, "unsupported mode %q\n", cfg.Mode)
		return 1
	}
}

func readInput(path string) (map[string]interface{}, error) {
	var reader io.Reader
	if path == "" {
		reader = os.Stdin
	} else {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		reader = file
	}

	decoder := json.NewDecoder(reader)
	decoder.UseNumber()

	var data map[string]interface{}
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}
	if data == nil {
		return nil, errors.New("input must be a JSON object")
	}
	return data, nil
}

func printPluginList(plugins []model.PluginDescriptor) {
	if len(plugins) == 0 {
		fmt.Println("no plugins found")
		return
	}

	for _, plugin := range plugins {
		fmt.Printf("- %s v%s [%s]\n", plugin.DisplayName(), plugin.Manifest.Version, plugin.Status)
		if plugin.Manifest.Description != "" {
			fmt.Printf("  description: %s\n", plugin.Manifest.Description)
		}
		fmt.Printf("  runtime: %s\n", plugin.Manifest.Runtime)
		if len(plugin.Manifest.Tags) > 0 {
			fmt.Printf("  tags: %s\n", strings.Join(plugin.Manifest.Tags, ", "))
		}
		if len(plugin.Manifest.Dependencies) > 0 {
			fmt.Printf("  dependencies: %s\n", formatDependencies(plugin.Manifest.Dependencies))
		}
		fmt.Printf("  policy: on_failure=%s, on_dependency_failure=%s\n", plugin.Manifest.OnFailure, plugin.Manifest.OnDependencyFailure)
		fmt.Printf("  directory: %s\n", plugin.Directory)
		if plugin.StatusReason != "" && plugin.StatusReason != string(plugin.Status) {
			fmt.Printf("  status_reason: %s\n", plugin.StatusReason)
		}
		if plugin.LoadError != "" {
			fmt.Printf("  error: %s\n", plugin.LoadError)
		}
	}
}

func runWatchMode(cfg config.Config, manager *core.Manager) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var data map[string]interface{}
	var err error
	if cfg.RunOnChange {
		data, err = readInput(cfg.InputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read input: %v\n", err)
			return 1
		}
	}

	last := snapshotPlugins(manager.List())
	fmt.Printf("watching %q every %s\n", cfg.PluginsDir, cfg.WatchInterval)
	printPluginList(manager.List())

	if cfg.RunOnChange {
		if err := printExecutionSummary(manager, data, cfg.Parallel, cfg.RequestID); err != nil {
			fmt.Fprintf(os.Stderr, "execute plugins: %v\n", err)
			return 1
		}
	}

	ticker := time.NewTicker(cfg.WatchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("watch stopped")
			return 0
		case <-ticker.C:
			if err := manager.Load(); err != nil {
				fmt.Fprintf(os.Stderr, "reload plugins: %v\n", err)
				continue
			}

			current := snapshotPlugins(manager.List())
			changes := diffSnapshots(last, current)
			if len(changes) == 0 {
				continue
			}

			fmt.Printf("[%s] plugin registry reloaded\n", time.Now().Format(time.RFC3339))
			for _, change := range changes {
				fmt.Printf("  - %s\n", change)
			}

			if cfg.RunOnChange {
				if err := printExecutionSummary(manager, data, cfg.Parallel, cfg.RequestID); err != nil {
					fmt.Fprintf(os.Stderr, "execute plugins: %v\n", err)
				}
			}

			last = current
		}
	}
}

func printExecutionSummary(manager *core.Manager, data map[string]interface{}, parallel bool, requestID string) error {
	summary, err := manager.Execute(context.Background(), data, parallel, requestID)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(summary)
}

type pluginSnapshot struct {
	Version      string
	Status       model.PluginStatus
	Runtime      string
	Enabled      bool
	StatusReason string
	ManifestPath string
}

func snapshotPlugins(plugins []model.PluginDescriptor) map[string]pluginSnapshot {
	snapshot := make(map[string]pluginSnapshot, len(plugins))
	for _, plugin := range plugins {
		snapshot[plugin.Manifest.Name] = pluginSnapshot{
			Version:      plugin.Manifest.Version,
			Status:       plugin.Status,
			Runtime:      plugin.Manifest.Runtime,
			Enabled:      plugin.Manifest.Enabled,
			StatusReason: plugin.StatusReason,
			ManifestPath: plugin.ManifestPath,
		}
	}
	return snapshot
}

func diffSnapshots(previous, current map[string]pluginSnapshot) []string {
	changes := make([]string, 0)
	seen := make(map[string]struct{}, len(previous)+len(current))

	for name, snapshot := range previous {
		seen[name] = struct{}{}
		currentSnapshot, exists := current[name]
		if !exists {
			changes = append(changes, fmt.Sprintf("removed plugin %s", name))
			continue
		}
		if snapshot != currentSnapshot {
			changes = append(changes, fmt.Sprintf(
				"updated plugin %s (status %s -> %s, version %s -> %s)",
				name,
				snapshot.Status,
				currentSnapshot.Status,
				snapshot.Version,
				currentSnapshot.Version,
			))
		}
	}

	for name := range current {
		if _, exists := seen[name]; exists {
			continue
		}
		changes = append(changes, fmt.Sprintf("added plugin %s", name))
	}

	sort.Strings(changes)
	return changes
}

func formatDependencies(dependencies []protocol.PluginDependency) string {
	parts := make([]string, 0, len(dependencies))
	for _, dependency := range dependencies {
		item := dependency.Name
		if dependency.Version != "" {
			item = fmt.Sprintf("%s(%s)", item, dependency.Version)
		}
		if dependency.Optional {
			item += "[optional]"
		}
		parts = append(parts, item)
	}
	return strings.Join(parts, ", ")
}
