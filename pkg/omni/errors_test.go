package omni

import (
	"errors"
	"testing"
	"time"
)

// TestLogError tests the LogError struct and its methods
func TestLogError(t *testing.T) {
	underlyingErr := errors.New("underlying error")
	timestamp := time.Now()

	logErr := LogError{
		Operation:   "write",
		Destination: "/var/log/test.log",
		Message:     "Failed to write log entry",
		Err:         underlyingErr,
		Level:       ErrorLevelMedium,
		Timestamp:   timestamp,
		Context: map[string]interface{}{
			"attempt": 1,
			"size":    1024,
		},
		Code: 500,
	}

	// Test Error() method
	if logErr.Error() != "Failed to write log entry" {
		t.Errorf("Expected error message 'Failed to write log entry', got '%s'", logErr.Error())
	}

	// Test Unwrap() method
	if logErr.Unwrap() != underlyingErr {
		t.Errorf("Expected unwrapped error to be underlying error, got %v", logErr.Unwrap())
	}

	// Test with nil underlying error
	logErrNil := LogError{
		Message: "Error without underlying cause",
		Err:     nil,
	}

	if logErrNil.Unwrap() != nil {
		t.Errorf("Expected unwrapped error to be nil, got %v", logErrNil.Unwrap())
	}
}

// TestErrorLevels tests the error level constants
func TestErrorLevels(t *testing.T) {
	tests := []struct {
		name     string
		level    ErrorLevel
		expected int
	}{
		{"ErrorLevelLow", ErrorLevelLow, 0},
		{"ErrorLevelWarn", ErrorLevelWarn, 1},
		{"ErrorLevelMedium", ErrorLevelMedium, 2},
		{"ErrorLevelHigh", ErrorLevelHigh, 3},
		{"ErrorLevelCritical", ErrorLevelCritical, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if int(tt.level) != tt.expected {
				t.Errorf("Expected %s to have value %d, got %d", tt.name, tt.expected, int(tt.level))
			}
		})
	}
}

// TestErrorHandlers tests the predefined error handlers
func TestErrorHandlers(t *testing.T) {
	logErr := LogError{
		Operation:   "test",
		Destination: "test",
		Message:     "test error",
		Level:       ErrorLevelLow,
		Timestamp:   time.Now(),
	}

	// Test SilentErrorHandler - should not panic
	SilentErrorHandler(logErr)

	// Test StderrErrorHandler - should not panic
	StderrErrorHandler(logErr)

	// Test that handlers are not nil
	if SilentErrorHandler == nil {
		t.Error("SilentErrorHandler should not be nil")
	}

	if StderrErrorHandler == nil {
		t.Error("StderrErrorHandler should not be nil")
	}
}

// TestLogErrorFields tests setting various fields on LogError
func TestLogErrorFields(t *testing.T) {
	logErr := LogError{}

	// Test field assignments
	logErr.Operation = "flush"
	logErr.Destination = "stdout"
	logErr.Message = "buffer overflow"
	logErr.Level = ErrorLevelHigh
	logErr.Code = 429
	logErr.Context = map[string]interface{}{
		"buffer_size": 4096,
		"overflow_by": 512,
	}

	// Verify fields
	if logErr.Operation != "flush" {
		t.Errorf("Expected operation 'flush', got '%s'", logErr.Operation)
	}
	if logErr.Destination != "stdout" {
		t.Errorf("Expected destination 'stdout', got '%s'", logErr.Destination)
	}
	if logErr.Message != "buffer overflow" {
		t.Errorf("Expected message 'buffer overflow', got '%s'", logErr.Message)
	}
	if logErr.Level != ErrorLevelHigh {
		t.Errorf("Expected level ErrorLevelHigh, got %v", logErr.Level)
	}
	if logErr.Code != 429 {
		t.Errorf("Expected code 429, got %d", logErr.Code)
	}
	if logErr.Context["buffer_size"] != 4096 {
		t.Errorf("Expected context buffer_size 4096, got %v", logErr.Context["buffer_size"])
	}
}
