package features

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// CompressionType defines the compression algorithm used for rotated log files.
type CompressionType int

const (
	// CompressionNone disables compression
	CompressionNone CompressionType = iota
	// CompressionGzip enables gzip compression
	CompressionGzip
)

// CompressionManager handles log file compression
type CompressionManager struct {
	mu              sync.RWMutex
	compressionType CompressionType
	compressMinAge  int
	compressWorkers int
	compressCh      chan string
	compressWg       sync.WaitGroup
	errorHandler     func(source, dest, msg string, err error)
	metricsHandler   func(string) // Function to track compression metrics
}

// NewCompressionManager creates a new compression manager
func NewCompressionManager() *CompressionManager {
	return &CompressionManager{
		compressionType: CompressionNone,
		compressMinAge:  1,
		compressWorkers: 1,
	}
}

// SetErrorHandler sets the error handling function
func (c *CompressionManager) SetErrorHandler(handler func(source, dest, msg string, err error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.errorHandler = handler
}

// SetMetricsHandler sets the metrics tracking function
func (c *CompressionManager) SetMetricsHandler(handler func(string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metricsHandler = handler
}

// SetCompression enables or disables compression for rotated log files
func (c *CompressionManager) SetCompression(compressionType CompressionType) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Validate compression type
	if compressionType != CompressionNone && compressionType != CompressionGzip {
		return fmt.Errorf("invalid compression type: %d", compressionType)
	}

	previousType := c.compressionType
	c.compressionType = compressionType

	// If we're enabling compression and it wasn't enabled before
	if c.compressionType != CompressionNone && previousType == CompressionNone {
		c.startWorkers()
	} else if c.compressionType == CompressionNone && previousType != CompressionNone {
		c.stopWorkers()
	}

	return nil
}

// SetMinAge sets the minimum number of rotations before a log file is compressed
func (c *CompressionManager) SetMinAge(age int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.compressMinAge = age
}

// SetWorkers sets the number of compression worker goroutines
func (c *CompressionManager) SetWorkers(workers int) {
	if workers < 1 {
		workers = 1
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Only update if compression is enabled
	if c.compressionType != CompressionNone {
		c.compressWorkers = workers

		// Restart workers with new count
		c.stopWorkers()
		c.startWorkers()
	} else {
		c.compressWorkers = workers
	}
}

// GetType returns the current compression type
func (c *CompressionManager) GetType() CompressionType {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.compressionType
}

// GetMinAge returns the minimum age for compression
func (c *CompressionManager) GetMinAge() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.compressMinAge
}

// startWorkers starts background goroutines for compression
func (c *CompressionManager) startWorkers() {
	// Create channel for compression jobs
	c.compressCh = make(chan string, 100)

	// Start worker goroutines
	for i := 0; i < c.compressWorkers; i++ {
		c.compressWg.Add(1)
		go func() {
			defer c.compressWg.Done()
			for path := range c.compressCh {
				if err := c.compressFile(path); err != nil {
					if c.errorHandler != nil {
						c.errorHandler("compress", "", fmt.Sprintf("Failed to compress file %s", path), err)
					}
				}
			}
		}()
	}
}

// stopWorkers stops the compression goroutines
func (c *CompressionManager) stopWorkers() {
	if c.compressCh != nil {
		close(c.compressCh)
		c.compressWg.Wait()
		c.compressCh = nil
	}
}

// Start starts the compression manager
func (c *CompressionManager) Start() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.compressionType != CompressionNone {
		c.startWorkers()
	}
}

// Stop stops the compression manager
func (c *CompressionManager) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopWorkers()
}

// QueueFile adds a file to the compression queue
func (c *CompressionManager) QueueFile(path string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.compressionType != CompressionNone && c.compressCh != nil {
		select {
		case c.compressCh <- path:
			// Successfully queued
		default:
			// Queue full, log error
			if c.errorHandler != nil {
				c.errorHandler("compress", "", fmt.Sprintf("Compression queue full, skipping compression for %s", path), nil)
			}
		}
	}
}

// compressFile compresses the given file using the configured compression type
func (c *CompressionManager) compressFile(path string) error {
	c.mu.RLock()
	compressionType := c.compressionType
	c.mu.RUnlock()

	if compressionType == CompressionNone {
		return nil
	}

	switch compressionType {
	case CompressionGzip:
		return c.compressFileGzip(path)
	default:
		return fmt.Errorf("unsupported compression type: %v", compressionType)
	}
}

// compressFileGzip compresses a file using gzip compression
func (c *CompressionManager) compressFileGzip(path string) error {
	// Clean the path to prevent directory traversal
	cleanPath := filepath.Clean(path)
	
	// Check if file exists
	if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
		return nil // file doesn't exist, nothing to compress
	}

	// Compressed file path
	compressedPath := filepath.Clean(cleanPath + ".gz")

	// Open source file
	src, err := os.Open(cleanPath)
	if err != nil {
		return fmt.Errorf("opening source file for compression: %w", err)
	}
	defer func() {
		if closeErr := src.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("closing source file: %w", closeErr)
		}
	}()

	// Create destination file
	dst, err := os.OpenFile(compressedPath, os.O_CREATE|os.O_WRONLY, 0644) // #nosec G302 - compressed log files
	if err != nil {
		return fmt.Errorf("creating compressed file: %w", err)
	}

	// Ensure cleanup on error
	cleanupDst := true
	defer func() {
		if cleanupDst {
			if closeErr := dst.Close(); closeErr != nil && err == nil {
				err = fmt.Errorf("closing destination file: %w", closeErr)
			}
			// Remove partially written file on error
			if err != nil {
				_ = os.Remove(compressedPath) // Best effort cleanup
			}
		}
	}()

	// Create gzip writer
	gw := gzip.NewWriter(dst)

	// Copy data from source to compressed destination
	_, err = io.Copy(gw, src)
	if err != nil {
		return fmt.Errorf("compressing file: %w", err)
	}

	// Close gzip writer and check error
	if err = gw.Close(); err != nil {
		return fmt.Errorf("closing gzip writer: %w", err)
	}

	// Close destination file and check error
	if err = dst.Close(); err != nil {
		return fmt.Errorf("closing compressed file: %w", err)
	}
	cleanupDst = false // Prevent deferred cleanup since we closed successfully

	// Remove the original file
	if err := os.Remove(cleanPath); err != nil {
		// Try to restore by removing the compressed file
		_ = os.Remove(compressedPath) // Best effort cleanup
		return fmt.Errorf("removing original file after compression: %w", err)
	}

	// Track compression metric
	if c.metricsHandler != nil {
		c.metricsHandler("compression_completed")
	}

	return nil
}

// GetStatus returns the current status of the compression manager
func (c *CompressionManager) GetStatus() CompressionStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	status := CompressionStatus{
		Type:        c.compressionType,
		MinAge:      c.compressMinAge,
		Workers:     c.compressWorkers,
		IsRunning:   c.compressCh != nil,
	}
	
	if c.compressCh != nil {
		status.QueueLength = len(c.compressCh)
		status.QueueCapacity = cap(c.compressCh)
	}
	
	return status
}

// CompressionStatus represents the status of the compression manager
type CompressionStatus struct {
	Type          CompressionType `json:"type"`
	MinAge        int             `json:"min_age"`
	Workers       int             `json:"workers"`
	IsRunning     bool            `json:"is_running"`
	QueueLength   int             `json:"queue_length"`
	QueueCapacity int             `json:"queue_capacity"`
}

// CompressFileSync compresses a file synchronously (blocking)
func (c *CompressionManager) CompressFileSync(path string) error {
	return c.compressFile(path)
}

// GetSupportedTypes returns all supported compression types
func GetSupportedCompressionTypes() []CompressionType {
	return []CompressionType{
		CompressionNone,
		CompressionGzip,
	}
}

// CompressionTypeString returns the string representation of a compression type
func CompressionTypeString(ct CompressionType) string {
	switch ct {
	case CompressionNone:
		return "none"
	case CompressionGzip:
		return "gzip"
	default:
		return "unknown"
	}
}

// ParseCompressionType parses a string into a CompressionType
func ParseCompressionType(s string) (CompressionType, error) {
	switch s {
	case "none":
		return CompressionNone, nil
	case "gzip":
		return CompressionGzip, nil
	default:
		return CompressionNone, fmt.Errorf("unsupported compression type: %s", s)
	}
}