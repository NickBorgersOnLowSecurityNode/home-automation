package plugin

import (
	"fmt"
	"log"
	"sort"
	"sync"
)

// Priority constants for plugin registration.
// Higher priority values override lower priority plugins with the same name.
const (
	// PriorityDefault is the default priority for plugins.
	// Public/reference implementations should use this priority.
	PriorityDefault = 0

	// PriorityOverride is used by private implementations to override
	// public plugins. Private plugins should use this priority to ensure
	// they take precedence over the default implementation.
	PriorityOverride = 100
)

// PluginInfo contains metadata about a registered plugin.
type PluginInfo struct {
	// Name is the unique identifier for the plugin.
	// Plugins with the same name will override based on priority.
	Name string

	// Description is a human-readable description of the plugin.
	Description string

	// Priority determines which plugin wins when multiple plugins
	// register with the same name. Higher priority wins.
	Priority int

	// Factory creates new instances of the plugin.
	Factory Factory

	// Order specifies the startup order. Lower values start first.
	// Plugins that depend on others should have higher order values.
	// Default is 50. State tracking should be 10, reset coordinator should be 90.
	Order int
}

// Registry manages plugin registration and instantiation.
// It supports priority-based override, allowing private implementations
// to replace public ones at compile time through import ordering.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]PluginInfo
	order   []string
}

// NewRegistry creates a new plugin registry.
func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]PluginInfo),
		order:   make([]string, 0),
	}
}

// Register adds a plugin to the registry.
// If a plugin with the same name already exists, the one with higher
// priority wins. If priorities are equal, the later registration wins.
func (r *Registry) Register(info PluginInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if info.Name == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}

	if info.Factory == nil {
		return fmt.Errorf("plugin %s: factory cannot be nil", info.Name)
	}

	// Set default order if not specified
	if info.Order == 0 {
		info.Order = 50
	}

	existing, exists := r.plugins[info.Name]
	if exists {
		if info.Priority < existing.Priority {
			log.Printf("Plugin %q registration skipped (priority %d < existing %d)",
				info.Name, info.Priority, existing.Priority)
			return nil
		}

		if info.Priority >= existing.Priority {
			log.Printf("Plugin %q being overridden (priority %d -> %d)",
				info.Name, existing.Priority, info.Priority)
		}
	}

	r.plugins[info.Name] = info

	if !exists {
		r.order = append(r.order, info.Name)
	}

	log.Printf("Plugin %q registered (priority %d, order %d): %s",
		info.Name, info.Priority, info.Order, info.Description)

	return nil
}

// Get returns the plugin info for a given name, or nil if not found.
func (r *Registry) Get(name string) *PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, ok := r.plugins[name]
	if !ok {
		return nil
	}
	return &info
}

// List returns all registered plugins sorted by their startup order.
func (r *Registry) List() []PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]PluginInfo, 0, len(r.plugins))
	for _, name := range r.order {
		result = append(result, r.plugins[name])
	}

	// Sort by order (lower first), then by name for stability
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Order != result[j].Order {
			return result[i].Order < result[j].Order
		}
		return result[i].Name < result[j].Name
	})

	return result
}

// CreateAll instantiates all registered plugins using the provided context.
// Plugins are created in order (by Order field), so dependencies should
// have lower Order values.
func (r *Registry) CreateAll(ctx *Context) ([]Plugin, error) {
	plugins := r.List()
	result := make([]Plugin, 0, len(plugins))

	for _, info := range plugins {
		plugin, err := info.Factory(ctx)
		if err != nil {
			// Clean up already-created plugins on error
			for i := len(result) - 1; i >= 0; i-- {
				result[i].Stop()
			}
			return nil, fmt.Errorf("failed to create plugin %s: %w", info.Name, err)
		}
		result = append(result, plugin)
	}

	return result, nil
}

// Names returns the names of all registered plugins.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]string, len(r.order))
	copy(result, r.order)
	return result
}

// Clear removes all registered plugins. Useful for testing.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.plugins = make(map[string]PluginInfo)
	r.order = make([]string, 0)
}

// Global registry instance
var globalRegistry = NewRegistry()

// Register adds a plugin to the global registry.
// This is typically called from init() functions in plugin packages.
func Register(info PluginInfo) error {
	return globalRegistry.Register(info)
}

// Get returns plugin info from the global registry.
func Get(name string) *PluginInfo {
	return globalRegistry.Get(name)
}

// List returns all plugins from the global registry.
func List() []PluginInfo {
	return globalRegistry.List()
}

// CreateAll creates all plugins from the global registry.
func CreateAll(ctx *Context) ([]Plugin, error) {
	return globalRegistry.CreateAll(ctx)
}

// Names returns all plugin names from the global registry.
func Names() []string {
	return globalRegistry.Names()
}

// ClearGlobal clears the global registry. Useful for testing.
func ClearGlobal() {
	globalRegistry.Clear()
}
