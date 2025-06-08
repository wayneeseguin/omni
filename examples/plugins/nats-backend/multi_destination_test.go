package main

import (
	"fmt"
	"testing"

	"github.com/wayneeseguin/omni/pkg/omni"
)

func TestNATSMultiDestination(t *testing.T) {
	// Test creating multiple NATS destinations
	destinations := []string{
		"nats://localhost:4222/logs.info",
		"nats://localhost:4222/logs.error?queue=error-handlers",
		"nats://localhost:4222/logs.debug?async=false",
	}

	backends := make([]*NATSBackend, 0, len(destinations))

	for _, uri := range destinations {
		backend, err := NewNATSBackendWithOptions(uri, false)
		if err != nil {
			t.Fatalf("Failed to create backend for %s: %v", uri, err)
		}
		backends = append(backends, backend)
	}

	// Verify each backend has correct configuration
	if backends[0].subject != "logs.info" || backends[0].async != true {
		t.Error("First backend configuration incorrect")
	}

	if backends[1].subject != "logs.error" || backends[1].queueGroup != "error-handlers" {
		t.Error("Second backend configuration incorrect")
	}

	if backends[2].subject != "logs.debug" || backends[2].async != false {
		t.Error("Third backend configuration incorrect")
	}

	// Clean up
	for _, backend := range backends {
		backend.Close()
	}
}

func TestNATSRoutingByLevel(t *testing.T) {
	// Test routing messages to different NATS subjects based on log level
	type routeConfig struct {
		level     int
		levelName string
		subject   string
		uri       string
	}

	routes := []routeConfig{
		{omni.LevelDebug, "DEBUG", "logs.debug", "nats://localhost:4222/logs.debug"},
		{omni.LevelInfo, "INFO", "logs.info", "nats://localhost:4222/logs.info"},
		{omni.LevelWarn, "WARN", "logs.warn", "nats://localhost:4222/logs.warn"},
		{omni.LevelError, "ERROR", "logs.error", "nats://localhost:4222/logs.error"},
	}

	// Create backends for each level
	levelBackends := make(map[int]*NATSBackend)

	for _, route := range routes {
		backend, err := NewNATSBackendWithOptions(route.uri, false)
		if err != nil {
			t.Fatalf("Failed to create backend for level %d: %v", route.level, err)
		}
		levelBackends[route.level] = backend
		defer backend.Close()
	}

	// Simulate routing logic
	messages := []struct {
		level     int
		levelName string
		message   string
	}{
		{omni.LevelDebug, "DEBUG", "Debug message"},
		{omni.LevelInfo, "INFO", "Info message"},
		{omni.LevelWarn, "WARN", "Warning message"},
		{omni.LevelError, "ERROR", "Error message"},
	}

	for _, msg := range messages {
		backend := levelBackends[msg.level]
		if backend == nil {
			t.Errorf("No backend for level %d", msg.level)
			continue
		}

		entry := []byte(fmt.Sprintf(`{"level":"%s","message":"%s"}`, msg.levelName, msg.message))
		if _, err := backend.bufferWrite(entry); err != nil {
			t.Errorf("Failed to write %s message: %v", msg.levelName, err)
		}
	}

	t.Log("Level-based routing test completed")
}

func TestNATSClusteredDestinations(t *testing.T) {
	// Test multiple NATS clusters
	clusters := []struct {
		name string
		uri  string
	}{
		{"primary", "nats://localhost:4222/cluster.primary"},
		{"secondary", "nats://localhost:4223,localhost:4224/cluster.secondary"},
		{"edge", "nats://edge-nats:4222/cluster.edge?reconnect_wait=5"},
	}

	for _, cluster := range clusters {
		t.Run(cluster.name, func(t *testing.T) {
			// For non-localhost addresses, we expect connection to fail in test
			if cluster.name == "secondary" || cluster.name == "edge" {
				_, err := NewNATSBackendWithOptions(cluster.uri, false)
				if err != nil {
					t.Logf("Expected: Could not create backend for %s cluster (no server running)", cluster.name)
					return
				}
			} else {
				backend, err := NewNATSBackendWithOptions(cluster.uri, false)
				if err != nil {
					t.Fatalf("Failed to create backend for %s cluster: %v", cluster.name, err)
				}
				defer backend.Close()

				// Verify configuration
				if backend.subject != fmt.Sprintf("cluster.%s", cluster.name) {
					t.Errorf("Expected subject cluster.%s, got %s", cluster.name, backend.subject)
				}
			}
		})
	}
}
