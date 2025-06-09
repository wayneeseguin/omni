package omni

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// SetLevel sets the minimum log level for the logger.
// Messages below this level will be filtered out and not logged.
//
// Parameters:
//   - level: The minimum log level (LevelTrace, LevelDebug, LevelInfo, LevelWarn, or LevelError)
//
// Example:
//
//	logger.SetLevel(omni.LevelWarn) // Only log warnings and errors
func (f *Omni) SetLevel(level int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.level = level
}

// GetLevel returns the current minimum log level.
// This method is thread-safe.
//
// Returns:
//   - int: The current log level (LevelTrace, LevelDebug, LevelInfo, LevelWarn, or LevelError)
//
// Example:
//
//	currentLevel := logger.GetLevel()
//	if currentLevel <= omni.LevelDebug {
//	    fmt.Println("Debug logging is enabled")
//	}
func (f *Omni) GetLevel() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.level
}

// TraceWithFormat logs a trace message with formatting.
// This is an internal method that handles level checking, sampling, and channel management.
// For public API, use Trace() or Tracef() instead.
//
// Parameters:
//   - format: Printf-style format string
//   - args: Arguments for the format string
func (f *Omni) TraceWithFormat(format string, args ...interface{}) {
	// Check if logger is closed
	if f.IsClosed() {
		return
	}

	if f.GetLevel() > LevelTrace {
		return
	}

	// Check if we should log this based on sampling
	if !f.shouldLog(LevelTrace, format, nil) {
		return
	}

	// Create log message and send to channel
	msg := LogMessage{
		Level:     LevelTrace,
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
		f.logError("channel", "", "Message channel full, dropping TRACE message", nil, ErrorLevelMedium)
	}
}

// DebugWithFormat logs a debug message with formatting.
// This is an internal method that handles level checking, sampling, and channel management.
// For public API, use Debug() or Debugf() instead.
//
// Parameters:
//   - format: Printf-style format string
//   - args: Arguments for the format string
func (f *Omni) DebugWithFormat(format string, args ...interface{}) {
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

// InfoWithFormat logs an info message with formatting.
// This is an internal method that handles level checking, sampling, and channel management.
// For public API, use Info() or Infof() instead.
//
// Parameters:
//   - format: Printf-style format string
//   - args: Arguments for the format string
func (f *Omni) InfoWithFormat(format string, args ...interface{}) {
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

// WarnWithFormat logs a warning message with formatting.
// This is an internal method that handles level checking, sampling, and channel management.
// For public API, use Warn() or Warnf() instead.
//
// Parameters:
//   - format: Printf-style format string
//   - args: Arguments for the format string
func (f *Omni) WarnWithFormat(format string, args ...interface{}) {
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

// ErrorWithFormat logs an error message with formatting.
// This is an internal method that handles level checking, sampling, and channel management.
// For public API, use Error() or Errorf() instead.
//
// Parameters:
//   - format: Printf-style format string
//   - args: Arguments for the format string
func (f *Omni) ErrorWithFormat(format string, args ...interface{}) {
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
		// Still write to stderr for backward compatibility (but only in non-test mode)
		if !isTestMode() {
			fmt.Fprintf(os.Stderr, "Warning: message channel full, writing Error message to STDERR directly\n")
			fmt.Fprint(os.Stderr, fmt.Sprintf(msg.Format, msg.Timestamp, msg.Level, msg.Args))
		}
	}
}

// log is an internal helper that delegates to the appropriate level method.
// It provides a unified interface for logging at any level.
//
// Parameters:
//   - level: The log level
//   - message: The message to log
func (f *Omni) log(level int, message string) {
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

// logf is an internal helper for formatted logging at any level.
// It handles level checking, filtering, sampling, and channel management.
//
// Parameters:
//   - level: The log level
//   - format: Printf-style format string
//   - args: Arguments for the format string
func (f *Omni) logf(level int, format string, args ...interface{}) {
	if f.GetLevel() > level {
		return
	}

	// Check if we should log this based on filters and sampling
	// Format the message for filter evaluation
	message := fmt.Sprintf(format, args...)
	if !f.shouldLog(level, message, nil) {
		return
	}

	// Create log message
	msg := LogMessage{
		Level:     level,
		Format:    format,
		Args:      args,
		Timestamp: time.Now(),
	}

	// Atomically check if closed and send message under lock
	f.mu.RLock()
	if f.closed {
		f.mu.RUnlock()
		return
	}

	// Try to send to channel, but don't block if channel is full
	select {
	case f.msgChan <- msg:
		// Message sent successfully
		f.mu.RUnlock()
	default:
		f.mu.RUnlock()
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
		// Still write to stderr for backward compatibility (but only in non-test mode)
		if !isTestMode() {
			fmt.Fprintf(os.Stderr, "Warning: message channel full, writing %s message to STDERR directly.\n", strings.Title(strings.ToLower(levelName)))
			fmt.Fprintln(os.Stderr, fmt.Sprintf(format, args...))
		}
	}
}

// Trace logs a message at TRACE level.
// The message is constructed by concatenating the arguments, similar to fmt.Sprint.
// Trace messages are typically used for very detailed diagnostic information.
//
// Parameters:
//   - args: Values to be logged
//
// Example:
//
//	logger.Trace("Entering function with params: ", param1, param2)
//	logger.Trace("Variable state: ", varName, "=", value)
func (f *Omni) Trace(args ...interface{}) {
	if f.GetLevel() <= LevelTrace {
		f.logf(LevelTrace, "%s", args...)
	}
}

// Tracef logs a formatted message at TRACE level.
// The message is constructed using fmt.Sprintf with the provided format string.
// Trace messages are typically used for very detailed diagnostic information.
//
// Parameters:
//   - format: Printf-style format string
//   - args: Arguments for the format string
//
// Example:
//
//	logger.Tracef("Entering function %s with params: %+v", funcName, params)
//	logger.Tracef("Variable %s = %v (type: %T)", varName, value, value)
func (f *Omni) Tracef(format string, args ...interface{}) {
	if f.GetLevel() <= LevelTrace {
		f.logf(LevelTrace, format, args...)
	}
}

// Debug logs a message at DEBUG level.
// The message is constructed by concatenating the arguments, similar to fmt.Sprint.
// Debug messages are typically used for detailed diagnostic information.
//
// Parameters:
//   - args: Values to be logged
//
// Example:
//
//	logger.Debug("Processing user ID: ", userID)
//	logger.Debug("Cache hit ratio: ", hitCount, "/", totalCount)
func (f *Omni) Debug(args ...interface{}) {
	if f.GetLevel() <= LevelDebug {
		f.logf(LevelDebug, "%s", args...)
	}
}

// Debugf logs a formatted message at DEBUG level.
// The message is constructed using fmt.Sprintf with the provided format string.
// Debug messages are typically used for detailed diagnostic information.
//
// Parameters:
//   - format: Printf-style format string
//   - args: Arguments for the format string
//
// Example:
//
//	logger.Debugf("Processing user ID: %d with options: %+v", userID, options)
//	logger.Debugf("Cache hit ratio: %.2f%%", (hitCount/totalCount)*100)
func (f *Omni) Debugf(format string, args ...interface{}) {
	if f.GetLevel() <= LevelDebug {
		f.logf(LevelDebug, format, args...)
	}
}

// Info logs a message at INFO level.
// The message is constructed by concatenating the arguments, similar to fmt.Sprint.
// Info messages are typically used for general informational messages about application flow.
//
// Parameters:
//   - args: Values to be logged
//
// Example:
//
//	logger.Info("Server started on port ", port)
//	logger.Info("Connected to database: ", dbName)
func (f *Omni) Info(args ...interface{}) {
	if f.GetLevel() <= LevelInfo {
		f.logf(LevelInfo, "%s", args...)
	}
}

// Infof logs a formatted message at INFO level.
// The message is constructed using fmt.Sprintf with the provided format string.
// Info messages are typically used for general informational messages about application flow.
//
// Parameters:
//   - format: Printf-style format string
//   - args: Arguments for the format string
//
// Example:
//
//	logger.Infof("Server started on port %d", port)
//	logger.Infof("Connected to database %s with %d connections", dbName, poolSize)
func (f *Omni) Infof(format string, args ...interface{}) {
	if f.GetLevel() <= LevelInfo {
		f.logf(LevelInfo, format, args...)
	}
}

// Warn logs a message at WARN level.
// The message is constructed by concatenating the arguments, similar to fmt.Sprint.
// Warning messages indicate potentially harmful situations that should be investigated.
//
// Parameters:
//   - args: Values to be logged
//
// Example:
//
//	logger.Warn("Connection pool usage at ", percentage, "% capacity")
//	logger.Warn("Deprecated API endpoint used: ", endpoint)
func (f *Omni) Warn(args ...interface{}) {
	if f.GetLevel() <= LevelWarn {
		f.logf(LevelWarn, "%s", args...)
	}
}

// Warnf logs a formatted message at WARN level.
// The message is constructed using fmt.Sprintf with the provided format string.
// Warning messages indicate potentially harmful situations that should be investigated.
//
// Parameters:
//   - format: Printf-style format string
//   - args: Arguments for the format string
//
// Example:
//
//	logger.Warnf("Connection pool usage at %d%% capacity", percentage)
//	logger.Warnf("Request took %dms, exceeding threshold of %dms", elapsed, threshold)
func (f *Omni) Warnf(format string, args ...interface{}) {
	if f.GetLevel() <= LevelWarn {
		f.logf(LevelWarn, format, args...)
	}
}

// Error logs a message at ERROR level.
// The message is constructed by concatenating the arguments, similar to fmt.Sprint.
// Error messages indicate serious problems that require immediate attention.
//
// Parameters:
//   - args: Values to be logged
//
// Example:
//
//	logger.Error("Failed to connect to database: ", err)
//	logger.Error("Panic recovered in handler: ", r)
func (f *Omni) Error(args ...interface{}) {
	if f.GetLevel() <= LevelError {
		f.logf(LevelError, "%s", fmt.Sprint(args...))
	}
}

// Errorf logs a formatted message at ERROR level.
// The message is constructed using fmt.Sprintf with the provided format string.
// Error messages indicate serious problems that require immediate attention.
//
// Parameters:
//   - format: Printf-style format string
//   - args: Arguments for the format string
//
// Example:
//
//	logger.Errorf("Failed to connect to database: %v", err)
//	logger.Errorf("Request failed after %d retries: %s", retries, err.Error())
func (f *Omni) Errorf(format string, args ...interface{}) {
	if f.GetLevel() <= LevelError {
		f.logf(LevelError, format, args...)
	}
}

// GetLogLevel converts a string representation of a log level to its numeric constant.
// It accepts level names in any case (e.g., "debug", "DEBUG", "Debug").
// If the level string is empty or unrecognized, it falls back to a default.
//
// Parameters:
//   - level: The level name ("debug", "info", "warn", "error")
//   - defaultLevel: Optional default level name if level is empty (defaults to "debug")
//
// Returns:
//   - int: The numeric log level constant
//
// Example:
//
//	level := GetLogLevel("INFO")           // Returns LevelInfo
//	level := GetLogLevel("", "warn")       // Returns LevelWarn (using default)
//	level := GetLogLevel("invalid")        // Returns LevelInfo (fallback)
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
