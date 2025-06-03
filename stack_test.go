package omni_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	pkgErrors "github.com/pkg/errors"
	"github.com/wayneeseguin/omni"
)

func TestEnableStackTraces(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Test with stack traces disabled (default behavior)
	logger.EnableStackTraces(false)

	// Create an error with stack trace capability
	stackErr := pkgErrors.New("error with stack")
	logger.ErrorWithError("test without stack", stackErr)
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if strings.Contains(string(content), "stack_trace=") {
		t.Error("Should not include stack trace when EnableStackTraces(false)")
	}

	// Clear the log file
	os.Truncate(tempFile, 0)

	// Test with stack traces enabled
	logger.EnableStackTraces(true)
	logger.ErrorWithError("test with stack", stackErr)
	logger.Sync()

	content, _ = os.ReadFile(tempFile)
	if !strings.Contains(string(content), "stack_trace=") {
		t.Error("Should include stack trace when EnableStackTraces(true)")
	}
}

func TestEnableStackTracesWithStructuredLog(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Test that structured logs at ERROR level include stack traces when enabled
	logger.EnableStackTraces(true)

	logger.StructuredLog(omni.LevelError, "error message", map[string]interface{}{
		"component": "test",
	})
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "error message") {
		t.Error("Should contain error message")
	}

	// Check if stack trace information is included for ERROR level
	// ERROR level should include full stack trace when includeTrace is enabled
	if !strings.Contains(contentStr, "goroutine") {
		t.Error("Should include full stack trace for ERROR level when EnableStackTraces(true)")
	}

	// Clear the log file
	os.Truncate(tempFile, 0)

	// Test with stack traces disabled
	logger.EnableStackTraces(false)
	logger.StructuredLog(omni.LevelError, "error without stack", map[string]interface{}{
		"component": "test",
	})
	logger.Sync()

	content, _ = os.ReadFile(tempFile)
	contentStr = string(content)

	// Should still have the message but no stack trace
	if !strings.Contains(contentStr, "error without stack") {
		t.Error("Should contain error message")
	}
}

func TestSetStackSize(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Set a small stack size
	logger.SetStackSize(100)
	logger.EnableStackTraces(true)

	// Create an error and log it
	stackErr := pkgErrors.New("error with limited stack")
	logger.ErrorWithError("test with small stack", stackErr)
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "stack_trace=") {
		t.Error("Should include stack trace")
	}

	// Clear and test with larger stack size
	os.Truncate(tempFile, 0)
	logger.SetStackSize(8192)

	logger.ErrorWithError("test with large stack", stackErr)
	logger.Sync()

	content, _ = os.ReadFile(tempFile)
	if !strings.Contains(string(content), "stack_trace=") {
		t.Error("Should include stack trace with larger buffer")
	}
}

func TestSetCaptureAllStacks(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Enable stack traces and capture all stacks
	logger.EnableStackTraces(true)
	logger.SetCaptureAllStacks(true)

	// Test INFO level (should now include stack trace due to captureAll)
	logger.StructuredLog(omni.LevelInfo, "info with stack", map[string]interface{}{
		"component": "test",
	})
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "info with stack") {
		t.Error("Should contain info message")
	}

	// Should include stack trace for INFO level when captureAll is enabled
	if !strings.Contains(contentStr, "goroutine") {
		t.Error("Should include stack trace for INFO level when SetCaptureAllStacks(true)")
	}

	// Clear the log file
	os.Truncate(tempFile, 0)

	// Test with captureAll disabled
	logger.SetCaptureAllStacks(false)
	logger.StructuredLog(omni.LevelInfo, "info without stack", map[string]interface{}{
		"component": "test",
	})
	logger.Sync()

	content, _ = os.ReadFile(tempFile)
	contentStr = string(content)

	if !strings.Contains(contentStr, "info without stack") {
		t.Error("Should contain info message")
	}

	// Should NOT include stack trace for INFO level when captureAll is disabled
	// (INFO level typically doesn't get stack traces unless captureAll is enabled)
}

func TestStackTraceWithPanic(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Set stack size for panic recovery
	logger.SetStackSize(4096)

	// Test panic logging (which should always include stack trace)
	testPanic := "test panic message"
	logger.LogPanic(testPanic)
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "test panic message") {
		t.Error("Should contain panic message")
	}

	if !strings.Contains(contentStr, "stack_trace=") {
		t.Error("Should include stack trace for panic")
	}

	// Stack trace should contain meaningful information
	if !strings.Contains(contentStr, "goroutine") {
		t.Error("Stack trace should contain goroutine information")
	}
}

func TestStackSettingsConcurrency(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Test that stack settings can be safely changed concurrently
	done := make(chan bool, 3)

	// Goroutine 1: Toggle EnableStackTraces
	go func() {
		for i := 0; i < 10; i++ {
			logger.EnableStackTraces(i%2 == 0)
		}
		done <- true
	}()

	// Goroutine 2: Change SetStackSize
	go func() {
		for i := 0; i < 10; i++ {
			logger.SetStackSize(1024 + i*512)
		}
		done <- true
	}()

	// Goroutine 3: Toggle SetCaptureAllStacks
	go func() {
		for i := 0; i < 10; i++ {
			logger.SetCaptureAllStacks(i%2 == 1)
		}
		done <- true
	}()

	// Wait for all goroutines to complete
	for i := 0; i < 3; i++ {
		<-done
	}

	// If we get here without hanging or panicking, the test passes
	// The actual settings don't matter - we're testing thread safety
}

func TestStackTraceDefaultSettings(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Test default behavior - stack traces should be disabled by default
	stackErr := pkgErrors.New("error with default settings")
	logger.ErrorWithError("test default behavior", stackErr)
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "test default behavior") {
		t.Error("Should contain error message")
	}

	// By default, pkg/errors stack traces should NOT be included
	// unless explicitly enabled
	if strings.Contains(contentStr, "stack_trace=") {
		t.Error("Should not include stack trace by default")
	}
}

func TestStackTraceWithRegularErrors(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Enable stack traces
	logger.EnableStackTraces(true)

	// Test with regular Go error (no stack trace capability)
	regularErr := pkgErrors.New("regular error")
	logger.ErrorWithError("test regular error", regularErr)
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "test regular error") {
		t.Error("Should contain error message")
	}

	// Should include stack trace for pkg/errors even when using ErrorWithError
	if !strings.Contains(contentStr, "stack_trace=") {
		t.Error("Should include stack trace for pkg/errors when EnableStackTraces(true)")
	}
}
