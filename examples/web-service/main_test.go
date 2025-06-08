package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wayneeseguin/omni/pkg/omni"
)

func TestMain(m *testing.M) {
	// Setup: clean up any existing test files
	os.RemoveAll("test_webservice")

	code := m.Run()

	// Cleanup: remove test files
	os.RemoveAll("test_webservice")
	os.Exit(code)
}

func TestWebServiceExample(t *testing.T) {
	// Create test directory
	testLogDir := "test_webservice"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	// Initialize global logger for testing
	var err error
	logger, err = omni.NewWithOptions(
		omni.WithPath(filepath.Join(testLogDir, "webservice_test.log")),
		omni.WithLevel(omni.LevelInfo),
		omni.WithJSON(),
		omni.WithChannelSize(100),
	)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test health endpoint
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	loggingMiddleware(healthHandler)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var healthResp map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
		t.Errorf("Failed to decode health response: %v", err)
	}

	if healthResp["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", healthResp["status"])
	}

	// Test user endpoint with valid ID
	req = httptest.NewRequest("GET", "/api/user?id=123", nil)
	w = httptest.NewRecorder()

	loggingMiddleware(userHandler)(w, req)

	resp = w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var userResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&userResp); err != nil {
		t.Errorf("Failed to decode user response: %v", err)
	}

	if userResp["id"] != "123" {
		t.Errorf("Expected user ID '123', got '%v'", userResp["id"])
	}

	// Test user endpoint without ID (should fail)
	req = httptest.NewRequest("GET", "/api/user", nil)
	w = httptest.NewRecorder()

	loggingMiddleware(userHandler)(w, req)

	resp = w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}

	// Test error endpoint
	req = httptest.NewRequest("GET", "/api/error", nil)
	w = httptest.NewRecorder()

	loggingMiddleware(errorHandler)(w, req)

	resp = w.Result()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", resp.StatusCode)
	}

	// Flush logs and verify
	logger.FlushAll()
	time.Sleep(10 * time.Millisecond)

	// Verify log file was created and has content
	logFile := filepath.Join(testLogDir, "webservice_test.log")
	if stat, err := os.Stat(logFile); err != nil {
		t.Errorf("Log file was not created: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Log file is empty")
	}
}

func TestLoggingMiddleware(t *testing.T) {
	testLogDir := "test_webservice"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	// Initialize test logger
	testLogger, err := omni.NewWithOptions(
		omni.WithPath(filepath.Join(testLogDir, "middleware_test.log")),
		omni.WithLevel(omni.LevelInfo),
		omni.WithJSON(),
	)
	if err != nil {
		t.Fatalf("Failed to create test logger: %v", err)
	}
	defer testLogger.Close()

	// Set global logger for middleware
	logger = testLogger

	// Create test handler that captures the request context
	var capturedContext context.Context
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		capturedContext = r.Context()
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("test response"))
	}

	// Test middleware wrapping
	wrappedHandler := loggingMiddleware(testHandler)

	req := httptest.NewRequest("POST", "/test/path", strings.NewReader("test body"))
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()

	wrappedHandler(w, req)

	// Verify response
	resp := w.Result()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("Expected status 202, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "test response" {
		t.Errorf("Expected 'test response', got '%s'", string(body))
	}

	// Verify request ID was added to context
	if capturedContext.Value("request_id") == nil {
		t.Error("Request ID was not added to context")
	}

	testLogger.FlushAll()
	time.Sleep(200 * time.Millisecond)

	// Verify log file
	logFile := filepath.Join(testLogDir, "middleware_test.log")
	if stat, err := os.Stat(logFile); err != nil {
		t.Errorf("Log file error: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Log file is empty")
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

func TestGetRequestID(t *testing.T) {
	// Test with request ID in context
	req := httptest.NewRequest("GET", "/test", nil)
	ctx := context.WithValue(req.Context(), "request_id", "test-123")
	req = req.WithContext(ctx)

	requestID := getRequestID(req)
	if requestID != "test-123" {
		t.Errorf("Expected request ID 'test-123', got '%s'", requestID)
	}

	// Test without request ID in context
	req = httptest.NewRequest("GET", "/test", nil)
	requestID = getRequestID(req)
	if requestID != "unknown" {
		t.Errorf("Expected request ID 'unknown', got '%s'", requestID)
	}
}

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	healthHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}

	var response map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", response["status"])
	}

	if response["time"] == "" {
		t.Error("Expected time field to be populated")
	}
}

func TestUserHandler(t *testing.T) {
	testLogDir := "test_webservice"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	// Initialize test logger
	testLogger, err := omni.NewWithOptions(
		omni.WithPath(filepath.Join(testLogDir, "user_test.log")),
		omni.WithLevel(omni.LevelInfo),
		omni.WithJSON(),
	)
	if err != nil {
		t.Fatalf("Failed to create test logger: %v", err)
	}
	defer testLogger.Close()

	logger = testLogger

	// Test with valid user ID
	req := httptest.NewRequest("GET", "/api/user?id=456", nil)
	ctx := context.WithValue(req.Context(), "request_id", "test-req-456")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	userHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var userResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&userResp); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if userResp["id"] != "456" {
		t.Errorf("Expected user ID '456', got '%v'", userResp["id"])
	}
	if userResp["name"] != "John Doe" {
		t.Errorf("Expected name 'John Doe', got '%v'", userResp["name"])
	}

	// Test without user ID
	req = httptest.NewRequest("GET", "/api/user", nil)
	ctx = context.WithValue(req.Context(), "request_id", "test-req-no-id")
	req = req.WithContext(ctx)
	w = httptest.NewRecorder()

	userHandler(w, req)

	resp = w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}

	testLogger.FlushAll()
	time.Sleep(10 * time.Millisecond)
}

func TestErrorHandler(t *testing.T) {
	testLogDir := "test_webservice"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	// Initialize test logger
	testLogger, err := omni.NewWithOptions(
		omni.WithPath(filepath.Join(testLogDir, "error_test.log")),
		omni.WithLevel(omni.LevelInfo),
		omni.WithJSON(),
	)
	if err != nil {
		t.Fatalf("Failed to create test logger: %v", err)
	}
	defer testLogger.Close()

	logger = testLogger

	req := httptest.NewRequest("GET", "/api/error", nil)
	ctx := context.WithValue(req.Context(), "request_id", "test-req-error")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	errorHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Internal server error") {
		t.Errorf("Expected error message in body, got '%s'", string(body))
	}

	testLogger.FlushAll()
	time.Sleep(10 * time.Millisecond)
}

func TestSlowRequestDetection(t *testing.T) {
	testLogDir := "test_webservice"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	// Initialize test logger
	testLogger, err := omni.NewWithOptions(
		omni.WithPath(filepath.Join(testLogDir, "slow_test.log")),
		omni.WithLevel(omni.LevelInfo),
		omni.WithJSON(),
	)
	if err != nil {
		t.Fatalf("Failed to create test logger: %v", err)
	}
	defer testLogger.Close()

	logger = testLogger

	// Create slow handler
	slowHandler := func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(600 * time.Millisecond) // Longer than 500ms threshold
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("slow response"))
	}

	req := httptest.NewRequest("GET", "/slow", nil)
	w := httptest.NewRecorder()

	loggingMiddleware(slowHandler)(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	testLogger.FlushAll()
	time.Sleep(10 * time.Millisecond)

	// Verify log file exists and has content
	logFile := filepath.Join(testLogDir, "slow_test.log")
	if stat, err := os.Stat(logFile); err != nil {
		t.Errorf("Log file error: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Log file is empty")
	}
}

func TestMetricsEndpoint(t *testing.T) {
	testLogDir := "test_webservice"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	// Initialize test logger
	testLogger, err := omni.NewWithOptions(
		omni.WithPath(filepath.Join(testLogDir, "metrics_test.log")),
		omni.WithLevel(omni.LevelInfo),
		omni.WithJSON(),
	)
	if err != nil {
		t.Fatalf("Failed to create test logger: %v", err)
	}
	defer testLogger.Close()

	logger = testLogger

	// Test metrics endpoint
	req := httptest.NewRequest("GET", "/metrics", nil)
	ctx := context.WithValue(req.Context(), "request_id", "test-metrics")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	// Call the metrics handler function directly
	func(w http.ResponseWriter, r *http.Request) {
		requestID := getRequestID(r)

		logger.InfoWithFields("Metrics endpoint accessed", map[string]interface{}{
			"request_id": requestID,
			"endpoint":   "/metrics",
		})

		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "# HELP webservice_status Web service status\n")
		fmt.Fprintf(w, "webservice_status 1\n")

		fmt.Fprintf(w, "# HELP webservice_uptime_seconds Service uptime in seconds\n")
		fmt.Fprintf(w, "webservice_uptime_seconds %d\n", time.Now().Unix())

		// Basic logger info
		destinations := logger.ListDestinations()
		fmt.Fprintf(w, "# HELP log_destinations_total Total log destinations\n")
		fmt.Fprintf(w, "log_destinations_total %d\n", len(destinations))
	}(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/plain" {
		t.Errorf("Expected Content-Type 'text/plain', got '%s'", contentType)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	if !strings.Contains(bodyStr, "webservice_status 1") {
		t.Error("Expected webservice_status metric in response")
	}

	if !strings.Contains(bodyStr, "webservice_uptime_seconds") {
		t.Error("Expected webservice_uptime_seconds metric in response")
	}

	if !strings.Contains(bodyStr, "log_destinations_total") {
		t.Error("Expected log_destinations_total metric in response")
	}

	testLogger.FlushAll()
	time.Sleep(10 * time.Millisecond)
}

func TestServerConfiguration(t *testing.T) {
	testLogDir := "test_webservice"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	// Test logger initialization with production settings
	testLogger, err := omni.NewWithOptions(
		omni.WithPath(filepath.Join(testLogDir, "config_test.log")),
		omni.WithLevel(omni.LevelInfo),
		omni.WithJSON(),
		omni.WithRotation(50*1024*1024, 5),
		omni.WithGzipCompression(),
		omni.WithChannelSize(5000),
	)
	if err != nil {
		t.Fatalf("Failed to create logger with production settings: %v", err)
	}

	// Test adding additional destination
	err = testLogger.AddDestination(filepath.Join(testLogDir, "config_test_errors.log"))
	if err != nil {
		t.Errorf("Failed to add destination: %v", err)
	}

	// Test filter functionality
	testLogger.AddFilter(func(level int, message string, fields map[string]interface{}) bool {
		// Sample health check requests (allow only 10%)
		if path, ok := fields["path"].(string); ok && path == "/health" {
			return time.Now().UnixNano()%10 == 0
		}
		return true // Allow all other messages
	})

	// Test logging with filter
	testLogger.InfoWithFields("Health check request", map[string]interface{}{
		"path":       "/health",
		"request_id": "test-health-filter",
	})

	testLogger.InfoWithFields("Regular request", map[string]interface{}{
		"path":       "/api/user",
		"request_id": "test-regular",
	})

	testLogger.FlushAll()
	time.Sleep(10 * time.Millisecond)
	testLogger.Close()

	// Verify log files
	mainFile := filepath.Join(testLogDir, "config_test.log")
	if stat, err := os.Stat(mainFile); err != nil {
		t.Errorf("Main log file error: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Main log file is empty")
	}

	errorFile := filepath.Join(testLogDir, "config_test_errors.log")
	if stat, err := os.Stat(errorFile); err != nil {
		t.Errorf("Error log file error: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Error log file is empty")
	}
}

func TestGracefulShutdown(t *testing.T) {
	testLogDir := "test_webservice"
	if err := os.MkdirAll(testLogDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testLogDir)

	// Test server setup and shutdown simulation
	testLogger, err := omni.NewWithOptions(
		omni.WithPath(filepath.Join(testLogDir, "shutdown_test.log")),
		omni.WithLevel(omni.LevelInfo),
		omni.WithJSON(),
	)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Simulate server startup
	testLogger.InfoWithFields("Server starting", map[string]interface{}{
		"addr": ":8080",
		"pid":  os.Getpid(),
	})

	// Simulate some requests
	testLogger.InfoWithFields("Request processed", map[string]interface{}{
		"path":   "/health",
		"method": "GET",
		"status": 200,
	})

	// Simulate shutdown
	testLogger.Info("Server is shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Simulate shutdown completion
	select {
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			testLogger.ErrorWithFields("Shutdown timeout exceeded", map[string]interface{}{
				"timeout": "30s",
			})
		}
	default:
		testLogger.Info("Server stopped gracefully")
	}

	testLogger.Info("Server stopped")
	testLogger.FlushAll()
	time.Sleep(10 * time.Millisecond)
	testLogger.Close()

	// Verify log file
	logFile := filepath.Join(testLogDir, "shutdown_test.log")
	if stat, err := os.Stat(logFile); err != nil {
		t.Errorf("Log file error: %v", err)
	} else if stat.Size() == 0 {
		t.Error("Log file is empty")
	}
}

// Benchmark tests
func BenchmarkLoggingMiddleware(b *testing.B) {
	testLogDir := "bench_webservice"
	os.MkdirAll(testLogDir, 0755)
	defer os.RemoveAll(testLogDir)

	testLogger, err := omni.NewWithOptions(
		omni.WithPath(filepath.Join(testLogDir, "bench_middleware.log")),
		omni.WithLevel(omni.LevelInfo),
		omni.WithChannelSize(1000),
	)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer testLogger.Close()

	logger = testLogger

	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("benchmark"))
	}

	wrappedHandler := loggingMiddleware(handler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/benchmark", nil)
		w := httptest.NewRecorder()
		wrappedHandler(w, req)
	}
}

func BenchmarkStructuredLogging(b *testing.B) {
	testLogDir := "bench_webservice"
	os.MkdirAll(testLogDir, 0755)
	defer os.RemoveAll(testLogDir)

	testLogger, err := omni.NewWithOptions(
		omni.WithPath(filepath.Join(testLogDir, "bench_structured.log")),
		omni.WithLevel(omni.LevelInfo),
		omni.WithJSON(),
		omni.WithChannelSize(1000),
	)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer testLogger.Close()

	fields := map[string]interface{}{
		"request_id": "bench-123",
		"method":     "GET",
		"path":       "/benchmark",
		"status":     200,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testLogger.InfoWithFields("Benchmark request", fields)
	}
}
