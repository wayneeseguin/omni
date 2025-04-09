package flexlog

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// defaultChannelSize is the default buffer size for the message channel
// initialized from environment variable FLEXLOG_CHANNEL_SIZE or defaults to 100
var defaultChannelSize = getDefaultChannelSize()

// Filter is a function that determines if a message should be logged
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

// New creates a new file logger
func New(path string) (*FlexLog, error) {
	// For backward compatibility, we treat this as a file-based logger
	// with flock backend by default
	return NewWithOptions(path, BackendFlock)
}

// NewWithOptions creates a new logger with specific options
func NewWithOptions(uri string, backendType int) (*FlexLog, error) {
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
		destinations:     make([]*Destination, 0),
	}

	// Add the destination based on backend type
	dest, err := f.createDestination(uri, backendType)
	if err != nil {
		return nil, err
	}

	// Set as default destination
	f.defaultDest = dest
	f.destinations = append(f.destinations, dest)

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

	// Start background worker goroutine for the destination
	f.workerWg.Add(1)
	go f.logWorker(dest)

	return f, nil
}

// logWorker is a background goroutine that processes log messages
func (f *FlexLog) logWorker(dest *Destination) {
	defer f.workerWg.Done()

	for {
		select {
		case msg, ok := <-f.msgChan:
			if !ok {
				// Channel closed, exit
				return
			}

			// Process the message
			f.processMessage(msg, dest)
		case <-dest.Done:
			// Destination closed, exit
			return
		}
	}
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

// writeLogEntry writes a structured log entry
func (f *FlexLog) writeLogEntry(entry LogEntry) {
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
		// Channel is full, log to stderr
		fmt.Fprintf(os.Stderr, "Warning: message channel full, dropping structured log entry\n")
	}
}

// SetChannelSize sets the buffer size for the message channel
func (f *FlexLog) SetChannelSize(size int) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Cannot change channel size once it's established
	return fmt.Errorf("cannot change channel size after logger is created")
}
