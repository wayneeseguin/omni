package formatters

import (
	"testing"
	"time"
)

func TestDefaultFormatOptions(t *testing.T) {
	opts := DefaultFormatOptions()

	if opts.TimestampFormat != time.RFC3339 {
		t.Errorf("expected RFC3339 timestamp format, got %s", opts.TimestampFormat)
	}

	if !opts.IncludeLevel {
		t.Error("expected IncludeLevel to be true by default")
	}

	if !opts.IncludeTime {
		t.Error("expected IncludeTime to be true by default")
	}

	if opts.LevelFormat != LevelFormatName {
		t.Errorf("expected LevelFormatName by default, got %v", opts.LevelFormat)
	}

	if opts.IndentJSON {
		t.Error("expected IndentJSON to be false by default")
	}

	if opts.FieldSeparator != " " {
		t.Errorf("expected space as field separator, got %q", opts.FieldSeparator)
	}

	if opts.TimeZone != time.UTC {
		t.Error("expected UTC timezone by default")
	}

	if opts.FlattenFields {
		t.Error("expected FlattenFields to be false by default")
	}

	if opts.IncludeSource {
		t.Error("expected IncludeSource to be false by default")
	}

	if opts.IncludeHost {
		t.Error("expected IncludeHost to be false by default")
	}
}

func TestLevelFormatConstants(t *testing.T) {
	// Ensure constants have expected values
	if LevelFormatName != 0 {
		t.Error("LevelFormatName should be 0")
	}
	if LevelFormatNameUpper != 1 {
		t.Error("LevelFormatNameUpper should be 1")
	}
	if LevelFormatNameLower != 2 {
		t.Error("LevelFormatNameLower should be 2")
	}
	if LevelFormatSymbol != 3 {
		t.Error("LevelFormatSymbol should be 3")
	}
}

func TestLogLevelConstants(t *testing.T) {
	// Ensure log level constants have expected values and order
	if LevelTrace != 0 {
		t.Error("LevelTrace should be 0")
	}
	if LevelDebug != 1 {
		t.Error("LevelDebug should be 1")
	}
	if LevelInfo != 2 {
		t.Error("LevelInfo should be 2")
	}
	if LevelWarn != 3 {
		t.Error("LevelWarn should be 3")
	}
	if LevelError != 4 {
		t.Error("LevelError should be 4")
	}

	// Ensure levels are in increasing order
	levels := []int{LevelTrace, LevelDebug, LevelInfo, LevelWarn, LevelError}
	for i := 1; i < len(levels); i++ {
		if levels[i] <= levels[i-1] {
			t.Errorf("log levels should be in increasing order, but %d <= %d", levels[i], levels[i-1])
		}
	}
}

func TestFormatConstants(t *testing.T) {
	// Ensure format constants have expected values
	if FormatText != 0 {
		t.Error("FormatText should be 0")
	}
	if FormatJSON != 1 {
		t.Error("FormatJSON should be 1")
	}
	if FormatCustom != 2 {
		t.Error("FormatCustom should be 2")
	}
}

func TestFormatOptions_Copy(t *testing.T) {
	// Test that format options can be copied properly
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		loc = time.UTC
	}

	original := FormatOptions{
		TimestampFormat: "2006-01-02",
		IncludeLevel:    false,
		IncludeTime:     false,
		LevelFormat:     LevelFormatSymbol,
		IndentJSON:      true,
		FieldSeparator:  " | ",
		TimeZone:        loc,
		FlattenFields:   true,
		IncludeSource:   true,
		IncludeHost:     true,
	}

	// Copy the options
	copied := original

	// Verify all fields are copied
	if copied.TimestampFormat != original.TimestampFormat {
		t.Error("TimestampFormat not copied correctly")
	}
	if copied.IncludeLevel != original.IncludeLevel {
		t.Error("IncludeLevel not copied correctly")
	}
	if copied.IncludeTime != original.IncludeTime {
		t.Error("IncludeTime not copied correctly")
	}
	if copied.LevelFormat != original.LevelFormat {
		t.Error("LevelFormat not copied correctly")
	}
	if copied.IndentJSON != original.IndentJSON {
		t.Error("IndentJSON not copied correctly")
	}
	if copied.FieldSeparator != original.FieldSeparator {
		t.Error("FieldSeparator not copied correctly")
	}
	if copied.TimeZone != original.TimeZone {
		t.Error("TimeZone not copied correctly")
	}
	if copied.FlattenFields != original.FlattenFields {
		t.Error("FlattenFields not copied correctly")
	}
	if copied.IncludeSource != original.IncludeSource {
		t.Error("IncludeSource not copied correctly")
	}
	if copied.IncludeHost != original.IncludeHost {
		t.Error("IncludeHost not copied correctly")
	}
}
