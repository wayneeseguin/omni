package flexlog

import (
	"fmt"
	"regexp"
)

// AddFilter adds a filter function that determines whether a log entry should be logged.
// Filters are applied in the order they were added. A message is logged only if all
// filters return true.
//
// Parameters:
//   - filter: Function that returns true to log the message, false to filter it out
//
// Returns:
//   - error: If the filter is nil
//
// Example:
//
//	logger.AddFilter(func(level int, msg string, fields map[string]interface{}) bool {
//	    return level >= flexlog.LevelWarn  // Only log warnings and above
//	})
func (f *FlexLog) AddFilter(filter FilterFunc) error {
	if filter == nil {
		return fmt.Errorf("filter cannot be nil")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.filters = append(f.filters, Filter(filter))
	return nil
}

// RemoveFilter removes a specific filter.
// Note: This is currently a no-op since Go functions cannot be compared.
// Use ClearFilters() to remove all filters instead.
//
// Parameters:
//   - filter: The filter to remove (currently ignored)
//
// Returns:
//   - error: Always returns nil
func (f *FlexLog) RemoveFilter(filter FilterFunc) error {
	// In Go, functions cannot be compared, so we can't remove a specific filter
	// This is a limitation of the interface design
	return nil
}

// ClearFilters removes all filters from the logger.
// After calling this, all messages will be logged (subject to level and sampling).
func (f *FlexLog) ClearFilters() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.filters = nil
}

// SetFieldFilter adds a filter that only logs entries containing specific field values.
// The filter checks if the specified field exists and matches any of the provided values.
//
// Parameters:
//   - field: The field name to check
//   - values: One or more values to match against
//
// Example:
//
//	logger.SetFieldFilter("environment", "production", "staging")  // Only log prod/staging
//	logger.SetFieldFilter("user_id", 12345, 67890)                // Only log specific users
func (f *FlexLog) SetFieldFilter(field string, values ...interface{}) {
	f.AddFilter(func(level int, message string, fields map[string]interface{}) bool {
		if fields == nil {
			return false
		}

		val, exists := fields[field]
		if !exists {
			return false
		}

		for _, v := range values {
			if val == v {
				return true
			}
		}
		return false
	})
}

// SetLevelFieldFilter adds a filter that only logs entries with a specific level and field value.
// This allows fine-grained control over what gets logged based on both level and context.
//
// Parameters:
//   - logLevel: The log level to filter for
//   - field: The field name to check
//   - value: The value the field must have
//
// Example:
//
//	logger.SetLevelFieldFilter(flexlog.LevelError, "component", "database")  // Only DB errors
func (f *FlexLog) SetLevelFieldFilter(logLevel int, field string, value interface{}) {
	f.AddFilter(func(level int, message string, fields map[string]interface{}) bool {
		if level != logLevel {
			return false
		}

		if fields == nil {
			return false
		}

		val, exists := fields[field]
		return exists && val == value
	})
}

// SetRegexFilter adds a filter that only logs entries matching a regex pattern.
// Messages that don't match the pattern will be filtered out.
//
// Parameters:
//   - pattern: Compiled regular expression to match against messages
//
// Example:
//
//	pattern := regexp.MustCompile(`(?i)error|fail|critical`)
//	logger.SetRegexFilter(pattern)  // Only log messages containing error-related words
func (f *FlexLog) SetRegexFilter(pattern *regexp.Regexp) {
	f.AddFilter(func(level int, message string, fields map[string]interface{}) bool {
		return pattern.MatchString(message)
	})
}

// SetExcludeRegexFilter adds a filter that excludes entries matching a regex pattern.
// Messages that match the pattern will be filtered out (opposite of SetRegexFilter).
//
// Parameters:
//   - pattern: Compiled regular expression to match against messages to exclude
//
// Example:
//
//	pattern := regexp.MustCompile(`(?i)debug|trace|verbose`)
//	logger.SetExcludeRegexFilter(pattern)  // Exclude verbose/debug messages
func (f *FlexLog) SetExcludeRegexFilter(pattern *regexp.Regexp) {
	f.AddFilter(func(level int, message string, fields map[string]interface{}) bool {
		return !pattern.MatchString(message)
	})
}
