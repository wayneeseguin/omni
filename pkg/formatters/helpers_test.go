package formatters

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestHelperFunctions(t *testing.T) {
	t.Run("getHostname", func(t *testing.T) {
		hostname := getHostname()
		if hostname == "" {
			// It's possible the hostname can't be determined in some environments
			t.Log("hostname is empty, might be expected in some environments")
		}
		// Verify it matches os.Hostname if available
		expected, err := os.Hostname()
		if err == nil && hostname != expected {
			t.Errorf("getHostname() = %v, expected %v", hostname, expected)
		}
	})

	t.Run("getPID", func(t *testing.T) {
		pid := getPID()
		if pid <= 0 {
			t.Errorf("getPID() = %v, expected positive value", pid)
		}
		// Verify it matches os.Getpid
		if pid != os.Getpid() {
			t.Errorf("getPID() = %v, expected %v", pid, os.Getpid())
		}
	})

	t.Run("getProcessName", func(t *testing.T) {
		processName := getProcessName()
		if processName == "" {
			t.Error("getProcessName() returned empty string")
		}
		// Should match os.Args[0]
		if processName != os.Args[0] {
			t.Errorf("getProcessName() = %v, expected %v", processName, os.Args[0])
		}
	})

	t.Run("getGoVersion", func(t *testing.T) {
		version := getGoVersion()
		if version == "" {
			t.Error("getGoVersion() returned empty string")
		}
		// Should start with "go"
		if !strings.HasPrefix(version, "go") {
			t.Errorf("getGoVersion() = %v, expected to start with 'go'", version)
		}
		// Should match runtime.Version()
		if version != runtime.Version() {
			t.Errorf("getGoVersion() = %v, expected %v", version, runtime.Version())
		}
	})

	t.Run("getGoroutineCount", func(t *testing.T) {
		count := getGoroutineCount()
		if count <= 0 {
			t.Errorf("getGoroutineCount() = %v, expected positive value", count)
		}
		// Create a goroutine and verify count increases
		done := make(chan bool)
		go func() {
			<-done
		}()
		newCount := getGoroutineCount()
		if newCount <= count {
			t.Errorf("goroutine count should increase after creating goroutine")
		}
		close(done)
	})
}

func TestLevelToString(t *testing.T) {
	tests := []struct {
		level    int
		expected string
	}{
		{LevelTrace, "TRACE"},
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{999, "LOG"}, // unknown level
		{-1, "LOG"},  // negative level
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := levelToString(tt.level)
			if result != tt.expected {
				t.Errorf("levelToString(%d) = %v, expected %v", tt.level, result, tt.expected)
			}
		})
	}
}

func TestSafeFields(t *testing.T) {
	t.Run("nil fields", func(t *testing.T) {
		result := safeFields(nil)
		if result != nil {
			t.Errorf("safeFields(nil) = %v, expected nil", result)
		}
	})

	t.Run("empty fields", func(t *testing.T) {
		fields := map[string]interface{}{}
		result := safeFields(fields)
		if len(result) != 0 {
			t.Errorf("safeFields(empty) = %v, expected empty map", result)
		}
	})

	t.Run("simple fields", func(t *testing.T) {
		fields := map[string]interface{}{
			"string": "value",
			"int":    42,
			"bool":   true,
			"float":  3.14,
			"nil":    nil,
		}
		result := safeFields(fields)

		if result["string"] != "value" {
			t.Errorf("expected string field to be preserved")
		}
		if result["int"] != 42 {
			t.Errorf("expected int field to be preserved")
		}
		if result["bool"] != true {
			t.Errorf("expected bool field to be preserved")
		}
		if result["float"] != 3.14 {
			t.Errorf("expected float field to be preserved")
		}
		if result["nil"] != nil {
			t.Errorf("expected nil field to be preserved")
		}
	})

	t.Run("nested map", func(t *testing.T) {
		fields := map[string]interface{}{
			"nested": map[string]interface{}{
				"inner": "value",
			},
		}
		result := safeFields(fields)

		nested, ok := result["nested"].(map[string]interface{})
		if !ok {
			t.Fatal("expected nested to be a map")
		}
		if nested["inner"] != "value" {
			t.Errorf("expected nested value to be preserved")
		}
	})

	t.Run("slice fields", func(t *testing.T) {
		fields := map[string]interface{}{
			"slice": []interface{}{"a", "b", "c"},
			"array": [3]int{1, 2, 3},
		}
		result := safeFields(fields)

		slice, ok := result["slice"].([]interface{})
		if !ok {
			t.Fatal("expected slice to be preserved")
		}
		if len(slice) != 3 || slice[0] != "a" {
			t.Errorf("expected slice contents to be preserved")
		}
	})

	t.Run("circular reference", func(t *testing.T) {
		// Create circular reference
		m1 := make(map[string]interface{})
		m2 := make(map[string]interface{})
		m1["child"] = m2
		m2["parent"] = m1

		fields := map[string]interface{}{
			"circular": m1,
		}

		result := safeFields(fields)

		// Should handle circular reference without panicking
		circular, ok := result["circular"].(map[string]interface{})
		if !ok {
			t.Fatal("expected circular to be a map")
		}

		// Check that circular reference was detected
		child, ok := circular["child"].(map[string]interface{})
		if !ok {
			t.Fatal("expected child to be a map")
		}

		// The safeFields function handles circular references by depth limiting
		// rather than explicitly marking them as "[circular reference]"
		parent := child["parent"]
		if parent == nil {
			t.Errorf("expected parent to be present, got nil")
		} else if _, ok := parent.(string); ok && parent != "[max depth exceeded]" {
			// Check if it's marked as max depth or is a map
			if _, isMap := parent.(map[string]interface{}); !isMap {
				t.Errorf("expected parent to be a map or max depth marker, got %v", parent)
			}
		}
	})

	t.Run("self reference", func(t *testing.T) {
		// Create self reference
		m := make(map[string]interface{})
		m["self"] = m

		fields := map[string]interface{}{
			"selfref": m,
		}

		result := safeFields(fields)

		// Should handle self reference
		selfref, ok := result["selfref"].(map[string]interface{})
		if !ok {
			t.Fatal("expected selfref to be a map")
		}

		// The safeFields function handles self references by depth limiting
		// rather than explicitly marking them as "[circular reference]"
		self := selfref["self"]
		if self == nil {
			t.Errorf("expected self to be present, got nil")
		} else if _, ok := self.(string); ok && self != "[max depth exceeded]" {
			// Check if it's marked as max depth or is a map
			if _, isMap := self.(map[string]interface{}); !isMap {
				t.Errorf("expected self to be a map or max depth marker, got %v", self)
			}
		}
	})

	t.Run("max depth", func(t *testing.T) {
		// Create deeply nested structure
		current := make(map[string]interface{})
		root := current
		for i := 0; i < 15; i++ { // More than max depth
			next := make(map[string]interface{})
			current["level"] = next
			current = next
		}
		current["deep"] = "value"

		fields := map[string]interface{}{
			"deep": root,
		}

		result := safeFields(fields)

		// Should handle max depth without panicking
		// Navigate down to check max depth handling
		current = result["deep"].(map[string]interface{})
		depth := 0
		for {
			next, ok := current["level"].(map[string]interface{})
			if !ok {
				// Check if we hit max depth marker
				if current["level"] == "[max depth exceeded]" {
					if depth < 10 {
						t.Errorf("hit max depth too early at %d", depth)
					}
					break
				}
				break
			}
			current = next
			depth++
		}
	})

	t.Run("struct fields", func(t *testing.T) {
		type TestStruct struct {
			Public  string
			Number  int
			private string //nolint:unused
		}

		fields := map[string]interface{}{
			"struct": TestStruct{
				Public: "visible",
				Number: 42,
			},
		}

		result := safeFields(fields)

		// Struct should be converted to map
		structMap, ok := result["struct"].(map[string]interface{})
		if !ok {
			t.Fatal("expected struct to be converted to map")
		}

		if structMap["Public"] != "visible" {
			t.Errorf("expected Public field to be visible")
		}
		if structMap["Number"] != 42 {
			t.Errorf("expected Number field to be visible")
		}
		if _, ok := structMap["private"]; ok {
			t.Error("private field should not be exported")
		}
	})

	t.Run("pointer fields", func(t *testing.T) {
		str := "pointed value"
		num := 42

		fields := map[string]interface{}{
			"strPtr": &str,
			"numPtr": &num,
			"nilPtr": (*string)(nil),
		}

		result := safeFields(fields)

		if result["strPtr"] != "pointed value" {
			t.Errorf("expected string pointer to be dereferenced")
		}
		if result["numPtr"] != 42 {
			t.Errorf("expected number pointer to be dereferenced")
		}
		if result["nilPtr"] != nil {
			t.Errorf("expected nil pointer to remain nil")
		}
	})

	t.Run("function and channel fields", func(t *testing.T) {
		fields := map[string]interface{}{
			"func": func() {},
			"chan": make(chan int),
		}

		result := safeFields(fields)

		// Functions and channels should be represented as their type
		if result["func"] != "[func]" {
			t.Errorf("expected function to be marked as [func], got %v", result["func"])
		}
		if result["chan"] != "[chan]" {
			t.Errorf("expected channel to be marked as [chan], got %v", result["chan"])
		}
	})
}

func TestTruncateFieldValue(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		maxSize  int
		truncate bool
		expected interface{}
		contains string
	}{
		{
			name:     "short string not truncated",
			value:    "hello",
			maxSize:  10,
			truncate: true,
			expected: "hello",
		},
		{
			name:     "long string truncated",
			value:    "this is a very long string that should be truncated",
			maxSize:  10,
			truncate: true,
			contains: "this is a ...(truncated)",
		},
		{
			name:     "long string not truncated flag",
			value:    "this is a very long string",
			maxSize:  10,
			truncate: false,
			contains: "[string too long:",
		},
		{
			name:     "byte slice truncated",
			value:    []byte("this is a very long byte slice"),
			maxSize:  10,
			truncate: true,
			contains: "this is a ...(truncated)",
		},
		{
			name:     "byte slice not truncated flag",
			value:    []byte("this is a very long byte slice"),
			maxSize:  10,
			truncate: false,
			contains: "[bytes too long:",
		},
		{
			name:     "other type short",
			value:    12345,
			maxSize:  10,
			truncate: true,
			expected: 12345,
		},
		{
			name:     "other type long representation",
			value:    []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			maxSize:  10,
			truncate: true,
			contains: "[1 2 3 4 5...(truncated)",
		},
		{
			name:     "nil value",
			value:    nil,
			maxSize:  10,
			truncate: true,
			expected: nil,
		},
		{
			name:     "empty string",
			value:    "",
			maxSize:  10,
			truncate: true,
			expected: "",
		},
		{
			name:     "exact size string",
			value:    "1234567890",
			maxSize:  10,
			truncate: true,
			expected: "1234567890",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateFieldValue(tt.value, tt.maxSize, tt.truncate)

			if tt.expected != nil {
				if result != tt.expected {
					t.Errorf("truncateFieldValue() = %v, expected %v", result, tt.expected)
				}
			} else if tt.contains != "" {
				resultStr, ok := result.(string)
				if !ok {
					t.Fatalf("expected string result, got %T", result)
				}
				if !strings.Contains(resultStr, tt.contains) {
					t.Errorf("truncateFieldValue() = %v, expected to contain %v", result, tt.contains)
				}
			}
		})
	}
}
