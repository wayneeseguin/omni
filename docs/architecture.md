# Omni Architecture

This document describes the internal architecture of Omni, design decisions, and extension points for developers who want to understand or extend the library.

## Overview

Omni is designed as a high-performance, flexible logging library with these core principles:

1. **Non-blocking**: Logging operations should never block the application
2. **Process-safe**: Multiple processes can safely write to the same log file
3. **Extensible**: Easy to add new backends, formatters, and features
4. **Zero dependencies**: Only standard library dependencies for core functionality
5. **Performance-focused**: Minimal allocations and efficient I/O

## Package Structure

Omni is organized into modular packages for better maintainability:

```
pkg/
├── omni/          # Core logger implementation
├── backends/      # Backend implementations (file, syslog, plugin)
├── features/      # Feature modules (compression, filtering, rotation)
├── formatters/    # Output formatters (JSON, text)
├── plugins/       # Plugin system and management
└── types/         # Common types and interfaces

internal/
├── buffer/        # Buffer pool and batch writer
├── metrics/       # Metrics collection
└── utils/         # Internal utilities (atomic, context, etc.)
```

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Application Code                          │
├─────────────────────────────────────────────────────────────────┤
│                          Logger API                              │
│  (Debug, Info, Warn, Error, WithFields, WithContext)           │
├─────────────────────────────────────────────────────────────────┤
│                      Message Channel                             │
│              (Buffered, Non-blocking Queue)                      │
├─────────────────────────────────────────────────────────────────┤
│                    Background Worker                             │
│         (Dispatcher Goroutine - processMessages)                 │
├─────────────────────────────────────────────────────────────────┤
│   Filtering  │  Sampling  │  Formatting  │  Error Handling      │
├─────────────────────────────────────────────────────────────────┤
│                    Destination Manager                           │
│              (Routes to Multiple Backends)                       │
├─────────────────────────────────────────────────────────────────┤
│   File Backend   │  Syslog Backend  │  Custom Backends         │
│   (with flock)   │                  │                          │
└─────────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Omni Structure (pkg/omni/logger.go)

```go
type Omni struct {
    // Message handling
    messages     chan *types.LogMessage  // Buffered channel for async processing
    done         chan struct{}           // Shutdown signal
    wg           sync.WaitGroup          // Tracks background goroutines
    
    // State management
    mu           sync.RWMutex            // Protects mutable state
    level        atomic.Int32            // Current log level (atomic for performance)
    format       int                     // Output format (text/json)
    
    // Primary destination (legacy support)
    path         string                  // Primary log file path
    file         *os.File               // Primary file handle
    writer       *bufio.Writer          // Buffered writer
    
    // Multi-destination support
    destinations sync.Map               // Thread-safe map of destinations
    
    // Features
    filters      []features.FilterFunc  // Message filters
    sampler      features.Sampler      // Sampling strategy
    errorHandler ErrorHandler          // Error callback
    
    // Metrics
    metrics      *metrics.Collector    // Performance metrics
}
```

### 2. Message Flow

#### Step 1: Message Creation
```go
func (f *Omni) Info(args ...interface{}) {
    // Fast path: level check
    if !f.IsLevelEnabled(LevelInfo) {
        return
    }
    
    // Create message
    msg := LogMessage{
        Level:     LevelInfo,
        Timestamp: time.Now(),
        Message:   fmt.Sprint(args...),
        Fields:    nil,
    }
    
    // Non-blocking send
    select {
    case f.messages <- msg:
        // Success
    default:
        // Channel full - increment dropped counter
        f.metrics.DroppedMessages.Add(1)
    }
}
```

#### Step 2: Background Processing
```go
func (f *Omni) processMessages() {
    defer f.wg.Done()
    
    for {
        select {
        case msg := <-f.messages:
            f.handleMessage(msg)
            
        case <-f.done:
            // Drain remaining messages
            f.drainMessages()
            return
        }
    }
}
```

#### Step 3: Message Handling
```go
func (f *Omni) handleMessage(msg LogMessage) {
    // 1. Apply filters
    if !f.shouldLog(msg) {
        return
    }
    
    // 2. Apply sampling
    if !f.sampler.ShouldSample(msg) {
        return
    }
    
    // 3. Format message
    formatted := f.formatMessage(msg)
    
    // 4. Write to destinations
    f.writeToDestinations(formatted)
    
    // 5. Update metrics
    f.metrics.MessageCounts[msg.Level].Add(1)
}
```

### 3. Destination Management

Each destination maintains its own state:

```go
type Destination struct {
    Name     string                   // Unique identifier
    URI      string                   // Connection string
    Backend  backends.Backend         // Backend implementation
    Enabled  atomic.Bool              // Runtime enable/disable
    
    // State tracking
    mu       sync.RWMutex            // Protects mutable fields
    Filter   features.FilterFunc     // Destination-specific filter
    
    // Metrics
    BytesWritten atomic.Uint64       // Total bytes written
    Errors       atomic.Uint64       // Error count
}
```

### 4. Synchronization Strategy

Omni uses a careful locking hierarchy to prevent deadlocks:

```
Lock Order (must be acquired in this order):
1. Omni.mu          - Protects logger state
2. Destination.mu      - Protects destination state  
3. File locks (flock)  - Process-level synchronization
```

Example:
```go
func (f *Omni) rotateDestination(dest *Destination) error {
    // 1. Lock Omni first (if needed)
    f.mu.Lock()
    defer f.mu.Unlock()
    
    // 2. Then lock destination
    dest.mu.Lock()
    defer dest.mu.Unlock()
    
    // 3. Finally acquire file lock
    if err := syscall.Flock(int(dest.File.Fd()), 
        syscall.LOCK_EX); err != nil {
        return err
    }
    defer syscall.Flock(int(dest.File.Fd()), 
        syscall.LOCK_UN)
    
    // Perform rotation...
}
```

## Design Decisions

### 1. Channel-Based Architecture

**Decision**: Use a buffered channel for message passing.

**Rationale**:
- Non-blocking logging operations
- Natural backpressure mechanism
- Simple concurrency model

**Trade-offs**:
- Messages can be dropped if channel is full
- Fixed memory overhead
- Single point of serialization

### 2. Process Safety via File Locks

**Decision**: Use Unix file locks (flock) for cross-process synchronization.

**Rationale**:
- Standard Unix mechanism
- Automatic cleanup on process exit
- Works across different processes

**Trade-offs**:
- Platform-specific (Unix only)
- Some performance overhead
- Requires exclusive locks for writes

### 3. Multiple Backend Support

**Decision**: Abstract backends behind interfaces.

```go
type Backend interface {
    Write(entry []byte) (int, error)
    Flush() error
    Close() error
    SupportsAtomic() bool
}
```

**Rationale**:
- Easy to add new backends
- Testing with mock backends
- Backend-specific optimizations

### 4. Atomic Operations for Hot Paths

**Decision**: Use atomic operations for frequently accessed fields.

```go
// Level check (hot path)
func (f *Omni) IsLevelEnabled(level int) bool {
    return level >= int(f.level.Load())
}

// Metrics updates
f.metrics.TotalMessages.Add(1)
```

**Rationale**:
- Avoid lock contention
- Better cache performance
- Predictable performance

## Extension Points

### 1. Custom Backends

Implement the Backend interface in pkg/backends/interfaces.go:

```go
import "github.com/wayneeseguin/omni/pkg/backends"

type CustomBackend struct {
    // Your fields
}

func (b *CustomBackend) Write(entry []byte) (int, error) {
    // Your implementation
}

func (b *CustomBackend) Flush() error {
    // Your implementation
}

func (b *CustomBackend) Close() error {
    // Your implementation
}

func (b *CustomBackend) SupportsAtomic() bool {
    return false // or true if your backend supports atomic writes
}

// Register backend
backends.RegisterBackend("custom", func(uri string) (backends.Backend, error) {
    return &CustomBackend{}, nil
})
```

### 2. Custom Formatters

Implement the Formatter interface in pkg/formatters/interfaces.go:

```go
import (
    "github.com/wayneeseguin/omni/pkg/formatters"
    "github.com/wayneeseguin/omni/pkg/types"
)

type CustomFormatter struct{}

func (f *CustomFormatter) Format(msg *types.LogMessage) ([]byte, error) {
    // Your formatting logic
    return []byte(formatted), nil
}

// Use it
logger.SetFormatter(&CustomFormatter{})
```

### 3. Custom Samplers

Implement the Sampler interface in pkg/features/interfaces.go:

```go
import (
    "github.com/wayneeseguin/omni/pkg/features"
    "github.com/wayneeseguin/omni/pkg/types"
)

type CustomSampler struct {
    // Your fields
}

func (s *CustomSampler) ShouldSample(msg *types.LogMessage) bool {
    // Your sampling logic
    return true
}

// Use it
logger.SetSampler(&CustomSampler{})
```

### 4. Middleware Pipeline

Add processing stages:

```go
type Middleware func(LogMessage) LogMessage

func (f *Omni) AddMiddleware(mw Middleware) {
    f.mu.Lock()
    defer f.mu.Unlock()
    f.middleware = append(f.middleware, mw)
}

// Example: Add request ID
func RequestIDMiddleware(msg LogMessage) LogMessage {
    if msg.Fields == nil {
        msg.Fields = make(map[string]interface{})
    }
    msg.Fields["request_id"] = getCurrentRequestID()
    return msg
}
```

## Performance Characteristics

### Memory Usage

- **Channel Buffer**: `channelSize * sizeof(LogMessage)` ≈ `channelSize * 200 bytes`
- **Buffer Pool**: Reusable buffers via `sync.Pool`
- **Per-Destination Buffer**: Default 4KB per destination

### CPU Usage

- **Lock-free Operations**: Level checks, metric updates
- **Minimal Allocations**: Buffer reuse, string interning
- **Batch Processing**: Amortize syscall overhead

### I/O Patterns

- **Buffered Writes**: Reduce syscall frequency
- **Async Compression**: Off critical path
- **Rotation**: Atomic rename operations

## Benchmarks

```
BenchmarkLogSimple-8           10000000       112 ns/op       0 B/op       0 allocs/op
BenchmarkLogWithFields-8        3000000       423 ns/op      96 B/op       2 allocs/op
BenchmarkLogJSON-8              2000000       734 ns/op     184 B/op       3 allocs/op
BenchmarkConcurrent-8           5000000       287 ns/op      32 B/op       1 allocs/op
```

## Future Considerations

### 1. Zero-Copy Formatting
Investigate using `io.WriterTo` interface for zero-copy writes.

### 2. Lock-Free Data Structures
Consider lock-free queues for message passing.

### 3. io_uring Support
For Linux, investigate io_uring for async I/O.

### 4. Structured Logging Protocol
Implement OpenTelemetry logging protocol support.

## Contributing

When contributing to Omni:

1. **Maintain Lock Hierarchy**: Always acquire locks in the documented order
2. **Benchmark Changes**: Run benchmarks before and after
3. **Test Concurrency**: Use `-race` flag in tests
4. **Document Decisions**: Update this document for architectural changes
5. **Backward Compatibility**: Maintain existing APIs

## Testing Architecture

### Unit Tests
- Mock backends for isolation
- Table-driven tests for comprehensive coverage
- Concurrent operation tests

### Integration Tests
- Multi-process file locking
- Rotation under load
- Failure injection

### Performance Tests
- Benchmarks for critical paths
- Memory allocation tracking
- Load tests for sustained throughput

## Security Considerations

1. **File Permissions**: Created files use 0644 by default
2. **Path Traversal**: Paths are cleaned and validated
3. **Resource Limits**: Bounded channels prevent memory exhaustion
4. **Error Information**: Sensitive data is not leaked in errors