package flexlog

import (
	"fmt"
)

// Debug logs a debug message
func (f *FlexLog) Debug(args ...interface{}) {
	if f.level <= LevelDebug {
		f.flexlogf("[DEBUG] %s", fmt.Sprint(args...))
	}
}

// Debugf logs a formatted debug message
func (f *FlexLog) Debugf(format string, args ...interface{}) {
	if f.level <= LevelDebug {
		f.flexlogf("[DEBUG] %s", fmt.Sprintf(format, args...))
	}
}

// Info logs an info message
func (f *FlexLog) Info(args ...interface{}) {
	if f.level <= LevelInfo {
		f.flexlogf("[INFO] %s", fmt.Sprint(args...))
	}
}

// Infof logs a formatted info message
func (f *FlexLog) Infof(format string, args ...interface{}) {
	if f.level <= LevelInfo {
		f.flexlogf("[INFO] %s", fmt.Sprintf(format, args...))
	}
}

// Warn logs a warning message
func (f *FlexLog) Warn(args ...interface{}) {
	if f.level <= LevelWarn {
		f.flexlogf("[WARN] %s", fmt.Sprint(args...))
	}
}

// Warnf logs a formatted warning message
func (f *FlexLog) Warnf(format string, args ...interface{}) {
	if f.level <= LevelWarn {
		f.flexlogf("[WARN] %s", fmt.Sprintf(format, args...))
	}
}

// Error logs an error message
func (f *FlexLog) Error(args ...interface{}) {
	if f.level <= LevelError {
		f.flexlogf("[ERROR] %s", fmt.Sprint(args...))
	}
}

// Errorf logs a formatted error message
func (f *FlexLog) Errorf(format string, args ...interface{}) {
	if f.level <= LevelError {
		f.flexlogf("[ERROR] %s", fmt.Sprintf(format, args...))
	}
}
