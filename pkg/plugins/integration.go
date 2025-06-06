package plugins

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	
	"github.com/wayneeseguin/omni/pkg/types"
)

// Integration provides helpers for integrating plugins with Omni
type Integration struct {
	manager *Manager
}

// NewIntegration creates a new plugin integration helper
func NewIntegration(manager *Manager) *Integration {
	return &Integration{
		manager: manager,
	}
}

// CreateBackendFromURI creates a backend instance from a URI using plugins
func (i *Integration) CreateBackendFromURI(uri string) (Backend, error) {
	// Parse URI
	parsedURI, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("parse URI: %w", err)
	}
	
	scheme := parsedURI.Scheme
	if scheme == "" {
		return nil, fmt.Errorf("URI missing scheme")
	}
	
	// Find backend plugin for scheme
	plugin, exists := i.manager.GetBackendPlugin(scheme)
	if !exists {
		return nil, fmt.Errorf("no backend plugin found for scheme %s", scheme)
	}
	
	// Extract config from URI query parameters
	config := make(map[string]interface{})
	for key, values := range parsedURI.Query() {
		if len(values) == 1 {
			config[key] = values[0]
		} else {
			config[key] = values
		}
	}
	
	// Create backend instance
	backend, err := plugin.CreateBackend(uri, config)
	if err != nil {
		return nil, fmt.Errorf("create backend: %w", err)
	}
	
	return backend, nil
}

// CreateFormatterByName creates a formatter instance by name using plugins
func (i *Integration) CreateFormatterByName(name string, config map[string]interface{}) (Formatter, error) {
	plugin, exists := i.manager.GetFormatterPlugin(name)
	if !exists {
		return nil, fmt.Errorf("no formatter plugin found for format %s", name)
	}
	
	formatter, err := plugin.CreateFormatter(config)
	if err != nil {
		return nil, fmt.Errorf("create formatter: %w", err)
	}
	
	return formatter, nil
}

// CreateFilterByType creates a filter instance by type using plugins
func (i *Integration) CreateFilterByType(filterType string, config map[string]interface{}) (types.FilterFunc, error) {
	plugin, exists := i.manager.GetFilterPlugin(filterType)
	if !exists {
		return nil, fmt.Errorf("no filter plugin found for type %s", filterType)
	}
	
	filter, err := plugin.CreateFilter(config)
	if err != nil {
		return nil, fmt.Errorf("create filter: %w", err)
	}
	
	return filter, nil
}

// ShutdownAll shuts down all loaded plugins
func (i *Integration) ShutdownAll(ctx context.Context) error {
	plugins := i.manager.ListPlugins()
	var errors []string
	
	for _, plugin := range plugins {
		if err := plugin.Shutdown(ctx); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", plugin.Name(), err))
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("shutdown errors:\n%s", strings.Join(errors, "\n"))
	}
	
	return nil
}

// GetAvailableBackends returns a list of available backend schemes
func (i *Integration) GetAvailableBackends() []string {
	i.manager.mu.RLock()
	defer i.manager.mu.RUnlock()
	
	schemes := make([]string, 0, len(i.manager.backends))
	for scheme := range i.manager.backends {
		schemes = append(schemes, scheme)
	}
	
	return schemes
}

// GetAvailableFormatters returns a list of available formatter names
func (i *Integration) GetAvailableFormatters() []string {
	i.manager.mu.RLock()
	defer i.manager.mu.RUnlock()
	
	formats := make([]string, 0, len(i.manager.formatters))
	for format := range i.manager.formatters {
		formats = append(formats, format)
	}
	
	return formats
}

// GetAvailableFilters returns a list of available filter types
func (i *Integration) GetAvailableFilters() []string {
	i.manager.mu.RLock()
	defer i.manager.mu.RUnlock()
	
	filterTypes := make([]string, 0, len(i.manager.filters))
	for filterType := range i.manager.filters {
		filterTypes = append(filterTypes, filterType)
	}
	
	return filterTypes
}

// ValidatePluginHealth performs health checks on all loaded plugins
func (i *Integration) ValidatePluginHealth() error {
	plugins := i.manager.ListPlugins()
	var errors []string
	
	for _, plugin := range plugins {
		// Try to get basic info
		name := plugin.Name()
		version := plugin.Version()
		
		if name == "" {
			errors = append(errors, "plugin with empty name detected")
		}
		if version == "" {
			errors = append(errors, fmt.Sprintf("plugin %s has empty version", name))
		}
		
		// Check type-specific functionality
		if backendPlugin, ok := plugin.(BackendPlugin); ok {
			schemes := backendPlugin.SupportedSchemes()
			if len(schemes) == 0 {
				errors = append(errors, fmt.Sprintf("backend plugin %s supports no schemes", name))
			}
		}
		
		if formatterPlugin, ok := plugin.(FormatterPlugin); ok {
			formatName := formatterPlugin.FormatName()
			if formatName == "" {
				errors = append(errors, fmt.Sprintf("formatter plugin %s has empty format name", name))
			}
		}
		
		if filterPlugin, ok := plugin.(FilterPlugin); ok {
			filterType := filterPlugin.FilterType()
			if filterType == "" {
				errors = append(errors, fmt.Sprintf("filter plugin %s has empty filter type", name))
			}
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("plugin health check failed:\n%s", strings.Join(errors, "\n"))
	}
	
	return nil
}

// PluginCapabilities represents the capabilities of loaded plugins
type PluginCapabilities struct {
	BackendSchemes []string `json:"backend_schemes"`
	FormatNames    []string `json:"format_names"`
	FilterTypes    []string `json:"filter_types"`
	PluginCount    int      `json:"plugin_count"`
}

// GetCapabilities returns the capabilities of all loaded plugins
func (i *Integration) GetCapabilities() PluginCapabilities {
	return PluginCapabilities{
		BackendSchemes: i.GetAvailableBackends(),
		FormatNames:    i.GetAvailableFormatters(),
		FilterTypes:    i.GetAvailableFilters(),
		PluginCount:    len(i.manager.ListPlugins()),
	}
}

// Helper functions for common plugin operations

// IsBackendSupported checks if a backend scheme is supported by any plugin
func (i *Integration) IsBackendSupported(scheme string) bool {
	_, exists := i.manager.GetBackendPlugin(scheme)
	return exists
}

// IsFormatterSupported checks if a formatter name is supported by any plugin
func (i *Integration) IsFormatterSupported(name string) bool {
	_, exists := i.manager.GetFormatterPlugin(name)
	return exists
}

// IsFilterSupported checks if a filter type is supported by any plugin
func (i *Integration) IsFilterSupported(filterType string) bool {
	_, exists := i.manager.GetFilterPlugin(filterType)
	return exists
}

// CreateDestinationConfig creates a configuration map for a destination
func CreateDestinationConfig(options ...DestinationOption) map[string]interface{} {
	config := make(map[string]interface{})
	for _, option := range options {
		option(config)
	}
	return config
}

// DestinationOption is a function that configures a destination
type DestinationOption func(map[string]interface{})

// WithBatchSize sets the batch size for a destination
func WithBatchSize(size int) DestinationOption {
	return func(config map[string]interface{}) {
		config["batch_size"] = size
	}
}

// WithFlushInterval sets the flush interval for a destination
func WithFlushInterval(seconds int) DestinationOption {
	return func(config map[string]interface{}) {
		config["flush_interval"] = seconds
	}
}

// WithRetryAttempts sets the retry attempts for a destination
func WithRetryAttempts(attempts int) DestinationOption {
	return func(config map[string]interface{}) {
		config["retry_attempts"] = attempts
	}
}

// WithTimeout sets the timeout for a destination
func WithTimeout(seconds int) DestinationOption {
	return func(config map[string]interface{}) {
		config["timeout"] = seconds
	}
}

// WithCustomConfig adds custom configuration to a destination
func WithCustomConfig(key string, value interface{}) DestinationOption {
	return func(config map[string]interface{}) {
		config[key] = value
	}
}