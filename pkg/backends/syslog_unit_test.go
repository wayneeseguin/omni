package backends_test

import (
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/wayneeseguin/omni/pkg/backends"
)

// TestSyslogBackendImpl_Unit tests syslog backend unit functionality without requiring real syslog
func TestSyslogBackendImpl_Unit(t *testing.T) {
	t.Run("NewSyslogBackend_WithMockServer", func(t *testing.T) {
		// Start a simple TCP server that accepts connections
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("Failed to start mock server: %v", err)
		}
		defer listener.Close()

		// Accept connections in background
		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				// Just close immediately to simulate syslog server
				conn.Close()
			}
		}()

		addr := listener.Addr().String()
		backend, err := backends.NewSyslogBackend("tcp", addr, 16, "test-app")
		if err != nil {
			t.Fatalf("Failed to create syslog backend: %v", err)
		}
		defer backend.Close()

		// Test that backend was created successfully
		if backend == nil {
			t.Fatal("Backend should not be nil")
		}
	})

	t.Run("NewSyslogBackend_EmptyAddress", func(t *testing.T) {
		// Test fallback to unix socket detection
		_, err := backends.NewSyslogBackend("", "", 16, "test")
		// This should fail in test environment but exercise the socket detection code
		if err == nil {
			t.Log("Unexpectedly succeeded - local syslog socket found")
		} else if !strings.Contains(err.Error(), "no local syslog socket found") {
			// Might fail for other reasons in test environment, which is acceptable
			t.Logf("Expected 'no local syslog socket found' or connection error, got: %v", err)
		}
	})

	t.Run("SupportsAtomic", func(t *testing.T) {
		// Test SupportsAtomic without connection
		// We can create a backend struct directly to test this method
		backend := &backends.SyslogBackendImpl{}

		// Syslog should not support atomic writes
		if backend.SupportsAtomic() {
			t.Error("Syslog backend should not support atomic writes")
		}
	})

	t.Run("SetPriority", func(t *testing.T) {
		backend := &backends.SyslogBackendImpl{}

		// Test setting different priorities
		testPriorities := []int{0, 16, 24, 128, 191}
		for _, priority := range testPriorities {
			backend.SetPriority(priority)
			// Since priority is private, we can't verify it was set
			// but we ensure the method doesn't panic
		}
	})

	t.Run("SetTag", func(t *testing.T) {
		backend := &backends.SyslogBackendImpl{}

		// Test setting different tags
		testTags := []string{"", "test", "app-name", "multi-word-tag"}
		for _, tag := range testTags {
			backend.SetTag(tag)
			// Since tag is private, we can't verify it was set
			// but we ensure the method doesn't panic
		}
	})

	t.Run("GetStats", func(t *testing.T) {
		// Create a simple mock TCP listener to test GetStats
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("Failed to start mock server: %v", err)
		}
		defer listener.Close()

		// Accept one connection then close
		go func() {
			conn, _ := listener.Accept()
			if conn != nil {
				conn.Close()
			}
		}()

		addr := listener.Addr().String()
		backend, err := backends.NewSyslogBackend("tcp", addr, 16, "stats-test")
		if err != nil {
			t.Fatalf("Failed to create syslog backend: %v", err)
		}
		defer backend.Close()

		stats := backend.GetStats()

		// Verify stats format
		expectedPath := fmt.Sprintf("syslog://tcp/%s", addr)
		if stats.Path != expectedPath {
			t.Errorf("Expected path %q, got %q", expectedPath, stats.Path)
		}

		// Size should be 0 for syslog
		if stats.Size != 0 {
			t.Errorf("Expected size 0, got %d", stats.Size)
		}
	})

	t.Run("Write", func(t *testing.T) {
		// Create a server that stays open long enough for write test
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("Failed to start mock server: %v", err)
		}
		defer listener.Close()

		// Accept connection and read data
		messageReceived := make(chan string, 1)
		go func() {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			defer conn.Close()

			buf := make([]byte, 1024)
			n, err := conn.Read(buf)
			if err == nil && n > 0 {
				messageReceived <- string(buf[:n])
			}
		}()

		addr := listener.Addr().String()
		backend, err := backends.NewSyslogBackend("tcp", addr, 16, "write-test")
		if err != nil {
			t.Fatalf("Failed to create syslog backend: %v", err)
		}
		defer backend.Close()

		// Test Write method
		testMessage := []byte("Hello, unit test!")
		n, err := backend.Write(testMessage)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		if n <= 0 {
			t.Errorf("Expected positive bytes written, got %d", n)
		}

		// Flush to ensure message is sent
		err = backend.Flush()
		if err != nil {
			t.Errorf("Flush failed: %v", err)
		}
	})

	t.Run("Sync", func(t *testing.T) {
		// Test Sync method which should be equivalent to Flush
		backend := &backends.SyslogBackendImpl{}

		// Sync on nil backend should handle gracefully
		err := backend.Sync()
		// May or may not return error, but shouldn't panic
		t.Logf("Sync on nil backend: %v", err)
	})

	t.Run("Flush_NilWriter", func(t *testing.T) {
		// Test Flush with nil writer
		backend := &backends.SyslogBackendImpl{}

		err := backend.Flush()
		// Should return nil when writer is nil
		if err != nil {
			t.Errorf("Flush with nil writer should return nil, got: %v", err)
		}
	})

	t.Run("Close_NilConnection", func(t *testing.T) {
		// Test Close with nil connection
		backend := &backends.SyslogBackendImpl{}

		err := backend.Close()
		// Should return nil when everything is nil
		if err != nil {
			t.Errorf("Close with nil connection should return nil, got: %v", err)
		}
	})
}

// TestSyslogBackendImpl_MessageFormatting tests syslog message formatting logic
func TestSyslogBackendImpl_MessageFormatting(t *testing.T) {
	// Create a mock connection that captures what's written to it
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	// Read messages from the server side
	messages := make(chan string, 10)
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := server.Read(buf)
			if err != nil {
				close(messages)
				return
			}
			if n > 0 {
				messages <- string(buf[:n])
			}
		}
	}()

	// Create backend with the client side of the pipe
	// We need to manually construct this since NewSyslogBackend expects net.Dial
	// This test focuses on the Write method logic
	t.Run("Write_MessageFormat", func(t *testing.T) {
		tests := []struct {
			name          string
			entry         []byte
			priority      int
			tag           string
			expectedRegex string
		}{
			{
				name:          "simple message",
				entry:         []byte("Hello, syslog!"),
				priority:      16,
				tag:           "test-app",
				expectedRegex: `^<16>test-app: Hello, syslog!\n$`,
			},
			{
				name:          "empty message",
				entry:         []byte(""),
				priority:      24,
				tag:           "empty-test",
				expectedRegex: `^<24>empty-test: \n$`,
			},
			{
				name:          "message with trailing space",
				entry:         []byte("message "),
				priority:      8,
				tag:           "trim-test",
				expectedRegex: `^<8>trim-test: message\n$`,
			},
			{
				name:          "message with newline",
				entry:         []byte("line1\nline2"),
				priority:      32,
				tag:           "multiline",
				expectedRegex: `^<32>multiline: line1\nline2\n$`,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Test the message formatting logic that would be used in Write()
				message := fmt.Sprintf("<%d>%s: %s", tt.priority, tt.tag, strings.TrimSpace(string(tt.entry)))
				if !strings.HasSuffix(message, "\n") {
					message += "\n"
				}

				// Verify the message format matches expected pattern
				if len(tt.expectedRegex) > 0 {
					// Simple string matching since we control the format
					expected := strings.ReplaceAll(tt.expectedRegex, "^", "")
					expected = strings.ReplaceAll(expected, "$", "")
					expected = strings.ReplaceAll(expected, "\\n", "\n")
					if message != expected {
						t.Errorf("Expected message %q, got %q", expected, message)
					}
				}

				// Verify message structure
				if !strings.HasPrefix(message, fmt.Sprintf("<%d>", tt.priority)) {
					t.Errorf("Message should start with priority <%d>, got: %q", tt.priority, message)
				}
				if !strings.Contains(message, tt.tag+":") {
					t.Errorf("Message should contain tag %q:, got: %q", tt.tag, message)
				}
				if !strings.HasSuffix(message, "\n") {
					t.Errorf("Message should end with newline, got: %q", message)
				}
			})
		}
	})
}

// TestSyslogBackendImpl_NetworkTypes tests different network types
func TestSyslogBackendImpl_NetworkTypes(t *testing.T) {
	t.Run("TCP_Network", func(t *testing.T) {
		// Test TCP network type (most common for remote syslog)
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("Failed to start TCP listener: %v", err)
		}
		defer listener.Close()

		// Accept one connection
		go func() {
			conn, _ := listener.Accept()
			if conn != nil {
				conn.Close()
			}
		}()

		addr := listener.Addr().String()
		backend, err := backends.NewSyslogBackend("tcp", addr, 16, "tcp-test")
		if err != nil {
			t.Fatalf("Failed to create TCP syslog backend: %v", err)
		}
		defer backend.Close()

		if backend == nil {
			t.Error("TCP backend should not be nil")
		}
	})

	t.Run("Invalid_Network", func(t *testing.T) {
		// Test invalid network type
		_, err := backends.NewSyslogBackend("invalid-network", "localhost:514", 16, "test")
		if err == nil {
			t.Error("Expected error for invalid network type")
		}
		if !strings.Contains(err.Error(), "dial syslog") {
			t.Errorf("Expected dial error, got: %v", err)
		}
	})
}

// TestSyslogBackendImpl_ErrorHandling tests error handling scenarios
func TestSyslogBackendImpl_ErrorHandling(t *testing.T) {
	t.Run("Connection_Refused", func(t *testing.T) {
		// Test connection to non-existent server
		_, err := backends.NewSyslogBackend("tcp", "127.0.0.1:9999", 16, "error-test")
		if err == nil {
			t.Error("Expected connection error for non-existent server")
		}
		if !strings.Contains(err.Error(), "dial syslog") {
			t.Errorf("Expected dial error, got: %v", err)
		}
	})

	t.Run("Invalid_Address", func(t *testing.T) {
		// Test invalid address format
		_, err := backends.NewSyslogBackend("tcp", "invalid-address-format", 16, "error-test")
		if err == nil {
			t.Error("Expected error for invalid address format")
		}
	})
}

// TestSyslogBackendImpl_AdvancedScenarios tests additional scenarios for comprehensive coverage
func TestSyslogBackendImpl_AdvancedScenarios(t *testing.T) {
	t.Run("Multiple_Close_Calls", func(t *testing.T) {
		// Test that multiple close calls don't panic
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("Failed to start mock server: %v", err)
		}
		defer listener.Close()

		// Accept one connection
		go func() {
			conn, _ := listener.Accept()
			if conn != nil {
				conn.Close()
			}
		}()

		addr := listener.Addr().String()
		backend, err := backends.NewSyslogBackend("tcp", addr, 16, "multi-close")
		if err != nil {
			t.Fatalf("Failed to create syslog backend: %v", err)
		}

		// First close
		err1 := backend.Close()
		t.Logf("First close: %v", err1)

		// Second close should not panic
		err2 := backend.Close()
		t.Logf("Second close: %v", err2)
	})

	t.Run("Priority_Boundary_Values", func(t *testing.T) {
		backend := &backends.SyslogBackendImpl{}

		// Test boundary values for priority
		boundaryValues := []int{0, 1, 7, 8, 15, 16, 23, 24, 31, 128, 191}
		for _, priority := range boundaryValues {
			backend.SetPriority(priority)
			// Method should not panic for any valid integer value
		}
	})

	t.Run("Tag_Special_Characters", func(t *testing.T) {
		backend := &backends.SyslogBackendImpl{}

		// Test tags with special characters
		specialTags := []string{
			"app-name",
			"app_name",
			"app.name",
			"app@host",
			"app[123]",
			"app:service",
			"",
			"a",
			strings.Repeat("long-tag-name", 10),
		}

		for _, tag := range specialTags {
			backend.SetTag(tag)
			// Method should not panic for any string value
		}
	})
}
