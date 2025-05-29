// Package main implements an XML formatter plugin for FlexLog
package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"time"

	"github.com/wayneeseguin/flexlog"
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
func (p *XMLFormatterPlugin) CreateFormatter(config map[string]interface{}) (flexlog.Formatter, error) {
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

// Format formats a log message as XML
func (f *XMLFormatter) Format(msg flexlog.LogMessage) ([]byte, error) {
	entry := XMLLogEntry{
		Timestamp: msg.Timestamp.Format(f.timeFormat),
		Level:     flexlog.LevelName(msg.Level),
		Message:   msg.Message,
	}
	
	// Add fields if enabled and present
	if f.includeFields && msg.Fields != nil {
		for key, value := range msg.Fields {
			entry.Fields = append(entry.Fields, XMLField{
				Key:   key,
				Value: fmt.Sprintf("%v", value),
			})
		}
	}
	
	// Marshal to XML
	return xml.MarshalIndent(entry, "", "  ")
}

// FlexLogPlugin is the plugin entry point
var FlexLogPlugin = &XMLFormatterPlugin{}

func main() {
	// This is a plugin, so main() is not used when loaded as a plugin
	fmt.Println("XML Formatter Plugin")
	fmt.Printf("Name: %s\n", FlexLogPlugin.Name())
	fmt.Printf("Version: %s\n", FlexLogPlugin.Version())
}