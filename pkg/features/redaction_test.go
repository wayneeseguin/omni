package features

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

func TestNewRedactor(t *testing.T) {
	patterns := []string{
		`\bpassword=\S+`,
		`\btoken:\s*"[^"]+`,
	}
	
	redactor, err := NewRedactor(patterns, "[REDACTED]")
	if err != nil {
		t.Fatalf("Failed to create redactor: %v", err)
	}
	
	if redactor == nil {
		t.Fatal("NewRedactor returned nil")
	}
	
	if len(redactor.patterns) != 2 {
		t.Errorf("Expected 2 patterns, got %d", len(redactor.patterns))
	}
	
	if redactor.replace != "[REDACTED]" {
		t.Errorf("Expected replace string '[REDACTED]', got '%s'", redactor.replace)
	}
	
	// Test with invalid pattern
	invalidPatterns := []string{`[`} // Invalid regex
	_, err = NewRedactor(invalidPatterns, "[REDACTED]")
	if err == nil {
		t.Error("Expected error for invalid regex pattern")
	}
}

func TestRedactorRedact(t *testing.T) {
	patterns := []string{
		`password=\S+`,
		`"token":\s*"[^"]+"`,  // Match JSON format with quotes
		`api_key=\S+`,
	}
	
	redactor, _ := NewRedactor(patterns, "[REDACTED]")
	
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Password in query string",
			input:    "url=https://example.com?user=john&password=secret123&foo=bar",
			expected: "url=https://example.com?user=john&[REDACTED]", // The rest of the string after match gets consumed
		},
		{
			name:     "Token in JSON",
			input:    `{"user": "john", "token": "abc123xyz", "data": "value"}`,
			expected: `{"user": "john", [REDACTED], "data": "value"}`,
		},
		{
			name:     "API key",
			input:    "config: api_key=sk_test_1234567890",
			expected: "config: [REDACTED]",
		},
		{
			name:     "Multiple patterns",
			input:    "password=secret api_key=key123",
			expected: "[REDACTED] [REDACTED]",
		},
		{
			name:     "No matches",
			input:    "This is a normal log message",
			expected: "This is a normal log message",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactor.Redact(tt.input)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestRedactorCache(t *testing.T) {
	redactor, _ := NewRedactor([]string{`secret=\w+`}, "[REDACTED]")
	
	// First call should cache
	input := "secret=abc123"
	result1 := redactor.Redact(input)
	
	// Second call should use cache
	result2 := redactor.Redact(input)
	
	if result1 != result2 {
		t.Error("Cache returned different results")
	}
	
	// Clear cache
	redactor.ClearCache()
	
	// Check cache was cleared
	redactor.mu.RLock()
	cacheSize := len(redactor.cache)
	redactor.mu.RUnlock()
	
	if cacheSize != 0 {
		t.Errorf("Expected empty cache after clear, got size %d", cacheSize)
	}
}

func TestIsSensitiveKey(t *testing.T) {
	tests := []struct {
		key      string
		expected bool
	}{
		{"password", true},
		{"PASSWORD", true},
		{"user_password", true},
		{"api_key", true},
		{"apikey", true},
		{"secret", true},
		{"token", true},
		{"access_token", true},
		{"refresh_token", true},
		{"authorization", true},
		{"auth_token", true},
		{"ssn", true},
		{"social_security", true},
		{"credit_card", true},
		{"creditcard", true},
		{"username", false},
		{"user_id", false},
		{"timestamp", false},
		{"", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := IsSensitiveKey(tt.key)
			if result != tt.expected {
				t.Errorf("IsSensitiveKey(%s) = %v, expected %v", tt.key, result, tt.expected)
			}
		})
	}
}

func TestRegexRedact(t *testing.T) {
	// Create a custom redactor for additional patterns
	customRedactor, _ := NewRedactor([]string{
		`custom_secret=\w+`,
	}, "[CUSTOM_REDACTED]")
	
	tests := []struct {
		name     string
		input    string
		contains []string // Strings that should be in the result
		absent   []string // Strings that should NOT be in the result
	}{
		{
			name:     "JSON field redaction",
			input:    `{"password": "secret123", "user": "john"}`,
			contains: []string{`"password": "[REDACTED]"`, `"user": "john"`},
			absent:   []string{"secret123"},
		},
		{
			name:     "Authorization header",
			input:    "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			contains: []string{"Authorization: Bearer [REDACTED]"},
			absent:   []string{"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"},
		},
		{
			name:     "Email redaction",
			input:    "Contact user at john.doe@example.com for details",
			contains: []string{"[REDACTED]"},
			absent:   []string{"john.doe@example.com"},
		},
		{
			name:     "Credit card number",
			input:    "Payment with card 4111-1111-1111-1111",
			contains: []string{"[REDACTED]"},
			absent:   []string{"4111-1111-1111-1111"},
		},
		{
			name:     "SSN format",
			input:    "SSN: 123-45-6789",
			contains: []string{"[REDACTED]"},
			absent:   []string{"123-45-6789"},
		},
		{
			name:     "API key patterns",
			input:    "Using key sk-1234567890abcdefghijklmnopqrstuvwxyz1234567890",
			contains: []string{"[REDACTED]"},
			absent:   []string{"sk-1234567890abcdefghijklmnopqrstuvwxyz1234567890"},
		},
		{
			name:     "Key-value pairs",
			input:    "password=mypass api_key=12345 token=abc123",
			contains: []string{"password=[REDACTED]", "api_key=[REDACTED]", "token=[REDACTED]"},
			absent:   []string{"mypass", "12345", "abc123"},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RegexRedact(tt.input, customRedactor)
			
			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected result to contain '%s', got '%s'", expected, result)
				}
			}
			
			for _, unexpected := range tt.absent {
				if strings.Contains(result, unexpected) {
					t.Errorf("Expected '%s' to be redacted from result, but found it in: %s", unexpected, result)
				}
			}
		})
	}
}

func TestRecursiveRedact(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected map[string]interface{}
	}{
		{
			name: "Simple map with sensitive fields",
			input: map[string]interface{}{
				"username": "john",
				"password": "secret123",
				"api_key":  "abc123",
			},
			expected: map[string]interface{}{
				"username": "john",
				"password": "[REDACTED]",
				"api_key":  "[REDACTED]",
			},
		},
		{
			name: "Nested map",
			input: map[string]interface{}{
				"user": map[string]interface{}{
					"name":     "john",
					"password": "secret",
				},
				"config": map[string]interface{}{
					"api_key": "key123",
					"timeout": 30,
				},
			},
			expected: map[string]interface{}{
				"user": map[string]interface{}{
					"name":     "john",
					"password": "[REDACTED]",
				},
				"config": map[string]interface{}{
					"api_key": "[REDACTED]",
					"timeout": 30,
				},
			},
		},
		{
			name: "Array of maps",
			input: map[string]interface{}{
				"users": []interface{}{
					map[string]interface{}{
						"name":     "john",
						"password": "pass1",
					},
					map[string]interface{}{
						"name":     "jane",
						"password": "pass2",
					},
				},
			},
			expected: map[string]interface{}{
				"users": []interface{}{
					map[string]interface{}{
						"name":     "john",
						"password": "[REDACTED]",
					},
					map[string]interface{}{
						"name":     "jane",
						"password": "[REDACTED]",
					},
				},
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to avoid modifying test data
			inputCopy := deepCopyMap(tt.input)
			
			RecursiveRedact(inputCopy, "", nil, nil)
			
			// Compare results
			if !mapsEqual(inputCopy, tt.expected) {
				t.Errorf("RecursiveRedact result mismatch.\nGot: %+v\nExpected: %+v", inputCopy, tt.expected)
			}
		})
	}
}

func TestFieldPathRedaction(t *testing.T) {
	fieldPathRules := []FieldPathRule{
		{Path: "user.ssn", Replacement: "[SSN-REDACTED]"},
		{Path: "payment.card_number", Replacement: "[CARD-REDACTED]"},
		{Path: "users.*.email", Replacement: "[EMAIL-REDACTED]"},
	}
	
	input := map[string]interface{}{
		"user": map[string]interface{}{
			"name": "John Doe",
			"ssn":  "123-45-6789",
		},
		"payment": map[string]interface{}{
			"method":      "credit_card",
			"card_number": "4111-1111-1111-1111",
		},
		"users": []interface{}{
			map[string]interface{}{
				"name":  "User1",
				"email": "user1@example.com",
			},
			map[string]interface{}{
				"name":  "User2",
				"email": "user2@example.com",
			},
		},
	}
	
	// Make a copy
	inputCopy := deepCopyMap(input)
	
	RecursiveRedact(inputCopy, "", nil, fieldPathRules)
	
	// Check specific field path redactions
	user := inputCopy.(map[string]interface{})["user"].(map[string]interface{})
	if user["ssn"] != "[SSN-REDACTED]" {
		t.Errorf("Expected user.ssn to be '[SSN-REDACTED]', got %v", user["ssn"])
	}
	
	payment := inputCopy.(map[string]interface{})["payment"].(map[string]interface{})
	if payment["card_number"] != "[CARD-REDACTED]" {
		t.Errorf("Expected payment.card_number to be '[CARD-REDACTED]', got %v", payment["card_number"])
	}
}

func TestMatchesPath(t *testing.T) {
	tests := []struct {
		path     string
		pattern  string
		expected bool
	}{
		{"user.profile.ssn", "user.profile.ssn", true},
		{"user.profile.name", "user.profile.ssn", false},
		{"users.0.email", "users.*.email", true},
		{"users.123.email", "users.*.email", true},
		{"users.email", "users.*.email", false},
		{"config.database.password", "config.*.password", true},
		{"config.cache.password", "config.*.password", true},
		{"password", "*.password", true},
	}
	
	for _, tt := range tests {
		t.Run(tt.path+"_"+tt.pattern, func(t *testing.T) {
			result := MatchesPath(tt.path, tt.pattern)
			if result != tt.expected {
				t.Errorf("MatchesPath(%s, %s) = %v, expected %v", tt.path, tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestRedactSensitiveWithLevel(t *testing.T) {
	config := &RedactionConfig{
		EnableBuiltInPatterns: true,
		SkipLevels:           []int{0}, // Skip DEBUG level
	}
	
	input := `{"password": "secret123", "debug": "info"}`
	
	// Test with DEBUG level (should skip)
	result := RedactSensitiveWithLevel(input, 0, config, nil, nil)
	if result != input {
		t.Error("Expected no redaction for skipped level")
	}
	
	// Test with INFO level (should redact)
	result = RedactSensitiveWithLevel(input, 2, config, nil, nil)
	if strings.Contains(result, "secret123") {
		t.Error("Expected password to be redacted")
	}
	
	// Test empty input
	result = RedactSensitiveWithLevel("", 2, config, nil, nil)
	if result != "" {
		t.Error("Expected empty string for empty input")
	}
}

func TestNewRedactionManager(t *testing.T) {
	rm := NewRedactionManager()
	
	if rm == nil {
		t.Fatal("NewRedactionManager returned nil")
	}
	
	if rm.fieldPathRules == nil {
		t.Error("fieldPathRules should be initialized")
	}
	
	if rm.contextualRules == nil {
		t.Error("contextualRules should be initialized")
	}
	
	if rm.metrics == nil {
		t.Error("metrics should be initialized")
	}
	
	if rm.hashSalt == "" {
		t.Error("hashSalt should be generated")
	}
}

func TestRedactionManagerConfiguration(t *testing.T) {
	rm := NewRedactionManager()
	
	// Test SetConfig
	config := &RedactionConfig{
		EnableBuiltInPatterns: true,
		EnableFieldRedaction:  true,
		MaxCacheSize:         100,
		SkipLevels:           []int{0},
	}
	
	rm.SetConfig(config)
	
	rm.mu.RLock()
	hasCache := rm.cache != nil
	rm.mu.RUnlock()
	
	if !hasCache {
		t.Error("Expected cache to be initialized when MaxCacheSize > 0")
	}
	
	// Test error handler
	rm.SetErrorHandler(func(source, dest, msg string, err error) {
		// Error handler configured
	})
	
	// Test metrics handler
	metricsHandlerCalled := false
	rm.SetMetricsHandler(func(event string) {
		metricsHandlerCalled = true
	})
	
	// Add a field path rule (should trigger metrics)
	rm.AddFieldPathRule(FieldPathRule{Path: "test.path", Replacement: "[TEST]"})
	
	if !metricsHandlerCalled {
		t.Error("Expected metrics handler to be called")
	}
}

func TestContextualRules(t *testing.T) {
	rm := NewRedactionManager()
	
	// Add contextual rule that redacts email for non-admin users
	rule := ContextualRule{
		Name: "non_admin_email",
		Condition: func(level int, fields map[string]interface{}) bool {
			if fields == nil {
				return false
			}
			role, ok := fields["role"].(string)
			return ok && role != "admin"
		},
		RedactFields: []string{"email", "phone"},
		Replacement:  "[RESTRICTED]",
	}
	
	rm.AddContextualRule(rule)
	
	// Test with non-admin user
	fields := map[string]interface{}{
		"role":  "user",
		"email": "user@example.com",
		"phone": "123-456-7890",
		"name":  "John",
	}
	
	_, redactedFields := rm.RedactMessage(2, "test message", fields)
	
	if redactedFields["email"] != "[RESTRICTED]" {
		t.Errorf("Expected email to be redacted for non-admin, got %v", redactedFields["email"])
	}
	
	if redactedFields["phone"] != "[RESTRICTED]" {
		t.Errorf("Expected phone to be redacted for non-admin, got %v", redactedFields["phone"])
	}
	
	if redactedFields["name"] != "John" {
		t.Errorf("Expected name to remain unchanged, got %v", redactedFields["name"])
	}
	
	// Test with admin user
	adminFields := map[string]interface{}{
		"role":  "admin",
		"email": "admin@example.com",
		"phone": "123-456-7890",
	}
	
	_, redactedAdminFields := rm.RedactMessage(2, "test message", adminFields)
	
	if redactedAdminFields["email"] == "[RESTRICTED]" {
		t.Error("Expected email NOT to be redacted for admin")
	}
}

func TestCreateSpecializedRedactors(t *testing.T) {
	tests := []struct {
		name        string
		createFunc  func() *Redactor
		testInput   string
		shouldMatch bool
	}{
		{
			name:        "Credit Card Redactor",
			createFunc:  CreateCreditCardRedactor,
			testInput:   "Card: 4111-1111-1111-1111",
			shouldMatch: true,
		},
		{
			name:        "SSN Redactor",
			createFunc:  CreateSSNRedactor,
			testInput:   "SSN: 123-45-6789",
			shouldMatch: true,
		},
		{
			name:        "Email Redactor",
			createFunc:  CreateEmailRedactor,
			testInput:   "Email: test@example.com",
			shouldMatch: true,
		},
		{
			name:        "API Key Redactor",
			createFunc:  CreateAPIKeyRedactor,
			testInput:   "Key: sk-1234567890abcdefghijklmnopqrstuvwxyz123456789012",
			shouldMatch: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redactor := tt.createFunc()
			result := redactor.Redact(tt.testInput)
			
			if tt.shouldMatch && result == tt.testInput {
				t.Errorf("Expected redactor to match and redact input, but got: %s", result)
			}
		})
	}
}

func TestConcurrentRedaction(t *testing.T) {
	rm := NewRedactionManager()
	
	config := &RedactionConfig{
		EnableBuiltInPatterns: true,
		MaxCacheSize:         100,
	}
	rm.SetConfig(config)
	
	// Add custom redactor
	customRedactor, _ := NewRedactor([]string{`secret=\w+`}, "[REDACTED]")
	rm.SetCustomRedactor(customRedactor)
	
	// Run concurrent redactions
	var wg sync.WaitGroup
	numGoroutines := 10
	numOperations := 100
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutine int) {
			defer wg.Done()
			
			for j := 0; j < numOperations; j++ {
				message := "Processing secret=abc123 for user"
				fields := map[string]interface{}{
					"password": "pass123",
					"index":    j,
				}
				
				redactedMsg, redactedFields := rm.RedactMessage(2, message, fields)
				
				if strings.Contains(redactedMsg, "abc123") {
					t.Error("Secret not redacted in message")
				}
				
				if redactedFields["password"] != "[REDACTED]" {
					t.Error("Password not redacted in fields")
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// Check metrics
	metrics := rm.GetMetrics()
	expectedTotal := uint64(numGoroutines * numOperations)
	
	if metrics.TotalProcessed != expectedTotal {
		t.Errorf("Expected %d total processed, got %d", expectedTotal, metrics.TotalProcessed)
	}
}

func TestRedactionMasking(t *testing.T) {
	rm := NewRedactionManager()
	rm.preserveStructure = true
	
	tests := []struct {
		input    string
		expected string // Expected pattern (X for masked chars)
	}{
		{
			input:    "123-45-6789",
			expected: "XXX-XX-6789", // Last 4 digits visible
		},
		{
			input:    "abcdefghij",
			expected: "XXXXXXXXXX",
		},
		{
			input:    "test@example.com",
			expected: "XXXX@XXXXXXX.XXX",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := rm.maskValue(tt.input)
			if result != tt.expected {
				t.Errorf("Expected masked value %s, got %s", tt.expected, result)
			}
		})
	}
}

// Helper functions

func deepCopyMap(input interface{}) interface{} {
	// Simple deep copy using JSON marshal/unmarshal
	data, _ := json.Marshal(input)
	var result interface{}
	json.Unmarshal(data, &result)
	return result
}

func mapsEqual(a, b interface{}) bool {
	// Simple equality check using JSON
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	return string(aJSON) == string(bJSON)
}