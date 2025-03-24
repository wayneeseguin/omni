package flocklogger

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	defaultMaxSize    = 10 * 1024 * 1024 // 10MB
	defaultMaxFiles   = 5
	defaultBufferSize = 4096
)

// FlockLogger implements file-based logging with rotation
type FlockLogger struct {
	mu          sync.Mutex
	file        *os.File
	writer      *bufio.Writer
	path        string
	maxSize     int64
	maxFiles    int
	currentSize int64
}

// NewFlockLogger creates a new file logger
func NewFlockLogger(path string) (*FlockLogger, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating log directory: %w", err)
	}

	// Open log file
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening log file: %w", err)
	}

	// Get current file size
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("getting file info: %w", err)
	}

	f := &FlockLogger{
		file:        file,
		writer:      bufio.NewWriterSize(file, defaultBufferSize),
		path:        path,
		maxSize:     defaultMaxSize,
		maxFiles:    defaultMaxFiles,
		currentSize: info.Size(),
	}

	return f, nil
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

// Flog writes a formatted log entry
func (f *FlockLogger) Flog(format string, args ...interface{}) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Format log entry
	now := time.Now()
	entry := fmt.Sprintf("[%s] %s\n",
		now.Format("2006-01-02 15:04:05.000"),
		fmt.Sprintf(format, args...))

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
			os.Remove(newPath) // Remove the oldest file that would exceed maxFiles
			if _, err := os.Stat(oldPath); err == nil {
				if err := os.Rename(oldPath, newPath); err != nil {
					return fmt.Errorf("rotating log file: %w", err)
				}
			}
			continue
		}

		// Rename existing files
		if _, err := os.Stat(oldPath); err == nil {
			if err := os.Rename(oldPath, newPath); err != nil {
				return fmt.Errorf("rotating log file: %w", err)
			}
		}
	}

	// Rename current file
	if err := os.Rename(f.path, fmt.Sprintf("%s.1", f.path)); err != nil {
		return fmt.Errorf("rotating current log: %w", err)
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

	if f.writer == nil || f.file == nil {
		return nil // Already closed or nil
	}

	if err := f.writer.Flush(); err != nil {
		return fmt.Errorf("flushing log: %w", err)
	}

	if err := f.file.Close(); err != nil {
		return fmt.Errorf("closing log file: %w", err)
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

	return f.writer.Flush()
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
