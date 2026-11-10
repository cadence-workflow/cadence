package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

func TestHostsMode(t *testing.T) {
	var cfg Config
	err := yaml.Unmarshal([]byte(getHostsConfig()), &cfg)
	assert.Nil(t, err)
	assert.Equal(t, "test", cfg.Name)
	assert.Equal(t, BootstrapModeHosts, cfg.BootstrapMode)
	assert.Equal(t, []string{"127.0.0.1:1111"}, cfg.BootstrapHosts)
	assert.Equal(t, time.Second*30, cfg.MaxJoinDuration)
	err = cfg.Validate()
	assert.Nil(t, err)
}

func TestFileMode(t *testing.T) {
	var cfg Config
	err := yaml.Unmarshal([]byte(getJSONConfig()), &cfg)
	assert.Nil(t, err)
	assert.Equal(t, "test", cfg.Name)
	assert.Equal(t, BootstrapModeFile, cfg.BootstrapMode)
	assert.Equal(t, "/tmp/file.json", cfg.BootstrapFile)
	assert.Equal(t, time.Second*30, cfg.MaxJoinDuration)
	err = cfg.Validate()
	assert.Nil(t, err)
}

func TestCustomMode(t *testing.T) {
	var cfg Config
	err := yaml.Unmarshal([]byte(getCustomConfig()), &cfg)
	assert.Nil(t, err)
	assert.Equal(t, "test", cfg.Name)
	assert.Equal(t, BootstrapModeCustom, cfg.BootstrapMode)
	assert.NotNil(t, cfg.Validate(), "custom bootstrap mode should not be supported")
}

func TestInvalidConfig(t *testing.T) {
	var cfg Config
	assert.NotNil(t, cfg.Validate())
	cfg.Name = "test"
	assert.NotNil(t, cfg.Validate())
	cfg.BootstrapMode = BootstrapModeNone
	assert.NotNil(t, cfg.Validate())
	_, err := parseBootstrapMode("unknown")
	assert.NotNil(t, err)
}

func getJSONConfig() string {
	return `name: "test"
bootstrapMode: "file"
bootstrapFile: "/tmp/file.json"
maxJoinDuration: 30s`
}

func getHostsConfig() string {
	return `name: "test"
bootstrapMode: "hosts"
bootstrapHosts: ["127.0.0.1:1111"]
maxJoinDuration: 30s`
}

func getCustomConfig() string {
	return `name: "test"
bootstrapMode: "custom"
maxJoinDuration: 30s`
}
