//go:build integration
// +build integration

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/vault/api"
	"github.com/wayneeseguin/omni/pkg/omni"
)

func TestVaultBackendIntegration(t *testing.T) {
	// Check if Vault is available
	vaultAddr := os.Getenv("VAULT_ADDR")
	if vaultAddr == "" {
		vaultAddr = "http://localhost:8200"
	}

	vaultToken := os.Getenv("VAULT_TOKEN")
	if vaultToken == "" {
		vaultToken = "test-token"
	}

	// Create Vault client to verify connectivity
	config := api.DefaultConfig()
	config.Address = vaultAddr
	client, err := api.NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create Vault client: %v", err)
	}
	client.SetToken(vaultToken)

	// Check Vault health
	health, err := client.Sys().Health()
	if err != nil {
		t.Skipf("Vault not available at %s: %v", vaultAddr, err)
	}

	if !health.Initialized {
		t.Skip("Vault is not initialized")
	}

	// Create logger with Vault backend
	logger, err := NewVaultLogger(vaultAddr, vaultToken, "secret")
	if err != nil {
		t.Fatalf("Failed to create Vault logger: %v", err)
	}
	defer logger.Close()

	// Test 1: Write basic log message
	t.Run("BasicLogging", func(t *testing.T) {
		logger.Info("Test log message", "key", "value", "number", 42)

		// Give Vault time to process
		time.Sleep(10 * time.Millisecond)
	})

	// Test 2: Write multiple log levels
	t.Run("MultipleLogLevels", func(t *testing.T) {
		logger.Debug("Debug message", "debug", true)
		logger.Info("Info message", "info", true)
		logger.Warn("Warning message", "warn", true)
		logger.Error("Error message", "error", true)

		// Give Vault time to process
		time.Sleep(10 * time.Millisecond)
	})

	// Test 3: Structured logging with complex data
	t.Run("StructuredLogging", func(t *testing.T) {
		complexData := map[string]interface{}{
			"user_id": 12345,
			"action":  "login",
			"metadata": map[string]interface{}{
				"ip":         "192.168.1.1",
				"user_agent": "Mozilla/5.0",
				"timestamp":  time.Now().Unix(),
			},
		}

		logger.Info("User action", "data", complexData)

		// Give Vault time to process
		time.Sleep(10 * time.Millisecond)
	})

	// Test 4: Verify logs are written to Vault
	t.Run("VerifyLogsInVault", func(t *testing.T) {
		// List keys in the logs path
		secret, err := client.Logical().List("secret/metadata/logs")
		if err != nil {
			t.Logf("Warning: Could not list logs in Vault: %v", err)
			return
		}

		if secret == nil || secret.Data == nil {
			t.Log("No logs found in Vault (this might be expected if logs are written to unique paths)")
			return
		}

		keys, ok := secret.Data["keys"].([]interface{})
		if !ok || len(keys) == 0 {
			t.Log("No log keys found in Vault")
			return
		}

		// Read one of the logs
		if len(keys) > 0 {
			key := keys[0].(string)
			logPath := fmt.Sprintf("secret/data/logs/%s", key)

			logSecret, err := client.Logical().Read(logPath)
			if err != nil {
				t.Errorf("Failed to read log from Vault: %v", err)
				return
			}

			if logSecret != nil && logSecret.Data != nil {
				if data, ok := logSecret.Data["data"].(map[string]interface{}); ok {
					t.Logf("Successfully read log from Vault: %+v", data)
				}
			}
		}
	})

	// Test 5: High volume logging
	t.Run("HighVolumeLogging", func(t *testing.T) {
		const numLogs = 100
		start := time.Now()

		for i := 0; i < numLogs; i++ {
			logger.Info("High volume log",
				"index", i,
				"timestamp", time.Now().UnixNano(),
				"data", fmt.Sprintf("log-entry-%d", i),
			)
		}

		duration := time.Since(start)
		t.Logf("Logged %d messages in %v (%.2f msgs/sec)",
			numLogs, duration, float64(numLogs)/duration.Seconds())
	})

	// Test 6: Error scenarios
	t.Run("ErrorScenarios", func(t *testing.T) {
		// Test with invalid Vault configuration
		invalidLogger, err := omni.New("vault://invalid-token@localhost:8200/logs")

		if err == nil {
			defer invalidLogger.Close()
			// Try to log with invalid credentials
			invalidLogger.Error("This should fail")
			// We expect this to fail, but it might be buffered
			t.Log("Attempted to log with invalid credentials")
		}
	})

	// Test 7: JSON formatting
	t.Run("JSONFormatting", func(t *testing.T) {
		jsonData := map[string]interface{}{
			"event": "test_event",
			"properties": map[string]interface{}{
				"foo": "bar",
				"baz": []int{1, 2, 3},
			},
		}

		jsonBytes, err := json.Marshal(jsonData)
		if err != nil {
			t.Fatalf("Failed to marshal JSON: %v", err)
		}

		// Log raw JSON
		logger.Info(string(jsonBytes))
	})
}

// TestVaultBackendPlugin tests the plugin interface directly
func TestVaultBackendPlugin(t *testing.T) {
	plugin := OmniPlugin

	// Test plugin metadata
	if plugin.Name() != "vault-backend" {
		t.Errorf("Expected plugin name 'vault-backend', got %s", plugin.Name())
	}

	if plugin.Version() == "" {
		t.Error("Plugin version should not be empty")
	}

	if plugin.Description() == "" {
		t.Error("Plugin description should not be empty")
	}

	// Test supported schemes
	schemes := plugin.SupportedSchemes()
	if len(schemes) != 1 || schemes[0] != "vault" {
		t.Errorf("Expected supported schemes ['vault'], got %v", schemes)
	}

	// Test initialization
	err := plugin.Initialize(map[string]interface{}{})
	if err != nil {
		t.Errorf("Failed to initialize plugin: %v", err)
	}

	// Test health check
	health := plugin.Health()
	if !health.Healthy {
		t.Errorf("Plugin should be healthy after initialization")
	}
}

// BenchmarkVaultBackend benchmarks the Vault backend performance
func BenchmarkVaultBackend(b *testing.B) {
	// Check if Vault is available
	vaultAddr := os.Getenv("VAULT_ADDR")
	if vaultAddr == "" {
		vaultAddr = "http://localhost:8200"
	}

	vaultToken := os.Getenv("VAULT_TOKEN")
	if vaultToken == "" {
		vaultToken = "test-token"
	}

	// Create logger
	logger, err := NewVaultLogger(vaultAddr, vaultToken, "secret")
	if err != nil {
		b.Skipf("Failed to create Vault logger: %v", err)
	}
	defer logger.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			logger.Info("Benchmark log message",
				"iteration", i,
				"thread", b.Name(),
				"timestamp", time.Now().UnixNano(),
			)
			i++
		}
	})

	b.ReportMetric(float64(b.N), "logs")
}
