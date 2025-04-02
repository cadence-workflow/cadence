package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultiFileKeyTrackingProvider(t *testing.T) {
	// Test with multiple in-memory sources
	baseConfig := []byte(`
db:
  login: user
  pass: secret
server:
  host: localhost
`)

	loggingConfig := []byte(`
log:
  level: info
  file: /var/log/app.log
`)

	metricsConfig := []byte(`
metrics:
  enabled: true
  port: 9090
cache:
  ttl: 3600
`)

	sources := []FileSource{
		{Path: "base.yaml", Content: baseConfig},
		{Path: "logging.yaml", Content: loggingConfig},
		{Path: "metrics.yaml", Content: metricsConfig},
	}

	// Create provider
	provider, err := NewKeyTrackingProviderFromSources(sources)
	assert.NoError(t, err)

	// Check all keys across all files are collected
	allKeys := provider.GetAllKeys()
	assert.ElementsMatch(t, []string{"db", "server", "log", "metrics", "cache"}, allKeys)

	// Initially no keys are accessed
	unusedKeys := provider.GetUnusedKeys()
	assert.ElementsMatch(t, []string{"db", "server", "log", "metrics", "cache"}, unusedKeys)

	// Test key merging by ensuring values from different files are available
	assert.True(t, provider.Get("db.login").HasValue())
	assert.True(t, provider.Get("log.level").HasValue())
	assert.True(t, provider.Get("metrics.enabled").HasValue())

	// Access some keys
	provider.Get("db")
	provider.Get("log.level")

	// Check tracked keys
	unusedKeys = provider.GetUnusedKeys()
	assert.ElementsMatch(t, []string{"cache", "server"}, unusedKeys)

	// Access remaining keys
	provider.Get("metrics")
	provider.Get("cache.ttl")
	provider.Get("server")

	// Now all keys should be accessed
	unusedKeys = provider.GetUnusedKeys()
	assert.Empty(t, unusedKeys)

	// Verification should pass
	err = provider.VerifyAllKeysAccessed()
	assert.NoError(t, err)
}

func TestKeyOverrideAcrossFiles(t *testing.T) {
	// Test that later files override earlier ones but all keys are still tracked
	baseConfig := []byte(`
common:
  setting: base-value
  base-only: base-specific
`)

	overrideConfig := []byte(`
common:
  setting: override-value
  override-only: override-specific
`)

	sources := []FileSource{
		{Path: "base.yaml", Content: baseConfig},
		{Path: "override.yaml", Content: overrideConfig},
	}

	// Create provider
	provider, err := NewKeyTrackingProviderFromSources(sources)
	require.NoError(t, err)

	// Only one "common" key since keys are merged
	allKeys := provider.GetAllKeys()
	assert.ElementsMatch(t, []string{"common"}, allKeys)

	// Verify override worked - we should get the value from the second file
	val := provider.Get("common.setting").Value()
	assert.Equal(t, "override-value", val)

	// Both nested keys should be accessible
	assert.True(t, provider.Get("common.base-only").HasValue())
	assert.True(t, provider.Get("common.override-only").HasValue())

	// Mark common as accessed
	provider.Get("common")

	// All keys should be accessed
	unusedKeys := provider.GetUnusedKeys()
	assert.Empty(t, unusedKeys)
}
