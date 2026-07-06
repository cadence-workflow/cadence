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

package provider

import (
	"fmt"
	"path/filepath"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/configstore"
	csc "github.com/uber/cadence/common/dynamicconfig/configstore/config"
	"github.com/uber/cadence/common/dynamicconfig/filebased"
	"github.com/uber/cadence/common/persistence"
)

func init() {
	// TODO: ideally remove this and handle per-instance registration during startup somehow,
	// as globals and inits have consistently caused issues.
	//
	// For now though, it's replacing a hard-coded switch statement, so an init func
	// is the most straightforward and should-be-identical conversion.

	must := func(err error) {
		if err != nil {
			panic(fmt.Errorf("failed to register default dynamic config client: %w", err))
		}
	}

	must(RegisterClient(dynamicconfig.FileBasedClient, dynamicconfig.FileBasedClient, func(cfg *config.YamlNode, container *BootstrapContainer) (dynamicconfig.Client, error) {
		var out filebased.Config
		if err := cfg.Decode(&out); err != nil {
			return nil, fmt.Errorf("bad config: %w", err)
		}
		out.Filepath = constructPathIfNeed(container.RootDir, out.Filepath)
		return filebased.NewClient(&out, container.Logger, container.Stopped)
	}))

	must(RegisterClient(dynamicconfig.ConfigStoreClient, dynamicconfig.ConfigStoreClient, func(cfg *config.YamlNode, container *BootstrapContainer) (dynamicconfig.Client, error) {
		var out csc.ClientConfig
		if err := cfg.Decode(&out); err != nil {
			return nil, fmt.Errorf("bad config: %w", err)
		}
		return configstore.NewConfigStoreClient(&out, container.PersistenceConfig, container.Logger, container.MetricsClient, persistence.DynamicConfig)
	}))

	must(RegisterClient(dynamicconfig.InMemoryClient, dynamicconfig.InMemoryClient, func(cfg *config.YamlNode, container *BootstrapContainer) (dynamicconfig.Client, error) {
		return dynamicconfig.NewInMemoryClient(), nil
	}))

	must(RegisterClient(dynamicconfig.NopClient, dynamicconfig.NopClient, func(cfg *config.YamlNode, container *BootstrapContainer) (dynamicconfig.Client, error) {
		return dynamicconfig.NewNopClient(), nil
	}))
}

// constructPathIfNeed would append the dir as the root dir
// when the file wasn't absolute path.
func constructPathIfNeed(dir string, file string) string {
	if !filepath.IsAbs(file) {
		return dir + "/" + file
	}
	return file
}
