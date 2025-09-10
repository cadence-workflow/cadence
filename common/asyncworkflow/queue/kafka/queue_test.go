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

package kafka

import (
	"errors"
	"testing"

	"github.com/Shopify/sarama"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/asyncworkflow/queue/provider"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
)

type MockDecoder struct {
	DecodeFunc func(v any) error
}

func (m *MockDecoder) Decode(v any) error {
	return m.DecodeFunc(v)
}

func TestNewQueue(t *testing.T) {
	tests := []struct {
		name      string
		decoder   *MockDecoder
		want      *queueImpl
		wantErr   bool
		errString string
	}{
		{
			name: "successful decoding",
			decoder: &MockDecoder{
				DecodeFunc: func(v any) error {
					out := v.(*queueConfig)
					out.Connection.Brokers = []string{"broker2", "broker1"}
					return nil
				},
			},
			want: &queueImpl{
				config: &queueConfig{
					Connection: connectionConfig{Brokers: []string{"broker1", "broker2"}},
				},
			},
			wantErr: false,
		},
		{
			name: "decoding failure",
			decoder: &MockDecoder{
				DecodeFunc: func(v any) error {
					return errors.New("decoding error")
				},
			},
			want:      nil,
			wantErr:   true,
			errString: "bad config: decoding error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newQueue(tt.decoder)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestCreateConsumer(t *testing.T) {
	testCases := []struct {
		name    string
		config  *queueConfig
		wantErr bool
	}{
		{
			name: "Success case",
			config: &queueConfig{
				Connection: connectionConfig{
					Brokers: []string{"localhost:9092"},
					TLS: config.TLS{
						Enabled: false,
					},
					SASL: config.SASL{
						Enabled: false,
					},
				},
				Topic: "test-topic",
			},
		},
		{
			name: "Invalid SASL config",
			config: &queueConfig{
				Connection: connectionConfig{
					Brokers: []string{"localhost:9092"},
					TLS: config.TLS{
						Enabled: false,
					},
					SASL: config.SASL{
						Enabled:   true,
						Algorithm: "test",
					},
				},
				Topic: "test-topic",
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockBroker := sarama.NewMockBroker(t, 0)
			defer mockBroker.Close()
			mockBroker.SetHandlerByMap(map[string]sarama.MockResponse{
				"MetadataRequest": sarama.NewMockMetadataResponse(t).
					SetBroker(mockBroker.Addr(), mockBroker.BrokerID()).
					SetLeader("test-topic", 0, mockBroker.BrokerID()).
					SetController(mockBroker.BrokerID()),
			})
			tc.config.Connection.Brokers = []string{mockBroker.Addr()}

			q := &queueImpl{
				config: tc.config,
			}
			p := &provider.Params{
				Logger:        testlogger.New(t),
				MetricsClient: metrics.NewNoopMetricsClient(),
			}
			consumer, err := q.CreateConsumer(p)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, consumer)
			}
		})
	}
}

func TestCreateProducer(t *testing.T) {
	testCases := []struct {
		name    string
		config  *queueConfig
		wantErr bool
	}{
		{
			name: "Success case",
			config: &queueConfig{
				Connection: connectionConfig{
					Brokers: []string{"localhost:9092"},
					TLS: config.TLS{
						Enabled: false,
					},
					SASL: config.SASL{
						Enabled: false,
					},
				},
				Topic: "test-topic",
			},
		},
		{
			name: "Invalid SASL config",
			config: &queueConfig{
				Connection: connectionConfig{
					Brokers: []string{"localhost:9092"},
					TLS: config.TLS{
						Enabled: false,
					},
					SASL: config.SASL{
						Enabled:   true,
						Algorithm: "test",
					},
				},
				Topic: "test-topic",
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockBroker := sarama.NewMockBroker(t, 0)
			defer mockBroker.Close()
			mockBroker.SetHandlerByMap(map[string]sarama.MockResponse{
				"MetadataRequest": sarama.NewMockMetadataResponse(t).
					SetBroker(mockBroker.Addr(), mockBroker.BrokerID()).
					SetLeader("test-topic", 0, mockBroker.BrokerID()).
					SetController(mockBroker.BrokerID()),
			})
			tc.config.Connection.Brokers = []string{mockBroker.Addr()}

			q := &queueImpl{
				config: tc.config,
			}
			p := &provider.Params{
				Logger:        testlogger.New(t),
				MetricsClient: metrics.NewNoopMetricsClient(),
			}
			consumer, err := q.CreateProducer(p)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, consumer)
			}
		})
	}
}
