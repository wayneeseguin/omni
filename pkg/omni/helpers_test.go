package omni

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestDefaultSampleKeyFunc(t *testing.T) {
	level := LevelInfo
	message := "test message"
	fields := map[string]interface{}{
		"key1": "value1",
		"key2": 123,
	}

	key := DefaultSampleKeyFunc(level, message, fields)
	expected := "2:test message" // LevelInfo = 2

	if key != expected {
		t.Errorf("Expected key %s, got %s", expected, key)
	}

	// Test with different level
	key2 := DefaultSampleKeyFunc(LevelError, "error message", nil)
	expected2 := "4:error message" // LevelError = 4

	if key2 != expected2 {
		t.Errorf("Expected key %s, got %s", expected2, key2)
	}
}

func TestIsTestMode(t *testing.T) {
	// This should return true when running under go test
	if !isTestMode() {
		t.Error("isTestMode should return true when running under go test")
	}
}

func TestGetDefaultErrorHandler(t *testing.T) {
	handler := getDefaultErrorHandler()
	if handler == nil {
		t.Error("getDefaultErrorHandler should not return nil")
	}

	// In test mode, should return silent error handler
	// We can't directly test this without examining internals,
	// but we can verify it doesn't panic
	testError := LogError{
		Message:     "test error",
		Operation:   "test",
		Timestamp:   time.Now(),
		Level:       ErrorLevelMedium,
		Destination: "",
	}

	// Should not panic
	handler(testError)
}

func TestGetDefaultChannelSize(t *testing.T) {
	// Test default value
	defaultSize := getDefaultChannelSize()
	if defaultSize <= 0 {
		t.Errorf("Expected positive channel size, got %d", defaultSize)
	}

	// Test with environment variable
	os.Setenv("OMNI_CHANNEL_SIZE", "250")
	defer os.Unsetenv("OMNI_CHANNEL_SIZE")

	envSize := getDefaultChannelSize()
	if envSize != 250 {
		t.Errorf("Expected channel size 250, got %d", envSize)
	}

	// Test with invalid environment variable
	os.Setenv("OMNI_CHANNEL_SIZE", "invalid")
	invalidSize := getDefaultChannelSize()
	if invalidSize != 100 { // Should fallback to default
		t.Errorf("Expected fallback to default size 100, got %d", invalidSize)
	}

	// Test with negative environment variable
	os.Setenv("OMNI_CHANNEL_SIZE", "-5")
	negativeSize := getDefaultChannelSize()
	if negativeSize != 100 { // Should fallback to default
		t.Errorf("Expected fallback to default size 100, got %d", negativeSize)
	}

	// Test with zero environment variable
	os.Setenv("OMNI_CHANNEL_SIZE", "0")
	zeroSize := getDefaultChannelSize()
	if zeroSize != 100 { // Should fallback to default
		t.Errorf("Expected fallback to default size 100, got %d", zeroSize)
	}
}

func TestGetHostname(t *testing.T) {
	hostname, err := GetHostname()
	if err != nil {
		t.Errorf("GetHostname failed: %v", err)
	}

	if hostname == "" {
		t.Error("Expected non-empty hostname")
	}

	t.Logf("Hostname: %s", hostname)
}

func TestNewBatchWriter(t *testing.T) {
	// Create a mock writer
	writer := &strings.Builder{}

	batchWriter := NewBatchWriter(writer, 1024, 10, 100*time.Millisecond)
	if batchWriter == nil {
		t.Error("NewBatchWriter should not return nil")
	}

	// For now, it should return the original writer
	if batchWriter != writer {
		t.Error("NewBatchWriter should return the original writer for now")
	}
}

func TestNewSyslog(t *testing.T) {
	t.Run("UnixSocketPath", func(t *testing.T) {
		// Test with Unix socket path
		_, err := NewSyslog("/dev/log", "test-tag")
		// This will likely fail on macOS/systems without /dev/log, but that's expected
		if err != nil {
			t.Logf("Expected error for Unix socket (system dependent): %v", err)
		}
	})

	t.Run("NetworkAddress", func(t *testing.T) {
		// Test with network address
		_, err := NewSyslog("localhost:514", "test-tag")
		// This will likely fail unless syslog server is running, but that's expected
		if err != nil {
			t.Logf("Expected error for network address (no server running): %v", err)
		}
	})

	t.Run("WithSyslogPrefix", func(t *testing.T) {
		// Test with syslog:// prefix
		_, err := NewSyslog("syslog://localhost:514", "test-tag")
		if err != nil {
			t.Logf("Expected error for prefixed address (no server running): %v", err)
		}
	})

	t.Run("InvalidAddress", func(t *testing.T) {
		// Test with invalid address
		_, err := NewSyslog("", "test-tag")
		if err == nil {
			t.Error("Expected error for empty address")
		}
	})
}

func TestErrorCodes(t *testing.T) {
	// Test that error codes are defined
	codes := []int{
		ErrCodeInvalidConfig,
		ErrCodeInvalidLevel,
		ErrCodeInvalidFormat,
	}

	for _, code := range codes {
		if code == 0 {
			t.Errorf("Error code should not be zero: %d", code)
		}
	}

	// Test that codes are unique
	codeMap := make(map[int]bool)
	for _, code := range codes {
		if codeMap[code] {
			t.Errorf("Duplicate error code: %d", code)
		}
		codeMap[code] = true
	}
}

func TestDefaultFormatOptions(t *testing.T) {
	options := DefaultFormatOptions()

	// Verify default values
	if options.TimestampFormat != time.RFC3339 {
		t.Errorf("Expected RFC3339 timestamp format, got %s", options.TimestampFormat)
	}

	if !options.IncludeLevel {
		t.Error("Expected IncludeLevel to be true")
	}

	if !options.IncludeTime {
		t.Error("Expected IncludeTime to be true")
	}

	if options.LevelFormat != LevelFormatName {
		t.Errorf("Expected LevelFormatName, got %d", options.LevelFormat)
	}

	if options.IndentJSON {
		t.Error("Expected IndentJSON to be false")
	}

	if options.FieldSeparator != " " {
		t.Errorf("Expected field separator ' ', got '%s'", options.FieldSeparator)
	}

	if options.TimeZone != time.UTC {
		t.Errorf("Expected UTC timezone, got %v", options.TimeZone)
	}
}

func TestIsTestModeWithDifferentArgs(t *testing.T) {
	// Save original args
	originalArgs := os.Args

	// Test with test-related arguments
	testArgs := [][]string{
		{"program", "-test.v"},
		{"program", "-test.run=TestSomething"},
		{"program.test"},
		{"/path/to/program.test", "arg1", "arg2"},
		{"go", "test", "./..."},
	}

	for _, args := range testArgs {
		os.Args = args
		// Note: This doesn't fully test the function since the executable
		// path detection will still use the real executable, but it tests
		// the argument parsing part
		result := isTestMode()
		t.Logf("Args %v -> isTestMode: %v", args, result)
	}

	// Restore original args
	os.Args = originalArgs
}

func TestChannelSizeEnvironmentInteraction(t *testing.T) {
	// Save original environment
	originalValue, exists := os.LookupEnv("OMNI_CHANNEL_SIZE")
	if exists {
		defer os.Setenv("OMNI_CHANNEL_SIZE", originalValue)
	} else {
		defer os.Unsetenv("OMNI_CHANNEL_SIZE")
	}

	// Test various environment values
	testCases := []struct {
		envValue     string
		expectedSize int
		description  string
	}{
		{"", 100, "empty value"},
		{"50", 50, "valid small value"},
		{"1000", 1000, "valid large value"},
		{"abc", 100, "non-numeric value"},
		{"-10", 100, "negative value"},
		{"0", 100, "zero value"},
		{"1.5", 100, "decimal value"},
		{"999999999999", 100, "extremely large value that might overflow"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			if tc.envValue == "" {
				os.Unsetenv("OMNI_CHANNEL_SIZE")
			} else {
				os.Setenv("OMNI_CHANNEL_SIZE", tc.envValue)
			}

			size := getDefaultChannelSize()
			if size != tc.expectedSize {
				t.Errorf("Expected size %d for env value '%s', got %d", 
					tc.expectedSize, tc.envValue, size)
			}
		})
	}
}

func TestErrorHandlerInTestMode(t *testing.T) {
	// Verify we get silent error handler in test mode
	handler := getDefaultErrorHandler()

	// Create a test error
	testError := LogError{
		Message:     "test error message",
		Operation:   "test_operation",
		Destination: "/test/path",
		Err:         nil,
		Timestamp:   time.Now(),
		Level:       ErrorLevelHigh,
	}

	// Call the handler - should not panic or output to stderr in test mode
	handler(testError)

	// If we get here without panic, the test passes
	t.Log("Error handler executed successfully in test mode")
}

// TestHelperFunctionRobustness tests edge cases and robustness
func TestHelperFunctionRobustness(t *testing.T) {
	t.Run("SampleKeyFuncWithNilFields", func(t *testing.T) {
		key := DefaultSampleKeyFunc(LevelDebug, "test", nil)
		expected := "1:test" // LevelDebug = 1
		if key != expected {
			t.Errorf("Expected %s, got %s", expected, key)
		}
	})

	t.Run("SampleKeyFuncWithEmptyMessage", func(t *testing.T) {
		key := DefaultSampleKeyFunc(LevelInfo, "", map[string]interface{}{"key": "value"})
		expected := "2:" // LevelInfo = 2, empty message
		if key != expected {
			t.Errorf("Expected %s, got %s", expected, key)
		}
	})

	t.Run("SampleKeyFuncWithSpecialCharacters", func(t *testing.T) {
		message := "test:message with spaces and symbols!@#$%"
		key := DefaultSampleKeyFunc(LevelWarn, message, nil)
		expected := "3:" + message // LevelWarn = 3
		if key != expected {
			t.Errorf("Expected %s, got %s", expected, key)
		}
	})

	t.Run("NewBatchWriterWithNilWriter", func(t *testing.T) {
		batchWriter := NewBatchWriter(nil, 1024, 10, 100*time.Millisecond)
		if batchWriter != nil {
			t.Error("Expected nil when passing nil writer")
		}
	})

	t.Run("NewBatchWriterWithZeroValues", func(t *testing.T) {
		writer := &strings.Builder{}
		batchWriter := NewBatchWriter(writer, 0, 0, 0)
		// Should still return the writer (current implementation)
		if batchWriter != writer {
			t.Error("Expected to return original writer even with zero values")
		}
	})
}

func TestConstantsAndDefaults(t *testing.T) {
	// Test that constants are reasonable
	if getDefaultChannelSize() < 1 {
		t.Error("Default channel size should be positive")
	}

	// Test format options defaults
	options := DefaultFormatOptions()
	if options.TimestampFormat == "" {
		t.Error("Default timestamp format should not be empty")
	}

	if options.FieldSeparator == "" {
		t.Error("Default field separator should not be empty")
	}

	if options.TimeZone == nil {
		t.Error("Default timezone should not be nil")
	}
}

// TestEnvironmentVariableHandling tests various environment variable scenarios
func TestEnvironmentVariableHandling(t *testing.T) {
	// Save original environment
	originalValue, exists := os.LookupEnv("OMNI_CHANNEL_SIZE")
	defer func() {
		if exists {
			os.Setenv("OMNI_CHANNEL_SIZE", originalValue)
		} else {
			os.Unsetenv("OMNI_CHANNEL_SIZE")
		}
	}()

	t.Run("VeryLargeChannelSize", func(t *testing.T) {
		os.Setenv("OMNI_CHANNEL_SIZE", "2147483647") // Max int32
		size := getDefaultChannelSize()
		if size != 2147483647 {
			t.Errorf("Expected %d, got %d", 2147483647, size)
		}
	})

	t.Run("ChannelSizeWithWhitespace", func(t *testing.T) {
		os.Setenv("OMNI_CHANNEL_SIZE", "  100  ")
		size := getDefaultChannelSize()
		// This should fail to parse and fallback to default
		if size != 100 {
			t.Errorf("Expected fallback to 100, got %d", size)
		}
	})

	t.Run("ChannelSizeWithHexValue", func(t *testing.T) {
		os.Setenv("OMNI_CHANNEL_SIZE", "0x64") // 100 in hex
		size := getDefaultChannelSize()
		// This should fail to parse and fallback to default
		if size != 100 {
			t.Errorf("Expected fallback to 100, got %d", size)
		}
	})
}