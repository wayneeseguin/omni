package flexlog

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
)

// FieldType represents the type of a structured field
type FieldType int

const (
	FieldTypeString FieldType = iota
	FieldTypeInt
	FieldTypeFloat
	FieldTypeBool
	FieldTypeTime
	FieldTypeDuration
	FieldTypeError
	FieldTypeStringer
	FieldTypeObject
	FieldTypeArray
	FieldTypeNil
)

// StructuredField represents a field with type information
type StructuredField struct {
	Key   string
	Value interface{}
	Type  FieldType
}

// FieldValidator is a function that validates a field value
type FieldValidator func(key string, value interface{}) error

// FieldNormalizer is a function that normalizes a field value
type FieldNormalizer func(key string, value interface{}) interface{}

// StructuredLogOptions contains options for structured logging
type StructuredLogOptions struct {
	// SortFields controls whether fields are sorted alphabetically
	SortFields bool

	// MaxFieldSize limits the maximum size of field values (0 = no limit)
	MaxFieldSize int

	// TruncateFields controls whether oversized fields are truncated
	TruncateFields bool

	// OmitEmptyFields controls whether fields with empty values are omitted
	OmitEmptyFields bool

	// FieldValidators contains validators for specific fields
	FieldValidators map[string]FieldValidator

	// FieldNormalizers contains normalizers for specific fields
	FieldNormalizers map[string]FieldNormalizer

	// RequiredFields contains fields that must be present
	RequiredFields []string

	// ForbiddenFields contains fields that must not be present
	ForbiddenFields []string

	// DefaultFields contains default values for fields
	DefaultFields map[string]interface{}

	// FieldOrder specifies the order of fields (if SortFields is false)
	FieldOrder []string
}

// SetStructuredLogOptions sets options for structured logging
func (f *FlexLog) SetStructuredLogOptions(opts StructuredLogOptions) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.structuredOpts = opts
}

// GetStructuredLogOptions returns the current structured logging options
func (f *FlexLog) GetStructuredLogOptions() StructuredLogOptions {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.structuredOpts
}

// ValidateAndNormalizeFields validates and normalizes structured fields
func (f *FlexLog) ValidateAndNormalizeFields(fields map[string]interface{}) (map[string]interface{}, error) {
	if fields == nil {
		fields = make(map[string]interface{})
	}

	opts := f.GetStructuredLogOptions()
	result := make(map[string]interface{})

	// Add default fields
	for k, v := range opts.DefaultFields {
		if _, exists := fields[k]; !exists {
			result[k] = v
		}
	}

	// Copy and process user fields
	for k, v := range fields {
		// Check forbidden fields
		for _, forbidden := range opts.ForbiddenFields {
			if k == forbidden {
				return nil, fmt.Errorf("forbidden field: %s", k)
			}
		}

		// Apply normalizer if exists
		if normalizer, exists := opts.FieldNormalizers[k]; exists {
			v = normalizer(k, v)
		}

		// Apply validator if exists
		if validator, exists := opts.FieldValidators[k]; exists {
			if err := validator(k, v); err != nil {
				return nil, fmt.Errorf("field validation failed for %s: %w", k, err)
			}
		}

		// Handle field size limits
		if opts.MaxFieldSize > 0 {
			v = truncateFieldValue(v, opts.MaxFieldSize, opts.TruncateFields)
		}

		// Skip empty fields if configured
		if opts.OmitEmptyFields && isEmptyValue(v) {
			continue
		}

		result[k] = v
	}

	// Check required fields
	for _, required := range opts.RequiredFields {
		if _, exists := result[required]; !exists {
			return nil, fmt.Errorf("required field missing: %s", required)
		}
	}

	return result, nil
}

// WithFields returns a new logger that will include the given fields in all log entries
func (f *FlexLog) WithFields(fields map[string]interface{}) Logger {
	// This creates a lightweight wrapper that includes fields
	return &fieldsLogger{
		logger: f,
		fields: fields,
	}
}

// WithField returns a new logger that will include the given field in all log entries
func (f *FlexLog) WithField(key string, value interface{}) Logger {
	return f.WithFields(map[string]interface{}{key: value})
}

// WithError returns a new logger that includes an error field
func (f *FlexLog) WithError(err error) Logger {
	if err == nil {
		return f
	}
	return f.WithField("error", err.Error())
}

// StructuredLogEntry creates a structured log entry with enhanced features
func (f *FlexLog) StructuredLogEntry(level int, message string, fields map[string]interface{}) (*LogEntry, error) {
	// Merge parent fields if this is a child logger
	if f.parent != nil && f.parentFields != nil {
		merged := make(map[string]interface{}, len(f.parentFields)+len(fields))
		for k, v := range f.parentFields {
			merged[k] = v
		}
		for k, v := range fields {
			merged[k] = v
		}
		fields = merged
	}

	// Validate and normalize fields
	normalizedFields, err := f.ValidateAndNormalizeFields(fields)
	if err != nil {
		return nil, err
	}

	// Create log entry
	entry := &LogEntry{
		Timestamp: f.formatTimestamp(time.Now()),
		Level:     levelToString(level),
		Message:   message,
		Fields:    normalizedFields,
	}

	// Add metadata fields
	addMetadataFields(entry, f)

	return entry, nil
}

// GetFieldType determines the type of a field value
func GetFieldType(value interface{}) FieldType {
	if value == nil {
		return FieldTypeNil
	}

	switch v := value.(type) {
	case string:
		return FieldTypeString
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return FieldTypeInt
	case float32, float64:
		return FieldTypeFloat
	case bool:
		return FieldTypeBool
	case time.Time:
		return FieldTypeTime
	case time.Duration:
		return FieldTypeDuration
	case error:
		return FieldTypeError
	case fmt.Stringer:
		return FieldTypeStringer
	default:
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Array, reflect.Slice:
			return FieldTypeArray
		case reflect.Map, reflect.Struct:
			return FieldTypeObject
		default:
			return FieldTypeString
		}
	}
}

// truncateFieldValue truncates a field value if it exceeds the maximum size
func truncateFieldValue(value interface{}, maxSize int, truncate bool) interface{} {
	switch v := value.(type) {
	case string:
		if len(v) > maxSize {
			if truncate {
				return v[:maxSize] + "...(truncated)"
			}
			return fmt.Sprintf("[string too long: %d bytes]", len(v))
		}
		return v
	case []byte:
		if len(v) > maxSize {
			if truncate {
				return string(v[:maxSize]) + "...(truncated)"
			}
			return fmt.Sprintf("[bytes too long: %d bytes]", len(v))
		}
		return v
	default:
		// For other types, convert to string and check
		str := fmt.Sprintf("%v", v)
		if len(str) > maxSize {
			if truncate {
				return str[:maxSize] + "...(truncated)"
			}
			return fmt.Sprintf("[value too long: %d bytes]", len(str))
		}
		return v
	}
}

// isEmptyValue checks if a value is considered empty
func isEmptyValue(value interface{}) bool {
	if value == nil {
		return true
	}

	switch v := value.(type) {
	case string:
		return v == ""
	case int, int8, int16, int32, int64:
		return v == 0
	case uint, uint8, uint16, uint32, uint64:
		return v == 0
	case float32, float64:
		return v == 0.0
	case bool:
		return !v
	case []interface{}:
		return len(v) == 0
	case map[string]interface{}:
		return len(v) == 0
	default:
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Array, reflect.Slice, reflect.Map:
			return rv.Len() == 0
		case reflect.Ptr, reflect.Interface:
			return rv.IsNil()
		}
		return false
	}
}

// addMetadataFields adds standard metadata fields to the log entry
func addMetadataFields(entry *LogEntry, logger *FlexLog) {
	// Add hostname if configured
	if logger.includeHostname {
		entry.Fields["hostname"] = getHostname()
	}

	// Add process info if configured
	if logger.includeProcess {
		entry.Fields["pid"] = getPID()
		entry.Fields["process"] = getProcessName()
	}

	// Add Go runtime info if configured
	if logger.includeRuntime {
		entry.Fields["go_version"] = getGoVersion()
		entry.Fields["goroutines"] = getGoroutineCount()
	}
}

// SortedFields returns fields in sorted order
func SortedFields(fields map[string]interface{}) []StructuredField {
	var result []StructuredField
	
	// Extract keys
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	
	// Sort keys
	sort.Strings(keys)
	
	// Create sorted fields
	for _, k := range keys {
		result = append(result, StructuredField{
			Key:   k,
			Value: fields[k],
			Type:  GetFieldType(fields[k]),
		})
	}
	
	return result
}

// OrderedFields returns fields in a specific order
func OrderedFields(fields map[string]interface{}, order []string) []StructuredField {
	var result []StructuredField
	seen := make(map[string]bool)
	
	// Add fields in specified order
	for _, key := range order {
		if value, exists := fields[key]; exists {
			result = append(result, StructuredField{
				Key:   key,
				Value: value,
				Type:  GetFieldType(value),
			})
			seen[key] = true
		}
	}
	
	// Add remaining fields in alphabetical order
	var remaining []string
	for k := range fields {
		if !seen[k] {
			remaining = append(remaining, k)
		}
	}
	sort.Strings(remaining)
	
	for _, k := range remaining {
		result = append(result, StructuredField{
			Key:   k,
			Value: fields[k],
			Type:  GetFieldType(fields[k]),
		})
	}
	
	return result
}

// Common field validators

// RequiredStringValidator ensures a field is a non-empty string
func RequiredStringValidator(key string, value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("field %s must be a string", key)
	}
	if strings.TrimSpace(str) == "" {
		return fmt.Errorf("field %s cannot be empty", key)
	}
	return nil
}

// NumericRangeValidator creates a validator for numeric ranges
func NumericRangeValidator(min, max float64) FieldValidator {
	return func(key string, value interface{}) error {
		var num float64
		switch v := value.(type) {
		case int:
			num = float64(v)
		case int64:
			num = float64(v)
		case float64:
			num = v
		case float32:
			num = float64(v)
		default:
			return fmt.Errorf("field %s must be numeric", key)
		}
		
		if num < min || num > max {
			return fmt.Errorf("field %s must be between %f and %f", key, min, max)
		}
		return nil
	}
}

// EnumValidator creates a validator for enum values
func EnumValidator(validValues ...string) FieldValidator {
	validSet := make(map[string]bool)
	for _, v := range validValues {
		validSet[v] = true
	}
	
	return func(key string, value interface{}) error {
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("field %s must be a string", key)
		}
		if !validSet[str] {
			return fmt.Errorf("field %s must be one of: %v", key, validValues)
		}
		return nil
	}
}

// Common field normalizers

// LowercaseNormalizer converts string values to lowercase
func LowercaseNormalizer(key string, value interface{}) interface{} {
	if str, ok := value.(string); ok {
		return strings.ToLower(str)
	}
	return value
}

// TrimNormalizer trims whitespace from string values
func TrimNormalizer(key string, value interface{}) interface{} {
	if str, ok := value.(string); ok {
		return strings.TrimSpace(str)
	}
	return value
}

// TimestampNormalizer converts various time formats to ISO8601
func TimestampNormalizer(key string, value interface{}) interface{} {
	switch v := value.(type) {
	case time.Time:
		return v.Format(time.RFC3339)
	case string:
		// Try to parse common formats
		for _, format := range []string{
			time.RFC3339,
			time.RFC1123,
			"2006-01-02 15:04:05",
			"2006-01-02",
		} {
			if t, err := time.Parse(format, v); err == nil {
				return t.Format(time.RFC3339)
			}
		}
		return v
	default:
		return value
	}
}