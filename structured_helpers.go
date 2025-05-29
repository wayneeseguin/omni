package flexlog

import (
	"os"
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