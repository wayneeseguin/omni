package flexlog

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// writeToDestination writes data to a destination using batch writer if enabled, otherwise direct write
func (f *FlexLog) writeToDestination(dest *Destination, data []byte) error {
	// This function is called with dest.mu already locked
	if dest.batchEnabled && dest.batchWriter != nil {
		// Use batch writer
		_, err := dest.batchWriter.Write(data)
		return err
	} else {
		// Use direct write
		if dest.Writer == nil {
			return fmt.Errorf("writer is nil")
		}
		if _, err := dest.Writer.Write(data); err != nil {
			return err
		}
		// Flush immediately for non-batched writes
		return dest.Writer.Flush()
	}
}

// writeStringToDestination writes string data to a destination using batch writer if enabled, otherwise direct write
func (f *FlexLog) writeStringToDestination(dest *Destination, data string) error {
	// This function is called with dest.mu already locked
	if dest.batchEnabled && dest.batchWriter != nil {
		// Use batch writer
		_, err := dest.batchWriter.WriteString(data)
		return err
	} else {
		// Use direct write
		if dest.Writer == nil {
			return fmt.Errorf("writer is nil")
		}
		if _, err := dest.Writer.WriteString(data); err != nil {
			return err
		}
		// Flush immediately for non-batched writes
		return dest.Writer.Flush()
	}
}

// processMessage processes a single log message
func (f *FlexLog) processMessage(msg LogMessage, dest *Destination) {
	// Defensive check - should never happen in normal operation
	if dest == nil {
		f.logError("process", "", "Attempted to process message for nil destination", nil, ErrorLevelHigh)
		return
	}
	
	var entry string
	var entrySize int64

	// Handle different backend types
	switch dest.Backend {
	case BackendFlock:
		// Process file-based message
		f.processFileMessage(msg, dest, &entry, &entrySize)

	case BackendSyslog:
		// Syslog backend (no locking needed)
		f.processSyslogMessage(msg, dest)

	case -1:
		// Custom backend (for testing) - treat like a simple writer
		f.processCustomMessage(msg, dest)

	default:
		f.logError("process", dest.Name, fmt.Sprintf("Unknown backend type: %d", dest.Backend), nil, ErrorLevelHigh)
		return
	}
}

// processCustomMessage processes a message for a custom backend (used in testing)
func (f *FlexLog) processCustomMessage(msg LogMessage, dest *Destination) {
	formatOpts := f.GetFormatOptions()
	format := f.GetFormat()

	// Get redactor reference while not holding any locks
	f.mu.Lock()
	redactor := f.redactor
	f.mu.Unlock()

	var entry string

	if msg.Raw != nil {
		// Raw bytes
		entry = string(msg.Raw)
	} else if msg.Entry != nil {
		// For structured entries
		if format == FormatJSON {
			// Use JSON format
			data, _ := json.Marshal(msg.Entry)
			entry = string(data) + "\n"
		} else {
			// Use text format for structured entries
			if formatOpts.IncludeTime {
				entry = fmt.Sprintf("[%s] ", msg.Entry.Timestamp)
			}
			if formatOpts.IncludeLevel {
				entry += fmt.Sprintf("[%s] ", msg.Entry.Level)
			}
			entry += msg.Entry.Message
			if len(msg.Entry.Fields) > 0 {
				entry += " "
				for k, v := range msg.Entry.Fields {
					entry += fmt.Sprintf("%s=%v ", k, v)
				}
			}
			if msg.Entry.StackTrace != "" {
				entry += fmt.Sprintf("stack_trace=%s ", msg.Entry.StackTrace)
			}
			entry += "\n"
		}
	} else {
		// Regular text format
		message := fmt.Sprintf(msg.Format, msg.Args...)

		// Apply redaction if configured
		if redactor != nil {
			message = redactor.Redact(message)
		}

		// Format based on level
		var levelStr string
		switch msg.Level {
		case LevelTrace:
			levelStr = "TRACE"
		case LevelDebug:
			levelStr = "DEBUG"
		case LevelInfo:
			levelStr = "INFO"
		case LevelWarn:
			levelStr = "WARN"
		case LevelError:
			levelStr = "ERROR"
		default:
			levelStr = "LOG"
		}

		// Format level based on format options
		if formatOpts.LevelFormat == LevelFormatNameLower {
			levelStr = strings.ToLower(levelStr)
		} else if formatOpts.LevelFormat == LevelFormatSymbol && len(levelStr) > 0 {
			levelStr = string(levelStr[0])
		}

		// Use string builder for more efficient string construction
		sb := GetStringBuilder()
		defer PutStringBuilder(sb)

		// Pre-calculate approximate size to reduce allocations
		estimatedSize := len(message) + 20 // message + brackets, spaces, newline
		if formatOpts.IncludeTime {
			estimatedSize += len(formatOpts.TimestampFormat) + 3 // timestamp + brackets + space
		}
		if formatOpts.IncludeLevel {
			estimatedSize += len(levelStr) + 3 // level + brackets + space
		}
		sb.Grow(estimatedSize)

		// Format the entry based on options
		if formatOpts.IncludeTime {
			sb.WriteByte('[')
			sb.WriteString(msg.Timestamp.Format(formatOpts.TimestampFormat))
			sb.WriteString("] ")
		}
		if formatOpts.IncludeLevel {
			sb.WriteByte('[')
			sb.WriteString(levelStr)
			sb.WriteString("] ")
		}
		sb.WriteString(message)
		sb.WriteByte('\n')

		entry = sb.String()
	}

	// Write to the custom writer
	if dest.Writer != nil {
		dest.mu.Lock()
		if err := f.writeStringToDestination(dest, entry); err != nil {
			dest.trackError()
			f.logError("write", dest.Name, "Failed to write to custom backend", err, ErrorLevelMedium)
		}
		dest.mu.Unlock()
	}
}

// processFileMessage processes a message for a file backend
func (f *FlexLog) processFileMessage(msg LogMessage, dest *Destination, entryPtr *string, entrySizePtr *int64) {
	// Get all needed values before acquiring any locks to avoid deadlock
	// Following lock ordering hierarchy: f.mu -> dest.mu -> dest.Lock
	formatOpts := f.GetFormatOptions()
	format := f.GetFormat()
	maxSize := f.GetMaxSize()

	// Get redactor reference (quick read with minimal lock time)
	f.mu.RLock()
	redactor := f.redactor
	f.mu.RUnlock()

	// File backend with flock locking
	// Note: We acquire file lock last according to lock hierarchy
	if err := dest.Lock.Lock(); err != nil {
		f.logError("lock", dest.Name, "Failed to acquire file lock", err, ErrorLevelHigh)
		return
	}
	defer dest.Lock.Unlock()

	var entry string
	var entrySize int64

	// Handle different message types
	if msg.Raw != nil {
		// Raw bytes to write
		entrySize = int64(len(msg.Raw))

		// Check if rotation needed
		if dest.Size+entrySize > maxSize {
			if err := f.rotateDestination(dest); err != nil {
				f.logError("rotate", dest.Name, "Failed to rotate log file", err, ErrorLevelMedium)
				return
			}
		}

		// Write the bytes
		// Note: dest.mu is acquired after file lock according to hierarchy
		dest.mu.Lock()
		writeStart := time.Now()
		if err := f.writeToDestination(dest, msg.Raw); err != nil {
			dest.mu.Unlock()
			dest.trackError()
			f.logError("write", dest.Name, "Failed to write to log file", err, ErrorLevelMedium)
			return
		}
		writeDuration := time.Since(writeStart)
		dest.Size += entrySize
		dest.mu.Unlock()

		// Track write metrics
		dest.trackWrite(entrySize, writeDuration)
		f.trackWrite(entrySize, writeDuration)
	} else if msg.Entry != nil {
		// Structured entry
		var entryData string
		if format == FormatJSON {
			// Process the JSON entry
			data, err := formatJSONEntry(msg.Entry)
			if err != nil {
				f.logError("format", dest.Name, "Failed to format JSON entry", err, ErrorLevelMedium)
				return
			}
			entryData = data
		} else {
			// Use text format for structured entries
			if formatOpts.IncludeTime {
				entryData = fmt.Sprintf("[%s] ", msg.Entry.Timestamp)
			}
			if formatOpts.IncludeLevel {
				entryData += fmt.Sprintf("[%s] ", msg.Entry.Level)
			}
			entryData += msg.Entry.Message
			if len(msg.Entry.Fields) > 0 {
				entryData += " "
				for k, v := range msg.Entry.Fields {
					entryData += fmt.Sprintf("%s=%v ", k, v)
				}
			}
			if msg.Entry.StackTrace != "" {
				entryData += fmt.Sprintf("stack_trace=%s ", msg.Entry.StackTrace)
			}
			entryData += "\n"
		}

		entry = entryData
		entrySize = int64(len(entryData))

		// Check if rotation needed
		if dest.Size+entrySize > maxSize {
			if err := f.rotateDestination(dest); err != nil {
				f.logError("rotate", dest.Name, "Failed to rotate log file", err, ErrorLevelMedium)
				return
			}
		}

		// Write the entry
		dest.mu.Lock()
		writeStart := time.Now()
		if err := f.writeToDestination(dest, []byte(entryData)); err != nil {
			dest.mu.Unlock()
			dest.trackError()
			f.logError("write", dest.Name, "Failed to write to log file", err, ErrorLevelMedium)
			return
		}
		writeDuration := time.Since(writeStart)
		dest.Size += entrySize
		dest.mu.Unlock()

		// Track write metrics
		dest.trackWrite(entrySize, writeDuration)
		f.trackWrite(entrySize, writeDuration)
	} else {
		// Regular text format
		message := fmt.Sprintf(msg.Format, msg.Args...)

		// Apply redaction if configured
		if redactor != nil {
			message = redactor.Redact(message)
		}

		// Format based on level
		var levelStr string
		switch msg.Level {
		case LevelTrace:
			levelStr = "TRACE"
		case LevelDebug:
			levelStr = "DEBUG"
		case LevelInfo:
			levelStr = "INFO"
		case LevelWarn:
			levelStr = "WARN"
		case LevelError:
			levelStr = "ERROR"
		default:
			levelStr = "LOG"
		}

		// Format level based on format options
		if formatOpts.LevelFormat == LevelFormatNameLower {
			levelStr = strings.ToLower(levelStr)
		} else if formatOpts.LevelFormat == LevelFormatSymbol && len(levelStr) > 0 {
			// Use just the first letter for symbol format
			levelStr = string(levelStr[0])
		}

		// Use string builder for more efficient string construction
		sb := GetStringBuilder()
		defer PutStringBuilder(sb)

		// Pre-calculate approximate size to reduce allocations
		estimatedSize := len(message) + 20 // message + brackets, spaces, newline
		if formatOpts.IncludeTime {
			estimatedSize += len(formatOpts.TimestampFormat) + 3 // timestamp + brackets + space
		}
		if formatOpts.IncludeLevel {
			estimatedSize += len(levelStr) + 3 // level + brackets + space
		}
		sb.Grow(estimatedSize)

		// Format the entry based on the logger's options
		if formatOpts.IncludeTime {
			sb.WriteByte('[')
			sb.WriteString(msg.Timestamp.Format(formatOpts.TimestampFormat))
			sb.WriteString("] ")
		}
		if formatOpts.IncludeLevel {
			sb.WriteByte('[')
			sb.WriteString(levelStr)
			sb.WriteString("] ")
		}
		sb.WriteString(message)
		sb.WriteByte('\n')

		entry = sb.String()

		// Assign the formatted entry to the entryPtr immediately after formatting
		// This ensures it's available even if we return early due to errors later
		*entryPtr = entry

		entrySize = int64(len(entry))

		// Check if rotation needed
		if dest.Size+entrySize > maxSize {
			if err := f.rotateDestination(dest); err != nil {
				f.logError("rotate", dest.Name, "Failed to rotate log file", err, ErrorLevelMedium)
				// Try to log to the file as well for visibility
				dest.mu.Lock()
				if dest.Writer != nil {
					fmt.Fprintf(dest.Writer, "[%s] ERROR: Failed to rotate log file: %v\n",
						msg.Timestamp.Format(formatOpts.TimestampFormat), err)
					dest.Writer.Flush()
				}
				dest.mu.Unlock()
				return
			}
		}

		// Write the entry
		dest.mu.Lock()
		writeStart := time.Now()
		if err := f.writeStringToDestination(dest, entry); err != nil {
			dest.mu.Unlock()
			dest.trackError()
			f.logError("write", dest.Name, "Failed to write to log file", err, ErrorLevelMedium)
			return
		}
		writeDuration := time.Since(writeStart)
		dest.Size += entrySize
		dest.mu.Unlock()

		// Track write metrics
		dest.trackWrite(entrySize, writeDuration)
		f.trackWrite(entrySize, writeDuration)
	}

	// Always set the return values before returning from the function
	// This ensures the caller gets the proper entry regardless of which path was taken
	*entryPtr = entry
	*entrySizePtr = entrySize
}

// processSyslogMessage processes a message for a syslog backend
func (f *FlexLog) processSyslogMessage(msg LogMessage, dest *Destination) {
	// Quick check without lock first
	if dest.SyslogConn == nil {
		f.logError("syslog", dest.Name, "Syslog connection not initialized", nil, ErrorLevelHigh)
		return
	}

	// Determine syslog priority based on log level
	priority := dest.SyslogConn.priority
	switch msg.Level {
	case LevelTrace:
		// Trace level (7) - same as debug in syslog
		priority = (priority & 0xFFF8) | 7
	case LevelDebug:
		// Debug level (7)
		priority = (priority & 0xFFF8) | 7
	case LevelInfo:
		// Info level (6)
		priority = (priority & 0xFFF8) | 6
	case LevelWarn:
		// Warning level (4)
		priority = (priority & 0xFFF8) | 4
	case LevelError:
		// Error level (3)
		priority = (priority & 0xFFF8) | 3
	}

	// Format message for syslog
	var content string

	if msg.Raw != nil {
		// Raw bytes
		content = string(msg.Raw)
	} else if msg.Entry != nil {
		// JSON entry
		jsonData, err := formatJSONEntry(msg.Entry)
		if err != nil {
			f.logError("format", dest.Name, "Failed to format JSON entry for syslog", err, ErrorLevelMedium)
			return
		}
		content = jsonData
	} else {
		// Regular message
		content = fmt.Sprintf(msg.Format, msg.Args...)
	}

	// Format according to RFC3164 or RFC5424
	// <PRI>TIMESTAMP HOSTNAME TAG: MSG
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "localhost"
	}

	syslogMsg := fmt.Sprintf("<%d>%s %s %s: %s\n",
		priority,
		msg.Timestamp.Format(time.RFC3339),
		hostname,
		dest.SyslogConn.tag,
		content)

	// Write to syslog connection
	dest.mu.Lock()
	if err := f.writeStringToDestination(dest, syslogMsg); err != nil {
		dest.mu.Unlock()
		f.logError("write", dest.Name, "Failed to write to syslog", err, ErrorLevelMedium)
		return
	}
	dest.mu.Unlock()
}
