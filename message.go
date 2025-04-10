package flexlog

import (
	"fmt"
	"os"
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

	default:
		fmt.Fprintf(os.Stderr, "Unknown backend type: %d\n", dest.Backend)
		return
	}
}

// processFileMessage processes a message for a file backend
func (f *FlexLog) processFileMessage(msg LogMessage, dest *Destination, entryPtr *string, entrySizePtr *int64) {
	// File backend with flock locking
	if err := dest.Lock.RLock(); err != nil {
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
		if dest.Size+entrySize > f.maxSize {
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
		// JSON format with structured entry
		if f.format == FormatJSON {
			// Process the JSON entry
			entryData, err := formatJSONEntry(msg.Entry)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to format JSON entry: %v\n", err)
				return
			}
			entrySize = int64(len(entryData))

			// Check if rotation needed
			if dest.Size+entrySize > f.maxSize {
				if err := f.rotateDestination(dest); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to rotate log file: %v\n", err)
					return
				}
			}

			// Write the JSON entry
			if _, err := dest.Writer.Write([]byte(entryData)); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to write to log file: %v\n", err)
				return
			}
			if err := dest.Writer.Flush(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to flush log file: %v\n", err)
				return
			}
			dest.Size += entrySize
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

		// Format the entry based on the logger's options
		if f.formatOptions.IncludeLevel && f.formatOptions.IncludeTime {
			entry = fmt.Sprintf("[%s] [%s] %s\n",
				msg.Timestamp.Format(f.formatOptions.TimestampFormat),
				levelStr,
				message)
		} else if f.formatOptions.IncludeTime {
			entry = fmt.Sprintf("[%s] %s\n",
				msg.Timestamp.Format(f.formatOptions.TimestampFormat),
				message)
		} else if f.formatOptions.IncludeLevel {
			entry = fmt.Sprintf("[%s] %s\n", levelStr, message)
		} else {
			entry = fmt.Sprintf("%s\n", message)
		}

		entrySize = int64(len(entry))

		// Check if rotation needed
		if dest.Size+entrySize > f.maxSize {
			if err := f.rotateDestination(dest); err != nil {
				fmt.Fprintf(dest.Writer, "[%s] ERROR: Failed to rotate log file: %v\n",
					msg.Timestamp.Format(f.formatOptions.TimestampFormat), err)
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

	// Set the return values
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
