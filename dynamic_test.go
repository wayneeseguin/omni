package flexlog

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDynamicConfig(t *testing.T) {
	t.Run("LoadAndApplyConfig", func(t *testing.T) {
		// Create temporary directory for test
		tempDir, err := ioutil.TempDir("", "flexlog-dynamic-test")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tempDir)

		// Create logger
		logPath := filepath.Join(tempDir, "test.log")
		logger, err := New(logPath)
		if err != nil {
			t.Fatal(err)
		}
		defer logger.Close()

		// Create config
		configPath := filepath.Join(tempDir, "config.json")
		level := LevelDebug
		format := FormatJSON
		samplingRate := 0.5
		config := &DynamicConfig{
			Level:        &level,
			Format:       &format,
			SamplingRate: &samplingRate,
			Fields: map[string]interface{}{
				"app":     "test",
				"version": "1.0.0",
			},
		}

		// Write config to file
		data, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := ioutil.WriteFile(configPath, data, 0644); err != nil {
			t.Fatal(err)
		}

		// Apply config
		if err := logger.ApplyDynamicConfig(config); err != nil {
			t.Fatal(err)
		}

		// Verify changes
		if logger.GetLevel() != LevelDebug {
			t.Errorf("Expected level %d, got %d", LevelDebug, logger.GetLevel())
		}
		if logger.GetFormat() != FormatJSON {
			t.Errorf("Expected format %d, got %d", FormatJSON, logger.GetFormat())
		}
		globalFields := logger.GetGlobalFields()
		if globalFields["app"] != "test" {
			t.Errorf("Expected global field app=test, got %v", globalFields["app"])
		}
	})

	t.Run("ConfigWatcher", func(t *testing.T) {
		// Create temporary directory for test
		tempDir, err := ioutil.TempDir("", "flexlog-watcher-test")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tempDir)

		// Create logger
		logPath := filepath.Join(tempDir, "test.log")
		logger, err := New(logPath)
		if err != nil {
			t.Fatal(err)
		}
		defer logger.Close()

		// Create initial config
		configPath := filepath.Join(tempDir, "config.json")
		level := LevelInfo
		config := &DynamicConfig{
			Level: &level,
		}
		data, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := ioutil.WriteFile(configPath, data, 0644); err != nil {
			t.Fatal(err)
		}

		// Enable dynamic config with very short interval for testing
		if err := logger.EnableDynamicConfig(configPath, 10*time.Millisecond); err != nil {
			t.Fatal(err)
		}
		defer logger.DisableDynamicConfig()

		// Poll for initial load instead of sleeping
		deadline := time.Now().Add(100 * time.Millisecond)
		for time.Now().Before(deadline) {
			if logger.GetLevel() == LevelInfo {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}

		// Update config file
		level = LevelDebug
		config.Level = &level
		data, err = json.MarshalIndent(config, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := ioutil.WriteFile(configPath, data, 0644); err != nil {
			t.Fatal(err)
		}

		// Poll for change instead of sleeping
		deadline = time.Now().Add(100 * time.Millisecond)
		changed := false
		for time.Now().Before(deadline) {
			if logger.GetLevel() == LevelDebug {
				changed = true
				break
			}
			time.Sleep(5 * time.Millisecond)
		}

		// Verify change was applied
		if !changed {
			t.Errorf("Expected level to change to %d, got %d", LevelDebug, logger.GetLevel())
		}
	})

	t.Run("ValidateConfig", func(t *testing.T) {
		tests := []struct {
			name    string
			config  *DynamicConfig
			wantErr bool
		}{
			{
				name: "valid config",
				config: &DynamicConfig{
					Level:  intPtr(LevelInfo),
					Format: intPtr(FormatJSON),
				},
				wantErr: false,
			},
			{
				name: "invalid level",
				config: &DynamicConfig{
					Level: intPtr(99),
				},
				wantErr: true,
			},
			{
				name: "invalid format",
				config: &DynamicConfig{
					Format: intPtr(99),
				},
				wantErr: true,
			},
			{
				name: "invalid sampling rate",
				config: &DynamicConfig{
					SamplingRate: float64Ptr(1.5),
				},
				wantErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := ValidateDynamicConfig(tt.config)
				if (err != nil) != tt.wantErr {
					t.Errorf("ValidateDynamicConfig() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("SaveAndLoadConfig", func(t *testing.T) {
		// Create temporary directory for test
		tempDir, err := ioutil.TempDir("", "flexlog-save-test")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tempDir)

		// Create logger with specific settings
		logPath := filepath.Join(tempDir, "test.log")
		logger, err := New(logPath)
		if err != nil {
			t.Fatal(err)
		}
		defer logger.Close()

		// Configure logger
		logger.SetLevel(LevelDebug)
		logger.SetFormat(FormatJSON)
		logger.SetMaxSize(1024 * 1024)
		logger.SetMaxFiles(5)
		logger.AddGlobalField("app", "test")
		logger.AddGlobalField("env", "testing")

		// Save config
		configPath := filepath.Join(tempDir, "saved-config.json")
		if err := logger.SaveDynamicConfig(configPath); err != nil {
			t.Fatal(err)
		}

		// Load config and verify
		data, err := ioutil.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}

		var config DynamicConfig
		if err := json.Unmarshal(data, &config); err != nil {
			t.Fatal(err)
		}

		if *config.Level != LevelDebug {
			t.Errorf("Expected saved level %d, got %d", LevelDebug, *config.Level)
		}
		if *config.Format != FormatJSON {
			t.Errorf("Expected saved format %d, got %d", FormatJSON, *config.Format)
		}
		if config.Fields["app"] != "test" {
			t.Errorf("Expected saved field app=test, got %v", config.Fields["app"])
		}
	})

	t.Run("DestinationManagement", func(t *testing.T) {
		// Create temporary directory for test
		tempDir, err := ioutil.TempDir("", "flexlog-dest-test")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tempDir)

		// Create logger
		logPath := filepath.Join(tempDir, "test.log")
		logger, err := New(logPath)
		if err != nil {
			t.Fatal(err)
		}
		defer logger.Close()

		// Create config with destination changes
		destPath := filepath.Join(tempDir, "secondary.log")
		config := &DynamicConfig{
			Destinations: []DynamicDestConfig{
				{
					Name:   "secondary",
					URI:    destPath,
					Action: "add",
				},
			},
		}

		// Apply config
		if err := logger.ApplyDynamicConfig(config); err != nil {
			t.Fatal(err)
		}

		// Poll for destination to be added
		deadline := time.Now().Add(50 * time.Millisecond)
		found := false
		for time.Now().Before(deadline) {
			dests := logger.ListDestinations()
			for _, d := range dests {
				if d == destPath {
					found = true
					break
				}
			}
			if found {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}

		// Verify destination was added
		if !found {
			t.Errorf("Expected destination %s to be added", destPath)
		}
	})
}

// Helper functions
func intPtr(i int) *int {
	return &i
}

func float64Ptr(f float64) *float64 {
	return &f
}

func boolPtr(b bool) *bool {
	return &b
}