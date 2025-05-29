package main

import (
	"context"
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/wayneeseguin/flexlog"
)

func TestXMLFormatterPlugin_Name(t *testing.T) {
	plugin := &XMLFormatterPlugin{}
	if plugin.Name() != "xml-formatter" {
		t.Errorf("Expected plugin name 'xml-formatter', got '%s'", plugin.Name())
	}
}

func TestXMLFormatterPlugin_Version(t *testing.T) {
	plugin := &XMLFormatterPlugin{}
	if plugin.Version() != "1.0.0" {
		t.Errorf("Expected plugin version '1.0.0', got '%s'", plugin.Version())
	}
}

func TestXMLFormatterPlugin_Initialize(t *testing.T) {
	plugin := &XMLFormatterPlugin{}
	
	// Initially not initialized
	if plugin.initialized {
		t.Error("Plugin should not be initialized initially")
	}
	
	// Initialize with empty config
	err := plugin.Initialize(map[string]interface{}{})
	if err != nil {
		t.Errorf("Initialize failed: %v", err)
	}
	
	if !plugin.initialized {
		t.Error("Plugin should be initialized after Initialize()")
	}
}

func TestXMLFormatterPlugin_Shutdown(t *testing.T) {
	plugin := &XMLFormatterPlugin{}
	plugin.initialized = true
	
	ctx := context.Background()
	err := plugin.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
	
	if plugin.initialized {
		t.Error("Plugin should not be initialized after Shutdown()")
	}
}

func TestXMLFormatterPlugin_FormatName(t *testing.T) {
	plugin := &XMLFormatterPlugin{}
	if plugin.FormatName() != "xml" {
		t.Errorf("Expected format name 'xml', got '%s'", plugin.FormatName())
	}
}

func TestXMLFormatterPlugin_CreateFormatter(t *testing.T) {
	plugin := &XMLFormatterPlugin{}
	
	// Should fail when not initialized
	_, err := plugin.CreateFormatter(map[string]interface{}{})
	if err == nil {
		t.Error("CreateFormatter should fail when plugin not initialized")
	}
	
	// Initialize plugin
	plugin.Initialize(map[string]interface{}{})
	
	// Should succeed when initialized
	formatter, err := plugin.CreateFormatter(map[string]interface{}{})
	if err != nil {
		t.Errorf("CreateFormatter failed: %v", err)
	}
	
	if formatter == nil {
		t.Error("CreateFormatter should return a formatter")
	}
	
	// Verify it's an XMLFormatter
	xmlFormatter, ok := formatter.(*XMLFormatter)
	if !ok {
		t.Error("CreateFormatter should return an XMLFormatter")
	}
	
	// Check default values
	if !xmlFormatter.includeFields {
		t.Error("Default includeFields should be true")
	}
	if xmlFormatter.timeFormat != time.RFC3339 {
		t.Errorf("Default timeFormat should be RFC3339, got %s", xmlFormatter.timeFormat)
	}
	if xmlFormatter.rootElement != "logEntry" {
		t.Errorf("Default rootElement should be 'logEntry', got %s", xmlFormatter.rootElement)
	}
}

func TestXMLFormatterPlugin_CreateFormatterWithConfig(t *testing.T) {
	plugin := &XMLFormatterPlugin{}
	plugin.Initialize(map[string]interface{}{})
	
	config := map[string]interface{}{
		"include_fields": false,
		"time_format":    "2006-01-02 15:04:05",
		"root_element":   "customEntry",
	}
	
	formatter, err := plugin.CreateFormatter(config)
	if err != nil {
		t.Errorf("CreateFormatter with config failed: %v", err)
	}
	
	xmlFormatter := formatter.(*XMLFormatter)
	
	if xmlFormatter.includeFields {
		t.Error("includeFields should be false from config")
	}
	if xmlFormatter.timeFormat != "2006-01-02 15:04:05" {
		t.Errorf("timeFormat should be '2006-01-02 15:04:05', got %s", xmlFormatter.timeFormat)
	}
	if xmlFormatter.rootElement != "customEntry" {
		t.Errorf("rootElement should be 'customEntry', got %s", xmlFormatter.rootElement)
	}
}

func TestLevelToString(t *testing.T) {
	tests := []struct {
		level    int
		expected string
	}{
		{flexlog.LevelTrace, "TRACE"},
		{flexlog.LevelDebug, "DEBUG"},
		{flexlog.LevelInfo, "INFO"},
		{flexlog.LevelWarn, "WARN"},
		{flexlog.LevelError, "ERROR"},
		{999, "UNKNOWN"},
	}
	
	for _, test := range tests {
		result := levelToString(test.level)
		if result != test.expected {
			t.Errorf("levelToString(%d) = %s, expected %s", test.level, result, test.expected)
		}
	}
}

func TestGetMessageText(t *testing.T) {
	// Test with Entry message
	msg1 := flexlog.LogMessage{
		Entry: &flexlog.LogEntry{
			Message: "Entry message",
		},
	}
	if getMessageText(msg1) != "Entry message" {
		t.Errorf("Expected 'Entry message', got '%s'", getMessageText(msg1))
	}
	
	// Test with format and args
	msg2 := flexlog.LogMessage{
		Format: "User %s logged in",
		Args:   []interface{}{"john"},
	}
	if getMessageText(msg2) != "User john logged in" {
		t.Errorf("Expected 'User john logged in', got '%s'", getMessageText(msg2))
	}
	
	// Test with format only
	msg3 := flexlog.LogMessage{
		Format: "Simple message",
	}
	if getMessageText(msg3) != "Simple message" {
		t.Errorf("Expected 'Simple message', got '%s'", getMessageText(msg3))
	}
	
	// Test with args only
	msg4 := flexlog.LogMessage{
		Args: []interface{}{"hello", "world"},
	}
	if getMessageText(msg4) != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", getMessageText(msg4))
	}
	
	// Test empty message
	msg5 := flexlog.LogMessage{}
	if getMessageText(msg5) != "" {
		t.Errorf("Expected empty string, got '%s'", getMessageText(msg5))
	}
}

func TestGetMessageFields(t *testing.T) {
	// Test with Entry fields
	fields := map[string]interface{}{
		"user_id": 123,
		"action":  "login",
	}
	msg1 := flexlog.LogMessage{
		Entry: &flexlog.LogEntry{
			Fields: fields,
		},
	}
	result := getMessageFields(msg1)
	if len(result) != 2 {
		t.Errorf("Expected 2 fields, got %d", len(result))
	}
	if result["user_id"] != 123 {
		t.Errorf("Expected user_id 123, got %v", result["user_id"])
	}
	
	// Test with no fields
	msg2 := flexlog.LogMessage{}
	result2 := getMessageFields(msg2)
	if result2 != nil {
		t.Errorf("Expected nil fields, got %v", result2)
	}
}

func TestXMLFormatter_Format(t *testing.T) {
	formatter := &XMLFormatter{
		includeFields: true,
		timeFormat:    time.RFC3339,
		rootElement:   "logEntry",
	}
	
	testTime := time.Date(2023, 12, 25, 10, 30, 45, 0, time.UTC)
	
	msg := flexlog.LogMessage{
		Level:     flexlog.LevelInfo,
		Timestamp: testTime,
		Entry: &flexlog.LogEntry{
			Message: "Test message",
			Fields: map[string]interface{}{
				"user_id": 123,
				"action":  "test",
			},
		},
	}
	
	result, err := formatter.Format(msg)
	if err != nil {
		t.Errorf("Format failed: %v", err)
	}
	
	// Parse the XML to verify structure
	var entry XMLLogEntry
	if err := xml.Unmarshal(result, &entry); err != nil {
		t.Errorf("Failed to unmarshal XML: %v", err)
	}
	
	// Verify content
	if entry.Level != "INFO" {
		t.Errorf("Expected level 'INFO', got '%s'", entry.Level)
	}
	if entry.Message != "Test message" {
		t.Errorf("Expected message 'Test message', got '%s'", entry.Message)
	}
	if entry.Timestamp != testTime.Format(time.RFC3339) {
		t.Errorf("Expected timestamp '%s', got '%s'", testTime.Format(time.RFC3339), entry.Timestamp)
	}
	
	// Verify fields
	if len(entry.Fields) != 2 {
		t.Errorf("Expected 2 fields, got %d", len(entry.Fields))
	}
	
	// Check that XML is well-formed and contains expected elements
	xmlStr := string(result)
	if !strings.Contains(xmlStr, "<logEntry>") {
		t.Error("XML should contain <logEntry> element")
	}
	if !strings.Contains(xmlStr, "<level>INFO</level>") {
		t.Error("XML should contain level element")
	}
	if !strings.Contains(xmlStr, "<message>Test message</message>") {
		t.Error("XML should contain message element")
	}
}

func TestXMLFormatter_FormatWithoutFields(t *testing.T) {
	formatter := &XMLFormatter{
		includeFields: false,
		timeFormat:    time.RFC3339,
		rootElement:   "logEntry",
	}
	
	testTime := time.Date(2023, 12, 25, 10, 30, 45, 0, time.UTC)
	
	msg := flexlog.LogMessage{
		Level:     flexlog.LevelError,
		Timestamp: testTime,
		Entry: &flexlog.LogEntry{
			Message: "Error occurred",
			Fields: map[string]interface{}{
				"error_code": 500,
			},
		},
	}
	
	result, err := formatter.Format(msg)
	if err != nil {
		t.Errorf("Format failed: %v", err)
	}
	
	var entry XMLLogEntry
	if err := xml.Unmarshal(result, &entry); err != nil {
		t.Errorf("Failed to unmarshal XML: %v", err)
	}
	
	// Should have no fields since includeFields is false
	if len(entry.Fields) != 0 {
		t.Errorf("Expected 0 fields when includeFields is false, got %d", len(entry.Fields))
	}
	
	if entry.Level != "ERROR" {
		t.Errorf("Expected level 'ERROR', got '%s'", entry.Level)
	}
	if entry.Message != "Error occurred" {
		t.Errorf("Expected message 'Error occurred', got '%s'", entry.Message)
	}
}

func TestXMLFormatter_FormatCustomTimeFormat(t *testing.T) {
	formatter := &XMLFormatter{
		includeFields: true,
		timeFormat:    "2006-01-02 15:04:05",
		rootElement:   "logEntry",
	}
	
	testTime := time.Date(2023, 12, 25, 10, 30, 45, 0, time.UTC)
	
	msg := flexlog.LogMessage{
		Level:     flexlog.LevelWarn,
		Timestamp: testTime,
		Entry: &flexlog.LogEntry{
			Message: "Warning message",
		},
	}
	
	result, err := formatter.Format(msg)
	if err != nil {
		t.Errorf("Format failed: %v", err)
	}
	
	var entry XMLLogEntry
	if err := xml.Unmarshal(result, &entry); err != nil {
		t.Errorf("Failed to unmarshal XML: %v", err)
	}
	
	expectedTimestamp := "2023-12-25 10:30:45"
	if entry.Timestamp != expectedTimestamp {
		t.Errorf("Expected timestamp '%s', got '%s'", expectedTimestamp, entry.Timestamp)
	}
}

func TestXMLFormatter_FormatMessageFromFormat(t *testing.T) {
	formatter := &XMLFormatter{
		includeFields: false,
		timeFormat:    time.RFC3339,
		rootElement:   "logEntry",
	}
	
	testTime := time.Date(2023, 12, 25, 10, 30, 45, 0, time.UTC)
	
	msg := flexlog.LogMessage{
		Level:     flexlog.LevelDebug,
		Timestamp: testTime,
		Format:    "Processing %s with ID %d",
		Args:      []interface{}{"user", 42},
	}
	
	result, err := formatter.Format(msg)
	if err != nil {
		t.Errorf("Format failed: %v", err)
	}
	
	var entry XMLLogEntry
	if err := xml.Unmarshal(result, &entry); err != nil {
		t.Errorf("Failed to unmarshal XML: %v", err)
	}
	
	expectedMessage := "Processing user with ID 42"
	if entry.Message != expectedMessage {
		t.Errorf("Expected message '%s', got '%s'", expectedMessage, entry.Message)
	}
	
	if entry.Level != "DEBUG" {
		t.Errorf("Expected level 'DEBUG', got '%s'", entry.Level)
	}
}

func TestXMLFormatterIntegration(t *testing.T) {
	// Test the full plugin workflow
	plugin := &XMLFormatterPlugin{}
	
	// Initialize plugin
	err := plugin.Initialize(map[string]interface{}{})
	if err != nil {
		t.Fatalf("Failed to initialize plugin: %v", err)
	}
	
	// Create formatter
	formatter, err := plugin.CreateFormatter(map[string]interface{}{
		"include_fields": true,
		"time_format":    time.RFC3339,
	})
	if err != nil {
		t.Fatalf("Failed to create formatter: %v", err)
	}
	
	// Create test message
	testTime := time.Now()
	msg := flexlog.LogMessage{
		Level:     flexlog.LevelInfo,
		Timestamp: testTime,
		Entry: &flexlog.LogEntry{
			Message: "Integration test message",
			Fields: map[string]interface{}{
				"test_id":     "integration_001",
				"environment": "test",
				"version":     "1.0.0",
			},
		},
	}
	
	// Format message
	result, err := formatter.Format(msg)
	if err != nil {
		t.Fatalf("Failed to format message: %v", err)
	}
	
	// Verify result is valid XML
	var entry XMLLogEntry
	if err := xml.Unmarshal(result, &entry); err != nil {
		t.Fatalf("Result is not valid XML: %v", err)
	}
	
	// Verify content
	if entry.Message != "Integration test message" {
		t.Errorf("Expected 'Integration test message', got '%s'", entry.Message)
	}
	if entry.Level != "INFO" {
		t.Errorf("Expected level 'INFO', got '%s'", entry.Level)
	}
	if len(entry.Fields) != 3 {
		t.Errorf("Expected 3 fields, got %d", len(entry.Fields))
	}
	
	// Shutdown plugin
	ctx := context.Background()
	err = plugin.Shutdown(ctx)
	if err != nil {
		t.Errorf("Failed to shutdown plugin: %v", err)
	}
}

func TestXMLFieldFormattingTypes(t *testing.T) {
	formatter := &XMLFormatter{
		includeFields: true,
		timeFormat:    time.RFC3339,
		rootElement:   "logEntry",
	}
	
	testTime := time.Now()
	
	// Test various field types
	msg := flexlog.LogMessage{
		Level:     flexlog.LevelInfo,
		Timestamp: testTime,
		Entry: &flexlog.LogEntry{
			Message: "Field types test",
			Fields: map[string]interface{}{
				"string_field":  "test_string",
				"int_field":     42,
				"float_field":   3.14,
				"bool_field":    true,
				"nil_field":     nil,
			},
		},
	}
	
	result, err := formatter.Format(msg)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}
	
	// Verify XML is valid
	var entry XMLLogEntry
	if err := xml.Unmarshal(result, &entry); err != nil {
		t.Fatalf("Failed to unmarshal XML: %v", err)
	}
	
	// Check that all fields are present and properly formatted
	if len(entry.Fields) != 5 {
		t.Errorf("Expected 5 fields, got %d", len(entry.Fields))
	}
	
	// Verify field values are properly converted to strings
	fieldMap := make(map[string]string)
	for _, field := range entry.Fields {
		fieldMap[field.Key] = field.Value
	}
	
	if fieldMap["string_field"] != "test_string" {
		t.Errorf("Expected string_field 'test_string', got '%s'", fieldMap["string_field"])
	}
	if fieldMap["int_field"] != "42" {
		t.Errorf("Expected int_field '42', got '%s'", fieldMap["int_field"])
	}
	if fieldMap["float_field"] != "3.14" {
		t.Errorf("Expected float_field '3.14', got '%s'", fieldMap["float_field"])
	}
	if fieldMap["bool_field"] != "true" {
		t.Errorf("Expected bool_field 'true', got '%s'", fieldMap["bool_field"])
	}
	if fieldMap["nil_field"] != "<nil>" {
		t.Errorf("Expected nil_field '<nil>', got '%s'", fieldMap["nil_field"])
	}
}

// Benchmark tests
func BenchmarkXMLFormatter_Format(b *testing.B) {
	formatter := &XMLFormatter{
		includeFields: true,
		timeFormat:    time.RFC3339,
		rootElement:   "logEntry",
	}
	
	testTime := time.Now()
	msg := flexlog.LogMessage{
		Level:     flexlog.LevelInfo,
		Timestamp: testTime,
		Entry: &flexlog.LogEntry{
			Message: "Benchmark test message",
			Fields: map[string]interface{}{
				"benchmark": true,
				"iteration": 0,
				"timestamp": testTime.Unix(),
			},
		},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := formatter.Format(msg)
		if err != nil {
			b.Fatalf("Format failed: %v", err)
		}
	}
}

func BenchmarkXMLFormatter_FormatNoFields(b *testing.B) {
	formatter := &XMLFormatter{
		includeFields: false,
		timeFormat:    time.RFC3339,
		rootElement:   "logEntry",
	}
	
	testTime := time.Now()
	msg := flexlog.LogMessage{
		Level:     flexlog.LevelInfo,
		Timestamp: testTime,
		Entry: &flexlog.LogEntry{
			Message: "Benchmark test message without fields",
		},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := formatter.Format(msg)
		if err != nil {
			b.Fatalf("Format failed: %v", err)
		}
	}
}