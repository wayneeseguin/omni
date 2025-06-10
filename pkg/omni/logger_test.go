package omni

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wayneeseguin/omni/pkg/types"
)

// TestNew tests creating a new logger
func TestNew(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Verify logger was created with defaults
	if logger.GetLevel() != LevelInfo {
		t.Errorf("Expected default level to be Info, got %d", logger.GetLevel())
	}

	if logger.GetMaxSize() != defaultMaxSize {
		t.Errorf("Expected default max size to be %d, got %d", defaultMaxSize, logger.GetMaxSize())
	}

	// Verify the file was created
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Errorf("Log file was not created")
	}
}

// TestNewWithOptions tests creating logger with options
func TestNewWithOptions(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := NewWithBackend(logPath, BackendFlock)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test with invalid backend
	_, err = NewWithBackend(logPath, 999)
	if err == nil {
		t.Errorf("Expected error for invalid backend type")
	}
}

// TestIsClosed tests the IsClosed method
func TestIsClosed(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Initially should not be closed
	if logger.IsClosed() {
		t.Errorf("Logger should not be closed initially")
	}

	// Close the logger
	logger.Close()

	// Now should be closed
	if !logger.IsClosed() {
		t.Errorf("Logger should be closed after Close()")
	}
}

// TestSettersAndGetters tests various setter/getter methods
func TestSettersAndGetters(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test SetLevel/GetLevel
	logger.SetLevel(LevelDebug)
	if logger.GetLevel() != LevelDebug {
		t.Errorf("Expected level Debug, got %d", logger.GetLevel())
	}

	// Test SetMaxSize/GetMaxSize
	logger.SetMaxSize(1024 * 1024) // 1MB
	if logger.GetMaxSize() != 1024*1024 {
		t.Errorf("Expected max size 1MB, got %d", logger.GetMaxSize())
	}

	// Test SetMaxFiles
	logger.SetMaxFiles(10)
	// Note: No getter for maxFiles, so we can't directly test it

	// Test SetFormat/GetFormat
	logger.SetFormat(FormatJSON)
	if logger.GetFormat() != FormatJSON {
		t.Errorf("Expected format JSON, got %d", logger.GetFormat())
	}
}

// TestBasicLogging tests basic logging functionality
func TestBasicLogging(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Log some messages
	logger.Info("Test info message")
	logger.Debug("Test debug message") // Should not appear with default Info level
	logger.Warn("Test warning message")
	logger.Error("Test error message")

	// Sync to ensure messages are written
	logger.Sync()

	// Read the log file
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// Verify messages
	if !strings.Contains(string(content), "Test info message") {
		t.Errorf("Info message not found in log")
	}

	if strings.Contains(string(content), "Test debug message") {
		t.Errorf("Debug message should not appear with Info level")
	}

	if !strings.Contains(string(content), "Test warning message") {
		t.Errorf("Warning message not found in log")
	}

	if !strings.Contains(string(content), "Test error message") {
		t.Errorf("Error message not found in log")
	}
}

// TestLogAfterClose tests that logging after close doesn't panic
func TestLogAfterClose(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Close the logger
	logger.Close()

	// These should not panic
	logger.Info("This should not panic")
	logger.Error("This should not panic either")
	logger.Debug("Debug after close")
	logger.Warn("Warning after close")
}

// TestSetChannelSize tests that channel size cannot be changed
func TestSetChannelSize(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Try to change channel size - should fail
	err = logger.SetChannelSize(1000)
	if err == nil {
		t.Errorf("Expected error when changing channel size")
	}
}

// TestStructuredLogging tests structured logging
func TestStructuredLogging(t *testing.T) {

	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Log with fields
	logger.InfoWithFields("User logged in", map[string]interface{}{
		"user_id": 123,
		"action":  "login",
		"ip":      "192.168.1.1",
	})

	// Sync and read
	logger.Sync()

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// Verify the message appears
	if !strings.Contains(string(content), "User logged in") {
		t.Errorf("Structured log message not found in log")
	}
}

// TestLogWorker tests that the log worker processes messages
func TestLogWorker(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Log messages at different levels
	logger.SetLevel(LevelDebug)

	logger.Debug("Debug message")
	logger.Info("Info message")
	logger.Warn("Warning message")
	logger.Error("Error message")

	// Give worker time to process
	logger.Sync()

	// Check that all messages were logged
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	messages := []string{"Debug message", "Info message", "Warning message", "Error message"}
	for _, msg := range messages {
		if !strings.Contains(string(content), msg) {
			t.Errorf("Message '%s' not found in log", msg)
		}
	}
}

// TestConcurrentLogging tests concurrent logging
func TestConcurrentLogging(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Log from multiple goroutines
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				logger.Infof("Goroutine %d, message %d", id, j)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Sync and check
	logger.Sync()

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// Should have 100 messages
	lines := strings.Split(string(content), "\n")
	// -1 because last line is empty
	if len(lines)-1 < 100 {
		t.Errorf("Expected at least 100 log lines, got %d", len(lines)-1)
	}
}

// TestGetMaxFiles tests the GetMaxFiles method
func TestGetMaxFiles(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test default value
	if logger.GetMaxFiles() != defaultMaxFiles {
		t.Errorf("Expected default max files to be %d, got %d", defaultMaxFiles, logger.GetMaxFiles())
	}

	// Test setting and getting different values
	testValues := []int{1, 5, 10, 100}
	for _, val := range testValues {
		logger.SetMaxFiles(val)
		if logger.GetMaxFiles() != val {
			t.Errorf("Expected max files to be %d, got %d", val, logger.GetMaxFiles())
		}
	}
}

// TestGlobalFields tests global field functionality
func TestGlobalFields(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Initially should have no global fields
	fields := logger.GetGlobalFields()
	if fields != nil {
		t.Errorf("Expected no global fields initially, got %v", fields)
	}

	// Test SetGlobalFields
	globalFields := map[string]interface{}{
		"service": "test-service",
		"version": "1.0.0",
		"env":     "test",
	}
	logger.SetGlobalFields(globalFields)

	retrievedFields := logger.GetGlobalFields()
	if len(retrievedFields) != 3 {
		t.Errorf("Expected 3 global fields, got %d", len(retrievedFields))
	}

	for key, expected := range globalFields {
		if actual, exists := retrievedFields[key]; !exists || actual != expected {
			t.Errorf("Expected field %s=%v, got %v (exists: %v)", key, expected, actual, exists)
		}
	}

	// Test AddGlobalField
	logger.AddGlobalField("hostname", "test-host")
	updatedFields := logger.GetGlobalFields()
	if len(updatedFields) != 4 {
		t.Errorf("Expected 4 global fields after adding one, got %d", len(updatedFields))
	}
	if updatedFields["hostname"] != "test-host" {
		t.Errorf("Expected hostname=test-host, got %v", updatedFields["hostname"])
	}

	// Test RemoveGlobalField
	logger.RemoveGlobalField("env")
	finalFields := logger.GetGlobalFields()
	if len(finalFields) != 3 {
		t.Errorf("Expected 3 global fields after removing one, got %d", len(finalFields))
	}
	if _, exists := finalFields["env"]; exists {
		t.Errorf("Expected 'env' field to be removed, but it still exists")
	}

	// Test that returned fields are a copy (cannot be modified externally)
	fieldsBeforeModification := logger.GetGlobalFields()
	fieldsBeforeModification["external_modification"] = "should not affect logger"
	fieldsAfterModification := logger.GetGlobalFields()
	if _, exists := fieldsAfterModification["external_modification"]; exists {
		t.Errorf("External modification of returned fields should not affect logger")
	}
}

// TestWriteLogEntry tests the writeLogEntry method directly (which includes global fields)
func TestWriteLogEntry(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set JSON format to easily verify field inclusion
	logger.SetFormat(FormatJSON)

	// Set global fields
	logger.SetGlobalFields(map[string]interface{}{
		"service": "test-service",
		"version": "1.0.0",
	})

	// Create a LogEntry and call writeLogEntry directly to test the functionality
	entry := LogEntry{
		Timestamp: "2024-01-01T00:00:00Z",
		Level:     "INFO",
		Message:   "Test message",
		Fields: map[string]interface{}{
			"user_id": 123,
			"action":  "test",
		},
	}

	// Call writeLogEntry directly to test global fields merging
	logger.writeLogEntry(entry)

	// Sync and read
	logger.Sync()

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)

	// Verify global fields are included
	if !strings.Contains(logContent, "service") {
		t.Errorf("Global field 'service' not found in log output")
	}
	if !strings.Contains(logContent, "test-service") {
		t.Errorf("Global field value 'test-service' not found in log output")
	}
	if !strings.Contains(logContent, "version") {
		t.Errorf("Global field 'version' not found in log output")
	}
	if !strings.Contains(logContent, "1.0.0") {
		t.Errorf("Global field value '1.0.0' not found in log output")
	}

	// Verify entry-specific fields are also included
	if !strings.Contains(logContent, "user_id") {
		t.Errorf("Entry field 'user_id' not found in log output")
	}
	if !strings.Contains(logContent, "123") {
		t.Errorf("Entry field value '123' not found in log output")
	}
	if !strings.Contains(logContent, "action") {
		t.Errorf("Entry field 'action' not found in log output")
	}
	if !strings.Contains(logContent, "test") {
		t.Errorf("Entry field value 'test' not found in log output")
	}
}

// TestGlobalFieldsPrecedence tests that entry fields take precedence over global fields
func TestGlobalFieldsPrecedence(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	logger.SetFormat(FormatJSON)

	// Set global field
	logger.SetGlobalFields(map[string]interface{}{
		"env": "production",
	})

	// Create entry with field that conflicts with global field
	entry := LogEntry{
		Timestamp: "2024-01-01T00:00:00Z",
		Level:     "INFO",
		Message:   "Test precedence",
		Fields: map[string]interface{}{
			"env": "test", // This should override the global "env" field
		},
	}

	// Call writeLogEntry directly to test precedence
	logger.writeLogEntry(entry)

	logger.Sync()

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)

	// The entry field should take precedence - we should see "test", not "production"
	if !strings.Contains(logContent, "test") {
		t.Errorf("Entry field value 'test' should take precedence over global field")
	}

	// Ensure we don't see the global field value in this context
	// Note: This is a simplified check - in real JSON, we'd parse and verify specific field values
	expectedCount := strings.Count(logContent, "production")
	if expectedCount > 0 {
		t.Errorf("Global field value 'production' should be overridden by entry field")
	}
}

// TestWithContext tests the WithContext method
func TestWithContext(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test WithContext returns a ContextLogger
	ctx := context.Background()
	contextLogger := logger.WithContext(ctx)

	if contextLogger == nil {
		t.Fatal("WithContext should return a non-nil Logger")
	}

	// Verify it implements the Logger interface by calling a method
	contextLogger.Info("Test context logger")

	// Sync and verify the message was logged
	logger.Sync()

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "Test context logger") {
		t.Errorf("Context logger message not found in log output")
	}
}

// TestAddGlobalFieldNilFields tests AddGlobalField when globalFields is initially nil
func TestAddGlobalFieldNilFields(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Initially globalFields should be nil
	if logger.GetGlobalFields() != nil {
		t.Errorf("Expected globalFields to be nil initially")
	}

	// Add a field when globalFields is nil
	logger.AddGlobalField("first_field", "first_value")

	fields := logger.GetGlobalFields()
	if fields == nil {
		t.Fatal("GlobalFields should not be nil after adding a field")
	}

	if len(fields) != 1 {
		t.Errorf("Expected 1 field, got %d", len(fields))
	}

	if fields["first_field"] != "first_value" {
		t.Errorf("Expected first_field=first_value, got %v", fields["first_field"])
	}
}

// TestRemoveGlobalFieldNilFields tests RemoveGlobalField when globalFields is nil
func TestRemoveGlobalFieldNilFields(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Try to remove a field when globalFields is nil - should not panic
	logger.RemoveGlobalField("nonexistent")

	// Should still be nil
	if logger.GetGlobalFields() != nil {
		t.Errorf("Expected globalFields to remain nil after removing from nil")
	}
}

// TestNewDestination tests the NewDestination function
func TestNewDestination(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		uri         string
		expectError bool
		backend     int
	}{
		{
			name:        "syslog destination",
			uri:         "syslog://localhost",
			expectError: false,
			backend:     BackendSyslog,
		},
		{
			name:        "file destination",
			uri:         "file://" + filepath.Join(tempDir, "test.log"),
			expectError: false,
			backend:     0, // file destinations don't set backend
		},
		{
			name:        "unsupported destination",
			uri:         "http://example.com",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest, err := NewDestination(tt.uri)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for URI %s, got none", tt.uri)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error for URI %s: %v", tt.uri, err)
			}

			if dest == nil {
				t.Fatal("Destination should not be nil")
			}

			if dest.URI != tt.uri {
				t.Errorf("Expected URI %s, got %s", tt.uri, dest.URI)
			}

			if tt.name == "syslog destination" && dest.Backend != BackendSyslog {
				t.Errorf("Expected syslog backend %d, got %d", BackendSyslog, dest.Backend)
			}

			if tt.name == "file destination" && dest.Writer == nil {
				t.Errorf("Expected file destination to have a Writer")
			}
		})
	}
}

// TestLoggerWorkerErrorPaths tests error handling in the background worker
func TestLoggerWorkerErrorPaths(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "worker_test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Test channel overflow behavior by filling the channel
	channelSize := logger.channelSize

	// Fill the channel beyond capacity to test overflow handling
	for i := 0; i < channelSize*2; i++ {
		select {
		case logger.msgChan <- types.LogMessage{
			Level:     LevelInfo,
			Format:    "overflow test",
			Timestamp: time.Now(),
		}:
		default:
			// Channel is full, which is expected behavior
		}
	}

	// Close and verify no goroutine leaks
	logger.Close()
}

// TestLoggerShutdownSequence tests proper shutdown sequence
func TestLoggerShutdownSequence(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "shutdown_test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Log some messages
	logger.Info("Before shutdown")
	logger.Debug("Debug message")

	// Test multiple close calls
	err1 := logger.Close()
	err2 := logger.Close()

	if err1 != nil {
		t.Errorf("First close should not error: %v", err1)
	}
	if err2 != nil {
		t.Errorf("Second close should not error: %v", err2)
	}

	// Operations after close should not panic but may not work
	logger.Info("After shutdown")
}

// TestLoggerChannelSizeConfiguration tests channel size configuration
func TestLoggerChannelSizeConfiguration(t *testing.T) {
	// Test with environment variable
	originalEnv := os.Getenv("OMNI_CHANNEL_SIZE")
	defer os.Setenv("OMNI_CHANNEL_SIZE", originalEnv)

	// Set custom channel size
	os.Setenv("OMNI_CHANNEL_SIZE", "50")

	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "channel_size_test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Verify channel size was set from environment
	if logger.channelSize != 50 {
		t.Errorf("Expected channel size 50, got %d", logger.channelSize)
	}
}

// TestLoggerErrorHandlerEdgeCases tests error handler edge cases
func TestLoggerErrorHandlerEdgeCases(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "error_handler_test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test setting nil error handler
	logger.SetErrorHandlerFunc(nil)

	// Test logging after setting nil error handler (should not panic)
	logger.Info("Message with nil error handler")

	// Test setting a custom error handler
	var handlerCalls int
	logger.SetErrorHandlerFunc(func(source, dest, msg string, err error) {
		handlerCalls++
	})

	// Normal logging should not trigger error handler
	logger.Info("Normal message")

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	if handlerCalls > 0 {
		t.Errorf("Error handler should not be called for normal operations, got %d calls", handlerCalls)
	}
}

// TestLoggerBackgroundWorkerState tests background worker state management
func TestLoggerBackgroundWorkerState(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "worker_state_test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Verify worker is started
	if !logger.workerStarted {
		t.Error("Worker should be started after logger creation")
	}

	// Log a message to ensure worker is functioning
	logger.Info("Worker test message")

	// Close and verify worker state
	logger.Close()

	if !logger.closed {
		t.Error("Logger should be marked as closed")
	}
}
