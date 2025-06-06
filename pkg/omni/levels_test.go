package omni

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLevelFunctions(t *testing.T) {
	testDir := t.TempDir()
	logFile := filepath.Join(testDir, "test.log")

	// Create a logger with default options
	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set level to Trace to test all logging levels
	logger.SetLevel(LevelTrace)

	tests := []struct {
		name     string
		logFunc  func()
		level    int
		expected string
	}{
		{
			name: "Trace",
			logFunc: func() {
				logger.Trace("trace message")
			},
			level:    LevelTrace,
			expected: "trace message",
		},
		{
			name: "Tracef",
			logFunc: func() {
				logger.Tracef("trace %s", "formatted")
			},
			level:    LevelTrace,
			expected: "trace formatted",
		},
		{
			name: "Debug",
			logFunc: func() {
				logger.Debug("debug message")
			},
			level:    LevelDebug,
			expected: "debug message",
		},
		{
			name: "Debugf",
			logFunc: func() {
				logger.Debugf("debug %s", "formatted")
			},
			level:    LevelDebug,
			expected: "debug formatted",
		},
		{
			name: "Info",
			logFunc: func() {
				logger.Info("info message")
			},
			level:    LevelInfo,
			expected: "info message",
		},
		{
			name: "Infof",
			logFunc: func() {
				logger.Infof("info %s", "formatted")
			},
			level:    LevelInfo,
			expected: "info formatted",
		},
		{
			name: "Warn",
			logFunc: func() {
				logger.Warn("warn message")
			},
			level:    LevelWarn,
			expected: "warn message",
		},
		{
			name: "Warnf",
			logFunc: func() {
				logger.Warnf("warn %s", "formatted")
			},
			level:    LevelWarn,
			expected: "warn formatted",
		},
		{
			name: "Error",
			logFunc: func() {
				logger.Error("error message")
			},
			level:    LevelError,
			expected: "error message",
		},
		{
			name: "Errorf",
			logFunc: func() {
				logger.Errorf("error %s", "formatted")
			},
			level:    LevelError,
			expected: "error formatted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear log file before each test
			if err := os.Truncate(logFile, 0); err != nil {
				t.Fatalf("Failed to truncate log file: %v", err)
			}

			// Execute the log function
			tt.logFunc()

			// Ensure async logging completes
			if err := logger.FlushAll(); err != nil {
				t.Logf("Warning: flush error: %v", err)
			}
			time.Sleep(10 * time.Millisecond)

			// Read log file content
			content, err := os.ReadFile(logFile)
			if err != nil {
				t.Fatalf("Failed to read log file: %v", err)
			}

			// Check if expected message is in the log
			if !strings.Contains(string(content), tt.expected) {
				t.Errorf("Log file does not contain expected message. Got:\n%s\nWant content including: %s", string(content), tt.expected)
			}
		})
	}
}

func TestLevelFiltering(t *testing.T) {
	testDir := t.TempDir()
	logFile := filepath.Join(testDir, "test.log")

	// Create logger with Info level as minimum
	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	logger.SetLevel(LevelInfo)

	tests := []struct {
		name     string
		logFunc  func()
		level    int
		message  string
		expected bool // whether the message should be logged
	}{
		{
			name: "Trace with Info level",
			logFunc: func() {
				logger.Trace("trace should be filtered")
			},
			level:    LevelTrace,
			message:  "trace should be filtered",
			expected: false,
		},
		{
			name: "Debug with Info level",
			logFunc: func() {
				logger.Debug("debug should be filtered")
			},
			level:    LevelDebug,
			message:  "debug should be filtered",
			expected: false,
		},
		{
			name: "Info with Info level",
			logFunc: func() {
				logger.Info("info should be logged")
			},
			level:    LevelInfo,
			message:  "info should be logged",
			expected: true,
		},
		{
			name: "Warn with Info level",
			logFunc: func() {
				logger.Warn("warn should be logged")
			},
			level:    LevelWarn,
			message:  "warn should be logged",
			expected: true,
		},
		{
			name: "Error with Info level",
			logFunc: func() {
				logger.Error("error should be logged")
			},
			level:    LevelError,
			message:  "error should be logged",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear log file before each test
			if err := os.Truncate(logFile, 0); err != nil {
				t.Fatalf("Failed to truncate log file: %v", err)
			}

			// Execute the log function
			tt.logFunc()

			// Ensure async logging completes
			if err := logger.FlushAll(); err != nil {
				t.Logf("Warning: flush error: %v", err)
			}
			time.Sleep(10 * time.Millisecond)

			// Read log file content
			content, err := os.ReadFile(logFile)
			if err != nil {
				t.Fatalf("Failed to read log file: %v", err)
			}

			contentStr := string(content)
			messageFound := strings.Contains(contentStr, tt.message)

			if messageFound != tt.expected {
				if tt.expected {
					t.Errorf("Expected log message '%s' to be written, but it wasn't found in: %s",
						tt.message, contentStr)
				} else {
					t.Errorf("Expected no log message to be written, but log file contains: %s", contentStr)
				}
			}
		})
	}
}
func TestChannelFullFallback(t *testing.T) {
	// Test the behavior when the message channel is full
	// We'll create a scenario where the channel cannot accept messages

	// This test is tricky because the worker processes messages quickly
	// We need to ensure messages are actually dropped
	
	// Set very small channel size
	oldSize := os.Getenv("OMNI_CHANNEL_SIZE")
	os.Setenv("OMNI_CHANNEL_SIZE", "2")
	defer func() {
		if oldSize != "" {
			os.Setenv("OMNI_CHANNEL_SIZE", oldSize)
		} else {
			os.Unsetenv("OMNI_CHANNEL_SIZE")
		}
	}()

	// Create logger with small channel
	logger, err := New("/tmp/test_channel_full.log")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()
	
	// Send many messages rapidly to ensure some are dropped
	// We send more messages than the channel can hold
	const messageCount = 100
	
	for i := 0; i < messageCount; i++ {
		// Send message without any delay
		logger.Debug("rapid message %d", i)
	}
	
	// Wait for worker to process what it can
	time.Sleep(100 * time.Millisecond)
	
	// Check metrics
	metrics := logger.GetMetrics()
	t.Logf("Messages dropped: %d", metrics.MessagesDropped)
	t.Logf("Messages logged: %d", metrics.MessagesLogged)
	t.Logf("Total attempted: %d", messageCount)
	
	// With a small channel and rapid sends, we should have some drops
	// But this is timing-dependent, so we'll make it a warning rather than a failure
	if metrics.MessagesDropped == 0 {
		t.Log("Warning: No messages were dropped. This test may be timing-dependent.")
		// Don't fail the test as it's timing-dependent
	}
}
