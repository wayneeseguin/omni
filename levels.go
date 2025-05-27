package flexlog

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
//   - level: The minimum log level (LevelDebug, LevelInfo, LevelWarn, or LevelError)
//
// Example:
//
//	logger.SetLevel(flexlog.LevelWarn) // Only log warnings and errors
func (f *FlexLog) SetLevel(level int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.level = level
}

// GetLevel returns the current minimum log level.
// This method is thread-safe.
//
// Returns:
//   - int: The current log level (LevelDebug, LevelInfo, LevelWarn, or LevelError)
//
// Example:
//
//	currentLevel := logger.GetLevel()
//	if currentLevel <= flexlog.LevelDebug {
//	    fmt.Println("Debug logging is enabled")
//	}
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
func (f *FlexLog) Debug(args ...interface{}) {
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
func (f *FlexLog) Debugf(format string, args ...interface{}) {
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
func (f *FlexLog) Info(args ...interface{}) {
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
func (f *FlexLog) Infof(format string, args ...interface{}) {
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
func (f *FlexLog) Warn(args ...interface{}) {
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
func (f *FlexLog) Warnf(format string, args ...interface{}) {
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
func (f *FlexLog) Error(args ...interface{}) {
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
