package plugins

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/wayneeseguin/omni/pkg/types"
)

// Test implementations of plugin interfaces

// mockPlugin implements the Plugin interface
type mockPlugin struct {
	name         string
	version      string
	description  string
	healthy      bool
	initFunc     func(config map[string]interface{}) error
	shutdownFunc func(ctx context.Context) error
}

func (p *mockPlugin) Name() string {
	return p.name
}

func (p *mockPlugin) Version() string {
	return p.version
}

func (p *mockPlugin) Description() string {
	return p.description
}

func (p *mockPlugin) Initialize(config map[string]interface{}) error {
	if p.initFunc != nil {
		return p.initFunc(config)
	}
	return nil
}

func (p *mockPlugin) Shutdown(ctx context.Context) error {
	if p.shutdownFunc != nil {
		return p.shutdownFunc(ctx)
	}
	return nil
}

func (p *mockPlugin) Health() HealthStatus {
	return HealthStatus{
		Healthy: p.healthy,
		Message: "test health status",
		Details: map[string]interface{}{"test": true},
	}
}

// mockBackend implements the Backend interface
type mockBackend struct {
	name           string
	version        string
	supportsAtomic bool
	writeFunc      func(entry []byte) (int, error)
	flushFunc      func() error
	closeFunc      func() error
	configureFunc  func(options map[string]interface{}) error
}

func (b *mockBackend) Write(entry []byte) (int, error) {
	if b.writeFunc != nil {
		return b.writeFunc(entry)
	}
	return len(entry), nil
}

func (b *mockBackend) Flush() error {
	if b.flushFunc != nil {
		return b.flushFunc()
	}
	return nil
}

func (b *mockBackend) Close() error {
	if b.closeFunc != nil {
		return b.closeFunc()
	}
	return nil
}

func (b *mockBackend) SupportsAtomic() bool {
	return b.supportsAtomic
}

func (b *mockBackend) Name() string {
	return b.name
}

func (b *mockBackend) Version() string {
	return b.version
}

func (b *mockBackend) Configure(options map[string]interface{}) error {
	if b.configureFunc != nil {
		return b.configureFunc(options)
	}
	return nil
}

// mockFilter implements the Filter interface
type mockFilter struct {
	name          string
	shouldLogFunc func(level int, message string, fields map[string]interface{}) bool
	configureFunc func(options map[string]interface{}) error
}

func (f *mockFilter) ShouldLog(level int, message string, fields map[string]interface{}) bool {
	if f.shouldLogFunc != nil {
		return f.shouldLogFunc(level, message, fields)
	}
	return true
}

func (f *mockFilter) Name() string {
	return f.name
}

func (f *mockFilter) Configure(options map[string]interface{}) error {
	if f.configureFunc != nil {
		return f.configureFunc(options)
	}
	return nil
}

// mockFormatter implements the Formatter interface
type mockFormatter struct {
	name          string
	formatFunc    func(msg types.LogMessage) ([]byte, error)
	configureFunc func(options map[string]interface{}) error
}

func (f *mockFormatter) Format(msg types.LogMessage) ([]byte, error) {
	if f.formatFunc != nil {
		return f.formatFunc(msg)
	}
	return []byte("formatted message"), nil
}

func (f *mockFormatter) Name() string {
	return f.name
}

func (f *mockFormatter) Configure(options map[string]interface{}) error {
	if f.configureFunc != nil {
		return f.configureFunc(options)
	}
	return nil
}

// mockBackendPlugin implements the BackendPlugin interface
type mockBackendPlugin struct {
	name              string
	version           string
	description       string
	healthy           bool
	initFunc          func(config map[string]interface{}) error
	shutdownFunc      func(ctx context.Context) error
	schemes           []string
	createBackendFunc func(uri string, config map[string]interface{}) (Backend, error)
	// Backend methods
	supportsAtomic bool
	writeFunc      func(entry []byte) (int, error)
	flushFunc      func() error
	closeFunc      func() error
	configureFunc  func(options map[string]interface{}) error
}

// Plugin interface methods
func (p *mockBackendPlugin) Name() string        { return p.name }
func (p *mockBackendPlugin) Version() string     { return p.version }
func (p *mockBackendPlugin) Description() string { return p.description }
func (p *mockBackendPlugin) Initialize(config map[string]interface{}) error {
	if p.initFunc != nil {
		return p.initFunc(config)
	}
	return nil
}
func (p *mockBackendPlugin) Shutdown(ctx context.Context) error {
	if p.shutdownFunc != nil {
		return p.shutdownFunc(ctx)
	}
	return nil
}
func (p *mockBackendPlugin) Health() HealthStatus {
	return HealthStatus{
		Healthy: p.healthy,
		Message: "test health status",
		Details: map[string]interface{}{"test": true},
	}
}

// Backend interface methods
func (p *mockBackendPlugin) Write(entry []byte) (int, error) {
	if p.writeFunc != nil {
		return p.writeFunc(entry)
	}
	return len(entry), nil
}
func (p *mockBackendPlugin) Flush() error {
	if p.flushFunc != nil {
		return p.flushFunc()
	}
	return nil
}
func (p *mockBackendPlugin) Close() error {
	if p.closeFunc != nil {
		return p.closeFunc()
	}
	return nil
}
func (p *mockBackendPlugin) SupportsAtomic() bool { return p.supportsAtomic }
func (p *mockBackendPlugin) Configure(options map[string]interface{}) error {
	if p.configureFunc != nil {
		return p.configureFunc(options)
	}
	return nil
}

// BackendPlugin specific methods
func (p *mockBackendPlugin) CreateBackend(uri string, config map[string]interface{}) (Backend, error) {
	if p.createBackendFunc != nil {
		return p.createBackendFunc(uri, config)
	}
	return &mockBackend{name: "created-backend"}, nil
}

func (p *mockBackendPlugin) SupportedSchemes() []string {
	return p.schemes
}

// mockFilterPlugin implements the FilterPlugin interface
type mockFilterPlugin struct {
	name             string
	version          string
	description      string
	healthy          bool
	initFunc         func(config map[string]interface{}) error
	shutdownFunc     func(ctx context.Context) error
	filterType       string
	createFilterFunc func(config map[string]interface{}) (types.FilterFunc, error)
	// Filter methods
	shouldLogFunc func(level int, message string, fields map[string]interface{}) bool
	configureFunc func(options map[string]interface{}) error
}

// Plugin interface methods
func (p *mockFilterPlugin) Name() string        { return p.name }
func (p *mockFilterPlugin) Version() string     { return p.version }
func (p *mockFilterPlugin) Description() string { return p.description }
func (p *mockFilterPlugin) Initialize(config map[string]interface{}) error {
	if p.initFunc != nil {
		return p.initFunc(config)
	}
	return nil
}
func (p *mockFilterPlugin) Shutdown(ctx context.Context) error {
	if p.shutdownFunc != nil {
		return p.shutdownFunc(ctx)
	}
	return nil
}
func (p *mockFilterPlugin) Health() HealthStatus {
	return HealthStatus{
		Healthy: p.healthy,
		Message: "test health status",
		Details: map[string]interface{}{"test": true},
	}
}

// Filter interface methods
func (p *mockFilterPlugin) ShouldLog(level int, message string, fields map[string]interface{}) bool {
	if p.shouldLogFunc != nil {
		return p.shouldLogFunc(level, message, fields)
	}
	return true
}
func (p *mockFilterPlugin) Configure(options map[string]interface{}) error {
	if p.configureFunc != nil {
		return p.configureFunc(options)
	}
	return nil
}

// FilterPlugin specific methods
func (p *mockFilterPlugin) CreateFilter(config map[string]interface{}) (types.FilterFunc, error) {
	if p.createFilterFunc != nil {
		return p.createFilterFunc(config)
	}
	return func(level int, message string, fields map[string]interface{}) bool {
		return true
	}, nil
}

func (p *mockFilterPlugin) FilterType() string {
	return p.filterType
}

// mockFormatterPlugin implements the FormatterPlugin interface
type mockFormatterPlugin struct {
	name                string
	version             string
	description         string
	healthy             bool
	initFunc            func(config map[string]interface{}) error
	shutdownFunc        func(ctx context.Context) error
	formatName          string
	createFormatterFunc func(config map[string]interface{}) (Formatter, error)
	// Formatter methods
	formatFunc    func(msg types.LogMessage) ([]byte, error)
	configureFunc func(options map[string]interface{}) error
}

// Plugin interface methods
func (p *mockFormatterPlugin) Name() string        { return p.name }
func (p *mockFormatterPlugin) Version() string     { return p.version }
func (p *mockFormatterPlugin) Description() string { return p.description }
func (p *mockFormatterPlugin) Initialize(config map[string]interface{}) error {
	if p.initFunc != nil {
		return p.initFunc(config)
	}
	return nil
}
func (p *mockFormatterPlugin) Shutdown(ctx context.Context) error {
	if p.shutdownFunc != nil {
		return p.shutdownFunc(ctx)
	}
	return nil
}
func (p *mockFormatterPlugin) Health() HealthStatus {
	return HealthStatus{
		Healthy: p.healthy,
		Message: "test health status",
		Details: map[string]interface{}{"test": true},
	}
}

// Formatter interface methods
func (p *mockFormatterPlugin) Format(msg types.LogMessage) ([]byte, error) {
	if p.formatFunc != nil {
		return p.formatFunc(msg)
	}
	return []byte("formatted message"), nil
}
func (p *mockFormatterPlugin) Configure(options map[string]interface{}) error {
	if p.configureFunc != nil {
		return p.configureFunc(options)
	}
	return nil
}

// FormatterPlugin specific methods
func (p *mockFormatterPlugin) CreateFormatter(config map[string]interface{}) (Formatter, error) {
	if p.createFormatterFunc != nil {
		return p.createFormatterFunc(config)
	}
	return &mockFormatter{name: "created-formatter"}, nil
}

func (p *mockFormatterPlugin) FormatName() string {
	return p.formatName
}

// TestPluginInterface tests the Plugin interface
func TestPluginInterface(t *testing.T) {
	initCalled := false
	shutdownCalled := false

	plugin := &mockPlugin{
		name:        "test-plugin",
		version:     "1.0.0",
		description: "Test plugin description",
		healthy:     true,
		initFunc: func(config map[string]interface{}) error {
			initCalled = true
			return nil
		},
		shutdownFunc: func(ctx context.Context) error {
			shutdownCalled = true
			return nil
		},
	}

	// Test basic methods
	if plugin.Name() != "test-plugin" {
		t.Errorf("Expected name 'test-plugin', got '%s'", plugin.Name())
	}

	if plugin.Version() != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", plugin.Version())
	}

	if plugin.Description() != "Test plugin description" {
		t.Errorf("Expected description 'Test plugin description', got '%s'", plugin.Description())
	}

	// Test Initialize
	err := plugin.Initialize(map[string]interface{}{"key": "value"})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	if !initCalled {
		t.Error("Initialize function was not called")
	}

	// Test Shutdown
	ctx := context.Background()
	err = plugin.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
	if !shutdownCalled {
		t.Error("Shutdown function was not called")
	}

	// Test Health
	health := plugin.Health()
	if !health.Healthy {
		t.Error("Expected healthy status")
	}
	if health.Message != "test health status" {
		t.Errorf("Expected message 'test health status', got '%s'", health.Message)
	}
	if health.Details["test"] != true {
		t.Error("Expected test detail to be true")
	}
}

// TestBackendInterface tests the Backend interface
func TestBackendInterface(t *testing.T) {
	writeCalled := false
	flushCalled := false
	closeCalled := false
	configureCalled := false

	backend := &mockBackend{
		name:           "test-backend",
		version:        "1.0.0",
		supportsAtomic: true,
		writeFunc: func(entry []byte) (int, error) {
			writeCalled = true
			return len(entry), nil
		},
		flushFunc: func() error {
			flushCalled = true
			return nil
		},
		closeFunc: func() error {
			closeCalled = true
			return nil
		},
		configureFunc: func(options map[string]interface{}) error {
			configureCalled = true
			return nil
		},
	}

	// Test Write
	data := []byte("test log entry")
	n, err := backend.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Expected %d bytes written, got %d", len(data), n)
	}
	if !writeCalled {
		t.Error("Write function was not called")
	}

	// Test Flush
	err = backend.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}
	if !flushCalled {
		t.Error("Flush function was not called")
	}

	// Test Close
	err = backend.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if !closeCalled {
		t.Error("Close function was not called")
	}

	// Test SupportsAtomic
	if !backend.SupportsAtomic() {
		t.Error("Expected SupportsAtomic to return true")
	}

	// Test Name and Version
	if backend.Name() != "test-backend" {
		t.Errorf("Expected name 'test-backend', got '%s'", backend.Name())
	}
	if backend.Version() != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", backend.Version())
	}

	// Test Configure
	err = backend.Configure(map[string]interface{}{"option": "value"})
	if err != nil {
		t.Fatalf("Configure failed: %v", err)
	}
	if !configureCalled {
		t.Error("Configure function was not called")
	}
}

// TestFilterInterface tests the Filter interface
func TestFilterInterface(t *testing.T) {
	shouldLogCalled := false
	configureCalled := false

	filter := &mockFilter{
		name: "test-filter",
		shouldLogFunc: func(level int, message string, fields map[string]interface{}) bool {
			shouldLogCalled = true
			return level >= 2 // Only log level 2 and above
		},
		configureFunc: func(options map[string]interface{}) error {
			configureCalled = true
			return nil
		},
	}

	// Test ShouldLog
	tests := []struct {
		level    int
		message  string
		fields   map[string]interface{}
		expected bool
	}{
		{1, "debug message", nil, false},
		{2, "info message", nil, true},
		{3, "warn message", map[string]interface{}{"key": "value"}, true},
	}

	for _, test := range tests {
		result := filter.ShouldLog(test.level, test.message, test.fields)
		if result != test.expected {
			t.Errorf("ShouldLog(%d, %s) = %v, expected %v",
				test.level, test.message, result, test.expected)
		}
	}

	if !shouldLogCalled {
		t.Error("ShouldLog function was not called")
	}

	// Test Name
	if filter.Name() != "test-filter" {
		t.Errorf("Expected name 'test-filter', got '%s'", filter.Name())
	}

	// Test Configure
	err := filter.Configure(map[string]interface{}{"threshold": 2})
	if err != nil {
		t.Fatalf("Configure failed: %v", err)
	}
	if !configureCalled {
		t.Error("Configure function was not called")
	}
}

// TestFormatterInterface tests the Formatter interface
func TestFormatterInterface(t *testing.T) {
	formatCalled := false
	configureCalled := false

	formatter := &mockFormatter{
		name: "test-formatter",
		formatFunc: func(msg types.LogMessage) ([]byte, error) {
			formatCalled = true
			// Convert numeric level to string for display
			levelStr := "INFO"
			if msg.Level == 0 {
				levelStr = "DEBUG"
			} else if msg.Level == 2 {
				levelStr = "WARN"
			} else if msg.Level == 3 {
				levelStr = "ERROR"
			}
			return []byte("[" + levelStr + "] " + msg.Format), nil
		},
		configureFunc: func(options map[string]interface{}) error {
			configureCalled = true
			return nil
		},
	}

	// Test Format
	msg := types.LogMessage{
		Level:     1, // INFO level (numeric)
		Format:    "Test message",
		Timestamp: time.Now(),
	}

	formatted, err := formatter.Format(msg)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	expected := "[INFO] Test message"
	if string(formatted) != expected {
		t.Errorf("Expected '%s', got '%s'", expected, string(formatted))
	}

	if !formatCalled {
		t.Error("Format function was not called")
	}

	// Test Name
	if formatter.Name() != "test-formatter" {
		t.Errorf("Expected name 'test-formatter', got '%s'", formatter.Name())
	}

	// Test Configure
	err = formatter.Configure(map[string]interface{}{"template": "custom"})
	if err != nil {
		t.Fatalf("Configure failed: %v", err)
	}
	if !configureCalled {
		t.Error("Configure function was not called")
	}
}

// TestBackendPluginInterface tests the BackendPlugin interface
func TestBackendPluginInterface(t *testing.T) {
	createBackendCalled := false

	plugin := &mockBackendPlugin{
		name:    "backend-plugin",
		version: "1.0.0",
		schemes: []string{"custom", "special"},
		createBackendFunc: func(uri string, config map[string]interface{}) (Backend, error) {
			createBackendCalled = true
			return &mockBackend{name: "created-backend"}, nil
		},
	}

	// Test SupportedSchemes
	schemes := plugin.SupportedSchemes()
	if len(schemes) != 2 {
		t.Errorf("Expected 2 schemes, got %d", len(schemes))
	}
	if schemes[0] != "custom" || schemes[1] != "special" {
		t.Error("Unexpected schemes")
	}

	// Test CreateBackend
	backend, err := plugin.CreateBackend("custom://localhost", map[string]interface{}{})
	if err != nil {
		t.Fatalf("CreateBackend failed: %v", err)
	}
	if backend == nil {
		t.Fatal("CreateBackend returned nil")
	}
	if !createBackendCalled {
		t.Error("CreateBackend function was not called")
	}
}

// TestFilterPluginInterface tests the FilterPlugin interface
func TestFilterPluginInterface(t *testing.T) {
	createFilterCalled := false

	plugin := &mockFilterPlugin{
		name:       "filter-plugin",
		version:    "1.0.0",
		filterType: "rate-limit",
		createFilterFunc: func(config map[string]interface{}) (types.FilterFunc, error) {
			createFilterCalled = true
			return func(level int, message string, fields map[string]interface{}) bool {
				return level >= 2
			}, nil
		},
	}

	// Test FilterType
	if plugin.FilterType() != "rate-limit" {
		t.Errorf("Expected filter type 'rate-limit', got '%s'", plugin.FilterType())
	}

	// Test CreateFilter
	filterFunc, err := plugin.CreateFilter(map[string]interface{}{"rate": 100})
	if err != nil {
		t.Fatalf("CreateFilter failed: %v", err)
	}
	if filterFunc == nil {
		t.Fatal("CreateFilter returned nil")
	}
	if !createFilterCalled {
		t.Error("CreateFilter function was not called")
	}

	// Test the created filter function
	if !filterFunc(2, "test", nil) {
		t.Error("Filter function should return true for level 2")
	}
	if filterFunc(1, "test", nil) {
		t.Error("Filter function should return false for level 1")
	}
}

// TestFormatterPluginInterface tests the FormatterPlugin interface
func TestFormatterPluginInterface(t *testing.T) {
	createFormatterCalled := false

	plugin := &mockFormatterPlugin{
		name:       "formatter-plugin",
		version:    "1.0.0",
		formatName: "xml",
		createFormatterFunc: func(config map[string]interface{}) (Formatter, error) {
			createFormatterCalled = true
			return &mockFormatter{name: "xml-formatter"}, nil
		},
	}

	// Test FormatName
	if plugin.FormatName() != "xml" {
		t.Errorf("Expected format name 'xml', got '%s'", plugin.FormatName())
	}

	// Test CreateFormatter
	formatter, err := plugin.CreateFormatter(map[string]interface{}{"pretty": true})
	if err != nil {
		t.Fatalf("CreateFormatter failed: %v", err)
	}
	if formatter == nil {
		t.Fatal("CreateFormatter returned nil")
	}
	if !createFormatterCalled {
		t.Error("CreateFormatter function was not called")
	}
}

// TestHealthStatus tests the HealthStatus structure
func TestHealthStatus(t *testing.T) {
	status := HealthStatus{
		Healthy: true,
		Message: "All systems operational",
		Details: map[string]interface{}{
			"uptime":      3600,
			"connections": 42,
			"errors":      0,
		},
	}

	if !status.Healthy {
		t.Error("Expected healthy status")
	}

	if status.Message != "All systems operational" {
		t.Errorf("Expected message 'All systems operational', got '%s'", status.Message)
	}

	if status.Details["uptime"] != 3600 {
		t.Error("Expected uptime detail to be 3600")
	}

	if status.Details["connections"] != 42 {
		t.Error("Expected connections detail to be 42")
	}

	if status.Details["errors"] != 0 {
		t.Error("Expected errors detail to be 0")
	}
}

// TestPluginInfo tests the PluginInfo structure
func TestPluginInfo(t *testing.T) {
	info := PluginInfo{
		Name:        "test-plugin",
		Version:     "2.0.0",
		Type:        "backend",
		Description: "A test backend plugin",
		Status:      "loaded",
		Details: map[string]interface{}{
			"schemes": []string{"http", "https"},
			"author":  "Test Author",
		},
	}

	if info.Name != "test-plugin" {
		t.Errorf("Expected name 'test-plugin', got '%s'", info.Name)
	}

	if info.Version != "2.0.0" {
		t.Errorf("Expected version '2.0.0', got '%s'", info.Version)
	}

	if info.Type != "backend" {
		t.Errorf("Expected type 'backend', got '%s'", info.Type)
	}

	if info.Description != "A test backend plugin" {
		t.Errorf("Expected description 'A test backend plugin', got '%s'", info.Description)
	}

	if info.Status != "loaded" {
		t.Errorf("Expected status 'loaded', got '%s'", info.Status)
	}

	schemes, ok := info.Details["schemes"].([]string)
	if !ok || len(schemes) != 2 {
		t.Error("Expected schemes detail with 2 items")
	}

	author, ok := info.Details["author"].(string)
	if !ok || author != "Test Author" {
		t.Error("Expected author detail to be 'Test Author'")
	}
}

// TestInterfaceErrors tests error handling in interfaces
func TestInterfaceErrors(t *testing.T) {
	t.Run("PluginInitializeError", func(t *testing.T) {
		initErr := errors.New("init error")
		plugin := &mockPlugin{
			initFunc: func(config map[string]interface{}) error {
				return initErr
			},
		}

		err := plugin.Initialize(nil)
		if err != initErr {
			t.Errorf("Expected init error, got %v", err)
		}
	})

	t.Run("PluginShutdownError", func(t *testing.T) {
		shutdownErr := errors.New("shutdown error")
		plugin := &mockPlugin{
			shutdownFunc: func(ctx context.Context) error {
				return shutdownErr
			},
		}

		err := plugin.Shutdown(context.Background())
		if err != shutdownErr {
			t.Errorf("Expected shutdown error, got %v", err)
		}
	})

	t.Run("BackendWriteError", func(t *testing.T) {
		writeErr := errors.New("write error")
		backend := &mockBackend{
			writeFunc: func(entry []byte) (int, error) {
				return 0, writeErr
			},
		}

		_, err := backend.Write([]byte("test"))
		if err != writeErr {
			t.Errorf("Expected write error, got %v", err)
		}
	})

	t.Run("FormatterFormatError", func(t *testing.T) {
		formatErr := errors.New("format error")
		formatter := &mockFormatter{
			formatFunc: func(msg types.LogMessage) ([]byte, error) {
				return nil, formatErr
			},
		}

		_, err := formatter.Format(types.LogMessage{})
		if err != formatErr {
			t.Errorf("Expected format error, got %v", err)
		}
	})

	t.Run("CreateBackendError", func(t *testing.T) {
		createErr := errors.New("create error")
		plugin := &mockBackendPlugin{
			createBackendFunc: func(uri string, config map[string]interface{}) (Backend, error) {
				return nil, createErr
			},
		}

		_, err := plugin.CreateBackend("test://", nil)
		if err != createErr {
			t.Errorf("Expected create error, got %v", err)
		}
	})
}

// TestInterfaceDefaults tests default behavior when functions are nil
func TestInterfaceDefaults(t *testing.T) {
	t.Run("PluginDefaults", func(t *testing.T) {
		plugin := &mockPlugin{}

		// Initialize should succeed with nil func
		err := plugin.Initialize(nil)
		if err != nil {
			t.Errorf("Expected nil error, got %v", err)
		}

		// Shutdown should succeed with nil func
		err = plugin.Shutdown(context.Background())
		if err != nil {
			t.Errorf("Expected nil error, got %v", err)
		}
	})

	t.Run("BackendDefaults", func(t *testing.T) {
		backend := &mockBackend{}

		// Write should succeed with nil func
		n, err := backend.Write([]byte("test"))
		if err != nil {
			t.Errorf("Expected nil error, got %v", err)
		}
		if n != 4 {
			t.Errorf("Expected 4 bytes written, got %d", n)
		}

		// Flush should succeed with nil func
		err = backend.Flush()
		if err != nil {
			t.Errorf("Expected nil error, got %v", err)
		}

		// Close should succeed with nil func
		err = backend.Close()
		if err != nil {
			t.Errorf("Expected nil error, got %v", err)
		}

		// Configure should succeed with nil func
		err = backend.Configure(nil)
		if err != nil {
			t.Errorf("Expected nil error, got %v", err)
		}
	})

	t.Run("FilterDefaults", func(t *testing.T) {
		filter := &mockFilter{}

		// ShouldLog should return true with nil func
		if !filter.ShouldLog(1, "test", nil) {
			t.Error("Expected ShouldLog to return true by default")
		}

		// Configure should succeed with nil func
		err := filter.Configure(nil)
		if err != nil {
			t.Errorf("Expected nil error, got %v", err)
		}
	})

	t.Run("FormatterDefaults", func(t *testing.T) {
		formatter := &mockFormatter{}

		// Format should return default message with nil func
		data, err := formatter.Format(types.LogMessage{})
		if err != nil {
			t.Errorf("Expected nil error, got %v", err)
		}
		if string(data) != "formatted message" {
			t.Errorf("Expected 'formatted message', got '%s'", string(data))
		}

		// Configure should succeed with nil func
		err = formatter.Configure(nil)
		if err != nil {
			t.Errorf("Expected nil error, got %v", err)
		}
	})
}

// TestConcurrentInterfaceAccess tests concurrent access to interface methods
func TestConcurrentInterfaceAccess(t *testing.T) {
	t.Run("ConcurrentPluginMethods", func(t *testing.T) {
		plugin := &mockPlugin{
			name:        "concurrent-plugin",
			version:     "1.0.0",
			description: "Concurrent test plugin",
			healthy:     true,
		}

		var wg sync.WaitGroup

		// Concurrent reads
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = plugin.Name()
				_ = plugin.Version()
				_ = plugin.Description()
				_ = plugin.Health()
			}()
		}

		wg.Wait()
	})

	t.Run("ConcurrentBackendWrites", func(t *testing.T) {
		var writeCount int32
		var mu sync.Mutex

		backend := &mockBackend{
			writeFunc: func(entry []byte) (int, error) {
				mu.Lock()
				defer mu.Unlock()
				writeCount++
				return len(entry), nil
			},
		}

		var wg sync.WaitGroup

		// Concurrent writes
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				data := []byte("concurrent write test")
				_, err := backend.Write(data)
				if err != nil {
					t.Errorf("Write %d failed: %v", id, err)
				}
			}(i)
		}

		wg.Wait()

		if writeCount != 50 {
			t.Errorf("Expected 50 writes, got %d", writeCount)
		}
	})
}

// TestPluginLifecycle tests complete plugin lifecycle
func TestPluginLifecycle(t *testing.T) {
	var lifecycle []string
	var mu sync.Mutex

	addEvent := func(event string) {
		mu.Lock()
		lifecycle = append(lifecycle, event)
		mu.Unlock()
	}

	plugin := &mockPlugin{
		name:    "lifecycle-plugin",
		version: "1.0.0",
		healthy: false, // Start unhealthy
	}

	plugin.initFunc = func(config map[string]interface{}) error {
		addEvent("initialize")
		plugin.healthy = true // Become healthy after init
		return nil
	}

	plugin.shutdownFunc = func(ctx context.Context) error {
		addEvent("shutdown")
		plugin.healthy = false // Become unhealthy after shutdown
		return nil
	}

	// Check initial health
	if plugin.Health().Healthy {
		t.Error("Plugin should start unhealthy")
	}

	// Initialize
	err := plugin.Initialize(map[string]interface{}{"startup": true})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Check health after init
	if !plugin.Health().Healthy {
		t.Error("Plugin should be healthy after initialization")
	}

	// Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = plugin.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	// Check health after shutdown
	if plugin.Health().Healthy {
		t.Error("Plugin should be unhealthy after shutdown")
	}

	// Verify lifecycle events
	mu.Lock()
	defer mu.Unlock()

	if len(lifecycle) != 2 {
		t.Errorf("Expected 2 lifecycle events, got %d", len(lifecycle))
	}

	if lifecycle[0] != "initialize" {
		t.Errorf("Expected first event to be 'initialize', got '%s'", lifecycle[0])
	}

	if lifecycle[1] != "shutdown" {
		t.Errorf("Expected second event to be 'shutdown', got '%s'", lifecycle[1])
	}
}
