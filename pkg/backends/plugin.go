package backends

import (
	"context"
	"fmt"
	"plugin"
	"sync"
	"time"
	
	"github.com/wayneeseguin/omni/pkg/plugins"
)





// PluginBackendImpl implements the Backend interface using a plugin-provided backend
type PluginBackendImpl struct {
	plugin       plugins.BackendPlugin
	backend      plugins.Backend // This is the plugin's backend interface
	scheme       string
	uri          string
	config       map[string]interface{}
	mu           sync.RWMutex
	writeCount   uint64
	bytesWritten uint64
}

// NewPluginBackend creates a new plugin-based backend
func NewPluginBackend(plugin plugins.BackendPlugin, uri string, config map[string]interface{}) (*PluginBackendImpl, error) {
	backend, err := plugin.CreateBackend(uri, config)
	if err != nil {
		return nil, fmt.Errorf("create plugin backend: %w", err)
	}
	
	return &PluginBackendImpl{
		plugin:  plugin,
		backend: backend,
		uri:     uri,
		config:  config,
	}, nil
}

// Write implements the Backend interface
func (pb *PluginBackendImpl) Write(data []byte) (int, error) {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	
	if pb.backend == nil {
		return 0, fmt.Errorf("plugin backend not initialized")
	}
	
	n, err := pb.backend.Write(data)
	if err == nil {
		pb.writeCount++
		// Only add positive byte counts to prevent underflow
		if n > 0 {
			pb.bytesWritten += uint64(n)
		}
	}
	return n, err
}


// Flush implements the Backend interface
func (pb *PluginBackendImpl) Flush() error {
	pb.mu.RLock()
	defer pb.mu.RUnlock()
	
	if pb.backend == nil {
		return fmt.Errorf("plugin backend not initialized")
	}
	
	return pb.backend.Flush()
}

// Close implements the Backend interface
func (pb *PluginBackendImpl) Close() error {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	
	if pb.backend == nil {
		return nil
	}
	
	err := pb.backend.Close()
	pb.backend = nil
	return err
}


// SupportsAtomic implements the Backend interface
func (pb *PluginBackendImpl) SupportsAtomic() bool {
	pb.mu.RLock()
	defer pb.mu.RUnlock()
	
	if pb.backend == nil {
		return false
	}
	
	return pb.backend.SupportsAtomic()
}

// Reset implements the Backend interface
func (pb *PluginBackendImpl) Reset() error {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	
	if pb.backend != nil {
		if err := pb.backend.Close(); err != nil {
			return fmt.Errorf("close existing backend: %w", err)
		}
	}
	
	// Recreate the backend
	backend, err := pb.plugin.CreateBackend(pb.uri, pb.config)
	if err != nil {
		return fmt.Errorf("recreate plugin backend: %w", err)
	}
	
	pb.backend = backend
	return nil
}

// GetPlugin returns the underlying plugin
func (pb *PluginBackendImpl) GetPlugin() plugins.BackendPlugin {
	return pb.plugin
}

// GetURI returns the URI used to create this backend
func (pb *PluginBackendImpl) GetURI() string {
	return pb.uri
}

// GetConfig returns the configuration used to create this backend
func (pb *PluginBackendImpl) GetConfig() map[string]interface{} {
	return pb.config
}

// Sync syncs the backend (delegates to Flush for plugins)
func (pb *PluginBackendImpl) Sync() error {
	return pb.Flush()
}

// GetStats returns backend statistics
func (pb *PluginBackendImpl) GetStats() BackendStats {
	pb.mu.RLock()
	defer pb.mu.RUnlock()
	
	return BackendStats{
		Path:         pb.uri,
		WriteCount:   pb.writeCount,
		BytesWritten: pb.bytesWritten,
	}
}

// PluginManager manages loaded plugins
type PluginManager struct {
	mu         sync.RWMutex
	backends   map[string]plugins.BackendPlugin
	formatters map[string]plugins.FormatterPlugin
	filters    map[string]plugins.FilterPlugin
	loaded     map[string]plugins.Plugin
}

// NewPluginManager creates a new plugin manager
func NewPluginManager() *PluginManager {
	return &PluginManager{
		backends:   make(map[string]plugins.BackendPlugin),
		formatters: make(map[string]plugins.FormatterPlugin),
		filters:    make(map[string]plugins.FilterPlugin),
		loaded:     make(map[string]plugins.Plugin),
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
	sym, err := p.Lookup("OmniPlugin")
	if err != nil {
		return fmt.Errorf("plugin %s missing OmniPlugin symbol: %w", path, err)
	}
	
	// Cast to plugin interface
	pluginInstance, ok := sym.(plugins.Plugin)
	if !ok {
		return fmt.Errorf("plugin %s OmniPlugin is not a Plugin interface", path)
	}
	
	// Check for duplicate names
	name := pluginInstance.Name()
	if _, exists := pm.loaded[name]; exists {
		return fmt.Errorf("plugin %s already loaded", name)
	}
	
	// Register the plugin
	pm.loaded[name] = pluginInstance
	
	// Register specific plugin types
	if backendPlugin, ok := pluginInstance.(plugins.BackendPlugin); ok {
		schemes := backendPlugin.SupportedSchemes()
		for _, scheme := range schemes {
			pm.backends[scheme] = backendPlugin
		}
	}
	
	if formatterPlugin, ok := pluginInstance.(plugins.FormatterPlugin); ok {
		formatName := formatterPlugin.FormatName()
		pm.formatters[formatName] = formatterPlugin
	}
	
	if filterPlugin, ok := pluginInstance.(plugins.FilterPlugin); ok {
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
	if backendPlugin, ok := pluginInstance.(plugins.BackendPlugin); ok {
		schemes := backendPlugin.SupportedSchemes()
		for _, scheme := range schemes {
			delete(pm.backends, scheme)
		}
	}
	
	if formatterPlugin, ok := pluginInstance.(plugins.FormatterPlugin); ok {
		formatName := formatterPlugin.FormatName()
		delete(pm.formatters, formatName)
	}
	
	if filterPlugin, ok := pluginInstance.(plugins.FilterPlugin); ok {
		filterType := filterPlugin.FilterType()
		delete(pm.filters, filterType)
	}
	
	delete(pm.loaded, name)
	
	return nil
}

// GetBackendPlugin returns a backend plugin for the given scheme
func (pm *PluginManager) GetBackendPlugin(scheme string) (plugins.BackendPlugin, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	plugin, exists := pm.backends[scheme]
	return plugin, exists
}

// GetFormatterPlugin returns a formatter plugin for the given format
func (pm *PluginManager) GetFormatterPlugin(format string) (plugins.FormatterPlugin, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	plugin, exists := pm.formatters[format]
	return plugin, exists
}

// GetFilterPlugin returns a filter plugin for the given type
func (pm *PluginManager) GetFilterPlugin(filterType string) (plugins.FilterPlugin, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	plugin, exists := pm.filters[filterType]
	return plugin, exists
}

// ListPlugins returns all loaded plugins
func (pm *PluginManager) ListPlugins() []plugins.Plugin {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	plugins := make([]plugins.Plugin, 0, len(pm.loaded))
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
		if backendPlugin, ok := plugin.(plugins.BackendPlugin); ok {
			info.Type = "backend"
			info.Details["supported_schemes"] = backendPlugin.SupportedSchemes()
		} else if formatterPlugin, ok := plugin.(plugins.FormatterPlugin); ok {
			info.Type = "formatter"
			info.Details["format_name"] = formatterPlugin.FormatName()
		} else if filterPlugin, ok := plugin.(plugins.FilterPlugin); ok {
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
func RegisterBackendPlugin(plugin plugins.BackendPlugin) error {
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
func RegisterFormatterPlugin(plugin plugins.FormatterPlugin) error {
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
func RegisterFilterPlugin(plugin plugins.FilterPlugin) error {
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

// ClearRegisteredPlugins clears all registered plugins (for testing only)
func ClearRegisteredPlugins() {
	defaultPluginManager.mu.Lock()
	defer defaultPluginManager.mu.Unlock()
	
	defaultPluginManager.backends = make(map[string]plugins.BackendPlugin)
	defaultPluginManager.formatters = make(map[string]plugins.FormatterPlugin)
	defaultPluginManager.filters = make(map[string]plugins.FilterPlugin)
	defaultPluginManager.loaded = make(map[string]plugins.Plugin)
}