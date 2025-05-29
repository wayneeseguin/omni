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

	"github.com/wayneeseguin/flexlog"
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
	logger *flexlog.FlexLog
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
		LogLevel:    flexlog.LevelInfo,
		Environment: getEnv("ENVIRONMENT", "development"),
	}
	
	if config.Environment == "development" {
		config.LogLevel = flexlog.LevelDebug
	}
	
	// Create logger with service context
	builder := flexlog.NewBuilder().
		WithPath(fmt.Sprintf("/var/log/%s.log", config.ServiceName)).
		WithLevel(config.LogLevel).
		WithJSON() // Always use JSON for microservices
	
	// Production settings
	if config.Environment == "production" {
		builder = builder.
			WithRotation(200*1024*1024, 20).  // 200MB files, keep 20
			WithGzipCompression().
			WithChannelSize(10000)
		
		// Add centralized logging destination
		if syslogAddr := getEnv("SYSLOG_ADDR", ""); syslogAddr != "" {
			builder = builder.WithDestination(fmt.Sprintf("syslog://%s", syslogAddr))
		}
	}
	
	var err error
	logger, err = builder.Build()
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}
	
	// Add global context fields
	logger = logger.WithFields(map[string]interface{}{
		"service":     config.ServiceName,
		"service_id":  config.ServiceID,
		"environment": config.Environment,
		"version":     "1.0.0",
		"host":        getHostname(),
	}).(*flexlog.FlexLog)
	
	// Configure sampling for production
	if config.Environment == "production" {
		logger.EnableAdaptiveSampling(flexlog.AdaptiveSamplingConfig{
			TargetRate:    1000,              // Target 1000 logs/second
			WindowSize:    time.Minute,
			MinSampleRate: 0.001,             // Never less than 0.1%
			MaxSampleRate: 1.0,               // Never more than 100%
			LevelExemptions: map[int]bool{
				flexlog.LevelWarn:  true,     // Always log warnings
				flexlog.LevelError: true,     // Always log errors
			},
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
	ctx = flexlog.WithTraceID(ctx, traceID)
	
	// Extract span information
	parentSpanID := r.Header.Get(ParentSpanIDHeader)
	spanID := generateSpanID()
	
	ctx = context.WithValue(ctx, "span_id", spanID)
	ctx = context.WithValue(ctx, "parent_span_id", parentSpanID)
	
	return ctx
}

// Propagate trace context to outgoing requests
func propagateTraceContext(ctx context.Context, req *http.Request) {
	if traceID := flexlog.GetTraceID(ctx); traceID != "" {
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
		
		// Create request logger with context
		reqLogger := flexlog.NewContextLogger(logger, ctx).
			WithFields(map[string]interface{}{
				"method":     r.Method,
				"path":       r.URL.Path,
				"remote_ip":  r.RemoteAddr,
				"user_agent": r.Header.Get("User-Agent"),
			})
		
		// Add logger to context
		ctx = context.WithValue(ctx, "logger", reqLogger)
		
		// Log request start
		reqLogger.Info("Request received")
		
		// Wrap response writer
		wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}
		
		// Process request
		next.ServeHTTP(wrapped, r.WithContext(ctx))
		
		// Calculate duration
		duration := time.Since(start)
		
		// Log request completion
		reqLogger.WithFields(map[string]interface{}{
			"status":      wrapped.statusCode,
			"duration_ms": duration.Milliseconds(),
			"bytes":       wrapped.bytesWritten,
		}).Info("Request completed")
		
		// Add trace headers to response
		w.Header().Set(TraceIDHeader, flexlog.GetTraceID(ctx))
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

// Get logger from context
func getLogger(ctx context.Context) flexlog.Logger {
	if l, ok := ctx.Value("logger").(flexlog.Logger); ok {
		return l
	}
	return logger
}

// Payment processing handler
func processPaymentHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := getLogger(ctx)
	
	// Parse request
	var payment PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&payment); err != nil {
		log.WithError(err).Warn("Invalid payment request")
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	
	// Add payment context
	log = log.WithFields(map[string]interface{}{
		"payment_id":   generatePaymentID(),
		"amount":       payment.Amount,
		"currency":     payment.Currency,
		"merchant_id":  payment.MerchantID,
	})
	
	log.Info("Processing payment")
	
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
		stepLog := log.WithField("step", step.name)
		stepLog.Debug("Starting payment step")
		
		// Simulate processing
		time.Sleep(step.duration)
		
		// Simulate failures
		if rand.Float64() < step.failRate {
			err := fmt.Errorf("%s failed", step.name)
			stepLog.WithError(err).Error("Payment step failed")
			
			respondWithError(w, "Payment processing failed", http.StatusInternalServerError)
			return
		}
		
		stepLog.Debug("Payment step completed")
	}
	
	// Call external service
	if err := callExternalService(ctx, payment); err != nil {
		log.WithError(err).Error("External service call failed")
		respondWithError(w, "External service error", http.StatusServiceUnavailable)
		return
	}
	
	// Success response
	response := PaymentResponse{
		PaymentID: payment.PaymentID,
		Status:    "completed",
		Timestamp: time.Now(),
	}
	
	log.WithField("response_status", response.Status).Info("Payment processed successfully")
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Call external service with trace propagation
func callExternalService(ctx context.Context, payment PaymentRequest) error {
	log := getLogger(ctx)
	
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
	log.WithFields(map[string]interface{}{
		"external_service": "payment-processor",
		"endpoint":         req.URL.String(),
	}).Debug("Calling external service")
	
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
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		logger.Shutdown(ctx)
	}()
	
	// Set up routes
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/api/payment", serviceMiddleware(processPaymentHandler))
	
	// Metrics endpoint
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics := logger.GetMetrics()
		
		// Export Prometheus-style metrics
		fmt.Fprintf(w, "# HELP service_log_messages_total Total log messages\n")
		fmt.Fprintf(w, "# TYPE service_log_messages_total counter\n")
		fmt.Fprintf(w, "service_log_messages_total{service=\"%s\"} %d\n", 
			config.ServiceName, metrics.TotalMessages)
		
		fmt.Fprintf(w, "# HELP service_log_errors_total Total logging errors\n")
		fmt.Fprintf(w, "# TYPE service_log_errors_total counter\n")
		fmt.Fprintf(w, "service_log_errors_total{service=\"%s\"} %d\n", 
			config.ServiceName, metrics.ErrorCount)
		
		// Add custom business metrics
		fmt.Fprintf(w, "# HELP service_uptime_seconds Service uptime\n")
		fmt.Fprintf(w, "# TYPE service_uptime_seconds gauge\n")
		fmt.Fprintf(w, "service_uptime_seconds{service=\"%s\"} %f\n", 
			config.ServiceName, time.Since(startTime).Seconds())
	})
	
	// Start server
	addr := ":8080"
	logger.WithField("addr", addr).Info("Service starting")
	
	if err := http.ListenAndServe(addr, nil); err != nil {
		logger.WithError(err).Fatal("Failed to start server")
	}
}