package omni

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DynamicConfig represents configuration that can be changed at runtime
type DynamicConfig struct {
	Level            *int                    `json:"level,omitempty"`
	Format           *int                    `json:"format,omitempty"`
	SamplingStrategy *int                    `json:"sampling_strategy,omitempty"`
	SamplingRate     *float64                `json:"sampling_rate,omitempty"`
	MaxSize          *int64                  `json:"max_size,omitempty"`
	MaxFiles         *int                    `json:"max_files,omitempty"`
	Compression      *int                    `json:"compression,omitempty"`
	Filters          []DynamicFilterConfig   `json:"filters,omitempty"`
	Destinations     []DynamicDestConfig     `json:"destinations,omitempty"`
	ErrorHandler     *string                 `json:"error_handler,omitempty"`
	Fields           map[string]interface{}  `json:"fields,omitempty"`
}

// DynamicFilterConfig represents a dynamic filter configuration
type DynamicFilterConfig struct {
	Type     string                 `json:"type"`
	Level    *int                   `json:"level,omitempty"`
	Pattern  string                 `json:"pattern,omitempty"`
	Fields   map[string]interface{} `json:"fields,omitempty"`
	Enabled  bool                   `json:"enabled"`
}

// DynamicDestConfig represents a dynamic destination configuration
type DynamicDestConfig struct {
	Name     string `json:"name"`
	URI      string `json:"uri"`
	Backend  *int   `json:"backend,omitempty"`
	Enabled  *bool  `json:"enabled,omitempty"`
	Action   string `json:"action"` // "add", "remove", "enable", "disable"
}

// ConfigWatcher watches for configuration changes
type ConfigWatcher struct {
	logger       *Omni
	configPath   string
	interval     time.Duration
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	lastModTime  time.Time
	mu           sync.RWMutex
	onChange     func(*DynamicConfig) error
	errorHandler func(error)
}

// NewConfigWatcher creates a new configuration watcher that monitors a config file for changes.
// The watcher periodically checks the file and applies configuration updates to the logger.
//
// Parameters:
//   - logger: The Omni instance to update
//   - configPath: Path to the configuration file
//   - interval: How often to check for changes
//
// Returns:
//   - *ConfigWatcher: The created watcher instance
func NewConfigWatcher(logger *Omni, configPath string, interval time.Duration) *ConfigWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &ConfigWatcher{
		logger:     logger,
		configPath: configPath,
		interval:   interval,
		ctx:        ctx,
		cancel:     cancel,
		onChange:   logger.ApplyDynamicConfig,
		errorHandler: func(err error) {
			logger.Error("Config watcher error:", err)
		},
	}
}

// Start begins watching for configuration changes.
// It loads the initial configuration from the file (if it exists) and starts
// a goroutine to monitor for file modifications.
//
// Returns:
//   - error: Any error encountered during initial configuration loading
func (w *ConfigWatcher) Start() error {
	// Get initial mod time
	info, err := os.Stat(w.configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat config file: %w", err)
	}
	if info != nil {
		w.mu.Lock()
		w.lastModTime = info.ModTime()
		w.mu.Unlock()
		
		// Load initial config
		if err := w.loadAndApplyConfig(); err != nil {
			return fmt.Errorf("failed to load initial config: %w", err)
		}
	}

	// Start watcher goroutine
	w.wg.Add(1)
	go w.watch()
	
	return nil
}

// Stop stops watching for configuration changes.
// It cancels the watch goroutine and waits for it to complete.
func (w *ConfigWatcher) Stop() {
	w.cancel()
	w.wg.Wait()
}

// SetOnChange sets the callback for configuration changes
func (w *ConfigWatcher) SetOnChange(fn func(*DynamicConfig) error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onChange = fn
}

// SetErrorHandler sets the error handler
func (w *ConfigWatcher) SetErrorHandler(fn func(error)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.errorHandler = fn
}

// watch monitors the configuration file for changes
func (w *ConfigWatcher) watch() {
	defer w.wg.Done()
	
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			if err := w.checkAndReload(); err != nil {
				w.mu.RLock()
				handler := w.errorHandler
				w.mu.RUnlock()
				if handler != nil {
					handler(err)
				}
			}
		}
	}
}

// checkAndReload checks if the config file has changed and reloads if necessary
func (w *ConfigWatcher) checkAndReload() error {
	info, err := os.Stat(w.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Config file doesn't exist yet, that's ok
			return nil
		}
		return fmt.Errorf("failed to stat config file: %w", err)
	}
	
	w.mu.RLock()
	lastMod := w.lastModTime
	w.mu.RUnlock()
	
	if info.ModTime().After(lastMod) {
		w.mu.Lock()
		w.lastModTime = info.ModTime()
		w.mu.Unlock()
		
		return w.loadAndApplyConfig()
	}
	
	return nil
}

// loadAndApplyConfig loads the configuration and applies it
func (w *ConfigWatcher) loadAndApplyConfig() error {
	config, err := w.loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	
	w.mu.RLock()
	onChange := w.onChange
	w.mu.RUnlock()
	
	if onChange != nil {
		if err := onChange(config); err != nil {
			return fmt.Errorf("failed to apply config: %w", err)
		}
	}
	
	return nil
}

// loadConfig loads the configuration from the JSON file.
//
// Returns:
//   - *DynamicConfig: The loaded configuration
//   - error: Any error encountered during loading or parsing
func (w *ConfigWatcher) loadConfig() (*DynamicConfig, error) {
	data, err := ioutil.ReadFile(w.configPath)
	if err != nil {
		return nil, err
	}
	
	var config DynamicConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	
	return &config, nil
}

// ApplyDynamicConfig applies dynamic configuration changes to the logger.
// It validates and applies changes to log level, format, sampling, destinations,
// and other runtime-configurable settings without requiring a restart.
//
// Parameters:
//   - config: The configuration to apply
//
// Returns:
//   - error: Any validation or application error
func (f *Omni) ApplyDynamicConfig(config *DynamicConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	// Apply level change
	if config.Level != nil {
		if *config.Level < LevelTrace || *config.Level > LevelError {
			return fmt.Errorf("invalid log level: %d", *config.Level)
		}
		f.level = *config.Level
	}
	
	// Apply format change
	if config.Format != nil {
		if *config.Format != FormatText && *config.Format != FormatJSON {
			return fmt.Errorf("invalid format: %d", *config.Format)
		}
		f.format = *config.Format
	}
	
	// Apply sampling changes
	if config.SamplingStrategy != nil || config.SamplingRate != nil {
		if config.SamplingStrategy != nil {
			f.samplingStrategy = *config.SamplingStrategy
		}
		if config.SamplingRate != nil {
			if *config.SamplingRate < 0 || *config.SamplingRate > 1 {
				return fmt.Errorf("invalid sampling rate: %f", *config.SamplingRate)
			}
			f.samplingRate = *config.SamplingRate
		}
	}
	
	// Apply rotation settings
	if config.MaxSize != nil {
		f.maxSize = *config.MaxSize
	}
	if config.MaxFiles != nil {
		f.maxFiles = *config.MaxFiles
	}
	
	// Apply compression
	if config.Compression != nil {
		f.compression = *config.Compression
	}
	
	// Apply global fields
	if config.Fields != nil {
		if f.globalFields == nil {
			f.globalFields = make(map[string]interface{})
		}
		for k, v := range config.Fields {
			f.globalFields[k] = v
		}
	}
	
	// Handle destination changes
	for _, destConfig := range config.Destinations {
		switch destConfig.Action {
		case "add":
			// Add destination (need to do this outside the lock)
			go func(dc DynamicDestConfig) {
				backend := BackendFlock
				if dc.Backend != nil {
					backend = *dc.Backend
				}
				if err := f.AddDestinationWithBackend(dc.URI, backend); err != nil {
					// Log error using the logger itself
					f.Error("Failed to add destination", "uri", dc.URI, "error", err)
				}
			}(destConfig)
			
		case "remove":
			// Remove destination (need to do this outside the lock)
			go func(name string) {
				if err := f.RemoveDestination(name); err != nil {
					f.Error("Failed to remove destination", "name", name, "error", err)
				}
			}(destConfig.Name)
			
		case "enable":
			if destConfig.Enabled != nil && *destConfig.Enabled {
				go func(name string) {
					if err := f.EnableDestination(name); err != nil {
						f.Error("Failed to enable destination", "name", name, "error", err)
					}
				}(destConfig.Name)
			}
			
		case "disable":
			if destConfig.Enabled != nil && !*destConfig.Enabled {
				go func(name string) {
					if err := f.DisableDestination(name); err != nil {
						f.Error("Failed to disable destination", "name", name, "error", err)
					}
				}(destConfig.Name)
			}
		}
	}
	
	return nil
}

// EnableDynamicConfig enables dynamic configuration with the specified file path and check interval
func (f *Omni) EnableDynamicConfig(configPath string, interval time.Duration) error {
	f.mu.Lock()
	if f.configWatcher != nil {
		f.mu.Unlock()
		return fmt.Errorf("dynamic config already enabled")
	}
	f.mu.Unlock()
	
	// Create config directory if it doesn't exist
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	
	// Create watcher
	watcher := NewConfigWatcher(f, configPath, interval)
	
	// Start watching
	if err := watcher.Start(); err != nil {
		return fmt.Errorf("failed to start config watcher: %w", err)
	}
	
	f.mu.Lock()
	f.configWatcher = watcher
	f.mu.Unlock()
	
	return nil
}

// DisableDynamicConfig disables dynamic configuration
func (f *Omni) DisableDynamicConfig() {
	f.mu.Lock()
	watcher := f.configWatcher
	f.configWatcher = nil
	f.mu.Unlock()
	
	if watcher != nil {
		watcher.Stop()
	}
}

// SaveDynamicConfig saves the current configuration to a file
func (f *Omni) SaveDynamicConfig(path string) error {
	f.mu.RLock()
	config := DynamicConfig{
		Level:            &f.level,
		Format:           &f.format,
		SamplingStrategy: &f.samplingStrategy,
		SamplingRate:     &f.samplingRate,
		MaxSize:          &f.maxSize,
		MaxFiles:         &f.maxFiles,
		Compression:      &f.compression,
		Fields:           f.globalFields,
	}
	
	// Add current destinations
	for _, dest := range f.Destinations {
		enabled := dest.Enabled
		config.Destinations = append(config.Destinations, DynamicDestConfig{
			Name:    dest.Name,
			URI:     dest.URI,
			Backend: &dest.Backend,
			Enabled: &enabled,
			Action:  "add",
		})
	}
	f.mu.RUnlock()
	
	// Marshal to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	
	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	
	// Write to file
	if err := ioutil.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	
	return nil
}

// ValidateDynamicConfig validates a dynamic configuration
func ValidateDynamicConfig(config *DynamicConfig) error {
	if config.Level != nil {
		if *config.Level < LevelTrace || *config.Level > LevelError {
			return fmt.Errorf("invalid log level: %d", *config.Level)
		}
	}
	
	if config.Format != nil {
		if *config.Format != FormatText && *config.Format != FormatJSON {
			return fmt.Errorf("invalid format: %d", *config.Format)
		}
	}
	
	if config.SamplingRate != nil {
		if *config.SamplingRate < 0 || *config.SamplingRate > 1 {
			return fmt.Errorf("invalid sampling rate: %f", *config.SamplingRate)
		}
	}
	
	if config.MaxSize != nil && *config.MaxSize < 0 {
		return fmt.Errorf("invalid max size: %d", *config.MaxSize)
	}
	
	if config.MaxFiles != nil && *config.MaxFiles < 0 {
		return fmt.Errorf("invalid max files: %d", *config.MaxFiles)
	}
	
	return nil
}