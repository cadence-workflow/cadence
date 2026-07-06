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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
)

func TestRegisterClient_Duplicate(t *testing.T) {
	err := RegisterClient(dynamicconfig.NopClient, dynamicconfig.NopClient, func(cfg *config.YamlNode, container *BootstrapContainer) (dynamicconfig.Client, error) {
		return dynamicconfig.NewNopClient(), nil
	})
	require.Error(t, err)
}

func TestGetClient_UnknownClient(t *testing.T) {
	p := NewClientProvider(nil, &BootstrapContainer{})
	_, err := p.GetClient("does-not-exist")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnknownClient)
}

func TestGetClient_BuiltinsWithoutConfig(t *testing.T) {
	p := NewClientProvider(config.DynamicConfigProviders{}, &BootstrapContainer{})

	nopClient, err := p.GetClient(dynamicconfig.NopClient)
	require.NoError(t, err)
	assert.NotNil(t, nopClient)

	memClient, err := p.GetClient(dynamicconfig.InMemoryClient)
	require.NoError(t, err)
	assert.NotNil(t, memClient)
}

func TestGetClient_CachesInstance(t *testing.T) {
	p := NewClientProvider(config.DynamicConfigProviders{}, &BootstrapContainer{})

	first, err := p.GetClient(dynamicconfig.NopClient)
	require.NoError(t, err)
	second, err := p.GetClient(dynamicconfig.NopClient)
	require.NoError(t, err)
	assert.Same(t, first, second)
}

func TestGetClient_ConstructorError(t *testing.T) {
	const name = "test-fake-erroring-client"
	require.NoError(t, RegisterClient(name, name, func(cfg *config.YamlNode, container *BootstrapContainer) (dynamicconfig.Client, error) {
		return nil, errors.New("boom")
	}))

	p := NewClientProvider(config.DynamicConfigProviders{}, &BootstrapContainer{})
	_, err := p.GetClient(name)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}
