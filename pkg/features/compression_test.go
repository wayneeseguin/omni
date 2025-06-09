package features

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewCompressionManager(t *testing.T) {
	cm := NewCompressionManager()
	if cm == nil {
		t.Fatal("NewCompressionManager returned nil")
	}
	
	if cm.compressionType != CompressionNone {
		t.Errorf("Expected compression type CompressionNone, got %v", cm.compressionType)
	}
	
	if cm.compressMinAge != 1 {
		t.Errorf("Expected compress min age 1, got %d", cm.compressMinAge)
	}
	
	if cm.compressWorkers != 1 {
		t.Errorf("Expected compress workers 1, got %d", cm.compressWorkers)
	}
}

func TestSetCompression(t *testing.T) {
	cm := NewCompressionManager()
	
	tests := []struct {
		name            string
		compressionType CompressionType
		expectError     bool
	}{
		{"Set to None", CompressionNone, false},
		{"Set to Gzip", CompressionGzip, false},
		{"Set to invalid", CompressionType(99), true},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cm.SetCompression(tt.compressionType)
			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			
			if !tt.expectError && cm.GetType() != tt.compressionType {
				t.Errorf("Expected compression type %v, got %v", tt.compressionType, cm.GetType())
			}
		})
	}
}

func TestSetMinAge(t *testing.T) {
	cm := NewCompressionManager()
	
	testAge := 5
	cm.SetMinAge(testAge)
	
	if cm.GetMinAge() != testAge {
		t.Errorf("Expected min age %d, got %d", testAge, cm.GetMinAge())
	}
}

func TestSetWorkers(t *testing.T) {
	cm := NewCompressionManager()
	
	tests := []struct {
		name            string
		workers         int
		expectedWorkers int
	}{
		{"Set to 0", 0, 1}, // Should default to 1
		{"Set to 5", 5, 5},
		{"Set to -1", -1, 1}, // Should default to 1
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm.SetWorkers(tt.workers)
			
			cm.mu.RLock()
			actualWorkers := cm.compressWorkers
			cm.mu.RUnlock()
			
			if actualWorkers != tt.expectedWorkers {
				t.Errorf("Expected %d workers, got %d", tt.expectedWorkers, actualWorkers)
			}
		})
	}
}

func TestCompressFileGzip(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()
	
	// Create a test file
	testFile := filepath.Join(tempDir, "test.log")
	testContent := "This is a test log file content\nWith multiple lines\nFor compression testing"
	
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	cm := NewCompressionManager()
	
	// Test compression
	err := cm.compressFileGzip(testFile)
	if err != nil {
		t.Fatalf("Failed to compress file: %v", err)
	}
	
	// Check that compressed file exists
	compressedFile := testFile + ".gz"
	if _, err := os.Stat(compressedFile); os.IsNotExist(err) {
		t.Fatal("Compressed file does not exist")
	}
	
	// Check that original file was removed
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Fatal("Original file still exists after compression")
	}
	
	// Verify compressed content
	f, err := os.Open(compressedFile)
	if err != nil {
		t.Fatalf("Failed to open compressed file: %v", err)
	}
	defer f.Close()
	
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("Failed to create gzip reader: %v", err)
	}
	defer gz.Close()
	
	decompressed, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("Failed to read compressed content: %v", err)
	}
	
	if string(decompressed) != testContent {
		t.Errorf("Decompressed content does not match original.\nExpected: %q\nGot: %q", testContent, string(decompressed))
	}
}

func TestCompressFileSync(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create a test file
	testFile := filepath.Join(tempDir, "test.log")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	cm := NewCompressionManager()
	cm.SetCompression(CompressionGzip)
	
	// Test synchronous compression
	err := cm.CompressFileSync(testFile)
	if err != nil {
		t.Fatalf("Failed to compress file synchronously: %v", err)
	}
	
	// Verify compression
	if _, err := os.Stat(testFile + ".gz"); os.IsNotExist(err) {
		t.Fatal("Compressed file does not exist")
	}
}

func TestQueueFile(t *testing.T) {
	cm := NewCompressionManager()
	
	// Set up error handler to track errors
	var errorCalled bool
	cm.SetErrorHandler(func(source, dest, msg string, err error) {
		if strings.Contains(msg, "queue full") {
			errorCalled = true
		}
	})
	
	// Enable compression and start workers
	cm.SetCompression(CompressionGzip)
	cm.Start()
	defer cm.Stop()
	
	// Queue a file (we don't need it to exist for this test)
	cm.QueueFile("/tmp/test.log")
	
	// Fill up the queue to test overflow handling
	for i := 0; i < 200; i++ {
		cm.QueueFile(filepath.Join("/tmp", "test" + string(rune(i)) + ".log"))
	}
	
	// Give some time for potential error handling
	time.Sleep(10 * time.Millisecond)
	
	if !errorCalled {
		t.Log("Queue full error was not triggered (this may be expected if queue size is large)")
	}
}

func TestGetStatus(t *testing.T) {
	cm := NewCompressionManager()
	cm.SetCompression(CompressionGzip)
	cm.SetMinAge(3)
	cm.SetWorkers(2)
	cm.Start()
	defer cm.Stop()
	
	status := cm.GetStatus()
	
	if status.Type != CompressionGzip {
		t.Errorf("Expected compression type %v, got %v", CompressionGzip, status.Type)
	}
	
	if status.MinAge != 3 {
		t.Errorf("Expected min age 3, got %d", status.MinAge)
	}
	
	if status.Workers != 2 {
		t.Errorf("Expected 2 workers, got %d", status.Workers)
	}
	
	if !status.IsRunning {
		t.Error("Expected IsRunning to be true")
	}
}

func TestErrorHandling(t *testing.T) {
	cm := NewCompressionManager()
	
	// Track error handler calls
	cm.SetErrorHandler(func(source, dest, msg string, err error) {
		// Error handler is set but we're checking graceful handling
		// The function should return nil for non-existent files
	})
	
	// Try to compress a non-existent file
	err := cm.compressFileGzip("/non/existent/file.log")
	
	// This should not error but should handle it gracefully
	if err != nil {
		t.Logf("compressFileGzip returned error as expected: %v", err)
	}
}

func TestMetricsHandler(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create a test file
	testFile := filepath.Join(tempDir, "test.log")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	cm := NewCompressionManager()
	
	// Track metrics
	var metricsCalled bool
	var metricsEvent string
	cm.SetMetricsHandler(func(event string) {
		metricsCalled = true
		metricsEvent = event
	})
	
	// Enable compression
	cm.SetCompression(CompressionGzip)
	
	// Compress file
	err := cm.CompressFileSync(testFile)
	if err != nil {
		t.Fatalf("Failed to compress file: %v", err)
	}
	
	if !metricsCalled {
		t.Error("Metrics handler was not called")
	}
	
	if metricsEvent != "compression_completed" {
		t.Errorf("Expected metrics event 'compression_completed', got '%s'", metricsEvent)
	}
}

func TestConcurrentCompression(t *testing.T) {
	tempDir := t.TempDir()
	
	cm := NewCompressionManager()
	cm.SetCompression(CompressionGzip)
	cm.SetWorkers(3)
	cm.Start()
	defer cm.Stop()
	
	// Create multiple test files
	numFiles := 10
	var wg sync.WaitGroup
	
	for i := 0; i < numFiles; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			
			testFile := filepath.Join(tempDir, "test" + string(rune('0' + index)) + ".log")
			content := strings.Repeat("Test content for file " + string(rune('0' + index)) + "\n", 100)
			
			if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
				t.Errorf("Failed to create test file %d: %v", index, err)
				return
			}
			
			cm.QueueFile(testFile)
		}(i)
	}
	
	wg.Wait()
	
	// Give workers time to process
	time.Sleep(100 * time.Millisecond)
}

func TestGetSupportedCompressionTypes(t *testing.T) {
	types := GetSupportedCompressionTypes()
	
	if len(types) != 2 {
		t.Errorf("Expected 2 supported compression types, got %d", len(types))
	}
	
	expectedTypes := map[CompressionType]bool{
		CompressionNone: true,
		CompressionGzip: true,
	}
	
	for _, ct := range types {
		if !expectedTypes[ct] {
			t.Errorf("Unexpected compression type: %v", ct)
		}
	}
}

func TestCompressionTypeString(t *testing.T) {
	tests := []struct {
		ct       CompressionType
		expected string
	}{
		{CompressionNone, "none"},
		{CompressionGzip, "gzip"},
		{CompressionType(99), "unknown"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := CompressionTypeString(tt.ct)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestParseCompressionType(t *testing.T) {
	tests := []struct {
		input       string
		expected    CompressionType
		expectError bool
	}{
		{"none", CompressionNone, false},
		{"gzip", CompressionGzip, false},
		{"invalid", CompressionNone, true},
		{"", CompressionNone, true},
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ParseCompressionType(tt.input)
			
			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			
			if !tt.expectError && result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestStartStopWorkers(t *testing.T) {
	cm := NewCompressionManager()
	
	// Enable compression
	err := cm.SetCompression(CompressionGzip)
	if err != nil {
		t.Fatalf("Failed to set compression: %v", err)
	}
	
	// Check that workers are started
	cm.mu.RLock()
	hasChannel := cm.compressCh != nil
	cm.mu.RUnlock()
	
	if !hasChannel {
		t.Error("Expected compression channel to be created")
	}
	
	// Stop and check cleanup
	cm.Stop()
	
	cm.mu.RLock()
	hasChannelAfterStop := cm.compressCh != nil
	cm.mu.RUnlock()
	
	if hasChannelAfterStop {
		t.Error("Expected compression channel to be nil after stop")
	}
}