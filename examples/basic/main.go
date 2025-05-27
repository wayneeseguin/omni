package main

import (
	"fmt"
	"log"

	"github.com/wayneeseguin/flexlog"
)

func main() {
	// Create a new logger
	logger, err := flexlog.NewFlexLog()
	if err != nil {
		log.Fatal(err)
	}
	defer logger.Close()

	// Add a file destination
	err = logger.AddDestination("file", flexlog.DestinationConfig{
		Backend:  flexlog.BackendFile,
		FilePath: "app.log",
		Format:   flexlog.FormatJSON,
		MinLevel: flexlog.DEBUG,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Basic logging at different levels
	logger.Debug("This is a debug message")
	logger.Info("Application started successfully")
	logger.Warn("This is a warning")
	logger.Error("This is an error message")

	// Formatted logging
	username := "john_doe"
	logger.Infof("User %s logged in", username)
	
	// Structured logging with fields
	logger.Info("User action",
		"user", username,
		"action", "login",
		"ip", "192.168.1.1",
		"timestamp", "2024-01-20T10:30:00Z",
	)

	// Log with error
	err = fmt.Errorf("database connection failed")
	logger.Error("Failed to connect to database",
		"error", err,
		"retry_count", 3,
		"max_retries", 5,
	)

	fmt.Println("Check app.log for the logged messages")
}