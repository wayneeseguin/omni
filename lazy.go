package omni

import (
	"fmt"
	"sync"
	"time"
)

// LazyMessage represents a message that delays formatting until it's actually needed.
// This improves performance by avoiding unnecessary string formatting when messages
// are filtered out by log level or sampling.
type LazyMessage struct {
	Level     int
	Format    string
	Args      []interface{}
	Timestamp time.Time
	Entry     *LogEntry
	Raw       []byte

	// Lazy evaluation fields
	formatted     string
	formattedOnce sync.Once
	formatErr     error
}

// String formats the message lazily.
// The formatting is performed only once and cached for subsequent calls.
// This method is thread-safe due to sync.Once.
//
// Returns:
//   - string: The formatted message
func (lm *LazyMessage) String() string {
	lm.formattedOnce.Do(func() {
		if lm.Raw != nil {
			lm.formatted = string(lm.Raw)
		} else if lm.Entry != nil {
			// Entry formatting is handled elsewhere
			lm.formatted = lm.Entry.Message
		} else {
			lm.formatted = fmt.Sprintf(lm.Format, lm.Args...)
		}
	})
	return lm.formatted
}

// ToLogMessage converts a LazyMessage to a regular LogMessage.
// This is used when the message needs to be processed immediately.
//
// Returns:
//   - LogMessage: A regular log message with the same content
func (lm *LazyMessage) ToLogMessage() LogMessage {
	return LogMessage{
		Level:     lm.Level,
		Format:    lm.Format,
		Args:      lm.Args,
		Timestamp: lm.Timestamp,
		Entry:     lm.Entry,
		Raw:       lm.Raw,
	}
}

// EnableLazyFormatting enables lazy formatting for the logger.
// When enabled, message formatting is deferred until the message is actually
// written to a destination. This can significantly improve performance when
// many messages are filtered out by log level or sampling.
//
// Example:
//
//	logger.EnableLazyFormatting()
//	// Debug messages won't be formatted if debug level is disabled
//	logger.Debug("Expensive formatting: %v", expensiveOperation())
func (f *Omni) EnableLazyFormatting() {
	f.mu.Lock()
	f.lazyFormatting = true
	f.mu.Unlock()
}

// DisableLazyFormatting disables lazy formatting for the logger.
// Messages will be formatted immediately when logged.
func (f *Omni) DisableLazyFormatting() {
	f.mu.Lock()
	f.lazyFormatting = false
	f.mu.Unlock()
}

// IsLazyFormattingEnabled returns whether lazy formatting is enabled.
// Use this to check the current lazy formatting state.
//
// Returns:
//   - bool: true if lazy formatting is enabled
func (f *Omni) IsLazyFormattingEnabled() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lazyFormatting
}
