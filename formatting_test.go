package flexlog_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
	
	"github.com/wayneeseguin/flexlog"
)

func TestSetGetFormat(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := flexlog.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()
	
	// Test initial format (should be text by default)
	if format := logger.GetFormat(); format != flexlog.FormatText {
		t.Errorf("Expected default format %d, got %d", flexlog.FormatText, format)
	}
	
	// Test setting JSON format
	logger.SetFormat(flexlog.FormatJSON)
	if format := logger.GetFormat(); format != flexlog.FormatJSON {
		t.Errorf("Expected format %d, got %d", flexlog.FormatJSON, format)
	}
	
	// Test setting back to text format
	logger.SetFormat(flexlog.FormatText)
	if format := logger.GetFormat(); format != flexlog.FormatText {
		t.Errorf("Expected format %d, got %d", flexlog.FormatText, format)
	}
}

func TestSetFormatOption(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := flexlog.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()
	
	tests := []struct {
		name        string
		option      flexlog.FormatOption
		value       interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid timestamp format",
			option:      flexlog.FormatOptionTimestampFormat,
			value:       "2006-01-02",
			expectError: false,
		},
		{
			name:        "invalid timestamp format type",
			option:      flexlog.FormatOptionTimestampFormat,
			value:       123,
			expectError: true,
			errorMsg:    "timestamp format must be a string",
		},
		{
			name:        "valid include level",
			option:      flexlog.FormatOptionIncludeLevel,
			value:       false,
			expectError: false,
		},
		{
			name:        "invalid include level type",
			option:      flexlog.FormatOptionIncludeLevel,
			value:       "true",
			expectError: true,
			errorMsg:    "include level must be a boolean",
		},
		{
			name:        "valid level format",
			option:      flexlog.FormatOptionLevelFormat,
			value:       flexlog.LevelFormatNameLower,
			expectError: false,
		},
		{
			name:        "invalid level format type",
			option:      flexlog.FormatOptionLevelFormat,
			value:       "lower",
			expectError: true,
			errorMsg:    "level format must be a LevelFormat",
		},
		{
			name:        "valid indent JSON",
			option:      flexlog.FormatOptionIndentJSON,
			value:       true,
			expectError: false,
		},
		{
			name:        "invalid indent JSON type",
			option:      flexlog.FormatOptionIndentJSON,
			value:       1,
			expectError: true,
			errorMsg:    "indent JSON must be a boolean",
		},
		{
			name:        "valid field separator",
			option:      flexlog.FormatOptionFieldSeparator,
			value:       " | ",
			expectError: false,
		},
		{
			name:        "invalid field separator type",
			option:      flexlog.FormatOptionFieldSeparator,
			value:       []byte(" | "),
			expectError: true,
			errorMsg:    "field separator must be a string",
		},
		{
			name:        "valid time zone",
			option:      flexlog.FormatOptionTimeZone,
			value:       time.UTC,
			expectError: false,
		},
		{
			name:        "invalid time zone type",
			option:      flexlog.FormatOptionTimeZone,
			value:       "UTC",
			expectError: true,
			errorMsg:    "time zone must be a *time.Location",
		},
		{
			name:        "valid include time",
			option:      flexlog.FormatOptionIncludeTime,
			value:       false,
			expectError: false,
		},
		{
			name:        "invalid include time type",
			option:      flexlog.FormatOptionIncludeTime,
			value:       0,
			expectError: true,
			errorMsg:    "include time must be a boolean",
		},
		{
			name:        "unknown format option",
			option:      flexlog.FormatOption(9999),
			value:       "test",
			expectError: true,
			errorMsg:    "unknown format option",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := logger.SetFormatOption(tt.option, tt.value)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestGetFormatOption(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := flexlog.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()
	
	// Set some options
	customFormat := "15:04:05"
	logger.SetFormatOption(flexlog.FormatOptionTimestampFormat, customFormat)
	logger.SetFormatOption(flexlog.FormatOptionIncludeLevel, false)
	logger.SetFormatOption(flexlog.FormatOptionLevelFormat, flexlog.LevelFormatSymbol)
	logger.SetFormatOption(flexlog.FormatOptionIndentJSON, true)
	logger.SetFormatOption(flexlog.FormatOptionFieldSeparator, " | ")
	logger.SetFormatOption(flexlog.FormatOptionTimeZone, time.UTC)
	logger.SetFormatOption(flexlog.FormatOptionIncludeTime, false)
	
	// Test getting options
	if val := logger.GetFormatOption(flexlog.FormatOptionTimestampFormat); val != customFormat {
		t.Errorf("Expected timestamp format %q, got %v", customFormat, val)
	}
	
	if val := logger.GetFormatOption(flexlog.FormatOptionIncludeLevel); val != false {
		t.Errorf("Expected include level false, got %v", val)
	}
	
	if val := logger.GetFormatOption(flexlog.FormatOptionLevelFormat); val != flexlog.LevelFormatSymbol {
		t.Errorf("Expected level format %v, got %v", flexlog.LevelFormatSymbol, val)
	}
	
	if val := logger.GetFormatOption(flexlog.FormatOptionIndentJSON); val != true {
		t.Errorf("Expected indent JSON true, got %v", val)
	}
	
	if val := logger.GetFormatOption(flexlog.FormatOptionFieldSeparator); val != " | " {
		t.Errorf("Expected field separator %q, got %v", " | ", val)
	}
	
	if val := logger.GetFormatOption(flexlog.FormatOptionTimeZone); val != time.UTC {
		t.Errorf("Expected time zone UTC, got %v", val)
	}
	
	if val := logger.GetFormatOption(flexlog.FormatOptionIncludeTime); val != false {
		t.Errorf("Expected include time false, got %v", val)
	}
	
	// Test unknown option
	if val := logger.GetFormatOption(flexlog.FormatOption(9999)); val != nil {
		t.Errorf("Expected nil for unknown option, got %v", val)
	}
}

func TestFormatOptionEffects(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(*flexlog.FlexLog)
		logMessage    string
		expectPattern string
	}{
		{
			name: "no time, no level",
			setupFunc: func(logger *flexlog.FlexLog) {
				logger.SetFormatOption(flexlog.FormatOptionIncludeTime, false)
				logger.SetFormatOption(flexlog.FormatOptionIncludeLevel, false)
			},
			logMessage:    "test message",
			expectPattern: "^test message\n$",
		},
		{
			name: "with time, no level",
			setupFunc: func(logger *flexlog.FlexLog) {
				logger.SetFormatOption(flexlog.FormatOptionIncludeTime, true)
				logger.SetFormatOption(flexlog.FormatOptionIncludeLevel, false)
			},
			logMessage:    "test message",
			expectPattern: `^\[\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}\] test message\n$`,
		},
		{
			name: "no time, with level",
			setupFunc: func(logger *flexlog.FlexLog) {
				logger.SetFormatOption(flexlog.FormatOptionIncludeTime, false)
				logger.SetFormatOption(flexlog.FormatOptionIncludeLevel, true)
			},
			logMessage:    "test message",
			expectPattern: `^\[INFO\] test message\n$`,
		},
		{
			name: "level format lower",
			setupFunc: func(logger *flexlog.FlexLog) {
				logger.SetFormatOption(flexlog.FormatOptionIncludeTime, false)
				logger.SetFormatOption(flexlog.FormatOptionIncludeLevel, true)
				logger.SetFormatOption(flexlog.FormatOptionLevelFormat, flexlog.LevelFormatNameLower)
			},
			logMessage:    "test message",
			expectPattern: `^\[info\] test message\n$`,
		},
		{
			name: "level format symbol",
			setupFunc: func(logger *flexlog.FlexLog) {
				logger.SetFormatOption(flexlog.FormatOptionIncludeTime, false)
				logger.SetFormatOption(flexlog.FormatOptionIncludeLevel, true)
				logger.SetFormatOption(flexlog.FormatOptionLevelFormat, flexlog.LevelFormatSymbol)
			},
			logMessage:    "test message",
			expectPattern: `^\[I\] test message\n$`,
		},
		{
			name: "custom timestamp format",
			setupFunc: func(logger *flexlog.FlexLog) {
				logger.SetFormatOption(flexlog.FormatOptionIncludeTime, true)
				logger.SetFormatOption(flexlog.FormatOptionIncludeLevel, false)
				logger.SetFormatOption(flexlog.FormatOptionTimestampFormat, "15:04:05")
			},
			logMessage:    "test message",
			expectPattern: `^\[\d{2}:\d{2}:\d{2}\] test message\n$`,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempFile := filepath.Join(t.TempDir(), "test.log")
			logger, err := flexlog.New(tempFile)
			if err != nil {
				t.Fatalf("Failed to create logger: %v", err)
			}
			
			// Apply setup
			tt.setupFunc(logger)
			
			// Log message
			logger.Info(tt.logMessage)
			logger.Sync()
			logger.CloseAll()
			
			// Read and check
			content, err := readFile(tempFile)
			if err != nil {
				t.Fatalf("Failed to read log file: %v", err)
			}
			
			if !matchesPattern(content, tt.expectPattern) {
				t.Errorf("Log output %q does not match expected pattern %q", content, tt.expectPattern)
			}
		})
	}
}

func TestJSONFormat(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := flexlog.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	
	// Set JSON format
	logger.SetFormat(flexlog.FormatJSON)
	
	// Log a structured entry
	logger.StructuredLog(flexlog.LevelInfo, "test message", map[string]interface{}{
		"user":   "admin",
		"action": "login",
		"status": 200,
	})
	
	logger.Sync()
	logger.CloseAll()
	
	// Read and check
	content, err := readFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	
	// Should contain JSON with the message
	if !strings.Contains(content, "test message") {
		t.Errorf("JSON output should contain the message, got: %s", content)
	}
	
	// Should be valid JSON structure
	if !strings.HasPrefix(content, "{") || !strings.Contains(content, "}") {
		t.Errorf("Output should be JSON format, got: %s", content)
	}
}

func TestTimeZoneFormatting(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := flexlog.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	
	// Load NYC timezone
	nyc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("Could not load NYC timezone")
	}
	
	// Set UTC timezone first
	logger.SetFormatOption(flexlog.FormatOptionTimeZone, time.UTC)
	logger.SetFormatOption(flexlog.FormatOptionTimestampFormat, "15:04:05 MST")
	logger.SetFormatOption(flexlog.FormatOptionIncludeLevel, false)
	
	logger.Info("UTC message")
	logger.Sync()
	
	// Switch to NYC timezone
	logger.SetFormatOption(flexlog.FormatOptionTimeZone, nyc)
	logger.Info("NYC message")
	logger.Sync()
	logger.CloseAll()
	
	// Read and check
	content, err := readFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	
	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) != 2 {
		t.Fatalf("Expected 2 log lines, got %d", len(lines))
	}
	
	// First line should have UTC
	if !strings.Contains(lines[0], "UTC") {
		t.Errorf("First line should contain UTC timezone, got: %s", lines[0])
	}
	
	// Second line should have EST or EDT
	if !strings.Contains(lines[1], "EST") && !strings.Contains(lines[1], "EDT") {
		t.Errorf("Second line should contain NYC timezone, got: %s", lines[1])
	}
}

func TestGetFormatOptions(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := flexlog.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()
	
	// Get default options
	opts := logger.GetFormatOptions()
	
	// Check defaults
	if opts.TimestampFormat != "2006-01-02 15:04:05.000" {
		t.Errorf("Default timestamp format incorrect: %s", opts.TimestampFormat)
	}
	
	if !opts.IncludeLevel {
		t.Error("Default should include level")
	}
	
	if !opts.IncludeTime {
		t.Error("Default should include time")
	}
	
	if opts.LevelFormat != flexlog.LevelFormatNameUpper {
		t.Errorf("Default level format should be upper case, got %v", opts.LevelFormat)
	}
	
	if opts.IndentJSON {
		t.Error("Default should not indent JSON")
	}
	
	if opts.FieldSeparator != " " {
		t.Errorf("Default field separator should be space, got %q", opts.FieldSeparator)
	}
	
	if opts.TimeZone != time.Local {
		t.Error("Default timezone should be Local")
	}
	
	// Modify options
	logger.SetFormatOption(flexlog.FormatOptionIncludeLevel, false)
	logger.SetFormatOption(flexlog.FormatOptionIndentJSON, true)
	
	// Get options again
	opts2 := logger.GetFormatOptions()
	
	if opts2.IncludeLevel {
		t.Error("Include level should be false after setting")
	}
	
	if !opts2.IndentJSON {
		t.Error("Indent JSON should be true after setting")
	}
}

func TestStructuredLogFormatting(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := flexlog.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	
	// Test text format with fields
	logger.SetFormat(flexlog.FormatText)
	logger.StructuredLog(flexlog.LevelInfo, "user action", map[string]interface{}{
		"user":   "alice",
		"action": "login",
		"ip":     "192.168.1.1",
	})
	
	logger.Sync()
	logger.CloseAll()
	
	// Read and check
	content, err := readFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	
	// Should contain the message and fields
	if !strings.Contains(content, "user action") {
		t.Errorf("Should contain message, got: %s", content)
	}
	
	// Should contain all fields
	if !strings.Contains(content, "user=alice") {
		t.Errorf("Should contain user field, got: %s", content)
	}
	
	if !strings.Contains(content, "action=login") {
		t.Errorf("Should contain action field, got: %s", content)
	}
	
	if !strings.Contains(content, "ip=192.168.1.1") {
		t.Errorf("Should contain ip field, got: %s", content)
	}
}

// Helper functions
func readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	return string(data), err
}

func matchesPattern(text, pattern string) bool {
	match, _ := regexp.MatchString(pattern, text)
	return match
}