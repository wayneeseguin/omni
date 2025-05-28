package flexlog

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSetCompression tests enabling and disabling compression
func TestSetCompression(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Initially compression should be off
	if logger.compression != int(CompressionNone) {
		t.Errorf("Expected compression to be none initially")
	}

	// Enable gzip compression
	logger.SetCompression(CompressionGzip)
	if logger.compression != int(CompressionGzip) {
		t.Errorf("Expected compression to be gzip")
	}

	// Should have started compression workers
	if logger.compressCh == nil {
		t.Errorf("Expected compression channel to be created")
	}

	// Disable compression
	logger.SetCompression(CompressionNone)
	if logger.compression != int(CompressionNone) {
		t.Errorf("Expected compression to be disabled")
	}
}

// TestSetCompressMinAge tests setting minimum rotation age for compression
func TestSetCompressMinAge(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set min age
	logger.SetCompressMinAge(3)

	// Check it was set
	logger.mu.Lock()
	age := logger.compressMinAge
	logger.mu.Unlock()

	if age != 3 {
		t.Errorf("Expected compressMinAge to be 3, got %d", age)
	}
}

// TestSetCompressWorkers tests setting number of compression workers
func TestSetCompressWorkers(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Enable compression first
	logger.SetCompression(CompressionGzip)

	// Set workers
	logger.SetCompressWorkers(5)

	// Check it was set
	logger.mu.Lock()
	workers := logger.compressWorkers
	logger.mu.Unlock()

	if workers != 5 {
		t.Errorf("Expected 5 compression workers, got %d", workers)
	}

	// Test minimum enforcement
	logger.SetCompressWorkers(0)

	logger.mu.Lock()
	workers = logger.compressWorkers
	logger.mu.Unlock()

	if workers != 1 {
		t.Errorf("Expected minimum 1 compression worker, got %d", workers)
	}
}

// TestCompressLogFile tests file compression through rotation
func TestCompressLogFile(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Enable compression and trigger rotation
	logger.SetCompression(CompressionGzip)
	logger.SetMaxSize(100)      // Small size
	logger.SetCompressMinAge(1) // Compress after 1 rotation

	// Write enough to trigger rotation
	testContent := "This is a test log file that should be compressed\n"
	for i := 0; i < 10; i++ {
		logger.Info(testContent)
	}

	// Sync and wait for compression
	logger.Sync()
	time.Sleep(500 * time.Millisecond)

	// Check for compressed files
	compressedFiles, err := filepath.Glob(filepath.Join(tempDir, "test.log.*.gz"))
	if err != nil {
		t.Fatalf("Failed to glob compressed files: %v", err)
	}

	if len(compressedFiles) == 0 {
		t.Errorf("Expected at least one compressed file")
		return
	}

	// Verify compressed content
	compressedFile, err := os.Open(compressedFiles[0])
	if err != nil {
		t.Fatalf("Failed to open compressed file: %v", err)
	}
	defer compressedFile.Close()

	gzReader, err := gzip.NewReader(compressedFile)
	if err != nil {
		t.Fatalf("Failed to create gzip reader: %v", err)
	}
	defer gzReader.Close()

	decompressed, err := io.ReadAll(gzReader)
	if err != nil {
		t.Fatalf("Failed to read compressed content: %v", err)
	}

	if !strings.Contains(string(decompressed), testContent) {
		t.Errorf("Decompressed content doesn't contain expected log message")
	}
}

// TestCompressionWorker tests the compression worker goroutine
func TestCompressionWorker(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Enable compression
	logger.SetCompression(CompressionGzip)
	logger.SetMaxSize(100) // Small size to trigger rotation

	// Write enough to trigger rotation
	for i := 0; i < 10; i++ {
		logger.Info("This is a test message that will trigger rotation")
	}

	// Sync and wait for compression
	logger.Sync()
	time.Sleep(500 * time.Millisecond)

	// Check for compressed files
	files, err := filepath.Glob(filepath.Join(tempDir, "test.log.*.gz"))
	if err != nil {
		t.Fatalf("Failed to glob files: %v", err)
	}

	if len(files) == 0 {
		t.Errorf("Expected at least one compressed file")
	}
}

// TestQueueForCompression tests the compression queue
func TestQueueForCompression(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Enable compression
	logger.SetCompression(CompressionGzip)

	// Create a test file
	testFile := filepath.Join(tempDir, "test.log.20240101-120000.000")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Queue it for compression
	logger.queueForCompression(testFile)

	// Wait for compression
	time.Sleep(200 * time.Millisecond)

	// Check compressed file exists
	compressedPath := testFile + ".gz"
	if _, err := os.Stat(compressedPath); os.IsNotExist(err) {
		t.Errorf("Compressed file does not exist")
	}
}

// TestCompressionWithRotation tests compression works with rotation
func TestCompressionWithRotation(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	// Don't defer close - we'll close it explicitly before checking files

	// Enable compression and small rotation size
	logger.SetCompression(CompressionGzip)
	logger.SetMaxSize(200)
	logger.SetCompressMinAge(1) // Compress immediately

	// Write multiple batches to trigger rotations
	for batch := 0; batch < 3; batch++ {
		for i := 0; i < 10; i++ {
			logger.Infof("Batch %d: Message %d", batch, i)
		}
		logger.Sync()
		time.Sleep(100 * time.Millisecond)
	}

	// Close the logger to ensure all compression completes
	logger.Close()

	// TODO: Close() should wait for compression workers to finish
	// For now, we need to wait manually to avoid race conditions
	// where we try to read files that are still being compressed
	time.Sleep(2 * time.Second)

	// Should have compressed files
	compressedFiles, err := filepath.Glob(filepath.Join(tempDir, "test.log.*.gz"))
	if err != nil {
		t.Fatalf("Failed to glob compressed files: %v", err)
	}

	if len(compressedFiles) == 0 {
		t.Errorf("Expected compressed files after rotation")
	}

	// Verify we can read the compressed files
	for _, file := range compressedFiles {
		compressedFile, err := os.Open(file)
		if err != nil {
			t.Errorf("Failed to open compressed file %s: %v", file, err)
			continue
		}

		gzReader, err := gzip.NewReader(compressedFile)
		if err != nil {
			compressedFile.Close()
			t.Errorf("Failed to create gzip reader for %s: %v", file, err)
			continue
		}

		content, err := io.ReadAll(gzReader)
		gzReader.Close()
		compressedFile.Close()

		if err != nil {
			t.Errorf("Failed to read compressed content from %s: %v", file, err)
			continue
		}

		// Should contain our log messages
		if !strings.Contains(string(content), "Batch") {
			t.Errorf("Compressed file %s doesn't contain expected content", file)
		}
	}
}

// TestCompressionChannelFull tests behavior when compression channel is full
func TestCompressionChannelFull(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Enable compression with small channel
	logger.SetCompression(CompressionGzip)
	logger.SetMaxSize(50) // Very small to trigger many rotations

	// Flood with messages to fill compression queue
	for i := 0; i < 50; i++ {
		logger.Infof("Message %d to trigger many rotations quickly", i)
	}

	// Should not panic or deadlock
	logger.Sync()
	time.Sleep(1 * time.Second)

	// Check we have some compressed files
	compressedFiles, err := filepath.Glob(filepath.Join(tempDir, "test.log.*.gz"))
	if err != nil {
		t.Fatalf("Failed to glob compressed files: %v", err)
	}

	// Should have at least some compressed files (not all may compress if queue was full)
	if len(compressedFiles) == 0 {
		t.Errorf("Expected at least some compressed files even with full queue")
	}
}
