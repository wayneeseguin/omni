package backends

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
)

// Removed Omni wrapper type - not needed as we can't extend types from other packages in Go

// DefaultBufferSize for file operations
const DefaultBufferSize = 32 * 1024 // 32 KB - matching omni package default

// Variable aliases for easier access
var (
	ErrDestinationNotFound = fmt.Errorf("destination not found")
)

// Error codes for backend operations
const (
	ErrCodeFileOpen = "file_open"
	ErrCodeFileLock = "file_lock"
)

// FileBackendImpl implements the Backend interface for file-based logging
type FileBackendImpl struct {
	file   *os.File
	writer *bufio.Writer
	lock   *flock.Flock
	path   string
	size   int64
}

// NewFileBackend creates a new file backend
func NewFileBackend(path string) (*FileBackendImpl, error) {
	// Create directory if needed
	dir := filepath.Dir(path)
	// #nosec G301 - log directories need to be accessible by other processes
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	// Clean the path to prevent directory traversal
	cleanPath := filepath.Clean(path)
	
	// Open file
	file, err := os.OpenFile(cleanPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644) // #nosec G302 - log files need to be readable
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}

	// Get file size
	info, err := file.Stat()
	if err != nil {
		_ = file.Close() // Best effort close on error path
		return nil, fmt.Errorf("stat file: %w", err)
	}

	// Create file lock
	lock := flock.New(cleanPath)

	return &FileBackendImpl{
		file:   file,
		writer: bufio.NewWriterSize(file, DefaultBufferSize),
		lock:   lock,
		path:   cleanPath,
		size:   info.Size(),
	}, nil
}

// Write writes a log entry to the file
func (fb *FileBackendImpl) Write(entry []byte) (int, error) {
	// Try to acquire lock
	if err := fb.lock.Lock(); err != nil {
		return 0, fmt.Errorf("acquire lock: %w", err)
	}
	defer func() {
		_ = fb.lock.Unlock() // Best effort unlock
	}()

	// Write entry
	n, err := fb.writer.Write(entry)
	if err != nil {
		return n, err
	}

	fb.size += int64(n)
	return n, nil
}

// Flush flushes buffered data to disk
func (fb *FileBackendImpl) Flush() error {
	if fb.writer != nil {
		return fb.writer.Flush()
	}
	return nil
}

// Close closes the file backend
func (fb *FileBackendImpl) Close() error {
	var errs []error

	// Flush writer
	if err := fb.Flush(); err != nil {
		errs = append(errs, fmt.Errorf("flush: %w", err))
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
func (fb *FileBackendImpl) SupportsAtomic() bool {
	return true
}

// Rotate rotates the log file
func (fb *FileBackendImpl) Rotate() error {
	// Implementation would go here
	return fmt.Errorf("not implemented")
}

// Size returns the current file size
func (fb *FileBackendImpl) Size() int64 {
	return fb.size
}

// Path returns the file path
func (fb *FileBackendImpl) Path() string {
	return fb.path
}

// GetFile returns the underlying file
func (fb *FileBackendImpl) GetFile() *os.File {
	return fb.file
}

// GetWriter returns the buffered writer
func (fb *FileBackendImpl) GetWriter() *bufio.Writer {
	return fb.writer
}

// GetLock returns the file lock
func (fb *FileBackendImpl) GetLock() *flock.Flock {
	return fb.lock
}

// GetSize returns the current file size
func (fb *FileBackendImpl) GetSize() int64 {
	return fb.size
}

// Sync syncs the file to disk
func (fb *FileBackendImpl) Sync() error {
	if err := fb.Flush(); err != nil {
		return err
	}
	if fb.file != nil {
		return fb.file.Sync()
	}
	return nil
}

// GetStats returns backend statistics
func (fb *FileBackendImpl) GetStats() BackendStats {
	// Ensure size is non-negative before conversion
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

