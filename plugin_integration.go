package flexlog

import (
	"fmt"
	"strings"
)

// AddDestinationWithPlugin adds a destination using a plugin backend
func (f *FlexLog) AddDestinationWithPlugin(uri string) error {
	// Parse URI to determine backend plugin
	scheme := ""
	if idx := strings.Index(uri, "://"); idx > 0 {
		scheme = uri[:idx]
	}
	
	// Check for plugin backend
	if plugin, exists := defaultPluginManager.GetBackendPlugin(scheme); exists {
		// Create backend using plugin
		backend, err := plugin.CreateBackend(uri, nil)
		if err != nil {
			return fmt.Errorf("create plugin backend: %w", err)
		}
		
		// Generate destination name
		name := generateDestinationName(uri)
		
		// Create destination with plugin backend
		dest := &Destination{
			Name:          name,
			URI:           uri,
			Backend:       BackendPlugin,
			PluginBackend: backend,
			Enabled:       true,
			Done:          make(chan struct{}),
		}
		
		// Initialize destination metrics
		dest.bytesWritten = 0
		dest.errors = 0
		dest.writeCount = 0
		
		// Add to destinations map
		f.destinations.Store(dest.Name, dest)
		
		return nil
	}
	
	// Fallback to regular backend detection
	return f.AddDestinationWithBackend(uri, BackendFlock)
}

// SetCustomFormatter sets a custom formatter for the logger
func (f *FlexLog) SetCustomFormatter(formatName string, config map[string]interface{}) error {
	// Check for plugin formatter
	if plugin, exists := defaultPluginManager.GetFormatterPlugin(formatName); exists {
		formatter, err := plugin.CreateFormatter(config)
		if err != nil {
			return fmt.Errorf("create plugin formatter: %w", err)
		}
		
		// Store the formatter (this would need to be implemented in FlexLog struct)
		f.customFormatter = formatter
		f.format = FormatCustom // New format type for custom formatters
		
		return nil
	}
	
	return fmt.Errorf("formatter plugin %s not found", formatName)
}

// AddFilter adds a filter function to the logger
func (f *FlexLog) AddFilter(filterFunc FilterFunc) {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	// Convert FilterFunc to Filter interface if needed
	filter := &filterWrapper{fn: filterFunc}
	f.filters = append(f.filters, filter)
}

// filterWrapper wraps FilterFunc to implement Filter interface
type filterWrapper struct {
	fn FilterFunc
}

func (fw *filterWrapper) ShouldLog(level int, message string, fields map[string]interface{}) bool {
	return fw.fn(level, message, fields)
}

// generateDestinationName generates a unique name for a destination
func generateDestinationName(uri string) string {
	// Simple implementation - in production would ensure uniqueness
	if idx := strings.LastIndex(uri, "/"); idx >= 0 {
		return uri[idx+1:]
	}
	return uri
}

// ParseLevel parses a level string to level constant
func ParseLevel(levelStr string) int {
	switch strings.ToLower(levelStr) {
	case "trace":
		return LevelTrace
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return -1 // Invalid level
	}
}

// LevelName returns the name for a log level
func LevelName(level int) string {
	switch level {
	case LevelTrace:
		return "TRACE"
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Builder extensions for plugin support

// WithCustomFormatter adds a custom formatter to the builder
func (b *Builder) WithCustomFormatter(formatName string, config map[string]interface{}) *Builder {
	b.customFormatter = formatName
	b.customFormatterConfig = config
	return b
}

// WithFilter adds a filter to the builder
func (b *Builder) WithFilter(filterFunc FilterFunc) *Builder {
	b.filters = append(b.filters, filterFunc)
	return b
}

// NewWithOptions extensions for plugin support

// WithCustomFormatter option for custom formatters
func WithCustomFormatter(formatName string, config map[string]interface{}) Option {
	return func(config *Config) error {
		config.CustomFormatter = formatName
		config.CustomFormatterConfig = config
		return nil
	}
}

// WithFilter option for filters
func WithFilter(filterFunc FilterFunc) Option {
	return func(config *Config) error {
		config.Filters = append(config.Filters, filterFunc)
		return nil
	}
}