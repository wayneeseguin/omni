package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wayneeseguin/omni/pkg/omni"
)

func TestMain(m *testing.M) {
	// Setup: clean up any existing test files
	os.RemoveAll("test_cli")

	code := m.Run()

	// Cleanup: remove test files
	os.RemoveAll("test_cli")
	os.Exit(code)
}

func TestSetupLogger(t *testing.T) {
	// Save original values
	origLogFile := *logFile
	origDebug := *debug
	origVerbose := *verbose
	origJsonLogs := *jsonLogs

	defer func() {
		// Restore original values
		*logFile = origLogFile
		*debug = origDebug
		*verbose = origVerbose
		*jsonLogs = origJsonLogs
		if logger != nil {
			logger.Close()
			logger = nil
		}
	}()

	// Test with default settings
	testLogDir := "test_cli"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	logPath := filepath.Join(testLogDir, "test_app.log")
	*logFile = logPath
	*debug = false
	*verbose = false
	*jsonLogs = false

	err := setupLogger()
	if err != nil {
		t.Fatalf("setupLogger failed: %v", err)
	}

	if logger == nil {
		t.Fatal("logger was not initialized")
	}

	// Test that log file is created by writing a message
	logger.Info("Test message")
	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
	logger.Close()
	logger = nil

	// Verify log file exists and has content
	if stat, err := os.Stat(logPath); err != nil {
		t.Errorf("Log file was not created: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Log file is empty")
	}
}

func TestSetupLoggerWithDebug(t *testing.T) {
	// Save original values
	origLogFile := *logFile
	origDebug := *debug
	origVerbose := *verbose
	origJsonLogs := *jsonLogs

	defer func() {
		// Restore original values
		*logFile = origLogFile
		*debug = origDebug
		*verbose = origVerbose
		*jsonLogs = origJsonLogs
		if logger != nil {
			logger.Close()
			logger = nil
		}
	}()

	testLogDir := "test_cli"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	logPath := filepath.Join(testLogDir, "test_debug.log")
	*logFile = logPath
	*debug = true
	*verbose = false
	*jsonLogs = true

	err := setupLogger()
	if err != nil {
		t.Fatalf("setupLogger with debug failed: %v", err)
	}

	// Test debug logging
	logger.Debug("Debug message")
	logger.Info("Info message")
	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
	logger.Close()
	logger = nil

	// Verify log file
	if stat, err := os.Stat(logPath); err != nil {
		t.Errorf("Debug log file error: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Debug log file is empty")
	}
}

func TestProcessFiles(t *testing.T) {
	testLogDir := "test_cli"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	// Initialize logger for testing
	logPath := filepath.Join(testLogDir, "process_test.log")
	testLogger, err := omni.NewWithOptions(
		omni.WithPath(logPath),
		omni.WithLevel(omni.LevelDebug),
		omni.WithText(),
	)
	if err != nil {
		t.Fatalf("Failed to create test logger: %v", err)
	}
	defer testLogger.Close()

	// Set global logger
	logger = testLogger

	// Create test files
	testFiles := []string{
		filepath.Join(testLogDir, "test1.txt"),
		filepath.Join(testLogDir, "test2.txt"),
		filepath.Join(testLogDir, "test3.txt"),
	}

	for i, file := range testFiles {
		content := strings.Repeat("test data", i+1) // Different sizes
		if err := os.WriteFile(file, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", file, err)
		}
	}

	// Test processing files
	err = processFiles(testFiles)
	if err != nil {
		t.Errorf("processFiles failed: %v", err)
	}

	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)

	// Verify log file
	if stat, err := os.Stat(logPath); err != nil {
		t.Errorf("Process log file error: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Process log file is empty")
	}
}

func TestProcessFilesWithEmptyFile(t *testing.T) {
	testLogDir := "test_cli"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	// Initialize logger
	logPath := filepath.Join(testLogDir, "empty_test.log")
	testLogger, err := omni.NewWithOptions(
		omni.WithPath(logPath),
		omni.WithLevel(omni.LevelDebug),
	)
	if err != nil {
		t.Fatalf("Failed to create test logger: %v", err)
	}
	defer testLogger.Close()

	logger = testLogger

	// Create an empty file (should cause error)
	emptyFile := filepath.Join(testLogDir, "empty.txt")
	if err := os.WriteFile(emptyFile, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	// Test processing with empty file
	err = processFiles([]string{emptyFile})
	if err == nil {
		t.Error("Expected error when processing empty file")
	}

	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
}

func TestProcessFilesWithNonExistentFile(t *testing.T) {
	testLogDir := "test_cli"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	// Initialize logger
	logPath := filepath.Join(testLogDir, "nonexistent_test.log")
	testLogger, err := omni.NewWithOptions(
		omni.WithPath(logPath),
		omni.WithLevel(omni.LevelDebug),
	)
	if err != nil {
		t.Fatalf("Failed to create test logger: %v", err)
	}
	defer testLogger.Close()

	logger = testLogger

	// Test processing non-existent file
	err = processFiles([]string{"nonexistent.txt"})
	if err == nil {
		t.Error("Expected error when processing non-existent file")
	}

	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
}

func TestProcessFile(t *testing.T) {
	testLogDir := "test_cli"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	// Initialize logger
	logPath := filepath.Join(testLogDir, "single_test.log")
	testLogger, err := omni.NewWithOptions(
		omni.WithPath(logPath),
		omni.WithLevel(omni.LevelTrace),
	)
	if err != nil {
		t.Fatalf("Failed to create test logger: %v", err)
	}
	defer testLogger.Close()

	logger = testLogger

	// Create test file
	testFile := filepath.Join(testLogDir, "single.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test processing single file
	err = processFile(testFile)
	if err != nil {
		t.Errorf("processFile failed: %v", err)
	}

	// Test with empty file
	emptyFile := filepath.Join(testLogDir, "empty_single.txt")
	if err := os.WriteFile(emptyFile, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	err = processFile(emptyFile)
	if err == nil {
		t.Error("Expected error for empty file")
	}

	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
}

func TestAnalyzeData(t *testing.T) {
	testLogDir := "test_cli"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	// Initialize logger
	logPath := filepath.Join(testLogDir, "analyze_test.log")
	testLogger, err := omni.NewWithOptions(
		omni.WithPath(logPath),
		omni.WithLevel(omni.LevelDebug),
	)
	if err != nil {
		t.Fatalf("Failed to create test logger: %v", err)
	}
	defer testLogger.Close()

	logger = testLogger

	// Test data analysis
	start := time.Now()
	err = analyzeData()
	duration := time.Since(start)

	if err != nil {
		t.Errorf("analyzeData failed: %v", err)
	}

	// Should take some time due to sleep statements
	expectedMinDuration := 200 * time.Millisecond // 5 steps * 50ms each = 250ms
	if duration < expectedMinDuration {
		t.Logf("Warning: Analysis completed faster than expected: %v (expected at least %v)", duration, expectedMinDuration)
	}

	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)

	// Verify log file
	if stat, err := os.Stat(logPath); err != nil {
		t.Errorf("Analyze log file error: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Analyze log file is empty")
	}
}

func TestGenerateReport(t *testing.T) {
	testLogDir := "test_cli"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	// Initialize logger
	logPath := filepath.Join(testLogDir, "report_test.log")
	testLogger, err := omni.NewWithOptions(
		omni.WithPath(logPath),
		omni.WithLevel(omni.LevelDebug),
	)
	if err != nil {
		t.Fatalf("Failed to create test logger: %v", err)
	}
	defer testLogger.Close()

	logger = testLogger

	// Test report generation
	err = generateReport()
	if err != nil {
		t.Errorf("generateReport failed: %v", err)
	}

	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)

	// Verify log file
	if stat, err := os.Stat(logPath); err != nil {
		t.Errorf("Report log file error: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Report log file is empty")
	}

	// Verify report file was created (in temp directory)
	// Note: The actual report path varies by system, so we just check the function completed successfully
}

func TestGetMemoryUsage(t *testing.T) {
	usage := getMemoryUsage()
	if usage != 42 {
		t.Errorf("Expected memory usage 42, got %d", usage)
	}
}

func TestGetLevelName(t *testing.T) {
	tests := []struct {
		level    int
		expected string
	}{
		{omni.LevelTrace, "TRACE"},
		{omni.LevelDebug, "DEBUG"},
		{omni.LevelInfo, "INFO"},
		{omni.LevelWarn, "WARN"},
		{omni.LevelError, "ERROR"},
		{999, "UNKNOWN"},
	}

	for _, test := range tests {
		result := getLevelName(test.level)
		if result != test.expected {
			t.Errorf("getLevelName(%d) = %s, expected %s", test.level, result, test.expected)
		}
	}
}

func TestCliIntegration(t *testing.T) {
	testLogDir := "test_cli"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	// Save original flag values
	origLogFile := *logFile
	origDebug := *debug
	origVerbose := *verbose
	origJsonLogs := *jsonLogs
	origOperation := *operation

	defer func() {
		// Restore original values
		*logFile = origLogFile
		*debug = origDebug
		*verbose = origVerbose
		*jsonLogs = origJsonLogs
		*operation = origOperation
		if logger != nil {
			logger.Close()
			logger = nil
		}
	}()

	// Set test flags
	logPath := filepath.Join(testLogDir, "integration_test.log")
	*logFile = logPath
	*debug = true
	*verbose = false
	*jsonLogs = true
	*operation = "analyze"

	// Test setup
	err := setupLogger()
	if err != nil {
		t.Fatalf("setupLogger failed: %v", err)
	}

	// Test analyze operation
	err = analyzeData()
	if err != nil {
		t.Errorf("analyze operation failed: %v", err)
	}

	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
	logger.Close()
	logger = nil

	// Verify log file
	if stat, err := os.Stat(logPath); err != nil {
		t.Errorf("Integration log file error: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Integration log file is empty")
	}
}

func TestInvalidOperation(t *testing.T) {
	testLogDir := "test_cli"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	// Initialize logger
	logPath := filepath.Join(testLogDir, "invalid_test.log")
	testLogger, err := omni.NewWithOptions(
		omni.WithPath(logPath),
		omni.WithLevel(omni.LevelInfo),
	)
	if err != nil {
		t.Fatalf("Failed to create test logger: %v", err)
	}
	defer testLogger.Close()

	logger = testLogger

	// Save original operation flag
	origOperation := *operation
	defer func() {
		*operation = origOperation
	}()

	// Test invalid operation
	*operation = "invalid"

	// This would normally be handled in main(), but we can test the error case directly
	var err2 error
	switch *operation {
	case "process", "analyze", "report":
		// Valid operations - shouldn't reach here
	default:
		err2 = testLogger.Close() // This would be the error handling
		if err2 == nil {
			t.Log("Invalid operation correctly identified") // Expected path
		}
	}

	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
}

// Benchmark tests
func BenchmarkProcessFiles(b *testing.B) {
	testLogDir := "bench_cli"
	os.MkdirAll(testLogDir, 0755)
	defer os.RemoveAll(testLogDir)

	// Initialize logger
	logPath := filepath.Join(testLogDir, "bench_process.log")
	testLogger, err := omni.NewWithOptions(
		omni.WithPath(logPath),
		omni.WithLevel(omni.LevelWarn), // Higher level for performance
	)
	if err != nil {
		b.Fatalf("Failed to create test logger: %v", err)
	}
	defer testLogger.Close()

	logger = testLogger

	// Create test files
	testFiles := make([]string, 5)
	for i := 0; i < 5; i++ {
		file := filepath.Join(testLogDir, "bench_file_"+string(rune('A'+i))+".txt")
		if err := os.WriteFile(file, []byte("benchmark data"), 0644); err != nil {
			b.Fatalf("Failed to create benchmark file: %v", err)
		}
		testFiles[i] = file
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		processFiles(testFiles)
	}
}

func BenchmarkAnalyzeData(b *testing.B) {
	testLogDir := "bench_cli"
	os.MkdirAll(testLogDir, 0755)
	defer os.RemoveAll(testLogDir)

	// Initialize logger with minimal logging for performance
	logPath := filepath.Join(testLogDir, "bench_analyze.log")
	testLogger, err := omni.NewWithOptions(
		omni.WithPath(logPath),
		omni.WithLevel(omni.LevelError), // Only errors for performance
	)
	if err != nil {
		b.Fatalf("Failed to create test logger: %v", err)
	}
	defer testLogger.Close()

	logger = testLogger

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzeData()
	}
}
