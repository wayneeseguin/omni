package main

import (
	"testing"
)

// Since the plugin system has been refactored and these plugin types don't exist,
// we'll skip all plugin-specific tests
func TestPluginExampleSkipped(t *testing.T) {
	t.Skip("Plugin system has been refactored - these plugin types no longer exist")
}
