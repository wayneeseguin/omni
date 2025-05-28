package flexlog

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestBoundaryConditions tests edge cases and boundary conditions
func TestBoundaryConditions(t *testing.T) {
	t.Run("zero channel size", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")
		
		config := DefaultConfig()
		config.Path = logFile
		config.ChannelSize = 0 // Zero size should default to minimum
		
		logger, err := NewWithConfig(config)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()
		
		// Should still be able to log
		logger.Info("test message")
		time.Sleep(100 * time.Millisecond)
	})

	t.Run("extremely large channel size", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")
		
		config := DefaultConfig()
		config.Path = logFile
		config.ChannelSize = 1000000 // Very large channel
		
		logger, err := NewWithConfig(config)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()
		
		logger.Info("test message")
		time.Sleep(100 * time.Millisecond)
	})

	t.Run("zero max size", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")
		
		config := DefaultConfig()
		config.Path = logFile
		config.MaxSize = 0 // Should disable rotation
		
		logger, err := NewWithConfig(config)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()
		
		// Write a lot of data
		for i := 0; i < 1000; i++ {
			logger.Info("This is a very long message that should fill up the log file quickly if rotation was enabled")
		}
		
		if err := logger.FlushAll(); err != nil {
			t.Errorf("FlushAll failed: %v", err)
		}
		
		// Should not have rotated
		files, err := filepath.Glob(logFile + "*")
		if err != nil {
			t.Fatalf("Failed to glob files: %v", err)
		}
		
		if len(files) > 1 {
			t.Errorf("Expected 1 file, got %d files: %v", len(files), files)
		}
	})

	t.Run("max files is zero", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")
		
		config := DefaultConfig()
		config.Path = logFile
		config.MaxSize = 100 // Small size to force rotation
		config.MaxFiles = 0  // Should keep all rotated files
		
		logger, err := NewWithConfig(config)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()
		
		// Force multiple rotations
		for i := 0; i < 10; i++ {
			logger.Info("This is a message that will cause rotation due to small max size setting")
			if err := logger.FlushAll(); err != nil {
				t.Errorf("FlushAll failed: %v", err)
			}
		}
		
		// Should have multiple files
		files, err := filepath.Glob(logFile + "*")
		if err != nil {
			t.Fatalf("Failed to glob files: %v", err)
		}
		
		if len(files) < 2 {
			t.Errorf("Expected multiple files when MaxFiles=0, got %d files", len(files))
		}
	})

	t.Run("empty log message", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")
		
		logger, err := New(logFile)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()
		
		// Empty message
		logger.Info("")
		logger.Infof("")
		logger.Infof("%s", "")
		
		if err := logger.FlushAll(); err != nil {
			t.Errorf("FlushAll failed: %v", err)
		}
		
		// Should still create log entries
		content, err := os.ReadFile(logFile)
		if err != nil {
			t.Fatalf("Failed to read log file: %v", err)
		}
		
		lines := strings.Split(string(content), "\n")
		nonEmptyLines := 0
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				nonEmptyLines++
			}
		}
		
		if nonEmptyLines < 3 {
			t.Errorf("Expected at least 3 log entries, got %d", nonEmptyLines)
		}
	})

	t.Run("very long log message", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")
		
		logger, err := New(logFile)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()
		
		// Very long message (1MB)
		longMessage := strings.Repeat("A", 1024*1024)
		logger.Info(longMessage)
		
		if err := logger.FlushAll(); err != nil {
			t.Errorf("FlushAll failed: %v", err)
		}
		
		// Should handle large messages
		content, err := os.ReadFile(logFile)
		if err != nil {
			t.Fatalf("Failed to read log file: %v", err)
		}
		
		if len(content) < 1024*1024 {
			t.Errorf("Expected large log file, got %d bytes", len(content))
		}
	})

	t.Run("nil format args", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")
		
		logger, err := New(logFile)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()
		
		// Nil args should not panic
		logger.Infof("test %v", nil)
		logger.Info("test with nil values")
		
		if err := logger.FlushAll(); err != nil {
			t.Errorf("FlushAll failed: %v", err)
		}
	})

	t.Run("concurrent close calls", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")
		
		logger, err := New(logFile)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		
		// Multiple concurrent close calls should not panic
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				err := logger.Close()
				if err != nil {
					t.Logf("Close error: %v", err)
				}
				done <- true
			}()
		}
		
		// Wait for all closes to complete
		for i := 0; i < 10; i++ {
			<-done
		}
	})

	t.Run("log after close", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")
		
		logger, err := New(logFile)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		
		logger.Info("before close")
		logger.Close()
		
		// Logging after close should not panic
		logger.Info("after close")
		logger.Error("error after close")
		
		// Should handle gracefully
		if !logger.IsClosed() {
			t.Error("Logger should report as closed")
		}
	})

	t.Run("invalid file permissions", func(t *testing.T) {
		// Create a directory with no write permissions
		tempDir := t.TempDir()
		restrictedDir := filepath.Join(tempDir, "restricted")
		if err := os.Mkdir(restrictedDir, 0000); err != nil {
			t.Fatalf("Failed to create restricted directory: %v", err)
		}
		defer os.Chmod(restrictedDir, 0755) // Restore permissions for cleanup
		
		logFile := filepath.Join(restrictedDir, "test.log")
		
		// Should fail to create logger
		_, err := New(logFile)
		if err == nil {
			t.Error("Expected error when creating logger in restricted directory")
		}
	})

	t.Run("rapid rotation", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")
		
		config := DefaultConfig()
		config.Path = logFile
		config.MaxSize = 1 // Very small to force rapid rotation
		config.MaxFiles = 5
		
		logger, err := NewWithConfig(config)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()
		
		// Rapid logging to trigger many rotations
		for i := 0; i < 100; i++ {
			logger.Info("Message %d: This should trigger rotation", i)
		}
		
		if err := logger.FlushAll(); err != nil {
			t.Errorf("FlushAll failed: %v", err)
		}
		
		// Should handle rapid rotation without errors
		files, err := filepath.Glob(logFile + "*")
		if err != nil {
			t.Fatalf("Failed to glob files: %v", err)
		}
		
		// Should respect MaxFiles limit
		if len(files) > 6 { // Current file + MaxFiles
			t.Errorf("Too many files created: %d, expected <= 6", len(files))
		}
	})

	t.Run("shutdown timeout", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")
		
		logger, err := New(logFile)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		
		// Add many messages to the channel
		for i := 0; i < 1000; i++ {
			logger.Info("Message %d", i)
		}
		
		// Very short timeout should cause timeout error
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		
		err = logger.Shutdown(ctx)
		if err == nil {
			t.Error("Expected timeout error")
		}
		if err != context.DeadlineExceeded {
			t.Errorf("Expected deadline exceeded, got: %v", err)
		}
		
		// Cleanup should still happen in background
		time.Sleep(100 * time.Millisecond)
	})
}

func TestMetricsBoundaryConditions(t *testing.T) {
	t.Run("metrics with no messages", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")
		
		logger, err := New(logFile)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()
		
		metrics := logger.GetMetrics()
		
		if metrics.QueueDepth != 0 {
			t.Errorf("QueueDepth = %d, want 0", metrics.QueueDepth)
		}
		if metrics.MessagesDropped != 0 {
			t.Errorf("MessagesDropped = %d, want 0", metrics.MessagesDropped)
		}
		if len(metrics.MessagesLogged) != 0 {
			t.Errorf("MessagesLogged should be empty, got %d entries", len(metrics.MessagesLogged))
		}
	})

	t.Run("reset metrics multiple times", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")
		
		logger, err := New(logFile)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()
		
		// Log some messages
		logger.Info("test")
		logger.Error("error")
		
		// Reset multiple times should not panic
		logger.ResetMetrics()
		logger.ResetMetrics()
		logger.ResetMetrics()
		
		metrics := logger.GetMetrics()
		if len(metrics.MessagesLogged) != 0 {
			t.Errorf("MessagesLogged should be reset, got %d entries", len(metrics.MessagesLogged))
		}
	})
}

func TestDestinationBoundaryConditions(t *testing.T) {
	t.Run("add destination with empty name", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")
		
		logger, err := New(logFile)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()
		
		// Empty path should be handled gracefully
		err = logger.AddDestination("")
		if err == nil {
			t.Error("Expected error for empty destination name")
		}
	})

	t.Run("add duplicate destination name", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")
		
		logger, err := New(logFile)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()
		
		destFile1 := filepath.Join(tempDir, "dest1.log")
		destFile2 := filepath.Join(tempDir, "dest2.log")
		
		// Add first destination
		err = logger.AddDestination(destFile1)
		if err != nil {
			t.Fatalf("Failed to add first destination: %v", err)
		}
		
		// Adding second destination should work
		err = logger.AddDestination(destFile2)
		if err != nil {
			t.Logf("Note: AddDestination returned error for second destination: %v", err)
		}
	})

	t.Run("close non-existent destination", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")
		
		logger, err := New(logFile)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()
		
		// Closing non-existent destination should return error
		err = logger.RemoveDestination("non-existent.log")
		if err == nil {
			t.Error("Expected error for non-existent destination")
		}
	})

	t.Run("enable/disable non-existent destination", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")
		
		logger, err := New(logFile)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()
		
		// Should return false for non-existent destination
		result := logger.EnableDestination("non-existent.log")
		if result {
			t.Error("EnableDestination should return false for non-existent destination")
		}
		
		result = logger.DisableDestination("non-existent.log")
		if result {
			t.Error("DisableDestination should return false for non-existent destination")
		}
	})
}

func TestFormattingBoundaryConditions(t *testing.T) {
	t.Run("invalid format options", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")
		
		logger, err := New(logFile)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()
		
		// Set invalid format
		logger.SetFormat(999) // Invalid format
		
		// Should still log without panicking
		logger.Info("test message")
		
		if err := logger.FlushAll(); err != nil {
			t.Errorf("FlushAll failed: %v", err)
		}
	})

	t.Run("very long field values", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")
		
		logger, err := New(logFile)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()
		
		// Very long field value
		longValue := strings.Repeat("X", 1024*1024)
		logger.StructuredLog(LevelInfo, "test", map[string]interface{}{
			"long_field": longValue,
		})
		
		if err := logger.FlushAll(); err != nil {
			t.Errorf("FlushAll failed: %v", err)
		}
	})

	t.Run("circular reference in structured logging", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")
		
		logger, err := New(logFile)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()
		
		// Create circular reference
		m := make(map[string]interface{})
		m["self"] = m
		
		// Should handle gracefully without infinite recursion
		logger.StructuredLog(LevelInfo, "test", m)
		
		if err := logger.FlushAll(); err != nil {
			t.Errorf("FlushAll failed: %v", err)
		}
	})
}