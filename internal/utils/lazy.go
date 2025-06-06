package utils

import (
	"fmt"
	"sync"
	"time"
	
	"github.com/wayneeseguin/omni/pkg/types"
)

// LazyMessage represents a message that delays formatting until it's actually needed.
// This improves performance by avoiding unnecessary string formatting when messages
// are filtered out by log level or sampling.
type LazyMessage struct {
	Level     int
	Format    string
	Args      []interface{}
	Timestamp time.Time
	Entry     *types.LogEntry
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
func (lm *LazyMessage) ToLogMessage() types.LogMessage {
	return types.LogMessage{
		Level:     lm.Level,
		Format:    lm.Format,
		Args:      lm.Args,
		Timestamp: lm.Timestamp,
		Entry:     lm.Entry,
		Raw:       lm.Raw,
	}
}

// NOTE: Lazy formatting methods are implemented in pkg/omni/integration.go
