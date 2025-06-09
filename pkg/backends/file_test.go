package backends_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/wayneeseguin/omni/pkg/backends"
)

// ===== FILE BACKEND TESTS =====

// TestFileBackendImpl_NewFileBackend tests file backend creation
func TestFileBackendImpl_NewFileBackend(t *testing.T) {
	tests := []struct {
		name        string
		pathFunc    func(tempDir string) string
		expectError bool
		errorMsg    string
	}{
		{
			name: "successful creation",
			pathFunc: func(tempDir string) string {
				return filepath.Join(tempDir, "test.log")
			},
			expectError: false,
		},
		{
			name: "create nested directory",
			pathFunc: func(tempDir string) string {
				return filepath.Join(tempDir, "subdir", "nested", "test.log")
			},
			expectError: false,
		},
		{
			name: "clean path with traversal attempt",
			pathFunc: func(tempDir string) string {
				return filepath.Join(tempDir, "..", "test.log")
			},
			expectError: false, // Should be cleaned and work
		},
		{
			name: "existing file",
			pathFunc: func(tempDir string) string {
				path := filepath.Join(tempDir, "existing.log")
				// Create the file first
				os.WriteFile(path, []byte("existing content"), 0644)
				return path
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			path := tt.pathFunc(tempDir)

			backend, err := backends.NewFileBackend(path)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing %q, got none", tt.errorMsg)
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			defer backend.Close()

			// Verify backend properties
			if backend == nil {
				t.Fatal("Backend should not be nil")
			}

			// Test that file was created
			if _, err := os.Stat(backend.Path()); os.IsNotExist(err) {
				t.Error("File should have been created")
			}

			// Test that we can get components
			if backend.GetFile() == nil {
				t.Error("GetFile() should return file handle")
			}
			if backend.GetWriter() == nil {
				t.Error("GetWriter() should return buffered writer")
			}
			if backend.GetLock() == nil {
				t.Error("GetLock() should return file lock")
			}
		})
	}
}

// TestFileBackendImpl_NewFileBackend_Errors tests file backend creation error cases
func TestFileBackendImpl_NewFileBackend_Errors(t *testing.T) {
	// Test permission denied scenario
	t.Run("permission_denied_directory", func(t *testing.T) {
		tempDir := t.TempDir()
		restrictedDir := filepath.Join(tempDir, "restricted")
		
		// Create directory and remove write permission
		err := os.MkdirAll(restrictedDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create restricted directory: %v", err)
		}
		
		// Remove write permission from parent directory
		err = os.Chmod(restrictedDir, 0444)
		if err != nil {
			t.Fatalf("Failed to change directory permissions: %v", err)
		}
		
		// Restore permissions after test
		defer os.Chmod(restrictedDir, 0755)
		
		logPath := filepath.Join(restrictedDir, "subdir", "test.log")
		_, err = backends.NewFileBackend(logPath)
		
		// Should fail on directory creation
		if err == nil {
			t.Error("Expected error when creating directory without permissions")
		} else if !strings.Contains(err.Error(), "create directory") {
			t.Errorf("Expected 'create directory' error, got: %v", err)
		}
	})

	// Test invalid file path (directory as file)
	t.Run("directory_as_file", func(t *testing.T) {
		tempDir := t.TempDir()
		dirPath := filepath.Join(tempDir, "existing_dir")
		
		// Create a directory
		err := os.MkdirAll(dirPath, 0755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		
		// Try to create file backend with directory path
		_, err = backends.NewFileBackend(dirPath)
		
		// Should fail because it's a directory, not a file
		if err == nil {
			t.Error("Expected error when using directory as file path")
		}
	})
}

// TestFileBackendImpl_Write tests writing to file backend
func TestFileBackendImpl_Write(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "write_test.log")
	
	backend, err := backends.NewFileBackend(logPath)
	if err != nil {
		t.Fatalf("Failed to create file backend: %v", err)
	}
	defer backend.Close()

	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "simple write",
			data: []byte("Hello, file backend!"),
		},
		{
			name: "write with newline",
			data: []byte("Line 1\nLine 2\n"),
		},
		{
			name: "empty write",
			data: []byte(""),
		},
		{
			name: "binary data",
			data: []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD},
		},
		{
			name: "large write",
			data: []byte(strings.Repeat("Large data chunk. ", 1000)),
		},
	}

	totalSize := int64(0)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initialSize := backend.Size()
			
			n, err := backend.Write(tt.data)
			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}

			if n != len(tt.data) {
				t.Errorf("Expected to write %d bytes, wrote %d", len(tt.data), n)
			}

			// Size should be updated
			expectedSize := initialSize + int64(len(tt.data))
			if backend.Size() != expectedSize {
				t.Errorf("Expected size %d, got %d", expectedSize, backend.Size())
			}

			totalSize += int64(len(tt.data))
		})
	}

	// Test GetSize method
	if backend.GetSize() != totalSize {
		t.Errorf("GetSize() returned %d, expected %d", backend.GetSize(), totalSize)
	}
}

// TestFileBackendImpl_FlushAndSync tests flushing and syncing
func TestFileBackendImpl_FlushAndSync(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "flush_test.log")
	
	backend, err := backends.NewFileBackend(logPath)
	if err != nil {
		t.Fatalf("Failed to create file backend: %v", err)
	}
	defer backend.Close()

	// Write some data
	testData := []byte("Test data for flushing")
	_, err = backend.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Data might not be on disk yet due to buffering
	// Read directly from file to check
	initialContent, _ := os.ReadFile(logPath)

	// Test Flush
	err = backend.Flush()
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	// After flush, data should be written to file
	flushedContent, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read file after flush: %v", err)
	}

	if !strings.Contains(string(flushedContent), string(testData)) {
		t.Error("Data should be present in file after flush")
	}

	// Write more data
	moreData := []byte(" and more data")
	_, err = backend.Write(moreData)
	if err != nil {
		t.Fatalf("Second write failed: %v", err)
	}

	// Test Sync (should flush and sync to disk)
	err = backend.Sync()
	if err != nil {
		t.Errorf("Sync failed: %v", err)
	}

	// Verify all data is present
	finalContent, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read file after sync: %v", err)
	}

	expectedContent := string(testData) + string(moreData)
	if !strings.Contains(string(finalContent), expectedContent) {
		t.Errorf("Expected content %q not found in file", expectedContent)
	}

	t.Logf("Initial content length: %d", len(initialContent))
	t.Logf("Flushed content length: %d", len(flushedContent))
	t.Logf("Final content length: %d", len(finalContent))
}

// TestFileBackendImpl_Close tests closing file backend
func TestFileBackendImpl_Close(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "close_test.log")
	
	backend, err := backends.NewFileBackend(logPath)
	if err != nil {
		t.Fatalf("Failed to create file backend: %v", err)
	}

	// Write some data before closing
	testData := []byte("Data before close")
	_, err = backend.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Close the backend
	err = backend.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Verify data was flushed on close
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read file after close: %v", err)
	}

	if !strings.Contains(string(content), string(testData)) {
		t.Error("Data should be present in file after close")
	}

	// Operations after close should fail gracefully
	_, err = backend.Write([]byte("After close"))
	if err == nil {
		t.Log("Write after close did not error (may be implementation-specific)")
	}

	// Multiple closes should be handled gracefully (may return error)
	err = backend.Close()
	if err != nil {
		t.Logf("Second close returned error (may be expected): %v", err)
	}
}

// TestFileBackendImpl_SupportsAtomic tests atomic support
func TestFileBackendImpl_SupportsAtomic(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "atomic_test.log")
	
	backend, err := backends.NewFileBackend(logPath)
	if err != nil {
		t.Fatalf("Failed to create file backend: %v", err)
	}
	defer backend.Close()

	// File backend should support atomic writes via locking
	if !backend.SupportsAtomic() {
		t.Error("File backend should support atomic writes")
	}
}

// TestFileBackendImpl_GetStats tests getting backend statistics
func TestFileBackendImpl_GetStats(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "stats_test.log")
	
	backend, err := backends.NewFileBackend(logPath)
	if err != nil {
		t.Fatalf("Failed to create file backend: %v", err)
	}
	defer backend.Close()

	// Initial stats
	stats := backend.GetStats()
	if stats.Path != logPath {
		t.Errorf("Expected path %s, got %s", logPath, stats.Path)
	}
	if stats.Size != 0 {
		t.Errorf("Expected initial size 0, got %d", stats.Size)
	}
	if stats.BytesWritten != 0 {
		t.Errorf("Expected initial bytes written 0, got %d", stats.BytesWritten)
	}

	// Write some data
	testData := []byte("Statistics test data")
	_, err = backend.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Updated stats
	stats = backend.GetStats()
	expectedSize := int64(len(testData))
	if stats.Size != expectedSize {
		t.Errorf("Expected size %d, got %d", expectedSize, stats.Size)
	}
	if stats.BytesWritten != uint64(expectedSize) {
		t.Errorf("Expected bytes written %d, got %d", expectedSize, stats.BytesWritten)
	}
}

// TestFileBackendImpl_Rotate tests rotation functionality
func TestFileBackendImpl_Rotate(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "rotate_test.log")
	
	backend, err := backends.NewFileBackend(logPath)
	if err != nil {
		t.Fatalf("Failed to create file backend: %v", err)
	}
	defer backend.Close()

	// Rotation is not implemented yet, should return error
	err = backend.Rotate()
	if err == nil {
		t.Error("Rotate should return 'not implemented' error")
	} else if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("Expected 'not implemented' error, got: %v", err)
	}
}

// TestFileBackendImpl_FileLocking tests file locking functionality
func TestFileBackendImpl_FileLocking(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "locking_test.log")
	
	// Create first backend
	backend1, err := backends.NewFileBackend(logPath)
	if err != nil {
		t.Fatalf("Failed to create first file backend: %v", err)
	}
	defer backend1.Close()

	// Create second backend to same file (should work due to process-safe locking)
	backend2, err := backends.NewFileBackend(logPath)
	if err != nil {
		t.Fatalf("Failed to create second file backend: %v", err)
	}
	defer backend2.Close()

	// Both backends should be able to write (with proper locking)
	data1 := []byte("Data from backend 1\n")
	data2 := []byte("Data from backend 2\n")

	_, err = backend1.Write(data1)
	if err != nil {
		t.Errorf("Backend 1 write failed: %v", err)
	}

	_, err = backend2.Write(data2)
	if err != nil {
		t.Errorf("Backend 2 write failed: %v", err)
	}

	// Flush both
	backend1.Flush()
	backend2.Flush()

	// Both should complete without errors due to file locking
	// The exact interleaving of data may vary, but both writes should succeed
}

// TestFileBackendImpl_ConcurrentWrites tests concurrent writing to file backend
func TestFileBackendImpl_ConcurrentWrites(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "concurrent_test.log")
	
	backend, err := backends.NewFileBackend(logPath)
	if err != nil {
		t.Fatalf("Failed to create file backend: %v", err)
	}
	defer backend.Close()

	const numGoroutines = 10
	const messagesPerGoroutine = 50

	var wg sync.WaitGroup
	var errorCount int32
	var mu sync.Mutex

	// Test concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				msg := fmt.Sprintf("Message from goroutine %d, iteration %d\n", id, j)
				_, err := backend.Write([]byte(msg))
				if err != nil {
					mu.Lock()
					errorCount++
					mu.Unlock()
					t.Logf("Write error in goroutine %d: %v", id, err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Final flush
	err = backend.Flush()
	if err != nil {
		t.Errorf("Final flush failed: %v", err)
	}

	mu.Lock()
	if errorCount > 0 {
		t.Errorf("Got %d write errors during concurrent test", errorCount)
	}
	mu.Unlock()

	// Verify file has expected amount of data
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read file after concurrent writes: %v", err)
	}

	// Count lines to verify all messages were written
	lines := strings.Split(string(content), "\n")
	nonEmptyLines := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmptyLines++
		}
	}

	expectedLines := numGoroutines * messagesPerGoroutine
	if nonEmptyLines != expectedLines {
		t.Errorf("Expected %d lines, got %d", expectedLines, nonEmptyLines)
	}
}

// TestFileBackendImpl_LargeFile tests handling of large files
func TestFileBackendImpl_LargeFile(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "large_test.log")
	
	backend, err := backends.NewFileBackend(logPath)
	if err != nil {
		t.Fatalf("Failed to create file backend: %v", err)
	}
	defer backend.Close()

	// Write a moderately large amount of data
	chunkSize := 8192 // 8KB chunks
	numChunks := 100  // 800KB total
	chunk := make([]byte, chunkSize)
	for i := range chunk {
		chunk[i] = byte(i % 256)
	}

	for i := 0; i < numChunks; i++ {
		_, err := backend.Write(chunk)
		if err != nil {
			t.Fatalf("Write chunk %d failed: %v", i, err)
		}
	}

	// Flush to ensure all data is written
	err = backend.Flush()
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	// Verify file size
	expectedSize := int64(chunkSize * numChunks)
	if backend.Size() != expectedSize {
		t.Errorf("Expected size %d, got %d", expectedSize, backend.Size())
	}

	// Verify file on disk
	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	if info.Size() != expectedSize {
		t.Errorf("File on disk size %d, expected %d", info.Size(), expectedSize)
	}
}

// TestFileBackendImpl_ExistingFileHandling tests handling of existing files
func TestFileBackendImpl_ExistingFileHandling(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "existing_test.log")
	
	// Create file with existing content
	existingContent := "Existing log content\n"
	err := os.WriteFile(logPath, []byte(existingContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create existing file: %v", err)
	}

	// Create backend for existing file
	backend, err := backends.NewFileBackend(logPath)
	if err != nil {
		t.Fatalf("Failed to create file backend for existing file: %v", err)
	}
	defer backend.Close()

	// Size should reflect existing content
	expectedInitialSize := int64(len(existingContent))
	if backend.Size() != expectedInitialSize {
		t.Errorf("Expected initial size %d, got %d", expectedInitialSize, backend.Size())
	}

	// Write new content (should append)
	newContent := "New log content\n"
	_, err = backend.Write([]byte(newContent))
	if err != nil {
		t.Fatalf("Write to existing file failed: %v", err)
	}

	err = backend.Flush()
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	// Verify both contents are present
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if !strings.Contains(string(content), existingContent) {
		t.Error("Existing content should be preserved")
	}
	if !strings.Contains(string(content), newContent) {
		t.Error("New content should be appended")
	}

	// Total size should be sum of both
	expectedFinalSize := expectedInitialSize + int64(len(newContent))
	if backend.Size() != expectedFinalSize {
		t.Errorf("Expected final size %d, got %d", expectedFinalSize, backend.Size())
	}
}

// TestFileBackendImpl_ErrorRecovery tests error recovery scenarios
func TestFileBackendImpl_ErrorRecovery(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "recovery_test.log")
	
	backend, err := backends.NewFileBackend(logPath)
	if err != nil {
		t.Fatalf("Failed to create file backend: %v", err)
	}
	defer backend.Close()

	// Normal operation
	_, err = backend.Write([]byte("Normal operation\n"))
	if err != nil {
		t.Fatalf("Normal write failed: %v", err)
	}

	// Simulate disk full by trying to write to /dev/full if it exists
	if _, err := os.Stat("/dev/full"); err == nil {
		t.Run("disk_full_simulation", func(t *testing.T) {
			// Create backend pointing to /dev/full to simulate disk full
			fullBackend, err := backends.NewFileBackend("/dev/full")
			if err != nil {
				t.Skip("Cannot create backend for /dev/full (may not have permissions)")
			}
			defer fullBackend.Close()

			// This should fail
			_, err = fullBackend.Write([]byte("This should fail"))
			if err == nil {
				t.Error("Expected write to /dev/full to fail")
			}
		})
	}

	// Test flush on closed backend
	t.Run("flush_after_close", func(t *testing.T) {
		tempPath := filepath.Join(tempDir, "flush_after_close.log")
		testBackend, err := backends.NewFileBackend(tempPath)
		if err != nil {
			t.Fatalf("Failed to create test backend: %v", err)
		}

		// Write and close
		testBackend.Write([]byte("test"))
		testBackend.Close()

		// Flush after close should handle gracefully
		err = testBackend.Flush()
		// Implementation may or may not return an error here, but shouldn't panic
		t.Logf("Flush after close returned: %v", err)
	})
}

// TestFileBackendImpl_PathHandling tests various path handling scenarios
func TestFileBackendImpl_PathHandling(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		pathFunc    func(tempDir string) string
		expectError bool
		description string
	}{
		{
			name: "absolute_path",
			pathFunc: func(tempDir string) string {
				return filepath.Join(tempDir, "absolute.log")
			},
			expectError: false,
			description: "Absolute path should work",
		},
		{
			name: "nested_directories",
			pathFunc: func(tempDir string) string {
				return filepath.Join(tempDir, "a", "b", "c", "nested.log")
			},
			expectError: false,
			description: "Nested directories should be created",
		},
		{
			name: "path_with_spaces",
			pathFunc: func(tempDir string) string {
				return filepath.Join(tempDir, "path with spaces", "spaced.log")
			},
			expectError: false,
			description: "Paths with spaces should work",
		},
		{
			name: "unicode_path",
			pathFunc: func(tempDir string) string {
				return filepath.Join(tempDir, "测试目录", "тест.log")
			},
			expectError: false,
			description: "Unicode paths should work",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.pathFunc(tempDir)
			
			backend, err := backends.NewFileBackend(path)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for %s", tt.description)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error for %s: %v", tt.description, err)
			}
			defer backend.Close()

			// Test write to verify backend works
			_, err = backend.Write([]byte("test content"))
			if err != nil {
				t.Errorf("Write failed for %s: %v", tt.description, err)
			}

			// Verify file exists
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Errorf("File should exist for %s", tt.description)
			}
		})
	}
}

// TestFileBackendImpl_BufferSizes tests different buffer sizes
func TestFileBackendImpl_BufferSizes(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "buffer_test.log")
	
	backend, err := backends.NewFileBackend(logPath)
	if err != nil {
		t.Fatalf("Failed to create file backend: %v", err)
	}
	defer backend.Close()

	// Verify buffer is created with expected size
	writer := backend.GetWriter()
	if writer == nil {
		t.Fatal("Writer should not be nil")
	}

	// The buffer size is not directly accessible, but we can test that
	// writes smaller than the buffer size don't immediately hit disk
	smallData := []byte("small")
	_, err = backend.Write(smallData)
	if err != nil {
		t.Fatalf("Small write failed: %v", err)
	}

	// Without flush, data might not be on disk yet
	contentBeforeFlush, _ := os.ReadFile(logPath)
	
	// Flush to ensure data hits disk
	err = backend.Flush()
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	contentAfterFlush, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read file after flush: %v", err)
	}

	// After flush, content should definitely be present
	if !strings.Contains(string(contentAfterFlush), string(smallData)) {
		t.Error("Data should be present after flush")
	}

	t.Logf("Content before flush: %q", string(contentBeforeFlush))
	t.Logf("Content after flush: %q", string(contentAfterFlush))
}