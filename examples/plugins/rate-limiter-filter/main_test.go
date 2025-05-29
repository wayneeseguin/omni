package main

import (
	"context"
	"testing"
	"time"

	"github.com/wayneeseguin/flexlog"
)

func TestRateLimiterFilterPlugin_Name(t *testing.T) {
	plugin := &RateLimiterFilterPlugin{}
	if plugin.Name() != "rate-limiter-filter" {
		t.Errorf("Expected plugin name 'rate-limiter-filter', got '%s'", plugin.Name())
	}
}

func TestRateLimiterFilterPlugin_Version(t *testing.T) {
	plugin := &RateLimiterFilterPlugin{}
	if plugin.Version() != "1.0.0" {
		t.Errorf("Expected plugin version '1.0.0', got '%s'", plugin.Version())
	}
}

func TestRateLimiterFilterPlugin_FilterType(t *testing.T) {
	plugin := &RateLimiterFilterPlugin{}
	if plugin.FilterType() != "rate-limiter" {
		t.Errorf("Expected filter type 'rate-limiter', got '%s'", plugin.FilterType())
	}
}

func TestRateLimiterFilterPlugin_Initialize(t *testing.T) {
	plugin := &RateLimiterFilterPlugin{}
	
	// Initially not initialized
	if plugin.initialized {
		t.Error("Plugin should not be initialized initially")
	}
	
	// Initialize with empty config
	err := plugin.Initialize(map[string]interface{}{})
	if err != nil {
		t.Errorf("Initialize failed: %v", err)
	}
	
	if !plugin.initialized {
		t.Error("Plugin should be initialized after Initialize()")
	}
}

func TestRateLimiterFilterPlugin_Shutdown(t *testing.T) {
	plugin := &RateLimiterFilterPlugin{}
	plugin.initialized = true
	
	ctx := context.Background()
	err := plugin.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
	
	if plugin.initialized {
		t.Error("Plugin should not be initialized after Shutdown()")
	}
}

func TestRateLimiterFilterPlugin_CreateFilter_NotInitialized(t *testing.T) {
	plugin := &RateLimiterFilterPlugin{}
	
	// Should fail when not initialized
	_, err := plugin.CreateFilter(map[string]interface{}{})
	if err == nil {
		t.Error("CreateFilter should fail when plugin not initialized")
	}
}

func TestRateLimiterFilterPlugin_CreateFilter_Basic(t *testing.T) {
	plugin := &RateLimiterFilterPlugin{}
	plugin.Initialize(map[string]interface{}{})
	
	config := map[string]interface{}{
		"rate":  50.0,
		"burst": 100.0,
	}
	
	filter, err := plugin.CreateFilter(config)
	if err != nil {
		t.Errorf("CreateFilter failed: %v", err)
	}
	
	if filter == nil {
		t.Error("CreateFilter should return a filter function")
	}
}

func TestRateLimiterFilterPlugin_CreateFilter_WithLevelLimits(t *testing.T) {
	plugin := &RateLimiterFilterPlugin{}
	plugin.Initialize(map[string]interface{}{})
	
	config := map[string]interface{}{
		"rate":  100.0,
		"burst": 200.0,
		"per_level": map[string]interface{}{
			"ERROR": map[string]interface{}{
				"rate":  10.0,
				"burst": 20.0,
			},
			"WARN": map[string]interface{}{
				"rate":  30.0,
				"burst": 60.0,
			},
		},
	}
	
	filter, err := plugin.CreateFilter(config)
	if err != nil {
		t.Errorf("CreateFilter with level limits failed: %v", err)
	}
	
	if filter == nil {
		t.Error("CreateFilter should return a filter function")
	}
}

func TestRateLimiterFilterPlugin_CreateFilter_WithPatternLimits(t *testing.T) {
	plugin := &RateLimiterFilterPlugin{}
	plugin.Initialize(map[string]interface{}{})
	
	config := map[string]interface{}{
		"rate":  100.0,
		"burst": 200.0,
		"per_pattern": map[string]interface{}{
			"database": map[string]interface{}{
				"rate":  5.0,
				"burst": 10.0,
			},
			"api": map[string]interface{}{
				"rate":  20.0,
				"burst": 40.0,
			},
		},
	}
	
	filter, err := plugin.CreateFilter(config)
	if err != nil {
		t.Errorf("CreateFilter with pattern limits failed: %v", err)
	}
	
	if filter == nil {
		t.Error("CreateFilter should return a filter function")
	}
}

func TestParseLevelString(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"TRACE", flexlog.LevelTrace},
		{"trace", flexlog.LevelTrace},
		{"DEBUG", flexlog.LevelDebug},
		{"debug", flexlog.LevelDebug},
		{"INFO", flexlog.LevelInfo},
		{"info", flexlog.LevelInfo},
		{"WARN", flexlog.LevelWarn},
		{"warn", flexlog.LevelWarn},
		{"WARNING", flexlog.LevelWarn},
		{"warning", flexlog.LevelWarn},
		{"ERROR", flexlog.LevelError},
		{"error", flexlog.LevelError},
		{"INVALID", -1},
		{"", -1},
	}
	
	for _, test := range tests {
		result := parseLevelString(test.input)
		if result != test.expected {
			t.Errorf("parseLevelString(%s) = %d, expected %d", test.input, result, test.expected)
		}
	}
}

func TestContainsPattern(t *testing.T) {
	tests := []struct {
		message  string
		pattern  string
		expected bool
	}{
		{"Database connection failed", "database", true},
		{"DATABASE CONNECTION FAILED", "database", true},
		{"API request timeout", "database", false},
		{"Hello world", "world", true},
		{"Hello World", "world", true},
		{"", "test", false},
		{"test", "", false},
		{"short", "very long pattern", false},
	}
	
	for _, test := range tests {
		result := containsPattern(test.message, test.pattern)
		if result != test.expected {
			t.Errorf("containsPattern(%q, %q) = %v, expected %v", 
				test.message, test.pattern, result, test.expected)
		}
	}
}

func TestContainsPatternInFields(t *testing.T) {
	// Test with nil fields
	result := containsPatternInFields(nil, "test")
	if result {
		t.Error("containsPatternInFields should return false for nil fields")
	}
	
	// Test with empty fields
	result = containsPatternInFields(map[string]interface{}{}, "test")
	if result {
		t.Error("containsPatternInFields should return false for empty fields")
	}
	
	// Test with matching key
	fields := map[string]interface{}{
		"database_id": 123,
		"user":        "john",
	}
	result = containsPatternInFields(fields, "database")
	if !result {
		t.Error("containsPatternInFields should return true when pattern matches key")
	}
	
	// Test with matching value
	fields = map[string]interface{}{
		"component": "database_driver",
		"user":      "john",
	}
	result = containsPatternInFields(fields, "database")
	if !result {
		t.Error("containsPatternInFields should return true when pattern matches value")
	}
	
	// Test with no match
	fields = map[string]interface{}{
		"api_key": "secret123",
		"user":    "john",
	}
	result = containsPatternInFields(fields, "database")
	if result {
		t.Error("containsPatternInFields should return false when pattern doesn't match")
	}
}

func TestRateLimiterFilter_BasicRateLimit(t *testing.T) {
	plugin := &RateLimiterFilterPlugin{}
	plugin.Initialize(map[string]interface{}{})
	
	config := map[string]interface{}{
		"rate":  1000.0, // Very high rate for testing
		"burst": 5.0,    // Low burst for testing
	}
	
	filter, err := plugin.CreateFilter(config)
	if err != nil {
		t.Fatalf("CreateFilter failed: %v", err)
	}
	
	// Should allow first 5 messages (burst capacity)
	allowed := 0
	blocked := 0
	
	for i := 0; i < 10; i++ {
		if filter(flexlog.LevelInfo, "Test message", nil) {
			allowed++
		} else {
			blocked++
		}
	}
	
	if allowed < 5 {
		t.Errorf("Expected at least 5 messages to be allowed (burst), got %d", allowed)
	}
	
	if blocked == 0 {
		t.Error("Expected some messages to be blocked after burst")
	}
}

func TestRateLimiterFilter_PerLevelRateLimit(t *testing.T) {
	plugin := &RateLimiterFilterPlugin{}
	plugin.Initialize(map[string]interface{}{})
	
	config := map[string]interface{}{
		"rate":  1000.0, // High global rate
		"burst": 1000.0, // High global burst
		"per_level": map[string]interface{}{
			"ERROR": map[string]interface{}{
				"rate":  1000.0, // High rate
				"burst": 3.0,    // Low burst for testing
			},
		},
	}
	
	filter, err := plugin.CreateFilter(config)
	if err != nil {
		t.Fatalf("CreateFilter failed: %v", err)
	}
	
	// Test ERROR level rate limiting
	errorAllowed := 0
	errorBlocked := 0
	
	for i := 0; i < 8; i++ {
		if filter(flexlog.LevelError, "Error message", nil) {
			errorAllowed++
		} else {
			errorBlocked++
		}
	}
	
	if errorAllowed < 3 {
		t.Errorf("Expected at least 3 ERROR messages to be allowed, got %d", errorAllowed)
	}
	
	if errorBlocked == 0 {
		t.Error("Expected some ERROR messages to be blocked")
	}
	
	// Test INFO level should not be limited by ERROR level config
	infoAllowed := 0
	
	for i := 0; i < 5; i++ {
		if filter(flexlog.LevelInfo, "Info message", nil) {
			infoAllowed++
		}
	}
	
	if infoAllowed != 5 {
		t.Errorf("Expected all 5 INFO messages to be allowed, got %d", infoAllowed)
	}
}

func TestRateLimiterFilter_PatternRateLimit(t *testing.T) {
	plugin := &RateLimiterFilterPlugin{}
	plugin.Initialize(map[string]interface{}{})
	
	config := map[string]interface{}{
		"rate":  1000.0, // High global rate
		"burst": 1000.0, // High global burst
		"per_pattern": map[string]interface{}{
			"database": map[string]interface{}{
				"rate":  1000.0, // High rate
				"burst": 2.0,    // Low burst for testing
			},
		},
	}
	
	filter, err := plugin.CreateFilter(config)
	if err != nil {
		t.Fatalf("CreateFilter failed: %v", err)
	}
	
	// Test pattern-based rate limiting
	dbAllowed := 0
	dbBlocked := 0
	
	for i := 0; i < 6; i++ {
		if filter(flexlog.LevelInfo, "Database connection failed", nil) {
			dbAllowed++
		} else {
			dbBlocked++
		}
	}
	
	if dbAllowed < 2 {
		t.Errorf("Expected at least 2 database messages to be allowed, got %d", dbAllowed)
	}
	
	if dbBlocked == 0 {
		t.Error("Expected some database messages to be blocked")
	}
	
	// Test non-matching pattern should not be limited
	otherAllowed := 0
	
	for i := 0; i < 5; i++ {
		if filter(flexlog.LevelInfo, "API request succeeded", nil) {
			otherAllowed++
		}
	}
	
	if otherAllowed != 5 {
		t.Errorf("Expected all 5 non-database messages to be allowed, got %d", otherAllowed)
	}
}

func TestRateLimiterFilter_PatternInFields(t *testing.T) {
	plugin := &RateLimiterFilterPlugin{}
	plugin.Initialize(map[string]interface{}{})
	
	config := map[string]interface{}{
		"rate":  1000.0, // High global rate
		"burst": 1000.0, // High global burst
		"per_pattern": map[string]interface{}{
			"database": map[string]interface{}{
				"rate":  1000.0, // High rate
				"burst": 3.0,    // Low burst for testing
			},
		},
	}
	
	filter, err := plugin.CreateFilter(config)
	if err != nil {
		t.Fatalf("CreateFilter failed: %v", err)
	}
	
	// Test pattern matching in fields
	fieldsAllowed := 0
	fieldsBlocked := 0
	
	for i := 0; i < 7; i++ {
		fields := map[string]interface{}{
			"component": "database_driver",
			"operation": "query",
		}
		
		if filter(flexlog.LevelWarn, "Operation failed", fields) {
			fieldsAllowed++
		} else {
			fieldsBlocked++
		}
	}
	
	if fieldsAllowed < 3 {
		t.Errorf("Expected at least 3 messages with database fields to be allowed, got %d", fieldsAllowed)
	}
	
	if fieldsBlocked == 0 {
		t.Error("Expected some messages with database fields to be blocked")
	}
}

func TestRateLimiterFilter_TokenRefill(t *testing.T) {
	plugin := &RateLimiterFilterPlugin{}
	plugin.Initialize(map[string]interface{}{})
	
	config := map[string]interface{}{
		"rate":  10.0, // 10 tokens per second
		"burst": 2.0,  // 2 token burst
	}
	
	filter, err := plugin.CreateFilter(config)
	if err != nil {
		t.Fatalf("CreateFilter failed: %v", err)
	}
	
	// Exhaust the burst
	allowed := 0
	for i := 0; i < 5; i++ {
		if filter(flexlog.LevelInfo, "Test message", nil) {
			allowed++
		}
	}
	
	if allowed < 2 {
		t.Errorf("Expected at least 2 messages initially allowed, got %d", allowed)
	}
	
	// Should be blocked now
	if filter(flexlog.LevelInfo, "Should be blocked", nil) {
		t.Error("Message should be blocked after exhausting burst")
	}
	
	// Wait for some tokens to refill (100ms should give us at least 1 token at 10/sec rate)
	time.Sleep(200 * time.Millisecond)
	
	// Should allow at least one more message
	if !filter(flexlog.LevelInfo, "Should be allowed after refill", nil) {
		t.Error("Message should be allowed after token refill")
	}
}

func TestRateLimiterFilterIntegration(t *testing.T) {
	// Test the full plugin workflow
	plugin := &RateLimiterFilterPlugin{}
	
	// Initialize plugin
	err := plugin.Initialize(map[string]interface{}{})
	if err != nil {
		t.Fatalf("Failed to initialize plugin: %v", err)
	}
	
	// Create comprehensive filter
	config := map[string]interface{}{
		"rate":  100.0,
		"burst": 150.0,
		"per_level": map[string]interface{}{
			"ERROR": map[string]interface{}{
				"rate":  5.0,
				"burst": 8.0,
			},
		},
		"per_pattern": map[string]interface{}{
			"critical": map[string]interface{}{
				"rate":  2.0,
				"burst": 3.0,
			},
		},
	}
	
	filter, err := plugin.CreateFilter(config)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}
	
	// Test various scenarios
	scenarios := []struct {
		level    int
		message  string
		fields   map[string]interface{}
		name     string
		maxTests int
		minAllow int
	}{
		{
			level:    flexlog.LevelInfo,
			message:  "Normal info message",
			fields:   nil,
			name:     "normal info",
			maxTests: 10,
			minAllow: 10, // Should all be allowed (high global limit)
		},
		{
			level:    flexlog.LevelError,
			message:  "Error occurred",
			fields:   nil,
			name:     "error level",
			maxTests: 15,
			minAllow: 8, // Should hit ERROR level limit
		},
		{
			level:    flexlog.LevelWarn,
			message:  "Critical system failure",
			fields:   nil,
			name:     "critical pattern",
			maxTests: 8,
			minAllow: 3, // Should hit pattern limit
		},
	}
	
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			allowed := 0
			for i := 0; i < scenario.maxTests; i++ {
				if filter(scenario.level, scenario.message, scenario.fields) {
					allowed++
				}
			}
			
			if allowed < scenario.minAllow {
				t.Errorf("Expected at least %d messages allowed for %s, got %d", 
					scenario.minAllow, scenario.name, allowed)
			}
		})
	}
	
	// Shutdown plugin
	ctx := context.Background()
	err = plugin.Shutdown(ctx)
	if err != nil {
		t.Errorf("Failed to shutdown plugin: %v", err)
	}
}

// Benchmark tests
func BenchmarkRateLimiterFilter_Basic(b *testing.B) {
	plugin := &RateLimiterFilterPlugin{}
	plugin.Initialize(map[string]interface{}{})
	
	config := map[string]interface{}{
		"rate":  1000000.0, // Very high rate to avoid limiting in benchmark
		"burst": 1000000.0,
	}
	
	filter, err := plugin.CreateFilter(config)
	if err != nil {
		b.Fatalf("CreateFilter failed: %v", err)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filter(flexlog.LevelInfo, "Benchmark message", nil)
	}
}

func BenchmarkRateLimiterFilter_WithPatterns(b *testing.B) {
	plugin := &RateLimiterFilterPlugin{}
	plugin.Initialize(map[string]interface{}{})
	
	config := map[string]interface{}{
		"rate":  1000000.0,
		"burst": 1000000.0,
		"per_pattern": map[string]interface{}{
			"benchmark": map[string]interface{}{
				"rate":  1000000.0,
				"burst": 1000000.0,
			},
		},
	}
	
	filter, err := plugin.CreateFilter(config)
	if err != nil {
		b.Fatalf("CreateFilter failed: %v", err)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filter(flexlog.LevelInfo, "Benchmark test message", nil)
	}
}

func BenchmarkRateLimiterFilter_WithFields(b *testing.B) {
	plugin := &RateLimiterFilterPlugin{}
	plugin.Initialize(map[string]interface{}{})
	
	config := map[string]interface{}{
		"rate":  1000000.0,
		"burst": 1000000.0,
		"per_pattern": map[string]interface{}{
			"benchmark": map[string]interface{}{
				"rate":  1000000.0,
				"burst": 1000000.0,
			},
		},
	}
	
	filter, err := plugin.CreateFilter(config)
	if err != nil {
		b.Fatalf("CreateFilter failed: %v", err)
	}
	
	fields := map[string]interface{}{
		"component": "benchmark_test",
		"iteration": 0,
		"timestamp": time.Now().Unix(),
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filter(flexlog.LevelInfo, "Benchmark message", fields)
	}
}