// +build integration

package natsplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/wayneeseguin/omni"
)

// TestMultipleNATSSubjectsIntegration tests routing to multiple NATS subjects
func TestMultipleNATSSubjectsIntegration(t *testing.T) {
	// Skip if NATS is not available
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		t.Skipf("NATS server not available: %v", err)
	}
	defer nc.Close()

	// Create and register plugin
	plugin := &NATSBackendPlugin{}
	if err := plugin.Initialize(nil); err != nil {
		t.Fatalf("Failed to initialize plugin: %v", err)
	}

	if err := omni.RegisterBackendPlugin(plugin); err != nil {
		t.Fatalf("Failed to register plugin: %v", err)
	}

	// Create logger
	logger, err := omni.NewBuilder().
		WithLevel(omni.LevelDebug).
		WithJSON().
		Build()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set up message collectors for different subjects
	collectors := map[string]*messageCollector{
		"logs.info":    newMessageCollector(),
		"logs.error":   newMessageCollector(),
		"logs.debug":   newMessageCollector(),
		"logs.metrics": newMessageCollector(),
		"logs.audit":   newMessageCollector(),
	}

	// Subscribe to each subject
	var subs []*nats.Subscription
	for subject, collector := range collectors {
		sub, err := nc.Subscribe(subject, collector.handler)
		if err != nil {
			t.Fatalf("Failed to subscribe to %s: %v", subject, err)
		}
		subs = append(subs, sub)
		t.Logf("Subscribed to: %s", subject)
	}

	// Clean up subscriptions
	defer func() {
		for _, sub := range subs {
			sub.Unsubscribe()
		}
	}()

	// Add NATS destinations
	destinations := []string{
		"nats://localhost:4222/logs.info",
		"nats://localhost:4222/logs.error",
		"nats://localhost:4222/logs.debug?async=false", // Sync for immediate delivery
		"nats://localhost:4222/logs.metrics?batch=5&flush_interval=100",
		"nats://localhost:4222/logs.audit?format=json",
	}

	for _, dest := range destinations {
		if err := logger.AddDestination(dest); err != nil {
			t.Fatalf("Failed to add destination %s: %v", dest, err)
		}
	}

	// Create level-specific loggers that write to specific subjects
	infoLogger, _ := omni.NewBuilder().WithJSON().Build()
	infoLogger.AddDestination("nats://localhost:4222/logs.info")
	defer infoLogger.Close()

	errorLogger, _ := omni.NewBuilder().WithJSON().Build()
	errorLogger.AddDestination("nats://localhost:4222/logs.error")
	defer errorLogger.Close()

	// Send test messages
	testCases := []struct {
		logger  *omni.Omni
		level   string
		message string
		fields  map[string]interface{}
		subject string
	}{
		{
			logger:  infoLogger,
			level:   "info",
			message: "Info message 1",
			fields:  map[string]interface{}{"type": "test", "id": 1},
			subject: "logs.info",
		},
		{
			logger:  errorLogger,
			level:   "error",
			message: "Error message 1",
			fields:  map[string]interface{}{"error": "test error", "code": 500},
			subject: "logs.error",
		},
		{
			logger:  logger,
			level:   "debug",
			message: "Debug message to debug subject",
			fields:  map[string]interface{}{"debug": true, "trace_id": "abc123"},
			subject: "logs.debug",
		},
		{
			logger:  logger,
			level:   "info",
			message: "Metrics update",
			fields:  map[string]interface{}{"cpu": 45.5, "memory": 2048},
			subject: "logs.metrics",
		},
		{
			logger:  logger,
			level:   "info",
			message: "User action",
			fields:  map[string]interface{}{"action": "login", "user": "test@example.com"},
			subject: "logs.audit",
		},
	}

	// Send messages
	for _, tc := range testCases {
		switch tc.level {
		case "debug":
			tc.logger.DebugWithFields(tc.message, tc.fields)
		case "info":
			tc.logger.InfoWithFields(tc.message, tc.fields)
		case "error":
			tc.logger.ErrorWithFields(tc.message, tc.fields)
		}
	}

	// Wait for messages to be delivered
	time.Sleep(500 * time.Millisecond)

	// Verify messages were routed correctly
	t.Log("\n=== Message Routing Verification ===")
	for subject, collector := range collectors {
		count := collector.getCount()
		t.Logf("Subject %s received %d messages", subject, count)
		
		if count > 0 {
			messages := collector.getMessages()
			for i, msg := range messages {
				var data map[string]interface{}
				if err := json.Unmarshal(msg.Data, &data); err == nil {
					t.Logf("  Message %d: %v", i+1, data["message"])
				}
			}
		}
	}

	// Verify specific routing
	if collectors["logs.info"].getCount() < 1 {
		t.Error("Expected at least 1 message on logs.info")
	}
	if collectors["logs.error"].getCount() < 1 {
		t.Error("Expected at least 1 message on logs.error")
	}
	if collectors["logs.debug"].getCount() < 1 {
		t.Error("Expected at least 1 message on logs.debug")
	}
}

// TestNATSWildcardSubscriptions tests wildcard subscription patterns
func TestNATSWildcardSubscriptions(t *testing.T) {
	// Skip if NATS is not available
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		t.Skipf("NATS server not available: %v", err)
	}
	defer nc.Close()

	// Create and register plugin
	plugin := &NATSBackendPlugin{}
	plugin.Initialize(nil)
	omni.RegisterBackendPlugin(plugin)

	// Create logger
	logger, err := omni.NewBuilder().
		WithLevel(omni.LevelDebug).
		WithJSON().
		Build()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set up wildcard subscribers
	wildcardCollectors := map[string]*messageCollector{
		"logs.*":          newMessageCollector(), // All logs at first level
		"logs.app.*":      newMessageCollector(), // All app logs
		"logs.*.error":    newMessageCollector(), // All error logs
		"logs.app.>":      newMessageCollector(), // All app logs and sub-levels
		"logs.app.api.*":  newMessageCollector(), // All API logs
	}

	// Subscribe with wildcards
	var subs []*nats.Subscription
	for pattern, collector := range wildcardCollectors {
		sub, err := nc.Subscribe(pattern, collector.handler)
		if err != nil {
			t.Fatalf("Failed to subscribe to %s: %v", pattern, err)
		}
		subs = append(subs, sub)
		t.Logf("Subscribed to pattern: %s", pattern)
	}
	defer func() {
		for _, sub := range subs {
			sub.Unsubscribe()
		}
	}()

	// Add hierarchical destinations
	hierarchicalDests := []string{
		"nats://localhost:4222/logs.app",
		"nats://localhost:4222/logs.app.api",
		"nats://localhost:4222/logs.app.api.v1",
		"nats://localhost:4222/logs.app.api.v2",
		"nats://localhost:4222/logs.app.worker",
		"nats://localhost:4222/logs.app.error",
		"nats://localhost:4222/logs.system",
		"nats://localhost:4222/logs.system.error",
	}

	for _, dest := range hierarchicalDests {
		if err := logger.AddDestination(dest); err != nil {
			t.Fatalf("Failed to add destination %s: %v", dest, err)
		}
	}

	// Send messages to different subjects
	logger.InfoWithFields("App started", map[string]interface{}{"subject": "logs.app"})
	logger.InfoWithFields("API request", map[string]interface{}{"subject": "logs.app.api"})
	logger.InfoWithFields("API v1 call", map[string]interface{}{"subject": "logs.app.api.v1"})
	logger.InfoWithFields("API v2 call", map[string]interface{}{"subject": "logs.app.api.v2"})
	logger.ErrorWithFields("App error", map[string]interface{}{"subject": "logs.app.error"})
	logger.ErrorWithFields("System error", map[string]interface{}{"subject": "logs.system.error"})

	// Wait for delivery
	time.Sleep(300 * time.Millisecond)

	// Verify wildcard matching
	t.Log("\n=== Wildcard Subscription Results ===")
	expectations := map[string]int{
		"logs.*":         2, // logs.app, logs.system
		"logs.app.*":     3, // logs.app.api, logs.app.worker, logs.app.error
		"logs.*.error":   2, // logs.app.error, logs.system.error
		"logs.app.>":     5, // All app messages including sub-levels
		"logs.app.api.*": 2, // logs.app.api.v1, logs.app.api.v2
	}

	for pattern, collector := range wildcardCollectors {
		count := collector.getCount()
		expected := expectations[pattern]
		t.Logf("Pattern %-15s matched %d messages (expected >= %d)", pattern, count, expected)
		
		if count < expected {
			t.Errorf("Pattern %s: expected at least %d messages, got %d", pattern, expected, count)
		}
	}
}

// TestNATSQueueGroupLoadBalancing tests load balancing across queue group members
func TestNATSQueueGroupLoadBalancing(t *testing.T) {
	// Skip if NATS is not available
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		t.Skipf("NATS server not available: %v", err)
	}
	defer nc.Close()

	// Create and register plugin
	plugin := &NATSBackendPlugin{}
	plugin.Initialize(nil)
	omni.RegisterBackendPlugin(plugin)

	// Create logger
	logger, err := omni.NewBuilder().
		WithLevel(omni.LevelInfo).
		WithJSON().
		Build()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Add destination with queue group
	if err := logger.AddDestination("nats://localhost:4222/logs.balanced?queue=workers"); err != nil {
		t.Fatalf("Failed to add destination: %v", err)
	}

	// Create multiple queue subscribers (workers)
	numWorkers := 3
	workers := make([]*queueWorker, numWorkers)
	var subs []*nats.Subscription

	for i := 0; i < numWorkers; i++ {
		worker := &queueWorker{
			id:       i,
			messages: make([]*nats.Msg, 0),
		}
		workers[i] = worker

		sub, err := nc.QueueSubscribe("logs.balanced", "workers", worker.handler)
		if err != nil {
			t.Fatalf("Failed to create queue subscriber %d: %v", i, err)
		}
		subs = append(subs, sub)
	}

	defer func() {
		for _, sub := range subs {
			sub.Unsubscribe()
		}
	}()

	// Send many messages
	messageCount := 100
	for i := 0; i < messageCount; i++ {
		logger.InfoWithFields(fmt.Sprintf("Message %d", i), map[string]interface{}{
			"sequence": i,
			"timestamp": time.Now().Unix(),
		})
	}

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	// Verify load distribution
	t.Log("\n=== Queue Group Load Distribution ===")
	totalReceived := 0
	for i, worker := range workers {
		count := worker.getCount()
		totalReceived += count
		percentage := float64(count) / float64(messageCount) * 100
		t.Logf("Worker %d received %d messages (%.1f%%)", i, count, percentage)
	}

	t.Logf("Total messages received: %d/%d", totalReceived, messageCount)

	// Verify all messages were delivered exactly once
	if totalReceived != messageCount {
		t.Errorf("Expected %d messages total, got %d", messageCount, totalReceived)
	}

	// Verify relatively even distribution (allow 20% variance)
	expectedPerWorker := messageCount / numWorkers
	tolerance := int(float64(expectedPerWorker) * 0.4) // 40% tolerance
	
	for i, worker := range workers {
		count := worker.getCount()
		if count < expectedPerWorker-tolerance || count > expectedPerWorker+tolerance {
			t.Errorf("Worker %d: uneven distribution, got %d messages (expected ~%d ±%d)",
				i, count, expectedPerWorker, tolerance)
		}
	}
}

// Helper types for testing

type messageCollector struct {
	mu       sync.Mutex
	messages []*nats.Msg
	count    int32
}

func newMessageCollector() *messageCollector {
	return &messageCollector{
		messages: make([]*nats.Msg, 0),
	}
}

func (mc *messageCollector) handler(msg *nats.Msg) {
	mc.mu.Lock()
	mc.messages = append(mc.messages, msg)
	mc.mu.Unlock()
	atomic.AddInt32(&mc.count, 1)
}

func (mc *messageCollector) getCount() int {
	return int(atomic.LoadInt32(&mc.count))
}

func (mc *messageCollector) getMessages() []*nats.Msg {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	result := make([]*nats.Msg, len(mc.messages))
	copy(result, mc.messages)
	return result
}

type queueWorker struct {
	id       int
	mu       sync.Mutex
	messages []*nats.Msg
	count    int32
}

func (qw *queueWorker) handler(msg *nats.Msg) {
	qw.mu.Lock()
	qw.messages = append(qw.messages, msg)
	qw.mu.Unlock()
	atomic.AddInt32(&qw.count, 1)
}

func (qw *queueWorker) getCount() int {
	return int(atomic.LoadInt32(&qw.count))
}

// TestComplexRoutingScenario tests a complex real-world routing scenario
func TestComplexRoutingScenario(t *testing.T) {
	// Skip if NATS is not available
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		t.Skipf("NATS server not available: %v", err)
	}
	defer nc.Close()

	// Create and register plugin
	plugin := &NATSBackendPlugin{}
	plugin.Initialize(nil)
	omni.RegisterBackendPlugin(plugin)

	// Scenario: Multi-service architecture with different log routing needs
	// - Frontend service: logs.frontend.{level}
	// - API service: logs.api.{version}.{level}
	// - Background workers: logs.worker.{type}.{level}
	// - System monitoring: logs.system.{component}

	// Create service-specific loggers
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Frontend logger
	frontendLogger, _ := omni.NewBuilder().
		WithLevel(omni.LevelInfo).
		WithJSON().
		Build()
	frontendLogger.AddDestination("nats://localhost:4222/logs.frontend.info")
	frontendLogger.AddDestination("nats://localhost:4222/logs.frontend.error")
	frontendLogger.AddDestination("nats://localhost:4222/logs.frontend.audit?queue=audit-processors")
	defer frontendLogger.Close()

	// API logger with versioning
	apiV1Logger, _ := omni.NewBuilder().
		WithLevel(omni.LevelDebug).
		WithJSON().
		Build()
	apiV1Logger.AddDestination("nats://localhost:4222/logs.api.v1.debug?async=false")
	apiV1Logger.AddDestination("nats://localhost:4222/logs.api.v1.info")
	apiV1Logger.AddDestination("nats://localhost:4222/logs.api.v1.error")
	defer apiV1Logger.Close()

	apiV2Logger, _ := omni.NewBuilder().
		WithLevel(omni.LevelInfo).
		WithJSON().
		Build()
	apiV2Logger.AddDestination("nats://localhost:4222/logs.api.v2.info?batch=50")
	apiV2Logger.AddDestination("nats://localhost:4222/logs.api.v2.error?queue=error-handlers")
	defer apiV2Logger.Close()

	// Worker loggers
	paymentWorkerLogger, _ := omni.NewBuilder().
		WithLevel(omni.LevelInfo).
		WithJSON().
		Build()
	paymentWorkerLogger.AddDestination("nats://localhost:4222/logs.worker.payment.info")
	paymentWorkerLogger.AddDestination("nats://localhost:4222/logs.worker.payment.error")
	paymentWorkerLogger.AddDestination("nats://localhost:4222/logs.worker.payment.audit")
	defer paymentWorkerLogger.Close()

	// System logger
	systemLogger, _ := omni.NewBuilder().
		WithLevel(omni.LevelInfo).
		WithJSON().
		Build()
	systemLogger.AddDestination("nats://localhost:4222/logs.system.health")
	systemLogger.AddDestination("nats://localhost:4222/logs.system.metrics?batch=100&flush_interval=1000")
	systemLogger.AddDestination("nats://localhost:4222/logs.system.alerts?queue=alert-handlers")
	defer systemLogger.Close()

	// Set up comprehensive monitoring
	monitor := &routingMonitor{
		subjects: make(map[string]*messageCollector),
	}

	// Subscribe to various patterns to monitor routing
	patterns := []string{
		"logs.>",                    // Everything
		"logs.frontend.*",          // All frontend logs
		"logs.api.>",               // All API logs (both versions)
		"logs.api.*.error",         // All API errors
		"logs.worker.>",            // All worker logs
		"logs.worker.payment.audit", // Payment audit trail
		"logs.system.*",            // All system logs
		"*.error",                  // All errors (if using different root)
	}

	for _, pattern := range patterns {
		collector := newMessageCollector()
		monitor.subjects[pattern] = collector
		sub, err := nc.Subscribe(pattern, collector.handler)
		if err != nil {
			t.Fatalf("Failed to subscribe to %s: %v", pattern, err)
		}
		defer sub.Unsubscribe()
	}

	// Simulate realistic logging scenarios
	var wg sync.WaitGroup

	// Frontend activity
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			select {
			case <-ctx.Done():
				return
			default:
				frontendLogger.InfoWithFields("Page view", map[string]interface{}{
					"page":       fmt.Sprintf("/page/%d", i),
					"user_id":    fmt.Sprintf("user_%d", i%5),
					"session_id": fmt.Sprintf("session_%d", i%3),
				})

				if i%5 == 0 {
					frontendLogger.ErrorWithFields("Frontend error", map[string]interface{}{
						"error":    "Resource not found",
						"path":     fmt.Sprintf("/missing/%d", i),
						"status":   404,
					})
				}

				time.Sleep(50 * time.Millisecond)
			}
		}
	}()

	// API v1 activity
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 15; i++ {
			select {
			case <-ctx.Done():
				return
			default:
				apiV1Logger.DebugWithFields("API v1 request", map[string]interface{}{
					"endpoint": fmt.Sprintf("/api/v1/resource/%d", i),
					"method":   "GET",
					"duration": 10 + i*2,
				})

				apiV1Logger.InfoWithFields("API v1 response", map[string]interface{}{
					"status":   200,
					"bytes":    1024 + i*100,
					"endpoint": fmt.Sprintf("/api/v1/resource/%d", i),
				})

				if i%7 == 0 {
					apiV1Logger.ErrorWithFields("API v1 error", map[string]interface{}{
						"error":    "Database timeout",
						"query":    "SELECT * FROM users",
						"duration": 5000,
					})
				}

				time.Sleep(75 * time.Millisecond)
			}
		}
	}()

	// API v2 activity
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			select {
			case <-ctx.Done():
				return
			default:
				apiV2Logger.InfoWithFields("API v2 request", map[string]interface{}{
					"endpoint":   fmt.Sprintf("/api/v2/data/%d", i),
					"method":     "POST",
					"request_id": fmt.Sprintf("req_v2_%d", i),
				})

				if i%4 == 0 {
					apiV2Logger.ErrorWithFields("API v2 validation error", map[string]interface{}{
						"error":  "Invalid input",
						"field":  "email",
						"value":  "invalid-email",
					})
				}

				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	// Payment worker activity
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 8; i++ {
			select {
			case <-ctx.Done():
				return
			default:
				paymentWorkerLogger.InfoWithFields("Processing payment", map[string]interface{}{
					"payment_id": fmt.Sprintf("pay_%d", i),
					"amount":     100.50 + float64(i)*10,
					"currency":   "USD",
				})

				// Audit trail
				paymentWorkerLogger.InfoWithFields("Payment audit", map[string]interface{}{
					"payment_id": fmt.Sprintf("pay_%d", i),
					"action":     "processed",
					"result":     "success",
					"timestamp":  time.Now().Unix(),
				})

				if i%6 == 0 && i > 0 {
					paymentWorkerLogger.ErrorWithFields("Payment failed", map[string]interface{}{
						"payment_id": fmt.Sprintf("pay_%d", i),
						"error":      "Insufficient funds",
						"retry":      true,
					})
				}

				time.Sleep(200 * time.Millisecond)
			}
		}
	}()

	// System monitoring
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 12; i++ {
			select {
			case <-ctx.Done():
				return
			default:
				systemLogger.InfoWithFields("Health check", map[string]interface{}{
					"status":       "healthy",
					"uptime":       i * 60,
					"connections":  50 + i,
				})

				systemLogger.InfoWithFields("System metrics", map[string]interface{}{
					"cpu_percent":   30.5 + float64(i%20),
					"memory_mb":     2048 + i*50,
					"disk_percent":  45.0 + float64(i%10),
					"load_average":  []float64{1.2, 1.5, 1.8},
				})

				if i%8 == 0 && i > 0 {
					systemLogger.InfoWithFields("System alert", map[string]interface{}{
						"alert_type": "high_memory",
						"threshold":  80,
						"current":    85.5,
						"severity":   "warning",
					})
				}

				time.Sleep(150 * time.Millisecond)
			}
		}
	}()

	// Wait for all simulations
	wg.Wait()
	time.Sleep(1 * time.Second) // Allow batched messages to flush

	// Analyze routing results
	t.Log("\n=== Complex Routing Analysis ===")
	t.Log("Pattern                       | Messages | Description")
	t.Log("------------------------------|----------|-------------")
	
	totalMessages := 0
	for pattern, collector := range monitor.subjects {
		count := collector.getCount()
		totalMessages += count
		t.Logf("%-29s | %8d | %s", pattern, count, getPatternDescription(pattern))
	}

	// Verify routing correctness
	allLogs := monitor.subjects["logs.>"].getCount()
	t.Logf("\nTotal messages captured: %d", allLogs)

	// Check specific routing rules
	frontendLogs := monitor.subjects["logs.frontend.*"].getCount()
	apiLogs := monitor.subjects["logs.api.>"].getCount()
	workerLogs := monitor.subjects["logs.worker.>"].getCount()
	systemLogs := monitor.subjects["logs.system.*"].getCount()

	t.Logf("\nService breakdown:")
	t.Logf("  Frontend: %d", frontendLogs)
	t.Logf("  API:      %d", apiLogs)
	t.Logf("  Workers:  %d", workerLogs)
	t.Logf("  System:   %d", systemLogs)

	// Verify error routing
	apiErrors := monitor.subjects["logs.api.*.error"].getCount()
	paymentAudits := monitor.subjects["logs.worker.payment.audit"].getCount()

	t.Logf("\nSpecial routes:")
	t.Logf("  API Errors:     %d", apiErrors)
	t.Logf("  Payment Audits: %d", paymentAudits)

	// Basic sanity checks
	if allLogs == 0 {
		t.Error("No messages were captured")
	}
	if frontendLogs == 0 {
		t.Error("No frontend logs captured")
	}
	if apiLogs == 0 {
		t.Error("No API logs captured")
	}
	if apiErrors == 0 {
		t.Error("No API errors captured despite error conditions")
	}
}

type routingMonitor struct {
	mu       sync.RWMutex
	subjects map[string]*messageCollector
}

func getPatternDescription(pattern string) string {
	descriptions := map[string]string{
		"logs.>":                    "All logs",
		"logs.frontend.*":          "Frontend logs",
		"logs.api.>":               "All API logs",
		"logs.api.*.error":         "API errors",
		"logs.worker.>":            "Worker logs",
		"logs.worker.payment.audit": "Payment audits",
		"logs.system.*":            "System logs",
		"*.error":                  "Error logs",
	}
	
	if desc, ok := descriptions[pattern]; ok {
		return desc
	}
	return "Custom pattern"
}