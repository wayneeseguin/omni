package flexlog

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// SetFormat sets the output format (text or JSON)
func (f *FlexLog) SetFormat(format LogFormat) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.format = int(format)
}

// GetFormat returns the current output format (thread-safe)
func (f *FlexLog) GetFormat() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.format
}

// SetFormatOption sets a format option
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

// GetFormatOption gets a format option
func (f *FlexLog) GetFormatOption(option FormatOption) interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()

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

// GetFormatOptions returns a copy of all format options (thread-safe)
func (f *FlexLog) GetFormatOptions() FormatOptions {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.formatOptions
}

// formatTimestamp formats a timestamp according to the current options
func (f *FlexLog) formatTimestamp(t time.Time) string {
	opts := f.GetFormatOptions()
	return t.In(opts.TimeZone).Format(opts.TimestampFormat)
}

// formatLevel formats a level string according to the current options
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

// defaultFormatOptions returns default formatting options
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

// formatJSONEntry formats a LogEntry as JSON
func formatJSONEntry(entry *LogEntry) (string, error) {
	// Use the formatJSON function to properly marshal the entire entry
	jsonStr, err := formatJSON(entry, false)
	if err != nil {
		return "", err
	}
	return jsonStr + "\n", nil
}

// formatJSON formats an object as JSON with optional indentation
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
