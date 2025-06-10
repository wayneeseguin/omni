package testing

import (
	"os"
	"testing"
)

// TestUnit tests the Unit function with different environment configurations
func TestUnit(t *testing.T) {
	// Save original environment
	originalUnitTests := os.Getenv("OMNI_UNIT_TESTS_ONLY")
	originalIntegrationTests := os.Getenv("OMNI_RUN_INTEGRATION_TESTS")

	defer func() {
		// Restore original environment
		if originalUnitTests == "" {
			os.Unsetenv("OMNI_UNIT_TESTS_ONLY")
		} else {
			os.Setenv("OMNI_UNIT_TESTS_ONLY", originalUnitTests)
		}
		if originalIntegrationTests == "" {
			os.Unsetenv("OMNI_RUN_INTEGRATION_TESTS")
		} else {
			os.Setenv("OMNI_RUN_INTEGRATION_TESTS", originalIntegrationTests)
		}
	}()

	tests := []struct {
		name                string
		unitTestsOnly       string
		runIntegrationTests string
		expectedUnit        bool
	}{
		{
			name:                "explicit unit tests only",
			unitTestsOnly:       "true",
			runIntegrationTests: "",
			expectedUnit:        true,
		},
		{
			name:                "explicit integration tests enabled",
			unitTestsOnly:       "",
			runIntegrationTests: "true",
			expectedUnit:        false,
		},
		{
			name:                "explicit integration tests disabled",
			unitTestsOnly:       "",
			runIntegrationTests: "false",
			expectedUnit:        true,
		},
		{
			name:                "default configuration",
			unitTestsOnly:       "",
			runIntegrationTests: "",
			expectedUnit:        true, // Default to unit mode
		},
		{
			name:                "unit tests override integration tests",
			unitTestsOnly:       "true",
			runIntegrationTests: "true",
			expectedUnit:        true, // Unit tests flag takes precedence
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear both environment variables first to ensure clean state
			os.Unsetenv("OMNI_UNIT_TESTS_ONLY")
			os.Unsetenv("OMNI_RUN_INTEGRATION_TESTS")

			// Set environment variables
			if tt.unitTestsOnly != "" {
				os.Setenv("OMNI_UNIT_TESTS_ONLY", tt.unitTestsOnly)
			}

			if tt.runIntegrationTests != "" {
				os.Setenv("OMNI_RUN_INTEGRATION_TESTS", tt.runIntegrationTests)
			}

			// Test Unit function
			result := Unit()
			if result != tt.expectedUnit {
				t.Errorf("Unit() = %v, expected %v", result, tt.expectedUnit)
			}
		})
	}
}

// TestIntegration tests the Integration function
func TestIntegration(t *testing.T) {
	// Save original environment
	originalUnitTests := os.Getenv("OMNI_UNIT_TESTS_ONLY")
	originalIntegrationTests := os.Getenv("OMNI_RUN_INTEGRATION_TESTS")

	defer func() {
		// Restore original environment
		if originalUnitTests == "" {
			os.Unsetenv("OMNI_UNIT_TESTS_ONLY")
		} else {
			os.Setenv("OMNI_UNIT_TESTS_ONLY", originalUnitTests)
		}
		if originalIntegrationTests == "" {
			os.Unsetenv("OMNI_RUN_INTEGRATION_TESTS")
		} else {
			os.Setenv("OMNI_RUN_INTEGRATION_TESTS", originalIntegrationTests)
		}
	}()

	tests := []struct {
		name                string
		unitTestsOnly       string
		runIntegrationTests string
		expectedIntegration bool
	}{
		{
			name:                "explicit unit tests only",
			unitTestsOnly:       "true",
			runIntegrationTests: "",
			expectedIntegration: false,
		},
		{
			name:                "explicit integration tests enabled",
			unitTestsOnly:       "",
			runIntegrationTests: "true",
			expectedIntegration: true,
		},
		{
			name:                "default configuration",
			unitTestsOnly:       "",
			runIntegrationTests: "",
			expectedIntegration: false, // Default to unit mode, so integration is false
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear both environment variables first to ensure clean state
			os.Unsetenv("OMNI_UNIT_TESTS_ONLY")
			os.Unsetenv("OMNI_RUN_INTEGRATION_TESTS")

			// Set environment variables
			if tt.unitTestsOnly != "" {
				os.Setenv("OMNI_UNIT_TESTS_ONLY", tt.unitTestsOnly)
			}

			if tt.runIntegrationTests != "" {
				os.Setenv("OMNI_RUN_INTEGRATION_TESTS", tt.runIntegrationTests)
			}

			// Test Integration function
			result := Integration()
			if result != tt.expectedIntegration {
				t.Errorf("Integration() = %v, expected %v", result, tt.expectedIntegration)
			}
		})
	}
}

// TestSkipIfUnit tests the SkipIfUnit function
func TestSkipIfUnit(t *testing.T) {
	// Save original environment
	originalIntegrationTests := os.Getenv("OMNI_RUN_INTEGRATION_TESTS")
	defer func() {
		if originalIntegrationTests == "" {
			os.Unsetenv("OMNI_RUN_INTEGRATION_TESTS")
		} else {
			os.Setenv("OMNI_RUN_INTEGRATION_TESTS", originalIntegrationTests)
		}
	}()

	t.Run("no_skip_in_integration_mode", func(t *testing.T) {
		// Clear both environment variables first to ensure clean state
		os.Unsetenv("OMNI_UNIT_TESTS_ONLY")
		os.Unsetenv("OMNI_RUN_INTEGRATION_TESTS")

		// Force integration mode
		os.Setenv("OMNI_RUN_INTEGRATION_TESTS", "true")

		// In integration mode, SkipIfUnit should not skip
		// We can test this by checking if Unit() returns false
		if Unit() {
			t.Error("Expected Unit() to return false in integration mode")
		}

		// The function should exist and be callable
		// We can't test the actual skip without causing our test to skip
		// But we can verify the logic is correct by checking the Unit() function
	})

	t.Run("would_skip_in_unit_mode", func(t *testing.T) {
		// Force unit mode
		os.Setenv("OMNI_RUN_INTEGRATION_TESTS", "false")

		// In unit mode, SkipIfUnit would skip (but we won't actually call it)
		// We verify the logic by checking if Unit() returns true
		if !Unit() {
			t.Error("Expected Unit() to return true in unit mode")
		}

		// Test that the function exists and can be called with different message patterns
		// We test this indirectly by testing the Unit() function it depends on
	})
}

// TestSkipIfIntegration tests the SkipIfIntegration function
func TestSkipIfIntegration(t *testing.T) {
	// Save original environment
	originalIntegrationTests := os.Getenv("OMNI_RUN_INTEGRATION_TESTS")
	defer func() {
		if originalIntegrationTests == "" {
			os.Unsetenv("OMNI_RUN_INTEGRATION_TESTS")
		} else {
			os.Setenv("OMNI_RUN_INTEGRATION_TESTS", originalIntegrationTests)
		}
	}()

	t.Run("would_skip_in_integration_mode", func(t *testing.T) {
		// Clear both environment variables first to ensure clean state
		os.Unsetenv("OMNI_UNIT_TESTS_ONLY")
		os.Unsetenv("OMNI_RUN_INTEGRATION_TESTS")

		// Force integration mode
		os.Setenv("OMNI_RUN_INTEGRATION_TESTS", "true")

		// In integration mode, SkipIfIntegration would skip (but we won't actually call it)
		// We verify the logic by checking if Integration() returns true
		if !Integration() {
			t.Error("Expected Integration() to return true in integration mode")
		}
	})

	t.Run("no_skip_in_unit_mode", func(t *testing.T) {
		// Force unit mode
		os.Setenv("OMNI_RUN_INTEGRATION_TESTS", "false")

		// In unit mode, SkipIfIntegration should not skip
		// We can test this by checking if Integration() returns false
		if Integration() {
			t.Error("Expected Integration() to return false in unit mode")
		}
	})
}

// TestShortFlagBehavior tests the interaction with testing.Short()
// Note: This is harder to test directly since testing.Short() is controlled by the test runner
func TestShortFlagBehavior(t *testing.T) {
	// Save original environment
	originalIntegrationTests := os.Getenv("OMNI_RUN_INTEGRATION_TESTS")
	originalUnitTests := os.Getenv("OMNI_UNIT_TESTS_ONLY")

	defer func() {
		if originalIntegrationTests == "" {
			os.Unsetenv("OMNI_RUN_INTEGRATION_TESTS")
		} else {
			os.Setenv("OMNI_RUN_INTEGRATION_TESTS", originalIntegrationTests)
		}
		if originalUnitTests == "" {
			os.Unsetenv("OMNI_UNIT_TESTS_ONLY")
		} else {
			os.Setenv("OMNI_UNIT_TESTS_ONLY", originalUnitTests)
		}
	}()

	t.Run("test_short_flag_compatibility", func(t *testing.T) {
		// Clear environment to test -short flag behavior
		os.Unsetenv("OMNI_UNIT_TESTS_ONLY")
		os.Unsetenv("OMNI_RUN_INTEGRATION_TESTS")

		// The Unit() function should return true if testing.Short() is true
		// OR if the default logic applies (no integration tests enabled)
		result := Unit()

		// In normal testing (without -short flag), this should default to true
		// With -short flag, it should also be true
		// We can't directly control testing.Short() here, but we can verify
		// the function works with the current test runner configuration
		if !result {
			// This might happen if integration tests are explicitly enabled
			t.Logf("Unit() returned false - may be due to integration test configuration")
		}
	})
}

// TestSkipFunctionsIndirect tests that the skip functions can be called
// This doesn't test the actual skipping but ensures the functions exist and can be invoked
func TestSkipFunctionsIndirect(t *testing.T) {
	// Save original environment
	originalIntegrationTests := os.Getenv("OMNI_RUN_INTEGRATION_TESTS")
	defer func() {
		if originalIntegrationTests == "" {
			os.Unsetenv("OMNI_RUN_INTEGRATION_TESTS")
		} else {
			os.Setenv("OMNI_RUN_INTEGRATION_TESTS", originalIntegrationTests)
		}
	}()

	// Test that we can call the skip functions when they should NOT skip
	// This way we exercise the code paths without actually skipping our test
	t.Run("test_skip_functions_when_not_skipping", func(t *testing.T) {
		// Clear both environment variables first to ensure clean state
		os.Unsetenv("OMNI_UNIT_TESTS_ONLY")
		os.Unsetenv("OMNI_RUN_INTEGRATION_TESTS")

		// Force integration mode - SkipIfUnit should not skip
		os.Setenv("OMNI_RUN_INTEGRATION_TESTS", "true")

		// This should not skip because we're in integration mode
		SkipIfUnit(t)
		SkipIfUnit(t, "Custom message for unit skip")

		// Force unit mode - SkipIfIntegration should not skip
		os.Setenv("OMNI_RUN_INTEGRATION_TESTS", "false")

		// This should not skip because we're in unit mode
		SkipIfIntegration(t)
		SkipIfIntegration(t, "Custom message for integration skip")
	})
}

// TestEnvironmentVariableEdgeCases tests edge cases for environment variable handling
func TestEnvironmentVariableEdgeCases(t *testing.T) {
	// Save original environment
	originalUnitTests := os.Getenv("OMNI_UNIT_TESTS_ONLY")
	originalIntegrationTests := os.Getenv("OMNI_RUN_INTEGRATION_TESTS")

	defer func() {
		// Restore original environment
		if originalUnitTests == "" {
			os.Unsetenv("OMNI_UNIT_TESTS_ONLY")
		} else {
			os.Setenv("OMNI_UNIT_TESTS_ONLY", originalUnitTests)
		}
		if originalIntegrationTests == "" {
			os.Unsetenv("OMNI_RUN_INTEGRATION_TESTS")
		} else {
			os.Setenv("OMNI_RUN_INTEGRATION_TESTS", originalIntegrationTests)
		}
	}()

	tests := []struct {
		name                string
		unitTestsOnly       string
		runIntegrationTests string
		expectedUnit        bool
		expectedIntegration bool
	}{
		{
			name:                "empty strings",
			unitTestsOnly:       "",
			runIntegrationTests: "",
			expectedUnit:        true,
			expectedIntegration: false,
		},
		{
			name:                "false values",
			unitTestsOnly:       "false",
			runIntegrationTests: "false",
			expectedUnit:        true,
			expectedIntegration: false,
		},
		{
			name:                "invalid values",
			unitTestsOnly:       "invalid",
			runIntegrationTests: "invalid",
			expectedUnit:        true, // defaults to unit mode for invalid values
			expectedIntegration: false,
		},
		{
			name:                "case sensitivity",
			unitTestsOnly:       "TRUE",
			runIntegrationTests: "TRUE",
			expectedUnit:        true, // unit tests flag takes precedence
			expectedIntegration: false,
		},
		{
			name:                "numeric values",
			unitTestsOnly:       "1",
			runIntegrationTests: "1",
			expectedUnit:        true, // unit tests flag takes precedence
			expectedIntegration: false,
		},
		{
			name:                "only integration enabled",
			unitTestsOnly:       "",
			runIntegrationTests: "true",
			expectedUnit:        false,
			expectedIntegration: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear both environment variables first to ensure clean state
			os.Unsetenv("OMNI_UNIT_TESTS_ONLY")
			os.Unsetenv("OMNI_RUN_INTEGRATION_TESTS")

			// Set environment variables
			if tt.unitTestsOnly != "" {
				os.Setenv("OMNI_UNIT_TESTS_ONLY", tt.unitTestsOnly)
			}

			if tt.runIntegrationTests != "" {
				os.Setenv("OMNI_RUN_INTEGRATION_TESTS", tt.runIntegrationTests)
			}

			// Test both functions
			unitResult := Unit()
			integrationResult := Integration()

			if unitResult != tt.expectedUnit {
				t.Errorf("Unit() = %v, expected %v", unitResult, tt.expectedUnit)
			}
			if integrationResult != tt.expectedIntegration {
				t.Errorf("Integration() = %v, expected %v", integrationResult, tt.expectedIntegration)
			}

			// Verify that Unit() and Integration() are mutually exclusive
			if unitResult && integrationResult {
				t.Error("Unit() and Integration() should not both return true")
			}
		})
	}
}
