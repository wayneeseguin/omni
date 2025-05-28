package main

import (
	"fmt"
	"log"

	"github.com/wayneeseguin/flexlog"
)

func main() {
	// Create a new logger with file destination
	logger, err := flexlog.New("app.log")
	if err != nil {
		log.Fatal(err)
	}
	defer logger.CloseAll()

	// Set level to TRACE to see all messages including TRACE level
	logger.SetLevel(flexlog.LevelTrace)

	// Basic logging at all levels
	logger.Trace("This is a trace message - very detailed diagnostic info")
	logger.Debug("This is a debug message - detailed diagnostic info")
	logger.Info("Application started successfully")
	logger.Warn("This is a warning")
	logger.Error("This is an error message")

	// Formatted logging
	username := "john_doe"
	logger.Tracef("Entering authentication flow for user: %s", username)
	logger.Debugf("Processing login for user: %s", username)
	logger.Infof("User %s logged in", username)
	
	// Structured logging with fields
	logger.TraceWithFields("Function entry", map[string]interface{}{
		"function": "processLogin",
		"user":     username,
		"step":     "validation",
	})

	logger.DebugWithFields("Cache lookup", map[string]interface{}{
		"user": username,
		"hit":  true,
		"ttl":  300,
	})

	logger.InfoWithFields("User action", map[string]interface{}{
		"user":      username,
		"action":    "login",
		"ip":        "192.168.1.1",
		"timestamp": "2024-01-20T10:30:00Z",
	})

	// Log with error
	err = fmt.Errorf("database connection failed")
	logger.ErrorWithFields("Failed to connect to database", map[string]interface{}{
		"error":       err.Error(),
		"retry_count": 3,
		"max_retries": 5,
	})

	fmt.Println("Check app.log for the logged messages")
}