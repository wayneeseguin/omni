package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/wayneeseguin/omni/pkg/omni"
)

func main() {
	// Example 1: Basic redaction with common patterns
	fmt.Println("=== Example 1: Basic Redaction ===")
	demonstrateBasicRedaction()

	// Example 2: Custom redaction patterns
	fmt.Println("\n=== Example 2: Custom Redaction Patterns ===")
	demonstrateCustomRedaction()

	// Example 3: Multiple pattern redaction
	fmt.Println("\n=== Example 3: Multiple Pattern Redaction ===")
	demonstrateMultiplePatterns()

	// Example 4: Structured logging with redaction
	fmt.Println("\n=== Example 4: Structured Logging with Redaction ===")
	demonstrateStructuredRedaction()

	// Clean up
	cleanupFiles()
}

func demonstrateBasicRedaction() {
	// Create logger with basic redaction patterns
	logger, err := omni.NewWithOptions(
		omni.WithPath("/tmp/redaction_basic.log"),
		omni.WithLevel(omni.LevelInfo),
		omni.WithJSON(),
		omni.WithRedaction([]string{
			`password=\w+`,
			`token=[\w-]+`,
		}, "[REDACTED]"),
	)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Log messages with sensitive data
	logger.Info("User login attempt with password=secret123 and token=abc-def-456")
	logger.Info("Configuration: password=mypassword, timeout=30")
	logger.Info("API call with token=xyz-789-token successful")

	fmt.Println("✓ Basic redaction completed - check /tmp/redaction_basic.log")
}

func demonstrateCustomRedaction() {
	// Create logger with custom patterns
	patterns := []string{
		`credit_card=\d{4}-\d{4}-\d{4}-\d{4}`,
		`ssn=\d{3}-\d{2}-\d{4}`,
		`email=[\w.-]+@[\w.-]+\.\w+`,
	}

	logger, err := omni.NewWithOptions(
		omni.WithPath("/tmp/redaction_custom.log"),
		omni.WithLevel(omni.LevelInfo),
		omni.WithJSON(),
		omni.WithRedaction(patterns, "[SENSITIVE]"),
	)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Log messages with sensitive personal data
	logger.Info("Processing payment for credit_card=1234-5678-9012-3456")
	logger.Info("User registration: email=john.doe@example.com, ssn=123-45-6789")
	logger.Info("Transaction completed for credit_card=9876-5432-1098-7654")

	fmt.Println("✓ Custom redaction completed - check /tmp/redaction_custom.log")
}

func demonstrateMultiplePatterns() {
	// Create logger with multiple pattern types
	patterns := []string{
		`"password":\s*"[^"]*"`, // JSON password fields
		`"apiKey":\s*"[^"]*"`,   // JSON API key fields
		`Bearer\s+[\w.-]+`,      // Authorization headers
		`key=[A-Za-z0-9]+`,      // URL parameters
	}

	logger, err := omni.NewWithOptions(
		omni.WithPath("/tmp/redaction_multiple.log"),
		omni.WithLevel(omni.LevelDebug),
		omni.WithText(),
		omni.WithRedaction(patterns, "***"),
	)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Log various types of sensitive data
	logger.Info(`API request body: {"username": "admin", "password": "secret123", "apiKey": "sk-proj-abc123"}`)
	logger.Debug("Authorization header: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9")
	logger.Info("Request URL: https://api.example.com/users?key=secretkey123&page=1")

	fmt.Println("✓ Multiple patterns redaction completed - check /tmp/redaction_multiple.log")
}

func demonstrateStructuredRedaction() {
	// Create logger with structured logging and redaction
	patterns := []string{
		`"credit_card":\s*"[^"]*"`,
		`"social_security":\s*"[^"]*"`,
		`"access_token":\s*"[^"]*"`,
	}

	logger, err := omni.NewWithOptions(
		omni.WithPath("/tmp/redaction_structured.log"),
		omni.WithLevel(omni.LevelInfo),
		omni.WithJSON(),
		omni.WithRedaction(patterns, "[PROTECTED]"),
	)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Log structured data containing sensitive information
	logger.InfoWithFields("User registration event", map[string]interface{}{
		"user_id":         "12345",
		"username":        "john_doe",
		"email":           "john@example.com",
		"credit_card":     "4532-1234-5678-9012",
		"social_security": "123-45-6789",
		"timestamp":       time.Now().Unix(),
	})

	logger.InfoWithFields("API authentication", map[string]interface{}{
		"user_id":      "67890",
		"access_token": "sk-live-abc123def456ghi789",
		"expires_at":   time.Now().Add(time.Hour).Unix(),
		"scope":        "read:users write:data",
	})

	// Test redaction in error messages
	logger.ErrorWithFields("Payment processing failed", map[string]interface{}{
		"error":       "Invalid credit card number",
		"credit_card": "1111-2222-3333-4444",
		"user_id":     "99999",
		"retry_count": 3,
	})

	fmt.Println("✓ Structured redaction completed - check /tmp/redaction_structured.log")
}

func cleanupFiles() {
	files := []string{
		"/tmp/redaction_basic.log",
		"/tmp/redaction_custom.log",
		"/tmp/redaction_multiple.log",
		"/tmp/redaction_structured.log",
	}

	for _, file := range files {
		if _, err := os.Stat(file); err == nil {
			_ = os.Remove(file) //nolint:gosec
		}
	}
}
