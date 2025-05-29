package flexlog

import (
	"context"
	"fmt"
	"plugin"
	"sync"
	"time"
)

// Plugin represents a FlexLog plugin
type Plugin interface {
	// Name returns the plugin name
	Name() string
	
	// Version returns the plugin version
	Version() string
	
	// Initialize initializes the plugin with configuration
	Initialize(config map[string]interface{}) error
	
	// Shutdown cleans up plugin resources
	Shutdown(ctx context.Context) error
}

// BackendPlugin interface for custom log backends
type BackendPlugin interface {
	Plugin
	
	// CreateBackend creates a new backend instance
	CreateBackend(uri string, config map[string]interface{}) (Backend, error)
	
	// SupportedSchemes returns URI schemes this plugin supports
	SupportedSchemes() []string
}

// FormatterPlugin interface for custom log formatters
type FormatterPlugin interface {
	Plugin
	
	// CreateFormatter creates a new formatter instance
	CreateFormatter(config map[string]interface{}) (Formatter, error)
	
	// FormatName returns the format name (e.g., "xml", "protobuf")
	FormatName() string
}

// FilterPlugin interface for custom log filters
type FilterPlugin interface {
	Plugin
	
	// CreateFilter creates a new filter instance
	CreateFilter(config map[string]interface{}) (FilterFunc, error)
	
	// FilterType returns the filter type name
	FilterType() string
}

// PluginManager manages loaded plugins
type PluginManager struct {
	mu         sync.RWMutex
	backends   map[string]BackendPlugin
	formatters map[string]FormatterPlugin
	filters    map[string]FilterPlugin
	loaded     map[string]Plugin
}

// NewPluginManager creates a new plugin manager
func NewPluginManager() *PluginManager {
	return &PluginManager{
		backends:   make(map[string]BackendPlugin),
		formatters: make(map[string]FormatterPlugin),
		filters:    make(map[string]FilterPlugin),
		loaded:     make(map[string]Plugin),
	}
}

// Global plugin manager instance
var defaultPluginManager = NewPluginManager()

// LoadPlugin loads a plugin from a shared library file
func (pm *PluginManager) LoadPlugin(path string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	// Load the plugin
	p, err := plugin.Open(path)
	if err != nil {
		return fmt.Errorf("open plugin %s: %w", path, err)
	}
	
	// Look for the plugin entry point
	sym, err := p.Lookup("FlexLogPlugin")
	if err != nil {
		return fmt.Errorf("plugin %s missing FlexLogPlugin symbol: %w", path, err)
	}
	
	// Cast to plugin interface
	pluginInstance, ok := sym.(Plugin)
	if !ok {
		return fmt.Errorf("plugin %s FlexLogPlugin is not a Plugin interface", path)
	}
	
	// Check for duplicate names
	name := pluginInstance.Name()
	if _, exists := pm.loaded[name]; exists {
		return fmt.Errorf("plugin %s already loaded", name)
	}
	
	// Register the plugin
	pm.loaded[name] = pluginInstance
	
	// Register specific plugin types
	if backendPlugin, ok := pluginInstance.(BackendPlugin); ok {
		schemes := backendPlugin.SupportedSchemes()
		for _, scheme := range schemes {
			pm.backends[scheme] = backendPlugin
		}
	}
	
	if formatterPlugin, ok := pluginInstance.(FormatterPlugin); ok {
		formatName := formatterPlugin.FormatName()
		pm.formatters[formatName] = formatterPlugin
	}
	
	if filterPlugin, ok := pluginInstance.(FilterPlugin); ok {
		filterType := filterPlugin.FilterType()
		pm.filters[filterType] = filterPlugin
	}
	
	return nil
}

// UnloadPlugin unloads a plugin by name
func (pm *PluginManager) UnloadPlugin(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	pluginInstance, exists := pm.loaded[name]
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
			delete(pm.backends, scheme)
		}
	}
	
	if formatterPlugin, ok := pluginInstance.(FormatterPlugin); ok {
		formatName := formatterPlugin.FormatName()
		delete(pm.formatters, formatName)
	}
	
	if filterPlugin, ok := pluginInstance.(FilterPlugin); ok {
		filterType := filterPlugin.FilterType()
		delete(pm.filters, filterType)
	}
	
	delete(pm.loaded, name)
	
	return nil
}

// GetBackendPlugin returns a backend plugin for the given scheme
func (pm *PluginManager) GetBackendPlugin(scheme string) (BackendPlugin, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	plugin, exists := pm.backends[scheme]
	return plugin, exists
}

// GetFormatterPlugin returns a formatter plugin for the given format
func (pm *PluginManager) GetFormatterPlugin(format string) (FormatterPlugin, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	plugin, exists := pm.formatters[format]
	return plugin, exists
}

// GetFilterPlugin returns a filter plugin for the given type
func (pm *PluginManager) GetFilterPlugin(filterType string) (FilterPlugin, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	plugin, exists := pm.filters[filterType]
	return plugin, exists
}

// ListPlugins returns all loaded plugins
func (pm *PluginManager) ListPlugins() []Plugin {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	plugins := make([]Plugin, 0, len(pm.loaded))
	for _, plugin := range pm.loaded {
		plugins = append(plugins, plugin)
	}
	
	return plugins
}

// InitializePlugin initializes a plugin with configuration
func (pm *PluginManager) InitializePlugin(name string, config map[string]interface{}) error {
	pm.mu.RLock()
	pluginInstance, exists := pm.loaded[name]
	pm.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("plugin %s not loaded", name)
	}
	
	return pluginInstance.Initialize(config)
}

// PluginInfo represents plugin information
type PluginInfo struct {
	Name    string                 `json:"name"`
	Version string                 `json:"version"`
	Type    string                 `json:"type"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// GetPluginInfo returns information about loaded plugins
func (pm *PluginManager) GetPluginInfo() []PluginInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	infos := make([]PluginInfo, 0, len(pm.loaded))
	
	for _, plugin := range pm.loaded {
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

// Global plugin management functions

// LoadPlugin loads a plugin using the default plugin manager
func LoadPlugin(path string) error {
	return defaultPluginManager.LoadPlugin(path)
}

// UnloadPlugin unloads a plugin using the default plugin manager
func UnloadPlugin(name string) error {
	return defaultPluginManager.UnloadPlugin(name)
}

// GetPluginManager returns the default plugin manager
func GetPluginManager() *PluginManager {
	return defaultPluginManager
}

// RegisterBackendPlugin registers a backend plugin directly (for built-in plugins)
func RegisterBackendPlugin(plugin BackendPlugin) error {
	defaultPluginManager.mu.Lock()
	defer defaultPluginManager.mu.Unlock()
	
	name := plugin.Name()
	if _, exists := defaultPluginManager.loaded[name]; exists {
		return fmt.Errorf("plugin %s already registered", name)
	}
	
	defaultPluginManager.loaded[name] = plugin
	
	schemes := plugin.SupportedSchemes()
	for _, scheme := range schemes {
		defaultPluginManager.backends[scheme] = plugin
	}
	
	return nil
}

// RegisterFormatterPlugin registers a formatter plugin directly
func RegisterFormatterPlugin(plugin FormatterPlugin) error {
	defaultPluginManager.mu.Lock()
	defer defaultPluginManager.mu.Unlock()
	
	name := plugin.Name()
	if _, exists := defaultPluginManager.loaded[name]; exists {
		return fmt.Errorf("plugin %s already registered", name)
	}
	
	defaultPluginManager.loaded[name] = plugin
	formatName := plugin.FormatName()
	defaultPluginManager.formatters[formatName] = plugin
	
	return nil
}

// RegisterFilterPlugin registers a filter plugin directly
func RegisterFilterPlugin(plugin FilterPlugin) error {
	defaultPluginManager.mu.Lock()
	defer defaultPluginManager.mu.Unlock()
	
	name := plugin.Name()
	if _, exists := defaultPluginManager.loaded[name]; exists {
		return fmt.Errorf("plugin %s already registered", name)
	}
	
	defaultPluginManager.loaded[name] = plugin
	filterType := plugin.FilterType()
	defaultPluginManager.filters[filterType] = plugin
	
	return nil
}