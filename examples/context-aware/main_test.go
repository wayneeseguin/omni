package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wayneeseguin/omni/pkg/omni"
)

func TestMain(m *testing.M) {
	// Setup: clean up any existing test files
	os.Remove("test_context.log")

	code := m.Run()

	// Cleanup: remove test files
	os.Remove("test_context.log")
	os.Exit(code)
}

func TestContextAwareExample(t *testing.T) {
	// Test logger creation
	logger, err := omni.New("test_context.log")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer func() {
		time.Sleep(10 * time.Millisecond) // Allow async operations to complete
		if err := logger.Close(); err != nil {
			t.Errorf("Error during close: %v", err)
		}
	}()

	// Set level to TRACE to see detailed context tracking
	logger.SetLevel(omni.LevelTrace)

	// Test handling a single request with context tracking
	handleRequest(logger, "test-req-1")

	// Flush to ensure all messages are written
	logger.FlushAll()

	// Verify log file was created and has content
	if stat, err := os.Stat("test_context.log"); err != nil {
		t.Errorf("Log file was not created: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Log file is empty")
	}
}

func TestRequestHandling(t *testing.T) {
	testLogDir := "test_context_requests"
	os.MkdirAll(testLogDir, 0755)
	defer os.RemoveAll(testLogDir)

	logger, err := omni.New(filepath.Join(testLogDir, "requests.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	logger.SetLevel(omni.LevelTrace)

	// Test multiple request handling
	requests := []string{"req-1", "req-2", "req-3"}
	for _, reqID := range requests {
		handleRequest(logger, reqID)
	}

	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
	logger.Close()

	// Verify log file has content
	logFile := filepath.Join(testLogDir, "requests.log")
	if stat, err := os.Stat(logFile); err != nil {
		t.Errorf("Log file error: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Log file is empty")
	}
}

func TestFetchUser(t *testing.T) {
	testLogDir := "test_fetch_user"
	os.MkdirAll(testLogDir, 0755)
	defer os.RemoveAll(testLogDir)

	logger, err := omni.New(filepath.Join(testLogDir, "fetch_user.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	logger.SetLevel(omni.LevelTrace)

	// Test successful user fetch
	ctx := context.WithValue(context.Background(), RequestIDKey{}, "test-fetch-1")
	err = fetchUser(ctx, logger)
	if err != nil {
		t.Errorf("fetchUser should not return error: %v", err)
	}

	// Test cancelled context
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), RequestIDKey{}, "test-fetch-cancelled"))
	cancel() // Cancel immediately
	err = fetchUser(ctx, logger)
	if err == nil {
		t.Error("fetchUser should return error for cancelled context")
	}

	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
	logger.Close()

	// Verify log file has content
	logFile := filepath.Join(testLogDir, "fetch_user.log")
	if stat, err := os.Stat(logFile); err != nil {
		t.Errorf("Log file error: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Log file is empty")
	}
}

func TestValidatePermissions(t *testing.T) {
	testLogDir := "test_validate_permissions"
	os.MkdirAll(testLogDir, 0755)
	defer os.RemoveAll(testLogDir)

	logger, err := omni.New(filepath.Join(testLogDir, "validate_permissions.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	logger.SetLevel(omni.LevelTrace)

	// Test successful permission validation
	ctx := context.WithValue(context.Background(), RequestIDKey{}, "test-perm-1")
	err = validatePermissions(ctx, logger)
	if err != nil {
		t.Errorf("validatePermissions should not return error: %v", err)
	}

	// Test cancelled context
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), RequestIDKey{}, "test-perm-cancelled"))
	cancel() // Cancel immediately
	err = validatePermissions(ctx, logger)
	if err == nil {
		t.Error("validatePermissions should return error for cancelled context")
	}

	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
	logger.Close()

	// Verify log file has content
	logFile := filepath.Join(testLogDir, "validate_permissions.log")
	if stat, err := os.Stat(logFile); err != nil {
		t.Errorf("Log file error: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Log file is empty")
	}
}

func TestGetRequestID(t *testing.T) {
	// Test with valid request ID
	ctx := context.WithValue(context.Background(), RequestIDKey{}, "test-123")
	reqID := getRequestID(ctx)
	if reqID != "test-123" {
		t.Errorf("Expected request ID 'test-123', got '%s'", reqID)
	}

	// Test with missing request ID
	ctx = context.Background()
	reqID = getRequestID(ctx)
	if reqID != "unknown" {
		t.Errorf("Expected request ID 'unknown', got '%s'", reqID)
	}

	// Test with wrong type in context
	ctx = context.WithValue(context.Background(), RequestIDKey{}, 12345)
	reqID = getRequestID(ctx)
	if reqID != "unknown" {
		t.Errorf("Expected request ID 'unknown' for wrong type, got '%s'", reqID)
	}
}

func TestContextTimeout(t *testing.T) {
	testLogDir := "test_context_timeout"
	os.MkdirAll(testLogDir, 0755)
	defer os.RemoveAll(testLogDir)

	logger, err := omni.New(filepath.Join(testLogDir, "timeout.log"))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	logger.SetLevel(omni.LevelTrace)

	// Test with very short timeout
	ctx := context.WithValue(context.Background(), RequestIDKey{}, "test-timeout")
	ctx, cancel := context.WithTimeout(ctx, 1*time.Microsecond) // Very short timeout
	defer cancel()

	// Wait for timeout
	time.Sleep(1 * time.Millisecond)

	err = fetchUser(ctx, logger)
	if err == nil {
		t.Error("fetchUser should return error for timed out context")
	}

	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
	logger.Close()

	// Verify log file has content
	logFile := filepath.Join(testLogDir, "timeout.log")
	if stat, err := os.Stat(logFile); err != nil {
		t.Errorf("Log file error: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Log file is empty")
	}
}

// Benchmark tests
func BenchmarkHandleRequest(b *testing.B) {
	testLogDir := "bench_context"
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
		handleRequest(logger, "bench-req")
	}
}

func BenchmarkGetRequestID(b *testing.B) {
	ctx := context.WithValue(context.Background(), RequestIDKey{}, "bench-123")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		getRequestID(ctx)
	}
}
