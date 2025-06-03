package omni

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// Benchmark data for testing
var (
	simpleJSONData = `{
		"username": "john.doe",
		"password": "secret123",
		"email": "john@example.com",
		"api_key": "sk-1234567890abcdef"
	}`

	complexJSONData = `{
		"users": [
			{
				"id": 1,
				"profile": {
					"name": "John Doe",
					"email": "john@example.com",
					"ssn": "123-45-6789",
					"credentials": {
						"password": "secret123",
						"api_keys": ["key1", "key2", "key3"],
						"auth_token": "token_abc123"
					}
				}
			},
			{
				"id": 2,
				"profile": {
					"name": "Jane Smith",
					"email": "jane@example.com",
					"ssn": "987-65-4321",
					"credentials": {
						"password": "secret456",
						"api_keys": ["key4", "key5", "key6"],
						"auth_token": "token_def456"
					}
				}
			}
		],
		"system": {
			"database": {
				"password": "db_secret",
				"connection_string": "postgresql://user:pass@localhost/db"
			},
			"api": {
				"secret_key": "api_secret_789",
				"endpoints": ["https://api1.example.com", "https://api2.example.com"]
			}
		}
	}`

	plainTextData = `Log entry with sensitive data:
		username=alice password=secret123 api_key=sk-abc123def456
		email=alice@example.com ssn=555-44-3333 phone=555-123-4567
		credit_card=4111-1111-1111-1111 token=Bearer eyJhbGciOiJIUzI1NiIs...`

	mixedContentData = `Request details: {"username": "bob", "password": "secret789"} 
		Headers: Authorization: Bearer token123, X-API-Key: key456
		Raw data: ssn=123-45-6789 email=bob@example.com phone=(555) 123-4567`
)

func BenchmarkRedactionSimpleJSON(b *testing.B) {
	logger, err := New("/dev/null")
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.redactSensitive(simpleJSONData)
	}
}

func BenchmarkRedactionComplexJSON(b *testing.B) {
	logger, err := New("/dev/null")
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.redactSensitive(complexJSONData)
	}
}

func BenchmarkRedactionPlainText(b *testing.B) {
	logger, err := New("/dev/null")
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.redactSensitive(plainTextData)
	}
}

func BenchmarkRedactionMixedContent(b *testing.B) {
	logger, err := New("/dev/null")
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.redactSensitive(mixedContentData)
	}
}

func BenchmarkRedactionWithCustomPatterns(b *testing.B) {
	logger, err := New("/dev/null")
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	// Add custom patterns
	customPatterns := []string{
		`\b\d{3}-\d{2}-\d{4}\b`,  // SSN
		`\b\d{4}[-\s]?\d{4}[-\s]?\d{4}[-\s]?\d{4}\b`, // Credit card
		`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`, // Email
		`\b\d{3}[-.]?\d{3}[-.]?\d{4}\b`, // Phone
	}
	logger.SetRedaction(customPatterns, "[CUSTOM-REDACTED]")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.redactSensitive(mixedContentData)
	}
}

func BenchmarkRedactionWithFieldPaths(b *testing.B) {
	logger, err := New("/dev/null")
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	// Add field path rules
	logger.AddFieldPathRule("users.*.profile.ssn", "[SSN-REDACTED]")
	logger.AddFieldPathRule("users.*.profile.email", "[EMAIL-REDACTED]")
	logger.AddFieldPathRule("system.database.password", "[DB-PASSWORD-REDACTED]")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.redactSensitive(complexJSONData)
	}
}

func BenchmarkRedactionWithCaching(b *testing.B) {
	logger, err := New("/dev/null")
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	// Add custom patterns to enable caching
	customPatterns := []string{
		`\b\d{3}-\d{2}-\d{4}\b`,  // SSN
		`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`, // Email
	}
	logger.SetRedaction(customPatterns, "[REDACTED]")

	// Pre-populate cache by running redaction once
	logger.redactSensitive(simpleJSONData)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// This should hit cache for repeated input
		logger.redactSensitive(simpleJSONData)
	}
}

func BenchmarkRedactionCompareWithoutRedaction(b *testing.B) {
	logger, err := New("/dev/null")
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.Run("WithRedaction", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			logger.redactSensitive(complexJSONData)
		}
	})

	b.Run("WithoutRedaction", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Just parse JSON without redaction
			var data interface{}
			json.Unmarshal([]byte(complexJSONData), &data)
			json.Marshal(data)
		}
	})
}

func BenchmarkRedactionLevelSkipping(b *testing.B) {
	logger, err := New("/dev/null")
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	// Configure to skip DEBUG level
	logger.SetRedactionConfig(&RedactionConfig{
		EnableBuiltInPatterns: true,
		EnableFieldRedaction:  true,
		EnableDataPatterns:    true,
		SkipLevels:           []int{LevelDebug},
		MaxCacheSize:         1000,
	})

	b.Run("DebugLevel_Skipped", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			logger.redactSensitiveWithLevel(complexJSONData, LevelDebug)
		}
	})

	b.Run("InfoLevel_Redacted", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			logger.redactSensitiveWithLevel(complexJSONData, LevelInfo)
		}
	})
}

func BenchmarkRedactionPatternCompilation(b *testing.B) {
	patterns := []string{
		`\b\d{3}-\d{2}-\d{4}\b`,
		`\b\d{4}[-\s]?\d{4}[-\s]?\d{4}[-\s]?\d{4}\b`,
		`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`,
		`\b\d{3}[-.]?\d{3}[-.]?\d{4}\b`,
		`\bsk-[a-zA-Z0-9]{48}\b`,
		`\bAKIA[0-9A-Z]{16}\b`,
		`\bghp_[a-zA-Z0-9]{36}\b`,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := NewRedactor(patterns, "[REDACTED]")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRedactionMemoryUsage(b *testing.B) {
	logger, err := New("/dev/null")
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	// Add patterns to enable caching
	customPatterns := []string{
		`\b\d{3}-\d{2}-\d{4}\b`,
		`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`,
	}
	logger.SetRedaction(customPatterns, "[REDACTED]")

	// Test with many different inputs to fill cache
	inputs := make([]string, 100)
	for i := 0; i < 100; i++ {
		inputs[i] = fmt.Sprintf(`{"user_%d": {"email": "user%d@example.com", "ssn": "123-45-%04d"}}`, i, i, i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.redactSensitive(inputs[i%len(inputs)])
	}

	// Force cleanup to test cache clearing
	logger.ClearRedactionCache()
}


func BenchmarkRedactionJSONParsing(b *testing.B) {
	logger, err := New("/dev/null")
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Close()

	b.Run("FastJSONCheck", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Test the fast JSON detection
			trimmed := strings.TrimSpace(complexJSONData)
			isLikelyJSON := (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
				(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]"))
			_ = isLikelyJSON
		}
	})

	b.Run("FullJSONParsing", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var data interface{}
			json.Unmarshal([]byte(complexJSONData), &data)
		}
	})
}