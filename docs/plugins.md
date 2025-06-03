# Omni Plugin System

Omni's plugin system allows you to extend the logger with custom backends, formatters, and filters without modifying the core library. This document covers how to create, load, and use plugins.

## Plugin Types

Omni supports three types of plugins:

1. **Backend Plugins** - Custom log destinations (e.g., databases, message queues, cloud services)
2. **Formatter Plugins** - Custom output formats (e.g., XML, Protocol Buffers, custom JSON)
3. **Filter Plugins** - Custom filtering logic (e.g., rate limiting, content-based filtering)

## Plugin Interfaces

### Backend Plugin

```go
type BackendPlugin interface {
    Plugin
    
    // CreateBackend creates a new backend instance
    CreateBackend(uri string, config map[string]interface{}) (Backend, error)
    
    // SupportedSchemes returns URI schemes this plugin supports
    SupportedSchemes() []string
}

type Backend interface {
    // Write writes a log entry to the backend
    Write(entry []byte) (int, error)
    
    // Flush ensures all buffered data is written
    Flush() error
    
    // Close closes the backend
    Close() error
    
    // SupportsAtomic returns whether the backend supports atomic writes
    SupportsAtomic() bool
}
```

### Formatter Plugin

```go
type FormatterPlugin interface {
    Plugin
    
    // CreateFormatter creates a new formatter instance
    CreateFormatter(config map[string]interface{}) (Formatter, error)
    
    // FormatName returns the format name
    FormatName() string
}

type Formatter interface {
    // Format formats a log message
    Format(msg LogMessage) ([]byte, error)
}
```

### Filter Plugin

```go
type FilterPlugin interface {
    Plugin
    
    // CreateFilter creates a new filter instance
    CreateFilter(config map[string]interface{}) (FilterFunc, error)
    
    // FilterType returns the filter type name
    FilterType() string
}

type FilterFunc func(level int, message string, fields map[string]interface{}) bool
```

### Base Plugin Interface

```go
type Plugin interface {
    // Name returns the plugin name
    Name() string
    
    // Version returns the plugin version
    Version() string
    
    // Initialize initializes the plugin with configuration
    Initialize(config map[string]interface{}) error
    
    // Shutdown cleans up plugin resources
    Shutdown(ctx context.Context) error
}
```

## Creating Plugins

### Example: XML Formatter Plugin

```go
package main

import (
    "context"
    "encoding/xml"
    "time"
    "github.com/wayneeseguin/omni"
)

type XMLFormatterPlugin struct {
    initialized bool
    config      map[string]interface{}
}

type XMLLogEntry struct {
    XMLName   xml.Name   `xml:"logEntry"`
    Timestamp string     `xml:"timestamp"`
    Level     string     `xml:"level"`
    Message   string     `xml:"message"`
    Fields    []XMLField `xml:"fields>field,omitempty"`
}

type XMLField struct {
    Key   string `xml:"key,attr"`
    Value string `xml:",chardata"`
}

type XMLFormatter struct {
    includeFields bool
    timeFormat    string
}

func (p *XMLFormatterPlugin) Name() string {
    return "xml-formatter"
}

func (p *XMLFormatterPlugin) Version() string {
    return "1.0.0"
}

func (p *XMLFormatterPlugin) Initialize(config map[string]interface{}) error {
    p.config = config
    p.initialized = true
    return nil
}

func (p *XMLFormatterPlugin) Shutdown(ctx context.Context) error {
    p.initialized = false
    return nil
}

func (p *XMLFormatterPlugin) CreateFormatter(config map[string]interface{}) (omni.Formatter, error) {
    formatter := &XMLFormatter{
        includeFields: true,
        timeFormat:    time.RFC3339,
    }
    
    if val, ok := config["include_fields"].(bool); ok {
        formatter.includeFields = val
    }
    
    if val, ok := config["time_format"].(string); ok {
        formatter.timeFormat = val
    }
    
    return formatter, nil
}

func (p *XMLFormatterPlugin) FormatName() string {
    return "xml"
}

func (f *XMLFormatter) Format(msg omni.LogMessage) ([]byte, error) {
    entry := XMLLogEntry{
        Timestamp: msg.Timestamp.Format(f.timeFormat),
        Level:     omni.LevelName(msg.Level),
        Message:   msg.Entry.Message,
    }
    
    if f.includeFields && msg.Entry.Fields != nil {
        for key, value := range msg.Entry.Fields {
            entry.Fields = append(entry.Fields, XMLField{
                Key:   key,
                Value: fmt.Sprintf("%v", value),
            })
        }
    }
    
    return xml.MarshalIndent(entry, "", "  ")
}

// Plugin entry point
var OmniPlugin = &XMLFormatterPlugin{}

func main() {
    // Plugin main function (not used when loaded as plugin)
}
```

### Example: Database Backend Plugin

```go
package main

import (
    "context"
    "database/sql"
    "net/url"
    "github.com/wayneeseguin/omni"
    _ "github.com/lib/pq" // PostgreSQL driver
)

type DatabaseBackendPlugin struct {
    initialized bool
}

type DatabaseBackend struct {
    db        *sql.DB
    tableName string
    stmt      *sql.Stmt
}

func (p *DatabaseBackendPlugin) Name() string {
    return "database-backend"
}

func (p *DatabaseBackendPlugin) Version() string {
    return "1.0.0"
}

func (p *DatabaseBackendPlugin) Initialize(config map[string]interface{}) error {
    p.initialized = true
    return nil
}

func (p *DatabaseBackendPlugin) Shutdown(ctx context.Context) error {
    p.initialized = false
    return nil
}

func (p *DatabaseBackendPlugin) SupportedSchemes() []string {
    return []string{"postgres", "mysql", "sqlite"}
}

func (p *DatabaseBackendPlugin) CreateBackend(uri string, config map[string]interface{}) (omni.Backend, error) {
    parsedURL, err := url.Parse(uri)
    if err != nil {
        return nil, err
    }
    
    // Extract table name from query params
    query := parsedURL.Query()
    tableName := query.Get("table")
    if tableName == "" {
        tableName = "logs"
    }
    
    // Connect to database
    db, err := sql.Open(parsedURL.Scheme, uri)
    if err != nil {
        return nil, err
    }
    
    backend := &DatabaseBackend{
        db:        db,
        tableName: tableName,
    }
    
    // Prepare insert statement
    if err := backend.prepare(); err != nil {
        db.Close()
        return nil, err
    }
    
    return backend, nil
}

func (b *DatabaseBackend) prepare() error {
    query := fmt.Sprintf(`
        INSERT INTO %s (timestamp, level, message, fields) 
        VALUES ($1, $2, $3, $4)
    `, b.tableName)
    
    stmt, err := b.db.Prepare(query)
    if err != nil {
        return err
    }
    
    b.stmt = stmt
    return nil
}

func (b *DatabaseBackend) Write(entry []byte) (int, error) {
    // Parse entry (assuming JSON format)
    var logEntry map[string]interface{}
    if err := json.Unmarshal(entry, &logEntry); err != nil {
        return 0, err
    }
    
    // Extract fields
    timestamp := logEntry["timestamp"]
    level := logEntry["level"]
    message := logEntry["message"]
    fields := logEntry["fields"]
    
    // Convert fields to JSON
    fieldsJSON, _ := json.Marshal(fields)
    
    // Execute insert
    _, err := b.stmt.Exec(timestamp, level, message, fieldsJSON)
    if err != nil {
        return 0, err
    }
    
    return len(entry), nil
}

func (b *DatabaseBackend) Flush() error {
    // Database writes are typically immediate
    return nil
}

func (b *DatabaseBackend) Close() error {
    if b.stmt != nil {
        b.stmt.Close()
    }
    if b.db != nil {
        return b.db.Close()
    }
    return nil
}

func (b *DatabaseBackend) SupportsAtomic() bool {
    return true // Database transactions are atomic
}

var OmniPlugin = &DatabaseBackendPlugin{}
```

## Building Plugins

### Build as Shared Library

```bash
# Build plugin as shared library
go build -buildmode=plugin -o xml-formatter.so xml-formatter/main.go

# Build with version information
go build -buildmode=plugin -ldflags "-X main.Version=1.0.0" -o xml-formatter.so xml-formatter/main.go
```

### Plugin Metadata File

Create a `plugin.json` file alongside your plugin:

```json
{
    "name": "xml-formatter",
    "version": "1.0.0",
    "description": "XML output formatter for Omni",
    "author": "Your Name",
    "license": "MIT",
    "type": "formatter",
    "config": {
        "include_fields": {
            "type": "boolean",
            "default": true,
            "description": "Include structured fields in XML output"
        },
        "time_format": {
            "type": "string",
            "default": "RFC3339",
            "description": "Timestamp format"
        }
    }
}
```

## Loading Plugins

### Manual Loading

```go
// Load a specific plugin
err := omni.LoadPlugin("./plugins/xml-formatter.so")
if err != nil {
    log.Fatalf("Failed to load plugin: %v", err)
}

// Use the plugin
logger, _ := omni.NewBuilder().
    WithPath("/var/log/app.log").
    WithCustomFormatter("xml", map[string]interface{}{
        "include_fields": true,
        "time_format": time.RFC3339,
    }).
    Build()
```

### Automatic Discovery

```go
// Set plugin search paths
omni.SetPluginSearchPaths([]string{
    "./plugins",
    "/usr/local/lib/omni/plugins",
    os.Getenv("HOME") + "/.omni/plugins",
})

// Discover and load all plugins
err := omni.DiscoverAndLoadPlugins()
if err != nil {
    log.Printf("Plugin loading errors: %v", err)
}
```

### Configuration-Based Loading

```go
// Load plugins from configuration
specs := []omni.PluginSpec{
    {
        Name: "xml-formatter",
        Path: "./plugins/xml-formatter.so",
        Config: map[string]interface{}{
            "include_fields": true,
        },
    },
    {
        Name: "redis-backend",
        URL:  "https://plugins.example.com/redis-backend-v1.0.0.so",
    },
}

discovery := omni.NewPluginDiscovery(omni.GetPluginManager())
err := discovery.LoadPluginSpecs(specs)
```

## Using Plugins

### Backend Plugins

```go
// Add Redis destination using plugin
logger.AddDestinationWithPlugin("redis://localhost:6379/0?key=app_logs&max=1000")

// Add database destination
logger.AddDestinationWithPlugin("postgres://user:pass@localhost/db?table=application_logs")

// Add Elasticsearch destination
logger.AddDestinationWithPlugin("elasticsearch://localhost:9200/logs/doc")
```

### Formatter Plugins

```go
// Use XML formatter
logger.SetCustomFormatter("xml", map[string]interface{}{
    "include_fields": true,
    "root_element": "logEntry",
})

// Use Protocol Buffers formatter
logger.SetCustomFormatter("protobuf", map[string]interface{}{
    "schema_file": "/etc/app/log_schema.proto",
})
```

### Filter Plugins

```go
// Add rate limiting filter
rateLimiter, _ := omni.GetPluginManager().GetFilterPlugin("rate-limiter")
filter, _ := rateLimiter.CreateFilter(map[string]interface{}{
    "rate": 100.0,  // 100 messages per second
    "burst": 200,   // burst of 200
})
logger.AddFilter(filter)

// Add content-based filter
contentFilter, _ := omni.GetPluginManager().GetFilterPlugin("content-filter")
filter2, _ := contentFilter.CreateFilter(map[string]interface{}{
    "blacklist": []string{"password", "secret", "token"},
    "whitelist_levels": []string{"ERROR", "WARN"},
})
logger.AddFilter(filter2)
```

## Plugin Management

### List Loaded Plugins

```go
manager := omni.GetPluginManager()
plugins := manager.GetPluginInfo()

for _, plugin := range plugins {
    fmt.Printf("Plugin: %s v%s (%s)\n", 
        plugin.Name, plugin.Version, plugin.Type)
}
```

### Unload Plugins

```go
// Unload specific plugin
err := omni.UnloadPlugin("xml-formatter")
if err != nil {
    log.Printf("Failed to unload plugin: %v", err)
}
```

### Plugin Health Monitoring

```go
// Monitor plugin health
go func() {
    ticker := time.NewTicker(time.Minute)
    defer ticker.Stop()
    
    for range ticker.C {
        plugins := omni.GetPluginManager().ListPlugins()
        for _, plugin := range plugins {
            // Check plugin health
            if err := plugin.Initialize(nil); err != nil {
                log.Printf("Plugin %s unhealthy: %v", plugin.Name(), err)
            }
        }
    }
}()
```

## Security Considerations

1. **Plugin Verification**: Verify plugin signatures before loading
2. **Sandboxing**: Run plugins in restricted environments
3. **Resource Limits**: Monitor plugin resource usage
4. **Access Control**: Limit plugin access to system resources

```go
// Example: Verify plugin before loading
func verifyPlugin(path string) error {
    // Check file permissions
    info, err := os.Stat(path)
    if err != nil {
        return err
    }
    
    if info.Mode()&0022 != 0 {
        return fmt.Errorf("plugin file is world-writable")
    }
    
    // Verify digital signature (implement as needed)
    return verifyDigitalSignature(path)
}
```

## Best Practices

1. **Error Handling**: Always handle plugin errors gracefully
2. **Configuration Validation**: Validate plugin configurations
3. **Resource Cleanup**: Ensure plugins clean up resources properly
4. **Documentation**: Document plugin APIs and configurations
5. **Testing**: Test plugins thoroughly in isolation
6. **Versioning**: Use semantic versioning for plugins
7. **Backwards Compatibility**: Maintain API compatibility

## Plugin Development Guidelines

1. **Interface Compliance**: Implement all required interface methods
2. **Thread Safety**: Ensure plugin code is thread-safe
3. **Error Propagation**: Return meaningful error messages
4. **Configuration**: Support flexible configuration options
5. **Metrics**: Expose plugin-specific metrics
6. **Logging**: Use Omni for plugin internal logging
7. **Documentation**: Provide comprehensive plugin documentation

## Troubleshooting

### Plugin Won't Load

```bash
# Check plugin file
file plugin.so
ldd plugin.so  # Check dependencies

# Verify plugin symbol
objdump -t plugin.so | grep OmniPlugin
```

### Plugin Crashes

```go
// Recover from plugin panics
defer func() {
    if r := recover(); r != nil {
        log.Printf("Plugin panic: %v", r)
        // Disable plugin or restart
    }
}()
```

### Performance Issues

```go
// Monitor plugin performance
start := time.Now()
result, err := plugin.SomeMethod()
duration := time.Since(start)

if duration > threshold {
    log.Printf("Plugin %s slow: %v", plugin.Name(), duration)
}
```

For more examples and advanced usage, see the [examples/plugins](../examples/plugins/) directory.