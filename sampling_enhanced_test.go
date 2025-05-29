package flexlog

import (
	"strings"
	"testing"
	"time"
)

func TestLevelBasedSampling(t *testing.T) {
	// Create logger
	tmpDir := t.TempDir()
	logPath := tmpDir + "/test.log"
	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Setup enhanced sampling
	logger.SetupEnhancedSampling()

	// Configure per-level sampling
	config := LevelSamplingConfig{
		TraceRate: 0.1,  // 10% of trace messages
		DebugRate: 0.3,  // 30% of debug messages
		InfoRate:  0.5,  // 50% of info messages
		WarnRate:  0.8,  // 80% of warn messages
		ErrorRate: 1.0,  // 100% of error messages
	}
	logger.SetLevelSampling(config)
	logger.SetLevel(LevelTrace)

	// Log many messages at each level
	numMessages := 100
	for i := 0; i < numMessages; i++ {
		logger.Trace("Trace message", i)
		logger.Debug("Debug message", i)
		logger.Info("Info message", i)
		logger.Warn("Warn message", i)
		logger.Error("Error message", i)
	}

	// Wait for processing
	logger.Sync()

	// Get metrics
	metrics := logger.GetSamplingMetrics()

	// Check that sampling rates are approximately correct
	checkSamplingRate := func(level int, expectedRate float64, levelName string) {
		levelMetrics := metrics.LevelMetrics[level]
		if levelMetrics.Total != uint64(numMessages) {
			t.Errorf("%s: Expected %d total messages, got %d", levelName, numMessages, levelMetrics.Total)
		}

		actualRate := float64(levelMetrics.Sampled) / float64(levelMetrics.Total)
		tolerance := 0.15 // Allow 15% deviation due to randomness

		if actualRate < expectedRate-tolerance || actualRate > expectedRate+tolerance {
			t.Errorf("%s: Expected sampling rate ~%.2f, got %.2f (sampled %d/%d)",
				levelName, expectedRate, actualRate, levelMetrics.Sampled, levelMetrics.Total)
		}
	}

	checkSamplingRate(LevelTrace, 0.1, "Trace")
	checkSamplingRate(LevelDebug, 0.3, "Debug")
	checkSamplingRate(LevelInfo, 0.5, "Info")
	checkSamplingRate(LevelWarn, 0.8, "Warn")
	checkSamplingRate(LevelError, 1.0, "Error")
}

func TestPatternBasedSampling(t *testing.T) {
	// Create logger
	tmpDir := t.TempDir()
	logPath := tmpDir + "/test.log"
	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Setup enhanced sampling
	logger.SetupEnhancedSampling()

	// Add pattern rules
	rules := []PatternSamplingRule{
		{
			Pattern:  "critical",
			Rate:     1.0, // Always log messages containing "critical"
			Override: true,
			Priority: 10,
		},
		{
			Pattern:  "debug",
			Rate:     0.1, // Only log 10% of messages containing "debug"
			Override: true,
			Priority: 5,
		},
		{
			Pattern:  "health",
			Rate:     0.0, // Never log health check messages
			Override: true,
			Priority: 15,
		},
	}

	for _, rule := range rules {
		if err := logger.AddPatternSamplingRule(rule); err != nil {
			t.Fatalf("Failed to add pattern rule: %v", err)
		}
	}

	// Set a global sampling rate that would normally drop messages
	logger.SetSampling(SamplingRandom, 0.0)

	// Log messages with different patterns
	for i := 0; i < 10; i++ {
		logger.Info("This is a critical error", i)
		logger.Info("This is a debug message", i)
		logger.Info("Health check endpoint", i)
		logger.Info("Normal message", i)
	}

	logger.Sync()

	// Read log content
	content := readFile(t, logPath)

	// Check that critical messages are always logged
	criticalCount := strings.Count(content, "critical error")
	if criticalCount != 10 {
		t.Errorf("Expected 10 critical messages, found %d", criticalCount)
	}

	// Check that health messages are never logged
	healthCount := strings.Count(content, "Health check")
	if healthCount != 0 {
		t.Errorf("Expected 0 health messages, found %d", healthCount)
	}

	// Check that normal messages are dropped (global rate is 0)
	normalCount := strings.Count(content, "Normal message")
	if normalCount != 0 {
		t.Errorf("Expected 0 normal messages, found %d", normalCount)
	}

	// Get metrics to verify pattern matches
	metrics := logger.GetSamplingMetrics()
	if metrics.PatternMatches["critical"] != 10 {
		t.Errorf("Expected 10 pattern matches for 'critical', got %d", metrics.PatternMatches["critical"])
	}
	if metrics.PatternMatches["health"] != 10 {
		t.Errorf("Expected 10 pattern matches for 'health', got %d", metrics.PatternMatches["health"])
	}
}

func TestAdaptiveSampling(t *testing.T) {
	// Create adaptive sampler
	config := AdaptiveSamplingConfig{
		TargetRate:       10.0, // Target 10 logs/second
		WindowDuration:   100 * time.Millisecond,
		MinRate:          0.1,
		MaxRate:          1.0,
		AdjustmentFactor: 0.2,
	}

	sampler := NewAdaptiveSampler(config)

	// Simulate high load (more than target rate)
	highLoadSamples := 0
	for i := 0; i < 50; i++ {
		if sampler.ShouldSample() {
			highLoadSamples++
		}
	}

	// Wait for window to reset
	time.Sleep(config.WindowDuration + 10*time.Millisecond)

	// Get rate after high load
	rateAfterHighLoad := sampler.GetCurrentRate()
	if rateAfterHighLoad >= 1.0 {
		t.Errorf("Expected rate to decrease after high load, but got %.2f", rateAfterHighLoad)
	}

	// Simulate low load
	lowLoadSamples := 0
	for i := 0; i < 5; i++ {
		if sampler.ShouldSample() {
			lowLoadSamples++
		}
	}

	// Wait for window to reset
	time.Sleep(config.WindowDuration + 10*time.Millisecond)

	// Get rate after low load
	rateAfterLowLoad := sampler.GetCurrentRate()
	if rateAfterLowLoad <= rateAfterHighLoad {
		t.Errorf("Expected rate to increase after low load (%.2f -> %.2f)", rateAfterHighLoad, rateAfterLowLoad)
	}
}

func TestSamplingMetrics(t *testing.T) {
	// Create logger
	tmpDir := t.TempDir()
	logPath := tmpDir + "/test.log"
	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Setup enhanced sampling
	logger.SetupEnhancedSampling()

	// Set 50% sampling rate
	logger.SetSampling(SamplingRandom, 0.5)

	// Log messages
	totalMessages := 1000
	for i := 0; i < totalMessages; i++ {
		logger.Info("Test message", i)
	}

	logger.Sync()

	// Get metrics
	metrics := logger.GetSamplingMetrics()

	// Verify total count
	if metrics.TotalMessages != uint64(totalMessages) {
		t.Errorf("Expected %d total messages, got %d", totalMessages, metrics.TotalMessages)
	}

	// Verify sampled + dropped = total
	if metrics.SampledMessages+metrics.DroppedMessages != metrics.TotalMessages {
		t.Errorf("Sampled (%d) + Dropped (%d) != Total (%d)",
			metrics.SampledMessages, metrics.DroppedMessages, metrics.TotalMessages)
	}

	// Check that sampling rate is approximately 50%
	actualRate := float64(metrics.SampledMessages) / float64(metrics.TotalMessages)
	if actualRate < 0.45 || actualRate > 0.55 {
		t.Errorf("Expected sampling rate ~0.5, got %.2f", actualRate)
	}

	// Reset metrics
	logger.ResetSamplingMetrics()
	metrics = logger.GetSamplingMetrics()

	// Verify reset
	if metrics.TotalMessages != 0 || metrics.SampledMessages != 0 || metrics.DroppedMessages != 0 {
		t.Error("Metrics were not properly reset")
	}
}

func TestConsistentSamplingWithEnhancement(t *testing.T) {
	// Create two loggers with same sampling configuration
	tmpDir := t.TempDir()
	logger1, err := New(tmpDir + "/test1.log")
	if err != nil {
		t.Fatalf("Failed to create logger1: %v", err)
	}
	defer logger1.Close()

	logger2, err := New(tmpDir + "/test2.log")
	if err != nil {
		t.Fatalf("Failed to create logger2: %v", err)
	}
	defer logger2.Close()

	// Setup consistent sampling with same rate
	logger1.SetupEnhancedSampling()
	logger2.SetupEnhancedSampling()
	
	logger1.SetSampling(SamplingConsistent, 0.3)
	logger2.SetSampling(SamplingConsistent, 0.3)

	// Log same messages in both loggers
	messages := []string{
		"User login event",
		"Database query executed",
		"Cache hit",
		"API request received",
		"Background job started",
		"File uploaded",
		"Email sent",
		"Payment processed",
		"Error occurred",
		"System health check",
	}

	for _, msg := range messages {
		logger1.Info(msg)
		logger2.Info(msg)
	}

	logger1.Sync()
	logger2.Sync()

	// Read both logs
	content1 := readFile(t, tmpDir+"/test1.log")
	content2 := readFile(t, tmpDir+"/test2.log")

	// Verify that both loggers made the same sampling decisions
	for _, msg := range messages {
		inLog1 := strings.Contains(content1, msg)
		inLog2 := strings.Contains(content2, msg)
		
		if inLog1 != inLog2 {
			t.Errorf("Inconsistent sampling for message %q: logger1=%v, logger2=%v", msg, inLog1, inLog2)
		}
	}
}

func TestPatternRulePriority(t *testing.T) {
	// Test that higher priority rules are evaluated first
	rules := []PatternSamplingRule{
		{Pattern: "b", Priority: 1},
		{Pattern: "a", Priority: 3},
		{Pattern: "c", Priority: 2},
	}

	sortPatternRules(rules)

	// Check order
	if rules[0].Pattern != "a" || rules[1].Pattern != "c" || rules[2].Pattern != "b" {
		t.Errorf("Rules not sorted by priority correctly: %v", rules)
	}
}