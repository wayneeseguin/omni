package flexlog

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
)

// createDestination creates a new destination based on URI and backend type
func (f *FlexLog) createDestination(uri string, backendType int) (*Destination, error) {
	dest := &Destination{
		URI:     uri,
		Backend: backendType,
		Done:    make(chan struct{}),
		Enabled: true,
	}

	var err error

	switch backendType {
	case BackendFlock:
		// Regular file with flock
		err = f.setupFlockDestination(dest)
	case BackendSyslog:
		// Syslog destination
		err = f.setupSyslogDestination(dest)
	default:
		return nil, fmt.Errorf("unknown backend type: %d", backendType)
	}

	if err != nil {
		return nil, err
	}

	return dest, nil
}

// setupFlockDestination sets up a file destination with flock
func (f *FlexLog) setupFlockDestination(dest *Destination) error {
	// Ensure directory exists
	dir := filepath.Dir(dest.URI)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating log directory: %w", err)
	}

	// Create lock file using flock
	lockPath := dest.URI + ".lock"
	dest.Lock = flock.New(lockPath)

	// Temporarily lock while we're setting up the file
	if err := dest.Lock.Lock(); err != nil {
		return fmt.Errorf("acquiring file lock: %w", err)
	}

	// Open log file
	file, err := os.OpenFile(dest.URI, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		dest.Lock.Unlock() // Unlock before returning on error
		return fmt.Errorf("opening log file: %w", err)
	}

	// Get current file size
	info, err := file.Stat()
	if err != nil {
		file.Close()
		dest.Lock.Unlock() // Unlock before returning on error
		return fmt.Errorf("getting file info: %w", err)
	}

	dest.File = file
	dest.Writer = bufio.NewWriterSize(file, defaultBufferSize)
	dest.Size = info.Size()

	// Release the lock now that we've set up the file
	dest.Lock.Unlock()

	return nil
}

// setupSyslogDestination sets up a syslog destination
func (f *FlexLog) setupSyslogDestination(dest *Destination) error {
	// Parse the URI to determine the network type and address
	// Supported formats:
	// - syslog:///dev/log (Unix socket)
	// - syslog://127.0.0.1:514 (UDP)
	// - syslog+tcp://127.0.0.1:514 (TCP)

	var network, address string

	if strings.HasPrefix(dest.URI, "syslog:///") {
		// Unix socket
		network = "unix"
		address = strings.TrimPrefix(dest.URI, "syslog:///")
	} else if strings.HasPrefix(dest.URI, "syslog+tcp://") {
		// TCP
		network = "tcp"
		address = strings.TrimPrefix(dest.URI, "syslog+tcp://")
	} else if strings.HasPrefix(dest.URI, "syslog://") {
		// UDP (default)
		network = "udp"
		address = strings.TrimPrefix(dest.URI, "syslog://")
	} else {
		return fmt.Errorf("invalid syslog URI format: %s", dest.URI)
	}

	// Create syslog connection
	conn, err := net.Dial(network, address)
	if err != nil {
		return fmt.Errorf("connecting to syslog: %w", err)
	}

	dest.SyslogConn = &syslogConn{
		network:  network,
		address:  address,
		conn:     conn,
		priority: 13, // default to notice priority (facility 1, severity 5)
		tag:      "flexlog",
	}

	dest.Writer = bufio.NewWriterSize(conn, defaultBufferSize)

	return nil
}

// The logWorker method is implemented in logger.go
// Here we declare additional methods for destination management

// writeMessage formats and writes a log message to a destination
func (f *FlexLog) writeMessage(dest *Destination, msg *LogMessage) {
	var err error
	var bytes []byte

	// If raw bytes provided, use those directly
	if len(msg.Raw) > 0 {
		bytes = msg.Raw
	} else {
		// Otherwise format according to logger settings
		if msg.Entry != nil {
			// Use structured entry if provided
			bytes, err = f.formatEntry(msg.Entry)
		} else {
			// Format regular message
			bytes, err = f.formatMessage(msg.Level, msg.Format, msg.Args...)
		}

		if err != nil {
			// If formatting fails, create a simple error message
			bytes = []byte(fmt.Sprintf("ERROR: Failed to format log message: %s\n", err))
		}
	}

	// Write to destination
	_, err = dest.Writer.Write(bytes)
	if err != nil {
		// Not much we can do if writing fails in the background worker
		// Consider adding error reporting channel in future
		return
	}

	// For file destination, update size tracking
	if dest.Backend == BackendFlock && dest.File != nil {
		dest.Size += int64(len(bytes))
	}

	// Flush if needed
	// Could implement periodic or size-based flushing here if desired
}

// formatMessage formats a log message based on the logger configuration
func (f *FlexLog) formatMessage(level int, format string, args ...interface{}) ([]byte, error) {
	// Simple implementation for now
	msg := fmt.Sprintf(format, args...)
	levelStr := getLevelString(level, f.formatOpts.LevelFormat)
	timestamp := time.Now().Format(f.formatOpts.TimestampFormat)

	fullMsg := fmt.Sprintf("%s %s: %s\n", timestamp, levelStr, msg)
	return []byte(fullMsg), nil
}

// formatEntry formats a structured log entry
func (f *FlexLog) formatEntry(entry *LogEntry) ([]byte, error) {
	// Simple implementation for now
	fullMsg := fmt.Sprintf("%s %s: %s\n", entry.Timestamp, entry.Level, entry.Message)
	return []byte(fullMsg), nil
}

// getLevelString converts a numeric level to a string representation
func getLevelString(level int, format LevelFormat) string {
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
		levelStr = fmt.Sprintf("LEVEL%d", level)
	}

	switch format {
	case LevelFormatNameLower:
		return strings.ToLower(levelStr)
	case LevelFormatSymbol:
		return levelStr[0:1] // First character as symbol
	default: // LevelFormatName, LevelFormatNameUpper
		return levelStr
	}
}

// AddDestination adds a new log destination
func (f *FlexLog) AddDestination(uri string) error {
	// Default to file backend for simplicity
	return f.AddDestinationWithBackend(uri, BackendFlock)
}

// AddDestinationWithBackend adds a new log destination with specified backend
func (f *FlexLog) AddDestinationWithBackend(uri string, backendType int) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Check if already closed
	if f.closed {
		return fmt.Errorf("logger is closed, cannot add destination")
	}

	// Check if destination already exists
	for _, dest := range f.Destinations {
		if dest.URI == uri {
			return fmt.Errorf("destination already exists: %s", uri)
		}
	}

	// Create the new destination
	dest, err := f.createDestination(uri, backendType)
	if err != nil {
		return err
	}

	// Set name to URI by default
	dest.Name = uri
	dest.Enabled = true

	// Add to destinations list
	f.Destinations = append(f.Destinations, dest)

	// Start worker for new destination
	f.workerWg.Add(1)
	go f.logWorker(dest)

	return nil
}

// RemoveDestination removes a log destination by URI
func (f *FlexLog) RemoveDestination(uri string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Find the destination
	index := -1
	for i, dest := range f.Destinations {
		if dest.URI == uri {
			index = i
			break
		}
	}

	if index == -1 {
		return fmt.Errorf("destination not found: %s", uri)
	}

	// Get the destination
	dest := f.Destinations[index]

	// Signal worker to stop
	close(dest.Done)

	// Clean up resources
	if dest.File != nil {
		// Flush and close file
		if dest.Writer != nil {
			dest.Writer.Flush()
		}
		dest.File.Close()
	}

	if dest.SyslogConn != nil && dest.SyslogConn.conn != nil {
		dest.SyslogConn.conn.Close()
	}

	if dest.Lock != nil {
		dest.Lock.Unlock()
	}

	// Remove from slice
	f.Destinations = append(f.Destinations[:index], f.Destinations[index+1:]...)

	return nil
}

// EnableDestination enables a destination by name
func (f *FlexLog) EnableDestination(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, dest := range f.Destinations {
		if dest.Name == name {
			dest.Enabled = true
			return true
		}
	}

	return false
}

// DisableDestination disables a destination by name
func (f *FlexLog) DisableDestination(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, dest := range f.Destinations {
		if dest.Name == name {
			dest.Enabled = false
			return true
		}
	}

	return false
}

// ListDestinations returns a list of all destinations
func (f *FlexLog) ListDestinations() []*Destination {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Create a copy to avoid race conditions
	destsCopy := make([]*Destination, len(f.Destinations))
	copy(destsCopy, f.Destinations)

	return destsCopy
}

// FlushAll flushes all destination buffers
func (f *FlexLog) FlushAll() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	var lastErr error

	for _, dest := range f.Destinations {
		if dest.Writer != nil {
			if err := dest.Writer.Flush(); err != nil {
				lastErr = err
			}
		}
	}

	return lastErr
}

// CloseAll closes all destinations and shuts down the logger
func (f *FlexLog) CloseAll() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return nil // Already closed
	}

	// Mark as closed
	f.closed = true

	// Drain the msgChan before closing it
	for len(f.msgChan) > 0 {
		<-f.msgChan
	}

	// Wait for all workers to finish
	f.workerWg.Wait()

	var lastErr error

	// Clean up destination resources
	for _, dest := range f.Destinations {
		// Flush buffer
		if dest.Writer != nil {
			if err := dest.Writer.Flush(); err != nil {
				lastErr = err
			}
		}

		// Close file if needed
		if dest.File != nil {
			if err := dest.File.Close(); err != nil {
				lastErr = err
			}
		}

		// Close syslog connection if needed
		if dest.SyslogConn != nil && dest.SyslogConn.conn != nil {
			if err := dest.SyslogConn.conn.Close(); err != nil {
				lastErr = err
			}
		}

		// Release flock if needed
		if dest.Lock != nil {
			if err := dest.Lock.Unlock(); err != nil {
				lastErr = err
			}
		}
	}

	// Clear destinations
	f.Destinations = nil

	return lastErr
}

// The Info and log methods are already implemented in levels.go
// This code is removed to avoid duplicate declarations

// AddWorker adds a worker for a destination (for testing)
func (f *FlexLog) AddWorker(dest *Destination) {
	f.workerWg.Add(1)
	go f.logWorker(dest)
}

// AddCustomDestination adds a custom destination with the provided writer (for testing)
func (f *FlexLog) AddCustomDestination(name string, writer *bufio.Writer) *Destination {
	f.mu.Lock()
	defer f.mu.Unlock()

	dest := &Destination{
		URI:     name,
		Name:    name,
		Backend: -1, // Custom backend
		Writer:  writer,
		Done:    make(chan struct{}),
		Enabled: true,
	}

	f.Destinations = append(f.Destinations, dest)

	// Start worker for this destination
	f.workerWg.Add(1)
	go f.logWorker(dest)

	return dest
}

// SetDestinationName sets a destination's name (for testing)
func (f *FlexLog) SetDestinationName(index int, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if index < 0 || index >= len(f.Destinations) {
		return fmt.Errorf("destination index out of range")
	}

	f.Destinations[index].Name = name
	return nil
}

// SetDestinationEnabled sets a destination's enabled flag (for testing)
func (f *FlexLog) SetDestinationEnabled(index int, enabled bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if index < 0 || index >= len(f.Destinations) {
		return fmt.Errorf("destination index out of range")
	}

	f.Destinations[index].Enabled = enabled
	return nil
}

// FlushDestination flushes a specific destination's buffer (for testing)
func (f *FlexLog) FlushDestination(dest *Destination) error {
	if dest.Writer != nil {
		return dest.Writer.Flush()
	}
	return nil
}
