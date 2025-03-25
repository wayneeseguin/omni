package flocklogger

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

const (
	defaultMaxSize    = 10 * 1024 * 1024 // 10MB
	defaultMaxFiles   = 5
	defaultBufferSize = 4096

	// Log levels
	LevelDebug = iota
	LevelInfo
	LevelWarn
	LevelError
)

// LogFormat defines the format for log output
type LogFormat int

const (
	// FormatText outputs logs as plain text (default)
	FormatText LogFormat = iota
	// FormatJSON outputs logs as JSON objects
	FormatJSON
)

// ErrorLevel represents additional error severity levels
type ErrorLevel int

const (
	// SeverityLow for minor errors
	SeverityLow ErrorLevel = iota
	// SeverityMedium for important errors
	SeverityMedium
	// SeverityHigh for critical errors
	SeverityHigh
	// SeverityCritical for fatal errors
	SeverityCritical
)

// FormatOption defines formatting options for log outputs
type FormatOption int

const (
	// FormatOptionTimestampFormat controls timestamp format
	FormatOptionTimestampFormat FormatOption = iota
	// FormatOptionIncludeLevel controls whether to include level prefix
	FormatOptionIncludeLevel
	// FormatOptionLevelFormat controls how level is formatted
	FormatOptionLevelFormat
	// FormatOptionIncludeLocation controls whether to include file/line
	FormatOptionIncludeLocation
	// FormatOptionIndentJSON controls JSON indentation
	FormatOptionIndentJSON
	// FormatOptionFieldSeparator controls field separator in text mode
	FormatOptionFieldSeparator
	// FormatOptionTimeZone controls timezone for timestamps
	FormatOptionTimeZone
)

// LevelFormat defines level format options
type LevelFormat int

const (
	// LevelFormatName shows level as name (e.g., "INFO")
	LevelFormatName LevelFormat = iota
	// LevelFormatNameUpper shows level as uppercase name (e.g., "INFO")
	LevelFormatNameUpper
	// LevelFormatNameLower shows level as lowercase name (e.g., "info")
	LevelFormatNameLower
	// LevelFormatSymbol shows level as symbol (e.g., "I" for INFO)
	LevelFormatSymbol
)

// CompressionType defines the compression algorithm used
type CompressionType int

const (
	// CompressionNone means no compression
	CompressionNone CompressionType = iota
	// CompressionGzip uses gzip compression
	// Future compression types can be added here
)

// FilterFunc is a function that determines if a log entry should be logged
type FilterFunc func(level int, message string, fields map[string]interface{}) bool

// SamplingStrategy defines how log sampling is performed
type SamplingStrategy int

const (
	// SamplingNone disables sampling
	SamplingNone SamplingStrategy = iota
	// SamplingRandom randomly samples logs
	SamplingRandom
	// SamplingConsistent uses consistent sampling based on message content
	SamplingConsistent
	// SamplingInterval samples every Nth message
	SamplingInterval
)

// FlockLogger implements file-based logging with rotation
type FlockLogger struct {
	mu               sync.Mutex
	file             *os.File
	writer           *bufio.Writer
	path             string
	maxSize          int64
	maxFiles         int
	currentSize      int64
	level            int // minimum log level
	lockFd           int // file descriptor for file locking
	format           LogFormat
	includeTrace     bool
	stackSize        int  // size of stack trace buffer
	captureAll       bool // capture stack traces for all levels, not just errors
	formatOptions    map[FormatOption]interface{}
	compression      CompressionType
	compressMinAge   int // minimum age (in rotations) before compressing
	compressWorkers  int // number of worker goroutines for compression
	compressCh       chan string
	maxAge           time.Duration                                                         // maximum age for log files
	cleanupInterval  time.Duration                                                         // how often to check for old logs
	cleanupTicker    *time.Ticker                                                          // ticker for periodic cleanup
	cleanupDone      chan struct{}                                                         // signal for cleanup goroutine shutdown
	filters          []FilterFunc                                                          // Log filters
	samplingStrategy SamplingStrategy                                                      // Sampling strategy
	samplingRate     float64                                                               // Sampling rate (0.0-1.0 for random, >1 for interval)
	sampleCounter    uint64                                                                // Atomic counter for interval sampling
	sampleKeyFunc    func(level int, message string, fields map[string]interface{}) string // Key function for consistent sampling
}

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp  string                 `json:"timestamp"`
	Level      string                 `json:"level"`
	Message    string                 `json:"message"`
	Fields     map[string]interface{} `json:"fields,omitempty"`
	StackTrace string                 `json:"stack_trace,omitempty"`
	File       string                 `json:"file,omitempty"`
	Line       int                    `json:"line,omitempty"`
}

// NewFlockLogger creates a new file logger
func NewFlockLogger(path string) (*FlockLogger, error) {
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

	f := &FlockLogger{
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
func (f *FlockLogger) acquireLock() error {
	return unix.Flock(f.lockFd, unix.LOCK_EX)
}

// releaseLock releases the file lock
func (f *FlockLogger) releaseLock() error {
	return unix.Flock(f.lockFd, unix.LOCK_UN)
}

// SetMaxSize sets the maximum log file size
func (f *FlockLogger) SetMaxSize(size int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.maxSize = size
}

// SetMaxFiles sets the maximum number of log files
func (f *FlockLogger) SetMaxFiles(count int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.maxFiles = count
}

// SetLevel sets the minimum log level
func (f *FlockLogger) SetLevel(level int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.level = level
}

// SetFormat sets the output format (text or JSON)
func (f *FlockLogger) SetFormat(format LogFormat) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.format = format
}

// EnableStackTraces enables stack traces for error logs
func (f *FlockLogger) EnableStackTraces(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.includeTrace = enabled
}

// SetStackSize sets the maximum stack trace buffer size
func (f *FlockLogger) SetStackSize(size int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stackSize = size
}

// SetCaptureAllStacks enables stack traces for all log levels
func (f *FlockLogger) SetCaptureAllStacks(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.captureAll = enabled
}

// SetFormatOption sets a format option
func (f *FlockLogger) SetFormatOption(option FormatOption, value interface{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Validate the option value
	switch option {
	case FormatOptionTimestampFormat:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("timestamp format must be a string")
		}
	case FormatOptionIncludeLevel:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("include level must be a boolean")
		}
	case FormatOptionLevelFormat:
		if _, ok := value.(LevelFormat); !ok {
			return fmt.Errorf("level format must be a LevelFormat")
		}
	case FormatOptionIncludeLocation:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("include location must be a boolean")
		}
	case FormatOptionIndentJSON:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("indent JSON must be a boolean")
		}
	case FormatOptionFieldSeparator:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("field separator must be a string")
		}
	case FormatOptionTimeZone:
		if _, ok := value.(*time.Location); !ok {
			return fmt.Errorf("time zone must be a *time.Location")
		}
	default:
		return fmt.Errorf("unknown format option: %v", option)
	}

	f.formatOptions[option] = value
	return nil
}

// GetFormatOption gets a format option
func (f *FlockLogger) GetFormatOption(option FormatOption) interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.formatOptions[option]
}

// formatTimestamp formats a timestamp according to the current options
func (f *FlockLogger) formatTimestamp(t time.Time) string {
	format := f.formatOptions[FormatOptionTimestampFormat].(string)
	tz := f.formatOptions[FormatOptionTimeZone].(*time.Location)
	return t.In(tz).Format(format)
}

// formatLevel formats a level string according to the current options
func (f *FlockLogger) formatLevel(level int) string {
	if includeLevel, ok := f.formatOptions[FormatOptionIncludeLevel].(bool); !ok || !includeLevel {
		return ""
	}

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

	format, _ := f.formatOptions[FormatOptionLevelFormat].(LevelFormat)
	switch format {
	case LevelFormatNameUpper:
		return levelStr
	case LevelFormatNameLower:
		return strings.ToLower(levelStr)
	case LevelFormatSymbol:
		return string(levelStr[0])
	case LevelFormatName:
		return levelStr
	default:
		return levelStr
	}
}

// defaultFormatOptions returns default formatting options
func defaultFormatOptions() map[FormatOption]interface{} {
	return map[FormatOption]interface{}{
		FormatOptionTimestampFormat: "2006-01-02 15:04:05.000",
		FormatOptionIncludeLevel:    true,
		FormatOptionLevelFormat:     LevelFormatNameUpper,
		FormatOptionIncludeLocation: false,
		FormatOptionIndentJSON:      false,
		FormatOptionFieldSeparator:  " ",
		FormatOptionTimeZone:        time.Local,
	}
}

// defaultSampleKeyFunc generates a default key for consistent sampling
func defaultSampleKeyFunc(level int, message string, fields map[string]interface{}) string {
	return message // Use the message as the key by default
}

// AddFilter adds a filter function that determines whether a log entry should be logged
func (f *FlockLogger) AddFilter(filter FilterFunc) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.filters = append(f.filters, filter)
}

// ClearFilters removes all filters
func (f *FlockLogger) ClearFilters() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.filters = nil
}

// SetFieldFilter adds a filter that only logs entries containing specific field values
func (f *FlockLogger) SetFieldFilter(field string, values ...interface{}) {
	f.AddFilter(func(level int, message string, fields map[string]interface{}) bool {
		if fields == nil {
			return false
		}

		val, exists := fields[field]
		if !exists {
			return false
		}

		for _, v := range values {
			if val == v {
				return true
			}
		}
		return false
	})
}

// SetLevelFieldFilter adds a filter that only logs entries with a specific level and field value
func (f *FlockLogger) SetLevelFieldFilter(logLevel int, field string, value interface{}) {
	f.AddFilter(func(level int, message string, fields map[string]interface{}) bool {
		if level != logLevel {
			return false
		}

		if fields == nil {
			return false
		}

		val, exists := fields[field]
		return exists && val == value
	})
}

// SetRegexFilter adds a filter that only logs entries matching a regex pattern
func (f *FlockLogger) SetRegexFilter(pattern *regexp.Regexp) {
	f.AddFilter(func(level int, message string, fields map[string]interface{}) bool {
		return pattern.MatchString(message)
	})
}

// SetExcludeRegexFilter adds a filter that excludes entries matching a regex pattern
func (f *FlockLogger) SetExcludeRegexFilter(pattern *regexp.Regexp) {
	f.AddFilter(func(level int, message string, fields map[string]interface{}) bool {
		return !pattern.MatchString(message)
	})
}

// SetSampling configures log sampling
func (f *FlockLogger) SetSampling(strategy SamplingStrategy, rate float64) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.samplingStrategy = strategy

	// Validate and normalize rate
	switch strategy {
	case SamplingRandom, SamplingConsistent:
		// Ensure rate is between 0 and 1
		if rate < 0 {
			rate = 0
		} else if rate > 1 {
			rate = 1
		}
	case SamplingInterval:
		// For interval, rate is the sampling interval
		if rate < 1 {
			rate = 1 // Log every message
		}
	}

	f.samplingRate = rate
	atomic.StoreUint64(&f.sampleCounter, 0) // Reset counter when changing sampling
}

// SetSampleKeyFunc sets the function used to generate the key for consistent sampling
func (f *FlockLogger) SetSampleKeyFunc(keyFunc func(level int, message string, fields map[string]interface{}) string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if keyFunc != nil {
		f.sampleKeyFunc = keyFunc
	}
}

// shouldLog determines if a log entry should be logged based on filters and sampling
func (f *FlockLogger) shouldLog(level int, message string, fields map[string]interface{}) bool {
	// Quick check for log level
	if level < f.level {
		return false
	}

	// Apply filters
	if len(f.filters) > 0 {
		pass := false
		for _, filter := range f.filters {
			if filter(level, message, fields) {
				pass = true
				break
			}
		}
		if !pass {
			return false
		}
	}

	// Apply sampling
	switch f.samplingStrategy {
	case SamplingNone:
		return true

	case SamplingRandom:
		return rand.Float64() < f.samplingRate

	case SamplingConsistent:
		if f.samplingRate >= 1.0 {
			return true
		}

		// Use hash-based sampling for consistency
		key := f.sampleKeyFunc(level, message, fields)
		h := fnv.New32a()
		h.Write([]byte(key))
		hash := h.Sum32()
		return float64(hash%1000)/1000.0 < f.samplingRate

	case SamplingInterval:
		if f.samplingRate <= 1.0 {
			return true
		}

		counter := atomic.AddUint64(&f.sampleCounter, 1)
		return counter%uint64(f.samplingRate) == 1
	}

	return true
}

// Debug logs a debug message
func (f *FlockLogger) Debug(args ...interface{}) {
	if f.level <= LevelDebug {
		f.flocklogf("[DEBUG] %s", fmt.Sprint(args...))
	}
}

// Debugf logs a formatted debug message
func (f *FlockLogger) Debugf(format string, args ...interface{}) {
	if f.level <= LevelDebug {
		f.flocklogf("[DEBUG] %s", fmt.Sprintf(format, args...))
	}
}

// Info logs an info message
func (f *FlockLogger) Info(args ...interface{}) {
	if f.level <= LevelInfo {
		f.flocklogf("[INFO] %s", fmt.Sprint(args...))
	}
}

// Infof logs a formatted info message
func (f *FlockLogger) Infof(format string, args ...interface{}) {
	if f.level <= LevelInfo {
		f.flocklogf("[INFO] %s", fmt.Sprintf(format, args...))
	}
}

// Warn logs a warning message
func (f *FlockLogger) Warn(args ...interface{}) {
	if f.level <= LevelWarn {
		f.flocklogf("[WARN] %s", fmt.Sprint(args...))
	}
}

// Warnf logs a formatted warning message
func (f *FlockLogger) Warnf(format string, args ...interface{}) {
	if f.level <= LevelWarn {
		f.flocklogf("[WARN] %s", fmt.Sprintf(format, args...))
	}
}

// Error logs an error message
func (f *FlockLogger) Error(args ...interface{}) {
	if f.level <= LevelError {
		f.flocklogf("[ERROR] %s", fmt.Sprint(args...))
	}
}

// Errorf logs a formatted error message
func (f *FlockLogger) Errorf(format string, args ...interface{}) {
	if f.level <= LevelError {
		f.flocklogf("[ERROR] %s", fmt.Sprintf(format, args...))
	}
}

// Flog logs a formatted message directly
func (f *FlockLogger) Flog(format string, args ...interface{}) {
	f.flocklogf(format, args...)
}

// log writes a log entry with the specified level
// This method is kept for potential backward compatibility
func (f *FlockLogger) log(level int, message string) {
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

	f.flocklogf("[%s] %s", levelStr, message)
}

// flocklogf writes a formatted log entry
func (f *FlockLogger) flocklogf(format string, args ...interface{}) {
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

// rotate rotates log files
func (f *FlockLogger) rotate() error {
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

	// Update FlockLogger state
	f.file = file
	f.writer = bufio.NewWriterSize(file, defaultBufferSize)
	f.currentSize = 0

	return nil
}

// Close flushes and closes the log file
func (f *FlockLogger) Close() error {
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
func (f *FlockLogger) Flush() error {
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

// StructuredLog logs a message with structured fields
func (f *FlockLogger) StructuredLog(level int, message string, fields map[string]interface{}) {
	// Check if we should log this based on filters and sampling
	if !f.shouldLog(level, message, fields) {
		return
	}

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

	entry := LogEntry{
		Timestamp: time.Now().Format("2006-01-02 15:04:05.000"),
		Level:     levelStr,
		Message:   message,
		Fields:    fields,
	}

	// Add file and line information
	if f.includeTrace || level == LevelError {
		_, file, line, ok := runtime.Caller(2)
		if ok {
			entry.File = file
			entry.Line = line
		}

		if (level == LevelError && f.includeTrace) || f.captureAll {
			// Capture stack trace for errors or when capturing all is enabled
			buf := make([]byte, f.stackSize)
			n := runtime.Stack(buf, false)
			entry.StackTrace = string(buf[:n])
		}
	}

	f.writeLogEntry(entry)
}

// writeLogEntry writes the log entry to the file
func (f *FlockLogger) writeLogEntry(entry LogEntry) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Format timestamp according to options
	entry.Timestamp = f.formatTimestamp(time.Now())

	var entryBytes []byte
	var err error

	// Format the log entry based on the selected format
	if f.format == FormatJSON {
		if indent, _ := f.formatOptions[FormatOptionIndentJSON].(bool); indent {
			entryBytes, err = json.MarshalIndent(entry, "", "  ")
		} else {
			entryBytes, err = json.Marshal(entry)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to marshal log entry to JSON: %v\n", err)
			return
		}
		entryBytes = append(entryBytes, '\n')
	} else {
		// Text format
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("[%s]", entry.Timestamp))

		if includeLevel, _ := f.formatOptions[FormatOptionIncludeLevel].(bool); includeLevel {
			sb.WriteString(fmt.Sprintf(" [%s]", entry.Level))
		}

		sb.WriteString(fmt.Sprintf(" %s", entry.Message))

		sep, _ := f.formatOptions[FormatOptionFieldSeparator].(string)
		if len(entry.Fields) > 0 {
			for k, v := range entry.Fields {
				sb.WriteString(fmt.Sprintf("%s%s=%v", sep, k, v))
			}
		}

		if includeLocation, _ := f.formatOptions[FormatOptionIncludeLocation].(bool); includeLocation && entry.File != "" {
			sb.WriteString(fmt.Sprintf("%sfile=%s:%d", sep, entry.File, entry.Line))
		}

		if entry.StackTrace != "" {
			sb.WriteString(fmt.Sprintf("\nStack Trace:\n%s", entry.StackTrace))
		}

		sb.WriteString("\n")
		entryBytes = []byte(sb.String())
	}

	// Acquire file lock for cross-process safety
	if err := f.acquireLock(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to acquire file lock: %v\n", err)
		return
	}
	defer f.releaseLock()

	// Check if rotation needed
	entrySize := int64(len(entryBytes))
	if f.currentSize+entrySize > f.maxSize {
		if err := f.rotate(); err != nil {
			// If rotation fails, try to write error to file
			now := time.Now().Format("2006-01-02 15:04:05.000")
			fmt.Fprintf(f.writer, "[%s] ERROR: Failed to rotate log file: %v\n", now, err)
			f.writer.Flush()
			return
		}
	}

	// Write entry
	if _, err := f.writer.Write(entryBytes); err != nil {
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

// DebugWithFields logs a debug message with structured fields
func (f *FlockLogger) DebugWithFields(message string, fields map[string]interface{}) {
	f.StructuredLog(LevelDebug, message, fields)
}

// InfoWithFields logs an info message with structured fields
func (f *FlockLogger) InfoWithFields(message string, fields map[string]interface{}) {
	f.StructuredLog(LevelInfo, message, fields)
}

// WarnWithFields logs a warning message with structured fields
func (f *FlockLogger) WarnWithFields(message string, fields map[string]interface{}) {
	f.StructuredLog(LevelWarn, message, fields)
}

// ErrorWithFields logs an error message with structured fields
func (f *FlockLogger) ErrorWithFields(message string, fields map[string]interface{}) {
	f.StructuredLog(LevelError, message, fields)
}

// ErrorWithError logs an error with stack trace
func (f *FlockLogger) ErrorWithError(message string, err error) {
	if f.level > LevelError {
		return
	}

	fields := map[string]interface{}{
		"error": err.Error(),
	}

	// Check if it's a wrapped error from pkg/errors
	if stackTracer, ok := err.(interface{ StackTrace() errors.StackTrace }); ok && f.includeTrace {
		stack := fmt.Sprintf("%+v", stackTracer.StackTrace())
		fields["stack_trace"] = stack
	}

	f.StructuredLog(LevelError, message, fields)
}

// ErrorWithErrorAndSeverity logs an error with stack trace and severity level
func (f *FlockLogger) ErrorWithErrorAndSeverity(message string, err error, severity ErrorLevel) {
	if f.level > LevelError {
		return
	}

	fields := map[string]interface{}{
		"error":    err.Error(),
		"severity": severityToString(severity),
	}

	// Check if it's a wrapped error from pkg/errors
	if stackTracer, ok := err.(interface{ StackTrace() errors.StackTrace }); ok && f.includeTrace {
		stack := fmt.Sprintf("%+v", stackTracer.StackTrace())
		fields["stack_trace"] = stack
	}

	f.StructuredLog(LevelError, message, fields)
}

// severityToString converts severity level to string
func severityToString(severity ErrorLevel) string {
	switch severity {
	case SeverityLow:
		return "low"
	case SeverityMedium:
		return "medium"
	case SeverityHigh:
		return "high"
	case SeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// WrapError wraps an error with stack trace information
func (f *FlockLogger) WrapError(err error, message string) error {
	return errors.Wrap(err, message)
}

// WrapErrorWithSeverity wraps an error with stack trace information and severity
func (f *FlockLogger) WrapErrorWithSeverity(err error, message string, severity ErrorLevel) error {
	wrapped := errors.Wrap(err, message)
	// Store severity in context or return a custom error type if needed
	// For simplicity, we'll just log it immediately
	f.ErrorWithErrorAndSeverity(message, wrapped, severity)
	return wrapped
}

// CauseOf returns the root cause of an error
func (f *FlockLogger) CauseOf(err error) error {
	return errors.Cause(err)
}

// WithStack attaches a stack trace to an error
func (f *FlockLogger) WithStack(err error) error {
	return errors.WithStack(err)
}

// IsErrorType checks if an error is of a specific type (when using errors.Is)
func (f *FlockLogger) IsErrorType(err, target error) bool {
	return errors.Is(err, target)
}

// FormatErrorVerbose returns a detailed error representation with stack trace
func (f *FlockLogger) FormatErrorVerbose(err error) string {
	if stackTracer, ok := err.(interface{ StackTrace() errors.StackTrace }); ok {
		return fmt.Sprintf("%+v\n%+v", err, stackTracer.StackTrace())
	}
	return fmt.Sprintf("%+v", err)
}

// LogPanic logs the error and stack trace for a recovered panic
func (f *FlockLogger) LogPanic(recovered interface{}) {
	// Capture the stack trace
	buf := make([]byte, f.stackSize)
	n := runtime.Stack(buf, false)
	stackTrace := string(buf[:n])

	var errMsg string
	switch v := recovered.(type) {
	case error:
		errMsg = v.Error()
	default:
		errMsg = fmt.Sprintf("%v", v)
	}

	fields := map[string]interface{}{
		"panic":       true,
		"stack_trace": stackTrace,
	}

	f.StructuredLog(LevelError, fmt.Sprintf("Recovered from panic: %s", errMsg), fields)
}

// SafeGo runs a function in a goroutine with panic recovery
func (f *FlockLogger) SafeGo(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				f.LogPanic(r)
			}
		}()
		fn()
	}()
}

// Sensitive patterns to redact
var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`("auth_token"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("password"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("secret"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("key"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("private_key"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("token"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`(Bearer\s+)[A-Za-z0-9-._~+/]+=*`),
}

// FlogRequest logs an API request safely
func (f *FlockLogger) FlogRequest(method, path string, headers map[string][]string, body string) {
	safeHeaders := make(map[string][]string)

	for k, v := range headers {
		// Skip sensitive headers
		if strings.ToLower(k) == "authorization" ||
			strings.Contains(strings.ToLower(k), "token") ||
			strings.Contains(strings.ToLower(k), "key") {
			safeHeaders[k] = []string{"[REDACTED]"}
			continue
		}
		safeHeaders[k] = v
	}

	f.Flog("API Request: %s %s\nHeaders: %v\nBody: %s",
		method, path, safeHeaders, f.redactSensitive(body))
}

// FlogResponse logs an API response safely
func (f *FlockLogger) FlogResponse(statusCode int, headers map[string][]string, body string) {
	safeHeaders := make(map[string][]string)

	for k, v := range headers {
		// Skip sensitive headers
		if strings.Contains(strings.ToLower(k), "token") ||
			strings.Contains(strings.ToLower(k), "key") {
			safeHeaders[k] = []string{"[REDACTED]"}
			continue
		}
		safeHeaders[k] = v
	}

	f.Flog("API Response: Status: %d\nHeaders: %v\nBody: %s",
		statusCode, safeHeaders, f.redactSensitive(body))
}

// redactSensitive replaces sensitive information with [REDACTED]
func (f *FlockLogger) redactSensitive(input string) string {
	if input == "" {
		return input
	}

	result := input

	for _, pattern := range sensitivePatterns {
		result = pattern.ReplaceAllString(result, "${1}\"[REDACTED]\"")
	}

	return result
}

// SetCompression enables or disables compression for rotated log files
func (f *FlockLogger) SetCompression(compressionType CompressionType) {
	f.mu.Lock()
	defer f.mu.Unlock()

	previousType := f.compression
	f.compression = compressionType

	// If we're enabling compression and it wasn't enabled before
	if f.compression != CompressionNone && previousType == CompressionNone {
		f.startCompressionWorkers()
	} else if f.compression == CompressionNone && previousType != CompressionNone {
		f.stopCompressionWorkers()
	}
}

// SetCompressMinAge sets the minimum rotation age before compressing logs
func (f *FlockLogger) SetCompressMinAge(age int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.compressMinAge = age
}

// SetCompressWorkers sets the number of compression worker goroutines
func (f *FlockLogger) SetCompressWorkers(workers int) {
	if workers < 1 {
		workers = 1
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// Only update if compression is enabled
	if f.compression != CompressionNone {
		oldWorkers := f.compressWorkers
		f.compressWorkers = workers

		// Restart workers with new count
		f.stopCompressionWorkers()
		f.startCompressionWorkers()
	} else {
		f.compressWorkers = workers
	}
}

// startCompressionWorkers starts background goroutines for compression
func (f *FlockLogger) startCompressionWorkers() {
	// Create channel for compression jobs
	f.compressCh = make(chan string, 100)

	// Start worker goroutines
	for i := 0; i < f.compressWorkers; i++ {
		go func() {
			for path := range f.compressCh {
				if err := f.compressFile(path); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to compress file %s: %v\n", path, err)
				}
			}
		}()
	}
}

// stopCompressionWorkers stops the compression goroutines
func (f *FlockLogger) stopCompressionWorkers() {
	if f.compressCh != nil {
		close(f.compressCh)
		f.compressCh = nil
	}
}

// compressFile compresses the given file using the configured compression type
func (f *FlockLogger) compressFile(path string) error {
	if f.compression == CompressionNone {
		return nil
	}

	switch f.compression {
	case CompressionGzip:
		return f.compressFileGzip(path)
	default:
		return fmt.Errorf("unsupported compression type: %v", f.compression)
	}
}

// compressFileGzip compresses a file using gzip
func (f *FlockLogger) compressFileGzip(path string) error {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // file doesn't exist, nothing to compress
	}

	// Compressed file path
	compressedPath := path + ".gz"

	// Open source file
	src, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening source file for compression: %w", err)
	}
	defer src.Close()

	// Create destination file
	dst, err := os.OpenFile(compressedPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("creating compressed file: %w", err)
	}

	// Create gzip writer
	gw := gzip.NewWriter(dst)
	defer gw.Close()

	// Copy data from source to compressed destination
	_, err = io.Copy(gw, src)
	if err != nil {
		dst.Close()
		return fmt.Errorf("compressing file: %w", err)
	}

	// Close both files explicitly before removing the original
	gw.Close()
	dst.Close()
	src.Close()

	// Remove the original file
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("removing original file after compression: %w", err)
	}

	return nil
}

// queueForCompression adds a file to the compression queue
func (f *FlockLogger) queueForCompression(path string) {
	if f.compression != CompressionNone && f.compressCh != nil {
		select {
		case f.compressCh <- path:
			// Successfully queued
		default:
			// Queue full, log to stderr
			fmt.Fprintf(os.Stderr, "Compression queue full, skipping compression for %s\n", path)
		}
	}
}

// SetMaxAge sets the maximum age for log files
// Logs older than this will be deleted during cleanup
// Use 0 to disable age-based cleanup
func (f *FlockLogger) SetMaxAge(duration time.Duration) {
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
func (f *FlockLogger) SetCleanupInterval(interval time.Duration) {
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
func (f *FlockLogger) startCleanupRoutine() {
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
func (f *FlockLogger) stopCleanupRoutine() {
	if f.cleanupTicker == nil {
		return
	}

	f.cleanupTicker.Stop()
	close(f.cleanupDone)
	f.cleanupTicker = nil
	f.cleanupDone = nil
}

// cleanupOldLogs removes log files older than maxAge
func (f *FlockLogger) cleanupOldLogs() error {
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
func (f *FlockLogger) RunCleanup() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cleanupOldLogs()
}
