package omni

// This file contains formatter integration that will replace inline formatting in message.go

// formatMessageWithFormatter uses the configured formatter to format a message
// If no formatter is configured, it falls back to the existing inline formatting
func (f *Omni) formatMessageWithFormatter(msg LogMessage) ([]byte, error) {
	// Try to use the configured formatter first
	formatter := f.GetFormatter()
	if formatter != nil {
		return formatter.Format(msg)
	}

	// Fallback to inline formatting for backward compatibility
	// This preserves the existing behavior when no formatter is set
	return nil, nil // Signal to use inline formatting
}
