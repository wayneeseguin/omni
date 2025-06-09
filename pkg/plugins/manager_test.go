package plugins

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// TestNewManager tests creating a new plugin manager
func TestNewManager(t *testing.T) {
	manager := NewManager()
	
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
	
	if manager.backends == nil {
		t.Error("backends map not initialized")
	}
	
	if manager.formatters == nil {
		t.Error("formatters map not initialized")
	}
	
	if manager.filters == nil {
		t.Error("filters map not initialized")
	}
	
	if manager.loaded == nil {
		t.Error("loaded map not initialized")
	}
}

// TestGetBackendPlugin tests getting backend plugins
func TestGetBackendPlugin(t *testing.T) {
	manager := NewManager()
	
	// Test non-existent plugin
	_, exists := manager.GetBackendPlugin("nonexistent")
	if exists {
		t.Error("Expected false for non-existent backend plugin")
	}
	
	// Register a test backend plugin
	testPlugin := &mockBackendPlugin{
		name:    "test-backend",
		schemes: []string{"test", "example"},
	}
	
	err := manager.RegisterBackendPlugin(testPlugin)
	if err != nil {
		t.Fatalf("Failed to register backend plugin: %v", err)
	}
	
	// Test getting registered plugin
	plugin, exists := manager.GetBackendPlugin("test")
	if !exists {
		t.Error("Expected true for registered backend plugin")
	}
	if plugin != testPlugin {
		t.Error("Retrieved plugin does not match registered plugin")
	}
	
	// Test second scheme
	plugin2, exists := manager.GetBackendPlugin("example")
	if !exists {
		t.Error("Expected true for second scheme")
	}
	if plugin2 != testPlugin {
		t.Error("Retrieved plugin does not match for second scheme")
	}
}

// TestGetFormatterPlugin tests getting formatter plugins
func TestGetFormatterPlugin(t *testing.T) {
	manager := NewManager()
	
	// Test non-existent plugin
	_, exists := manager.GetFormatterPlugin("nonexistent")
	if exists {
		t.Error("Expected false for non-existent formatter plugin")
	}
	
	// Register a test formatter plugin
	testPlugin := &mockFormatterPlugin{
		name:       "test-formatter",
		formatName: "xml",
	}
	
	err := manager.RegisterFormatterPlugin(testPlugin)
	if err != nil {
		t.Fatalf("Failed to register formatter plugin: %v", err)
	}
	
	// Test getting registered plugin
	plugin, exists := manager.GetFormatterPlugin("xml")
	if !exists {
		t.Error("Expected true for registered formatter plugin")
	}
	if plugin != testPlugin {
		t.Error("Retrieved plugin does not match registered plugin")
	}
}

// TestGetFilterPlugin tests getting filter plugins
func TestGetFilterPlugin(t *testing.T) {
	manager := NewManager()
	
	// Test non-existent plugin
	_, exists := manager.GetFilterPlugin("nonexistent")
	if exists {
		t.Error("Expected false for non-existent filter plugin")
	}
	
	// Register a test filter plugin
	testPlugin := &mockFilterPlugin{
		name:       "test-filter",
		filterType: "rate-limit",
	}
	
	err := manager.RegisterFilterPlugin(testPlugin)
	if err != nil {
		t.Fatalf("Failed to register filter plugin: %v", err)
	}
	
	// Test getting registered plugin
	plugin, exists := manager.GetFilterPlugin("rate-limit")
	if !exists {
		t.Error("Expected true for registered filter plugin")
	}
	if plugin != testPlugin {
		t.Error("Retrieved plugin does not match registered plugin")
	}
}

// TestListPlugins tests listing all loaded plugins
func TestListPlugins(t *testing.T) {
	manager := NewManager()
	
	// Initially should be empty
	plugins := manager.ListPlugins()
	if len(plugins) != 0 {
		t.Errorf("Expected 0 plugins initially, got %d", len(plugins))
	}
	
	// Register various plugins
	backendPlugin := &mockBackendPlugin{
		name:    "backend1",
		schemes: []string{"scheme1"},
	}
	manager.RegisterBackendPlugin(backendPlugin)
	
	formatterPlugin := &mockFormatterPlugin{
		name:       "formatter1",
		formatName: "format1",
	}
	manager.RegisterFormatterPlugin(formatterPlugin)
	
	filterPlugin := &mockFilterPlugin{
		name:       "filter1",
		filterType: "type1",
	}
	manager.RegisterFilterPlugin(filterPlugin)
	
	// List should now have 3 plugins
	plugins = manager.ListPlugins()
	if len(plugins) != 3 {
		t.Errorf("Expected 3 plugins after registration, got %d", len(plugins))
	}
	
	// Check that all plugins are present
	pluginNames := make(map[string]bool)
	for _, p := range plugins {
		pluginNames[p.Name()] = true
	}
	
	if !pluginNames["backend1"] {
		t.Error("backend1 not found in plugin list")
	}
	if !pluginNames["formatter1"] {
		t.Error("formatter1 not found in plugin list")
	}
	if !pluginNames["filter1"] {
		t.Error("filter1 not found in plugin list")
	}
}

// TestInitializePlugin tests plugin initialization
func TestInitializePlugin(t *testing.T) {
	manager := NewManager()
	
	// Test non-existent plugin
	err := manager.InitializePlugin("nonexistent", nil)
	if err == nil {
		t.Error("Expected error for non-existent plugin")
	}
	
	// Register a test plugin
	initCalled := false
	var initConfig map[string]interface{}
	
	testPlugin := &mockPlugin{
		name: "test-plugin",
		initFunc: func(config map[string]interface{}) error {
			initCalled = true
			initConfig = config
			return nil
		},
	}
	
	manager.loaded["test-plugin"] = testPlugin
	
	// Initialize plugin
	config := map[string]interface{}{"key": "value"}
	err = manager.InitializePlugin("test-plugin", config)
	if err != nil {
		t.Fatalf("InitializePlugin failed: %v", err)
	}
	
	if !initCalled {
		t.Error("Initialize was not called on plugin")
	}
	
	if initConfig["key"] != "value" {
		t.Error("Config was not passed correctly to plugin")
	}
}

// TestGetPluginInfo tests getting plugin information
func TestGetPluginInfo(t *testing.T) {
	manager := NewManager()
	
	// Register different types of plugins
	backendPlugin := &mockBackendPlugin{
		name:    "backend-plugin",
		version: "1.0.0",
		schemes: []string{"http", "https"},
	}
	manager.RegisterBackendPlugin(backendPlugin)
	
	formatterPlugin := &mockFormatterPlugin{
		name:       "formatter-plugin",
		version:    "2.0.0",
		formatName: "xml",
	}
	manager.RegisterFormatterPlugin(formatterPlugin)
	
	filterPlugin := &mockFilterPlugin{
		name:       "filter-plugin",
		version:    "3.0.0",
		filterType: "rate-limit",
	}
	manager.RegisterFilterPlugin(filterPlugin)
	
	// Get plugin info
	infos := manager.GetPluginInfo()
	if len(infos) != 3 {
		t.Errorf("Expected 3 plugin infos, got %d", len(infos))
	}
	
	// Check backend plugin info
	for _, info := range infos {
		if info.Name == "backend-plugin" {
			if info.Type != "backend" {
				t.Errorf("Expected type 'backend', got '%s'", info.Type)
			}
			if info.Version != "1.0.0" {
				t.Errorf("Expected version '1.0.0', got '%s'", info.Version)
			}
			schemes, ok := info.Details["supported_schemes"].([]string)
			if !ok || len(schemes) != 2 {
				t.Error("Expected supported_schemes in details")
			}
		}
		
		if info.Name == "formatter-plugin" {
			if info.Type != "formatter" {
				t.Errorf("Expected type 'formatter', got '%s'", info.Type)
			}
			formatName, ok := info.Details["format_name"].(string)
			if !ok || formatName != "xml" {
				t.Error("Expected format_name in details")
			}
		}
		
		if info.Name == "filter-plugin" {
			if info.Type != "filter" {
				t.Errorf("Expected type 'filter', got '%s'", info.Type)
			}
			filterType, ok := info.Details["filter_type"].(string)
			if !ok || filterType != "rate-limit" {
				t.Error("Expected filter_type in details")
			}
		}
	}
}

// TestUnloadPlugin tests plugin unloading
func TestUnloadPlugin(t *testing.T) {
	manager := NewManager()
	
	// Test unloading non-existent plugin
	err := manager.UnloadPlugin("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent plugin")
	}
	
	// Register a backend plugin
	shutdownCalled := false
	backendPlugin := &mockBackendPlugin{
		name:    "test-backend",
		schemes: []string{"test"},
		shutdownFunc: func(ctx context.Context) error {
			shutdownCalled = true
			return nil
		},
	}
	
	err = manager.RegisterBackendPlugin(backendPlugin)
	if err != nil {
		t.Fatalf("Failed to register plugin: %v", err)
	}
	
	// Verify plugin is registered
	_, exists := manager.GetBackendPlugin("test")
	if !exists {
		t.Error("Plugin should be registered")
	}
	
	// Unload plugin
	err = manager.UnloadPlugin("test-backend")
	if err != nil {
		t.Fatalf("UnloadPlugin failed: %v", err)
	}
	
	if !shutdownCalled {
		t.Error("Shutdown was not called on plugin")
	}
	
	// Verify plugin is removed
	_, exists = manager.GetBackendPlugin("test")
	if exists {
		t.Error("Plugin should be removed after unload")
	}
	
	// Verify plugin is removed from loaded map
	_, exists = manager.loaded["test-backend"]
	if exists {
		t.Error("Plugin should be removed from loaded map")
	}
}

// TestUnloadPluginShutdownError tests unloading with shutdown error
func TestUnloadPluginShutdownError(t *testing.T) {
	manager := NewManager()
	
	shutdownErr := errors.New("shutdown error")
	plugin := &mockPlugin{
		name: "error-plugin",
		shutdownFunc: func(ctx context.Context) error {
			return shutdownErr
		},
	}
	
	manager.loaded["error-plugin"] = plugin
	
	err := manager.UnloadPlugin("error-plugin")
	if err == nil {
		t.Error("Expected error when shutdown fails")
	}
	if !errors.Is(err, shutdownErr) {
		t.Error("Error should wrap shutdown error")
	}
}

// TestRegisterBackendPluginDuplicate tests registering duplicate backend plugin
func TestRegisterBackendPluginDuplicate(t *testing.T) {
	manager := NewManager()
	
	plugin1 := &mockBackendPlugin{
		name:    "duplicate",
		schemes: []string{"test"},
	}
	
	err := manager.RegisterBackendPlugin(plugin1)
	if err != nil {
		t.Fatalf("First registration failed: %v", err)
	}
	
	// Try to register plugin with same name
	plugin2 := &mockBackendPlugin{
		name:    "duplicate",
		schemes: []string{"other"},
	}
	
	err = manager.RegisterBackendPlugin(plugin2)
	if err == nil {
		t.Error("Expected error for duplicate plugin name")
	}
}

// TestRegisterFormatterPluginDuplicate tests registering duplicate formatter plugin
func TestRegisterFormatterPluginDuplicate(t *testing.T) {
	manager := NewManager()
	
	plugin1 := &mockFormatterPlugin{
		name:       "duplicate",
		formatName: "xml",
	}
	
	err := manager.RegisterFormatterPlugin(plugin1)
	if err != nil {
		t.Fatalf("First registration failed: %v", err)
	}
	
	// Try to register plugin with same name
	plugin2 := &mockFormatterPlugin{
		name:       "duplicate",
		formatName: "json",
	}
	
	err = manager.RegisterFormatterPlugin(plugin2)
	if err == nil {
		t.Error("Expected error for duplicate plugin name")
	}
}

// TestRegisterFilterPluginDuplicate tests registering duplicate filter plugin
func TestRegisterFilterPluginDuplicate(t *testing.T) {
	manager := NewManager()
	
	plugin1 := &mockFilterPlugin{
		name:       "duplicate",
		filterType: "type1",
	}
	
	err := manager.RegisterFilterPlugin(plugin1)
	if err != nil {
		t.Fatalf("First registration failed: %v", err)
	}
	
	// Try to register plugin with same name
	plugin2 := &mockFilterPlugin{
		name:       "duplicate",
		filterType: "type2",
	}
	
	err = manager.RegisterFilterPlugin(plugin2)
	if err == nil {
		t.Error("Expected error for duplicate plugin name")
	}
}

// TestConcurrentAccess tests concurrent access to manager
func TestConcurrentAccess(t *testing.T) {
	manager := NewManager()
	
	// Register initial plugins
	for i := 0; i < 10; i++ {
		plugin := &mockBackendPlugin{
			name:    "backend" + string(rune('0'+i)),
			schemes: []string{"scheme" + string(rune('0'+i))},
		}
		manager.RegisterBackendPlugin(plugin)
	}
	
	var wg sync.WaitGroup
	
	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			// Get backend plugin
			scheme := "scheme" + string(rune('0'+(id%10)))
			_, exists := manager.GetBackendPlugin(scheme)
			if !exists {
				t.Errorf("Expected plugin for scheme %s", scheme)
			}
			
			// List plugins
			plugins := manager.ListPlugins()
			if len(plugins) < 10 {
				t.Error("Expected at least 10 plugins")
			}
			
			// Get plugin info
			infos := manager.GetPluginInfo()
			if len(infos) < 10 {
				t.Error("Expected at least 10 plugin infos")
			}
		}(i)
	}
	
	// Concurrent writes (registrations)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			plugin := &mockFormatterPlugin{
				name:       "formatter" + string(rune('0'+id)),
				formatName: "format" + string(rune('0'+id)),
			}
			
			err := manager.RegisterFormatterPlugin(plugin)
			if err != nil {
				t.Errorf("Failed to register formatter: %v", err)
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify all plugins are registered
	plugins := manager.ListPlugins()
	if len(plugins) != 20 { // 10 backends + 10 formatters
		t.Errorf("Expected 20 plugins total, got %d", len(plugins))
	}
}

// TestUnloadWithMultipleSchemes tests unloading backend with multiple schemes
func TestUnloadWithMultipleSchemes(t *testing.T) {
	manager := NewManager()
	
	plugin := &mockBackendPlugin{
		name:    "multi-scheme",
		schemes: []string{"http", "https", "ftp"},
	}
	
	err := manager.RegisterBackendPlugin(plugin)
	if err != nil {
		t.Fatalf("Failed to register plugin: %v", err)
	}
	
	// Verify all schemes are registered
	for _, scheme := range plugin.schemes {
		_, exists := manager.GetBackendPlugin(scheme)
		if !exists {
			t.Errorf("Scheme %s should be registered", scheme)
		}
	}
	
	// Unload plugin
	err = manager.UnloadPlugin("multi-scheme")
	if err != nil {
		t.Fatalf("Failed to unload plugin: %v", err)
	}
	
	// Verify all schemes are removed
	for _, scheme := range plugin.schemes {
		_, exists := manager.GetBackendPlugin(scheme)
		if exists {
			t.Errorf("Scheme %s should be removed", scheme)
		}
	}
}

// TestUnloadPluginTypes tests unloading different plugin types
func TestUnloadPluginTypes(t *testing.T) {
	manager := NewManager()
	
	// Register formatter plugin
	formatterPlugin := &mockFormatterPlugin{
		name:       "test-formatter",
		formatName: "test-format",
	}
	manager.RegisterFormatterPlugin(formatterPlugin)
	
	// Register filter plugin
	filterPlugin := &mockFilterPlugin{
		name:       "test-filter",
		filterType: "test-type",
	}
	manager.RegisterFilterPlugin(filterPlugin)
	
	// Unload formatter
	err := manager.UnloadPlugin("test-formatter")
	if err != nil {
		t.Errorf("Failed to unload formatter: %v", err)
	}
	
	_, exists := manager.GetFormatterPlugin("test-format")
	if exists {
		t.Error("Formatter should be removed")
	}
	
	// Unload filter
	err = manager.UnloadPlugin("test-filter")
	if err != nil {
		t.Errorf("Failed to unload filter: %v", err)
	}
	
	_, exists = manager.GetFilterPlugin("test-type")
	if exists {
		t.Error("Filter should be removed")
	}
}

// TestUnloadTimeout tests plugin unload with timeout
func TestUnloadTimeout(t *testing.T) {
	manager := NewManager()
	
	plugin := &mockPlugin{
		name: "responsive-plugin",
		shutdownFunc: func(ctx context.Context) error {
			// Respond properly to context cancellation
			select {
			case <-time.After(1 * time.Second):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}
	
	manager.loaded["responsive-plugin"] = plugin
	
	// This should complete successfully (no timeout)
	start := time.Now()
	err := manager.UnloadPlugin("responsive-plugin")
	elapsed := time.Since(start)
	
	if err != nil {
		t.Errorf("Expected successful unload, got error: %v", err)
	}
	
	// Should complete in reasonable time (much less than 30s timeout)
	if elapsed > 5*time.Second {
		t.Errorf("Unload took too long: %v", elapsed)
	}
	
	// Verify plugin was removed
	if _, exists := manager.loaded["responsive-plugin"]; exists {
		t.Error("Plugin should have been removed from loaded map")
	}
}

// TestPluginInfoUnknownType tests plugin info for unknown plugin type
func TestPluginInfoUnknownType(t *testing.T) {
	manager := NewManager()
	
	// Register a plain plugin (not backend/formatter/filter)
	plugin := &mockPlugin{
		name:    "unknown-type",
		version: "1.0.0",
	}
	
	manager.loaded["unknown-type"] = plugin
	
	infos := manager.GetPluginInfo()
	if len(infos) != 1 {
		t.Fatalf("Expected 1 plugin info, got %d", len(infos))
	}
	
	info := infos[0]
	if info.Type != "unknown" {
		t.Errorf("Expected type 'unknown', got '%s'", info.Type)
	}
}
