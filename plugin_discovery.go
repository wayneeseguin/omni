package omni

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// PluginDiscovery handles automatic plugin discovery and loading
type PluginDiscovery struct {
	searchPaths []string
	pattern     string
	manager     *PluginManager
}

// NewPluginDiscovery creates a new plugin discovery instance
func NewPluginDiscovery(manager *PluginManager) *PluginDiscovery {
	return &PluginDiscovery{
		searchPaths: getDefaultSearchPaths(),
		pattern:     "*.so", // Default to shared library files
		manager:     manager,
	}
}

// getDefaultSearchPaths returns default search paths for plugins
func getDefaultSearchPaths() []string {
	paths := []string{
		"./plugins",
		"/usr/local/lib/omni/plugins",
		"/usr/lib/omni/plugins",
	}
	
	// Add paths from environment variable
	if envPaths := os.Getenv("OMNI_PLUGIN_PATH"); envPaths != "" {
		for _, path := range strings.Split(envPaths, ":") {
			if path = strings.TrimSpace(path); path != "" {
				paths = append(paths, path)
			}
		}
	}
	
	// Add user-specific plugin directory
	if homeDir, err := os.UserHomeDir(); err == nil {
		userPluginDir := filepath.Join(homeDir, ".omni", "plugins")
		paths = append(paths, userPluginDir)
	}
	
	return paths
}

// SetSearchPaths sets custom search paths
func (pd *PluginDiscovery) SetSearchPaths(paths []string) {
	pd.searchPaths = paths
}

// AddSearchPath adds a search path
func (pd *PluginDiscovery) AddSearchPath(path string) {
	pd.searchPaths = append(pd.searchPaths, path)
}

// SetPattern sets the file pattern for plugin files
func (pd *PluginDiscovery) SetPattern(pattern string) {
	pd.pattern = pattern
}

// DiscoverPlugins discovers plugins in search paths
func (pd *PluginDiscovery) DiscoverPlugins() ([]string, error) {
	var discovered []string
	
	for _, searchPath := range pd.searchPaths {
		// Check if directory exists
		if _, err := os.Stat(searchPath); os.IsNotExist(err) {
			continue
		}
		
		// Find matching files
		matches, err := filepath.Glob(filepath.Join(searchPath, pd.pattern))
		if err != nil {
			return nil, fmt.Errorf("glob pattern %s in %s: %w", pd.pattern, searchPath, err)
		}
		
		discovered = append(discovered, matches...)
	}
	
	return discovered, nil
}

// LoadDiscoveredPlugins discovers and loads all plugins
func (pd *PluginDiscovery) LoadDiscoveredPlugins() error {
	pluginPaths, err := pd.DiscoverPlugins()
	if err != nil {
		return fmt.Errorf("discover plugins: %w", err)
	}
	
	var errors []string
	loaded := 0
	
	for _, pluginPath := range pluginPaths {
		if err := pd.manager.LoadPlugin(pluginPath); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", pluginPath, err))
		} else {
			loaded++
		}
	}
	
	if len(errors) > 0 && loaded == 0 {
		return fmt.Errorf("failed to load any plugins:\n%s", strings.Join(errors, "\n"))
	}
	
	return nil
}

// PluginSpec represents a plugin specification for automatic loading
type PluginSpec struct {
	Name   string                 `json:"name"`
	Path   string                 `json:"path,omitempty"`
	URL    string                 `json:"url,omitempty"`
	Config map[string]interface{} `json:"config,omitempty"`
}

// LoadPluginSpecs loads plugins from specifications
func (pd *PluginDiscovery) LoadPluginSpecs(specs []PluginSpec) error {
	var errors []string
	
	for _, spec := range specs {
		var pluginPath string
		
		if spec.Path != "" {
			// Load from local path
			pluginPath = spec.Path
		} else if spec.URL != "" {
			// Download from URL (future implementation)
			return fmt.Errorf("URL-based plugin loading not yet implemented")
		} else {
			// Search in plugin paths
			found := false
			for _, searchPath := range pd.searchPaths {
				candidate := filepath.Join(searchPath, spec.Name+".so")
				if _, err := os.Stat(candidate); err == nil {
					pluginPath = candidate
					found = true
					break
				}
			}
			
			if !found {
				errors = append(errors, fmt.Sprintf("plugin %s not found", spec.Name))
				continue
			}
		}
		
		// Load the plugin
		if err := pd.manager.LoadPlugin(pluginPath); err != nil {
			errors = append(errors, fmt.Sprintf("load %s: %v", spec.Name, err))
			continue
		}
		
		// Initialize with config if provided
		if spec.Config != nil {
			if err := pd.manager.InitializePlugin(spec.Name, spec.Config); err != nil {
				errors = append(errors, fmt.Sprintf("initialize %s: %v", spec.Name, err))
			}
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("plugin loading errors:\n%s", strings.Join(errors, "\n"))
	}
	
	return nil
}

// ScanForPluginConfigs scans for plugin configuration files
func (pd *PluginDiscovery) ScanForPluginConfigs() ([]string, error) {
	var configs []string
	
	for _, searchPath := range pd.searchPaths {
		// Look for plugin.json files
		configPattern := filepath.Join(searchPath, "*/plugin.json")
		matches, err := filepath.Glob(configPattern)
		if err != nil {
			continue
		}
		
		configs = append(configs, matches...)
		
		// Also look for omni-plugins.json
		globalConfig := filepath.Join(searchPath, "omni-plugins.json")
		if _, err := os.Stat(globalConfig); err == nil {
			configs = append(configs, globalConfig)
		}
	}
	
	return configs, nil
}

// PluginMetadata represents plugin metadata
type PluginMetadata struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Author      string            `json:"author"`
	License     string            `json:"license"`
	Dependencies []string         `json:"dependencies,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// Default plugin discovery instance
var defaultPluginDiscovery = NewPluginDiscovery(defaultPluginManager)

// DiscoverAndLoadPlugins discovers and loads plugins using the default discovery
func DiscoverAndLoadPlugins() error {
	return defaultPluginDiscovery.LoadDiscoveredPlugins()
}

// SetPluginSearchPaths sets search paths for the default discovery
func SetPluginSearchPaths(paths []string) {
	defaultPluginDiscovery.SetSearchPaths(paths)
}

// AddPluginSearchPath adds a search path for the default discovery
func AddPluginSearchPath(path string) {
	defaultPluginDiscovery.AddSearchPath(path)
}

// PluginRegistry maintains a registry of available plugins
type PluginRegistry struct {
	plugins map[string]PluginMetadata
	mu      sync.RWMutex
}

// NewPluginRegistry creates a new plugin registry
func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		plugins: make(map[string]PluginMetadata),
	}
}

// Register registers plugin metadata
func (pr *PluginRegistry) Register(metadata PluginMetadata) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	
	pr.plugins[metadata.Name] = metadata
}

// Get retrieves plugin metadata
func (pr *PluginRegistry) Get(name string) (PluginMetadata, bool) {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	
	metadata, exists := pr.plugins[name]
	return metadata, exists
}

// List returns all registered plugin metadata
func (pr *PluginRegistry) List() []PluginMetadata {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	
	plugins := make([]PluginMetadata, 0, len(pr.plugins))
	for _, metadata := range pr.plugins {
		plugins = append(plugins, metadata)
	}
	
	return plugins
}

// Global plugin registry
var defaultPluginRegistry = NewPluginRegistry()

// RegisterPluginMetadata registers plugin metadata in the default registry
func RegisterPluginMetadata(metadata PluginMetadata) {
	defaultPluginRegistry.Register(metadata)
}

// GetPluginMetadata retrieves plugin metadata from the default registry
func GetPluginMetadata(name string) (PluginMetadata, bool) {
	return defaultPluginRegistry.Get(name)
}

// ListPluginMetadata returns all plugin metadata from the default registry
func ListPluginMetadata() []PluginMetadata {
	return defaultPluginRegistry.List()
}