package plugins

import (
	"context"

	"github.com/wayneeseguin/omni/pkg/types"
)

// Backend interface for plugin-based log backends
type Backend interface {
	// Write writes a log entry to the backend
	Write(entry []byte) (int, error)

	// Flush ensures all buffered data is written
	Flush() error

	// Close closes the backend
	Close() error

	// SupportsAtomic returns whether the backend supports atomic writes
	SupportsAtomic() bool

	// Name returns the plugin name
	Name() string

	// Version returns the plugin version
	Version() string

	// Configure configures the plugin with options
	Configure(options map[string]interface{}) error
}

// Filter interface for plugin-based log filters
type Filter interface {
	// ShouldLog determines if a message should be logged
	ShouldLog(level int, message string, fields map[string]interface{}) bool

	// Name returns the filter name
	Name() string

	// Configure configures the filter with options
	Configure(options map[string]interface{}) error
}

// Formatter interface for plugin-based log formatters
type Formatter interface {
	// Format formats a log message
	Format(msg types.LogMessage) ([]byte, error)

	// Name returns the formatter name
	Name() string

	// Configure configures the formatter with options
	Configure(options map[string]interface{}) error
}

// Plugin represents a general plugin
type Plugin interface {
	// Name returns the plugin name
	Name() string

	// Version returns the plugin version
	Version() string

	// Description returns a description of what the plugin does
	Description() string

	// Initialize initializes the plugin
	Initialize(config map[string]interface{}) error

	// Shutdown gracefully shuts down the plugin
	Shutdown(ctx context.Context) error

	// Health returns the health status of the plugin
	Health() HealthStatus
}

// HealthStatus represents the health status of a plugin
type HealthStatus struct {
	Healthy bool
	Message string
	Details map[string]interface{}
}

// ManagerInterface manages plugin lifecycle and discovery
type ManagerInterface interface {
	// RegisterBackend registers a backend plugin
	RegisterBackend(name string, factory BackendFactory) error

	// RegisterFilter registers a filter plugin
	RegisterFilter(name string, factory FilterFactory) error

	// RegisterFormatter registers a formatter plugin
	RegisterFormatter(name string, factory FormatterFactory) error

	// GetBackend gets a backend plugin by name
	GetBackend(name string) (Backend, error)

	// GetFilter gets a filter plugin by name
	GetFilter(name string) (Filter, error)

	// GetFormatter gets a formatter plugin by name
	GetFormatter(name string) (Formatter, error)

	// ListPlugins returns a list of all registered plugins
	ListPlugins() []PluginInfo

	// LoadFromDirectory loads plugins from a directory
	LoadFromDirectory(dir string) error

	// UnloadPlugin unloads a plugin by name
	UnloadPlugin(name string) error
}

// Factory interfaces for creating plugin instances
type BackendFactory func(config map[string]interface{}) (Backend, error)
type FilterFactory func(config map[string]interface{}) (Filter, error)
type FormatterFactory func(config map[string]interface{}) (Formatter, error)

// PluginInfo contains information about a plugin
type PluginInfo struct {
	Name        string                 `json:"name"`
	Version     string                 `json:"version"`
	Type        string                 `json:"type"` // "backend", "filter", "formatter"
	Description string                 `json:"description"`
	Status      string                 `json:"status"` // "loaded", "unloaded", "error"
	Details     map[string]interface{} `json:"details,omitempty"`
}

// Discovery interface for plugin discovery
type Discovery interface {
	// DiscoverPlugins discovers plugins in the specified paths
	DiscoverPlugins(paths []string) ([]PluginInfo, error)

	// LoadPlugin loads a plugin from a file
	LoadPlugin(path string) (Plugin, error)

	// ValidatePlugin validates a plugin before loading
	ValidatePlugin(path string) error
}

// BackendPlugin interface for backend plugins
type BackendPlugin interface {
	Plugin
	Backend

	// CreateBackend creates a new backend instance
	CreateBackend(uri string, config map[string]interface{}) (Backend, error)

	// SupportedSchemes returns URI schemes this plugin supports
	SupportedSchemes() []string
}

// FilterPlugin interface for filter plugins
type FilterPlugin interface {
	Plugin
	Filter

	// CreateFilter creates a new filter instance
	CreateFilter(config map[string]interface{}) (types.FilterFunc, error)

	// FilterType returns the filter type name
	FilterType() string
}

// FormatterPlugin interface for formatter plugins
type FormatterPlugin interface {
	Plugin
	Formatter

	// CreateFormatter creates a new formatter instance
	CreateFormatter(config map[string]interface{}) (Formatter, error)

	// FormatName returns the format name (e.g., "xml", "protobuf")
	FormatName() string
}
