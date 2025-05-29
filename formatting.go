package flexlog

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// SetFormat sets the output format for log messages.
// The format determines how log entries are serialized when written to destinations.
//
// Parameters:
//   - format: The output format (FormatText, FormatJSON, or FormatCustom)
//
// Returns:
//   - error: If an invalid format is specified
//
// Example:
//
//	logger.SetFormat(flexlog.FormatJSON)  // Switch to JSON output
//	logger.SetFormat(flexlog.FormatText)  // Switch to text output
func (f *FlexLog) SetFormat(format int) error {
	if format < FormatText || format > FormatCustom {
		return fmt.Errorf("invalid format: %d", format)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.format = format
	return nil
}

// GetFormat returns the current output format.
// This method is thread-safe.
//
// Returns:
//   - int: The current format (FormatText, FormatJSON, or FormatCustom)
func (f *FlexLog) GetFormat() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.format
}

// SetFormatOption sets a specific formatting option.
// Format options control various aspects of how log messages are rendered.
//
// Parameters:
//   - option: The format option to set
//   - value: The value for the option (type depends on option)
//
// Returns:
//   - error: If the value type doesn't match the option requirements
//
// Option-specific value types:
//   - FormatOptionTimestampFormat: string (Go time format)
//   - FormatOptionIncludeLevel: bool
//   - FormatOptionLevelFormat: LevelFormat
//   - FormatOptionIncludeLocation: bool
//   - FormatOptionIndentJSON: bool
//   - FormatOptionFieldSeparator: string
//   - FormatOptionTimeZone: *time.Location
//   - FormatOptionIncludeTime: bool
//
// Example:
//
//	logger.SetFormatOption(flexlog.FormatOptionTimestampFormat, "2006-01-02 15:04:05")
//	logger.SetFormatOption(flexlog.FormatOptionIncludeLevel, true)
//	logger.SetFormatOption(flexlog.FormatOptionLevelFormat, flexlog.LevelFormatSymbol)
func (f *FlexLog) SetFormatOption(option FormatOption, value interface{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Validate and set the option value
	switch option {
	case FormatOptionTimestampFormat:
		if strValue, ok := value.(string); ok {
			f.formatOptions.TimestampFormat = strValue
		} else {
			return fmt.Errorf("timestamp format must be a string")
		}
	case FormatOptionIncludeLevel:
		if boolValue, ok := value.(bool); ok {
			f.formatOptions.IncludeLevel = boolValue
		} else {
			return fmt.Errorf("include level must be a boolean")
		}
	case FormatOptionLevelFormat:
		if formatValue, ok := value.(LevelFormat); ok {
			f.formatOptions.LevelFormat = formatValue
		} else {
			return fmt.Errorf("level format must be a LevelFormat")
		}
	case FormatOptionIndentJSON:
		if boolValue, ok := value.(bool); ok {
			f.formatOptions.IndentJSON = boolValue
		} else {
			return fmt.Errorf("indent JSON must be a boolean")
		}
	case FormatOptionFieldSeparator:
		if strValue, ok := value.(string); ok {
			f.formatOptions.FieldSeparator = strValue
		} else {
			return fmt.Errorf("field separator must be a string")
		}
	case FormatOptionTimeZone:
		if tzValue, ok := value.(*time.Location); ok {
			f.formatOptions.TimeZone = tzValue
		} else {
			return fmt.Errorf("time zone must be a *time.Location")
		}
	case FormatOptionIncludeTime:
		if boolValue, ok := value.(bool); ok {
			f.formatOptions.IncludeTime = boolValue
		} else {
			return fmt.Errorf("include time must be a boolean")
		}
	default:
		return fmt.Errorf("unknown format option: %v", option)
	}

	return nil
}

// GetFormatOption retrieves the current value of a format option.
// This method is thread-safe.
//
// Parameters:
//   - option: The format option to retrieve
//
// Returns:
//   - interface{}: The current value of the option, or nil if option is unknown
//
// Example:
//
//	format := logger.GetFormatOption(flexlog.FormatOptionTimestampFormat).(string)
//	includeLevel := logger.GetFormatOption(flexlog.FormatOptionIncludeLevel).(bool)
func (f *FlexLog) GetFormatOption(option FormatOption) interface{} {
	f.mu.RLock()
	defer f.mu.RUnlock()

	switch option {
	case FormatOptionTimestampFormat:
		return f.formatOptions.TimestampFormat
	case FormatOptionIncludeLevel:
		return f.formatOptions.IncludeLevel
	case FormatOptionLevelFormat:
		return f.formatOptions.LevelFormat
	case FormatOptionIndentJSON:
		return f.formatOptions.IndentJSON
	case FormatOptionFieldSeparator:
		return f.formatOptions.FieldSeparator
	case FormatOptionTimeZone:
		return f.formatOptions.TimeZone
	case FormatOptionIncludeTime:
		return f.formatOptions.IncludeTime
	default:
		return nil
	}
}

// GetFormatOptions returns a copy of all format options.
// This method is thread-safe and returns a copy to prevent external modification.
//
// Returns:
//   - FormatOptions: A copy of the current format options
//
// Example:
//
//	opts := logger.GetFormatOptions()
//	fmt.Printf("Timestamp format: %s\n", opts.TimestampFormat)
//	fmt.Printf("Include level: %v\n", opts.IncludeLevel)
func (f *FlexLog) GetFormatOptions() FormatOptions {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.formatOptions
}

// formatTimestamp formats a timestamp according to the current format options.
// This is an internal helper method used by message formatting functions.
//
// Parameters:
//   - t: The time to format
//
// Returns:
//   - string: The formatted timestamp
func (f *FlexLog) formatTimestamp(t time.Time) string {
	opts := f.GetFormatOptions()
	return t.In(opts.TimeZone).Format(opts.TimestampFormat)
}

// formatLevel formats a log level according to the current format options.
// This is an internal helper method that respects the LevelFormat setting.
//
// Parameters:
//   - level: The numeric log level
//
// Returns:
//   - string: The formatted level string, or empty if IncludeLevel is false
func (f *FlexLog) formatLevel(level int) string {
	opts := f.GetFormatOptions()
	if !opts.IncludeLevel {
		return ""
	}

	var levelStr string
	switch level {
	case LevelDebug:
		levelStr = "DEBUG"
	case LevelInfo:
		levelStr = "INFO"
	case LevelWarn:
		levelStr = "WARN"
	case LevelError:
		levelStr = "ERROR"
	default:
		levelStr = "LOG"
	}

	switch opts.LevelFormat {
	case LevelFormatNameUpper:
		return levelStr
	case LevelFormatNameLower:
		return strings.ToLower(levelStr)
	case LevelFormatSymbol:
		return string(levelStr[0])
	case LevelFormatName:
		return levelStr
	default:
		return levelStr
	}
}

// defaultFormatOptions returns the default formatting options.
// These defaults provide a reasonable configuration for most use cases:
//   - Timestamps in local timezone with millisecond precision
//   - Log levels included in uppercase format
//   - No JSON indentation for compact output
//   - Space as field separator
//
// Returns:
//   - FormatOptions: The default format options
func defaultFormatOptions() FormatOptions {
	return FormatOptions{
		TimestampFormat: "2006-01-02 15:04:05.000",
		IncludeLevel:    true,
		IncludeTime:     true,
		LevelFormat:     LevelFormatNameUpper,
		IndentJSON:      false,
		FieldSeparator:  " ",
		TimeZone:        time.Local,
	}
}

// formatJSONEntry formats a LogEntry as JSON.
// This function ensures consistent JSON formatting for structured log entries.
//
// Parameters:
//   - entry: The log entry to format
//
// Returns:
//   - []byte: The JSON-encoded entry with newline
//   - error: Any JSON marshaling error
func formatJSONEntry(entry *LogEntry) ([]byte, error) {
	// Get a buffer from the pool
	buf := GetBuffer()
	defer PutBuffer(buf)
	
	// Use json.Encoder to write directly to buffer
	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false) // Avoid HTML escaping for performance
	
	if err := encoder.Encode(entry); err != nil {
		return nil, err
	}
	
	// Make a copy since we're returning the buffer to the pool
	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())
	return result, nil
}

// formatJSON formats an object as JSON with optional indentation.
// This is a utility function used internally for JSON formatting.
//
// Parameters:
//   - v: The value to format as JSON
//   - indent: Whether to indent the JSON output with 2 spaces
//
// Returns:
//   - string: The JSON-formatted string
//   - error: Any JSON marshaling error
func formatJSON(v interface{}, indent bool) (string, error) {
	var data []byte
	var err error

	if indent {
		data, err = json.MarshalIndent(v, "", "  ")
	} else {
		data, err = json.Marshal(v)
	}

	if err != nil {
		return "", err
	}

	return string(data), nil
}
