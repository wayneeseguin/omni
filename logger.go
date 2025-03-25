package flexlog

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

  import "github.com/gofrs/flock"
)

// New creates a new file logger
func New(path string) (*FlexLog, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating log directory: %w", err)
	}

	// Open lock file
	lockPath := path + ".lock"
	lockFd, err := openLockFile(lockPath)
	if err != nil {
		return nil, fmt.Errorf("opening lock file: %w", err)
	}

	// Acquire the lock before opening the log file
	if err := unix.Flock(lockFd, unix.LOCK_EX); err != nil {
		unix.Close(lockFd)
		return nil, fmt.Errorf("acquiring file lock: %w", err)
	}

	// Open log file
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		unix.Flock(lockFd, unix.LOCK_UN)
		unix.Close(lockFd)
		return nil, fmt.Errorf("opening log file: %w", err)
	}

	// Get current file size
	info, err := file.Stat()
	if err != nil {
		file.Close()
		unix.Flock(lockFd, unix.LOCK_UN)
		unix.Close(lockFd)
		return nil, fmt.Errorf("getting file info: %w", err)
	}

	// Release the lock temporarily
	if err := unix.Flock(lockFd, unix.LOCK_UN); err != nil {
		file.Close()
		unix.Close(lockFd)
		return nil, fmt.Errorf("releasing file lock: %w", err)
	}

	f := &FlexLog{
		file:             file,
		writer:           bufio.NewWriterSize(file, defaultBufferSize),
		path:             path,
		maxSize:          defaultMaxSize,
		maxFiles:         defaultMaxFiles,
		currentSize:      info.Size(),
		level:            LevelInfo, // Default to Info level
		lockFd:           lockFd,
		format:           FormatText, // Default to text format
		includeTrace:     false,
		stackSize:        4096, // Default stack trace buffer size
		captureAll:       false,
		formatOptions:    defaultFormatOptions(),
		compression:      CompressionNone,
		compressMinAge:   1,   // compress files after 1 rotation by default
		compressWorkers:  1,   // use 1 compression worker by default
		compressCh:       nil, // initialize only when compression is enabled
		maxAge:           0,   // 0 means no age-based cleanup
		cleanupInterval:  1 * time.Hour,
		cleanupTicker:    nil,
		cleanupDone:      nil,
		filters:          nil,
		samplingStrategy: SamplingNone,
		samplingRate:     1.0, // Default to no sampling (log everything)
		sampleCounter:    0,
		sampleKeyFunc:    defaultSampleKeyFunc,
	}

	return f, nil
}

// openLockFile opens or creates a lock file
func openLockFile(path string) (int, error) {
	fd, err := unix.Open(path, unix.O_CREAT|unix.O_RDWR, 0644)
	if err != nil {
		return -1, err
	}
	return fd, nil
}

// acquireLock acquires the file lock
func (f *FlexLog) acquireLock() error {
	return unix.Flock(f.lockFd, unix.LOCK_EX)
}

// releaseLock releases the file lock
func (f *FlexLog) releaseLock() error {
	return unix.Flock(f.lockFd, unix.LOCK_UN)
}

// SetMaxSize sets the maximum log file size
func (f *FlexLog) SetMaxSize(size int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.maxSize = size
}

// SetMaxFiles sets the maximum number of log files
func (f *FlexLog) SetMaxFiles(count int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.maxFiles = count
}

// SetLevel sets the minimum log level
func (f *FlexLog) SetLevel(level int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.level = level
}

// Close flushes and closes the log file
func (f *FlexLog) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Stop background processes
	f.stopCleanupRoutine()
	f.stopCompressionWorkers()

	if f.writer == nil || f.file == nil {
		return nil // Already closed or nil
	}

	// Acquire lock before closing
	if err := f.acquireLock(); err != nil {
		return fmt.Errorf("acquiring lock for close: %w", err)
	}
	defer f.releaseLock()

	if err := f.writer.Flush(); err != nil {
		return fmt.Errorf("flushing log: %w", err)
	}

	if err := f.file.Close(); err != nil {
		return fmt.Errorf("closing log file: %w", err)
	}

	// Close the lock file
	if f.lockFd > 0 {
		unix.Close(f.lockFd)
		f.lockFd = -1
	}

	f.writer = nil
	f.file = nil

	return nil
}

// Flush forces a flush of the buffer
func (f *FlexLog) Flush() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.writer == nil {
		return fmt.Errorf("logger is closed")
	}

	// Acquire lock before flushing
	if err := f.acquireLock(); err != nil {
		return fmt.Errorf("acquiring lock for flush: %w", err)
	}
	defer f.releaseLock()

	return f.writer.Flush()
}

// flexlogf writes a formatted log entry
func (f *FlexLog) flexlogf(format string, args ...interface{}) {
	// Check if we should log this based on sampling first (quick check before acquiring lock)
	if !f.shouldLog(LevelInfo, format, nil) {
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// Format log entry
	now := time.Now()
	message := fmt.Sprintf(format, args...)

	if f.format == FormatJSON {
		// When using JSON format, redirect to structured logging
		f.mu.Unlock()
		entry := LogEntry{
			Timestamp: now.Format("2006-01-02 15:04:05.000"),
			Level:     "LOG",
			Message:   message,
		}

		// Add file and line information
		if f.includeTrace {
			_, file, line, ok := runtime.Caller(2)
			if ok {
				entry.File = file
				entry.Line = line
			}
		}

		f.writeLogEntry(entry)
		f.mu.Lock()
		return
	}

	// Text format - continue with existing implementation
	entry := fmt.Sprintf("[%s] %s\n",
		now.Format("2006-01-02 15:04:05.000"),
		message)

	// Acquire file lock for cross-process safety
	if err := f.acquireLock(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to acquire file lock: %v\n", err)
		return
	}
	defer f.releaseLock()

	// Check if rotation needed
	entrySize := int64(len(entry))
	if f.currentSize+entrySize > f.maxSize {
		if err := f.rotate(); err != nil {
			// If rotation fails, try to write error to file
			fmt.Fprintf(f.writer, "[%s] ERROR: Failed to rotate log file: %v\n",
				now.Format("2006-01-02 15:04:05.000"), err)
			f.writer.Flush()
			return
		}
	}

	// Write entry
	if _, err := f.writer.WriteString(entry); err != nil {
		// If write fails, try to write error to stderr
		fmt.Fprintf(os.Stderr, "Failed to write to log file: %v\n", err)
		return
	}

	// Update size and flush periodically
	f.currentSize += entrySize
	if f.currentSize%defaultBufferSize == 0 {
		f.writer.Flush()
	}
}

// Flog logs a formatted message directly
func (f *FlexLog) Flog(format string, args ...interface{}) {
	f.flexlogf(format, args...)
}

// log writes a log entry with the specified level
// This method is kept for potential backward compatibility
func (f *FlexLog) log(level int, message string) {
	var levelStr string
	switch level {
	case LevelDebug:
		levelStr = "DEBUG"
	case LevelInfo:
		levelStr = "INFO"
	case LevelWarn:
		levelStr = "WARN"
	case LevelError:
		levelStr = "ERROR"
	default:
		levelStr = "LOG"
	}

	f.flexlogf("[%s] %s", levelStr, message)
}

func (f *FlexLog) Write(p []byte) (n int, err error) {

fileLock := flock.New("/var/lock/go-lock.lock")
locked, err := fileLock.TryLock()
if err != nil {
	// handle locking error
}

if locked {
	// do work
	fileLock.Unlock()
}
	f.mu.Lock()
	defer f.mu.Unlock()

	// First write to the file
	if f.file == nil {
		if err = f.open(); err != nil {
			return 0, err
		}
	}

	// If this write would exceed the max size, rotate first
	if f.size+int64(len(p)) > f.maxSize && f.maxSize > 0 {
		if err = f.rotate(); err != nil {
			return 0, err
		}
	}

	n, err = f.file.Write(p)
	f.size += int64(n)

	// Then write to all destinations
	if len(f.destinations) > 0 {
		f.mu.Unlock()
		_, destErr := f.writeToDestinations(p)
		f.mu.Lock()

		// Only override the error if we succeeded writing to the file
		if err == nil {
			err = destErr
		}
	}

	return n, err
}

// open is used by Write to open the file if it's nil
func (f *FlexLog) open() error {
	var err error
	f.file, err = os.OpenFile(f.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	// Get current file size
	info, err := f.file.Stat()
	if err != nil {
		f.file.Close()
		f.file = nil
		return err
	}
	f.size = info.Size()
	return nil
}
