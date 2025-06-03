package omni

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestFieldTypes(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected FieldType
	}{
		{"nil", nil, FieldTypeNil},
		{"string", "hello", FieldTypeString},
		{"int", 42, FieldTypeInt},
		{"float", 3.14, FieldTypeFloat},
		{"bool", true, FieldTypeBool},
		{"time", time.Now(), FieldTypeTime},
		{"duration", time.Hour, FieldTypeDuration},
		{"error", errors.New("test"), FieldTypeError},
		{"array", []int{1, 2, 3}, FieldTypeArray},
		{"object", map[string]int{"a": 1}, FieldTypeObject},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetFieldType(tt.value)
			if result != tt.expected {
				t.Errorf("GetFieldType(%v) = %v, want %v", tt.value, result, tt.expected)
			}
		})
	}
}

func TestFieldValidation(t *testing.T) {
	// Create logger
	tmpDir := t.TempDir()
	logPath := tmpDir + "/test.log"
	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set up structured log options with validators
	opts := StructuredLogOptions{
		RequiredFields:  []string{"user_id", "action"},
		ForbiddenFields: []string{"password", "secret"},
		FieldValidators: map[string]FieldValidator{
			"user_id": RequiredStringValidator,
			"age":     NumericRangeValidator(0, 150),
			"status":  EnumValidator("active", "inactive", "pending"),
		},
		DefaultFields: map[string]interface{}{
			"app_version": "1.0.0",
		},
	}
	logger.SetStructuredLogOptions(opts)

	tests := []struct {
		name    string
		fields  map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid fields",
			fields: map[string]interface{}{
				"user_id": "user123",
				"action":  "login",
				"age":     25,
				"status":  "active",
			},
			wantErr: false,
		},
		{
			name: "missing required field",
			fields: map[string]interface{}{
				"action": "login",
			},
			wantErr: true,
			errMsg:  "required field missing: user_id",
		},
		{
			name: "forbidden field",
			fields: map[string]interface{}{
				"user_id":  "user123",
				"action":   "login",
				"password": "secret123",
			},
			wantErr: true,
			errMsg:  "forbidden field: password",
		},
		{
			name: "invalid enum value",
			fields: map[string]interface{}{
				"user_id": "user123",
				"action":  "login",
				"status":  "deleted",
			},
			wantErr: true,
			errMsg:  "field status must be one of",
		},
		{
			name: "out of range numeric",
			fields: map[string]interface{}{
				"user_id": "user123",
				"action":  "login",
				"age":     200,
			},
			wantErr: true,
			errMsg:  "field age must be between",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := logger.ValidateAndNormalizeFields(tt.fields)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAndNormalizeFields() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("Expected error containing %q, got %q", tt.errMsg, err.Error())
			}
		})
	}
}

func TestFieldNormalization(t *testing.T) {
	// Create logger
	tmpDir := t.TempDir()
	logPath := tmpDir + "/test.log"
	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set up normalizers
	opts := StructuredLogOptions{
		FieldNormalizers: map[string]FieldNormalizer{
			"email":     LowercaseNormalizer,
			"username":  TrimNormalizer,
			"timestamp": TimestampNormalizer,
		},
		OmitEmptyFields: true,
	}
	logger.SetStructuredLogOptions(opts)

	fields := map[string]interface{}{
		"email":       "USER@EXAMPLE.COM",
		"username":    "  john_doe  ",
		"timestamp":   "2023-01-01 12:00:00",
		"empty_field": "",
		"null_field":  nil,
	}

	normalized, err := logger.ValidateAndNormalizeFields(fields)
	if err != nil {
		t.Fatalf("Normalization failed: %v", err)
	}

	// Check normalizations
	if normalized["email"] != "user@example.com" {
		t.Errorf("Email not lowercased: %v", normalized["email"])
	}
	if normalized["username"] != "john_doe" {
		t.Errorf("Username not trimmed: %v", normalized["username"])
	}
	
	// Check empty fields were omitted
	if _, exists := normalized["empty_field"]; exists {
		t.Error("Empty field was not omitted")
	}
	if _, exists := normalized["null_field"]; exists {
		t.Error("Null field was not omitted")
	}
}

func TestWithFields(t *testing.T) {
	// Create logger
	tmpDir := t.TempDir()
	logPath := tmpDir + "/test.log"
	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()
	logger.SetFormat(FormatJSON)

	// Create child logger with fields
	childLogger := logger.WithFields(map[string]interface{}{
		"service": "api",
		"version": "1.0",
	})

	// Add more fields
	grandchildLogger := childLogger.WithField("component", "auth")

	// Log with grandchild logger
	grandchildLogger.WithFields(map[string]interface{}{
		"user_id": "user123",
	}).Info("User authenticated")

	// Wait for processing
	logger.Sync()

	// Verify all fields are included
	content := readFile(t, logPath)
	expectedFields := []string{
		`"service":"api"`,
		`"version":"1.0"`,
		`"component":"auth"`,
		`"user_id":"user123"`,
	}

	for _, expected := range expectedFields {
		if !strings.Contains(content, expected) {
			t.Errorf("Expected field %s not found in log", expected)
		}
	}
}

func TestFieldTruncation(t *testing.T) {
	// Create logger
	tmpDir := t.TempDir()
	logPath := tmpDir + "/test.log"
	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set up truncation
	opts := StructuredLogOptions{
		MaxFieldSize:   20,
		TruncateFields: true,
	}
	logger.SetStructuredLogOptions(opts)

	longString := strings.Repeat("a", 100)
	fields := map[string]interface{}{
		"long_field": longString,
		"short_field": "short",
	}

	normalized, err := logger.ValidateAndNormalizeFields(fields)
	if err != nil {
		t.Fatalf("Normalization failed: %v", err)
	}

	// Check truncation
	truncated := normalized["long_field"].(string)
	if !strings.HasSuffix(truncated, "...(truncated)") {
		t.Error("Long field was not truncated")
	}
	if len(truncated) > 40 { // 20 + suffix
		t.Error("Truncated field is too long")
	}

	// Check short field is unchanged
	if normalized["short_field"] != "short" {
		t.Error("Short field was modified")
	}
}

func TestSortedFields(t *testing.T) {
	fields := map[string]interface{}{
		"zebra": 1,
		"alpha": 2,
		"beta":  3,
		"delta": 4,
	}

	sorted := SortedFields(fields)
	
	// Check order
	expectedOrder := []string{"alpha", "beta", "delta", "zebra"}
	for i, field := range sorted {
		if field.Key != expectedOrder[i] {
			t.Errorf("Expected key %s at position %d, got %s", expectedOrder[i], i, field.Key)
		}
	}
}

func TestOrderedFields(t *testing.T) {
	fields := map[string]interface{}{
		"name":    "John",
		"age":     30,
		"email":   "john@example.com",
		"country": "USA",
		"city":    "New York",
	}

	order := []string{"name", "email", "age"}
	ordered := OrderedFields(fields, order)

	// Check that specified fields come first in order
	if ordered[0].Key != "name" {
		t.Errorf("Expected 'name' first, got %s", ordered[0].Key)
	}
	if ordered[1].Key != "email" {
		t.Errorf("Expected 'email' second, got %s", ordered[1].Key)
	}
	if ordered[2].Key != "age" {
		t.Errorf("Expected 'age' third, got %s", ordered[2].Key)
	}

	// Check that remaining fields are present
	foundCity := false
	foundCountry := false
	for _, field := range ordered[3:] {
		if field.Key == "city" {
			foundCity = true
		}
		if field.Key == "country" {
			foundCountry = true
		}
	}
	if !foundCity || !foundCountry {
		t.Error("Remaining fields not included in ordered output")
	}
}

func TestMetadataFields(t *testing.T) {
	// Create logger
	tmpDir := t.TempDir()
	logPath := tmpDir + "/test.log"
	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()
	logger.SetFormat(FormatJSON)

	// Enable metadata
	logger.includeHostname = true
	logger.includeProcess = true
	logger.includeRuntime = true

	// Log a message
	logger.Info("Test with metadata")
	logger.Sync()

	// Verify metadata fields
	content := readFile(t, logPath)
	metadataFields := []string{
		`"hostname":`,
		`"pid":`,
		`"process":`,
		`"go_version":`,
		`"goroutines":`,
	}

	for _, field := range metadataFields {
		if !strings.Contains(content, field) {
			t.Errorf("Expected metadata field %s not found", field)
		}
	}
}