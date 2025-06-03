package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/wayneeseguin/omni"
)

func TestXMLFormatterPlugin_Name(t *testing.T) {
	plugin := &XMLFormatterPlugin{}
	if plugin.Name() != "xml-formatter" {
		t.Errorf("Expected plugin name 'xml-formatter', got '%s'", plugin.Name())
	}
}

func TestXMLFormatterPlugin_Version(t *testing.T) {
	plugin := &XMLFormatterPlugin{}
	if plugin.Version() != "1.0.0" {
		t.Errorf("Expected plugin version '1.0.0', got '%s'", plugin.Version())
	}
}

func TestXMLFormatterPlugin_FormatName(t *testing.T) {
	plugin := &XMLFormatterPlugin{}
	if plugin.FormatName() != "xml" {
		t.Errorf("Expected format name 'xml', got '%s'", plugin.FormatName())
	}
}

func TestXMLFormatterPlugin_Initialize(t *testing.T) {
	plugin := &XMLFormatterPlugin{}
	
	// Initially not initialized
	if plugin.initialized {
		t.Error("Plugin should not be initialized initially")
	}
	
	// Initialize
	err := plugin.Initialize(map[string]interface{}{})
	if err != nil {
		t.Errorf("Initialize failed: %v", err)
	}
	
	if !plugin.initialized {
		t.Error("Plugin should be initialized after Initialize()")
	}
}

func TestXMLFormatterPlugin_Shutdown(t *testing.T) {
	plugin := &XMLFormatterPlugin{}
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

func TestXMLFormatterPlugin_CreateFormatter_NotInitialized(t *testing.T) {
	plugin := &XMLFormatterPlugin{}
	
	// Should fail when not initialized
	_, err := plugin.CreateFormatter(map[string]interface{}{})
	if err == nil {
		t.Error("CreateFormatter should fail when plugin not initialized")
	}
}

func TestXMLFormatterPlugin_CreateFormatter_Success(t *testing.T) {
	plugin := &XMLFormatterPlugin{}
	plugin.Initialize(map[string]interface{}{})
	
	formatter, err := plugin.CreateFormatter(map[string]interface{}{})
	if err != nil {
		t.Errorf("CreateFormatter failed: %v", err)
	}
	
	if formatter == nil {
		t.Error("CreateFormatter should return a formatter")
	}
}

func TestMockXMLFormatter_Format(t *testing.T) {
	formatter := &MockXMLFormatter{}
	
	// Test with format and args
	msg1 := omni.LogMessage{
		Level:     omni.LevelInfo,
		Format:    "User %s logged in",
		Args:      []interface{}{"john"},
		Timestamp: time.Date(2023, 12, 25, 10, 30, 45, 0, time.UTC),
	}
	
	result1, err := formatter.Format(msg1)
	if err != nil {
		t.Errorf("Format failed: %v", err)
	}
	
	xml1 := string(result1)
	if !strings.Contains(xml1, "<level>INFO</level>") {
		t.Error("XML should contain level")
	}
	if !strings.Contains(xml1, "<message>User john logged in</message>") {
		t.Error("XML should contain formatted message")
	}
	if !strings.Contains(xml1, "<time>2023-12-25T10:30:45Z</time>") {
		t.Error("XML should contain timestamp")
	}
	
	// Test with Entry and fields
	msg2 := omni.LogMessage{
		Level:     omni.LevelError,
		Timestamp: time.Date(2023, 12, 25, 10, 30, 45, 0, time.UTC),
		Entry: &omni.LogEntry{
			Message: "Database error",
			Fields: map[string]interface{}{
				"error_code": "DB001",
				"retry_count": 3,
			},
		},
	}
	
	result2, err := formatter.Format(msg2)
	if err != nil {
		t.Errorf("Format with entry failed: %v", err)
	}
	
	xml2 := string(result2)
	if !strings.Contains(xml2, "<level>ERROR</level>") {
		t.Error("XML should contain ERROR level")
	}
	if !strings.Contains(xml2, "<message>Database error</message>") {
		t.Error("XML should contain entry message")
	}
	if !strings.Contains(xml2, "<fields>") {
		t.Error("XML should contain fields section")
	}
	if !strings.Contains(xml2, "<error_code>DB001</error_code>") {
		t.Error("XML should contain error_code field")
	}
	if !strings.Contains(xml2, "<retry_count>3</retry_count>") {
		t.Error("XML should contain retry_count field")
	}
}

func TestRedisBackendPlugin_Name(t *testing.T) {
	plugin := &RedisBackendPlugin{}
	if plugin.Name() != "redis-backend" {
		t.Errorf("Expected plugin name 'redis-backend', got '%s'", plugin.Name())
	}
}

func TestRedisBackendPlugin_Version(t *testing.T) {
	plugin := &RedisBackendPlugin{}
	if plugin.Version() != "1.0.0" {
		t.Errorf("Expected plugin version '1.0.0', got '%s'", plugin.Version())
	}
}

func TestRedisBackendPlugin_SupportedSchemes(t *testing.T) {
	plugin := &RedisBackendPlugin{}
	schemes := plugin.SupportedSchemes()
	
	if len(schemes) != 1 {
		t.Errorf("Expected 1 supported scheme, got %d", len(schemes))
	}
	
	if schemes[0] != "redis" {
		t.Errorf("Expected supported scheme 'redis', got '%s'", schemes[0])
	}
}

func TestRedisBackendPlugin_Initialize(t *testing.T) {
	plugin := &RedisBackendPlugin{}
	
	// Initially not initialized
	if plugin.initialized {
		t.Error("Plugin should not be initialized initially")
	}
	
	// Initialize
	err := plugin.Initialize(map[string]interface{}{})
	if err != nil {
		t.Errorf("Initialize failed: %v", err)
	}
	
	if !plugin.initialized {
		t.Error("Plugin should be initialized after Initialize()")
	}
}

func TestRedisBackendPlugin_Shutdown(t *testing.T) {
	plugin := &RedisBackendPlugin{}
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

func TestRedisBackendPlugin_CreateBackend_NotInitialized(t *testing.T) {
	plugin := &RedisBackendPlugin{}
	
	// Should fail when not initialized
	_, err := plugin.CreateBackend("redis://localhost:6379", map[string]interface{}{})
	if err == nil {
		t.Error("CreateBackend should fail when plugin not initialized")
	}
}

func TestRedisBackendPlugin_CreateBackend_MockFailure(t *testing.T) {
	plugin := &RedisBackendPlugin{}
	plugin.Initialize(map[string]interface{}{})
	
	// Should fail with mock error
	_, err := plugin.CreateBackend("redis://localhost:6379", map[string]interface{}{})
	if err == nil {
		t.Error("CreateBackend should fail with mock error")
	}
	
	if !strings.Contains(err.Error(), "Redis not available") {
		t.Errorf("Expected Redis error, got: %v", err)
	}
}

func TestMockRedisBackend_Operations(t *testing.T) {
	backend := &MockRedisBackend{}
	
	// Test Write
	testData := []byte("test log entry")
	n, err := backend.Write(testData)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Expected to write %d bytes, got %d", len(testData), n)
	}
	
	// Test Flush
	err = backend.Flush()
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}
	
	// Test Close
	err = backend.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
	
	// Test SupportsAtomic
	if !backend.SupportsAtomic() {
		t.Error("MockRedisBackend should support atomic operations")
	}
}

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
	
	// Initialize
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

func TestRateLimiterFilterPlugin_CreateFilter_WithDefaults(t *testing.T) {
	plugin := &RateLimiterFilterPlugin{}
	plugin.Initialize(map[string]interface{}{})
	
	filter, err := plugin.CreateFilter(map[string]interface{}{})
	if err != nil {
		t.Errorf("CreateFilter failed: %v", err)
	}
	
	if filter == nil {
		t.Error("CreateFilter should return a filter function")
	}
	
	// Test default behavior - should allow burst then start limiting
	allowed := 0
	blocked := 0
	
	for i := 0; i < 30; i++ {
		if filter(omni.LevelInfo, "test message", nil) {
			allowed++
		} else {
			blocked++
		}
	}
	
	if allowed < 10 {
		t.Errorf("Expected at least 10 messages to be allowed (burst), got %d", allowed)
	}
	
	if blocked == 0 {
		t.Error("Expected some messages to be blocked")
	}
}

func TestRateLimiterFilterPlugin_CreateFilter_WithConfig(t *testing.T) {
	plugin := &RateLimiterFilterPlugin{}
	plugin.Initialize(map[string]interface{}{})
	
	config := map[string]interface{}{
		"rate":  100.0, // High rate
		"burst": 5.0,   // Small burst for testing
	}
	
	filter, err := plugin.CreateFilter(config)
	if err != nil {
		t.Errorf("CreateFilter with config failed: %v", err)
	}
	
	// Test configured behavior
	allowed := 0
	blocked := 0
	
	for i := 0; i < 10; i++ {
		if filter(omni.LevelInfo, "test message", nil) {
			allowed++
		} else {
			blocked++
		}
	}
	
	if allowed < 5 {
		t.Errorf("Expected at least 5 messages to be allowed (burst=5), got %d", allowed)
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
		if filter(omni.LevelInfo, "test message", nil) {
			allowed++
		}
	}
	
	if allowed < 2 {
		t.Errorf("Expected at least 2 messages initially allowed, got %d", allowed)
	}
	
	// Should be blocked now
	if filter(omni.LevelInfo, "should be blocked", nil) {
		t.Error("Message should be blocked after exhausting burst")
	}
	
	// Wait for some tokens to refill
	time.Sleep(200 * time.Millisecond)
	
	// Should allow at least one more message
	if !filter(omni.LevelInfo, "should be allowed after refill", nil) {
		t.Error("Message should be allowed after token refill")
	}
}

func TestPluginIntegration(t *testing.T) {
	// Test plugin registration and interaction
	
	// Initialize plugins
	xmlPlugin := &XMLFormatterPlugin{}
	if err := xmlPlugin.Initialize(map[string]interface{}{}); err != nil {
		t.Fatalf("Failed to initialize XML plugin: %v", err)
	}
	
	redisPlugin := &RedisBackendPlugin{}
	if err := redisPlugin.Initialize(map[string]interface{}{}); err != nil {
		t.Fatalf("Failed to initialize Redis plugin: %v", err)
	}
	
	rateLimiterPlugin := &RateLimiterFilterPlugin{}
	if err := rateLimiterPlugin.Initialize(map[string]interface{}{}); err != nil {
		t.Fatalf("Failed to initialize rate limiter plugin: %v", err)
	}
	
	// Test XML formatter
	xmlFormatter, err := xmlPlugin.CreateFormatter(map[string]interface{}{})
	if err != nil {
		t.Errorf("Failed to create XML formatter: %v", err)
	}
	
	testMsg := omni.LogMessage{
		Level:     omni.LevelInfo,
		Format:    "Test message",
		Timestamp: time.Now(),
	}
	
	formatted, err := xmlFormatter.Format(testMsg)
	if err != nil {
		t.Errorf("Failed to format message: %v", err)
	}
	
	if !strings.Contains(string(formatted), "<log>") {
		t.Error("Formatted message should contain XML tags")
	}
	
	// Test rate limiter filter
	filter, err := rateLimiterPlugin.CreateFilter(map[string]interface{}{
		"rate":  100.0,
		"burst": 3.0,
	})
	if err != nil {
		t.Errorf("Failed to create filter: %v", err)
	}
	
	// Test filter functionality
	filterAllowed := 0
	for i := 0; i < 10; i++ {
		if filter(omni.LevelInfo, "filter test", nil) {
			filterAllowed++
		}
	}
	
	if filterAllowed < 3 {
		t.Errorf("Expected at least 3 messages to pass filter, got %d", filterAllowed)
	}
	
	// Shutdown plugins
	ctx := context.Background()
	if err := xmlPlugin.Shutdown(ctx); err != nil {
		t.Errorf("XML plugin shutdown failed: %v", err)
	}
	if err := redisPlugin.Shutdown(ctx); err != nil {
		t.Errorf("Redis plugin shutdown failed: %v", err)
	}
	if err := rateLimiterPlugin.Shutdown(ctx); err != nil {
		t.Errorf("Rate limiter plugin shutdown failed: %v", err)
	}
}

// Benchmark tests
func BenchmarkXMLFormatter_Format(b *testing.B) {
	formatter := &MockXMLFormatter{}
	
	msg := omni.LogMessage{
		Level:     omni.LevelInfo,
		Format:    "Benchmark test message",
		Args:      []interface{}{"value1", 123},
		Timestamp: time.Now(),
		Entry: &omni.LogEntry{
			Message: "Benchmark test message value1 123",
			Fields: map[string]interface{}{
				"benchmark": true,
				"iteration": 0,
			},
		},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := formatter.Format(msg)
		if err != nil {
			b.Fatalf("Format failed: %v", err)
		}
	}
}

func BenchmarkRateLimiterFilter_Allow(b *testing.B) {
	plugin := &RateLimiterFilterPlugin{}
	plugin.Initialize(map[string]interface{}{})
	
	filter, err := plugin.CreateFilter(map[string]interface{}{
		"rate":  1000000.0, // Very high rate to avoid limiting in benchmark
		"burst": 1000000.0,
	})
	if err != nil {
		b.Fatalf("CreateFilter failed: %v", err)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filter(omni.LevelInfo, "benchmark message", nil)
	}
}

func BenchmarkRedisBackend_Write(b *testing.B) {
	backend := &MockRedisBackend{}
	testData := []byte("benchmark log entry with some data")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := backend.Write(testData)
		if err != nil {
			b.Fatalf("Write failed: %v", err)
		}
	}
}