# Redaction Example

This example demonstrates Omni's comprehensive data redaction capabilities for secure logging.

## Overview

Omni provides automatic redaction of sensitive data in log messages to prevent accidental exposure of credentials, personal information, and other sensitive data. This is crucial for:

- **Security Compliance**: Meet security standards and regulations
- **API Logging**: Safely log HTTP requests/responses without exposing tokens
- **Data Privacy**: Protect user PII and sensitive information
- **Audit Trails**: Maintain detailed logs while keeping sensitive data secure

## Features Demonstrated

### 1. Built-in Redaction Patterns

Omni automatically redacts common sensitive field names:
- `password`, `passwd`, `pass`
- `secret`, `api_key`, `apikey`
- `auth_token`, `authorization`
- `private_key`, `token`
- `access_token`, `refresh_token`

### 2. Custom Redaction Patterns

Add your own regex patterns for specific data types:
- Social Security Numbers (SSN)
- Credit card numbers
- Email addresses
- Phone numbers
- Custom API key formats

### 3. HTTP Request/Response Logging

Safely log HTTP interactions with automatic redaction of:
- Authorization headers
- Sensitive body fields
- Custom header patterns

### 4. Nested Data Redaction

Redaction works recursively through:
- Nested objects
- Arrays of objects
- Mixed JSON and text content
- Complex data structures

## Running the Example

```bash
# Run the main example
go run main.go

# Run the tests
go test -v

# Run performance benchmarks
go test -v -run TestRedactionPerformance
```

## Expected Output

The example creates `redacted.log` with JSON-formatted log entries where sensitive fields are replaced with `[REDACTED]`.

Example log entry:
```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "level": "INFO",
  "message": "User login attempt",
  "fields": {
    "username": "john.doe",
    "password": "[REDACTED]",
    "api_key": "[REDACTED]",
    "auth_token": "[REDACTED]",
    "session_id": "sess_123456"
  }
}
```

## Configuration Examples

### Basic Redaction Setup

```go
logger, err := omni.New(
    omni.WithDestination("app.log", omni.BackendFlock),
    omni.WithFormat(omni.FormatJSON),
)
// Built-in patterns are automatically active
```

### Custom Patterns

```go
// Add custom redaction patterns
customPatterns := []string{
    `\b\d{3}-\d{2}-\d{4}\b`,  // SSN pattern
    `\b\d{4}[-\s]?\d{4}[-\s]?\d{4}[-\s]?\d{4}\b`, // Credit card
    `\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`, // Email
}

logger.SetRedaction(customPatterns, "[CUSTOM-REDACTED]")
```

### HTTP Logging

```go
// Automatically redacts sensitive headers and body fields
logger.LogRequest(httpRequest)
logger.LogResponse(httpResponse, responseBody)
```

## Security Best Practices

1. **Always Enable Redaction**: Enable redaction for production logging
2. **Test Patterns**: Verify custom patterns work with your data
3. **Review Logs**: Regularly audit logs to ensure no sensitive data leaks
4. **Use Structured Logging**: JSON format makes redaction more reliable
5. **Custom Replacement Text**: Use descriptive replacement text for debugging

## Performance Considerations

Redaction adds processing overhead:
- JSON parsing for structured data
- Regex matching for patterns
- Memory allocation for string replacement

For high-throughput applications:
- Test performance impact
- Consider per-level redaction settings
- Monitor memory usage

## Common Patterns

### Government IDs
```go
patterns := []string{
    `\b\d{3}-\d{2}-\d{4}\b`,        // US SSN
    `\b[A-Z]\d{8}[A-Z]\b`,          // UK National Insurance
    `\b\d{3}\s\d{3}\s\d{3}\b`,      // Canadian SIN
}
```

### Financial Data
```go
patterns := []string{
    `\b\d{4}[-\s]?\d{4}[-\s]?\d{4}[-\s]?\d{4}\b`, // Credit cards
    `\b\d{9,18}\b`,                                 // Bank account numbers
    `\b[A-Z]{2}\d{2}[A-Z0-9]{4}\d{7}([A-Z0-9]?){0,16}\b`, // IBAN
}
```

### API Keys and Tokens
```go
patterns := []string{
    `\bsk-[a-zA-Z0-9]{48}\b`,       // OpenAI API keys
    `\bAKIA[0-9A-Z]{16}\b`,         // AWS Access Keys
    `\bghp_[a-zA-Z0-9]{36}\b`,      // GitHub Personal Access Tokens
}
```

## Testing

The example includes comprehensive tests:
- `TestRedactionExample`: Verifies basic redaction functionality
- `TestRedactionPerformance`: Measures redaction performance impact
- `TestRedactionCompleteness`: Ensures all sensitive patterns are caught

Run tests to verify redaction works in your environment:
```bash
go test -v -cover
```