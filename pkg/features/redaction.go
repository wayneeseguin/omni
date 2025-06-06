package features

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
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

// Redactor handles pattern-based redaction with performance optimizations.
// It applies a set of regex patterns to replace sensitive data.
type Redactor struct {
	patterns []*regexp.Regexp
	replace  string
	mu       sync.RWMutex
	cache    map[string]string // Cache for compiled patterns to avoid repeated compilation
}

// RedactionManager provides comprehensive redaction management
type RedactionManager struct {
	mu                sync.RWMutex
	config            *RedactionConfig
	customRedactor    *Redactor
	fieldPathRules    []FieldPathRule
	errorHandler      func(source, dest, msg string, err error)
	metricsHandler    func(string)
	metrics           *RedactionMetrics
	cache             *RedactionCache
	hashSalt          string // For consistent hashing
	preserveStructure bool   // Whether to preserve value structure
	contextualRules   map[string]ContextualRule
}

// RedactionMetrics tracks redaction statistics
type RedactionMetrics struct {
	TotalProcessed     uint64
	TotalRedacted      uint64
	FieldsRedacted     map[string]uint64
	PatternsMatched    map[string]uint64
	ProcessingTimeNs   int64
	CacheHits          uint64
	CacheMisses        uint64
	LastUpdate         time.Time
}

// RedactionCache provides caching for redaction operations
type RedactionCache struct {
	mu       sync.RWMutex
	cache    map[string]string
	maxSize  int
	ttl      time.Duration
	hits     uint64
	misses   uint64
	evictions uint64
}

// ContextualRule defines context-aware redaction rules
type ContextualRule struct {
	Name        string
	Condition   func(level int, fields map[string]interface{}) bool
	RedactFields []string
	Replacement string
}

// RedactionMode defines how redaction is performed
type RedactionMode int

const (
	// RedactionModeReplace replaces sensitive data with placeholder
	RedactionModeReplace RedactionMode = iota
	// RedactionModeHash replaces with consistent hash
	RedactionModeHash
	// RedactionModeMask partially masks the value
	RedactionModeMask
	// RedactionModeRemove removes the field entirely
	RedactionModeRemove
)

// LogRequest logs an API request with automatic redaction of sensitive data.
// It redacts authorization headers and sensitive fields in the request body.
//
// Note: This function provides the interface but the actual implementation
// should be in the omni package where private fields can be accessed.
//
// Parameters:
//   - logger: The Omni logger instance (interface{})
//   - method: HTTP method (GET, POST, etc.)
//   - path: Request path
//   - headers: Request headers
//   - body: Request body
//
// Example:
//
//	features.LogRequest(logger, "POST", "/api/login", headers, body)
func LogRequest(logger interface{}, method, path string, headers map[string][]string, body string) {
	// Note: This function should be implemented in the omni package
	// where private fields can be accessed
}

// LogResponse logs an API response with automatic redaction of sensitive data.
// It redacts sensitive headers and fields in the response body.
//
// Note: This function provides the interface but the actual implementation
// should be in the omni package where private fields can be accessed.
//
// Parameters:
//   - logger: The Omni logger instance (interface{})
//   - statusCode: HTTP status code
//   - headers: Response headers
//   - body: Response body
//
// Example:
//
//	features.LogResponse(logger, 200, responseHeaders, responseBody)
func LogResponse(logger interface{}, statusCode int, headers map[string][]string, body string) {
	// Note: This function should be implemented in the omni package
	// where private fields can be accessed
}

// RedactSensitive replaces sensitive information with [REDACTED].
// It uses optimized redaction with configurable options and lazy JSON parsing.
//
// Parameters:
//   - input: The string to redact
//   - config: Redaction configuration
//   - redactor: Custom redactor instance
//   - fieldPathRules: Field path rules for targeted redaction
//
// Returns:
//   - string: The redacted string
func RedactSensitive(input string, config *RedactionConfig, redactor *Redactor, fieldPathRules []FieldPathRule) string {
	// Default to INFO level (2) when no level is specified
	return RedactSensitiveWithLevel(input, 2, config, redactor, fieldPathRules)
}

// RedactSensitiveWithLevel replaces sensitive information with level-aware redaction.
// It checks if redaction should be skipped for the given level.
//
// Parameters:
//   - input: The string to redact
//   - level: The log level for this message
//   - config: Redaction configuration
//   - redactor: Custom redactor instance
//   - fieldPathRules: Field path rules for targeted redaction
//
// Returns:
//   - string: The redacted string
func RedactSensitiveWithLevel(input string, level int, config *RedactionConfig, redactor *Redactor, fieldPathRules []FieldPathRule) string {
	if input == "" {
		return input
	}

	// Check if redaction is configured to skip this level
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
			RecursiveRedact(data, "", redactor, fieldPathRules)
			redacted, err := json.Marshal(data)
			if err == nil {
				return string(redacted)
			}
		}
	}

	// Fall back to regex-based redaction for non-JSON content
	return RegexRedact(input, redactor)
}

// RecursiveRedact walks the JSON structure and redacts sensitive values.
// It recursively processes maps and arrays to find and redact sensitive fields.
// Also handles field path-based redaction.
//
// Parameters:
//   - v: The value to process (can be map, slice, or other types)
//   - currentPath: The current JSON path (e.g., "user.profile")
//   - redactor: Custom redactor instance
//   - fieldPathRules: Field path rules for targeted redaction
func RecursiveRedact(v interface{}, currentPath string, redactor *Redactor, fieldPathRules []FieldPathRule) {
	switch val := v.(type) {
	case map[string]interface{}:
		for k, v2 := range val {
			// Build the current field path
			fieldPath := k
			if currentPath != "" {
				fieldPath = currentPath + "." + k
			}
			
			// Check if this specific path should be redacted
			if ShouldRedactPath(fieldPath, fieldPathRules) {
				val[k] = GetReplacementForPath(fieldPath, fieldPathRules)
			} else if str, ok := v2.(string); ok {
				// Apply custom redaction patterns first to string values
				redacted := str
				if redactor != nil {
					redacted = redactor.Redact(str)
				}
				
				// If no custom pattern matched but field name is sensitive, use built-in redaction
				if redacted == str && IsSensitiveKey(k) {
					redacted = "[REDACTED]"
				}
				
				// Update the value if it was redacted
				if redacted != str {
					val[k] = redacted
				}
				
				// Continue recursively even for strings in case of nested objects
				RecursiveRedact(v2, fieldPath, redactor, fieldPathRules)
			} else if IsSensitiveKey(k) {
				// Fall back to keyword-based redaction for non-string values
				val[k] = "[REDACTED]"
			} else {
				// Continue recursively
				RecursiveRedact(v2, fieldPath, redactor, fieldPathRules)
			}
		}
	case []interface{}:
		for i, item := range val {
			// For arrays, use index in path like "users[0]" but also support wildcard matching
			indexPath := currentPath + "[" + fmt.Sprintf("%d", i) + "]"
			wildcardPath := currentPath + "[*]"
			
			switch itemVal := item.(type) {
			case map[string]interface{}, []interface{}:
				RecursiveRedact(itemVal, indexPath, redactor, fieldPathRules)
				val[i] = itemVal
			default:
				// Check if the array element path should be redacted
				if ShouldRedactPath(indexPath, fieldPathRules) || ShouldRedactPath(wildcardPath, fieldPathRules) {
					val[i] = GetReplacementForPath(indexPath, fieldPathRules)
				} else if str, ok := item.(string); ok {
					// Apply custom redaction patterns to string values in arrays
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

// ShouldRedactPath checks if a specific field path should be redacted
func ShouldRedactPath(path string, fieldPathRules []FieldPathRule) bool {
	// Check field path rules
	for _, rule := range fieldPathRules {
		if MatchesPath(path, rule.Path) {
			return true
		}
	}
	
	return false
}

// GetReplacementForPath gets the replacement text for a specific field path
func GetReplacementForPath(path string, fieldPathRules []FieldPathRule) string {
	// Check field path rules for custom replacement
	for _, rule := range fieldPathRules {
		if MatchesPath(path, rule.Path) {
			return rule.Replacement
		}
	}
	
	return "[REDACTED]" // Default replacement
}

// MatchesPath checks if a field path matches a pattern (supports wildcards)
func MatchesPath(path, pattern string) bool {
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

// RegexRedact applies fallback regex-based redaction on raw text.
// Used when JSON parsing fails or for non-JSON content.
//
// Parameters:
//   - input: The string to redact
//   - redactor: Custom redactor instance
//
// Returns:
//   - string: The redacted string
func RegexRedact(input string, redactor *Redactor) string {
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
	if redactor != nil {
		result = redactor.Redact(result)
	}
	
	return result
}

// IsSensitiveKey checks if a key is considered sensitive.
// It performs case-insensitive matching against known sensitive keywords.
//
// Parameters:
//   - key: The field name to check
//
// Returns:
//   - bool: true if the key is sensitive
func IsSensitiveKey(key string) bool {
	k := strings.ToLower(key)
	for _, sensitive := range sensitiveKeywords {
		if strings.Contains(k, sensitive) {
			return true
		}
	}
	return false
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
//	redactor, err := features.NewRedactor([]string{
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
// Note: This function provides the interface but the actual implementation
// should be in the omni package where private fields can be accessed.
//
// Parameters:
//   - logger: The Omni logger instance (interface{})
//   - patterns: Array of regex patterns to match sensitive data
//   - replace: The replacement string
//
// Returns:
//   - error: If any pattern fails to compile
//
// Example:
//
//	features.SetRedaction(logger, []string{
//	    `password=\S+`,           // Redact password parameters
//	    `api_key:\s*"[^"]+"`      // Redact API keys
//	}, "[REDACTED]")
func SetRedaction(logger interface{}, patterns []string, replace string) error {
	_, err := NewRedactor(patterns, replace)
	if err != nil {
		return err
	}

	// Note: This function should be implemented in the omni package
	// where private fields can be accessed
	return nil
}

// SetRedactionConfig sets the redaction configuration with advanced options.
//
// Note: This function provides the interface but the actual implementation
// should be in the omni package where private fields can be accessed.
//
// Parameters:
//   - logger: The Omni logger instance (interface{})
//   - config: RedactionConfig with various redaction behavior settings
//
// Example:
//
//	features.SetRedactionConfig(logger, &features.RedactionConfig{
//	    EnableBuiltInPatterns: true,
//	    EnableFieldRedaction: true,
//	    EnableDataPatterns: true,
//	    SkipLevels: []int{0}, // Don't redact debug logs (level 0)
//	    MaxCacheSize: 5000,
//	})
func SetRedactionConfig(logger interface{}, config *RedactionConfig) {
	// Note: This function should be implemented in the omni package
	// where private fields can be accessed
}

// GetRedactionConfig returns the current redaction configuration.
//
// Note: This function provides the interface but the actual implementation
// should be in the omni package where private fields can be accessed.
//
// Parameters:
//   - logger: The Omni logger instance (interface{})
//
// Returns:
//   - *RedactionConfig: The current redaction configuration or nil if not set
func GetRedactionConfig(logger interface{}) *RedactionConfig {
	// Note: This function should be implemented in the omni package
	// where private fields can be accessed
	return nil
}

// EnableRedactionForLevel enables or disables redaction for a specific log level.
//
// Note: This function provides the interface but the actual implementation
// should be in the omni package where private fields can be accessed.
//
// Parameters:
//   - logger: The Omni logger instance (interface{})
//   - level: The log level to configure
//   - enable: Whether to enable redaction for this level
func EnableRedactionForLevel(logger interface{}, level int, enable bool) {
	// Note: This function should be implemented in the omni package
	// where private fields can be accessed
}

// ClearRedactionCache clears the redaction cache to free memory.
//
// Note: This function provides the interface but the actual implementation
// should be in the omni package where private fields can be accessed.
func ClearRedactionCache(logger interface{}) {
	// Note: This function should be implemented in the omni package
	// where private fields can be accessed
}

// AddFieldPathRule adds a field path rule for targeted redaction.
//
// Note: This function provides the interface but the actual implementation
// should be in the omni package where private fields can be accessed.
//
// Parameters:
//   - logger: The Omni logger instance (interface{})
//   - path: JSON field path like "user.profile.ssn" or "users.*.email"
//   - replacement: Custom replacement text for this path
//
// Example:
//
//	features.AddFieldPathRule(logger, "user.profile.ssn", "[SSN-REDACTED]")
//	features.AddFieldPathRule(logger, "users.*.email", "[EMAIL-REDACTED]")
func AddFieldPathRule(logger interface{}, path, replacement string) {
	// Note: This function should be implemented in the omni package
	// where private fields can be accessed
}

// RemoveFieldPathRule removes a field path rule.
//
// Note: This function provides the interface but the actual implementation
// should be in the omni package where private fields can be accessed.
//
// Parameters:
//   - logger: The Omni logger instance (interface{})
//   - path: The field path to remove
func RemoveFieldPathRule(logger interface{}, path string) {
	// Note: This function should be implemented in the omni package
	// where private fields can be accessed
}

// GetFieldPathRules returns all configured field path rules.
//
// Note: This function provides the interface but the actual implementation
// should be in the omni package where private fields can be accessed.
//
// Parameters:
//   - logger: The Omni logger instance (interface{})
//
// Returns:
//   - []FieldPathRule: A copy of all field path rules
func GetFieldPathRules(logger interface{}) []FieldPathRule {
	// Note: This function should be implemented in the omni package
	// where private fields can be accessed
	return nil
}

// ClearFieldPathRules removes all field path rules.
//
// Note: This function provides the interface but the actual implementation
// should be in the omni package where private fields can be accessed.
func ClearFieldPathRules(logger interface{}) {
	// Note: This function should be implemented in the omni package
	// where private fields can be accessed
}

// NewRedactionManager creates a new redaction manager
func NewRedactionManager() *RedactionManager {
	return &RedactionManager{
		fieldPathRules:  make([]FieldPathRule, 0),
		contextualRules: make(map[string]ContextualRule),
		metrics: &RedactionMetrics{
			FieldsRedacted:  make(map[string]uint64),
			PatternsMatched: make(map[string]uint64),
			LastUpdate:      time.Now(),
		},
		hashSalt: generateRandomSalt(),
	}
}

// generateRandomSalt generates a random salt for hashing
func generateRandomSalt() string {
	// In production, this should use crypto/rand
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// SetConfig sets the redaction configuration
func (rm *RedactionManager) SetConfig(config *RedactionConfig) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.config = config
	
	// Initialize cache if configured
	if config != nil && config.MaxCacheSize > 0 {
		rm.cache = &RedactionCache{
			cache:   make(map[string]string),
			maxSize: config.MaxCacheSize,
			ttl:     time.Hour, // Default TTL
		}
	}
}

// SetErrorHandler sets the error handling function
func (rm *RedactionManager) SetErrorHandler(handler func(source, dest, msg string, err error)) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.errorHandler = handler
}

// SetMetricsHandler sets the metrics tracking function
func (rm *RedactionManager) SetMetricsHandler(handler func(string)) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.metricsHandler = handler
}

// SetCustomRedactor sets a custom redactor
func (rm *RedactionManager) SetCustomRedactor(redactor *Redactor) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.customRedactor = redactor
}

// AddFieldPathRule adds a field path rule
func (rm *RedactionManager) AddFieldPathRule(rule FieldPathRule) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.fieldPathRules = append(rm.fieldPathRules, rule)
	
	if rm.metricsHandler != nil {
		rm.metricsHandler(fmt.Sprintf("redaction_rule_added_%s", rule.Path))
	}
}

// RemoveFieldPathRule removes a field path rule
func (rm *RedactionManager) RemoveFieldPathRule(path string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	newRules := make([]FieldPathRule, 0, len(rm.fieldPathRules))
	for _, rule := range rm.fieldPathRules {
		if rule.Path != path {
			newRules = append(newRules, rule)
		}
	}
	rm.fieldPathRules = newRules
}

// AddContextualRule adds a context-aware redaction rule
func (rm *RedactionManager) AddContextualRule(rule ContextualRule) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.contextualRules[rule.Name] = rule
}

// RedactMessage redacts a log message with full context
func (rm *RedactionManager) RedactMessage(level int, message string, fields map[string]interface{}) (string, map[string]interface{}) {
	start := time.Now()
	defer func() {
		if rm.metrics != nil {
			elapsed := time.Since(start).Nanoseconds()
			atomic.AddInt64(&rm.metrics.ProcessingTimeNs, elapsed)
			atomic.AddUint64(&rm.metrics.TotalProcessed, 1)
		}
	}()
	
	// Check cache first
	cacheKey := rm.generateCacheKey(message, fields)
	if rm.cache != nil {
		if cached, hit := rm.cache.Get(cacheKey); hit {
			return cached, fields // Return cached result
		}
	}
	
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	// Skip redaction for configured levels
	if rm.config != nil {
		for _, skipLevel := range rm.config.SkipLevels {
			if level == skipLevel {
				return message, fields
			}
		}
	}
	
	// Apply contextual rules
	redactedFields := rm.applyContextualRules(level, fields)
	
	// Redact message
	redactedMessage := rm.redactString(message)
	
	// Cache result
	if rm.cache != nil {
		rm.cache.Set(cacheKey, redactedMessage)
	}
	
	return redactedMessage, redactedFields
}

// applyContextualRules applies context-aware redaction rules
func (rm *RedactionManager) applyContextualRules(level int, fields map[string]interface{}) map[string]interface{} {
	if fields == nil {
		return nil
	}
	
	result := make(map[string]interface{})
	for k, v := range fields {
		result[k] = v
	}
	
	// Apply contextual rules
	for _, rule := range rm.contextualRules {
		if rule.Condition(level, fields) {
			for _, field := range rule.RedactFields {
				if _, exists := result[field]; exists {
					result[field] = rule.Replacement
					rm.trackFieldRedaction(field)
				}
			}
		}
	}
	
	// Apply standard field redaction
	RecursiveRedact(result, "", rm.customRedactor, rm.fieldPathRules)
	
	return result
}

// redactString applies string redaction
func (rm *RedactionManager) redactString(input string) string {
	if input == "" {
		return input
	}
	
	result := input
	
	// Apply custom redactor
	if rm.customRedactor != nil {
		result = rm.customRedactor.Redact(result)
	}
	
	// Apply built-in patterns if enabled
	if rm.config != nil && rm.config.EnableBuiltInPatterns {
		result = rm.applyBuiltInPatterns(result)
	}
	
	return result
}

// applyBuiltInPatterns applies built-in sensitive data patterns
func (rm *RedactionManager) applyBuiltInPatterns(input string) string {
	result := input
	
	// Apply data patterns
	for _, pattern := range builtInDataPatterns {
		matches := pattern.FindAllString(result, -1)
		for _, match := range matches {
			replacement := rm.getReplacementForPattern(match, pattern)
			result = strings.Replace(result, match, replacement, -1)
			rm.trackPatternMatch(pattern.String())
		}
	}
	
	return result
}

// getReplacementForPattern gets appropriate replacement based on mode
func (rm *RedactionManager) getReplacementForPattern(value string, pattern *regexp.Regexp) string {
	if rm.preserveStructure {
		// Preserve structure (e.g., XXX-XX-1234 for SSN)
		return rm.maskValue(value)
	}
	
	// Use hash for consistent replacement
	if rm.hashSalt != "" {
		return rm.hashValue(value)
	}
	
	return "[REDACTED]"
}

// maskValue masks a value while preserving structure
func (rm *RedactionManager) maskValue(value string) string {
	runes := []rune(value)
	result := make([]rune, len(runes))
	
	for i, r := range runes {
		if r >= '0' && r <= '9' {
			// Keep last 4 digits visible
			if i < len(runes)-4 {
				result[i] = 'X'
			} else {
				result[i] = r
			}
		} else if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			result[i] = 'X'
		} else {
			// Preserve special characters
			result[i] = r
		}
	}
	
	return string(result)
}

// hashValue creates a consistent hash of the value
func (rm *RedactionManager) hashValue(value string) string {
	h := sha256.New()
	h.Write([]byte(rm.hashSalt + value))
	hash := h.Sum(nil)
	return fmt.Sprintf("[HASH:%s]", hex.EncodeToString(hash)[:8])
}

// generateCacheKey generates a cache key
func (rm *RedactionManager) generateCacheKey(message string, fields map[string]interface{}) string {
	key := message
	if len(fields) > 0 {
		// Add field keys to cache key
		fieldKeys := make([]string, 0, len(fields))
		for k := range fields {
			fieldKeys = append(fieldKeys, k)
		}
		key += ":" + strings.Join(fieldKeys, ",")
	}
	return key
}

// trackFieldRedaction tracks field redaction metrics
func (rm *RedactionManager) trackFieldRedaction(field string) {
	if rm.metrics != nil && rm.metrics.FieldsRedacted != nil {
		rm.metrics.FieldsRedacted[field]++
		atomic.AddUint64(&rm.metrics.TotalRedacted, 1)
	}
}

// trackPatternMatch tracks pattern match metrics
func (rm *RedactionManager) trackPatternMatch(pattern string) {
	if rm.metrics != nil && rm.metrics.PatternsMatched != nil {
		rm.metrics.PatternsMatched[pattern]++
	}
}

// GetMetrics returns current metrics
func (rm *RedactionManager) GetMetrics() RedactionMetrics {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	if rm.metrics == nil {
		return RedactionMetrics{}
	}
	
	// Create a copy
	metrics := RedactionMetrics{
		TotalProcessed:   atomic.LoadUint64(&rm.metrics.TotalProcessed),
		TotalRedacted:    atomic.LoadUint64(&rm.metrics.TotalRedacted),
		ProcessingTimeNs: atomic.LoadInt64(&rm.metrics.ProcessingTimeNs),
		LastUpdate:       rm.metrics.LastUpdate,
		FieldsRedacted:   make(map[string]uint64),
		PatternsMatched:  make(map[string]uint64),
	}
	
	// Copy maps
	for k, v := range rm.metrics.FieldsRedacted {
		metrics.FieldsRedacted[k] = v
	}
	for k, v := range rm.metrics.PatternsMatched {
		metrics.PatternsMatched[k] = v
	}
	
	// Add cache metrics
	if rm.cache != nil {
		metrics.CacheHits = atomic.LoadUint64(&rm.cache.hits)
		metrics.CacheMisses = atomic.LoadUint64(&rm.cache.misses)
	}
	
	return metrics
}

// Cache implementation methods

// Get retrieves from cache
func (c *RedactionCache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	value, exists := c.cache[key]
	if exists {
		atomic.AddUint64(&c.hits, 1)
		return value, true
	}
	
	atomic.AddUint64(&c.misses, 1)
	return "", false
}

// Set stores in cache
func (c *RedactionCache) Set(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Simple eviction
	if len(c.cache) >= c.maxSize {
		// Remove a random entry
		for k := range c.cache {
			delete(c.cache, k)
			atomic.AddUint64(&c.evictions, 1)
			break
		}
	}
	
	c.cache[key] = value
}

// Additional helper functions

// CreateCreditCardRedactor creates a redactor for credit card numbers
func CreateCreditCardRedactor() *Redactor {
	patterns := []string{
		`\b4\d{3}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`,     // Visa
		`\b5[1-5]\d{2}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`, // MasterCard
		`\b3[47]\d{2}[\s-]?\d{6}[\s-]?\d{5}\b`,             // Amex
		`\b6011[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`,        // Discover
	}
	
	redactor, _ := NewRedactor(patterns, "[CC-REDACTED]")
	return redactor
}

// CreateSSNRedactor creates a redactor for SSN patterns
func CreateSSNRedactor() *Redactor {
	patterns := []string{
		`\b\d{3}-\d{2}-\d{4}\b`,
		`\b\d{3}\s\d{2}\s\d{4}\b`,
		`\b\d{9}\b`,
	}
	
	redactor, _ := NewRedactor(patterns, "[SSN-REDACTED]")
	return redactor
}

// CreateEmailRedactor creates a redactor for email addresses
func CreateEmailRedactor() *Redactor {
	patterns := []string{
		`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`,
	}
	
	redactor, _ := NewRedactor(patterns, "[EMAIL-REDACTED]")
	return redactor
}

// CreateAPIKeyRedactor creates a redactor for common API key formats
func CreateAPIKeyRedactor() *Redactor {
	patterns := []string{
		`\bsk-[a-zA-Z0-9]{48}\b`,                              // OpenAI
		`\bAKIA[0-9A-Z]{16}\b`,                                // AWS
		`\bghp_[a-zA-Z0-9]{36,40}\b`,                         // GitHub
		`\bxoxb-[0-9]{10,13}-[0-9]{10,13}-[a-zA-Z0-9]{24}\b`, // Slack
	}
	
	redactor, _ := NewRedactor(patterns, "[API-KEY-REDACTED]")
	return redactor
}