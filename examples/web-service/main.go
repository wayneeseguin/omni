package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wayneeseguin/omni/pkg/omni"
)

// Global logger instance
var logger *omni.Omni

// Middleware for request logging
func loggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Generate request ID
		requestID := fmt.Sprintf("req-%d", time.Now().UnixNano())
		
		// Add request ID to context
		ctx := context.WithValue(r.Context(), "request_id", requestID)
		
		// Log request start
		logger.InfoWithFields("Request started", map[string]interface{}{
			"request_id": requestID,
			"method":     r.Method,
			"path":       r.URL.Path,
			"remote_ip":  r.RemoteAddr,
			"user_agent": r.Header.Get("User-Agent"),
		})
		
		// Wrap response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}
		
		// Call next handler
		next.ServeHTTP(wrapped, r.WithContext(ctx))
		
		// Log request completion
		duration := time.Since(start)
		logger.InfoWithFields("Request completed", map[string]interface{}{
			"request_id":  requestID,
			"method":      r.Method,
			"path":        r.URL.Path,
			"status":      wrapped.statusCode,
			"duration_ms": duration.Milliseconds(),
			"bytes":       wrapped.bytesWritten,
		})
		
		// Log slow requests as warnings
		if duration > 500*time.Millisecond {
			logger.WarnWithFields("Slow request detected", map[string]interface{}{
				"request_id":  requestID,
				"path":        r.URL.Path,
				"duration_ms": duration.Milliseconds(),
			})
		}
	}
}

// Response writer wrapper to capture status code
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

// Get request ID from context
func getRequestID(r *http.Request) string {
	if id, ok := r.Context().Value("request_id").(string); ok {
		return id
	}
	return "unknown"
}

// API handlers
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{ //nolint:gosec
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func userHandler(w http.ResponseWriter, r *http.Request) {
	requestID := getRequestID(r)
	
	// Simulate user lookup
	userID := r.URL.Query().Get("id")
	if userID == "" {
		logger.WarnWithFields("Missing user ID parameter", map[string]interface{}{
			"request_id": requestID,
			"path":       r.URL.Path,
		})
		http.Error(w, "User ID required", http.StatusBadRequest)
		return
	}
	
	// Log user access
	logger.InfoWithFields("User data accessed", map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"endpoint":   "/api/user",
	})
	
	// Simulate some processing
	time.Sleep(100 * time.Millisecond)
	
	// Return user data
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:gosec
		"id":    userID,
		"name":  "John Doe",
		"email": "john@example.com",
	})
}

func errorHandler(w http.ResponseWriter, r *http.Request) {
	requestID := getRequestID(r)
	
	// Simulate an error
	err := fmt.Errorf("database connection failed")
	
	// Log error with context
	logger.ErrorWithFields("Failed to process request", map[string]interface{}{
		"request_id": requestID,
		"error":      err.Error(),
		"component":  "database",
		"endpoint":   "/api/error",
	})
	
	http.Error(w, "Internal server error", http.StatusInternalServerError)
}

func main() {
	// Initialize logger with production settings
	var err error
	logger, err = omni.NewWithOptions(
		omni.WithPath("/tmp/webservice.log"), // Use /tmp for demo
		omni.WithLevel(omni.LevelInfo),
		omni.WithJSON(),                    // JSON for log aggregation
		omni.WithRotation(50*1024*1024, 5), // 50MB files, keep 5
		omni.WithGzipCompression(),         // Compress rotated logs
		omni.WithChannelSize(5000),         // Larger buffer for web service
	)
	
	if err != nil {
		// Fallback to stderr
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	
	// Ensure logger cleanup
	defer func() {
		logger.Info("Shutting down web service")
		_ = logger.Close() //nolint:gosec
	}()
	
	// Add additional log destination for errors
	err = logger.AddDestination("/tmp/webservice-errors.log")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to add error destination: %v\n", err)
	}
	
	// Add a filter for sampling health checks
	_ = logger.AddFilter(func(level int, message string, fields map[string]interface{}) bool { //nolint:gosec
		// Sample health check requests (allow only 10%)
		if path, ok := fields["path"].(string); ok && path == "/health" {
			return time.Now().UnixNano()%10 == 0
		}
		return true // Allow all other messages
	})
	
	// Set up HTTP routes
	http.HandleFunc("/health", loggingMiddleware(healthHandler))
	http.HandleFunc("/api/user", loggingMiddleware(userHandler))
	http.HandleFunc("/api/error", loggingMiddleware(errorHandler))
	
	// Set up metrics endpoint
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
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
	})
	
	// Start server
	server := &http.Server{
		Addr:         ":8080",
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	
	// Handle graceful shutdown
	done := make(chan bool)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	
	go func() {
		<-quit
		logger.Info("Server is shutting down...")
		
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		
		if err := server.Shutdown(ctx); err != nil {
			logger.ErrorWithFields("Could not gracefully shutdown the server", map[string]interface{}{
				"error": err.Error(),
			})
		}
		close(done)
	}()
	
	logger.InfoWithFields("Server starting", map[string]interface{}{
		"addr": server.Addr,
		"pid":  os.Getpid(),
	})
	
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.ErrorWithFields("Could not start server", map[string]interface{}{
			"error": err.Error(),
			"addr":  server.Addr,
		})
		os.Exit(1)
	}
	
	<-done
	logger.Info("Server stopped")
}