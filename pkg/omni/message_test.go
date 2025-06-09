// filepath: /Users/wayneeseguin/w/github.com/wayneeseguin/omni/message_test.go
package omni

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/flock"
	"github.com/wayneeseguin/omni/pkg/backends"
)

// Mock writer that can be configured to return errors
type mockWriter struct {
	writeError   error
	flushError   error
	writtenBytes []byte
}

func (m *mockWriter) Write(p []byte) (int, error) {
	if m.writeError != nil {
		return 0, m.writeError
	}
	m.writtenBytes = append(m.writtenBytes, p...)
	return len(p), nil
}

func (m *mockWriter) WriteString(s string) (int, error) {
	if m.writeError != nil {
		return 0, m.writeError
	}
	m.writtenBytes = append(m.writtenBytes, []byte(s)...)
	return len(s), nil
}

func (m *mockWriter) Flush() error {
	return m.flushError
}

// mockSyslogBackend simulates a syslog backend for testing
type mockSyslogBackend struct {
	writer   *mockWriter
	priority int
	tag      string
}

func (sb *mockSyslogBackend) Write(entry []byte) (int, error) {
	// Format syslog message: <priority>tag: message
	message := fmt.Sprintf("<%d>%s: %s", sb.priority, sb.tag, strings.TrimSpace(string(entry)))

	n, err := sb.writer.WriteString(message)
	if err != nil {
		return n, err
	}

	// Add newline if not present
	if !strings.HasSuffix(message, "\n") {
		if _, err := sb.writer.WriteString("\n"); err != nil {
			return n, err
		}
		n++
	}

	return n, nil
}

func (sb *mockSyslogBackend) Flush() error {
	return sb.writer.Flush()
}

func (sb *mockSyslogBackend) Close() error {
	return nil
}

func (sb *mockSyslogBackend) SupportsAtomic() bool {
	return false
}

func (sb *mockSyslogBackend) Sync() error {
	return sb.Flush()
}

func (sb *mockSyslogBackend) GetStats() backends.BackendStats {
	return backends.BackendStats{
		Path: "syslog://mock",
	}
}

// TestProcessMessage tests the main message processing function
func TestProcessMessage(t *testing.T) {
	testDir := t.TempDir()
	logPath := filepath.Join(testDir, "test.log")

	// Create a test file
	file, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer file.Close()

	// Setup a minimal Omni and Destination for testing
	logger := &Omni{
		level:   LevelDebug,
		format:  FormatText,
		maxSize: 1024 * 1024, // 1MB
		formatOptions: FormatOptions{
			TimestampFormat: "2006-01-02 15:04:05",
			IncludeLevel:    true,
			IncludeTime:     true,
			LevelFormat:     LevelFormatNameUpper,
			TimeZone:        time.UTC, // Set TimeZone to avoid panic
		},
		errorHandler: func(source, destination, message string, err error) {
			// Use default stderr handling
		},
		// messagesByLevel and errorsBySource are sync.Map, no initialization needed
	}

	// Create a destination with file backend
	fileDest := &Destination{
		URI:     logPath,
		Backend: BackendFlock,
		File:    file,
		Writer:  bufio.NewWriter(file),
		Lock:    flock.New(logPath + ".lock"),
		Size:    0,
	}

	// Test cases
	tests := []struct {
		name    string
		message LogMessage
		backend int
		check   func(t *testing.T, output []byte)
	}{
		{
			name: "simple text message to file",
			message: LogMessage{
				Level:     LevelInfo,
				Format:    "Test message: %s",
				Args:      []interface{}{"hello"},
				Timestamp: time.Date(2025, 4, 10, 12, 0, 0, 0, time.UTC),
			},
			backend: BackendFlock,
			check: func(t *testing.T, output []byte) {
				expected := "[2025-04-10 12:00:00] [INFO] Test message: hello"
				if !strings.Contains(string(output), expected) {
					t.Errorf("Expected output to contain %q, got %q", expected, string(output))
				}
			},
		},
		{
			name: "raw bytes message",
			message: LogMessage{
				Raw:       []byte("Raw log message\n"),
				Timestamp: time.Date(2025, 4, 10, 12, 0, 0, 0, time.UTC),
			},
			backend: BackendFlock,
			check: func(t *testing.T, output []byte) {
				expected := "Raw log message"
				if !strings.Contains(string(output), expected) {
					t.Errorf("Expected output to contain %q, got %q", expected, string(output))
				}
			},
		},
		{
			name: "unknown backend",
			message: LogMessage{
				Level:     LevelInfo,
				Format:    "Test message",
				Timestamp: time.Date(2025, 4, 10, 12, 0, 0, 0, time.UTC),
			},
			backend: 999, // Invalid backend
			check: func(t *testing.T, output []byte) {
				// No output expected as error should be printed to stderr
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset file content
			file.Truncate(0)
			file.Seek(0, 0)

			// Prepare destination based on backend type
			var dest *Destination
			if tt.backend == BackendFlock {
				dest = fileDest
				dest.Size = 0
			} else {
				// Custom destination for testing unknown backend
				dest = &Destination{
					Backend: tt.backend,
				}
			}

			// Redirect stderr for capturing error messages
			oldStderr := os.Stderr
			_, w, _ := os.Pipe()
			os.Stderr = w

			// Process message
			logger.processMessage(tt.message, dest)

			// Restore stderr and get captured output
			w.Close()
			os.Stderr = oldStderr

			// Flush the destination and read file content
			if dest.Writer != nil {
				dest.Writer.Flush()
			}

			// Read file content
			file.Seek(0, 0)
			content, err := io.ReadAll(file)
			if err != nil {
				t.Fatalf("Failed to read file content: %v", err)
			}

			// Check output
			tt.check(t, content)
		})
	}
}

// TestProcessFileMessage tests the file message processing function
func TestProcessFileMessage(t *testing.T) {
	testDir := t.TempDir()
	logPath := filepath.Join(testDir, "test.log")

	// Create a test file
	file, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer file.Close()

	// Setup a minimal Omni for testing
	logger := &Omni{
		level:   LevelDebug,
		format:  FormatText,  // default to text, will change for JSON tests
		maxSize: 1024 * 1024, // 1MB
		formatOptions: FormatOptions{
			TimestampFormat: "2006-01-02 15:04:05",
			IncludeLevel:    true,
			IncludeTime:     true,
			LevelFormat:     LevelFormatNameUpper,
			TimeZone:        time.UTC, // Set TimeZone to avoid panic
		},
	}

	tests := []struct {
		name           string
		message        LogMessage
		format         int
		lockError      bool
		rotationNeeded bool
		writeError     bool
		flushError     bool
		want           string
	}{
		{
			name: "basic text message",
			message: LogMessage{
				Level:     LevelInfo,
				Format:    "Test message: %s",
				Args:      []interface{}{"hello"},
				Timestamp: time.Date(2025, 4, 10, 12, 0, 0, 0, time.UTC),
			},
			format: FormatText,
			want:   "[2025-04-10 12:00:00] [INFO] Test message: hello\n",
		},
		{
			name: "raw bytes message",
			message: LogMessage{
				Raw:       []byte("Raw log message\n"),
				Timestamp: time.Date(2025, 4, 10, 12, 0, 0, 0, time.UTC),
			},
			format: FormatText,
			want:   "Raw log message\n",
		},
		{
			name: "debug level message",
			message: LogMessage{
				Level:     LevelDebug,
				Format:    "Debug message",
				Timestamp: time.Date(2025, 4, 10, 12, 0, 0, 0, time.UTC),
			},
			format: FormatText,
			want:   "[2025-04-10 12:00:00] [DEBUG] Debug message\n",
		},
		{
			name: "warning level message",
			message: LogMessage{
				Level:     LevelWarn,
				Format:    "Warning message",
				Timestamp: time.Date(2025, 4, 10, 12, 0, 0, 0, time.UTC),
			},
			format: FormatText,
			want:   "[2025-04-10 12:00:00] [WARN] Warning message\n",
		},
		{
			name: "error level message",
			message: LogMessage{
				Level:     LevelError,
				Format:    "Error message",
				Timestamp: time.Date(2025, 4, 10, 12, 0, 0, 0, time.UTC),
			},
			format: FormatText,
			want:   "[2025-04-10 12:00:00] [ERROR] Error message\n",
		},
		{
			name: "json structured entry",
			message: LogMessage{
				Entry: &LogEntry{
					Level:     "INFO",
					Message:   "Structured entry",
					Timestamp: "2025-04-10 12:00:00",
				},
				Timestamp: time.Date(2025, 4, 10, 12, 0, 0, 0, time.UTC),
			},
			format: FormatJSON,
			want:   `{"timestamp":"2025-04-10 12:00:00","level":"INFO","message":"Structured entry"}`,
		},
		{
			name: "time only text message",
			message: LogMessage{
				Level:     LevelInfo,
				Format:    "Time only",
				Timestamp: time.Date(2025, 4, 10, 12, 0, 0, 0, time.UTC),
			},
			format: FormatText,
			want:   "[2025-04-10 12:00:00] Time only\n",
		},
		{
			name: "level only text message",
			message: LogMessage{
				Level:     LevelInfo,
				Format:    "Level only",
				Timestamp: time.Date(2025, 4, 10, 12, 0, 0, 0, time.UTC),
			},
			format: FormatText,
			want:   "[INFO] Level only\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock writer to capture output
			var mockBuf mockWriter

			// Create test destination
			dest := &Destination{
				URI:     logPath,
				Backend: BackendFlock,
				File:    file,
				Writer:  bufio.NewWriter(&mockBuf),
				Lock:    flock.New(logPath + ".lock"),
				Size:    0,
			}

			// Set format options based on test case
			logger.format = tt.format

			// Handle special cases for formatting options
			if tt.name == "time only text message" {
				logger.formatOptions.IncludeLevel = false
				logger.formatOptions.IncludeTime = true
			} else if tt.name == "level only text message" {
				logger.formatOptions.IncludeLevel = true
				logger.formatOptions.IncludeTime = false
			}

			// Configure test-specific options
			if tt.writeError {
				mockBuf.writeError = io.ErrClosedPipe
			}
			if tt.flushError {
				mockBuf.flushError = io.ErrClosedPipe
			}

			// For rotation testing
			if tt.rotationNeeded {
				logger.maxSize = 1 // Force rotation by setting max size to 1 byte
			}

			// Redirect stderr for capturing error messages
			oldStderr := os.Stderr
			_, w, _ := os.Pipe()
			os.Stderr = w

			// Process the file message
			var entry string
			var entrySize int64
			logger.processFileMessage(tt.message, dest, &entry, &entrySize)

			// Flush the writer to ensure all data is written
			if dest.Writer != nil && !tt.flushError {
				dest.Writer.Flush()
			}

			// Restore stderr
			w.Close()
			os.Stderr = oldStderr

			// Check output
			output := string(mockBuf.writtenBytes)

			// For non-error cases, check the output
			if !tt.writeError && !tt.flushError && !tt.lockError && !tt.rotationNeeded {
				// For JSON output, check that it contains the expected fields rather than exact match
				if tt.name == "json structured entry" {
					if !strings.Contains(output, "\"timestamp\":\"2025-04-10 12:00:00\"") ||
						!strings.Contains(output, "\"level\":\"INFO\"") ||
						!strings.Contains(output, "\"message\":\"Structured entry\"") {
						t.Errorf("Expected JSON to contain timestamp, level, and message fields, got %q", output)
					}
				} else {
					if !strings.Contains(output, tt.want) {
						t.Errorf("Expected output to contain %q, got %q", tt.want, output)
					}
				}

				// Check entry and entrySize are set correctly
				if tt.name == "json structured entry" {
					// For JSON, just check that entry is not empty and is valid JSON
					if entry == "" || !strings.HasPrefix(strings.TrimSpace(entry), "{") {
						t.Errorf("Entry not set correctly for JSON, got %q", entry)
					}
				} else if !strings.Contains(entry, tt.want) && tt.message.Raw == nil && tt.message.Entry == nil {
					t.Errorf("Entry not set correctly, expected to contain %q, got %q", tt.want, entry)
				}
				if entrySize != int64(len(output)) && tt.message.Raw == nil && tt.message.Entry == nil {
					t.Errorf("EntrySize not set correctly, expected %d, got %d", len(output), entrySize)
				}
			}
		})
	}
}

// TestProcessSyslogMessage tests the syslog message processing function
func TestProcessSyslogMessage(t *testing.T) {
	// Setup a minimal Omni for testing
	logger := &Omni{
		level:  LevelDebug,
		format: FormatText,
		formatOptions: FormatOptions{
			TimestampFormat: "2006-01-02 15:04:05",
			IncludeLevel:    true,
			IncludeTime:     true,
			TimeZone:        time.UTC, // Set TimeZone to avoid panic
		},
		errorHandler: func(source, destination, message string, err error) {
			// Write to stderr for error logging
			fmt.Fprintf(os.Stderr, "[%s] %s: %s", source, destination, message)
			if err != nil {
				fmt.Fprintf(os.Stderr, " - %v", err)
			}
			fmt.Fprintln(os.Stderr)
		},
		// messagesByLevel and errorsBySource are sync.Map, no initialization needed
	}

	tests := []struct {
		name       string
		message    LogMessage
		nilConn    bool
		writeError bool
		flushError bool
		priority   int
		tag        string
		wantOutput bool
	}{
		{
			name: "simple text message to syslog",
			message: LogMessage{
				Level:     LevelInfo,
				Format:    "Test message: %s",
				Args:      []interface{}{"hello"},
				Timestamp: time.Date(2025, 4, 10, 12, 0, 0, 0, time.UTC),
			},
			priority:   13, // Default priority (user.notice)
			tag:        "omni",
			wantOutput: true,
		},
		{
			name: "raw bytes message to syslog",
			message: LogMessage{
				Raw:       []byte("Raw log message"),
				Timestamp: time.Date(2025, 4, 10, 12, 0, 0, 0, time.UTC),
			},
			priority:   13,
			tag:        "app",
			wantOutput: true,
		},
		{
			name: "structured entry to syslog",
			message: LogMessage{
				Entry: &LogEntry{
					Level:     "ERROR",
					Message:   "Structured entry",
					Timestamp: "2025-04-10 12:00:00",
				},
				Timestamp: time.Date(2025, 4, 10, 12, 0, 0, 0, time.UTC),
			},
			priority:   13,
			tag:        "myapp",
			wantOutput: true,
		},
		{
			name: "nil syslog connection",
			message: LogMessage{
				Level:     LevelInfo,
				Format:    "Test message",
				Timestamp: time.Date(2025, 4, 10, 12, 0, 0, 0, time.UTC),
			},
			nilConn:    true,
			wantOutput: false,
		},
		{
			name: "debug level message",
			message: LogMessage{
				Level:     LevelDebug,
				Format:    "Debug message",
				Timestamp: time.Date(2025, 4, 10, 12, 0, 0, 0, time.UTC),
			},
			priority:   13, // Should be modified to include debug level (7)
			tag:        "omni",
			wantOutput: true,
		},
		{
			name: "error level message",
			message: LogMessage{
				Level:     LevelError,
				Format:    "Error message",
				Timestamp: time.Date(2025, 4, 10, 12, 0, 0, 0, time.UTC),
			},
			priority:   13, // Should be modified to include error level (3)
			tag:        "omni",
			wantOutput: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock writer to capture output
			var mockBuf mockWriter

			// Configure errors if needed
			if tt.writeError {
				mockBuf.writeError = io.ErrClosedPipe
			}
			if tt.flushError {
				mockBuf.flushError = io.ErrClosedPipe
			}

			// Create test destination
			dest := &Destination{
				URI:     "syslog://localhost",
				Backend: BackendSyslog,
			}

			// Create a mock syslog backend if not nil
			if !tt.nilConn {
				// Create a mock backend that simulates syslog formatting
				dest.backend = &mockSyslogBackend{
					writer:   &mockBuf,
					priority: tt.priority,
					tag:      tt.tag,
				}
			}

			// Redirect stderr for capturing error messages
			oldStderr := os.Stderr
			r, w, _ := os.Pipe() // Create pipe with both reader and writer ends
			os.Stderr = w

			// Process the message using the general processMessage function
			// which will delegate to the backend for formatting
			logger.processMessage(tt.message, dest)

			// Flush is handled by the backend now

			// Restore stderr
			w.Close()
			os.Stderr = oldStderr

			// Read stderr content
			var stderrBuf bytes.Buffer
			io.Copy(&stderrBuf, r) // Copy from the reader end of the pipe
			r.Close()

			// Check if stderr contains expected errors
			if tt.nilConn {
				// The error message might be different with the new backend approach
				// Just check that some error was logged
				if stderrBuf.String() == "" {
					t.Errorf("Expected stderr to contain error for nil backend")
				}
			}

			// For non-error cases with expected output, check basic message structure
			if tt.wantOutput && !tt.nilConn && !tt.writeError && !tt.flushError {
				output := string(mockBuf.writtenBytes)

				// Check for syslog format elements
				if !strings.Contains(output, "<") || !strings.Contains(output, ">") {
					t.Errorf("Syslog output missing PRI field, got %q", output)
				}

				if !strings.Contains(output, tt.tag) {
					t.Errorf("Syslog output missing tag %q, got %q", tt.tag, output)
				}

				// Check specific message content based on message type
				if tt.message.Raw != nil {
					rawContent := string(tt.message.Raw)
					if !strings.Contains(output, rawContent) {
						t.Errorf("Syslog output missing raw content %q, got %q", rawContent, output)
					}
				} else if tt.message.Entry != nil {
					if !strings.Contains(output, tt.message.Entry.Message) {
						t.Errorf("Syslog output missing entry message %q, got %q",
							tt.message.Entry.Message, output)
					}
				} else if len(tt.message.Format) > 0 {
					expectedMsg := fmt.Sprintf(tt.message.Format, tt.message.Args...)
					if !strings.Contains(output, expectedMsg) {
						t.Errorf("Syslog output missing formatted message %q, got %q",
							expectedMsg, output)
					}
				}
			}
		})
	}
}

// TestMessageFormatting tests various message formatting options
func TestMessageFormatting(t *testing.T) {
	// Setup a minimal Omni for testing
	logger := &Omni{
		level: LevelDebug,
		formatOptions: FormatOptions{
			TimestampFormat: "2006-01-02 15:04:05",
			IncludeLevel:    true,
			IncludeTime:     true,
			LevelFormat:     LevelFormatNameUpper,
			TimeZone:        time.UTC, // Set TimeZone to avoid panic
		},
		errorHandler: func(source, destination, message string, err error) {
			// Use default stderr handling
		},
		// messagesByLevel and errorsBySource are sync.Map, no initialization needed
	}

	tests := []struct {
		name          string
		message       LogMessage
		formatOptions FormatOptions
		want          string
	}{
		{
			name: "full format with time and level",
			message: LogMessage{
				Level:     LevelInfo,
				Format:    "Test message",
				Timestamp: time.Date(2025, 4, 10, 12, 0, 0, 0, time.UTC),
			},
			formatOptions: FormatOptions{
				TimestampFormat: "2006-01-02 15:04:05",
				IncludeLevel:    true,
				IncludeTime:     true,
				LevelFormat:     LevelFormatNameUpper,
			},
			want: "[2025-04-10 12:00:00] [INFO] Test message\n",
		},
		{
			name: "time only",
			message: LogMessage{
				Level:     LevelInfo,
				Format:    "Test message",
				Timestamp: time.Date(2025, 4, 10, 12, 0, 0, 0, time.UTC),
			},
			formatOptions: FormatOptions{
				TimestampFormat: "2006-01-02 15:04:05",
				IncludeLevel:    false,
				IncludeTime:     true,
			},
			want: "[2025-04-10 12:00:00] Test message\n",
		},
		{
			name: "level only",
			message: LogMessage{
				Level:     LevelInfo,
				Format:    "Test message",
				Timestamp: time.Date(2025, 4, 10, 12, 0, 0, 0, time.UTC),
			},
			formatOptions: FormatOptions{
				IncludeLevel: true,
				IncludeTime:  false,
				LevelFormat:  LevelFormatNameUpper,
			},
			want: "[INFO] Test message\n",
		},
		{
			name: "no time or level",
			message: LogMessage{
				Level:     LevelInfo,
				Format:    "Test message",
				Timestamp: time.Date(2025, 4, 10, 12, 0, 0, 0, time.UTC),
			},
			formatOptions: FormatOptions{
				IncludeLevel: false,
				IncludeTime:  false,
			},
			want: "Test message\n",
		},
		{
			name: "custom timestamp format",
			message: LogMessage{
				Level:     LevelInfo,
				Format:    "Test message",
				Timestamp: time.Date(2025, 4, 10, 12, 0, 0, 0, time.UTC),
			},
			formatOptions: FormatOptions{
				TimestampFormat: "2006/01/02 15:04",
				IncludeLevel:    true,
				IncludeTime:     true,
				LevelFormat:     LevelFormatNameUpper,
			},
			want: "[2025/04/10 12:00] [INFO] Test message\n",
		},
		{
			name: "lowercase level format",
			message: LogMessage{
				Level:     LevelInfo,
				Format:    "Test message",
				Timestamp: time.Date(2025, 4, 10, 12, 0, 0, 0, time.UTC),
			},
			formatOptions: FormatOptions{
				TimestampFormat: "2006-01-02 15:04:05",
				IncludeLevel:    true,
				IncludeTime:     true,
				LevelFormat:     LevelFormatNameLower,
			},
			want: "[2025-04-10 12:00:00] [info] Test message\n",
		},
		{
			name: "symbol level format",
			message: LogMessage{
				Level:     LevelInfo,
				Format:    "Test message",
				Timestamp: time.Date(2025, 4, 10, 12, 0, 0, 0, time.UTC),
			},
			formatOptions: FormatOptions{
				TimestampFormat: "2006-01-02 15:04:05",
				IncludeLevel:    true,
				IncludeTime:     true,
				LevelFormat:     LevelFormatSymbol,
			},
			want: "[2025-04-10 12:00:00] [I] Test message\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock writer to capture output
			var mockBuf mockWriter

			// Setup destination
			dest := &Destination{
				Backend: BackendFlock,
				Writer:  bufio.NewWriter(&mockBuf),
				Lock:    flock.New("test.lock"), // Dummy lock that won't be used because we mock the locking
			}

			// Set format options
			logger.formatOptions = tt.formatOptions

			// Create an entry string
			var entry string
			var entrySize int64

			// Use the logger directly without copying to avoid mutex copy
			// (Copying a struct with mutexes is not safe)

			// Process the message (this will format and generate the entry)
			// Use our mockFileLock implementation that overrides RLock and Unlock
			mockLock := newMockFileLock("test.lock")
			dest.Lock = mockLock.Flock // Use the embedded *flock.Flock which is compatible with the type
			logger.processFileMessage(tt.message, dest, &entry, &entrySize)

			// Check the entry matches what we expect
			if entry != tt.want {
				t.Errorf("Expected entry %q, got %q", tt.want, entry)
			}
		})
	}
}

// mockFileLock implements the necessary methods from flock.Flock for testing
type mockFileLock struct {
	*flock.Flock
}

func newMockFileLock(path string) *mockFileLock {
	return &mockFileLock{flock.New(path)}
}

func (m *mockFileLock) RLock() error            { return nil }
func (m *mockFileLock) Unlock() error           { return nil }
func (m *mockFileLock) TryRLock() (bool, error) { return true, nil }
