package omni_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pkgErrors "github.com/pkg/errors"
	"github.com/wayneeseguin/omni"
)

func TestSeverityToString(t *testing.T) {
	// This tests the internal severityToString function indirectly through ErrorWithErrorAndSeverity
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	tests := []struct {
		name     string
		severity omni.ErrorLevel
		expected string
	}{
		{"low severity", omni.SeverityLow, "low"},
		{"medium severity", omni.SeverityMedium, "medium"},
		{"high severity", omni.SeverityHigh, "high"},
		{"critical severity", omni.SeverityCritical, "critical"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear the log file
			os.Truncate(tempFile, 0)

			testErr := errors.New("test error")
			logger.ErrorWithErrorAndSeverity("test message", testErr, tt.severity)
			logger.Sync()

			content, err := os.ReadFile(tempFile)
			if err != nil {
				t.Fatalf("Failed to read log file: %v", err)
			}

			if !strings.Contains(string(content), fmt.Sprintf("severity=%s", tt.expected)) {
				t.Errorf("Expected severity %s in log output, got: %s", tt.expected, string(content))
			}
		})
	}
}

func TestErrorWithError(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Test basic error logging
	testErr := errors.New("test error message")
	logger.ErrorWithError("operation failed", testErr)
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "operation failed") {
		t.Error("Expected message 'operation failed' in log output")
	}

	if !strings.Contains(contentStr, "error=test error message") {
		t.Error("Expected error field in log output")
	}

	// Test that level filtering works
	logger.SetLevel(omni.LevelError + 1) // Set level higher than error
	os.Truncate(tempFile, 0)

	logger.ErrorWithError("should be filtered", testErr)
	logger.Sync()

	content, _ = os.ReadFile(tempFile)
	if len(content) > 0 {
		t.Error("Expected no output when error level is filtered")
	}
}

func TestErrorWithErrorWithStackTrace(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Enable stack traces
	logger.EnableStackTraces(true)

	// Create an error with stack trace using pkg/errors
	testErr := pkgErrors.New("error with stack")
	logger.ErrorWithError("operation failed with stack", testErr)
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "operation failed with stack") {
		t.Error("Expected message in log output")
	}

	if !strings.Contains(contentStr, "stack_trace=") {
		t.Error("Expected stack trace in log output when includeTrace is enabled")
	}
}

func TestErrorWithErrorAndSeverity(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	testErr := errors.New("critical error")
	logger.ErrorWithErrorAndSeverity("system failure", testErr, omni.SeverityCritical)
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "system failure") {
		t.Error("Expected message in log output")
	}

	if !strings.Contains(contentStr, "error=critical error") {
		t.Error("Expected error field in log output")
	}

	if !strings.Contains(contentStr, "severity=critical") {
		t.Error("Expected severity field in log output")
	}
}

func TestWrapError(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	originalErr := errors.New("original error")
	wrappedErr := logger.WrapError(originalErr, "additional context")

	// Check that the wrapped error contains both messages
	if !strings.Contains(wrappedErr.Error(), "additional context") {
		t.Error("Wrapped error should contain the wrapping message")
	}

	if !strings.Contains(wrappedErr.Error(), "original error") {
		t.Error("Wrapped error should contain the original error")
	}

	// Check that it's a pkg/errors wrapped error
	if cause := logger.CauseOf(wrappedErr); cause.Error() != originalErr.Error() {
		t.Errorf("Expected cause to be original error, got: %v", cause)
	}
}

func TestWrapErrorWithSeverity(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	originalErr := errors.New("original error")
	wrappedErr := logger.WrapErrorWithSeverity(originalErr, "wrapped with severity", omni.SeverityHigh)
	logger.Sync()

	// Check that the error is wrapped
	if !strings.Contains(wrappedErr.Error(), "wrapped with severity") {
		t.Error("Wrapped error should contain the wrapping message")
	}

	// Check that it was logged immediately
	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "wrapped with severity") {
		t.Error("Expected wrapping message to be logged")
	}

	if !strings.Contains(contentStr, "severity=high") {
		t.Error("Expected severity to be logged")
	}
}

func TestCauseOf(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	originalErr := errors.New("root cause")
	wrappedErr := pkgErrors.Wrap(originalErr, "layer 1")
	doubleWrapped := pkgErrors.Wrap(wrappedErr, "layer 2")

	cause := logger.CauseOf(doubleWrapped)
	if cause.Error() != originalErr.Error() {
		t.Errorf("Expected root cause, got: %v", cause)
	}

	// Test with unwrapped error
	simpleCause := logger.CauseOf(originalErr)
	if simpleCause != originalErr {
		t.Errorf("Expected same error for unwrapped error, got: %v", simpleCause)
	}
}

func TestWithStack(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	originalErr := errors.New("simple error")
	stackErr := logger.WithStack(originalErr)

	// Check that it has stack trace capabilities
	if _, ok := stackErr.(interface{ StackTrace() pkgErrors.StackTrace }); !ok {
		t.Error("Error with stack should implement StackTrace interface")
	}

	// Error message should be preserved
	if stackErr.Error() != originalErr.Error() {
		t.Errorf("Expected same error message, got: %v", stackErr.Error())
	}
}

func TestIsErrorType(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Create a sentinel error
	var ErrSentinel = errors.New("sentinel error")

	// Test direct comparison
	if !logger.IsErrorType(ErrSentinel, ErrSentinel) {
		t.Error("Should match same error")
	}

	// Test wrapped error
	wrappedErr := pkgErrors.Wrap(ErrSentinel, "wrapped")
	if !logger.IsErrorType(wrappedErr, ErrSentinel) {
		t.Error("Should match wrapped error")
	}

	// Test different error
	differentErr := errors.New("different error")
	if logger.IsErrorType(differentErr, ErrSentinel) {
		t.Error("Should not match different error")
	}
}

func TestFormatErrorVerbose(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Test with stack trace error
	stackErr := pkgErrors.New("error with stack")
	verboseFormat := logger.FormatErrorVerbose(stackErr)

	if !strings.Contains(verboseFormat, "error with stack") {
		t.Error("Verbose format should contain error message")
	}

	// Should contain stack information for pkg/errors errors
	if !strings.Contains(verboseFormat, "errors_test.go") {
		t.Error("Verbose format should contain stack trace information")
	}

	// Test with simple error
	simpleErr := errors.New("simple error")
	simpleFormat := logger.FormatErrorVerbose(simpleErr)

	if !strings.Contains(simpleFormat, "simple error") {
		t.Error("Verbose format should contain error message for simple errors")
	}
}

func TestLogPanic(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Test with error panic
	testErr := errors.New("panic error")
	logger.LogPanic(testErr)
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "Recovered from panic") {
		t.Error("Expected panic recovery message")
	}

	if !strings.Contains(contentStr, "panic error") {
		t.Error("Expected panic error message")
	}

	if !strings.Contains(contentStr, "panic=true") {
		t.Error("Expected panic field")
	}

	if !strings.Contains(contentStr, "stack_trace=") {
		t.Error("Expected stack trace field")
	}

	// Clear log for next test
	os.Truncate(tempFile, 0)

	// Test with string panic
	logger.LogPanic("string panic")
	logger.Sync()

	content, _ = os.ReadFile(tempFile)
	contentStr = string(content)

	if !strings.Contains(contentStr, "string panic") {
		t.Error("Expected string panic message")
	}
}

func TestSafeGo(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Test function that panics
	panicFunc := func() {
		panic("test panic from goroutine")
	}

	logger.SafeGo(panicFunc)

	// Give time for goroutine to execute and panic
	time.Sleep(100 * time.Millisecond)
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "Recovered from panic") {
		t.Error("Expected panic recovery message from SafeGo")
	}

	if !strings.Contains(contentStr, "test panic from goroutine") {
		t.Error("Expected panic message from goroutine")
	}

	// Clear log for next test
	os.Truncate(tempFile, 0)

	// Test function that doesn't panic
	normalFunc := func() {
		// Normal execution, no panic
	}

	logger.SafeGo(normalFunc)
	time.Sleep(100 * time.Millisecond)
	logger.Sync()

	content, _ = os.ReadFile(tempFile)
	if len(content) > 0 {
		t.Error("Expected no log output from normal function execution")
	}
}

func TestErrorLevelFiltering(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Set log level higher than error to filter out error messages
	logger.SetLevel(omni.LevelError + 1)

	testErr := errors.New("filtered error")

	// These should all be filtered out
	logger.ErrorWithError("should be filtered", testErr)
	logger.ErrorWithErrorAndSeverity("should be filtered", testErr, omni.SeverityHigh)
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if len(content) > 0 {
		t.Error("Expected no output when error level is filtered")
	}

	// Reset level to allow errors
	logger.SetLevel(omni.LevelError)
	logger.ErrorWithError("should not be filtered", testErr)
	logger.Sync()

	content, _ = os.ReadFile(tempFile)
	if !strings.Contains(string(content), "should not be filtered") {
		t.Error("Expected error message when level allows it")
	}
}

func TestStackTraceIncludeToggle(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	testErr := pkgErrors.New("error with potential stack")

	// Test with includeTrace disabled (default)
	logger.EnableStackTraces(false)
	logger.ErrorWithError("no stack trace", testErr)
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if strings.Contains(string(content), "stack_trace=") {
		t.Error("Should not include stack trace when includeTrace is false")
	}

	// Clear log
	os.Truncate(tempFile, 0)

	// Test with includeTrace enabled
	logger.EnableStackTraces(true)
	logger.ErrorWithError("with stack trace", testErr)
	logger.Sync()

	content, _ = os.ReadFile(tempFile)
	if !strings.Contains(string(content), "stack_trace=") {
		t.Error("Should include stack trace when includeTrace is true")
	}
}
