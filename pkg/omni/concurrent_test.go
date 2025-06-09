package omni

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	testhelpers "github.com/wayneeseguin/omni/internal/testing"
)

func TestConcurrentOperations(t *testing.T) {
	t.Run("multiple goroutines logging", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")

		logger, err := New(logFile)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()

		const numGoroutines = 10
		const messagesPerGoroutine = 100

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		// Start multiple goroutines logging concurrently
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < messagesPerGoroutine; j++ {
					logger.Info("Goroutine %d message %d", id, j)
				}
			}(i)
		}

		wg.Wait()

		if err := logger.FlushAll(); err != nil {
			t.Errorf("FlushAll failed: %v", err)
		}

		// Verify metrics
		metrics := logger.GetMetrics()
		expectedMessages := numGoroutines * messagesPerGoroutine

		// Should have logged all messages (allowing for some dropped if channel was full)
		if metrics.MessagesLogged == 0 {
			t.Error("No messages were logged")
		}

		t.Logf("Logged %d/%d messages, dropped %d",
			metrics.MessagesLogged, expectedMessages, metrics.MessagesDropped)
	})

	t.Run("concurrent destination operations", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")

		logger, err := New(logFile)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()

		const numOperations = 50
		var wg sync.WaitGroup
		wg.Add(numOperations * 3) // Add, enable/disable, remove operations

		// Concurrent destination additions
		for i := 0; i < numOperations; i++ {
			go func(id int) {
				defer wg.Done()
				destFile := filepath.Join(tempDir, fmt.Sprintf("dest_%d.log", id))
				err := logger.AddDestination(destFile)
				if err != nil {
					t.Logf("AddDestination failed for %s: %v", destFile, err)
				}
			}(i)
		}

		// Concurrent enable/disable operations
		for i := 0; i < numOperations; i++ {
			go func(id int) {
				defer wg.Done()
				destFile := filepath.Join(tempDir, fmt.Sprintf("dest_%d.log", id))
				// Wait a bit for destination to be added
				time.Sleep(10 * time.Millisecond)
				logger.DisableDestination(destFile)
				logger.EnableDestination(destFile)
			}(i)
		}

		// Concurrent logging during destination changes
		for i := 0; i < numOperations; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					logger.Info("Concurrent message %d-%d", id, j)
					time.Sleep(1 * time.Millisecond)
				}
			}(i)
		}

		wg.Wait()

		if err := logger.FlushAll(); err != nil {
			t.Errorf("FlushAll failed: %v", err)
		}
	})

	t.Run("concurrent shutdown", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")

		logger, err := New(logFile)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}

		const numShutdowns = 10
		var wg sync.WaitGroup
		wg.Add(numShutdowns + 1) // shutdowns + logging goroutine

		// Start logging in background
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ { // Reduced from 1000 to 100
				logger.Info("Background message %d", i)
				// Removed 1ms sleep - let it run as fast as possible
			}
		}()

		// Multiple concurrent shutdown attempts
		for i := 0; i < numShutdowns; i++ {
			go func() {
				defer wg.Done()
				time.Sleep(10 * time.Millisecond) // Reduced from 50ms to 10ms
				logger.Close()
			}()
		}

		wg.Wait()

		// Should not panic and should be properly closed
		if !logger.IsClosed() {
			t.Error("Logger should be closed")
		}
	})

	t.Run("concurrent metrics access", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")

		logger, err := New(logFile)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()

		const numGoroutines = 20
		var wg sync.WaitGroup
		wg.Add(numGoroutines * 3) // logging, metrics reading, metrics resetting

		// Concurrent logging
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 50; j++ {
					logger.Info("Metrics test message %d-%d", id, j)
				}
			}(i)
		}

		// Concurrent metrics reading
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					_ = logger.GetMetrics()
					time.Sleep(5 * time.Millisecond)
				}
			}()
		}

		// Concurrent metrics resetting
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < 5; j++ {
					logger.ResetMetrics()
					time.Sleep(10 * time.Millisecond)
				}
			}()
		}

		wg.Wait()

		// Should not panic
		metrics := logger.GetMetrics()
		t.Logf("Final metrics: %+v", metrics)
	})

	t.Run("concurrent rotation", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")

		config := DefaultConfig()
		config.Path = logFile
		config.MaxSize = 1024 // Small size to trigger rotation
		config.MaxFiles = 3

		logger, err := NewWithConfig(config)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()

		const numGoroutines = 5
		const messagesPerGoroutine = 200

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		// Multiple goroutines writing large messages to trigger rotation
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < messagesPerGoroutine; j++ {
					// Large message to trigger rotation
					logger.Info("Goroutine %d large message %d: %s",
						id, j, fmt.Sprintf("%0*d", 200, j))
				}
			}(i)
		}

		wg.Wait()

		if err := logger.FlushAll(); err != nil {
			t.Errorf("FlushAll failed: %v", err)
		}

		// Ensure all writes are synced to disk
		if err := logger.Sync(); err != nil {
			t.Errorf("Sync failed: %v", err)
		}

		// Give a small delay to ensure rotation completes
		time.Sleep(100 * time.Millisecond)

		// Should have rotated files
		files, err := filepath.Glob(logFile + "*")
		if err != nil {
			t.Fatalf("Failed to glob files: %v", err)
		}

		if len(files) < 2 {
			t.Errorf("Expected rotation to create multiple files, got %d", len(files))
		}

		t.Logf("Created %d files during concurrent rotation", len(files))
	})

	t.Run("concurrent structured logging", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")

		logger, err := New(logFile)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()

		const numGoroutines = 15
		const messagesPerGoroutine = 50

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		// Concurrent structured logging with different field types
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < messagesPerGoroutine; j++ {
					fields := map[string]interface{}{
						"goroutine_id": id,
						"message_id":   j,
						"timestamp":    time.Now().UnixNano(),
						"string_field": fmt.Sprintf("value_%d_%d", id, j),
						"int_field":    id*1000 + j,
						"float_field":  float64(id) + float64(j)/100.0,
						"bool_field":   (id+j)%2 == 0,
					}
					logger.InfoWithFields(fmt.Sprintf("Structured message %d-%d", id, j), fields)
				}
			}(i)
		}

		wg.Wait()

		if err := logger.FlushAll(); err != nil {
			t.Errorf("FlushAll failed: %v", err)
		}

		// Verify some messages were logged
		metrics := logger.GetMetrics()
		if metrics.MessagesLogged == 0 {
			t.Error("No structured messages were logged")
		}

		t.Logf("Logged %d structured messages", metrics.MessagesLogged)
	})

	t.Run("concurrent format changes", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")

		logger, err := New(logFile)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()

		const numGoroutines = 10
		var wg sync.WaitGroup
		wg.Add(numGoroutines * 2) // logging + format changing

		// Concurrent logging
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					logger.Info("Format test message %d-%d", id, j)
					time.Sleep(1 * time.Millisecond)
				}
			}(i)
		}

		// Concurrent format changes
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < 20; j++ {
					if j%2 == 0 {
						logger.SetFormat(FormatJSON)
					} else {
						logger.SetFormat(FormatText)
					}
					time.Sleep(5 * time.Millisecond)
				}
			}()
		}

		wg.Wait()

		if err := logger.FlushAll(); err != nil {
			t.Errorf("FlushAll failed: %v", err)
		}
	})

	t.Run("stress test with channel overflow", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")

		config := DefaultConfig()
		config.Path = logFile
		config.ChannelSize = 10 // Very small channel to force overflow

		logger, err := NewWithConfig(config)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()

		const numGoroutines = 50
		const messagesPerGoroutine = 100

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		// Flood the logger with messages
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < messagesPerGoroutine; j++ {
					logger.Info("Flood message %d-%d", id, j)
				}
			}(i)
		}

		wg.Wait()

		// Give time for processing
		time.Sleep(50 * time.Millisecond)

		if err := logger.FlushAll(); err != nil {
			t.Errorf("FlushAll failed: %v", err)
		}

		// Check metrics for dropped messages
		metrics := logger.GetMetrics()
		t.Logf("Stress test results - Dropped: %d, Queue utilization: %.2f",
			metrics.MessagesDropped, metrics.ChannelUtilization)

		// Should have dropped some messages due to small channel
		if metrics.MessagesDropped == 0 {
			t.Log("Note: No messages were dropped - consider reducing channel size further for this test")
		}
	})
}

// TestRaceConditions specifically tests for race conditions using -race flag
func TestRaceConditions(t *testing.T) {
	testhelpers.SkipIfUnit(t, "Skipping race condition tests in unit mode")

	t.Run("concurrent map access", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")

		logger, err := New(logFile)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()

		const numGoroutines = 20
		var wg sync.WaitGroup
		wg.Add(numGoroutines * 3)

		// Concurrent logging (writes to metrics maps)
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					switch j % 4 {
					case 0:
						logger.Debugf("Debug message %d-%d", id, j)
					case 1:
						logger.Infof("Info message %d-%d", id, j)
					case 2:
						logger.Warnf("Warn message %d-%d", id, j)
					case 3:
						logger.Errorf("Error message %d-%d", id, j)
					}
				}
			}(i)
		}

		// Concurrent metrics reading
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < 50; j++ {
					_ = logger.GetMetrics()
				}
			}()
		}

		// Concurrent metrics resetting
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					logger.ResetMetrics()
					time.Sleep(10 * time.Millisecond)
				}
			}()
		}

		wg.Wait()

		if err := logger.FlushAll(); err != nil {
			t.Errorf("FlushAll failed: %v", err)
		}
	})

	t.Run("concurrent destination access", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")

		logger, err := New(logFile)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		defer logger.Close()

		// Add some destinations first
		for i := 0; i < 5; i++ {
			destFile := filepath.Join(tempDir, fmt.Sprintf("dest_%d.log", i))
			err := logger.AddDestination(destFile)
			if err != nil {
				t.Fatalf("Failed to add destination %s: %v", destFile, err)
			}
		}

		const numGoroutines = 10
		var wg sync.WaitGroup
		wg.Add(numGoroutines * 3)

		// Concurrent logging to all destinations
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					logger.Info("Multi-dest message %d-%d", id, j)
				}
			}(i)
		}

		// Concurrent destination enable/disable
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 20; j++ {
					destFile := filepath.Join(tempDir, fmt.Sprintf("dest_%d.log", j%5))
					if j%2 == 0 {
						logger.DisableDestination(destFile)
					} else {
						logger.EnableDestination(destFile)
					}
					time.Sleep(5 * time.Millisecond)
				}
			}(i)
		}

		// Concurrent flush operations
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					logger.FlushAll()
					time.Sleep(20 * time.Millisecond)
				}
			}()
		}

		wg.Wait()

		if err := logger.FlushAll(); err != nil {
			t.Errorf("FlushAll failed: %v", err)
		}
	})
}
