// +build integration

package backends_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/wayneeseguin/omni/pkg/omni"
)

func TestSyslogIntegrationWithDocker(t *testing.T) {
	// Skip if Docker syslog not available
	syslogAddr := os.Getenv("OMNI_SYSLOG_TEST_ADDR")
	if syslogAddr == "" {
		t.Skip("OMNI_SYSLOG_TEST_ADDR not set, skipping Docker syslog integration test")
	}

	proto := os.Getenv("OMNI_SYSLOG_TEST_PROTO")
	if proto == "" {
		proto = "tcp"
	}

	// Create logger with Docker syslog backend
	logger, err := omni.NewSyslog(syslogAddr, "omni-docker-test")
	if err != nil {
		t.Fatalf("Failed to create syslog logger: %v", err)
	}
	defer logger.Close()

	// Test various log levels
	testCases := []struct {
		level   int
		message string
	}{
		{omni.LevelDebug, "Docker syslog debug message"},
		{omni.LevelInfo, "Docker syslog info message"},
		{omni.LevelWarn, "Docker syslog warning message"},
		{omni.LevelError, "Docker syslog error message"},
	}

	for _, tc := range testCases {
		switch tc.level {
		case omni.LevelDebug:
			logger.Debug(tc.message)
		case omni.LevelInfo:
			logger.Info(tc.message)
		case omni.LevelWarn:
			logger.Warn(tc.message)
		case omni.LevelError:
			logger.Error(tc.message)
		}
	}

	// Test structured logging
	logger.StructuredLog(omni.LevelInfo, "Docker structured log test", map[string]interface{}{
		"component":   "integration-test",
		"environment": "docker",
		"timestamp":   time.Now().Unix(),
	})

	// Sync to ensure messages are sent
	logger.Sync()

	// Give syslog time to process messages
	time.Sleep(100 * time.Millisecond)

	// Verify logs were written (check log file if mounted)
	logFile := "./test-logs/omni-test.log"
	if _, err := os.Stat(logFile); err == nil {
		data, err := os.ReadFile(logFile)
		if err != nil {
			t.Errorf("Failed to read log file: %v", err)
		} else {
			logContent := string(data)
			for _, tc := range testCases {
				if !strings.Contains(logContent, tc.message) {
					t.Errorf("Expected to find message %q in logs, but didn't", tc.message)
				}
			}
		}
	}
}

func TestSyslogReconnectionWithDocker(t *testing.T) {
	// Skip if Docker syslog not available
	syslogAddr := os.Getenv("OMNI_SYSLOG_TEST_ADDR")
	if syslogAddr == "" {
		t.Skip("OMNI_SYSLOG_TEST_ADDR not set, skipping Docker syslog reconnection test")
	}

	proto := os.Getenv("OMNI_SYSLOG_TEST_PROTO")
	if proto == "" {
		proto = "tcp"
	}

	// Create logger
	logger, err := omni.NewSyslog(syslogAddr, "omni-reconnect-test")
	if err != nil {
		t.Fatalf("Failed to create syslog logger: %v", err)
	}
	defer logger.Close()

	// Log initial message
	logger.Info("Message before simulated disconnect")
	logger.Sync()

	// Note: Actually disconnecting/reconnecting would require Docker container manipulation
	// For now, we just test that logging continues to work
	time.Sleep(100 * time.Millisecond)

	// Log after potential reconnection
	logger.Info("Message after simulated reconnect")
	logger.Sync()
}

func TestSyslogMultipleDestinationsWithDocker(t *testing.T) {
	// Skip if Docker syslog not available
	syslogAddr := os.Getenv("OMNI_SYSLOG_TEST_ADDR")
	if syslogAddr == "" {
		t.Skip("OMNI_SYSLOG_TEST_ADDR not set, skipping Docker syslog multi-destination test")
	}

	// Create logger with file destination
	tempFile := t.TempDir() + "/multi-dest.log"
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Add syslog destination
	proto := os.Getenv("OMNI_SYSLOG_TEST_PROTO")
	if proto == "" {
		proto = "tcp"
	}
	
	err = logger.AddDestinationWithBackend(syslogAddr, omni.BackendSyslog)
	if err != nil {
		t.Fatalf("Failed to add syslog destination: %v", err)
	}

	// Log messages
	messages := []string{
		"Multi-destination test message 1",
		"Multi-destination test message 2",
		"Multi-destination test message 3",
	}

	for _, msg := range messages {
		logger.Info(msg)
	}

	logger.Sync()
	time.Sleep(100 * time.Millisecond)

	// Verify file destination
	data, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	fileContent := string(data)
	for _, msg := range messages {
		if !strings.Contains(fileContent, msg) {
			t.Errorf("Expected to find message %q in file logs, but didn't", msg)
		}
	}

	// Verify we have both destinations
	destinations := logger.ListDestinations()
	if len(destinations) != 2 {
		t.Errorf("Expected 2 destinations, got %d", len(destinations))
	}
}

func TestSyslogPriorityHandlingWithDocker(t *testing.T) {
	// Skip if Docker syslog not available
	syslogAddr := os.Getenv("OMNI_SYSLOG_TEST_ADDR")
	if syslogAddr == "" {
		t.Skip("OMNI_SYSLOG_TEST_ADDR not set, skipping Docker syslog priority test")
	}

	proto := os.Getenv("OMNI_SYSLOG_TEST_PROTO")
	if proto == "" {
		proto = "tcp"
	}

	// Create logger
	logger, err := omni.NewSyslog(syslogAddr, "omni-priority-test")
	if err != nil {
		t.Fatalf("Failed to create syslog logger: %v", err)
	}
	defer logger.Close()

	// Test logging with different messages
	// Note: SetSyslogPriority method is not implemented yet
	messages := []string{
		"Test emergency message",
		"Test notice message",
		"Test local0 emergency",
		"Test local0 debug",
		"Test local0 info",
	}

	for _, msg := range messages {
		logger.Info(msg)
	}

	logger.Sync()
}