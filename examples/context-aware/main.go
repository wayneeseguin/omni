package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/wayneeseguin/flexlog"
)

// RequestIDKey is the context key for request ID
type RequestIDKey struct{}

func main() {
	logger, err := flexlog.NewFlexLog()
	if err != nil {
		log.Fatal(err)
	}
	defer logger.Close()

	// Add destination
	err = logger.AddDestination("file", flexlog.DestinationConfig{
		Backend:  flexlog.BackendFile,
		FilePath: "context-aware.log",
		Format:   flexlog.FormatJSON,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Simulate handling multiple requests
	for i := 0; i < 5; i++ {
		requestID := fmt.Sprintf("req-%d", i+1)
		handleRequest(logger, requestID)
		time.Sleep(100 * time.Millisecond)
	}
}

func handleRequest(logger *flexlog.FlexLog, requestID string) {
	// Create context with request ID
	ctx := context.WithValue(context.Background(), RequestIDKey{}, requestID)
	
	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	logger.InfoContext(ctx, "Request started",
		"request_id", requestID,
		"method", "GET",
		"path", "/api/users",
	)

	// Simulate processing steps
	if err := fetchUser(ctx, logger); err != nil {
		logger.ErrorContext(ctx, "Failed to fetch user",
			"error", err,
			"request_id", requestID,
		)
		return
	}

	if err := validatePermissions(ctx, logger); err != nil {
		logger.WarnContext(ctx, "Permission validation failed",
			"error", err,
			"request_id", requestID,
		)
	}

	// Simulate context cancellation
	if requestID == "req-3" {
		cancel()
		select {
		case <-ctx.Done():
			logger.ErrorContext(ctx, "Request cancelled",
				"request_id", requestID,
				"reason", ctx.Err(),
			)
			return
		default:
		}
	}

	logger.InfoContext(ctx, "Request completed successfully",
		"request_id", requestID,
		"duration_ms", 45,
	)
}

func fetchUser(ctx context.Context, logger *flexlog.FlexLog) error {
	logger.DebugContext(ctx, "Fetching user from database")
	
	// Simulate database query
	select {
	case <-time.After(20 * time.Millisecond):
		logger.DebugContext(ctx, "User fetched successfully", "user_id", 12345)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func validatePermissions(ctx context.Context, logger *flexlog.FlexLog) error {
	logger.DebugContext(ctx, "Validating user permissions")
	
	// Simulate permission check
	select {
	case <-time.After(10 * time.Millisecond):
		logger.DebugContext(ctx, "Permissions validated", "role", "admin")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}