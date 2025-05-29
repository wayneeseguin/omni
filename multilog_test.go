package flexlog_test

import (
	//"bufio"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	//"fmt"
	//"io"
	//"os"
	//"path/filepath"
	//"strings"
	"sync"

	"github.com/wayneeseguin/flexlog"
	//"testing"
	//"time"
	//"github.com/wayneeseguin/flexlog"
)

// testWriter is a simple io.Writer implementation that captures output for testing
type testWriter struct {
	buffer bytes.Buffer
	mu     sync.Mutex
}

func (tw *testWriter) Write(p []byte) (n int, err error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	return tw.buffer.Write(p)
}

func (tw *testWriter) String() string {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	return tw.buffer.String()
}

func (tw *testWriter) Reset() {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	tw.buffer.Reset()
}

// customDestination creates a destination with a custom writer for testing
func customDestination(uri string, writer io.Writer) *flexlog.Destination {
	return &flexlog.Destination{
		URI:     uri,
		Backend: -1,                                   // Custom backend
		Writer:  bufio.NewWriterSize(writer, 32*1024), // defaultBufferSize is 32KB
		Done:    make(chan struct{}),
		Enabled: true,
		Name:    uri,
	}
}

// TestAddDestination tests adding new destinations to a logger
func TestAddDestination(t *testing.T) {
	// Create temp dir for test
	tempDir := t.TempDir()

	// Create initial logger
	primaryLogPath := filepath.Join(tempDir, "primary.log")
	logger, err := flexlog.New(primaryLogPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Verify initial state
	dests := logger.ListDestinations()
	if len(dests) != 1 {
		t.Errorf("Expected 1 destination, got %d", len(dests))
	}

	// Add a second destination
	secondLogPath := filepath.Join(tempDir, "secondary.log")
	if err := logger.AddDestination(secondLogPath); err != nil {
		t.Fatalf("Failed to add second destination: %v", err)
	}

	// Give time for the destination to be fully registered
	time.Sleep(100 * time.Millisecond)

	// Verify we have two destinations now
	dests = logger.ListDestinations()
	if len(dests) != 2 {
		t.Errorf("Expected 2 destinations, got %d", len(dests))
	}

	// Log a message
	logger.Info("Test message to multiple destinations")

	// Sync to ensure message is written
	if err := logger.Sync(); err != nil {
		t.Errorf("Failed to sync: %v", err)
	}

	// Verify both files have the message
	for _, path := range []string{primaryLogPath, secondLogPath} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("Failed to read file %s: %v", path, err)
			continue
		}

		if !strings.Contains(string(content), "Test message to multiple destinations") {
			t.Errorf("Expected message not found in %s", path)
		}
	}
}

// TestAddDestinationWithBackend tests adding different backend types
func TestAddDestinationWithBackend(t *testing.T) {
	// Create temp dir for test
	tempDir := t.TempDir()

	// Create initial logger
	primaryLogPath := filepath.Join(tempDir, "primary.log")
	logger, err := flexlog.New(primaryLogPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// For syslog, we'll use a mock destination since we can't guarantee
	// syslog availability in all test environments
	// Create custom writer for test
	customWriter := &testWriter{}

	// Add the custom destination to the logger
	customDest := customDestination("custom://test", customWriter)

	// Create a buffered writer for the custom writer
	bufferedWriter := bufio.NewWriterSize(customWriter, 32*1024)

	// Use the public method to add a custom destination
	logger.AddCustomDestination("custom://test", bufferedWriter)

	// Log a message
	logger.Info("Test message to custom destination")

	// Sync to ensure message is written
	if err := logger.Sync(); err != nil {
		t.Errorf("Failed to sync: %v", err)
	}

	// Verify the message went to the custom writer
	customDest.Writer.Flush()
	if !strings.Contains(customWriter.String(), "Test message to custom destination") {
		t.Errorf("Expected message not found in custom destination")
	}
}

// TestRemoveDestination tests removing a destination
func TestRemoveDestination(t *testing.T) {
	// Create temp dir for test
	tempDir := t.TempDir()

	// Create initial logger
	primaryLogPath := filepath.Join(tempDir, "primary.log")
	logger, err := flexlog.New(primaryLogPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Add a second destination
	secondLogPath := filepath.Join(tempDir, "secondary.log")
	if err := logger.AddDestination(secondLogPath); err != nil {
		t.Fatalf("Failed to add second destination: %v", err)
	}

	// Verify we have two destinations
	dests := logger.ListDestinations()
	if len(dests) != 2 {
		t.Errorf("Expected 2 destinations, got %d", len(dests))
	}

	// Remove the second destination
	if err := logger.RemoveDestination(secondLogPath); err != nil {
		t.Fatalf("Failed to remove destination: %v", err)
	}

	// Verify we're back to one destination
	dests = logger.ListDestinations()
	if len(dests) != 1 {
		t.Errorf("Expected 1 destination, got %d", len(dests))
	}
}

// TestEnableDisableDestination tests enabling and disabling destinations
func TestEnableDisableDestination(t *testing.T) {
	// Create temp dir for test
	tempDir := t.TempDir()

	// Create initial logger
	primaryLogPath := filepath.Join(tempDir, "primary.log")
	logger, err := flexlog.New(primaryLogPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Add a second destination
	secondLogPath := filepath.Join(tempDir, "secondary.log")
	if err := logger.AddDestination(secondLogPath); err != nil {
		t.Fatalf("Failed to add second destination: %v", err)
	}

	// Set names for easier reference using the exported methods
	logger.SetDestinationName(0, "primary")
	logger.SetDestinationName(1, "secondary")
	logger.SetDestinationEnabled(0, true)
	logger.SetDestinationEnabled(1, true)

	// Disable the second destination
	if err := logger.DisableDestination("secondary"); err != nil {
		t.Fatalf("Failed to disable destination: %v", err)
	}

	// Log a message
	logger.Info("This should only go to primary")

	// Sync to ensure message is written
	if err := logger.Sync(); err != nil {
		t.Errorf("Failed to sync: %v", err)
	}

	// Check that message is only in primary log
	primaryContent, err := os.ReadFile(primaryLogPath)
	if err != nil {
		t.Fatalf("Failed to read primary log: %v", err)
	}
	if !strings.Contains(string(primaryContent), "This should only go to primary") {
		t.Errorf("Expected message not found in primary log")
	}

	// No need to wait after sync

	// Check if the second file is empty or doesn't contain our message
	secondExists := true
	secondContent, err := os.ReadFile(secondLogPath)
	if err != nil {
		if os.IsNotExist(err) {
			secondExists = false
		} else {
			t.Fatalf("Error checking second log file: %v", err)
		}
	}

	// If the file exists, verify our message is not in it
	if secondExists && strings.Contains(string(secondContent), "This should only go to primary") {
		t.Errorf("Message found in disabled destination")
	}

	// Re-enable the second destination
	if err := logger.EnableDestination("secondary"); err != nil {
		t.Fatalf("Failed to enable destination: %v", err)
	}

	// Log another message
	logger.Info("This should go to both")

	// Sync to ensure message is written
	if err := logger.Sync(); err != nil {
		t.Errorf("Failed to sync: %v", err)
	}

	// Check both files
	primaryContent, _ = os.ReadFile(primaryLogPath)
	secondContent, _ = os.ReadFile(secondLogPath)
	if !strings.Contains(string(primaryContent), "This should go to both") {
		t.Errorf("Expected message not found in primary log")
	}
	if !strings.Contains(string(secondContent), "This should go to both") {
		t.Errorf("Expected message not found in secondary log")
	}
}

// TestListDestinations tests listing all destinations
func TestListDestinations(t *testing.T) {
	// Create temp dir for test
	tempDir := t.TempDir()

	// Create initial logger
	primaryLogPath := filepath.Join(tempDir, "primary.log")
	logger, err := flexlog.New(primaryLogPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Add several destinations
	secondLogPath := filepath.Join(tempDir, "secondary.log")
	thirdLogPath := filepath.Join(tempDir, "third.log")

	if err := logger.AddDestination(secondLogPath); err != nil {
		t.Fatalf("Failed to add second destination: %v", err)
	}
	if err := logger.AddDestination(thirdLogPath); err != nil {
		t.Fatalf("Failed to add third destination: %v", err)
	}

	// Set names for easier reference using the exported methods
	logger.SetDestinationName(0, "primary")
	logger.SetDestinationName(1, "secondary")
	logger.SetDestinationName(2, "third")

	// List destinations
	dests := logger.ListDestinations()

	// Verify we have the correct number
	if len(dests) != 3 {
		t.Errorf("Expected 3 destinations, got %d", len(dests))
	}

	// Verify all names are present
	foundNames := make(map[string]bool)
	for _, dest := range dests {
		foundNames[dest] = true
	}

	expectedNames := []string{"primary", "secondary", "third"}
	for _, name := range expectedNames {
		if !foundNames[name] {
			t.Errorf("Expected to find destination named '%s'", name)
		}
	}
}

// TestFlushAll tests flushing all destination buffers
func TestFlushAll(t *testing.T) {
	// Create temp dir for test
	tempDir := t.TempDir()

	// Create initial logger
	primaryLogPath := filepath.Join(tempDir, "primary.log")
	logger, err := flexlog.New(primaryLogPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Add a second destination
	secondLogPath := filepath.Join(tempDir, "secondary.log")
	if err := logger.AddDestination(secondLogPath); err != nil {
		t.Fatalf("Failed to add second destination: %v", err)
	}

	// Log a message
	logger.Info("Test flush all message")

	// Files might be empty before flush due to buffering
	primaryEmpty := true
	secondEmpty := true

	primaryContent, _ := os.ReadFile(primaryLogPath)
	if len(primaryContent) > 0 {
		primaryEmpty = false
	}

	secondContent, _ := os.ReadFile(secondLogPath)
	if len(secondContent) > 0 {
		secondEmpty = false
	}

	// Sync to ensure messages are written
	if err := logger.Sync(); err != nil {
		t.Fatalf("Failed to sync destinations: %v", err)
	}

	// Now both files should have content
	primaryContent, _ = os.ReadFile(primaryLogPath)
	secondContent, _ = os.ReadFile(secondLogPath)

	if !strings.Contains(string(primaryContent), "Test flush all message") {
		t.Errorf("Expected message not found in primary log after flush")
		if primaryEmpty {
			t.Log("Primary log was initially empty")
		}
	}

	if !strings.Contains(string(secondContent), "Test flush all message") {
		t.Errorf("Expected message not found in secondary log after flush")
		if secondEmpty {
			t.Log("Secondary log was initially empty")
		}
	}
}

// TestCloseAll tests closing all destinations
func TestCloseAll(t *testing.T) {
	// Create temp dir for test
	tempDir := t.TempDir()

	// Create initial logger
	primaryLogPath := filepath.Join(tempDir, "primary.log")
	logger, err := flexlog.New(primaryLogPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Add a second destination
	secondLogPath := filepath.Join(tempDir, "secondary.log")
	if err := logger.AddDestination(secondLogPath); err != nil {
		t.Fatalf("Failed to add second destination: %v", err)
	}

	// Log a message
	logger.Info("Test close all message")

	// Close all destinations
	if err := logger.CloseAll(); err != nil {
		t.Fatalf("Failed to close destinations: %v", err)
	}

	// Verify both files have the message (should have been flushed)
	primaryContent, _ := os.ReadFile(primaryLogPath)
	secondContent, _ := os.ReadFile(secondLogPath)

	if !strings.Contains(string(primaryContent), "Test close all message") {
		t.Errorf("Expected message not found in primary log after close")
	}

	if !strings.Contains(string(secondContent), "Test close all message") {
		t.Errorf("Expected message not found in secondary log after close")
	}

	// Verify that destinations are cleared
	if len(logger.Destinations) != 0 {
		t.Errorf("Expected destinations to be cleared, but got %d", len(logger.Destinations))
	}

	// Trying to write after close should not panic
	logger.Info("This should not panic but may not be logged")
}

// TestWriteToConcurrently tests concurrent writing to multiple destinations
func TestWriteToConcurrently(t *testing.T) {
	// Create temp dir for test
	tempDir := t.TempDir()

	// Create initial logger
	primaryLogPath := filepath.Join(tempDir, "primary.log")
	logger, err := flexlog.New(primaryLogPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Add several destinations
	const numDests = 5
	writers := make([]*testWriter, numDests)

	for i := 0; i < numDests; i++ {
		writers[i] = &testWriter{}
		bufferedWriter := bufio.NewWriterSize(writers[i], 32*1024)
		logger.AddCustomDestination(fmt.Sprintf("test://dest%d", i), bufferedWriter)
	}

	// Log multiple messages concurrently
	const numMessages = 10
	var wg sync.WaitGroup

	for i := 0; i < numMessages; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			logger.Infof("Concurrent message %d", idx)
		}(i)
	}

	// Wait for all logging calls to complete
	wg.Wait()

	// Flush and give workers time to process
	logger.FlushAll()
	time.Sleep(100 * time.Millisecond)

	// Flush all destinations
	logger.FlushAll()

	// Verify each writer received all messages
	for i, writer := range writers {
		output := writer.String()
		for j := 0; j < numMessages; j++ {
			expected := fmt.Sprintf("Concurrent message %d", j)
			if !strings.Contains(output, expected) {
				t.Errorf("Writer %d missing message %d", i, j)
			}
		}
	}
}

// TestSetLogPath tests changing the log file path
func TestSetLogPath(t *testing.T) {
	// Create temp dir for test
	tempDir := t.TempDir()

	// Create initial logger
	initialPath := filepath.Join(tempDir, "initial.log")
	logger, err := flexlog.New(initialPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Log an initial message
	logger.Info("Initial message")
	logger.Sync() // Use Sync instead of FlushAll to ensure message is processed

	// Verify the message was written to the initial file
	initialContent, err := os.ReadFile(initialPath)
	if err != nil {
		t.Fatalf("Failed to read initial log: %v", err)
	}
	if !strings.Contains(string(initialContent), "Initial message") {
		t.Errorf("Expected message not found in initial log")
	}

	// Change the log path without moving the existing file
	newPath := filepath.Join(tempDir, "subdir", "new.log")
	if err := logger.SetLogPath(newPath, false); err != nil {
		t.Fatalf("Failed to change log path: %v", err)
	}

	// Log a new message
	logger.Info("New message")
	logger.Sync() // Use Sync to ensure message is processed

	// Verify both files have the expected content
	newContent, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("Failed to read new log: %v", err)
	}

	// Re-read initial file to ensure it hasn't changed
	initialContent, err = os.ReadFile(initialPath)
	if err != nil {
		t.Fatalf("Failed to re-read initial log: %v", err)
	}
	if !strings.Contains(string(initialContent), "Initial message") {
		t.Errorf("Expected initial message not found in initial log")
	}

	// Initial file should not have the new message
	if strings.Contains(string(initialContent), "New message") {
		t.Errorf("New message found in initial log when it shouldn't be")
	}

	// New file should have the new message
	if !strings.Contains(string(newContent), "New message") {
		t.Errorf("Expected new message not found in new log")
	}

	// New file should not have the initial message
	if strings.Contains(string(newContent), "Initial message") {
		t.Errorf("Initial message found in new log when it shouldn't be")
	}

	// Test with moving the file
	thirdPath := filepath.Join(tempDir, "moved.log")

	// Write another message to the current log
	logger.Info("Another message")
	logger.Sync() // Ensure message is written before moving

	// Change the path with move=true
	if err := logger.SetLogPath(thirdPath, true); err != nil {
		t.Fatalf("Failed to change log path with move: %v", err)
	}

	// Write to the new location
	logger.Info("After move message")
	logger.Sync() // Use Sync to ensure message is processed

	// Check that new file location has both messages
	thirdContent, err := os.ReadFile(thirdPath)
	if err != nil {
		t.Fatalf("Failed to read third log: %v", err)
	}

	if !strings.Contains(string(thirdContent), "Another message") {
		t.Errorf("Expected 'Another message' not found in moved log")
	}

	if !strings.Contains(string(thirdContent), "After move message") {
		t.Errorf("Expected 'After move message' not found in moved log")
	}
}
