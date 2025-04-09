package flexlog

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
)

// SetCompression enables or disables compression for rotated log files
func (f *FlexLog) SetCompression(compressionType CompressionType) {
	f.mu.Lock()
	defer f.mu.Unlock()

	previousType := CompressionType(f.compression)
	f.compression = int(compressionType)

	// If we're enabling compression and it wasn't enabled before
	if f.compression != int(CompressionNone) && int(previousType) == int(CompressionNone) {
		f.startCompressionWorkers()
	} else if f.compression == int(CompressionNone) && int(previousType) != int(CompressionNone) {
		f.stopCompressionWorkers()
	}
}

// SetCompressMinAge sets the minimum rotation age before compressing logs
func (f *FlexLog) SetCompressMinAge(age int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.compressMinAge = age
}

// SetCompressWorkers sets the number of compression worker goroutines
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

// startCompressionWorkers starts background goroutines for compression
func (f *FlexLog) startCompressionWorkers() {
	// Create channel for compression jobs
	f.compressCh = make(chan string, 100)

	// Start worker goroutines
	for i := 0; i < f.compressWorkers; i++ {
		go func() {
			for path := range f.compressCh {
				if err := f.compressFile(path); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to compress file %s: %v\n", path, err)
				}
			}
		}()
	}
}

// stopCompressionWorkers stops the compression goroutines
func (f *FlexLog) stopCompressionWorkers() {
	if f.compressCh != nil {
		close(f.compressCh)
		f.compressCh = nil
	}
}

// compressFile compresses the given file using the configured compression type
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

// compressFileGzip compresses a file using gzip
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
	defer src.Close()

	// Create destination file
	dst, err := os.OpenFile(compressedPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("creating compressed file: %w", err)
	}

	// Create gzip writer
	gw := gzip.NewWriter(dst)
	defer gw.Close()

	// Copy data from source to compressed destination
	_, err = io.Copy(gw, src)
	if err != nil {
		dst.Close()
		return fmt.Errorf("compressing file: %w", err)
	}

	// Close both files explicitly before removing the original
	gw.Close()
	dst.Close()
	src.Close()

	// Remove the original file
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("removing original file after compression: %w", err)
	}

	return nil
}

// queueForCompression adds a file to the compression queue
func (f *FlexLog) queueForCompression(path string) {
	if f.compression != int(CompressionNone) && f.compressCh != nil {
		select {
		case f.compressCh <- path:
			// Successfully queued
		default:
			// Queue full, log to stderr
			fmt.Fprintf(os.Stderr, "Compression queue full, skipping compression for %s\n", path)
		}
	}
}
