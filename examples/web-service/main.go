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

	"github.com/wayneeseguin/flexlog"
)

// Global logger instance
var logger *flexlog.FlexLog

// Middleware for request logging
func loggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Generate request ID
		requestID := fmt.Sprintf("req-%d", time.Now().UnixNano())
		
		// Create request-scoped logger
		reqLogger := logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"method":     r.Method,
			"path":       r.URL.Path,
			"remote_ip":  r.RemoteAddr,
			"user_agent": r.Header.Get("User-Agent"),
		})
		
		// Add request ID to context
		ctx := context.WithValue(r.Context(), "request_id", requestID)
		ctx = context.WithValue(ctx, "logger", reqLogger)
		
		// Log request start
		reqLogger.Info("Request started")
		
		// Wrap response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}
		
		// Call next handler
		next.ServeHTTP(wrapped, r.WithContext(ctx))
		
		// Log request completion
		duration := time.Since(start)
		reqLogger.WithFields(map[string]interface{}{
			"status":      wrapped.statusCode,
			"duration_ms": duration.Milliseconds(),
			"bytes":       wrapped.bytesWritten,
		}).Info("Request completed")
		
		// Log slow requests as warnings
		if duration > 500*time.Millisecond {
			reqLogger.WithField("duration_ms", duration.Milliseconds()).
				Warn("Slow request detected")
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

// Get logger from context
func getLogger(r *http.Request) *flexlog.FlexLog {
	if l, ok := r.Context().Value("logger").(*flexlog.FlexLog); ok {
		return l
	}
	return logger
}

// API handlers
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func userHandler(w http.ResponseWriter, r *http.Request) {
	log := getLogger(r)
	
	// Simulate user lookup
	userID := r.URL.Query().Get("id")
	if userID == "" {
		log.Warn("Missing user ID parameter")
		http.Error(w, "User ID required", http.StatusBadRequest)
		return
	}
	
	// Log user access
	log.WithField("user_id", userID).Info("User data accessed")
	
	// Simulate some processing
	time.Sleep(100 * time.Millisecond)
	
	// Return user data
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":    userID,
		"name":  "John Doe",
		"email": "john@example.com",
	})
}

func errorHandler(w http.ResponseWriter, r *http.Request) {
	log := getLogger(r)
	
	// Simulate an error
	err := fmt.Errorf("database connection failed")
	
	// Log error with stack trace
	log.WithError(err).
		WithField("component", "database").
		Error("Failed to process request")
	
	http.Error(w, "Internal server error", http.StatusInternalServerError)
}

func main() {
	// Initialize logger with production settings
	var err error
	logger, err = flexlog.NewBuilder().
		WithPath("/var/log/webservice.log").
		WithLevel(flexlog.LevelInfo).
		WithJSON().                         // JSON for log aggregation
		WithRotation(100*1024*1024, 10).   // 100MB files, keep 10
		WithGzipCompression().             // Compress rotated logs
		WithChannelSize(5000).             // Larger buffer for web service
		Build()
	
	if err != nil {
		// Fallback to stderr
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	
	// Ensure logger cleanup
	defer func() {
		logger.Info("Shutting down web service")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := logger.Shutdown(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error during logger shutdown: %v\n", err)
		}
	}()
	
	// Set up error handling
	logger.SetErrorHandler(func(err flexlog.LogError) {
		// Could send to monitoring system
		fmt.Fprintf(os.Stderr, "[%s] Logging error: %v\n", err.Level, err.Err)
	})
	
	// Add additional log destination for errors
	logger.AddDestination("/var/log/webservice-errors.log")
	
	// Configure sampling for high-volume endpoints
	logger.EnablePatternBasedSampling([]flexlog.PatternSamplingRule{
		{
			Pattern: "health check",
			Rate:    0.01, // Sample 1% of health checks
		},
	})
	
	// Set up HTTP routes
	http.HandleFunc("/health", loggingMiddleware(healthHandler))
	http.HandleFunc("/api/user", loggingMiddleware(userHandler))
	http.HandleFunc("/api/error", loggingMiddleware(errorHandler))
	
	// Set up metrics endpoint
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics := logger.GetMetrics()
		
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "# HELP log_messages_total Total log messages\n")
		fmt.Fprintf(w, "log_messages_total %d\n", metrics.TotalMessages)
		
		fmt.Fprintf(w, "# HELP log_errors_total Total logging errors\n")
		fmt.Fprintf(w, "log_errors_total %d\n", metrics.ErrorCount)
		
		fmt.Fprintf(w, "# HELP log_channel_usage Channel buffer usage ratio\n")
		fmt.Fprintf(w, "log_channel_usage %f\n", metrics.ChannelUsage)
		
		// Per-level metrics
		for level, count := range metrics.MessageCounts {
			levelName := flexlog.LevelName(level)
			fmt.Fprintf(w, "# HELP log_messages_%s_total Total %s messages\n", 
				levelName, levelName)
			fmt.Fprintf(w, "log_messages_%s_total %d\n", levelName, count)
		}
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
			logger.WithError(err).Error("Could not gracefully shutdown the server")
		}
		close(done)
	}()
	
	logger.WithField("addr", server.Addr).Info("Server starting")
	
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.WithError(err).Fatal("Could not listen on", server.Addr)
	}
	
	<-done
	logger.Info("Server stopped")
}