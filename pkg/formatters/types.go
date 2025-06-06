package formatters

import (
	"fmt"
	"time"
)

// FormatOptions controls the output format
type FormatOptions struct {
	TimestampFormat string
	IncludeLevel    bool
	IncludeTime     bool
	LevelFormat     LevelFormat
	IndentJSON      bool
	FieldSeparator  string
	TimeZone        *time.Location
	FlattenFields   bool // Whether to flatten nested fields in JSON output
	IncludeSource   bool // Whether to include source field
	IncludeHost     bool // Whether to include hostname field
}

// LevelFormat defines level format options
type LevelFormat int

// FormatOption defines format option types
type FormatOption int

const (
	// LevelFormatName formats levels as their names (DEBUG, INFO, etc)
	LevelFormatName LevelFormat = iota
	// LevelFormatNameUpper formats levels as uppercase names
	LevelFormatNameUpper
	// LevelFormatNameLower formats levels as lowercase names
	LevelFormatNameLower
	// LevelFormatSymbol formats levels as single-character symbols
	LevelFormatSymbol
)

// DefaultFormatOptions returns default formatting options
func DefaultFormatOptions() FormatOptions {
	return FormatOptions{
		TimestampFormat: time.RFC3339,
		IncludeLevel:    true,
		IncludeTime:     true,
		LevelFormat:     LevelFormatName,
		IndentJSON:      false,
		FieldSeparator:  " ",
		TimeZone:        time.UTC,
		FlattenFields:   false,
	}
}

// Helper utilities from enhanced.go

// truncateFieldValue truncates a field value if it exceeds the maximum size.
// It handles strings, byte slices, and arrays/slices of other types.
func truncateFieldValue(value interface{}, maxSize int, truncate bool) interface{} {
	switch v := value.(type) {
	case string:
		if len(v) > maxSize {
			if truncate {
				return v[:maxSize] + "...(truncated)"
			}
			return fmt.Sprintf("[string too long: %d bytes]", len(v))
		}
		return v
	case []byte:
		if len(v) > maxSize {
			if truncate {
				return string(v[:maxSize]) + "...(truncated)"
			}
			return fmt.Sprintf("[bytes too long: %d bytes]", len(v))
		}
		return v
	default:
		// For other types, convert to string and check
		str := fmt.Sprintf("%v", v)
		if len(str) > maxSize {
			if truncate {
				return str[:maxSize] + "...(truncated)"
			}
			return fmt.Sprintf("[value too long: %d bytes]", len(str))
		}
		return v
	}
}

// Log level constants
const (
	LevelTrace = 0
	LevelDebug = 1
	LevelInfo  = 2
	LevelWarn  = 3
	LevelError = 4
)