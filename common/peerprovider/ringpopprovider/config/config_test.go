// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/uber/ringpop-go/discovery/statichosts"
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
	assert.NotNil(t, cfg.Validate())
	cfg.DiscoveryProvider = statichosts.New("127.0.0.1")
	assert.Nil(t, cfg.Validate())
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
