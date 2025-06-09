package formatters

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/wayneeseguin/omni/pkg/types"
)

// JSONFormatter formats log messages as JSON
type JSONFormatter struct {
	Options       FormatOptions
	IncludeFields []string // Optional: specific fields to include
	ExcludeFields []string // Optional: fields to exclude
}

// NewJSONFormatter creates a new JSON formatter
func NewJSONFormatter() *JSONFormatter {
	return &JSONFormatter{
		Options: DefaultFormatOptions(),
	}
}

// Format formats a log message as JSON
func (f *JSONFormatter) Format(msg types.LogMessage) ([]byte, error) {
	// Handle raw bytes - pass through as-is
	if msg.Raw != nil {
		return msg.Raw, nil
	}

	// Handle structured entries
	if msg.Entry != nil {
		return f.formatStructuredEntry(msg.Entry)
	}

	// Create a JSON entry from the regular message
	entry := f.createJSONEntry(msg)

	// Marshal to JSON with circular reference protection
	data, err := f.safeMarshal(entry)
	if err != nil {
		return nil, err
	}

	// Add newline for line-delimited JSON
	data = append(data, '\n')

	return data, nil
}

// formatStructuredEntry formats a structured log entry as JSON
func (f *JSONFormatter) formatStructuredEntry(entry *types.LogEntry) ([]byte, error) {
	// Create a map for JSON output
	jsonEntry := make(map[string]interface{})

	// Add timestamp if included
	if f.Options.IncludeTime {
		jsonEntry["timestamp"] = entry.Timestamp
	}

	// Add level if included
	if f.Options.IncludeLevel {
		jsonEntry["level"] = entry.Level
	}

	// Add message
	jsonEntry["message"] = entry.Message

	// Add fields
	if len(entry.Fields) > 0 {
		// Check if we should flatten fields or nest them
		if f.Options.FlattenFields {
			// Add fields directly to the root
			for k, v := range entry.Fields {
				if !f.shouldExcludeField(k) {
					jsonEntry[k] = v
				}
			}
		} else {
			// Nest fields under a "fields" key
			filteredFields := make(map[string]interface{})
			for k, v := range entry.Fields {
				if !f.shouldExcludeField(k) {
					filteredFields[k] = v
				}
			}
			if len(filteredFields) > 0 {
				jsonEntry["fields"] = filteredFields
			}
		}
	}

	// Add stack trace if present
	if entry.StackTrace != "" {
		jsonEntry["stack_trace"] = entry.StackTrace
	}

	// Add metadata if configured
	if entry.Metadata != nil && len(entry.Metadata) > 0 {
		jsonEntry["metadata"] = entry.Metadata
	}

	// Marshal to JSON with circular reference protection
	data, err := f.safeMarshal(jsonEntry)
	if err != nil {
		return nil, err
	}

	// Add newline for line-delimited JSON
	data = append(data, '\n')

	return data, nil
}

// createJSONEntry creates a JSON-serializable entry from a log message
func (f *JSONFormatter) createJSONEntry(msg types.LogMessage) map[string]interface{} {
	entry := make(map[string]interface{})

	// Add timestamp if included
	if f.Options.IncludeTime {
		entry["timestamp"] = f.formatTimestamp(msg.Timestamp)
	}

	// Add level if included
	if f.Options.IncludeLevel {
		entry["level"] = f.formatLevel(msg.Level)
	}

	// Format and add message
	message := ""
	if msg.Format != "" {
		if len(msg.Args) > 0 {
			message = fmt.Sprintf(msg.Format, msg.Args...)
		} else {
			message = msg.Format
		}
	}
	entry["message"] = message

	return entry
}

// formatTimestamp formats a timestamp for JSON output
func (f *JSONFormatter) formatTimestamp(t time.Time) string {
	// Use RFC3339 for JSON by default, or custom format if specified
	if f.Options.TimestampFormat == "" || f.Options.TimestampFormat == "RFC3339" {
		return t.In(f.Options.TimeZone).Format(time.RFC3339)
	}
	return t.In(f.Options.TimeZone).Format(f.Options.TimestampFormat)
}

// formatLevel formats a log level for JSON output
func (f *JSONFormatter) formatLevel(level int) string {
	switch level {
	case LevelTrace:
		return "trace"
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return "log"
	}
}

// shouldExcludeField checks if a field should be excluded from output
func (f *JSONFormatter) shouldExcludeField(field string) bool {
	// Check exclude list
	for _, excluded := range f.ExcludeFields {
		if field == excluded {
			return true
		}
	}

	// If include list is specified, only include fields in the list
	if len(f.IncludeFields) > 0 {
		for _, included := range f.IncludeFields {
			if field == included {
				return false
			}
		}
		return true // Not in include list
	}

	return false
}

// WithIncludeFields sets fields to include in JSON output
func (f *JSONFormatter) WithIncludeFields(fields ...string) *JSONFormatter {
	f.IncludeFields = fields
	return f
}

// WithExcludeFields sets fields to exclude from JSON output
func (f *JSONFormatter) WithExcludeFields(fields ...string) *JSONFormatter {
	f.ExcludeFields = fields
	return f
}

// safeMarshal marshals data to JSON with circular reference protection
func (f *JSONFormatter) safeMarshal(data interface{}) ([]byte, error) {
	// Use a simple approach - attempt to marshal, and if it fails with likely
	// circular reference, replace with a safe representation
	result, err := json.Marshal(data)
	if err != nil {
		// If marshaling fails, it's likely due to circular reference or other issues
		// Create a safe representation
		safe := f.makeSafe(data)
		return json.Marshal(safe)
	}
	return result, nil
}

// makeSafe creates a safe representation of data that won't cause circular reference issues
func (f *JSONFormatter) makeSafe(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		safe := make(map[string]interface{})
		for k, val := range v {
			// Detect obvious self-references by key name
			if k == "self" {
				safe[k] = "[circular reference]"
			} else {
				safe[k] = f.makeSafeRecursive(val, 0, 5) // max depth 5
			}
		}
		return safe
	default:
		return f.makeSafeRecursive(data, 0, 5)
	}
}

// makeSafeRecursive creates a safe representation with depth limiting
func (f *JSONFormatter) makeSafeRecursive(data interface{}, depth, maxDepth int) interface{} {
	if depth >= maxDepth {
		return "[max depth reached]"
	}

	switch v := data.(type) {
	case map[string]interface{}:
		safe := make(map[string]interface{})
		for k, val := range v {
			safe[k] = f.makeSafeRecursive(val, depth+1, maxDepth)
		}
		return safe
	case []interface{}:
		safe := make([]interface{}, len(v))
		for i, val := range v {
			safe[i] = f.makeSafeRecursive(val, depth+1, maxDepth)
		}
		return safe
	default:
		return data
	}
}
