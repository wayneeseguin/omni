package main

import (
	"fmt"
	"time"

	"github.com/wayneeseguin/flocklogger"
)

func main() {
	// Create logger
	logger, err := flocklogger.NewFlockLogger("./logs/formatted.log")
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		return
	}
	defer logger.Close()

	// Example 1: Basic Text Format with Custom Timestamp
	logger.SetFormat(flocklogger.FormatText)
	logger.SetFormatOption(flocklogger.FormatOptionTimestampFormat, "Jan 02 15:04:05")
	logger.Info("This log has a custom timestamp format")

	// Example 2: Text Format with Lowercase Levels
	logger.SetFormatOption(flocklogger.FormatOptionLevelFormat, flocklogger.LevelFormatNameLower)
	logger.Warn("This log has lowercase level format")

	// Example 3: Text Format with Symbol Levels
	logger.SetFormatOption(flocklogger.FormatOptionLevelFormat, flocklogger.LevelFormatSymbol)
	logger.Error("This log has symbol level format")

	// Example 4: Include Source Location
	logger.SetFormatOption(flocklogger.FormatOptionIncludeLocation, true)
	logger.Info("This log includes source file location")

	// Example 5: UTC Timezone
	logger.SetFormatOption(flocklogger.FormatOptionTimeZone, time.UTC)
	logger.Info("This log uses UTC timezone for timestamps")

	// Example 6: Custom Field Separator
	logger.SetFormatOption(flocklogger.FormatOptionFieldSeparator, " | ")
	logger.InfoWithFields("Custom field separator", map[string]interface{}{
		"user":   "admin",
		"action": "login",
	})

	// Example 7: JSON Format with Indentation
	logger.SetFormat(flocklogger.FormatJSON)
	logger.SetFormatOption(flocklogger.FormatOptionIndentJSON, true)
	logger.InfoWithFields("Indented JSON log", map[string]interface{}{
		"user":   "admin",
		"action": "view",
		"page":   "dashboard",
	})

	// Example 8: JSON Format without Level
	logger.SetFormatOption(flocklogger.FormatOptionIncludeLevel, false)
	logger.InfoWithFields("JSON without level field", map[string]interface{}{
		"status": "complete",
	})

	fmt.Println("Check logs/formatted.log for examples of different formatting options")
}
