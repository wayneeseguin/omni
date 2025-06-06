package formatters

import (
	"fmt"
	"strings"
	"time"
	
	"github.com/wayneeseguin/omni/pkg/types"
)

// TextFormatter formats log messages as human-readable text
type TextFormatter struct {
	Options FormatOptions
}

// NewTextFormatter creates a new text formatter
func NewTextFormatter() *TextFormatter {
	return &TextFormatter{
		Options: DefaultFormatOptions(),
	}
}

// Format formats a log message as text
func (f *TextFormatter) Format(msg types.LogMessage) ([]byte, error) {
	var result strings.Builder
	
	// Handle raw bytes - pass through as-is
	if msg.Raw != nil {
		return msg.Raw, nil
	}
	
	// Handle structured entries
	if msg.Entry != nil {
		return f.formatStructuredEntry(msg.Entry)
	}
	
	// Format regular message
	message := fmt.Sprintf(msg.Format, msg.Args...)
	
	// Format timestamp if included
	if f.Options.IncludeTime {
		timestamp := f.formatTimestamp(msg.Timestamp)
		result.WriteString("[")
		result.WriteString(timestamp)
		result.WriteString("] ")
	}
	
	// Format level if included
	if f.Options.IncludeLevel {
		levelStr := f.formatLevel(msg.Level)
		result.WriteString("[")
		result.WriteString(levelStr)
		result.WriteString("] ")
	}
	
	// Add the message
	result.WriteString(message)
	
	// Add newline if not present
	if !strings.HasSuffix(message, "\n") {
		result.WriteString("\n")
	}
	
	return []byte(result.String()), nil
}

// formatStructuredEntry formats a structured log entry as text
func (f *TextFormatter) formatStructuredEntry(entry *types.LogEntry) ([]byte, error) {
	var result strings.Builder
	
	// Format timestamp if included
	if f.Options.IncludeTime {
		result.WriteString("[")
		result.WriteString(entry.Timestamp)
		result.WriteString("] ")
	}
	
	// Format level if included
	if f.Options.IncludeLevel {
		result.WriteString("[")
		result.WriteString(entry.Level)
		result.WriteString("] ")
	}
	
	// Add the message
	result.WriteString(entry.Message)
	
	// Add fields
	if len(entry.Fields) > 0 {
		result.WriteString(" ")
		for k, v := range entry.Fields {
			result.WriteString(k)
			result.WriteString("=")
			result.WriteString(fmt.Sprintf("%v", v))
			result.WriteString(" ")
		}
	}
	
	// Add stack trace if present
	if entry.StackTrace != "" {
		result.WriteString("stack_trace=")
		result.WriteString(entry.StackTrace)
		result.WriteString(" ")
	}
	
	// Ensure newline at end
	result.WriteString("\n")
	
	return []byte(result.String()), nil
}

// formatTimestamp formats a timestamp according to the formatter options
func (f *TextFormatter) formatTimestamp(t time.Time) string {
	return t.In(f.Options.TimeZone).Format(f.Options.TimestampFormat)
}

// formatLevel formats a log level according to the formatter options
func (f *TextFormatter) formatLevel(level int) string {
	var levelStr string
	
	switch level {
	case LevelTrace:
		levelStr = "TRACE"
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
	
	// Apply level format
	switch f.Options.LevelFormat {
	case LevelFormatNameLower:
		levelStr = strings.ToLower(levelStr)
	case LevelFormatSymbol:
		if len(levelStr) > 0 {
			levelStr = string(levelStr[0])
		}
	case LevelFormatNameUpper:
		// Already uppercase
	}
	
	return levelStr
}

// FormatFields formats fields as key=value pairs
func (f *TextFormatter) FormatFields(fields map[string]interface{}) string {
	if len(fields) == 0 {
		return ""
	}
	
	var parts []string
	for k, v := range fields {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	
	return strings.Join(parts, " ")
}