package flexlog

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/flock"
)

// createDestination creates a new destination based on URI and backend type
func (f *FlexLog) createDestination(uri string, backendType int) (*Destination, error) {
	dest := &Destination{
		URI:     uri,
		Backend: backendType,
		Done:    make(chan struct{}),
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

	// Acquire the lock before opening the log file - blocking call
	if err := dest.Lock.RLock(); err != nil {
		return fmt.Errorf("acquiring file lock: %w", err)
	}
	defer dest.Lock.Unlock()

	// Open log file
	file, err := os.OpenFile(dest.URI, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}

	// Get current file size
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return fmt.Errorf("getting file info: %w", err)
	}

	dest.File = file
	dest.Writer = bufio.NewWriterSize(file, defaultBufferSize)
	dest.Size = info.Size()

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

	// Default to standard syslog port if not specified
	if !strings.Contains(address, ":") && network != "unix" {
		address = address + ":514"
	}

	// Open connection to syslog server
	conn, err := net.Dial(network, address)
	if err != nil {
		return fmt.Errorf("connecting to syslog server: %w", err)
	}

	// Use a buffered writer for the connection
	dest.Writer = bufio.NewWriterSize(conn, defaultBufferSize)

	// Store the connection details
	dest.SyslogConn = &syslogConn{
		network:  network,
		address:  address,
		conn:     conn,
		priority: 13,        // Default to notice level, user facility (1*8 + 5)
		tag:      "flexlog", // Default tag
	}

	return nil
}

// AddDestination adds a new log destination with the default backend (flock)
func (f *FlexLog) AddDestination(uri string) error {
	return f.AddDestinationWithBackend(uri, BackendFlock)
}

// AddDestinationWithBackend adds a new log destination with a specific backend type
func (f *FlexLog) AddDestinationWithBackend(uri string, backendType int) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Create a new destination
	dest, err := f.createDestination(uri, backendType)
	if err != nil {
		return err
	}

	// Add to destinations
	f.destinations = append(f.destinations, dest)

	// Start a worker goroutine for this destination
	f.workerWg.Add(1)
	go f.logWorker(dest)

	return nil
}

// RemoveDestination removes a log destination
func (f *FlexLog) RemoveDestination(uri string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for i, dest := range f.destinations {
		if dest.URI == uri {
			// Signal the worker to stop
			close(dest.Done)

			// Flush and close the destination
			dest.Writer.Flush()

			// Close based on backend type
			if dest.Backend == BackendFlock && dest.File != nil {
				dest.File.Close()
			} else if dest.Backend == BackendSyslog && dest.SyslogConn != nil && dest.SyslogConn.conn != nil {
				dest.SyslogConn.conn.Close()
			}

			// Remove from destinations slice
			f.destinations = append(f.destinations[:i], f.destinations[i+1:]...)

			return nil
		}
	}

	return fmt.Errorf("destination not found: %s", uri)
}

// writeToDestinations writes raw bytes to all destinations
func (f *FlexLog) writeToDestinations(p []byte) (int, error) {
	var lastErr error

	// Create a message for the raw bytes
	msg := LogMessage{
		Raw:       p,
		Timestamp: time.Now(),
	}

	// Send the message to the channel for processing
	select {
	case f.msgChan <- msg:
		// Message sent successfully
	default:
		// Channel is full, log to stderr
		lastErr = fmt.Errorf("message channel full, dropping log message")
		fmt.Fprintf(os.Stderr, "Warning: %v\n", lastErr)
	}

	return len(p), lastErr
}

// EnableDestination enables a previously added destination
func (f *FlexLog) EnableDestination(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	for i, dest := range f.destinations {
		if dest.Name == name {
			f.destinations[i].Enabled = true
			return true
		}
	}

	return false
}

// DisableDestination disables a destination without removing it
func (f *FlexLog) DisableDestination(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	for i, dest := range f.destinations {
		if dest.Name == name {
			f.destinations[i].Enabled = false
			return true
		}
	}

	return false
}

// ListDestinations returns a copy of all configured destinations
func (f *FlexLog) ListDestinations() []LogDestination {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Create a copy to avoid race conditions
	destCopy := make([]LogDestination, len(f.destinations))

	// Convert each *Destination to LogDestination
	for i, dest := range f.destinations {
		destCopy[i] = LogDestination{
			Writer:  dest.Writer,
			Name:    dest.Name,
			Enabled: dest.Enabled,
		}
	}

	return destCopy
}

// writeToDestinationsConcurrently writes data to all enabled destinations concurrently
func (f *FlexLog) writeToDestinationsConcurrently(p []byte) (int, error) {
	if len(f.destinations) == 0 {
		return len(p), nil // No destinations, pretend we wrote everything
	}

	var lastErr error
	var wg sync.WaitGroup

	// Create a copy of destinations to avoid holding the lock during writes
	f.mu.Lock()
	// Only use enabled destinations
	enabledDests := make([]*Destination, 0, len(f.destinations))
	for _, d := range f.destinations {
		if d.Enabled {
			enabledDests = append(enabledDests, d)
		}
	}
	f.mu.Unlock()

	// Write to each destination concurrently
	errChan := make(chan error, len(enabledDests))
	for _, dest := range enabledDests {
		wg.Add(1)
		go func(d *Destination) {
			defer wg.Done()
			_, err := d.Writer.Write(p)
			if err != nil {
				errChan <- err
			}
		}(dest)
	}

	// Wait for all writes to complete
	wg.Wait()
	close(errChan)

	// Return the last error if any occurred
	for err := range errChan {
		lastErr = err
	}

	return len(p), lastErr
}

// FlushAll flushes all destinations
func (f *FlexLog) FlushAll() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	var lastErr error

	for _, dest := range f.destinations {
		if err := dest.Writer.Flush(); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// CloseAll closes all destinations and stops all workers
func (f *FlexLog) CloseAll() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Stop all background workers
	close(f.msgChan)

	// Wait for all workers to finish
	f.workerWg.Wait()

	// Stop background processes
	f.stopCompressionWorkers()

	// Close all destinations
	var lastErr error
	for _, dest := range f.destinations {
		if err := dest.Writer.Flush(); err != nil && lastErr == nil {
			lastErr = err
		}

		if err := dest.File.Close(); err != nil && lastErr == nil {
			lastErr = err
		}
	}

	// Clear destinations
	f.destinations = nil
	f.defaultDest = nil
	f.writer = nil
	f.file = nil

	return lastErr
}
