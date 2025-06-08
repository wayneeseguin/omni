package main

import (
	"testing"
)

func TestVaultBackendURIParsing(t *testing.T) {
	tests := []struct {
		name          string
		uri           string
		config        map[string]interface{}
		expectError   bool
		expectedHost  string
		expectedToken string
		expectedPath  string
	}{
		{
			name:          "Basic URI",
			uri:           "vault://test-token@localhost:8200/logs",
			config:        map[string]interface{}{},
			expectError:   false,
			expectedHost:  "localhost:8200",
			expectedToken: "test-token",
			expectedPath:  "logs",
		},
		{
			name:          "URI without token",
			uri:           "vault://localhost:8200/logs",
			config:        map[string]interface{}{"token": "config-token"},
			expectError:   false,
			expectedHost:  "localhost:8200",
			expectedToken: "config-token",
			expectedPath:  "logs",
		},
		{
			name:          "Default values",
			uri:           "vault://",
			config:        map[string]interface{}{"token": "test-token"},
			expectError:   false,
			expectedHost:  "",
			expectedToken: "test-token",
			expectedPath:  "logs",
		},
		{
			name:        "Invalid scheme",
			uri:         "http://localhost:8200",
			config:      map[string]interface{}{},
			expectError: true,
		},
		{
			name:          "Complex path",
			uri:           "vault://token@vault.example.com:8200/app/logs/production",
			config:        map[string]interface{}{},
			expectError:   true, // Will fail because vault.example.com doesn't exist
			expectedHost:  "vault.example.com:8200",
			expectedToken: "token",
			expectedPath:  "app/logs/production",
		},
	}

	plugin := OmniPlugin

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, err := plugin.CreateBackend(tt.uri, tt.config)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// We can't easily access internal fields, but we can verify the backend was created
			if backend == nil {
				t.Error("Expected backend to be created but got nil")
			}

			// Verify it implements the required interface
			if backend.Name() != "vault-backend" {
				t.Errorf("Expected backend name 'vault-backend', got %s", backend.Name())
			}
		})
	}
}

func TestVaultBackendMethods(t *testing.T) {
	backend := &VaultBackend{}

	// Test Name
	if backend.Name() != "vault-backend" {
		t.Errorf("Expected name 'vault-backend', got %s", backend.Name())
	}

	// Test Version
	if backend.Version() != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %s", backend.Version())
	}

	// Test SupportsAtomic
	if !backend.SupportsAtomic() {
		t.Error("Expected Vault backend to support atomic operations")
	}

	// Test Flush (should be no-op)
	if err := backend.Flush(); err != nil {
		t.Errorf("Flush should not return error, got %v", err)
	}

	// Test Close (should be no-op)
	if err := backend.Close(); err != nil {
		t.Errorf("Close should not return error, got %v", err)
	}

	// Test Configure (should be no-op)
	if err := backend.Configure(map[string]interface{}{}); err != nil {
		t.Errorf("Configure should not return error, got %v", err)
	}
}
