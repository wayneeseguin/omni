package flexlog

import (
	"regexp"
	"strings"
)

// Sensitive patterns to redact
var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`("auth_token"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("password"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("secret"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("key"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("private_key"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`("token"\s*:\s*)"[^"]*"`),
	regexp.MustCompile(`(Bearer\s+)[A-Za-z0-9-._~+/]+=*`),
}

// FlogRequest logs an API request safely
func (f *FlexLog) LogRequest(method, path string, headers map[string][]string, body string) {
	safeHeaders := make(map[string][]string)

	for k, v := range headers {
		// Skip sensitive headers
		if strings.ToLower(k) == "authorization" ||
			strings.Contains(strings.ToLower(k), "token") ||
			strings.Contains(strings.ToLower(k), "key") {
			safeHeaders[k] = []string{"[REDACTED]"}
			continue
		}
		safeHeaders[k] = v
	}

	f.logf(LevelInfo, "API Request: %s %s\nHeaders: %v\nBody: %s", method, path, safeHeaders, f.redactSensitive(body))
}

// FlogResponse logs an API response safely
func (f *FlexLog) LogResponse(statusCode int, headers map[string][]string, body string) {
	safeHeaders := make(map[string][]string)

	for k, v := range headers {
		// Skip sensitive headers
		if strings.Contains(strings.ToLower(k), "token") ||
			strings.Contains(strings.ToLower(k), "key") {
			safeHeaders[k] = []string{"[REDACTED]"}
			continue
		}
		safeHeaders[k] = v
	}

	f.logf(LevelInfo, "API Response: Status: %d\nHeaders: %v\nBody: %s", statusCode, safeHeaders, f.redactSensitive(body))
}

// redactSensitive replaces sensitive information with [REDACTED]
func (f *FlexLog) redactSensitive(input string) string {
	if input == "" {
		return input
	}

	result := input

	for _, pattern := range sensitivePatterns {
		result = pattern.ReplaceAllString(result, "${1}\"[REDACTED]\"")
	}

	return result
}
