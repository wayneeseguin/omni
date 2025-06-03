package omni

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMessageProcessing(t *testing.T) {
	// Create temporary directory for tests
	tmpDir := t.TempDir()

	t.Run("FileBackend", func(t *testing.T) {
		logPath := filepath.Join(tmpDir, "test_process_file.log")
		logger, err := New(logPath)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()

		// Create a test message
		msg := LogMessage{
			Level:     LevelInfo,
			Format:    "Test message %d",
			Args:      []interface{}{42},
			Timestamp: time.Now(),
		}

		// Process the message
		dest := logger.Destinations[0]
		logger.processMessage(msg, dest)

		// Flush to ensure write
		logger.FlushAll()

		// Read the file and verify
		content, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("Failed to read log file: %v", err)
		}

		if !bytes.Contains(content, []byte("Test message 42")) {
			t.Errorf("Log file doesn't contain expected message. Got: %s", content)
		}
	})

	t.Run("CustomBackend", func(t *testing.T) {
		// Create a custom writer
		var buf bytes.Buffer
		writer := bufio.NewWriter(&buf)

		logger, err := New("/tmp/test.log")
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()

		// Add custom destination
		dest := logger.AddCustomDestination("test-custom", writer)

		// Create a test message
		msg := LogMessage{
			Level:     LevelWarn,
			Format:    "Warning: %s",
			Args:      []interface{}{"test warning"},
			Timestamp: time.Now(),
		}

		// Process the message
		logger.processMessage(msg, dest)
		writer.Flush()

		// Verify output
		output := buf.String()
		if !bytes.Contains([]byte(output), []byte("Warning: test warning")) {
			t.Errorf("Custom backend doesn't contain expected message. Got: %s", output)
		}

		if !bytes.Contains([]byte(output), []byte("WARN")) {
			t.Errorf("Custom backend doesn't contain log level. Got: %s", output)
		}
	})

	t.Run("StructuredEntry", func(t *testing.T) {
		logPath := filepath.Join(tmpDir, "test_structured.log")
		logger, err := New(logPath)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()

		// Create a structured entry
		entry := LogEntry{
			Timestamp: time.Now().Format(time.RFC3339),
			Level:     "ERROR",
			Message:   "Database connection failed",
			Fields: map[string]interface{}{
				"host":    "localhost",
				"port":    5432,
				"attempt": 3,
			},
			StackTrace: "stack trace here",
		}

		// Create message with entry
		msg := LogMessage{
			Entry:     &entry,
			Timestamp: time.Now(),
		}

		// Process the message
		dest := logger.Destinations[0]
		logger.processMessage(msg, dest)

		// Flush to ensure write
		logger.FlushAll()

		// Read and verify
		content, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("Failed to read log file: %v", err)
		}

		contentStr := string(content)
		if !bytes.Contains(content, []byte("Database connection failed")) {
			t.Errorf("Missing message in output: %s", contentStr)
		}

		if !bytes.Contains(content, []byte("host=localhost")) {
			t.Errorf("Missing field in output: %s", contentStr)
		}

		if !bytes.Contains(content, []byte("stack_trace=")) {
			t.Errorf("Missing stack trace in output: %s", contentStr)
		}
	})

	t.Run("RawBytes", func(t *testing.T) {
		logPath := filepath.Join(tmpDir, "test_raw.log")
		logger, err := New(logPath)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()

		// Create message with raw bytes
		rawData := []byte("Raw log data\n")
		msg := LogMessage{
			Raw:       rawData,
			Timestamp: time.Now(),
		}

		// Process the message
		dest := logger.Destinations[0]
		logger.processMessage(msg, dest)

		// Flush to ensure write
		logger.FlushAll()

		// Read and verify
		content, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("Failed to read log file: %v", err)
		}

		if !bytes.Equal(content, rawData) {
			t.Errorf("Raw data mismatch. Expected: %s, Got: %s", rawData, content)
		}
	})
}

func TestProcessMessageWithRotation(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test_rotation.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set small max size to trigger rotation
	logger.SetMaxSize(50) // 50 bytes

	dest := logger.Destinations[0]

	// Write messages to trigger rotation
	for i := 0; i < 5; i++ {
		msg := LogMessage{
			Level:     LevelInfo,
			Format:    "This is a longer message to trigger rotation: %d",
			Args:      []interface{}{i},
			Timestamp: time.Now(),
		}
		logger.processMessage(msg, dest)
	}

	// Flush to ensure writes
	logger.FlushAll()

	// Check that rotation occurred
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	rotatedFiles := 0
	for _, f := range files {
		if f.Name() != "test_rotation.log" && !f.IsDir() {
			rotatedFiles++
		}
	}

	if rotatedFiles == 0 {
		t.Error("No rotated files found")
	}
}

func TestProcessMessageErrors(t *testing.T) {
	t.Run("NilWriter", func(t *testing.T) {
		logger, err := New("/tmp/test.log")
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()

		// Create destination with nil writer
		dest := &Destination{
			Name:    "nil-writer",
			Backend: -1, // Custom backend
			Writer:  nil,
			Enabled: true,
		}

		// This should not panic
		msg := LogMessage{
			Level:     LevelError,
			Format:    "Test error",
			Timestamp: time.Now(),
		}

		// Process should handle nil writer gracefully
		logger.processMessage(msg, dest)
	})

	t.Run("InvalidBackend", func(t *testing.T) {
		logger, err := New("/tmp/test.log")
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()

		// Create destination with invalid backend
		dest := &Destination{
			Name:    "invalid-backend",
			Backend: 999, // Invalid backend type
			Enabled: true,
		}

		// Set custom error handler to capture errors
		var capturedError string
		logger.SetErrorHandler(func(err LogError) {
			capturedError = err.Message
		})

		msg := LogMessage{
			Level:     LevelInfo,
			Format:    "Test",
			Timestamp: time.Now(),
		}

		// Process should handle invalid backend
		logger.processMessage(msg, dest)

		if capturedError == "" {
			t.Error("Expected error for invalid backend type")
		}
	})
}

func TestProcessMessageFormatting(t *testing.T) {
	tests := []struct {
		name            string
		formatOpts      FormatOptions
		level           int
		expectedParts   []string
		unexpectedParts []string
	}{
		{
			name: "WithTimestamp",
			formatOpts: FormatOptions{
				IncludeTime:     true,
				IncludeLevel:    false,
				TimestampFormat: "15:04:05",
			},
			level:           LevelInfo,
			expectedParts:   []string{":"},
			unexpectedParts: []string{"INFO"},
		},
		{
			name: "WithLevel",
			formatOpts: FormatOptions{
				IncludeTime:  false,
				IncludeLevel: true,
				LevelFormat:  LevelFormatName,
			},
			level:         LevelWarn,
			expectedParts: []string{"WARN"},
		},
		{
			name: "LevelSymbol",
			formatOpts: FormatOptions{
				IncludeTime:  false,
				IncludeLevel: true,
				LevelFormat:  LevelFormatSymbol,
			},
			level:         LevelError,
			expectedParts: []string{"[E]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			logPath := filepath.Join(tmpDir, "test_format.log")

			logger, err := New(logPath)
			if err != nil {
				t.Fatalf("Failed to create logger: %v", err)
			}
			defer logger.Close()

			// Set format options
			logger.mu.Lock()
			logger.formatOptions = tt.formatOpts
			logger.mu.Unlock()

			// Process message
			msg := LogMessage{
				Level:     tt.level,
				Format:    "Test formatting",
				Timestamp: time.Now(),
			}

			dest := logger.Destinations[0]
			logger.processMessage(msg, dest)
			logger.FlushAll()

			// Read and verify
			content, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatalf("Failed to read log file: %v", err)
			}

			contentStr := string(content)

			// Check expected parts
			for _, part := range tt.expectedParts {
				if !bytes.Contains(content, []byte(part)) {
					t.Errorf("Expected '%s' in output, got: %s", part, contentStr)
				}
			}

			// Check unexpected parts
			for _, part := range tt.unexpectedParts {
				if bytes.Contains(content, []byte(part)) {
					t.Errorf("Unexpected '%s' in output, got: %s", part, contentStr)
				}
			}
		})
	}
}
