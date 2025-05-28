package flexlog

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestFlexLogError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *FlexLogError
		expected string
	}{
		{
			name: "error with destination",
			err: &FlexLogError{
				Code:        ErrCodeFileWrite,
				Op:          "write",
				Path:        "/tmp/test.log",
				Err:         errors.New("disk full"),
				Time:        time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
				Destination: "main-log",
			},
			expected: "[2023-01-01 12:00:00] write operation failed on /tmp/test.log (destination: main-log): disk full",
		},
		{
			name: "error without destination",
			err: &FlexLogError{
				Code: ErrCodeFileRotate,
				Op:   "rotate",
				Path: "/tmp/test.log",
				Err:  errors.New("permission denied"),
				Time: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			},
			expected: "[2023-01-01 12:00:00] rotate operation failed on /tmp/test.log: permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.err.Error()
			if actual != tt.expected {
				t.Errorf("Error() = %q, want %q", actual, tt.expected)
			}
		})
	}
}

func TestFlexLogError_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")
	flexErr := &FlexLogError{
		Code: ErrCodeFileWrite,
		Op:   "write",
		Path: "/tmp/test.log",
		Err:  originalErr,
		Time: time.Now(),
	}

	unwrapped := flexErr.Unwrap()
	if unwrapped != originalErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, originalErr)
	}
}

func TestFlexLogError_Is(t *testing.T) {
	originalErr := errors.New("original error")
	flexErr1 := &FlexLogError{
		Code: ErrCodeFileWrite,
		Op:   "write",
		Path: "/tmp/test.log",
		Err:  originalErr,
		Time: time.Now(),
	}
	flexErr2 := &FlexLogError{
		Code: ErrCodeFileWrite,
		Op:   "write",
		Path: "/tmp/other.log",
		Err:  errors.New("different error"),
		Time: time.Now(),
	}
	flexErr3 := &FlexLogError{
		Code: ErrCodeFileFlush,
		Op:   "flush",
		Path: "/tmp/test.log",
		Err:  originalErr,
		Time: time.Now(),
	}

	tests := []struct {
		name     string
		err      *FlexLogError
		target   error
		expected bool
	}{
		{
			name:     "same error code",
			err:      flexErr1,
			target:   flexErr2,
			expected: true,
		},
		{
			name:     "different error code",
			err:      flexErr1,
			target:   flexErr3,
			expected: false,
		},
		{
			name:     "underlying error match",
			err:      flexErr1,
			target:   originalErr,
			expected: true,
		},
		{
			name:     "nil target",
			err:      flexErr1,
			target:   nil,
			expected: false,
		},
		{
			name:     "different error type",
			err:      flexErr1,
			target:   errors.New("different"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.err.Is(tt.target)
			if actual != tt.expected {
				t.Errorf("Is() = %v, want %v", actual, tt.expected)
			}
		})
	}
}

func TestNewFlexLogError(t *testing.T) {
	originalErr := errors.New("test error")
	flexErr := NewFlexLogError(ErrCodeFileWrite, "write", "/tmp/test.log", originalErr)

	if flexErr.Code != ErrCodeFileWrite {
		t.Errorf("Code = %v, want %v", flexErr.Code, ErrCodeFileWrite)
	}
	if flexErr.Op != "write" {
		t.Errorf("Op = %v, want %v", flexErr.Op, "write")
	}
	if flexErr.Path != "/tmp/test.log" {
		t.Errorf("Path = %v, want %v", flexErr.Path, "/tmp/test.log")
	}
	if flexErr.Err != originalErr {
		t.Errorf("Err = %v, want %v", flexErr.Err, originalErr)
	}
	if flexErr.Context == nil {
		t.Error("Context should be initialized")
	}
	if time.Since(flexErr.Time) > time.Second {
		t.Error("Time should be recent")
	}
}

func TestFlexLogError_WithDestination(t *testing.T) {
	flexErr := NewFlexLogError(ErrCodeFileWrite, "write", "/tmp/test.log", nil)
	result := flexErr.WithDestination("test-dest")

	if result != flexErr {
		t.Error("WithDestination should return the same instance")
	}
	if flexErr.Destination != "test-dest" {
		t.Errorf("Destination = %v, want %v", flexErr.Destination, "test-dest")
	}
}

func TestFlexLogError_WithContext(t *testing.T) {
	flexErr := NewFlexLogError(ErrCodeFileWrite, "write", "/tmp/test.log", nil)
	result := flexErr.WithContext("key", "value")

	if result != flexErr {
		t.Error("WithContext should return the same instance")
	}
	if flexErr.Context["key"] != "value" {
		t.Errorf("Context[key] = %v, want %v", flexErr.Context["key"], "value")
	}
}

func TestErrorHelperFunctions(t *testing.T) {
	originalErr := errors.New("test error")

	tests := []struct {
		name     string
		fn       func() *FlexLogError
		code     ErrorCode
		op       string
		path     string
		checkErr bool
	}{
		{
			name:     "ErrFileOpen",
			fn:       func() *FlexLogError { return ErrFileOpen("/tmp/test.log", originalErr) },
			code:     ErrCodeFileOpen,
			op:       "open",
			path:     "/tmp/test.log",
			checkErr: true,
		},
		{
			name:     "ErrFileWrite",
			fn:       func() *FlexLogError { return ErrFileWrite("/tmp/test.log", originalErr) },
			code:     ErrCodeFileWrite,
			op:       "write",
			path:     "/tmp/test.log",
			checkErr: true,
		},
		{
			name:     "ErrFileFlush",
			fn:       func() *FlexLogError { return ErrFileFlush("/tmp/test.log", originalErr) },
			code:     ErrCodeFileFlush,
			op:       "flush",
			path:     "/tmp/test.log",
			checkErr: true,
		},
		{
			name:     "ErrFileRotate",
			fn:       func() *FlexLogError { return ErrFileRotate("/tmp/test.log", originalErr) },
			code:     ErrCodeFileRotate,
			op:       "rotate",
			path:     "/tmp/test.log",
			checkErr: true,
		},
		{
			name:     "ErrChannelFull",
			fn:       func() *FlexLogError { return NewChannelFullError("write") },
			code:     ErrCodeChannelFull,
			op:       "write",
			path:     "",
			checkErr: false,
		},
		{
			name:     "ErrDestinationNotFound",
			fn:       func() *FlexLogError { return ErrDestinationNotFound("test-dest") },
			code:     ErrCodeDestinationNotFound,
			op:       "find",
			path:     "test-dest",
			checkErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()

			if err.Code != tt.code {
				t.Errorf("Code = %v, want %v", err.Code, tt.code)
			}
			if err.Op != tt.op {
				t.Errorf("Op = %v, want %v", err.Op, tt.op)
			}
			if err.Path != tt.path {
				t.Errorf("Path = %v, want %v", err.Path, tt.path)
			}
			if tt.checkErr && err.Err != originalErr {
				t.Errorf("Err = %v, want %v", err.Err, originalErr)
			}
		})
	}
}

func TestErrShutdownTimeout(t *testing.T) {
	duration := 5 * time.Second
	err := ErrShutdownTimeout(duration)

	if err.Code != ErrCodeShutdownTimeout {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeShutdownTimeout)
	}
	if err.Op != "shutdown" {
		t.Errorf("Op = %v, want %v", err.Op, "shutdown")
	}
	if err.Context["timeout"] != duration {
		t.Errorf("Context[timeout] = %v, want %v", err.Context["timeout"], duration)
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "channel full error",
			err:      NewFlexLogError(ErrCodeChannelFull, "write", "", nil),
			expected: true,
		},
		{
			name:     "compression queue full error",
			err:      NewFlexLogError(ErrCodeCompressionQueueFull, "compress", "", nil),
			expected: true,
		},
		{
			name:     "file lock error",
			err:      NewFlexLogError(ErrCodeFileLock, "lock", "", nil),
			expected: true,
		},
		{
			name:     "file write error",
			err:      NewFlexLogError(ErrCodeFileWrite, "write", "", nil),
			expected: false,
		},
		{
			name:     "resource temporarily unavailable",
			err:      errors.New("resource temporarily unavailable"),
			expected: true,
		},
		{
			name:     "too many open files",
			err:      errors.New("too many open files"),
			expected: true,
		},
		{
			name:     "no space left on device",
			err:      errors.New("no space left on device"),
			expected: true,
		},
		{
			name:     "generic error",
			err:      errors.New("generic error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := IsRetryable(tt.err)
			if actual != tt.expected {
				t.Errorf("IsRetryable() = %v, want %v", actual, tt.expected)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{
			name:     "exact match",
			s:        "hello",
			substr:   "hello",
			expected: true,
		},
		{
			name:     "substring found",
			s:        "hello world",
			substr:   "world",
			expected: true,
		},
		{
			name:     "case insensitive match",
			s:        "Hello World",
			substr:   "hello",
			expected: true,
		},
		{
			name:     "substring not found",
			s:        "hello world",
			substr:   "foo",
			expected: false,
		},
		{
			name:     "empty substring",
			s:        "hello",
			substr:   "",
			expected: true,
		},
		{
			name:     "empty string",
			s:        "",
			substr:   "hello",
			expected: false,
		},
		{
			name:     "both empty",
			s:        "",
			substr:   "",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := contains(tt.s, tt.substr)
			if actual != tt.expected {
				t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, actual, tt.expected)
			}
		})
	}
}

func TestContainsHelper(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{
			name:     "exact match",
			s:        "hello",
			substr:   "hello",
			expected: true,
		},
		{
			name:     "case insensitive lower to upper",
			s:        "hello",
			substr:   "HELLO",
			expected: true,
		},
		{
			name:     "case insensitive upper to lower",
			s:        "HELLO",
			substr:   "hello",
			expected: true,
		},
		{
			name:     "mixed case",
			s:        "Hello World",
			substr:   "WORLD",
			expected: true,
		},
		{
			name:     "no match",
			s:        "hello",
			substr:   "xyz",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := containsHelper(tt.s, tt.substr)
			if actual != tt.expected {
				t.Errorf("containsHelper(%q, %q) = %v, want %v", tt.s, tt.substr, actual, tt.expected)
			}
		})
	}
}

// Benchmark tests for error handling
func BenchmarkFlexLogError_Error(b *testing.B) {
	err := &FlexLogError{
		Code:        ErrCodeFileWrite,
		Op:          "write",
		Path:        "/tmp/test.log",
		Err:         fmt.Errorf("disk full"),
		Time:        time.Now(),
		Destination: "main-log",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = err.Error()
	}
}

func BenchmarkIsRetryable(b *testing.B) {
	err := NewFlexLogError(ErrCodeChannelFull, "write", "", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = IsRetryable(err)
	}
}

func BenchmarkContains(b *testing.B) {
	s := "resource temporarily unavailable - please try again"
	substr := "temporarily unavailable"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = contains(s, substr)
	}
}
