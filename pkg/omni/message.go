package omni

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
	
	"github.com/wayneeseguin/omni/pkg/features"
)

// writeToDestination writes binary data to a destination using batch writer if enabled, otherwise direct write.
// This method assumes dest.mu is already locked by the caller.
//
// Parameters:
//   - dest: The destination to write to
//   - data: The binary data to write
//
// Returns:
//   - error: Any error encountered during writing
func (f *Omni) writeToDestination(dest *Destination, data []byte) error {
	// This function is called with dest.mu already locked
	if dest.batchEnabled && dest.batchWriter != nil {
		// TODO: Implement batch writer properly
		// _, err := dest.batchWriter.Write(data)
		// return err
		return fmt.Errorf("batch writing not implemented yet")
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

// writeStringToDestination writes string data to a destination using batch writer if enabled, otherwise direct write.
// This method assumes dest.mu is already locked by the caller. It handles automatic flushing based on
// buffer size and flush intervals.
//
// Parameters:
//   - dest: The destination to write to
//   - data: The string data to write
//
// Returns:
//   - error: Any error encountered during writing
func (f *Omni) writeStringToDestination(dest *Destination, data string) error {
	// This function is called with dest.mu already locked
	if dest.batchEnabled && dest.batchWriter != nil {
		// TODO: Implement batch writer properly
		// _, err := dest.batchWriter.WriteString(data)
		// return err
		return fmt.Errorf("batch writing not implemented yet")
	} else {
		// Use direct write
		if dest.Writer == nil {
			return fmt.Errorf("writer is nil")
		}
		if _, err := dest.Writer.WriteString(data); err != nil {
			return err
		}
		// Always check if we need to flush based on buffer size
		f.checkFlushSize(dest)

		// For now, always flush immediately
		// TODO: Add flush interval support

		// Otherwise flush immediately for non-batched writes
		return dest.Writer.Flush()
	}
}

// processMessage processes a single log message and routes it to the appropriate backend handler.
// It performs defensive checks and handles file-based, syslog, and custom backends.
//
// Parameters:
//   - msg: The log message to process
//   - dest: The destination to send the message to
func (f *Omni) processMessage(msg LogMessage, dest *Destination) error {
	// Defensive check - should never happen in normal operation
	if dest == nil {
		f.logError("process", "", "Attempted to process message for nil destination", nil, ErrorLevelHigh)
		return fmt.Errorf("nil destination")
	}
	
	// Format the message using the configured formatter
	data, err := f.formatMessage(msg)
	if err != nil {
		f.logError("format", dest.URI, "Failed to format message", err, ErrorLevelMedium)
		return err
	}
	
	// Apply redaction if configured
	if f.redactionManager != nil && msg.Entry != nil && msg.Entry.Fields != nil {
		redactedMsg, redactedFields := f.redactionManager.(*features.RedactionManager).RedactMessage(msg.Level, msg.Entry.Message, msg.Entry.Fields)
		msg.Entry.Message = redactedMsg
		msg.Entry.Fields = redactedFields
	}
	
	// Write to backend
	if dest.backend != nil {
		writeStart := time.Now()
		n, err := dest.backend.Write(data)
		writeDuration := time.Since(writeStart)
		
		// Flush to ensure data is written to disk immediately
		if err == nil {
			if flushErr := dest.backend.Flush(); flushErr != nil {
				f.logError("flush", dest.URI, "Failed to flush backend", flushErr, ErrorLevelLow)
			}
		}
		
		if err != nil {
			dest.trackError()
			f.logError("write", dest.URI, "Failed to write to backend", err, ErrorLevelMedium)
			
			// Trigger recovery if configured
			if f.recoveryManager != nil {
				f.RecoverFromError(err, msg, dest)
			}
			return err
		}
		
		// Track metrics
		dest.trackWrite(int64(n), writeDuration)
		f.trackWrite(int64(n), writeDuration)
		
		// Update size for file backends
		dest.mu.Lock()
		dest.Size += int64(n)
		needsRotation := f.maxSize > 0 && dest.Size > f.maxSize
		dest.mu.Unlock()
		
		// Check if rotation needed
		if needsRotation && dest.Backend == BackendFlock {
			if err := f.rotateDestination(dest); err != nil {
				f.logError("rotate", dest.URI, "Failed to rotate log file", err, ErrorLevelMedium)
			}
		}
	} else {
		// Fallback to legacy implementation for backward compatibility
		switch dest.Backend {
		case BackendFlock:
			var entry string
			var entrySize int64
			f.processFileMessage(msg, dest, &entry, &entrySize)
		case BackendSyslog:
			f.processSyslogMessage(msg, dest)
		case BackendPlugin:
			f.processPluginMessage(msg, dest)
		case -1:
			f.processCustomMessage(msg, dest)
		default:
			f.logError("process", dest.URI, fmt.Sprintf("Unknown backend type: %d", dest.Backend), nil, ErrorLevelHigh)
			return fmt.Errorf("unknown backend type: %d", dest.Backend)
		}
	}
	
	return nil
}

// processCustomMessage processes a message for a custom backend (primarily used in testing).
// It formats the message according to the configured format options and writes it to the
// custom writer without file locking.
//
// Parameters:
//   - msg: The log message to process
//   - dest: The destination with custom backend
func (f *Omni) processCustomMessage(msg LogMessage, dest *Destination) {
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
			// Apply redaction to fields (built-in redaction is always active)
			entryToFormat := msg.Entry
			if msg.Entry.Fields != nil {
				// Create a copy to avoid modifying the original
				entryCopy := *msg.Entry
				// Apply recursive redaction to fields
				f.recursiveRedact(entryCopy.Fields)
				entryToFormat = &entryCopy
			}
			
			// Use JSON format
			data, _ := json.Marshal(entryToFormat)
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
		} else if f.redactionManager != nil {
			// Use redaction manager for simple messages too
			redactedMsg, _ := f.redactionManager.(*features.RedactionManager).RedactMessage(msg.Level, message, nil)
			message = redactedMsg
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
			f.logError("write", dest.URI, "Failed to write to custom backend", err, ErrorLevelMedium)
		}
		dest.mu.Unlock()
	}
}

// processFileMessage processes a message for a file backend with Unix file locking.
// It handles log rotation, formatting, redaction, and metrics tracking. The function
// follows the lock ordering hierarchy: f.mu -> dest.mu -> dest.Lock to prevent deadlocks.
//
// Parameters:
//   - msg: The log message to process
//   - dest: The file destination
//   - entryPtr: Pointer to store the formatted entry string
//   - entrySizePtr: Pointer to store the entry size in bytes
func (f *Omni) processFileMessage(msg LogMessage, dest *Destination, entryPtr *string, entrySizePtr *int64) {
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
	if dest.Lock != nil {
		if err := dest.Lock.Lock(); err != nil {
			f.logError("lock", dest.URI, "Failed to acquire file lock", err, ErrorLevelHigh)
			return
		}
		defer dest.Lock.Unlock()
	}

	var entry string
	var entrySize int64

	// Handle different message types
	if msg.Raw != nil {
		// Raw bytes to write
		entrySize = int64(len(msg.Raw))

		// Check if rotation needed (protect access to dest.Size)
		dest.mu.RLock()
		needsRotation := maxSize > 0 && dest.Size+entrySize > maxSize
		dest.mu.RUnlock()
		
		if needsRotation {
			if err := f.rotateDestination(dest); err != nil {
				f.logError("rotate", dest.URI, "Failed to rotate log file", err, ErrorLevelMedium)
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
			f.logError("write", dest.URI, "Failed to write to log file", err, ErrorLevelMedium)
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
			// Apply redaction to fields (built-in redaction is always active)
			entryToFormat := msg.Entry
			if msg.Entry.Fields != nil {
				// Create a copy to avoid modifying the original
				entryCopy := *msg.Entry
				// Apply recursive redaction to fields
				f.recursiveRedact(entryCopy.Fields)
				entryToFormat = &entryCopy
			}
			
			// Process the JSON entry
			data, err := formatJSONEntry(entryToFormat)
			if err != nil {
				f.logError("format", dest.URI, "Failed to format JSON entry", err, ErrorLevelMedium)
				return
			}
			entryData = string(data)
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

		// Check if rotation needed (protect access to dest.Size)
		dest.mu.RLock()
		needsRotation := maxSize > 0 && dest.Size+entrySize > maxSize
		dest.mu.RUnlock()
		
		if needsRotation {
			if err := f.rotateDestination(dest); err != nil {
				f.logError("rotate", dest.URI, "Failed to rotate log file", err, ErrorLevelMedium)
				return
			}
		}

		// Write the entry
		dest.mu.Lock()
		writeStart := time.Now()
		if err := f.writeToDestination(dest, []byte(entryData)); err != nil {
			dest.mu.Unlock()
			dest.trackError()
			f.logError("write", dest.URI, "Failed to write to log file", err, ErrorLevelMedium)
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
		} else if f.redactionManager != nil {
			// Use redaction manager for simple messages too
			redactedMsg, _ := f.redactionManager.(*features.RedactionManager).RedactMessage(msg.Level, message, nil)
			message = redactedMsg
		}

		// Check if we should format as JSON
		if format == FormatJSON {
			// Create a structured entry with metadata
			structEntry := &LogEntry{
				Timestamp: f.formatTimestamp(msg.Timestamp),
				Level:     levelToString(msg.Level),
				Message:   message,
				Fields:    make(map[string]interface{}),
			}

			// Add metadata fields
			addMetadataFields(structEntry, f)

			// Format as JSON
			jsonData, err := formatJSONEntry(structEntry)
			if err == nil {
				entry = string(jsonData)
				entrySize = int64(len(entry))

				// Assign the formatted entry to the entryPtr immediately after formatting
				*entryPtr = entry

				// Check if rotation needed
				if maxSize > 0 && dest.Size+entrySize > maxSize {
					if err := f.rotateDestination(dest); err != nil {
						f.logError("rotate", dest.URI, "Failed to rotate log file", err, ErrorLevelMedium)
						return
					}
				}

				// Write the entry
				dest.mu.Lock()
				writeStart := time.Now()
				if err := f.writeStringToDestination(dest, entry); err != nil {
					dest.mu.Unlock()
					dest.trackError()
					f.logError("write", dest.URI, "Failed to write to log file", err, ErrorLevelMedium)
					return
				}
				writeDuration := time.Since(writeStart)
				dest.Size += entrySize
				dest.mu.Unlock()

				// Track write metrics
				dest.trackWrite(entrySize, writeDuration)
				f.trackWrite(entrySize, writeDuration)
				return
			}
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

		// Check if rotation needed (protect access to dest.Size)
		dest.mu.RLock()
		needsRotation := maxSize > 0 && dest.Size+entrySize > maxSize
		dest.mu.RUnlock()
		
		if needsRotation {
			if err := f.rotateDestination(dest); err != nil {
				f.logError("rotate", dest.URI, "Failed to rotate log file", err, ErrorLevelMedium)
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
			f.logError("write", dest.URI, "Failed to write to log file", err, ErrorLevelMedium)

			// Trigger recovery if configured
			if f.recoveryManager != nil {
				f.RecoverFromError(err, msg, dest)
			}
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

// processSyslogMessage processes a message for a syslog backend.
// It formats messages according to RFC3164/RFC5424 syslog format and maps
// log levels to appropriate syslog priorities.
//
// Parameters:
//   - msg: The log message to process
//   - dest: The syslog destination
func (f *Omni) processSyslogMessage(msg LogMessage, dest *Destination) {
	// Quick check without lock first
	// Check if backend is initialized
	if dest.backend == nil {
		f.logError("syslog", dest.URI, "Syslog connection not initialized", nil, ErrorLevelHigh)
		return
	}

	// Format message for syslog
	var content string

	if msg.Raw != nil {
		// Raw bytes
		content = string(msg.Raw)
	} else if msg.Entry != nil {
		// Apply redaction to fields (built-in redaction is always active)
		entryToFormat := msg.Entry
		if msg.Entry.Fields != nil {
			// Create a copy to avoid modifying the original
			entryCopy := *msg.Entry
			// Apply recursive redaction to fields
			f.recursiveRedact(entryCopy.Fields)
			entryToFormat = &entryCopy
		}
		
		// JSON entry
		jsonData, err := formatJSONEntry(entryToFormat)
		if err != nil {
			f.logError("format", dest.URI, "Failed to format JSON entry for syslog", err, ErrorLevelMedium)
			return
		}
		content = string(jsonData)
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

	// For now, just write the content directly
	// The syslog backend will handle proper formatting
	syslogMsg := content

	// Write to syslog connection
	dest.mu.Lock()
	if err := f.writeStringToDestination(dest, syslogMsg); err != nil {
		dest.mu.Unlock()
		f.logError("write", dest.URI, "Failed to write to syslog", err, ErrorLevelMedium)
		return
	}
	dest.mu.Unlock()
}

// processPluginMessage processes a message for a plugin backend.
// It formats the message based on the configured format options and delegates
// writing to the plugin's backend implementation.
//
// Parameters:
//   - msg: The log message to process
//   - dest: The destination with plugin backend
func (f *Omni) processPluginMessage(msg LogMessage, dest *Destination) {
	// Check if backend is initialized
	if dest.backend == nil {
		f.logError("plugin", dest.URI, "Plugin backend not initialized", nil, ErrorLevelHigh)
		return
	}

	formatOpts := f.GetFormatOptions()
	format := f.GetFormat()

	// Get redactor reference while not holding any locks
	f.mu.Lock()
	redactor := f.redactor
	f.mu.Unlock()

	var entry []byte

	if msg.Raw != nil {
		// Raw bytes
		entry = msg.Raw
	} else if msg.Entry != nil {
		// For structured entries
		if format == FormatJSON {
			// Apply redaction to fields (built-in redaction is always active)
			entryToFormat := msg.Entry
			if msg.Entry.Fields != nil {
				// Create a copy to avoid modifying the original
				entryCopy := *msg.Entry
				// Apply recursive redaction to fields
				f.recursiveRedact(entryCopy.Fields)
				entryToFormat = &entryCopy
			}
			
			// Use JSON format
			data, err := json.Marshal(entryToFormat)
			if err != nil {
				f.logError("format", dest.URI, "Failed to format JSON entry", err, ErrorLevelMedium)
				return
			}
			entry = append(data, '\n')
		} else {
			// Use text format for structured entries
			var textEntry string
			if formatOpts.IncludeTime {
				textEntry = fmt.Sprintf("[%s] ", msg.Entry.Timestamp)
			}
			if formatOpts.IncludeLevel {
				textEntry += fmt.Sprintf("[%s] ", msg.Entry.Level)
			}
			textEntry += msg.Entry.Message
			if len(msg.Entry.Fields) > 0 {
				textEntry += " "
				for k, v := range msg.Entry.Fields {
					textEntry += fmt.Sprintf("%s=%v ", k, v)
				}
			}
			if msg.Entry.StackTrace != "" {
				textEntry += fmt.Sprintf("stack_trace=%s ", msg.Entry.StackTrace)
			}
			textEntry += "\n"
			entry = []byte(textEntry)
		}
	} else {
		// Regular text format
		message := fmt.Sprintf(msg.Format, msg.Args...)

		// Apply redaction if configured
		if redactor != nil {
			message = redactor.Redact(message)
		} else if f.redactionManager != nil {
			// Use redaction manager for simple messages too
			redactedMsg, _ := f.redactionManager.(*features.RedactionManager).RedactMessage(msg.Level, message, nil)
			message = redactedMsg
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

		entry = []byte(sb.String())
	}

	// Write to the plugin backend
	writeStart := time.Now()
	_, err := dest.backend.Write(entry)
	writeDuration := time.Since(writeStart)
	
	if err != nil {
		dest.trackError()
		f.logError("plugin-write", dest.URI, "Failed to write to plugin backend", err, ErrorLevelMedium)
	} else {
		// Track metrics
		entrySize := int64(len(entry))
		dest.trackWrite(entrySize, writeDuration)
		f.trackWrite(entrySize, writeDuration)
	}
}
