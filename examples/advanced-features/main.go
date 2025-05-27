package main

import (
	"errors"
	"log"
	"math/rand"
	"time"

	"github.com/wayneeseguin/flexlog"
)

func main() {
	// Create logger with advanced configuration
	config := flexlog.Config{
		ChannelSize:   5000,
		DefaultLevel:  flexlog.DEBUG,
		EnableMetrics: true,
		
		// Enable redaction for sensitive data
		Redaction: flexlog.RedactionConfig{
			Enabled: true,
			Patterns: []string{
				`\b\d{4}-\d{4}-\d{4}-\d{4}\b`, // Credit card pattern
				`\b\d{3}-\d{2}-\d{4}\b`,        // SSN pattern
			},
			Fields: []string{"password", "api_key", "secret"},
		},
		
		// Enable sampling to reduce log volume
		Sampling: flexlog.SamplingConfig{
			Enabled:     true,
			Rate:        0.1, // Log 10% of debug messages
			Levels:      []flexlog.LogLevel{flexlog.DEBUG},
			BurstSize:   50,
			BurstWindow: time.Minute,
		},
	}

	logger, err := flexlog.NewFlexLogWithConfig(config)
	if err != nil {
		log.Fatal(err)
	}
	defer logger.Close()

	// Add destination with rotation
	err = logger.AddDestination("rotating", flexlog.DestinationConfig{
		Backend:    flexlog.BackendFile,
		FilePath:   "logs/app.log",
		Format:     flexlog.FormatJSON,
		MinLevel:   flexlog.DEBUG,
		MaxSize:    1024 * 1024, // 1MB for demo purposes
		MaxBackups: 3,
		Compress:   true,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Demonstrate redaction
	logger.Info("User registration",
		"username", "john_doe",
		"password", "super-secret-123", // Will be redacted
		"email", "john@example.com",
		"credit_card", "1234-5678-9012-3456", // Will be redacted
	)

	// Demonstrate sampling - only ~10% of these will be logged
	for i := 0; i < 100; i++ {
		logger.Debug("High frequency debug message",
			"iteration", i,
			"random_value", rand.Float64(),
		)
	}

	// Demonstrate error with stack trace
	err = doSomethingThatFails()
	if err != nil {
		logger.ErrorWithStack("Operation failed", err,
			"operation", "data_processing",
			"retry_count", 3,
		)
	}

	// Demonstrate lazy evaluation
	for i := 0; i < 10; i++ {
		logger.Debug("Expensive calculation result",
			"result", flexlog.Lazy(func() interface{} {
				// This will only be computed if the message is actually logged
				return expensiveCalculation(i)
			}),
		)
	}

	// Demonstrate filtering
	filter := flexlog.NewFieldFilter("log_type", "audit")
	auditLogger, err := flexlog.NewFlexLogWithConfig(flexlog.Config{
		DefaultLevel: flexlog.INFO,
		Filters:      []flexlog.Filter{filter},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer auditLogger.Close()

	err = auditLogger.AddDestination("audit", flexlog.DestinationConfig{
		Backend:  flexlog.BackendFile,
		FilePath: "logs/audit.log",
		Format:   flexlog.FormatJSON,
	})
	if err != nil {
		log.Fatal(err)
	}

	// These will be logged to audit log
	auditLogger.Info("User login", "log_type", "audit", "user", "admin", "ip", "192.168.1.1")
	auditLogger.Info("Permission change", "log_type", "audit", "user", "admin", "action", "grant_access")
	
	// This won't be logged (no log_type=audit field)
	auditLogger.Info("Regular message", "user", "admin", "action", "view_dashboard")

	// Generate enough logs to trigger rotation
	for i := 0; i < 10000; i++ {
		logger.Info("Bulk message to trigger rotation",
			"index", i,
			"timestamp", time.Now().Unix(),
			"data", generateRandomString(100),
		)
	}

	// Display final metrics
	metrics := logger.GetMetrics()
	log.Printf("\nFinal Metrics:")
	log.Printf("  Total messages: %d", metrics.TotalMessages)
	log.Printf("  Dropped messages: %d", metrics.DroppedMessages)
	log.Printf("  Sampled messages: %d", metrics.SampledMessages)
	log.Printf("  Redacted fields: %d", metrics.RedactedFields)
	log.Printf("  Average latency: %v", metrics.AverageLatency)
}

func doSomethingThatFails() error {
	return errors.New("simulated failure in nested function")
}

func expensiveCalculation(n int) int {
	// Simulate expensive computation
	time.Sleep(10 * time.Millisecond)
	result := 0
	for i := 0; i < n*1000; i++ {
		result += i * i
	}
	return result
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}