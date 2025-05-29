// Package flexlog provides a flexible, high-performance logging library for Go applications.
// It supports multiple concurrent destinations, structured logging, log rotation, compression,
// filtering, sampling, and process-safe file logging using Unix file locks.
//
// Example:
//
//	logger, err := flexlog.NewFlexLog()
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer logger.Close()
//
//	logger.Info("Application started", "version", "1.0.0")
package flexlog

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// defaultChannelSize is the default buffer size for the message channel
// initialized from environment variable FLEXLOG_CHANNEL_SIZE or defaults to 100
var defaultChannelSize = getDefaultChannelSize()

// Filter is a function that determines if a message should be logged.
// It receives the log level, message, and fields, and returns true if the message should be logged.
type Filter func(level int, message string, fields map[string]interface{}) bool

// getDefaultChannelSize retrieves the default channel size from an environment variable or uses the default value
func getDefaultChannelSize() int {
	if value, exists := os.LookupEnv("FLEXLOG_CHANNEL_SIZE"); exists {
		if size, err := strconv.Atoi(value); err == nil && size > 0 {
			return size
		}
	}
	return 100 // Default to 100 if not specified in environment
}

// isTestMode detects if we're running under go test
func isTestMode() bool {
	// Check if we're running under go test
	if exe, err := os.Executable(); err == nil {
		if filepath.Base(exe) == "go" || strings.Contains(exe, ".test") {
			return true
		}
	}
	
	// Check for testing package import via command line
	for _, arg := range os.Args {
		if strings.Contains(arg, "test") {
			return true
		}
	}
	
	return false
}

// New creates a new file logger with default settings.
// The logger uses file locking (flock) for process-safe concurrent writes.
//
// Parameters:
//   - path: The file path where logs will be written
//
// Returns:
//   - *FlexLog: The logger instance
//   - error: Any error encountered during creation
//
// Example:
//
//	logger, err := flexlog.New("/var/log/app.log")
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer logger.Close()
func New(path string) (*FlexLog, error) {
	// For backward compatibility, we treat this as a file-based logger
	// with flock backend by default
	return NewWithBackend(path, BackendFlock)
}

// NewWithBackend creates a new logger with specific backend type.
//
// Parameters:
//   - uri: The destination URI (file path for file backend, syslog address for syslog backend)
//   - backendType: The backend type (BackendFlock or BackendSyslog)
//
// Returns:
//   - *FlexLog: The logger instance
//   - error: Any error encountered during creation
func NewWithBackend(uri string, backendType int) (*FlexLog, error) {
	// Get the channel size from environment or use default
	channelSize := getDefaultChannelSize()
	formatOptions := defaultFormatOptions()

	// Create a new instance with default settings
	f := &FlexLog{
		maxSize:          defaultMaxSize,
		maxFiles:         defaultMaxFiles,
		level:            LevelInfo,  // Default to Info level
		format:           FormatText, // Default to text format
		includeTrace:     false,
		stackSize:        4096, // Default stack trace buffer size
		captureAll:       false,
		formatOptions:    formatOptions,
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
		msgChan:          make(chan LogMessage, channelSize),
		channelSize:      channelSize,
		Destinations:     make([]*Destination, 0),
		messageQueue:     make(chan *LogMessage, channelSize),
	}

	// Message level counters will be lazily initialized on first use

	// Add the destination based on backend type
	dest, err := f.createDestination(uri, backendType)
	if err != nil {
		return nil, err
	}

	// Set as default destination
	f.defaultDest = dest
	f.Destinations = append(f.Destinations, dest)

	// If it's a file backend, set the file and writer references at the logger level too
	// for backward compatibility
	if backendType == BackendFlock {
		f.file = dest.File
		f.writer = dest.Writer
		f.path = uri
		f.fileLock = dest.Lock
		f.currentSize = dest.Size
		f.size = dest.Size
	}

	// Set default error handler (silent during tests to avoid noisy output)
	if isTestMode() {
		f.errorHandler = SilentErrorHandler
	} else {
		f.errorHandler = StderrErrorHandler
	}

	// Metrics sync.Map fields will be lazily initialized on first use

	// Start the single message dispatcher
	f.workerWg.Add(1)
	f.workerStarted = true
	go f.messageDispatcher()

	return f, nil
}

// messageDispatcher is the single background goroutine that processes all messages
func (f *FlexLog) messageDispatcher() {
	defer f.workerWg.Done()

	for msg := range f.msgChan {
		// Track that we've successfully received a message
		if msg.Entry != nil {
			// Convert level string to int
			levelInt := LevelInfo // default
			switch msg.Entry.Level {
			case "DEBUG":
				levelInt = LevelDebug
			case "INFO":
				levelInt = LevelInfo
			case "WARN":
				levelInt = LevelWarn
			case "ERROR":
				levelInt = LevelError
			}
			f.trackMessageLogged(levelInt)
		} else {
			// For non-structured messages, use the level from LogMessage
			f.trackMessageLogged(msg.Level)
		}

		// Send to all enabled destinations
		f.mu.RLock()
		destinations := make([]*Destination, len(f.Destinations))
		copy(destinations, f.Destinations)
		f.mu.RUnlock()

		for _, dest := range destinations {
			// Skip disabled destinations
			dest.mu.RLock()
			enabled := dest.Enabled
			dest.mu.RUnlock()
			if !enabled {
				continue
			}

			// Process the message for this destination
			f.processMessage(msg, dest)
		}
	}
}

// IsClosed returns true if the logger has been closed
func (f *FlexLog) IsClosed() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.closed
}

// SetMaxSize sets the maximum log file size
func (f *FlexLog) SetMaxSize(size int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.maxSize = size
}

// GetMaxSize returns the maximum log file size (thread-safe)
func (f *FlexLog) GetMaxSize() int64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.maxSize
}

// SetMaxFiles sets the maximum number of log files
func (f *FlexLog) SetMaxFiles(count int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.maxFiles = count
}

// GetMaxFiles returns the maximum number of log files
func (f *FlexLog) GetMaxFiles() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.maxFiles
}

// SetGlobalFields sets global fields that will be included in all log entries
func (f *FlexLog) SetGlobalFields(fields map[string]interface{}) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.globalFields = fields
}

// AddGlobalField adds a single global field
func (f *FlexLog) AddGlobalField(key string, value interface{}) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.globalFields == nil {
		f.globalFields = make(map[string]interface{})
	}
	f.globalFields[key] = value
}

// RemoveGlobalField removes a global field
func (f *FlexLog) RemoveGlobalField(key string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.globalFields != nil {
		delete(f.globalFields, key)
	}
}

// GetGlobalFields returns a copy of the current global fields
func (f *FlexLog) GetGlobalFields() map[string]interface{} {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.globalFields == nil {
		return nil
	}
	// Return a copy to prevent external modification
	copy := make(map[string]interface{}, len(f.globalFields))
	for k, v := range f.globalFields {
		copy[k] = v
	}
	return copy
}

// IsLevelEnabled checks if a log level is enabled
func (f *FlexLog) IsLevelEnabled(level int) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return level >= f.level
}

// WithContext returns a new Logger that includes context values in all log entries
func (f *FlexLog) WithContext(ctx context.Context) Logger {
	return NewContextLogger(f, ctx)
}

// writeLogEntry writes a structured log entry
func (f *FlexLog) writeLogEntry(entry LogEntry) {
	// Merge global fields with entry fields
	if f.globalFields != nil && len(f.globalFields) > 0 {
		if entry.Fields == nil {
			entry.Fields = make(map[string]interface{})
		}
		// Add global fields (entry fields take precedence)
		for k, v := range f.globalFields {
			if _, exists := entry.Fields[k]; !exists {
				entry.Fields[k] = v
			}
		}
	}
	
	// Create a message for the structured entry
	msg := LogMessage{
		Entry:     &entry,
		Timestamp: time.Now(),
	}

	// Send message to channel
	select {
	case f.msgChan <- msg:
		// Message sent successfully
	default:
		// Channel is full, log to stderr (but only in non-test mode)
		if !isTestMode() {
			fmt.Fprintf(os.Stderr, "Warning: message channel full, dropping structured log entry\n")
		}
	}
}

// SetChannelSize sets the buffer size for the message channel
func (f *FlexLog) SetChannelSize(size int) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Cannot change channel size once it's established
	return fmt.Errorf("cannot change channel size after logger is created")
}

// NewDestination creates a new Destination based on the provided URI.
func NewDestination(uri string) (*Destination, error) {
	// Parse the URI to determine the destination type
	if strings.HasPrefix(uri, "syslog://") {
		return &Destination{
			URI:        uri,
			SyslogConn: nil, // Placeholder for actual syslog connection setup
		}, nil
	}

	if strings.HasPrefix(uri, "file://") {
		filePath := strings.TrimPrefix(uri, "file://")
		file, err := os.Create(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to create file destination: %w", err)
		}
		return &Destination{
			URI:    uri,
			Writer: bufio.NewWriter(file),
		}, nil
	}

	return nil, fmt.Errorf("unsupported destination URI: %s", uri)
}
