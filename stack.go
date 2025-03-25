package flexlog

// EnableStackTraces enables stack traces for error logs
func (f *FlexLog) EnableStackTraces(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.includeTrace = enabled
}

// SetStackSize sets the maximum stack trace buffer size
func (f *FlexLog) SetStackSize(size int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stackSize = size
}

// SetCaptureAllStacks enables stack traces for all log levels
func (f *FlexLog) SetCaptureAllStacks(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.captureAll = enabled
}
