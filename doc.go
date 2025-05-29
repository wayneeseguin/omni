// Package flexlog provides a flexible, high-performance logging library for Go applications.
// It supports multiple concurrent destinations, structured logging, log rotation, compression,
// filtering, sampling, and process-safe file logging using Unix file locks.
//
// Key Features:
//
//   - Process-safe concurrent logging with Unix file locks (flock)
//   - Multiple output destinations (files, syslog)
//   - Structured logging with key-value pairs
//   - Automatic log rotation based on file size
//   - Compression of rotated logs (gzip)
//   - Flexible filtering and sampling
//   - Dynamic runtime configuration
//   - Context-aware logging with trace IDs
//   - High-performance with minimal allocations
//   - Thread-safe operations
//
// Basic Usage:
//
//	logger, err := flexlog.New("/var/log/app.log")
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer logger.Close()
//
//	logger.Info("Application started")
//	logger.Error("Failed to connect", "host", "db.example.com", "port", 5432)
//
// Using Builder Pattern:
//
//	logger, err := flexlog.NewBuilder().
//		WithPath("/var/log/app.log").
//		WithLevel(flexlog.LevelInfo).
//		WithJSON().
//		WithRotation(100*1024*1024, 10).
//		WithGzipCompression().
//		Build()
//
// Using Functional Options:
//
//	logger, err := flexlog.NewWithOptions(
//		flexlog.WithPath("/var/log/app.log"),
//		flexlog.WithLevel(flexlog.LevelInfo),
//		flexlog.WithJSON(),
//		flexlog.WithRotation(100*1024*1024, 10),
//		flexlog.WithProductionDefaults(),
//	)
//
// Multiple Destinations:
//
//	logger, err := flexlog.New("/var/log/app.log")
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Add additional destinations
//	err = logger.AddDestination("/var/log/errors.log")
//	err = logger.AddDestination("syslog://localhost:514")
//
// Structured Logging:
//
//	logger.WithFields(map[string]interface{}{
//		"user_id":    123,
//		"request_id": "abc-123",
//		"method":     "POST",
//	}).Info("Request processed")
//
//	// Or use the fluent interface
//	logger.WithField("component", "auth").
//		WithField("action", "login").
//		Info("User authenticated")
//
// Context Integration:
//
//	ctx := flexlog.WithRequestID(context.Background(), "req-123")
//	ctx = flexlog.WithUserID(ctx, "user-456")
//
//	logger.StructuredLogWithContext(ctx, flexlog.LevelInfo, 
//		"Processing request", nil)
//
// Dynamic Configuration:
//
//	// Enable configuration watching
//	err = logger.EnableDynamicConfig("/etc/myapp/logging.json", 
//		10 * time.Second)
//
//	// Configuration file can control:
//	// - Log levels
//	// - Output formats
//	// - Sampling rates
//	// - Destinations
//	// - Global fields
//
// Performance Considerations:
//
// FlexLog is designed for high-performance logging with minimal impact on application
// performance. Key optimizations include:
//
//   - Non-blocking message channel with configurable buffer size
//   - Background worker for I/O operations
//   - Efficient memory usage with buffer pooling
//   - Lock-free operations where possible
//   - Batched writes for better throughput
//
// The default channel size is 100 messages, but can be configured via the
// FLEXLOG_CHANNEL_SIZE environment variable or during initialization.
//
// Thread Safety:
//
// All FlexLog methods are thread-safe and can be called concurrently from multiple
// goroutines. The library uses appropriate synchronization mechanisms to ensure
// data consistency without sacrificing performance.
//
// Process Safety:
//
// FlexLog uses Unix file locks (flock) to ensure multiple processes can safely write
// to the same log file. This is particularly useful for applications that fork or
// when multiple instances write to shared logs.
//
// Error Handling:
//
// FlexLog provides comprehensive error handling with structured error types:
//
//	logger.SetErrorHandler(func(err flexlog.LogError) {
//		// Handle logging errors
//		fmt.Printf("[%s] %s: %v\n", err.Level, err.Source, err.Err)
//	})
//
//	// Check specific error types
//	if flexlog.IsFileError(err) {
//		// Handle file-related errors
//	}
//
// For more examples and detailed documentation, see the project repository at
// https://github.com/wayneeseguin/flexlog
package flexlog