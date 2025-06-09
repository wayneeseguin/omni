package backends

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
)

// SyslogBackendImpl implements the Backend interface for syslog
type SyslogBackendImpl struct {
	network  string
	address  string
	conn     net.Conn
	writer   *bufio.Writer
	priority int
	tag      string
	mu       sync.Mutex // Protects concurrent access to writer
}

// NewSyslogBackend creates a new syslog backend
func NewSyslogBackend(network, address string, priority int, tag string) (*SyslogBackendImpl, error) {
	// Default to local syslog if no address specified
	if address == "" {
		// Try common Unix socket paths
		for _, path := range []string{"/dev/log", "/var/run/syslog", "/var/run/log"} {
			if _, err := os.Stat(path); err == nil {
				network = "unix"
				address = path
				break
			}
		}
		if address == "" {
			return nil, fmt.Errorf("no local syslog socket found")
		}
	}

	// Connect to syslog
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, fmt.Errorf("dial syslog: %w", err)
	}

	return &SyslogBackendImpl{
		network:  network,
		address:  address,
		conn:     conn,
		writer:   bufio.NewWriter(conn),
		priority: priority,
		tag:      tag,
	}, nil
}

// Write writes a log entry to syslog
func (sb *SyslogBackendImpl) Write(entry []byte) (int, error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	
	// Format syslog message: <priority>tag: message
	message := fmt.Sprintf("<%d>%s: %s", sb.priority, sb.tag, strings.TrimSpace(string(entry)))
	
	n, err := sb.writer.WriteString(message)
	if err != nil {
		return n, err
	}
	
	// Add newline if not present
	if !strings.HasSuffix(message, "\n") {
		if _, err := sb.writer.WriteString("\n"); err != nil {
			return n, err
		}
		n++
	}
	
	return n, nil
}

// Flush flushes buffered data
func (sb *SyslogBackendImpl) Flush() error {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	
	if sb.writer != nil {
		return sb.writer.Flush()
	}
	return nil
}

// Close closes the syslog connection
func (sb *SyslogBackendImpl) Close() error {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	
	var errs []error
	
	// Flush writer
	if sb.writer != nil {
		if err := sb.writer.Flush(); err != nil {
			errs = append(errs, fmt.Errorf("flush: %w", err))
		}
	}
	
	// Close connection
	if sb.conn != nil {
		if err := sb.conn.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close conn: %w", err))
		}
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}

// SupportsAtomic returns false as syslog doesn't support atomic writes
func (sb *SyslogBackendImpl) SupportsAtomic() bool {
	return false
}

// SetPriority sets the syslog priority
func (sb *SyslogBackendImpl) SetPriority(priority int) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	sb.priority = priority
}

// SetTag sets the syslog tag
func (sb *SyslogBackendImpl) SetTag(tag string) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	sb.tag = tag
}

// Sync syncs the backend (flushes for syslog)
func (sb *SyslogBackendImpl) Sync() error {
	return sb.Flush()
}

// GetStats returns backend statistics
func (sb *SyslogBackendImpl) GetStats() BackendStats {
	return BackendStats{
		Path: fmt.Sprintf("syslog://%s/%s", sb.network, sb.address),
	}
}