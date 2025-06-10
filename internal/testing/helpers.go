package testing

import (
	"os"
	"testing"
)

// Unit returns true if running in unit test mode.
// Unit tests should be fast and not require external services.
// This is determined by checking if the -short flag is set or
// if OMNI_UNIT_TESTS_ONLY environment variable is set.
func Unit() bool {
	// Check if explicitly running unit tests only (highest priority)
	if os.Getenv("OMNI_UNIT_TESTS_ONLY") == "true" {
		return true
	}

	// Check if integration tests are explicitly enabled (overrides -short flag for testing)
	if os.Getenv("OMNI_RUN_INTEGRATION_TESTS") == "true" {
		return false
	}

	// Check if integration tests are explicitly disabled
	if os.Getenv("OMNI_RUN_INTEGRATION_TESTS") == "false" {
		return true
	}

	// Check if running with -short flag (backward compatibility)
	if testing.Short() {
		return true
	}

	// Default to unit mode if not explicitly running integration tests
	return true
}

// Integration returns true if running in integration test mode.
// Integration tests may require external services like databases, message queues, etc.
func Integration() bool {
	return !Unit()
}

// SkipIfUnit skips the test if running in unit test mode.
func SkipIfUnit(t *testing.T, message ...string) {
	if Unit() {
		msg := "Skipping integration test in unit mode"
		if len(message) > 0 {
			msg = message[0]
		}
		t.Skip(msg)
	}
}

// SkipIfIntegration skips the test if running in integration test mode.
func SkipIfIntegration(t *testing.T, message ...string) {
	if Integration() {
		msg := "Skipping unit-only test in integration mode"
		if len(message) > 0 {
			msg = message[0]
		}
		t.Skip(msg)
	}
}
