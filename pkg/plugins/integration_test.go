package plugins

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/wayneeseguin/omni/pkg/types"
)

// TestNewIntegration tests creating a new integration helper
func TestNewIntegration(t *testing.T) {
	manager := NewManager()
	integration := NewIntegration(manager)
	
	if integration == nil {
		t.Fatal("NewIntegration returned nil")
	}
	
	if integration.manager != manager {
		t.Error("Integration manager not set correctly")
	}
}

// TestCreateBackendFromURI tests creating backends from URIs
func TestCreateBackendFromURI(t *testing.T) {
	manager := NewManager()
	integration := NewIntegration(manager)
	
	// Test with no registered plugins
	_, err := integration.CreateBackendFromURI("test://localhost")
	if err == nil {
		t.Error("Expected error for unregistered scheme")
	}
	
	// Register a test backend plugin
	backendCreated := false
	var capturedURI string
	var capturedConfig map[string]interface{}
	
	plugin := &mockBackendPlugin{
		name: "test-backend",
		schemes: []string{"test"},
		createBackendFunc: func(uri string, config map[string]interface{}) (Backend, error) {
			backendCreated = true
			capturedURI = uri
			capturedConfig = config
			return &mockBackend{name: "test-backend-instance"}, nil
		},
	}
	
	err = manager.RegisterBackendPlugin(plugin)
	if err != nil {
		t.Fatalf("Failed to register plugin: %v", err)
	}
	
	// Create backend from URI
	backend, err := integration.CreateBackendFromURI("test://localhost:8080/path?param1=value1&param2=value2")
	if err != nil {
		t.Fatalf("CreateBackendFromURI failed: %v", err)
	}
	
	if backend == nil {
		t.Fatal("Backend is nil")
	}
	
	if !backendCreated {
		t.Error("Backend creation function was not called")
	}
	
	if capturedURI != "test://localhost:8080/path?param1=value1&param2=value2" {
		t.Errorf("Unexpected URI passed to plugin: %s", capturedURI)
	}
	
	// Check captured config from query parameters
	if capturedConfig["param1"] != "value1" {
		t.Error("param1 not extracted from URI")
	}
	if capturedConfig["param2"] != "value2" {
		t.Error("param2 not extracted from URI")
	}
}

// TestCreateBackendFromURIErrors tests error cases
func TestCreateBackendFromURIErrors(t *testing.T) {
	manager := NewManager()
	integration := NewIntegration(manager)
	
	tests := []struct {
		name string
		uri  string
	}{
		{"Invalid URI", "://invalid"},
		{"Missing scheme", "localhost:8080"},
	}
	
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := integration.CreateBackendFromURI(test.uri)
			if err == nil {
				t.Error("Expected error")
			}
		})
	}
	
	// Test backend creation error
	plugin := &mockBackendPlugin{
		name: "error-backend",
		schemes: []string{"error"},
		createBackendFunc: func(uri string, config map[string]interface{}) (Backend, error) {
			return nil, errors.New("backend creation failed")
		},
	}
	
	manager.RegisterBackendPlugin(plugin)
	
	_, err := integration.CreateBackendFromURI("error://localhost")
	if err == nil {
		t.Error("Expected error from backend creation")
	}
}

// TestCreateFormatterByName tests creating formatters by name
func TestCreateFormatterByName(t *testing.T) {
	manager := NewManager()
	integration := NewIntegration(manager)
	
	// Test with no registered plugins
	_, err := integration.CreateFormatterByName("xml", nil)
	if err == nil {
		t.Error("Expected error for unregistered formatter")
	}
	
	// Register a test formatter plugin
	formatterCreated := false
	var capturedConfig map[string]interface{}
	
	plugin := &mockFormatterPlugin{
		name: "xml-formatter",
		formatName: "xml",
		createFormatterFunc: func(config map[string]interface{}) (Formatter, error) {
			formatterCreated = true
			capturedConfig = config
			return &mockFormatter{name: "xml-instance"}, nil
		},
	}
	
	err = manager.RegisterFormatterPlugin(plugin)
	if err != nil {
		t.Fatalf("Failed to register plugin: %v", err)
	}
	
	// Create formatter
	config := map[string]interface{}{"pretty": true, "indent": "  "}
	formatter, err := integration.CreateFormatterByName("xml", config)
	if err != nil {
		t.Fatalf("CreateFormatterByName failed: %v", err)
	}
	
	if formatter == nil {
		t.Fatal("Formatter is nil")
	}
	
	if !formatterCreated {
		t.Error("Formatter creation function was not called")
	}
	
	// Check captured config
	if capturedConfig["pretty"] != true {
		t.Error("pretty config not passed correctly")
	}
	if capturedConfig["indent"] != "  " {
		t.Error("indent config not passed correctly")
	}
}

// TestCreateFilterByType tests creating filters by type
func TestCreateFilterByType(t *testing.T) {
	manager := NewManager()
	integration := NewIntegration(manager)
	
	// Test with no registered plugins
	_, err := integration.CreateFilterByType("rate-limit", nil)
	if err == nil {
		t.Error("Expected error for unregistered filter")
	}
	
	// Register a test filter plugin
	filterCreated := false
	var capturedConfig map[string]interface{}
	
	plugin := &mockFilterPlugin{
		name: "rate-limiter",
		filterType: "rate-limit",
		createFilterFunc: func(config map[string]interface{}) (types.FilterFunc, error) {
			filterCreated = true
			capturedConfig = config
			return func(level int, message string, fields map[string]interface{}) bool {
				return true
			}, nil
		},
	}
	
	err = manager.RegisterFilterPlugin(plugin)
	if err != nil {
		t.Fatalf("Failed to register plugin: %v", err)
	}
	
	// Create filter
	config := map[string]interface{}{"rate": 100, "window": "1m"}
	filter, err := integration.CreateFilterByType("rate-limit", config)
	if err != nil {
		t.Fatalf("CreateFilterByType failed: %v", err)
	}
	
	if filter == nil {
		t.Fatal("Filter is nil")
	}
	
	if !filterCreated {
		t.Error("Filter creation function was not called")
	}
	
	// Check captured config
	if capturedConfig["rate"] != 100 {
		t.Error("rate config not passed correctly")
	}
	if capturedConfig["window"] != "1m" {
		t.Error("window config not passed correctly")
	}
	
	// Test the filter function
	if !filter(1, "test", nil) {
		t.Error("Filter function should return true")
	}
}

// TestShutdownAll tests shutting down all plugins
func TestShutdownAll(t *testing.T) {
	manager := NewManager()
	integration := NewIntegration(manager)
	
	// Test with no plugins
	err := integration.ShutdownAll(context.Background())
	if err != nil {
		t.Errorf("ShutdownAll with no plugins failed: %v", err)
	}
	
	// Register multiple plugins
	shutdownCounts := make(map[string]int)
	var mu sync.Mutex
	
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("plugin-%d", i)
		plugin := &mockPlugin{
			name: name,
			shutdownFunc: func(ctx context.Context) error {
				mu.Lock()
				shutdownCounts[name]++
				mu.Unlock()
				return nil
			},
		}
		manager.loaded[name] = plugin
	}
	
	// Shutdown all
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err = integration.ShutdownAll(ctx)
	if err != nil {
		t.Errorf("ShutdownAll failed: %v", err)
	}
	
	// Verify all plugins were shut down
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("plugin-%d", i)
		if shutdownCounts[name] != 1 {
			t.Errorf("Plugin %s shutdown count = %d, expected 1", name, shutdownCounts[name])
		}
	}
}

// TestShutdownAllWithErrors tests shutdown with plugin errors
func TestShutdownAllWithErrors(t *testing.T) {
	manager := NewManager()
	integration := NewIntegration(manager)
	
	// Register plugins with different behaviors
	plugin1 := &mockPlugin{
		name: "success-plugin",
		shutdownFunc: func(ctx context.Context) error {
			return nil
		},
	}
	
	plugin2 := &mockPlugin{
		name: "error-plugin",
		shutdownFunc: func(ctx context.Context) error {
			return errors.New("shutdown failed")
		},
	}
	
	plugin3 := &mockPlugin{
		name: "another-success",
		shutdownFunc: func(ctx context.Context) error {
			return nil
		},
	}
	
	manager.loaded["success-plugin"] = plugin1
	manager.loaded["error-plugin"] = plugin2
	manager.loaded["another-success"] = plugin3
	
	// Shutdown all
	err := integration.ShutdownAll(context.Background())
	if err == nil {
		t.Error("Expected error when one plugin fails")
	}
	
	// Error should mention the failing plugin
	if err.Error() == "" || !contains(err.Error(), "error-plugin") {
		t.Error("Error should mention the failing plugin")
	}
}

// TestGetAvailableBackends tests listing available backends
func TestGetAvailableBackends(t *testing.T) {
	manager := NewManager()
	integration := NewIntegration(manager)
	
	// Initially should be empty
	backends := integration.GetAvailableBackends()
	if len(backends) != 0 {
		t.Errorf("Expected 0 backends initially, got %d", len(backends))
	}
	
	// Register backend plugins
	plugin1 := &mockBackendPlugin{
		name: "http-backend",
		schemes:    []string{"http", "https"},
	}
	manager.RegisterBackendPlugin(plugin1)
	
	plugin2 := &mockBackendPlugin{
		name: "ftp-backend",
		schemes:    []string{"ftp", "sftp"},
	}
	manager.RegisterBackendPlugin(plugin2)
	
	// Get available backends
	backends = integration.GetAvailableBackends()
	if len(backends) != 4 {
		t.Errorf("Expected 4 backends, got %d", len(backends))
	}
	
	// Check that all schemes are present
	expected := []string{"http", "https", "ftp", "sftp"}
	for _, scheme := range expected {
		found := false
		for _, backend := range backends {
			if backend == scheme {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected scheme %s not found", scheme)
		}
	}
}

// TestGetAvailableFormatters tests listing available formatters
func TestGetAvailableFormatters(t *testing.T) {
	manager := NewManager()
	integration := NewIntegration(manager)
	
	// Initially should be empty
	formatters := integration.GetAvailableFormatters()
	if len(formatters) != 0 {
		t.Errorf("Expected 0 formatters initially, got %d", len(formatters))
	}
	
	// Register formatter plugins
	formats := []string{"xml", "yaml", "csv"}
	for _, format := range formats {
		plugin := &mockFormatterPlugin{
			name: format + "-formatter",
			formatName: format,
		}
		manager.RegisterFormatterPlugin(plugin)
	}
	
	// Get available formatters
	formatters = integration.GetAvailableFormatters()
	if len(formatters) != 3 {
		t.Errorf("Expected 3 formatters, got %d", len(formatters))
	}
	
	// Check that all formats are present
	for _, format := range formats {
		found := false
		for _, formatter := range formatters {
			if formatter == format {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected format %s not found", format)
		}
	}
}

// TestGetAvailableFilters tests listing available filters
func TestGetAvailableFilters(t *testing.T) {
	manager := NewManager()
	integration := NewIntegration(manager)
	
	// Initially should be empty
	filters := integration.GetAvailableFilters()
	if len(filters) != 0 {
		t.Errorf("Expected 0 filters initially, got %d", len(filters))
	}
	
	// Register filter plugins
	filterTypes := []string{"rate-limit", "level", "regex"}
	for _, filterType := range filterTypes {
		plugin := &mockFilterPlugin{
			name: filterType + "-filter",
			filterType: filterType,
		}
		manager.RegisterFilterPlugin(plugin)
	}
	
	// Get available filters
	filters = integration.GetAvailableFilters()
	if len(filters) != 3 {
		t.Errorf("Expected 3 filters, got %d", len(filters))
	}
	
	// Check that all filter types are present
	for _, filterType := range filterTypes {
		found := false
		for _, filter := range filters {
			if filter == filterType {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected filter type %s not found", filterType)
		}
	}
}

// TestValidatePluginHealth tests plugin health validation
func TestValidatePluginHealth(t *testing.T) {
	manager := NewManager()
	integration := NewIntegration(manager)
	
	// Test with healthy plugins
	healthyBackend := &mockBackendPlugin{
		name:    "healthy-backend",
		version: "1.0.0",
		schemes: []string{"test"},
	}
	manager.RegisterBackendPlugin(healthyBackend)
	
	healthyFormatter := &mockFormatterPlugin{
		name:    "healthy-formatter",
		version: "1.0.0",
		formatName: "test",
	}
	manager.RegisterFormatterPlugin(healthyFormatter)
	
	healthyFilter := &mockFilterPlugin{
		name:    "healthy-filter",
		version: "1.0.0",
		filterType: "test",
	}
	manager.RegisterFilterPlugin(healthyFilter)
	
	err := integration.ValidatePluginHealth()
	if err != nil {
		t.Errorf("ValidatePluginHealth failed for healthy plugins: %v", err)
	}
	
	// Add unhealthy plugin with empty name
	unhealthyPlugin := &mockPlugin{
		name:    "", // Empty name
		version: "1.0.0",
	}
	manager.loaded["unhealthy"] = unhealthyPlugin
	
	err = integration.ValidatePluginHealth()
	if err == nil {
		t.Error("Expected error for plugin with empty name")
	}
	
	// Fix the name but add empty version
	unhealthyPlugin.name = "unhealthy"
	unhealthyPlugin.version = ""
	
	err = integration.ValidatePluginHealth()
	if err == nil {
		t.Error("Expected error for plugin with empty version")
	}
	
	// Add backend with no schemes
	badBackend := &mockBackendPlugin{
		name:    "bad-backend",
		version: "1.0.0",
		schemes: []string{}, // No schemes
	}
	manager.loaded["bad-backend"] = badBackend
	
	err = integration.ValidatePluginHealth()
	if err == nil {
		t.Error("Expected error for backend with no schemes")
	}
	
	// Add formatter with empty format name
	badFormatter := &mockFormatterPlugin{
		name:       "bad-formatter",
		version:    "1.0.0",
		formatName: "", // Empty format name
	}
	manager.loaded["bad-formatter"] = badFormatter
	
	err = integration.ValidatePluginHealth()
	if err == nil {
		t.Error("Expected error for formatter with empty format name")
	}
	
	// Add filter with empty filter type
	badFilter := &mockFilterPlugin{
		name:       "bad-filter",
		version:    "1.0.0",
		filterType: "", // Empty filter type
	}
	manager.loaded["bad-filter"] = badFilter
	
	err = integration.ValidatePluginHealth()
	if err == nil {
		t.Error("Expected error for filter with empty filter type")
	}
}

// TestGetCapabilities tests getting plugin capabilities
func TestGetCapabilities(t *testing.T) {
	manager := NewManager()
	integration := NewIntegration(manager)
	
	// Initially should have empty capabilities
	caps := integration.GetCapabilities()
	if caps.PluginCount != 0 {
		t.Errorf("Expected 0 plugins, got %d", caps.PluginCount)
	}
	if len(caps.BackendSchemes) != 0 {
		t.Errorf("Expected 0 backend schemes, got %d", len(caps.BackendSchemes))
	}
	if len(caps.FormatNames) != 0 {
		t.Errorf("Expected 0 format names, got %d", len(caps.FormatNames))
	}
	if len(caps.FilterTypes) != 0 {
		t.Errorf("Expected 0 filter types, got %d", len(caps.FilterTypes))
	}
	
	// Register various plugins
	backendPlugin := &mockBackendPlugin{
		name: "multi-backend",
		schemes:    []string{"http", "https", "ws", "wss"},
	}
	manager.RegisterBackendPlugin(backendPlugin)
	
	formatterPlugin1 := &mockFormatterPlugin{
		name: "xml-formatter",
		formatName: "xml",
	}
	manager.RegisterFormatterPlugin(formatterPlugin1)
	
	formatterPlugin2 := &mockFormatterPlugin{
		name: "yaml-formatter",
		formatName: "yaml",
	}
	manager.RegisterFormatterPlugin(formatterPlugin2)
	
	filterPlugin := &mockFilterPlugin{
		name: "rate-filter",
		filterType: "rate-limit",
	}
	manager.RegisterFilterPlugin(filterPlugin)
	
	// Get capabilities
	caps = integration.GetCapabilities()
	if caps.PluginCount != 4 {
		t.Errorf("Expected 4 plugins, got %d", caps.PluginCount)
	}
	if len(caps.BackendSchemes) != 4 {
		t.Errorf("Expected 4 backend schemes, got %d", len(caps.BackendSchemes))
	}
	if len(caps.FormatNames) != 2 {
		t.Errorf("Expected 2 format names, got %d", len(caps.FormatNames))
	}
	if len(caps.FilterTypes) != 1 {
		t.Errorf("Expected 1 filter type, got %d", len(caps.FilterTypes))
	}
}

// TestIsSupportedMethods tests the support checking methods
func TestIsSupportedMethods(t *testing.T) {
	manager := NewManager()
	integration := NewIntegration(manager)
	
	// Initially nothing should be supported
	if integration.IsBackendSupported("http") {
		t.Error("http should not be supported initially")
	}
	if integration.IsFormatterSupported("xml") {
		t.Error("xml should not be supported initially")
	}
	if integration.IsFilterSupported("rate-limit") {
		t.Error("rate-limit should not be supported initially")
	}
	
	// Register plugins
	backendPlugin := &mockBackendPlugin{
		name: "http-backend",
		schemes:    []string{"http", "https"},
	}
	manager.RegisterBackendPlugin(backendPlugin)
	
	formatterPlugin := &mockFormatterPlugin{
		name: "xml-formatter",
		formatName: "xml",
	}
	manager.RegisterFormatterPlugin(formatterPlugin)
	
	filterPlugin := &mockFilterPlugin{
		name: "rate-filter",
		filterType: "rate-limit",
	}
	manager.RegisterFilterPlugin(filterPlugin)
	
	// Now they should be supported
	if !integration.IsBackendSupported("http") {
		t.Error("http should be supported")
	}
	if !integration.IsBackendSupported("https") {
		t.Error("https should be supported")
	}
	if integration.IsBackendSupported("ftp") {
		t.Error("ftp should not be supported")
	}
	
	if !integration.IsFormatterSupported("xml") {
		t.Error("xml should be supported")
	}
	if integration.IsFormatterSupported("yaml") {
		t.Error("yaml should not be supported")
	}
	
	if !integration.IsFilterSupported("rate-limit") {
		t.Error("rate-limit should be supported")
	}
	if integration.IsFilterSupported("level") {
		t.Error("level should not be supported")
	}
}

// TestCreateDestinationConfig tests destination configuration helpers
func TestCreateDestinationConfig(t *testing.T) {
	// Test empty config
	config := CreateDestinationConfig()
	if len(config) != 0 {
		t.Error("Expected empty config")
	}
	
	// Test with single option
	config = CreateDestinationConfig(WithBatchSize(1000))
	if config["batch_size"] != 1000 {
		t.Errorf("Expected batch_size=1000, got %v", config["batch_size"])
	}
	
	// Test with multiple options
	config = CreateDestinationConfig(
		WithBatchSize(500),
		WithFlushInterval(10),
		WithRetryAttempts(3),
		WithTimeout(30),
		WithCustomConfig("custom_key", "custom_value"),
	)
	
	if config["batch_size"] != 500 {
		t.Errorf("Expected batch_size=500, got %v", config["batch_size"])
	}
	if config["flush_interval"] != 10 {
		t.Errorf("Expected flush_interval=10, got %v", config["flush_interval"])
	}
	if config["retry_attempts"] != 3 {
		t.Errorf("Expected retry_attempts=3, got %v", config["retry_attempts"])
	}
	if config["timeout"] != 30 {
		t.Errorf("Expected timeout=30, got %v", config["timeout"])
	}
	if config["custom_key"] != "custom_value" {
		t.Errorf("Expected custom_key=custom_value, got %v", config["custom_key"])
	}
}

// TestConcurrentIntegrationOperations tests concurrent access to integration
func TestConcurrentIntegrationOperations(t *testing.T) {
	manager := NewManager()
	integration := NewIntegration(manager)
	
	// Register plugins for all types
	for i := 0; i < 5; i++ {
		backendPlugin := &mockBackendPlugin{
			name:    fmt.Sprintf("backend-%d", i),
			schemes: []string{fmt.Sprintf("scheme%d", i)},
		}
		manager.RegisterBackendPlugin(backendPlugin)
		
		formatterPlugin := &mockFormatterPlugin{
			name:       fmt.Sprintf("formatter-%d", i),
			formatName: fmt.Sprintf("format%d", i),
		}
		manager.RegisterFormatterPlugin(formatterPlugin)
		
		filterPlugin := &mockFilterPlugin{
			name:       fmt.Sprintf("filter-%d", i),
			filterType: fmt.Sprintf("type%d", i),
		}
		manager.RegisterFilterPlugin(filterPlugin)
	}
	
	var wg sync.WaitGroup
	
	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			// Get available plugins
			backends := integration.GetAvailableBackends()
			if len(backends) != 5 {
				t.Errorf("Expected 5 backends, got %d", len(backends))
			}
			
			formatters := integration.GetAvailableFormatters()
			if len(formatters) != 5 {
				t.Errorf("Expected 5 formatters, got %d", len(formatters))
			}
			
			filters := integration.GetAvailableFilters()
			if len(filters) != 5 {
				t.Errorf("Expected 5 filters, got %d", len(filters))
			}
			
			// Check support
			scheme := fmt.Sprintf("scheme%d", id%5)
			if !integration.IsBackendSupported(scheme) {
				t.Errorf("Expected %s to be supported", scheme)
			}
			
			format := fmt.Sprintf("format%d", id%5)
			if !integration.IsFormatterSupported(format) {
				t.Errorf("Expected %s to be supported", format)
			}
			
			filterType := fmt.Sprintf("type%d", id%5)
			if !integration.IsFilterSupported(filterType) {
				t.Errorf("Expected %s to be supported", filterType)
			}
			
			// Get capabilities
			caps := integration.GetCapabilities()
			if caps.PluginCount != 15 { // 5 of each type
				t.Errorf("Expected 15 plugins, got %d", caps.PluginCount)
			}
		}(i)
	}
	
	wg.Wait()
}

// TestCreateBackendFromURIWithMultipleValues tests URI parsing with multiple values
func TestCreateBackendFromURIWithMultipleValues(t *testing.T) {
	manager := NewManager()
	integration := NewIntegration(manager)
	
	var capturedConfig map[string]interface{}
	
	plugin := &mockBackendPlugin{
		name: "test-backend",
		schemes:    []string{"test"},
		createBackendFunc: func(uri string, config map[string]interface{}) (Backend, error) {
			capturedConfig = config
			return &mockBackend{}, nil
		},
	}
	
	manager.RegisterBackendPlugin(plugin)
	
	// URI with multiple values for same parameter
	_, err := integration.CreateBackendFromURI("test://localhost?param=value1&param=value2&param=value3")
	if err != nil {
		t.Fatalf("CreateBackendFromURI failed: %v", err)
	}
	
	// Should capture as array
	params, ok := capturedConfig["param"].([]string)
	if !ok {
		t.Fatal("Expected param to be []string")
	}
	
	if len(params) != 3 {
		t.Errorf("Expected 3 values, got %d", len(params))
	}
	
	expected := []string{"value1", "value2", "value3"}
	for i, val := range params {
		if val != expected[i] {
			t.Errorf("Expected param[%d]=%s, got %s", i, expected[i], val)
		}
	}
}

// TestIntegrationErrorPropagation tests error propagation through integration
func TestIntegrationErrorPropagation(t *testing.T) {
	manager := NewManager()
	integration := NewIntegration(manager)
	
	// Test formatter creation error
	formatterErr := errors.New("formatter creation failed")
	formatterPlugin := &mockFormatterPlugin{
		name: "error-formatter",
		formatName: "error",
		createFormatterFunc: func(config map[string]interface{}) (Formatter, error) {
			return nil, formatterErr
		},
	}
	manager.RegisterFormatterPlugin(formatterPlugin)
	
	_, err := integration.CreateFormatterByName("error", nil)
	if err == nil {
		t.Error("Expected error from formatter creation")
	}
	if !contains(err.Error(), "create formatter") {
		t.Error("Error should mention formatter creation")
	}
	
	// Test filter creation error
	filterErr := errors.New("filter creation failed")
	filterPlugin := &mockFilterPlugin{
		name: "error-filter",
		filterType: "error",
		createFilterFunc: func(config map[string]interface{}) (types.FilterFunc, error) {
			return nil, filterErr
		},
	}
	manager.RegisterFilterPlugin(filterPlugin)
	
	_, err = integration.CreateFilterByType("error", nil)
	if err == nil {
		t.Error("Expected error from filter creation")
	}
	if !contains(err.Error(), "create filter") {
		t.Error("Error should mention filter creation")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && s != "" && substr != "" && 
		(s == substr || (len(s) >= len(substr) && (s[:len(substr)] == substr || 
		contains(s[1:], substr))))
}

// TestURIEdgeCases tests edge cases in URI parsing
func TestURIEdgeCases(t *testing.T) {
	manager := NewManager()
	integration := NewIntegration(manager)
	
	tests := []struct {
		name        string
		uri         string
		expectError bool
	}{
		{"Empty URI", "", true},
		{"Just scheme", "test://", false},
		{"Invalid URL encoding", "test://localhost?param=%ZZ", false}, // url.Parse handles this
		{"Special characters", "test://localhost?param=value+with+spaces", false},
		{"Fragment", "test://localhost#fragment", false},
		{"User info", "test://user:pass@localhost", false},
		{"IPv6", "test://[::1]:8080", false},
	}
	
	// Register a test backend
	plugin := &mockBackendPlugin{
		name: "test-backend",
		schemes:    []string{"test"},
		createBackendFunc: func(uri string, config map[string]interface{}) (Backend, error) {
			return &mockBackend{}, nil
		},
	}
	manager.RegisterBackendPlugin(plugin)
	
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := integration.CreateBackendFromURI(test.uri)
			if test.expectError && err == nil {
				t.Error("Expected error")
			} else if !test.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// TestShutdownTimeout tests shutdown with timeout
func TestShutdownTimeout(t *testing.T) {
	manager := NewManager()
	integration := NewIntegration(manager)
	
	// Register a plugin that takes too long to shutdown
	plugin := &mockPlugin{
		name: "slow-plugin",
		shutdownFunc: func(ctx context.Context) error {
			select {
			case <-time.After(10 * time.Second):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}
	manager.loaded["slow-plugin"] = plugin
	
	// Shutdown with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	start := time.Now()
	err := integration.ShutdownAll(ctx)
	elapsed := time.Since(start)
	
	if err == nil {
		t.Error("Expected error due to timeout")
	}
	
	// Should complete quickly due to context timeout
	if elapsed > 500*time.Millisecond {
		t.Error("Shutdown took too long despite timeout")
	}
}

// TestParseURIErrorHandling tests error handling in URI parsing
func TestParseURIErrorHandling(t *testing.T) {
	// Test url.Parse error case
	_, err := url.Parse("://invalid")
	if err == nil {
		t.Error("Expected error from url.Parse for invalid URI")
	}
}
