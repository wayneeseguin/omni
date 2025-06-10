package main

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/wayneeseguin/omni/pkg/omni"
)

func TestRedactionExample(t *testing.T) {
	// Create a test log file
	logFile := "test_redacted.log"
	defer os.Remove(logFile)

	// Create logger with redaction
	logger, err := omni.NewWithOptions(
		omni.WithPath(logFile),
		omni.WithJSON(),
		omni.WithRedaction([]string{}, "[REDACTED]"), // Enable built-in redaction
	)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test built-in redaction
	t.Run("BuiltInRedaction", func(t *testing.T) {
		logger.InfoWithFields("Test message", map[string]interface{}{
			"password":   "secret123",
			"api_key":    "sk-test123",
			"safe_field": "visible",
		})

		// Wait for message to be written and flush
		time.Sleep(100 * time.Millisecond)
		logger.FlushAll()

		// Read and verify log content
		content := readLastLogEntry(t, logFile)
		t.Logf("Log content: %s", content)

		// Check that sensitive fields are redacted
		if strings.Contains(content, "secret123") {
			t.Error("Password was not redacted")
		}
		if strings.Contains(content, "sk-test123") {
			t.Error("API key was not redacted")
		}
		if !strings.Contains(content, "visible") {
			t.Error("Safe field was incorrectly redacted")
		}
		if !strings.Contains(content, "[REDACTED]") {
			t.Error("Expected [REDACTED] placeholder not found")
		}
	})

	// Test custom patterns
	t.Run("CustomPatterns", func(t *testing.T) {
		// Create a new logger with custom SSN pattern
		ssnLogger, err := omni.NewWithOptions(
			omni.WithPath("ssn_test.log"),
			omni.WithJSON(),
			omni.WithRedaction([]string{`\b\d{3}-\d{2}-\d{4}\b`}, "[SSN-REDACTED]"),
		)
		if err != nil {
			t.Fatalf("Failed to create logger with SSN redaction: %v", err)
		}
		defer ssnLogger.Close()
		defer os.Remove("ssn_test.log")

		ssnLogger.InfoWithFields("Customer data", map[string]interface{}{
			"name": "John Doe",
			"ssn":  "123-45-6789",
		})

		time.Sleep(100 * time.Millisecond)
		ssnLogger.FlushAll()

		content := readLastLogEntry(t, "ssn_test.log")
		t.Logf("SSN Log content: %s", content)

		if strings.Contains(content, "123-45-6789") {
			t.Error("SSN was not redacted")
		}
		if !strings.Contains(content, "[REDACTED]") {
			t.Error("Expected redaction placeholder not found")
		}
	})

	// Test nested redaction
	t.Run("NestedRedaction", func(t *testing.T) {
		logger.InfoWithFields("Nested data", map[string]interface{}{
			"user": map[string]interface{}{
				"name": "Alice",
				"credentials": map[string]interface{}{
					"password": "nested_secret",
					"token":    "nested_token",
				},
			},
		})

		time.Sleep(50 * time.Millisecond)

		content := readLastLogEntry(t, logFile)

		if strings.Contains(content, "nested_secret") {
			t.Error("Nested password was not redacted")
		}
		if strings.Contains(content, "nested_token") {
			t.Error("Nested token was not redacted")
		}
	})

	// Test array redaction
	t.Run("ArrayRedaction", func(t *testing.T) {
		logger.InfoWithFields("Array data", map[string]interface{}{
			"api_keys": []string{"key1", "key2", "key3"},
			"passwords": []interface{}{
				"pass1",
				map[string]interface{}{"value": "pass2"},
			},
		})

		time.Sleep(50 * time.Millisecond)

		content := readLastLogEntry(t, logFile)

		// All keys and passwords should be redacted
		if strings.Contains(content, "key1") || strings.Contains(content, "key2") {
			t.Error("API keys in array were not redacted")
		}
		if strings.Contains(content, "pass1") || strings.Contains(content, "pass2") {
			t.Error("Passwords in array were not redacted")
		}
	})
}

func TestRedactionPerformance(t *testing.T) {
	// Create logger with redaction
	logger, err := omni.NewWithOptions(
		omni.WithPath("perf_test.log"),
		omni.WithJSON(),
	)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()
	defer os.Remove("perf_test.log")

	// Add multiple custom patterns
	patterns := []string{
		`\b\d{3}-\d{2}-\d{4}\b`,                               // SSN
		`\b\d{4}[-\s]?\d{4}[-\s]?\d{4}[-\s]?\d{4}\b`,          // Credit card
		`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`, // Email
	}
	err = logger.SetRedaction(patterns, "[REDACTED]")
	if err != nil {
		t.Fatalf("Failed to set redaction patterns: %v", err)
	}

	// Measure time for logging with redaction
	start := time.Now()
	iterations := 1000

	for i := 0; i < iterations; i++ {
		logger.InfoWithFields("Performance test", map[string]interface{}{
			"iteration":   i,
			"email":       "test@example.com",
			"ssn":         "123-45-6789",
			"credit_card": "4111-1111-1111-1111",
			"password":    "secret123",
			"data": map[string]interface{}{
				"nested_email": "nested@example.com",
				"nested_token": "token123",
			},
		})
	}

	elapsed := time.Since(start)
	perMessage := elapsed / time.Duration(iterations)

	t.Logf("Logged %d messages in %v", iterations, elapsed)
	t.Logf("Average time per message: %v", perMessage)

	// Ensure it's reasonably fast (adjust threshold as needed)
	if perMessage > 1*time.Millisecond {
		t.Errorf("Redaction performance is too slow: %v per message", perMessage)
	}
}

func readLastLogEntry(t *testing.T, filename string) string {
	file, err := os.Open(filename)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer file.Close()

	var lastLine string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lastLine = scanner.Text()
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	return lastLine
}

func TestRedactionCompleteness(t *testing.T) {
	// This test verifies that all sensitive patterns are properly redacted
	logFile := "completeness_test.log"
	defer os.Remove(logFile)

	logger, err := omni.NewWithOptions(
		omni.WithPath(logFile),
		omni.WithJSON(),
	)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Test all built-in sensitive field names
	sensitiveFields := []string{
		"password", "passwd", "pass",
		"secret", "api_key", "apikey",
		"auth_token", "auth-token", "authorization",
		"private_key", "privatekey", "token",
		"access_token", "refresh_token",
	}

	testData := make(map[string]interface{})
	for _, field := range sensitiveFields {
		testData[field] = "sensitive_value_" + field
	}

	logger.InfoWithFields("Sensitive fields test", testData)
	time.Sleep(100 * time.Millisecond)
	logger.FlushAll()

	// Read log and parse JSON
	content := readLastLogEntry(t, logFile)
	t.Logf("Completeness log content: %s", content)

	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(content), &logEntry); err != nil {
		t.Fatalf("Failed to parse log entry: %v", err)
	}

	// Check fields in the log entry
	if fields, ok := logEntry["fields"].(map[string]interface{}); ok {
		for fieldName, fieldValue := range fields {
			if strings.Contains(strings.ToLower(fieldName), "password") ||
				strings.Contains(strings.ToLower(fieldName), "secret") ||
				strings.Contains(strings.ToLower(fieldName), "key") ||
				strings.Contains(strings.ToLower(fieldName), "token") {
				if fieldValue != "[REDACTED]" {
					t.Errorf("Field %s was not properly redacted: %v", fieldName, fieldValue)
				}
			}
		}
	}
}
