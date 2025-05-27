package flexlog_test

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
	
	"github.com/wayneeseguin/flexlog"
)

func TestAddFilter(t *testing.T) {
	tests := []struct {
		name       string
		filter     flexlog.Filter
		logLevel   int
		logMessage string
		logFields  map[string]interface{}
		shouldLog  bool
	}{
		{
			name: "filter allows INFO level",
			filter: func(level int, message string, fields map[string]interface{}) bool {
				return level >= flexlog.LevelInfo
			},
			logLevel:   flexlog.LevelInfo,
			logMessage: "info message",
			shouldLog:  true,
		},
		{
			name: "filter blocks DEBUG level",
			filter: func(level int, message string, fields map[string]interface{}) bool {
				return level >= flexlog.LevelInfo
			},
			logLevel:   flexlog.LevelDebug,
			logMessage: "debug message",
			shouldLog:  false,
		},
		{
			name: "filter checks message content",
			filter: func(level int, message string, fields map[string]interface{}) bool {
				return strings.Contains(message, "important")
			},
			logLevel:   flexlog.LevelInfo,
			logMessage: "this is important",
			shouldLog:  true,
		},
		{
			name: "filter rejects based on message content",
			filter: func(level int, message string, fields map[string]interface{}) bool {
				return strings.Contains(message, "important")
			},
			logLevel:   flexlog.LevelInfo,
			logMessage: "this is not relevant",
			shouldLog:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tempFile := filepath.Join(t.TempDir(), "test.log")
			
			// Initialize logger
			logger, _ := flexlog.New(tempFile)
			logger.AddFilter(tt.filter)
			
			// Log message
			switch tt.logLevel {
			case flexlog.LevelDebug:
				logger.Debug(tt.logMessage)
			case flexlog.LevelInfo:
				logger.Info(tt.logMessage)
			case flexlog.LevelWarn:
				logger.Warn(tt.logMessage)
			case flexlog.LevelError:
				logger.Error(tt.logMessage)
			}
			
			// Wait for processing
			logger.Sync()
			logger.CloseAll()
			
			// Check if message was logged
			content, err := os.ReadFile(tempFile)
			if err != nil {
				t.Fatalf("Failed to read log file: %v", err)
			}
			
			hasContent := len(content) > 0
			if hasContent != tt.shouldLog {
				t.Errorf("Expected shouldLog=%v, but hasContent=%v", tt.shouldLog, hasContent)
			}
		})
	}
}

func TestClearFilters(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	
	logger, _ := flexlog.New(tempFile)
	
	// Add a restrictive filter
	logger.AddFilter(func(level int, message string, fields map[string]interface{}) bool {
		return false // Block all messages
	})
	
	// Try to log - should be blocked
	logger.Info("blocked message")
	logger.Sync()
	
	content, _ := os.ReadFile(tempFile)
	if len(content) > 0 {
		t.Error("Message should have been blocked by filter")
	}
	
	// Clear filters
	logger.ClearFilters()
	
	// Try to log again - should succeed
	logger.Info("allowed message")
	logger.Sync()
	logger.CloseAll()
	
	content, _ = os.ReadFile(tempFile)
	if !strings.Contains(string(content), "allowed message") {
		t.Error("Message should have been logged after clearing filters")
	}
}

func TestSetFieldFilter(t *testing.T) {
	tests := []struct {
		name         string
		filterField  string
		filterValues []interface{}
		logFields    map[string]interface{}
		shouldLog    bool
	}{
		{
			name:         "matches single field value",
			filterField:  "user",
			filterValues: []interface{}{"admin"},
			logFields:    map[string]interface{}{"user": "admin"},
			shouldLog:    true,
		},
		{
			name:         "matches one of multiple values",
			filterField:  "role",
			filterValues: []interface{}{"admin", "editor", "viewer"},
			logFields:    map[string]interface{}{"role": "editor"},
			shouldLog:    true,
		},
		{
			name:         "no match for field value",
			filterField:  "user",
			filterValues: []interface{}{"admin"},
			logFields:    map[string]interface{}{"user": "guest"},
			shouldLog:    false,
		},
		{
			name:         "field not present",
			filterField:  "user",
			filterValues: []interface{}{"admin"},
			logFields:    map[string]interface{}{"other": "value"},
			shouldLog:    false,
		},
		{
			name:         "nil fields",
			filterField:  "user",
			filterValues: []interface{}{"admin"},
			logFields:    nil,
			shouldLog:    false,
		},
		{
			name:         "matches numeric value",
			filterField:  "status",
			filterValues: []interface{}{200, 201},
			logFields:    map[string]interface{}{"status": 200},
			shouldLog:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempFile := filepath.Join(t.TempDir(), "test.log")
			
			logger, _ := flexlog.New(tempFile)
			logger.SetFieldFilter(tt.filterField, tt.filterValues...)
			
			// Log with fields
			logger.StructuredLog(flexlog.LevelInfo, "test message", tt.logFields)
			
			// Wait and close
			logger.Sync()
			logger.CloseAll()
			
			// Check result
			content, _ := os.ReadFile(tempFile)
			hasContent := len(content) > 0
			
			if hasContent != tt.shouldLog {
				t.Errorf("Expected shouldLog=%v, but hasContent=%v", tt.shouldLog, hasContent)
			}
		})
	}
}

func TestSetLevelFieldFilter(t *testing.T) {
	tests := []struct {
		name         string
		filterLevel  int
		filterField  string
		filterValue  interface{}
		logLevel     int
		logFields    map[string]interface{}
		shouldLog    bool
	}{
		{
			name:        "matches level and field",
			filterLevel: flexlog.LevelError,
			filterField: "component",
			filterValue: "database",
			logLevel:    flexlog.LevelError,
			logFields:   map[string]interface{}{"component": "database"},
			shouldLog:   true,
		},
		{
			name:        "wrong level",
			filterLevel: flexlog.LevelError,
			filterField: "component",
			filterValue: "database",
			logLevel:    flexlog.LevelInfo,
			logFields:   map[string]interface{}{"component": "database"},
			shouldLog:   false,
		},
		{
			name:        "wrong field value",
			filterLevel: flexlog.LevelError,
			filterField: "component",
			filterValue: "database",
			logLevel:    flexlog.LevelError,
			logFields:   map[string]interface{}{"component": "api"},
			shouldLog:   false,
		},
		{
			name:        "field not present",
			filterLevel: flexlog.LevelError,
			filterField: "component",
			filterValue: "database",
			logLevel:    flexlog.LevelError,
			logFields:   map[string]interface{}{"other": "value"},
			shouldLog:   false,
		},
		{
			name:        "nil fields",
			filterLevel: flexlog.LevelError,
			filterField: "component",
			filterValue: "database",
			logLevel:    flexlog.LevelError,
			logFields:   nil,
			shouldLog:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempFile := filepath.Join(t.TempDir(), "test.log")
			
			logger, _ := flexlog.New(tempFile)
			logger.SetLevelFieldFilter(tt.filterLevel, tt.filterField, tt.filterValue)
			
			// Log with appropriate level and fields
			logger.StructuredLog(tt.logLevel, "test message", tt.logFields)
			
			// Wait and close
			logger.Sync()
			logger.CloseAll()
			
			// Check result
			content, _ := os.ReadFile(tempFile)
			hasContent := len(content) > 0
			
			if hasContent != tt.shouldLog {
				t.Errorf("Expected shouldLog=%v, but hasContent=%v", tt.shouldLog, hasContent)
			}
		})
	}
}

func TestSetRegexFilter(t *testing.T) {
	tests := []struct {
		name      string
		pattern   string
		message   string
		shouldLog bool
	}{
		{
			name:      "matches pattern",
			pattern:   `user:\s*\w+`,
			message:   "Login attempt for user: admin",
			shouldLog: true,
		},
		{
			name:      "does not match pattern",
			pattern:   `user:\s*\w+`,
			message:   "System startup complete",
			shouldLog: false,
		},
		{
			name:      "matches with wildcards",
			pattern:   `error.*database`,
			message:   "error connecting to database",
			shouldLog: true,
		},
		{
			name:      "case sensitive match",
			pattern:   `ERROR`,
			message:   "error occurred",
			shouldLog: false,
		},
		{
			name:      "matches beginning of line",
			pattern:   `^Error`,
			message:   "Error: invalid input",
			shouldLog: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempFile := filepath.Join(t.TempDir(), "test.log")
			
			logger, _ := flexlog.New(tempFile)
			
			pattern, err := regexp.Compile(tt.pattern)
			if err != nil {
				t.Fatalf("Failed to compile regex: %v", err)
			}
			logger.SetRegexFilter(pattern)
			
			// Log message
			logger.Info(tt.message)
			
			// Wait and close
			logger.Sync()
			logger.CloseAll()
			
			// Check result
			content, _ := os.ReadFile(tempFile)
			hasContent := strings.Contains(string(content), tt.message)
			
			if hasContent != tt.shouldLog {
				t.Errorf("Expected shouldLog=%v, but hasContent=%v", tt.shouldLog, hasContent)
			}
		})
	}
}

func TestSetExcludeRegexFilter(t *testing.T) {
	tests := []struct {
		name      string
		pattern   string
		message   string
		shouldLog bool
	}{
		{
			name:      "excludes matching pattern",
			pattern:   `DEBUG:`,
			message:   "DEBUG: verbose output",
			shouldLog: false,
		},
		{
			name:      "allows non-matching pattern",
			pattern:   `DEBUG:`,
			message:   "INFO: normal operation",
			shouldLog: true,
		},
		{
			name:      "excludes health check logs",
			pattern:   `health.*check`,
			message:   "health check passed",
			shouldLog: false,
		},
		{
			name:      "allows other logs",
			pattern:   `health.*check`,
			message:   "user login successful",
			shouldLog: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempFile := filepath.Join(t.TempDir(), "test.log")
			
			logger, _ := flexlog.New(tempFile)
			
			pattern, err := regexp.Compile(tt.pattern)
			if err != nil {
				t.Fatalf("Failed to compile regex: %v", err)
			}
			logger.SetExcludeRegexFilter(pattern)
			
			// Log message
			logger.Info(tt.message)
			
			// Wait and close
			logger.Sync()
			logger.CloseAll()
			
			// Check result
			content, _ := os.ReadFile(tempFile)
			hasContent := strings.Contains(string(content), tt.message)
			
			if hasContent != tt.shouldLog {
				t.Errorf("Expected shouldLog=%v, but hasContent=%v", tt.shouldLog, hasContent)
			}
		})
	}
}

func TestMultipleFilters(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	
	logger, _ := flexlog.New(tempFile)
	
	// Add multiple filters - all must pass
	// 1. Level must be INFO or higher
	logger.AddFilter(func(level int, message string, fields map[string]interface{}) bool {
		return level >= flexlog.LevelInfo
	})
	
	// 2. Message must not contain "ignore"
	ignorePattern, _ := regexp.Compile(`ignore`)
	logger.SetExcludeRegexFilter(ignorePattern)
	
	// 3. Must have a specific field
	logger.SetFieldFilter("component", "api", "web")
	
	// Test various combinations
	testCases := []struct {
		level     int
		message   string
		fields    map[string]interface{}
		shouldLog bool
	}{
		{
			level:     flexlog.LevelInfo,
			message:   "API request",
			fields:    map[string]interface{}{"component": "api"},
			shouldLog: true, // All filters pass
		},
		{
			level:     flexlog.LevelDebug,
			message:   "API request",
			fields:    map[string]interface{}{"component": "api"},
			shouldLog: false, // Level filter fails
		},
		{
			level:     flexlog.LevelInfo,
			message:   "ignore this message",
			fields:    map[string]interface{}{"component": "api"},
			shouldLog: false, // Exclude filter fails
		},
		{
			level:     flexlog.LevelInfo,
			message:   "API request",
			fields:    map[string]interface{}{"component": "database"},
			shouldLog: false, // Field filter fails
		},
	}
	
	for i, tc := range testCases {
		// Clear file
		os.Truncate(tempFile, 0)
		
		// Log based on level
		logger.StructuredLog(tc.level, tc.message, tc.fields)
		
		// Check result
		logger.Sync()
		content, _ := os.ReadFile(tempFile)
		hasContent := len(content) > 0
		
		if hasContent != tc.shouldLog {
			t.Errorf("Test case %d: Expected shouldLog=%v, but hasContent=%v", 
				i, tc.shouldLog, hasContent)
		}
	}
	
	logger.CloseAll()
}

func TestFilterPerformance(t *testing.T) {
	// Set a larger channel size for this performance test
	oldSize := os.Getenv("FLEXLOG_CHANNEL_SIZE")
	os.Setenv("FLEXLOG_CHANNEL_SIZE", "10000")
	defer func() {
		if oldSize != "" {
			os.Setenv("FLEXLOG_CHANNEL_SIZE", oldSize)
		} else {
			os.Unsetenv("FLEXLOG_CHANNEL_SIZE")
		}
	}()
	
	// Create a memory destination for performance testing
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, _ := flexlog.New(tempFile)
	logger.AddCustomDestination("memory", writer) // Custom backend
	
	// Add a complex filter
	logger.AddFilter(func(level int, message string, fields map[string]interface{}) bool {
		// Simulate complex filtering logic
		if level < flexlog.LevelInfo {
			return false
		}
		if fields != nil {
			if val, ok := fields["skip"]; ok && val.(bool) {
				return false
			}
		}
		return !strings.Contains(message, "exclude")
	})
	
	// Measure filtering performance
	start := time.Now()
	iterations := 10000
	
	for i := 0; i < iterations; i++ {
		if i%2 == 0 {
			logger.Debug("This should be filtered by level")
		} else if i%3 == 0 {
			logger.StructuredLog(flexlog.LevelInfo, "Filtered by field", map[string]interface{}{"skip": true})
		} else if i%5 == 0 {
			logger.Info("This contains exclude word")
		} else {
			logger.Info("This message passes all filters")
		}
	}
	
	logger.Sync()
	elapsed := time.Since(start)
	
	// Check that filtering is reasonably fast
	perMessage := elapsed / time.Duration(iterations)
	if perMessage > time.Microsecond*100 {
		t.Logf("Warning: Filtering is slow: %v per message", perMessage)
	}
	
	logger.CloseAll()
}

func TestFilterIntegration(t *testing.T) {
	// Test filters work correctly with multiple destinations
	file1 := filepath.Join(t.TempDir(), "filtered.log")
	
	logger, _ := flexlog.New(file1)
	
	// Add filter to logger (affects all destinations)
	logger.AddFilter(func(level int, message string, fields map[string]interface{}) bool {
		return level >= flexlog.LevelWarn
	})
	
	// Log at various levels
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warning message")
	logger.Error("error message")
	
	// Wait and check
	logger.Sync()
	logger.CloseAll()
	
	// Check filtered destination
	content1, _ := os.ReadFile(file1)
	if strings.Contains(string(content1), "debug message") {
		t.Error("Debug message should have been filtered")
	}
	if strings.Contains(string(content1), "info message") {
		t.Error("Info message should have been filtered")
	}
	if !strings.Contains(string(content1), "warning message") {
		t.Error("Warning message should not have been filtered")
	}
	if !strings.Contains(string(content1), "error message") {
		t.Error("Error message should not have been filtered")
	}
}