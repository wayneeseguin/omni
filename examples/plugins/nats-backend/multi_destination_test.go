package natsplugin

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wayneeseguin/omni"
)

// ensureNATSPluginRegistered ensures the NATS plugin is registered (helper for tests)
func ensureNATSPluginRegistered(t *testing.T) {
	plugin := &NATSBackendPlugin{}
	if err := plugin.Initialize(nil); err != nil {
		t.Fatalf("Failed to initialize plugin: %v", err)
	}

	// Register the plugin (ignore if already registered)
	if err := omni.RegisterBackendPlugin(plugin); err != nil {
		if !strings.Contains(err.Error(), "already registered") {
			t.Fatalf("Failed to register plugin: %v", err)
		}
	}
}

// TestMultipleNATSDestinations tests Omni with multiple NATS destinations
func TestMultipleNATSDestinations(t *testing.T) {
	ensureNATSPluginRegistered(t)

	// Create logger
	logger, err := omni.NewBuilder().
		WithLevel(omni.LevelDebug).
		WithJSON().
		Build()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test configurations for multiple NATS destinations
	destinations := []struct {
		name    string
		uri     string
		purpose string
	}{
		{
			name:    "info-logs",
			uri:     "nats://localhost:4222/logs.app.info?queue=info-processors",
			purpose: "Info level logs",
		},
		{
			name:    "error-logs",
			uri:     "nats://localhost:4222/logs.app.error?queue=error-processors",
			purpose: "Error level logs",
		},
		{
			name:    "debug-logs",
			uri:     "nats://localhost:4222/logs.app.debug?async=false",
			purpose: "Debug logs (synchronous)",
		},
		{
			name:    "metrics",
			uri:     "nats://localhost:4222/metrics.app?batch=50&flush_interval=1000",
			purpose: "Application metrics",
		},
		{
			name:    "audit-trail",
			uri:     "nats://localhost:4222/audit.app.events?format=json",
			purpose: "Audit trail events",
		},
	}

	// Add all destinations
	addedDestinations := 0
	for _, dest := range destinations {
		err := logger.AddDestination(dest.uri)
		if err != nil {
			// This is expected if NATS is not running
			t.Logf("Note: Could not add destination %s: %v (NATS may not be running)", dest.name, err)
		} else {
			addedDestinations++
			t.Logf("Added destination: %s (%s)", dest.name, dest.purpose)
		}
	}

	// If no destinations were added (NATS not running), we can still test the structure
	t.Logf("Successfully configured %d NATS destinations", addedDestinations)

	// Verify we can get destination information
	dests := logger.Destinations
	t.Logf("Total destinations configured: %d", len(dests))

	// Log test messages
	testMessages := []struct {
		level   string
		message string
		fields  map[string]interface{}
	}{
		{
			level:   "info",
			message: "Application started",
			fields: map[string]interface{}{
				"version": "1.0.0",
				"pid":     12345,
			},
		},
		{
			level:   "error",
			message: "Database connection failed",
			fields: map[string]interface{}{
				"error":   "timeout",
				"retries": 3,
			},
		},
		{
			level:   "debug",
			message: "Processing request",
			fields: map[string]interface{}{
				"request_id": "abc-123",
				"duration":   150,
			},
		},
		{
			level:   "info",
			message: "Metrics update",
			fields: map[string]interface{}{
				"cpu_usage":    45.5,
				"memory_usage": 1024,
				"requests":     1000,
			},
		},
		{
			level:   "info",
			message: "User login",
			fields: map[string]interface{}{
				"user_id":    "user-456",
				"ip_address": "192.168.1.100",
				"timestamp":  time.Now().Unix(),
			},
		},
	}

	// Log messages
	for _, msg := range testMessages {
		switch msg.level {
		case "debug":
			logger.DebugWithFields(msg.message, msg.fields)
		case "info":
			logger.InfoWithFields(msg.message, msg.fields)
		case "error":
			logger.ErrorWithFields(msg.message, msg.fields)
		}
	}

	// Allow some time for async processing
	time.Sleep(100 * time.Millisecond)

	// Test destination metrics
	for _, dest := range dests {
		if dest.Backend == omni.BackendPlugin {
			t.Logf("Destination %s added successfully (Backend: %d)",
				dest.Name,
				dest.Backend)
		}
	}
}

// TestMultipleNATSDestinationsWithFailover tests failover between NATS destinations
func TestMultipleNATSDestinationsWithFailover(t *testing.T) {
	ensureNATSPluginRegistered(t)

	// Create logger with primary file destination
	logger, err := omni.NewBuilder().
		WithDestination("/tmp/omni-nats-test.log").
		WithLevel(omni.LevelInfo).
		Build()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Add multiple NATS clusters for failover
	clusters := []struct {
		name string
		uri  string
	}{
		{
			name: "primary-cluster",
			uri:  "nats://nats1:4222,nats2:4222,nats3:4222/logs.production?max_reconnect=5",
		},
		{
			name: "secondary-cluster",
			uri:  "nats://backup-nats1:4222,backup-nats2:4222/logs.backup?max_reconnect=3",
		},
		{
			name: "edge-cluster",
			uri:  "nats://edge-nats:4222/logs.edge?reconnect_wait=5",
		},
	}

	// Add all clusters
	for _, cluster := range clusters {
		err := logger.AddDestination(cluster.uri)
		if err != nil {
			t.Logf("Note: Could not add cluster %s: %v", cluster.name, err)
		} else {
			t.Logf("Added NATS cluster: %s", cluster.name)
		}
	}

	// Simulate application logging under various conditions
	logger.Info("Testing multi-cluster NATS setup")
	logger.InfoWithFields("Cluster configuration test", map[string]interface{}{
		"clusters":    len(clusters),
		"test_type":   "failover",
		"environment": "test",
	})
}

// TestNATSDestinationRouting tests routing logs to different NATS subjects based on level
func TestNATSDestinationRouting(t *testing.T) {
	ensureNATSPluginRegistered(t)

	// Create multiple loggers for different purposes
	loggers := make(map[string]*omni.Omni)

	// Application logger - routes to different subjects by level
	appLogger, err := omni.NewBuilder().
		WithLevel(omni.LevelDebug).
		WithJSON().
		Build()
	if err != nil {
		t.Fatalf("Failed to create app logger: %v", err)
	}
	loggers["app"] = appLogger

	// Audit logger - dedicated for audit events
	auditLogger, err := omni.NewBuilder().
		WithLevel(omni.LevelInfo).
		WithJSON().
		Build()
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}
	loggers["audit"] = auditLogger

	// Metrics logger - for performance metrics
	metricsLogger, err := omni.NewBuilder().
		WithLevel(omni.LevelInfo).
		WithJSON().
		Build()
	if err != nil {
		t.Fatalf("Failed to create metrics logger: %v", err)
	}
	loggers["metrics"] = metricsLogger

	// Configure routing for each logger
	routing := map[string][]string{
		"app": {
			"nats://localhost:4222/logs.app.all",          // All app logs
			"nats://localhost:4222/logs.app.errors?queue=error-handlers", // Errors only (filtered client-side)
			"nats://localhost:4222/logs.app.debug?async=false",          // Debug logs (sync)
		},
		"audit": {
			"nats://localhost:4222/audit.events?format=json",
			"nats://localhost:4222/audit.archive?batch=100&flush_interval=5000", // Batched for archival
		},
		"metrics": {
			"nats://localhost:4222/metrics.performance?batch=50",
			"nats://localhost:4222/metrics.realtime?async=true&batch=1", // Near real-time
		},
	}

	// Add destinations to each logger
	for loggerName, destinations := range routing {
		logger := loggers[loggerName]
		for _, dest := range destinations {
			if err := logger.AddDestination(dest); err != nil {
				t.Logf("Note: Could not add destination for %s: %v", loggerName, err)
			}
		}
	}

	// Cleanup
	defer func() {
		for name, logger := range loggers {
			logger.Close()
			t.Logf("Closed logger: %s", name)
		}
	}()

	// Simulate various log scenarios
	var wg sync.WaitGroup

	// App logging simulation
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			loggers["app"].DebugWithFields("Debug trace", map[string]interface{}{
				"iteration": i,
				"component": "worker",
			})
			
			if i%3 == 0 {
				loggers["app"].ErrorWithFields("Simulated error", map[string]interface{}{
					"error_code": fmt.Sprintf("ERR_%d", i),
					"severity":   "medium",
				})
			}
			
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Audit logging simulation
	wg.Add(1)
	go func() {
		defer wg.Done()
		events := []string{"login", "logout", "create", "update", "delete"}
		for i := 0; i < 5; i++ {
			loggers["audit"].InfoWithFields("Audit event", map[string]interface{}{
				"event_type": events[i%len(events)],
				"user_id":    fmt.Sprintf("user_%d", i),
				"ip":         fmt.Sprintf("192.168.1.%d", i+100),
				"timestamp":  time.Now().Unix(),
			})
			time.Sleep(20 * time.Millisecond)
		}
	}()

	// Metrics logging simulation
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 8; i++ {
			loggers["metrics"].InfoWithFields("Performance metrics", map[string]interface{}{
				"cpu_percent":    float64(30+i*5) + float64(i%3)*0.5,
				"memory_mb":      512 + i*50,
				"requests_sec":   100 + i*10,
				"response_ms":    20 + i%10,
				"active_conns":   50 + i*2,
				"timestamp":      time.Now().Unix(),
			})
			time.Sleep(15 * time.Millisecond)
		}
	}()

	// Wait for all simulations to complete
	wg.Wait()

	// Give async operations time to complete
	time.Sleep(200 * time.Millisecond)

	// Report on destination statistics
	t.Log("\n=== Destination Statistics ===")
	for name, logger := range loggers {
		t.Logf("\nLogger: %s", name)
		dests := logger.Destinations
		for _, dest := range dests {
			if dest.Backend == omni.BackendPlugin {
				t.Logf("  - %s: Backend=%d, Enabled=%t",
					dest.URI,
					dest.Backend,
					dest.Enabled)
			}
		}
	}
}

// TestNATSSubjectHierarchy tests NATS subject hierarchy patterns
func TestNATSSubjectHierarchy(t *testing.T) {
	ensureNATSPluginRegistered(t)

	// Create logger
	logger, err := omni.NewBuilder().
		WithLevel(omni.LevelDebug).
		WithJSON().
		Build()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test hierarchical subject structure
	// This allows subscribers to use wildcards like:
	// - logs.* (all logs)
	// - logs.app.* (all app logs)
	// - logs.*.error (all error logs)
	// - logs.app.service1.* (all service1 logs)
	subjects := []struct {
		subject     string
		description string
	}{
		{"nats://localhost:4222/logs", "Root logs subject"},
		{"nats://localhost:4222/logs.app", "Application logs"},
		{"nats://localhost:4222/logs.app.service1", "Service 1 logs"},
		{"nats://localhost:4222/logs.app.service2", "Service 2 logs"},
		{"nats://localhost:4222/logs.app.service1.error", "Service 1 errors"},
		{"nats://localhost:4222/logs.app.service2.error", "Service 2 errors"},
		{"nats://localhost:4222/logs.system", "System logs"},
		{"nats://localhost:4222/logs.system.health", "Health check logs"},
		{"nats://localhost:4222/logs.audit", "Audit logs"},
		{"nats://localhost:4222/logs.metrics", "Metrics logs"},
	}

	// Add all subjects as destinations
	addedCount := 0
	for _, subj := range subjects {
		if err := logger.AddDestination(subj.subject); err != nil {
			t.Logf("Note: Could not add subject %s: %v", subj.subject, err)
		} else {
			addedCount++
			t.Logf("Added: %s - %s", subj.subject, subj.description)
		}
	}

	t.Logf("\nConfigured %d hierarchical NATS subjects", addedCount)
	t.Log("Subscribers can use wildcards to receive specific log categories")
	t.Log("Examples:")
	t.Log("  - logs.* → all top-level log categories")
	t.Log("  - logs.app.* → all application logs")
	t.Log("  - logs.*.error → all error logs across services")
	t.Log("  - logs.app.service1.* → all logs from service1")
}

// TestNATSPerformanceWithMultipleDestinations benchmarks multiple NATS destinations
func TestNATSPerformanceWithMultipleDestinations(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	ensureNATSPluginRegistered(t)

	// Create logger
	logger, err := omni.NewBuilder().
		WithLevel(omni.LevelInfo).
		WithJSON().
		Build()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Add multiple destinations with different configurations
	configs := []struct {
		name string
		uri  string
	}{
		{"sync-small", "nats://localhost:4222/perf.sync?async=false"},
		{"async-small", "nats://localhost:4222/perf.async.small?batch=10&flush_interval=50"},
		{"async-medium", "nats://localhost:4222/perf.async.medium?batch=100&flush_interval=100"},
		{"async-large", "nats://localhost:4222/perf.async.large?batch=500&flush_interval=500"},
		{"queue-group", "nats://localhost:4222/perf.queue?queue=workers&batch=100"},
	}

	for _, cfg := range configs {
		if err := logger.AddDestination(cfg.uri); err != nil {
			t.Logf("Note: Could not add destination %s: %v", cfg.name, err)
		}
	}

	// Measure performance
	messageCount := 1000
	start := time.Now()

	for i := 0; i < messageCount; i++ {
		logger.InfoWithFields("Performance test message", map[string]interface{}{
			"sequence":    i,
			"timestamp":   time.Now().UnixNano(),
			"test_id":     "perf-test-multi-dest",
			"payload":     fmt.Sprintf("Message payload %d with some data to increase size", i),
			"metrics": map[string]interface{}{
				"cpu":    50.5 + float64(i%10),
				"memory": 1024 + i%100,
				"disk":   80 + i%20,
			},
		})
	}

	// Allow async operations to complete
	time.Sleep(1 * time.Second)

	elapsed := time.Since(start)
	msgsPerSec := float64(messageCount) / elapsed.Seconds()

	t.Logf("\nPerformance Results:")
	t.Logf("  Total messages: %d", messageCount)
	t.Logf("  Total time: %v", elapsed)
	t.Logf("  Messages/second: %.2f", msgsPerSec)
	t.Logf("  Destinations: %d", len(configs))
	t.Logf("  Effective msgs/sec per destination: %.2f", msgsPerSec*float64(len(configs)))

	// Check destination configuration
	dests := logger.Destinations
	for _, dest := range dests {
		if dest.Backend == omni.BackendPlugin {
			t.Logf("\nDestination: %s", dest.URI)
			t.Logf("  Backend: %d", dest.Backend)
			t.Logf("  Enabled: %t", dest.Enabled)
			t.Logf("  Name: %s", dest.Name)
		}
	}
}