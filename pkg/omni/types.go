package omni

import (
	"bufio"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/gofrs/flock"
	"github.com/wayneeseguin/omni/pkg/backends"
	"github.com/wayneeseguin/omni/pkg/features"
	"github.com/wayneeseguin/omni/pkg/formatters"
	"github.com/wayneeseguin/omni/pkg/types"
)

// Re-export shared types for backward compatibility
type Backend = types.Backend
type Formatter = types.Formatter
type LogMessage = types.LogMessage
type LogEntry = types.LogEntry
type BackendStats = types.BackendStats
type FilterFunc = types.FilterFunc

// Re-export formatter types for backward compatibility
type FormatOptions = formatters.FormatOptions
type LevelFormat = formatters.LevelFormat
type FormatOption = formatters.FormatOption

// Re-export features types for backward compatibility
type Redactor = features.Redactor

// BatchConfig defines batching configuration
type BatchConfig struct {
	MaxSize       int
	MaxCount      int
	FlushInterval time.Duration
}

// BatchWriter interface for batch writing operations
type BatchWriter interface {
	WriteBatch([][]byte) error
	Flush() error
}

// BackendFactory creates backend instances for destinations
type BackendFactory interface {
	// CreateDestination creates a destination with the appropriate backend
	CreateDestination(uri string, backendType int) (*Destination, error)
	// CloseDestination properly closes a destination
	CloseDestination(dest *Destination) error
}

// Lock Ordering Hierarchy:
// To prevent deadlocks, always acquire locks in this order:
// 1. f.mu (Omni main mutex) - acquire first
// 2. dest.mu (Destination mutex) - acquire second
// 3. dest.Lock (File lock) - acquire last
// Never acquire a higher-level lock while holding a lower-level lock.

// ErrorLevel is defined in errors.go


// FileBackend represents a file-based backend with flock
type FileBackend struct {
	mu       sync.Mutex
	file     *os.File
	writer   *bufio.Writer
	Lock     *flock.Flock // File lock for process-safe logging
	filepath string
}

// Write implements Backend interface
func (fb *FileBackend) Write(data []byte) (int, error) {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	
	if fb.writer != nil {
		return fb.writer.Write(data)
	}
	return 0, nil
}

// Flush implements Backend interface
func (fb *FileBackend) Flush() error {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	
	if fb.writer != nil {
		return fb.writer.Flush()
	}
	return nil
}

// Close implements Backend interface
func (fb *FileBackend) Close() error {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	
	if fb.writer != nil {
		fb.writer.Flush()
	}
	if fb.file != nil {
		fb.file.Close()
	}
	if fb.Lock != nil {
		fb.Lock.Unlock()
	}
	return nil
}

// SupportsAtomic implements Backend interface
func (fb *FileBackend) SupportsAtomic() bool {
	return true // File backend supports atomic writes with flock
}

// SyslogBackend represents a syslog backend
type SyslogBackend struct {
	mu     sync.Mutex
	writer io.Writer
	conn   net.Conn
}

// Write implements Backend interface
func (sb *SyslogBackend) Write(data []byte) (int, error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	
	if sb.writer != nil {
		return sb.writer.Write(data)
	}
	return 0, nil
}

// Flush implements Backend interface
func (sb *SyslogBackend) Flush() error {
	// Syslog connections don't need explicit flushing
	return nil
}

// Close implements Backend interface
func (sb *SyslogBackend) Close() error {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	
	if sb.conn != nil {
		return sb.conn.Close()
	}
	return nil
}

// SupportsAtomic implements Backend interface
func (sb *SyslogBackend) SupportsAtomic() bool {
	return false // Syslog doesn't guarantee atomic writes
}

// Destination represents a log destination with a backend
type Destination struct {
	URI           string
	Backend       int               // Backend type ID
	backend       backends.Backend  // Backend implementation
	File          *os.File      // For file-based destinations
	Writer        *bufio.Writer // For buffered writing
	Lock          *flock.Flock  // File lock for process safety
	Size          int64         // Current file size
	Done          chan struct{} // Done channel for shutdown
	Enabled       bool          // Whether destination is enabled
	mu            sync.RWMutex
	isHealthy      bool
	lastError      error
	bytesWritten   uint64
	errorCount     uint64
	lastWrite      time.Time
	writeCount     uint64
	totalWriteTime time.Duration
	maxWriteTime   time.Duration
	
	// Batch processing fields
	batchEnabled   bool
	batchMaxSize   int
	batchMaxCount  int
	batchWriter    interface{} // Generic batch writer
}

// GetBackend returns the backend for this destination
func (d *Destination) GetBackend() backends.Backend {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.backend
}

// SetBackend sets the backend for this destination
func (d *Destination) SetBackend(backend backends.Backend) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.backend = backend
}

// IsHealthy returns whether the destination is healthy
func (d *Destination) IsHealthy() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.isHealthy
}

// SetHealthy sets the health status of the destination
func (d *Destination) SetHealthy(healthy bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.isHealthy = healthy
}

// GetStats returns statistics for this destination
func (d *Destination) GetStats() BackendStats {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	return BackendStats{
		BytesWritten: d.bytesWritten,
		ErrorCount:   d.errorCount,
	}
}

// Write writes data to the destination
func (d *Destination) Write(data []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	if d.backend == nil {
		return 0, nil
	}
	
	n, err := d.backend.Write(data)
	if err != nil {
		d.errorCount++
		d.lastError = err
		d.isHealthy = false
		return n, err
	}
	
	d.bytesWritten += uint64(n)
	d.lastWrite = time.Now()
	d.isHealthy = true
	return n, nil
}

// Flush flushes the destination
func (d *Destination) Flush() error {
	d.mu.RLock()
	backend := d.backend
	d.mu.RUnlock()
	
	if backend != nil {
		return backend.Flush()
	}
	return nil
}

// Close closes the destination
func (d *Destination) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	if d.backend != nil {
		return d.backend.Close()
	}
	return nil
}