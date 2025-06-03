package omni

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLogWithContext(t *testing.T) {
	// Create logger
	tmpDir := t.TempDir()
	logPath := tmpDir + "/test.log"
	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test successful logging
	ctx := context.Background()
	err = logger.LogWithContext(ctx, LevelInfo, "Test message %d", 123)
	if err != nil {
		t.Errorf("LogWithContext failed: %v", err)
	}

	// Wait for message to be processed
	logger.Sync()

	// Verify message was logged
	content := readFile(t, logPath)
	if !strings.Contains(content, "Test message 123") {
		t.Errorf("Expected message not found in log")
	}
}

func TestLogWithContextCancellation(t *testing.T) {
	// Create logger
	tmpDir := t.TempDir()
	logPath := tmpDir + "/test.log"
	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Try to log with cancelled context
	err = logger.LogWithContext(ctx, LevelInfo, "Should not log")
	if err == nil {
		t.Error("Expected error from cancelled context")
	}
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

func TestLogWithContextTimeout(t *testing.T) {
	// Create logger with small channel
	tmpDir := t.TempDir()
	logPath := tmpDir + "/test.log"

	// Create logger with channel size 0 to ensure blocking
	logger := &Omni{
		msgChan:       make(chan LogMessage, 0),
		formatOptions: defaultFormatOptions(),
		level:         LevelInfo,
	}

	// Set up minimal required fields
	logger.path = logPath
	logger.Destinations = []*Destination{}

	// Try to log with immediate timeout
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := logger.LogWithContext(ctx, LevelInfo, "Should fail")
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

func TestStructuredLogWithContext(t *testing.T) {
	// Create logger
	tmpDir := t.TempDir()
	logPath := tmpDir + "/test.log"
	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	logger.SetFormat(FormatJSON)

	// Create context with values
	ctx := context.WithValue(context.Background(), ContextKeyRequestID, "req-123")
	ctx = context.WithValue(ctx, ContextKeyTraceID, "trace-456")

	// Log with context
	fields := map[string]interface{}{
		"user_id": 42,
		"action":  "login",
	}
	t.Logf("Fields before logging: %v", fields)
	err = logger.StructuredLogWithContext(ctx, LevelInfo, "User logged in", fields)
	t.Logf("Fields after logging: %v", fields)
	if err != nil {
		t.Errorf("StructuredLogWithContext failed: %v", err)
	}

	// Wait for message to be processed
	logger.Sync()

	// Verify message was logged with context values
	content := readFile(t, logPath)
	t.Logf("Log content: %s", content)
	if !strings.Contains(content, "\"request_id\":\"req-123\"") {
		t.Error("Request ID from context not found in log")
	}
	if !strings.Contains(content, "\"trace_id\":\"trace-456\"") {
		t.Error("Trace ID from context not found in log")
	}
	if !strings.Contains(content, "\"user_id\":42") {
		t.Error("User ID field not found in log")
	}
}

func TestLevelMethodsWithContext(t *testing.T) {
	// Create logger
	tmpDir := t.TempDir()
	logPath := tmpDir + "/test.log"
	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	ctx := context.Background()

	// Test all level methods
	tests := []struct {
		name   string
		fn     func(context.Context, string, ...interface{}) error
		msg    string
		expect string
	}{
		{"DebugWithContext", logger.DebugWithContext, "Debug message", "[DEBUG]"},
		{"InfoWithContext", logger.InfoWithContext, "Info message", "[INFO]"},
		{"WarnWithContext", logger.WarnWithContext, "Warn message", "[WARN]"},
		{"ErrorWithContext", logger.ErrorWithContext, "Error message", "[ERROR]"},
	}

	// Set level to debug to capture all messages
	logger.SetLevel(LevelDebug)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn(ctx, tt.msg)
			if err != nil {
				t.Errorf("%s failed: %v", tt.name, err)
			}
		})
	}

	// Wait for messages to be processed
	logger.Sync()

	// Verify all messages were logged
	content := readFile(t, logPath)
	for _, tt := range tests {
		if !strings.Contains(content, tt.msg) {
			t.Errorf("Expected message '%s' not found", tt.msg)
		}
		if !strings.Contains(content, tt.expect) {
			t.Errorf("Expected level '%s' not found", tt.expect)
		}
	}
}

func TestCloseWithContext(t *testing.T) {
	// Create logger
	tmpDir := t.TempDir()
	logPath := tmpDir + "/test.log"
	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Close with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err = logger.CloseWithContext(ctx)
	if err != nil {
		t.Errorf("CloseWithContext failed: %v", err)
	}

	// Verify logger is closed
	if !logger.IsClosed() {
		t.Error("Logger should be closed")
	}
}

func TestFlushWithContext(t *testing.T) {
	// Create logger
	tmpDir := t.TempDir()
	logPath := tmpDir + "/test.log"
	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Log a message
	logger.Info("Test message")

	// Give the message time to reach the channel
	time.Sleep(50 * time.Millisecond)

	// Flush with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err = logger.FlushWithContext(ctx)
	if err != nil {
		t.Errorf("FlushWithContext failed: %v", err)
	}

	// Verify message was flushed
	content := readFile(t, logPath)
	if !strings.Contains(content, "Test message") {
		t.Error("Message not found after flush")
	}
}

// readFile is a helper function to read file contents
func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", path, err)
	}
	return string(content)
}

func TestContextValueExtraction(t *testing.T) {
	// Create logger
	tmpDir := t.TempDir()
	logPath := tmpDir + "/test.log"
	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()
	logger.SetFormat(FormatJSON)

	// Create context with values
	ctx := context.Background()
	ctx = WithRequestID(ctx, "req-123")
	ctx = WithTraceID(ctx, "trace-456")
	ctx = WithUserID(ctx, "user-789")
	ctx = context.WithValue(ctx, ContextKeySessionID, "session-abc")
	ctx = context.WithValue(ctx, ContextKeyTenantID, "tenant-xyz")

	// Log with context
	err = logger.StructuredLogWithContext(ctx, LevelInfo, "Test with context", nil)
	if err != nil {
		t.Errorf("StructuredLogWithContext failed: %v", err)
	}

	// Wait for processing
	logger.Sync()

	// Verify all context values were extracted
	content := readFile(t, logPath)
	expectedValues := []string{
		`"request_id":"req-123"`,
		`"trace_id":"trace-456"`,
		`"user_id":"user-789"`,
		`"session_id":"session-abc"`,
		`"tenant_id":"tenant-xyz"`,
	}

	for _, expected := range expectedValues {
		if !strings.Contains(content, expected) {
			t.Errorf("Expected context value %s not found in log", expected)
		}
	}
}

func TestContextLogger(t *testing.T) {
	// Create logger
	tmpDir := t.TempDir()
	logPath := tmpDir + "/test.log"
	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()
	logger.SetFormat(FormatJSON)

	// Create context with values
	ctx := context.Background()
	ctx = WithRequestID(ctx, "req-999")
	ctx = WithUserID(ctx, "user-123")

	// Create context logger
	ctxLogger := NewContextLogger(logger, ctx)
	
	// Add fields
	ctxLogger = ctxLogger.WithField("app", "test-app").(*ContextLogger)
	ctxLogger = ctxLogger.WithFields(map[string]interface{}{
		"version": "1.0.0",
		"env":     "test",
	}).(*ContextLogger)

	// Log messages
	ctxLogger.Info("Info message")
	ctxLogger.Debug("Debug message")
	ctxLogger.Warn("Warning message")
	ctxLogger.Error("Error message")

	// Wait for processing
	logger.Sync()

	// Verify messages and fields
	content := readFile(t, logPath)
	
	// Check that all messages were logged
	if !strings.Contains(content, "Info message") {
		t.Error("Info message not found")
	}
	if !strings.Contains(content, "Warning message") {
		t.Error("Warning message not found")
	}
	if !strings.Contains(content, "Error message") {
		t.Error("Error message not found")
	}

	// Check that context values and fields are included
	expectedValues := []string{
		`"request_id":"req-999"`,
		`"user_id":"user-123"`,
		`"app":"test-app"`,
		`"version":"1.0.0"`,
		`"env":"test"`,
	}

	for _, expected := range expectedValues {
		if !strings.Contains(content, expected) {
			t.Errorf("Expected field %s not found in log", expected)
		}
	}
}

func TestContextHelpers(t *testing.T) {
	// Test WithContextFields
	ctx := context.Background()
	fields := map[string]interface{}{
		"field1": "value1",
		"field2": 123,
		"field3": true,
	}
	ctx = WithContextFields(ctx, fields)

	// Verify values were set
	for key, expectedValue := range fields {
		if value := ctx.Value(ContextKey(key)); value != expectedValue {
			t.Errorf("Expected %s=%v, got %v", key, expectedValue, value)
		}
	}

	// Test GetRequestID
	ctx = WithRequestID(ctx, "req-test")
	if id, ok := GetRequestID(ctx); !ok || id != "req-test" {
		t.Errorf("GetRequestID failed, got %s, %v", id, ok)
	}

	// Test GetTraceID
	ctx = WithTraceID(ctx, "trace-test")
	if id, ok := GetTraceID(ctx); !ok || id != "trace-test" {
		t.Errorf("GetTraceID failed, got %s, %v", id, ok)
	}

	// Test with missing values
	emptyCtx := context.Background()
	if _, ok := GetRequestID(emptyCtx); ok {
		t.Error("GetRequestID should return false for missing value")
	}
	if _, ok := GetTraceID(emptyCtx); ok {
		t.Error("GetTraceID should return false for missing value")
	}
}

func TestStructuredLogWithContextFields(t *testing.T) {
	// Create logger
	tmpDir := t.TempDir()
	logPath := tmpDir + "/test.log"
	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()
	logger.SetFormat(FormatJSON)

	// Create context with multiple values
	ctx := context.Background()
	ctx = context.WithValue(ctx, ContextKeyRequestID, "req-001")
	ctx = context.WithValue(ctx, ContextKeyTraceID, "trace-001")
	ctx = context.WithValue(ctx, ContextKeySpanID, "span-001")
	ctx = context.WithValue(ctx, ContextKeyOperation, "test-operation")
	ctx = context.WithValue(ctx, ContextKeyService, "test-service")
	ctx = context.WithValue(ctx, ContextKeyVersion, "v1.2.3")

	// Log with additional fields
	fields := map[string]interface{}{
		"custom_field": "custom_value",
		"numeric":      42,
	}

	err = logger.StructuredLogWithContext(ctx, LevelInfo, "Structured message", fields)
	if err != nil {
		t.Errorf("StructuredLogWithContext failed: %v", err)
	}

	// Wait for processing
	logger.Sync()

	// Verify all values are in the log
	content := readFile(t, logPath)
	expectedValues := []string{
		`"request_id":"req-001"`,
		`"trace_id":"trace-001"`,
		`"span_id":"span-001"`,
		`"operation":"test-operation"`,
		`"service":"test-service"`,
		`"version":"v1.2.3"`,
		`"custom_field":"custom_value"`,
		`"numeric":42`,
	}

	for _, expected := range expectedValues {
		if !strings.Contains(content, expected) {
			t.Errorf("Expected value %s not found in log", expected)
		}
	}
}
