package flexlog

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"
)

// createTestRedactionLogger creates a test logger that doesn't need flock
func createTestRedactionLogger() *FlexLog {
	// Create a minimal logger for testing redaction functionality only
	logger := &FlexLog{
		level:        LevelInfo,
		format:       FormatText,
		msgChan:      make(chan LogMessage, 10),
		channelSize:  10,
		Destinations: make([]*Destination, 0),
	}

	return logger
}

func TestRedactSensitive(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "no sensitive data",
			input:    `{"message": "Hello World", "level": "info"}`,
			expected: `{"message": "Hello World", "level": "info"}`,
		},
		{
			name:     "auth_token field",
			input:    `{"auth_token": "secret123", "message": "login"}`,
			expected: `{"auth_token": "[REDACTED]", "message": "login"}`,
		},
		{
			name:     "auth_token with whitespace",
			input:    `{"auth_token"  :  "secret123", "message": "login"}`,
			expected: `{"auth_token"  :  "[REDACTED]", "message": "login"}`,
		},
		{
			name:     "password field",
			input:    `{"password": "p@ssw0rd", "username": "user1"}`,
			expected: `{"password": "[REDACTED]", "username": "user1"}`,
		},
		{
			name:     "secret field",
			input:    `{"secret": "very-secret-value", "api": "endpoint"}`,
			expected: `{"secret": "[REDACTED]", "api": "endpoint"}`,
		},
		{
			name:     "key field",
			input:    `{"key": "api-key-12345", "request": "getData"}`,
			expected: `{"key": "[REDACTED]", "request": "getData"}`,
		},
		{
			name:     "private_key field",
			input:    `{"private_key": "-----BEGIN RSA PRIVATE KEY-----\nabc123\n-----END RSA PRIVATE KEY-----", "type": "rsa"}`,
			expected: `{"private_key": "[REDACTED]", "type": "rsa"}`,
		},
		{
			name:     "token field",
			input:    `{"token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9", "valid": true}`,
			expected: `{"token": "[REDACTED]", "valid": true}`,
		},
		{
			name:     "bearer token",
			input:    `Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9`,
			expected: `Authorization: Bearer [REDACTED]`,
		},
		{
			name:     "multiple sensitive fields",
			input:    `{"username": "user1", "password": "secret", "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9", "message": "login request"}`,
			expected: `{"username": "user1", "password": "[REDACTED]", "token": "[REDACTED]", "message": "login request"}`,
		},
		{
			name:     "nested JSON",
			input:    `{"user": {"name": "john", "password": "secret"}, "auth": {"token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"}}`,
			expected: `{"user": {"name": "john", "password": "[REDACTED]"}, "auth": {"token": "[REDACTED]"}}`,
		},
	}

	logger := createTestRedactionLogger()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := logger.redactSensitive(tt.input)
			normalizedGot := normalizeJSON(t, result)
			normalizedWant := normalizeJSON(t, tt.expected)

			if normalizedGot != normalizedWant {
				t.Errorf("redactSensitive() got = %q, want %q", normalizedGot, normalizedWant)
			}
		})
	}
}

func TestLogRequest(t *testing.T) {
	// Test cases
	tests := []struct {
		name    string
		method  string
		path    string
		headers map[string][]string
		body    string
		expect  []string // Substrings that should be in the output
		absent  []string // Substrings that should NOT be in the output
	}{
		{
			name:   "basic request",
			method: "GET",
			path:   "/api/users",
			headers: map[string][]string{
				"Content-Type": {"application/json"},
				"User-Agent":   {"test-client"},
			},
			body: `{"filter": "active=true"}`,
			expect: []string{
				"GET /api/users",
				"Content-Type", "application/json",
				"User-Agent", "test-client",
				`{"filter":"active=true"}`,
			},
			absent: []string{
				"[REDACTED]",
			},
		},
		{
			name:   "request with auth header",
			method: "POST",
			path:   "/api/login",
			headers: map[string][]string{
				"Content-Type":  {"application/json"},
				"Authorization": {"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"},
			},
			body: `{"username": "user1", "password": "secret123"}`,
			expect: []string{
				"POST /api/login",
				"Content-Type", "application/json",
				"Authorization", "[REDACTED]",
				"username", "user1",
				"password", "[REDACTED]",
			},
			absent: []string{
				"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
				"secret123",
			},
		},
		{
			name:   "request with token header",
			method: "GET",
			path:   "/api/data",
			headers: map[string][]string{
				"X-API-Token": {"api-token-12345"},
				"Accept":      {"application/json"},
			},
			body: `{}`,
			expect: []string{
				"GET /api/data",
				"X-API-Token", "[REDACTED]",
				"Accept", "application/json",
			},
			absent: []string{
				"api-token-12345",
			},
		},
		{
			name:   "request with sensitive body",
			method: "PUT",
			path:   "/api/users/1",
			headers: map[string][]string{
				"Content-Type": {"application/json"},
			},
			body: `{"name": "John Doe", "token": "user-token-abc", "key": "private-key-123"}`,
			expect: []string{
				"PUT /api/users/1",
				"Content-Type", "application/json",
				"name", "John Doe",
				"token", "[REDACTED]",
				"key", "[REDACTED]",
			},
			absent: []string{
				"user-token-abc",
				"private-key-123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a buffer to capture log output for this test
			var buf bytes.Buffer

			// Create a test logger that writes to our buffer
			logger := createTestLogger(&buf)

			// Call LogRequest
			logger.LogRequest(tt.method, tt.path, tt.headers, tt.body)

			// Flush the logger to ensure all messages are processed
			if err := logger.FlushAll(); err != nil {
				t.Logf("Warning: flush error: %v", err)
			}

			// Close the logger to stop the worker goroutine
			close(logger.msgChan)

			// Add a small delay to ensure flush completes
			time.Sleep(10 * time.Millisecond)

			output := buf.String()

			// Check expected substrings
			for _, substr := range tt.expect {
				if !strings.Contains(output, substr) {
					t.Errorf("LogRequest output missing expected string: %q", substr)
				}
			}

			// Check absent substrings
			for _, substr := range tt.absent {
				if strings.Contains(output, substr) {
					t.Errorf("LogRequest output contains unexpected string: %q", substr)
				}
			}
		})
	}

}

func TestLogResponse(t *testing.T) {
	// Test cases
	tests := []struct {
		name       string
		statusCode int
		headers    map[string][]string
		body       string
		expect     []string // Substrings that should be in the output
		absent     []string // Substrings that should NOT be in the output
	}{
		{
			name:       "basic response",
			statusCode: 200,
			headers: map[string][]string{
				"Content-Type":   {"application/json"},
				"Content-Length": {"42"},
			},
			body: `{"success": true, "message": "Operation completed"}`,
			expect: []string{
				"Status: 200",
				"Content-Type", "application/json",
				"Content-Length", "42",
				"success", "true",
				"Operation completed",
			},
			absent: []string{
				"[REDACTED]",
			},
		},
		{
			name:       "response with token",
			statusCode: 200,
			headers: map[string][]string{
				"Content-Type":     {"application/json"},
				"X-Response-Token": {"secret-response-token"},
			},
			body: `{"success": true, "auth_token": "user-auth-token-123"}`,
			expect: []string{
				"Status: 200",
				"Content-Type", "application/json",
				"X-Response-Token", "[REDACTED]",
				"success", "true",
				"auth_token", "[REDACTED]",
			},
			absent: []string{
				"secret-response-token",
				"user-auth-token-123",
			},
		},
		{
			name:       "error response with sensitive data",
			statusCode: 401,
			headers: map[string][]string{
				"Content-Type": {"application/json"},
			},
			body: `{"error": "Invalid credentials", "debug": {"attempted_token": "invalid-token-123"}}`,
			expect: []string{
				"Status: 401",
				"Content-Type", "application/json",
				"error", "Invalid credentials",
				"token", "[REDACTED]",
			},
			absent: []string{
				"invalid-token-123",
			},
		},
		{
			name:       "response with multiple sensitive fields",
			statusCode: 200,
			headers: map[string][]string{
				"Content-Type": {"application/json"},
				"X-API-Key":    {"api-key-12345"},
			},
			body: `{
				"user": {
					"name": "Jane Doe",
					"token": "user-token-xyz",
					"secret": "user-secret-123"
				},
				"session": {
					"key": "session-key-456"
				}
			}`,
			expect: []string{
				"Status: 200",
				"Content-Type", "application/json",
				"X-API-Key", "[REDACTED]",
				"name", "Jane Doe",
				"token", "[REDACTED]",
				"secret", "[REDACTED]",
				"key", "[REDACTED]",
			},
			absent: []string{
				"api-key-12345",
				"user-token-xyz",
				"user-secret-123",
				"session-key-456",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a buffer to capture log output for this test
			var buf bytes.Buffer

			// Create a test logger that writes to our buffer
			logger := createTestLogger(&buf)

			logger.LogResponse(tt.statusCode, tt.headers, tt.body)

			// Flush the logger to ensure all messages are processed
			if err := logger.FlushAll(); err != nil {
				t.Logf("Warning: flush error: %v", err)
			}

			// Close the logger to stop the worker goroutine
			close(logger.msgChan)

			// Add a small delay to ensure flush completes
			time.Sleep(10 * time.Millisecond)

			output := buf.String()

			// Check expected substrings
			for _, substr := range tt.expect {
				if !strings.Contains(output, substr) {
					t.Errorf("LogResponse output missing expected string: %q", substr)
				}
			}

			// Check absent substrings
			for _, substr := range tt.absent {
				if strings.Contains(output, substr) {
					t.Errorf("LogResponse output contains unexpected string: %q", substr)
				}
			}
		})
	}
}

func TestSensitivePatternsMatch(t *testing.T) {
	// This test ensures all the regex patterns work as expected
	patterns := map[string]struct {
		input    string
		expected string
	}{
		"auth_token pattern": {
			input:    `{"auth_token": "super-secret-token"}`,
			expected: `{"auth_token": "[REDACTED]"}`,
		},
		"password pattern": {
			input:    `{"password": "p@ssw0rd123!"}`,
			expected: `{"password": "[REDACTED]"}`,
		},
		"secret pattern": {
			input:    `{"secret": "this-is-secret"}`,
			expected: `{"secret": "[REDACTED]"}`,
		},
		"key pattern": {
			input:    `{"key": "api-key-value"}`,
			expected: `{"key": "[REDACTED]"}`,
		},
		"private_key pattern": {
			input:    `{"private_key": "-----BEGIN PRIVATE KEY-----\\nABC123\\n-----END PRIVATE KEY-----"}`,
			expected: `{"private_key": "[REDACTED]"}`,
		},
		"token pattern": {
			input:    `{"token": "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0"}`,
			expected: `{"token": "[REDACTED]"}`,
		},
		"bearer token pattern": {
			input:    `Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0`,
			expected: `Authorization: Bearer [REDACTED]`,
		},
		"complex json with multiple sensitive fields": {
			input: `{
				"user": "john",
				"auth_token": "token123",
				"config": {
					"password": "pass456",
					"secret": "secret789"
				}
			}`,
			expected: `{
				"user": "john",
				"auth_token": "[REDACTED]",
				"config": {
					"password": "[REDACTED]",
					"secret": "[REDACTED]"
				}
			}`,
		},
	}

	logger := createTestRedactionLogger()

	for name, tc := range patterns {
		t.Run(name, func(t *testing.T) {
			result := logger.redactSensitive(tc.input)

			// Compare JSON by normalizing whitespace for better error messages
			normalizedResult := normalizeJSON(t, result)
			normalizedExpected := normalizeJSON(t, tc.expected)

			if normalizedResult != normalizedExpected {
				t.Errorf("\nExpected: %s\nGot:      %s", tc.expected, result)
			}
		})
	}
}

func TestCustomPatternRedaction(t *testing.T) {
	// Add a custom pattern to the redaction list temporarily
	customPattern := regexp.MustCompile(`("ssn"\s*:\s*)"[^"]*"`)

	// Make a backup of the original patterns
	originalPatterns := make([]*regexp.Regexp, len(sensitivePatterns))
	copy(originalPatterns, sensitivePatterns)

	// Add the custom pattern
	sensitivePatterns = append(sensitivePatterns, customPattern)

	// Restore the original patterns after the test
	defer func() {
		sensitivePatterns = originalPatterns
	}()

	logger := createTestRedactionLogger()

	input := `{"name": "John Doe", "ssn": "123-45-6789", "address": "123 Main St"}`
	expected := `{"name": "John Doe", "ssn": "[REDACTED]", "address": "123 Main St"}`

	result := logger.regexRedact(input) // Use regex explicitly for custom patterns

	normalizedResult := normalizeJSON(t, result)
	normalizedExpected := normalizeJSON(t, expected)

	if normalizedResult != normalizedExpected {
		t.Errorf("\nExpected: %s\nGot:      %s", normalizedExpected, normalizedResult)
	}
}

// createTestLogger creates a logger that writes to the provided writer for testing
func createTestLogger(writer io.Writer) *FlexLog {
	// Create a buffered writer
	bufWriter := bufio.NewWriter(writer)

	// Create a minimal logger for testing with required fields
	logger := &FlexLog{
		level:        LevelInfo,
		format:       FormatText,
		msgChan:      make(chan LogMessage, 10),
		channelSize:  10,
		Destinations: make([]*Destination, 0),
	}

	// Create a destination that writes to our buffer
	dest := &Destination{
		Name:    "test",
		Backend: BackendFlock,
		Enabled: true,
		Done:    make(chan struct{}),
		Writer:  bufWriter,
	}

	logger.defaultDest = dest
	logger.Destinations = append(logger.Destinations, dest)

	// Start a worker goroutine to handle messages
	go func() {
		for msg := range logger.msgChan {
			// Directly write to our test buffer and flush immediately
			if msg.Level >= logger.level {
				// For API request/response logs, we need to make sure the entire formatted message gets written
				fmt.Fprintf(writer, msg.Format+"\n", msg.Args...)
				if fWriter, ok := writer.(interface{ Flush() error }); ok {
					_ = fWriter.Flush()
				} else {
					bufWriter.Flush()
				}
			}
		}
	}()

	return logger
}

// normalizeJSON normalizes JSON strings for comparison by parsing and re-encoding
func normalizeJSON(t *testing.T, jsonStr string) string {
	var data interface{}

	// Handle non-JSON strings
	if !strings.Contains(jsonStr, "{") {
		return strings.TrimSpace(jsonStr)
	}

	err := json.Unmarshal([]byte(jsonStr), &data)
	if err != nil {
		// If it's not valid JSON, just return the trimmed string
		return strings.TrimSpace(jsonStr)
	}

	normalized, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("Failed to re-marshal JSON: %v", err)
	}

	return string(normalized)
}
