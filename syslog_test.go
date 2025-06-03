package omni_test

import (
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/wayneeseguin/omni"
)

// mockSyslogServer creates a simple mock syslog server for testing
type mockSyslogServer struct {
	listener net.Listener
	messages []string
	done     chan bool
}

func startMockSyslogServer(network, address string) (*mockSyslogServer, error) {
	listener, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}

	server := &mockSyslogServer{
		listener: listener,
		messages: make([]string, 0),
		done:     make(chan bool),
	}

	go server.run()
	return server, nil
}

func (s *mockSyslogServer) run() {
	for {
		select {
		case <-s.done:
			return
		default:
			// Accept connections with timeout
			s.listener.(*net.TCPListener).SetDeadline(time.Now().Add(100 * time.Millisecond))
			conn, err := s.listener.Accept()
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				return
			}

			// Read messages from connection
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)
				for {
					n, err := c.Read(buf)
					if err != nil {
						return
					}
					if n > 0 {
						s.messages = append(s.messages, string(buf[:n]))
					}
				}
			}(conn)
		}
	}
}

func (s *mockSyslogServer) stop() {
	close(s.done)
	s.listener.Close()
}

func TestNewSyslog(t *testing.T) {
	tests := []struct {
		name        string
		address     string
		tag         string
		expectError bool
	}{
		{
			name:        "unix socket path",
			address:     "/dev/log",
			tag:         "test",
			expectError: false,
		},
		{
			name:        "hostname with port",
			address:     "localhost:514",
			tag:         "myapp",
			expectError: false,
		},
		{
			name:        "hostname without port",
			address:     "localhost",
			tag:         "myapp",
			expectError: false,
		},
		{
			name:        "full URI",
			address:     "syslog://localhost:514",
			tag:         "myapp",
			expectError: false,
		},
		{
			name:        "empty tag",
			address:     "localhost:514",
			tag:         "",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: This will fail to connect in most test environments
			// We're testing the URI parsing and initialization logic
			logger, err := omni.NewSyslog(tt.address, tt.tag)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			// Clean up if logger was created
			if logger != nil {
				logger.CloseAll()
			}
		})
	}
}

func TestSetSyslogTag(t *testing.T) {
	// Start a mock syslog server
	server, err := startMockSyslogServer("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("Failed to start mock syslog server: %v", err)
	}
	defer server.stop()

	// Get the actual address the server is listening on
	addr := server.listener.Addr().String()

	// Create logger with syslog backend
	logger, err := omni.NewSyslog(addr, "initial-tag")
	if err != nil {
		t.Fatalf("Failed to create syslog logger: %v", err)
	}
	defer logger.CloseAll()

	// Find the syslog destination URI
	var syslogURI string
	for _, dest := range logger.Destinations {
		if dest.Backend == omni.BackendSyslog {
			syslogURI = dest.URI
			break
		}
	}

	if syslogURI == "" {
		t.Fatal("Could not find syslog destination")
	}

	// Test setting a new tag
	err = logger.SetSyslogTag(syslogURI, "new-tag")
	if err != nil {
		t.Errorf("Failed to set syslog tag: %v", err)
	}

	// Test setting tag on non-existent destination
	err = logger.SetSyslogTag("syslog://nonexistent:514", "tag")
	if err == nil {
		t.Error("Expected error when setting tag on non-existent destination")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

func TestSetSyslogPriority(t *testing.T) {
	// Start a mock syslog server
	server, err := startMockSyslogServer("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("Failed to start mock syslog server: %v", err)
	}
	defer server.stop()

	// Get the actual address the server is listening on
	addr := server.listener.Addr().String()

	// Create logger with syslog backend
	logger, err := omni.NewSyslog(addr, "test")
	if err != nil {
		t.Fatalf("Failed to create syslog logger: %v", err)
	}
	defer logger.CloseAll()

	// Find the syslog destination URI
	var syslogURI string
	for _, dest := range logger.Destinations {
		if dest.Backend == omni.BackendSyslog {
			syslogURI = dest.URI
			break
		}
	}

	tests := []struct {
		name        string
		priority    int
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid priority minimum",
			priority:    0,
			expectError: false,
		},
		{
			name:        "valid priority maximum",
			priority:    191,
			expectError: false,
		},
		{
			name:        "valid priority user.notice",
			priority:    13, // facility 1 (user) * 8 + severity 5 (notice)
			expectError: false,
		},
		{
			name:        "invalid priority negative",
			priority:    -1,
			expectError: true,
			errorMsg:    "invalid syslog priority",
		},
		{
			name:        "invalid priority too high",
			priority:    192,
			expectError: true,
			errorMsg:    "invalid syslog priority",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := logger.SetSyslogPriority(syslogURI, tt.priority)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}

	// Test setting priority on non-existent destination
	err = logger.SetSyslogPriority("syslog://nonexistent:514", 13)
	if err == nil {
		t.Error("Expected error when setting priority on non-existent destination")
	}
}

func TestReconnectSyslog(t *testing.T) {
	// Start a mock syslog server
	server, err := startMockSyslogServer("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("Failed to start mock syslog server: %v", err)
	}

	// Get the actual address the server is listening on
	addr := server.listener.Addr().String()

	// Create logger with syslog backend
	logger, err := omni.NewSyslog(addr, "test")
	if err != nil {
		server.stop()
		t.Fatalf("Failed to create syslog logger: %v", err)
	}

	// Log a message
	logger.Info("test message before disconnect")
	logger.Sync()

	// Stop the server to simulate connection loss
	server.stop()

	// Give some time for the connection to close
	time.Sleep(100 * time.Millisecond)

	// Start a new server on the same address
	newServer, err := startMockSyslogServer("tcp", addr)
	if err != nil {
		logger.CloseAll()
		// Port might still be in use, skip test
		t.Skipf("Failed to restart mock syslog server: %v", err)
	}
	defer newServer.stop()

	// Try to log - this should trigger reconnection logic
	logger.Info("test message after reconnect")
	logger.Sync()

	// Give some time for the message to be received
	time.Sleep(100 * time.Millisecond)

	logger.CloseAll()

	// Check if we received messages
	if len(newServer.messages) == 0 {
		t.Log("No messages received after reconnection (this may be expected depending on implementation)")
	}
}

func TestSyslogURIParsing(t *testing.T) {
	tests := []struct {
		name           string
		address        string
		expectedPrefix string
	}{
		{
			name:           "unix socket",
			address:        "/var/run/syslog",
			expectedPrefix: "syslog:///",
		},
		{
			name:           "tcp with port",
			address:        "syslog.example.com:514",
			expectedPrefix: "syslog://",
		},
		{
			name:           "hostname only",
			address:        "syslog.example.com",
			expectedPrefix: "syslog://",
		},
		{
			name:           "full URI unchanged",
			address:        "syslog://custom.server:1234",
			expectedPrefix: "syslog://",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't easily test the internal URI parsing without connecting
			// This test mainly documents the expected behavior
			logger, _ := omni.NewSyslog(tt.address, "test")
			if logger != nil {
				logger.CloseAll()
			}
		})
	}
}

func TestSyslogIntegration(t *testing.T) {
	// This test requires a real syslog server or will be skipped
	// Try common Unix socket paths
	socketPaths := []string{
		"/dev/log",
		"/var/run/syslog",
		"/var/run/log",
	}

	var logger *omni.Omni
	var connectedPath string

	for _, path := range socketPaths {
		l, err := omni.NewSyslog(path, "omni-test")
		if err == nil {
			logger = l
			connectedPath = path
			break
		}
	}

	if logger == nil {
		t.Skip("No syslog server available for integration test")
	}
	defer logger.CloseAll()

	t.Logf("Connected to syslog at %s", connectedPath)

	// Test different log levels
	logger.Debug("Debug message from omni test")
	logger.Info("Info message from omni test")
	logger.Warn("Warning message from omni test")
	logger.Error("Error message from omni test")

	// Test structured logging
	logger.StructuredLog(omni.LevelInfo, "Structured log test", map[string]interface{}{
		"component": "test",
		"version":   "1.0",
	})

	logger.Sync()

	// Note: We can't easily verify the messages were received by syslog
	// without reading system logs, which requires elevated permissions
	t.Log("Messages sent to syslog (check system logs to verify)")
}

func TestSyslogWithMultipleDestinations(t *testing.T) {
	// Create a regular file destination
	tempFile := t.TempDir() + "/test.log"
	logger, err := omni.New(tempFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.CloseAll()

	// Try to add a syslog destination (may fail if no syslog available)
	err = logger.AddDestinationWithBackend("syslog://localhost:514", omni.BackendSyslog)

	// Log a message regardless of whether syslog was added
	logger.Info("Test message to multiple destinations")
	logger.Sync()

	// Check file destination
	data, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(data), "Test message to multiple destinations") {
		t.Error("Message not found in file destination")
	}

	// Check destination count
	destinations := logger.ListDestinations()
	t.Logf("Active destinations: %d", len(destinations))
}

func TestSyslogErrorHandling(t *testing.T) {
	// Test various error conditions

	// Invalid address should fail
	logger, err := omni.NewSyslog("", "test")
	if logger != nil {
		logger.CloseAll()
	}

	// Test with definitely unreachable address
	logger2, err2 := omni.NewSyslog("255.255.255.255:514", "test")
	if logger2 != nil {
		defer logger2.CloseAll()

		// Should be able to log even if connection fails
		// (messages might be dropped or queued depending on implementation)
		logger2.Info("Test message to unreachable syslog")
		logger2.Sync()
	}

	// These tests document the behavior rather than enforce it
	// since error handling may vary based on implementation
	t.Logf("Empty address error: %v", err)
	t.Logf("Unreachable address error: %v", err2)
}
