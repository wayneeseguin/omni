package natsplugin

import (
	"testing"
)

func TestNATSBackendPlugin(t *testing.T) {
	// Test plugin initialization
	plugin := &NATSBackendPlugin{}
	
	// Test plugin metadata
	if plugin.Name() != "nats" {
		t.Errorf("Expected plugin name 'nats', got %s", plugin.Name())
	}
	
	if plugin.Version() != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %s", plugin.Version())
	}
	
	// Test initialization
	config := map[string]interface{}{
		"test": "value",
	}
	
	err := plugin.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize plugin: %v", err)
	}
	
	// Test health status
	health := plugin.Health()
	if !health.Healthy {
		t.Error("Plugin should be healthy after initialization")
	}
	
	// Test supported schemes
	schemes := plugin.SupportedSchemes()
	if len(schemes) != 1 || schemes[0] != "nats" {
		t.Errorf("Expected supported scheme 'nats', got %v", schemes)
	}
}

func TestNATSBackendCreation(t *testing.T) {
	plugin := &NATSBackendPlugin{}
	plugin.Initialize(nil)
	
	// Test backend creation without connection
	backend, err := NewNATSBackendWithOptions("nats://localhost:4222/test.logs", false)
	if err != nil {
		t.Fatalf("Failed to create NATS backend: %v", err)
	}
	
	// Check backend properties
	if backend.subject != "test.logs" {
		t.Errorf("Expected subject 'test.logs', got %s", backend.subject)
	}
	
	if backend.async != true {
		t.Error("Expected async to be true by default")
	}
	
	if backend.batchSize != 100 {
		t.Errorf("Expected batch size 100, got %d", backend.batchSize)
	}
}

func TestNATSBackendURIParsing(t *testing.T) {
	tests := []struct {
		name          string
		uri           string
		expectedSubject string
		expectedQueue   string
		expectedAsync   bool
		expectedBatch   int
		wantErr       bool
	}{
		{
			name:          "basic URI",
			uri:           "nats://localhost:4222/logs.app",
			expectedSubject: "logs.app",
			expectedAsync:   true,
			expectedBatch:   100,
		},
		{
			name:          "with queue group",
			uri:           "nats://localhost:4222/logs.app?queue=workers",
			expectedSubject: "logs.app",
			expectedQueue:   "workers",
			expectedAsync:   true,
			expectedBatch:   100,
		},
		{
			name:          "sync mode",
			uri:           "nats://localhost:4222/logs.app?async=false",
			expectedSubject: "logs.app",
			expectedAsync:   false,
			expectedBatch:   100,
		},
		{
			name:          "custom batch size",
			uri:           "nats://localhost:4222/logs.app?batch=500",
			expectedSubject: "logs.app",
			expectedAsync:   true,
			expectedBatch:   500,
		},
		{
			name:    "invalid scheme",
			uri:     "http://localhost:4222/logs.app",
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, err := NewNATSBackendWithOptions(tt.uri, false)
			
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			
			if backend.subject != tt.expectedSubject {
				t.Errorf("Expected subject %s, got %s", tt.expectedSubject, backend.subject)
			}
			
			if backend.queueGroup != tt.expectedQueue {
				t.Errorf("Expected queue %s, got %s", tt.expectedQueue, backend.queueGroup)
			}
			
			if backend.async != tt.expectedAsync {
				t.Errorf("Expected async %v, got %v", tt.expectedAsync, backend.async)
			}
			
			if backend.batchSize != tt.expectedBatch {
				t.Errorf("Expected batch size %d, got %d", tt.expectedBatch, backend.batchSize)
			}
		})
	}
}

func TestNATSBackendBuffering(t *testing.T) {
	backend, err := NewNATSBackendWithOptions("nats://localhost:4222/test?batch=3", false)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	
	// Simulate buffering without connection
	entry1 := []byte("log entry 1")
	entry2 := []byte("log entry 2")
	
	// Buffer first entry
	n, err := backend.bufferWrite(entry1)
	if err != nil {
		t.Fatalf("Failed to buffer entry: %v", err)
	}
	if n != len(entry1) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(entry1), n)
	}
	
	// Buffer second entry
	n, err = backend.bufferWrite(entry2)
	if err != nil {
		t.Fatalf("Failed to buffer entry: %v", err)
	}
	
	// Check buffer length
	backend.bufferMu.Lock()
	bufferLen := len(backend.buffer)
	backend.bufferMu.Unlock()
	
	if bufferLen != 2 {
		t.Errorf("Expected buffer length 2, got %d", bufferLen)
	}
}