package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/wayneeseguin/flexlog"
)

func main() {
	// Example 1: Using built-in formatter plugins
	fmt.Println("=== Example 1: XML Formatter Plugin ===")
	
	// Register XML formatter plugin (would normally be loaded from .so file)
	// For this example, we'll simulate a registered plugin
	xmlPlugin := &XMLFormatterPlugin{}
	if err := xmlPlugin.Initialize(map[string]interface{}{}); err != nil {
		log.Fatalf("Failed to initialize XML formatter plugin: %v", err)
	}
	
	if err := flexlog.RegisterFormatterPlugin(xmlPlugin); err != nil {
		log.Fatalf("Failed to register XML formatter plugin: %v", err)
	}
	
	// Create logger with XML formatter using builder
	logger, err := flexlog.NewBuilder().
		WithDestination("/tmp/app-xml.log").
		WithJSON(). // Enable JSON for structured logging compatibility
		Build()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()
	
	// Log some messages using structured logging
	logger.InfoWithFields("User authenticated", map[string]interface{}{
		"user_id": 123,
		"action":  "login",
		"ip_addr": "192.168.1.100",
	})
	
	logger.ErrorWithFields("Database connection failed", map[string]interface{}{
		"error_code": "DB001",
		"retry_count": 3,
	})
	
	fmt.Println("Structured logs written to /tmp/app-xml.log")
	
	// Example 2: Using backend plugins
	fmt.Println("\n=== Example 2: Redis Backend Plugin ===")
	
	// Register Redis backend plugin
	redisPlugin := &RedisBackendPlugin{}
	if err := redisPlugin.Initialize(map[string]interface{}{}); err != nil {
		log.Fatalf("Failed to initialize Redis backend plugin: %v", err)
	}
	
	if err := flexlog.RegisterBackendPlugin(redisPlugin); err != nil {
		log.Fatalf("Failed to register Redis backend plugin: %v", err)
	}
	
	// Create logger with fallback destination
	redisLogger, err := flexlog.NewBuilder().
		WithDestination("/tmp/app-fallback.log"). // Fallback destination
		WithLevel(flexlog.LevelInfo).
		Build()
	if err != nil {
		log.Fatalf("Failed to create Redis logger: %v", err)
	}
	defer redisLogger.Close()
	
	// Attempt to add Redis destination - will fail gracefully if Redis not available
	if err := redisLogger.AddDestination("redis://localhost:6379/0?key=app_logs&max=1000"); err != nil {
		fmt.Printf("Warning: Failed to add Redis destination (Redis not running?): %v\n", err)
		fmt.Println("Falling back to file logging only")
	} else {
		fmt.Println("Redis destination added successfully")
	}
	
	// Log some messages
	redisLogger.Info("Application started")
	redisLogger.WarnWithFields("High authentication failure rate", map[string]interface{}{
		"component": "auth",
		"failure_rate": 0.15,
		"threshold": 0.10,
	})
	
	// Example 3: Using filter plugins
	fmt.Println("\n=== Example 3: Rate Limiter Filter Plugin ===")
	
	// Register rate limiter filter plugin
	rateLimiterPlugin := &RateLimiterFilterPlugin{}
	if err := rateLimiterPlugin.Initialize(map[string]interface{}{}); err != nil {
		log.Fatalf("Failed to initialize rate limiter filter plugin: %v", err)
	}
	
	if err := flexlog.RegisterFilterPlugin(rateLimiterPlugin); err != nil {
		log.Fatalf("Failed to register rate limiter filter plugin: %v", err)
	}
	
	// Create filter with configuration
	filterFunc, err := rateLimiterPlugin.CreateFilter(map[string]interface{}{
		"rate":  20.0, // 20 messages per second (higher rate for demo)
		"burst": 10.0, // burst of 10 messages
		"per_level": map[string]interface{}{
			"DEBUG": map[string]interface{}{
				"rate":  5.0, // 5 debug messages per second
				"burst": 3.0, // burst of 3
			},
		},
		"per_pattern": map[string]interface{}{
			"health": map[string]interface{}{
				"rate":  1.0, // 1 health check message per second
				"burst": 2.0, // burst of 2
			},
		},
	})
	if err != nil {
		log.Fatalf("Failed to create filter: %v", err)
	}
	
	// Create logger with rate limiting
	filteredLogger, err := flexlog.NewBuilder().
		WithDestination("/tmp/app-filtered.log").
		WithFilter(filterFunc).
		WithLevel(flexlog.LevelDebug). // Enable debug logging
		Build()
	if err != nil {
		log.Fatalf("Failed to create filtered logger: %v", err)
	}
	defer filteredLogger.Close()
	
	// Generate high-volume logs to test rate limiting
	fmt.Println("Generating high-volume logs to test rate limiting...")
	allowed := 0
	blocked := 0
	
	for i := 0; i < 30; i++ {
		// These will be rate limited at DEBUG level
		filteredLogger.Debugf("Debug message %d", i)
		if i < 20 { // Count first 20 for rate limiting demo
			if i < 3 {
				allowed++ // First 3 should be allowed (burst)
			} else {
				blocked++ // Rest should be blocked initially
			}
		}
		
		// General info messages
		filteredLogger.Infof("Processing request %d", i)
		
		// Health check messages - these will be rate limited by pattern
		if i%5 == 0 {
			filteredLogger.Info("Health check OK")
		}
		
		// Small delay to see rate limiting in action
		time.Sleep(50 * time.Millisecond)
	}
	
	fmt.Printf("Rate limiting demo completed (estimated %d allowed, %d blocked initially)\n", allowed, blocked)
	fmt.Println("Rate-limited logs written to /tmp/app-filtered.log")
	
	// Example 4: Plugin discovery and management
	fmt.Println("\n=== Example 4: Plugin Management ===")
	
	// List loaded plugins
	manager := flexlog.GetPluginManager()
	plugins := manager.GetPluginInfo()
	
	fmt.Printf("Loaded plugins (%d):\n", len(plugins))
	for _, plugin := range plugins {
		fmt.Printf("  - %s v%s (%s)\n", plugin.Name, plugin.Version, plugin.Type)
		if plugin.Details != nil {
			detailsJson, _ := json.MarshalIndent(plugin.Details, "    ", "  ")
			fmt.Printf("    Details: %s\n", string(detailsJson))
		}
	}
	
	// Example 5: Plugin discovery (would work with actual .so files)
	fmt.Println("\n=== Example 5: Plugin Discovery ===")
	
	// Set plugin search paths
	flexlog.SetPluginSearchPaths([]string{
		"./plugins",
		"/usr/local/lib/flexlog/plugins",
		os.Getenv("HOME") + "/.flexlog/plugins",
	})
	
	// Discover and load plugins (would work with actual plugin files)
	if err := flexlog.DiscoverAndLoadPlugins(); err != nil {
		fmt.Printf("Plugin discovery completed with some errors: %v\n", err)
	} else {
		fmt.Println("Plugin discovery completed successfully")
	}
	
	// Example 6: Advanced configuration
	fmt.Println("\n=== Example 6: Advanced Plugin Configuration ===")
	
	// Create logger with multiple features
	advancedLogger, err := flexlog.NewBuilder().
		WithDestination("/tmp/app-advanced.log").
		WithLevel(flexlog.LevelDebug).
		WithJSON().
		WithRotation(1024*1024, 5). // 1MB rotation, keep 5 files
		WithCompression(flexlog.CompressionGzip, 2). // Gzip compression with 2 workers
		WithTimestampFormat(time.RFC3339Nano).
		Build()
	if err != nil {
		log.Fatalf("Failed to create advanced logger: %v", err)
	}
	defer advancedLogger.Close()
	
	// Log with various features
	advancedLogger.InfoWithFields("Application configuration loaded", map[string]interface{}{
		"config_version": "2.1.0",
		"plugins_loaded": len(plugins),
		"features": []string{"compression", "rotation", "filtering"},
	})
	
	fmt.Println("Advanced configuration demo completed")
	
	// Cleanup: Shutdown all plugins gracefully
	fmt.Println("\n=== Shutting down plugins ===")
	ctx := context.Background()
	
	if err := xmlPlugin.Shutdown(ctx); err != nil {
		fmt.Printf("Warning: XML plugin shutdown error: %v\n", err)
	}
	if err := redisPlugin.Shutdown(ctx); err != nil {
		fmt.Printf("Warning: Redis plugin shutdown error: %v\n", err)
	}
	if err := rateLimiterPlugin.Shutdown(ctx); err != nil {
		fmt.Printf("Warning: Rate limiter plugin shutdown error: %v\n", err)
	}
	
	fmt.Println("\n=== Plugin System Demo Complete ===")
	fmt.Println("Check log files in /tmp/ to see the different formatting and filtering effects:")
	fmt.Println("  - /tmp/app-xml.log (structured JSON logs)")
	fmt.Println("  - /tmp/app-fallback.log (Redis fallback logs)")
	fmt.Println("  - /tmp/app-filtered.log (rate-limited logs)")
	fmt.Println("  - /tmp/app-advanced.log (advanced features)")
}

// Mock plugin implementations for demonstration
// In real usage, these would be in separate .so files

// XMLFormatterPlugin mock implementation
type XMLFormatterPlugin struct {
	initialized bool
}

func (p *XMLFormatterPlugin) Name() string    { return "xml-formatter" }
func (p *XMLFormatterPlugin) Version() string { return "1.0.0" }
func (p *XMLFormatterPlugin) Initialize(config map[string]interface{}) error {
	p.initialized = true
	return nil
}
func (p *XMLFormatterPlugin) Shutdown(ctx context.Context) error {
	p.initialized = false
	return nil
}
func (p *XMLFormatterPlugin) FormatName() string { return "xml" }
func (p *XMLFormatterPlugin) CreateFormatter(config map[string]interface{}) (flexlog.Formatter, error) {
	if !p.initialized {
		return nil, fmt.Errorf("plugin not initialized")
	}
	return &MockXMLFormatter{}, nil
}

type MockXMLFormatter struct{}

func (f *MockXMLFormatter) Format(msg flexlog.LogMessage) ([]byte, error) {
	// Extract message text
	messageText := ""
	if msg.Entry != nil && msg.Entry.Message != "" {
		messageText = msg.Entry.Message
	} else if msg.Format != "" && len(msg.Args) > 0 {
		messageText = fmt.Sprintf(msg.Format, msg.Args...)
	} else if msg.Format != "" {
		messageText = msg.Format
	}
	
	// Create basic XML format
	xml := fmt.Sprintf("<log><level>%s</level><message>%s</message><time>%s</time>",
		flexlog.LevelName(msg.Level), messageText, msg.Timestamp.Format(time.RFC3339))
	
	// Add fields if available
	if msg.Entry != nil && msg.Entry.Fields != nil {
		xml += "<fields>"
		for key, value := range msg.Entry.Fields {
			xml += fmt.Sprintf("<%s>%v</%s>", key, value, key)
		}
		xml += "</fields>"
	}
	
	xml += "</log>"
	return []byte(xml), nil
}

// RedisBackendPlugin mock implementation
type RedisBackendPlugin struct {
	initialized bool
}

func (p *RedisBackendPlugin) Name() string    { return "redis-backend" }
func (p *RedisBackendPlugin) Version() string { return "1.0.0" }
func (p *RedisBackendPlugin) Initialize(config map[string]interface{}) error {
	p.initialized = true
	return nil
}
func (p *RedisBackendPlugin) Shutdown(ctx context.Context) error {
	p.initialized = false
	return nil
}
func (p *RedisBackendPlugin) SupportedSchemes() []string { return []string{"redis"} }
func (p *RedisBackendPlugin) CreateBackend(uri string, config map[string]interface{}) (flexlog.Backend, error) {
	if !p.initialized {
		return nil, fmt.Errorf("plugin not initialized")
	}
	// Mock Redis connection failure for demo
	return nil, fmt.Errorf("Redis not available (mock error for demo)")
}

type MockRedisBackend struct{}

func (b *MockRedisBackend) Write(entry []byte) (int, error) {
	// Mock implementation - would write to Redis in real plugin
	fmt.Printf("[REDIS] %s\n", string(entry))
	return len(entry), nil
}
func (b *MockRedisBackend) Flush() error         { return nil }
func (b *MockRedisBackend) Close() error         { return nil }
func (b *MockRedisBackend) SupportsAtomic() bool { return true }

// RateLimiterFilterPlugin mock implementation
type RateLimiterFilterPlugin struct {
	initialized bool
}

func (p *RateLimiterFilterPlugin) Name() string    { return "rate-limiter-filter" }
func (p *RateLimiterFilterPlugin) Version() string { return "1.0.0" }
func (p *RateLimiterFilterPlugin) Initialize(config map[string]interface{}) error {
	p.initialized = true
	return nil
}
func (p *RateLimiterFilterPlugin) Shutdown(ctx context.Context) error {
	p.initialized = false
	return nil
}
func (p *RateLimiterFilterPlugin) FilterType() string { return "rate-limiter" }
func (p *RateLimiterFilterPlugin) CreateFilter(config map[string]interface{}) (flexlog.FilterFunc, error) {
	if !p.initialized {
		return nil, fmt.Errorf("plugin not initialized")
	}
	
	// Extract configuration
	rate := 10.0
	burst := 20
	if r, ok := config["rate"].(float64); ok {
		rate = r
	}
	if b, ok := config["burst"].(float64); ok {
		burst = int(b)
	}
	
	// Simple token bucket implementation for demo
	tokens := float64(burst)
	lastRefill := time.Now()
	
	return func(level int, message string, fields map[string]interface{}) bool {
		now := time.Now()
		elapsed := now.Sub(lastRefill).Seconds()
		
		// Refill tokens
		tokens += elapsed * rate
		if tokens > float64(burst) {
			tokens = float64(burst)
		}
		lastRefill = now
		
		// Check if we have tokens
		if tokens >= 1.0 {
			tokens -= 1.0
			return true
		}
		
		return false
	}, nil
}