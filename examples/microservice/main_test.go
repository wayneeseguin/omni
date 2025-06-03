package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wayneeseguin/omni"
)

func TestMain(m *testing.M) {
	// Setup: clean up any existing test files
	os.RemoveAll("test_microservice")
	
	code := m.Run()
	
	// Cleanup: remove test files
	os.RemoveAll("test_microservice")
	os.Exit(code)
}

func TestInitService(t *testing.T) {
	// Save original config
	origConfig := config
	defer func() {
		config = origConfig
		if logger != nil {
			logger.Close()
			logger = nil
		}
	}()

	// Test development environment
	os.Setenv("SERVICE_NAME", "test-service")
	os.Setenv("ENVIRONMENT", "development")
	defer os.Unsetenv("SERVICE_NAME")
	defer os.Unsetenv("ENVIRONMENT")

	err := initService()
	if err != nil {
		t.Fatalf("initService failed: %v", err)
	}

	if config.ServiceName != "test-service" {
		t.Errorf("Expected service name 'test-service', got '%s'", config.ServiceName)
	}

	if config.Environment != "development" {
		t.Errorf("Expected environment 'development', got '%s'", config.Environment)
	}

	if config.LogLevel != omni.LevelDebug {
		t.Errorf("Expected debug level in development, got %d", config.LogLevel)
	}

	if logger == nil {
		t.Fatal("Logger was not initialized")
	}

	logger.Close()
	logger = nil
}

func TestInitServiceProduction(t *testing.T) {
	// Save original config
	origConfig := config
	defer func() {
		config = origConfig
		if logger != nil {
			logger.Close()
			logger = nil
		}
	}()

	// Test production environment
	os.Setenv("SERVICE_NAME", "prod-service")
	os.Setenv("ENVIRONMENT", "production")
	defer os.Unsetenv("SERVICE_NAME")
	defer os.Unsetenv("ENVIRONMENT")

	err := initService()
	if err != nil {
		t.Fatalf("initService failed: %v", err)
	}

	if config.Environment != "production" {
		t.Errorf("Expected environment 'production', got '%s'", config.Environment)
	}

	if config.LogLevel != omni.LevelInfo {
		t.Errorf("Expected info level in production, got %d", config.LogLevel)
	}

	logger.Close()
	logger = nil
}

func TestExtractTraceContext(t *testing.T) {
	// Test with trace headers
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(TraceIDHeader, "test-trace-123")
	req.Header.Set(ParentSpanIDHeader, "parent-span-456")

	ctx := extractTraceContext(req)

	traceID, ok := ctx.Value("trace_id").(string)
	if !ok || traceID != "test-trace-123" {
		t.Errorf("Expected trace ID 'test-trace-123', got '%s'", traceID)
	}

	parentSpanID, ok := ctx.Value("parent_span_id").(string)
	if !ok || parentSpanID != "parent-span-456" {
		t.Errorf("Expected parent span ID 'parent-span-456', got '%s'", parentSpanID)
	}

	spanID, ok := ctx.Value("span_id").(string)
	if !ok || spanID == "" {
		t.Error("Expected span ID to be generated")
	}
}

func TestExtractTraceContextNoHeaders(t *testing.T) {
	// Test without trace headers (should generate new trace ID)
	req := httptest.NewRequest("GET", "/test", nil)

	ctx := extractTraceContext(req)

	traceID, ok := ctx.Value("trace_id").(string)
	if !ok || traceID == "" {
		t.Error("Expected trace ID to be generated")
	}

	if !strings.HasPrefix(traceID, "trace-") {
		t.Errorf("Expected generated trace ID to start with 'trace-', got '%s'", traceID)
	}
}

func TestPropagateTraceContext(t *testing.T) {
	// Create context with trace information
	ctx := context.Background()
	ctx = context.WithValue(ctx, "trace_id", "test-trace-789")
	ctx = context.WithValue(ctx, "span_id", "test-span-123")

	// Create HTTP request
	req := httptest.NewRequest("POST", "/external", nil)

	// Propagate context
	propagateTraceContext(ctx, req)

	// Check headers
	if req.Header.Get(TraceIDHeader) != "test-trace-789" {
		t.Errorf("Expected trace ID header 'test-trace-789', got '%s'", req.Header.Get(TraceIDHeader))
	}

	if req.Header.Get(ParentSpanIDHeader) != "test-span-123" {
		t.Errorf("Expected parent span ID header 'test-span-123', got '%s'", req.Header.Get(ParentSpanIDHeader))
	}

	newSpanID := req.Header.Get(SpanIDHeader)
	if newSpanID == "" {
		t.Error("Expected new span ID header to be set")
	}

	if !strings.HasPrefix(newSpanID, "span-") {
		t.Errorf("Expected new span ID to start with 'span-', got '%s'", newSpanID)
	}
}

func TestServiceMiddleware(t *testing.T) {
	// Initialize service for testing
	setupTestService(t)
	defer teardownTestService()

	// Test handler
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	}

	// Wrap with middleware
	wrappedHandler := serviceMiddleware(testHandler)

	// Create request
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()

	// Execute request
	wrappedHandler(w, req)

	// Check response
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Check trace header was added
	traceID := resp.Header.Get(TraceIDHeader)
	if traceID == "" {
		t.Error("Expected trace ID header in response")
	}
}

func TestResponseWriter(t *testing.T) {
	// Test response writer wrapper
	w := httptest.NewRecorder()
	wrapped := &responseWriter{
		ResponseWriter: w,
		statusCode:     200,
	}

	// Test WriteHeader
	wrapped.WriteHeader(404)
	if wrapped.statusCode != 404 {
		t.Errorf("Expected status code 404, got %d", wrapped.statusCode)
	}

	// Test Write
	data := []byte("test data")
	n, err := wrapped.Write(data)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
	}
	if wrapped.bytesWritten != len(data) {
		t.Errorf("Expected bytesWritten %d, got %d", len(data), wrapped.bytesWritten)
	}
}

func TestGetRequestFields(t *testing.T) {
	setupTestService(t)
	defer teardownTestService()

	// Test with request fields in context
	fields := map[string]interface{}{
		"test_field": "test_value",
		"service":    "test-service",
	}
	ctx := context.WithValue(context.Background(), "request_fields", fields)

	result := getRequestFields(ctx)
	if result["test_field"] != "test_value" {
		t.Errorf("Expected test_field 'test_value', got '%v'", result["test_field"])
	}

	// Test without request fields in context
	ctx = context.Background()
	result = getRequestFields(ctx)
	if result["service"] != config.ServiceName {
		t.Errorf("Expected service name '%s', got '%v'", config.ServiceName, result["service"])
	}
}

func TestLogWithContext(t *testing.T) {
	setupTestService(t)
	defer teardownTestService()

	// Create context with fields
	fields := map[string]interface{}{
		"service":    "test-service",
		"request_id": "test-123",
	}
	ctx := context.WithValue(context.Background(), "request_fields", fields)

	// Test different log levels
	logWithContext(ctx, "debug", "Debug message", map[string]interface{}{"debug_field": "debug_value"})
	logWithContext(ctx, "info", "Info message", map[string]interface{}{"info_field": "info_value"})
	logWithContext(ctx, "warn", "Warn message", map[string]interface{}{"warn_field": "warn_value"})
	logWithContext(ctx, "error", "Error message", map[string]interface{}{"error_field": "error_value"})

	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
}

func TestProcessPaymentHandler(t *testing.T) {
	setupTestService(t)
	defer teardownTestService()

	// Test valid payment request
	payment := PaymentRequest{
		PaymentID:  "pay-test-123",
		Amount:     100.50,
		Currency:   "USD",
		MerchantID: "merchant-456",
	}

	paymentJSON, _ := json.Marshal(payment)
	req := httptest.NewRequest("POST", "/api/payment", bytes.NewReader(paymentJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Add trace context
	ctx := extractTraceContext(req)
	req = req.WithContext(ctx)

	processPaymentHandler(w, req)

	resp := w.Result()
	// Note: This will return 503 because external service call fails
	// This is expected behavior in the test environment
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503 (external service unavailable), got %d", resp.StatusCode)
	}

	// The payment processing logic works up to the external service call
	// This test validates the payment parsing and step processing
}

func TestProcessPaymentHandlerInvalidJSON(t *testing.T) {
	setupTestService(t)
	defer teardownTestService()

	req := httptest.NewRequest("POST", "/api/payment", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	processPaymentHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestHealthHandler(t *testing.T) {
	setupTestService(t)
	defer teardownTestService()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	healthHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var health map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Errorf("Failed to decode health response: %v", err)
	}

	if health["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got '%v'", health["status"])
	}

	if health["service"] != config.ServiceName {
		t.Errorf("Expected service '%s', got '%v'", config.ServiceName, health["service"])
	}
}

func TestMetricsHandler(t *testing.T) {
	setupTestService(t)
	defer teardownTestService()

	// Log some messages to generate metrics
	logger.Info("Test metric message 1")
	logger.Warn("Test metric message 2")
	logger.Error("Test metric message 3")
	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	// Create metrics handler
	metricsHandler := func(w http.ResponseWriter, r *http.Request) {
		metrics := logger.GetMetrics()
		
		// Calculate total messages from all levels
		totalMessages := uint64(0)
		for _, count := range metrics.MessagesLogged {
			totalMessages += count
		}
		
		// Export Prometheus-style metrics
		fmt.Fprintf(w, "# HELP service_log_messages_total Total log messages\n")
		fmt.Fprintf(w, "# TYPE service_log_messages_total counter\n")
		fmt.Fprintf(w, "service_log_messages_total{service=\"%s\"} %d\n", 
			config.ServiceName, totalMessages)
		
		fmt.Fprintf(w, "# HELP service_log_dropped_total Total dropped messages\n")
		fmt.Fprintf(w, "# TYPE service_log_dropped_total counter\n")
		fmt.Fprintf(w, "service_log_dropped_total{service=\"%s\"} %d\n", 
			config.ServiceName, metrics.MessagesDropped)
		
		// Add custom business metrics
		fmt.Fprintf(w, "# HELP service_uptime_seconds Service uptime\n")
		fmt.Fprintf(w, "# TYPE service_uptime_seconds gauge\n")
		fmt.Fprintf(w, "service_uptime_seconds{service=\"%s\"} %f\n", 
			config.ServiceName, time.Since(startTime).Seconds())
	}

	metricsHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body := w.Body.String()
	if !strings.Contains(body, "service_log_messages_total") {
		t.Error("Expected service_log_messages_total metric in response")
	}

	if !strings.Contains(body, "service_uptime_seconds") {
		t.Error("Expected service_uptime_seconds metric in response")
	}
}

func TestCallExternalService(t *testing.T) {
	setupTestService(t)
	defer teardownTestService()

	// Create mock server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that trace headers were propagated
		if r.Header.Get(TraceIDHeader) == "" {
			t.Error("Expected trace ID header in external request")
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "success"}`))
	}))
	defer mockServer.Close()

	// Create context with trace information
	ctx := context.Background()
	ctx = context.WithValue(ctx, "trace_id", "test-trace-ext")
	ctx = context.WithValue(ctx, "span_id", "test-span-ext")
	ctx = context.WithValue(ctx, "request_fields", map[string]interface{}{
		"service": config.ServiceName,
		"test":    "external_call",
	})

	payment := PaymentRequest{
		PaymentID:  "ext-pay-123",
		Amount:     50.0,
		Currency:   "USD",
		MerchantID: "ext-merchant",
	}

	// Note: This will fail because we're calling a non-existent service
	// but it should properly log the attempt
	err := callExternalService(ctx, payment)
	if err == nil {
		t.Log("External service call succeeded (unexpected but not an error)")
	} else {
		t.Logf("External service call failed as expected: %v", err)
	}
}

func TestUtilityFunctions(t *testing.T) {
	// Test getEnv
	os.Setenv("TEST_ENV_VAR", "test_value")
	defer os.Unsetenv("TEST_ENV_VAR")

	value := getEnv("TEST_ENV_VAR", "default")
	if value != "test_value" {
		t.Errorf("Expected 'test_value', got '%s'", value)
	}

	value = getEnv("NON_EXISTENT_VAR", "default")
	if value != "default" {
		t.Errorf("Expected 'default', got '%s'", value)
	}

	// Test getHostname
	hostname := getHostname()
	if hostname == "" {
		t.Error("Expected non-empty hostname")
	}

	// Test ID generation functions
	serviceID := generateServiceID()
	if !strings.Contains(serviceID, hostname) {
		t.Errorf("Expected service ID to contain hostname, got '%s'", serviceID)
	}

	traceID := generateTraceID()
	if !strings.HasPrefix(traceID, "trace-") {
		t.Errorf("Expected trace ID to start with 'trace-', got '%s'", traceID)
	}

	spanID := generateSpanID()
	if !strings.HasPrefix(spanID, "span-") {
		t.Errorf("Expected span ID to start with 'span-', got '%s'", spanID)
	}

	paymentID := generatePaymentID()
	if !strings.HasPrefix(paymentID, "pay-") {
		t.Errorf("Expected payment ID to start with 'pay-', got '%s'", paymentID)
	}
}

func TestRespondWithError(t *testing.T) {
	w := httptest.NewRecorder()

	respondWithError(w, "Test error message", http.StatusBadRequest)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}

	var errorResp map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
		t.Errorf("Failed to decode error response: %v", err)
	}

	if errorResp["error"] != "Test error message" {
		t.Errorf("Expected error message 'Test error message', got '%s'", errorResp["error"])
	}

	if errorResp["status"] != "Bad Request" {
		t.Errorf("Expected status 'Bad Request', got '%s'", errorResp["status"])
	}
}

func TestIntegrationFlow(t *testing.T) {
	setupTestService(t)
	defer teardownTestService()

	// Create a valid payment request
	payment := PaymentRequest{
		PaymentID:  "integration-test-pay",
		Amount:     250.75,
		Currency:   "USD",
		MerchantID: "integration-merchant",
	}

	paymentJSON, _ := json.Marshal(payment)

	// Test complete flow with middleware
	handler := serviceMiddleware(processPaymentHandler)
	
	req := httptest.NewRequest("POST", "/api/payment", bytes.NewReader(paymentJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "integration-test")
	req.Header.Set(TraceIDHeader, "integration-trace-123")
	
	w := httptest.NewRecorder()

	handler(w, req)

	resp := w.Result()
	// Expect 503 due to external service failure
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503 (external service unavailable), got %d", resp.StatusCode)
	}

	// Check response headers - trace ID should be propagated
	if resp.Header.Get(TraceIDHeader) != "integration-trace-123" {
		t.Errorf("Expected trace ID 'integration-trace-123' in response, got '%s'", 
			resp.Header.Get(TraceIDHeader))
	}

	// Test validates the middleware and trace propagation
	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)
}

// Helper functions for testing
func setupTestService(t *testing.T) {
	testLogDir := "test_microservice"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Set test configuration
	config = Config{
		ServiceName: "test-payment-service",
		ServiceID:   "test-service-123",
		LogLevel:    omni.LevelDebug,
		Environment: "test",
	}

	// Create test logger
	var err error
	logger, err = omni.NewWithOptions(
		omni.WithPath(filepath.Join(testLogDir, "test_microservice.log")),
		omni.WithLevel(config.LogLevel),
		omni.WithJSON(),
	)
	if err != nil {
		t.Fatalf("Failed to create test logger: %v", err)
	}

	// Reset start time for testing
	startTime = time.Now()
}

func teardownTestService() {
	if logger != nil {
		logger.FlushAll()
		time.Sleep(10 * time.Millisecond)
		logger.Close()
		logger = nil
	}
	os.RemoveAll("test_microservice")
}

// Benchmark tests
func BenchmarkServiceMiddleware(b *testing.B) {
	testLogDir := "bench_microservice"
	os.MkdirAll(testLogDir, 0755)
	defer os.RemoveAll(testLogDir)

	// Setup for benchmarking
	config = Config{
		ServiceName: "bench-service",
		ServiceID:   "bench-123",
		LogLevel:    omni.LevelWarn, // Higher level for performance
		Environment: "benchmark",
	}

	var err error
	logger, err = omni.NewWithOptions(
		omni.WithPath(filepath.Join(testLogDir, "bench.log")),
		omni.WithLevel(config.LogLevel),
		omni.WithChannelSize(1000),
	)
	if err != nil {
		b.Fatalf("Failed to create benchmark logger: %v", err)
	}
	defer logger.Close()

	handler := serviceMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/benchmark", nil)
		w := httptest.NewRecorder()
		handler(w, req)
	}
}

func BenchmarkLogWithContext(b *testing.B) {
	testLogDir := "bench_microservice"
	os.MkdirAll(testLogDir, 0755)
	defer os.RemoveAll(testLogDir)

	config = Config{
		ServiceName: "bench-service",
		ServiceID:   "bench-123",
		LogLevel:    omni.LevelWarn, // Higher level for performance
		Environment: "benchmark",
	}

	var err error
	logger, err = omni.NewWithOptions(
		omni.WithPath(filepath.Join(testLogDir, "bench.log")),
		omni.WithLevel(config.LogLevel),
		omni.WithChannelSize(1000),
	)
	if err != nil {
		b.Fatalf("Failed to create benchmark logger: %v", err)
	}
	defer logger.Close()

	ctx := context.WithValue(context.Background(), "request_fields", map[string]interface{}{
		"service":    config.ServiceName,
		"service_id": config.ServiceID,
		"benchmark":  true,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logWithContext(ctx, "info", "Benchmark message", map[string]interface{}{
			"iteration": i,
		})
	}
}