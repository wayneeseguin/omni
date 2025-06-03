# Omni Examples

This directory contains examples demonstrating various features and use cases of Omni.

## Examples

### Core Examples

1. **basic** - Simple logging example showing basic features
   - Basic logger initialization
   - Different log levels
   - Simple structured logging

2. **advanced-features** - Demonstrates advanced features
   - Multiple destinations
   - Log filtering
   - Log rotation and compression
   - Error handling
   - Dynamic configuration

3. **context-aware** - Shows context integration for distributed tracing
   - Context propagation
   - Trace ID handling
   - Request correlation
   - Contextual logging

4. **multiple-destinations** - Logging to multiple outputs simultaneously
   - File and syslog backends
   - Destination-specific filtering
   - Dynamic destination management

5. **performance-optimized** - High-performance configuration
   - Message batching
   - Log sampling strategies
   - Buffer optimization
   - Metrics monitoring

### Real-World Examples

6. **web-service** - HTTP web service with comprehensive logging
   - Request/response logging middleware
   - Structured logging for web requests
   - Performance monitoring
   - Graceful shutdown
   - Metrics endpoint

7. **cli-application** - Command-line application logging
   - Verbose/debug modes
   - Progress tracking
   - File processing with detailed logs
   - User-friendly console output

8. **microservice** - Microservice with distributed tracing
   - Trace context propagation
   - Service mesh integration
   - Adaptive sampling
   - Health checks and metrics
   - External service calls

## Running Examples

Each example can be run directly:

```bash
# Basic example
cd basic
go run main.go

# Web service (runs on port 8080)
cd web-service
go run main.go
# Test with: curl http://localhost:8080/health

# CLI application
cd cli-application
go run main.go -verbose -op process file1.txt file2.txt

# Microservice
cd microservice
go run main.go
# Test with: curl http://localhost:8080/api/payment -d '{"amount":100,"currency":"USD"}'
```

## Building Examples

To build all examples:

```bash
for dir in */; do
    if [ -f "$dir/main.go" ]; then
        echo "Building $dir"
        cd "$dir"
        go build -o "${dir%/}"
        cd ..
    fi
done
```

## Example Features by Category

### Basic Logging
- basic/main.go - Simple logging operations
- cli-application/main.go - Console logging with levels

### Structured Logging
- advanced-features/main.go - Field-based logging
- web-service/main.go - HTTP request context
- microservice/main.go - Service metadata

### Performance
- performance-optimized/main.go - Batching and sampling
- web-service/main.go - High-volume endpoint handling
- microservice/main.go - Adaptive sampling

### Distributed Systems
- context-aware/main.go - Context propagation
- microservice/main.go - Trace ID handling
- web-service/main.go - Request correlation

### Error Handling
- advanced-features/main.go - Error callbacks
- cli-application/main.go - Graceful error logging
- web-service/main.go - HTTP error responses

### Configuration
- advanced-features/main.go - Dynamic configuration
- cli-application/main.go - CLI flags for logging
- microservice/main.go - Environment-based config

## Best Practices Demonstrated

1. **Logger Initialization**
   - Single logger instance per application
   - Proper cleanup with defer
   - Error handling during setup

2. **Structured Logging**
   - Consistent field names
   - Context propagation
   - Request-scoped loggers

3. **Performance**
   - Appropriate log levels
   - Sampling for high-volume logs
   - Efficient field usage

4. **Production Readiness**
   - Log rotation
   - Compression
   - Metrics exposure
   - Graceful shutdown

## Common Patterns

### HTTP Middleware
```go
func loggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // See web-service/main.go
    }
}
```

### CLI Verbosity
```go
if *verbose {
    logger.SetLevel(omni.LevelDebug)
}
// See cli-application/main.go
```

### Microservice Context
```go
ctx = omni.WithTraceID(ctx, traceID)
logger.StructuredLogWithContext(ctx, level, msg, fields)
// See microservice/main.go
```

## Testing the Examples

Most examples include test scenarios:

```bash
# Web service load test
ab -n 10000 -c 100 http://localhost:8080/api/user?id=123

# CLI application with files
touch test{1..100}.txt
go run main.go -op process test*.txt

# Microservice trace propagation
curl -H "X-Trace-ID: trace-123" http://localhost:8080/api/payment
```