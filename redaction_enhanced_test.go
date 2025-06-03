package flexlog

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuiltInDataPatterns(t *testing.T) {
	logger := createTestRedactionLogger()

	tests := []struct {
		name     string
		input    string
		contains []string // Should contain these after redaction
		absent   []string // Should not contain these after redaction
	}{
		{
			name:     "SSN patterns",
			input:    "User SSN: 123-45-6789 and alternate format 987 65 4321 and compact 555443333",
			contains: []string{"[REDACTED]"},
			absent:   []string{"123-45-6789", "987 65 4321", "555443333"},
		},
		{
			name:     "Credit card patterns",
			input:    "Visa: 4111-1111-1111-1111, MasterCard: 5555 5555 5555 4444, Amex: 3782 822463 10005",
			contains: []string{"[REDACTED]"},
			absent:   []string{"4111-1111-1111-1111", "5555 5555 5555 4444", "3782 822463 10005"},
		},
		{
			name:     "Email patterns",
			input:    "Contact us at support@example.com or admin@test-domain.org",
			contains: []string{"[REDACTED]"},
			absent:   []string{"support@example.com", "admin@test-domain.org"},
		},
		{
			name:     "Phone patterns",
			input:    "Call 555-123-4567 or (555) 987-6543 for support",
			contains: []string{"[REDACTED]"},
			absent:   []string{"555-123-4567", "(555) 987-6543"},
		},
		{
			name:     "API key patterns",
			input:    "OpenAI: sk-1234567890abcdef1234567890abcdef1234567890abcdef, AWS: AKIAIOSFODNN7EXAMPLE, GitHub: ghp_1234567890abcdef1234567890abcdef123456",
			contains: []string{"[REDACTED]"},
			absent:   []string{"sk-1234567890abcdef1234567890abcdef1234567890abcdef", "AKIAIOSFODNN7EXAMPLE", "ghp_1234567890abcdef1234567890abcdef123456"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := logger.regexRedact(tt.input)

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected redacted output to contain %q, got: %s", expected, result)
				}
			}

			for _, absent := range tt.absent {
				if strings.Contains(result, absent) {
					t.Errorf("Expected redacted output to NOT contain %q, got: %s", absent, result)
				}
			}
		})
	}
}

func TestExpandedSensitiveKeywords(t *testing.T) {
	logger := createTestRedactionLogger()

	testData := map[string]interface{}{
		"password":      "secret123",
		"passwd":        "secret456",
		"pass":          "secret789",
		"api_key":       "key123",
		"apikey":        "key456",
		"client_secret": "client123",
		"oauth":         "oauth456",
		"jwt":           "jwt789",
		"ssn":           "123-45-6789",
		"credit_card":   "4111111111111111",
		"email":         "user@example.com",
		"phone":         "555-123-4567",
		"safe_field":    "this should not be redacted",
	}

	jsonData, _ := json.Marshal(testData)
	result := logger.redactSensitive(string(jsonData))

	var redactedData map[string]interface{}
	json.Unmarshal([]byte(result), &redactedData)

	sensitiveFields := []string{
		"password", "passwd", "pass", "api_key", "apikey", 
		"client_secret", "oauth", "jwt", "ssn", "credit_card", 
		"email", "phone",
	}

	for _, field := range sensitiveFields {
		if value, exists := redactedData[field]; exists {
			if value != "[REDACTED]" {
				t.Errorf("Field %s was not redacted: %v", field, value)
			}
		}
	}

	// Check that safe field is not redacted
	if redactedData["safe_field"] != "this should not be redacted" {
		t.Error("Safe field was incorrectly redacted")
	}
}

func TestFieldPathRedaction(t *testing.T) {
	logger := createTestRedactionLogger()

	// Add field path rules
	logger.AddFieldPathRule("user.profile.ssn", "[SSN-REDACTED]")
	logger.AddFieldPathRule("users.*.email", "[EMAIL-REDACTED]")
	logger.AddFieldPathRule("system.database.password", "[DB-PASSWORD-REDACTED]")

	testData := map[string]interface{}{
		"user": map[string]interface{}{
			"name": "John Doe",
			"profile": map[string]interface{}{
				"ssn":   "123-45-6789",
				"email": "john@example.com",
			},
		},
		"users": []interface{}{
			map[string]interface{}{
				"name":  "Alice",
				"email": "alice@example.com",
			},
			map[string]interface{}{
				"name":  "Bob",
				"email": "bob@example.com",
			},
		},
		"system": map[string]interface{}{
			"database": map[string]interface{}{
				"host":     "localhost",
				"password": "db_secret",
			},
		},
	}

	jsonData, _ := json.Marshal(testData)
	result := logger.redactSensitive(string(jsonData))

	// Check that specific paths are redacted with custom text
	if !strings.Contains(result, "[SSN-REDACTED]") {
		t.Error("SSN field path was not redacted with custom text")
	}
	if !strings.Contains(result, "[EMAIL-REDACTED]") {
		t.Error("Email field path was not redacted with custom text")
	}
	if !strings.Contains(result, "[DB-PASSWORD-REDACTED]") {
		t.Error("Database password field path was not redacted with custom text")
	}

	// Check that original values are not present
	if strings.Contains(result, "123-45-6789") {
		t.Error("Original SSN value found in redacted output")
	}
	if strings.Contains(result, "db_secret") {
		t.Error("Original database password found in redacted output")
	}
}

func TestRedactionConfig(t *testing.T) {
	logger := createTestRedactionLogger()

	// Test level-based skipping
	config := &RedactionConfig{
		EnableBuiltInPatterns: true,
		EnableFieldRedaction:  true,
		EnableDataPatterns:    true,
		SkipLevels:           []int{LevelDebug},
		MaxCacheSize:         1000,
	}
	logger.SetRedactionConfig(config)

	testInput := `{"password": "secret123", "api_key": "key456"}`

	// Debug level should skip redaction
	debugResult := logger.redactSensitiveWithLevel(testInput, LevelDebug)
	if !strings.Contains(debugResult, "secret123") {
		t.Error("Debug level should skip redaction but didn't")
	}

	// Info level should apply redaction
	infoResult := logger.redactSensitiveWithLevel(testInput, LevelInfo)
	if strings.Contains(infoResult, "secret123") {
		t.Error("Info level should apply redaction but didn't")
	}
}

func TestRedactionCaching(t *testing.T) {
	logger := createTestRedactionLogger()

	// Add custom patterns to enable caching
	patterns := []string{`\b\d{3}-\d{2}-\d{4}\b`} // SSN pattern
	logger.SetRedaction(patterns, "[REDACTED]")

	testInput := "SSN: 123-45-6789"

	// First call should populate cache
	result1 := logger.redactor.Redact(testInput)
	
	// Second call should hit cache
	result2 := logger.redactor.Redact(testInput)

	if result1 != result2 {
		t.Error("Cached redaction results should be identical")
	}

	if !strings.Contains(result1, "[REDACTED]") {
		t.Error("Redaction did not work")
	}

	if strings.Contains(result1, "123-45-6789") {
		t.Error("Original SSN found in redacted output")
	}
}

func TestFieldPathWildcards(t *testing.T) {
	logger := createTestRedactionLogger()

	// Add wildcard field path rule
	logger.AddFieldPathRule("users.*.profile.email", "[EMAIL-REDACTED]")

	testData := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{
				"id": 1,
				"profile": map[string]interface{}{
					"name":  "Alice",
					"email": "alice@example.com",
				},
			},
			map[string]interface{}{
				"id": 2,
				"profile": map[string]interface{}{
					"name":  "Bob",
					"email": "bob@example.com",
				},
			},
		},
	}

	jsonData, _ := json.Marshal(testData)
	result := logger.redactSensitive(string(jsonData))

	// Both emails should be redacted
	emailCount := strings.Count(result, "[EMAIL-REDACTED]")
	if emailCount < 2 {
		t.Errorf("Expected 2 email redactions, got %d", emailCount)
	}

	// Original emails should not be present
	if strings.Contains(result, "alice@example.com") || strings.Contains(result, "bob@example.com") {
		t.Error("Original email addresses found in redacted output")
	}
}

func TestRedactionLevelAPI(t *testing.T) {
	logger := createTestRedactionLogger()

	// Test enabling/disabling redaction for specific levels
	logger.EnableRedactionForLevel(LevelDebug, false)
	logger.EnableRedactionForLevel(LevelError, true)

	config := logger.GetRedactionConfig()
	if config == nil {
		t.Fatal("Redaction config should not be nil")
	}

	// Check that DEBUG is in skip list
	debugSkipped := false
	for _, level := range config.SkipLevels {
		if level == LevelDebug {
			debugSkipped = true
			break
		}
	}
	if !debugSkipped {
		t.Error("DEBUG level should be in skip list")
	}

	// Test removing from skip list
	logger.EnableRedactionForLevel(LevelDebug, true)
	config = logger.GetRedactionConfig()
	
	debugSkipped = false
	for _, level := range config.SkipLevels {
		if level == LevelDebug {
			debugSkipped = true
			break
		}
	}
	if debugSkipped {
		t.Error("DEBUG level should not be in skip list after enabling")
	}
}

func TestFieldPathRuleManagement(t *testing.T) {
	logger := createTestRedactionLogger()

	// Add rules
	logger.AddFieldPathRule("user.ssn", "[SSN-REDACTED]")
	logger.AddFieldPathRule("user.email", "[EMAIL-REDACTED]")

	rules := logger.GetFieldPathRules()
	if len(rules) != 2 {
		t.Errorf("Expected 2 rules, got %d", len(rules))
	}

	// Update existing rule
	logger.AddFieldPathRule("user.ssn", "[UPDATED-SSN-REDACTED]")
	rules = logger.GetFieldPathRules()
	if len(rules) != 2 {
		t.Errorf("Expected 2 rules after update, got %d", len(rules))
	}

	// Check updated replacement
	found := false
	for _, rule := range rules {
		if rule.Path == "user.ssn" && rule.Replacement == "[UPDATED-SSN-REDACTED]" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Rule was not updated correctly")
	}

	// Remove rule
	logger.RemoveFieldPathRule("user.email")
	rules = logger.GetFieldPathRules()
	if len(rules) != 1 {
		t.Errorf("Expected 1 rule after removal, got %d", len(rules))
	}

	// Clear all rules
	logger.ClearFieldPathRules()
	rules = logger.GetFieldPathRules()
	if len(rules) != 0 {
		t.Errorf("Expected 0 rules after clearing, got %d", len(rules))
	}
}

func TestRedactionPerformanceOptimizations(t *testing.T) {
	logger := createTestRedactionLogger()

	// Test JSON detection optimization
	jsonInput := `{"password": "secret123"}`
	nonJSONInput := "password=secret123"

	// Both should be redacted, but through different paths
	jsonResult := logger.redactSensitive(jsonInput)
	textResult := logger.redactSensitive(nonJSONInput)

	if strings.Contains(jsonResult, "secret123") {
		t.Error("JSON redaction failed")
	}
	if strings.Contains(textResult, "secret123") {
		t.Error("Text redaction failed")
	}

	// Test cache clearing
	logger.SetRedaction([]string{`test`}, "[REDACTED]")
	logger.ClearRedactionCache()
	// This should not panic and should work normally
	result := logger.redactor.Redact("test string")
	if !strings.Contains(result, "[REDACTED]") {
		t.Error("Redaction failed after cache clear")
	}
}

func TestRedactionEdgeCases(t *testing.T) {
	logger := createTestRedactionLogger()

	tests := []struct {
		name  string
		input string
	}{
		{"empty_string", ""},
		{"null_json", "null"},
		{"malformed_json", `{"key": "value"`},
		{"deeply_nested", `{"a":{"b":{"c":{"d":{"password":"secret"}}}}}`},
		{"mixed_arrays", `{"users":[{"password":"secret1"},{"password":"secret2"}]}`},
		{"unicode_content", `{"password":"πάσσωορδ","email":"tëst@éxample.com"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			result := logger.redactSensitive(tt.input)
			
			// Basic validation - should return some result
			if tt.input != "" && result == "" && tt.name != "empty_string" {
				t.Error("Redaction returned empty result for non-empty input")
			}
		})
	}
}