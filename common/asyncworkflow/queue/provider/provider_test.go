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

package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/messaging"
)

func TestAsyncQueueProducerProvider(t *testing.T) {
	testCases := []struct {
		name      string
		queueType string
		setup     func()
		wantErr   bool
	}{
		{
			name:      "Success case",
			queueType: "q1",
			wantErr:   false,
		},
		{
			name:      "Duplicate type",
			queueType: "q2",
			setup: func() {
				RegisterAsyncQueueProducerProvider("q2", func(*config.YamlNode, *Params) (messaging.Producer, error) {
					return nil, nil
				})
			},
			wantErr: true,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := GetAsyncQueueProducerProvider(tt.queueType)
			assert.False(t, ok)

			if tt.setup != nil {
				tt.setup()
			}

			err := RegisterAsyncQueueProducerProvider(tt.queueType, func(*config.YamlNode, *Params) (messaging.Producer, error) {
				return nil, nil
			})
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			_, ok = GetAsyncQueueProducerProvider(tt.queueType)
			assert.True(t, ok)
		})
	}
}

func TestAsyncQueueConsumerProvider(t *testing.T) {
	testCases := []struct {
		name      string
		queueType string
		setup     func()
		wantErr   bool
	}{
		{
			name:      "Success case",
			queueType: "q1",
			wantErr:   false,
		},
		{
			name:      "Duplicate type",
			queueType: "q2",
			setup: func() {
				RegisterAsyncQueueConsumerProvider("q2", func(*config.YamlNode, *Params) (messaging.Consumer, error) {
					return nil, nil
				})
			},
			wantErr: true,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := GetAsyncQueueConsumerProvider(tt.queueType)
			assert.False(t, ok)

			if tt.setup != nil {
				tt.setup()
			}

			err := RegisterAsyncQueueConsumerProvider(tt.queueType, func(*config.YamlNode, *Params) (messaging.Consumer, error) {
				return nil, nil
			})
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			_, ok = GetAsyncQueueConsumerProvider(tt.queueType)
			assert.True(t, ok)
		})
	}
}
