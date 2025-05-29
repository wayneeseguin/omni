package flexlog

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

// NewSyslog creates a new logger with a syslog backend
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

// reconnectSyslog attempts to reconnect to a syslog server
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

// SetSyslogTag sets the tag for a syslog destination
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

// SetSyslogPriority sets the priority for a syslog destination
// Priority is constructed as (facility * 8) + severity
// Facility: 0-23 (default: 1 for user-level)
// Severity: 0-7 (emergency to debug, default: 5 for notice)
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
