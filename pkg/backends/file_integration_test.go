// +build integration

package backends_test

import (
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

func TestFileBackendDiskFull(t *testing.T) {
	testPath := os.Getenv("OMNI_DISKFULL_TEST_PATH")
	if testPath == "" {
		t.Skip("OMNI_DISKFULL_TEST_PATH not set, skipping disk full test")
	}

	t.Run("basic_disk_full", testBasicDiskFull)
	t.Run("partial_write_handling", testPartialWriteHandling)
	t.Run("recovery_after_space_freed", testRecoveryAfterSpaceFreed)
	t.Run("concurrent_writers_disk_full", testConcurrentWritersDiskFull)
	t.Run("large_buffer_disk_full", testLargeBufferDiskFull)
}

func TestFileBackendDiskFullWithRotation(t *testing.T) {
	testPath := os.Getenv("OMNI_DISKFULL_TEST_PATH")
	if testPath == "" {
		t.Skip("OMNI_DISKFULL_TEST_PATH not set, skipping disk full rotation test")
	}

	t.Run("disk_full_with_rotation", testDiskFullWithRotation)
	t.Run("disk_full_rotation_race_condition", testDiskFullRotationRaceCondition)
}

func testBasicDiskFull(t *testing.T) {
	testPath := os.Getenv("OMNI_DISKFULL_TEST_PATH")
	logPath := filepath.Join(testPath, "basic_disk_full.log")
	
	// Clean up any existing files
	os.RemoveAll(logPath)
	
	backend, err := backends.NewFileBackend(logPath)
	if err != nil {
		t.Fatalf("Failed to create file backend: %v", err)
	}
	defer backend.Close()
	
	// Write data until disk is full
	chunk := make([]byte, 1024) // 1KB chunks
	for i := range chunk {
		chunk[i] = byte('A' + (i % 26))
	}
	
	var writeErr error
	totalWritten := 0
	
	for i := 0; i < 2048; i++ { // Try to write 2MB (should fail before that)
		n, err := backend.Write(chunk)
		totalWritten += n
		
		if err != nil {
			writeErr = err
			t.Logf("Write failed after %d bytes: %v", totalWritten, err)
			break
		}
		
		// Flush periodically to ensure data hits disk
		if i%10 == 0 {
			backend.Flush()
		}
	}
	
	// We should have gotten an error
	if writeErr == nil {
		t.Error("Expected write to fail when disk is full")
	}
	
	// Verify error contains expected message
	errStr := writeErr.Error()
	if !strings.Contains(errStr, "no space left") && 
	   !strings.Contains(errStr, "ENOSPC") && 
	   !strings.Contains(errStr, "disk full") {
		t.Errorf("Unexpected error message: %v", writeErr)
	}
	
	// Backend should still be functional for reads
	stats := backend.GetStats()
	t.Logf("Backend stats: Path=%s, Size=%d, BytesWritten=%d", 
		stats.Path, stats.Size, stats.BytesWritten)
}

func testPartialWriteHandling(t *testing.T) {
	testPath := os.Getenv("OMNI_DISKFULL_TEST_PATH")
	logPath := filepath.Join(testPath, "partial_write.log")
	
	// Clean up
	os.RemoveAll(logPath)
	
	backend, err := backends.NewFileBackend(logPath)
	if err != nil {
		t.Fatalf("Failed to create file backend: %v", err)
	}
	defer backend.Close()
	
	// Fill most of the disk
	smallChunk := make([]byte, 512) // 512 bytes
	for i := range smallChunk {
		smallChunk[i] = 'X'
	}
	
	// Write until we have ~100KB left
	bytesWritten := 0
	targetBytes := 900 * 1024 // Leave ~100KB free in 1MB filesystem
	
	for bytesWritten < targetBytes {
		n, err := backend.Write(smallChunk)
		if err != nil {
			t.Logf("Stopped filling at %d bytes: %v", bytesWritten, err)
			break
		}
		bytesWritten += n
	}
	
	backend.Flush()
	
	// Now try to write a large chunk that won't fit completely
	largeChunk := make([]byte, 200*1024) // 200KB
	for i := range largeChunk {
		largeChunk[i] = 'L'
	}
	
	n, err := backend.Write(largeChunk)
	t.Logf("Large write: requested=%d, written=%d, error=%v", 
		len(largeChunk), n, err)
	
	// Should have written some bytes but not all
	if n == 0 && err != nil {
		t.Log("No partial write - filesystem may not support it")
	} else if n > 0 && n < len(largeChunk) {
		t.Logf("Partial write successful: %d of %d bytes", n, len(largeChunk))
	}
}

func testRecoveryAfterSpaceFreed(t *testing.T) {
	testPath := os.Getenv("OMNI_DISKFULL_TEST_PATH")
	
	// Create multiple files to fill disk
	files := make([]*backends.FileBackendImpl, 3)
	filePaths := make([]string, 3)
	
	// Fill disk with multiple files
	for i := 0; i < 3; i++ {
		filePaths[i] = filepath.Join(testPath, fmt.Sprintf("recovery_%d.log", i))
		os.RemoveAll(filePaths[i])
		
		backend, err := backends.NewFileBackend(filePaths[i])
		if err != nil {
			t.Fatalf("Failed to create backend %d: %v", i, err)
		}
		files[i] = backend
		
		// Write data
		data := make([]byte, 300*1024) // 300KB each
		for j := range data {
			data[j] = byte('0' + i)
		}
		
		_, err = backend.Write(data)
		backend.Flush()
		
		if err != nil {
			t.Logf("File %d write stopped: %v", i, err)
			break
		}
	}
	
	// Close first file and remove it to free space
	files[0].Close()
	err := os.Remove(filePaths[0])
	if err != nil {
		t.Fatalf("Failed to remove file: %v", err)
	}
	
	// Try to write to remaining file
	testData := []byte("Recovery test - space should be available now\n")
	n, err := files[1].Write(testData)
	
	if err != nil {
		t.Errorf("Write after freeing space failed: %v", err)
	} else if n != len(testData) {
		t.Errorf("Partial write after freeing space: %d of %d bytes", n, len(testData))
	} else {
		t.Log("Successfully wrote after freeing space")
	}
	
	// Cleanup
	for i := 1; i < 3; i++ {
		if files[i] != nil {
			files[i].Close()
		}
	}
}

func testConcurrentWritersDiskFull(t *testing.T) {
	testPath := os.Getenv("OMNI_DISKFULL_TEST_PATH")
	logPath := filepath.Join(testPath, "concurrent.log")
	
	os.RemoveAll(logPath)
	
	backend, err := backends.NewFileBackend(logPath)
	if err != nil {
		t.Fatalf("Failed to create file backend: %v", err)
	}
	defer backend.Close()
	
	var wg sync.WaitGroup
	errors := make(chan error, 10)
	
	// Start 5 concurrent writers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			data := make([]byte, 100*1024) // 100KB each
			for j := range data {
				data[j] = byte('A' + id)
			}
			
			// Keep writing until error
			for j := 0; j < 5; j++ {
				_, err := backend.Write(data)
				if err != nil {
					errors <- fmt.Errorf("writer %d: %v", id, err)
					return
				}
				
				// Small delay to allow interleaving
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}
	
	wg.Wait()
	close(errors)
	
	// Collect errors
	errorCount := 0
	for err := range errors {
		errorCount++
		t.Logf("Concurrent write error: %v", err)
	}
	
	if errorCount == 0 {
		t.Error("Expected at least some writes to fail with disk full")
	} else {
		t.Logf("Got %d disk full errors from concurrent writers", errorCount)
	}
}

func testLargeBufferDiskFull(t *testing.T) {
	testPath := os.Getenv("OMNI_DISKFULL_TEST_PATH")
	logPath := filepath.Join(testPath, "large_buffer.log")
	
	os.RemoveAll(logPath)
	
	backend, err := backends.NewFileBackend(logPath)
	if err != nil {
		t.Fatalf("Failed to create file backend: %v", err)
	}
	defer backend.Close()
	
	// Write without flushing to test buffer behavior
	chunk := make([]byte, 32*1024) // 32KB chunks (matches default buffer size)
	for i := range chunk {
		chunk[i] = 'B'
	}
	
	writeCount := 0
	for i := 0; i < 100; i++ { // Try to write 3.2MB
		_, err := backend.Write(chunk)
		if err != nil {
			t.Logf("Buffered write failed after %d writes: %v", writeCount, err)
			break
		}
		writeCount++
	}
	
	// Now flush - this might fail
	err = backend.Flush()
	if err != nil {
		t.Logf("Flush failed as expected: %v", err)
	} else if writeCount > 32 { // More than 1MB worth
		t.Error("Expected flush to fail when buffer exceeds disk space")
	}
}

func testDiskFullWithRotation(t *testing.T) {
	testPath := os.Getenv("OMNI_DISKFULL_TEST_PATH")
	logPath := filepath.Join(testPath, "rotation_test.log")
	
	// Clean up
	os.RemoveAll(testPath)
	os.MkdirAll(testPath, 0755)
	
	// Create rotation manager
	rotMgr := features.NewRotationManager()
	rotMgr.SetMaxFiles(2) // Keep only 2 rotated files
	
	// Track errors
	var errorLog []string
	errorHandler := func(source, dest, msg string, err error) {
		logMsg := fmt.Sprintf("[%s] %s -> %s: %s", source, dest, msg, err)
		errorLog = append(errorLog, logMsg)
		t.Log(logMsg)
	}
	
	// Create backend with rotation support
	backend, err := backends.NewFileBackendWithRotation(logPath, rotMgr)
	if err != nil {
		t.Fatalf("Failed to create file backend: %v", err)
	}
	backend.SetMaxRetries(2) // Allow 2 retries on disk full
	backend.SetErrorHandler(errorHandler)
	defer backend.Close()
	
	// Write data to fill disk
	chunk := make([]byte, 200*1024) // 200KB chunks
	for i := range chunk {
		chunk[i] = byte('A' + (i % 26))
	}
	
	filesCreated := 0
	
	// Keep writing until we've triggered rotation multiple times
	for i := 0; i < 10; i++ {
		n, err := backend.Write(chunk)
		
		if err != nil {
			t.Logf("Write %d failed after %d bytes: %v", i, n, err)
			
			// Check if rotation happened
			rotatedFiles, _ := rotMgr.GetRotatedFiles(logPath)
			if len(rotatedFiles) > filesCreated {
				filesCreated = len(rotatedFiles)
				t.Logf("Rotation triggered, now have %d rotated files", filesCreated)
			}
		} else {
			t.Logf("Write %d succeeded: %d bytes", i, n)
		}
		
		// Flush to ensure data hits disk
		backend.Flush()
	}
	
	// Verify rotation happened
	if filesCreated == 0 {
		t.Error("Expected at least one rotation to occur")
	}
	
	// Verify old files were cleaned up (should have at most 2)
	rotatedFiles, _ := rotMgr.GetRotatedFiles(logPath)
	if len(rotatedFiles) > 2 {
		t.Errorf("Expected at most 2 rotated files, found %d", len(rotatedFiles))
	}
	
	// Verify we can still write after cleanup
	testData := []byte("Final write after rotations\n")
	n, err := backend.Write(testData)
	if err != nil {
		t.Errorf("Final write failed: %v", err)
	} else if n != len(testData) {
		t.Errorf("Final write incomplete: %d of %d bytes", n, len(testData))
	}
	
	// Log error summary
	if len(errorLog) > 0 {
		t.Logf("Total errors logged: %d", len(errorLog))
	}
}

func testDiskFullRotationRaceCondition(t *testing.T) {
	testPath := os.Getenv("OMNI_DISKFULL_TEST_PATH")
	logPath := filepath.Join(testPath, "race_test.log")
	
	// Clean up
	os.RemoveAll(testPath)
	os.MkdirAll(testPath, 0755)
	
	// Create shared rotation manager
	rotMgr := features.NewRotationManager()
	rotMgr.SetMaxFiles(1) // Very aggressive cleanup
	
	// Create backend
	backend, err := backends.NewFileBackendWithRotation(logPath, rotMgr)
	if err != nil {
		t.Fatalf("Failed to create file backend: %v", err)
	}
	backend.SetMaxRetries(3)
	defer backend.Close()
	
	var wg sync.WaitGroup
	errors := make(chan error, 20)
	rotations := int32(0)
	
	// Monitor rotations
	rotMgr.SetMetricsHandler(func(metric string) {
		if metric == "rotation_completed" {
			atomic.AddInt32(&rotations, 1)
		}
	})
	
	// Multiple writers trying to fill disk
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			data := make([]byte, 150*1024) // 150KB
			for j := range data {
				data[j] = byte('0' + id)
			}
			
			for j := 0; j < 5; j++ {
				_, err := backend.Write(data)
				if err != nil {
					errors <- fmt.Errorf("writer %d iteration %d: %v", id, j, err)
				}
				time.Sleep(5 * time.Millisecond)
			}
		}(i)
	}
	
	wg.Wait()
	close(errors)
	
	// Check results
	errorCount := 0
	for err := range errors {
		errorCount++
		t.Logf("Concurrent error: %v", err)
	}
	
	rotationCount := atomic.LoadInt32(&rotations)
	t.Logf("Total rotations triggered: %d", rotationCount)
	
	if rotationCount == 0 {
		t.Error("Expected at least one rotation during concurrent writes")
	}
	
	// Verify only 1 rotated file remains (as configured)
	rotatedFiles, _ := rotMgr.GetRotatedFiles(logPath)
	if len(rotatedFiles) > 1 {
		t.Errorf("Expected at most 1 rotated file, found %d", len(rotatedFiles))
	}
}