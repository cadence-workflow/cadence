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
	"errors"
	"fmt"
	"sync"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/syncmap"
)

//go:generate mockgen -package=$GOPACKAGE -destination=provider_mock.go -self_package=github.com/uber/cadence/common/dynamicconfig/provider github.com/uber/cadence/common/dynamicconfig/provider Provider

var (
	// ErrUnknownClient is the error for an unregistered dynamic config client name
	ErrUnknownClient = errors.New("unknown dynamic config client")
)

type (
	// BootstrapContainer holds the shared dependencies needed to construct any dynamicconfig.Client.
	BootstrapContainer struct {
		Logger            log.Logger
		MetricsClient     metrics.Client
		PersistenceConfig *config.Persistence
		RootDir           string
		Stopped           chan struct{}
	}

	// Provider returns a dynamicconfig.Client by name.
	// The client for a given name is created only once and cached.
	Provider interface {
		GetClient(name string) (dynamicconfig.Client, error)
	}

	clientProvider struct {
		sync.RWMutex

		configs   config.DynamicConfigProviders
		container *BootstrapContainer

		// Key is the client name.
		clients map[string]dynamicconfig.Client
	}

	constructor struct {
		fn func(cfg *config.YamlNode, container *BootstrapContainer) (dynamicconfig.Client, error)
		// yaml key where this config exists, under dynamicconfig.configs.
		// This almost certainly should be the same as the client name, but that'll need more work.
		configKey string
	}
)

var constructors = syncmap.New[string, constructor]()

// RegisterClient registers a constructor for a dynamicconfig.Client under the given name.
func RegisterClient(name, configKey string, fn func(cfg *config.YamlNode, container *BootstrapContainer) (dynamicconfig.Client, error)) error {
	inserted := constructors.Put(name, constructor{
		fn:        fn,
		configKey: configKey,
	})
	if !inserted {
		return fmt.Errorf("dynamic config client already registered for name %q", name)
	}
	return nil
}

// NewClientProvider returns a new dynamic config client Provider.
func NewClientProvider(configs config.DynamicConfigProviders, container *BootstrapContainer) Provider {
	return &clientProvider{
		configs:   configs,
		container: container,
		clients:   make(map[string]dynamicconfig.Client),
	}
}

func (p *clientProvider) GetClient(name string) (dynamicconfig.Client, error) {
	p.RLock()
	if client, ok := p.clients[name]; ok {
		p.RUnlock()
		return client, nil
	}
	p.RUnlock()

	c, ok := constructors.Get(name)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownClient, name)
	}

	// Config is optional: clients that don't need one (e.g. in-memory, nop) can ignore a nil cfg.
	cfg := p.configs[c.configKey]

	client, err := c.fn(cfg, p.container)
	if err != nil {
		return nil, fmt.Errorf("dynamic config client constructor failed for name %q, config key %q: err: %w", name, c.configKey, err)
	}

	p.Lock()
	defer p.Unlock()
	if existingClient, ok := p.clients[name]; ok {
		return existingClient, nil
	}
	p.clients[name] = client
	return client, nil
}
