package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/wayneeseguin/omni/pkg/omni"
)

func TestMain(m *testing.M) {
	// Setup: clean up any existing test files
	os.Remove("test_app.log")

	code := m.Run()

	// Cleanup: remove test files
	os.Remove("test_app.log")
	os.Exit(code)
}

func TestBasicExample(t *testing.T) {
	// Test logger creation
	logger, err := omni.New("test_app.log")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer func() {
		if err := logger.Close(); err != nil {
			t.Errorf("Error during close: %v", err)
		}
	}()

	// Test setting log level
	logger.SetLevel(omni.LevelTrace)

	// Test basic logging at all levels
	logger.Trace("Test trace message")
	logger.Debug("Test debug message")
	logger.Info("Test info message")
	logger.Warn("Test warning message")
	logger.Error("Test error message")

	// Test formatted logging
	username := "test_user"
	logger.Tracef("Test formatted trace for user: %s", username)
	logger.Debugf("Test formatted debug for user: %s", username)
	logger.Infof("Test formatted info for user: %s", username)

	// Test structured logging with fields
	logger.TraceWithFields("Test trace with fields", map[string]interface{}{
		"function": "testFunction",
		"user":     username,
		"step":     "testing",
	})

	logger.DebugWithFields("Test debug with fields", map[string]interface{}{
		"user": username,
		"hit":  true,
		"ttl":  300,
	})

	logger.InfoWithFields("Test info with fields", map[string]interface{}{
		"user":      username,
		"action":    "test",
		"ip":        "127.0.0.1",
		"timestamp": "2024-01-20T10:30:00Z",
	})

	// Test error logging with fields
	testErr := fmt.Errorf("test error")
	logger.ErrorWithFields("Test error with fields", map[string]interface{}{
		"error":       testErr.Error(),
		"retry_count": 1,
		"max_retries": 3,
	})

	// Sync to ensure all messages are processed and written
	if err := logger.Sync(); err != nil {
		t.Errorf("Failed to sync logger: %v", err)
	}

	// Verify log file was created and has content
	if stat, err := os.Stat("test_app.log"); err != nil {
		t.Errorf("Log file was not created: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Log file is empty")
	}
}

func TestLoggingLevels(t *testing.T) {
	testLogDir := "test_levels"
	os.MkdirAll(testLogDir, 0755)
	defer os.RemoveAll(testLogDir)

	// Test different log levels
	levels := []struct {
		level int
		name  string
	}{
		{omni.LevelTrace, "TRACE"},
		{omni.LevelDebug, "DEBUG"},
		{omni.LevelInfo, "INFO"},
		{omni.LevelWarn, "WARN"},
		{omni.LevelError, "ERROR"},
	}

	for i, l := range levels {
		// Create a unique log file for each level test
		logFile := filepath.Join(testLogDir, fmt.Sprintf("levels_%d.log", i))
		logger, err := omni.New(logFile)
		if err != nil {
			t.Fatalf("Failed to create logger for level %s: %v", l.name, err)
		}

		logger.SetLevel(l.level)

		// Only messages at or above the set level should be logged
		logger.Trace("trace message")
		logger.Debug("debug message")
		logger.Info("info message")
		logger.Warn("warn message")
		logger.Error("error message")

		if err := logger.Sync(); err != nil {
			t.Errorf("Failed to sync logger for level %s: %v", l.name, err)
		}
		logger.Close()

		// Verify file has content when level is appropriate
		if stat, err := os.Stat(logFile); err != nil {
			t.Errorf("Log file error for level %s: %v", l.name, err)
		} else if stat.Size() == 0 && l.level <= omni.LevelError {
			t.Errorf("Log file is empty for level %s", l.name)
		}
	}
}

func TestStructuredLogging(t *testing.T) {
	testLogDir := "test_structured"
	os.MkdirAll(testLogDir, 0755)
	defer os.RemoveAll(testLogDir)

	logger, err := omni.New(filepath.Join(testLogDir, "structured.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	logger.SetLevel(omni.LevelTrace)

	// Test various field types
	fields := map[string]interface{}{
		"string_field": "test_value",
		"int_field":    42,
		"float_field":  3.14,
		"bool_field":   true,
		"slice_field":  []string{"a", "b", "c"},
		"map_field":    map[string]string{"key": "value"},
	}

	logger.InfoWithFields("Test structured logging with various field types", fields)

	if err := logger.Sync(); err != nil {
		t.Errorf("Failed to sync logger: %v", err)
	}
	logger.Close()

	// Verify log file has content
	logFile := filepath.Join(testLogDir, "structured.log")
	if stat, err := os.Stat(logFile); err != nil {
		t.Errorf("Log file error: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Log file is empty")
	}
}

func TestFormattedLogging(t *testing.T) {
	testLogDir := "test_formatted"
	os.MkdirAll(testLogDir, 0755)
	defer os.RemoveAll(testLogDir)

	logger, err := omni.New(filepath.Join(testLogDir, "formatted.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	logger.SetLevel(omni.LevelTrace)

	// Test formatted logging with different argument types
	logger.Tracef("Trace: %s %d %f %t", "string", 42, 3.14, true)
	logger.Debugf("Debug: %s %d %f %t", "string", 42, 3.14, true)
	logger.Infof("Info: %s %d %f %t", "string", 42, 3.14, true)
	logger.Warnf("Warn: %s %d %f %t", "string", 42, 3.14, true)
	logger.Errorf("Error: %s %d %f %t", "string", 42, 3.14, true)

	if err := logger.Sync(); err != nil {
		t.Errorf("Failed to sync logger: %v", err)
	}
	logger.Close()

	// Verify log file has content
	logFile := filepath.Join(testLogDir, "formatted.log")
	if stat, err := os.Stat(logFile); err != nil {
		t.Errorf("Log file error: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Log file is empty")
	}
}

// Benchmark tests
func BenchmarkBasicLogging(b *testing.B) {
	testLogDir := "bench_basic"
	os.MkdirAll(testLogDir, 0755)
	defer os.RemoveAll(testLogDir)

	logger, err := omni.New(filepath.Join(testLogDir, "bench.log"))
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Use higher log level for performance
	logger.SetLevel(omni.LevelInfo)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("Benchmark message")
	}
}

func BenchmarkStructuredLogging(b *testing.B) {
	testLogDir := "bench_structured"
	os.MkdirAll(testLogDir, 0755)
	defer os.RemoveAll(testLogDir)

	logger, err := omni.New(filepath.Join(testLogDir, "bench_structured.log"))
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	logger.SetLevel(omni.LevelInfo)

	fields := map[string]interface{}{
		"user":   "bench_user",
		"action": "bench_action",
		"count":  42,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.InfoWithFields("Benchmark structured message", fields)
	}
}

func BenchmarkFormattedLogging(b *testing.B) {
	testLogDir := "bench_formatted"
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
