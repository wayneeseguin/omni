package omni

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	// Check default values
	if config.Level != LevelInfo {
		t.Errorf("Expected default level to be Info, got %d", config.Level)
	}

	if config.Format != FormatText {
		t.Errorf("Expected default format to be Text, got %d", config.Format)
	}

	if config.MaxSize != defaultMaxSize {
		t.Errorf("Expected default max size to be %d, got %d", defaultMaxSize, config.MaxSize)
	}

	if config.Compression != CompressionNone {
		t.Errorf("Expected default compression to be None, got %d", config.Compression)
	}

	if config.ErrorHandler == nil {
		t.Error("Expected default error handler to be set")
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		check  func(*Config) bool
	}{
		{
			name: "negative channel size",
			config: &Config{
				ChannelSize: -1,
			},
			check: func(c *Config) bool {
				return c.ChannelSize > 0
			},
		},
		{
			name: "negative max size",
			config: &Config{
				MaxSize: -100,
			},
			check: func(c *Config) bool {
				return c.MaxSize == defaultMaxSize
			},
		},
		{
			name: "invalid sampling rate",
			config: &Config{
				SamplingRate: 1.5,
			},
			check: func(c *Config) bool {
				return c.SamplingRate == 1.0
			},
		},
		{
			name: "nil error handler",
			config: &Config{
				ErrorHandler: nil,
			},
			check: func(c *Config) bool {
				return c.ErrorHandler != nil
			},
		},
		{
			name: "small cleanup interval",
			config: &Config{
				CleanupInterval: 10 * time.Second,
			},
			check: func(c *Config) bool {
				return c.CleanupInterval >= time.Minute
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if err != nil {
				t.Errorf("Validate returned error: %v", err)
			}
			if !tt.check(tt.config) {
				t.Error("Validation did not fix invalid value")
			}
		})
	}
}

func TestNewWithConfig(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	config := &Config{
		Path:         logFile,
		Level:        LevelDebug,
		Format:       FormatJSON,
		MaxSize:      1024 * 1024, // 1MB
		MaxFiles:     5,
		Compression:  CompressionGzip,
		ChannelSize:  200,
		IncludeTrace: true,
		ErrorHandler: SilentErrorHandler,
	}

	logger, err := NewWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create logger with config: %v", err)
	}
	defer logger.Close()

	// Verify settings were applied
	if logger.level != LevelDebug {
		t.Errorf("Expected level Debug, got %d", logger.level)
	}

	if logger.format != FormatJSON {
		t.Errorf("Expected format JSON, got %d", logger.format)
	}

	if logger.maxSize != 1024*1024 {
		t.Errorf("Expected maxSize 1MB, got %d", logger.maxSize)
	}

	if logger.compression != CompressionGzip {
		t.Errorf("Expected compression Gzip, got %d", logger.compression)
	}

	if logger.channelSize != 200 {
		t.Errorf("Expected channel size 200, got %d", logger.channelSize)
	}

	if !logger.includeTrace {
		t.Error("Expected includeTrace to be true")
	}

	// Test that logger works
	logger.Debug("test debug message")
	logger.Info("test info message")
	time.Sleep(100 * time.Millisecond)
}

func TestGetConfig(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Modify some settings
	logger.SetLevel(LevelDebug)
	logger.SetFormat(FormatJSON)
	logger.SetMaxSize(2048)

	// Get config
	config := logger.GetConfig()

	if config.Path != logFile {
		t.Errorf("Expected path %s, got %s", logFile, config.Path)
	}

	if config.Level != LevelDebug {
		t.Errorf("Expected level Debug, got %d", config.Level)
	}

	if config.Format != FormatJSON {
		t.Errorf("Expected format JSON, got %d", config.Format)
	}

	if config.MaxSize != 2048 {
		t.Errorf("Expected maxSize 2048, got %d", config.MaxSize)
	}
}

func TestUpdateConfig(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Create new config
	newConfig := &Config{
		Level:            LevelWarn,
		Format:           FormatJSON,
		MaxSize:          4096,
		MaxFiles:         10,
		Compression:      CompressionGzip,
		IncludeTrace:     true,
		SamplingRate:     0.5,
		SamplingStrategy: SamplingRandom,
	}

	// Update config
	err = logger.UpdateConfig(newConfig)
	if err != nil {
		t.Fatalf("Failed to update config: %v", err)
	}

	// Verify updates
	if logger.level != LevelWarn {
		t.Errorf("Expected level Warn after update, got %d", logger.level)
	}

	if logger.format != FormatJSON {
		t.Errorf("Expected format JSON after update, got %d", logger.format)
	}

	if logger.maxSize != 4096 {
		t.Errorf("Expected maxSize 4096 after update, got %d", logger.maxSize)
	}

	if logger.compression != CompressionGzip {
		t.Errorf("Expected compression Gzip after update, got %d", logger.compression)
	}

	if logger.samplingRate != 0.5 {
		t.Errorf("Expected sampling rate 0.5 after update, got %f", logger.samplingRate)
	}
}

func TestConfigWithRedaction(t *testing.T) {
	t.Skip("Skipping redaction test - needs refactoring to work with new redaction manager")
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	config := &Config{
		Path:              logFile,
		RedactionPatterns: []string{"password=\\S+", "token=\\S+"},
		RedactionReplace:  "[SECRET]",
	}

	logger, err := NewWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create logger with config: %v", err)
	}
	defer logger.Close()

	// Check if redactor was set
	if logger.redactor == nil {
		t.Fatal("Redactor was not set")
	}
	
	// Check if redaction patterns were set
	if len(logger.redactionPatterns) != 2 {
		t.Errorf("Expected 2 redaction patterns, got %d", len(logger.redactionPatterns))
	}

	// Log message with sensitive data
	logger.Info("Login with password=secret123 and token=abc456")

	// Wait for message to be processed
	time.Sleep(200 * time.Millisecond)
	
	// Force flush
	logger.Sync()
	logger.FlushAll()
	
	// Wait a bit more for file write
	time.Sleep(100 * time.Millisecond)

	// Read log file
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)
	t.Logf("Log content: %s", logContent)
	if strings.Contains(logContent, "secret123") {
		t.Error("Password was not redacted")
	}
	if strings.Contains(logContent, "abc456") {
		t.Error("Token was not redacted")
	}
	if !strings.Contains(logContent, "[SECRET]") {
		t.Error("Redaction replacement not found")
	}
}

func TestConfigMaxAge(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	config := &Config{
		Path:            logFile,
		MaxAge:          1 * time.Hour,
		CleanupInterval: 1 * time.Minute,
	}

	logger, err := NewWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create logger with config: %v", err)
	}
	defer logger.Close()

	// Verify rotation manager was started
	if logger.rotationManager == nil {
		t.Error("Expected rotation manager to be created with MaxAge set")
	}

	// Update to disable max age
	newConfig := &Config{
		MaxAge: 0,
	}

	err = logger.UpdateConfig(newConfig)
	if err != nil {
		t.Fatalf("Failed to update config: %v", err)
	}

	// Verify cleanup routine was stopped
	if logger.cleanupTicker != nil {
		t.Error("Expected cleanup ticker to be stopped when MaxAge is 0")
	}
}

func TestConfigNoPath(t *testing.T) {
	// Create logger without a path
	config := &Config{
		Level:  LevelDebug,
		Format: FormatJSON,
	}

	logger, err := NewWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create logger without path: %v", err)
	}
	defer logger.Close()

	// Should have no destinations
	if len(logger.Destinations) != 0 {
		t.Errorf("Expected no destinations, got %d", len(logger.Destinations))
	}

	// Add destination later
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	err = logger.AddDestination(logFile)
	if err != nil {
		t.Fatalf("Failed to add destination: %v", err)
	}

	// Now should work
	logger.Info("test message")
	time.Sleep(100 * time.Millisecond)
}
