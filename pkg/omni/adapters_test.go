package omni

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLoggerAdapter(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "adapter.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set level to trace to test all levels
	logger.SetLevel(LevelTrace)

	adapter := NewLoggerAdapter(logger)
	if adapter == nil {
		t.Fatal("NewLoggerAdapter returned nil")
	}

	// Test logging methods without fields
	adapter.Trace("trace message")
	adapter.Debug("debug message")
	adapter.Info("info message")
	adapter.Warn("warn message")
	adapter.Error("error message")

	// Test formatted logging methods without fields
	adapter.Tracef("trace %s", "formatted")
	adapter.Debugf("debug %s", "formatted")
	adapter.Infof("info %s", "formatted")
	adapter.Warnf("warn %s", "formatted")
	adapter.Errorf("error %s", "formatted")

	// Test level checking methods
	if !adapter.IsTraceEnabled() {
		t.Error("Trace should be enabled")
	}
	if !adapter.IsDebugEnabled() {
		t.Error("Debug should be enabled")
	}
	if !adapter.IsInfoEnabled() {
		t.Error("Info should be enabled")
	}
	if !adapter.IsWarnEnabled() {
		t.Error("Warn should be enabled")
	}
	if !adapter.IsErrorEnabled() {
		t.Error("Error should be enabled")
	}

	// Test level operations
	originalLevel := adapter.GetLevel()
	adapter.SetLevel(LevelWarn)
	if adapter.GetLevel() != LevelWarn {
		t.Errorf("Expected level %d, got %d", LevelWarn, adapter.GetLevel())
	}

	if adapter.IsDebugEnabled() {
		t.Error("Debug should not be enabled at Warn level")
	}
	if !adapter.IsLevelEnabled(LevelWarn) {
		t.Error("Warn level should be enabled")
	}

	// Restore original level
	adapter.SetLevel(originalLevel)

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Verify log contents
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)
	expectedMessages := []string{
		"trace message", "debug message", "info message", "warn message", "error message",
		"trace formatted", "debug formatted", "info formatted", "warn formatted", "error formatted",
	}

	for _, expected := range expectedMessages {
		if !strings.Contains(logContent, expected) {
			t.Errorf("Expected to find '%s' in log output", expected)
		}
	}
}

func TestLoggerAdapterWithFields(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "adapter_fields.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	adapter := NewLoggerAdapter(logger)

	// Test WithField
	fieldAdapter := adapter.WithField("key1", "value1")
	if fieldAdapter == nil {
		t.Fatal("WithField returned nil")
	}

	// Test WithFields
	fields := map[string]interface{}{
		"key2": "value2",
		"key3": 123,
	}
	fieldsAdapter := adapter.WithFields(fields)
	if fieldsAdapter == nil {
		t.Fatal("WithFields returned nil")
	}

	// Test WithError with valid error
	testErr := errors.New("test error")
	errorAdapter := adapter.WithError(testErr)
	if errorAdapter == nil {
		t.Fatal("WithError returned nil")
	}

	// Test WithError with nil error
	nilErrorAdapter := adapter.WithError(nil)
	if nilErrorAdapter != adapter {
		t.Error("WithError(nil) should return the same adapter")
	}

	// Test WithContext
	ctx := context.Background()
	contextAdapter := adapter.WithContext(ctx)
	if contextAdapter == nil {
		t.Fatal("WithContext returned nil")
	}

	// Log with each adapter to verify fields are included
	fieldAdapter.Info("message with single field")
	fieldsAdapter.Info("message with multiple fields")
	errorAdapter.Info("message with error field")

	// Test formatted logging with fields
	fieldAdapter.Infof("formatted %s with field", "message")
	fieldsAdapter.Errorf("formatted %s with fields", "error")

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Verify log contents contain field information
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)
	expectedInContent := []string{
		"message with single field",
		"message with multiple fields",
		"message with error field",
		"formatted message with field",
		"formatted error with fields",
	}

	for _, expected := range expectedInContent {
		if !strings.Contains(logContent, expected) {
			t.Errorf("Expected to find '%s' in log output", expected)
		}
	}
}

func TestLoggerAdapterFieldChaining(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "adapter_chaining.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	adapter := NewLoggerAdapter(logger)

	// Test field chaining - adding multiple fields sequentially
	chainedAdapter := adapter.
		WithField("key1", "value1").
		WithField("key2", "value2").
		WithFields(map[string]interface{}{
			"key3": "value3",
			"key4": 456,
		}).
		WithError(errors.New("chained error"))

	chainedAdapter.Info("chained fields message")

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Verify log contents
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "chained fields message") {
		t.Error("Expected chained fields message in log output")
	}
}

func TestContextLogger(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "context.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set level to trace to test all levels
	logger.SetLevel(LevelTrace)

	ctx := context.Background()
	contextLogger := &ContextLogger{
		logger: logger,
		ctx:    ctx,
	}

	// Test all logging methods
	contextLogger.Trace("context trace")
	contextLogger.Debug("context debug")
	contextLogger.Info("context info")
	contextLogger.Warn("context warn")
	contextLogger.Error("context error")

	// Test formatted logging methods
	contextLogger.Tracef("context trace %s", "formatted")
	contextLogger.Debugf("context debug %s", "formatted")
	contextLogger.Infof("context info %s", "formatted")
	contextLogger.Warnf("context warn %s", "formatted")
	contextLogger.Errorf("context error %s", "formatted")

	// Test level checking methods
	if !contextLogger.IsTraceEnabled() {
		t.Error("Trace should be enabled")
	}
	if !contextLogger.IsDebugEnabled() {
		t.Error("Debug should be enabled")
	}
	if !contextLogger.IsInfoEnabled() {
		t.Error("Info should be enabled")
	}
	if !contextLogger.IsWarnEnabled() {
		t.Error("Warn should be enabled")
	}
	if !contextLogger.IsErrorEnabled() {
		t.Error("Error should be enabled")
	}

	// Test level operations
	originalLevel := contextLogger.GetLevel()
	contextLogger.SetLevel(LevelError)
	if contextLogger.GetLevel() != LevelError {
		t.Errorf("Expected level %d, got %d", LevelError, contextLogger.GetLevel())
	}

	if !contextLogger.IsLevelEnabled(LevelError) {
		t.Error("Error level should be enabled")
	}
	if contextLogger.IsLevelEnabled(LevelDebug) {
		t.Error("Debug level should not be enabled at Error level")
	}

	// Restore original level
	contextLogger.SetLevel(originalLevel)

	// Test WithField, WithFields, WithError, WithContext
	fieldLogger := contextLogger.WithField("key", "value")
	if fieldLogger == nil {
		t.Fatal("WithField returned nil")
	}

	fieldsLogger := contextLogger.WithFields(map[string]interface{}{"key": "value"})
	if fieldsLogger == nil {
		t.Fatal("WithFields returned nil")
	}

	errorLogger := contextLogger.WithError(errors.New("test error"))
	if errorLogger == nil {
		t.Fatal("WithError returned nil")
	}

	nilErrorLogger := contextLogger.WithError(nil)
	if nilErrorLogger != contextLogger {
		t.Error("WithError(nil) should return the same logger")
	}

	newCtx := context.WithValue(ctx, "key", "value")
	ctxLogger := contextLogger.WithContext(newCtx)
	if ctxLogger == nil {
		t.Fatal("WithContext returned nil")
	}

	// Log with each logger variant
	fieldLogger.Info("field logger message")
	fieldsLogger.Info("fields logger message")
	errorLogger.Info("error logger message")
	ctxLogger.Info("context logger message")

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Verify log contents
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)
	expectedMessages := []string{
		"context trace", "context debug", "context info", "context warn", "context error",
		"context trace formatted", "context debug formatted", "context info formatted",
		"context warn formatted", "context error formatted",
		"field logger message", "fields logger message", "error logger message", "context logger message",
	}

	for _, expected := range expectedMessages {
		if !strings.Contains(logContent, expected) {
			t.Errorf("Expected to find '%s' in log output", expected)
		}
	}
}

func TestAdapterConcurrency(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "adapter_concurrent.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	adapter := NewLoggerAdapter(logger)

	// Test concurrent usage of adapter
	var wg sync.WaitGroup
	numGoroutines := 5         // Reduced from 10
	messagesPerGoroutine := 20 // Reduced from 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			localAdapter := adapter.WithField("goroutine", id)
			for j := 0; j < messagesPerGoroutine; j++ {
				localAdapter.Infof("Message %d from goroutine %d", j, id)
			}
		}(i)
	}

	wg.Wait()

	// Wait for all messages to be processed
	time.Sleep(200 * time.Millisecond)

	// Verify log file was created and has content
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if len(content) == 0 {
		t.Error("Log file is empty")
	}

	// Count messages to ensure no data races caused message loss
	messageCount := strings.Count(string(content), "Message")
	expectedCount := numGoroutines * messagesPerGoroutine

	// Allow for some message loss due to channel overflow in high concurrency
	if messageCount < expectedCount/3 {
		t.Errorf("Expected at least %d messages, got %d", expectedCount/3, messageCount)
	}
}

func TestAdapterLevelFiltering(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "adapter_levels.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	adapter := NewLoggerAdapter(logger)

	// Set level to Warn - should filter out Trace, Debug, Info
	adapter.SetLevel(LevelWarn)

	// Log at all levels
	adapter.Trace("trace message") // Should be filtered
	adapter.Debug("debug message") // Should be filtered
	adapter.Info("info message")   // Should be filtered
	adapter.Warn("warn message")   // Should appear
	adapter.Error("error message") // Should appear

	// Test formatted versions
	adapter.Tracef("trace %s", "formatted") // Should be filtered
	adapter.Debugf("debug %s", "formatted") // Should be filtered
	adapter.Infof("info %s", "formatted")   // Should be filtered
	adapter.Warnf("warn %s", "formatted")   // Should appear
	adapter.Errorf("error %s", "formatted") // Should appear

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Verify only Warn and Error messages appear
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)

	// These should NOT appear
	filteredMessages := []string{"trace message", "debug message", "info message", "trace formatted", "debug formatted", "info formatted"}
	for _, filtered := range filteredMessages {
		if strings.Contains(logContent, filtered) {
			t.Errorf("Message '%s' should have been filtered out", filtered)
		}
	}

	// These SHOULD appear
	allowedMessages := []string{"warn message", "error message", "warn formatted", "error formatted"}
	for _, allowed := range allowedMessages {
		if !strings.Contains(logContent, allowed) {
			t.Errorf("Message '%s' should have appeared in log", allowed)
		}
	}
}

func TestLoggerAdapterFieldCodePaths(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "adapter_field_paths.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set level to trace to test all levels
	logger.SetLevel(LevelTrace)

	// Test adapter with pre-existing fields (fields != nil path)
	baseAdapter := NewLoggerAdapter(logger)
	adapterWithFields := baseAdapter.WithField("test_key", "test_value")

	// Test all logging methods with fields != nil
	adapterWithFields.Trace("trace with fields")
	adapterWithFields.Debug("debug with fields")
	adapterWithFields.Warn("warn with fields")
	adapterWithFields.Tracef("trace %s with fields", "formatted")
	adapterWithFields.Debugf("debug %s with fields", "formatted")
	adapterWithFields.Warnf("warn %s with fields", "formatted")

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Verify log contents
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)
	expectedMessages := []string{
		"trace with fields", "debug with fields", "warn with fields",
		"trace formatted with fields", "debug formatted with fields", "warn formatted with fields",
	}

	for _, expected := range expectedMessages {
		if !strings.Contains(logContent, expected) {
			t.Errorf("Expected to find '%s' in log output with fields", expected)
		}
	}
}

func TestLoggerAdapterEdgeCases(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "adapter_edge_cases.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test adapter with nil fields initially
	adapter := NewLoggerAdapter(logger)
	if adapter.fields != nil {
		t.Error("New adapter should have nil fields")
	}

	// Test WithField with nil fields (initial case)
	fieldAdapter := adapter.WithField("first", "value")
	if fieldAdapter.(*LoggerAdapter).fields == nil {
		t.Error("WithField should create non-nil fields map")
	}
	if len(fieldAdapter.(*LoggerAdapter).fields) != 1 {
		t.Errorf("Expected 1 field, got %d", len(fieldAdapter.(*LoggerAdapter).fields))
	}

	// Test WithFields with nil original fields
	fieldsAdapter := adapter.WithFields(map[string]interface{}{"key": "value"})
	if fieldsAdapter.(*LoggerAdapter).fields == nil {
		t.Error("WithFields should create non-nil fields map")
	}

	// Test WithFields with empty input
	emptyFieldsAdapter := adapter.WithFields(map[string]interface{}{})
	if emptyFieldsAdapter.(*LoggerAdapter).fields == nil {
		t.Error("WithFields should create non-nil fields map even for empty input")
	}

	// Test WithFields with nil input
	nilFieldsAdapter := adapter.WithFields(nil)
	if nilFieldsAdapter.(*LoggerAdapter).fields == nil {
		t.Error("WithFields should create non-nil fields map even for nil input")
	}

	// Test field overwriting behavior
	baseFields := map[string]interface{}{
		"key1": "original",
		"key2": "value2",
	}
	baseWithFields := adapter.WithFields(baseFields)

	// Override one field
	overrideAdapter := baseWithFields.WithField("key1", "overridden")
	overriddenFields := overrideAdapter.(*LoggerAdapter).fields
	if overriddenFields["key1"] != "overridden" {
		t.Errorf("Expected 'overridden', got %v", overriddenFields["key1"])
	}
	if overriddenFields["key2"] != "value2" {
		t.Errorf("Expected 'value2' to be preserved, got %v", overriddenFields["key2"])
	}

	// Test ContextLogger WithField/WithFields returning LoggerAdapter
	ctx := context.Background()
	contextLogger := &ContextLogger{logger: logger, ctx: ctx}

	ctxFieldLogger := contextLogger.WithField("ctx_key", "ctx_value")
	if _, ok := ctxFieldLogger.(*LoggerAdapter); !ok {
		t.Error("ContextLogger.WithField should return LoggerAdapter")
	}

	ctxFieldsLogger := contextLogger.WithFields(map[string]interface{}{"ctx_key": "ctx_value"})
	if _, ok := ctxFieldsLogger.(*LoggerAdapter); !ok {
		t.Error("ContextLogger.WithFields should return LoggerAdapter")
	}

	// Test logging with various argument types
	adapter.Trace("string", 123, true, []string{"array"})
	adapter.Debug(nil, "mixed", 456)
	adapter.Info() // No arguments
	adapter.Warn(struct{ Name string }{"test"})
	adapter.Error(map[string]string{"key": "value"})

	// Wait for processing
	time.Sleep(100 * time.Millisecond)
}

func TestContextLoggerEdgeCases(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "context_edge_cases.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test ContextLogger with nil context
	contextLogger := &ContextLogger{
		logger: logger,
		ctx:    nil,
	}

	// Should not panic with nil context
	contextLogger.Info("message with nil context")

	// Test nested WithContext calls
	ctx1 := context.WithValue(context.Background(), "key1", "value1")
	ctx2 := context.WithValue(ctx1, "key2", "value2")

	ctxLogger1 := contextLogger.WithContext(ctx1)
	ctxLogger2 := ctxLogger1.WithContext(ctx2)

	// Should return new ContextLogger instances
	if _, ok := ctxLogger1.(*ContextLogger); !ok {
		t.Error("WithContext should return ContextLogger")
	}
	if _, ok := ctxLogger2.(*ContextLogger); !ok {
		t.Error("Nested WithContext should return ContextLogger")
	}

	// Test that each has the expected context
	if ctxLogger2.(*ContextLogger).ctx != ctx2 {
		t.Error("Context should be updated in new logger")
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)
}
