package backends_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wayneeseguin/omni/pkg/backends"
)

// ===== PLUGIN DISCOVERY TESTS =====

// TestPluginDiscovery_NewPluginDiscovery tests creating plugin discovery
func TestPluginDiscovery_NewPluginDiscovery(t *testing.T) {
	manager := backends.NewPluginManager()
	discovery := backends.NewPluginDiscovery(manager)

	if discovery == nil {
		t.Fatal("Plugin discovery should not be nil")
	}
}

// TestPluginDiscovery_SearchPaths tests search path management
func TestPluginDiscovery_SearchPaths(t *testing.T) {
	manager := backends.NewPluginManager()
	discovery := backends.NewPluginDiscovery(manager)

	// Test setting search paths
	testPaths := []string{"/test/path1", "/test/path2", "/test/path3"}
	discovery.SetSearchPaths(testPaths)

	// Test adding search path
	additionalPath := "/test/path4"
	discovery.AddSearchPath(additionalPath)

	// We can't directly verify the paths without exposing them,
	// but we can test that the methods don't panic and are accepted
}

// TestPluginDiscovery_SetPattern tests setting file patterns
func TestPluginDiscovery_SetPattern(t *testing.T) {
	manager := backends.NewPluginManager()
	discovery := backends.NewPluginDiscovery(manager)

	patterns := []string{
		"*.so",
		"*.dylib",
		"*.dll",
		"plugin_*.so",
		"libomni_*.so",
	}

	for _, pattern := range patterns {
		discovery.SetPattern(pattern)
		// Pattern setting should not panic
	}
}

// TestPluginDiscovery_DiscoverPlugins tests plugin discovery
func TestPluginDiscovery_DiscoverPlugins(t *testing.T) {
	// Create temporary directory structure for testing
	tempDir := t.TempDir()

	// Create subdirectories
	pluginDir1 := filepath.Join(tempDir, "plugins1")
	pluginDir2 := filepath.Join(tempDir, "plugins2")
	err := os.MkdirAll(pluginDir1, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	err = os.MkdirAll(pluginDir2, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create fake plugin files
	pluginFiles := []string{
		filepath.Join(pluginDir1, "plugin1.so"),
		filepath.Join(pluginDir1, "plugin2.so"),
		filepath.Join(pluginDir2, "plugin3.so"),
		filepath.Join(pluginDir1, "not_a_plugin.txt"), // Should not match *.so pattern
	}

	for _, file := range pluginFiles {
		err := os.WriteFile(file, []byte("fake plugin content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", file, err)
		}
	}

	manager := backends.NewPluginManager()
	discovery := backends.NewPluginDiscovery(manager)
	discovery.SetSearchPaths([]string{pluginDir1, pluginDir2})
	discovery.SetPattern("*.so")

	// Test discovery
	discovered, err := discovery.DiscoverPlugins()
	if err != nil {
		t.Fatalf("Plugin discovery failed: %v", err)
	}

	// Should find 3 .so files
	expectedCount := 3
	if len(discovered) != expectedCount {
		t.Errorf("Expected %d plugins, found %d", expectedCount, len(discovered))
	}

	// Check that discovered files are correct
	expectedFiles := []string{
		filepath.Join(pluginDir1, "plugin1.so"),
		filepath.Join(pluginDir1, "plugin2.so"),
		filepath.Join(pluginDir2, "plugin3.so"),
	}

	for _, expected := range expectedFiles {
		found := false
		for _, discovered := range discovered {
			if discovered == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected file %s not found in discovered plugins", expected)
		}
	}
}

// TestPluginDiscovery_DiscoverPlugins_NonexistentPath tests discovery with nonexistent paths
func TestPluginDiscovery_DiscoverPlugins_NonexistentPath(t *testing.T) {
	manager := backends.NewPluginManager()
	discovery := backends.NewPluginDiscovery(manager)

	// Set paths that don't exist
	discovery.SetSearchPaths([]string{"/nonexistent/path1", "/nonexistent/path2"})
	discovery.SetPattern("*.so")

	// Discovery should still work, just return empty results
	discovered, err := discovery.DiscoverPlugins()
	if err != nil {
		t.Fatalf("Plugin discovery should not fail for nonexistent paths: %v", err)
	}

	if len(discovered) != 0 {
		t.Errorf("Expected 0 plugins from nonexistent paths, found %d", len(discovered))
	}
}

// TestPluginDiscovery_DiscoverPlugins_MixedPaths tests discovery with mixed existing/nonexistent paths
func TestPluginDiscovery_DiscoverPlugins_MixedPaths(t *testing.T) {
	// Create one real directory
	tempDir := t.TempDir()
	pluginDir := filepath.Join(tempDir, "plugins")
	err := os.MkdirAll(pluginDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create a plugin file
	pluginFile := filepath.Join(pluginDir, "real_plugin.so")
	err = os.WriteFile(pluginFile, []byte("fake plugin"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	manager := backends.NewPluginManager()
	discovery := backends.NewPluginDiscovery(manager)

	// Mix real and nonexistent paths
	discovery.SetSearchPaths([]string{
		"/nonexistent/path1",
		pluginDir,
		"/nonexistent/path2",
	})
	discovery.SetPattern("*.so")

	discovered, err := discovery.DiscoverPlugins()
	if err != nil {
		t.Fatalf("Plugin discovery failed: %v", err)
	}

	// Should find the one real plugin
	if len(discovered) != 1 {
		t.Errorf("Expected 1 plugin, found %d", len(discovered))
	}

	if len(discovered) > 0 && discovered[0] != pluginFile {
		t.Errorf("Expected %s, got %s", pluginFile, discovered[0])
	}
}

// TestPluginDiscovery_LoadPluginSpecs tests loading plugins from specifications
func TestPluginDiscovery_LoadPluginSpecs(t *testing.T) {
	// Create temporary directory with fake plugin
	tempDir := t.TempDir()
	pluginDir := filepath.Join(tempDir, "plugins")
	err := os.MkdirAll(pluginDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create fake plugin file
	pluginFile := filepath.Join(pluginDir, "test_plugin.so")
	err = os.WriteFile(pluginFile, []byte("fake plugin"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	manager := backends.NewPluginManager()
	discovery := backends.NewPluginDiscovery(manager)
	discovery.SetSearchPaths([]string{pluginDir})

	tests := []struct {
		name        string
		specs       []backends.PluginSpec
		expectError bool
		errorMsg    string
	}{
		{
			name: "plugin_with_explicit_path",
			specs: []backends.PluginSpec{
				{
					Name: "test_plugin",
					Path: pluginFile,
					Config: map[string]interface{}{
						"setting": "value",
					},
				},
			},
			expectError: true, // Will fail because we can't actually load .so files in tests
			errorMsg:    "",   // Error message will vary
		},
		{
			name: "plugin_search_by_name",
			specs: []backends.PluginSpec{
				{
					Name: "test_plugin", // Will search for test_plugin.so in search paths
				},
			},
			expectError: true, // Will fail because we can't actually load .so files in tests
			errorMsg:    "",
		},
		{
			name: "plugin_with_url",
			specs: []backends.PluginSpec{
				{
					Name: "url_plugin",
					URL:  "https://example.com/plugin.so",
				},
			},
			expectError: true,
			errorMsg:    "URL-based plugin loading not yet implemented",
		},
		{
			name: "nonexistent_plugin",
			specs: []backends.PluginSpec{
				{
					Name: "nonexistent_plugin",
				},
			},
			expectError: true,
			errorMsg:    "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := discovery.LoadPluginSpecs(tt.specs)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// TestPluginDiscovery_ScanForPluginConfigs tests scanning for plugin configuration files
func TestPluginDiscovery_ScanForPluginConfigs(t *testing.T) {
	// Create temporary directory structure
	tempDir := t.TempDir()

	// Create plugin directories with config files
	pluginDir1 := filepath.Join(tempDir, "plugins1")
	pluginDir2 := filepath.Join(tempDir, "plugins2")
	subPluginDir1 := filepath.Join(pluginDir1, "sub1")
	subPluginDir2 := filepath.Join(pluginDir1, "sub2")

	dirs := []string{pluginDir1, pluginDir2, subPluginDir1, subPluginDir2}
	for _, dir := range dirs {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Create configuration files
	configFiles := map[string]string{
		filepath.Join(subPluginDir1, "plugin.json"):    `{"name": "plugin1", "version": "1.0.0"}`,
		filepath.Join(subPluginDir2, "plugin.json"):    `{"name": "plugin2", "version": "2.0.0"}`,
		filepath.Join(pluginDir1, "omni-plugins.json"): `{"plugins": ["plugin1", "plugin2"]}`,
		filepath.Join(pluginDir2, "omni-plugins.json"): `{"plugins": ["plugin3"]}`,
	}

	for file, content := range configFiles {
		err := os.WriteFile(file, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create config file %s: %v", file, err)
		}
	}

	manager := backends.NewPluginManager()
	discovery := backends.NewPluginDiscovery(manager)
	discovery.SetSearchPaths([]string{pluginDir1, pluginDir2})

	configs, err := discovery.ScanForPluginConfigs()
	if err != nil {
		t.Fatalf("Config scan failed: %v", err)
	}

	// Should find plugin.json files and omni-plugins.json files
	expectedCount := 4
	if len(configs) != expectedCount {
		t.Errorf("Expected %d config files, found %d", expectedCount, len(configs))
	}

	// Verify specific files are found
	expectedConfigs := []string{
		filepath.Join(subPluginDir1, "plugin.json"),
		filepath.Join(subPluginDir2, "plugin.json"),
		filepath.Join(pluginDir1, "omni-plugins.json"),
		filepath.Join(pluginDir2, "omni-plugins.json"),
	}

	for _, expected := range expectedConfigs {
		found := false
		for _, config := range configs {
			if config == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected config file %s not found", expected)
		}
	}
}

// TestPluginDiscovery_LoadDiscoveredPlugins tests loading discovered plugins
func TestPluginDiscovery_LoadDiscoveredPlugins(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	pluginDir := filepath.Join(tempDir, "plugins")
	err := os.MkdirAll(pluginDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create fake plugin files
	pluginFiles := []string{
		filepath.Join(pluginDir, "plugin1.so"),
		filepath.Join(pluginDir, "plugin2.so"),
	}

	for _, file := range pluginFiles {
		err := os.WriteFile(file, []byte("fake plugin"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	manager := backends.NewPluginManager()
	discovery := backends.NewPluginDiscovery(manager)
	discovery.SetSearchPaths([]string{pluginDir})
	discovery.SetPattern("*.so")

	// This will fail because we can't actually load .so files in tests,
	// but it tests the discovery and loading attempt logic
	err = discovery.LoadDiscoveredPlugins()
	if err == nil {
		t.Log("LoadDiscoveredPlugins unexpectedly succeeded (maybe no plugins found)")
	} else {
		// Expected to fail - we're just testing that it doesn't panic
		// and follows the correct code paths
		t.Logf("LoadDiscoveredPlugins failed as expected: %v", err)
	}
}

// ===== PLUGIN REGISTRY TESTS =====

// TestPluginRegistry_NewPluginRegistry tests creating plugin registry
func TestPluginRegistry_NewPluginRegistry(t *testing.T) {
	registry := backends.NewPluginRegistry()
	if registry == nil {
		t.Fatal("Plugin registry should not be nil")
	}

	// Should start empty
	plugins := registry.List()
	if len(plugins) != 0 {
		t.Errorf("Expected 0 plugins in new registry, got %d", len(plugins))
	}
}

// TestPluginRegistry_RegisterAndGet tests registering and retrieving plugin metadata
func TestPluginRegistry_RegisterAndGet(t *testing.T) {
	registry := backends.NewPluginRegistry()

	metadata := backends.PluginMetadata{
		Name:         "test-plugin",
		Version:      "1.0.0",
		Description:  "A test plugin",
		Author:       "Test Author",
		License:      "MIT",
		Dependencies: []string{"dep1", "dep2"},
		Config: map[string]interface{}{
			"setting1": "value1",
			"setting2": 42,
		},
	}

	// Register plugin metadata
	registry.Register(metadata)

	// Retrieve plugin metadata
	retrieved, exists := registry.Get("test-plugin")
	if !exists {
		t.Fatal("Plugin metadata should exist after registration")
	}

	// Verify metadata fields
	if retrieved.Name != metadata.Name {
		t.Errorf("Expected name %s, got %s", metadata.Name, retrieved.Name)
	}
	if retrieved.Version != metadata.Version {
		t.Errorf("Expected version %s, got %s", metadata.Version, retrieved.Version)
	}
	if retrieved.Description != metadata.Description {
		t.Errorf("Expected description %s, got %s", metadata.Description, retrieved.Description)
	}
	if retrieved.Author != metadata.Author {
		t.Errorf("Expected author %s, got %s", metadata.Author, retrieved.Author)
	}
	if retrieved.License != metadata.License {
		t.Errorf("Expected license %s, got %s", metadata.License, retrieved.License)
	}

	// Check dependencies
	if len(retrieved.Dependencies) != len(metadata.Dependencies) {
		t.Errorf("Expected %d dependencies, got %d", len(metadata.Dependencies), len(retrieved.Dependencies))
	}

	// Check config
	if len(retrieved.Config) != len(metadata.Config) {
		t.Errorf("Expected %d config items, got %d", len(metadata.Config), len(retrieved.Config))
	}
}

// TestPluginRegistry_GetNonexistent tests retrieving nonexistent plugin metadata
func TestPluginRegistry_GetNonexistent(t *testing.T) {
	registry := backends.NewPluginRegistry()

	_, exists := registry.Get("nonexistent-plugin")
	if exists {
		t.Error("Should not find nonexistent plugin metadata")
	}
}

// TestPluginRegistry_List tests listing plugin metadata
func TestPluginRegistry_List(t *testing.T) {
	registry := backends.NewPluginRegistry()

	// Register multiple plugins
	plugins := []backends.PluginMetadata{
		{
			Name:        "plugin1",
			Version:     "1.0.0",
			Description: "First plugin",
		},
		{
			Name:        "plugin2",
			Version:     "2.0.0",
			Description: "Second plugin",
		},
		{
			Name:        "plugin3",
			Version:     "1.5.0",
			Description: "Third plugin",
		},
	}

	for _, plugin := range plugins {
		registry.Register(plugin)
	}

	// List all plugins
	listed := registry.List()

	if len(listed) != len(plugins) {
		t.Errorf("Expected %d plugins, got %d", len(plugins), len(listed))
	}

	// Verify all plugins are present
	for _, expected := range plugins {
		found := false
		for _, listed := range listed {
			if listed.Name == expected.Name {
				found = true
				if listed.Version != expected.Version {
					t.Errorf("Plugin %s: expected version %s, got %s",
						expected.Name, expected.Version, listed.Version)
				}
				break
			}
		}
		if !found {
			t.Errorf("Plugin %s not found in list", expected.Name)
		}
	}
}

// TestPluginRegistry_OverwriteRegistration tests overwriting plugin registration
func TestPluginRegistry_OverwriteRegistration(t *testing.T) {
	registry := backends.NewPluginRegistry()

	// Register initial plugin
	initial := backends.PluginMetadata{
		Name:        "overwrite-test",
		Version:     "1.0.0",
		Description: "Initial version",
	}
	registry.Register(initial)

	// Register updated plugin with same name
	updated := backends.PluginMetadata{
		Name:        "overwrite-test",
		Version:     "2.0.0",
		Description: "Updated version",
	}
	registry.Register(updated)

	// Should have the updated version
	retrieved, exists := registry.Get("overwrite-test")
	if !exists {
		t.Fatal("Plugin should exist after update")
	}

	if retrieved.Version != "2.0.0" {
		t.Errorf("Expected updated version 2.0.0, got %s", retrieved.Version)
	}
	if retrieved.Description != "Updated version" {
		t.Errorf("Expected updated description, got %s", retrieved.Description)
	}

	// Should still have only one plugin with this name
	listed := registry.List()
	count := 0
	for _, plugin := range listed {
		if plugin.Name == "overwrite-test" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Expected 1 plugin with name 'overwrite-test', found %d", count)
	}
}

// ===== GLOBAL FUNCTIONS TESTS =====

// TestGlobalDiscoveryFunctions tests global discovery functions
func TestGlobalDiscoveryFunctions(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	pluginDir := filepath.Join(tempDir, "global_test")
	err := os.MkdirAll(pluginDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create fake plugin file
	pluginFile := filepath.Join(pluginDir, "global_plugin.so")
	err = os.WriteFile(pluginFile, []byte("fake plugin"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test SetPluginSearchPaths
	backends.SetPluginSearchPaths([]string{pluginDir})

	// Test AddPluginSearchPath
	extraDir := filepath.Join(tempDir, "extra")
	err = os.MkdirAll(extraDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create extra directory: %v", err)
	}
	backends.AddPluginSearchPath(extraDir)

	// Test DiscoverAndLoadPlugins
	// This will likely fail because we can't load .so files in tests,
	// but it tests the code path
	err = backends.DiscoverAndLoadPlugins()
	if err != nil {
		// Expected to fail in test environment
		t.Logf("DiscoverAndLoadPlugins failed as expected: %v", err)
	}
}

// TestGlobalRegistryFunctions tests global registry functions
func TestGlobalRegistryFunctions(t *testing.T) {
	metadata := backends.PluginMetadata{
		Name:        "global-test-plugin",
		Version:     "1.0.0",
		Description: "Global test plugin",
	}

	// Test RegisterPluginMetadata
	backends.RegisterPluginMetadata(metadata)

	// Test GetPluginMetadata
	retrieved, exists := backends.GetPluginMetadata("global-test-plugin")
	if !exists {
		t.Fatal("Plugin metadata should exist after global registration")
	}

	if retrieved.Name != metadata.Name {
		t.Errorf("Expected name %s, got %s", metadata.Name, retrieved.Name)
	}

	// Test ListPluginMetadata
	listed := backends.ListPluginMetadata()
	found := false
	for _, plugin := range listed {
		if plugin.Name == "global-test-plugin" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Global test plugin not found in list")
	}
}

// TestPluginDiscovery_Environment tests environment variable handling
func TestPluginDiscovery_Environment(t *testing.T) {
	// Test with OMNI_PLUGIN_PATH environment variable
	originalEnv := os.Getenv("OMNI_PLUGIN_PATH")
	defer func() {
		if originalEnv != "" {
			os.Setenv("OMNI_PLUGIN_PATH", originalEnv)
		} else {
			os.Unsetenv("OMNI_PLUGIN_PATH")
		}
	}()

	// Create temp directories
	tempDir := t.TempDir()
	envDir1 := filepath.Join(tempDir, "env1")
	envDir2 := filepath.Join(tempDir, "env2")

	for _, dir := range []string{envDir1, envDir2} {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Set environment variable
	envPath := envDir1 + ":" + envDir2 + ": : :" // Include empty entries to test trimming
	os.Setenv("OMNI_PLUGIN_PATH", envPath)

	// Create new discovery (should pick up environment)
	manager := backends.NewPluginManager()
	discovery := backends.NewPluginDiscovery(manager)

	// Test that environment paths are being considered
	// We can't directly verify this without exposing internal state,
	// but we can test that the creation doesn't fail
	if discovery == nil {
		t.Fatal("Discovery should be created successfully with environment")
	}
}
