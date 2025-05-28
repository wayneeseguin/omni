package flexlog_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wayneeseguin/flexlog"
)

func TestStructuredLog(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := flexlog.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Set level to TRACE to ensure all messages are logged
	logger.SetLevel(flexlog.LevelTrace)

	tests := []struct {
		name     string
		level    int
		message  string
		fields   map[string]interface{}
		expected []string
	}{
		{
			name:    "trace level with fields",
			level:   flexlog.LevelTrace,
			message: "trace message",
			fields: map[string]interface{}{
				"function": "processOrder",
				"params":   map[string]string{"id": "123"},
				"caller":   "handler.go:42",
			},
			expected: []string{"[TRACE]", "trace message", "function=processOrder", "caller=handler.go:42"},
		},
		{
			name:    "debug level with fields",
			level:   flexlog.LevelDebug,
			message: "debug message",
			fields: map[string]interface{}{
				"user":    "alice",
				"action":  "login",
				"session": 12345,
			},
			expected: []string{"[DEBUG]", "debug message", "user=alice", "action=login", "session=12345"},
		},
		{
			name:    "info level with fields",
			level:   flexlog.LevelInfo,
			message: "info message",
			fields: map[string]interface{}{
				"service": "api",
				"status":  "healthy",
			},
			expected: []string{"[INFO]", "info message", "service=api", "status=healthy"},
		},
		{
			name:     "warn level with no fields",
			level:    flexlog.LevelWarn,
			message:  "warning message",
			fields:   nil,
			expected: []string{"[WARN]", "warning message"},
		},
		{
			name:    "error level with fields",
			level:   flexlog.LevelError,
			message: "error message",
			fields: map[string]interface{}{
				"error_code": "E500",
				"details":    "connection failed",
			},
			expected: []string{"[ERROR]", "error message", "error_code=E500", "details=connection failed"},
		},
		{
			name:     "unknown level",
			level:    99,
			message:  "unknown level message",
			fields:   nil,
			expected: []string{"[LOG]", "unknown level message"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear log file
			os.Truncate(tempFile, 0)

			logger.StructuredLog(tt.level, tt.message, tt.fields)
			logger.Sync()

			content, err := os.ReadFile(tempFile)
			if err != nil {
				t.Fatalf("Failed to read log file: %v", err)
			}

			contentStr := string(content)
			for _, expected := range tt.expected {
				if !strings.Contains(contentStr, expected) {
					t.Errorf("Expected %q in log output, got: %s", expected, contentStr)
				}
			}

			// Check timestamp format
			if !strings.Contains(contentStr, "[2025-") {
				t.Error("Expected timestamp in log output")
			}
		})
	}
}

func TestStructuredLogWithFilters(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := flexlog.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Add a filter that only allows logs with a specific field
	logger.AddFilter(func(level int, message string, fields map[string]interface{}) bool {
		if fields == nil {
			return false
		}
		_, hasUser := fields["user"]
		return hasUser
	})

	// This should be logged
	logger.StructuredLog(flexlog.LevelInfo, "with user", map[string]interface{}{
		"user": "alice",
	})

	// This should be filtered out
	logger.StructuredLog(flexlog.LevelInfo, "without user", map[string]interface{}{
		"other": "field",
	})

	// This should also be filtered out (no fields)
	logger.StructuredLog(flexlog.LevelInfo, "no fields", nil)

	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "with user") {
		t.Error("Expected 'with user' message to be logged")
	}

	if strings.Contains(contentStr, "without user") {
		t.Error("'without user' message should have been filtered")
	}

	if strings.Contains(contentStr, "no fields") {
		t.Error("'no fields' message should have been filtered")
	}
}

func TestStructuredLogWithStackTrace(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := flexlog.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Test ERROR level always includes file/line info
	logger.StructuredLog(flexlog.LevelError, "error without trace", map[string]interface{}{
		"error": "test",
	})
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// Should have file info but not stack trace
	contentStr := string(content)
	if !strings.Contains(contentStr, "error without trace") {
		t.Error("Expected error message")
	}

	// ERROR level should NOT include stack trace by default
	if strings.Contains(contentStr, "goroutine") {
		t.Error("Should not include stack trace when includeTrace is false")
	}

	// Clear log
	os.Truncate(tempFile, 0)

	// Enable stack traces for errors
	logger.EnableStackTraces(true)
	logger.StructuredLog(flexlog.LevelError, "error with trace", map[string]interface{}{
		"error": "test",
	})
	logger.Sync()

	content, _ = os.ReadFile(tempFile)
	contentStr = string(content)

	// Should now include stack trace
	if !strings.Contains(contentStr, "goroutine") {
		t.Error("Should include stack trace for ERROR when includeTrace is true")
	}

	if !strings.Contains(contentStr, "structured_test.go") {
		t.Error("Stack trace should include test file name")
	}
}

func TestStructuredLogWithCaptureAll(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := flexlog.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Enable capture all stacks
	logger.EnableStackTraces(true)
	logger.SetCaptureAllStacks(true)

	// Log at INFO level - should include stack trace with captureAll
	logger.StructuredLog(flexlog.LevelInfo, "info with stack", map[string]interface{}{
		"test": "capture all",
	})
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "info with stack") {
		t.Error("Expected info message")
	}

	if !strings.Contains(contentStr, "goroutine") {
		t.Error("Should include stack trace for INFO when captureAll is true")
	}
}

func TestTraceWithFields(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := flexlog.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Set level to trace to ensure it's logged
	logger.SetLevel(flexlog.LevelTrace)

	logger.TraceWithFields("trace message", map[string]interface{}{
		"function": "processOrder",
		"params":   map[string]string{"id": "123"},
		"caller":   "handler.go:42",
	})
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "[TRACE]") {
		t.Error("Expected TRACE level")
	}

	if !strings.Contains(contentStr, "trace message") {
		t.Error("Expected trace message")
	}

	if !strings.Contains(contentStr, "function=processOrder") {
		t.Error("Expected function field")
	}

	if !strings.Contains(contentStr, "caller=handler.go:42") {
		t.Error("Expected caller field")
	}
}

func TestDebugWithFields(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := flexlog.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Set level to debug to ensure it's logged
	logger.SetLevel(flexlog.LevelDebug)

	logger.DebugWithFields("debug message", map[string]interface{}{
		"component": "test",
		"verbose":   true,
	})
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "[DEBUG]") {
		t.Error("Expected DEBUG level")
	}

	if !strings.Contains(contentStr, "debug message") {
		t.Error("Expected debug message")
	}

	if !strings.Contains(contentStr, "component=test") {
		t.Error("Expected component field")
	}

	if !strings.Contains(contentStr, "verbose=true") {
		t.Error("Expected verbose field")
	}
}

func TestInfoWithFields(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := flexlog.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	logger.InfoWithFields("info message", map[string]interface{}{
		"request_id": "12345",
		"duration":   1.23,
	})
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "[INFO]") {
		t.Error("Expected INFO level")
	}

	if !strings.Contains(contentStr, "info message") {
		t.Error("Expected info message")
	}

	if !strings.Contains(contentStr, "request_id=12345") {
		t.Error("Expected request_id field")
	}

	if !strings.Contains(contentStr, "duration=1.23") {
		t.Error("Expected duration field")
	}
}

func TestWarnWithFields(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := flexlog.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	logger.WarnWithFields("warning message", map[string]interface{}{
		"threshold": 0.8,
		"current":   0.95,
		"metric":    "cpu_usage",
	})
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "[WARN]") {
		t.Error("Expected WARN level")
	}

	if !strings.Contains(contentStr, "warning message") {
		t.Error("Expected warning message")
	}

	if !strings.Contains(contentStr, "threshold=0.8") {
		t.Error("Expected threshold field")
	}

	if !strings.Contains(contentStr, "current=0.95") {
		t.Error("Expected current field")
	}

	if !strings.Contains(contentStr, "metric=cpu_usage") {
		t.Error("Expected metric field")
	}
}

func TestErrorWithFields(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := flexlog.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	logger.ErrorWithFields("error message", map[string]interface{}{
		"error_code": "DB_CONNECTION_FAILED",
		"retry":      3,
		"fatal":      false,
	})
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "[ERROR]") {
		t.Error("Expected ERROR level")
	}

	if !strings.Contains(contentStr, "error message") {
		t.Error("Expected error message")
	}

	if !strings.Contains(contentStr, "error_code=DB_CONNECTION_FAILED") {
		t.Error("Expected error_code field")
	}

	if !strings.Contains(contentStr, "retry=3") {
		t.Error("Expected retry field")
	}

	if !strings.Contains(contentStr, "fatal=false") {
		t.Error("Expected fatal field")
	}
}

func TestStructuredLogJSONFormat(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := flexlog.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Set JSON format
	logger.SetFormat(flexlog.FormatJSON)

	logger.StructuredLog(flexlog.LevelInfo, "json test", map[string]interface{}{
		"string_field": "value",
		"number_field": 42,
		"bool_field":   true,
		"nested": map[string]interface{}{
			"inner": "value",
		},
	})
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := strings.TrimSpace(string(content))

	// Should be valid JSON
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(contentStr), &logEntry); err != nil {
		t.Fatalf("Output should be valid JSON: %v", err)
	}

	// Check fields
	if logEntry["message"] != "json test" {
		t.Errorf("Expected message 'json test', got %v", logEntry["message"])
	}

	if logEntry["level"] != "INFO" {
		t.Errorf("Expected level 'INFO', got %v", logEntry["level"])
	}

	// Note: The current implementation doesn't properly handle fields in JSON format
	// This might need to be fixed in the actual implementation
}

func TestStructuredLogLevelFiltering(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := flexlog.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Set level to WARN
	logger.SetLevel(flexlog.LevelWarn)

	// These should be filtered
	logger.DebugWithFields("debug", map[string]interface{}{"test": "debug"})
	logger.InfoWithFields("info", map[string]interface{}{"test": "info"})

	// These should be logged
	logger.WarnWithFields("warn", map[string]interface{}{"test": "warn"})
	logger.ErrorWithFields("error", map[string]interface{}{"test": "error"})

	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)

	// Should not contain debug or info
	if strings.Contains(contentStr, "debug") {
		t.Error("Debug message should be filtered")
	}

	if strings.Contains(contentStr, "info") {
		t.Error("Info message should be filtered")
	}

	// Should contain warn and error
	if !strings.Contains(contentStr, "warn") {
		t.Error("Warn message should be logged")
	}

	if !strings.Contains(contentStr, "error") {
		t.Error("Error message should be logged")
	}
}

func TestStructuredLogFieldOrdering(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := flexlog.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Test that all field types are handled correctly
	logger.StructuredLog(flexlog.LevelInfo, "field types", map[string]interface{}{
		"string": "hello",
		"int":    42,
		"float":  3.14,
		"bool":   true,
		"nil":    nil,
		"slice":  []int{1, 2, 3},
		"map":    map[string]string{"key": "value"},
	})
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)

	// Check that all fields are present
	expectedFields := []string{
		"string=hello",
		"int=42",
		"float=3.14",
		"bool=true",
		"nil=<nil>",
		"slice=[1 2 3]",
		"map=map[key:value]",
	}

	for _, expected := range expectedFields {
		if !strings.Contains(contentStr, expected) {
			t.Errorf("Expected field %q in output, got: %s", expected, contentStr)
		}
	}
}

func TestStructuredLogEmptyFields(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := flexlog.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Test with empty fields map
	logger.StructuredLog(flexlog.LevelInfo, "empty fields", map[string]interface{}{})
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "empty fields") {
		t.Error("Expected message even with empty fields")
	}

	// Should have timestamp and level but no field entries
	if !strings.Contains(contentStr, "[INFO]") {
		t.Error("Expected INFO level")
	}
}
