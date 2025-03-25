package flexlog

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// rotate rotates log files
func (f *FlexLog) rotate() error {
	// Lock already acquired in flocklogf

	// Flush current file
	if err := f.writer.Flush(); err != nil {
		return fmt.Errorf("flushing current log: %w", err)
	}

	// Close current file
	if err := f.file.Close(); err != nil {
		return fmt.Errorf("closing current log: %w", err)
	}

	// Rotate existing files
	for i := f.maxFiles - 1; i > 0; i-- {
		oldPath := fmt.Sprintf("%s.%d", f.path, i)
		newPath := fmt.Sprintf("%s.%d", f.path, i+1)

		// Remove oldest file if it exists
		if i == f.maxFiles-1 {
			// Check if we need to remove a compressed version
			compressedPath := newPath + ".gz"
			if _, err := os.Stat(compressedPath); err == nil {
				os.Remove(compressedPath)
			} else {
				os.Remove(newPath) // Remove uncompressed version if it exists
			}

			// Move the previous one up
			if _, err := os.Stat(oldPath); err == nil {
				if err := os.Rename(oldPath, newPath); err != nil {
					return fmt.Errorf("rotating log file: %w", err)
				}

				// Queue for compression if it's old enough
				if i+1 >= f.compressMinAge && f.compression != CompressionNone {
					f.queueForCompression(newPath)
				}
			} else {
				// Check if we have a compressed version to rotate
				compressedOldPath := oldPath + ".gz"
				if _, err := os.Stat(compressedOldPath); err == nil {
					compressedNewPath := newPath + ".gz"
					if err := os.Rename(compressedOldPath, compressedNewPath); err != nil {
						return fmt.Errorf("rotating compressed log file: %w", err)
					}
				}
			}
			continue
		}

		// Handle both compressed and uncompressed files
		if _, err := os.Stat(oldPath); err == nil {
			if err := os.Rename(oldPath, newPath); err != nil {
				return fmt.Errorf("rotating log file: %w", err)
			}

			// Queue for compression if it's old enough
			if i+1 >= f.compressMinAge && f.compression != CompressionNone {
				f.queueForCompression(newPath)
			}
		} else {
			// Check for compressed version
			compressedOldPath := oldPath + ".gz"
			if _, err := os.Stat(compressedOldPath); err == nil {
				compressedNewPath := newPath + ".gz"
				if err := os.Rename(compressedOldPath, compressedNewPath); err != nil {
					return fmt.Errorf("rotating compressed log file: %w", err)
				}
			}
		}
	}

	// Rename current file
	if err := os.Rename(f.path, fmt.Sprintf("%s.1", f.path)); err != nil {
		return fmt.Errorf("rotating current log: %w", err)
	}

	// Queue the just-rotated file for compression if needed
	if f.compressMinAge <= 1 && f.compression != CompressionNone {
		f.queueForCompression(fmt.Sprintf("%s.1", f.path))
	}

	// Open new file
	file, err := os.OpenFile(f.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("creating new log file: %w", err)
	}

	// Update FlexLog state
	f.file = file
	f.writer = bufio.NewWriterSize(file, defaultBufferSize)
	f.currentSize = 0

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

	go func() {
		for {
			select {
			case <-f.cleanupTicker.C:
				if err := f.cleanupOldLogs(); err != nil {
					fmt.Fprintf(os.Stderr, "Error cleaning up old logs: %v\n", err)
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
	close(f.cleanupDone)
	f.cleanupTicker = nil
	f.cleanupDone = nil
}

// cleanupOldLogs removes log files older than maxAge
func (f *FlexLog) cleanupOldLogs() error {
	if f.maxAge == 0 {
		return nil // Age-based cleanup disabled
	}

	// Try to acquire lock, but don't block for too long
	locked := make(chan bool, 1)
	go func() {
		f.mu.Lock()
		locked <- true
	}()

	// Wait for lock with timeout
	select {
	case <-locked:
		defer f.mu.Unlock()
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timed out waiting for lock in cleanupOldLogs")
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

	// Match patterns for both normal and compressed log files
	pattern := regexp.MustCompile(fmt.Sprintf(`^%s(\.\d+)?(?:\.gz)?$`, regexp.QuoteMeta(base)))

	for _, file := range files {
		// Skip directories
		if file.IsDir() {
			continue
		}

		// Check if this file matches our pattern
		if !pattern.MatchString(file.Name()) {
			continue
		}

		// Skip the current active log file
		if file.Name() == base {
			continue
		}

		filePath := filepath.Join(dir, file.Name())

		// Get file info for timestamp
		info, err := file.Info()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting file info for %s: %v\n", filePath, err)
			continue
		}

		// Check if file is older than cutoff
		if info.ModTime().Before(cutoff) {
			// Remove the file
			if err := os.Remove(filePath); err != nil {
				fmt.Fprintf(os.Stderr, "Error removing old log file %s: %v\n", filePath, err)
			} else {
				fmt.Fprintf(os.Stderr, "Removed old log file: %s (age: %v)\n",
					filePath, time.Since(info.ModTime()))
			}
		}
	}

	return nil
}

// RunCleanup immediately runs the cleanup process for old log files
func (f *FlexLog) RunCleanup() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cleanupOldLogs()
}
