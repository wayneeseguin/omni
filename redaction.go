package omni

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// sensitiveKeywords contains field names that should be redacted
var sensitiveKeywords = []string{
	"auth_token", "password", "passwd", "pass", "secret", "key", "private_key", "token", 
	"access_token", "refresh_token", "api_key", "apikey", "authorization",
	"client_secret", "client_id", "session_token", "bearer", "oauth", "jwt",
	"ssn", "social_security", "credit_card", "creditcard", "card_number", "cvv", "cvc",
	"email", "phone", "mobile", "telephone", "address", "zip", "postal",
}

// builtInDataPatterns contains regex patterns for common sensitive data types
var builtInDataPatterns = []*regexp.Regexp{
	// US Social Security Numbers
	regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
	regexp.MustCompile(`\b\d{3}\s\d{2}\s\d{4}\b`),
	regexp.MustCompile(`\b\d{9}\b`),
	
	// Credit Card Numbers (major brands)
	regexp.MustCompile(`\b4\d{3}[-\s]?\d{4}[-\s]?\d{4}[-\s]?\d{4}\b`), // Visa
	regexp.MustCompile(`\b5[1-5]\d{2}[-\s]?\d{4}[-\s]?\d{4}[-\s]?\d{4}\b`), // MasterCard
	regexp.MustCompile(`\b3[47]\d{2}[-\s]?\d{6}[-\s]?\d{5}\b`), // American Express
	regexp.MustCompile(`\b6011[-\s]?\d{4}[-\s]?\d{4}[-\s]?\d{4}\b`), // Discover
	
	// Email Addresses
	regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`),
	
	// Phone Numbers (US format)
	regexp.MustCompile(`\b\d{3}[-.]?\d{3}[-.]?\d{4}\b`),
	regexp.MustCompile(`\(\d{3}\)\s*\d{3}[-.]?\d{4}\b`),
	
	// Common API Key Formats
	regexp.MustCompile(`\bsk-[a-zA-Z0-9]{48}\b`), // OpenAI
	regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`), // AWS Access Key
	regexp.MustCompile(`\bghp_[a-zA-Z0-9]{36,40}\b`), // GitHub Personal Access Token
	regexp.MustCompile(`\bxoxb-[0-9]{10,13}-[0-9]{10,13}-[a-zA-Z0-9]{24}\b`), // Slack Bot Token
	
	// Common Secrets
	regexp.MustCompile(`\b[A-Za-z0-9+/]{40,}={0,2}\b`), // Base64 encoded secrets (40+ chars)
	regexp.MustCompile(`\b[a-f0-9]{32}\b`), // MD5 hashes (often used as tokens)
	regexp.MustCompile(`\b[a-f0-9]{64}\b`), // SHA256 hashes
}

// sensitivePatterns contains regex patterns to redact sensitive data in JSON fields
var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`("auth_token"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("password"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("passwd"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("pass"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("secret"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("key"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("private_key"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("token"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("access_token"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("refresh_token"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("api_key"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("apikey"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("authorization"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("client_secret"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("session_token"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("bearer"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("oauth"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("jwt"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("ssn"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("social_security"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("credit_card"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("creditcard"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("card_number"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("cvv"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("cvc"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("email"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("phone"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("mobile"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("telephone"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`(Authorization:[ \t]*Bearer[ \t]+)[^ \t\n\r]+`),
}

// jsonFieldPatterns contains patterns for JSON field redaction (all except the last Authorization header pattern)
var jsonFieldPatterns = sensitivePatterns[:len(sensitivePatterns)-1]

// otherPatterns contains non-JSON sensitive patterns (the Authorization header pattern)
var otherPatterns = sensitivePatterns[len(sensitivePatterns)-1:]

// LogRequest logs an API request with automatic redaction of sensitive data.
// It redacts authorization headers and sensitive fields in the request body.
//
// Parameters:
//   - method: HTTP method (GET, POST, etc.)
//   - path: Request path
//   - headers: Request headers
//   - body: Request body
//
// Example:
//
//	logger.LogRequest("POST", "/api/login", headers, body)
func (f *Omni) LogRequest(method, path string, headers map[string][]string, body string) {
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

// LogResponse logs an API response with automatic redaction of sensitive data.
// It redacts sensitive headers and fields in the response body.
//
// Parameters:
//   - statusCode: HTTP status code
//   - headers: Response headers
//   - body: Response body
//
// Example:
//
//	logger.LogResponse(200, responseHeaders, responseBody)
func (f *Omni) LogResponse(statusCode int, headers map[string][]string, body string) {
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

// redactSensitive replaces sensitive information with [REDACTED].
// It uses optimized redaction with configurable options and lazy JSON parsing.
//
// Parameters:
//   - input: The string to redact
//
// Returns:
//   - string: The redacted string
func (f *Omni) redactSensitive(input string) string {
	return f.redactSensitiveWithLevel(input, LevelInfo)
}

// redactSensitiveWithLevel replaces sensitive information with level-aware redaction.
// It checks if redaction should be skipped for the given level.
//
// Parameters:
//   - input: The string to redact
//   - level: The log level for this message
//
// Returns:
//   - string: The redacted string
func (f *Omni) redactSensitiveWithLevel(input string, level int) string {
	if input == "" {
		return input
	}

	// Check if redaction is configured to skip this level
	f.mu.RLock()
	config := f.redactionConfig
	f.mu.RUnlock()
	
	if config != nil {
		for _, skipLevel := range config.SkipLevels {
			if level == skipLevel {
				return input // Skip redaction for this level
			}
		}
	}

	// Try to check if this looks like JSON before parsing (performance optimization)
	trimmed := strings.TrimSpace(input)
	isLikelyJSON := (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
		(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]"))

	if isLikelyJSON {
		var data interface{}
		err := json.Unmarshal([]byte(input), &data)
		if err == nil {
			f.recursiveRedact(data)
			redacted, err := json.Marshal(data)
			if err == nil {
				return string(redacted)
			}
		}
	}

	// Fall back to regex-based redaction for non-JSON content
	return f.regexRedact(input)
}

// recursiveRedact walks the JSON structure and redacts sensitive values.
// It recursively processes maps and arrays to find and redact sensitive fields.
// Also handles field path-based redaction.
//
// Parameters:
//   - v: The value to process (can be map, slice, or other types)
func (f *Omni) recursiveRedact(v interface{}) {
	f.recursiveRedactWithPath(v, "")
}

// recursiveRedactWithPath walks the JSON structure with path tracking for field-based redaction.
//
// Parameters:
//   - v: The value to process (can be map, slice, or other types)
//   - currentPath: The current JSON path (e.g., "user.profile")
func (f *Omni) recursiveRedactWithPath(v interface{}, currentPath string) {
	switch val := v.(type) {
	case map[string]interface{}:
		for k, v2 := range val {
			// Build the current field path
			fieldPath := k
			if currentPath != "" {
				fieldPath = currentPath + "." + k
			}
			
			// Check if this specific path should be redacted
			if f.shouldRedactPath(fieldPath) {
				val[k] = f.getReplacementForPath(fieldPath)
			} else if str, ok := v2.(string); ok {
				// Apply custom redaction patterns first to string values
				f.mu.RLock()
				redactor := f.redactor
				f.mu.RUnlock()
				redacted := str
				if redactor != nil {
					redacted = redactor.Redact(str)
				}
				
				// If no custom pattern matched but field name is sensitive, use built-in redaction
				if redacted == str && isSensitiveKey(k) {
					redacted = "[REDACTED]"
				}
				
				// Update the value if it was redacted
				if redacted != str {
					val[k] = redacted
				}
				
				// Continue recursively even for strings in case of nested objects
				f.recursiveRedactWithPath(v2, fieldPath)
			} else if isSensitiveKey(k) {
				// Fall back to keyword-based redaction for non-string values
				val[k] = "[REDACTED]"
			} else {
				// Continue recursively
				f.recursiveRedactWithPath(v2, fieldPath)
			}
		}
	case []interface{}:
		for i, item := range val {
			// For arrays, use index in path like "users[0]" but also support wildcard matching
			indexPath := currentPath + "[" + fmt.Sprintf("%d", i) + "]"
			wildcardPath := currentPath + "[*]"
			
			switch itemVal := item.(type) {
			case map[string]interface{}, []interface{}:
				f.recursiveRedactWithPath(itemVal, indexPath)
				val[i] = itemVal
			default:
				// Check if the array element path should be redacted
				if f.shouldRedactPath(indexPath) || f.shouldRedactPath(wildcardPath) {
					val[i] = f.getReplacementForPath(indexPath)
				} else if str, ok := item.(string); ok {
					// Apply custom redaction patterns to string values in arrays
					f.mu.RLock()
					redactor := f.redactor
					f.mu.RUnlock()
					if redactor != nil {
						redacted := redactor.Redact(str)
						if redacted != str {
							val[i] = redacted
						}
					}
				}
			}
		}
	}
}

// shouldRedactPath checks if a specific field path should be redacted
func (f *Omni) shouldRedactPath(path string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	// Check configured field paths
	if f.redactionConfig != nil {
		for _, configPath := range f.redactionConfig.FieldPaths {
			if matchesPath(path, configPath) {
				return true
			}
		}
	}
	
	// Check field path rules
	for _, rule := range f.fieldPathRules {
		if matchesPath(path, rule.Path) {
			return true
		}
	}
	
	return false
}

// getReplacementForPath gets the replacement text for a specific field path
func (f *Omni) getReplacementForPath(path string) string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	// Check field path rules for custom replacement
	for _, rule := range f.fieldPathRules {
		if matchesPath(path, rule.Path) {
			return rule.Replacement
		}
	}
	
	return "[REDACTED]" // Default replacement
}

// matchesPath checks if a field path matches a pattern (supports wildcards)
func matchesPath(path, pattern string) bool {
	// Simple exact match for now
	if path == pattern {
		return true
	}
	
	// Support wildcard matching (basic implementation)
	if strings.Contains(pattern, "*") {
		// Convert pattern to regex-like matching
		regexPattern := strings.ReplaceAll(pattern, "*", ".*")
		matched, _ := regexp.MatchString("^"+regexPattern+"$", path)
		return matched
	}
	
	return false
}

// regexRedact applies fallback regex-based redaction on raw text.
// Used when JSON parsing fails or for non-JSON content.
//
// Parameters:
//   - input: The string to redact
//
// Returns:
//   - string: The redacted string
func (f *Omni) regexRedact(input string) string {
	result := input
	
	// Apply JSON field patterns first
	for _, pattern := range sensitivePatterns {
		if strings.Contains(pattern.String(), "\"[^\"]*\"") {
			result = pattern.ReplaceAllString(result, `${1}"[REDACTED]"`)
		} else {
			result = pattern.ReplaceAllString(result, `${1}[REDACTED]`)
		}
	}
	
	// Apply built-in data patterns for content redaction
	for _, pattern := range builtInDataPatterns {
		result = pattern.ReplaceAllString(result, "[REDACTED]")
	}
	
	// Also apply simple key=value patterns for common formats
	keyValuePatterns := []*regexp.Regexp{
		regexp.MustCompile(`\bpassword\s*=\s*\S+`),
		regexp.MustCompile(`\bapi_key\s*=\s*\S+`),
		regexp.MustCompile(`\btoken\s*=\s*\S+`),
		regexp.MustCompile(`\bsecret\s*=\s*\S+`),
	}
	
	for _, pattern := range keyValuePatterns {
		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			parts := strings.Split(match, "=")
			if len(parts) >= 2 {
				return parts[0] + "=[REDACTED]"
			}
			return "[REDACTED]"
		})
	}
	
	// Apply custom redactor if set
	f.mu.RLock()
	if f.redactor != nil {
		result = f.redactor.Redact(result)
	}
	f.mu.RUnlock()
	
	return result
}

// isSensitiveKey checks if a key is considered sensitive.
// It performs case-insensitive matching against known sensitive keywords.
//
// Parameters:
//   - key: The field name to check
//
// Returns:
//   - bool: true if the key is sensitive
func isSensitiveKey(key string) bool {
	k := strings.ToLower(key)
	for _, sensitive := range sensitiveKeywords {
		if strings.Contains(k, sensitive) {
			return true
		}
	}
	return false
}

// Redactor handles pattern-based redaction with performance optimizations.
// It applies a set of regex patterns to replace sensitive data.
type Redactor struct {
	patterns []*regexp.Regexp
	replace  string
	mu       sync.RWMutex
	cache    map[string]string // Cache for compiled patterns to avoid repeated compilation
}

// RedactionConfig holds configuration for redaction behavior
type RedactionConfig struct {
	EnableBuiltInPatterns bool     // Whether to apply built-in data patterns (SSN, credit cards, etc.)
	EnableFieldRedaction  bool     // Whether to apply JSON field redaction
	EnableDataPatterns    bool     // Whether to apply data pattern redaction in content
	MaxCacheSize         int      // Maximum size of redaction cache
	SkipLevels           []int    // Log levels to skip redaction for (e.g., DEBUG)
	FieldPaths           []string // Specific field paths to redact (e.g., "user.profile.ssn")
}

// FieldPathRule defines a rule for redacting specific field paths
type FieldPathRule struct {
	Path        string // JSON path like "user.profile.ssn"
	Replacement string // Custom replacement text for this path
}

// NewRedactor creates a new redactor with custom patterns and performance optimizations.
//
// Parameters:
//   - patterns: Array of regex patterns to match sensitive data
//   - replace: The replacement string (e.g., "[REDACTED]")
//
// Returns:
//   - *Redactor: The configured redactor
//   - error: If any pattern fails to compile
//
// Example:
//
//	redactor, err := NewRedactor([]string{
//	    `\b\d{3}-\d{2}-\d{4}\b`,  // SSN pattern
//	    `\b\d{16}\b`,             // Credit card pattern
//	}, "[REDACTED]")
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
		cache:    make(map[string]string),
	}, nil
}

// Redact applies redaction patterns to a string with caching for performance.
//
// Parameters:
//   - input: The string to redact
//
// Returns:
//   - string: The redacted string with patterns replaced
func (r *Redactor) Redact(input string) string {
	// Check cache first for performance
	r.mu.RLock()
	if cached, exists := r.cache[input]; exists {
		r.mu.RUnlock()
		return cached
	}
	r.mu.RUnlock()
	
	// Apply redaction patterns
	result := input
	for _, pattern := range r.patterns {
		result = pattern.ReplaceAllString(result, r.replace)
	}
	
	// Cache the result (with size limit to prevent memory leaks)
	r.mu.Lock()
	if len(r.cache) < 1000 { // Limit cache size
		r.cache[input] = result
	}
	r.mu.Unlock()
	
	return result
}

// ClearCache clears the redaction cache to free memory
func (r *Redactor) ClearCache() {
	r.mu.Lock()
	r.cache = make(map[string]string)
	r.mu.Unlock()
}

// SetRedaction sets custom redaction patterns for the logger.
// These patterns will be applied to all log messages to remove sensitive data.
//
// Parameters:
//   - patterns: Array of regex patterns to match sensitive data
//   - replace: The replacement string
//
// Returns:
//   - error: If any pattern fails to compile
//
// Example:
//
//	logger.SetRedaction([]string{
//	    `password=\S+`,           // Redact password parameters
//	    `api_key:\s*"[^"]+"`      // Redact API keys
//	}, "[REDACTED]")
func (f *Omni) SetRedaction(patterns []string, replace string) error {
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

// SetRedactionConfig sets the redaction configuration with advanced options.
//
// Parameters:
//   - config: RedactionConfig with various redaction behavior settings
//
// Example:
//
//	logger.SetRedactionConfig(&RedactionConfig{
//	    EnableBuiltInPatterns: true,
//	    EnableFieldRedaction: true,
//	    EnableDataPatterns: true,
//	    SkipLevels: []int{LevelDebug}, // Don't redact debug logs
//	    MaxCacheSize: 5000,
//	})
func (f *Omni) SetRedactionConfig(config *RedactionConfig) {
	f.mu.Lock()
	f.redactionConfig = config
	f.mu.Unlock()
}

// GetRedactionConfig returns the current redaction configuration.
//
// Returns:
//   - *RedactionConfig: The current redaction configuration or nil if not set
func (f *Omni) GetRedactionConfig() *RedactionConfig {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.redactionConfig
}

// EnableRedactionForLevel enables or disables redaction for a specific log level.
//
// Parameters:
//   - level: The log level to configure
//   - enable: Whether to enable redaction for this level
func (f *Omni) EnableRedactionForLevel(level int, enable bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	if f.redactionConfig == nil {
		f.redactionConfig = &RedactionConfig{
			EnableBuiltInPatterns: true,
			EnableFieldRedaction:  true,
			EnableDataPatterns:    true,
			MaxCacheSize:         1000,
		}
	}
	
	if enable {
		// Remove level from skip list if it exists
		newSkipLevels := make([]int, 0, len(f.redactionConfig.SkipLevels))
		for _, skipLevel := range f.redactionConfig.SkipLevels {
			if skipLevel != level {
				newSkipLevels = append(newSkipLevels, skipLevel)
			}
		}
		f.redactionConfig.SkipLevels = newSkipLevels
	} else {
		// Add level to skip list if not already there
		found := false
		for _, skipLevel := range f.redactionConfig.SkipLevels {
			if skipLevel == level {
				found = true
				break
			}
		}
		if !found {
			f.redactionConfig.SkipLevels = append(f.redactionConfig.SkipLevels, level)
		}
	}
}

// ClearRedactionCache clears the redaction cache to free memory.
func (f *Omni) ClearRedactionCache() {
	f.mu.RLock()
	redactor := f.redactor
	f.mu.RUnlock()
	
	if redactor != nil {
		redactor.ClearCache()
	}
}

// AddFieldPathRule adds a field path rule for targeted redaction.
//
// Parameters:
//   - path: JSON field path like "user.profile.ssn" or "users.*.email"
//   - replacement: Custom replacement text for this path
//
// Example:
//
//	logger.AddFieldPathRule("user.profile.ssn", "[SSN-REDACTED]")
//	logger.AddFieldPathRule("users.*.email", "[EMAIL-REDACTED]")
func (f *Omni) AddFieldPathRule(path, replacement string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	// Check if rule already exists and update it
	for i, rule := range f.fieldPathRules {
		if rule.Path == path {
			f.fieldPathRules[i].Replacement = replacement
			return
		}
	}
	
	// Add new rule
	f.fieldPathRules = append(f.fieldPathRules, FieldPathRule{
		Path:        path,
		Replacement: replacement,
	})
}

// RemoveFieldPathRule removes a field path rule.
//
// Parameters:
//   - path: The field path to remove
func (f *Omni) RemoveFieldPathRule(path string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	for i, rule := range f.fieldPathRules {
		if rule.Path == path {
			// Remove the rule by slicing
			f.fieldPathRules = append(f.fieldPathRules[:i], f.fieldPathRules[i+1:]...)
			break
		}
	}
}

// GetFieldPathRules returns all configured field path rules.
//
// Returns:
//   - []FieldPathRule: A copy of all field path rules
func (f *Omni) GetFieldPathRules() []FieldPathRule {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	// Return a copy to prevent external modification
	rules := make([]FieldPathRule, len(f.fieldPathRules))
	copy(rules, f.fieldPathRules)
	return rules
}

// ClearFieldPathRules removes all field path rules.
func (f *Omni) ClearFieldPathRules() {
	f.mu.Lock()
	f.fieldPathRules = nil
	f.mu.Unlock()
}
