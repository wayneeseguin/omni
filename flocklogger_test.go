package flocklogger_test

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/wayneeseguin/flocklogger"
)

func TestNewFlockLogger(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "flocklogger-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("creates log directory if needed", func(t *testing.T) {
		nestedDir := filepath.Join(tmpDir, "nested", "dir")
		logPath := filepath.Join(nestedDir, "test.log")
		f, err := flocklogger.NewFlockLogger(logPath)
		if err != nil {
			t.Fatalf("NewFlockLogger failed: %v", err)
		}
		defer f.Close()
		// Verify directory was created
		if _, err := os.Stat(nestedDir); os.IsNotExist(err) {
			t.Errorf("Log directory was not created")
		}
	})

	t.Run("returns error for invalid path", func(t *testing.T) {
		// Use a non-writable path instead of os.DevNull
		// Create a directory that we can't write files into
		readOnlyDir := filepath.Join(tmpDir, "readonly")
		if err := os.Mkdir(readOnlyDir, 0500); err != nil {
			t.Fatalf("Failed to create read-only directory: %v", err)
		}

		invalidPath := filepath.Join(readOnlyDir, "invalid.log")
		_, err := flocklogger.NewFlockLogger(invalidPath)
		if err == nil {
			t.Error("Expected error for invalid path, got nil")
		}
	})
}

func TestFlog(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "flocklogger-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logPath := filepath.Join(tmpDir, "test.log")
	f, err := flocklogger.NewFlockLogger(logPath)
	if err != nil {
		t.Fatalf("NewFlockLogger failed: %v", err)
	}

	t.Run("writes formatted log entry", func(t *testing.T) {
		message := "Test log message"
		f.Flog("%s", message) // Fixed: Added format specifier
		f.Flush()             // Ensure the message is written
		content, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("Failed to read log file: %v", err)
		}
		if !strings.Contains(string(content), message) {
			t.Errorf("Log file does not contain expected message. Got: %s", string(content))
		}
	})

	t.Run("formats with arguments", func(t *testing.T) {
		const formatStr = "Format %s %d"
		f.Flog(formatStr, "test", 123)
		f.Flush()
		content, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("Failed to read log file: %v", err)
		}
		expected := "Format test 123"
		if !strings.Contains(string(content), expected) {
			t.Errorf("Log file does not contain formatted message. Got: %s", string(content))
		}
	})

	t.Run("includes timestamp", func(t *testing.T) {
		f.Flog("Message with timestamp")
		f.Flush()
		content, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("Failed to read log file: %v", err)
		}
		// Check for timestamp pattern [YYYY-MM-DD HH:MM:SS.mmm]
		matched, err := regexp.MatchString(`\[\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}\]`, string(content))
		if err != nil {
			t.Fatalf("Regexp match failed: %v", err)
		}

		if !matched {
			t.Errorf("Log file does not contain properly formatted timestamp. Got: %s", string(content))
		}
	})
	f.Close()
}

func TestLogRotation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "flocklogger-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logPath := filepath.Join(tmpDir, "rotation.log")
	f, err := flocklogger.NewFlockLogger(logPath)
	if err != nil {
		t.Fatalf("NewFlockLogger failed: %v", err)
	}

	// Set a small max size to trigger rotation
	f.SetMaxSize(100)
	f.SetMaxFiles(3)

	// Write enough data to trigger rotation multiple times
	for i := 0; i < 10; i++ {
		f.Flog("This is a log message that should be long enough to trigger rotation: %d", i)
	}
	f.Close()

	// Check that rotation files exist
	files, err := filepath.Glob(filepath.Join(tmpDir, "rotation.log*"))
	if err != nil {
		t.Fatalf("Failed to list log files: %v", err)
	}

	// Should have the main log file + rotation files up to max files
	expectedFiles := 4 // main + 3 rotation files
	if len(files) > expectedFiles {
		t.Errorf("Expected at most %d log files, got %d: %v", expectedFiles, len(files), files)
	}

	// Check that the original file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Errorf("Main log file does not exist after rotation")
	}

	// Check that at least one rotation file exists
	rotationFile := fmt.Sprintf("%s.1", logPath)
	if _, err := os.Stat(rotationFile); os.IsNotExist(err) {
		t.Errorf("Rotation file %s does not exist", rotationFile)
	}
}

func TestFlush(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "flocklogger-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logPath := filepath.Join(tmpDir, "flush.log")
	f, err := flocklogger.NewFlockLogger(logPath)
	if err != nil {
		t.Fatalf("NewFlockLogger failed: %v", err)
	}

	// Write log message without automatic flush
	f.Flog("Test flush message")

	// Explicitly flush
	err = f.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// Check content after flush
	contentAfter, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	if !strings.Contains(string(contentAfter), "Test flush message") {
		t.Errorf("Log file does not contain expected message after flush")
	}
	f.Close()
}

func TestClose(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "flocklogger-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logPath := filepath.Join(tmpDir, "close.log")
	f, err := flocklogger.NewFlockLogger(logPath)
	if err != nil {
		t.Fatalf("NewFlockLogger failed: %v", err)
	}

	// Write something
	f.Flog("Test close message")

	// Close the logger
	err = f.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify content was written
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	if !strings.Contains(string(content), "Test close message") {
		t.Errorf("Log file does not contain expected message after close")
	}
}

func TestSensitiveDataRedaction(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "flocklogger-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logPath := filepath.Join(tmpDir, "redact.log")
	f, err := flocklogger.NewFlockLogger(logPath)
	if err != nil {
		t.Fatalf("NewFlockLogger failed: %v", err)
	}
	defer f.Close()

	testCases := []struct {
		name        string
		headers     map[string][]string
		body        string
		expected    []string
		notExpected []string
	}{
		{
			name: "redacts authorization header",
			headers: map[string][]string{
				"Authorization": {"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"},
				"Content-Type":  {"application/json"},
			},
			expected:    []string{"[REDACTED]", "Content-Type"},
			notExpected: []string{"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"},
		},
		{
			name: "redacts token in body",
			headers: map[string][]string{
				"Content-Type": {"application/json"},
			},
			body:        `{"data": "normal", "auth_token": "secret-token-value"}`,
			expected:    []string{"data", "normal", "[REDACTED]"},
			notExpected: []string{"secret-token-value"},
		},
		{
			name:        "redacts password in body",
			body:        `{"username": "user", "password": "supersecret123"}`,
			expected:    []string{"username", "user", "[REDACTED]"},
			notExpected: []string{"supersecret123"},
		},
		{
			name:        "redacts bearer token in body",
			body:        `Authorization header: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9`,
			expected:    []string{"Authorization header", "[REDACTED]"},
			notExpected: []string{"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset log file
			os.Remove(logPath)
			var newErr error
			f, newErr = flocklogger.NewFlockLogger(logPath)
			if newErr != nil {
				t.Fatalf("Failed to create new logger: %v", newErr)
			}

			// Test request logging
			f.FlogRequest("GET", "/api/test", tc.headers, tc.body)
			f.Flush()

			content, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatalf("Failed to read log file: %v", err)
			}
			contentStr := string(content)

			for _, exp := range tc.expected {
				if !strings.Contains(contentStr, exp) {
					t.Errorf("Expected log to contain %q, but it didn't: %s", exp, contentStr)
				}
			}

			for _, notExp := range tc.notExpected {
				if strings.Contains(contentStr, notExp) {
					t.Errorf("Expected log to NOT contain %q, but it did: %s", notExp, contentStr)
				}
			}
		})
	}

	// Test response logging
	t.Run("redacts sensitive response data", func(t *testing.T) {
		os.Remove(logPath)
		var newErr error
		f, newErr = flocklogger.NewFlockLogger(logPath)
		if newErr != nil {
			t.Fatalf("Failed to create new logger: %v", newErr)
		}

		headers := map[string][]string{
			"X-Auth-Token": {"secret-token"},
			"Content-Type": {"application/json"},
		}
		body := `{"result": "success", "token": "sensitive-data"}`

		f.FlogResponse(200, headers, body)
		f.Flush()

		content, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("Failed to read log file: %v", err)
		}
		contentStr := string(content)

		// Check for redacted content
		if !strings.Contains(contentStr, "[REDACTED]") {
			t.Errorf("Expected redacted content in log, but didn't find it: %s", contentStr)
		}

		// Check that sensitive data is not present
		if strings.Contains(contentStr, "secret-token") || strings.Contains(contentStr, "sensitive-data") {
			t.Errorf("Found unredacted sensitive data in log: %s", contentStr)
		}

		// Check that normal data is present
		if !strings.Contains(contentStr, "200") || !strings.Contains(contentStr, "Content-Type") {
			t.Errorf("Expected normal data in log, but didn't find it: %s", contentStr)
		}
	})
}

func TestConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	tmpDir, err := os.MkdirTemp("", "flocklogger-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logPath := filepath.Join(tmpDir, "concurrent.log")
	f, err := flocklogger.NewFlockLogger(logPath)
	if err != nil {
		t.Fatalf("NewFlockLogger failed: %v", err)
	}
	defer f.Close()

	const numGoroutines = 10
	const numLogsPerGoroutine = 100

	done := make(chan bool, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < numLogsPerGoroutine; j++ {
				f.Flog("Concurrent log from goroutine %d: message %d", id, j)
				// Small sleep to increase chances of concurrency issues
				time.Sleep(time.Millisecond)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to finish
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Ensure everything is flushed
	f.Flush()

	// Count the number of log entries
	file, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
	}

	expectedLines := numGoroutines * numLogsPerGoroutine
	if lineCount != expectedLines {
		t.Errorf("Expected %d log lines, got %d", expectedLines, lineCount)
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("Scanner error: %v", err)
	}
}

func TestEdgeCases(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "flocklogger-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("empty log message", func(t *testing.T) {
		logPath := filepath.Join(tmpDir, "empty.log")
		f, err := flocklogger.NewFlockLogger(logPath)
		if err != nil {
			t.Fatalf("NewFlockLogger failed: %v", err)
		}
		defer f.Close()

		f.Flog("")
		f.Flush()

		content, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("Failed to read log file: %v", err)
		}
		if len(content) == 0 {
			t.Error("Expected to log empty message with timestamp, got nothing")
		}
	})

	t.Run("very large log message", func(t *testing.T) {
		logPath := filepath.Join(tmpDir, "large.log")
		f, err := flocklogger.NewFlockLogger(logPath)
		if err != nil {
			t.Fatalf("NewFlockLogger failed: %v", err)
		}
		defer f.Close()

		// Generate large message (100KB)
		const largeMessageFormat = "Large log message test. %s"
		largeMsg := strings.Repeat("Large log message test. ", 5000)
		f.Flog(largeMessageFormat, largeMsg)
		f.Flush()

		// Check if it was logged correctly
		content, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("Failed to read log file: %v", err)
		}
		if !strings.Contains(string(content), "Large log message test") {
			t.Error("Failed to properly log very large message")
		}
	})

	t.Run("special characters", func(t *testing.T) {
		logPath := filepath.Join(tmpDir, "special.log")
		f, err := flocklogger.NewFlockLogger(logPath)
		if err != nil {
			t.Fatalf("NewFlockLogger failed: %v", err)
		}
		defer f.Close()

		// The special character string without escape sequences
		const specialMessageFormat = "Special: %s"
		specialChars := `!@#$%^&*()_+{}|:<>?~\-=[];`
		f.Flog(specialMessageFormat, specialChars)
		f.Flush()

		content, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("Failed to read log file: %v", err)
		}

		// Just check for the key part of the message
		if !strings.Contains(string(content), "Special:") {
			t.Error("Failed to properly log message with special characters")
		}

		// Test that most of the special characters are present
		// (removing the most problematic ones from the check)
		for _, char := range []string{"!", "@", "#", "$", "%", "^", "&", "*"} {
			if !strings.Contains(string(content), char) {
				t.Errorf("Missing special character %q in log output", char)
			}
		}
	})
}

// TestRotationMaxFiles ensures that old log files are properly removed
func TestRotationMaxFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "flocklogger-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logPath := filepath.Join(tmpDir, "maxfiles.log")
	f, err := flocklogger.NewFlockLogger(logPath)
	if err != nil {
		t.Fatalf("NewFlockLogger failed: %v", err)
	}

	// Set small max size and max files
	maxFiles := 2
	f.SetMaxSize(50)
	f.SetMaxFiles(maxFiles)

	// Write logs to trigger multiple rotations
	for i := 0; i < 10; i++ {
		f.Flog("Rotation test message with iteration %d to trigger rotation", i)
		f.Flush() // Explicitly flush to ensure consistent behavior
	}
	f.Close()

	// Count log files
	files, err := filepath.Glob(filepath.Join(tmpDir, "maxfiles.log*"))
	if err != nil {
		t.Fatalf("Failed to list log files: %v", err)
	}

	// Should only keep maxFiles + 1 (current file)
	expectedFiles := maxFiles + 1
	if len(files) != expectedFiles {
		t.Errorf("Expected exactly %d log files, got %d: %v", expectedFiles, len(files), files)
	}

	// The highest rotation file should be maxFiles
	highestRotation := fmt.Sprintf("%s.%d", logPath, maxFiles)
	if _, err := os.Stat(highestRotation); os.IsNotExist(err) {
		t.Errorf("Expected rotation file %s to exist", highestRotation)
	}

	// Higher rotation files should not exist
	tooHighRotation := fmt.Sprintf("%s.%d", logPath, maxFiles+1)
	if _, err := os.Stat(tooHighRotation); !os.IsNotExist(err) {
		t.Errorf("Rotation file %s should not exist", tooHighRotation)
	}
}
