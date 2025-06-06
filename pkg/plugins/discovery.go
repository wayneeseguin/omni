package plugins

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DiscoveryImpl handles automatic plugin discovery and loading
type DiscoveryImpl struct {
	searchPaths []string
	pattern     string
	manager     *Manager
}

// NewDiscovery creates a new plugin discovery instance
func NewDiscovery(manager *Manager) *DiscoveryImpl {
	return &DiscoveryImpl{
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
func (d *DiscoveryImpl) SetSearchPaths(paths []string) {
	d.searchPaths = paths
}

// AddSearchPath adds a search path
func (d *DiscoveryImpl) AddSearchPath(path string) {
	d.searchPaths = append(d.searchPaths, path)
}

// SetPattern sets the file pattern for plugin files
func (d *DiscoveryImpl) SetPattern(pattern string) {
	d.pattern = pattern
}

// DiscoverPlugins discovers plugins in search paths
func (d *DiscoveryImpl) DiscoverPlugins() ([]string, error) {
	var discovered []string
	
	for _, searchPath := range d.searchPaths {
		// Check if directory exists
		if _, err := os.Stat(searchPath); os.IsNotExist(err) {
			continue
		}
		
		// Find matching files
		matches, err := filepath.Glob(filepath.Join(searchPath, d.pattern))
		if err != nil {
			return nil, fmt.Errorf("glob pattern %s in %s: %w", d.pattern, searchPath, err)
		}
		
		discovered = append(discovered, matches...)
	}
	
	return discovered, nil
}

// LoadDiscoveredPlugins discovers and loads all plugins
func (d *DiscoveryImpl) LoadDiscoveredPlugins() error {
	pluginPaths, err := d.DiscoverPlugins()
	if err != nil {
		return fmt.Errorf("discover plugins: %w", err)
	}
	
	var errors []string
	loaded := 0
	
	for _, pluginPath := range pluginPaths {
		if err := d.manager.LoadPlugin(pluginPath); err != nil {
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
func (d *DiscoveryImpl) LoadPluginSpecs(specs []PluginSpec) error {
	var errors []string
	
	for _, spec := range specs {
		var pluginPath string
		
		if spec.Path != "" {
			// Load from local path
			pluginPath = spec.Path
		} else if spec.URL != "" {
			// Download from URL
			var err error
			pluginPath, err = d.downloadPlugin(spec.URL, spec.Name)
			if err != nil {
				errors = append(errors, fmt.Sprintf("download plugin %s: %v", spec.Name, err))
				continue
			}
		} else {
			// Search in plugin paths
			found := false
			for _, searchPath := range d.searchPaths {
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
		if err := d.manager.LoadPlugin(pluginPath); err != nil {
			errors = append(errors, fmt.Sprintf("load plugin %s: %v", spec.Name, err))
			continue
		}
		
		// Initialize if config provided
		if spec.Config != nil {
			if err := d.manager.InitializePlugin(spec.Name, spec.Config); err != nil {
				errors = append(errors, fmt.Sprintf("initialize plugin %s: %v", spec.Name, err))
			}
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("plugin loading errors:\n%s", strings.Join(errors, "\n"))
	}
	
	return nil
}

// LoadPluginConfig loads plugin specifications from a JSON file
func (d *DiscoveryImpl) LoadPluginConfig(configPath string) error {
	file, err := os.Open(configPath)
	if err != nil {
		return fmt.Errorf("open plugin config %s: %w", configPath, err)
	}
	defer file.Close()
	
	var specs []PluginSpec
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&specs); err != nil {
		return fmt.Errorf("decode plugin config: %w", err)
	}
	
	return d.LoadPluginSpecs(specs)
}

// downloadPlugin downloads a plugin from a URL
func (d *DiscoveryImpl) downloadPlugin(url, name string) (string, error) {
	// Create temp directory for downloaded plugins
	tempDir := filepath.Join(os.TempDir(), "omni-plugins")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("create temp directory: %w", err)
	}
	
	// Download the file
	client := &http.Client{
		Timeout: 5 * time.Minute,
	}
	
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("download plugin: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download plugin: HTTP %d", resp.StatusCode)
	}
	
	// Save to file
	pluginPath := filepath.Join(tempDir, name+".so")
	file, err := os.Create(pluginPath)
	if err != nil {
		return "", fmt.Errorf("create plugin file: %w", err)
	}
	defer file.Close()
	
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		os.Remove(pluginPath)
		return "", fmt.Errorf("save plugin file: %w", err)
	}
	
	// Make executable
	if err := os.Chmod(pluginPath, 0755); err != nil {
		os.Remove(pluginPath)
		return "", fmt.Errorf("chmod plugin file: %w", err)
	}
	
	return pluginPath, nil
}

// WatchPluginDirectory watches a directory for new plugins
func (d *DiscoveryImpl) WatchPluginDirectory(dir string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	knownPlugins := make(map[string]bool)
	
	// Initial scan
	matches, _ := filepath.Glob(filepath.Join(dir, d.pattern))
	for _, match := range matches {
		knownPlugins[match] = true
	}
	
	for range ticker.C {
		matches, err := filepath.Glob(filepath.Join(dir, d.pattern))
		if err != nil {
			continue
		}
		
		for _, match := range matches {
			if !knownPlugins[match] {
				// New plugin found
				if err := d.manager.LoadPlugin(match); err == nil {
					knownPlugins[match] = true
				}
			}
		}
	}
}

// PluginMetadata represents plugin metadata from a plugin.json file
type PluginMetadata struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Description  string   `json:"description"`
	Author       string   `json:"author"`
	License      string   `json:"license"`
	Homepage     string   `json:"homepage"`
	Type         string   `json:"type"` // backend, formatter, filter
	Schemes      []string `json:"schemes,omitempty"` // For backend plugins
	FormatName   string   `json:"format_name,omitempty"` // For formatter plugins
	FilterType   string   `json:"filter_type,omitempty"` // For filter plugins
	Dependencies []string `json:"dependencies,omitempty"`
}

// LoadPluginMetadata loads plugin metadata from a JSON file
func LoadPluginMetadata(metadataPath string) (*PluginMetadata, error) {
	file, err := os.Open(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("open metadata file: %w", err)
	}
	defer file.Close()
	
	var metadata PluginMetadata
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&metadata); err != nil {
		return nil, fmt.Errorf("decode metadata: %w", err)
	}
	
	return &metadata, nil
}