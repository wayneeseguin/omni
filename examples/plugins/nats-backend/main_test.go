package natsplugin

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNATSBackendPlugin_Initialize(t *testing.T) {
	plugin := &NATSBackendPlugin{}
	
	config := map[string]interface{}{
		"key": "value",
	}
	
	err := plugin.Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	
	if !plugin.initialized {
		t.Error("Plugin not marked as initialized")
	}
	
	if plugin.config["key"] != "value" {
		t.Error("Config not stored correctly")
	}
}

func TestNATSBackendPlugin_SupportedSchemes(t *testing.T) {
	plugin := &NATSBackendPlugin{}
	schemes := plugin.SupportedSchemes()
	
	if len(schemes) != 1 || schemes[0] != "nats" {
		t.Errorf("Expected [nats], got %v", schemes)
	}
}

func TestNATSBackendPlugin_CreateBackend_NotInitialized(t *testing.T) {
	plugin := &NATSBackendPlugin{}
	
	_, err := plugin.CreateBackend("nats://localhost:4222/test", nil)
	if err == nil {
		t.Error("Expected error when creating backend without initialization")
	}
}

func TestNewNATSBackend_URIParsing(t *testing.T) {
	tests := []struct {
		name          string
		uri           string
		expectError   bool
		expectSubject string
		expectQueue   string
		expectAsync   bool
		expectBatch   int
	}{
		{
			name:          "basic URI",
			uri:           "nats://localhost:4222/logs.test",
			expectSubject: "logs.test",
			expectAsync:   true,
			expectBatch:   100,
		},
		{
			name:          "URI with queue group",
			uri:           "nats://localhost:4222/logs.test?queue=workers",
			expectSubject: "logs.test",
			expectQueue:   "workers",
			expectAsync:   true,
			expectBatch:   100,
		},
		{
			name:          "URI with async disabled",
			uri:           "nats://localhost:4222/logs.test?async=false",
			expectSubject: "logs.test",
			expectAsync:   false,
			expectBatch:   100,
		},
		{
			name:          "URI with custom batch size",
			uri:           "nats://localhost:4222/logs.test?batch=200",
			expectSubject: "logs.test",
			expectAsync:   true,
			expectBatch:   200,
		},
		{
			name:        "invalid scheme",
			uri:         "http://localhost:4222/logs.test",
			expectError: true,
		},
		{
			name:        "invalid URI",
			uri:         "not-a-uri",
			expectError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, err := NewNATSBackendWithOptions(tt.uri, false)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			
			// Since we can't connect to NATS in unit tests, check the backend properties
			if backend.subject != tt.expectSubject {
				t.Errorf("Expected subject %s, got %s", tt.expectSubject, backend.subject)
			}
			
			if backend.queueGroup != tt.expectQueue {
				t.Errorf("Expected queue %s, got %s", tt.expectQueue, backend.queueGroup)
			}
			
			if backend.async != tt.expectAsync {
				t.Errorf("Expected async %v, got %v", tt.expectAsync, backend.async)
			}
			
			if backend.batchSize != tt.expectBatch {
				t.Errorf("Expected batch size %d, got %d", tt.expectBatch, backend.batchSize)
			}
			
			// Clean up
			backend.Close()
		})
	}
}

func TestNATSBackend_Write(t *testing.T) {
	// This test would require a mock NATS connection
	// For now, we'll just test the basic structure
	backend := &NATSBackend{
		async:     false,
		batchSize: 0,
		buffer:    make([][]byte, 0),
	}
	
	// Test that Write returns error when connection is nil
	_, err := backend.Write([]byte("test"))
	if err == nil {
		t.Error("Expected error when writing with nil connection")
	}
}

func TestNATSBackend_BufferWrite(t *testing.T) {
	backend := &NATSBackend{
		async:         true,
		batchSize:     3,
		buffer:        make([][]byte, 0),
		flushInterval: 100 * time.Millisecond,
	}
	
	// Test adding to buffer
	entry1 := []byte("entry1")
	n, err := backend.bufferWrite(entry1)
	if err != nil {
		t.Fatalf("bufferWrite failed: %v", err)
	}
	if n != len(entry1) {
		t.Errorf("Expected %d bytes written, got %d", len(entry1), n)
	}
	
	if len(backend.buffer) != 1 {
		t.Errorf("Expected buffer length 1, got %d", len(backend.buffer))
	}
	
	// Test that buffer is copied
	entry1[0] = 'X'
	if backend.buffer[0][0] == 'X' {
		t.Error("Buffer entry was not copied")
	}
}

func TestNATSBackend_SupportsAtomic(t *testing.T) {
	backend := &NATSBackend{
		atomicSupport: false,
	}
	
	if backend.SupportsAtomic() {
		t.Error("Expected SupportsAtomic to return false")
	}
}

func TestNATSBackend_FormatMessage(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		entry    interface{}
		expected string
	}{
		{
			name:   "JSON format with map",
			format: "json",
			entry: map[string]interface{}{
				"level":   "INFO",
				"message": "test message",
			},
			expected: `{"level":"INFO","message":"test message"}`,
		},
		{
			name:     "text format with bytes",
			format:   "text",
			entry:    []byte("test text"),
			expected: "test text",
		},
		{
			name:     "text format with string",
			format:   "text",
			entry:    "test string",
			expected: "test string",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := &NATSBackend{
				format: tt.format,
			}
			
			result, err := backend.formatMessage(tt.entry)
			if err != nil {
				t.Fatalf("formatMessage failed: %v", err)
			}
			
			if string(result) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, string(result))
			}
		})
	}
}

func TestNATSBackendPlugin_Shutdown(t *testing.T) {
	plugin := &NATSBackendPlugin{}
	
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	
	err := plugin.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

func TestNATSBackend_URIAuthentication(t *testing.T) {
	tests := []struct {
		name         string
		uri          string
		expectAuth   bool
		expectSecure bool
	}{
		{
			name:         "URI with authentication",
			uri:          "nats://user:pass@localhost:4222/logs",
			expectAuth:   true,
			expectSecure: false,
		},
		{
			name:         "URI with TLS",
			uri:          "nats://localhost:4222/logs?tls=true",
			expectAuth:   false,
			expectSecure: true,
		},
		{
			name:         "URI with auth and TLS",
			uri:          "nats://user:pass@localhost:4222/logs?tls=true",
			expectAuth:   true,
			expectSecure: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the URI to check options were set
			backend, err := NewNATSBackendWithOptions(tt.uri, false)
			if err != nil {
				// Expected to fail connection, but should parse options
				if !strings.Contains(err.Error(), "failed to connect") {
					t.Fatalf("Unexpected error: %v", err)
				}
			} else {
				backend.Close()
			}
			
			// We can't easily test the options were applied without a real connection
			// This would be better tested with integration tests
		})
	}
}