package omni

import (
	"bufio"
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
	
	"github.com/wayneeseguin/omni/internal/buffer"
	"github.com/wayneeseguin/omni/internal/metrics"
	"github.com/wayneeseguin/omni/pkg/backends"
	"github.com/wayneeseguin/omni/pkg/features"
	"github.com/wayneeseguin/omni/pkg/formatters"
	"github.com/wayneeseguin/omni/pkg/plugins"
)

// pluginBackendWrapper wraps a plugin backend to implement backends.Backend
type pluginBackendWrapper struct {
	pluginBackend plugins.Backend
	uri           string
	writeCount    uint64
	bytesWritten  uint64
	mu            sync.Mutex
}

func (w *pluginBackendWrapper) Write(entry []byte) (int, error) {
	n, err := w.pluginBackend.Write(entry)
	if err == nil {
		w.mu.Lock()
		w.writeCount++
		// Only add positive byte counts to prevent underflow
		if n > 0 {
			w.bytesWritten += uint64(n)
		}
		w.mu.Unlock()
	}
	return n, err
}

func (w *pluginBackendWrapper) Flush() error {
	return w.pluginBackend.Flush()
}

func (w *pluginBackendWrapper) Close() error {
	return w.pluginBackend.Close()
}

func (w *pluginBackendWrapper) SupportsAtomic() bool {
	return w.pluginBackend.SupportsAtomic()
}

func (w *pluginBackendWrapper) Sync() error {
	// Try to sync, fall back to flush if not supported
	if syncer, ok := w.pluginBackend.(interface{ Sync() error }); ok {
		return syncer.Sync()
	}
	return w.Flush()
}

func (w *pluginBackendWrapper) GetStats() backends.BackendStats {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	return backends.BackendStats{
		Path:         w.uri,
		WriteCount:   w.writeCount,
		BytesWritten: w.bytesWritten,
	}
}

// Package integration implementations

// createDestination creates a destination with proper backend integration
func (f *Omni) createDestination(uri string, backendType int) (*Destination, error) {
	// Create backend instance
	var backend backends.Backend
	var err error
	
	switch backendType {
	case BackendFlock:
		backend, err = backends.NewFileBackend(uri)
	case BackendSyslog:
		// Parse URI to extract network, address, priority, tag
		network, address := "unix", "/dev/log" // defaults
		tag := "omni"
		priority := 16 // local0.info (facility 16, severity 6)
		
		// Parse syslog URI: syslog://address or syslog:///path
		if strings.HasPrefix(uri, "syslog://") {
			address = strings.TrimPrefix(uri, "syslog://")
			if strings.HasPrefix(address, "/") {
				// Unix socket
				network = "unix"
			} else {
				// Network address
				network = "tcp"
				if !strings.Contains(address, ":") {
					address += ":514" // default syslog port
				}
			}
		} else {
			// Direct address
			if strings.HasPrefix(uri, "/") {
				// Unix socket path
				network = "unix"
				address = uri
			} else {
				// Network address
				network = "tcp"
				address = uri
				if !strings.Contains(address, ":") {
					address += ":514"
				}
			}
		}
		
		backend, err = backends.NewSyslogBackend(network, address, priority, tag)
	default:
		// Try plugin backends
		if f.pluginManager != nil {
			pluginIntegration := plugins.NewIntegration(f.pluginManager)
			pluginBackend, err := pluginIntegration.CreateBackendFromURI(uri)
			if err == nil {
				// Wrap the plugin backend
				backend = &pluginBackendWrapper{
					pluginBackend: pluginBackend,
					uri:          uri,
				}
			}
		} else {
			return nil, fmt.Errorf("unsupported backend type: %d", backendType)
		}
	}
	
	if err != nil {
		return nil, err
	}
	
	// Create destination
	dest := &Destination{
		URI:     uri,
		Backend: backendType,
		Done:    make(chan struct{}),
		Enabled: true,
	}
	
	// Set backend using thread-safe method
	dest.SetBackend(backend)
	
	// For file backends, set additional fields
	if fileBackend, ok := backend.(*backends.FileBackendImpl); ok {
		dest.File = fileBackend.GetFile()
		dest.Writer = fileBackend.GetWriter()
		dest.Lock = fileBackend.GetLock()
		dest.Size = fileBackend.GetSize()
	}
	
	return dest, nil
}

// SetRedaction integrates with the redaction feature
func (f *Omni) SetRedaction(patterns []string, replace string) error {
	// Create redactor
	redactor, err := features.NewRedactor(patterns, replace)
	if err != nil {
		return err
	}
	
	f.mu.Lock()
	defer f.mu.Unlock()
	
	// Initialize redaction manager if needed
	if f.redactionManager == nil {
		f.redactionManager = features.NewRedactionManager()
		f.redactionManager.(*features.RedactionManager).SetErrorHandler(func(source, dest, msg string, err error) {
			f.logError(source, dest, msg, err, ErrorLevelWarn)
		})
	}
	
	f.redactionManager.(*features.RedactionManager).SetCustomRedactor(redactor)
	
	// Configure redaction manager to enable built-in redaction
	config := &features.RedactionConfig{
		EnableBuiltInPatterns: true,
		EnableFieldRedaction:  true,
		EnableDataPatterns:    true,
		MaxCacheSize:         1000,
	}
	f.redactionManager.(*features.RedactionManager).SetConfig(config)
	
	// Also set the legacy redactor field for backward compatibility
	f.redactor = redactor
	f.redactionPatterns = patterns
	f.redactionReplace = replace
	
	return nil
}

// EnableLazyFormatting enables lazy formatting
func (f *Omni) EnableLazyFormatting() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lazyFormatting = true
}

// DisableLazyFormatting disables lazy formatting
func (f *Omni) DisableLazyFormatting() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lazyFormatting = false
}

// IsLazyFormattingEnabled returns whether lazy formatting is enabled
func (f *Omni) IsLazyFormattingEnabled() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.lazyFormatting
}

// Compression integration

func (f *Omni) startCompressionWorkers() {
	if f.compressionManager == nil {
		f.compressionManager = features.NewCompressionManager()
		f.compressionManager.SetErrorHandler(func(source, dest, msg string, err error) {
			f.logError(source, dest, msg, err, ErrorLevelWarn)
		})
		f.compressionManager.SetMetricsHandler(f.trackMetric)
	}
	
	f.compressionManager.Start()
	
	// Set compression callback for rotation manager
	if f.rotationManager != nil {
		f.rotationManager.SetCompressionCallback(func(path string) {
			f.compressionManager.QueueFile(path)
		})
	}
}

func (f *Omni) stopCompressionWorkers() {
	if f.compressionManager != nil {
		f.compressionManager.Stop()
	}
}

// Rotation integration

func (f *Omni) startCleanupRoutine() {
	if f.rotationManager == nil {
		f.rotationManager = features.NewRotationManager()
		f.rotationManager.SetErrorHandler(func(source, dest, msg string, err error) {
			f.logError(source, dest, msg, err, ErrorLevelWarn)
		})
		f.rotationManager.SetMetricsHandler(f.trackMetric)
	}
	
	// Add all log paths to rotation manager
	f.mu.RLock()
	for _, dest := range f.Destinations {
		if dest.Backend == BackendFlock {
			f.rotationManager.AddLogPath(dest.URI)
		}
	}
	f.mu.RUnlock()
	
	f.rotationManager.Start()
}

func (f *Omni) stopCleanupRoutine() {
	if f.rotationManager != nil {
		f.rotationManager.Stop()
	}
}

// Buffer pool integration

// GetStringBuilder gets a string builder from the pool
func GetStringBuilder() *strings.Builder {
	return buffer.GetStringBuilder()
}

// PutStringBuilder returns a string builder to the pool
func PutStringBuilder(builder *strings.Builder) {
	buffer.PutStringBuilder(builder)
}

// WithFields methods for fluent API

func (f *Omni) WithFields(fields map[string]interface{}) Logger {
	return &LoggerAdapter{
		logger: f,
		fields: fields,
	}
}

func (f *Omni) WithField(key string, value interface{}) Logger {
	return f.WithFields(map[string]interface{}{key: value})
}

func (f *Omni) WithError(err error) Logger {
	if err == nil {
		return &LoggerAdapter{logger: f}
	}
	return f.WithField("error", err.Error())
}

// Manager interface implementations

func (f *Omni) SetFormat(format int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	// Validate format
	if format < FormatText || format > FormatJSON {
		return fmt.Errorf("invalid format: %d", format)
	}
	
	f.format = format
	
	// Update formatter based on format
	switch format {
	case FormatJSON:
		f.formatter = formatters.NewJSONFormatter()
	case FormatText:
		f.formatter = formatters.NewTextFormatter()
	}
	
	return nil
}

func (f *Omni) GetFormat() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.format
}

func (f *Omni) SetCompression(compressionType int) error {
	if f.compressionManager == nil {
		f.compressionManager = features.NewCompressionManager()
		f.compressionManager.SetErrorHandler(func(source, dest, msg string, err error) {
			f.logError(source, dest, msg, err, ErrorLevelWarn)
		})
		f.compressionManager.SetMetricsHandler(f.trackMetric)
	}
	
	return f.compressionManager.SetCompression(features.CompressionType(compressionType))
}

func (f *Omni) SetMaxAge(duration time.Duration) error {
	f.mu.Lock()
	f.maxAge = duration
	f.mu.Unlock()
	
	if f.rotationManager == nil {
		f.rotationManager = features.NewRotationManager()
		f.rotationManager.SetErrorHandler(func(source, dest, msg string, err error) {
			f.logError(source, dest, msg, err, ErrorLevelWarn)
		})
		f.rotationManager.SetMetricsHandler(f.trackMetric)
	}
	
	return f.rotationManager.SetMaxAge(duration)
}

// Destination management

func (f *Omni) SetDestinationEnabled(index int, enabled bool) error {
	f.mu.RLock()
	if index < 0 || index >= len(f.Destinations) {
		f.mu.RUnlock()
		return fmt.Errorf("invalid destination index: %d", index)
	}
	dest := f.Destinations[index]
	f.mu.RUnlock()
	
	dest.mu.Lock()
	dest.Enabled = enabled
	dest.mu.Unlock()
	
	return nil
}

func (f *Omni) AddDestination(uri string) error {
	// Auto-detect backend type from URI
	backendType := BackendFlock // Default
	if strings.HasPrefix(uri, "syslog://") {
		backendType = BackendSyslog
	}
	
	return f.AddDestinationWithBackend(uri, backendType)
}

func (f *Omni) AddDestinationWithBackend(uri string, backendType int) error {
	dest, err := f.createDestination(uri, backendType)
	if err != nil {
		return err
	}
	
	f.mu.Lock()
	defer f.mu.Unlock()
	
	// Check if destination already exists
	for _, existing := range f.Destinations {
		if existing.URI == uri {
			return fmt.Errorf("destination already exists: %s", uri)
		}
	}
	
	f.Destinations = append(f.Destinations, dest)
	
	// If it's a file destination, add to rotation manager
	if backendType == BackendFlock && f.rotationManager != nil {
		f.rotationManager.AddLogPath(uri)
	}
	
	return nil
}

func (f *Omni) RemoveDestination(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	for i, dest := range f.Destinations {
		if dest.URI == name {
			// Close the destination using thread-safe method
			_ = dest.Close() // Best effort close
			
			// Remove from rotation manager
			if dest.Backend == BackendFlock && f.rotationManager != nil {
				f.rotationManager.RemoveLogPath(name)
			}
			
			// Clear defaultDest if this is the default destination
			if f.defaultDest == dest {
				f.defaultDest = nil
			}
			
			// Remove from slice
			f.Destinations = append(f.Destinations[:i], f.Destinations[i+1:]...)
			return nil
		}
	}
	
	return fmt.Errorf("destination not found: %s", name)
}

func (f *Omni) ListDestinations() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	names := make([]string, len(f.Destinations))
	for i, dest := range f.Destinations {
		names[i] = dest.URI
	}
	
	return names
}

func (f *Omni) EnableDestination(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	for _, dest := range f.Destinations {
		if dest.URI == name {
			dest.mu.Lock()
			dest.Enabled = true
			dest.mu.Unlock()
			return nil
		}
	}
	
	return fmt.Errorf("destination not found: %s", name)
}

func (f *Omni) DisableDestination(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	for _, dest := range f.Destinations {
		if dest.URI == name {
			dest.mu.Lock()
			dest.Enabled = false
			dest.mu.Unlock()
			return nil
		}
	}
	
	return fmt.Errorf("destination not found: %s", name)
}

// Flush and close operations

func (f *Omni) Flush() error {
	// Flush default destination
	if f.defaultDest != nil && f.defaultDest.backend != nil {
		return f.defaultDest.backend.Flush()
	}
	return nil
}

func (f *Omni) FlushAll() error {
	f.mu.RLock()
	destinations := make([]*Destination, len(f.Destinations))
	copy(destinations, f.Destinations)
	f.mu.RUnlock()
	
	var errs []error
	for _, dest := range destinations {
		// Use the thread-safe Flush method
		if err := dest.Flush(); err != nil {
			errs = append(errs, fmt.Errorf("flush %s: %w", dest.URI, err))
		}
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("flush errors: %v", errs)
	}
	
	return nil
}

func (f *Omni) Sync() error {
	// First, send a sync message to the dispatcher to ensure all pending messages are processed
	syncDone := make(chan struct{})
	syncMsg := LogMessage{
		Level:     -1, // Special level to indicate sync message
		Timestamp: time.Now(),
		SyncDone:  syncDone,
	}
	
	// Send sync message through the channel
	select {
	case f.msgChan <- syncMsg:
		// Wait for sync message to be processed
		<-syncDone
	default:
		// Channel is full or closed, continue with flush
	}
	
	// Now sync all destinations
	f.mu.RLock()
	destinations := make([]*Destination, len(f.Destinations))
	copy(destinations, f.Destinations)
	f.mu.RUnlock()
	
	var errs []error
	for _, dest := range destinations {
		backend := dest.GetBackend()
		if backend != nil {
			if fileBackend, ok := backend.(backends.FileBackend); ok {
				if err := fileBackend.Sync(); err != nil {
					errs = append(errs, fmt.Errorf("sync %s: %w", dest.URI, err))
				}
			}
		}
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("sync errors: %v", errs)
	}
	
	return nil
}

func (f *Omni) Close() error {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return nil
	}
	f.closed = true
	f.mu.Unlock()
	
	// Close message channel to stop dispatcher
	close(f.msgChan)
	
	// Wait for dispatcher to finish
	f.workerWg.Wait()
	
	// Stop managers
	f.stopCompressionWorkers()
	f.stopCleanupRoutine()
	
	// Close recovery manager
	if f.recoveryManager != nil {
		_ = f.recoveryManager.Close() // Best effort close
	}
	
	// Close all destinations
	f.mu.RLock()
	destinations := make([]*Destination, len(f.Destinations))
	copy(destinations, f.Destinations)
	f.mu.RUnlock()
	
	var errs []error
	for _, dest := range destinations {
		if err := dest.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close %s: %w", dest.URI, err))
		}
	}
	
	// Clear the destinations
	f.mu.Lock()
	f.Destinations = nil
	f.defaultDest = nil
	f.mu.Unlock()
	
	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	
	return nil
}

func (f *Omni) Shutdown(ctx context.Context) error {
	// Graceful shutdown with context
	done := make(chan error, 1)
	
	go func() {
		done <- f.Close()
	}()
	
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Metrics integration

func (f *Omni) GetMetrics() LoggerMetrics {
	if f.metricsCollector == nil {
		return LoggerMetrics{}
	}
	
	stats := f.metricsCollector.GetStats()
	
	// Convert internal metrics to LoggerMetrics
	messagesByLevel := make(map[int]uint64)
	f.messagesByLevel.Range(func(key, value interface{}) bool {
		if level, ok := key.(int); ok {
			if count, ok := value.(uint64); ok {
				messagesByLevel[level] = count
			}
		}
		return true
	})
	
	// Count active destinations
	f.mu.RLock()
	activeCount := 0
	disabledCount := 0
	for _, dest := range f.Destinations {
		dest.mu.RLock()
		if dest.Enabled {
			activeCount++
		} else {
			disabledCount++
		}
		dest.mu.RUnlock()
	}
	f.mu.RUnlock()
	
	// Get error counts by source
	errorsBySource := make(map[string]uint64)
	if f.metricsCollector != nil {
		// Get metrics from collector which includes ErrorsBySource
		metrics := f.metricsCollector.GetMetrics(0, 0, nil)
		errorsBySource = metrics.ErrorsBySource
	}
	
	return LoggerMetrics{
		MessagesLogged:       stats.WriteCount,
		MessagesDropped:      stats.DroppedCount,
		BytesWritten:         stats.BytesWritten,
		ErrorCount:           stats.ErrorCount,
		ErrorsBySource:       errorsBySource,
		MessagesByLevel:      messagesByLevel,
		WriteCount:           stats.WriteCount,
		ActiveDestinations:   activeCount,
		DisabledDestinations: disabledCount,
	}
}

func (f *Omni) ResetMetrics() {
	if f.metricsCollector != nil {
		f.metricsCollector.ResetMetrics()
	}
	
	// Reset message counters by clearing each key
	f.messagesByLevel.Range(func(key, value interface{}) bool {
		f.messagesByLevel.Delete(key)
		return true
	})
}

func (f *Omni) GetErrors() <-chan LogError {
	if f.errorChannel == nil {
		f.errorChannel = make(chan LogError, 100)
	}
	return f.errorChannel
}

// Filter integration

func (f *Omni) AddFilter(filter FilterFunc) error {
	if f.filterManager == nil {
		f.filterManager = features.NewFilterManager()
		f.filterManager.SetErrorHandler(func(source, dest, msg string, err error) {
			f.logError(source, dest, msg, err, ErrorLevelWarn)
		})
		f.filterManager.SetMetricsHandler(f.trackMetric)
	}
	
	// Convert to features.FilterFunc
	featuresFilter := features.FilterFunc(filter)
	return f.filterManager.AddFilter(featuresFilter)
}

func (f *Omni) RemoveFilter(filter FilterFunc) error {
	// Filters can't be removed by function reference easily
	// This would need a different approach with named filters
	return fmt.Errorf("removing filters by reference not supported, use ClearFilters instead")
}

func (f *Omni) ClearFilters() {
	if f.filterManager != nil {
		f.filterManager.ClearFilters()
	}
}

// Sampling integration

func (f *Omni) SetSampling(strategy int, rate float64) error {
	if f.samplingManager == nil {
		f.samplingManager = features.NewSamplingManager()
		f.samplingManager.SetErrorHandler(func(source, dest, msg string, err error) {
			f.logError(source, dest, msg, err, ErrorLevelWarn)
		})
		f.samplingManager.SetMetricsHandler(f.trackMetric)
	}
	
	f.mu.Lock()
	f.samplingStrategy = strategy
	f.samplingRate = rate
	f.mu.Unlock()
	
	return f.samplingManager.SetStrategy(features.SamplingStrategy(strategy), rate)
}

func (f *Omni) GetSamplingRate() float64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.samplingRate
}

// Error tracking

func (f *Omni) GetErrorCount() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.errorCount
}

func (f *Omni) GetMessageCount(level int) uint64 {
	if val, ok := f.messagesByLevel.Load(level); ok {
		return val.(uint64)
	}
	return 0
}

// SetErrorHandler implements the ErrorReporter interface
func (f *Omni) SetErrorHandler(handler ErrorHandler) {
	f.mu.Lock()
	if handler == nil {
		f.errorHandler = nil
	} else {
		f.errorHandler = func(source, destination, message string, err error) {
			// Convert to LogError and call the handler
			logErr := LogError{
				Operation:   source,
				Destination: destination,
				Message:     message,
				Err:         err,
				Timestamp:   time.Now(),
				Level:       ErrorLevelWarn,
			}
			handler(logErr)
		}
	}
	f.mu.Unlock()
}

// SetErrorHandlerFunc sets the error handler using the internal function signature
func (f *Omni) SetErrorHandlerFunc(handler func(source, destination, message string, err error)) {
	f.mu.Lock()
	f.errorHandler = handler
	f.mu.Unlock()
}

func (f *Omni) GetLastError() *LogError {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.lastError
}

// Internal helper methods

func (f *Omni) shouldLog(level int, format string, fields map[string]interface{}) bool {
	// Check level
	if !f.IsLevelEnabled(level) {
		return false
	}
	
	// Check filters
	if f.filterManager != nil {
		if !f.filterManager.ApplyFilters(level, format, fields) {
			f.trackMessageDropped()
			return false
		}
	}
	
	// Check sampling
	if f.samplingManager != nil {
		if !f.samplingManager.ShouldLog(level, format, fields) {
			f.trackMessageDropped()
			return false
		}
	}
	
	return true
}

func (f *Omni) trackMessageDropped() {
	if f.metricsCollector != nil {
		f.metricsCollector.TrackDropped()
	}
}

func (f *Omni) logError(source, destination, message string, err error, level ErrorLevel) {
	// Create log error
	logErr := LogError{
		Timestamp:   time.Now(),
		Operation:   source,
		Destination: destination,
		Message:     message,
		Err:         err,
		Level:       level,
	}
	
	// Store last error
	f.mu.Lock()
	f.lastError = &logErr
	f.errorCount++
	f.mu.Unlock()
	
	// Send to error channel if available
	if f.errorChannel != nil {
		select {
		case f.errorChannel <- logErr:
		default:
			// Channel full, don't block
		}
	}
	
	// Call error handler
	if f.errorHandler != nil {
		f.errorHandler(source, destination, message, err)
	}
	
	// Track in metrics
	if f.metricsCollector != nil {
		f.metricsCollector.TrackError(source)
	}
}

func (f *Omni) trackMessageLogged(level int) {
	// Update counter
	f.messagesByLevel.Store(level, f.GetMessageCount(level)+1)
	
	// Track in metrics
	if f.metricsCollector != nil {
		f.metricsCollector.TrackMessage(level)
	}
}

func (f *Omni) trackMetric(event string) {
	// Track custom metrics
	if f.metricsCollector != nil {
		// Simple event tracking
		switch event {
		case "rotation_completed":
			f.metricsCollector.TrackRotation()
		case "compression_completed":
			f.metricsCollector.TrackCompression()
		}
	}
}

func (f *Omni) checkFlushSize(dest *Destination) {
	// Check if destination needs flushing based on size
	if dest.Size > 0 && dest.Size >= f.maxSize/2 {
		// Use thread-safe flush method
		_ = dest.Flush() // Best effort flush
	}
}

func (f *Omni) GetFormatOptions() FormatOptions {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.formatOptions
}

func (f *Omni) recursiveRedact(fields map[string]interface{}) {
	if f.redactionManager == nil {
		return
	}
	
	// Redact fields in place
	features.RecursiveRedact(fields, "", nil, nil)
}

// NewContextLogger creates a context-aware logger
func NewContextLogger(logger *Omni, ctx context.Context) Logger {
	return &ContextLogger{
		logger: logger,
		ctx:    ctx,
	}
}

func (f *Omni) rotateDestination(dest *Destination) error {
	if dest.Backend != BackendFlock {
		return nil // Only file destinations support rotation
	}
	
	if f.rotationManager == nil {
		f.rotationManager = features.NewRotationManager()
		f.rotationManager.SetErrorHandler(func(source, dest, msg string, err error) {
			f.logError(source, dest, msg, err, ErrorLevelWarn)
		})
		f.rotationManager.SetMetricsHandler(f.trackMetric)
	}
	
	// Get writer from backend using thread-safe method
	var writer *bufio.Writer
	backend := dest.GetBackend()
	if backend != nil {
		// Check if it's a file backend that has GetWriter
		if fileBackend, ok := backend.(backends.FileBackend); ok {
			writer = fileBackend.GetWriter()
		}
	}
	
	rotatedPath, err := f.rotationManager.RotateFile(dest.URI, writer)
	if err != nil {
		return err
	}
	
	// Re-open the file
	newDest, err := f.createDestination(dest.URI, dest.Backend)
	if err != nil {
		return err
	}
	
	// Update destination using thread-safe method
	dest.SetBackend(newDest.GetBackend())
	dest.mu.Lock()
	dest.File = newDest.File
	dest.Writer = newDest.Writer
	dest.Lock = newDest.Lock
	dest.Size = 0
	dest.mu.Unlock()
	
	// Queue for compression if enabled
	if f.compressionManager != nil && f.compression != CompressionNone {
		f.compressionManager.QueueFile(rotatedPath)
	}
	
	return nil
}

func (f *Omni) trackWrite(size int64, duration time.Duration) {
	if f.metricsCollector != nil {
		f.metricsCollector.TrackWrite(size, duration)
	}
}

// NOTE: Redactor methods are implemented in the features package

// Destination helpers
func (d *Destination) trackError() {
	d.mu.Lock()
	d.errorCount++
	d.lastWrite = time.Now()
	d.mu.Unlock()
}

func (d *Destination) trackWrite(size int64, duration time.Duration) {
	d.mu.Lock()
	d.writeCount++
	// Only add positive byte counts to prevent underflow
	if size > 0 {
		d.bytesWritten += uint64(size)
	}
	d.totalWriteTime += duration
	if duration > d.maxWriteTime {
		d.maxWriteTime = duration
	}
	d.mu.Unlock()
}

// Format helpers

func formatJSONEntry(entry *LogEntry) ([]byte, error) {
	formatter := formatters.NewJSONFormatter()
	
	// Convert LogEntry to LogMessage for formatter
	msg := LogMessage{
		Entry:     entry,
		Timestamp: time.Now(),
	}
	
	return formatter.Format(msg)
}

func (f *Omni) formatTimestamp(t time.Time) string {
	f.mu.RLock()
	format := f.formatOptions.TimestampFormat
	f.mu.RUnlock()
	
	if format == "" {
		format = time.RFC3339
	}
	
	return t.Format(format)
}

func levelToString(level int) string {
	switch level {
	case LevelTrace:
		return "TRACE"
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return fmt.Sprintf("LEVEL%d", level)
	}
}

func addMetadataFields(entry *LogEntry, f *Omni) {
	if entry.Fields == nil {
		entry.Fields = make(map[string]interface{})
	}
	
	// Add metadata fields if configured
	if f.formatOptions.IncludeSource {
		entry.Fields["source"] = "omni"
	}
	
	if f.formatOptions.IncludeHost {
		if hostname, err := GetHostname(); err == nil {
			entry.Fields["host"] = hostname
		}
	}
}

func (f *Omni) RecoverFromError(err error, msg LogMessage, dest *Destination) {
	if f.recoveryManager == nil {
		f.recoveryManager = features.NewRecoveryManager(nil)
		f.recoveryManager.SetErrorHandler(func(source, dest, msg string, err error) {
			f.logError(source, dest, msg, err, ErrorLevelWarn)
		})
		f.recoveryManager.SetMetricsHandler(f.trackMetric)
	}
	
	// Create a write function for retry
	writeFunc := func() error {
		return f.processMessage(msg, dest)
	}
	
	f.recoveryManager.HandleError(err, msg, dest.URI, writeFunc)
}

// Error helpers

func NewOmniError(code int, operation, target string, err error) LogError {
	return LogError{
		Timestamp: time.Now(),
		Operation: operation,
		Message:   fmt.Sprintf("Error in %s: %v", operation, err),
		Err:       err,
		Code:      code,
	}
}

func (e LogError) WithDestination(dest string) LogError {
	e.Destination = dest
	return e
}

func (e LogError) WithContext(key, value string) LogError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// Initialize collectors and managers
func (f *Omni) initializeManagers() {
	// Initialize metrics collector
	if f.metricsCollector == nil {
		f.metricsCollector = metrics.NewCollector()
	}
}

// getDestinationStats returns statistics for all destinations
func (f *Omni) getDestinationStats() map[string]backends.BackendStats {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	stats := make(map[string]backends.BackendStats)
	for _, dest := range f.Destinations {
		backend := dest.GetBackend()
		if backend != nil {
			stats[dest.URI] = backend.GetStats()
		}
	}
	
	return stats
}

// WithFields methods implementation moved from stubs
func (f *Omni) TraceWithFields(msg string, fields map[string]interface{}) {
	if !f.IsLevelEnabled(LevelTrace) {
		return
	}
	entry := &LogEntry{
		Timestamp: f.formatTimestamp(time.Now()),
		Level:     "TRACE",
		Message:   msg,
		Fields:    fields,
	}
	f.logStructured(LevelTrace, entry)
}

func (f *Omni) DebugWithFields(msg string, fields map[string]interface{}) {
	if !f.IsLevelEnabled(LevelDebug) {
		return
	}
	entry := &LogEntry{
		Timestamp: f.formatTimestamp(time.Now()),
		Level:     "DEBUG", 
		Message:   msg,
		Fields:    fields,
	}
	f.logStructured(LevelDebug, entry)
}

func (f *Omni) InfoWithFields(msg string, fields map[string]interface{}) {
	if !f.IsLevelEnabled(LevelInfo) {
		return
	}
	entry := &LogEntry{
		Timestamp: f.formatTimestamp(time.Now()),
		Level:     "INFO",
		Message:   msg,
		Fields:    fields,
	}
	f.logStructured(LevelInfo, entry)
}

func (f *Omni) WarnWithFields(msg string, fields map[string]interface{}) {
	if !f.IsLevelEnabled(LevelWarn) {
		return
	}
	entry := &LogEntry{
		Timestamp: f.formatTimestamp(time.Now()),
		Level:     "WARN",
		Message:   msg,
		Fields:    fields,
	}
	f.logStructured(LevelWarn, entry)
}

func (f *Omni) ErrorWithFields(msg string, fields map[string]interface{}) {
	if !f.IsLevelEnabled(LevelError) {
		return
	}
	entry := &LogEntry{
		Timestamp: f.formatTimestamp(time.Now()),
		Level:     "ERROR",
		Message:   msg,
		Fields:    fields,
	}
	f.logStructured(LevelError, entry)
}

// logStructured sends a structured log entry
func (f *Omni) logStructured(level int, entry *LogEntry) {
	if !f.shouldLog(level, entry.Message, entry.Fields) {
		return
	}
	
	// Sanitize fields to prevent circular references
	if entry.Fields != nil {
		entry.Fields = f.sanitizeFields(entry.Fields)
	}
	
	// Apply redaction
	if f.redactionManager != nil && entry.Fields != nil {
		redactedMsg, redactedFields := f.redactionManager.(*features.RedactionManager).RedactMessage(level, entry.Message, entry.Fields)
		entry.Message = redactedMsg
		entry.Fields = redactedFields
	}
	
	// Add metadata
	addMetadataFields(entry, f)
	
	// Create LogMessage with structured entry
	msg := LogMessage{
		Level:     level,
		Timestamp: time.Now(),
		Entry:     entry,
	}
	
	f.trackMessageLogged(level)
	f.dispatchMessage(msg)
}

// sanitizeFields removes circular references from fields map
func (f *Omni) sanitizeFields(fields map[string]interface{}) map[string]interface{} {
	visited := make(map[uintptr]bool)
	result := f.sanitizeFieldsRecursive(fields, visited, 0, 10)
	if safeMap, ok := result.(map[string]interface{}); ok {
		return safeMap
	}
	// If sanitization failed, return empty map
	return make(map[string]interface{})
}

// sanitizeFieldsRecursive removes circular references with depth limiting
func (f *Omni) sanitizeFieldsRecursive(data interface{}, visited map[uintptr]bool, depth, maxDepth int) interface{} {
	if depth > maxDepth {
		return "[max depth exceeded]"
	}
	
	switch v := data.(type) {
	case map[string]interface{}:
		// Check for circular reference using map's address safely
		rv := reflect.ValueOf(v)
		if rv.Kind() != reflect.Map || rv.IsNil() {
			return nil
		}
		mapAddr := rv.Pointer()
		if visited[mapAddr] {
			return "[circular reference detected]"
		}
		visited[mapAddr] = true
		defer delete(visited, mapAddr)
		
		sanitized := make(map[string]interface{})
		for k, val := range v {
			sanitized[k] = f.sanitizeFieldsRecursive(val, visited, depth+1, maxDepth)
		}
		return sanitized
		
	case []interface{}:
		sanitized := make([]interface{}, len(v))
		for i, val := range v {
			sanitized[i] = f.sanitizeFieldsRecursive(val, visited, depth+1, maxDepth)
		}
		return sanitized
		
	case map[interface{}]interface{}:
		// Convert generic map to string-keyed map
		sanitized := make(map[string]interface{})
		for k, val := range v {
			keyStr := fmt.Sprintf("%v", k)
			sanitized[keyStr] = f.sanitizeFieldsRecursive(val, visited, depth+1, maxDepth)
		}
		return sanitized
		
	default:
		// For primitive types and other safe types, return as-is
		return data
	}
}

// dispatchMessage sends a message to all destinations
func (f *Omni) dispatchMessage(msg LogMessage) {
	// Send to message channel for async processing
	select {
	case f.msgChan <- msg:
		// Message sent successfully
	default:
		// Channel full
		f.trackMessageDropped()
		if f.errorHandler != nil {
			f.errorHandler("dispatch", "", "Message channel full", nil)
		}
	}
}

// SetFormatter sets the formatter for the logger
func (f *Omni) SetFormatter(formatter Formatter) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.formatter = formatter
}

// GetFormatter returns the current formatter
func (f *Omni) GetFormatter() Formatter {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.formatter
}

// formatMessage uses the configured formatter to format a log message
func (f *Omni) formatMessage(msg LogMessage) ([]byte, error) {
	formatter := f.GetFormatter()
	if formatter != nil {
		return formatter.Format(msg)
	}
	
	// Fallback to default formatting based on format type
	switch f.format {
	case FormatJSON:
		if f.jsonFormatter == nil {
			f.jsonFormatter = formatters.NewJSONFormatter()
		}
		return f.jsonFormatter.Format(msg)
	default:
		if f.textFormatter == nil {
			f.textFormatter = formatters.NewTextFormatter()
		}
		// Update formatter options from logger's formatOptions
		f.mu.RLock()
		f.textFormatter.Options = formatters.FormatOptions(f.formatOptions)
		f.mu.RUnlock()
		return f.textFormatter.Format(msg)
	}
}

// StructuredLog logs a structured message with the given level and fields
func (f *Omni) StructuredLog(level int, message string, fields map[string]interface{}) {
	if !f.IsLevelEnabled(level) {
		return
	}
	
	entry := &LogEntry{
		Timestamp: f.formatTimestamp(time.Now()),
		Level:     levelToString(level),
		Message:   message,
		Fields:    fields,
	}
	
	f.logStructured(level, entry)
}

// CloseAll is an alias for Close() for backward compatibility
func (f *Omni) CloseAll() error {
	return f.Close()
}