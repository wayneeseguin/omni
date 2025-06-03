package omni

import (
	"errors"
	"testing"
)

func TestFileError(t *testing.T) {
	baseErr := errors.New("permission denied")

	tests := []struct {
		name     string
		err      *FileError
		wantMsg  string
		wantPath string
		wantOp   string
	}{
		{
			name: "open error",
			err: &FileError{
				Op:   "open",
				Path: "/var/log/app.log",
				Err:  baseErr,
			},
			wantMsg:  "file open error on /var/log/app.log: permission denied",
			wantPath: "/var/log/app.log",
			wantOp:   "open",
		},
		{
			name: "write error",
			err: &FileError{
				Op:   "write",
				Path: "/tmp/test.log",
				Err:  errors.New("disk full"),
			},
			wantMsg:  "file write error on /tmp/test.log: disk full",
			wantPath: "/tmp/test.log",
			wantOp:   "write",
		},
		{
			name: "rotate error",
			err: &FileError{
				Op:   "rename",
				Path: "/var/log/app.log",
				Err:  errors.New("file exists"),
			},
			wantMsg:  "file rename error on /var/log/app.log: file exists",
			wantPath: "/var/log/app.log",
			wantOp:   "rename",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("FileError.Error() = %v, want %v", got, tt.wantMsg)
			}

			if tt.err.Path != tt.wantPath {
				t.Errorf("FileError.Path = %v, want %v", tt.err.Path, tt.wantPath)
			}

			if tt.err.Op != tt.wantOp {
				t.Errorf("FileError.Op = %v, want %v", tt.err.Op, tt.wantOp)
			}

			// Test Unwrap
			if unwrapped := tt.err.Unwrap(); unwrapped == nil {
				t.Error("FileError.Unwrap() returned nil")
			}
		})
	}
}

func TestDestinationError(t *testing.T) {
	baseErr := errors.New("connection refused")

	tests := []struct {
		name     string
		err      *DestinationError
		wantMsg  string
		wantName string
		wantOp   string
	}{
		{
			name: "write error",
			err: &DestinationError{
				Name: "syslog-dest",
				Op:   "write",
				Err:  baseErr,
			},
			wantMsg:  "destination syslog-dest: write failed: connection refused",
			wantName: "syslog-dest",
			wantOp:   "write",
		},
		{
			name: "flush error",
			err: &DestinationError{
				Name: "file-dest",
				Op:   "flush",
				Err:  errors.New("io timeout"),
			},
			wantMsg:  "destination file-dest: flush failed: io timeout",
			wantName: "file-dest",
			wantOp:   "flush",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("DestinationError.Error() = %v, want %v", got, tt.wantMsg)
			}

			if tt.err.Name != tt.wantName {
				t.Errorf("DestinationError.Name = %v, want %v", tt.err.Name, tt.wantName)
			}

			if tt.err.Op != tt.wantOp {
				t.Errorf("DestinationError.Op = %v, want %v", tt.err.Op, tt.wantOp)
			}

			// Test Unwrap
			if unwrapped := tt.err.Unwrap(); unwrapped == nil {
				t.Error("DestinationError.Unwrap() returned nil")
			}
		})
	}
}

func TestConfigError(t *testing.T) {
	tests := []struct {
		name      string
		err       *ConfigError
		wantMsg   string
		wantField string
		wantValue interface{}
	}{
		{
			name: "invalid size",
			err: &ConfigError{
				Field: "maxSize",
				Value: -1,
				Err:   errors.New("must be positive"),
			},
			wantMsg:   "config error: field maxSize with value -1: must be positive",
			wantField: "maxSize",
			wantValue: -1,
		},
		{
			name: "invalid path",
			err: &ConfigError{
				Field: "logPath",
				Value: "",
				Err:   errors.New("cannot be empty"),
			},
			wantMsg:   "config error: field logPath with value : cannot be empty",
			wantField: "logPath",
			wantValue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("ConfigError.Error() = %v, want %v", got, tt.wantMsg)
			}

			if tt.err.Field != tt.wantField {
				t.Errorf("ConfigError.Field = %v, want %v", tt.err.Field, tt.wantField)
			}

			if tt.err.Value != tt.wantValue {
				t.Errorf("ConfigError.Value = %v, want %v", tt.err.Value, tt.wantValue)
			}

			// Test Unwrap
			if unwrapped := tt.err.Unwrap(); unwrapped == nil {
				t.Error("ConfigError.Unwrap() returned nil")
			}
		})
	}
}

func TestCommonErrors(t *testing.T) {
	// Test that all common errors are defined
	commonErrors := []struct {
		name string
		err  error
	}{
		{"ErrLoggerClosed", ErrLoggerClosed},
		{"ErrInvalidDestination", ErrInvalidDestination},
		{"ErrDestinationExists", ErrDestinationExists},
		{"ErrInvalidIndex", ErrInvalidIndex},
		{"ErrChannelFull", ErrChannelFull},
		{"ErrNilWriter", ErrNilWriter},
		{"ErrRotationFailed", ErrRotationFailed},
		{"ErrCompressionFailed", ErrCompressionFailed},
		{"ErrInvalidConfiguration", ErrInvalidConfiguration},
	}

	for _, tt := range commonErrors {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Errorf("%s is nil", tt.name)
			}

			// Verify error message is not empty
			if tt.err.Error() == "" {
				t.Errorf("%s has empty error message", tt.name)
			}
		})
	}
}

func TestErrorWrapping(t *testing.T) {
	baseErr := errors.New("base error")

	fileErr := &FileError{
		Op:   "test",
		Path: "/test",
		Err:  baseErr,
	}

	// Test that we can use errors.Is
	if !errors.Is(fileErr, fileErr) {
		t.Error("errors.Is failed for FileError")
	}

	// Test that we can unwrap to get the base error
	var unwrappedFileErr *FileError
	if errors.As(fileErr, &unwrappedFileErr) {
		if unwrappedFileErr != fileErr {
			t.Error("errors.As failed to match FileError")
		}
	}
}
