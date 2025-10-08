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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/uber-go/tally"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
)

func TestNewKafkaClient(t *testing.T) {
	metricsClient := metrics.NewClient(tally.NoopScope, metrics.History, metrics.HistogramMigration{})
	logger := testlogger.New(t)
	testCases := []struct {
		name        string
		config      *config.KafkaConfig
		checkApp    bool
		expectedErr string
	}{
		{
			name: "Missing clusters",
			config: &config.KafkaConfig{
				Clusters: map[string]config.ClusterConfig{},
			},
			checkApp:    true,
			expectedErr: "Empty Kafka Cluster Config",
		},
		{
			name: "Missing topics",
			config: &config.KafkaConfig{
				Clusters: map[string]config.ClusterConfig{
					"testCluster": {
						Brokers: []string{"testBrokers"},
					},
				},
				Topics: map[string]config.TopicConfig{},
			},
			checkApp:    true,
			expectedErr: "Empty Topics Config",
		},
		{
			name: "Missing Applications",
			config: &config.KafkaConfig{
				Clusters: map[string]config.ClusterConfig{
					"test-cluster": {
						Brokers: []string{"test-brokers"},
					},
				},
				Topics: map[string]config.TopicConfig{
					"test-topic": {
						Cluster: "test-cluster",
					},
				},
				Applications: map[string]config.TopicList{},
			},
			checkApp:    true,
			expectedErr: "Empty Applications Config",
		},
		{
			name: "Missing topics config",
			config: &config.KafkaConfig{
				Clusters: map[string]config.ClusterConfig{
					"test-cluster": {
						Brokers: []string{"test-brokers"},
					},
				},
				Topics: map[string]config.TopicConfig{
					"test-topic": {
						Cluster: "test-cluster",
					},
				},
				Applications: map[string]config.TopicList{
					"test-app": {
						Topic:    "test-topic",
						DLQTopic: "test-topic-dlq",
					},
				},
			},
			checkApp:    true,
			expectedErr: "Missing Topic Config for Topic test-topic-dlq",
		},
		{
			name: "Normal Case",
			config: &config.KafkaConfig{
				Clusters: map[string]config.ClusterConfig{
					"test-cluster": {
						Brokers: []string{"test-brokers"},
					},
				},
				Topics: map[string]config.TopicConfig{
					"test-topic": {
						Cluster: "test-cluster",
					},
					"test-topic-dlq": {
						Cluster: "test-cluster",
					},
				},
				Applications: map[string]config.TopicList{
					"test-app": {
						Topic:    "test-topic",
						DLQTopic: "test-topic-dlq",
					},
				},
			},
			checkApp: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					assert.Equal(t, tc.expectedErr, r)
				}
			}()
			kafkaClient := NewKafkaClient(tc.config, metricsClient, logger, nil, tc.checkApp)
			// Type assert to *clientImpl to access struct fields
			client, ok := kafkaClient.(*clientImpl)
			assert.True(t, ok, "Expected kafkaClient to be of type *clientImpl")
			assert.Equal(t, tc.config, client.config)
		})
	}
}
