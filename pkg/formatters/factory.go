package formatters

import (
	"fmt"
	"sync"
	
	"github.com/wayneeseguin/omni/pkg/types"
)

// Factory creates formatter instances
type Factory struct {
	mu         sync.RWMutex
	formatters map[string]FormatterConstructor
}

// FormatterConstructor is a function that creates a formatter
type FormatterConstructor func() (types.Formatter, error)

// NewFactory creates a new formatter factory with default formatters registered
func NewFactory() *Factory {
	f := &Factory{
		formatters: make(map[string]FormatterConstructor),
	}
	
	// Register default formatters
	f.Register("text", func() (types.Formatter, error) {
		return NewTextFormatter(), nil
	})
	
	f.Register("json", func() (types.Formatter, error) {
		return NewJSONFormatter(), nil
	})
	
	return f
}

// Register registers a new formatter constructor
func (f *Factory) Register(name string, constructor FormatterConstructor) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	if name == "" {
		return fmt.Errorf("formatter name cannot be empty")
	}
	
	if constructor == nil {
		return fmt.Errorf("formatter constructor cannot be nil")
	}
	
	f.formatters[name] = constructor
	return nil
}

// CreateFormatter creates a formatter by name
func (f *Factory) CreateFormatter(name string) (types.Formatter, error) {
	f.mu.RLock()
	constructor, exists := f.formatters[name]
	f.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("formatter %q not registered", name)
	}
	
	return constructor()
}

// Format type constants
const (
	FormatText   = 0
	FormatJSON   = 1
	FormatCustom = 2
)

// CreateFormatterByType creates a formatter by type constant
func (f *Factory) CreateFormatterByType(formatType int) (types.Formatter, error) {
	switch formatType {
	case FormatText:
		return f.CreateFormatter("text")
	case FormatJSON:
		return f.CreateFormatter("json")
	case FormatCustom:
		// For custom format, we need a name from somewhere
		// This would typically come from configuration
		return nil, fmt.Errorf("custom formatter requires explicit name")
	default:
		return nil, fmt.Errorf("unknown format type: %d", formatType)
	}
}

// ListFormatters returns the names of all registered formatters
func (f *Factory) ListFormatters() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	names := make([]string, 0, len(f.formatters))
	for name := range f.formatters {
		names = append(names, name)
	}
	
	return names
}

// DefaultFactory is the global formatter factory
var DefaultFactory = NewFactory()

// Register registers a formatter with the default factory
func Register(name string, constructor FormatterConstructor) error {
	return DefaultFactory.Register(name, constructor)
}

// CreateFormatter creates a formatter using the default factory
func CreateFormatter(name string) (types.Formatter, error) {
	return DefaultFactory.CreateFormatter(name)
}

// CreateFormatterByType creates a formatter by type using the default factory
func CreateFormatterByType(formatType int) (types.Formatter, error) {
	return DefaultFactory.CreateFormatterByType(formatType)
}