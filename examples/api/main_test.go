package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wayneeseguin/omni"
)

func TestMain(m *testing.M) {
	// Setup: clean up any existing test files
	os.RemoveAll("test_api")
	
	code := m.Run()
	
	// Cleanup: remove test files
	os.RemoveAll("test_api")
	os.Exit(code)
}

func TestBasicExample(t *testing.T) {
	// Test basic API usage
	testLogDir := "test_api"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	logger, err := omni.New(filepath.Join(testLogDir, "basic_api.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Test basic configuration
	logger.SetLevel(omni.LevelInfo)
	logger.SetFormat(omni.FormatJSON)

	// Test all logging methods
	logger.Trace("Trace message")
	logger.Debug("Debug message")
	logger.Info("Info message")
	logger.Warn("Warning message")
	logger.Error("Error message")

	// Test formatted logging
	logger.Infof("User %s logged in at %s", "test_user", time.Now().Format(time.RFC3339))

	// Test structured logging
	logger.InfoWithFields("User action", map[string]interface{}{
		"user_id": "12345",
		"action":  "test",
		"success": true,
	})

	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
	logger.Close()

	// Verify log file was created
	logFile := filepath.Join(testLogDir, "basic_api.log")
	if stat, err := os.Stat(logFile); err != nil {
		t.Errorf("Log file was not created: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Log file is empty")
	}
}

func TestOptionsExample(t *testing.T) {
	testLogDir := "test_api"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	// Test production configuration
	prodLogger, err := omni.NewWithOptions(
		omni.WithPath(filepath.Join(testLogDir, "prod.log")),
		omni.WithLevel(omni.LevelInfo),
		omni.WithRotation(1024*1024, 3), // Small for testing
		omni.WithGzipCompression(),
		omni.WithJSON(),
		omni.WithChannelSize(500),
	)
	if err != nil {
		t.Fatalf("Failed to create production logger: %v", err)
	}

	// Test development configuration
	devLogger, err := omni.NewWithOptions(
		omni.WithPath(filepath.Join(testLogDir, "dev.log")),
		omni.WithLevel(omni.LevelTrace),
		omni.WithText(),
		omni.WithStackTrace(4096),
	)
	if err != nil {
		t.Fatalf("Failed to create dev logger: %v", err)
	}

	// Use the loggers
	prodLogger.Info("Production logger test message")
	devLogger.Debug("Development logger test message")

	// Test filtering
	prodLogger.AddFilter(func(level int, message string, fields map[string]interface{}) bool {
		return level >= omni.LevelInfo
	})

	prodLogger.Debug("Filtered debug message")
	prodLogger.Info("Allowed info message")

	prodLogger.FlushAll()
	devLogger.FlushAll()
	time.Sleep(10 * time.Millisecond)
	prodLogger.Close()
	devLogger.Close()

	// Verify both log files were created
	prodFile := filepath.Join(testLogDir, "prod.log")
	devFile := filepath.Join(testLogDir, "dev.log")

	for _, file := range []string{prodFile, devFile} {
		if stat, err := os.Stat(file); err != nil {
			t.Errorf("Log file %s was not created: %v", file, err)
		} else if stat.Size() == 0 {
			t.Errorf("Log file %s is empty", file)
		}
	}
}

func TestAdvancedExample(t *testing.T) {
	testLogDir := "test_api"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	logger, err := omni.New(filepath.Join(testLogDir, "advanced.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Test advanced configuration
	logger.SetLevel(omni.LevelDebug)
	logger.SetFormat(omni.FormatJSON)
	logger.SetMaxSize(5 * 1024 * 1024) // 5MB
	logger.SetMaxFiles(2)
	logger.SetCompression(omni.CompressionGzip)
	logger.EnableStackTraces(true)

	// Test multiple destinations
	copyDest := filepath.Join(testLogDir, "advanced_copy.log")
	err = logger.AddDestination(copyDest)
	if err != nil {
		t.Fatalf("Failed to add destination: %v", err)
	}

	// Test destination management
	destinations := logger.ListDestinations()
	if len(destinations) < 2 {
		t.Errorf("Expected at least 2 destinations, got %d", len(destinations))
	}

	// Test disable/enable destination
	logger.DisableDestination(copyDest)
	logger.Info("Message with destination disabled")

	logger.EnableDestination(copyDest)
	logger.Info("Message with destination enabled")

	// Test doWork function
	doWork(logger)

	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
	logger.Close()

	// Verify log files
	mainFile := filepath.Join(testLogDir, "advanced.log")
	if stat, err := os.Stat(mainFile); err != nil {
		t.Errorf("Main log file was not created: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Main log file is empty")
	}

	// Copy file should also exist
	if stat, err := os.Stat(copyDest); err != nil {
		t.Errorf("Copy log file was not created: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Copy log file is empty")
	}
}

func TestDoWork(t *testing.T) {
	testLogDir := "test_api"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	logger, err := omni.New(filepath.Join(testLogDir, "dowork.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	logger.SetLevel(omni.LevelTrace)

	// Test the doWork function
	doWork(logger)

	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
	logger.Close()

	// Verify log file
	logFile := filepath.Join(testLogDir, "dowork.log")
	if stat, err := os.Stat(logFile); err != nil {
		t.Errorf("Log file was not created: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Log file is empty")
	}
}

func TestExpensiveDebugOperation(t *testing.T) {
	// Test the expensive debug operation
	start := time.Now()
	result := expensiveDebugOperation()
	duration := time.Since(start)

	if result != "detailed debug information" {
		t.Errorf("Expected 'detailed debug information', got '%s'", result)
	}

	// Should take at least 10ms due to sleep
	if duration < 10*time.Millisecond {
		t.Errorf("Expected operation to take at least 10ms, took %v", duration)
	}
}

func TestErrorHandlingExample(t *testing.T) {
	testLogDir := "test_api"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	// Test error handling with invalid path
	_, err := omni.New("/invalid/path/that/does/not/exist/test.log")
	if err == nil {
		t.Error("Expected error for invalid path, got nil")
	}

	// Test successful logger creation
	logger, err := omni.New(filepath.Join(testLogDir, "error_handling.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Test various operations
	logger.SetLevel(omni.LevelError)
	logger.Debug("Filtered debug message")
	logger.Error("Error message")

	// Test error recovery
	logger.ErrorWithFields("Test error", map[string]interface{}{
		"error_type": "test",
		"code":       500,
	})

	// Test operations after errors
	logger.Info("Post-error message")

	// Test flush and sync
	logger.FlushAll()
	logger.Sync()

	time.Sleep(10 * time.Millisecond)
	logger.Close()

	// Verify log file
	logFile := filepath.Join(testLogDir, "error_handling.log")
	if stat, err := os.Stat(logFile); err != nil {
		t.Errorf("Log file was not created: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Log file is empty")
	}
}

func TestLoggerConfiguration(t *testing.T) {
	testLogDir := "test_api"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	logger, err := omni.New(filepath.Join(testLogDir, "config_test.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test level configuration
	levels := []int{
		omni.LevelTrace,
		omni.LevelDebug,
		omni.LevelInfo,
		omni.LevelWarn,
		omni.LevelError,
	}

	for _, level := range levels {
		logger.SetLevel(level)
		
		// Test all logging methods
		logger.Trace("Trace test")
		logger.Debug("Debug test")
		logger.Info("Info test")
		logger.Warn("Warn test")
		logger.Error("Error test")
	}

	// Test format configuration
	logger.SetFormat(omni.FormatJSON)
	logger.Info("JSON format test")

	logger.SetFormat(omni.FormatText)
	logger.Info("Text format test")

	// Test stack traces
	logger.EnableStackTraces(true)
	logger.Error("Error with stack trace")

	logger.EnableStackTraces(false)
	logger.Error("Error without stack trace")

	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)

	// Verify log file
	logFile := filepath.Join(testLogDir, "config_test.log")
	if stat, err := os.Stat(logFile); err != nil {
		t.Errorf("Log file was not created: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Log file is empty")
	}
}

func TestStructuredLogging(t *testing.T) {
	testLogDir := "test_api"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	logger, err := omni.NewWithOptions(
		omni.WithPath(filepath.Join(testLogDir, "structured.log")),
		omni.WithLevel(omni.LevelInfo),
		omni.WithJSON(),
	)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Test various field types
	logger.InfoWithFields("Complex structured data", map[string]interface{}{
		"string_field":  "test_value",
		"int_field":     42,
		"float_field":   3.14159,
		"bool_field":    true,
		"array_field":   []string{"a", "b", "c"},
		"nested_object": map[string]interface{}{
			"nested_string": "nested_value",
			"nested_int":    123,
		},
		"timestamp": time.Now().Unix(),
	})

	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
	logger.Close()

	// Verify log file
	logFile := filepath.Join(testLogDir, "structured.log")
	if stat, err := os.Stat(logFile); err != nil {
		t.Errorf("Log file was not created: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Log file is empty")
	}
}

// Benchmark tests
func BenchmarkBasicLogging(b *testing.B) {
	testLogDir := "bench_api"
	os.MkdirAll(testLogDir, 0755)
	defer os.RemoveAll(testLogDir)

	logger, err := omni.New(filepath.Join(testLogDir, "bench_basic.log"))
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	logger.SetLevel(omni.LevelInfo)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("Benchmark message")
	}
}

func BenchmarkStructuredLogging(b *testing.B) {
	testLogDir := "bench_api"
	os.MkdirAll(testLogDir, 0755)
	defer os.RemoveAll(testLogDir)

	logger, err := omni.NewWithOptions(
		omni.WithPath(filepath.Join(testLogDir, "bench_structured.log")),
		omni.WithLevel(omni.LevelInfo),
		omni.WithJSON(),
	)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	fields := map[string]interface{}{
		"user_id": "bench_user",
		"action":  "benchmark",
		"count":   42,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.InfoWithFields("Benchmark structured message", fields)
	}
}

func BenchmarkFormattedLogging(b *testing.B) {
	testLogDir := "bench_api"
	os.MkdirAll(testLogDir, 0755)
	defer os.RemoveAll(testLogDir)

	logger, err := omni.New(filepath.Join(testLogDir, "bench_formatted.log"))
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	logger.SetLevel(omni.LevelInfo)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Infof("Benchmark formatted message %d", i)
	}
}

func BenchmarkDoWork(b *testing.B) {
	testLogDir := "bench_api"
	os.MkdirAll(testLogDir, 0755)
	defer os.RemoveAll(testLogDir)

	logger, err := omni.New(filepath.Join(testLogDir, "bench_dowork.log"))
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	logger.SetLevel(omni.LevelInfo) // Higher level for performance

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		doWork(logger)
	}
}