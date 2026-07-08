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

package openfeatureprovider

import (
	"errors"
	"testing"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/stretchr/testify/assert"
)

func TestRegisterAndGet(t *testing.T) {
	fakeConstructor := func(Decoder) (openfeature.FeatureProvider, error) {
		return nil, nil
	}

	tests := []struct {
		name    string
		setup   func(providerName string)
		wantErr bool
	}{
		{
			name: "first registration succeeds",
		},
		{
			name: "duplicate registration fails",
			setup: func(providerName string) {
				_ = Register(providerName, fakeConstructor)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providerName := "test-" + t.Name()
			if tt.setup != nil {
				tt.setup(providerName)
			}

			err := Register(providerName, fakeConstructor)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			constructor, ok := Get(providerName)
			assert.True(t, ok)
			assert.NotNil(t, constructor)
		})
	}
}

func TestGet_Unknown(t *testing.T) {
	_, ok := Get("does-not-exist")
	assert.False(t, ok)
}

type fakeDecoder struct {
	err error
}

func (d fakeDecoder) Decode(out any) error {
	return d.err
}

func TestConstructor_PropagatesDecodeError(t *testing.T) {
	wantErr := errors.New("boom")
	constructor := func(cfg Decoder) (openfeature.FeatureProvider, error) {
		var out struct{}
		return nil, cfg.Decode(&out)
	}

	_, err := constructor(fakeDecoder{err: wantErr})
	assert.ErrorIs(t, err, wantErr)
}
