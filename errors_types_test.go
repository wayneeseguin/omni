package omni

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestOmniError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *OmniError
		expected string
	}{
		{
			name: "error with destination",
			err: &OmniError{
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
			err: &OmniError{
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

func TestOmniError_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")
	omniErr := &OmniError{
		Code: ErrCodeFileWrite,
		Op:   "write",
		Path: "/tmp/test.log",
		Err:  originalErr,
		Time: time.Now(),
	}

	unwrapped := omniErr.Unwrap()
	if unwrapped != originalErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, originalErr)
	}
}

func TestOmniError_Is(t *testing.T) {
	originalErr := errors.New("original error")
	omniErr1 := &OmniError{
		Code: ErrCodeFileWrite,
		Op:   "write",
		Path: "/tmp/test.log",
		Err:  originalErr,
		Time: time.Now(),
	}
	omniErr2 := &OmniError{
		Code: ErrCodeFileWrite,
		Op:   "write",
		Path: "/tmp/other.log",
		Err:  errors.New("different error"),
		Time: time.Now(),
	}
	omniErr3 := &OmniError{
		Code: ErrCodeFileFlush,
		Op:   "flush",
		Path: "/tmp/test.log",
		Err:  originalErr,
		Time: time.Now(),
	}

	tests := []struct {
		name     string
		err      *OmniError
		target   error
		expected bool
	}{
		{
			name:     "same error code",
			err:      omniErr1,
			target:   omniErr2,
			expected: true,
		},
		{
			name:     "different error code",
			err:      omniErr1,
			target:   omniErr3,
			expected: false,
		},
		{
			name:     "underlying error match",
			err:      omniErr1,
			target:   originalErr,
			expected: true,
		},
		{
			name:     "nil target",
			err:      omniErr1,
			target:   nil,
			expected: false,
		},
		{
			name:     "different error type",
			err:      omniErr1,
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

func TestNewOmniError(t *testing.T) {
	originalErr := errors.New("test error")
	omniErr := NewOmniError(ErrCodeFileWrite, "write", "/tmp/test.log", originalErr)

	if omniErr.Code != ErrCodeFileWrite {
		t.Errorf("Code = %v, want %v", omniErr.Code, ErrCodeFileWrite)
	}
	if omniErr.Op != "write" {
		t.Errorf("Op = %v, want %v", omniErr.Op, "write")
	}
	if omniErr.Path != "/tmp/test.log" {
		t.Errorf("Path = %v, want %v", omniErr.Path, "/tmp/test.log")
	}
	if omniErr.Err != originalErr {
		t.Errorf("Err = %v, want %v", omniErr.Err, originalErr)
	}
	if omniErr.Context == nil {
		t.Error("Context should be initialized")
	}
	if time.Since(omniErr.Time) > time.Second {
		t.Error("Time should be recent")
	}
}

func TestOmniError_WithDestination(t *testing.T) {
	omniErr := NewOmniError(ErrCodeFileWrite, "write", "/tmp/test.log", nil)
	result := omniErr.WithDestination("test-dest")

	if result != omniErr {
		t.Error("WithDestination should return the same instance")
	}
	if omniErr.Destination != "test-dest" {
		t.Errorf("Destination = %v, want %v", omniErr.Destination, "test-dest")
	}
}

func TestOmniError_WithContext(t *testing.T) {
	omniErr := NewOmniError(ErrCodeFileWrite, "write", "/tmp/test.log", nil)
	result := omniErr.WithContext("key", "value")

	if result != omniErr {
		t.Error("WithContext should return the same instance")
	}
	if omniErr.Context["key"] != "value" {
		t.Errorf("Context[key] = %v, want %v", omniErr.Context["key"], "value")
	}
}

func TestErrorHelperFunctions(t *testing.T) {
	originalErr := errors.New("test error")

	tests := []struct {
		name     string
		fn       func() *OmniError
		code     ErrorCode
		op       string
		path     string
		checkErr bool
	}{
		{
			name:     "ErrFileOpen",
			fn:       func() *OmniError { return ErrFileOpen("/tmp/test.log", originalErr) },
			code:     ErrCodeFileOpen,
			op:       "open",
			path:     "/tmp/test.log",
			checkErr: true,
		},
		{
			name:     "ErrFileWrite",
			fn:       func() *OmniError { return ErrFileWrite("/tmp/test.log", originalErr) },
			code:     ErrCodeFileWrite,
			op:       "write",
			path:     "/tmp/test.log",
			checkErr: true,
		},
		{
			name:     "ErrFileFlush",
			fn:       func() *OmniError { return ErrFileFlush("/tmp/test.log", originalErr) },
			code:     ErrCodeFileFlush,
			op:       "flush",
			path:     "/tmp/test.log",
			checkErr: true,
		},
		{
			name:     "ErrFileRotate",
			fn:       func() *OmniError { return ErrFileRotate("/tmp/test.log", originalErr) },
			code:     ErrCodeFileRotate,
			op:       "rotate",
			path:     "/tmp/test.log",
			checkErr: true,
		},
		{
			name:     "ErrChannelFull",
			fn:       func() *OmniError { return NewChannelFullError("write") },
			code:     ErrCodeChannelFull,
			op:       "write",
			path:     "",
			checkErr: false,
		},
		{
			name:     "ErrDestinationNotFound",
			fn:       func() *OmniError { return NewDestinationNotFoundError("test-dest") },
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
	err := NewShutdownTimeoutError(duration)

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
			err:      NewOmniError(ErrCodeChannelFull, "write", "", nil),
			expected: true,
		},
		{
			name:     "compression queue full error",
			err:      NewOmniError(ErrCodeCompressionQueueFull, "compress", "", nil),
			expected: true,
		},
		{
			name:     "file lock error",
			err:      NewOmniError(ErrCodeFileLock, "lock", "", nil),
			expected: true,
		},
		{
			name:     "file write error",
			err:      NewOmniError(ErrCodeFileWrite, "write", "", nil),
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
func BenchmarkOmniError_Error(b *testing.B) {
	err := &OmniError{
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
	err := NewOmniError(ErrCodeChannelFull, "write", "", nil)

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
