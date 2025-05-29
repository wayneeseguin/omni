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
	if err := flexlog.RegisterFormatterPlugin(xmlPlugin); err != nil {
		log.Fatalf("Failed to register XML formatter plugin: %v", err)
	}
	
	// Create logger with XML formatter
	logger, err := flexlog.NewBuilder().
		WithPath("/tmp/app-xml.log").
		WithCustomFormatter("xml", map[string]interface{}{
			"include_fields": true,
			"time_format":    time.RFC3339,
		}).
		Build()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()
	
	// Log some messages
	logger.WithFields(map[string]interface{}{
		"user_id": 123,
		"action":  "login",
	}).Info("User authenticated")
	
	logger.Error("Database connection failed")
	
	fmt.Println("XML formatted logs written to /tmp/app-xml.log")
	
	// Example 2: Using backend plugins
	fmt.Println("\n=== Example 2: Redis Backend Plugin ===")
	
	// Register Redis backend plugin
	redisPlugin := &RedisBackendPlugin{}
	if err := flexlog.RegisterBackendPlugin(redisPlugin); err != nil {
		log.Fatalf("Failed to register Redis backend plugin: %v", err)
	}
	
	// Create logger with Redis backend
	redisLogger, err := flexlog.NewBuilder().
		WithPath("/tmp/app-fallback.log"). // Fallback destination
		Build()
	if err != nil {
		log.Fatalf("Failed to create Redis logger: %v", err)
	}
	defer redisLogger.Close()
	
	// Add Redis destination (would connect to actual Redis in production)
	if err := redisLogger.AddDestinationWithPlugin("redis://localhost:6379/0?key=app_logs&max=1000"); err != nil {
		fmt.Printf("Warning: Failed to add Redis destination (Redis not running?): %v\n", err)
	} else {
		fmt.Println("Redis destination added successfully")
	}
	
	// Log some messages
	redisLogger.Info("Application started")
	redisLogger.WithField("component", "auth").Warn("High authentication failure rate")
	
	// Example 3: Using filter plugins
	fmt.Println("\n=== Example 3: Rate Limiter Filter Plugin ===")
	
	// Register rate limiter filter plugin
	rateLimiterPlugin := &RateLimiterFilterPlugin{}
	if err := flexlog.RegisterFilterPlugin(rateLimiterPlugin); err != nil {
		log.Fatalf("Failed to register rate limiter filter plugin: %v", err)
	}
	
	// Create filter with configuration
	filterFunc, err := rateLimiterPlugin.CreateFilter(map[string]interface{}{
		"rate":  10.0, // 10 messages per second
		"burst": 20,   // burst of 20 messages
		"per_level": map[string]interface{}{
			"debug": map[string]interface{}{
				"rate":  1.0, // 1 debug message per second
				"burst": 5,   // burst of 5
			},
		},
		"per_pattern": map[string]interface{}{
			"health": map[string]interface{}{
				"rate":  0.1, // 0.1 health check messages per second
				"burst": 1,
			},
		},
	})
	if err != nil {
		log.Fatalf("Failed to create filter: %v", err)
	}
	
	// Create logger with rate limiting
	filteredLogger, err := flexlog.NewBuilder().
		WithPath("/tmp/app-filtered.log").
		WithFilter(filterFunc).
		Build()
	if err != nil {
		log.Fatalf("Failed to create filtered logger: %v", err)
	}
	defer filteredLogger.Close()
	
	// Generate high-volume logs to test rate limiting
	fmt.Println("Generating high-volume logs to test rate limiting...")
	for i := 0; i < 100; i++ {
		filteredLogger.Debug("Debug message", "iteration", i)
		filteredLogger.Info("Processing request", "id", i)
		if i%10 == 0 {
			filteredLogger.Info("Health check") // Should be rate limited
		}
		
		// Small delay to see rate limiting in action
		time.Sleep(10 * time.Millisecond)
	}
	
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
	
	fmt.Println("\n=== Plugin System Demo Complete ===")
	fmt.Println("Check log files in /tmp/ to see the different formatting and filtering effects")
}

// Mock plugin implementations for demonstration
// In real usage, these would be in separate .so files

// XMLFormatterPlugin mock implementation
type XMLFormatterPlugin struct{}

func (p *XMLFormatterPlugin) Name() string                                              { return "xml-formatter" }
func (p *XMLFormatterPlugin) Version() string                                           { return "1.0.0" }
func (p *XMLFormatterPlugin) Initialize(config map[string]interface{}) error           { return nil }
func (p *XMLFormatterPlugin) Shutdown(ctx context.Context) error                       { return nil }
func (p *XMLFormatterPlugin) FormatName() string                                        { return "xml" }
func (p *XMLFormatterPlugin) CreateFormatter(config map[string]interface{}) (flexlog.Formatter, error) {
	return &MockXMLFormatter{}, nil
}

type MockXMLFormatter struct{}

func (f *MockXMLFormatter) Format(msg flexlog.LogMessage) ([]byte, error) {
	return []byte(fmt.Sprintf("<log><level>%s</level><message>%s</message><time>%s</time></log>",
		flexlog.LevelName(msg.Level), msg.Entry.Message, msg.Timestamp.Format(time.RFC3339))), nil
}

// RedisBackendPlugin mock implementation
type RedisBackendPlugin struct{}

func (p *RedisBackendPlugin) Name() string                                        { return "redis-backend" }
func (p *RedisBackendPlugin) Version() string                                     { return "1.0.0" }
func (p *RedisBackendPlugin) Initialize(config map[string]interface{}) error     { return nil }
func (p *RedisBackendPlugin) Shutdown(ctx context.Context) error                 { return nil }
func (p *RedisBackendPlugin) SupportedSchemes() []string                          { return []string{"redis"} }
func (p *RedisBackendPlugin) CreateBackend(uri string, config map[string]interface{}) (flexlog.Backend, error) {
	return &MockRedisBackend{}, nil
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
type RateLimiterFilterPlugin struct{}

func (p *RateLimiterFilterPlugin) Name() string                                   { return "rate-limiter-filter" }
func (p *RateLimiterFilterPlugin) Version() string                                { return "1.0.0" }
func (p *RateLimiterFilterPlugin) Initialize(config map[string]interface{}) error { return nil }
func (p *RateLimiterFilterPlugin) Shutdown(ctx context.Context) error            { return nil }
func (p *RateLimiterFilterPlugin) FilterType() string                             { return "rate-limiter" }
func (p *RateLimiterFilterPlugin) CreateFilter(config map[string]interface{}) (flexlog.FilterFunc, error) {
	// Simple mock rate limiter - always allow first 10 messages, then reject every other
	var counter int
	return func(level int, message string, fields map[string]interface{}) bool {
		counter++
		if counter <= 10 {
			return true
		}
		return counter%2 == 0 // Allow every other message after first 10
	}, nil
}