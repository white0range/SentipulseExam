package registry

import (
	"fmt"

	"sentipulseexam/internal/model"
)

type Registry struct {
	plugins []model.PluginDescriptor
	index   map[string]int
}

func New() *Registry {
	return &Registry{
		plugins: make([]model.PluginDescriptor, 0),
		index:   make(map[string]int),
	}
}

func (r *Registry) Add(descriptor model.PluginDescriptor) error {
	if descriptor.Manifest.Name != "" {
		if _, exists := r.index[descriptor.Manifest.Name]; exists {
			return fmt.Errorf("duplicate plugin name %q", descriptor.Manifest.Name)
		}
		r.index[descriptor.Manifest.Name] = len(r.plugins)
	}
	r.plugins = append(r.plugins, descriptor)
	return nil
}

func (r *Registry) List() []model.PluginDescriptor {
	out := make([]model.PluginDescriptor, len(r.plugins))
	copy(out, r.plugins)
	return out
}

func (r *Registry) Find(name string) (model.PluginDescriptor, bool) {
	position, ok := r.index[name]
	if !ok {
		return model.PluginDescriptor{}, false
	}
	return r.plugins[position], true
}

func (r *Registry) Enabled() []model.PluginDescriptor {
	enabled := make([]model.PluginDescriptor, 0)
	for _, plugin := range r.plugins {
		if plugin.Status == model.PluginStatusEnabled {
			enabled = append(enabled, plugin)
		}
	}
	return enabled
}
