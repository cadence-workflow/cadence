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

package peerprovider

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/config/yaml"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/syncmap"
)

func TestProviderRetrunsErrorWhenNoProviderRegistered(t *testing.T) {
	// Reset plugins
	plugins = syncmap.New[string, plugin]()
	a := Provider{
		config:    nil,
		container: Container{},
	}
	p, err := a.Provider()
	assert.Nil(t, p)
	assert.EqualError(t, err, "no configured peer providers found")
}

func TestProviderRetrunsErrorWhenPluginAlreadyRegistered(t *testing.T) {
	// Reset plugins
	plugins = syncmap.New[string, plugin]()
	err := Register("provider1", func(cfg *yaml.Node, container Container) (membership.PeerProvider, error) {
		return nil, nil
	})
	assert.NoError(t, err)
	err = Register("provider2", func(cfg *yaml.Node, container Container) (membership.PeerProvider, error) {
		return nil, nil
	})
	assert.Error(t, err)
}

func TestConfigIsPickedUp(t *testing.T) {
	// Reset plugins
	plugins = syncmap.New[string, plugin]()

	peerProviderConfig := map[string]*yaml.Node{}
	peerProviderConfig["provider1"] = &yaml.Node{}

	pp := New(peerProviderConfig, Container{})
	err := Register("provider1", func(cfg *yaml.Node, container Container) (membership.PeerProvider, error) {
		return nil, nil
	})
	assert.NoError(t, err)
	_, err = pp.Provider()
	assert.NoError(t, err)
}

func TestErrorWhenConfigIsNotProvided(t *testing.T) {
	// Reset plugins
	plugins = syncmap.New[string, plugin]()
	pp := New(config.PeerProvider{}, Container{})
	err := Register("provider1", func(cfg *yaml.Node, container Container) (membership.PeerProvider, error) {
		return nil, nil
	})
	p, err := pp.Provider()
	assert.Nil(t, p)
	assert.EqualError(t, err, "no configuration for \"provider1\" peer provider found")
}
