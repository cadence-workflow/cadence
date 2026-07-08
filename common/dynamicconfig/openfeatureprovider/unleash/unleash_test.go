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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeDecoder struct {
	cfg Config
	err error
}

func (d fakeDecoder) Decode(out any) error {
	if d.err != nil {
		return d.err
	}
	*out.(*Config) = d.cfg
	return nil
}

func TestNewProvider(t *testing.T) {
	tests := []struct {
		name    string
		decoder fakeDecoder
		wantErr string
	}{
		{
			name:    "decode error propagates",
			decoder: fakeDecoder{err: errors.New("boom")},
			wantErr: "failed to decode unleash provider config: boom",
		},
		{
			name:    "missing url",
			decoder: fakeDecoder{cfg: Config{AppName: "cadence"}},
			wantErr: "requires url",
		},
		{
			name:    "missing appName",
			decoder: fakeDecoder{cfg: Config{URL: "https://unleash.example.com/api"}},
			wantErr: "requires appName",
		},
		{
			name: "valid minimal config",
			decoder: fakeDecoder{cfg: Config{
				URL:     "https://unleash.example.com/api",
				AppName: "cadence",
			}},
		},
		{
			name: "valid full config",
			decoder: fakeDecoder{cfg: Config{
				URL:             "https://unleash.example.com/api",
				AppName:         "cadence",
				InstanceID:      "instance-1",
				Environment:     "production",
				APIToken:        "default:production.some-secret",
				RefreshInterval: 30 * time.Second,
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := newProvider(tt.decoder)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				assert.Nil(t, provider)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, provider)
		})
	}
}
