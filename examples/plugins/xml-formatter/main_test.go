package main

import (
	"testing"
)

// Since the plugin system has been refactored and XMLFormatterPlugin doesn't exist,
// we'll skip all XML formatter plugin tests
func TestXMLFormatterPluginSkipped(t *testing.T) {
	t.Skip("XMLFormatterPlugin has been removed in the refactored code")
}
