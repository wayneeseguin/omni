package flexlog

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// SetLevel sets the minimum log level
func (f *FlexLog) SetLevel(level int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.level = level
}

// GetLevel returns the current log level (thread-safe)
func (f *FlexLog) GetLevel() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.level
}

// DebugWithFormat logs a debug message with formatting
func (f *FlexLog) DebugWithFormat(format string, args ...interface{}) {
	// Check if logger is closed
	if f.IsClosed() {
		return
	}
	
	if f.GetLevel() > LevelDebug {
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
		// Channel is full
		f.trackMessageDropped()
		f.logError("channel", "", "Message channel full, dropping DEBUG message", nil, ErrorLevelMedium)
	}
}

// InfoWithFormat logs an info message with formatting
func (f *FlexLog) InfoWithFormat(format string, args ...interface{}) {
	// Check if logger is closed
	if f.IsClosed() {
		return
	}
	
	if f.GetLevel() > LevelInfo {
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
		// Channel is full
		f.trackMessageDropped()
		f.logError("channel", "", "Message channel full, dropping INFO message", nil, ErrorLevelMedium)
	}
}

// WarnWithFormat logs a warning message with formatting
func (f *FlexLog) WarnWithFormat(format string, args ...interface{}) {
	// Check if logger is closed
	if f.IsClosed() {
		return
	}
	
	if f.GetLevel() > LevelWarn {
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
		// Channel is full
		f.trackMessageDropped()
		f.logError("channel", "", "Message channel full, dropping WARN message", nil, ErrorLevelMedium)
	}
}

// ErrorWithFormat logs an error message with formatting
func (f *FlexLog) ErrorWithFormat(format string, args ...interface{}) {
	// Check if logger is closed
	if f.IsClosed() {
		return
	}
	
	if f.GetLevel() > LevelError {
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
		// Channel is full
		f.trackMessageDropped()
		f.logError("channel", "", "Message channel full, dropping ERROR message", nil, ErrorLevelHigh)
		// Still write to stderr for backward compatibility
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
	// Check if logger is closed
	if f.IsClosed() {
		return
	}
	
	if f.GetLevel() > level {
		return
	}

	// Check if we should log this based on filters and sampling
	// Format the message for filter evaluation
	message := fmt.Sprintf(format, args...)
	if !f.shouldLog(level, message, nil) {
		return
	}

	// Create log message and send to channel
	msg := LogMessage{
		Level:     level,
		Format:    format,
		Args:      args,
		Timestamp: time.Now(),
	}

	// Try to send to channel, but don't block if channel is full
	select {
	case f.msgChan <- msg:
		// Message sent successfully
	default:
		// Channel is full
		f.trackMessageDropped()
		var levelName string
		switch level {
		case LevelDebug:
			levelName = "DEBUG"
		case LevelInfo:
			levelName = "INFO"
		case LevelWarn:
			levelName = "WARN"
		case LevelError:
			levelName = "ERROR"
		default:
			levelName = "UNKNOWN"
		}

		f.logError("channel", "", fmt.Sprintf("Message channel full, dropping %s message", levelName), nil, ErrorLevelHigh)
		// Still write to stderr for backward compatibility
		fmt.Fprintf(os.Stderr, "Warning: message channel full, writing %s message to STDERR directly.\n", strings.Title(strings.ToLower(levelName)))
		fmt.Fprintln(os.Stderr, fmt.Sprintf(format, args...))
	}
}

// Debug logs a debug message
func (f *FlexLog) Debug(args ...interface{}) {
	if f.GetLevel() <= LevelDebug {
		f.logf(LevelDebug, "%s", args...)
	}
}

// Debugf logs a formatted debug message
func (f *FlexLog) Debugf(format string, args ...interface{}) {
	if f.GetLevel() <= LevelDebug {
		f.logf(LevelDebug, format, args...)
	}
}

// Info logs an info message
func (f *FlexLog) Info(args ...interface{}) {
	if f.GetLevel() <= LevelInfo {
		f.logf(LevelInfo, "%s", args...)
	}
}

// Infof logs a formatted info message
func (f *FlexLog) Infof(format string, args ...interface{}) {
	if f.GetLevel() <= LevelInfo {
		f.logf(LevelInfo, format, args...)
	}
}

// Warn logs a warning message
func (f *FlexLog) Warn(args ...interface{}) {
	if f.GetLevel() <= LevelWarn {
		f.logf(LevelWarn, "%s", args...)
	}
}

// Warnf logs a formatted warning message
func (f *FlexLog) Warnf(format string, args ...interface{}) {
	if f.GetLevel() <= LevelWarn {
		f.logf(LevelWarn, format, args...)
	}
}

// Error logs an error message
func (f *FlexLog) Error(args ...interface{}) {
	if f.GetLevel() <= LevelError {
		f.logf(LevelError, "%s", fmt.Sprint(args...))
	}
}

// Errorf logs a formatted error message
func (f *FlexLog) Errorf(format string, args ...interface{}) {
	if f.GetLevel() <= LevelError {
		f.logf(LevelError, format, args...)
	}
}

// Function to look up log level based on string
// how to set defaultLevel as optional parameter argument

func GetLogLevel(level string, defaultLevel ...string) int {
	l := strings.ToLower(level)
	if l == "" {
		if len(defaultLevel) > 0 && defaultLevel[0] != "" {
			l = defaultLevel[0]
		} else {
			l = "debug" // fallback default if not given
		}
	}

	switch l {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}
