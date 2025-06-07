package natsplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/wayneeseguin/omni/pkg/backends"
	"github.com/wayneeseguin/omni/pkg/omni"
	"github.com/wayneeseguin/omni/pkg/plugins"
)

// NATSBackendPlugin implements BackendPluginInterface
type NATSBackendPlugin struct {
	initialized bool
	config      map[string]interface{}
}

// Ensure NATSBackend implements the omni.Backend interface
var _ omni.Backend = (*NATSBackend)(nil)

// NATSBackend implements Backend interface
type NATSBackend struct {
	conn       *nats.Conn
	subject    string
	queueGroup string
	options    []nats.Option

	// Performance options
	async      bool
	batchSize  int
	flushTimer *time.Timer
	flushInterval time.Duration

	// Buffering
	buffer   [][]byte
	bufferMu sync.Mutex

	// Metrics
	atomicSupport bool
	
	// Message format
	format string
}

// Name implements Plugin interface
func (p *NATSBackendPlugin) Name() string {
	return "nats"
}

// Version implements Plugin interface
func (p *NATSBackendPlugin) Version() string {
	return "1.0.0"
}

// Initialize implements Plugin interface
func (p *NATSBackendPlugin) Initialize(config map[string]interface{}) error {
	p.config = config
	p.initialized = true
	return nil
}

// Shutdown implements Plugin interface
func (p *NATSBackendPlugin) Shutdown(ctx context.Context) error {
	// Nothing to cleanup at plugin level
	return nil
}

// Description implements Plugin interface
func (p *NATSBackendPlugin) Description() string {
	return "NATS backend plugin for distributed logging"
}

// Health implements Plugin interface
func (p *NATSBackendPlugin) Health() plugins.HealthStatus {
	return plugins.HealthStatus{
		Healthy: p.initialized,
		Message: "NATS backend plugin is ready",
		Details: map[string]interface{}{
			"initialized": p.initialized,
		},
	}
}

// CreateBackend implements BackendPluginInterface
func (p *NATSBackendPlugin) CreateBackend(uri string, config map[string]interface{}) (omni.Backend, error) {
	if !p.initialized {
		return nil, fmt.Errorf("plugin not initialized")
	}

	backend, err := NewNATSBackend(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to create NATS backend: %w", err)
	}

	return backend, nil
}

// SupportedSchemes implements BackendPluginInterface
func (p *NATSBackendPlugin) SupportedSchemes() []string {
	return []string{"nats"}
}

// The plugin itself doesn't need Write/Close/etc methods
// Those are implemented by the NATSBackend instances it creates

// NewNATSBackend creates a new NATS backend from URI
func NewNATSBackend(uri string) (*NATSBackend, error) {
	return NewNATSBackendWithOptions(uri, true)
}

// NewNATSBackendWithOptions creates a new NATS backend with options
func NewNATSBackendWithOptions(uri string, connect bool) (*NATSBackend, error) {
	parsedURL, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid URI: %w", err)
	}

	if parsedURL.Scheme != "nats" {
		return nil, fmt.Errorf("invalid scheme: %s (expected 'nats')", parsedURL.Scheme)
	}

	backend := &NATSBackend{
		subject:       strings.TrimPrefix(parsedURL.Path, "/"),
		async:         true,
		batchSize:     100,
		flushInterval: 100 * time.Millisecond,
		atomicSupport: false,
		format:        "json",
		buffer:        make([][]byte, 0),
	}

	// Parse query parameters
	query := parsedURL.Query()
	
	if queue := query.Get("queue"); queue != "" {
		backend.queueGroup = queue
	}
	
	if asyncStr := query.Get("async"); asyncStr != "" {
		backend.async, _ = strconv.ParseBool(asyncStr)
	}
	
	if batchStr := query.Get("batch"); batchStr != "" {
		if batch, err := strconv.Atoi(batchStr); err == nil {
			backend.batchSize = batch
		}
	}
	
	if flushStr := query.Get("flush_interval"); flushStr != "" {
		if flush, err := strconv.Atoi(flushStr); err == nil {
			backend.flushInterval = time.Duration(flush) * time.Millisecond
		}
	}
	
	if format := query.Get("format"); format != "" {
		backend.format = format
	}

	// Build NATS options
	backend.options = []nats.Option{
		nats.Name("omni-nats-backend"),
	}

	// Add reconnection options
	if maxReconnectStr := query.Get("max_reconnect"); maxReconnectStr != "" {
		if maxReconnect, err := strconv.Atoi(maxReconnectStr); err == nil {
			backend.options = append(backend.options, nats.MaxReconnects(maxReconnect))
		}
	}

	if reconnectWaitStr := query.Get("reconnect_wait"); reconnectWaitStr != "" {
		if reconnectWait, err := strconv.Atoi(reconnectWaitStr); err == nil {
			backend.options = append(backend.options, nats.ReconnectWait(time.Duration(reconnectWait)*time.Second))
		}
	}

	// TLS support
	if tlsStr := query.Get("tls"); tlsStr != "" {
		if tls, _ := strconv.ParseBool(tlsStr); tls {
			backend.options = append(backend.options, nats.Secure())
		}
	}

	// Authentication
	if parsedURL.User != nil {
		username := parsedURL.User.Username()
		password, _ := parsedURL.User.Password()
		backend.options = append(backend.options, nats.UserInfo(username, password))
	}

	// Build server URLs
	servers := []string{}
	if parsedURL.Host != "" {
		servers = append(servers, fmt.Sprintf("nats://%s", parsedURL.Host))
	}

	// Connect to NATS if requested
	if connect && len(servers) > 0 {
		conn, err := nats.Connect(strings.Join(servers, ","), backend.options...)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to NATS: %w", err)
		}
		backend.conn = conn
	}

	// Start flush timer if async and connected
	if backend.async && backend.batchSize > 0 && backend.conn != nil {
		backend.startFlushTimer()
	}

	return backend, nil
}

// Write implements Backend interface
func (n *NATSBackend) Write(entry []byte) (int, error) {
	if n.async && n.batchSize > 0 {
		return n.bufferWrite(entry)
	}
	return n.directWrite(entry)
}

// bufferWrite adds entry to buffer for batching
func (n *NATSBackend) bufferWrite(entry []byte) (int, error) {
	n.bufferMu.Lock()
	defer n.bufferMu.Unlock()

	// Make a copy of the entry
	entryCopy := make([]byte, len(entry))
	copy(entryCopy, entry)
	
	n.buffer = append(n.buffer, entryCopy)

	// Flush if buffer is full
	if len(n.buffer) >= n.batchSize {
		if err := n.flushBufferLocked(); err != nil {
			return 0, err
		}
	}

	return len(entry), nil
}

// directWrite publishes entry immediately
func (n *NATSBackend) directWrite(entry []byte) (int, error) {
	if n.conn == nil {
		return 0, fmt.Errorf("NATS connection not established")
	}
	if err := n.conn.Publish(n.subject, entry); err != nil {
		return 0, fmt.Errorf("failed to publish: %w", err)
	}
	return len(entry), nil
}

// Flush implements Backend interface
func (n *NATSBackend) Flush() error {
	if n.async && len(n.buffer) > 0 {
		return n.flushBuffer()
	}
	if n.conn != nil {
		return n.conn.Flush()
	}
	return nil
}

// flushBuffer flushes buffered messages
func (n *NATSBackend) flushBuffer() error {
	n.bufferMu.Lock()
	defer n.bufferMu.Unlock()
	return n.flushBufferLocked()
}

// flushBufferLocked flushes buffer (must be called with lock held)
func (n *NATSBackend) flushBufferLocked() error {
	if len(n.buffer) == 0 {
		return nil
	}

	if n.conn == nil {
		return fmt.Errorf("NATS connection not established")
	}

	// Publish all buffered messages
	for _, entry := range n.buffer {
		if err := n.conn.Publish(n.subject, entry); err != nil {
			return fmt.Errorf("failed to publish buffered message: %w", err)
		}
	}

	// Clear buffer
	n.buffer = n.buffer[:0]

	// Flush connection
	return n.conn.Flush()
}

// startFlushTimer starts the periodic flush timer
func (n *NATSBackend) startFlushTimer() {
	n.flushTimer = time.AfterFunc(n.flushInterval, func() {
		_ = n.Flush() //nolint:gosec
		n.startFlushTimer() // Restart timer
	})
}

// Close implements Backend interface
func (n *NATSBackend) Close() error {
	// Stop flush timer
	if n.flushTimer != nil {
		n.flushTimer.Stop()
	}

	// Flush any remaining messages
	if err := n.Flush(); err != nil {
		return err
	}

	// Close connection
	if n.conn != nil {
		n.conn.Close()
	}
	return nil
}

// SupportsAtomic implements Backend interface
func (n *NATSBackend) SupportsAtomic() bool {
	return n.atomicSupport
}

// Sync implements Backend interface
func (n *NATSBackend) Sync() error {
	return n.Flush()
}

// GetStats implements Backend interface
func (n *NATSBackend) GetStats() backends.BackendStats {
	return backends.BackendStats{
		Path: n.subject,
		// TODO: Track actual statistics
	}
}

// formatMessage formats a log entry based on configured format
func (n *NATSBackend) formatMessage(entry interface{}) ([]byte, error) {
	switch n.format {
	case "json":
		return json.Marshal(entry)
	case "text":
		// For text format, we expect the entry to already be formatted
		if bytes, ok := entry.([]byte); ok {
			return bytes, nil
		}
		return []byte(fmt.Sprintf("%v", entry)), nil
	default:
		return json.Marshal(entry)
	}
}

// Export plugin for dynamic loading
var BackendPlugin NATSBackendPlugin

// OmniPlugin is the main plugin export (for .so files)
var OmniPlugin = &NATSBackendPlugin{}

// init function removed - plugin will be loaded via plugin system
// The NATSBackendPlugin doesn't implement the full BackendPlugin interface,
// it implements a simpler interface for creating backends