package main

import (
	"testing"
)

// Since the plugin system has been refactored and RedisBackendPlugin doesn't exist,
// we'll skip all Redis backend plugin tests
func TestRedisBackendPluginSkipped(t *testing.T) {
	t.Skip("RedisBackendPlugin has been removed in the refactored code")
}