package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wayneeseguin/omni/pkg/omni"
)

func TestMain(m *testing.M) {
	// Setup: create test directory
	os.MkdirAll("test_logs", 0755)
	defer os.RemoveAll("test_logs")

	code := m.Run()
	os.Exit(code)
}

func TestAdvancedFeaturesExample(t *testing.T) {
	// Create test log directory
	testLogDir := "test_logs"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	// Test simple logger creation first
	logger, err := omni.New(filepath.Join(testLogDir, "test_app.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Test graceful shutdown
	defer func() {
		if err := logger.Close(); err != nil {
			t.Errorf("Error during close: %v", err)
		}
	}()

	// Test basic logging
	logger.Trace("Test trace message")
	logger.Debug("Test debug message")
	logger.Info("Test info message")
	logger.Warn("Test warn message")
	logger.Error("Test error message")

	// Test structured logging
	logger.InfoWithFields("Test structured log", map[string]interface{}{
		"test_field": "test_value",
		"number":     42,
	})

	// Test processRequest function
	processRequest(logger, "test_user", "test_action")

	// Test error handling with structured logging
	err = doSomethingThatFails()
	if err != nil {
		logger.ErrorWithFields("Test error with details", map[string]interface{}{
			"error": err.Error(),
			"test":  true,
		})
	}

	// Flush logs
	logger.FlushAll()

	// Verify log file was created
	logFile := filepath.Join(testLogDir, "test_app.log")
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Errorf("Log file was not created: %s", logFile)
	}
}

func TestProcessRequest(t *testing.T) {
	// Create test logger
	testLogDir := "test_logs_process"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	logger, err := omni.New(filepath.Join(testLogDir, "process_test.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer func() {
		if err := logger.Close(); err != nil {
			t.Errorf("Error during close: %v", err)
		}
	}()

	// Set trace level to see all messages
	logger.SetLevel(omni.LevelTrace)

	// Test processRequest function
	processRequest(logger, "test_user_123", "login")

	// Flush to ensure logs are written
	logger.FlushAll()

	// Verify log file exists and has content
	logFile := filepath.Join(testLogDir, "process_test.log")
	if stat, err := os.Stat(logFile); err != nil {
		t.Errorf("Log file error: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Log file is empty")
	}
}

func TestGenerateTransactionID(t *testing.T) {
	id1 := generateTransactionID()
	id2 := generateTransactionID()

	// Test that IDs are generated
	if id1 == "" {
		t.Error("Transaction ID should not be empty")
	}
	if id2 == "" {
		t.Error("Transaction ID should not be empty")
	}

	// Test that IDs are different
	if id1 == id2 {
		t.Error("Transaction IDs should be unique")
	}

	// Test that ID has expected length
	if len(id1) != 16 {
		t.Errorf("Expected transaction ID length of 16, got %d", len(id1))
	}
}

func TestGenerateRandomString(t *testing.T) {
	tests := []int{1, 5, 10, 50, 100}

	for _, length := range tests {
		str := generateRandomString(length)
		if len(str) != length {
			t.Errorf("Expected string length %d, got %d", length, len(str))
		}

		// Test that string contains only expected characters
		const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		for _, char := range str {
			found := false
			for _, validChar := range charset {
				if char == validChar {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("String contains invalid character: %c", char)
			}
		}
	}
}

func TestDoSomethingThatFails(t *testing.T) {
	err := doSomethingThatFails()
	if err == nil {
		t.Error("doSomethingThatFails should return an error")
	}

	expectedMsg := "simulated failure in nested function"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

// Benchmark tests
func BenchmarkProcessRequest(b *testing.B) {
	testLogDir := "bench_logs"
	os.MkdirAll(testLogDir, 0755)
	defer os.RemoveAll(testLogDir)

	logger, err := omni.New(filepath.Join(testLogDir, "bench.log"))
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer func() {
		if err := logger.Close(); err != nil {
			b.Errorf("Error during close: %v", err)
		}
	}()

	// Set higher log level for performance
	logger.SetLevel(omni.LevelInfo)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		processRequest(logger, "bench_user", "bench_action")
	}
}

func BenchmarkGenerateTransactionID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		generateTransactionID()
	}
}
