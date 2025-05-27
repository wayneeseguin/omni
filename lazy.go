package flexlog

import (
	"fmt"
	"sync"
	"time"
)

// LazyMessage represents a message that delays formatting until it's actually needed
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

// String formats the message lazily
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

// ToLogMessage converts a LazyMessage to a regular LogMessage
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

// EnableLazyFormatting enables lazy formatting for the logger
func (f *FlexLog) EnableLazyFormatting() {
	f.mu.Lock()
	f.lazyFormatting = true
	f.mu.Unlock()
}

// DisableLazyFormatting disables lazy formatting for the logger
func (f *FlexLog) DisableLazyFormatting() {
	f.mu.Lock()
	f.lazyFormatting = false
	f.mu.Unlock()
}

// IsLazyFormattingEnabled returns whether lazy formatting is enabled
func (f *FlexLog) IsLazyFormattingEnabled() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lazyFormatting
}