package flexlog

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
)

const KeepLogFile = false
const MoveLogFile = true

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
		// Best effort unlock
		if unlockErr := dest.Lock.Unlock(); unlockErr != nil {
			return fmt.Errorf("opening log file: %w (also failed to unlock: %v)", err, unlockErr)
		}
		return fmt.Errorf("opening log file: %w", err)
	}

	// Get current file size
	info, err := file.Stat()
	if err != nil {
		file.Close()
		// Best effort unlock
		if unlockErr := dest.Lock.Unlock(); unlockErr != nil {
			return fmt.Errorf("getting file info: %w (also failed to unlock: %v)", err, unlockErr)
		}
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
	dest.mu.Lock()
	_, err = dest.Writer.Write(bytes)
	dest.mu.Unlock()
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
	case LevelTrace:
		levelStr = "TRACE"
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

// AddDestination adds a new log destination with default file backend.
// This is a convenience method that creates a file-based destination.
//
// Parameters:
//   - uri: The file path for the log destination
//
// Returns:
//   - error: Any error encountered while adding the destination
//
// Example:
//
//	err := logger.AddDestination("/var/log/app-errors.log")
//	if err != nil {
//	    log.Fatal("Failed to add error log destination:", err)
//	}
func (f *FlexLog) AddDestination(uri string) error {
	// Default to file backend for simplicity
	return f.AddDestinationWithBackend(uri, BackendFlock)
}

// AddDestinationWithBackend adds a new log destination with specified backend type.
// This method allows you to create destinations with different backend implementations.
//
// Parameters:
//   - uri: The destination URI (file path for BackendFlock, syslog address for BackendSyslog)
//   - backendType: The backend type (BackendFlock or BackendSyslog)
//
// Returns:
//   - error: Any error encountered while adding the destination
//
// Example:
//
//	// Add a file-based destination
//	err := logger.AddDestinationWithBackend("/var/log/app.log", flexlog.BackendFlock)
//	
//	// Add a syslog destination
//	err := logger.AddDestinationWithBackend("localhost:514", flexlog.BackendSyslog)
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

	// No need to start a new worker - the single dispatcher handles all destinations

	return nil
}

// RemoveDestination removes a log destination by URI.
// The destination is flushed and properly closed before removal.
//
// Parameters:
//   - uri: The URI of the destination to remove
//
// Returns:
//   - error: Any error encountered during removal
//
// Example:
//
//	err := logger.RemoveDestination("/var/log/app-debug.log")
//	if err != nil {
//	    log.Printf("Failed to remove debug log: %v", err)
//	}
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
	var lastErr error
	
	if dest.File != nil {
		// Flush and close file
		if dest.Writer != nil {
			dest.mu.Lock()
			if err := dest.Writer.Flush(); err != nil {
				lastErr = fmt.Errorf("flushing writer: %w", err)
			}
			dest.mu.Unlock()
		}
		if err := dest.File.Close(); err != nil && lastErr == nil {
			lastErr = fmt.Errorf("closing file: %w", err)
		}
	}

	if dest.SyslogConn != nil && dest.SyslogConn.conn != nil {
		if err := dest.SyslogConn.conn.Close(); err != nil && lastErr == nil {
			lastErr = fmt.Errorf("closing syslog connection: %w", err)
		}
	}

	if dest.Lock != nil {
		if err := dest.Lock.Unlock(); err != nil && lastErr == nil {
			lastErr = fmt.Errorf("unlocking file: %w", err)
		}
	}

	// Remove from slice
	f.Destinations = append(f.Destinations[:index], f.Destinations[index+1:]...)

	return lastErr
}

// EnableDestination enables a previously disabled destination by name.
// Enabled destinations will receive log messages.
//
// Parameters:
//   - name: The name of the destination to enable
//
// Returns:
//   - bool: true if the destination was found and enabled, false otherwise
//
// Example:
//
//	if logger.EnableDestination("error-log") {
//	    fmt.Println("Error logging re-enabled")
//	}
func (f *FlexLog) EnableDestination(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, dest := range f.Destinations {
		if dest.Name == name {
			dest.mu.Lock()
			dest.Enabled = true
			dest.mu.Unlock()
			return true
		}
	}

	return false
}

// DisableDestination disables a destination by name.
// Disabled destinations will not receive log messages but remain configured.
//
// Parameters:
//   - name: The name of the destination to disable
//
// Returns:
//   - bool: true if the destination was found and disabled, false otherwise
//
// Example:
//
//	// Temporarily disable debug logging
//	if logger.DisableDestination("debug-log") {
//	    fmt.Println("Debug logging temporarily disabled")
//	}
func (f *FlexLog) DisableDestination(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, dest := range f.Destinations {
		if dest.Name == name {
			dest.mu.Lock()
			dest.Enabled = false
			dest.mu.Unlock()
			return true
		}
	}

	return false
}

// CloseDestination closes and removes a specific destination by name.
// The destination is flushed before closing to ensure no messages are lost.
//
// Parameters:
//   - name: The name of the destination to close
//
// Returns:
//   - error: Any error encountered during closing, or if destination not found
//
// Example:
//
//	err := logger.CloseDestination("temporary-debug-log")
//	if err != nil {
//	    log.Printf("Error closing debug log: %v", err)
//	}
func (f *FlexLog) CloseDestination(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for i, dest := range f.Destinations {
		if dest.Name == name {
			// Close the destination
			if err := f.closeDestination(dest); err != nil {
				return err
			}

			// Remove from the list
			f.Destinations = append(f.Destinations[:i], f.Destinations[i+1:]...)
			
			// If this was the default destination, clear it
			if f.defaultDest == dest {
				f.defaultDest = nil
			}
			
			return nil
		}
	}

	return fmt.Errorf("destination not found: %s", name)
}

// closeDestination closes a single destination
func (f *FlexLog) closeDestination(dest *Destination) error {
	dest.mu.Lock()
	defer dest.mu.Unlock()

	// Flush the buffer
	if dest.Writer != nil {
		if err := dest.Writer.Flush(); err != nil {
			f.logError("flush", dest.Name, "Failed to flush destination", err, ErrorLevelMedium)
		}
	}

	// Close based on backend type
	switch dest.Backend {
	case BackendFlock:
		// Close the file
		if dest.File != nil {
			if err := dest.File.Close(); err != nil {
				return fmt.Errorf("closing file: %w", err)
			}
		}
		// Unlock the file
		if dest.Lock != nil {
			if err := dest.Lock.Unlock(); err != nil {
				return fmt.Errorf("unlocking file: %w", err)
			}
		}
	case BackendSyslog:
		// Close syslog connection
		if dest.SyslogConn != nil && dest.SyslogConn.conn != nil {
			if closer, ok := dest.SyslogConn.conn.(io.Closer); ok {
				if err := closer.Close(); err != nil {
					return fmt.Errorf("closing syslog connection: %w", err)
				}
			}
		}
	}

	return nil
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

// FlushAll flushes all destination buffers to ensure all pending messages are written.
// This is useful when you need to ensure all log messages are persisted immediately.
//
// Returns:
//   - error: The last error encountered during flushing, if any
//
// Example:
//
//	// Ensure all logs are written before shutdown
//	if err := logger.FlushAll(); err != nil {
//	    log.Printf("Warning: failed to flush all logs: %v", err)
//	}
func (f *FlexLog) FlushAll() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	var lastErr error

	for _, dest := range f.Destinations {
		dest.mu.Lock()
		if dest.Writer != nil {
			if err := dest.Writer.Flush(); err != nil {
				lastErr = err
			}
		}
		dest.mu.Unlock()
	}

	return lastErr
}

// Sync ensures all pending log messages are written to their destinations
// This is useful for testing or when you need to ensure all logs are written
func (f *FlexLog) Sync() error {
	// Don't sync if closed
	if f.IsClosed() {
		return nil
	}
	
	// Wait a bit for messages to be processed
	// This is a simple implementation - a more robust one would use channels
	time.Sleep(200 * time.Millisecond)
	
	// Now flush all destinations
	return f.FlushAll()
}

// Close closes the logger and all its destinations.
// This method ensures all pending messages are written before closing.
// It is an alias for CloseAll for backward compatibility.
//
// Returns:
//   - error: Any error encountered during closing
//
// Example:
//
//	defer func() {
//	    if err := logger.Close(); err != nil {
//	        log.Printf("Error closing logger: %v", err)
//	    }
//	}()
func (f *FlexLog) Close() error {
	return f.CloseAll()
}

// CloseAll closes all destinations and shuts down the logger completely.
// This method:
//   - Stops accepting new messages
//   - Waits for all pending messages to be processed
//   - Flushes all buffers
//   - Closes all destinations
//   - Stops background workers (compression, cleanup)
//
// Returns:
//   - error: The last error encountered during shutdown, if any
//
// Example:
//
//	// Graceful shutdown
//	if err := logger.CloseAll(); err != nil {
//	    log.Printf("Error during logger shutdown: %v", err)
//	}
func (f *FlexLog) CloseAll() error {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return nil // Already closed
	}

	// Mark as closed
	f.closed = true
	
	// Get a copy of destinations to avoid holding lock during cleanup
	destinations := make([]*Destination, len(f.Destinations))
	copy(destinations, f.Destinations)
	
	// Close the message channel to signal workers to stop
	close(f.msgChan)
	f.mu.Unlock()

	// Wait for all workers to finish processing remaining messages
	// This must be done without holding the lock
	f.workerWg.Wait()

	var lastErr error

	// Stop compression workers if running
	f.mu.Lock()
	f.stopCompressionWorkers()
	f.stopCleanupRoutine()
	f.mu.Unlock()
	
	// Clean up destination resources
	for _, dest := range destinations {
		if err := f.closeDestination(dest); err != nil {
			lastErr = err
		}
	}

	// Clear destinations
	f.Destinations = nil

	return lastErr
}

// The Info and log methods are already implemented in levels.go
// This code is removed to avoid duplicate declarations

// AddWorker adds a worker for a destination (for testing)
// DEPRECATED: No longer needed with single dispatcher architecture
func (f *FlexLog) AddWorker(dest *Destination) {
	// No-op - single dispatcher handles all destinations
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

	// No need to start a new worker - the single dispatcher handles all destinations

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

	dest := f.Destinations[index]
	dest.mu.Lock()
	dest.Enabled = enabled
	dest.mu.Unlock()
	return nil
}

// FlushDestination flushes a specific destination's buffer (for testing)
func (f *FlexLog) FlushDestination(dest *Destination) error {
	if dest.Writer != nil {
		dest.mu.Lock()
		err := dest.Writer.Flush()
		dest.mu.Unlock()
		return err
	}
	return nil
}

// SetLogPath changes the path of the primary log file.
// If no primary log file exists, an error will be returned.
// This will close the current file, rename it if requested, and open a new one at the specified path.
func (f *FlexLog) SetLogPath(newPath string, moveExistingFile bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Find the primary destination (the first file-based destination)
	if len(f.Destinations) == 0 {
		return fmt.Errorf("no destinations found")
	}

	primaryIdx := -1
	for i, dest := range f.Destinations {
		if dest.Backend == BackendFlock {
			primaryIdx = i
			break
		}
	}

	if primaryIdx == -1 {
		return fmt.Errorf("no file destination found")
	}

	dest := f.Destinations[primaryIdx]
	oldPath := dest.URI

	// Ensure directory exists for new log path
	dir := filepath.Dir(newPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating log directory: %w", err)
	}

	// Flush and close current file
	if dest.Writer != nil {
		dest.mu.Lock()
		if err := dest.Writer.Flush(); err != nil {
			dest.mu.Unlock()
			return fmt.Errorf("flushing current log: %w", err)
		}
		dest.mu.Unlock()
	}

	if dest.File != nil {
		if err := dest.File.Close(); err != nil {
			return fmt.Errorf("closing current log: %w", err)
		}
	}

	// Release the current lock if any
	if dest.Lock != nil {
		if err := dest.Lock.Unlock(); err != nil {
			// Log error but continue - this is not fatal
			fmt.Fprintf(os.Stderr, "Warning: failed to release lock for %s: %v\n", oldPath, err)
		}
	}

	// Move existing file if requested
	if moveExistingFile && oldPath != "" && oldPath != newPath {
		// Only try to rename if the old file exists
		if _, err := os.Stat(oldPath); err == nil {
			if err := os.Rename(oldPath, newPath); err != nil {
				return fmt.Errorf("moving log file: %w", err)
			}
		}
	}

	// Setup new destination
	lockPath := newPath + ".lock"
	dest.Lock = flock.New(lockPath)

	// Acquire lock on new file
	if err := dest.Lock.Lock(); err != nil {
		return fmt.Errorf("acquiring file lock: %w", err)
	}

	// Open new file
	file, err := os.OpenFile(newPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		dest.Lock.Unlock() // Release lock on error
		return fmt.Errorf("opening new log file: %w", err)
	}

	// Get current file size
	info, err := file.Stat()
	if err != nil {
		file.Close()
		dest.Lock.Unlock()
		return fmt.Errorf("getting file info: %w", err)
	}

	// Update destination
	dest.mu.Lock()
	dest.URI = newPath
	dest.File = file
	dest.Writer = bufio.NewWriterSize(file, defaultBufferSize)
	dest.Size = info.Size()
	dest.mu.Unlock()

	// Update logger-level references for backward compatibility
	f.path = newPath
	f.file = file
	f.writer = dest.Writer
	f.fileLock = dest.Lock
	f.currentSize = dest.Size
	f.size = dest.Size

	// Don't release the lock here - it should be held while writing
	// The lock will be released when the file is closed or changed again

	return nil
}
