# Disk Full Handling Example

This example demonstrates Omni's automatic disk full recovery feature, which ensures your application continues logging even when disk space is exhausted.

## Features Demonstrated

- **Automatic Disk Full Detection**: Recognizes when writes fail due to lack of space
- **Intelligent Log Rotation**: Automatically rotates current log file when disk is full
- **Space Recovery**: Removes oldest rotated logs to free up disk space
- **Retry Mechanism**: Retries failed writes after freeing space
- **Error Monitoring**: Tracks and reports disk full events

## How It Works

1. **Detection**: The file backend detects disk full errors (ENOSPC, "no space left", etc.)
2. **Rotation**: Automatically rotates the current log file to create a new one
3. **Cleanup**: Removes oldest rotated logs based on configured limits
4. **Recovery**: Retries the failed write operation
5. **Continuation**: Resumes normal logging operations

## Running the Example

```bash
go run main.go
```

## Example Output

```
=== Omni Disk Full Handling Example ===

Simulating high-volume logging...
(In production, disk full would trigger automatic rotation)

Progress: 100 messages logged, 0 rotations, 0 disk full events
Progress: 200 messages logged, 1 rotations, 0 disk full events
✓ Log rotation completed (total: 1)
Progress: 300 messages logged, 1 rotations, 0 disk full events

=== Summary ===
Messages logged: 1000
Log rotations: 2
Disk full events: 0
Errors handled: 0
Duration: 157.123ms
Rate: 6367.89 messages/second

Rotated log files:
  - example-diskfull.log.20240115-143052.123 (102.45 KB, rotated at 14:30:52)
  - example-diskfull.log.20240115-143053.456 (98.76 KB, rotated at 14:30:53)
```

## Configuration Options

### Rotation Manager Settings

```go
rotMgr := features.NewRotationManager()
rotMgr.SetMaxFiles(5)                      // Keep only 5 rotated files
rotMgr.SetMaxAge(7 * 24 * time.Hour)      // Keep logs for 7 days
rotMgr.SetCleanupInterval(time.Hour)      // Check hourly for old logs
```

### Backend Settings

```go
backend.SetMaxRetries(3)  // Retry up to 3 times on disk full
```

### Error Handling

```go
backend.SetErrorHandler(func(source, dest, msg string, err error) {
    // Monitor disk full events
    if strings.Contains(msg, "disk full") {
        alertOps("Disk full condition on " + dest)
    }
})
```

## Production Considerations

1. **Monitor Disk Space**: Set up alerts before disk becomes full
2. **Configure Retention**: Balance between log retention and available space
3. **Test Recovery**: Verify disk full handling in your environment
4. **Set Appropriate Limits**: Configure `MaxFiles` based on your disk capacity
5. **Enable Compression**: Use compression for rotated logs to save space

## Testing Disk Full Scenarios

To test disk full handling in a controlled environment:

```bash
# Run integration tests with Docker
make test-diskfull

# The test creates a 1MB tmpfs filesystem to simulate disk full conditions
```

## Best Practices

1. **Use with Critical Logs**: Enable for logs that must not be lost
2. **Monitor Rotation Events**: Track how often disk full occurs
3. **Adjust Retention**: Reduce `MaxFiles` if disk full happens frequently
4. **Enable Compression**: Compress old logs to maximize available space
5. **Alert on Disk Full**: Set up monitoring to alert operations team

## Related Examples

- [Log Rotation](../rotation/) - Basic log rotation example
- [Compression](../compression/) - Log compression example
- [Performance Optimized](../performance-optimized/) - High-throughput logging