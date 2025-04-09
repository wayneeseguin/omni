package flexlog

import (
	"fmt"
	"os"
	"time"
)

// SetLevel sets the minimum log level
func (f *FlexLog) SetLevel(level int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.level = level
}

// DebugWithFormat logs a debug message with formatting
func (f *FlexLog) DebugWithFormat(format string, args ...interface{}) {
	if f.level > LevelDebug {
		return
	}

	// Check if we should log this based on sampling
	if !f.shouldLog(LevelDebug, format, nil) {
		return
	}

	// Create log message and send to channel
	msg := LogMessage{
		Level:     LevelDebug,
		Format:    format,
		Args:      args,
		Timestamp: time.Now(),
	}

	// Try to send to channel, but don't block if channel is full
	select {
	case f.msgChan <- msg:
		// Message sent successfully
	default:
		// Channel is full, log to stderr
		fmt.Fprintf(os.Stderr, "Warning: message channel full, writing Debug to STDERR directly.\n")
	}
}

// InfoWithFormat logs an info message with formatting
func (f *FlexLog) InfoWithFormat(format string, args ...interface{}) {
	if f.level > LevelInfo {
		return
	}

	// Check if we should log this based on sampling
	if !f.shouldLog(LevelInfo, format, nil) {
		return
	}

	// Create log message and send to channel
	msg := LogMessage{
		Level:     LevelInfo,
		Format:    format,
		Args:      args,
		Timestamp: time.Now(),
	}

	// Try to send to channel, but don't block if channel is full
	select {
	case f.msgChan <- msg:
		// Message sent successfully
	default:
		// Channel is full, log to stderr
		fmt.Fprintf(os.Stderr, "Warning: message channel full, writing Info message to STDERR directly.\n")
	}
}

// WarnWithFormat logs a warning message with formatting
func (f *FlexLog) WarnWithFormat(format string, args ...interface{}) {
	if f.level > LevelWarn {
		return
	}

	// Check if we should log this based on sampling
	if !f.shouldLog(LevelWarn, format, nil) {
		return
	}

	// Create log message and send to channel
	msg := LogMessage{
		Level:     LevelWarn,
		Format:    format,
		Args:      args,
		Timestamp: time.Now(),
	}

	// Try to send to channel, but don't block if channel is full
	select {
	case f.msgChan <- msg:
		// Message sent successfully
	default:
		// Channel is full, log to stderr
		fmt.Fprintf(os.Stderr, "Warning: message channel full, writing Warn message to STDERR directly.\n")
		fmt.Fprintf(os.Stderr, fmt.Sprintf(msg.Format, msg.Timestamp, msg.Level, msg.Args))
	}
}

// ErrorWithFormat logs an error message with formatting
func (f *FlexLog) ErrorWithFormat(format string, args ...interface{}) {
	if f.level > LevelError {
		return
	}

	// Check if we should log this based on sampling
	if !f.shouldLog(LevelError, format, nil) {
		return
	}

	// Create log message and send to channel
	msg := LogMessage{
		Level:     LevelError,
		Format:    format,
		Args:      args,
		Timestamp: time.Now(),
	}

	// Try to send to channel, but don't block if channel is full
	select {
	case f.msgChan <- msg:
		// Message sent successfully
	default:
		// Channel is full, log to stderr and also attempt to log directly
		fmt.Fprintf(os.Stderr, "Warning: message channel full, writing Error message to STDERR directly\n")
		fmt.Fprintf(os.Stderr, fmt.Sprintf(msg.Format, msg.Timestamp, msg.Level, msg.Args))
	}
}

// log delegates to the appropriate level method
func (f *FlexLog) log(level int, message string) {
	switch level {
	case LevelDebug:
		f.Debug(message)
	case LevelInfo:
		f.Info(message)
	case LevelWarn:
		f.Warn(message)
	case LevelError:
		f.Error(message)
	default:
		f.Info(message)
	}
}

func (f *FlexLog) logf(level int, format string, args ...interface{}) {
	// Create log message and send to channelSize
	msg := LogMessage{
		Level:     level,
		Format:    format,
		Args:      args,
		Timestamp: time.Now(),
	}
	select {
	case f.msgChan <- msg:
	// Message sent successfully
	default:
		// Channel is full, log to stderr and also attempt to log directly
		fmt.Fprintf(os.Stderr, fmt.Sprintf("%s Warning: message channel full, writing message to STDERR\n", msg.Timestamp))
		fmt.Fprintf(os.Stderr, fmt.Sprintf(msg.Format, msg.Timestamp, msg.Level, msg.Args))
	}
}

// Debug logs a debug message
func (f *FlexLog) Debug(args ...interface{}) {
	if f.level <= LevelDebug {
		f.logf(LevelDebug, "%s", args...)
	}
}

// Debugf logs a formatted debug message
func (f *FlexLog) Debugf(format string, args ...interface{}) {
	if f.level <= LevelDebug {
		f.logf(LevelDebug, format, args...)
	}
}

// Info logs an info message
func (f *FlexLog) Info(args ...interface{}) {
	if f.level <= LevelInfo {
		f.logf(LevelInfo, "%s", args...)
	}
}

// Infof logs a formatted info message
func (f *FlexLog) Infof(format string, args ...interface{}) {
	if f.level <= LevelInfo {
		f.logf(LevelInfo, format, args...)
	}
}

// Warn logs a warning message
func (f *FlexLog) Warn(args ...interface{}) {
	if f.level <= LevelWarn {
		f.logf(LevelWarn, "%s", args...)
	}
}

// Warnf logs a formatted warning message
func (f *FlexLog) Warnf(format string, args ...interface{}) {
	if f.level <= LevelWarn {
		f.logf(LevelWarn, format, args...)
	}
}

// Error logs an error message
func (f *FlexLog) Error(args ...interface{}) {
	if f.level <= LevelError {
		f.logf(LevelError, "%s", fmt.Sprint(args...))
	}
}

// Errorf logs a formatted error message
func (f *FlexLog) Errorf(format string, args ...interface{}) {
	if f.level <= LevelError {
		f.logf(LevelError, format, args...)
	}
}
