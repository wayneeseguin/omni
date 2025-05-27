package flexlog

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// processMessage processes a single log message
func (f *FlexLog) processMessage(msg LogMessage, dest *Destination) {
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
		fmt.Fprintf(os.Stderr, "Unknown backend type: %d\n", dest.Backend)
		return
	}
}

// processCustomMessage processes a message for a custom backend (used in testing)
func (f *FlexLog) processCustomMessage(msg LogMessage, dest *Destination) {
	formatOpts := f.GetFormatOptions()
	format := f.GetFormat()
	
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
		
		// Format based on level
		var levelStr string
		switch msg.Level {
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
		} else if formatOpts.LevelFormat == LevelFormatSymbol {
			levelStr = string(levelStr[0])
		}
		
		// Format the entry based on options
		if formatOpts.IncludeLevel && formatOpts.IncludeTime {
			entry = fmt.Sprintf("[%s] [%s] %s\n",
				msg.Timestamp.Format(formatOpts.TimestampFormat),
				levelStr,
				message)
		} else if formatOpts.IncludeTime {
			entry = fmt.Sprintf("[%s] %s\n",
				msg.Timestamp.Format(formatOpts.TimestampFormat),
				message)
		} else if formatOpts.IncludeLevel {
			entry = fmt.Sprintf("[%s] %s\n", levelStr, message)
		} else {
			entry = fmt.Sprintf("%s\n", message)
		}
	}

	// Write to the custom writer
	if dest.Writer != nil {
		dest.Writer.WriteString(entry)
		dest.Writer.Flush()
	}
}

// processFileMessage processes a message for a file backend
func (f *FlexLog) processFileMessage(msg LogMessage, dest *Destination, entryPtr *string, entrySizePtr *int64) {
	// Get all needed values before acquiring file lock to avoid deadlock
	formatOpts := f.GetFormatOptions()
	format := f.GetFormat()
	maxSize := f.GetMaxSize()
	
	// File backend with flock locking
	if err := dest.Lock.Lock(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to acquire file lock: %v\n", err)
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
				fmt.Fprintf(os.Stderr, "Failed to rotate log file: %v\n", err)
				return
			}
		}

		// Write the bytes
		if _, err := dest.Writer.Write(msg.Raw); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write to log file: %v\n", err)
			return
		}
		if err := dest.Writer.Flush(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to flush log file: %v\n", err)
			return
		}
		dest.Size += entrySize
	} else if msg.Entry != nil {
		// Structured entry
		var entryData string
		if format == FormatJSON {
			// Process the JSON entry
			data, err := formatJSONEntry(msg.Entry)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to format JSON entry: %v\n", err)
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
		
		entrySize = int64(len(entryData))

		// Check if rotation needed
		if dest.Size+entrySize > maxSize {
			if err := f.rotateDestination(dest); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to rotate log file: %v\n", err)
				return
			}
		}

		// Write the entry
		if _, err := dest.Writer.Write([]byte(entryData)); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write to log file: %v\n", err)
			return
		}
		if err := dest.Writer.Flush(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to flush log file: %v\n", err)
			return
		}
		dest.Size += entrySize
	} else {
		// Regular text format
		message := fmt.Sprintf(msg.Format, msg.Args...)

		// Format based on level
		var levelStr string
		switch msg.Level {
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
		} else if formatOpts.LevelFormat == LevelFormatSymbol {
			// Use just the first letter for symbol format
			levelStr = string(levelStr[0])
		}

		// Format the entry based on the logger's options
		if formatOpts.IncludeLevel && formatOpts.IncludeTime {
			entry = fmt.Sprintf("[%s] [%s] %s\n",
				msg.Timestamp.Format(formatOpts.TimestampFormat),
				levelStr,
				message)
		} else if formatOpts.IncludeTime {
			entry = fmt.Sprintf("[%s] %s\n",
				msg.Timestamp.Format(formatOpts.TimestampFormat),
				message)
		} else if formatOpts.IncludeLevel {
			entry = fmt.Sprintf("[%s] %s\n", levelStr, message)
		} else {
			entry = fmt.Sprintf("%s\n", message)
		}

		// Assign the formatted entry to the entryPtr immediately after formatting
		// This ensures it's available even if we return early due to errors later
		*entryPtr = entry

		entrySize = int64(len(entry))

		// Check if rotation needed
		if dest.Size+entrySize > maxSize {
			if err := f.rotateDestination(dest); err != nil {
				fmt.Fprintf(dest.Writer, "[%s] ERROR: Failed to rotate log file: %v\n",
					msg.Timestamp.Format(formatOpts.TimestampFormat), err)
				dest.Writer.Flush()
				return
			}
		}

		// Write the entry
		if _, err := dest.Writer.WriteString(entry); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write to log file: %v\n", err)
			return
		}
		if err := dest.Writer.Flush(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to flush log file: %v\n", err)
			return
		}
		dest.Size += entrySize
	}

	// Always set the return values before returning from the function
	// This ensures the caller gets the proper entry regardless of which path was taken
	*entryPtr = entry
	*entrySizePtr = entrySize
}

// processSyslogMessage processes a message for a syslog backend
func (f *FlexLog) processSyslogMessage(msg LogMessage, dest *Destination) {
	if dest.SyslogConn == nil {
		fmt.Fprintf(os.Stderr, "Syslog connection not initialized\n")
		return
	}

	// Determine syslog priority based on log level
	priority := dest.SyslogConn.priority
	switch msg.Level {
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
			fmt.Fprintf(os.Stderr, "Failed to format JSON entry for syslog: %v\n", err)
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
	if _, err := dest.Writer.WriteString(syslogMsg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write to syslog: %v\n", err)
		return
	}

	// Flush the writer
	if err := dest.Writer.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to flush syslog writer: %v\n", err)
	}
}
