package formatters

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/wayneeseguin/omni/pkg/types"
)

func TestTextFormatter_Format(t *testing.T) {
	tests := []struct {
		name    string
		msg     types.LogMessage
		options FormatOptions
		wantErr bool
		check   func(t *testing.T, result string)
	}{
		{
			name: "basic message",
			msg: types.LogMessage{
				Level:     LevelInfo,
				Format:    "test message",
				Timestamp: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			},
			options: DefaultFormatOptions(),
			check: func(t *testing.T, result string) {
				if !strings.Contains(result, "test message") {
					t.Errorf("expected 'test message' in output, got %s", result)
				}
				if !strings.Contains(result, "[INFO]") {
					t.Errorf("expected '[INFO]' in output, got %s", result)
				}
				if !strings.Contains(result, "2023-01-01") {
					t.Errorf("expected timestamp in output, got %s", result)
				}
			},
		},
		{
			name: "message with args",
			msg: types.LogMessage{
				Level:     LevelDebug,
				Format:    "message with %s and %d",
				Args:      []interface{}{"string", 42},
				Timestamp: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			},
			options: DefaultFormatOptions(),
			check: func(t *testing.T, result string) {
				if !strings.Contains(result, "message with string and 42") {
					t.Errorf("expected formatted message, got %s", result)
				}
			},
		},
		{
			name: "raw bytes passthrough",
			msg: types.LogMessage{
				Raw: []byte("raw log data"),
			},
			options: DefaultFormatOptions(),
			check: func(t *testing.T, result string) {
				if result != "raw log data" {
					t.Errorf("expected raw data passthrough, got %s", result)
				}
			},
		},
		{
			name: "structured entry",
			msg: types.LogMessage{
				Entry: &types.LogEntry{
					Level:     "error",
					Message:   "structured message",
					Timestamp: "2023-01-01T12:00:00Z",
					Fields: map[string]interface{}{
						"user_id": 123,
						"action":  "login",
					},
					StackTrace: "stack trace here",
				},
			},
			options: DefaultFormatOptions(),
			check: func(t *testing.T, result string) {
				if !strings.Contains(result, "structured message") {
					t.Errorf("expected message in output, got %s", result)
				}
				if !strings.Contains(result, "user_id=123") {
					t.Errorf("expected user_id field in output, got %s", result)
				}
				if !strings.Contains(result, "action=login") {
					t.Errorf("expected action field in output, got %s", result)
				}
				if !strings.Contains(result, "stack_trace=stack trace here") {
					t.Errorf("expected stack trace in output, got %s", result)
				}
			},
		},
		{
			name: "without timestamp",
			msg: types.LogMessage{
				Level:     LevelInfo,
				Format:    "no timestamp",
				Timestamp: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			},
			options: FormatOptions{
				IncludeTime:  false,
				IncludeLevel: true,
				TimeZone:     time.UTC,
			},
			check: func(t *testing.T, result string) {
				if strings.Contains(result, "2023") {
					t.Errorf("timestamp should not be included, got %s", result)
				}
				if !strings.HasPrefix(result, "[INFO]") {
					t.Errorf("expected to start with [INFO], got %s", result)
				}
			},
		},
		{
			name: "without level",
			msg: types.LogMessage{
				Level:     LevelInfo,
				Format:    "no level",
				Timestamp: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			},
			options: FormatOptions{
				IncludeTime:     true,
				IncludeLevel:    false,
				TimeZone:        time.UTC,
				TimestampFormat: time.RFC3339,
			},
			check: func(t *testing.T, result string) {
				if strings.Contains(result, "[INFO]") {
					t.Errorf("level should not be included, got %s", result)
				}
				if !strings.Contains(result, "2023") {
					t.Errorf("expected timestamp in output, got %s", result)
				}
			},
		},
		{
			name: "all log levels",
			msg: types.LogMessage{
				Format:    "level test",
				Timestamp: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			},
			options: DefaultFormatOptions(),
			check: func(t *testing.T, result string) {
				levels := []struct {
					level    int
					expected string
				}{
					{LevelTrace, "[TRACE]"},
					{LevelDebug, "[DEBUG]"},
					{LevelInfo, "[INFO]"},
					{LevelWarn, "[WARN]"},
					{LevelError, "[ERROR]"},
					{999, "[LOG]"}, // unknown level
				}

				for _, lvl := range levels {
					f := NewTextFormatter()
					f.Options = DefaultFormatOptions()
					msg := types.LogMessage{
						Level:     lvl.level,
						Format:    "level test",
						Timestamp: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
					}
					data, err := f.Format(msg)
					if err != nil {
						t.Fatalf("failed to format: %v", err)
					}
					if !strings.Contains(string(data), lvl.expected) {
						t.Errorf("expected %s for level %d, got %s", lvl.expected, lvl.level, string(data))
					}
				}
			},
		},
		{
			name: "level format uppercase",
			msg: types.LogMessage{
				Level:     LevelInfo,
				Format:    "uppercase",
				Timestamp: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			},
			options: FormatOptions{
				IncludeTime:  true,
				IncludeLevel: true,
				LevelFormat:  LevelFormatNameUpper,
				TimeZone:     time.UTC,
			},
			check: func(t *testing.T, result string) {
				if !strings.Contains(result, "[INFO]") {
					t.Errorf("expected uppercase INFO, got %s", result)
				}
			},
		},
		{
			name: "level format lowercase",
			msg: types.LogMessage{
				Level:     LevelInfo,
				Format:    "lowercase",
				Timestamp: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			},
			options: FormatOptions{
				IncludeTime:  true,
				IncludeLevel: true,
				LevelFormat:  LevelFormatNameLower,
				TimeZone:     time.UTC,
			},
			check: func(t *testing.T, result string) {
				if !strings.Contains(result, "[info]") {
					t.Errorf("expected lowercase info, got %s", result)
				}
			},
		},
		{
			name: "level format symbol",
			msg: types.LogMessage{
				Level:     LevelError,
				Format:    "symbol",
				Timestamp: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			},
			options: FormatOptions{
				IncludeTime:  true,
				IncludeLevel: true,
				LevelFormat:  LevelFormatSymbol,
				TimeZone:     time.UTC,
			},
			check: func(t *testing.T, result string) {
				if !strings.Contains(result, "[E]") {
					t.Errorf("expected symbol E, got %s", result)
				}
			},
		},
		{
			name: "custom timestamp format",
			msg: types.LogMessage{
				Level:     LevelInfo,
				Format:    "custom time",
				Timestamp: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			},
			options: FormatOptions{
				IncludeTime:     true,
				IncludeLevel:    true,
				TimestampFormat: "2006-01-02 15:04:05",
				TimeZone:        time.UTC,
			},
			check: func(t *testing.T, result string) {
				if !strings.Contains(result, "[2023-01-01 12:00:00]") {
					t.Errorf("expected custom timestamp format, got %s", result)
				}
			},
		},
		{
			name: "message with newline",
			msg: types.LogMessage{
				Level:     LevelInfo,
				Format:    "message with newline\n",
				Timestamp: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			},
			options: DefaultFormatOptions(),
			check: func(t *testing.T, result string) {
				// Should not add extra newline
				if strings.Count(result, "\n") != 1 {
					t.Errorf("expected single newline, got %d", strings.Count(result, "\n"))
				}
			},
		},
		{
			name: "empty structured entry fields",
			msg: types.LogMessage{
				Entry: &types.LogEntry{
					Level:     "info",
					Message:   "no fields",
					Timestamp: "2023-01-01T12:00:00Z",
					Fields:    map[string]interface{}{},
				},
			},
			options: DefaultFormatOptions(),
			check: func(t *testing.T, result string) {
				if !strings.Contains(result, "no fields") {
					t.Errorf("expected message in output, got %s", result)
				}
				// Should not have trailing spaces from empty fields
				lines := strings.Split(strings.TrimSpace(result), "\n")
				for _, line := range lines {
					if strings.HasSuffix(line, " ") {
						t.Errorf("line should not end with space: %q", line)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewTextFormatter()
			f.Options = tt.options

			result, err := f.Format(tt.msg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Format() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.check != nil {
				tt.check(t, string(result))
			}

			// Verify newline is added (except for raw passthrough)
			if tt.msg.Raw == nil && !strings.HasSuffix(string(result), "\n") {
				t.Error("expected newline at end of text output")
			}
		})
	}
}

func TestTextFormatter_FormatFields(t *testing.T) {
	f := NewTextFormatter()

	tests := []struct {
		name     string
		fields   map[string]interface{}
		expected string
	}{
		{
			name:     "empty fields",
			fields:   map[string]interface{}{},
			expected: "",
		},
		{
			name: "single field",
			fields: map[string]interface{}{
				"key": "value",
			},
			expected: "key=value",
		},
		{
			name: "multiple fields",
			fields: map[string]interface{}{
				"user_id": 123,
				"action":  "login",
				"success": true,
			},
			expected: "=", // Should contain key=value patterns
		},
		{
			name: "special characters",
			fields: map[string]interface{}{
				"message": "hello world",
				"path":    "/var/log/app.log",
			},
			expected: "=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := f.FormatFields(tt.fields)

			if tt.expected == "" && result != "" {
				t.Errorf("expected empty string, got %s", result)
			} else if tt.expected != "" && !strings.Contains(result, tt.expected) {
				t.Errorf("expected result to contain %s, got %s", tt.expected, result)
			}

			// Verify all fields are present
			for k, v := range tt.fields {
				expected := fmt.Sprintf("%s=%v", k, v)
				if !strings.Contains(result, expected) {
					t.Errorf("expected field %s in result, got %s", expected, result)
				}
			}
		})
	}
}

func TestTextFormatter_TimeZone(t *testing.T) {
	f := NewTextFormatter()

	// Set timezone to EST
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("America/New_York timezone not available")
	}
	f.Options.TimeZone = loc
	f.Options.TimestampFormat = "2006-01-02 15:04:05 MST"

	msg := types.LogMessage{
		Level:     LevelInfo,
		Format:    "timezone test",
		Timestamp: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
	}

	result, err := f.Format(msg)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	// UTC 12:00 should be EST 07:00
	if !strings.Contains(string(result), "07:00:00") {
		t.Errorf("expected time in EST, got %s", string(result))
	}
}

func TestTextFormatter_EmptyMessage(t *testing.T) {
	f := NewTextFormatter()

	msg := types.LogMessage{
		Level:     LevelInfo,
		Format:    "",
		Timestamp: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
	}

	result, err := f.Format(msg)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	// Should still have timestamp and level
	if !strings.Contains(string(result), "[INFO]") {
		t.Errorf("expected level in output, got %s", string(result))
	}

	// Should end with newline
	if !strings.HasSuffix(string(result), "\n") {
		t.Error("expected newline at end")
	}
}

func TestTextFormatter_NilFields(t *testing.T) {
	f := NewTextFormatter()

	msg := types.LogMessage{
		Entry: &types.LogEntry{
			Level:     "info",
			Message:   "nil test",
			Timestamp: "2023-01-01T12:00:00Z",
			Fields: map[string]interface{}{
				"nil_field":   nil,
				"valid_field": "value",
			},
		},
	}

	result, err := f.Format(msg)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	// Should handle nil gracefully
	if !strings.Contains(string(result), "nil_field=<nil>") {
		t.Errorf("expected nil_field=<nil> in output, got %s", string(result))
	}
	if !strings.Contains(string(result), "valid_field=value") {
		t.Errorf("expected valid_field=value in output, got %s", string(result))
	}
}
