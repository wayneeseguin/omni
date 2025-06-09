package backends

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gofrs/flock"
	"github.com/wayneeseguin/omni/pkg/features"
)

// FileBackendWithRotation extends FileBackendImpl with disk-full handling
type FileBackendWithRotation struct {
	file            *os.File
	writer          *bufio.Writer
	lock            *flock.Flock
	path            string
	size            int64
	mu              sync.Mutex
	rotationManager *features.RotationManager
	maxRetries      int
	errorHandler    func(source, dest, msg string, err error)
}

// NewFileBackendWithRotation creates a new file backend with rotation support
func NewFileBackendWithRotation(path string, rotMgr *features.RotationManager) (*FileBackendWithRotation, error) {
	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	// Clean the path to prevent directory traversal
	cleanPath := filepath.Clean(path)
	
	// Open file
	file, err := os.OpenFile(cleanPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}

	// Get file size
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("stat file: %w", err)
	}

	// Create file lock
	lock := flock.New(cleanPath)

	// Add log path to rotation manager
	if rotMgr != nil {
		rotMgr.AddLogPath(cleanPath)
	}

	return &FileBackendWithRotation{
		file:            file,
		writer:          bufio.NewWriterSize(file, DefaultBufferSize),
		lock:            lock,
		path:            cleanPath,
		size:            info.Size(),
		rotationManager: rotMgr,
		maxRetries:      3, // Default to 3 retries
	}, nil
}

// SetMaxRetries sets the maximum number of retries on disk full
func (fb *FileBackendWithRotation) SetMaxRetries(retries int) {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	fb.maxRetries = retries
}

// SetErrorHandler sets the error handler function
func (fb *FileBackendWithRotation) SetErrorHandler(handler func(source, dest, msg string, err error)) {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	fb.errorHandler = handler
}

// Write writes a log entry to the file with disk-full handling
func (fb *FileBackendWithRotation) Write(entry []byte) (int, error) {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	
	retries := 0
	for retries <= fb.maxRetries {
		// Try to acquire lock
		if err := fb.lock.Lock(); err != nil {
			return 0, fmt.Errorf("acquire lock: %w", err)
		}
		
		// Write entry
		n, err := fb.writer.Write(entry)
		fb.lock.Unlock()
		
		if err == nil {
			fb.size += int64(n)
			return n, nil
		}
		
		// Check if error is disk full
		if !isDiskFullError(err) {
			return n, err
		}
		
		// Handle disk full
		if retries < fb.maxRetries {
			if fb.errorHandler != nil {
				fb.errorHandler("write", fb.path, fmt.Sprintf("Disk full, attempting rotation (retry %d/%d)", retries+1, fb.maxRetries), err)
			}
			
			if handleErr := fb.handleDiskFull(); handleErr != nil {
				return n, fmt.Errorf("disk full handling failed: %w", handleErr)
			}
			retries++
			continue
		}
		
		return n, err
	}
	
	return 0, fmt.Errorf("write failed after %d retries", fb.maxRetries)
}

// isDiskFullError checks if an error indicates disk is full
func isDiskFullError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "no space left") ||
		strings.Contains(errStr, "enospc") ||
		strings.Contains(errStr, "disk full") ||
		strings.Contains(errStr, "out of disk space")
}

// handleDiskFull handles disk full condition by rotating logs
func (fb *FileBackendWithRotation) handleDiskFull() error {
	// If no rotation manager, we can't do anything
	if fb.rotationManager == nil {
		return fmt.Errorf("no rotation manager configured")
	}
	
	// Flush current buffer
	if err := fb.writer.Flush(); err != nil && !isDiskFullError(err) {
		return fmt.Errorf("flush before rotation: %w", err)
	}
	
	// Close current file
	if err := fb.file.Close(); err != nil {
		return fmt.Errorf("close before rotation: %w", err)
	}
	
	// Rotate current log
	rotatedPath, err := fb.rotationManager.RotateFile(fb.path, nil)
	if err != nil {
		return fmt.Errorf("rotate file: %w", err)
	}
	
	if fb.errorHandler != nil {
		fb.errorHandler("rotation", fb.path, fmt.Sprintf("Rotated log to %s", rotatedPath), nil)
	}
	
	// Clean up old logs to free space
	if err := fb.rotationManager.RunCleanup(fb.path); err != nil {
		// Log error but don't fail - we still want to try opening new file
		if fb.errorHandler != nil {
			fb.errorHandler("cleanup", fb.path, "Cleanup failed during disk full", err)
		}
	}
	
	// If still no space, try aggressive cleanup
	if err := fb.removeOldestLogs(); err != nil {
		if fb.errorHandler != nil {
			fb.errorHandler("cleanup", fb.path, "Failed to remove oldest logs", err)
		}
	}
	
	// Reopen file
	file, err := os.OpenFile(fb.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("reopen file after rotation: %w", err)
	}
	
	fb.file = file
	fb.writer = bufio.NewWriterSize(file, DefaultBufferSize)
	fb.size = 0
	fb.lock = flock.New(fb.path)
	
	return nil
}

// removeOldestLogs removes oldest rotated logs until we have space
func (fb *FileBackendWithRotation) removeOldestLogs() error {
	if fb.rotationManager == nil {
		return fmt.Errorf("no rotation manager configured")
	}
	
	rotatedFiles, err := fb.rotationManager.GetRotatedFiles(fb.path)
	if err != nil {
		return err
	}
	
	if len(rotatedFiles) == 0 {
		return fmt.Errorf("no rotated files to remove")
	}
	
	// Remove oldest files first (list is sorted newest to oldest)
	removed := 0
	for i := len(rotatedFiles) - 1; i >= 0; i-- {
		if err := os.Remove(rotatedFiles[i].Path); err != nil {
			if fb.errorHandler != nil {
				fb.errorHandler("cleanup", rotatedFiles[i].Path, "Failed to remove old log", err)
			}
			continue
		}
		
		removed++
		if fb.errorHandler != nil {
			fb.errorHandler("cleanup", rotatedFiles[i].Path, "Removed old log to free space", nil)
		}
		
		// Try to create a small test file to check if we have space
		testPath := fb.path + ".test"
		if testFile, err := os.Create(testPath); err == nil {
			testFile.Close()
			os.Remove(testPath)
			return nil // We have space now
		}
		
		// Continue removing files if still no space
	}
	
	if removed == 0 {
		return fmt.Errorf("unable to remove any files")
	}
	
	return fmt.Errorf("removed %d files but still insufficient space", removed)
}

// Flush flushes buffered data to disk
func (fb *FileBackendWithRotation) Flush() error {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	
	if fb.writer != nil {
		return fb.writer.Flush()
	}
	return nil
}

// Close closes the file backend
func (fb *FileBackendWithRotation) Close() error {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	
	var errs []error

	// Remove from rotation manager
	if fb.rotationManager != nil {
		fb.rotationManager.RemoveLogPath(fb.path)
	}

	// Flush writer
	if fb.writer != nil {
		if err := fb.writer.Flush(); err != nil {
			errs = append(errs, fmt.Errorf("flush: %w", err))
		}
	}

	// Unlock file
	if fb.lock != nil {
		if err := fb.lock.Unlock(); err != nil {
			errs = append(errs, fmt.Errorf("unlock: %w", err))
		}
	}

	// Close file
	if fb.file != nil {
		if err := fb.file.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close file: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}

// SupportsAtomic returns true as file backend supports atomic writes via locking
func (fb *FileBackendWithRotation) SupportsAtomic() bool {
	return true
}

// Rotate manually rotates the log file
func (fb *FileBackendWithRotation) Rotate() error {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	
	if fb.rotationManager == nil {
		return fmt.Errorf("no rotation manager configured")
	}
	
	// Flush before rotation
	if err := fb.writer.Flush(); err != nil {
		return fmt.Errorf("flush before rotation: %w", err)
	}
	
	// Close current file
	if err := fb.file.Close(); err != nil {
		return fmt.Errorf("close before rotation: %w", err)
	}
	
	// Rotate
	_, err := fb.rotationManager.RotateFile(fb.path, nil)
	if err != nil {
		return err
	}
	
	// Reopen
	file, err := os.OpenFile(fb.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("reopen after rotation: %w", err)
	}
	
	fb.file = file
	fb.writer = bufio.NewWriterSize(file, DefaultBufferSize)
	fb.size = 0
	
	return nil
}

// Size returns the current file size
func (fb *FileBackendWithRotation) Size() int64 {
	return fb.size
}

// Path returns the file path
func (fb *FileBackendWithRotation) Path() string {
	return fb.path
}

// GetFile returns the underlying file
func (fb *FileBackendWithRotation) GetFile() *os.File {
	return fb.file
}

// GetWriter returns the buffered writer
func (fb *FileBackendWithRotation) GetWriter() *bufio.Writer {
	return fb.writer
}

// GetLock returns the file lock
func (fb *FileBackendWithRotation) GetLock() *flock.Flock {
	return fb.lock
}

// GetSize returns the current file size
func (fb *FileBackendWithRotation) GetSize() int64 {
	return fb.size
}

// Sync syncs the file to disk
func (fb *FileBackendWithRotation) Sync() error {
	if err := fb.Flush(); err != nil {
		return err
	}
	
	fb.mu.Lock()
	defer fb.mu.Unlock()
	
	if fb.file != nil {
		return fb.file.Sync()
	}
	return nil
}

// GetStats returns backend statistics
func (fb *FileBackendWithRotation) GetStats() BackendStats {
	bytesWritten := uint64(0)
	if fb.size > 0 {
		bytesWritten = uint64(fb.size)
	}
	
	return BackendStats{
		Path:         fb.path,
		Size:         fb.size,
		WriteCount:   0, // Would need to track this
		BytesWritten: bytesWritten,
	}
}