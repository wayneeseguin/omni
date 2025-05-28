package flexlog

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

// RotationTimeFormat is the timestamp format used for rotated log files
// Format is sortable and includes millisecond precision
const RotationTimeFormat = "20060102-150405.000"

// rotate rotates log files using timestamp-based naming
func (f *FlexLog) rotate() error {
	// Lock already acquired in flocklogf

	// Flush current file
	if f.writer != nil {
		if err := f.writer.Flush(); err != nil {
			return fmt.Errorf("flushing current log: %w", err)
		}
	}

	// Close current file
	if f.file != nil {
		if err := f.file.Close(); err != nil {
			return fmt.Errorf("closing current log: %w", err)
		}
	}

	// Generate timestamp for rotation
	timestamp := time.Now().Format(RotationTimeFormat)
	rotatedPath := fmt.Sprintf("%s.%s", f.path, timestamp)

	// Rename current file to timestamped name
	if err := os.Rename(f.path, rotatedPath); err != nil {
		return fmt.Errorf("rotating current log: %w", err)
	}

	// Queue for compression if enabled
	if f.compression != CompressionNone {
		f.queueForCompression(rotatedPath)
	}

	// Open new file
	file, err := os.OpenFile(f.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return &FileError{Op: "open", Path: f.path, Err: err}
	}

	// Create new writer - if this fails, we need to close the file
	newWriter := bufio.NewWriterSize(file, defaultBufferSize)

	// Update FlexLog state atomically
	f.file = file
	f.writer = newWriter
	f.currentSize = 0

	// Clean up old files if needed
	if f.maxFiles > 0 {
		if err := f.cleanupOldFiles(); err != nil {
			// Log error but don't fail rotation
			f.logError("cleanup", "", "Failed to cleanup old files", err, ErrorLevelLow)
		}
	}

	return nil
}

// SetMaxAge sets the maximum age for log files
// Logs older than this will be deleted during cleanup
// Use 0 to disable age-based cleanup
func (f *FlexLog) SetMaxAge(duration time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.maxAge = duration

	// Start or stop the cleanup process based on the new setting
	if f.maxAge > 0 {
		f.startCleanupRoutine()
	} else if f.maxAge == 0 && f.cleanupTicker != nil {
		f.stopCleanupRoutine()
	}
}

// SetCleanupInterval sets how often to check for and remove old log files
// Default is 1 hour
func (f *FlexLog) SetCleanupInterval(interval time.Duration) {
	if interval < time.Minute {
		interval = time.Minute // Enforce a reasonable minimum
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// Only update if cleanup is already running
	if f.cleanupTicker != nil {
		f.stopCleanupRoutine()
		f.cleanupInterval = interval
		f.startCleanupRoutine()
	} else {
		f.cleanupInterval = interval
	}
}

// startCleanupRoutine starts the background goroutine for age-based pruning
func (f *FlexLog) startCleanupRoutine() {
	// Don't start if already running or max age is 0
	if f.cleanupTicker != nil || f.maxAge == 0 {
		return
	}

	f.cleanupTicker = time.NewTicker(f.cleanupInterval)
	f.cleanupDone = make(chan struct{})

	f.cleanupWg.Add(1)
	go func() {
		defer f.cleanupWg.Done()
		defer func() {
			if r := recover(); r != nil {
				f.logError("cleanup", "", "Panic in cleanup routine", fmt.Errorf("%v", r), ErrorLevelHigh)
			}
		}()

		for {
			select {
			case <-f.cleanupTicker.C:
				if err := f.cleanupOldLogs(); err != nil {
					f.logError("cleanup", "", "Error cleaning up old logs", err, ErrorLevelMedium)
				}
			case <-f.cleanupDone:
				return
			}
		}
	}()
}

// stopCleanupRoutine stops the background cleanup goroutine
func (f *FlexLog) stopCleanupRoutine() {
	if f.cleanupTicker == nil {
		return
	}

	f.cleanupTicker.Stop()
	if f.cleanupDone != nil {
		close(f.cleanupDone)
	}

	// Wait for the cleanup goroutine to finish
	f.cleanupWg.Wait()

	f.cleanupTicker = nil
	f.cleanupDone = nil
}

// cleanupOldLogs removes log files older than maxAge
func (f *FlexLog) cleanupOldLogs() error {
	if f.maxAge == 0 {
		return nil // Age-based cleanup disabled
	}

	// Try to acquire lock with timeout
	lockAcquired := make(chan bool)
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()

	go func() {
		f.mu.Lock()
		select {
		case lockAcquired <- true:
			// Successfully sent signal that lock was acquired
		default:
			// Channel closed or timeout occurred, release the lock
			f.mu.Unlock()
		}
	}()

	// Wait for lock with timeout
	select {
	case <-lockAcquired:
		defer f.mu.Unlock()
	case <-timeout.C:
		close(lockAcquired) // Signal the goroutine to release the lock if it acquires it
		return fmt.Errorf("timed out waiting for lock in cleanupOldLogs")
	}

	// Check if we have a valid path
	if f.path == "" {
		return nil // No primary log file to clean up
	}

	// Get the directory and pattern for log files
	dir := filepath.Dir(f.path)
	base := filepath.Base(f.path)

	// Start the timestamp for comparison
	cutoff := time.Now().Add(-f.maxAge)

	// List directory
	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading log directory: %w", err)
	}

	// Match patterns for timestamp-based log files
	// Pattern: base.YYYYMMDD-HHMMSS.sss or base.YYYYMMDD-HHMMSS.sss.gz
	pattern := regexp.MustCompile(fmt.Sprintf(`^%s\.(\d{8}-\d{6}\.\d{3})(?:\.gz)?$`, regexp.QuoteMeta(base)))

	for _, file := range files {
		// Skip directories
		if file.IsDir() {
			continue
		}

		// Check if this file matches our pattern
		matches := pattern.FindStringSubmatch(file.Name())
		if len(matches) != 2 {
			continue
		}

		// Skip the current active log file
		if file.Name() == base {
			continue
		}

		filePath := filepath.Join(dir, file.Name())

		// Parse timestamp from filename
		fileTime, err := time.Parse(RotationTimeFormat, matches[1])
		if err != nil {
			f.logError("cleanup", "", fmt.Sprintf("Error parsing timestamp from %s", file.Name()), err, ErrorLevelLow)
			continue
		}

		// Check if file is older than cutoff
		if fileTime.Before(cutoff) {
			// Remove the file
			if err := os.Remove(filePath); err != nil {
				f.logError("cleanup", "", fmt.Sprintf("Error removing old log file %s", filePath), err, ErrorLevelLow)
			} else {
				fmt.Fprintf(os.Stderr, "Removed old log file: %s (age: %v)\n",
					filePath, time.Since(fileTime))
			}
		}
	}

	return nil
}

// cleanupOldFiles removes old rotated files based on maxFiles count
func (f *FlexLog) cleanupOldFiles() error {
	if f.maxFiles <= 0 {
		return nil // No file count limit
	}

	// Get the directory and base name
	dir := filepath.Dir(f.path)
	base := filepath.Base(f.path)

	// List directory
	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading log directory: %w", err)
	}

	// Match patterns for timestamp-based log files
	// Pattern: base.YYYYMMDD-HHMMSS.sss or base.YYYYMMDD-HHMMSS.sss.gz
	pattern := regexp.MustCompile(fmt.Sprintf(`^%s\.(\d{8}-\d{6}\.\d{3})(?:\.gz)?$`, regexp.QuoteMeta(base)))

	// Collect matching files with their timestamps
	type logFile struct {
		path      string
		timestamp string
	}
	var logFiles []logFile

	for _, file := range files {
		// Skip directories
		if file.IsDir() {
			continue
		}

		// Check if this file matches our pattern
		matches := pattern.FindStringSubmatch(file.Name())
		if len(matches) != 2 {
			continue
		}

		logFiles = append(logFiles, logFile{
			path:      filepath.Join(dir, file.Name()),
			timestamp: matches[1],
		})
	}

	// Sort by timestamp (newest first)
	sort.Slice(logFiles, func(i, j int) bool {
		return logFiles[i].timestamp > logFiles[j].timestamp
	})

	// Remove files beyond maxFiles limit
	if len(logFiles) > f.maxFiles {
		for i := f.maxFiles; i < len(logFiles); i++ {
			if err := os.Remove(logFiles[i].path); err != nil {
				f.logError("cleanup", "", fmt.Sprintf("Error removing old log file %s", logFiles[i].path), err, ErrorLevelLow)
			} else {
				fmt.Fprintf(os.Stderr, "Removed old log file (exceeded maxFiles): %s\n", logFiles[i].path)
			}
		}
	}

	return nil
}

// RunCleanup immediately runs the cleanup process for old log files
func (f *FlexLog) RunCleanup() error {
	// Don't call cleanupOldLogs with lock held, as it tries to acquire lock itself
	return f.cleanupOldLogs()
}

// rotateDestination rotates a specific destination's log file
func (f *FlexLog) rotateDestination(dest *Destination) error {
	// Only rotate file-based destinations
	if dest.Backend != BackendFlock {
		return nil
	}

	// Flush the current file
	dest.mu.Lock()
	if dest.Writer != nil {
		if err := dest.Writer.Flush(); err != nil {
			dest.mu.Unlock()
			return fmt.Errorf("flushing log: %w", err)
		}
	}
	dest.mu.Unlock()

	// Close the file
	if dest.File != nil {
		if err := dest.File.Close(); err != nil {
			return fmt.Errorf("closing log file: %w", err)
		}
	}

	// Generate timestamp for rotation
	timestamp := time.Now().Format(RotationTimeFormat)
	rotatedPath := fmt.Sprintf("%s.%s", dest.URI, timestamp)

	// Rename the current file
	if err := os.Rename(dest.URI, rotatedPath); err != nil {
		return &FileError{Op: "rename", Path: dest.URI, Err: err}
	}

	// Open a new file
	newFile, err := os.OpenFile(dest.URI, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// Try to restore the original file
		if restoreErr := os.Rename(rotatedPath, dest.URI); restoreErr != nil {
			return &FileError{Op: "open", Path: dest.URI,
				Err: fmt.Errorf("%w (failed to restore original: %v)", err, restoreErr)}
		}
		return &FileError{Op: "open", Path: dest.URI, Err: err}
	}

	// Create new writer - ensure we close file if this somehow fails
	newWriter := bufio.NewWriterSize(newFile, defaultBufferSize)

	// Close old file handle if it exists
	dest.mu.Lock()
	oldFile := dest.File
	dest.File = newFile
	dest.Writer = newWriter
	dest.Size = 0
	dest.mu.Unlock()

	// Close old file after updating references
	if oldFile != nil {
		if err := oldFile.Close(); err != nil {
			// Log the error but continue
			f.logError("rotate", dest.Name, "Failed to close old file", err, ErrorLevelLow)
		}
	}

	// Track rotation metrics
	dest.trackRotation()
	f.trackRotation()

	// Queue for compression if configured
	if f.compression != CompressionNone && f.compressCh != nil {
		select {
		case f.compressCh <- rotatedPath:
			// Successfully queued for compression
		default:
			// Compression queue full, just log and continue
			dest.mu.Lock()
			fmt.Fprintf(dest.Writer, "[%s] WARNING: Compression queue full, skipping compression for %s\n",
				time.Now().Format("2006-01-02 15:04:05.000"), rotatedPath)
			dest.mu.Unlock()
		}
	}

	// Clean up old files if needed
	if f.maxFiles > 0 {
		if err := f.cleanupOldFilesForDestination(dest.URI); err != nil {
			// Log error but don't fail rotation
			fmt.Fprintf(os.Stderr, "Warning: failed to cleanup old files for destination: %v\n", err)
		}
	}

	return nil
}

// cleanupOldFilesForDestination removes old rotated files for a specific destination
func (f *FlexLog) cleanupOldFilesForDestination(path string) error {
	if f.maxFiles <= 0 {
		return nil // No file count limit
	}

	// Get the directory and base name
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	// List directory
	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading log directory: %w", err)
	}

	// Match patterns for timestamp-based log files
	// Pattern: base.YYYYMMDD-HHMMSS.sss or base.YYYYMMDD-HHMMSS.sss.gz
	pattern := regexp.MustCompile(fmt.Sprintf(`^%s\.(\d{8}-\d{6}\.\d{3})(?:\.gz)?$`, regexp.QuoteMeta(base)))

	// Collect matching files with their timestamps
	type logFile struct {
		path      string
		timestamp string
	}
	var logFiles []logFile

	for _, file := range files {
		// Skip directories
		if file.IsDir() {
			continue
		}

		// Check if this file matches our pattern
		matches := pattern.FindStringSubmatch(file.Name())
		if len(matches) != 2 {
			continue
		}

		logFiles = append(logFiles, logFile{
			path:      filepath.Join(dir, file.Name()),
			timestamp: matches[1],
		})
	}

	// Sort by timestamp (newest first)
	sort.Slice(logFiles, func(i, j int) bool {
		return logFiles[i].timestamp > logFiles[j].timestamp
	})

	// Remove files beyond maxFiles limit
	if len(logFiles) > f.maxFiles {
		for i := f.maxFiles; i < len(logFiles); i++ {
			if err := os.Remove(logFiles[i].path); err != nil {
				f.logError("cleanup", "", fmt.Sprintf("Error removing old log file %s", logFiles[i].path), err, ErrorLevelLow)
			} else {
				fmt.Fprintf(os.Stderr, "Removed old log file (exceeded maxFiles): %s\n", logFiles[i].path)
			}
		}
	}

	return nil
}
