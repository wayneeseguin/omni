package main

import (
	"fmt"
	"regexp"
	"time"

	"github.com/wayneeseguin/flocklogger"
)

func main() {
	// Create logger
	logger, err := flocklogger.NewFlockLogger("./logs/filtered.log")
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		return
	}
	defer logger.Close()

	// Example 1: Basic Filtering by Regular Expression
	fmt.Println("Example 1: Regex Filtering - only logs containing 'important'")
	logger.SetRegexFilter(regexp.MustCompile(`important`))

	logger.Info("This message won't be logged")
	logger.Info("This important message will be logged")
	logger.Info("Another message that's ignored")

	// Clear filters for next example
	logger.ClearFilters()

	// Example 2: Field Filtering
	fmt.Println("\nExample 2: Field Filtering - only logs with user=admin")
	logger.SetFieldFilter("user", "admin")

	logger.InfoWithFields("User login", map[string]interface{}{"user": "guest"})
	logger.InfoWithFields("Admin login", map[string]interface{}{"user": "admin"})
	logger.InfoWithFields("Another event", map[string]interface{}{"action": "view"})

	// Clear filters for next example
	logger.ClearFilters()

	// Example 3: Excluding Pattern
	fmt.Println("\nExample 3: Exclude Filtering - exclude logs containing 'debug'")
	logger.SetExcludeRegexFilter(regexp.MustCompile(`debug`))

	logger.Info("Normal message will be logged")
	logger.Info("Message with debug info will be excluded")
	logger.Info("Another normal message")

	// Clear filters for next example
	logger.ClearFilters()

	// Example 4: Random Sampling
	fmt.Println("\nExample 4: Random Sampling - log approximately 30% of messages")
	logger.SetSampling(flocklogger.SamplingRandom, 0.3)

	for i := 0; i < 20; i++ {
		logger.Infof("Random sample message %d", i)
	}

	// Example 5: Interval Sampling
	fmt.Println("\nExample 5: Interval Sampling - log every 5th message")
	logger.SetSampling(flocklogger.SamplingInterval, 5)

	for i := 0; i < 20; i++ {
		logger.Infof("Interval sample message %d", i)
	}

	// Example 6: Consistent Sampling
	fmt.Println("\nExample 6: Consistent Sampling - same messages always sampled the same way")
	logger.SetSampling(flocklogger.SamplingConsistent, 0.5)

	// Run twice to show consistency
	for j := 0; j < 2; j++ {
		fmt.Printf("\nRun %d of consistent sampling:\n", j+1)
		for i := 0; i < 5; i++ {
			// These will be consistently sampled across runs
			logger.Infof("User %d logged in", i)
			logger.Infof("Payment processed for order #%d", i)
		}
		time.Sleep(time.Millisecond * 500)
	}

	// Example 7: Combined Filtering and Sampling
	fmt.Println("\nExample 7: Combining filtering and sampling")
	logger.ClearFilters()
	logger.SetRegexFilter(regexp.MustCompile(`error|warning`))
	logger.SetSampling(flocklogger.SamplingInterval, 2)

	for i := 0; i < 10; i++ {
		logger.Info("Normal message")
		logger.Warn("Warning message")
		logger.Error("Error message")
	}

	// Example 8: Custom key function for consistent sampling
	fmt.Println("\nExample 8: Custom key function for consistent sampling")
	logger.SetSampling(flocklogger.SamplingConsistent, 0.5)
	logger.SetSampleKeyFunc(func(level int, message string, fields map[string]interface{}) string {
		// Sample based on user ID, not the whole message
		if fields != nil {
			if userID, ok := fields["user_id"].(string); ok {
				return userID
			}
		}
		return message
	})

	// All logs for the same user will be consistently sampled
	users := []string{"user1", "user2", "user3", "user4", "user5"}
	for _, user := range users {
		for i := 0; i < 3; i++ {
			logger.InfoWithFields(
				fmt.Sprintf("Action %d for user", i),
				map[string]interface{}{"user_id": user, "action": fmt.Sprintf("action%d", i)},
			)
		}
	}

	fmt.Println("\nCheck logs/filtered.log to see which messages were logged")
}
