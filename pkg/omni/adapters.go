package omni

import (
	"context"
	"fmt"
)

// LoggerAdapter implements the Logger interface and wraps an Omni instance
type LoggerAdapter struct {
	logger *Omni
	fields map[string]interface{}
}

// NewLoggerAdapter creates a new logger adapter
func NewLoggerAdapter(logger *Omni) *LoggerAdapter {
	return &LoggerAdapter{
		logger: logger,
	}
}

// Trace logs a message at trace level
func (a *LoggerAdapter) Trace(args ...interface{}) {
	if a.fields != nil {
		a.logger.TraceWithFields(fmt.Sprint(args...), a.fields)
	} else {
		a.logger.Trace(args...)
	}
}

// Debug logs a message at debug level
func (a *LoggerAdapter) Debug(args ...interface{}) {
	if a.fields != nil {
		a.logger.DebugWithFields(fmt.Sprint(args...), a.fields)
	} else {
		a.logger.Debug(args...)
	}
}

// Info logs a message at info level
func (a *LoggerAdapter) Info(args ...interface{}) {
	if a.fields != nil {
		a.logger.InfoWithFields(fmt.Sprint(args...), a.fields)
	} else {
		a.logger.Info(args...)
	}
}

// Warn logs a message at warn level
func (a *LoggerAdapter) Warn(args ...interface{}) {
	if a.fields != nil {
		a.logger.WarnWithFields(fmt.Sprint(args...), a.fields)
	} else {
		a.logger.Warn(args...)
	}
}

// Error logs a message at error level
func (a *LoggerAdapter) Error(args ...interface{}) {
	if a.fields != nil {
		a.logger.ErrorWithFields(fmt.Sprint(args...), a.fields)
	} else {
		a.logger.Error(args...)
	}
}

// Tracef logs a formatted message at trace level
func (a *LoggerAdapter) Tracef(format string, args ...interface{}) {
	if a.fields != nil {
		a.logger.TraceWithFields(fmt.Sprintf(format, args...), a.fields)
	} else {
		a.logger.Tracef(format, args...)
	}
}

// Debugf logs a formatted message at debug level
func (a *LoggerAdapter) Debugf(format string, args ...interface{}) {
	if a.fields != nil {
		a.logger.DebugWithFields(fmt.Sprintf(format, args...), a.fields)
	} else {
		a.logger.Debugf(format, args...)
	}
}

// Infof logs a formatted message at info level
func (a *LoggerAdapter) Infof(format string, args ...interface{}) {
	if a.fields != nil {
		a.logger.InfoWithFields(fmt.Sprintf(format, args...), a.fields)
	} else {
		a.logger.Infof(format, args...)
	}
}

// Warnf logs a formatted message at warn level
func (a *LoggerAdapter) Warnf(format string, args ...interface{}) {
	if a.fields != nil {
		a.logger.WarnWithFields(fmt.Sprintf(format, args...), a.fields)
	} else {
		a.logger.Warnf(format, args...)
	}
}

// Errorf logs a formatted message at error level
func (a *LoggerAdapter) Errorf(format string, args ...interface{}) {
	if a.fields != nil {
		a.logger.ErrorWithFields(fmt.Sprintf(format, args...), a.fields)
	} else {
		a.logger.Errorf(format, args...)
	}
}

// WithField returns a new logger with an additional field
func (a *LoggerAdapter) WithField(key string, value interface{}) Logger {
	newFields := make(map[string]interface{})
	for k, v := range a.fields {
		newFields[k] = v
	}
	newFields[key] = value

	return &LoggerAdapter{
		logger: a.logger,
		fields: newFields,
	}
}

// WithFields returns a new logger with additional fields
func (a *LoggerAdapter) WithFields(fields map[string]interface{}) Logger {
	newFields := make(map[string]interface{})
	for k, v := range a.fields {
		newFields[k] = v
	}
	for k, v := range fields {
		newFields[k] = v
	}

	return &LoggerAdapter{
		logger: a.logger,
		fields: newFields,
	}
}

// WithError returns a new logger with an error field
func (a *LoggerAdapter) WithError(err error) Logger {
	if err == nil {
		return a
	}
	return a.WithField("error", err.Error())
}

// WithContext returns a new logger with context values
func (a *LoggerAdapter) WithContext(ctx context.Context) Logger {
	// For now, just return self - context handling can be added later
	return a
}

// IsTraceEnabled returns true if trace level is enabled
func (a *LoggerAdapter) IsTraceEnabled() bool {
	return a.logger.IsLevelEnabled(LevelTrace)
}

// IsDebugEnabled returns true if debug level is enabled
func (a *LoggerAdapter) IsDebugEnabled() bool {
	return a.logger.IsLevelEnabled(LevelDebug)
}

// IsInfoEnabled returns true if info level is enabled
func (a *LoggerAdapter) IsInfoEnabled() bool {
	return a.logger.IsLevelEnabled(LevelInfo)
}

// IsWarnEnabled returns true if warn level is enabled
func (a *LoggerAdapter) IsWarnEnabled() bool {
	return a.logger.IsLevelEnabled(LevelWarn)
}

// IsErrorEnabled returns true if error level is enabled
func (a *LoggerAdapter) IsErrorEnabled() bool {
	return a.logger.IsLevelEnabled(LevelError)
}

// SetLevel sets the log level
func (a *LoggerAdapter) SetLevel(level int) {
	a.logger.SetLevel(level)
}

// GetLevel returns the current log level
func (a *LoggerAdapter) GetLevel() int {
	return a.logger.GetLevel()
}

// IsLevelEnabled returns true if the given level is enabled
func (a *LoggerAdapter) IsLevelEnabled(level int) bool {
	return a.logger.IsLevelEnabled(level)
}

// ContextLogger implements the Logger interface with context support
type ContextLogger struct {
	logger *Omni
	ctx    context.Context
}

// Trace logs a message at trace level
func (c *ContextLogger) Trace(args ...interface{}) {
	c.logger.Trace(args...)
}

// Debug logs a message at debug level
func (c *ContextLogger) Debug(args ...interface{}) {
	c.logger.Debug(args...)
}

// Info logs a message at info level
func (c *ContextLogger) Info(args ...interface{}) {
	c.logger.Info(args...)
}

// Warn logs a message at warn level
func (c *ContextLogger) Warn(args ...interface{}) {
	c.logger.Warn(args...)
}

// Error logs a message at error level
func (c *ContextLogger) Error(args ...interface{}) {
	c.logger.Error(args...)
}

// Tracef logs a formatted message at trace level
func (c *ContextLogger) Tracef(format string, args ...interface{}) {
	c.logger.Tracef(format, args...)
}

// Debugf logs a formatted message at debug level
func (c *ContextLogger) Debugf(format string, args ...interface{}) {
	c.logger.Debugf(format, args...)
}

// Infof logs a formatted message at info level
func (c *ContextLogger) Infof(format string, args ...interface{}) {
	c.logger.Infof(format, args...)
}

// Warnf logs a formatted message at warn level
func (c *ContextLogger) Warnf(format string, args ...interface{}) {
	c.logger.Warnf(format, args...)
}

// Errorf logs a formatted message at error level
func (c *ContextLogger) Errorf(format string, args ...interface{}) {
	c.logger.Errorf(format, args...)
}

// WithField returns a new logger with an additional field
func (c *ContextLogger) WithField(key string, value interface{}) Logger {
	return &LoggerAdapter{
		logger: c.logger,
		fields: map[string]interface{}{key: value},
	}
}

// WithFields returns a new logger with additional fields
func (c *ContextLogger) WithFields(fields map[string]interface{}) Logger {
	return &LoggerAdapter{
		logger: c.logger,
		fields: fields,
	}
}

// WithError returns a new logger with an error field
func (c *ContextLogger) WithError(err error) Logger {
	if err == nil {
		return c
	}
	return c.WithField("error", err.Error())
}

// WithContext returns a new logger with context values
func (c *ContextLogger) WithContext(ctx context.Context) Logger {
	return &ContextLogger{
		logger: c.logger,
		ctx:    ctx,
	}
}

// IsTraceEnabled returns true if trace level is enabled
func (c *ContextLogger) IsTraceEnabled() bool {
	return c.logger.IsLevelEnabled(LevelTrace)
}

// IsDebugEnabled returns true if debug level is enabled
func (c *ContextLogger) IsDebugEnabled() bool {
	return c.logger.IsLevelEnabled(LevelDebug)
}

// IsInfoEnabled returns true if info level is enabled
func (c *ContextLogger) IsInfoEnabled() bool {
	return c.logger.IsLevelEnabled(LevelInfo)
}

// IsWarnEnabled returns true if warn level is enabled
func (c *ContextLogger) IsWarnEnabled() bool {
	return c.logger.IsLevelEnabled(LevelWarn)
}

// IsErrorEnabled returns true if error level is enabled
func (c *ContextLogger) IsErrorEnabled() bool {
	return c.logger.IsLevelEnabled(LevelError)
}

// SetLevel sets the log level
func (c *ContextLogger) SetLevel(level int) {
	c.logger.SetLevel(level)
}

// GetLevel returns the current log level
func (c *ContextLogger) GetLevel() int {
	return c.logger.GetLevel()
}

// IsLevelEnabled returns true if the given level is enabled
func (c *ContextLogger) IsLevelEnabled(level int) bool {
	return c.logger.IsLevelEnabled(level)
}
