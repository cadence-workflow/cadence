// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package fastgogenerate

import (
	"os"
	"time"

	"github.com/uber/cadence/tools/fastgogenerate/plugins/mockgen"
)

// Config contains the configuration for the fastgogenerate tool
type Config struct {
	// PluginName is the name of the plugin to be used
	// Default value - empty
	PluginName string

	// Debug enables debug logs
	// Default value - false
	Debug bool

	// CacheDisabled disables the cache so that the plugin will always be executed
	// Default value - false
	CacheDisabled bool

	// CacheDirPath is the path to the cache directory
	// Default value - os.TempDir()
	CacheDirPath string

	// Timeout is the timeout for the plugin execution
	// Default value - 1 minute
	Timeout time.Duration

	// BinarySuffixDisabled disables the usage of BinarySuffix to
	// determine the original binary name
	// By default is not disabled, fastgogenerate will use the <plugin_name>.<BinarySuffix> as a path plugin
	// otherwise it will use the binary name specified in a plugin config
	// Default value - false
	BinarySuffixDisabled bool

	// BinarySuffix is the suffix for the original binary
	// Default value - ".bin"
	BinarySuffix string

	// Mockgen is the configuration for the mockgen plugin
	Mockgen mockgen.Config
}

// LoadConfig loads the configuration from environment variables
func LoadConfig() Config {
	var cfg = Config{
		PluginName:           os.Getenv("FASTGOGENERATE_PLUGIN_NAME"),
		Debug:                os.Getenv("FASTGOGENERATE_DEBUG") != "",
		CacheDisabled:        os.Getenv("FASTGOGENERATE_CACHE_DISABLED") != "",
		Timeout:              time.Minute,
		BinarySuffixDisabled: os.Getenv("FASTGOGENERATE_BINARY_SUFFIX_DISABLED") != "",
		BinarySuffix:         "bin",
		CacheDirPath:         os.TempDir(),
		Mockgen: mockgen.Config{
			BinaryPath: os.Getenv("FASTGOGENERATE_MOCKGEN_BINARY_PATH"),
		},
	}

	if val := os.Getenv("FASTGOGENERATE_TIMEOUT"); val != "" {
		if timeout, err := time.ParseDuration(val); err == nil {
			cfg.Timeout = timeout
		}
	}

	if val := os.Getenv("FASTGOGENERATE_ORIGINAL_BINARY_SUFFIX"); val != "" {
		cfg.BinarySuffix = val
	}

	if val := os.Getenv("FASTGOGENERATE_CACHE_PATH"); val != "" {
		cfg.CacheDirPath = val
	}

	return cfg
}
