// Package main implements an XML formatter plugin for Omni
package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"github.com/wayneeseguin/omni"
)

// XMLFormatterPlugin implements the FormatterPlugin interface
type XMLFormatterPlugin struct {
	initialized bool
	config      map[string]interface{}
}

// XMLLogEntry represents a log entry in XML format
type XMLLogEntry struct {
	XMLName   xml.Name               `xml:"logEntry"`
	Timestamp string                 `xml:"timestamp"`
	Level     string                 `xml:"level"`
	Message   string                 `xml:"message"`
	Fields    []XMLField             `xml:"fields>field,omitempty"`
}

// XMLField represents a structured field in XML
type XMLField struct {
	Key   string `xml:"key,attr"`
	Value string `xml:",chardata"`
}

// XMLFormatter implements the Formatter interface
type XMLFormatter struct {
	includeFields bool
	timeFormat    string
	rootElement   string
}

// Name returns the plugin name
func (p *XMLFormatterPlugin) Name() string {
	return "xml-formatter"
}

// Version returns the plugin version
func (p *XMLFormatterPlugin) Version() string {
	return "1.0.0"
}

// Initialize initializes the plugin with configuration
func (p *XMLFormatterPlugin) Initialize(config map[string]interface{}) error {
	p.config = config
	p.initialized = true
	return nil
}

// Shutdown cleans up plugin resources
func (p *XMLFormatterPlugin) Shutdown(ctx context.Context) error {
	p.initialized = false
	return nil
}

// CreateFormatter creates a new XML formatter instance
func (p *XMLFormatterPlugin) CreateFormatter(config map[string]interface{}) (omni.Formatter, error) {
	if !p.initialized {
		return nil, fmt.Errorf("plugin not initialized")
	}
	
	formatter := &XMLFormatter{
		includeFields: true,
		timeFormat:    time.RFC3339,
		rootElement:   "logEntry",
	}
	
	// Apply configuration
	if val, ok := config["include_fields"].(bool); ok {
		formatter.includeFields = val
	}
	
	if val, ok := config["time_format"].(string); ok {
		formatter.timeFormat = val
	}
	
	if val, ok := config["root_element"].(string); ok {
		formatter.rootElement = val
	}
	
	return formatter, nil
}

// FormatName returns the format name
func (p *XMLFormatterPlugin) FormatName() string {
	return "xml"
}

// levelToString converts a log level integer to string
func levelToString(level int) string {
	switch level {
	case omni.LevelTrace:
		return "TRACE"
	case omni.LevelDebug:
		return "DEBUG"
	case omni.LevelInfo:
		return "INFO"
	case omni.LevelWarn:
		return "WARN"
	case omni.LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// getMessageText extracts the message text from LogMessage
func getMessageText(msg omni.LogMessage) string {
	// If Entry exists and has a message, use that
	if msg.Entry != nil && msg.Entry.Message != "" {
		return msg.Entry.Message
	}
	
	// If there's a format string, format it with args
	if msg.Format != "" && len(msg.Args) > 0 {
		return fmt.Sprintf(msg.Format, msg.Args...)
	}
	
	// If there's just a format string without args, use it as is
	if msg.Format != "" {
		return msg.Format
	}
	
	// Fallback: convert args to string with spaces
	if len(msg.Args) > 0 {
		strArgs := make([]string, len(msg.Args))
		for i, arg := range msg.Args {
			strArgs[i] = fmt.Sprintf("%v", arg)
		}
		return strings.Join(strArgs, " ")
	}
	
	return ""
}

// getMessageFields extracts fields from LogMessage
func getMessageFields(msg omni.LogMessage) map[string]interface{} {
	if msg.Entry != nil && msg.Entry.Fields != nil {
		return msg.Entry.Fields
	}
	return nil
}

// Format formats a log message as XML
func (f *XMLFormatter) Format(msg omni.LogMessage) ([]byte, error) {
	entry := XMLLogEntry{
		Timestamp: msg.Timestamp.Format(f.timeFormat),
		Level:     levelToString(msg.Level),
		Message:   getMessageText(msg),
	}
	
	// Add fields if enabled and present
	if f.includeFields {
		fields := getMessageFields(msg)
		if fields != nil {
			for key, value := range fields {
				entry.Fields = append(entry.Fields, XMLField{
					Key:   key,
					Value: fmt.Sprintf("%v", value),
				})
			}
		}
	}
	
	// Marshal to XML
	return xml.MarshalIndent(entry, "", "  ")
}

// OmniPlugin is the plugin entry point
var OmniPlugin = &XMLFormatterPlugin{}

func main() {
	// Example usage demonstrating the XML formatter plugin
	fmt.Println("XML Formatter Plugin")
	fmt.Printf("Name: %s\n", OmniPlugin.Name())
	fmt.Printf("Version: %s\n", OmniPlugin.Version())
	
	// Initialize the plugin
	if err := OmniPlugin.Initialize(map[string]interface{}{}); err != nil {
		fmt.Printf("Failed to initialize plugin: %v\n", err)
		return
	}
	
	// Create a formatter
	formatter, err := OmniPlugin.CreateFormatter(map[string]interface{}{
		"include_fields": true,
		"time_format":    time.RFC3339,
	})
	if err != nil {
		fmt.Printf("Failed to create formatter: %v\n", err)
		return
	}
	
	// Create a sample log message
	sampleMsg := omni.LogMessage{
		Level:     omni.LevelInfo,
		Format:    "User %s logged in",
		Args:      []interface{}{"john_doe"},
		Timestamp: time.Now(),
		Entry: &omni.LogEntry{
			Message: "User john_doe logged in",
			Level:   "INFO",
			Fields: map[string]interface{}{
				"user_id":    12345,
				"ip_address": "192.168.1.100",
				"method":     "POST",
			},
		},
	}
	
	// Format the message
	formatted, err := formatter.Format(sampleMsg)
	if err != nil {
		fmt.Printf("Failed to format message: %v\n", err)
		return
	}
	
	fmt.Printf("\nSample XML output:\n%s\n", string(formatted))
}