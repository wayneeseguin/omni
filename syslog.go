package flexlog

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

// NewSyslog creates a new logger with a syslog backend.
// This is a convenience function that sets up a FlexLog instance configured
// to send logs to a syslog server. Supports both Unix domain sockets and network connections.
//
// Parameters:
//   - address: The syslog server address. Can be:
//     - Unix socket path (e.g., "/dev/log", "/var/run/syslog")
//     - Hostname:port (e.g., "localhost:514")
//     - Just hostname (defaults to port 514)
//     - Full URI (e.g., "syslog://localhost:514")
//   - tag: The syslog tag/program name (can be empty)
//
// Returns:
//   - *FlexLog: The configured logger instance
//   - error: Connection or configuration error
//
// Example:
//
//	// Connect to local syslog via Unix socket
//	logger, err := flexlog.NewSyslog("/dev/log", "myapp")
//	
//	// Connect to remote syslog server
//	logger, err := flexlog.NewSyslog("syslog.example.com:514", "myapp")
func NewSyslog(address string, tag string) (*FlexLog, error) {
	// Determine the proper URI format based on the address
	var uri string

	if strings.HasPrefix(address, "/") {
		// Unix socket
		uri = "syslog:///" + address
	} else if strings.Contains(address, "://") {
		// Address already has a scheme, use as is
		uri = address
	} else if strings.Contains(address, ":") {
		// Hostname:port format for UDP (default)
		uri = "syslog://" + address
	} else {
		// Just hostname, add default port for UDP
		uri = "syslog://" + address + ":514"
	}

	// Create logger with syslog backend
	logger, err := NewWithBackend(uri, BackendSyslog)
	if err != nil {
		return nil, err
	}

	// Set tag if provided
	if tag != "" {
		if err := logger.SetSyslogTag(uri, tag); err != nil {
			logger.CloseAll()
			return nil, err
		}
	}

	return logger, nil
}

// reconnectSyslog attempts to reconnect to a syslog server.
// This internal method handles reconnection logic when the syslog connection is lost.
// It closes any existing connection and establishes a new one.
//
// Parameters:
//   - dest: The destination to reconnect
//
// Returns:
//   - error: Reconnection error if failed
func (f *FlexLog) reconnectSyslog(dest *Destination) error {
	if dest.SyslogConn == nil {
		return fmt.Errorf("syslog connection not initialized")
	}

	// Close existing connection if any
	if dest.SyslogConn.conn != nil {
		dest.SyslogConn.conn.Close()
	}

	// Reopen connection
	conn, err := net.Dial(dest.SyslogConn.network, dest.SyslogConn.address)
	if err != nil {
		return fmt.Errorf("reconnecting to syslog server: %w", err)
	}

	// Update connection and writer
	dest.mu.Lock()
	dest.SyslogConn.conn = conn
	dest.Writer = bufio.NewWriterSize(conn, defaultBufferSize)
	dest.mu.Unlock()

	return nil
}

// SetSyslogTag sets the tag for a syslog destination.
// The tag identifies the program or process that is logging.
// It appears in syslog messages and helps with filtering and identification.
//
// Parameters:
//   - uri: The syslog destination URI to update
//   - tag: The tag/program name to use
//
// Returns:
//   - error: If the destination is not found or not a syslog destination
//
// Example:
//
//	err := logger.SetSyslogTag("syslog://localhost:514", "myapp-worker")
func (f *FlexLog) SetSyslogTag(uri string, tag string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, dest := range f.Destinations {
		if dest.URI == uri && dest.Backend == BackendSyslog && dest.SyslogConn != nil {
			dest.SyslogConn.tag = tag
			return nil
		}
	}

	return fmt.Errorf("syslog destination not found: %s", uri)
}

// SetSyslogPriority sets the priority for a syslog destination.
// Priority is constructed as (facility * 8) + severity.
// This allows control over how syslog categorizes and handles messages.
//
// Parameters:
//   - uri: The syslog destination URI to update
//   - priority: The syslog priority value (0-191)
//
// Priority calculation:
//   - Facility: 0-23 (default: 1 for user-level)
//   - Severity: 0-7 (emergency to debug, default: 5 for notice)
//   - Priority = (facility * 8) + severity
//
// Common facilities:
//   - 0: Kernel messages
//   - 1: User-level messages
//   - 2: Mail system
//   - 3: System daemons
//   - 4: Security/authorization messages
//   - 16-23: Local use (local0-local7)
//
// Severity levels:
//   - 0: Emergency
//   - 1: Alert
//   - 2: Critical
//   - 3: Error
//   - 4: Warning
//   - 5: Notice
//   - 6: Informational
//   - 7: Debug
//
// Returns:
//   - error: If priority is invalid or destination not found
//
// Example:
//
//	// Set to local0.info (facility=16, severity=6)
//	err := logger.SetSyslogPriority("syslog://localhost:514", 16*8+6)
func (f *FlexLog) SetSyslogPriority(uri string, priority int) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Validate priority
	if priority < 0 || priority > 191 {
		return fmt.Errorf("invalid syslog priority: %d (must be 0-191)", priority)
	}

	for _, dest := range f.Destinations {
		if dest.URI == uri && dest.Backend == BackendSyslog && dest.SyslogConn != nil {
			dest.SyslogConn.priority = priority
			return nil
		}
	}

	return fmt.Errorf("syslog destination not found: %s", uri)
}
