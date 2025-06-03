package omni_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wayneeseguin/omni"
)

func TestSetSamplingNone(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Set sampling to none (default)
	logger.SetSampling(omni.SamplingNone, 0)

	// All messages should be logged
	for i := 0; i < 10; i++ {
		logger.Info(fmt.Sprintf("message %d", i))
	}
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// All 10 messages should be present
	for i := 0; i < 10; i++ {
		expected := fmt.Sprintf("message %d", i)
		if !strings.Contains(string(content), expected) {
			t.Errorf("Expected message %q to be logged", expected)
		}
	}
}

func TestSetSamplingRandom(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Set random sampling at 50%
	logger.SetSampling(omni.SamplingRandom, 0.5)

	// Log many messages - reduced from 1000 to 200 for faster test
	totalMessages := 200
	for i := 0; i < totalMessages; i++ {
		logger.Info(fmt.Sprintf("random message %d", i))
	}
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// Count logged messages
	loggedCount := 0
	for i := 0; i < totalMessages; i++ {
		if strings.Contains(string(content), fmt.Sprintf("random message %d", i)) {
			loggedCount++
		}
	}

	// Should be roughly 50% (allow 40-60% range for randomness)
	expectedMin := int(float64(totalMessages) * 0.4)
	expectedMax := int(float64(totalMessages) * 0.6)

	if loggedCount < expectedMin || loggedCount > expectedMax {
		t.Errorf("Expected %d-%d messages to be logged, got %d", expectedMin, expectedMax, loggedCount)
	}

	t.Logf("Random sampling at 50%%: logged %d/%d messages (%.1f%%)",
		loggedCount, totalMessages, float64(loggedCount)/float64(totalMessages)*100)
}

func TestSetSamplingRandomBoundaries(t *testing.T) {
	tests := []struct {
		name       string
		rate       float64
		expectAll  bool
		expectNone bool
	}{
		{"rate 0", 0.0, false, true},
		{"rate 1", 1.0, true, false},
		{"rate -0.5 (clamped to 0)", -0.5, false, true},
		{"rate 1.5 (clamped to 1)", 1.5, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempFile := filepath.Join(t.TempDir(), "test.log")
			logger, err := omni.New(tempFile)
			if err != nil {
				t.Fatalf("Failed to create logger: %v", err)
			}
			defer logger.CloseAll()

			logger.SetSampling(omni.SamplingRandom, tt.rate)

			// Log multiple messages
			for i := 0; i < 100; i++ {
				logger.Info(fmt.Sprintf("test %d", i))
			}
			logger.Sync()

			content, err := os.ReadFile(tempFile)
			if err != nil {
				t.Fatalf("Failed to read log file: %v", err)
			}

			hasContent := len(content) > 0

			if tt.expectAll && !hasContent {
				t.Error("Expected all messages to be logged")
			}

			if tt.expectNone && hasContent {
				t.Error("Expected no messages to be logged")
			}
		})
	}
}

func TestSetSamplingConsistent(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Set consistent sampling at 50%
	logger.SetSampling(omni.SamplingConsistent, 0.5)

	// Log the same messages multiple times
	messages := []string{"message A", "message B", "message C", "message D", "message E"}

	// First pass - record which messages are logged
	firstPassLogged := make(map[string]bool)
	for _, msg := range messages {
		logger.Info(msg)
	}
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	for _, msg := range messages {
		firstPassLogged[msg] = strings.Contains(string(content), msg)
	}

	// Clear log file
	os.Truncate(tempFile, 0)

	// Second pass - same messages should have same sampling decision
	for _, msg := range messages {
		logger.Info(msg)
	}
	logger.Sync()

	content, _ = os.ReadFile(tempFile)
	for _, msg := range messages {
		isLogged := strings.Contains(string(content), msg)
		if firstPassLogged[msg] != isLogged {
			t.Errorf("Consistent sampling failed for message %q: first pass=%v, second pass=%v",
				msg, firstPassLogged[msg], isLogged)
		}
	}
}

func TestSetSamplingInterval(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Set interval sampling - log every 3rd message
	logger.SetSampling(omni.SamplingInterval, 3)

	// Log 20 messages
	for i := 1; i <= 20; i++ {
		logger.Info(fmt.Sprintf("interval message %d", i))
	}
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// Should log messages 1, 4, 7, 10, 13, 16, 19
	expectedMessages := []int{1, 4, 7, 10, 13, 16, 19}
	for _, i := range expectedMessages {
		msg := fmt.Sprintf("interval message %d", i)
		if !strings.Contains(string(content), msg) {
			t.Errorf("Expected message %q to be logged", msg)
		}
	}

	// Should not log messages 2, 3, 5, 6, etc.
	notExpectedMessages := []int{2, 3, 5, 6, 8, 9, 11, 12}
	for _, i := range notExpectedMessages {
		msg := fmt.Sprintf("interval message %d", i)
		if strings.Contains(string(content), msg) {
			t.Errorf("Did not expect message %q to be logged", msg)
		}
	}
}

func TestSetSamplingIntervalBoundaries(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Test interval <= 1 (should log everything)
	logger.SetSampling(omni.SamplingInterval, 0.5)

	for i := 0; i < 5; i++ {
		logger.Info(fmt.Sprintf("test %d", i))
	}
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// All messages should be logged when interval <= 1
	for i := 0; i < 5; i++ {
		if !strings.Contains(string(content), fmt.Sprintf("test %d", i)) {
			t.Errorf("Expected all messages to be logged when interval <= 1")
		}
	}
}

func TestSetSampleKeyFunc(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Set custom key function that uses a field value
	logger.SetSampleKeyFunc(func(level int, message string, fields map[string]interface{}) string {
		if fields != nil {
			if userID, ok := fields["user_id"]; ok {
				return fmt.Sprintf("%v", userID)
			}
		}
		return message
	})

	// Set consistent sampling at 50%
	logger.SetSampling(omni.SamplingConsistent, 0.5)

	// Log messages with same user_id - should have same sampling decision
	user1Messages := []string{"login", "action1", "action2", "logout"}
	for _, msg := range user1Messages {
		logger.StructuredLog(omni.LevelInfo, msg, map[string]interface{}{
			"user_id": "user123",
		})
	}
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// Count how many user123 messages were logged
	user1Count := 0
	for _, msg := range user1Messages {
		if strings.Contains(string(content), msg) {
			user1Count++
		}
	}

	// Either all or none should be logged (consistent for same user)
	if user1Count != 0 && user1Count != len(user1Messages) {
		t.Errorf("Expected consistent sampling for same user_id, got %d/%d messages",
			user1Count, len(user1Messages))
	}

	// Clear log
	os.Truncate(tempFile, 0)

	// Log messages with different user_id - might have different sampling decision
	logger.StructuredLog(omni.LevelInfo, "different user", map[string]interface{}{
		"user_id": "user456",
	})
	logger.Sync()

	// This is just to ensure the custom key function works, not to test specific behavior
}

func TestSamplingWithFilters(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Add a filter that only allows ERROR level
	logger.AddFilter(func(level int, message string, fields map[string]interface{}) bool {
		return level >= omni.LevelError
	})

	// Set sampling to 50%
	logger.SetSampling(omni.SamplingRandom, 0.5)

	// Log many messages at different levels
	for i := 0; i < 100; i++ {
		logger.Info(fmt.Sprintf("info %d", i))
		logger.Error(fmt.Sprintf("error %d", i))
	}
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)

	// No info messages should be logged (filtered)
	if strings.Contains(contentStr, "info") {
		t.Error("Info messages should be filtered out")
	}

	// Some error messages should be logged (filtered by sampling)
	errorCount := 0
	for i := 0; i < 100; i++ {
		if strings.Contains(contentStr, fmt.Sprintf("error %d", i)) {
			errorCount++
		}
	}

	// Should have some but not all error messages
	if errorCount == 0 || errorCount == 100 {
		t.Errorf("Expected some error messages due to sampling, got %d/100", errorCount)
	}
}

func TestSamplingLevelCheck(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Set level to WARN
	logger.SetLevel(omni.LevelWarn)

	// Set sampling to log everything
	logger.SetSampling(omni.SamplingNone, 1.0)

	// Log at different levels
	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error")
	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)

	// Level check happens before sampling
	if strings.Contains(contentStr, "debug") {
		t.Error("Debug should be filtered by level")
	}

	if strings.Contains(contentStr, "info") {
		t.Error("Info should be filtered by level")
	}

	if !strings.Contains(contentStr, "warn") {
		t.Error("Warn should be logged")
	}

	if !strings.Contains(contentStr, "error") {
		t.Error("Error should be logged")
	}
}

func TestSamplingCounterReset(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Set interval sampling - every 3rd message
	logger.SetSampling(omni.SamplingInterval, 3)

	// Log some messages
	logger.Info("msg 1") // Should log (1st)
	logger.Info("msg 2") // Should not log
	logger.Info("msg 3") // Should not log
	logger.Info("msg 4") // Should log (4th)

	// Change sampling - this should reset the counter
	logger.SetSampling(omni.SamplingInterval, 2)

	logger.Info("msg 5") // Should log (1st after reset)
	logger.Info("msg 6") // Should not log
	logger.Info("msg 7") // Should log (3rd)

	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)

	// Check expected messages
	expectedLogged := []string{"msg 1", "msg 4", "msg 5", "msg 7"}
	for _, msg := range expectedLogged {
		if !strings.Contains(contentStr, msg) {
			t.Errorf("Expected %q to be logged", msg)
		}
	}

	expectedNotLogged := []string{"msg 2", "msg 3", "msg 6"}
	for _, msg := range expectedNotLogged {
		if strings.Contains(contentStr, msg) {
			t.Errorf("Did not expect %q to be logged", msg)
		}
	}
}

func TestSamplingConcurrency(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test.log")
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Set interval sampling
	logger.SetSampling(omni.SamplingInterval, 5)

	// Log from multiple goroutines
	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func(goroutine int) {
			for j := 0; j < 20; j++ {
				logger.Info(fmt.Sprintf("goroutine %d message %d", goroutine, j))
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 5; i++ {
		<-done
	}

	logger.Sync()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// Count total messages logged
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	loggedCount := 0
	for _, line := range lines {
		if line != "" {
			loggedCount++
		}
	}

	// With interval sampling of 5, we expect roughly 1/5 of 100 messages = 20
	expectedCount := 20
	tolerance := 5 // Allow some variance due to concurrency

	if loggedCount < expectedCount-tolerance || loggedCount > expectedCount+tolerance {
		t.Errorf("Expected approximately %d messages, got %d", expectedCount, loggedCount)
	}
}
