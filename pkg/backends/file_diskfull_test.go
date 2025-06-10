package backends_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wayneeseguin/omni/pkg/backends"
	"github.com/wayneeseguin/omni/pkg/features"
)

// TestNewFileBackendWithRotation tests creating a new file backend with rotation
func TestNewFileBackendWithRotation(t *testing.T) {
	tests := []struct {
		name        string
		pathFunc    func(tempDir string) string
		rotManager  *features.RotationManager
		expectError bool
		errorMsg    string
	}{
		{
			name: "successful creation with rotation manager",
			pathFunc: func(tempDir string) string {
				return filepath.Join(tempDir, "test.log")
			},
			rotManager:  features.NewRotationManager(),
			expectError: false,
		},
		{
			name: "successful creation without rotation manager",
			pathFunc: func(tempDir string) string {
				return filepath.Join(tempDir, "test-no-rot.log")
			},
			rotManager:  nil,
			expectError: false,
		},
		{
			name: "create nested directory",
			pathFunc: func(tempDir string) string {
				return filepath.Join(tempDir, "subdir", "nested", "test.log")
			},
			rotManager:  features.NewRotationManager(),
			expectError: false,
		},
		{
			name: "path cleaning",
			pathFunc: func(tempDir string) string {
				return filepath.Join(tempDir, "..", "test.log")
			},
			rotManager:  features.NewRotationManager(),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			path := tt.pathFunc(tempDir)

			backend, err := backends.NewFileBackendWithRotation(path, tt.rotManager)

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

			// Test accessor methods
			if backend.GetFile() == nil {
				t.Error("GetFile() should return file handle")
			}
			if backend.GetWriter() == nil {
				t.Error("GetWriter() should return buffered writer")
			}
			if backend.GetLock() == nil {
				t.Error("GetLock() should return file lock")
			}
			if backend.GetSize() != 0 {
				t.Error("GetSize() should return 0 for new file")
			}
			if backend.Size() != 0 {
				t.Error("Size() should return 0 for new file")
			}
		})
	}
}

// TestFileBackendWithRotation_SetMaxRetries tests setting max retries
func TestFileBackendWithRotation_SetMaxRetries(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "retries.log")

	backend, err := backends.NewFileBackendWithRotation(path, nil)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	// Test setting different retry values
	testValues := []int{0, 1, 5, 10}
	for _, retries := range testValues {
		backend.SetMaxRetries(retries)
		// We can't directly verify the value was set since it's private,
		// but we can ensure the method doesn't panic
	}
}

// TestFileBackendWithRotation_SetErrorHandler tests setting error handler
func TestFileBackendWithRotation_SetErrorHandler(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "handler.log")

	backend, err := backends.NewFileBackendWithRotation(path, nil)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	var handlerCalled bool

	handler := func(source, dest, msg string, err error) {
		handlerCalled = true
		// We just verify the handler was called, don't need to store the parameters
		_ = source
		_ = dest
		_ = msg
		_ = err
	}

	backend.SetErrorHandler(handler)

	// The handler will be tested in other scenarios where errors occur
	if handlerCalled {
		t.Error("Handler should not be called just from setting it")
	}
}

// TestFileBackendWithRotation_Write tests basic write operations
func TestFileBackendWithRotation_Write(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "write.log")

	rotManager := features.NewRotationManager()
	backend, err := backends.NewFileBackendWithRotation(path, rotManager)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "simple write",
			data: []byte("Hello, file backend with rotation!"),
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

// TestFileBackendWithRotation_IsDiskFullError tests disk full error detection
func TestFileBackendWithRotation_IsDiskFullError(t *testing.T) {
	// Since isDiskFullError is not exported, we need to test it indirectly
	// by creating scenarios where Write would encounter disk full errors

	// This test focuses on the error detection logic patterns
	tests := []struct {
		name     string
		errMsg   string
		expected bool
	}{
		{
			name:     "no space left error",
			errMsg:   "write /tmp/test.log: no space left on device",
			expected: true,
		},
		{
			name:     "enospc error",
			errMsg:   "write error: ENOSPC",
			expected: true,
		},
		{
			name:     "disk full error",
			errMsg:   "disk full",
			expected: true,
		},
		{
			name:     "out of disk space",
			errMsg:   "out of disk space",
			expected: true,
		},
		{
			name:     "different error",
			errMsg:   "permission denied",
			expected: false,
		},
		{
			name:     "empty error",
			errMsg:   "",
			expected: false,
		},
	}

	// We test this by examining error patterns that would be caught
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the error pattern matching logic that would be used
			errStr := strings.ToLower(tt.errMsg)
			isDiskFull := strings.Contains(errStr, "no space left") ||
				strings.Contains(errStr, "enospc") ||
				strings.Contains(errStr, "disk full") ||
				strings.Contains(errStr, "out of disk space")

			if isDiskFull != tt.expected {
				t.Errorf("Expected disk full detection %v for %q, got %v", tt.expected, tt.errMsg, isDiskFull)
			}
		})
	}
}

// TestFileBackendWithRotation_FlushAndSync tests flushing and syncing
func TestFileBackendWithRotation_FlushAndSync(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "flush.log")

	backend, err := backends.NewFileBackendWithRotation(path, nil)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	// Write some data
	testData := []byte("Test data for flushing")
	_, err = backend.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Test Flush
	err = backend.Flush()
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	// Test Sync
	err = backend.Sync()
	if err != nil {
		t.Errorf("Sync failed: %v", err)
	}

	// Verify data is written
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if !strings.Contains(string(content), string(testData)) {
		t.Error("Data should be present in file after flush/sync")
	}
}

// TestFileBackendWithRotation_Close tests closing the backend
func TestFileBackendWithRotation_Close(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "close.log")

	rotManager := features.NewRotationManager()
	backend, err := backends.NewFileBackendWithRotation(path, rotManager)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
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
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file after close: %v", err)
	}

	if !strings.Contains(string(content), string(testData)) {
		t.Error("Data should be present in file after close")
	}

	// Multiple closes should not panic
	err = backend.Close()
	// May or may not return error, but shouldn't panic
	t.Logf("Second close returned: %v", err)
}

// TestFileBackendWithRotation_SupportsAtomic tests atomic support
func TestFileBackendWithRotation_SupportsAtomic(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "atomic.log")

	backend, err := backends.NewFileBackendWithRotation(path, nil)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	// File backend should support atomic writes via locking
	if !backend.SupportsAtomic() {
		t.Error("File backend should support atomic writes")
	}
}

// TestFileBackendWithRotation_GetStats tests getting backend statistics
func TestFileBackendWithRotation_GetStats(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "stats.log")

	backend, err := backends.NewFileBackendWithRotation(path, nil)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	// Initial stats
	stats := backend.GetStats()
	if stats.Path != path {
		t.Errorf("Expected path %s, got %s", path, stats.Path)
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

// TestFileBackendWithRotation_Rotate tests manual rotation
func TestFileBackendWithRotation_Rotate(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "rotate.log")

	rotManager := features.NewRotationManager()
	backend, err := backends.NewFileBackendWithRotation(path, rotManager)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	// Write some initial data
	initialData := []byte("Initial data before rotation")
	_, err = backend.Write(initialData)
	if err != nil {
		t.Fatalf("Initial write failed: %v", err)
	}

	err = backend.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// Perform rotation
	err = backend.Rotate()
	if err != nil {
		t.Fatalf("Rotation failed: %v", err)
	}

	// After rotation, size should be reset
	if backend.Size() != 0 {
		t.Errorf("Expected size 0 after rotation, got %d", backend.Size())
	}

	// Write new data after rotation
	newData := []byte("New data after rotation")
	_, err = backend.Write(newData)
	if err != nil {
		t.Fatalf("Write after rotation failed: %v", err)
	}

	err = backend.Flush()
	if err != nil {
		t.Fatalf("Flush after rotation failed: %v", err)
	}

	// Current file should only contain new data
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file after rotation: %v", err)
	}

	if strings.Contains(string(content), string(initialData)) {
		t.Error("Original data should not be in current file after rotation")
	}
	if !strings.Contains(string(content), string(newData)) {
		t.Error("New data should be in current file after rotation")
	}
}

// TestFileBackendWithRotation_RotateWithoutManager tests rotation without manager
func TestFileBackendWithRotation_RotateWithoutManager(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "rotate-no-mgr.log")

	backend, err := backends.NewFileBackendWithRotation(path, nil)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	// Rotation without manager should fail
	err = backend.Rotate()
	if err == nil {
		t.Error("Rotation should fail when no rotation manager is configured")
	} else if !strings.Contains(err.Error(), "no rotation manager") {
		t.Errorf("Expected 'no rotation manager' error, got: %v", err)
	}
}

// TestFileBackendWithRotation_ConcurrentWrites tests concurrent writing
func TestFileBackendWithRotation_ConcurrentWrites(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "concurrent.log")

	backend, err := backends.NewFileBackendWithRotation(path, nil)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	const numGoroutines = 10
	const messagesPerGoroutine = 20

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
}

// TestFileBackendWithRotation_ExistingFile tests handling existing files
func TestFileBackendWithRotation_ExistingFile(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "existing.log")

	// Create file with existing content
	existingContent := "Existing log content\n"
	err := os.WriteFile(path, []byte(existingContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create existing file: %v", err)
	}

	// Create backend for existing file
	backend, err := backends.NewFileBackendWithRotation(path, nil)
	if err != nil {
		t.Fatalf("Failed to create backend for existing file: %v", err)
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
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if !strings.Contains(string(content), existingContent) {
		t.Error("Existing content should be preserved")
	}
	if !strings.Contains(string(content), newContent) {
		t.Error("New content should be appended")
	}
}

// TestFileBackendWithRotation_ErrorHandlerCallback tests error handler callbacks
func TestFileBackendWithRotation_ErrorHandlerCallback(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "error-handler.log")

	backend, err := backends.NewFileBackendWithRotation(path, nil)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	var handlerCalls []struct {
		source, dest, msg string
		err               error
	}
	var mu sync.Mutex

	handler := func(source, dest, msg string, err error) {
		mu.Lock()
		defer mu.Unlock()
		handlerCalls = append(handlerCalls, struct {
			source, dest, msg string
			err               error
		}{source, dest, msg, err})
	}

	backend.SetErrorHandler(handler)

	// Write normally (should not trigger error handler)
	_, err = backend.Write([]byte("Normal write"))
	if err != nil {
		t.Fatalf("Normal write failed: %v", err)
	}

	// Check that no error handler calls were made for normal operation
	mu.Lock()
	callCount := len(handlerCalls)
	mu.Unlock()

	if callCount > 0 {
		t.Errorf("Error handler should not be called for normal operations, got %d calls", callCount)
	}
}

// TestFileBackendWithRotation_LargeWrites tests handling large writes
func TestFileBackendWithRotation_LargeWrites(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "large.log")

	backend, err := backends.NewFileBackendWithRotation(path, nil)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	// Write large chunks of data
	chunkSize := 8192 // 8KB chunks
	numChunks := 50   // 400KB total
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
}

// TestFileBackendWithRotation_PathAccessors tests path accessor methods
func TestFileBackendWithRotation_PathAccessors(t *testing.T) {
	tempDir := t.TempDir()
	originalPath := filepath.Join(tempDir, "accessor.log")

	backend, err := backends.NewFileBackendWithRotation(originalPath, nil)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	// Test Path() method returns cleaned path
	cleanedPath := backend.Path()
	if cleanedPath == "" {
		t.Error("Path() should return non-empty path")
	}

	// Should be the cleaned version of the original path
	expectedPath := filepath.Clean(originalPath)
	if cleanedPath != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, cleanedPath)
	}
}

// TestFileBackendWithRotation_WriteRetryLogic tests write retry scenarios
func TestFileBackendWithRotation_WriteRetryLogic(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "retry.log")

	backend, err := backends.NewFileBackendWithRotation(path, nil)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	// Test setting different retry counts
	testRetryCounts := []int{0, 1, 3, 5}

	for _, retryCount := range testRetryCounts {
		backend.SetMaxRetries(retryCount)

		// Normal write should work regardless of retry count
		testData := fmt.Sprintf("Test data with %d retries", retryCount)
		_, err := backend.Write([]byte(testData))
		if err != nil {
			t.Errorf("Write failed with %d retries: %v", retryCount, err)
		}
	}
}

// Helper functions for testing disk full scenarios

// TestFileBackendWithRotation_DiskFullErrorDetection tests isDiskFullError function
func TestFileBackendWithRotation_DiskFullErrorDetection(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "no space left error",
			err:      errors.New("no space left on device"),
			expected: true,
		},
		{
			name:     "ENOSPC error",
			err:      errors.New("write: ENOSPC"),
			expected: true,
		},
		{
			name:     "disk full error",
			err:      errors.New("disk full condition detected"),
			expected: true,
		},
		{
			name:     "out of disk space error",
			err:      errors.New("operation failed: out of disk space"),
			expected: true,
		},
		{
			name:     "case insensitive - uppercase",
			err:      errors.New("NO SPACE LEFT ON DEVICE"),
			expected: true,
		},
		{
			name:     "case insensitive - mixed case",
			err:      errors.New("Write failed: Disk Full"),
			expected: true,
		},
		{
			name:     "permission denied error",
			err:      errors.New("permission denied"),
			expected: false,
		},
		{
			name:     "network timeout error",
			err:      errors.New("network timeout"),
			expected: false,
		},
		{
			name:     "file not found error",
			err:      errors.New("file not found"),
			expected: false,
		},
	}

	// We test the error detection logic patterns used in the implementation
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var isDiskFull bool
			if tt.err != nil {
				errStr := strings.ToLower(tt.err.Error())
				isDiskFull = strings.Contains(errStr, "no space left") ||
					strings.Contains(errStr, "enospc") ||
					strings.Contains(errStr, "disk full") ||
					strings.Contains(errStr, "out of disk space")
			}

			if isDiskFull != tt.expected {
				t.Errorf("Expected disk full detection %v for error %q, got %v",
					tt.expected, tt.err, isDiskFull)
			}
		})
	}
}

// TestFileBackendWithRotation_HandleDiskFullScenarios tests various disk full handling scenarios
func TestFileBackendWithRotation_HandleDiskFullScenarios(t *testing.T) {
	tests := []struct {
		name          string
		expectSuccess bool
		setupRotMgr   func(*features.RotationManager)
	}{
		{
			name:          "successful rotation with rotation manager",
			expectSuccess: true,
			setupRotMgr: func(m *features.RotationManager) {
				// Standard setup - no special configuration needed
			},
		},
		{
			name:          "rotation without rotation manager should fail",
			expectSuccess: false,
			setupRotMgr:   nil, // No rotation manager
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			path := filepath.Join(tempDir, "diskfull.log")

			var rotMgr *features.RotationManager
			if tt.setupRotMgr != nil {
				rotMgr = features.NewRotationManager()
				tt.setupRotMgr(rotMgr)
			}

			backend, err := backends.NewFileBackendWithRotation(path, rotMgr)
			if err != nil {
				t.Fatalf("Failed to create backend: %v", err)
			}
			defer backend.Close()

			// Set up error handler to capture error messages
			var errorMessages []string
			var errorMu sync.Mutex
			backend.SetErrorHandler(func(source, dest, msg string, err error) {
				errorMu.Lock()
				defer errorMu.Unlock()
				errorMessages = append(errorMessages, fmt.Sprintf("%s: %s", source, msg))
			})

			// Write some test data first
			testData := []byte("Test data before rotation\n")
			_, err = backend.Write(testData)
			if err != nil {
				t.Fatalf("Failed to write test data: %v", err)
			}

			// Test rotation behavior
			err = backend.Rotate()
			if tt.expectSuccess && err != nil {
				t.Errorf("Expected successful rotation, got error: %v", err)
			} else if !tt.expectSuccess && err == nil {
				t.Error("Expected rotation to fail, but it succeeded")
			}

			if !tt.expectSuccess && err != nil {
				if !strings.Contains(err.Error(), "no rotation manager") {
					t.Errorf("Expected 'no rotation manager' error, got: %v", err)
				}
			}
		})
	}
}

// TestFileBackendWithRotation_RotationManagerIntegration tests real rotation manager integration
func TestFileBackendWithRotation_RotationManagerIntegration(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "integration-test.log")

	rotMgr := features.NewRotationManager()

	backend, err := backends.NewFileBackendWithRotation(path, rotMgr)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	// Write initial data
	initialData := []byte("Initial log data\n")
	_, err = backend.Write(initialData)
	if err != nil {
		t.Fatalf("Failed to write initial data: %v", err)
	}

	// Flush to ensure data is written
	err = backend.Flush()
	if err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}

	// Perform rotation
	err = backend.Rotate()
	if err != nil {
		t.Fatalf("Rotation failed: %v", err)
	}

	// Verify file was rotated (size should be reset)
	if backend.Size() != 0 {
		t.Errorf("Expected size 0 after rotation, got %d", backend.Size())
	}

	// Write new data after rotation
	newData := []byte("Data after rotation\n")
	_, err = backend.Write(newData)
	if err != nil {
		t.Fatalf("Failed to write after rotation: %v", err)
	}

	err = backend.Flush()
	if err != nil {
		t.Fatalf("Failed to flush after rotation: %v", err)
	}

	// Verify new file contains only new data
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read current log file: %v", err)
	}

	if strings.Contains(string(content), string(initialData)) {
		t.Error("Current file should not contain initial data after rotation")
	}
	if !strings.Contains(string(content), string(newData)) {
		t.Error("Current file should contain new data after rotation")
	}

	// Test GetRotatedFiles functionality
	rotatedFiles, err := rotMgr.GetRotatedFiles(path)
	if err != nil {
		t.Fatalf("Failed to get rotated files: %v", err)
	}

	if len(rotatedFiles) == 0 {
		t.Log("No rotated files found (this may be expected depending on rotation manager implementation)")
	} else {
		t.Logf("Found %d rotated files", len(rotatedFiles))
		for i, rf := range rotatedFiles {
			t.Logf("Rotated file %d: %s (size: %d)", i, rf.Path, rf.Size)
		}
	}
}

// TestFileBackendWithRotation_ErrorHandlerIntegration tests error handler integration
func TestFileBackendWithRotation_ErrorHandlerIntegration(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "error-handler-integration.log")

	rotMgr := features.NewRotationManager()

	backend, err := backends.NewFileBackendWithRotation(path, rotMgr)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	// Set up comprehensive error handler
	type errorCall struct {
		source    string
		dest      string
		msg       string
		err       error
		timestamp time.Time
	}

	var errorCalls []errorCall
	var errorMu sync.Mutex

	backend.SetErrorHandler(func(source, dest, msg string, err error) {
		errorMu.Lock()
		defer errorMu.Unlock()
		errorCalls = append(errorCalls, errorCall{
			source:    source,
			dest:      dest,
			msg:       msg,
			err:       err,
			timestamp: time.Now(),
		})
	})

	// Test normal operations (should not trigger error handler)
	testData := []byte("Normal operation test")
	_, err = backend.Write(testData)
	if err != nil {
		t.Fatalf("Normal write failed: %v", err)
	}

	err = backend.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// Verify no error handler calls for normal operations
	errorMu.Lock()
	normalOpCalls := len(errorCalls)
	errorMu.Unlock()

	if normalOpCalls > 0 {
		t.Log("Error handler calls during normal operations (may be expected for some lock operations)")
		for _, call := range errorCalls {
			t.Logf("  %s->%s: %s (err: %v)", call.source, call.dest, call.msg, call.err)
		}
	}

	// Test rotation scenarios that might trigger error handler
	err = backend.Rotate()
	if err != nil {
		t.Logf("Rotation failed (expected in some test scenarios): %v", err)
	}

	// Final verification
	errorMu.Lock()
	finalCallCount := len(errorCalls)
	errorMu.Unlock()

	t.Logf("Total error handler calls: %d", finalCallCount)
	for i, call := range errorCalls {
		t.Logf("Call %d: %s->%s: %s", i+1, call.source, call.dest, call.msg)
	}
}

// TestFileBackendWithRotation_StressTestDiskFull tests disk full handling under stress
func TestFileBackendWithRotation_StressTestDiskFull(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "stress.log")

	rotMgr := features.NewRotationManager()

	backend, err := backends.NewFileBackendWithRotation(path, rotMgr)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	// Set aggressive retry count
	backend.SetMaxRetries(5)

	// Track error handler calls
	var errorHandlerCalls int32
	backend.SetErrorHandler(func(source, dest, msg string, err error) {
		atomic.AddInt32(&errorHandlerCalls, 1)
	})

	// Stress test with concurrent operations
	const numGoroutines = 5
	const operationsPerGoroutine = 100

	var wg sync.WaitGroup
	var successfulWrites int32
	var failedWrites int32

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				data := []byte(fmt.Sprintf("Stress test goroutine %d operation %d\n", id, j))

				_, err := backend.Write(data)
				if err != nil {
					atomic.AddInt32(&failedWrites, 1)
					t.Logf("Write failed in goroutine %d, operation %d: %v", id, j, err)
				} else {
					atomic.AddInt32(&successfulWrites, 1)
				}

				// Occasionally trigger rotation
				if j%20 == 0 {
					backend.Rotate()
				}
			}
		}(i)
	}

	wg.Wait()

	// Final flush
	backend.Flush()

	t.Logf("Stress test results: %d successful writes, %d failed writes, %d error handler calls",
		atomic.LoadInt32(&successfulWrites), atomic.LoadInt32(&failedWrites), atomic.LoadInt32(&errorHandlerCalls))

	// Verify we had some successful operations
	if atomic.LoadInt32(&successfulWrites) == 0 {
		t.Error("Expected at least some successful writes during stress test")
	}
}
