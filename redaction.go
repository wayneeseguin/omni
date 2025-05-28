package flexlog

import (
	"encoding/json"
	"regexp"
	"strings"
)

var sensitiveKeywords = []string{
	"auth_token", "password", "secret", "key", "private_key", "token", "access_token",
}

// Sensitive patterns to redact for non-JSON fields
var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`("auth_token"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("password"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("secret"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("key"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("private_key"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("token"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`(Authorization:[ \t]*Bearer[ \t]+)[^ \t\n\r]+`),
}

// JSON field patterns (the first 6 patterns in sensitivePatterns)
var jsonFieldPatterns = sensitivePatterns[:6]

// Other sensitive patterns (the Bearer pattern)
var otherPatterns = sensitivePatterns[6:]

// LogRequest logs an API request safely
func (f *FlexLog) LogRequest(method, path string, headers map[string][]string, body string) {
	// Format string with placeholders for all values
	format := "%s %s"
	args := []interface{}{method, path}

	// Add headers with explicit keys and values to ensure they appear in output
	for k, v := range headers {
		format += "\n%s: %s"
		headerValue := v[0]
		if strings.ToLower(k) == "authorization" ||
			strings.Contains(strings.ToLower(k), "token") ||
			strings.Contains(strings.ToLower(k), "key") {
			headerValue = "[REDACTED]"
		}
		args = append(args, k, headerValue)
	}

	// Add body
	format += "\n%s"
	args = append(args, f.redactSensitive(body))

	// Log with format and arguments
	f.logf(LevelInfo, format, args...)
}

// LogResponse logs an API response safely
func (f *FlexLog) LogResponse(statusCode int, headers map[string][]string, body string) {
	// Format string with placeholders for all values
	format := "Status: %d"
	args := []interface{}{statusCode}

	// Add headers with explicit keys and values to ensure they appear in output
	for k, v := range headers {
		format += "\n%s: %s"
		headerValue := v[0]
		if strings.Contains(strings.ToLower(k), "token") ||
			strings.Contains(strings.ToLower(k), "key") {
			headerValue = "[REDACTED]"
		}
		args = append(args, k, headerValue)
	}

	// Add body
	format += "\n%s"
	args = append(args, f.redactSensitive(body))

	// Log with format and arguments
	f.logf(LevelInfo, format, args...)
}

// redactSensitive replaces sensitive information with [REDACTED]
func (f *FlexLog) redactSensitive(input string) string {
	if input == "" {
		return input
	}

	var data interface{}
	err := json.Unmarshal([]byte(input), &data)
	if err != nil {
		f.Debugf("Falling back to regex, unmarshal error: %v", err) // ✅ ADD THIS
		return f.regexRedact(input)
	}

	f.recursiveRedact(data)

	redacted, err := json.Marshal(data)
	if err != nil {
		f.Debugf("Failed to marshal redacted JSON: %v", err) // ✅ ADD THIS
		return input
	}
	return string(redacted)
}

// recursiveRedact walks the JSON structure and redacts sensitive values
func (f *FlexLog) recursiveRedact(v interface{}) {
	switch val := v.(type) {
	case map[string]interface{}:
		for k, v2 := range val {
			if isSensitiveKey(k) {
				f.Debugf("Redacting sensitive key: %s", k)
				val[k] = "[REDACTED]"
			} else {
				f.recursiveRedact(v2)
			}
		}
	case []interface{}:
		for i, item := range val {
			// 🔧 This is the fix: re-assign any redacted structure
			switch itemVal := item.(type) {
			case map[string]interface{}, []interface{}:
				f.recursiveRedact(itemVal)
				val[i] = itemVal
			default:
				// do nothing for primitives
			}
		}
	}

}

// regexRedact applies fallback regex-based redaction on raw text
func (f *FlexLog) regexRedact(input string) string {
	result := input
	for _, pattern := range sensitivePatterns {
		if strings.Contains(pattern.String(), "\"[^\"]*\"") {
			result = pattern.ReplaceAllString(result, `${1}"[REDACTED]"`)
		} else {
			result = pattern.ReplaceAllString(result, `${1}[REDACTED]`)
		}
	}
	return result
}

// isSensitiveKey checks if a key is considered sensitive
func isSensitiveKey(key string) bool {
	k := strings.ToLower(key)
	for _, sensitive := range sensitiveKeywords {
		if strings.Contains(k, sensitive) {
			return true
		}
	}
	return false
}

// Redactor handles pattern-based redaction
type Redactor struct {
	patterns []*regexp.Regexp
	replace  string
}

// NewRedactor creates a new redactor with custom patterns
func NewRedactor(patterns []string, replace string) (*Redactor, error) {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}
		compiled = append(compiled, re)
	}

	return &Redactor{
		patterns: compiled,
		replace:  replace,
	}, nil
}

// Redact applies redaction patterns to a string
func (r *Redactor) Redact(input string) string {
	result := input
	for _, pattern := range r.patterns {
		result = pattern.ReplaceAllString(result, r.replace)
	}
	return result
}

// SetRedaction sets custom redaction patterns for the logger
func (f *FlexLog) SetRedaction(patterns []string, replace string) error {
	redactor, err := NewRedactor(patterns, replace)
	if err != nil {
		return err
	}

	f.mu.Lock()
	f.redactor = redactor
	f.redactionPatterns = patterns
	f.redactionReplace = replace
	f.mu.Unlock()

	return nil
}
