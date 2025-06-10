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

		if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "deadline exceeded") {
			t.Errorf("Expected timeout or deadline exceeded error, got: %v", err)
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
	err = logger.Close()
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

// TestShutdownWithActiveWorkers tests shutdown when workers are actively processing
func TestShutdownWithActiveWorkers(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "worker_shutdown.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Enable features that create workers
	logger.SetCompression(CompressionGzip)
	logger.SetMaxSize(1024)     // Small size to trigger rotation
	logger.SetMaxAge(time.Hour) // Enable cleanup routine

	// Start heavy logging to keep workers busy
	go func() {
		for i := 0; i < 1000; i++ {
			logger.Infof("Worker test message %d with content %s", i, strings.Repeat("X", 100))
			time.Sleep(time.Millisecond)
		}
	}()

	// Let workers get busy
	time.Sleep(100 * time.Millisecond)

	// Shutdown should wait for workers to complete
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	err = logger.Shutdown(ctx)
	shutdownDuration := time.Since(start)

	if err != nil {
		t.Errorf("Shutdown with active workers failed: %v", err)
	}

	t.Logf("Shutdown with active workers took: %v", shutdownDuration)

	// Verify logger is properly closed
	if !logger.IsClosed() {
		t.Error("Logger should be closed after shutdown")
	}
}

// TestForceShutdownOnTimeout tests forced shutdown when graceful shutdown times out
func TestForceShutdownOnTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "force_shutdown.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Add multiple destinations to slow down shutdown
	for i := 0; i < 5; i++ {
		destPath := filepath.Join(tmpDir, fmt.Sprintf("dest_%d.log", i))
		if err := logger.AddDestination(destPath); err != nil {
			t.Fatalf("Failed to add destination %d: %v", i, err)
		}
	}

	// Use a channel to control the goroutine
	stopLogging := make(chan struct{})
	loggerDone := make(chan struct{})

	// Flood the logger with messages continuously
	go func() {
		defer close(loggerDone)
		// Keep sending messages to ensure channel stays full
		for {
			select {
			case <-stopLogging:
				return
			default:
				// Send messages rapidly to keep channel full
				logger.Infof("Force shutdown test message with longer content to ensure processing takes time")
			}
		}
	}()

	// Let messages accumulate in the channel
	time.Sleep(50 * time.Millisecond)

	// Use an extremely short timeout to force timeout scenario
	// Using 1 nanosecond to ensure timeout happens
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	start := time.Now()
	err = logger.Shutdown(ctx)
	shutdownDuration := time.Since(start)

	// Should return timeout error
	if err == nil {
		t.Error("Expected timeout error for short shutdown timeout")
	} else if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "deadline exceeded") {
		t.Errorf("Expected timeout error, got: %v", err)
	}

	t.Logf("Force shutdown attempt took: %v", shutdownDuration)

	// Stop the logging goroutine
	close(stopLogging)
	<-loggerDone

	// Clean up with proper timeout
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	logger.Shutdown(ctx2)

	// Give a small amount of time for any background cleanup to complete
	time.Sleep(10 * time.Millisecond)
}

// TestShutdownErrorRecovery tests shutdown behavior when errors occur during shutdown
func TestShutdownErrorRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "error_shutdown.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Add multiple destinations
	for i := 0; i < 3; i++ {
		destPath := filepath.Join(tmpDir, fmt.Sprintf("dest_%d.log", i))
		err = logger.AddDestination(destPath)
		if err != nil {
			t.Fatalf("Failed to add destination %d: %v", i, err)
		}
	}

	// Log some messages
	for i := 0; i < 10; i++ {
		logger.Infof("Shutdown error test %d", i)
	}

	// Cause errors by removing log directory
	os.RemoveAll(tmpDir)

	// Shutdown should handle errors gracefully
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = logger.Shutdown(ctx)
	// May return error due to file operations failing, but should not panic
	if err != nil {
		t.Logf("Shutdown returned error (expected): %v", err)
	}

	// Should still be marked as closed
	if !logger.IsClosed() {
		t.Error("Logger should be closed even after shutdown errors")
	}
}

// TestConcurrentShutdownCalls tests multiple concurrent shutdown calls
func TestConcurrentShutdownCalls(t *testing.T) {
	logger, err := New("/tmp/concurrent_shutdown.log")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Start logging
	go func() {
		for i := 0; i < 1000; i++ {
			logger.Infof("Concurrent shutdown test %d", i)
			time.Sleep(time.Millisecond)
		}
	}()

	var wg sync.WaitGroup
	numShutdowns := 10
	errors := make([]error, numShutdowns)

	// Start multiple concurrent shutdowns
	for i := 0; i < numShutdowns; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			errors[idx] = logger.Shutdown(ctx)
		}(i)
	}

	wg.Wait()

	// All shutdowns should complete without panic
	// First shutdown should succeed, others may return immediately
	successCount := 0
	for i, err := range errors {
		if err == nil {
			successCount++
		} else {
			t.Logf("Shutdown %d returned error: %v", i, err)
		}
	}

	if successCount == 0 {
		t.Error("At least one shutdown should succeed")
	}

	// Logger should be closed
	if !logger.IsClosed() {
		t.Error("Logger should be closed after concurrent shutdowns")
	}
}

// TestShutdownMetrics tests that metrics are properly finalized during shutdown
func TestShutdownMetrics(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "metrics_shutdown.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Log messages
	numMessages := 100
	for i := 0; i < numMessages; i++ {
		logger.Infof("Metrics test message %d", i)
	}

	// Get metrics before shutdown
	preShutdownMetrics := logger.GetMetrics()

	// Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = logger.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	// Get metrics after shutdown
	postShutdownMetrics := logger.GetMetrics()

	// Metrics should be finalized
	t.Logf("Pre-shutdown - Logged: %d, Dropped: %d, Errors: %d",
		preShutdownMetrics.MessagesLogged, preShutdownMetrics.MessagesDropped, preShutdownMetrics.ErrorCount)
	t.Logf("Post-shutdown - Logged: %d, Dropped: %d, Errors: %d",
		postShutdownMetrics.MessagesLogged, postShutdownMetrics.MessagesDropped, postShutdownMetrics.ErrorCount)

	// Should have processed all or most messages
	if postShutdownMetrics.MessagesLogged == 0 {
		t.Error("Expected some messages to be logged")
	}

	// Total messages should be accounted for
	totalProcessed := postShutdownMetrics.MessagesLogged + postShutdownMetrics.MessagesDropped
	if totalProcessed < uint64(numMessages/2) { // Allow for some race conditions
		t.Errorf("Expected at least %d messages processed, got %d", numMessages/2, totalProcessed)
	}
}

// TestShutdownWithChannelDrain tests that shutdown properly drains the message channel
func TestShutdownWithChannelDrain(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "channel_drain.log")

	// Set small channel size
	os.Setenv("OMNI_CHANNEL_SIZE", "10")
	defer os.Unsetenv("OMNI_CHANNEL_SIZE")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Fill channel with more messages than capacity
	numMessages := 50
	for i := 0; i < numMessages; i++ {
		logger.Infof("Channel drain test %d", i)
	}

	// Small delay to let some messages queue up
	time.Sleep(10 * time.Millisecond)

	// Shutdown should drain remaining messages
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = logger.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	// Check that messages were processed
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	messageCount := strings.Count(string(content), "Channel drain test")
	t.Logf("Processed %d out of %d messages during shutdown", messageCount, numMessages)

	// Should have processed at least some messages
	if messageCount == 0 {
		t.Error("Expected some messages to be processed during shutdown")
	}
}

// TestShutdownWithFlush tests that shutdown properly flushes all destinations
func TestShutdownWithFlush(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "flush_shutdown.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Add multiple destinations
	dest2 := filepath.Join(tmpDir, "dest2.log")
	dest3 := filepath.Join(tmpDir, "dest3.log")
	logger.AddDestination(dest2)
	logger.AddDestination(dest3)

	// Log messages
	for i := 0; i < 20; i++ {
		logger.Infof("Flush test message %d", i)
	}

	// Shutdown should flush all destinations
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = logger.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	// Check all destinations have content
	destinations := []string{logPath, dest2, dest3}
	for i, dest := range destinations {
		content, err := os.ReadFile(dest)
		if err != nil {
			t.Errorf("Failed to read destination %d (%s): %v", i, dest, err)
			continue
		}

		messageCount := strings.Count(string(content), "Flush test message")
		t.Logf("Destination %d has %d messages", i, messageCount)

		if messageCount == 0 {
			t.Errorf("Destination %d should have messages after flush", i)
		}
	}
}

// TestShutdownReentrance tests that shutdown is reentrant and safe
func TestShutdownReentrance(t *testing.T) {
	logger, err := New("/tmp/reentrant_shutdown.log")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// First shutdown
	ctx1, cancel1 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel1()

	err1 := logger.Shutdown(ctx1)
	if err1 != nil {
		t.Errorf("First shutdown failed: %v", err1)
	}

	// Subsequent shutdowns should be safe and fast
	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)

		start := time.Now()
		err := logger.Shutdown(ctx)
		duration := time.Since(start)
		cancel()

		if err != nil {
			t.Errorf("Reentrant shutdown %d failed: %v", i, err)
		}

		// Should return quickly since already shut down
		if duration > 50*time.Millisecond {
			t.Errorf("Reentrant shutdown %d took too long: %v", i, duration)
		}
	}

	// Should still be closed
	if !logger.IsClosed() {
		t.Error("Logger should remain closed after reentrant shutdowns")
	}
}
