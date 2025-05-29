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

// RotationTimeFormat is the timestamp format used for rotated log files.
// The format is sortable and includes millisecond precision to avoid collisions.
// Example: "20060102-150405.000" produces "20240115-143052.123"
const RotationTimeFormat = "20060102-150405.000"

// rotate rotates the primary log file when it reaches the maximum size.
// It creates a new file with a timestamp suffix and continues logging to a fresh file.
// This method assumes the logger mutex is already locked by the caller.
//
// Returns:
//   - error: Any error encountered during rotation
func (f *FlexLog) rotate() error {
	// Lock already acquired in flocklogf

	// Find the primary destination (if any)
	var primaryDest *Destination
	for _, dest := range f.Destinations {
		if dest.URI == f.path && dest.Backend == BackendFlock {
			primaryDest = dest
			break
		}
	}

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

	// Generate timestamp for rotation (always use UTC for consistency)
	timestamp := time.Now().UTC().Format(RotationTimeFormat)
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
		return NewFlexLogError(ErrCodeFileOpen, "open", f.path, err)
	}

	// Create new writer - if this fails, we need to close the file
	newWriter := bufio.NewWriterSize(file, defaultBufferSize)

	// Update FlexLog state atomically
	f.file = file
	f.writer = newWriter
	f.currentSize = 0

	// Also update the primary destination if it exists
	if primaryDest != nil {
		primaryDest.mu.Lock()
		primaryDest.File = file
		primaryDest.Writer = newWriter
		primaryDest.Size = 0
		primaryDest.mu.Unlock()
	}

	// Clean up old files if needed
	if f.maxFiles > 0 {
		if err := f.cleanupOldFiles(); err != nil {
			// Log error but don't fail rotation
			f.logError("cleanup", "", "Failed to cleanup old files", err, ErrorLevelLow)
		}
	}

	return nil
}

// SetMaxAge sets the maximum age for log files.
// Log files older than this duration will be deleted during cleanup.
// Use 0 to disable age-based cleanup.
//
// Parameters:
//   - duration: Maximum age for log files
//
// Returns:
//   - error: Always returns nil (kept for API compatibility)
//
// Example:
//
//	logger.SetMaxAge(7 * 24 * time.Hour)  // Keep logs for 7 days
//	logger.SetMaxAge(0)                    // Disable age-based cleanup
func (f *FlexLog) SetMaxAge(duration time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.maxAge = duration

	// Start or stop the cleanup process based on the new setting
	if f.maxAge > 0 {
		f.startCleanupRoutine()
	} else if f.maxAge == 0 && f.cleanupTicker != nil {
		f.stopCleanupRoutine()
	}
	return nil
}

// SetCleanupInterval sets how often to check for and remove old log files.
// The default interval is 1 hour. The minimum allowed interval is 1 minute.
//
// Parameters:
//   - interval: How often to run cleanup (minimum 1 minute)
//
// Example:
//
//	logger.SetCleanupInterval(30 * time.Minute)  // Check every 30 minutes
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

// startCleanupRoutine starts the background goroutine for age-based log file cleanup.
// This method is called automatically when SetMaxAge is set to a non-zero value.
// It will not start if already running or if maxAge is 0.
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

// stopCleanupRoutine stops the background cleanup goroutine.
// It waits for the goroutine to finish before returning.
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

// cleanupOldLogs removes log files older than maxAge.
// It runs periodically in the background when age-based cleanup is enabled.
// The method uses a timeout to avoid blocking if the mutex cannot be acquired.
//
// Returns:
//   - error: Any error encountered during cleanup
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
		// Using the timestamp when the file was rotated (from filename)
		if time.Since(fileTime) > f.maxAge {
			// Remove the file
			if err := os.Remove(filePath); err != nil {
				f.logError("cleanup", "", fmt.Sprintf("Error removing old log file %s", filePath), err, ErrorLevelLow)
			} else if !isTestMode() {
				fmt.Fprintf(os.Stderr, "Removed old log file: %s (age: %v)\n",
					filePath, time.Since(fileTime))
			}
		}
	}

	return nil
}

// cleanupOldFiles removes old rotated files based on maxFiles count.
// It keeps the most recent files up to the maxFiles limit and removes older ones.
// Files are identified by their timestamp suffix and sorted chronologically.
//
// Returns:
//   - error: Any error encountered during cleanup
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
			} else if !isTestMode() {
				fmt.Fprintf(os.Stderr, "Removed old log file (exceeded maxFiles): %s\n", logFiles[i].path)
			}
		}
	}

	return nil
}

// RunCleanup immediately runs the cleanup process for old log files.
// This can be called manually to trigger cleanup outside of the regular schedule.
//
// Returns:
//   - error: Any error encountered during cleanup
func (f *FlexLog) RunCleanup() error {
	// Don't call cleanupOldLogs with lock held, as it tries to acquire lock itself
	return f.cleanupOldLogs()
}

// rotateDestination rotates a specific destination's log file.
// It renames the current log file with a timestamp suffix and creates a new file
// for continued logging. Only applies to file-based destinations.
//
// Parameters:
//   - dest: The destination to rotate
//
// Returns:
//   - error: Any error encountered during rotation
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

	// Note: We'll close the file after updating references to avoid race conditions

	// Generate timestamp for rotation (always use UTC for consistency)
	timestamp := time.Now().UTC().Format(RotationTimeFormat)
	rotatedPath := fmt.Sprintf("%s.%s", dest.URI, timestamp)

	// Rename the current file
	if err := os.Rename(dest.URI, rotatedPath); err != nil {
		return NewFlexLogError(ErrCodeFileRotate, "rename", dest.URI, err)
	}

	// Open a new file
	newFile, err := os.OpenFile(dest.URI, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// Try to restore the original file
		if restoreErr := os.Rename(rotatedPath, dest.URI); restoreErr != nil {
			return NewFlexLogError(ErrCodeFileOpen, "open", dest.URI,
				fmt.Errorf("%w (failed to restore original: %v)", err, restoreErr))
		}
		return NewFlexLogError(ErrCodeFileOpen, "open", dest.URI, err)
	}

	// Create new writer - ensure we close file if this somehow fails
	newWriter := bufio.NewWriterSize(newFile, defaultBufferSize)
	// Note: bufio.NewWriterSize cannot fail, it just returns a writer

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

// cleanupOldFilesForDestination removes old rotated files for a specific destination.
// It keeps the most recent files up to the maxFiles limit for the given path.
//
// Parameters:
//   - path: The log file path to cleanup rotated files for
//
// Returns:
//   - error: Any error encountered during cleanup
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
			} else if !isTestMode() {
				fmt.Fprintf(os.Stderr, "Removed old log file (exceeded maxFiles): %s\n", logFiles[i].path)
			}
		}
	}

	return nil
}
