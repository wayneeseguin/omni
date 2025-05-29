// Package main implements a rate limiter filter plugin for FlexLog
package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/wayneeseguin/flexlog"
)

// RateLimiterFilterPlugin implements the FilterPlugin interface
type RateLimiterFilterPlugin struct {
	initialized bool
	config      map[string]interface{}
}

// RateLimiterFilter implements rate limiting for log messages
type RateLimiterFilter struct {
	mu           sync.Mutex
	rate         float64           // messages per second
	burst        int               // burst capacity
	tokens       float64           // current tokens
	lastRefill   time.Time         // last token refill time
	perLevel     map[int]*rateLimiter // per-level rate limiters
	perPattern   map[string]*rateLimiter // per-pattern rate limiters
	global       *rateLimiter      // global rate limiter
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
func (p *RateLimiterFilterPlugin) CreateFilter(config map[string]interface{}) (flexlog.FilterFunc, error) {
	if !p.initialized {
		return nil, fmt.Errorf("plugin not initialized")
	}
	
	filter := &RateLimiterFilter{
		rate:       100.0, // default: 100 messages per second
		burst:      200,   // default: burst of 200 messages
		perLevel:   make(map[int]*rateLimiter),
		perPattern: make(map[string]*rateLimiter),
	}
	
	// Apply configuration
	if val, ok := config["rate"].(float64); ok {
		filter.rate = val
	}
	
	if val, ok := config["burst"].(float64); ok {
		filter.burst = int(val)
	}
	
	// Configure per-level limits
	if levels, ok := config["per_level"].(map[string]interface{}); ok {
		for levelStr, limitConfig := range levels {
			level := flexlog.ParseLevel(levelStr)
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
		rate:       filter.rate,
		burst:      filter.burst,
		tokens:     float64(filter.burst),
		lastRefill: time.Now(),
	}
	
	return filter.shouldAllow, nil
}

// FilterType returns the filter type name
func (p *RateLimiterFilterPlugin) FilterType() string {
	return "rate-limiter"
}

// shouldAllow determines if a message should be allowed based on rate limits
func (f *RateLimiterFilter) shouldAllow(msg flexlog.LogMessage) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	now := time.Now()
	
	// Check global rate limit first
	if !f.checkRateLimit(f.global, now) {
		return false
	}
	
	// Check per-level rate limit
	if limiter, exists := f.perLevel[msg.Level]; exists {
		if !f.checkRateLimit(limiter, now) {
			return false
		}
	}
	
	// Check per-pattern rate limits
	for pattern, limiter := range f.perPattern {
		if containsPattern(msg.Message, pattern) || containsPatternInFields(msg.Fields, pattern) {
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

// FlexLogPlugin is the plugin entry point
var FlexLogPlugin = &RateLimiterFilterPlugin{}

func main() {
	// This is a plugin, so main() is not used when loaded as a plugin
	fmt.Println("Rate Limiter Filter Plugin")
	fmt.Printf("Name: %s\n", FlexLogPlugin.Name())
	fmt.Printf("Version: %s\n", FlexLogPlugin.Version())
	fmt.Printf("Filter type: %s\n", FlexLogPlugin.FilterType())
}