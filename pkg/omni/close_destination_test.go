package omni

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	testhelpers "github.com/wayneeseguin/omni/internal/testing"
)

func TestRemoveDestination(t *testing.T) {
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
	time.Sleep(10 * time.Millisecond)

	// Verify both destinations exist
	dests := logger.ListDestinations()
	if len(dests) != 2 {
		t.Fatalf("Expected 2 destinations, got %d", len(dests))
	}

	// Close the second destination
	err = logger.RemoveDestination(logFile2)
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
	time.Sleep(10 * time.Millisecond)

	// Try to close non-existent destination
	err = logger.RemoveDestination("nonexistent")
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
	err = logger.RemoveDestination(defaultName)
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

func TestRemoveDestinationWhileLogging(t *testing.T) {
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
		for i := 0; i < 20; i++ {
			logger.Info("concurrent message %d", i)
			time.Sleep(1 * time.Millisecond) // Reduced from 10ms to 1ms
		}
		done <- true
	}()

	// Wait a bit then close second destination
	time.Sleep(10 * time.Millisecond) // Reduced from 200ms to 10ms
	err = logger.RemoveDestination(logFile2)
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

	// Skip if running in unit mode
	testhelpers.SkipIfUnit(t, "Skipping syslog test in unit mode")

	// Skip unless integration tests are explicitly enabled
	if os.Getenv("OMNI_RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping syslog test. Set OMNI_RUN_INTEGRATION_TESTS=true to run")
		return
	}

	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	logger, err := New(logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Determine syslog address
	syslogAddr := "syslog://localhost"
	if addr := os.Getenv("OMNI_SYSLOG_TEST_ADDR"); addr != "" {
		syslogAddr = "syslog://" + addr
	}

	// Add syslog destination
	err = logger.AddDestination(syslogAddr)
	if err != nil {
		// If syslog connection fails, skip the test
		t.Skipf("Cannot connect to syslog at %s, skipping test: %v", syslogAddr, err)
		return
	}

	// Log a message
	logger.Info("test syslog message")
	time.Sleep(10 * time.Millisecond)

	// Close syslog destination
	err = logger.RemoveDestination(syslogAddr)
	if err != nil {
		t.Errorf("Failed to close syslog destination: %v", err)
	}

	// Verify syslog destination is gone
	dests := logger.ListDestinations()
	for _, dest := range dests {
		if dest == syslogAddr {
			t.Error("Syslog destination still exists after close")
		}
	}
}
