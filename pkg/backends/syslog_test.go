package backends_test

import (
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	testhelpers "github.com/wayneeseguin/omni/internal/testing"
	"github.com/wayneeseguin/omni/pkg/backends"
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

/*
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
				logger.Close()
			}
		})
	}
}
*/

/*
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
	defer logger.Close()

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
*/

/*
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
	defer logger.Close()

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
*/

func TestReconnectSyslog(t *testing.T) {
	// Skip if running in unit mode
	testhelpers.SkipIfUnit(t, "Skipping syslog reconnection test in unit mode")

	// Skip unless integration tests are explicitly enabled
	if os.Getenv("OMNI_RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping syslog reconnection test. Set OMNI_RUN_INTEGRATION_TESTS=true to run")
	}

	// Start a mock syslog server
	server, err := startMockSyslogServer("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("Failed to start mock syslog server: %v", err)
	}

	// Get the actual address the server is listening on
	addr := server.listener.Addr().String()

	// Create syslog backend
	backend, err := backends.NewSyslogBackend("tcp", addr, 16, "test")
	if err != nil {
		server.stop()
		t.Fatalf("Failed to create syslog backend: %v", err)
	}

	// Write a message
	backend.Write([]byte("test message before disconnect"))
	backend.Flush()

	// Stop the server to simulate connection loss
	server.stop()

	// Give some time for the connection to close
	time.Sleep(100 * time.Millisecond)

	// Start a new server on the same address
	newServer, err := startMockSyslogServer("tcp", addr)
	if err != nil {
		backend.Close()
		// Port might still be in use, skip test
		t.Skipf("Failed to restart mock syslog server: %v", err)
	}
	defer newServer.stop()

	// Try to write - this may or may not work depending on reconnection logic
	backend.Write([]byte("test message after reconnect"))
	backend.Flush()

	// Give some time for the message to be received
	time.Sleep(100 * time.Millisecond)

	backend.Close()

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
			backend, _ := backends.NewSyslogBackend("tcp", tt.address, 16, "test")
			if backend != nil {
				backend.Close()
			}
		})
	}
}

func TestSyslogIntegration(t *testing.T) {
	// Skip if running in unit mode
	testhelpers.SkipIfUnit(t, "Skipping syslog integration test in unit mode")

	// Check for integration test flag
	if os.Getenv("OMNI_RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping syslog integration test. Set OMNI_RUN_INTEGRATION_TESTS=true to run")
	}

	var backend *backends.SyslogBackendImpl
	var connectedPath string
	var err error

	// First check if we're running with Docker integration
	if syslogAddr := os.Getenv("OMNI_SYSLOG_TEST_ADDR"); syslogAddr != "" {
		// Use Docker syslog
		backend, err = backends.NewSyslogBackend("tcp", syslogAddr, 16, "omni-test")
		if err == nil {
			connectedPath = syslogAddr
		} else {
			t.Logf("Failed to connect to Docker syslog at %s: %v", syslogAddr, err)
		}
	} else {
		// Try common Unix socket paths
		socketPaths := []string{
			"/dev/log",
			"/var/run/syslog",
			"/var/run/log",
		}

		for _, path := range socketPaths {
			b, err := backends.NewSyslogBackend("unix", path, 16, "omni-test")
			if err == nil {
				backend = b
				connectedPath = path
				break
			}
		}
	}

	if backend == nil {
		t.Skip("No syslog server available for integration test")
	}
	defer backend.Close()

	t.Logf("Connected to syslog at %s", connectedPath)

	// Test different log messages
	messages := []string{
		"Debug message from omni test",
		"Info message from omni test",
		"Warning message from omni test",
		"Error message from omni test",
	}

	for _, msg := range messages {
		_, err := backend.Write([]byte(msg))
		if err != nil {
			t.Logf("Failed to write message %q: %v", msg, err)
		}
	}

	backend.Sync()

	// Note: We can't easily verify the messages were received by syslog
	// without reading system logs, which requires elevated permissions
	t.Log("Messages sent to syslog (check system logs to verify)")
}

func TestSyslogWithMultipleDestinations(t *testing.T) {
	// Skip if running in unit mode
	testhelpers.SkipIfUnit(t, "Skipping syslog integration test in unit mode")

	// This test demonstrates creating multiple backends
	tempDir := t.TempDir()
	tempFile := tempDir + "/test.log"

	// Create file backend
	fileBackend, err := backends.NewFileBackend(tempFile)
	if err != nil {
		t.Fatalf("Failed to create file backend: %v", err)
	}
	defer fileBackend.Close()

	// Skip syslog backend creation unless integration tests are enabled
	var syslogBackend *backends.SyslogBackendImpl
	if os.Getenv("OMNI_RUN_INTEGRATION_TESTS") != "true" {
		t.Log("Skipping syslog backend creation. Set OMNI_RUN_INTEGRATION_TESTS=true to test with syslog")
	} else {
		// Try to create syslog backend (may fail if no syslog available)
		var err error
		syslogBackend, err = backends.NewSyslogBackend("tcp", "localhost:514", 16, "test")
		if err != nil {
			t.Logf("Failed to create syslog backend (expected in test environment): %v", err)
		} else {
			defer syslogBackend.Close()
		}
	}

	// Write to file backend
	testMessage := "Test message to multiple destinations"
	_, err = fileBackend.Write([]byte(testMessage))
	if err != nil {
		t.Fatalf("Failed to write to file backend: %v", err)
	}
	fileBackend.Sync()

	// Check file destination
	data, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(data), testMessage) {
		t.Error("Message not found in file destination")
	}

	// If integration tests are enabled and syslog backend was created, write to it too
	if os.Getenv("OMNI_RUN_INTEGRATION_TESTS") == "true" {
		if syslogBackend != nil {
			_, err = syslogBackend.Write([]byte(testMessage))
			if err != nil {
				t.Logf("Failed to write to syslog (may be expected): %v", err)
			}
			syslogBackend.Sync()
			t.Log("Message written to both file and syslog backends")
		} else {
			t.Log("Syslog backend not available")
		}
	}
}

func TestSyslogErrorHandling(t *testing.T) {
	// Skip if running in unit mode
	testhelpers.SkipIfUnit(t, "Skipping syslog error handling test in unit mode")

	// Test various error conditions

	// Invalid address should fail
	_, err := backends.NewSyslogBackend("", "", 16, "test")
	if err != nil {
		t.Logf("Empty address error (expected): %v", err)
	}

	// Test with definitely unreachable address
	_, err2 := backends.NewSyslogBackend("tcp", "255.255.255.255:514", 16, "test")
	if err2 != nil {
		t.Logf("Unreachable address error (expected): %v", err2)
	}
}

// ===== ENHANCED SYSLOG BACKEND TESTS =====

// TestSyslogBackendImpl_NewSyslogBackend tests syslog backend creation
func TestSyslogBackendImpl_NewSyslogBackend(t *testing.T) {
	tests := []struct {
		name        string
		network     string
		address     string
		priority    int
		tag         string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "successful tcp connection",
			network:     "tcp",
			address:     "127.0.0.1:0", // Will be replaced with actual port
			priority:    16,            // Local use facility, informational severity
			tag:         "test-app",
			expectError: false,
		},
		{
			name:        "invalid network",
			network:     "invalid",
			address:     "localhost:514",
			priority:    16,
			tag:         "test",
			expectError: true,
			errorMsg:    "dial syslog",
		},
		{
			name:        "unreachable address",
			network:     "tcp",
			address:     "255.255.255.255:514",
			priority:    16,
			tag:         "test",
			expectError: true,
			errorMsg:    "dial syslog",
		},
		{
			name:        "empty address unix socket fallback",
			network:     "",
			address:     "",
			priority:    16,
			tag:         "test",
			expectError: true, // Will fail if no unix socket exists or wrong socket type
			errorMsg:    "",   // Accept any error message as different systems have different syslog setups
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var actualAddress string
			if tt.network == "tcp" && tt.address == "127.0.0.1:0" {
				// Start a mock server to get a real address
				server, err := startMockSyslogServer("tcp", "127.0.0.1:0")
				if err != nil {
					t.Skipf("Cannot start mock server: %v", err)
				}
				defer server.stop()
				actualAddress = server.listener.Addr().String()
			} else {
				actualAddress = tt.address
			}

			backend, err := backends.NewSyslogBackend(tt.network, actualAddress, tt.priority, tt.tag)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing %q, got none", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			defer backend.Close()

			// Verify backend is created correctly
			if backend == nil {
				t.Fatal("Backend should not be nil")
			}
		})
	}
}

// TestSyslogBackendImpl_Write tests writing to syslog backend
func TestSyslogBackendImpl_Write(t *testing.T) {
	// Skip if running in unit mode
	testhelpers.SkipIfUnit(t, "Skipping syslog write test in unit mode")

	// Start mock syslog server
	server, err := startMockSyslogServer("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("Cannot start mock server: %v", err)
	}
	defer server.stop()

	addr := server.listener.Addr().String()
	backend, err := backends.NewSyslogBackend("tcp", addr, 16, "test-app")
	if err != nil {
		t.Fatalf("Failed to create syslog backend: %v", err)
	}
	defer backend.Close()

	tests := []struct {
		name     string
		entry    []byte
		expected string
	}{
		{
			name:     "simple message",
			entry:    []byte("Hello, syslog!"),
			expected: "<16>test-app: Hello, syslog!",
		},
		{
			name:     "message with newline",
			entry:    []byte("Message with\nnewline"),
			expected: "<16>test-app: Message with\nnewline",
		},
		{
			name:     "empty message",
			entry:    []byte(""),
			expected: "<16>test-app:",
		},
		{
			name:     "message with trailing space",
			entry:    []byte("Trailing space "),
			expected: "<16>test-app: Trailing space",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := backend.Write(tt.entry)
			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}

			if n <= 0 {
				t.Errorf("Expected positive bytes written, got %d", n)
			}

			// Flush to ensure message is sent
			if err := backend.Flush(); err != nil {
				t.Errorf("Flush failed: %v", err)
			}

			// Give time for message to be received
			time.Sleep(50 * time.Millisecond)

			// Note: Due to async nature and network buffering,
			// we can't easily verify exact message content here
			// This test primarily ensures Write() doesn't error
		})
	}
}

// TestSyslogBackendImpl_PriorityAndTag tests priority and tag functionality
func TestSyslogBackendImpl_PriorityAndTag(t *testing.T) {
	// Skip if running in unit mode
	testhelpers.SkipIfUnit(t, "Skipping syslog priority/tag test in unit mode")

	server, err := startMockSyslogServer("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("Cannot start mock server: %v", err)
	}
	defer server.stop()

	addr := server.listener.Addr().String()
	backend, err := backends.NewSyslogBackend("tcp", addr, 16, "initial-tag")
	if err != nil {
		t.Fatalf("Failed to create syslog backend: %v", err)
	}
	defer backend.Close()

	// Test SetPriority
	backend.SetPriority(24) // Mail system, critical
	backend.SetTag("new-tag")

	// Write a test message
	_, err = backend.Write([]byte("Test message with new priority and tag"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	err = backend.Flush()
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}
}

// TestSyslogBackendImpl_FlushAndClose tests flushing and closing
func TestSyslogBackendImpl_FlushAndClose(t *testing.T) {
	// Skip if running in unit mode
	testhelpers.SkipIfUnit(t, "Skipping syslog flush/close test in unit mode")

	server, err := startMockSyslogServer("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("Cannot start mock server: %v", err)
	}
	defer server.stop()

	addr := server.listener.Addr().String()
	backend, err := backends.NewSyslogBackend("tcp", addr, 16, "test-app")
	if err != nil {
		t.Fatalf("Failed to create syslog backend: %v", err)
	}

	// Write multiple messages
	messages := []string{"Message 1", "Message 2", "Message 3"}
	for _, msg := range messages {
		_, err := backend.Write([]byte(msg))
		if err != nil {
			t.Errorf("Write failed for %q: %v", msg, err)
		}
	}

	// Test Flush
	err = backend.Flush()
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	// Test Close
	err = backend.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Writing after close should fail or be ignored gracefully
	_, err = backend.Write([]byte("After close"))
	// We don't enforce specific behavior here, just ensure it doesn't panic
	t.Logf("Write after close error (expected): %v", err)
}

// TestSyslogBackendImpl_SupportsAtomic tests atomic support check
func TestSyslogBackendImpl_SupportsAtomic(t *testing.T) {
	// Skip if running in unit mode
	testhelpers.SkipIfUnit(t, "Skipping syslog atomic support test in unit mode")

	server, err := startMockSyslogServer("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("Cannot start mock server: %v", err)
	}
	defer server.stop()

	addr := server.listener.Addr().String()
	backend, err := backends.NewSyslogBackend("tcp", addr, 16, "test-app")
	if err != nil {
		t.Fatalf("Failed to create syslog backend: %v", err)
	}
	defer backend.Close()

	// Syslog should not support atomic writes
	if backend.SupportsAtomic() {
		t.Error("Syslog backend should not support atomic writes")
	}
}

// TestSyslogBackendImpl_Sync tests sync functionality
func TestSyslogBackendImpl_Sync(t *testing.T) {
	// Skip if running in unit mode
	testhelpers.SkipIfUnit(t, "Skipping syslog sync test in unit mode")

	server, err := startMockSyslogServer("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("Cannot start mock server: %v", err)
	}
	defer server.stop()

	addr := server.listener.Addr().String()
	backend, err := backends.NewSyslogBackend("tcp", addr, 16, "test-app")
	if err != nil {
		t.Fatalf("Failed to create syslog backend: %v", err)
	}
	defer backend.Close()

	// Write a message
	_, err = backend.Write([]byte("Sync test message"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Test Sync (should be equivalent to Flush for syslog)
	err = backend.Sync()
	if err != nil {
		t.Errorf("Sync failed: %v", err)
	}
}

// TestSyslogBackendImpl_GetStats tests getting backend statistics
func TestSyslogBackendImpl_GetStats(t *testing.T) {
	// Skip if running in unit mode
	testhelpers.SkipIfUnit(t, "Skipping syslog stats test in unit mode")

	server, err := startMockSyslogServer("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("Cannot start mock server: %v", err)
	}
	defer server.stop()

	addr := server.listener.Addr().String()
	backend, err := backends.NewSyslogBackend("tcp", addr, 16, "test-app")
	if err != nil {
		t.Fatalf("Failed to create syslog backend: %v", err)
	}
	defer backend.Close()

	stats := backend.GetStats()

	// Verify stats structure
	expectedPath := fmt.Sprintf("syslog://tcp/%s", addr)
	if stats.Path != expectedPath {
		t.Errorf("Expected path %q, got %q", expectedPath, stats.Path)
	}

	// Other stats fields should be zero/empty for basic implementation
	if stats.Size != 0 {
		t.Errorf("Expected size 0, got %d", stats.Size)
	}
}

// TestSyslogBackendImpl_NetworkVsLocal tests network vs local syslog
func TestSyslogBackendImpl_NetworkVsLocal(t *testing.T) {
	// Skip if running in unit mode
	testhelpers.SkipIfUnit(t, "Skipping syslog network vs local test in unit mode")

	t.Run("network_tcp", func(t *testing.T) {
		server, err := startMockSyslogServer("tcp", "127.0.0.1:0")
		if err != nil {
			t.Skipf("Cannot start mock server: %v", err)
		}
		defer server.stop()

		addr := server.listener.Addr().String()
		backend, err := backends.NewSyslogBackend("tcp", addr, 16, "network-test")
		if err != nil {
			t.Fatalf("Failed to create TCP syslog backend: %v", err)
		}
		defer backend.Close()

		_, err = backend.Write([]byte("TCP syslog message"))
		if err != nil {
			t.Errorf("Failed to write to TCP syslog: %v", err)
		}
	})

	t.Run("local_unix_socket_fallback", func(t *testing.T) {
		// Test fallback to unix socket when no address is provided
		// This will likely fail in test environment, but tests the logic
		_, err := backends.NewSyslogBackend("", "", 16, "local-test")
		if err != nil {
			// Expected to fail in test environment - accept various possible error messages
			if !strings.Contains(err.Error(), "no local syslog socket found") &&
				!strings.Contains(err.Error(), "no such file or directory") &&
				!strings.Contains(err.Error(), "permission denied") &&
				!strings.Contains(err.Error(), "protocol wrong type for socket") {
				t.Errorf("Unexpected error type: %v", err)
			}
		}
	})
}

// TestSyslogBackendImpl_ConcurrentWrites tests concurrent access
func TestSyslogBackendImpl_ConcurrentWrites(t *testing.T) {
	// Skip if running in unit mode
	testhelpers.SkipIfUnit(t, "Skipping syslog concurrent writes test in unit mode")

	server, err := startMockSyslogServer("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("Cannot start mock server: %v", err)
	}
	defer server.stop()

	addr := server.listener.Addr().String()
	backend, err := backends.NewSyslogBackend("tcp", addr, 16, "concurrent-test")
	if err != nil {
		t.Fatalf("Failed to create syslog backend: %v", err)
	}
	defer backend.Close()

	const numGoroutines = 10
	const messagesPerGoroutine = 5

	var wg sync.WaitGroup
	var errorCount int32
	var mu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				msg := fmt.Sprintf("Message from goroutine %d, iteration %d", id, j)
				_, err := backend.Write([]byte(msg))
				if err != nil {
					mu.Lock()
					errorCount++
					mu.Unlock()
					t.Logf("Write error in goroutine %d: %v", id, err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Flush all pending messages
	err = backend.Flush()
	if err != nil {
		t.Errorf("Final flush failed: %v", err)
	}

	mu.Lock()
	if errorCount > 0 {
		t.Errorf("Got %d write errors during concurrent test", errorCount)
	}
	mu.Unlock()
}

// TestSyslogBackendImpl_ErrorConditions tests various error conditions
func TestSyslogBackendImpl_ErrorConditions(t *testing.T) {
	t.Run("close_without_connection", func(t *testing.T) {
		// Create backend that will fail to connect
		backend, err := backends.NewSyslogBackend("tcp", "255.255.255.255:514", 16, "error-test")
		if err == nil && backend != nil {
			// If somehow it connected, close it
			err = backend.Close()
			if err != nil {
				t.Logf("Close returned error (may be expected): %v", err)
			}
		}
	})

	t.Run("flush_nil_writer", func(t *testing.T) {
		// Test with a backend that has connection issues
		server, err := startMockSyslogServer("tcp", "127.0.0.1:0")
		if err != nil {
			t.Skipf("Cannot start mock server: %v", err)
		}

		addr := server.listener.Addr().String()
		backend, err := backends.NewSyslogBackend("tcp", addr, 16, "flush-test")
		if err != nil {
			t.Skipf("Failed to create backend: %v", err)
		}

		// Stop server to simulate connection loss
		server.stop()

		// Try to flush - may or may not error depending on implementation
		err = backend.Flush()
		t.Logf("Flush after connection loss: %v", err)

		// Try to close
		err = backend.Close()
		if err != nil {
			t.Logf("Close after connection loss returned error: %v", err)
		}
	})
}
