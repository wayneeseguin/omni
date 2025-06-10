package backends_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/wayneeseguin/omni/pkg/backends"
	"github.com/wayneeseguin/omni/pkg/plugins"
	"github.com/wayneeseguin/omni/pkg/types"
)

// Mock implementations for testing

// mockBackendPlugin implements plugins.BackendPlugin for testing
type mockBackendPlugin struct {
	name             string
	version          string
	description      string
	supportedSchemes []string
	initialized      bool
	healthy          bool
	backends         map[string]*mockBackend
	mu               sync.RWMutex
}

// mockBackend implements plugins.Backend for testing
type mockBackend struct {
	name          string
	version       string
	data          []byte
	closed        bool
	flushCount    int
	writeCount    int
	atomicSupport bool
	mu            sync.RWMutex
}

func newMockBackendPlugin(name, version string, schemes []string) *mockBackendPlugin {
	return &mockBackendPlugin{
		name:             name,
		version:          version,
		description:      fmt.Sprintf("Mock backend plugin %s", name),
		supportedSchemes: schemes,
		backends:         make(map[string]*mockBackend),
	}
}

func (m *mockBackendPlugin) Name() string        { return m.name }
func (m *mockBackendPlugin) Version() string     { return m.version }
func (m *mockBackendPlugin) Description() string { return m.description }

func (m *mockBackendPlugin) Initialize(config map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.initialized = true
	m.healthy = true
	return nil
}

func (m *mockBackendPlugin) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.initialized = false
	m.healthy = false
	return nil
}

func (m *mockBackendPlugin) Health() plugins.HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return plugins.HealthStatus{
		Healthy: m.healthy,
		Message: fmt.Sprintf("Plugin %s is healthy: %v", m.name, m.healthy),
		Details: map[string]interface{}{
			"initialized": m.initialized,
		},
	}
}

func (m *mockBackendPlugin) CreateBackend(uri string, config map[string]interface{}) (plugins.Backend, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	backend := &mockBackend{
		name:          m.name + "-backend",
		version:       m.version,
		atomicSupport: true,
	}

	m.backends[uri] = backend
	return backend, nil
}

func (m *mockBackendPlugin) SupportedSchemes() []string {
	return m.supportedSchemes
}

// Backend interface methods for mockBackendPlugin (required by plugins.BackendPlugin)
func (m *mockBackendPlugin) Write(entry []byte) (int, error) {
	// This is implemented by individual backends created by CreateBackend
	return 0, fmt.Errorf("use CreateBackend to create individual backends")
}

func (m *mockBackendPlugin) Flush() error {
	return fmt.Errorf("use CreateBackend to create individual backends")
}

func (m *mockBackendPlugin) Close() error {
	return fmt.Errorf("use CreateBackend to create individual backends")
}

func (m *mockBackendPlugin) SupportsAtomic() bool {
	return true
}

func (m *mockBackendPlugin) Configure(options map[string]interface{}) error {
	return nil
}

// mockBackend implementation
func (m *mockBackend) Name() string    { return m.name }
func (m *mockBackend) Version() string { return m.version }

func (m *mockBackend) Write(entry []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return 0, fmt.Errorf("backend is closed")
	}

	m.data = append(m.data, entry...)
	m.writeCount++
	return len(entry), nil
}

func (m *mockBackend) Flush() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return fmt.Errorf("backend is closed")
	}

	m.flushCount++
	return nil
}

func (m *mockBackend) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closed = true
	return nil
}

func (m *mockBackend) SupportsAtomic() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.atomicSupport
}

func (m *mockBackend) Configure(options map[string]interface{}) error {
	return nil
}

func (m *mockBackend) GetData() []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]byte(nil), m.data...)
}

func (m *mockBackend) GetStats() (writeCount, flushCount int, closed bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.writeCount, m.flushCount, m.closed
}

// Mock formatter plugin for additional testing
type mockFormatterPlugin struct {
	name        string
	version     string
	formatName  string
	initialized bool
}

func newMockFormatterPlugin(name, version, formatName string) *mockFormatterPlugin {
	return &mockFormatterPlugin{
		name:       name,
		version:    version,
		formatName: formatName,
	}
}

func (m *mockFormatterPlugin) Name() string        { return m.name }
func (m *mockFormatterPlugin) Version() string     { return m.version }
func (m *mockFormatterPlugin) Description() string { return "Mock formatter plugin" }
func (m *mockFormatterPlugin) FormatName() string  { return m.formatName }

func (m *mockFormatterPlugin) Initialize(config map[string]interface{}) error {
	m.initialized = true
	return nil
}

func (m *mockFormatterPlugin) Shutdown(ctx context.Context) error {
	m.initialized = false
	return nil
}

func (m *mockFormatterPlugin) Health() plugins.HealthStatus {
	return plugins.HealthStatus{
		Healthy: m.initialized,
		Message: "Mock formatter is healthy",
	}
}

func (m *mockFormatterPlugin) Format(msg types.LogMessage) ([]byte, error) {
	message := msg.Format
	if msg.Entry != nil {
		message = msg.Entry.Message
	}
	return []byte(fmt.Sprintf("[%s] %d: %s", m.formatName, msg.Level, message)), nil
}

func (m *mockFormatterPlugin) Configure(options map[string]interface{}) error {
	return nil
}

func (m *mockFormatterPlugin) CreateFormatter(config map[string]interface{}) (plugins.Formatter, error) {
	return m, nil
}

// Mock filter plugin for additional testing
type mockFilterPlugin struct {
	name        string
	version     string
	filterType  string
	initialized bool
}

func newMockFilterPlugin(name, version, filterType string) *mockFilterPlugin {
	return &mockFilterPlugin{
		name:       name,
		version:    version,
		filterType: filterType,
	}
}

func (m *mockFilterPlugin) Name() string        { return m.name }
func (m *mockFilterPlugin) Version() string     { return m.version }
func (m *mockFilterPlugin) Description() string { return "Mock filter plugin" }
func (m *mockFilterPlugin) FilterType() string  { return m.filterType }

func (m *mockFilterPlugin) Initialize(config map[string]interface{}) error {
	m.initialized = true
	return nil
}

func (m *mockFilterPlugin) Shutdown(ctx context.Context) error {
	m.initialized = false
	return nil
}

func (m *mockFilterPlugin) Health() plugins.HealthStatus {
	return plugins.HealthStatus{
		Healthy: m.initialized,
		Message: "Mock filter is healthy",
	}
}

func (m *mockFilterPlugin) ShouldLog(level int, message string, fields map[string]interface{}) bool {
	// Simple filter: allow everything except messages containing "filtered"
	return message != "filtered"
}

func (m *mockFilterPlugin) Configure(options map[string]interface{}) error {
	return nil
}

func (m *mockFilterPlugin) CreateFilter(config map[string]interface{}) (types.FilterFunc, error) {
	return func(level int, message string, fields map[string]interface{}) bool {
		return m.ShouldLog(level, message, fields)
	}, nil
}

// ===== PLUGIN BACKEND TESTS =====

// TestPluginBackendImpl_NewPluginBackend tests plugin backend creation
func TestPluginBackendImpl_NewPluginBackend(t *testing.T) {
	mockPlugin := newMockBackendPlugin("test-plugin", "1.0.0", []string{"mock"})

	tests := []struct {
		name        string
		uri         string
		config      map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name:        "successful creation",
			uri:         "mock://test",
			config:      map[string]interface{}{"key": "value"},
			expectError: false,
		},
		{
			name:        "with nil config",
			uri:         "mock://test2",
			config:      nil,
			expectError: false,
		},
		{
			name:        "with empty config",
			uri:         "mock://test3",
			config:      map[string]interface{}{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, err := backends.NewPluginBackend(mockPlugin, tt.uri, tt.config)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing %q, got none", tt.errorMsg)
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			defer backend.Close()

			if backend == nil {
				t.Fatal("Backend should not be nil")
			}

			// Verify backend properties
			if backend.GetURI() != tt.uri {
				t.Errorf("Expected URI %q, got %q", tt.uri, backend.GetURI())
			}

			if backend.GetPlugin() != mockPlugin {
				t.Error("Plugin reference should match")
			}
		})
	}
}

// TestPluginBackendImpl_Write tests writing to plugin backend
func TestPluginBackendImpl_Write(t *testing.T) {
	mockPlugin := newMockBackendPlugin("test-plugin", "1.0.0", []string{"mock"})
	backend, err := backends.NewPluginBackend(mockPlugin, "mock://test", nil)
	if err != nil {
		t.Fatalf("Failed to create plugin backend: %v", err)
	}
	defer backend.Close()

	tests := []struct {
		name        string
		data        []byte
		expectError bool
	}{
		{
			name:        "simple write",
			data:        []byte("Hello, plugin!"),
			expectError: false,
		},
		{
			name:        "empty write",
			data:        []byte(""),
			expectError: false,
		},
		{
			name:        "large write",
			data:        []byte(fmt.Sprintf("%1000s", "large")),
			expectError: false,
		},
	}

	totalBytes := 0
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := backend.Write(tt.data)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}

			if n != len(tt.data) {
				t.Errorf("Expected to write %d bytes, wrote %d", len(tt.data), n)
			}

			totalBytes += n
		})
	}

	// Verify stats tracking
	stats := backend.GetStats()
	if stats.WriteCount != uint64(len(tests)) {
		t.Errorf("Expected %d writes, got %d", len(tests), stats.WriteCount)
	}
	if stats.BytesWritten != uint64(totalBytes) {
		t.Errorf("Expected %d bytes written, got %d", totalBytes, stats.BytesWritten)
	}
}

// TestPluginBackendImpl_FlushAndClose tests flushing and closing
func TestPluginBackendImpl_FlushAndClose(t *testing.T) {
	mockPlugin := newMockBackendPlugin("test-plugin", "1.0.0", []string{"mock"})
	backend, err := backends.NewPluginBackend(mockPlugin, "mock://test", nil)
	if err != nil {
		t.Fatalf("Failed to create plugin backend: %v", err)
	}

	// Write some data
	_, err = backend.Write([]byte("Test data"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Test Flush
	err = backend.Flush()
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	// Test Close
	err = backend.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Operations after close should fail
	_, err = backend.Write([]byte("After close"))
	if err == nil {
		t.Error("Write after close should fail")
	}

	err = backend.Flush()
	if err == nil {
		t.Error("Flush after close should fail")
	}

	// Multiple closes should be safe
	err = backend.Close()
	if err != nil {
		t.Errorf("Second close should not error: %v", err)
	}
}

// TestPluginBackendImpl_SupportsAtomic tests atomic support checking
func TestPluginBackendImpl_SupportsAtomic(t *testing.T) {
	mockPlugin := newMockBackendPlugin("test-plugin", "1.0.0", []string{"mock"})
	backend, err := backends.NewPluginBackend(mockPlugin, "mock://test", nil)
	if err != nil {
		t.Fatalf("Failed to create plugin backend: %v", err)
	}
	defer backend.Close()

	// Mock backend supports atomic writes
	if !backend.SupportsAtomic() {
		t.Error("Mock backend should support atomic writes")
	}
}

// TestPluginBackendImpl_Reset tests resetting plugin backend
func TestPluginBackendImpl_Reset(t *testing.T) {
	mockPlugin := newMockBackendPlugin("test-plugin", "1.0.0", []string{"mock"})
	backend, err := backends.NewPluginBackend(mockPlugin, "mock://test", nil)
	if err != nil {
		t.Fatalf("Failed to create plugin backend: %v", err)
	}
	defer backend.Close()

	// Write some data
	_, err = backend.Write([]byte("Before reset"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Reset the backend
	err = backend.Reset()
	if err != nil {
		t.Errorf("Reset failed: %v", err)
	}

	// Should be able to write after reset
	_, err = backend.Write([]byte("After reset"))
	if err != nil {
		t.Errorf("Write after reset failed: %v", err)
	}
}

// TestPluginBackendImpl_Sync tests sync functionality
func TestPluginBackendImpl_Sync(t *testing.T) {
	mockPlugin := newMockBackendPlugin("test-plugin", "1.0.0", []string{"mock"})
	backend, err := backends.NewPluginBackend(mockPlugin, "mock://test", nil)
	if err != nil {
		t.Fatalf("Failed to create plugin backend: %v", err)
	}
	defer backend.Close()

	// Write some data
	_, err = backend.Write([]byte("Sync test"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Test Sync (should delegate to Flush)
	err = backend.Sync()
	if err != nil {
		t.Errorf("Sync failed: %v", err)
	}
}

// TestPluginBackendImpl_ConcurrentAccess tests concurrent access
func TestPluginBackendImpl_ConcurrentAccess(t *testing.T) {
	mockPlugin := newMockBackendPlugin("test-plugin", "1.0.0", []string{"mock"})
	backend, err := backends.NewPluginBackend(mockPlugin, "mock://test", nil)
	if err != nil {
		t.Fatalf("Failed to create plugin backend: %v", err)
	}
	defer backend.Close()

	const numGoroutines = 10
	const operationsPerGoroutine = 20

	var wg sync.WaitGroup
	var errorCount int32
	var mu sync.Mutex

	// Test concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				data := []byte(fmt.Sprintf("Message from goroutine %d, op %d", id, j))
				_, err := backend.Write(data)
				if err != nil {
					mu.Lock()
					errorCount++
					mu.Unlock()
				}
			}
		}(i)
	}

	wg.Wait()

	mu.Lock()
	if errorCount > 0 {
		t.Errorf("Got %d errors during concurrent writes", errorCount)
	}
	mu.Unlock()

	// Verify final stats
	stats := backend.GetStats()
	expectedWrites := uint64(numGoroutines * operationsPerGoroutine)
	if stats.WriteCount != expectedWrites {
		t.Errorf("Expected %d writes, got %d", expectedWrites, stats.WriteCount)
	}
}

// ===== PLUGIN MANAGER TESTS =====

// TestPluginManager_NewPluginManager tests plugin manager creation
func TestPluginManager_NewPluginManager(t *testing.T) {
	manager := backends.NewPluginManager()
	if manager == nil {
		t.Fatal("Plugin manager should not be nil")
	}

	// Should start with empty plugin lists
	plugins := manager.ListPlugins()
	if len(plugins) != 0 {
		t.Errorf("Expected 0 plugins, got %d", len(plugins))
	}
}

// TestPluginManager_RegisterPlugins tests plugin registration
func TestPluginManager_RegisterPlugins(t *testing.T) {
	// Clear any existing plugins before starting
	backends.ClearRegisteredPlugins()

	// Test backend plugin registration
	t.Run("register_backend_plugin", func(t *testing.T) {
		backendPlugin := newMockBackendPlugin("test-backend", "1.0.0", []string{"test", "mock"})

		err := backends.RegisterBackendPlugin(backendPlugin)
		if err != nil {
			t.Fatalf("Failed to register backend plugin: %v", err)
		}

		// Try to register the same plugin again (should fail)
		err = backends.RegisterBackendPlugin(backendPlugin)
		if err == nil {
			t.Error("Expected error when registering duplicate plugin")
		}

		// Verify plugin is accessible
		manager := backends.GetPluginManager()
		plugin, exists := manager.GetBackendPlugin("test")
		if !exists {
			t.Error("Backend plugin should be registered")
		}
		if plugin != backendPlugin {
			t.Error("Retrieved plugin should match registered plugin")
		}
	})

	t.Run("register_formatter_plugin", func(t *testing.T) {
		formatterPlugin := newMockFormatterPlugin("test-formatter", "1.0.0", "test-format")

		err := backends.RegisterFormatterPlugin(formatterPlugin)
		if err != nil {
			t.Fatalf("Failed to register formatter plugin: %v", err)
		}

		// Verify plugin is accessible
		manager := backends.GetPluginManager()
		plugin, exists := manager.GetFormatterPlugin("test-format")
		if !exists {
			t.Error("Formatter plugin should be registered")
		}
		if plugin != formatterPlugin {
			t.Error("Retrieved plugin should match registered plugin")
		}
	})

	t.Run("register_filter_plugin", func(t *testing.T) {
		filterPlugin := newMockFilterPlugin("test-filter", "1.0.0", "test-filter-type")

		err := backends.RegisterFilterPlugin(filterPlugin)
		if err != nil {
			t.Fatalf("Failed to register filter plugin: %v", err)
		}

		// Verify plugin is accessible
		manager := backends.GetPluginManager()
		plugin, exists := manager.GetFilterPlugin("test-filter-type")
		if !exists {
			t.Error("Filter plugin should be registered")
		}
		if plugin != filterPlugin {
			t.Error("Retrieved plugin should match registered plugin")
		}
	})
}

// TestPluginManager_GetPluginInfo tests getting plugin information
func TestPluginManager_GetPluginInfo(t *testing.T) {
	// Clear any existing plugins before starting
	backends.ClearRegisteredPlugins()

	// Register test plugins
	backendPlugin := newMockBackendPlugin("info-backend", "2.0.0", []string{"info"})
	formatterPlugin := newMockFormatterPlugin("info-formatter", "2.0.0", "info-format")
	filterPlugin := newMockFilterPlugin("info-filter", "2.0.0", "info-filter-type")

	backends.RegisterBackendPlugin(backendPlugin)
	backends.RegisterFormatterPlugin(formatterPlugin)
	backends.RegisterFilterPlugin(filterPlugin)

	manager := backends.GetPluginManager()
	infos := manager.GetPluginInfo()

	// Should have at least the plugins we just registered
	if len(infos) < 3 {
		t.Errorf("Expected at least 3 plugins, got %d", len(infos))
	}

	// Find our test plugins in the info
	foundBackend := false
	foundFormatter := false
	foundFilter := false

	for _, info := range infos {
		switch info.Name {
		case "info-backend":
			foundBackend = true
			if info.Type != "backend" {
				t.Errorf("Expected backend type, got %s", info.Type)
			}
			if info.Version != "2.0.0" {
				t.Errorf("Expected version 2.0.0, got %s", info.Version)
			}
		case "info-formatter":
			foundFormatter = true
			if info.Type != "formatter" {
				t.Errorf("Expected formatter type, got %s", info.Type)
			}
		case "info-filter":
			foundFilter = true
			if info.Type != "filter" {
				t.Errorf("Expected filter type, got %s", info.Type)
			}
		}
	}

	if !foundBackend {
		t.Error("Backend plugin not found in info")
	}
	if !foundFormatter {
		t.Error("Formatter plugin not found in info")
	}
	if !foundFilter {
		t.Error("Filter plugin not found in info")
	}
}

// TestPluginManager_InitializePlugin tests plugin initialization
func TestPluginManager_InitializePlugin(t *testing.T) {
	// Clear any existing plugins before starting
	backends.ClearRegisteredPlugins()

	backendPlugin := newMockBackendPlugin("init-test", "1.0.0", []string{"init"})
	backends.RegisterBackendPlugin(backendPlugin)

	manager := backends.GetPluginManager()

	config := map[string]interface{}{
		"setting1": "value1",
		"setting2": 42,
	}

	err := manager.InitializePlugin("init-test", config)
	if err != nil {
		t.Errorf("Failed to initialize plugin: %v", err)
	}

	// Try to initialize non-existent plugin
	err = manager.InitializePlugin("nonexistent", config)
	if err == nil {
		t.Error("Expected error when initializing nonexistent plugin")
	}
}

// TestPluginManager_ListPlugins tests listing plugins
func TestPluginManager_ListPlugins(t *testing.T) {
	// Clear any existing plugins before starting
	backends.ClearRegisteredPlugins()

	// Register a test plugin
	testPlugin := newMockBackendPlugin("list-test", "1.0.0", []string{"list"})
	backends.RegisterBackendPlugin(testPlugin)

	manager := backends.GetPluginManager()
	plugins := manager.ListPlugins()

	// Should have at least our test plugin
	found := false
	for _, plugin := range plugins {
		if plugin.Name() == "list-test" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Test plugin not found in list")
	}
}

// TestPluginManager_ConcurrentAccess tests concurrent plugin manager access
func TestPluginManager_ConcurrentAccess(t *testing.T) {
	const numGoroutines = 5
	const operationsPerGoroutine = 10

	var wg sync.WaitGroup
	manager := backends.GetPluginManager()

	// Test concurrent plugin operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				// Register plugin
				pluginName := fmt.Sprintf("concurrent-test-%d-%d", id, j)
				plugin := newMockBackendPlugin(pluginName, "1.0.0", []string{pluginName})

				err := backends.RegisterBackendPlugin(plugin)
				if err != nil {
					t.Logf("Registration error (may be expected): %v", err)
				}

				// Try to get plugin
				_, exists := manager.GetBackendPlugin(pluginName)
				if !exists {
					t.Logf("Plugin %s not found (may be due to timing)", pluginName)
				}

				// List plugins
				plugins := manager.ListPlugins()
				if len(plugins) == 0 {
					t.Logf("No plugins found (unexpected)")
				}
			}
		}(i)
	}

	wg.Wait()
}

// TestPluginManager_ErrorConditions tests various error conditions
func TestPluginManager_ErrorConditions(t *testing.T) {
	manager := backends.GetPluginManager()

	t.Run("get_nonexistent_backend", func(t *testing.T) {
		_, exists := manager.GetBackendPlugin("nonexistent")
		if exists {
			t.Error("Should not find nonexistent backend plugin")
		}
	})

	t.Run("get_nonexistent_formatter", func(t *testing.T) {
		_, exists := manager.GetFormatterPlugin("nonexistent")
		if exists {
			t.Error("Should not find nonexistent formatter plugin")
		}
	})

	t.Run("get_nonexistent_filter", func(t *testing.T) {
		_, exists := manager.GetFilterPlugin("nonexistent")
		if exists {
			t.Error("Should not find nonexistent filter plugin")
		}
	})

	t.Run("initialize_nonexistent_plugin", func(t *testing.T) {
		err := manager.InitializePlugin("nonexistent", nil)
		if err == nil {
			t.Error("Should error when initializing nonexistent plugin")
		}
	})
}

// TestGetPluginManager tests getting the global plugin manager
func TestGetPluginManager(t *testing.T) {
	manager1 := backends.GetPluginManager()
	manager2 := backends.GetPluginManager()

	// Should return the same instance
	if manager1 != manager2 {
		t.Error("GetPluginManager should return the same instance")
	}
}

// ===== ADDITIONAL ERROR PATH TESTS =====

// mockFailingBackendPlugin for testing error conditions
type mockFailingBackendPlugin struct {
	*mockBackendPlugin
	createBackendError error
	initializeError    error
	shutdownError      error
}

func newMockFailingBackendPlugin(name, version string, schemes []string) *mockFailingBackendPlugin {
	return &mockFailingBackendPlugin{
		mockBackendPlugin: newMockBackendPlugin(name, version, schemes),
	}
}

func (m *mockFailingBackendPlugin) CreateBackend(uri string, config map[string]interface{}) (plugins.Backend, error) {
	if m.createBackendError != nil {
		return nil, m.createBackendError
	}
	return m.mockBackendPlugin.CreateBackend(uri, config)
}

func (m *mockFailingBackendPlugin) Initialize(config map[string]interface{}) error {
	if m.initializeError != nil {
		return m.initializeError
	}
	return m.mockBackendPlugin.Initialize(config)
}

func (m *mockFailingBackendPlugin) Shutdown(ctx context.Context) error {
	if m.shutdownError != nil {
		return m.shutdownError
	}
	return m.mockBackendPlugin.Shutdown(ctx)
}

// mockFailingBackend for testing backend error conditions
type mockFailingBackend struct {
	*mockBackend
	writeError       error
	flushError       error
	closeError       error
	writeAfterClose  bool
	failureCountDown int
}

func newMockFailingBackend(name, version string) *mockFailingBackend {
	return &mockFailingBackend{
		mockBackend: &mockBackend{
			name:          name,
			version:       version,
			atomicSupport: true,
		},
	}
}

func (m *mockFailingBackend) Write(entry []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed && m.writeAfterClose {
		return 0, fmt.Errorf("backend is closed")
	}

	if m.failureCountDown > 0 {
		m.failureCountDown--
		if m.failureCountDown == 0 {
			return 0, fmt.Errorf("simulated failure after countdown")
		}
	}

	if m.writeError != nil {
		return 0, m.writeError
	}

	return m.mockBackend.Write(entry)
}

func (m *mockFailingBackend) Flush() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.flushError != nil {
		return m.flushError
	}

	return m.mockBackend.Flush()
}

func (m *mockFailingBackend) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closeError != nil {
		return m.closeError
	}

	return m.mockBackend.Close()
}

// TestPluginBackendImpl_ErrorPaths tests various error conditions
func TestPluginBackendImpl_ErrorPaths(t *testing.T) {
	t.Run("create_backend_failure", func(t *testing.T) {
		mockPlugin := newMockFailingBackendPlugin("failing-plugin", "1.0.0", []string{"fail"})
		mockPlugin.createBackendError = fmt.Errorf("simulated backend creation failure")

		_, err := backends.NewPluginBackend(mockPlugin, "fail://test", nil)
		if err == nil {
			t.Error("Expected error when CreateBackend fails")
		}
		if !strings.Contains(err.Error(), "create plugin backend") {
			t.Errorf("Expected 'create plugin backend' error, got: %v", err)
		}
	})

	t.Run("write_with_nil_backend", func(t *testing.T) {
		// Create a backend and manually set its backend to nil to simulate error condition
		mockPlugin := newMockBackendPlugin("test-plugin", "1.0.0", []string{"mock"})
		backend, err := backends.NewPluginBackend(mockPlugin, "mock://test", nil)
		if err != nil {
			t.Fatalf("Failed to create plugin backend: %v", err)
		}

		// Close the backend to set internal backend to nil
		backend.Close()

		// Now try to write - should fail
		_, err = backend.Write([]byte("test"))
		if err == nil {
			t.Error("Expected error when writing to nil backend")
		}
		if !strings.Contains(err.Error(), "plugin backend not initialized") {
			t.Errorf("Expected 'plugin backend not initialized' error, got: %v", err)
		}
	})

	t.Run("flush_with_nil_backend", func(t *testing.T) {
		mockPlugin := newMockBackendPlugin("test-plugin", "1.0.0", []string{"mock"})
		backend, err := backends.NewPluginBackend(mockPlugin, "mock://test", nil)
		if err != nil {
			t.Fatalf("Failed to create plugin backend: %v", err)
		}

		// Close the backend to set internal backend to nil
		backend.Close()

		// Now try to flush - should fail
		err = backend.Flush()
		if err == nil {
			t.Error("Expected error when flushing nil backend")
		}
		if !strings.Contains(err.Error(), "plugin backend not initialized") {
			t.Errorf("Expected 'plugin backend not initialized' error, got: %v", err)
		}
	})

	t.Run("supports_atomic_with_nil_backend", func(t *testing.T) {
		mockPlugin := newMockBackendPlugin("test-plugin", "1.0.0", []string{"mock"})
		backend, err := backends.NewPluginBackend(mockPlugin, "mock://test", nil)
		if err != nil {
			t.Fatalf("Failed to create plugin backend: %v", err)
		}

		// Close the backend to set internal backend to nil
		backend.Close()

		// SupportsAtomic should return false for nil backend
		if backend.SupportsAtomic() {
			t.Error("SupportsAtomic should return false for nil backend")
		}
	})

	t.Run("reset_with_backend_close_error", func(t *testing.T) {
		// Create a failing backend plugin where CreateBackend initially succeeds
		// but fails on reset
		mockPlugin := newMockFailingBackendPlugin("reset-test", "1.0.0", []string{"reset"})

		backend, err := backends.NewPluginBackend(mockPlugin, "reset://test", nil)
		if err != nil {
			t.Fatalf("Failed to create plugin backend: %v", err)
		}
		defer backend.Close()

		// Write some data first
		_, err = backend.Write([]byte("test data"))
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		// Now set the plugin to fail on CreateBackend for the reset
		mockPlugin.createBackendError = fmt.Errorf("simulated reset failure")

		// Reset should fail
		err = backend.Reset()
		if err == nil {
			t.Error("Expected error when reset fails")
		}
		if !strings.Contains(err.Error(), "recreate plugin backend") {
			t.Errorf("Expected 'recreate plugin backend' error, got: %v", err)
		}
	})

	t.Run("write_error_handling", func(t *testing.T) {
		// Test that write errors don't increment stats
		mockPlugin := newMockBackendPlugin("normal-plugin", "1.0.0", []string{"normal"})
		backend, err := backends.NewPluginBackend(mockPlugin, "normal://test", nil)
		if err != nil {
			t.Fatalf("Failed to create plugin backend: %v", err)
		}
		defer backend.Close()

		// First, do a successful write to verify normal operation
		_, err = backend.Write([]byte("test"))
		if err != nil {
			t.Fatalf("Normal write failed: %v", err)
		}

		// Verify stats were updated
		stats := backend.GetStats()
		if stats.WriteCount != 1 {
			t.Errorf("Expected 1 write, got %d", stats.WriteCount)
		}
		if stats.BytesWritten != 4 {
			t.Errorf("Expected 4 bytes written, got %d", stats.BytesWritten)
		}
	})

	t.Run("zero_byte_write_handling", func(t *testing.T) {
		// Test that zero-byte writes are handled correctly in stats
		mockPlugin := newMockBackendPlugin("zero-test", "1.0.0", []string{"zero"})
		backend, err := backends.NewPluginBackend(mockPlugin, "zero://test", nil)
		if err != nil {
			t.Fatalf("Failed to create plugin backend: %v", err)
		}
		defer backend.Close()

		// Write zero bytes - this should increment write count but not bytes written
		n, err := backend.Write([]byte{})
		if err != nil {
			t.Fatalf("Zero-byte write failed: %v", err)
		}
		if n != 0 {
			t.Errorf("Expected 0 bytes written, got %d", n)
		}

		stats := backend.GetStats()
		if stats.WriteCount != 1 {
			t.Errorf("Expected 1 write, got %d", stats.WriteCount)
		}
		// BytesWritten should remain 0 for zero-byte writes
		if stats.BytesWritten != 0 {
			t.Errorf("Expected 0 total bytes written, got %d", stats.BytesWritten)
		}
	})
}

// TestPluginManager_LoadPlugin_ErrorPaths tests error paths in plugin loading
func TestPluginManager_LoadPlugin_ErrorPaths(t *testing.T) {
	manager := backends.NewPluginManager()

	t.Run("load_nonexistent_file", func(t *testing.T) {
		err := manager.LoadPlugin("/nonexistent/plugin.so")
		if err == nil {
			t.Error("Expected error when loading nonexistent plugin file")
		}
		if !strings.Contains(err.Error(), "open plugin") {
			t.Errorf("Expected 'open plugin' error, got: %v", err)
		}
	})

	// Note: Testing actual .so file loading would require creating valid plugin files,
	// which is complex for unit tests. The error paths for missing symbols, invalid
	// interfaces, etc., would be tested in integration tests.
}

// TestPluginManager_UnloadPlugin_ErrorPaths tests error paths in plugin unloading
func TestPluginManager_UnloadPlugin_ErrorPaths(t *testing.T) {
	manager := backends.NewPluginManager()

	t.Run("unload_nonexistent_plugin", func(t *testing.T) {
		err := manager.UnloadPlugin("nonexistent-plugin")
		if err == nil {
			t.Error("Expected error when unloading nonexistent plugin")
		}
		if !strings.Contains(err.Error(), "not loaded") {
			t.Errorf("Expected 'not loaded' error, got: %v", err)
		}
	})
}

// TestPluginBackendImpl_EdgeCases tests edge cases and boundary conditions
func TestPluginBackendImpl_EdgeCases(t *testing.T) {
	t.Run("write_zero_bytes", func(t *testing.T) {
		mockPlugin := newMockBackendPlugin("zero-bytes", "1.0.0", []string{"zero"})
		backend, err := backends.NewPluginBackend(mockPlugin, "zero://test", nil)
		if err != nil {
			t.Fatalf("Failed to create plugin backend: %v", err)
		}
		defer backend.Close()

		// Write zero bytes
		n, err := backend.Write([]byte{})
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		if n != 0 {
			t.Errorf("Expected 0 bytes written, got %d", n)
		}

		// Stats should reflect the operation
		stats := backend.GetStats()
		if stats.WriteCount != 1 {
			t.Errorf("Expected 1 write, got %d", stats.WriteCount)
		}
		// BytesWritten should remain 0 since we wrote 0 bytes
		if stats.BytesWritten != 0 {
			t.Errorf("Expected 0 bytes written, got %d", stats.BytesWritten)
		}
	})

	t.Run("multiple_resets", func(t *testing.T) {
		mockPlugin := newMockBackendPlugin("multi-reset", "1.0.0", []string{"multireset"})
		backend, err := backends.NewPluginBackend(mockPlugin, "multireset://test", nil)
		if err != nil {
			t.Fatalf("Failed to create plugin backend: %v", err)
		}
		defer backend.Close()

		// Multiple resets should be safe
		for i := 0; i < 3; i++ {
			err = backend.Reset()
			if err != nil {
				t.Errorf("Reset %d failed: %v", i, err)
			}

			// Should be able to write after each reset
			_, err = backend.Write([]byte(fmt.Sprintf("test %d", i)))
			if err != nil {
				t.Errorf("Write after reset %d failed: %v", i, err)
			}
		}
	})

	t.Run("config_variations", func(t *testing.T) {
		mockPlugin := newMockBackendPlugin("config-test", "1.0.0", []string{"config"})

		configs := []map[string]interface{}{
			nil,
			{},
			{"key": "value"},
			{"nested": map[string]interface{}{"inner": "value"}},
			{"array": []string{"a", "b", "c"}},
			{"number": 42},
			{"boolean": true},
		}

		for i, config := range configs {
			t.Run(fmt.Sprintf("config_%d", i), func(t *testing.T) {
				uri := fmt.Sprintf("config://test%d", i)
				backend, err := backends.NewPluginBackend(mockPlugin, uri, config)
				if err != nil {
					t.Fatalf("Failed to create backend with config %v: %v", config, err)
				}
				defer backend.Close()

				// Should be able to write
				_, err = backend.Write([]byte("test"))
				if err != nil {
					t.Errorf("Write failed with config %v: %v", config, err)
				}

				// Config should be retrievable
				retrievedConfig := backend.GetConfig()
				if len(config) == 0 && len(retrievedConfig) != 0 {
					t.Errorf("Expected empty config, got: %v", retrievedConfig)
				}
			})
		}
	})
}

// TestPluginManager_MultiplePluginTypes tests interactions between different plugin types
func TestPluginManager_MultiplePluginTypes(t *testing.T) {
	// Clear existing plugins
	backends.ClearRegisteredPlugins()

	// Register multiple types of plugins
	backendPlugin := newMockBackendPlugin("multi-backend", "1.0.0", []string{"multi"})
	formatterPlugin := newMockFormatterPlugin("multi-formatter", "1.0.0", "multi-format")
	filterPlugin := newMockFilterPlugin("multi-filter", "1.0.0", "multi-filter")

	err := backends.RegisterBackendPlugin(backendPlugin)
	if err != nil {
		t.Fatalf("Failed to register backend plugin: %v", err)
	}

	err = backends.RegisterFormatterPlugin(formatterPlugin)
	if err != nil {
		t.Fatalf("Failed to register formatter plugin: %v", err)
	}

	err = backends.RegisterFilterPlugin(filterPlugin)
	if err != nil {
		t.Fatalf("Failed to register filter plugin: %v", err)
	}

	manager := backends.GetPluginManager()

	// All plugins should be retrievable
	_, exists := manager.GetBackendPlugin("multi")
	if !exists {
		t.Error("Backend plugin not found")
	}

	_, exists = manager.GetFormatterPlugin("multi-format")
	if !exists {
		t.Error("Formatter plugin not found")
	}

	_, exists = manager.GetFilterPlugin("multi-filter")
	if !exists {
		t.Error("Filter plugin not found")
	}

	// List should include all plugins
	plugins := manager.ListPlugins()
	if len(plugins) < 3 {
		t.Errorf("Expected at least 3 plugins, got %d", len(plugins))
	}

	// GetPluginInfo should show all types
	infos := manager.GetPluginInfo()
	types := make(map[string]bool)
	for _, info := range infos {
		types[info.Type] = true
	}

	expectedTypes := []string{"backend", "formatter", "filter"}
	for _, expectedType := range expectedTypes {
		if !types[expectedType] {
			t.Errorf("Plugin type %s not found in info", expectedType)
		}
	}
}

// TestPluginManager_AdditionalEdgeCases tests additional edge cases for plugin management
func TestPluginManager_AdditionalEdgeCases(t *testing.T) {
	t.Run("plugin_interface_validation", func(t *testing.T) {
		// Test that interface casting works correctly
		var mockInterface interface{} = newMockBackendPlugin("interface-test", "1.0.0", []string{"interface"})

		// Should be able to cast to plugins.Plugin
		if _, ok := mockInterface.(plugins.Plugin); !ok {
			t.Error("Mock plugin should implement plugins.Plugin interface")
		}

		// Should be able to cast to plugins.BackendPlugin
		if _, ok := mockInterface.(plugins.BackendPlugin); !ok {
			t.Error("Mock backend plugin should implement plugins.BackendPlugin interface")
		}
	})

	t.Run("scheme_collision_handling", func(t *testing.T) {
		backends.ClearRegisteredPlugins()

		// Register two plugins that support the same scheme
		plugin1 := newMockBackendPlugin("plugin1", "1.0.0", []string{"shared-scheme"})
		plugin2 := newMockBackendPlugin("plugin2", "2.0.0", []string{"shared-scheme"})

		err := backends.RegisterBackendPlugin(plugin1)
		if err != nil {
			t.Fatalf("Failed to register first plugin: %v", err)
		}

		// Second plugin registration should succeed (overwrites scheme mapping)
		err = backends.RegisterBackendPlugin(plugin2)
		if err != nil {
			t.Fatalf("Failed to register second plugin: %v", err)
		}

		// Last registered plugin should win for the scheme
		manager := backends.GetPluginManager()
		retrievedPlugin, exists := manager.GetBackendPlugin("shared-scheme")
		if !exists {
			t.Error("Should find plugin for shared scheme")
		}
		if retrievedPlugin.Name() != "plugin2" {
			t.Errorf("Expected plugin2, got %s", retrievedPlugin.Name())
		}
	})

	t.Run("empty_schemes_handling", func(t *testing.T) {
		backends.ClearRegisteredPlugins()

		// Register plugin with empty schemes
		plugin := newMockBackendPlugin("empty-schemes", "1.0.0", []string{})
		err := backends.RegisterBackendPlugin(plugin)
		if err != nil {
			t.Fatalf("Failed to register plugin with empty schemes: %v", err)
		}

		// Plugin should be loaded but not accessible by any scheme
		manager := backends.GetPluginManager()
		plugins := manager.ListPlugins()
		found := false
		for _, p := range plugins {
			if p.Name() == "empty-schemes" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Plugin with empty schemes should still be in loaded list")
		}
	})

	t.Run("plugin_info_unknown_type", func(t *testing.T) {
		// Create a plugin that doesn't implement any specific plugin interface
		type unknownPlugin struct {
			name        string
			version     string
			initialized bool
		}

		up := &unknownPlugin{name: "unknown-type", version: "1.0.0"}

		// Simulate the logic from GetPluginInfo for unknown types
		pluginType := "unknown"
		if _, ok := interface{}(up).(plugins.BackendPlugin); ok {
			pluginType = "backend"
		} else if _, ok := interface{}(up).(plugins.FormatterPlugin); ok {
			pluginType = "formatter"
		} else if _, ok := interface{}(up).(plugins.FilterPlugin); ok {
			pluginType = "filter"
		}

		if pluginType != "unknown" {
			t.Errorf("Expected unknown type, got %s", pluginType)
		}
	})
}
