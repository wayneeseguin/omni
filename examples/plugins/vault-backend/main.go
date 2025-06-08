package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/vault/api"
	"github.com/wayneeseguin/omni/pkg/backends"
	"github.com/wayneeseguin/omni/pkg/omni"
	"github.com/wayneeseguin/omni/pkg/plugins"
)

// VaultBackendPlugin implements the BackendPlugin interface for Vault
type VaultBackendPlugin struct {
	mu          sync.RWMutex
	initialized bool
	description string
}

// VaultBackend implements the Backend interface for HashiCorp Vault
type VaultBackend struct {
	client       *api.Client
	kvPath       string
	mountPath    string
	mu           sync.Mutex
	writeCount   uint64
	bytesWritten uint64
}

// Export the plugin instance
var OmniPlugin = &VaultBackendPlugin{
	description: "HashiCorp Vault backend for storing logs in KV secrets engine",
}

// Plugin interface methods

func (p *VaultBackendPlugin) Name() string {
	return "vault-backend"
}

func (p *VaultBackendPlugin) Version() string {
	return "1.0.0"
}

func (p *VaultBackendPlugin) Description() string {
	return p.description
}

func (p *VaultBackendPlugin) Initialize(config map[string]interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.initialized = true
	return nil
}

func (p *VaultBackendPlugin) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.initialized = false
	return nil
}

func (p *VaultBackendPlugin) Health() plugins.HealthStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return plugins.HealthStatus{
		Healthy: p.initialized,
		Message: "Vault backend plugin is operational",
		Details: map[string]interface{}{
			"initialized": p.initialized,
		},
	}
}

// BackendPlugin interface methods

func (p *VaultBackendPlugin) CreateBackend(uri string, config map[string]interface{}) (plugins.Backend, error) {
	parsedURL, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid URI: %w", err)
	}

	if parsedURL.Scheme != "vault" {
		return nil, fmt.Errorf("unsupported scheme: %s", parsedURL.Scheme)
	}

	// Extract connection details from URI
	// Format: vault://token@host:port/path/to/logs
	token := ""
	if parsedURL.User != nil {
		token = parsedURL.User.Username()
	}

	address := fmt.Sprintf("http://%s", parsedURL.Host)
	if parsedURL.Host == "" {
		address = "http://localhost:8200"
	}

	// Path in vault where to store logs
	kvPath := strings.TrimPrefix(parsedURL.Path, "/")
	if kvPath == "" {
		kvPath = "logs"
	}

	// Allow override from config
	if configToken, ok := config["token"].(string); ok && configToken != "" {
		token = configToken
	}
	if configAddr, ok := config["address"].(string); ok && configAddr != "" {
		address = configAddr
	}
	if configPath, ok := config["path"].(string); ok && configPath != "" {
		kvPath = configPath
	}

	// Default mount path
	mountPath := "secret"
	if configMount, ok := config["mount"].(string); ok && configMount != "" {
		mountPath = configMount
	}

	// Create Vault client
	vaultConfig := api.DefaultConfig()
	vaultConfig.Address = address

	client, err := api.NewClient(vaultConfig)
	if err != nil {
		return nil, fmt.Errorf("create vault client: %w", err)
	}

	if token != "" {
		client.SetToken(token)
	}

	backend := &VaultBackend{
		client:    client,
		kvPath:    kvPath,
		mountPath: mountPath,
	}

	// Test connection
	_, err = client.Sys().Health()
	if err != nil {
		return nil, fmt.Errorf("vault health check failed: %w", err)
	}

	return backend, nil
}

func (p *VaultBackendPlugin) SupportedSchemes() []string {
	return []string{"vault"}
}

// Backend interface methods for individual instances

func (vb *VaultBackend) Write(entry []byte) (int, error) {
	vb.mu.Lock()
	defer vb.mu.Unlock()

	// Parse the log entry
	var logData map[string]interface{}
	if err := json.Unmarshal(entry, &logData); err != nil {
		// If not JSON, store as plain text
		logData = map[string]interface{}{
			"message":   string(entry),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
	}

	// Generate a unique key for this log entry
	key := fmt.Sprintf("%s/%d-%d", vb.kvPath, time.Now().UnixNano(), vb.writeCount)

	// Prepare the data for Vault KV v2
	data := map[string]interface{}{
		"data": logData,
		"options": map[string]interface{}{
			"cas": 0,
		},
	}

	// Write to Vault
	path := fmt.Sprintf("%s/data/%s", vb.mountPath, key)
	_, err := vb.client.Logical().Write(path, data)
	if err != nil {
		return 0, fmt.Errorf("write to vault: %w", err)
	}

	vb.writeCount++
	vb.bytesWritten += uint64(len(entry))

	return len(entry), nil
}

func (vb *VaultBackend) Flush() error {
	// Vault writes are immediate, no buffering
	return nil
}

func (vb *VaultBackend) Close() error {
	// Nothing to close for Vault client
	return nil
}

func (vb *VaultBackend) SupportsAtomic() bool {
	// Vault supports atomic operations via CAS
	return true
}

func (vb *VaultBackend) Name() string {
	return "vault-backend"
}

func (vb *VaultBackend) Version() string {
	return "1.0.0"
}

func (vb *VaultBackend) Configure(options map[string]interface{}) error {
	// Configuration is handled during creation
	return nil
}

// Add methods to VaultBackendPlugin to satisfy plugins.Backend interface
func (p *VaultBackendPlugin) Write(entry []byte) (int, error) {
	// This is a plugin factory, not a backend instance
	return 0, fmt.Errorf("write not supported on plugin factory")
}

func (p *VaultBackendPlugin) Flush() error {
	return nil
}

func (p *VaultBackendPlugin) Close() error {
	return p.Shutdown(context.Background())
}

func (p *VaultBackendPlugin) SupportsAtomic() bool {
	return true
}

func (p *VaultBackendPlugin) Configure(options map[string]interface{}) error {
	return p.Initialize(options)
}

// Register the plugin in init
func init() {
	// Register the plugin with omni's plugin system
	if err := backends.RegisterBackendPlugin(OmniPlugin); err != nil {
		// Log error but don't panic - allows use as library
		fmt.Printf("Failed to register vault backend plugin: %v\n", err)
	}
}

// Compile-time interface checks
var (
	_ plugins.Plugin        = (*VaultBackendPlugin)(nil)
	_ plugins.BackendPlugin = (*VaultBackendPlugin)(nil)
	_ plugins.Backend       = (*VaultBackend)(nil)
)

// Helper function to create a logger with vault backend (for testing)
func NewVaultLogger(address, token, mountPath string) (*omni.Omni, error) {
	// Format address for URI
	host := strings.TrimPrefix(address, "http://")
	host = strings.TrimPrefix(host, "https://")
	uri := fmt.Sprintf("vault://%s@%s/logs", token, host)

	// Create logger with vault destination
	logger, err := omni.New(uri)
	if err != nil {
		return nil, fmt.Errorf("create vault logger: %w", err)
	}

	return logger, nil
}

// Main function for standalone execution
func main() {
	fmt.Println("Vault Backend Plugin for Omni")
	fmt.Printf("Version: %s\n", OmniPlugin.Version())
	fmt.Printf("Description: %s\n", OmniPlugin.Description())
	fmt.Printf("Supported schemes: %v\n", OmniPlugin.SupportedSchemes())
}
