package flexlog

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
)

// SetCompression enables or disables compression for rotated log files.
// When enabled, rotated log files will be compressed using the specified algorithm.
//
// Parameters:
//   - compressionType: Type of compression (CompressionNone or CompressionGzip)
//
// Returns:
//   - error: If an invalid compression type is specified
//
// Example:
//
//	logger.SetCompression(flexlog.CompressionGzip)  // Enable gzip compression
//	logger.SetCompression(flexlog.CompressionNone)  // Disable compression
func (f *FlexLog) SetCompression(compressionType int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	// Validate compression type
	if compressionType != CompressionNone && compressionType != CompressionGzip {
		return fmt.Errorf("invalid compression type: %d", compressionType)
	}

	previousType := f.compression
	f.compression = compressionType

	// If we're enabling compression and it wasn't enabled before
	if f.compression != CompressionNone && previousType == CompressionNone {
		f.startCompressionWorkers()
	} else if f.compression == CompressionNone && previousType != CompressionNone {
		f.stopCompressionWorkers()
	}
	
	return nil
}

// SetCompressMinAge sets the minimum number of rotations before a log file is compressed.
// This prevents compressing files that might still be actively read.
//
// Parameters:
//   - age: Number of rotations to wait before compressing (default: 1)
//
// Example:
//
//	logger.SetCompressMinAge(3)  // Compress files after 3 rotations
func (f *FlexLog) SetCompressMinAge(age int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.compressMinAge = age
}

// SetCompressWorkers sets the number of compression worker goroutines.
// More workers can speed up compression when many files need to be compressed.
//
// Parameters:
//   - workers: Number of worker goroutines (minimum: 1, default: 1)
//
// Example:
//
//	logger.SetCompressWorkers(4)  // Use 4 workers for parallel compression
func (f *FlexLog) SetCompressWorkers(workers int) {
	if workers < 1 {
		workers = 1
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// Only update if compression is enabled
	if f.compression != int(CompressionNone) {
		f.compressWorkers = workers

		// Restart workers with new count
		f.stopCompressionWorkers()
		f.startCompressionWorkers()
	} else {
		f.compressWorkers = workers
	}
}

// startCompressionWorkers starts background goroutines for compression.
// This method is called automatically when compression is enabled.
// The workers process files from the compression queue in parallel.
func (f *FlexLog) startCompressionWorkers() {
	// Create channel for compression jobs
	f.compressCh = make(chan string, 100)

	// Start worker goroutines
	for i := 0; i < f.compressWorkers; i++ {
		f.compressWg.Add(1)
		go func() {
			defer f.compressWg.Done()
			for path := range f.compressCh {
				if err := f.compressFile(path); err != nil {
					f.logError("compress", "", fmt.Sprintf("Failed to compress file %s", path), err, ErrorLevelMedium)
				}
			}
		}()
	}
}

// stopCompressionWorkers stops the compression goroutines.
// It closes the compression channel and waits for all workers to finish.
func (f *FlexLog) stopCompressionWorkers() {
	if f.compressCh != nil {
		close(f.compressCh)
		f.compressWg.Wait()
		f.compressCh = nil
	}
}

// compressFile compresses the given file using the configured compression type.
// This method is called by compression workers to process queued files.
//
// Parameters:
//   - path: Path to the file to compress
//
// Returns:
//   - error: Any error encountered during compression
func (f *FlexLog) compressFile(path string) error {
	if f.compression == int(CompressionNone) {
		return nil
	}

	switch CompressionType(f.compression) {
	case CompressionGzip:
		return f.compressFileGzip(path)
	default:
		return fmt.Errorf("unsupported compression type: %v", f.compression)
	}
}

// compressFileGzip compresses a file using gzip compression.
// It creates a .gz file and removes the original file after successful compression.
//
// Parameters:
//   - path: Path to the file to compress
//
// Returns:
//   - error: Any error encountered during compression
func (f *FlexLog) compressFileGzip(path string) error {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // file doesn't exist, nothing to compress
	}

	// Compressed file path
	compressedPath := path + ".gz"

	// Open source file
	src, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening source file for compression: %w", err)
	}
	defer func() {
		if closeErr := src.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("closing source file: %w", closeErr)
		}
	}()

	// Create destination file
	dst, err := os.OpenFile(compressedPath, os.O_CREATE|os.O_WRONLY, 0644)
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
				os.Remove(compressedPath)
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
	if err := os.Remove(path); err != nil {
		// Try to restore by removing the compressed file
		os.Remove(compressedPath)
		return fmt.Errorf("removing original file after compression: %w", err)
	}

	// Track compression metric
	f.trackCompression()

	return nil
}

// queueForCompression adds a file to the compression queue.
// Files are processed asynchronously by compression workers.
// If the queue is full, the file will be skipped and an error logged.
//
// Parameters:
//   - path: Path to the file to queue for compression
func (f *FlexLog) queueForCompression(path string) {
	if f.compression != int(CompressionNone) && f.compressCh != nil {
		select {
		case f.compressCh <- path:
			// Successfully queued
		default:
			// Queue full, log to stderr
			f.logError("compress", "", fmt.Sprintf("Compression queue full, skipping compression for %s", path), nil, ErrorLevelLow)
		}
	}
}
