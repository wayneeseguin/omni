package formatters

import (
	"github.com/wayneeseguin/omni/pkg/types"
)

// EnhancedFormatter provides enhanced formatting capabilities
type EnhancedFormatter interface {
	types.Formatter

	// SetFieldOrder sets the order of fields in output
	SetFieldOrder(fields []string)

	// SetFieldFilter sets which fields to include/exclude
	SetFieldFilter(include []string, exclude []string)

	// SetIndentation sets indentation for structured output
	SetIndentation(indent string)
}

// BatchFormatter can format multiple messages at once
type BatchFormatter interface {
	types.Formatter

	// FormatBatch formats multiple messages as a batch
	FormatBatch(messages []types.LogMessage) ([]byte, error)
}

// StreamFormatter formats messages for streaming output
type StreamFormatter interface {
	types.Formatter

	// StartStream initializes streaming output
	StartStream() ([]byte, error)

	// EndStream finalizes streaming output
	EndStream() ([]byte, error)
}

// ContextualFormatter includes contextual information in formatting
type ContextualFormatter interface {
	types.Formatter

	// WithContext adds contextual information to the formatter
	WithContext(key string, value interface{}) ContextualFormatter

	// ClearContext removes all contextual information
	ClearContext()
}
