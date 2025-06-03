package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/wayneeseguin/omni"
)

// RequestIDKey is the context key for request ID
type RequestIDKey struct{}

func main() {
	logger, err := omni.New("context-aware.log")
	if err != nil {
		log.Fatal(err)
	}
	defer logger.Close()

	// Set level to TRACE to see detailed context tracking
	logger.SetLevel(omni.LevelTrace)

	// Simulate handling multiple requests with context tracking
	for i := 0; i < 5; i++ {
		requestID := fmt.Sprintf("req-%d", i+1)
		handleRequest(logger, requestID)
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("Check context-aware.log for the logged messages")
}

func handleRequest(logger *omni.Omni, requestID string) {
	// Create context with request ID
	ctx := context.WithValue(context.Background(), RequestIDKey{}, requestID)

	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Extract request ID from context for logging
	reqID := getRequestID(ctx)

	logger.TraceWithFields("Request handler started", map[string]interface{}{
		"request_id": reqID,
		"function":   "handleRequest",
	})

	logger.InfoWithFields("Request started", map[string]interface{}{
		"request_id": reqID,
		"method":     "GET",
		"path":       "/api/users",
	})

	// Simulate processing steps with detailed tracing
	logger.TraceWithFields("Starting user fetch", map[string]interface{}{
		"request_id": reqID,
		"step":       "fetch_user",
	})

	if err := fetchUser(ctx, logger); err != nil {
		logger.ErrorWithFields("Failed to fetch user", map[string]interface{}{
			"error":      err.Error(),
			"request_id": reqID,
		})
		return
	}

	logger.TraceWithFields("Starting permission validation", map[string]interface{}{
		"request_id": reqID,
		"step":       "validate_permissions",
	})

	if err := validatePermissions(ctx, logger); err != nil {
		logger.WarnWithFields("Permission validation failed", map[string]interface{}{
			"error":      err.Error(),
			"request_id": reqID,
		})
	}

	// Simulate context cancellation for demonstration
	if requestID == "req-3" {
		logger.TraceWithFields("Simulating request cancellation", map[string]interface{}{
			"request_id": reqID,
		})
		cancel()
		select {
		case <-ctx.Done():
			logger.ErrorWithFields("Request cancelled", map[string]interface{}{
				"request_id": reqID,
				"reason":     ctx.Err().Error(),
			})
			return
		default:
		}
	}

	logger.InfoWithFields("Request completed successfully", map[string]interface{}{
		"request_id":  reqID,
		"duration_ms": 45,
	})

	logger.TraceWithFields("Request handler completed", map[string]interface{}{
		"request_id": reqID,
		"function":   "handleRequest",
	})
}

func fetchUser(ctx context.Context, logger *omni.Omni) error {
	reqID := getRequestID(ctx)

	logger.TraceWithFields("Database query starting", map[string]interface{}{
		"request_id": reqID,
		"operation":  "fetchUser",
		"table":      "users",
	})

	logger.DebugWithFields("Fetching user from database", map[string]interface{}{
		"request_id": reqID,
	})

	// Simulate database query
	select {
	case <-time.After(20 * time.Millisecond):
		logger.DebugWithFields("User fetched successfully", map[string]interface{}{
			"user_id":    12345,
			"request_id": reqID,
		})

		logger.TraceWithFields("Database query completed", map[string]interface{}{
			"request_id": reqID,
			"operation":  "fetchUser",
			"result":     "success",
		})
		return nil
	case <-ctx.Done():
		logger.TraceWithFields("Database query cancelled", map[string]interface{}{
			"request_id": reqID,
			"operation":  "fetchUser",
			"reason":     ctx.Err().Error(),
		})
		return ctx.Err()
	}
}

func validatePermissions(ctx context.Context, logger *omni.Omni) error {
	reqID := getRequestID(ctx)

	logger.TraceWithFields("Permission validation starting", map[string]interface{}{
		"request_id": reqID,
		"operation":  "validatePermissions",
	})

	logger.DebugWithFields("Validating user permissions", map[string]interface{}{
		"request_id": reqID,
	})

	// Simulate permission check
	select {
	case <-time.After(10 * time.Millisecond):
		logger.DebugWithFields("Permissions validated", map[string]interface{}{
			"role":       "admin",
			"request_id": reqID,
		})

		logger.TraceWithFields("Permission validation completed", map[string]interface{}{
			"request_id": reqID,
			"operation":  "validatePermissions",
			"result":     "success",
		})
		return nil
	case <-ctx.Done():
		logger.TraceWithFields("Permission validation cancelled", map[string]interface{}{
			"request_id": reqID,
			"operation":  "validatePermissions",
			"reason":     ctx.Err().Error(),
		})
		return ctx.Err()
	}
}

func getRequestID(ctx context.Context) string {
	if reqID, ok := ctx.Value(RequestIDKey{}).(string); ok {
		return reqID
	}
	return "unknown"
}
