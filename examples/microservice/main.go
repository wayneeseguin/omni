package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/wayneeseguin/omni/pkg/omni"
)

// Service configuration
type Config struct {
	ServiceName string
	ServiceID   string
	LogLevel    int
	Environment string
}

// Global logger and config
var (
	logger *omni.Omni
	config Config
)

// Trace headers
const (
	TraceIDHeader      = "X-Trace-ID"
	SpanIDHeader       = "X-Span-ID"
	ParentSpanIDHeader = "X-Parent-Span-ID"
)

// Initialize service
func initService() error {
	// Load configuration
	config = Config{
		ServiceName: getEnv("SERVICE_NAME", "payment-service"),
		ServiceID:   getEnv("SERVICE_ID", generateServiceID()),
		LogLevel:    omni.LevelInfo,
		Environment: getEnv("ENVIRONMENT", "development"),
	}
	
	if config.Environment == "development" {
		config.LogLevel = omni.LevelDebug
	}
	
	// Create logger with service context
	options := []omni.Option{
		omni.WithPath(fmt.Sprintf("/tmp/%s.log", config.ServiceName)), // Use /tmp for demo
		omni.WithLevel(config.LogLevel),
		omni.WithJSON(), // Always use JSON for microservices
	}
	
	// Production settings
	if config.Environment == "production" {
		options = append(options,
			omni.WithRotation(200*1024*1024, 20),  // 200MB files, keep 20
			omni.WithGzipCompression(),
			omni.WithChannelSize(10000),
		)
	}
	
	var err error
	logger, err = omni.NewWithOptions(options...)
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}
	
	// Log service initialization with context fields
	logger.InfoWithFields("Service initializing", map[string]interface{}{
		"service":     config.ServiceName,
		"service_id":  config.ServiceID,
		"environment": config.Environment,
		"version":     "1.0.0",
		"host":        getHostname(),
	})
	
	// Configure sampling for production
	if config.Environment == "production" {
		// Add sampling filter for non-critical messages
		logger.AddFilter(func(level int, message string, fields map[string]interface{}) bool {
			// Always allow warnings and errors
			if level >= omni.LevelWarn {
				return true
			}
			// Sample other messages at 10%
			return time.Now().UnixNano()%10 == 0
		})
	}
	
	logger.Info("Service initialized")
	
	return nil
}

// Extract or generate trace context
func extractTraceContext(r *http.Request) context.Context {
	ctx := r.Context()
	
	// Extract trace ID
	traceID := r.Header.Get(TraceIDHeader)
	if traceID == "" {
		traceID = generateTraceID()
	}
	ctx = context.WithValue(ctx, "trace_id", traceID)
	
	// Extract span information
	parentSpanID := r.Header.Get(ParentSpanIDHeader)
	spanID := generateSpanID()
	
	ctx = context.WithValue(ctx, "span_id", spanID)
	ctx = context.WithValue(ctx, "parent_span_id", parentSpanID)
	
	return ctx
}

// Propagate trace context to outgoing requests
func propagateTraceContext(ctx context.Context, req *http.Request) {
	if traceID, ok := ctx.Value("trace_id").(string); ok && traceID != "" {
		req.Header.Set(TraceIDHeader, traceID)
	}
	
	if spanID, ok := ctx.Value("span_id").(string); ok {
		req.Header.Set(ParentSpanIDHeader, spanID)
		req.Header.Set(SpanIDHeader, generateSpanID())
	}
}

// Service middleware
func serviceMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Extract trace context
		ctx := extractTraceContext(r)
		
		// Create request context fields
		requestFields := map[string]interface{}{
			"service":     config.ServiceName,
			"service_id":  config.ServiceID,
			"method":      r.Method,
			"path":        r.URL.Path,
			"remote_ip":   r.RemoteAddr,
			"user_agent":  r.Header.Get("User-Agent"),
		}
		
		// Add trace context if available
		if traceID, ok := ctx.Value("trace_id").(string); ok {
			requestFields["trace_id"] = traceID
		}
		if spanID, ok := ctx.Value("span_id").(string); ok {
			requestFields["span_id"] = spanID
		}
		
		// Add request fields to context
		ctx = context.WithValue(ctx, "request_fields", requestFields)
		
		// Log request start
		logger.InfoWithFields("Request received", requestFields)
		
		// Add trace headers to response before processing
		if traceID, ok := ctx.Value("trace_id").(string); ok {
			w.Header().Set(TraceIDHeader, traceID)
		}
		
		// Wrap response writer
		wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}
		
		// Process request
		next.ServeHTTP(wrapped, r.WithContext(ctx))
		
		// Calculate duration
		duration := time.Since(start)
		
		// Get request fields and add completion data
		completionFields := make(map[string]interface{})
		if requestFields, ok := ctx.Value("request_fields").(map[string]interface{}); ok {
			for k, v := range requestFields {
				completionFields[k] = v
			}
		}
		completionFields["status"] = wrapped.statusCode
		completionFields["duration_ms"] = duration.Milliseconds()
		completionFields["bytes"] = wrapped.bytesWritten
		
		// Log request completion
		logger.InfoWithFields("Request completed", completionFields)
	}
}

// Response writer wrapper
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(data []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(data)
	rw.bytesWritten += n
	return n, err
}

// Get request fields from context
func getRequestFields(ctx context.Context) map[string]interface{} {
	if fields, ok := ctx.Value("request_fields").(map[string]interface{}); ok {
		return fields
	}
	return map[string]interface{}{
		"service":    config.ServiceName,
		"service_id": config.ServiceID,
	}
}

// Log with context fields
func logWithContext(ctx context.Context, level string, message string, additionalFields map[string]interface{}) {
	fields := getRequestFields(ctx)
	for k, v := range additionalFields {
		fields[k] = v
	}
	
	switch level {
	case "debug":
		logger.DebugWithFields(message, fields)
	case "info":
		logger.InfoWithFields(message, fields)
	case "warn":
		logger.WarnWithFields(message, fields)
	case "error":
		logger.ErrorWithFields(message, fields)
	}
}

// Payment processing handler
func processPaymentHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Parse request
	var payment PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&payment); err != nil {
		logWithContext(ctx, "warn", "Invalid payment request", map[string]interface{}{
			"error": err.Error(),
		})
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	
	// Add payment context to request fields
	paymentFields := getRequestFields(ctx)
	paymentID := generatePaymentID()
	paymentFields["payment_id"] = paymentID
	paymentFields["amount"] = payment.Amount
	paymentFields["currency"] = payment.Currency
	paymentFields["merchant_id"] = payment.MerchantID
	ctx = context.WithValue(ctx, "request_fields", paymentFields)
	
	logger.InfoWithFields("Processing payment", paymentFields)
	
	// Simulate payment processing steps
	steps := []struct {
		name     string
		duration time.Duration
		failRate float64
	}{
		{"validate_merchant", 50 * time.Millisecond, 0.01},
		{"check_fraud", 100 * time.Millisecond, 0.05},
		{"authorize_payment", 200 * time.Millisecond, 0.02},
		{"capture_funds", 150 * time.Millisecond, 0.01},
	}
	
	for _, step := range steps {
		stepFields := getRequestFields(ctx)
		stepFields["step"] = step.name
		
		logger.DebugWithFields("Starting payment step", stepFields)
		
		// Simulate processing
		time.Sleep(step.duration)
		
		// Simulate failures
		if rand.Float64() < step.failRate {
			err := fmt.Errorf("%s failed", step.name)
			stepFields["error"] = err.Error()
			logger.ErrorWithFields("Payment step failed", stepFields)
			
			respondWithError(w, "Payment processing failed", http.StatusInternalServerError)
			return
		}
		
		logger.DebugWithFields("Payment step completed", stepFields)
	}
	
	// Call external service
	if err := callExternalService(ctx, payment); err != nil {
		logWithContext(ctx, "error", "External service call failed", map[string]interface{}{
			"error": err.Error(),
		})
		respondWithError(w, "External service error", http.StatusServiceUnavailable)
		return
	}
	
	// Success response
	response := PaymentResponse{
		PaymentID: payment.PaymentID,
		Status:    "completed",
		Timestamp: time.Now(),
	}
	
	logWithContext(ctx, "info", "Payment processed successfully", map[string]interface{}{
		"response_status": response.Status,
		"payment_id":      response.PaymentID,
	})
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Call external service with trace propagation
func callExternalService(ctx context.Context, payment PaymentRequest) error {
	// Create request
	reqBody, _ := json.Marshal(payment)
	req, err := http.NewRequest("POST", "http://external-service/api/process", 
		bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	
	// Propagate trace context
	propagateTraceContext(ctx, req)
	
	// Log outgoing request
	logWithContext(ctx, "debug", "Calling external service", map[string]interface{}{
		"external_service": "payment-processor",
		"endpoint":         req.URL.String(),
	})
	
	// Make request with timeout
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("external service request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("external service returned status %d", resp.StatusCode)
	}
	
	return nil
}

// Health check handler
func healthHandler(w http.ResponseWriter, r *http.Request) {
	// Simple health check - in production would check dependencies
	health := map[string]interface{}{
		"status":  "healthy",
		"service": config.ServiceName,
		"uptime":  time.Since(startTime).String(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// Types
type PaymentRequest struct {
	PaymentID  string  `json:"payment_id"`
	Amount     float64 `json:"amount"`
	Currency   string  `json:"currency"`
	MerchantID string  `json:"merchant_id"`
}

type PaymentResponse struct {
	PaymentID string    `json:"payment_id"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// Utility functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getHostname() string {
	hostname, _ := os.Hostname()
	return hostname
}

func generateServiceID() string {
	return fmt.Sprintf("%s-%d", getHostname(), os.Getpid())
}

func generateTraceID() string {
	return fmt.Sprintf("trace-%d-%d", time.Now().UnixNano(), rand.Int63())
}

func generateSpanID() string {
	return fmt.Sprintf("span-%d", rand.Int63())
}

func generatePaymentID() string {
	return fmt.Sprintf("pay-%d", time.Now().UnixNano())
}

func respondWithError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error":  message,
		"status": http.StatusText(status),
	})
}

var startTime = time.Now()

func main() {
	// Initialize service
	if err := initService(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize service: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		logger.Info("Service shutting down")
		logger.FlushAll()
		logger.Close()
	}()
	
	// Set up routes
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/api/payment", serviceMiddleware(processPaymentHandler))
	
	// Metrics endpoint
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics := logger.GetMetrics()
		
		// Calculate total messages from all levels
		totalMessages := metrics.MessagesLogged
		
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
	})
	
	// Start server
	addr := ":8080"
	logger.InfoWithFields("Service starting", map[string]interface{}{
		"addr":        addr,
		"service":     config.ServiceName,
		"service_id":  config.ServiceID,
		"environment": config.Environment,
	})
	
	if err := http.ListenAndServe(addr, nil); err != nil {
		logger.ErrorWithFields("Failed to start server", map[string]interface{}{
			"error": err.Error(),
			"addr":  addr,
		})
		os.Exit(1)
	}
}