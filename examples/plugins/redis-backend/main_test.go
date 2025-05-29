package main

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

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
	
	// Initialize with empty config
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
	
	expectedMsg := "plugin not initialized"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error to contain '%s', got: %v", expectedMsg, err)
	}
}

func TestRedisBackendPlugin_CreateBackend_InvalidScheme(t *testing.T) {
	plugin := &RedisBackendPlugin{}
	plugin.Initialize(map[string]interface{}{})
	
	// Should fail with invalid scheme
	_, err := plugin.CreateBackend("http://localhost:8080", map[string]interface{}{})
	if err == nil {
		t.Error("CreateBackend should fail with invalid scheme")
	}
	
	expectedMsg := "unsupported scheme"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error to contain '%s', got: %v", expectedMsg, err)
	}
}

func TestRedisBackendPlugin_CreateBackend_InvalidURI(t *testing.T) {
	plugin := &RedisBackendPlugin{}
	plugin.Initialize(map[string]interface{}{})
	
	// Should fail with invalid URI
	_, err := plugin.CreateBackend("redis://[invalid-uri", map[string]interface{}{})
	if err == nil {
		t.Error("CreateBackend should fail with invalid URI")
	}
}

func TestRedisBackendPlugin_CreateBackend_URIParsing(t *testing.T) {
	plugin := &RedisBackendPlugin{}
	plugin.Initialize(map[string]interface{}{})
	
	// This will fail to connect but should parse the URI correctly
	uri := "redis://localhost:6379/0?key=test:logs&max=500&expire=1800"
	
	_, err := plugin.CreateBackend(uri, map[string]interface{}{})
	// We expect this to fail because Redis is not running, but we can
	// check that the error is about connection, not parsing
	if err != nil && !strings.Contains(err.Error(), "connect") {
		t.Errorf("Expected connection error, got: %v", err)
	}
}

func TestRedisBackend_SupportsAtomic(t *testing.T) {
	backend := &RedisBackend{atomicSupport: true}
	
	if !backend.SupportsAtomic() {
		t.Error("RedisBackend should support atomic operations")
	}
}

func TestRedisBackend_Flush(t *testing.T) {
	backend := &RedisBackend{}
	
	// Flush should be a no-op for Redis
	err := backend.Flush()
	if err != nil {
		t.Errorf("Flush should not return error, got: %v", err)
	}
}

func TestRedisBackend_Close_NoConnection(t *testing.T) {
	backend := &RedisBackend{}
	
	// Close with no connection should not error
	err := backend.Close()
	if err != nil {
		t.Errorf("Close with no connection should not error, got: %v", err)
	}
}

// MockConn implements net.Conn for testing
type MockConn struct {
	writeData []byte
	readData  []byte
	readIndex int
	closed    bool
	responses []string
	respIndex int
}

func (m *MockConn) Read(b []byte) (int, error) {
	if m.closed {
		return 0, fmt.Errorf("connection closed")
	}
	
	// If we have predefined responses, use them
	if len(m.responses) > 0 {
		if m.respIndex >= len(m.responses) {
			// Reset for repeated reads
			m.respIndex = 0
		}
		response := m.responses[m.respIndex]
		m.respIndex++
		n := copy(b, []byte(response))
		return n, nil
	}
	
	// Fallback to readData
	if m.readIndex >= len(m.readData) {
		return 0, fmt.Errorf("no more data")
	}
	
	n := copy(b, m.readData[m.readIndex:])
	m.readIndex += n
	return n, nil
}

func (m *MockConn) Write(b []byte) (int, error) {
	if m.closed {
		return 0, fmt.Errorf("connection closed")
	}
	m.writeData = append(m.writeData, b...)
	return len(b), nil
}

func (m *MockConn) Close() error {
	m.closed = true
	return nil
}

func (m *MockConn) LocalAddr() net.Addr                { return nil }
func (m *MockConn) RemoteAddr() net.Addr               { return nil }
func (m *MockConn) SetDeadline(t time.Time) error      { return nil }
func (m *MockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *MockConn) SetWriteDeadline(t time.Time) error { return nil }

func TestRedisBackend_Write_MockConnection(t *testing.T) {
	backend := &RedisBackend{
		key:        "test:logs",
		maxEntries: 100,
		expiration: 0,
	}
	
	// Create mock connection with successful Redis responses
	mockConn := &MockConn{
		responses: []string{":1\r\n", "+OK\r\n"}, // LPUSH response, LTRIM response
	}
	backend.conn = mockConn
	
	testEntry := []byte(`{"message":"test log"}`)
	
	n, err := backend.Write(testEntry)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
	
	if n != len(testEntry) {
		t.Errorf("Expected to write %d bytes, got %d", len(testEntry), n)
	}
	
	// Verify Redis commands were written
	written := string(mockConn.writeData)
	if !strings.Contains(written, "LPUSH") {
		t.Error("Expected LPUSH command in written data")
	}
	if !strings.Contains(written, "LTRIM") {
		t.Error("Expected LTRIM command in written data")
	}
	if !strings.Contains(written, "test:logs") {
		t.Error("Expected key name in written data")
	}
}

func TestRedisBackend_Write_WithExpiration(t *testing.T) {
	backend := &RedisBackend{
		key:        "test:logs",
		maxEntries: 100,
		expiration: 30 * time.Second,
	}
	
	// Create mock connection with successful Redis responses
	mockConn := &MockConn{
		responses: []string{":1\r\n", "+OK\r\n", ":1\r\n"}, // LPUSH, LTRIM, EXPIRE responses
	}
	backend.conn = mockConn
	
	testEntry := []byte(`{"message":"test with expiration"}`)
	
	_, err := backend.Write(testEntry)
	if err != nil {
		t.Errorf("Write with expiration failed: %v", err)
	}
	
	// Verify EXPIRE command was written
	written := string(mockConn.writeData)
	if !strings.Contains(written, "EXPIRE") {
		t.Error("Expected EXPIRE command in written data")
	}
}

func TestRedisBackend_ReadResponse_Success(t *testing.T) {
	backend := &RedisBackend{}
	
	mockConn := &MockConn{
		readData: []byte(":1\r\n"), // Integer response
	}
	backend.conn = mockConn
	
	err := backend.readResponse()
	if err != nil {
		t.Errorf("readResponse should succeed with valid response, got: %v", err)
	}
}

func TestRedisBackend_ReadResponse_Error(t *testing.T) {
	backend := &RedisBackend{}
	
	mockConn := &MockConn{
		readData: []byte("-ERR unknown command\r\n"), // Error response
	}
	backend.conn = mockConn
	
	err := backend.readResponse()
	if err == nil {
		t.Error("readResponse should fail with Redis error response")
	}
	
	if !strings.Contains(err.Error(), "Redis error") {
		t.Errorf("Expected Redis error message, got: %v", err)
	}
}

func TestRedisBackend_Write_ConnectionFailure(t *testing.T) {
	backend := &RedisBackend{
		addr: "invalid:999999", // Invalid address to force connection failure
		key:  "test:logs",
	}
	
	testEntry := []byte(`{"message":"test"}`)
	
	_, err := backend.Write(testEntry)
	if err == nil {
		t.Error("Write should fail with invalid connection address")
	}
}

func TestRedisBackend_Close_WithConnection(t *testing.T) {
	backend := &RedisBackend{}
	
	mockConn := &MockConn{}
	backend.conn = mockConn
	
	err := backend.Close()
	if err != nil {
		t.Errorf("Close should not error, got: %v", err)
	}
	
	if !mockConn.closed {
		t.Error("Mock connection should be closed")
	}
}

func TestRedisBackendPlugin_Integration(t *testing.T) {
	// Test the full plugin workflow
	plugin := &RedisBackendPlugin{}
	
	// Initialize plugin
	err := plugin.Initialize(map[string]interface{}{})
	if err != nil {
		t.Fatalf("Failed to initialize plugin: %v", err)
	}
	
	// Test URI parsing with various parameters
	testURIs := []struct {
		uri         string
		shouldFail  bool
		description string
	}{
		{"redis://localhost:6379", false, "basic URI"},
		{"redis://localhost:6379/0", false, "URI with database"},
		{"redis://localhost:6379?key=logs", false, "URI with key parameter"},
		{"redis://localhost:6379?max=500", false, "URI with max parameter"},
		{"redis://localhost:6379?expire=3600", false, "URI with expire parameter"},
		{"redis://localhost:6379?key=app:logs&max=1000&expire=1800", false, "URI with all parameters"},
		{"http://localhost:8080", true, "invalid scheme"},
		{"redis://[invalid", true, "malformed URI"},
	}
	
	for _, test := range testURIs {
		t.Run(test.description, func(t *testing.T) {
			backend, err := plugin.CreateBackend(test.uri, map[string]interface{}{})
			
			if test.shouldFail {
				if err == nil {
					t.Errorf("Expected CreateBackend to fail for %s", test.uri)
				}
			} else {
				// We expect connection to fail (Redis not running), but parsing should succeed
				if err != nil {
					if !strings.Contains(err.Error(), "connect") {
						t.Errorf("Expected connection error for %s, got: %v", test.uri, err)
					}
				} else {
					// If somehow it succeeded, clean up
					backend.Close()
				}
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

func TestRedisURIParsing(t *testing.T) {
	plugin := &RedisBackendPlugin{}
	plugin.Initialize(map[string]interface{}{})
	
	// Test various URI formats and parameter combinations
	testCases := []struct {
		uri            string
		expectedKey    string
		expectedMax    int
		expectedExpire time.Duration
		description    string
	}{
		{
			uri:            "redis://localhost:6379",
			expectedKey:    "flexlog:entries",
			expectedMax:    10000,
			expectedExpire: 0,
			description:    "default values",
		},
		{
			uri:            "redis://localhost:6379?key=custom:logs",
			expectedKey:    "custom:logs",
			expectedMax:    10000,
			expectedExpire: 0,
			description:    "custom key",
		},
		{
			uri:            "redis://localhost:6379?max=500",
			expectedKey:    "flexlog:entries",
			expectedMax:    500,
			expectedExpire: 0,
			description:    "custom max entries",
		},
		{
			uri:            "redis://localhost:6379?expire=3600",
			expectedKey:    "flexlog:entries",
			expectedMax:    10000,
			expectedExpire: 3600 * time.Second,
			description:    "custom expiration",
		},
		{
			uri:            "redis://localhost:6379?key=app:logs&max=1000&expire=1800",
			expectedKey:    "app:logs",
			expectedMax:    1000,
			expectedExpire: 1800 * time.Second,
			description:    "all custom parameters",
		},
	}
	
	for _, test := range testCases {
		t.Run(test.description, func(t *testing.T) {
			// This will fail to connect, but we can still check the parsing
			// by examining the error details or by mocking the connection
			
			backend, err := plugin.CreateBackend(test.uri, map[string]interface{}{})
			if err != nil {
				// Expected due to no Redis server, but we can still verify
				// the error is about connection, not parsing
				if !strings.Contains(err.Error(), "connect") {
					t.Errorf("Expected connection error, got parsing error: %v", err)
				}
				return
			}
			
			// If somehow connection succeeded, verify the backend properties
			redisBackend, ok := backend.(*RedisBackend)
			if !ok {
				t.Fatalf("Expected *RedisBackend, got %T", backend)
			}
			
			if redisBackend.key != test.expectedKey {
				t.Errorf("Expected key '%s', got '%s'", test.expectedKey, redisBackend.key)
			}
			
			if redisBackend.maxEntries != test.expectedMax {
				t.Errorf("Expected max entries %d, got %d", test.expectedMax, redisBackend.maxEntries)
			}
			
			if redisBackend.expiration != test.expectedExpire {
				t.Errorf("Expected expiration %v, got %v", test.expectedExpire, redisBackend.expiration)
			}
			
			backend.Close()
		})
	}
}

// Benchmark tests
func BenchmarkRedisBackend_Write(b *testing.B) {
	backend := &RedisBackend{
		key:        "benchmark:logs",
		maxEntries: 10000,
		expiration: 0,
	}
	
	// Use mock connection for benchmarking
	mockConn := &MockConn{
		responses: []string{":1\r\n", "+OK\r\n"}, // LPUSH, LTRIM responses (will cycle)
	}
	backend.conn = mockConn
	
	testEntry := []byte(`{"timestamp":"2023-12-25T10:30:45Z","level":"INFO","message":"Benchmark test"}`)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := backend.Write(testEntry)
		if err != nil {
			b.Fatalf("Write failed: %v", err)
		}
	}
}

func BenchmarkRedisBackend_WriteWithExpiration(b *testing.B) {
	backend := &RedisBackend{
		key:        "benchmark:logs",
		maxEntries: 10000,
		expiration: 30 * time.Second,
	}
	
	// Use mock connection for benchmarking
	mockConn := &MockConn{
		responses: []string{":1\r\n", "+OK\r\n", ":1\r\n"}, // LPUSH, LTRIM, EXPIRE responses (will cycle)
	}
	backend.conn = mockConn
	
	testEntry := []byte(`{"timestamp":"2023-12-25T10:30:45Z","level":"INFO","message":"Benchmark with expiration"}`)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := backend.Write(testEntry)
		if err != nil {
			b.Fatalf("Write failed: %v", err)
		}
	}
}