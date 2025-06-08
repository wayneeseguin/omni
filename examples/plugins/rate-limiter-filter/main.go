// Package main implements a rate limiter filter plugin for Omni
package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/wayneeseguin/omni/pkg/omni"
)

// RateLimiterFilterPlugin implements the FilterPlugin interface
type RateLimiterFilterPlugin struct {
	initialized bool
	config      map[string]interface{}
}

// RateLimiterFilter implements rate limiting for log messages
type RateLimiterFilter struct {
	mu         sync.Mutex
	perLevel   map[int]*rateLimiter    // per-level rate limiters
	perPattern map[string]*rateLimiter // per-pattern rate limiters
	global     *rateLimiter            // global rate limiter
}

// rateLimiter implements token bucket algorithm
type rateLimiter struct {
	rate       float64
	burst      int
	tokens     float64
	lastRefill time.Time
}

// Name returns the plugin name
func (p *RateLimiterFilterPlugin) Name() string {
	return "rate-limiter-filter"
}

// Version returns the plugin version
func (p *RateLimiterFilterPlugin) Version() string {
	return "1.0.0"
}

// Initialize initializes the plugin with configuration
func (p *RateLimiterFilterPlugin) Initialize(config map[string]interface{}) error {
	p.config = config
	p.initialized = true
	return nil
}

// Shutdown cleans up plugin resources
func (p *RateLimiterFilterPlugin) Shutdown(ctx context.Context) error {
	p.initialized = false
	return nil
}

// CreateFilter creates a new rate limiter filter instance
func (p *RateLimiterFilterPlugin) CreateFilter(config map[string]interface{}) (omni.FilterFunc, error) {
	if !p.initialized {
		return nil, fmt.Errorf("plugin not initialized")
	}

	// Set default global rate and burst
	globalRate := 100.0 // default: 100 messages per second
	globalBurst := 200  // default: burst of 200 messages

	// Apply configuration
	if val, ok := config["rate"].(float64); ok {
		globalRate = val
	}

	if val, ok := config["burst"].(float64); ok {
		globalBurst = int(val)
	}

	filter := &RateLimiterFilter{
		perLevel:   make(map[int]*rateLimiter),
		perPattern: make(map[string]*rateLimiter),
	}

	// Configure per-level limits
	if levels, ok := config["per_level"].(map[string]interface{}); ok {
		for levelStr, limitConfig := range levels {
			level := parseLevelString(levelStr)
			if level >= 0 {
				if limitMap, ok := limitConfig.(map[string]interface{}); ok {
					limiter := &rateLimiter{
						rate:  100.0,
						burst: 200,
					}

					if rate, ok := limitMap["rate"].(float64); ok {
						limiter.rate = rate
					}
					if burst, ok := limitMap["burst"].(float64); ok {
						limiter.burst = int(burst)
					}

					limiter.tokens = float64(limiter.burst)
					limiter.lastRefill = time.Now()
					filter.perLevel[level] = limiter
				}
			}
		}
	}

	// Configure per-pattern limits
	if patterns, ok := config["per_pattern"].(map[string]interface{}); ok {
		for pattern, limitConfig := range patterns {
			if limitMap, ok := limitConfig.(map[string]interface{}); ok {
				limiter := &rateLimiter{
					rate:  100.0,
					burst: 200,
				}

				if rate, ok := limitMap["rate"].(float64); ok {
					limiter.rate = rate
				}
				if burst, ok := limitMap["burst"].(float64); ok {
					limiter.burst = int(burst)
				}

				limiter.tokens = float64(limiter.burst)
				limiter.lastRefill = time.Now()
				filter.perPattern[pattern] = limiter
			}
		}
	}

	// Initialize global rate limiter
	filter.global = &rateLimiter{
		rate:       globalRate,
		burst:      globalBurst,
		tokens:     float64(globalBurst),
		lastRefill: time.Now(),
	}

	return filter.shouldAllow, nil
}

// FilterType returns the filter type name
func (p *RateLimiterFilterPlugin) FilterType() string {
	return "rate-limiter"
}

// shouldAllow determines if a message should be allowed based on rate limits
func (f *RateLimiterFilter) shouldAllow(level int, message string, fields map[string]interface{}) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	now := time.Now()

	// Check global rate limit first
	if !f.checkRateLimit(f.global, now) {
		return false
	}

	// Check per-level rate limit
	if limiter, exists := f.perLevel[level]; exists {
		if !f.checkRateLimit(limiter, now) {
			return false
		}
	}

	// Check per-pattern rate limits
	for pattern, limiter := range f.perPattern {
		if containsPattern(message, pattern) || containsPatternInFields(fields, pattern) {
			if !f.checkRateLimit(limiter, now) {
				return false
			}
		}
	}

	return true
}

// checkRateLimit implements token bucket algorithm
func (f *RateLimiterFilter) checkRateLimit(limiter *rateLimiter, now time.Time) bool {
	// Refill tokens based on elapsed time
	elapsed := now.Sub(limiter.lastRefill).Seconds()
	limiter.tokens += elapsed * limiter.rate

	// Cap at burst limit
	if limiter.tokens > float64(limiter.burst) {
		limiter.tokens = float64(limiter.burst)
	}

	limiter.lastRefill = now

	// Check if we have tokens available
	if limiter.tokens >= 1.0 {
		limiter.tokens -= 1.0
		return true
	}

	return false
}

// containsPattern checks if message contains pattern
func containsPattern(message, pattern string) bool {
	// Simple substring match - could be enhanced with regex
	return message != "" && pattern != "" &&
		len(message) >= len(pattern) &&
		findSubstring(message, pattern)
}

// findSubstring performs case-insensitive substring search
func findSubstring(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}

	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if toLower(s[i+j]) != toLower(substr[j]) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// toLower converts character to lowercase
func toLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}

// parseLevelString converts level string to level integer
func parseLevelString(levelStr string) int {
	switch levelStr {
	case "TRACE", "trace":
		return omni.LevelTrace
	case "DEBUG", "debug":
		return omni.LevelDebug
	case "INFO", "info":
		return omni.LevelInfo
	case "WARN", "warn", "WARNING", "warning":
		return omni.LevelWarn
	case "ERROR", "error":
		return omni.LevelError
	default:
		return -1 // Invalid level
	}
}

// containsPatternInFields checks if any field contains the pattern
func containsPatternInFields(fields map[string]interface{}, pattern string) bool {
	if fields == nil {
		return false
	}

	for key, value := range fields {
		if containsPattern(key, pattern) {
			return true
		}

		if str := fmt.Sprintf("%v", value); containsPattern(str, pattern) {
			return true
		}
	}

	return false
}

// OmniPlugin is the plugin entry point
var OmniPlugin = &RateLimiterFilterPlugin{}

func main() {
	// Example usage demonstrating the Rate Limiter Filter plugin
	fmt.Println("Rate Limiter Filter Plugin")
	fmt.Printf("Name: %s\n", OmniPlugin.Name())
	fmt.Printf("Version: %s\n", OmniPlugin.Version())
	fmt.Printf("Filter type: %s\n", OmniPlugin.FilterType())

	// Initialize the plugin
	if err := OmniPlugin.Initialize(map[string]interface{}{}); err != nil {
		fmt.Printf("Failed to initialize plugin: %v\n", err)
		return
	}

	// Demo: Create a basic rate limiter filter
	fmt.Println("\nDemo: Creating basic rate limiter filter...")

	basicConfig := map[string]interface{}{
		"rate":  10.0, // 10 messages per second
		"burst": 5.0,  // burst capacity of 5 (smaller for clearer demo)
	}

	basicFilter, err := OmniPlugin.CreateFilter(basicConfig)
	if err != nil {
		fmt.Printf("Failed to create basic filter: %v\n", err)
		return
	}

	// Test basic rate limiting
	fmt.Println("Testing basic rate limiting (should allow first 5, then start limiting):")
	allowed := 0
	blocked := 0

	for i := 0; i < 10; i++ {
		result := basicFilter(omni.LevelInfo, fmt.Sprintf("Message %d", i+1), nil)
		fmt.Printf("  Message %d: %v\n", i+1, result)
		if result {
			allowed++
		} else {
			blocked++
		}
	}

	fmt.Printf("  Total - Allowed: %d, Blocked: %d\n", allowed, blocked)

	// Demo: Create advanced rate limiter with per-level limits
	fmt.Println("\nDemo: Creating advanced rate limiter with per-level limits...")

	advancedConfig := map[string]interface{}{
		"rate":  100.0, // Global: 100 messages per second
		"burst": 200.0, // Global burst capacity
		"per_level": map[string]interface{}{
			"ERROR": map[string]interface{}{
				"rate":  5.0, // ERROR: 5 messages per second
				"burst": 3.0, // ERROR burst capacity (smaller for demo)
			},
			"WARN": map[string]interface{}{
				"rate":  20.0, // WARN: 20 messages per second
				"burst": 5.0,  // WARN burst capacity (smaller for demo)
			},
		},
		"per_pattern": map[string]interface{}{
			"database": map[string]interface{}{
				"rate":  2.0, // Database messages: 2 per second
				"burst": 2.0, // Database burst capacity (smaller for demo)
			},
		},
	}

	advancedFilter, err := OmniPlugin.CreateFilter(advancedConfig)
	if err != nil {
		fmt.Printf("Failed to create advanced filter: %v\n", err)
		return
	}

	// Test per-level rate limiting
	fmt.Println("Testing per-level rate limiting for ERROR messages (should allow 3, then block):")
	errorAllowed := 0
	errorBlocked := 0

	for i := 0; i < 8; i++ {
		result := advancedFilter(omni.LevelError, fmt.Sprintf("Error message %d", i+1), nil)
		fmt.Printf("  ERROR %d: %v\n", i+1, result)
		if result {
			errorAllowed++
		} else {
			errorBlocked++
		}
	}

	fmt.Printf("  ERROR Total - Allowed: %d, Blocked: %d\n", errorAllowed, errorBlocked)

	// Test pattern-based rate limiting
	fmt.Println("Testing pattern-based rate limiting for 'database' messages (should allow 2, then block):")
	dbAllowed := 0
	dbBlocked := 0

	for i := 0; i < 6; i++ {
		result := advancedFilter(omni.LevelInfo, fmt.Sprintf("Database connection %d failed", i+1), nil)
		fmt.Printf("  DB %d: %v\n", i+1, result)
		if result {
			dbAllowed++
		} else {
			dbBlocked++
		}
	}

	fmt.Printf("  Database pattern - Allowed: %d, Blocked: %d\n", dbAllowed, dbBlocked)

	// Test with fields
	fmt.Println("Testing rate limiting with structured fields:")
	fieldsAllowed := 0
	fieldsBlocked := 0

	for i := 0; i < 8; i++ {
		fields := map[string]interface{}{
			"component": "database",
			"operation": "query",
			"user_id":   12345,
		}

		if advancedFilter(omni.LevelWarn, fmt.Sprintf("Query timeout %d", i+1), fields) {
			fieldsAllowed++
		} else {
			fieldsBlocked++
		}
	}

	fmt.Printf("  With fields - Allowed: %d, Blocked: %d\n", fieldsAllowed, fieldsBlocked)

	// Wait a bit to allow token bucket to refill
	fmt.Println("\nWaiting 1 second for token bucket refill...")
	time.Sleep(1 * time.Second)

	// Test again after refill
	fmt.Println("Testing again after token refill:")
	postRefillAllowed := 0

	for i := 0; i < 5; i++ {
		if basicFilter(omni.LevelInfo, fmt.Sprintf("Post-refill message %d", i+1), nil) {
			postRefillAllowed++
		}
	}

	fmt.Printf("  Post-refill allowed: %d\n", postRefillAllowed)

	// Shutdown the plugin
	ctx := context.Background()
	if err := OmniPlugin.Shutdown(ctx); err != nil {
		fmt.Printf("Failed to shutdown plugin: %v\n", err)
	} else {
		fmt.Println("\nPlugin shutdown successfully")
	}
}
