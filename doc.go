// Package flexlog provides a flexible, high-performance logging library for Go applications.
// It supports multiple concurrent destinations, structured logging, log rotation, compression,
// filtering, sampling, and process-safe file logging using Unix file locks.
//
// FlexLog is designed for production environments where reliability, performance, and
// flexibility are critical. It provides a comprehensive set of features while maintaining
// a simple and intuitive API.
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
//   - Batch writing for improved throughput
//   - Error recovery with fallback destinations
//   - Sensitive data redaction
//   - Comprehensive metrics and monitoring
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
//		WithLevel(flexlog.LevelInfo).
//		WithJSON().
//		WithDestination("/var/log/app.log",
//			flexlog.WithBatching(8192, 100*time.Millisecond)).
//		WithRotation(100*1024*1024, 10).
//		WithCompression(flexlog.CompressionGzip, 2).
//		WithErrorHandler(flexlog.StderrErrorHandler).
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
//	// With field validation
//	logger.SetStructuredLogOptions(flexlog.StructuredLogOptions{
//		EnableValidation: true,
//		RequiredFields:   []string{"request_id"},
//		FieldNormalizer:  flexlog.SnakeCaseNormalizer,
//	})
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
//   - Lazy formatting to avoid unnecessary string operations
//   - Atomic operations for metrics collection
//
// The default channel size is 100 messages, but can be configured via the
// FLEXLOG_CHANNEL_SIZE environment variable or during initialization.
//
// For high-throughput applications, consider:
//
//	config := flexlog.DefaultConfig()
//	config.ChannelSize = 10000
//	config.EnableBatching = true
//	config.BatchMaxSize = 64 * 1024  // 64KB batches
//	config.EnableLazyFormat = true
//	logger, err := flexlog.NewWithConfig(config)
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
//	// Configure error recovery
//	logger.SetRecoveryConfig(&flexlog.RecoveryConfig{
//		MaxRetries:        3,
//		RetryDelay:        100 * time.Millisecond,
//		BackoffMultiplier: 2.0,
//		FallbackPath:      "/var/log/app-fallback.log",
//		Strategy:          flexlog.RecoveryRetry,
//	})
//
//	// Check specific error types
//	if flexErr, ok := err.(*flexlog.FlexLogError); ok {
//		switch flexErr.Code {
//		case flexlog.ErrCodeFileWrite:
//			// Handle write errors
//		case flexlog.ErrCodeChannelFull:
//			// Handle backpressure
//		}
//	}
//
// Monitoring and Metrics:
//
// FlexLog provides comprehensive metrics for monitoring logging system health:
//
//	metrics := logger.GetMetrics()
//	fmt.Printf("Messages logged: %d\n", metrics.MessagesLogged[flexlog.LevelInfo])
//	fmt.Printf("Messages dropped: %d\n", metrics.MessagesDropped)
//	fmt.Printf("Queue utilization: %.2f%%\n", metrics.QueueUtilization*100)
//	fmt.Printf("Average write time: %v\n", metrics.AverageWriteTime)
//
//	// Monitor individual destinations
//	for _, dest := range metrics.Destinations {
//		fmt.Printf("Destination %s: %d bytes written, %d errors\n",
//			dest.Name, dest.BytesWritten, dest.Errors)
//	}
//
// Advanced Features:
//
//   - API Request/Response Logging with automatic redaction
//   - Plugin system for custom formatters and backends
//   - Dynamic configuration with hot reloading
//   - Context propagation for distributed tracing
//   - Sampling strategies (random, rate-based, adaptive)
//   - Field validation and normalization
//   - Automatic retry with exponential backoff
//
// For more examples and detailed documentation, see the project repository at
// https://github.com/wayneeseguin/flexlog
package flexlog