package flexlog

import (
	"fmt"
	"regexp"
)

// AddFilter adds a filter function that determines whether a log entry should be logged
func (f *FlexLog) AddFilter(filter FilterFunc) error {
	if filter == nil {
		return fmt.Errorf("filter cannot be nil")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.filters = append(f.filters, Filter(filter))
	return nil
}

// RemoveFilter removes a specific filter (note: this is a no-op since functions can't be compared)
func (f *FlexLog) RemoveFilter(filter FilterFunc) error {
	// In Go, functions cannot be compared, so we can't remove a specific filter
	// This is a limitation of the interface design
	return nil
}

// ClearFilters removes all filters
func (f *FlexLog) ClearFilters() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.filters = nil
}

// SetFieldFilter adds a filter that only logs entries containing specific field values
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

// SetLevelFieldFilter adds a filter that only logs entries with a specific level and field value
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

// SetRegexFilter adds a filter that only logs entries matching a regex pattern
func (f *FlexLog) SetRegexFilter(pattern *regexp.Regexp) {
	f.AddFilter(func(level int, message string, fields map[string]interface{}) bool {
		return pattern.MatchString(message)
	})
}

// SetExcludeRegexFilter adds a filter that excludes entries matching a regex pattern
func (f *FlexLog) SetExcludeRegexFilter(pattern *regexp.Regexp) {
	f.AddFilter(func(level int, message string, fields map[string]interface{}) bool {
		return !pattern.MatchString(message)
	})
}
