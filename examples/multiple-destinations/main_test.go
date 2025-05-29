package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wayneeseguin/flexlog"
)

func TestMain(m *testing.M) {
	// Setup: clean up any existing test files
	os.RemoveAll("test_logs")
	
	code := m.Run()
	
	// Cleanup: remove test files
	os.RemoveAll("test_logs")
	os.Exit(code)
}

func TestMultipleDestinationsExample(t *testing.T) {
	// Create test logs directory
	testLogDir := "test_logs"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	// Create primary logger
	logger, err := flexlog.New(filepath.Join(testLogDir, "all.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	logger.SetLevel(flexlog.LevelTrace)

	// Add multiple destinations
	destinations := []string{
		filepath.Join(testLogDir, "errors.log"),
		filepath.Join(testLogDir, "structured.log"),
		filepath.Join(testLogDir, "audit.log"),
	}

	for _, dest := range destinations {
		err = logger.AddDestination(dest)
		if err != nil {
			t.Fatalf("Failed to add destination %s: %v", dest, err)
		}
	}

	// Test logging to all destinations
	logger.Info("Test message to all destinations")
	logger.ErrorWithFields("Test error with fields", map[string]interface{}{
		"error_code": 500,
		"source":     "test",
	})

	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
	logger.Close()

	// Verify all log files were created and have content
	allFiles := append(destinations, filepath.Join(testLogDir, "all.log"))
	for _, logFile := range allFiles {
		if stat, err := os.Stat(logFile); err != nil {
			t.Errorf("Log file was not created: %s, error: %v", logFile, err)
		} else if stat.Size() == 0 {
			t.Errorf("Log file is empty: %s", logFile)
		}
	}
}

func TestDestinationManagement(t *testing.T) {
	testLogDir := "test_destination_mgmt"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	logger, err := flexlog.New(filepath.Join(testLogDir, "primary.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	logger.SetLevel(flexlog.LevelInfo)

	// Add destinations
	dest1 := filepath.Join(testLogDir, "dest1.log")
	dest2 := filepath.Join(testLogDir, "dest2.log")

	err = logger.AddDestination(dest1)
	if err != nil {
		t.Fatalf("Failed to add destination 1: %v", err)
	}

	err = logger.AddDestination(dest2)
	if err != nil {
		t.Fatalf("Failed to add destination 2: %v", err)
	}

	// Test listing destinations
	destinations := logger.ListDestinations()
	if len(destinations) < 3 { // primary + 2 added
		t.Errorf("Expected at least 3 destinations, got %d", len(destinations))
	}

	// Test logging before disable
	logger.Info("Message before disable")

	// Test disabling a destination
	logger.DisableDestination(dest1)
	logger.Info("Message after disabling dest1")

	// Test re-enabling a destination
	logger.EnableDestination(dest1)
	logger.Info("Message after re-enabling dest1")

	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
	logger.Close()

	// Verify primary and dest2 have content
	primaryFile := filepath.Join(testLogDir, "primary.log")
	for _, file := range []string{primaryFile, dest2} {
		if stat, err := os.Stat(file); err != nil {
			t.Errorf("Log file error %s: %v", file, err)
		} else if stat.Size() == 0 {
			t.Errorf("Log file is empty: %s", file)
		}
	}
}

func TestAddRemoveDestinations(t *testing.T) {
	testLogDir := "test_add_remove"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	logger, err := flexlog.New(filepath.Join(testLogDir, "main.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	logger.SetLevel(flexlog.LevelInfo)

	// Test adding destinations
	dest := filepath.Join(testLogDir, "temp.log")
	err = logger.AddDestination(dest)
	if err != nil {
		t.Fatalf("Failed to add destination: %v", err)
	}

	// Log a message
	logger.Info("Test message with temp destination")

	// Test removing destination
	logger.RemoveDestination(dest)

	// Log another message (should not go to removed destination)
	logger.Info("Test message after removing temp destination")

	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
	logger.Close()

	// Verify main log has content
	mainFile := filepath.Join(testLogDir, "main.log")
	if stat, err := os.Stat(mainFile); err != nil {
		t.Errorf("Main log file error: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Main log file is empty")
	}
}

func TestMultipleDestinationsWithDifferentLevels(t *testing.T) {
	testLogDir := "test_multi_levels"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	logger, err := flexlog.New(filepath.Join(testLogDir, "all_levels.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Set to trace level to capture all messages
	logger.SetLevel(flexlog.LevelTrace)

	// Add additional destinations
	err = logger.AddDestination(filepath.Join(testLogDir, "copy1.log"))
	if err != nil {
		t.Fatalf("Failed to add destination: %v", err)
	}

	err = logger.AddDestination(filepath.Join(testLogDir, "copy2.log"))
	if err != nil {
		t.Fatalf("Failed to add destination: %v", err)
	}

	// Test all log levels
	logger.Trace("Trace message")
	logger.Debug("Debug message")
	logger.Info("Info message")
	logger.Warn("Warn message")
	logger.Error("Error message")

	// Test structured logging
	logger.InfoWithFields("Structured message", map[string]interface{}{
		"field1": "value1",
		"field2": 42,
		"field3": true,
	})

	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
	logger.Close()

	// Verify all destination files have content
	files := []string{
		filepath.Join(testLogDir, "all_levels.log"),
		filepath.Join(testLogDir, "copy1.log"),
		filepath.Join(testLogDir, "copy2.log"),
	}

	for _, file := range files {
		if stat, err := os.Stat(file); err != nil {
			t.Errorf("Log file error %s: %v", file, err)
		} else if stat.Size() == 0 {
			t.Errorf("Log file is empty: %s", file)
		}
	}
}

func TestDestinationBackends(t *testing.T) {
	testLogDir := "test_backends"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	logger, err := flexlog.New(filepath.Join(testLogDir, "primary.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	logger.SetLevel(flexlog.LevelInfo)

	// Test adding destination with specific backend
	flockDest := filepath.Join(testLogDir, "flock.log")
	err = logger.AddDestinationWithBackend(flockDest, flexlog.BackendFlock)
	if err != nil {
		t.Fatalf("Failed to add flock destination: %v", err)
	}

	// Log messages
	logger.Info("Test message to flock backend")
	logger.ErrorWithFields("Test error", map[string]interface{}{
		"backend": "flock",
		"test":    true,
	})

	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
	logger.Close()

	// Verify files have content
	files := []string{
		filepath.Join(testLogDir, "primary.log"),
		flockDest,
	}

	for _, file := range files {
		if stat, err := os.Stat(file); err != nil {
			t.Errorf("Log file error %s: %v", file, err)
		} else if stat.Size() == 0 {
			t.Errorf("Log file is empty: %s", file)
		}
	}
}

// Benchmark tests
func BenchmarkMultipleDestinations(b *testing.B) {
	testLogDir := "bench_multi"
	os.MkdirAll(testLogDir, 0755)
	defer os.RemoveAll(testLogDir)

	logger, err := flexlog.New(filepath.Join(testLogDir, "bench.log"))
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Add multiple destinations
	for i := 0; i < 3; i++ {
		dest := filepath.Join(testLogDir, fmt.Sprintf("dest%d.log", i))
		logger.AddDestination(dest)
	}

	logger.SetLevel(flexlog.LevelInfo)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("Benchmark message to multiple destinations")
	}
}

func BenchmarkDestinationManagement(b *testing.B) {
	testLogDir := "bench_mgmt"
	os.MkdirAll(testLogDir, 0755)
	defer os.RemoveAll(testLogDir)

	logger, err := flexlog.New(filepath.Join(testLogDir, "bench.log"))
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	dest := filepath.Join(testLogDir, "toggle.log")
	logger.AddDestination(dest)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%2 == 0 {
			logger.DisableDestination(dest)
		} else {
			logger.EnableDestination(dest)
		}
		logger.Info("Toggle test message")
	}
}