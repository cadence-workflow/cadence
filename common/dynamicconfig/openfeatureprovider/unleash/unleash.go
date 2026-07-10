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

package unleash

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"time"

	unleashclient "github.com/Unleash/unleash-client-go/v4"
	unleashprovider "github.com/open-feature/go-sdk-contrib/providers/unleash/pkg"
	"github.com/open-feature/go-sdk/openfeature"

	"github.com/uber/cadence/common/dynamicconfig/openfeatureprovider"
)

// ProviderName is the name this plugin registers under; set
// dynamicconfig.openfeature.providerName to this value to select it.
const ProviderName = "unleash"

// Config is Unleash's own provider configuration. It is decoded from the
// "provider" block of dynamicconfig.openfeature config when providerName is
// "unleash" - see common/dynamicconfig/openfeatureclient/config.Config.
type Config struct {
	// URL is the Unleash API URL, e.g. "https://unleash.example.com/api".
	URL string `yaml:"url"`
	// AppName identifies this application to Unleash.
	AppName string `yaml:"appName"`
	// InstanceID identifies this process instance to Unleash.
	InstanceID string `yaml:"instanceId"`
	// Environment is the Unleash environment to evaluate flags against.
	Environment string `yaml:"environment"`
	// APIToken authenticates against Unleash's API. Required by Unleash OSS
	// and Enterprise servers (a client-side/backend token, e.g.
	// "default:development.unleash-insecure-api-token"); left empty for
	// servers that don't enforce authentication.
	APIToken string `yaml:"apiToken"`
	// RefreshInterval controls how often the client polls Unleash for flag updates.
	// Defaults to the Unleash client library's own default when zero.
	RefreshInterval time.Duration `yaml:"refreshInterval"`
	// BootstrapFile, if set, points to a JSON file in the same format as
	// Unleash's `/api/client/features` response. Its contents seed the
	// client's flag repository before the first successful poll, so
	// evaluations succeed immediately even if Unleash is briefly unreachable
	// at startup - primarily meant for local dev/docker-compose, to bootstrap
	// Unleash with predefined flags without a manual admin-UI setup step.
	BootstrapFile string `yaml:"bootstrapFile"`
}

func newProvider(cfg openfeatureprovider.Decoder) (openfeature.FeatureProvider, error) {
	var c Config
	if err := cfg.Decode(&c); err != nil {
		return nil, fmt.Errorf("failed to decode unleash provider config: %w", err)
	}
	if c.URL == "" {
		return nil, fmt.Errorf("unleash provider config requires url")
	}
	if c.AppName == "" {
		return nil, fmt.Errorf("unleash provider config requires appName")
	}

	options := []unleashclient.ConfigOption{
		unleashclient.WithUrl(c.URL),
		unleashclient.WithAppName(c.AppName),
	}
	if c.InstanceID != "" {
		options = append(options, unleashclient.WithInstanceId(c.InstanceID))
	}
	if c.Environment != "" {
		options = append(options, unleashclient.WithEnvironment(c.Environment))
	}
	if c.APIToken != "" {
		options = append(options, unleashclient.WithCustomHeaders(http.Header{"Authorization": {c.APIToken}}))
	}
	if c.RefreshInterval > 0 {
		options = append(options, unleashclient.WithRefreshInterval(c.RefreshInterval))
	}
	if c.BootstrapFile != "" {
		data, err := os.ReadFile(c.BootstrapFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read unleash bootstrap file %q: %w", c.BootstrapFile, err)
		}
		// BootstrapStorage.Load reads its Reader synchronously and only once,
		// during unleash-client-go's Initialize call - so an in-memory
		// bytes.Reader over the file contents is enough, no need to keep the
		// file open for the client's lifetime.
		options = append(options, unleashclient.WithStorage(&unleashclient.BootstrapStorage{Reader: bytes.NewReader(data)}))
	}

	provider, err := unleashprovider.NewProvider(unleashprovider.ProviderConfig{Options: options})
	if err != nil {
		return nil, fmt.Errorf("failed to create unleash provider: %w", err)
	}
	return provider, nil
}
