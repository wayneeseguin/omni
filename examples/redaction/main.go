package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/wayneeseguin/omni"
)

func main() {
	// Create logger with redaction enabled
	logger, err := omni.New("redacted.log")
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Close()

	// Example 1: Built-in redaction patterns
	fmt.Println("=== Example 1: Built-in Redaction Patterns ===")
	demonstrateBuiltInRedaction(logger)

	// Example 2: Custom redaction patterns
	fmt.Println("\n=== Example 2: Custom Redaction Patterns ===")
	demonstrateCustomRedaction(logger)

	// Example 3: HTTP request/response logging
	fmt.Println("\n=== Example 3: HTTP Request/Response Logging ===")
	demonstrateHTTPRedaction(logger)

	// Example 4: Structured logging with sensitive data
	fmt.Println("\n=== Example 4: Structured Logging with Redaction ===")
	demonstrateStructuredRedaction(logger)

	// Example 5: Complex nested data redaction
	fmt.Println("\n=== Example 5: Complex Nested Data Redaction ===")
	demonstrateNestedRedaction(logger)

	// Example 6: Performance optimizations and level-based redaction
	fmt.Println("\n=== Example 6: Performance Optimizations ===")
	demonstratePerformanceOptimizations(logger)

	// Example 7: Field path redaction with wildcards
	fmt.Println("\n=== Example 7: Field Path Redaction ===")
	demonstrateFieldPathRedaction(logger)

	// Wait for messages to be processed
	time.Sleep(100 * time.Millisecond)
	
	fmt.Println("\nCheck 'redacted.log' to see the redacted output")
}

func demonstrateBuiltInRedaction(logger *omni.Omni) {
	// These sensitive fields will be automatically redacted
	logger.Info("User login attempt", map[string]interface{}{
		"username": "john.doe",
		"password": "super_secret_123",  // Will be redacted
		"api_key": "sk-1234567890abcdef", // Will be redacted
		"auth_token": "Bearer eyJhbGciOiJIUzI1NiIs...", // Will be redacted
		"session_id": "sess_123456",
	})

	// Log with various sensitive patterns
	logger.Info("Configuration loaded", map[string]interface{}{
		"database": map[string]interface{}{
			"host": "db.example.com",
			"port": 5432,
			"password": "db_password_123", // Will be redacted
			"connection_string": "postgresql://user:secret@localhost/db", // Consider adding pattern for this
		},
		"api": map[string]interface{}{
			"endpoint": "https://api.example.com",
			"secret_key": "api_secret_key_456", // Will be redacted
			"public_key": "pk_test_123",
		},
	})
}

func demonstrateCustomRedaction(logger *omni.Omni) {
	// Add custom redaction patterns
	customPatterns := []string{
		`\b\d{3}-\d{2}-\d{4}\b`,  // SSN pattern
		`\b\d{4}[-\s]?\d{4}[-\s]?\d{4}[-\s]?\d{4}\b`, // Credit card pattern
		`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`, // Email pattern
		`\b\d{3}[-.]?\d{3}[-.]?\d{4}\b`, // Phone number pattern
	}

	// Apply custom patterns
	logger.SetRedaction(customPatterns, "[REDACTED]")

	// Log messages with custom sensitive data
	logger.Info("Customer record", map[string]interface{}{
		"name": "Jane Smith",
		"email": "jane.smith@example.com", // Will be redacted by custom pattern
		"ssn": "123-45-6789", // Will be redacted by custom pattern
		"phone": "555-123-4567", // Will be redacted by custom pattern
		"credit_card": "4111 1111 1111 1111", // Will be redacted by custom pattern
		"account_id": "ACC-12345",
	})
}

func demonstrateHTTPRedaction(logger *omni.Omni) {
	// Create a mock HTTP request
	reqBody := `{
		"username": "alice",
		"password": "alice_password_789",
		"email": "alice@example.com",
		"profile": {
			"ssn": "987-65-4321",
			"api_token": "token_xyz789"
		}
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...")
	req.Header.Set("X-API-Key", "api_key_12345")
	req.Header.Set("User-Agent", "MyApp/1.0")

	// Log the request (sensitive headers and body fields will be redacted)
	logger.LogRequest(req.Method, req.URL.Path, req.Header, reqBody)

	// Create a mock HTTP response
	respBody := `{
		"success": true,
		"user_id": "usr_123",
		"auth_token": "new_auth_token_456",
		"refresh_token": "refresh_token_789",
		"session": {
			"id": "sess_abc123",
			"secret": "session_secret_key"
		}
	}`

	respHeaders := map[string][]string{
		"Content-Type": []string{"application/json"},
		"X-Auth-Token": []string{"response_token_123"},
		"Set-Cookie": []string{"session=abc123; HttpOnly"},
	}

	// Log the response (sensitive headers and body fields will be redacted)
	logger.LogResponse(http.StatusOK, respHeaders, respBody)
}

func demonstrateStructuredRedaction(logger *omni.Omni) {
	// Create a structured logger
	structLogger := logger.WithFields(map[string]interface{}{
		"service": "user-service",
		"version": "1.0.0",
		"environment": "production",
	})

	// Log with additional sensitive fields
	structLogger.Info("Payment processed", map[string]interface{}{
		"transaction_id": "txn_123456",
		"amount": 99.99,
		"currency": "USD",
		"card_number": "4242 4242 4242 4242", // Will be redacted
		"cvv": "123", // Consider adding pattern for this
		"billing": map[string]interface{}{
			"name": "Bob Johnson",
			"email": "bob@example.com", // Will be redacted if custom pattern is active
			"address": "123 Main St",
		},
	})

	// Log with nested sensitive data
	structLogger.Warn("Authentication failed", map[string]interface{}{
		"attempt": 3,
		"ip_address": "192.168.1.100",
		"provided_credentials": map[string]interface{}{
			"username": "bob",
			"password": "wrong_password", // Will be redacted
			"otp_code": "123456",
		},
		"headers": map[string]interface{}{
			"Authorization": "Basic dXNlcjpwYXNz", // Will be redacted
			"X-API-Secret": "secret_123", // Will be redacted
		},
	})
}

func demonstrateNestedRedaction(logger *omni.Omni) {
	// Complex nested structure with arrays
	complexData := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{
				"id": 1,
				"name": "User One",
				"credentials": map[string]interface{}{
					"password": "pass1", // Will be redacted
					"api_keys": []interface{}{
						"key_1_abc", // Will be redacted
						"key_2_def", // Will be redacted
					},
				},
			},
			map[string]interface{}{
				"id": 2,
				"name": "User Two",
				"credentials": map[string]interface{}{
					"password": "pass2", // Will be redacted
					"tokens": map[string]interface{}{
						"access_token": "access_xyz", // Will be redacted
						"refresh_token": "refresh_abc", // Will be redacted
					},
				},
			},
		},
		"system": map[string]interface{}{
			"database": map[string]interface{}{
				"master_password": "master_pass", // Will be redacted
				"replicas": []interface{}{
					map[string]interface{}{
						"host": "replica1.db",
						"password": "replica1_pass", // Will be redacted
					},
					map[string]interface{}{
						"host": "replica2.db",
						"password": "replica2_pass", // Will be redacted
					},
				},
			},
			"services": map[string]interface{}{
				"auth": map[string]interface{}{
					"secret_key": "auth_secret", // Will be redacted
					"private_key": "-----BEGIN PRIVATE KEY-----...", // Will be redacted
				},
			},
		},
	}

	logger.Info("System configuration dump", complexData)

	// Mixed content with both JSON and plain text
	logger.Error("Security audit failed",
		"Details: Found exposed credentials in config files",
		"password=exposed_password", // Will be redacted
		"api_key=exposed_key", // Will be redacted
		"Config file contents: {\"db_password\": \"another_password\"}", // JSON content will be parsed and redacted
	)
}

func demonstratePerformanceOptimizations(logger *omni.Omni) {
	// Configure redaction to skip DEBUG level for performance
	logger.SetRedactionConfig(&omni.RedactionConfig{
		EnableBuiltInPatterns: true,
		EnableFieldRedaction:  true,
		EnableDataPatterns:    true,
		SkipLevels:           []int{omni.LevelDebug}, // Don't redact debug logs
		MaxCacheSize:         2000,
	})

	// Debug logs won't be redacted (better performance for debug)
	logger.Debug("Debug info", map[string]interface{}{
		"password": "debug_password", // This won't be redacted due to level skipping
		"api_key": "debug_key",
		"user_id": "12345",
	})

	// Production logs will still be redacted
	logger.Info("Production info", map[string]interface{}{
		"password": "prod_password", // This will be redacted
		"api_key": "prod_key",
		"user_id": "12345",
	})

	// Demonstrate caching - repeated calls will use cached results
	for i := 0; i < 3; i++ {
		logger.Info("Repeated log entry", map[string]interface{}{
			"iteration": i,
			"password": "same_password", // Cached redaction after first call
			"timestamp": time.Now().Unix(),
		})
	}

	// Clear cache manually if needed (for memory management)
	logger.ClearRedactionCache()
}

func demonstrateFieldPathRedaction(logger *omni.Omni) {
	// Configure specific field paths for targeted redaction
	logger.AddFieldPathRule("user.credentials.password", "[USER-PASSWORD-REDACTED]")
	logger.AddFieldPathRule("user.profile.ssn", "[SSN-REDACTED]")
	logger.AddFieldPathRule("config.*.secret", "[CONFIG-SECRET-REDACTED]") // Wildcard
	logger.AddFieldPathRule("users.*.email", "[USER-EMAIL-REDACTED]") // Array wildcard

	// Log data that will be redacted based on field paths
	logger.Info("User management operation", map[string]interface{}{
		"action": "user_update",
		"user": map[string]interface{}{
			"id": "user123",
			"name": "John Doe",
			"credentials": map[string]interface{}{
				"username": "john.doe",
				"password": "super_secret", // Will be redacted with custom text
				"last_login": "2024-01-15T10:30:00Z",
			},
			"profile": map[string]interface{}{
				"email": "john@example.com", // Regular redaction
				"ssn": "123-45-6789", // Will be redacted with custom text
				"age": 30,
			},
		},
		"config": map[string]interface{}{
			"database": map[string]interface{}{
				"host": "db.example.com",
				"secret": "db_secret_key", // Will be redacted with wildcard rule
			},
			"api": map[string]interface{}{
				"endpoint": "https://api.example.com",
				"secret": "api_secret_key", // Will be redacted with wildcard rule
			},
		},
		"users": []interface{}{
			map[string]interface{}{
				"id": 1,
				"name": "Alice",
				"email": "alice@example.com", // Will be redacted with wildcard rule
			},
			map[string]interface{}{
				"id": 2,
				"name": "Bob",
				"email": "bob@example.com", // Will be redacted with wildcard rule
			},
		},
	})

	// Demonstrate removing field path rules
	logger.RemoveFieldPathRule("config.*.secret")
	
	// Show current rules
	rules := logger.GetFieldPathRules()
	logger.Info("Current field path rules", map[string]interface{}{
		"rule_count": len(rules),
		"rules": rules,
	})
}