package omni

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestShutdownOperations(t *testing.T) {
	t.Run("GracefulShutdown", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "test_shutdown.log")

		logger, err := New(logPath)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}

		// Write some messages
		messageCount := 10
		for i := 0; i < messageCount; i++ {
			logger.Infof("Message %d", i)
		}

		// Shutdown with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = logger.Shutdown(ctx)
		if err != nil {
			t.Errorf("Shutdown failed: %v", err)
		}

		// Verify all messages were written
		content, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("Failed to read log file: %v", err)
		}

		for i := 0; i < messageCount; i++ {
			expected := fmt.Sprintf("Message %d", i)
			if !strings.Contains(string(content), expected) {
				t.Errorf("Missing message %d in log file", i)
			}
		}

		// Verify logger is closed
		if !logger.IsClosed() {
			t.Error("Logger should be closed after shutdown")
		}
	})

	t.Run("ShutdownTimeout", func(t *testing.T) {
		logger, err := New("/tmp/test.log")
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}

		// Create a very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		// This should timeout
		err = logger.Shutdown(ctx)
		if err == nil {
			t.Error("Expected timeout error")
		}

		if !strings.Contains(err.Error(), "timeout") {
			t.Errorf("Expected timeout error, got: %v", err)
		}
	})

	t.Run("ShutdownWithPendingMessages", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "test_pending.log")

		logger, err := New(logPath)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}

		// Fill the channel with messages
		for i := 0; i < defaultChannelSize*2; i++ {
			go func(n int) {
				logger.Info("Pending message %d", n)
			}(i)
		}

		// Give time for messages to queue
		time.Sleep(100 * time.Millisecond)

		// Shutdown should process all pending messages
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err = logger.Shutdown(ctx)
		if err != nil {
			t.Errorf("Shutdown failed: %v", err)
		}

		// Read log to verify messages were processed
		content, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("Failed to read log file: %v", err)
		}

		// Should have processed at least some messages
		if !strings.Contains(string(content), "Pending message") {
			t.Error("No pending messages were processed")
		}
	})

	t.Run("DoubleShutdown", func(t *testing.T) {
		logger, err := New("/tmp/test.log")
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}

		// First shutdown
		ctx := context.Background()
		err = logger.Shutdown(ctx)
		if err != nil {
			t.Errorf("First shutdown failed: %v", err)
		}

		// Second shutdown should be safe and return immediately
		err = logger.Shutdown(ctx)
		if err != nil {
			t.Errorf("Second shutdown failed: %v", err)
		}
	})
}

func TestShutdownWithWorkers(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test_workers.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Enable compression to start workers
	logger.SetCompression(CompressionGzip)

	// Set max age to start cleanup routine
	logger.SetMaxAge(24 * time.Hour)

	// Write some messages
	logger.Info("Test with workers")

	// Shutdown should stop all workers
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = logger.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	// Verify workers are stopped
	logger.mu.Lock()
	if logger.cleanupTicker != nil {
		t.Error("Cleanup ticker should be nil after shutdown")
	}
	if logger.compressCh != nil {
		// Channel should be closed
		select {
		case _, ok := <-logger.compressCh:
			if ok {
				t.Error("Compress channel should be closed")
			}
		default:
			t.Error("Compress channel not properly closed")
		}
	}
	logger.mu.Unlock()
}

func TestConcurrentShutdown(t *testing.T) {
	logger, err := New("/tmp/test.log")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Start multiple goroutines trying to shutdown
	var wg sync.WaitGroup
	shutdownCount := 10
	errors := make([]error, shutdownCount)

	for i := 0; i < shutdownCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			errors[idx] = logger.Shutdown(ctx)
		}(i)
	}

	wg.Wait()

	// All shutdowns should succeed without panic
	for i, err := range errors {
		if err != nil {
			t.Errorf("Shutdown %d failed: %v", i, err)
		}
	}
}

func TestCloseAll(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := New("/tmp/test.log")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Add multiple destinations
	for i := 0; i < 3; i++ {
		path := filepath.Join(tmpDir, fmt.Sprintf("dest%d.log", i))
		err = logger.AddDestination(path)
		if err != nil {
			t.Fatalf("Failed to add destination %d: %v", i, err)
		}
	}

	// Write to all destinations
	logger.Info("Test message to all")

	// Close all
	err = logger.CloseAll()
	if err != nil {
		t.Errorf("CloseAll failed: %v", err)
	}

	// Verify all destinations are closed
	if len(logger.Destinations) != 0 {
		t.Errorf("Expected 0 destinations after CloseAll, got %d", len(logger.Destinations))
	}

	// Verify logger is closed
	if !logger.IsClosed() {
		t.Error("Logger should be closed after CloseAll")
	}
}

func TestCloseOperation(t *testing.T) {
	logger, err := New("/tmp/test.log")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Close is an alias for CloseAll
	err = logger.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Verify logger is closed
	if !logger.IsClosed() {
		t.Error("Logger should be closed after Close")
	}
}

func TestShutdownMessageDraining(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test_drain.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Custom error handler to capture shutdown message
	var shutdownMessage string
	logger.SetErrorHandler(func(err LogError) {
		if strings.Contains(err.Message, "Shutting down") {
			shutdownMessage = err.Message
		}
	})

	// Queue many messages
	messageCount := 50
	for i := 0; i < messageCount; i++ {
		go logger.Info("Drain test %d", i)
	}

	// Give time for messages to queue
	time.Sleep(50 * time.Millisecond)

	// Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = logger.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	// Check if shutdown message included pending count
	if shutdownMessage != "" && !strings.Contains(shutdownMessage, "pending messages") {
		t.Logf("Shutdown message: %s", shutdownMessage)
	}

	// Verify messages were written
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// Count how many messages were actually written
	writtenCount := strings.Count(string(content), "Drain test")
	t.Logf("Written %d out of %d messages", writtenCount, messageCount)

	if writtenCount == 0 {
		t.Error("No messages were written during shutdown")
	}
}
