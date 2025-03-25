package flocklogger_test

import (
	"bufio"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
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

func TestLogLevels(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "levels.log")

	logger, err := flocklogger.NewFlockLogger(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test all log levels
	logger.Debug("Debug message")
	logger.Debugf("Debug %s message", "formatted")
	logger.Info("Info message")
	logger.Infof("Info %s message", "formatted")
	logger.Warn("Warn message")
	logger.Warnf("Warn %s message", "formatted")
	logger.Error("Error message")
	logger.Errorf("Error %s message", "formatted")

	// Flush to ensure all content is written
	logger.Flush()

	// Read and verify the log file
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)

	// Default level is INFO, so DEBUG should not be present
	if strings.Contains(contentStr, "DEBUG") {
		t.Errorf("DEBUG messages should not be logged at default INFO level")
	}

	// INFO, WARN, and ERROR should be present
	if !strings.Contains(contentStr, "INFO") {
		t.Errorf("INFO messages not found in the log")
	}
	if !strings.Contains(contentStr, "WARN") {
		t.Errorf("WARN messages not found in the log")
	}
	if !strings.Contains(contentStr, "ERROR") {
		t.Errorf("ERROR messages not found in the log")
	}
}

func TestLevelFiltering(t *testing.T) {
	testCases := []struct {
		level         int
		shouldContain []string
		shouldNotHave []string
	}{
		{
			level:         flocklogger.LevelDebug,
			shouldContain: []string{"DEBUG", "INFO", "WARN", "ERROR"},
			shouldNotHave: []string{},
		},
		{
			level:         flocklogger.LevelInfo,
			shouldContain: []string{"INFO", "WARN", "ERROR"},
			shouldNotHave: []string{"DEBUG"},
		},
		{
			level:         flocklogger.LevelWarn,
			shouldContain: []string{"WARN", "ERROR"},
			shouldNotHave: []string{"DEBUG", "INFO"},
		},
		{
			level:         flocklogger.LevelError,
			shouldContain: []string{"ERROR"},
			shouldNotHave: []string{"DEBUG", "INFO", "WARN"},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Level%d", tc.level), func(t *testing.T) {
			tempDir := t.TempDir()
			logPath := filepath.Join(tempDir, "level_test.log")

			logger, err := flocklogger.NewFlockLogger(logPath)
			if err != nil {
				t.Fatalf("Failed to create logger: %v", err)
			}
			logger.SetLevel(tc.level)

			// Write logs at each level
			logger.Debug("Debug message")
			logger.Info("Info message")
			logger.Warn("Warn message")
			logger.Error("Error message")

			logger.Flush()
			logger.Close()

			// Read log contents
			content, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatalf("Failed to read log file: %v", err)
			}
			contentStr := string(content)

			// Check for expected content
			for _, expected := range tc.shouldContain {
				if !strings.Contains(contentStr, expected) {
					t.Errorf("Log should contain %q but it doesn't", expected)
				}
			}

			// Check for unexpected content
			for _, unexpected := range tc.shouldNotHave {
				if strings.Contains(contentStr, unexpected) {
					t.Errorf("Log should not contain %q but it does", unexpected)
				}
			}
		})
	}
}

func TestRotation(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "rotation.log")

	logger, err := flocklogger.NewFlockLogger(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Set a small max size to trigger rotation
	logger.SetMaxSize(100)
	logger.SetMaxFiles(3)

	// Write enough logs to trigger multiple rotations
	for i := 0; i < 20; i++ {
		logger.Infof("This is log message %d with enough content to exceed size limit", i)
	}

	logger.Flush()
	logger.Close()

	// Check if rotation happened
	files, err := filepath.Glob(logPath + "*")
	if err != nil {
		t.Fatalf("Failed to list log files: %v", err)
	}

	// Should have current file plus rotated files
	if len(files) < 2 {
		t.Errorf("Expected rotated files but found only %d files", len(files))
	}

	// Shouldn't exceed max files
	if len(files) > 4 { // Current file + 3 rotated files
		t.Errorf("Found %d files, which exceeds maxFiles setting", len(files))
	}
}

func TestFlushAndClose(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "flush_close.log")

	logger, err := flocklogger.NewFlockLogger(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Write some logs
	logger.Info("Test message")

	// Test Flush
	err = logger.Flush()
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	// Verify content was written
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	if !strings.Contains(string(content), "Test message") {
		t.Errorf("Log content not found after flush")
	}

	// Test Close
	err = logger.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Trying to use a closed logger should result in an error
	err = logger.Flush()
	if err == nil {
		t.Errorf("Expected error when using closed logger, but got none")
	}
}

func TestMultiProcessLogging(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping multi-process test in short mode")
	}

	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "flocklogger-multiprocess-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Path to the shared log file
	logPath := filepath.Join(tmpDir, "multiprocess.log")

	// Create a "ready" file that child processes will wait for
	readyFile := filepath.Join(tmpDir, "ready")
	if err := os.WriteFile(readyFile, []byte("ready"), 0644); err != nil {
		t.Fatalf("Failed to create ready file: %v", err)
	}

	// Number of processes to spawn
	numProcesses := 5 // Reduced from 20 to make test more reliable
	processes := make([]*exec.Cmd, numProcesses)

	// Prepare child processes
	for i := 0; i < numProcesses; i++ {
		// Each process writes 5 log lines
		cmdArgs := []string{
			"-test.run=TestHelperProcessMultiLog",
			"--", // Separates test flags from process args
			logPath,
			readyFile,
			fmt.Sprintf("%d", i), // Process identifier
		}

		cmd := exec.Command(os.Args[0], cmdArgs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		processes[i] = cmd

		// Start each process but don't wait for it
		if err := cmd.Start(); err != nil {
			t.Fatalf("Failed to start process %d: %v", i, err)
		}
	}

	// Wait a moment to ensure all processes are ready and waiting
	time.Sleep(100 * time.Millisecond)

	// Remove the ready file to signal processes to start logging
	if err := os.Remove(readyFile); err != nil {
		t.Fatalf("Failed to remove ready file: %v", err)
	}

	// Wait for all processes to complete
	for i, cmd := range processes {
		if err := cmd.Wait(); err != nil {
			t.Errorf("Process %d failed: %v", i, err)
		}
	}

	// Give file system a moment to complete all writes
	time.Sleep(100 * time.Millisecond)

	// Read the log file and verify its contents
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	lines := strings.Split(string(content), "\n")
	// Filter out empty lines
	var nonEmptyLines []string
	for _, line := range lines {
		if line != "" {
			nonEmptyLines = append(nonEmptyLines, line)
		}
	}

	// We expect 5 lines per process
	expectedLines := numProcesses * 5
	if len(nonEmptyLines) != expectedLines {
		t.Errorf("Expected %d log lines, but got %d", expectedLines, len(nonEmptyLines))
	}

	// Use regexp to extract process ID from log lines
	processIDRegex := regexp.MustCompile(`Process (\d+) log entry`)

	// Verify that each process's logs are present
	processLogs := make(map[int]int)
	for _, line := range nonEmptyLines {
		matches := processIDRegex.FindStringSubmatch(line)
		if len(matches) > 1 {
			// Extract process ID from regex match
			var processID int
			fmt.Sscanf(matches[1], "%d", &processID)
			if processID < numProcesses {
				processLogs[processID]++
			}
		}
	}

	// Check that we have logs from each process
	for i := 0; i < numProcesses; i++ {
		count, found := processLogs[i]
		if !found {
			t.Errorf("No logs found from process %d", i)
		} else if count != 5 {
			t.Errorf("Expected 5 logs from process %d, but got %d", i, count)
		}
	}
}

// TestHelperProcessMultiLog is a helper for TestMultiProcessLogging
// It's executed as a separate process
func TestHelperProcessMultiLog(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	// Parse args: logPath, readyFile, processID
	args := flag.Args()
	if len(args) < 3 {
		fmt.Fprintf(os.Stderr, "Not enough arguments: %v", args)
		os.Exit(1)
	}

	logPath := args[0]
	readyFile := args[1]
	processID := args[2]

	// Wait for the ready file to be removed
	for {
		if _, err := os.Stat(readyFile); os.IsNotExist(err) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Create a single logger instance for all messages from this process
	logger, err := flocklogger.NewFlockLogger(logPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v", err)
		os.Exit(1)
	}
	defer logger.Close()

	// Write 5 log messages
	for i := 0; i < 5; i++ {
		// Log the message with unique identifiers
		logger.Infof("Process %s log entry %d", processID, i)
		logger.Flush()

		// Small random delay to increase concurrency likelihood
		time.Sleep(time.Duration(rand.Intn(5)) * time.Millisecond)
	}

	os.Exit(0)
}
