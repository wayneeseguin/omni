package omni

// EnableStackTraces enables or disables stack traces for error logs.
// When enabled, error log entries will include stack trace information.
//
// Parameters:
//   - enabled: true to enable stack traces, false to disable
//
// Example:
//
//	logger.EnableStackTraces(true)   // Enable stack traces for errors
//	logger.EnableStackTraces(false)  // Disable stack traces
func (f *Omni) EnableStackTraces(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.includeTrace = enabled
}

// SetStackSize sets the maximum stack trace buffer size.
// Larger sizes capture more stack frames but use more memory.
//
// Parameters:
//   - size: Buffer size in bytes (default: 4096)
//
// Example:
//
//	logger.SetStackSize(8192)  // Double the default stack buffer size
func (f *Omni) SetStackSize(size int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stackSize = size
}

// SetCaptureAllStacks enables stack traces for all log levels.
// By default, stack traces are only captured for error level logs.
// Enable this to include stack traces in debug, info, and warning logs.
//
// Parameters:
//   - enabled: true to capture stacks for all levels, false for errors only
//
// Example:
//
//	logger.SetCaptureAllStacks(true)  // Include stack traces in all log levels
func (f *Omni) SetCaptureAllStacks(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.captureAll = enabled
}
