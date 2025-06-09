package formatters

import (
	"fmt"
	"os"
	"reflect"
	"runtime"
)

var (
	hostname    string
	processName string
	goVersion   string
)

func init() {
	// Cache values that don't change
	hostname, _ = os.Hostname()
	processName = os.Args[0]
	goVersion = runtime.Version()
}

// getHostname returns the cached hostname
func getHostname() string {
	return hostname
}

// getPID returns the current process ID
func getPID() int {
	return os.Getpid()
}

// getProcessName returns the cached process name
func getProcessName() string {
	return processName
}

// getGoVersion returns the cached Go version
func getGoVersion() string {
	return goVersion
}

// getGoroutineCount returns the current number of goroutines
func getGoroutineCount() int {
	return runtime.NumGoroutine()
}

// levelToString converts a numeric log level to its string representation
func levelToString(level int) string {
	switch level {
	case LevelTrace:
		return "TRACE"
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "LOG"
	}
}

// safeFields creates a safe copy of fields that handles circular references
func safeFields(fields map[string]interface{}) map[string]interface{} {
	if fields == nil {
		return nil
	}

	// Use a set to track visited values by their address
	visited := make(map[uintptr]bool)
	result := make(map[string]interface{})

	for k, v := range fields {
		result[k] = safeFieldsCopy(v, visited, 0)
	}

	return result
}

// safeFieldsCopy recursively copies fields with circular reference detection
func safeFieldsCopy(value interface{}, visited map[uintptr]bool, depth int) interface{} {
	// Limit recursion depth to prevent stack overflow
	const maxDepth = 10
	if depth > maxDepth {
		return "[max depth exceeded]"
	}

	if value == nil {
		return nil
	}

	// Use reflection to handle different types
	v := reflect.ValueOf(value)

	switch v.Kind() {
	case reflect.Map:
		// Check if this is a reference type that could be circular
		if v.CanAddr() {
			addr := v.Pointer()
			if visited[addr] {
				return "[circular reference]"
			}
			visited[addr] = true
			defer delete(visited, addr)
		}

		// Create safe copy
		result := make(map[string]interface{})
		iter := v.MapRange()
		for iter.Next() {
			key := iter.Key()
			if key.Kind() == reflect.String {
				result[key.String()] = safeFieldsCopy(iter.Value().Interface(), visited, depth+1)
			}
		}
		return result

	case reflect.Slice, reflect.Array:
		// Check if this is a reference type that could be circular
		if v.CanAddr() {
			addr := v.Pointer()
			if visited[addr] {
				return "[circular reference]"
			}
			visited[addr] = true
			defer delete(visited, addr)
		}

		// Create safe copy
		result := make([]interface{}, v.Len())
		for i := 0; i < v.Len(); i++ {
			result[i] = safeFieldsCopy(v.Index(i).Interface(), visited, depth+1)
		}
		return result

	case reflect.Ptr:
		if v.IsNil() {
			return nil
		}
		// Check for circular reference
		addr := v.Pointer()
		if visited[addr] {
			return "[circular reference]"
		}
		visited[addr] = true
		defer delete(visited, addr)

		return safeFieldsCopy(v.Elem().Interface(), visited, depth+1)

	case reflect.Struct:
		// Convert struct to map for JSON serialization
		result := make(map[string]interface{})
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			field := t.Field(i)
			if field.IsExported() {
				result[field.Name] = safeFieldsCopy(v.Field(i).Interface(), visited, depth+1)
			}
		}
		return result

	default:
		// Primitive types, functions, channels, etc.
		if v.Kind() == reflect.Func || v.Kind() == reflect.Chan {
			return fmt.Sprintf("[%s]", v.Kind())
		}
		return value
	}
}
