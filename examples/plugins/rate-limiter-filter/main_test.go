package main

import (
	"testing"
)

// Since the plugin system has been refactored and RateLimiterFilterPlugin doesn't exist,
// we'll skip all rate limiter filter plugin tests
func TestRateLimiterFilterPluginSkipped(t *testing.T) {
	t.Skip("RateLimiterFilterPlugin has been removed in the refactored code")
}