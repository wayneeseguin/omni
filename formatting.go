package flexlog

import (
	"fmt"
	"strings"
	"time"
)

// SetFormat sets the output format (text or JSON)
func (f *FlexLog) SetFormat(format LogFormat) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.format = format
}

// SetFormatOption sets a format option
func (f *FlexLog) SetFormatOption(option FormatOption, value interface{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Validate the option value
	switch option {
	case FormatOptionTimestampFormat:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("timestamp format must be a string")
		}
	case FormatOptionIncludeLevel:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("include level must be a boolean")
		}
	case FormatOptionLevelFormat:
		if _, ok := value.(LevelFormat); !ok {
			return fmt.Errorf("level format must be a LevelFormat")
		}
	case FormatOptionIncludeLocation:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("include location must be a boolean")
		}
	case FormatOptionIndentJSON:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("indent JSON must be a boolean")
		}
	case FormatOptionFieldSeparator:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("field separator must be a string")
		}
	case FormatOptionTimeZone:
		if _, ok := value.(*time.Location); !ok {
			return fmt.Errorf("time zone must be a *time.Location")
		}
	default:
		return fmt.Errorf("unknown format option: %v", option)
	}

	f.formatOptions[option] = value
	return nil
}

// GetFormatOption gets a format option
func (f *FlexLog) GetFormatOption(option FormatOption) interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.formatOptions[option]
}

// formatTimestamp formats a timestamp according to the current options
func (f *FlexLog) formatTimestamp(t time.Time) string {
	format := f.formatOptions[FormatOptionTimestampFormat].(string)
	tz := f.formatOptions[FormatOptionTimeZone].(*time.Location)
	return t.In(tz).Format(format)
}

// formatLevel formats a level string according to the current options
func (f *FlexLog) formatLevel(level int) string {
	if includeLevel, ok := f.formatOptions[FormatOptionIncludeLevel].(bool); !ok || !includeLevel {
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

	format, _ := f.formatOptions[FormatOptionLevelFormat].(LevelFormat)
	switch format {
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
func defaultFormatOptions() map[FormatOption]interface{} {
	return map[FormatOption]interface{}{
		FormatOptionTimestampFormat: "2006-01-02 15:04:05.000",
		FormatOptionIncludeLevel:    true,
		FormatOptionLevelFormat:     LevelFormatNameUpper,
		FormatOptionIncludeLocation: false,
		FormatOptionIndentJSON:      false,
		FormatOptionFieldSeparator:  " ",
		FormatOptionTimeZone:        time.Local,
	}
}
