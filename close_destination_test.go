package flexlog

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCloseDestination(t *testing.T) {
	dir := t.TempDir()
	logFile1 := filepath.Join(dir, "test1.log")
	logFile2 := filepath.Join(dir, "test2.log")

	// Create logger with primary destination
	logger, err := New(logFile1)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Add a second destination
	err = logger.AddDestination(logFile2)
	if err != nil {
		t.Fatalf("Failed to add second destination: %v", err)
	}

	// Log some messages
	logger.Info("message to both destinations")
	time.Sleep(100 * time.Millisecond)

	// Verify both destinations exist
	dests := logger.ListDestinations()
	if len(dests) != 2 {
		t.Fatalf("Expected 2 destinations, got %d", len(dests))
	}

	// Close the second destination
	err = logger.CloseDestination(logFile2)
	if err != nil {
		t.Errorf("Failed to close destination: %v", err)
	}

	// Verify only one destination remains
	dests = logger.ListDestinations()
	if len(dests) != 1 {
		t.Errorf("Expected 1 destination after close, got %d", len(dests))
	}

	// Try to log again - should only go to first destination
	logger.Info("message to first destination only")
	time.Sleep(100 * time.Millisecond)

	// Try to close non-existent destination
	err = logger.CloseDestination("nonexistent")
	if err == nil {
		t.Error("Expected error when closing non-existent destination")
	}
}

func TestCloseDefaultDestination(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Get the default destination name
	dests := logger.ListDestinations()
	if len(dests) != 1 {
		t.Fatalf("Expected 1 destination, got %d", len(dests))
	}
	defaultName := dests[0]

	// Close the default destination
	err = logger.CloseDestination(defaultName)
	if err != nil {
		t.Errorf("Failed to close default destination: %v", err)
	}

	// Verify no destinations remain
	dests = logger.ListDestinations()
	if len(dests) != 0 {
		t.Errorf("Expected 0 destinations after closing default, got %d", len(dests))
	}

	// Verify defaultDest is nil
	if logger.defaultDest != nil {
		t.Error("Expected defaultDest to be nil after closing")
	}
}

func TestCloseDestinationWhileLogging(t *testing.T) {
	dir := t.TempDir()
	logFile1 := filepath.Join(dir, "test1.log")
	logFile2 := filepath.Join(dir, "test2.log")

	logger, err := New(logFile1)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Add second destination
	err = logger.AddDestination(logFile2)
	if err != nil {
		t.Fatalf("Failed to add second destination: %v", err)
	}

	// Start logging in background
	done := make(chan bool)
	go func() {
		for i := 0; i < 100; i++ {
			logger.Info("concurrent message %d", i)
			time.Sleep(10 * time.Millisecond)
		}
		done <- true
	}()

	// Wait a bit then close second destination
	time.Sleep(200 * time.Millisecond)
	err = logger.CloseDestination(logFile2)
	if err != nil {
		t.Errorf("Failed to close destination during logging: %v", err)
	}

	// Wait for logging to complete
	<-done

	// Verify only one destination remains
	dests := logger.ListDestinations()
	if len(dests) != 1 {
		t.Errorf("Expected 1 destination after concurrent close, got %d", len(dests))
	}
}

func TestCloseSyslogDestination(t *testing.T) {
	// Skip if running on Windows
	if os.Getenv("GOOS") == "windows" {
		t.Skip("Syslog not supported on Windows")
		return
	}

	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Add syslog destination
	err = logger.AddDestination("syslog://localhost")
	if err != nil {
		// If syslog connection fails, skip the test
		t.Skip("Cannot connect to syslog, skipping test")
		return
	}

	// Log a message
	logger.Info("test syslog message")
	time.Sleep(100 * time.Millisecond)

	// Close syslog destination
	err = logger.CloseDestination("syslog://localhost")
	if err != nil {
		t.Errorf("Failed to close syslog destination: %v", err)
	}

	// Verify syslog destination is gone
	dests := logger.ListDestinations()
	for _, dest := range dests {
		if dest == "syslog://localhost" {
			t.Error("Syslog destination still exists after close")
		}
	}
}
