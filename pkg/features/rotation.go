package features

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"
)

// RotationTimeFormat is the timestamp format used for rotated log files.
// The format is sortable and includes millisecond precision to avoid collisions.
// Example: "20060102-150405.000" produces "20240115-143052.123"
const RotationTimeFormat = "20060102-150405.000"

// RotationManager handles log file rotation and cleanup
type RotationManager struct {
	mu              sync.RWMutex
	maxAge          time.Duration
	maxFiles        int
	cleanupInterval time.Duration
	cleanupTicker   *time.Ticker
	cleanupDone     chan struct{}
	cleanupWg       sync.WaitGroup
	errorHandler    func(source, dest, msg string, err error)
	metricsHandler  func(string) // Function to track rotation metrics

	// Compression callback
	compressionCallback func(path string)

	// Current log paths being managed
	logPaths []string
	pathsMu  sync.RWMutex
}

// NewRotationManager creates a new rotation manager
func NewRotationManager() *RotationManager {
	return &RotationManager{
		cleanupInterval: time.Hour, // Default 1 hour cleanup interval
	}
}

// SetErrorHandler sets the error handling function
func (r *RotationManager) SetErrorHandler(handler func(source, dest, msg string, err error)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errorHandler = handler
}

// SetCompressionCallback sets the callback function for queuing files for compression
func (r *RotationManager) SetCompressionCallback(callback func(path string)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.compressionCallback = callback
}

// SetMetricsHandler sets the metrics tracking function
func (r *RotationManager) SetMetricsHandler(handler func(string)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metricsHandler = handler
}

// AddLogPath adds a log path to be managed by this rotation manager
func (r *RotationManager) AddLogPath(path string) {
	r.pathsMu.Lock()
	defer r.pathsMu.Unlock()

	// Check if path already exists
	for _, existing := range r.logPaths {
		if existing == path {
			return
		}
	}
	r.logPaths = append(r.logPaths, path)
}

// RemoveLogPath removes a log path from management
func (r *RotationManager) RemoveLogPath(path string) {
	r.pathsMu.Lock()
	defer r.pathsMu.Unlock()

	for i, existing := range r.logPaths {
		if existing == path {
			r.logPaths = append(r.logPaths[:i], r.logPaths[i+1:]...)
			return
		}
	}
}

// SetMaxAge sets the maximum age for log files
func (r *RotationManager) SetMaxAge(duration time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.maxAge = duration

	// Start or stop the cleanup process based on the new setting
	if r.maxAge > 0 {
		r.startCleanupRoutine()
	} else if r.maxAge == 0 && r.cleanupTicker != nil {
		r.stopCleanupRoutine()
	}
	return nil
}

// SetMaxFiles sets the maximum number of rotated files to keep
func (r *RotationManager) SetMaxFiles(count int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.maxFiles = count
}

// SetCleanupInterval sets how often to check for and remove old log files
func (r *RotationManager) SetCleanupInterval(interval time.Duration) {
	if interval < time.Minute {
		interval = time.Minute // Enforce a reasonable minimum
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Only update if cleanup is already running
	if r.cleanupTicker != nil {
		r.stopCleanupRoutine()
		r.cleanupInterval = interval
		r.startCleanupRoutine()
	} else {
		r.cleanupInterval = interval
	}
}

// startCleanupRoutine starts the background goroutine for age-based log file cleanup
func (r *RotationManager) startCleanupRoutine() {
	// Don't start if already running or max age is 0
	if r.cleanupTicker != nil || r.maxAge == 0 {
		return
	}

	r.cleanupTicker = time.NewTicker(r.cleanupInterval)
	r.cleanupDone = make(chan struct{})

	r.cleanupWg.Add(1)
	go func() {
		defer r.cleanupWg.Done()
		defer func() {
			if p := recover(); p != nil {
				if r.errorHandler != nil {
					r.errorHandler("cleanup", "", "Panic in cleanup routine", fmt.Errorf("%v", p))
				}
			}
		}()

		for {
			select {
			case <-r.cleanupTicker.C:
				// Get copy of log paths to clean up
				r.pathsMu.RLock()
				paths := make([]string, len(r.logPaths))
				copy(paths, r.logPaths)
				r.pathsMu.RUnlock()

				// Clean up each managed log path
				for _, path := range paths {
					if err := r.RunCleanup(path); err != nil && r.errorHandler != nil {
						r.errorHandler("cleanup", path, "Failed to run cleanup", err)
					}
				}
			case <-r.cleanupDone:
				return
			}
		}
	}()
}

// stopCleanupRoutine stops the background cleanup goroutine
func (r *RotationManager) stopCleanupRoutine() {
	if r.cleanupTicker == nil {
		return
	}

	r.cleanupTicker.Stop()
	if r.cleanupDone != nil {
		close(r.cleanupDone)
	}

	// Wait for the cleanup goroutine to finish
	r.cleanupWg.Wait()

	r.cleanupTicker = nil
	r.cleanupDone = nil
}

// Start starts the rotation manager
func (r *RotationManager) Start() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.maxAge > 0 {
		r.startCleanupRoutine()
	}
}

// Stop stops the rotation manager
func (r *RotationManager) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopCleanupRoutine()
}

// RotateFile rotates a log file by renaming it with a timestamp suffix
func (r *RotationManager) RotateFile(path string, writer *bufio.Writer) (string, error) {
	// Flush the writer if provided
	if writer != nil {
		if err := writer.Flush(); err != nil {
			return "", fmt.Errorf("flushing log: %w", err)
		}
	}

	// Clean the path to prevent directory traversal
	cleanPath := filepath.Clean(path)

	// Generate timestamp for rotation (always use UTC for consistency)
	timestamp := time.Now().UTC().Format(RotationTimeFormat)
	rotatedPath := fmt.Sprintf("%s.%s", cleanPath, timestamp)

	// Rename the current file
	if err := os.Rename(cleanPath, rotatedPath); err != nil {
		return "", fmt.Errorf("rotating log: %w", err)
	}

	// Queue for compression if callback is set
	r.mu.RLock()
	compressionCallback := r.compressionCallback
	metricsHandler := r.metricsHandler
	r.mu.RUnlock()

	if compressionCallback != nil {
		compressionCallback(rotatedPath)
	}

	// Track rotation metric
	if metricsHandler != nil {
		metricsHandler("rotation_completed")
	}

	return rotatedPath, nil
}

// CleanupOldLogs removes log files older than maxAge.
func (r *RotationManager) CleanupOldLogs(logPath string) error {
	r.mu.RLock()
	maxAge := r.maxAge
	r.mu.RUnlock()
	if maxAge == 0 {
		return nil // Age-based cleanup disabled
	}

	// Check if we have a valid path
	if logPath == "" {
		return nil // No primary log file to clean up
	}

	// Get the directory and pattern for log files
	dir := filepath.Dir(logPath)
	base := filepath.Base(logPath)

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
			if r.errorHandler != nil {
				r.errorHandler("cleanup", file.Name(), "Error parsing timestamp", err)
			}
			continue
		}

		// Check if file is older than cutoff
		// Using the timestamp when the file was rotated (from filename)
		if time.Since(fileTime) > maxAge {
			// Remove the file
			if err := os.Remove(filePath); err != nil {
				if r.errorHandler != nil {
					r.errorHandler("cleanup", filePath, "Failed to remove old log file", err)
				}
			} else {
				// Track cleanup metric
				if r.metricsHandler != nil {
					r.metricsHandler("cleanup_completed")
				}
			}
		}
	}

	return nil
}

// CleanupOldFiles removes old rotated files based on maxFiles count.
func (r *RotationManager) CleanupOldFiles(logPath string) error {
	r.mu.RLock()
	maxFiles := r.maxFiles
	r.mu.RUnlock()

	if maxFiles <= 0 {
		return nil // No file count limit
	}

	// Get the directory and base name
	dir := filepath.Dir(logPath)
	base := filepath.Base(logPath)

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
	if len(logFiles) > maxFiles {
		for i := maxFiles; i < len(logFiles); i++ {
			if err := os.Remove(logFiles[i].path); err != nil {
				if r.errorHandler != nil {
					r.errorHandler("cleanup", logFiles[i].path, "Failed to remove old log file (exceeded maxFiles)", err)
				}
			} else {
				// Track cleanup metric
				if r.metricsHandler != nil {
					r.metricsHandler("cleanup_completed")
				}
			}
		}
	}

	return nil
}

// RunCleanup immediately runs the cleanup process for old log files
func (r *RotationManager) RunCleanup(logPath string) error {
	if err := r.CleanupOldLogs(logPath); err != nil {
		return err
	}
	return r.CleanupOldFiles(logPath)
}

// GetRotatedFiles returns a list of rotated files for the given log path
func (r *RotationManager) GetRotatedFiles(logPath string) ([]RotatedFileInfo, error) {
	dir := filepath.Dir(logPath)
	base := filepath.Base(logPath)

	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading log directory: %w", err)
	}

	// Match patterns for timestamp-based log files
	pattern := regexp.MustCompile(fmt.Sprintf(`^%s\.(\d{8}-\d{6}\.\d{3})(?:\.gz)?$`, regexp.QuoteMeta(base)))

	var rotatedFiles []RotatedFileInfo
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		matches := pattern.FindStringSubmatch(file.Name())
		if len(matches) != 2 {
			continue
		}

		filePath := filepath.Join(dir, file.Name())
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			continue
		}

		fileTime, err := time.Parse(RotationTimeFormat, matches[1])
		if err != nil {
			continue
		}

		rotatedFiles = append(rotatedFiles, RotatedFileInfo{
			Path:         filePath,
			Name:         file.Name(),
			Size:         fileInfo.Size(),
			RotationTime: fileTime,
			IsCompressed: filepath.Ext(file.Name()) == ".gz",
		})
	}

	// Sort by rotation time (newest first)
	sort.Slice(rotatedFiles, func(i, j int) bool {
		return rotatedFiles[i].RotationTime.After(rotatedFiles[j].RotationTime)
	})

	return rotatedFiles, nil
}

// RotatedFileInfo contains information about a rotated log file
type RotatedFileInfo struct {
	Path         string    `json:"path"`
	Name         string    `json:"name"`
	Size         int64     `json:"size"`
	RotationTime time.Time `json:"rotation_time"`
	IsCompressed bool      `json:"is_compressed"`
}

// GetStatus returns the current status of the rotation manager
func (r *RotationManager) GetStatus() RotationStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return RotationStatus{
		MaxAge:          r.maxAge,
		MaxFiles:        r.maxFiles,
		CleanupInterval: r.cleanupInterval,
		IsRunning:       r.cleanupTicker != nil,
	}
}

// RotationStatus represents the status of the rotation manager
type RotationStatus struct {
	MaxAge          time.Duration `json:"max_age"`
	MaxFiles        int           `json:"max_files"`
	CleanupInterval time.Duration `json:"cleanup_interval"`
	IsRunning       bool          `json:"is_running"`
}

// GetMaxAge returns the maximum age for log files
func (r *RotationManager) GetMaxAge() time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.maxAge
}

// GetMaxFiles returns the maximum number of files to keep
func (r *RotationManager) GetMaxFiles() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.maxFiles
}

// GetCleanupInterval returns the cleanup interval
func (r *RotationManager) GetCleanupInterval() time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cleanupInterval
}

// IsRunning returns whether the cleanup routine is running
func (r *RotationManager) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cleanupTicker != nil
}
