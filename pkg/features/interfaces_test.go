package features

import (
	"errors"
	"regexp"
	"testing"
	"time"
)

// TestRegexFilter tests the RegexFilter implementation
func TestRegexFilter(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		include     bool
		message     string
		level       int
		fields      map[string]interface{}
		expectedLog bool
		expectError bool
	}{
		{
			name:        "include filter matches",
			pattern:     "error",
			include:     true,
			message:     "this is an error message",
			expectedLog: true,
		},
		{
			name:        "include filter no match",
			pattern:     "error",
			include:     true,
			message:     "this is a debug message",
			expectedLog: false,
		},
		{
			name:        "exclude filter matches",
			pattern:     "debug",
			include:     false,
			message:     "this is a debug message",
			expectedLog: false,
		},
		{
			name:        "exclude filter no match",
			pattern:     "debug",
			include:     false,
			message:     "this is an error message",
			expectedLog: true,
		},
		{
			name:        "case sensitive match",
			pattern:     "Error",
			include:     true,
			message:     "this is an error message",
			expectedLog: false,
		},
		{
			name:        "case insensitive pattern",
			pattern:     "(?i)error",
			include:     true,
			message:     "this is an ERROR message",
			expectedLog: true,
		},
		{
			name:        "complex regex pattern",
			pattern:     `\b\d{3}-\d{3}-\d{4}\b`,
			include:     true,
			message:     "Contact us at 555-123-4567",
			expectedLog: true,
		},
		{
			name:        "complex regex no match",
			pattern:     `\b\d{3}-\d{3}-\d{4}\b`,
			include:     true,
			message:     "Contact us at email@example.com",
			expectedLog: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern, err := regexp.Compile(tt.pattern)
			if err != nil {
				if !tt.expectError {
					t.Fatalf("Failed to compile regex pattern %q: %v", tt.pattern, err)
				}
				return
			}

			filter := &RegexFilter{
				Pattern: pattern,
				Include: tt.include,
			}

			// Test ShouldLog method
			result := filter.ShouldLog(tt.level, tt.message, tt.fields)
			if result != tt.expectedLog {
				t.Errorf("ShouldLog() = %v, expected %v", result, tt.expectedLog)
			}

			// Test Name method
			if filter.Name() != "regex" {
				t.Errorf("Name() = %q, expected %q", filter.Name(), "regex")
			}
		})
	}
}

// TestLevelFilter tests the LevelFilter implementation
func TestLevelFilter(t *testing.T) {
	// Define test level constants for clarity
	const (
		LevelDebug = 0
		LevelInfo  = 1
		LevelWarn  = 2
		LevelError = 3
	)

	tests := []struct {
		name        string
		minLevel    int
		maxLevel    int
		testLevel   int
		message     string
		fields      map[string]interface{}
		expectedLog bool
	}{
		{
			name:        "level within range",
			minLevel:    LevelInfo,
			maxLevel:    LevelError,
			testLevel:   LevelWarn,
			expectedLog: true,
		},
		{
			name:        "level below range",
			minLevel:    LevelInfo,
			maxLevel:    LevelError,
			testLevel:   LevelDebug,
			expectedLog: false,
		},
		{
			name:        "level above range",
			minLevel:    LevelDebug,
			maxLevel:    LevelWarn,
			testLevel:   LevelError,
			expectedLog: false,
		},
		{
			name:        "level at minimum",
			minLevel:    LevelWarn,
			maxLevel:    LevelError,
			testLevel:   LevelWarn,
			expectedLog: true,
		},
		{
			name:        "level at maximum",
			minLevel:    LevelInfo,
			maxLevel:    LevelWarn,
			testLevel:   LevelWarn,
			expectedLog: true,
		},
		{
			name:        "single level range",
			minLevel:    LevelError,
			maxLevel:    LevelError,
			testLevel:   LevelError,
			expectedLog: true,
		},
		{
			name:        "single level range no match",
			minLevel:    LevelError,
			maxLevel:    LevelError,
			testLevel:   LevelWarn,
			expectedLog: false,
		},
		{
			name:        "wide open range",
			minLevel:    LevelDebug,
			maxLevel:    LevelError,
			testLevel:   LevelInfo,
			expectedLog: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := &LevelFilter{
				MinLevel: tt.minLevel,
				MaxLevel: tt.maxLevel,
			}

			// Test ShouldLog method
			result := filter.ShouldLog(tt.testLevel, tt.message, tt.fields)
			if result != tt.expectedLog {
				t.Errorf("ShouldLog() = %v, expected %v", result, tt.expectedLog)
			}

			// Test Name method
			if filter.Name() != "level" {
				t.Errorf("Name() = %q, expected %q", filter.Name(), "level")
			}
		})
	}
}

// TestConfigurationStructs tests the configuration struct types
func TestConfigurationStructs(t *testing.T) {
	t.Run("CompressionConfig", func(t *testing.T) {
		config := CompressionConfig{
			Type:    1,
			MinAge:  3,
			Workers: 4,
			Enabled: true,
		}

		if config.Type != 1 {
			t.Errorf("Expected Type = 1, got %d", config.Type)
		}
		if config.MinAge != 3 {
			t.Errorf("Expected MinAge = 3, got %d", config.MinAge)
		}
		if config.Workers != 4 {
			t.Errorf("Expected Workers = 4, got %d", config.Workers)
		}
		if !config.Enabled {
			t.Errorf("Expected Enabled = true, got %v", config.Enabled)
		}
	})

	t.Run("RotationConfig", func(t *testing.T) {
		maxAge := 24 * time.Hour
		config := RotationConfig{
			MaxSize:      1024 * 1024,
			MaxFiles:     10,
			MaxAge:       maxAge,
			KeepOriginal: true,
		}

		if config.MaxSize != 1024*1024 {
			t.Errorf("Expected MaxSize = %d, got %d", 1024*1024, config.MaxSize)
		}
		if config.MaxFiles != 10 {
			t.Errorf("Expected MaxFiles = 10, got %d", config.MaxFiles)
		}
		if config.MaxAge != maxAge {
			t.Errorf("Expected MaxAge = %v, got %v", maxAge, config.MaxAge)
		}
		if !config.KeepOriginal {
			t.Errorf("Expected KeepOriginal = true, got %v", config.KeepOriginal)
		}
	})

	t.Run("SamplingConfig", func(t *testing.T) {
		keyFunc := func(level int, message string, fields map[string]interface{}) string {
			return "test-key"
		}
		config := SamplingConfig{
			Strategy: 1,
			Rate:     0.5,
			KeyFunc:  keyFunc,
		}

		if config.Strategy != 1 {
			t.Errorf("Expected Strategy = 1, got %d", config.Strategy)
		}
		if config.Rate != 0.5 {
			t.Errorf("Expected Rate = 0.5, got %f", config.Rate)
		}
		if config.KeyFunc == nil {
			t.Error("Expected KeyFunc to be set")
		}

		// Test the key function
		key := config.KeyFunc(0, "test", nil)
		if key != "test-key" {
			t.Errorf("Expected key = 'test-key', got %q", key)
		}
	})

	t.Run("FieldRedactionRule", func(t *testing.T) {
		rule := FieldRedactionRule{
			FieldPath: "user.password",
			Pattern:   `\w+`,
			Replace:   "***",
		}

		if rule.FieldPath != "user.password" {
			t.Errorf("Expected FieldPath = 'user.password', got %q", rule.FieldPath)
		}
		if rule.Pattern != `\w+` {
			t.Errorf("Expected Pattern = '\\w+', got %q", rule.Pattern)
		}
		if rule.Replace != "***" {
			t.Errorf("Expected Replace = '***', got %q", rule.Replace)
		}
	})
}

// MockCompressor implements the Compressor interface for testing
type MockCompressor struct {
	compressFunc func(src, dest string) error
	typeValue    int
	extension    string
}

func (m *MockCompressor) Compress(src, dest string) error {
	if m.compressFunc != nil {
		return m.compressFunc(src, dest)
	}
	return nil
}

func (m *MockCompressor) Type() int {
	return m.typeValue
}

func (m *MockCompressor) Extension() string {
	return m.extension
}

// TestCompressorInterface tests the Compressor interface
func TestCompressorInterface(t *testing.T) {
	t.Run("successful compression", func(t *testing.T) {
		compressor := &MockCompressor{
			compressFunc: func(src, dest string) error {
				if src == "" || dest == "" {
					return errors.New("invalid parameters")
				}
				return nil
			},
			typeValue: 1,
			extension: ".gz",
		}

		// Test interface compliance
		var c Compressor = compressor

		// Test Compress method
		err := c.Compress("source.log", "dest.log.gz")
		if err != nil {
			t.Errorf("Compress() failed: %v", err)
		}

		// Test Type method
		if c.Type() != 1 {
			t.Errorf("Type() = %d, expected 1", c.Type())
		}

		// Test Extension method
		if c.Extension() != ".gz" {
			t.Errorf("Extension() = %q, expected '.gz'", c.Extension())
		}
	})

	t.Run("compression error", func(t *testing.T) {
		compressor := &MockCompressor{
			compressFunc: func(src, dest string) error {
				return errors.New("compression failed")
			},
		}

		err := compressor.Compress("source.log", "dest.log.gz")
		if err == nil {
			t.Error("Expected compression error, got nil")
		}
	})
}

// MockRotator implements the Rotator interface for testing
type MockRotator struct {
	rotateFunc       func(currentPath string) error
	shouldRotateFunc func(size int64, age time.Duration) bool
	getFilenameFunc  func(basePath string, index int) string
}

func (m *MockRotator) Rotate(currentPath string) error {
	if m.rotateFunc != nil {
		return m.rotateFunc(currentPath)
	}
	return nil
}

func (m *MockRotator) ShouldRotate(size int64, age time.Duration) bool {
	if m.shouldRotateFunc != nil {
		return m.shouldRotateFunc(size, age)
	}
	return false
}

func (m *MockRotator) GetRotatedFilename(basePath string, index int) string {
	if m.getFilenameFunc != nil {
		return m.getFilenameFunc(basePath, index)
	}
	return ""
}

// TestRotatorInterface tests the Rotator interface
func TestRotatorInterface(t *testing.T) {
	t.Run("rotation needed", func(t *testing.T) {
		rotator := &MockRotator{
			rotateFunc: func(currentPath string) error {
				return nil
			},
			shouldRotateFunc: func(size int64, age time.Duration) bool {
				return size > 1024*1024 || age > 24*time.Hour
			},
			getFilenameFunc: func(basePath string, index int) string {
				return basePath + ".1"
			},
		}

		// Test interface compliance
		var r Rotator = rotator

		// Test ShouldRotate with size trigger
		if !r.ShouldRotate(2*1024*1024, time.Hour) {
			t.Error("ShouldRotate should return true for large size")
		}

		// Test ShouldRotate with age trigger
		if !r.ShouldRotate(1024, 25*time.Hour) {
			t.Error("ShouldRotate should return true for old age")
		}

		// Test ShouldRotate with no trigger
		if r.ShouldRotate(1024, time.Hour) {
			t.Error("ShouldRotate should return false for small size and young age")
		}

		// Test GetRotatedFilename
		filename := r.GetRotatedFilename("/var/log/app.log", 1)
		if filename != "/var/log/app.log.1" {
			t.Errorf("GetRotatedFilename() = %q, expected '/var/log/app.log.1'", filename)
		}

		// Test Rotate
		err := r.Rotate("/var/log/app.log")
		if err != nil {
			t.Errorf("Rotate() failed: %v", err)
		}
	})

	t.Run("rotation error", func(t *testing.T) {
		rotator := &MockRotator{
			rotateFunc: func(currentPath string) error {
				return errors.New("rotation failed")
			},
		}

		err := rotator.Rotate("/var/log/app.log")
		if err == nil {
			t.Error("Expected rotation error, got nil")
		}
	})
}

// MockSampler implements the Sampler interface for testing
type MockSampler struct {
	shouldSampleFunc func(level int, message string, fields map[string]interface{}) bool
	rate             float64
}

func (m *MockSampler) ShouldSample(level int, message string, fields map[string]interface{}) bool {
	if m.shouldSampleFunc != nil {
		return m.shouldSampleFunc(level, message, fields)
	}
	return true
}

func (m *MockSampler) Rate() float64 {
	return m.rate
}

func (m *MockSampler) SetRate(rate float64) {
	m.rate = rate
}

// TestSamplerInterface tests the Sampler interface
func TestSamplerInterface(t *testing.T) {
	t.Run("sampling behavior", func(t *testing.T) {
		sampler := &MockSampler{
			shouldSampleFunc: func(level int, message string, fields map[string]interface{}) bool {
				// Sample only error level messages
				return level >= 3
			},
			rate: 0.5,
		}

		// Test interface compliance
		var s Sampler = sampler

		// Test ShouldSample for error level (should sample)
		if !s.ShouldSample(3, "error message", nil) {
			t.Error("ShouldSample should return true for error level")
		}

		// Test ShouldSample for debug level (should not sample)
		if s.ShouldSample(0, "debug message", nil) {
			t.Error("ShouldSample should return false for debug level")
		}

		// Test Rate getter
		if s.Rate() != 0.5 {
			t.Errorf("Rate() = %f, expected 0.5", s.Rate())
		}

		// Test SetRate and getter
		s.SetRate(0.25)
		if s.Rate() != 0.25 {
			t.Errorf("Rate() = %f, expected 0.25 after SetRate", s.Rate())
		}
	})
}

// MockRedactor implements the RedactorInterface for testing
type MockRedactor struct {
	patterns []string
}

func (m *MockRedactor) Redact(message string) string {
	// Simple redaction: replace "password" with "***"
	if message == "password123" {
		return "***"
	}
	return message
}

func (m *MockRedactor) RedactFields(fields map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range fields {
		if k == "password" {
			result[k] = "***"
		} else {
			result[k] = v
		}
	}
	return result
}

func (m *MockRedactor) AddPattern(pattern string) {
	m.patterns = append(m.patterns, pattern)
}

func (m *MockRedactor) RemovePattern(pattern string) {
	for i, p := range m.patterns {
		if p == pattern {
			m.patterns = append(m.patterns[:i], m.patterns[i+1:]...)
			break
		}
	}
}

// TestRedactorInterface tests the RedactorInterface
func TestRedactorInterface(t *testing.T) {
	redactor := &MockRedactor{}

	// Test interface compliance
	var r RedactorInterface = redactor

	t.Run("message redaction", func(t *testing.T) {
		result := r.Redact("password123")
		if result != "***" {
			t.Errorf("Redact() = %q, expected '***'", result)
		}

		result = r.Redact("normal message")
		if result != "normal message" {
			t.Errorf("Redact() = %q, expected 'normal message'", result)
		}
	})

	t.Run("field redaction", func(t *testing.T) {
		fields := map[string]interface{}{
			"username": "john",
			"password": "secret123",
			"email":    "john@example.com",
		}

		result := r.RedactFields(fields)
		if result["password"] != "***" {
			t.Errorf("RedactFields() password = %v, expected '***'", result["password"])
		}
		if result["username"] != "john" {
			t.Errorf("RedactFields() username = %v, expected 'john'", result["username"])
		}
		if result["email"] != "john@example.com" {
			t.Errorf("RedactFields() email = %v, expected 'john@example.com'", result["email"])
		}
	})

	t.Run("pattern management", func(t *testing.T) {
		r.AddPattern("pattern1")
		r.AddPattern("pattern2")

		if len(redactor.patterns) != 2 {
			t.Errorf("Expected 2 patterns, got %d", len(redactor.patterns))
		}

		r.RemovePattern("pattern1")
		if len(redactor.patterns) != 1 {
			t.Errorf("Expected 1 pattern after removal, got %d", len(redactor.patterns))
		}

		if redactor.patterns[0] != "pattern2" {
			t.Errorf("Expected remaining pattern to be 'pattern2', got %q", redactor.patterns[0])
		}
	})
}

// MockRecovery implements the Recovery interface for testing
type MockRecovery struct {
	recoverableErrors map[string]bool
	fallbackOptions   []string
}

func (m *MockRecovery) Recover(err error) error {
	if err.Error() == "recoverable error" {
		return nil // Successfully recovered
	}
	return err // Cannot recover
}

func (m *MockRecovery) IsRecoverable(err error) bool {
	if m.recoverableErrors == nil {
		return false
	}
	return m.recoverableErrors[err.Error()]
}

func (m *MockRecovery) GetFallbackOptions() []string {
	return m.fallbackOptions
}

// TestRecoveryInterface tests the Recovery interface
func TestRecoveryInterface(t *testing.T) {
	recovery := &MockRecovery{
		recoverableErrors: map[string]bool{
			"recoverable error":   true,
			"unrecoverable error": false,
		},
		fallbackOptions: []string{"option1", "option2"},
	}

	// Test interface compliance
	var r Recovery = recovery

	t.Run("error recovery", func(t *testing.T) {
		// Test recoverable error
		err := errors.New("recoverable error")
		if !r.IsRecoverable(err) {
			t.Error("IsRecoverable should return true for recoverable error")
		}

		recoveredErr := r.Recover(err)
		if recoveredErr != nil {
			t.Errorf("Recover() should return nil for recoverable error, got %v", recoveredErr)
		}

		// Test unrecoverable error
		err = errors.New("unrecoverable error")
		if r.IsRecoverable(err) {
			t.Error("IsRecoverable should return false for unrecoverable error")
		}

		recoveredErr = r.Recover(err)
		if recoveredErr == nil {
			t.Error("Recover() should return error for unrecoverable error")
		}
	})

	t.Run("fallback options", func(t *testing.T) {
		options := r.GetFallbackOptions()
		if len(options) != 2 {
			t.Errorf("Expected 2 fallback options, got %d", len(options))
		}
		if options[0] != "option1" || options[1] != "option2" {
			t.Errorf("Expected fallback options [option1, option2], got %v", options)
		}
	})
}

// TestFilterInterface tests the Filter interface with both implementations
func TestFilterInterface(t *testing.T) {
	t.Run("regex filter as interface", func(t *testing.T) {
		pattern, _ := regexp.Compile("error")
		regexFilter := &RegexFilter{
			Pattern: pattern,
			Include: true,
		}

		// Test interface compliance
		var filter Filter = regexFilter

		if !filter.ShouldLog(0, "error message", nil) {
			t.Error("Filter should log error message")
		}

		if filter.Name() != "regex" {
			t.Errorf("Filter name = %q, expected 'regex'", filter.Name())
		}
	})

	t.Run("level filter as interface", func(t *testing.T) {
		levelFilter := &LevelFilter{
			MinLevel: 1,
			MaxLevel: 3,
		}

		// Test interface compliance
		var filter Filter = levelFilter

		if !filter.ShouldLog(2, "test message", nil) {
			t.Error("Filter should log level 2 message")
		}

		if filter.ShouldLog(0, "test message", nil) {
			t.Error("Filter should not log level 0 message")
		}

		if filter.Name() != "level" {
			t.Errorf("Filter name = %q, expected 'level'", filter.Name())
		}
	})
}

// MockLogger provides a simple implementation of the Logger interface for testing
type MockLogger struct {
	logs   []string
	closed bool
}

func (m *MockLogger) Debug(args ...interface{})                 { m.logs = append(m.logs, "debug") }
func (m *MockLogger) Info(args ...interface{})                  { m.logs = append(m.logs, "info") }
func (m *MockLogger) Warn(args ...interface{})                  { m.logs = append(m.logs, "warn") }
func (m *MockLogger) Error(args ...interface{})                 { m.logs = append(m.logs, "error") }
func (m *MockLogger) Debugf(format string, args ...interface{}) { m.logs = append(m.logs, "debugf") }
func (m *MockLogger) Infof(format string, args ...interface{})  { m.logs = append(m.logs, "infof") }
func (m *MockLogger) Warnf(format string, args ...interface{})  { m.logs = append(m.logs, "warnf") }
func (m *MockLogger) Errorf(format string, args ...interface{}) { m.logs = append(m.logs, "errorf") }
func (m *MockLogger) Close() error                              { m.closed = true; return nil }

// TestLoggerInterface tests the Logger interface
func TestLoggerInterface(t *testing.T) {
	logger := &MockLogger{}

	// Test interface compliance
	var l Logger = logger

	// Test all logging methods
	l.Debug("test")
	l.Info("test")
	l.Warn("test")
	l.Error("test")
	l.Debugf("test %s", "format")
	l.Infof("test %s", "format")
	l.Warnf("test %s", "format")
	l.Errorf("test %s", "format")

	expectedLogs := []string{"debug", "info", "warn", "error", "debugf", "infof", "warnf", "errorf"}
	for i, expected := range expectedLogs {
		if i >= len(logger.logs) || logger.logs[i] != expected {
			t.Errorf("Expected log[%d] = %q, got %q", i, expected, logger.logs[i])
		}
	}

	// Test Close
	err := l.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
	if !logger.closed {
		t.Error("Logger should be closed after Close() call")
	}
}
