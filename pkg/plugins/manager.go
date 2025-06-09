package plugins

import (
	"context"
	"fmt"
	"plugin"
	"sync"
	"time"
)

// Manager manages loaded plugins
type Manager struct {
	mu         sync.RWMutex
	backends   map[string]BackendPlugin
	formatters map[string]FormatterPlugin
	filters    map[string]FilterPlugin
	loaded     map[string]Plugin
}

// NewManager creates a new plugin manager
func NewManager() *Manager {
	return &Manager{
		backends:   make(map[string]BackendPlugin),
		formatters: make(map[string]FormatterPlugin),
		filters:    make(map[string]FilterPlugin),
		loaded:     make(map[string]Plugin),
	}
}

// LoadPlugin loads a plugin from a shared library file
func (m *Manager) LoadPlugin(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Load the plugin
	p, err := plugin.Open(path)
	if err != nil {
		return fmt.Errorf("open plugin %s: %w", path, err)
	}

	// Look for the plugin entry point
	sym, err := p.Lookup("OmniPlugin")
	if err != nil {
		return fmt.Errorf("plugin %s missing OmniPlugin symbol: %w", path, err)
	}

	// Cast to plugin interface
	pluginInstance, ok := sym.(Plugin)
	if !ok {
		return fmt.Errorf("plugin %s OmniPlugin is not a Plugin interface", path)
	}

	// Check for duplicate names
	name := pluginInstance.Name()
	if _, exists := m.loaded[name]; exists {
		return fmt.Errorf("plugin %s already loaded", name)
	}

	// Register the plugin
	m.loaded[name] = pluginInstance

	// Register specific plugin types
	if backendPlugin, ok := pluginInstance.(BackendPlugin); ok {
		schemes := backendPlugin.SupportedSchemes()
		for _, scheme := range schemes {
			m.backends[scheme] = backendPlugin
		}
	}

	if formatterPlugin, ok := pluginInstance.(FormatterPlugin); ok {
		formatName := formatterPlugin.FormatName()
		m.formatters[formatName] = formatterPlugin
	}

	if filterPlugin, ok := pluginInstance.(FilterPlugin); ok {
		filterType := filterPlugin.FilterType()
		m.filters[filterType] = filterPlugin
	}

	return nil
}

// UnloadPlugin unloads a plugin by name
func (m *Manager) UnloadPlugin(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pluginInstance, exists := m.loaded[name]
	if !exists {
		return fmt.Errorf("plugin %s not loaded", name)
	}

	// Shutdown the plugin
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := pluginInstance.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown plugin %s: %w", name, err)
	}

	// Remove from registries
	if backendPlugin, ok := pluginInstance.(BackendPlugin); ok {
		schemes := backendPlugin.SupportedSchemes()
		for _, scheme := range schemes {
			delete(m.backends, scheme)
		}
	}

	if formatterPlugin, ok := pluginInstance.(FormatterPlugin); ok {
		formatName := formatterPlugin.FormatName()
		delete(m.formatters, formatName)
	}

	if filterPlugin, ok := pluginInstance.(FilterPlugin); ok {
		filterType := filterPlugin.FilterType()
		delete(m.filters, filterType)
	}

	delete(m.loaded, name)

	return nil
}

// GetBackendPlugin returns a backend plugin for the given scheme
func (m *Manager) GetBackendPlugin(scheme string) (BackendPlugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plugin, exists := m.backends[scheme]
	return plugin, exists
}

// GetFormatterPlugin returns a formatter plugin for the given format
func (m *Manager) GetFormatterPlugin(format string) (FormatterPlugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plugin, exists := m.formatters[format]
	return plugin, exists
}

// GetFilterPlugin returns a filter plugin for the given type
func (m *Manager) GetFilterPlugin(filterType string) (FilterPlugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plugin, exists := m.filters[filterType]
	return plugin, exists
}

// ListPlugins returns all loaded plugins
func (m *Manager) ListPlugins() []Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plugins := make([]Plugin, 0, len(m.loaded))
	for _, plugin := range m.loaded {
		plugins = append(plugins, plugin)
	}

	return plugins
}

// InitializePlugin initializes a plugin with configuration
func (m *Manager) InitializePlugin(name string, config map[string]interface{}) error {
	m.mu.RLock()
	pluginInstance, exists := m.loaded[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("plugin %s not loaded", name)
	}

	return pluginInstance.Initialize(config)
}

// GetPluginInfo returns information about loaded plugins
func (m *Manager) GetPluginInfo() []PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]PluginInfo, 0, len(m.loaded))

	for _, plugin := range m.loaded {
		info := PluginInfo{
			Name:    plugin.Name(),
			Version: plugin.Version(),
			Details: make(map[string]interface{}),
		}

		// Determine plugin type and add specific details
		if backendPlugin, ok := plugin.(BackendPlugin); ok {
			info.Type = "backend"
			info.Details["supported_schemes"] = backendPlugin.SupportedSchemes()
		} else if formatterPlugin, ok := plugin.(FormatterPlugin); ok {
			info.Type = "formatter"
			info.Details["format_name"] = formatterPlugin.FormatName()
		} else if filterPlugin, ok := plugin.(FilterPlugin); ok {
			info.Type = "filter"
			info.Details["filter_type"] = filterPlugin.FilterType()
		} else {
			info.Type = "unknown"
		}

		infos = append(infos, info)
	}

	return infos
}

// RegisterBackendPlugin registers a backend plugin directly (for built-in plugins)
func (m *Manager) RegisterBackendPlugin(plugin BackendPlugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := plugin.Name()
	if _, exists := m.loaded[name]; exists {
		return fmt.Errorf("plugin %s already registered", name)
	}

	m.loaded[name] = plugin

	schemes := plugin.SupportedSchemes()
	for _, scheme := range schemes {
		m.backends[scheme] = plugin
	}

	return nil
}

// RegisterFormatterPlugin registers a formatter plugin directly
func (m *Manager) RegisterFormatterPlugin(plugin FormatterPlugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := plugin.Name()
	if _, exists := m.loaded[name]; exists {
		return fmt.Errorf("plugin %s already registered", name)
	}

	m.loaded[name] = plugin
	formatName := plugin.FormatName()
	m.formatters[formatName] = plugin

	return nil
}

// RegisterFilterPlugin registers a filter plugin directly
func (m *Manager) RegisterFilterPlugin(plugin FilterPlugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := plugin.Name()
	if _, exists := m.loaded[name]; exists {
		return fmt.Errorf("plugin %s already registered", name)
	}

	m.loaded[name] = plugin
	filterType := plugin.FilterType()
	m.filters[filterType] = plugin

	return nil
}
