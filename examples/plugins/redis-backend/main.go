// Package main implements a Redis backend plugin for Omni
package main

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wayneeseguin/omni/pkg/omni"
)

// RedisBackendPlugin implements the BackendPluginInterface interface
type RedisBackendPlugin struct {
	initialized bool
	config      map[string]interface{}
}

// RedisBackend implements the Backend interface for Redis
type RedisBackend struct {
	conn          net.Conn
	addr          string
	key           string
	maxEntries    int
	expiration    time.Duration
	atomicSupport bool
}

// Name returns the plugin name
func (p *RedisBackendPlugin) Name() string {
	return "redis-backend"
}

// Version returns the plugin version
func (p *RedisBackendPlugin) Version() string {
	return "1.0.0"
}

// Initialize initializes the plugin with configuration
func (p *RedisBackendPlugin) Initialize(config map[string]interface{}) error {
	p.config = config
	p.initialized = true
	return nil
}

// Shutdown cleans up plugin resources
func (p *RedisBackendPlugin) Shutdown(ctx context.Context) error {
	p.initialized = false
	return nil
}

// CreateBackend creates a new Redis backend instance
func (p *RedisBackendPlugin) CreateBackend(uri string, config map[string]interface{}) (omni.Backend, error) {
	if !p.initialized {
		return nil, fmt.Errorf("plugin not initialized")
	}

	// Parse Redis URI: redis://host:port/db?key=logkey&max=1000&expire=3600
	parsedURL, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("parse Redis URI: %w", err)
	}

	if parsedURL.Scheme != "redis" {
		return nil, fmt.Errorf("unsupported scheme: %s", parsedURL.Scheme)
	}

	addr := parsedURL.Host
	if addr == "" {
		addr = "localhost:6379"
	}

	// Parse query parameters
	query := parsedURL.Query()
	key := query.Get("key")
	if key == "" {
		key = "omni:entries"
	}

	maxEntries := 10000 // default
	if maxStr := query.Get("max"); maxStr != "" {
		if max, err := strconv.Atoi(maxStr); err == nil {
			maxEntries = max
		}
	}

	var expiration time.Duration
	if expireStr := query.Get("expire"); expireStr != "" {
		if exp, err := strconv.Atoi(expireStr); err == nil {
			expiration = time.Duration(exp) * time.Second
		}
	}

	backend := &RedisBackend{
		addr:          addr,
		key:           key,
		maxEntries:    maxEntries,
		expiration:    expiration,
		atomicSupport: true, // Redis supports atomic operations
	}

	// Connect to Redis
	if err := backend.connect(); err != nil {
		return nil, fmt.Errorf("connect to Redis: %w", err)
	}

	return backend, nil
}

// SupportedSchemes returns URI schemes this plugin supports
func (p *RedisBackendPlugin) SupportedSchemes() []string {
	return []string{"redis"}
}

// connect establishes connection to Redis
func (b *RedisBackend) connect() error {
	conn, err := net.DialTimeout("tcp", b.addr, 5*time.Second)
	if err != nil {
		return err
	}

	b.conn = conn
	return nil
}

// Write writes a log entry to Redis
func (b *RedisBackend) Write(entry []byte) (int, error) {
	if b.conn == nil {
		if err := b.connect(); err != nil {
			return 0, err
		}
	}

	// Use Redis LPUSH to add to list and LTRIM to maintain max entries
	timestamp := time.Now().Unix()
	value := fmt.Sprintf("%d:%s", timestamp, string(entry))

	// LPUSH command
	cmd := fmt.Sprintf("*3\r\n$5\r\nLPUSH\r\n$%d\r\n%s\r\n$%d\r\n%s\r\n",
		len(b.key), b.key, len(value), value)

	if _, err := b.conn.Write([]byte(cmd)); err != nil {
		return 0, err
	}

	// Read response
	if err := b.readResponse(); err != nil {
		return 0, err
	}

	// LTRIM to maintain max entries
	if b.maxEntries > 0 {
		trimCmd := fmt.Sprintf("*4\r\n$5\r\nLTRIM\r\n$%d\r\n%s\r\n$1\r\n0\r\n$%d\r\n%d\r\n",
			len(b.key), b.key, len(strconv.Itoa(b.maxEntries-1)), b.maxEntries-1)

		if _, err := b.conn.Write([]byte(trimCmd)); err != nil {
			return 0, err
		}

		if err := b.readResponse(); err != nil {
			return 0, err
		}
	}

	// Set expiration if configured
	if b.expiration > 0 {
		expireCmd := fmt.Sprintf("*3\r\n$6\r\nEXPIRE\r\n$%d\r\n%s\r\n$%d\r\n%d\r\n",
			len(b.key), b.key, len(strconv.Itoa(int(b.expiration.Seconds()))), int(b.expiration.Seconds()))

		if _, err := b.conn.Write([]byte(expireCmd)); err != nil {
			return 0, err
		}

		if err := b.readResponse(); err != nil {
			return 0, err
		}
	}

	return len(entry), nil
}

// readResponse reads and validates Redis response
func (b *RedisBackend) readResponse() error {
	buffer := make([]byte, 1024)
	n, err := b.conn.Read(buffer)
	if err != nil {
		return err
	}

	response := string(buffer[:n])
	if strings.HasPrefix(response, "-") {
		return fmt.Errorf("Redis error: %s", strings.TrimSpace(response[1:]))
	}

	return nil
}

// Flush flushes any buffered data (no-op for Redis)
func (b *RedisBackend) Flush() error {
	// Redis writes are immediate, no buffering
	return nil
}

// Close closes the Redis connection
func (b *RedisBackend) Close() error {
	if b.conn != nil {
		return b.conn.Close()
	}
	return nil
}

// SupportsAtomic returns whether Redis supports atomic writes
func (b *RedisBackend) SupportsAtomic() bool {
	return b.atomicSupport
}

// OmniPlugin is the plugin entry point
var OmniPlugin = &RedisBackendPlugin{}

func main() {
	// Example usage demonstrating the Redis backend plugin
	fmt.Println("Redis Backend Plugin")
	fmt.Printf("Name: %s\n", OmniPlugin.Name())
	fmt.Printf("Version: %s\n", OmniPlugin.Version())
	fmt.Printf("Supported schemes: %v\n", OmniPlugin.SupportedSchemes())

	// Initialize the plugin
	if err := OmniPlugin.Initialize(map[string]interface{}{}); err != nil {
		fmt.Printf("Failed to initialize plugin: %v\n", err)
		return
	}

	// Demo creating a backend (won't connect to Redis in demo mode)
	fmt.Println("\nDemo: Creating Redis backend...")

	// Parse a sample Redis URI
	sampleURI := "redis://localhost:6379/0?key=demo:logs&max=1000&expire=3600"
	fmt.Printf("Sample URI: %s\n", sampleURI)

	// This would normally create a Redis backend, but we'll just demonstrate
	// the parsing logic without actually connecting
	backend, err := OmniPlugin.CreateBackend(sampleURI, map[string]interface{}{})
	if err != nil {
		fmt.Printf("Note: Backend creation failed (Redis not available): %v\n", err)
		fmt.Println("This is expected when Redis is not running.")
	} else {
		fmt.Println("Redis backend created successfully!")

		// Demo writing a log entry
		testEntry := []byte(`{"timestamp":"2023-12-25T10:30:45Z","level":"INFO","message":"Test log entry"}`)

		n, err := backend.Write(testEntry)
		if err != nil {
			fmt.Printf("Failed to write log entry: %v\n", err)
		} else {
			fmt.Printf("Successfully wrote %d bytes to Redis\n", n)
		}

		// Clean up
		_ = backend.Close() //nolint:gosec
	}

	// Shutdown the plugin
	ctx := context.Background()
	if err := OmniPlugin.Shutdown(ctx); err != nil {
		fmt.Printf("Failed to shutdown plugin: %v\n", err)
	} else {
		fmt.Println("Plugin shutdown successfully")
	}
}
