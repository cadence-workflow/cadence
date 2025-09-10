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
	"fmt"
	"sort"
	"time"

	"github.com/Shopify/sarama"

	"github.com/uber/cadence/common/asyncworkflow/queue/consumer"
	"github.com/uber/cadence/common/asyncworkflow/queue/provider"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/messaging/kafka"
	"github.com/uber/cadence/common/metrics"
)

type (
	queueImpl struct {
		config *queueConfig
	}
)

func newQueue(decoder provider.Decoder) (provider.Queue, error) {
	var out queueConfig
	if err := decoder.Decode(&out); err != nil {
		return nil, fmt.Errorf("bad config: %w", err)
	}
	sort.Strings(out.Connection.Brokers)
	return &queueImpl{
		config: &out,
	}, nil
}

func (q *queueImpl) ID() string {
	return q.config.ID()
}

func (q *queueImpl) CreateConsumer(p *provider.Params) (provider.Consumer, error) {
	consumerGroup := fmt.Sprintf("%s-asyncwf-consumer", q.config.Topic)
	dlqTopic := fmt.Sprintf("%s-dlq", q.config.Topic)
	dlqConfig, err := newSaramaConfigWithAuth(&q.config.Connection.TLS, &q.config.Connection.SASL)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka sarama config: %w", err)
	}
	dlqConfig.Producer.Return.Successes = true
	dlqProducer, err := newProducer(dlqTopic, q.config.Connection.Brokers, dlqConfig, p.MetricsClient, p.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka producer for dlq: %w", err)
	}

	consumerConfig, err := newSaramaConfigWithAuth(&q.config.Connection.TLS, &q.config.Connection.SASL)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka sarama config: %w", err)
	}

	consumerConfig.Consumer.Fetch.Default = 30 * 1024 * 1024 // 30MB.
	consumerConfig.Consumer.Return.Errors = true
	consumerConfig.Consumer.Offsets.AutoCommit.Enable = false // Use manual commit
	consumerConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
	consumerConfig.Consumer.MaxProcessingTime = 250 * time.Millisecond
	kafkaConsumer, err := kafka.NewKafkaConsumer(dlqProducer, q.config.Connection.Brokers, q.config.Topic, consumerGroup, consumerConfig, p.MetricsClient, p.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka consumer: %w", err)
	}
	p.Logger.Info("Creating async wf consumer", tag.KafkaTopicName(q.config.Topic))
	return consumer.New(q.ID(), kafkaConsumer, p.Logger, p.MetricsClient, p.FrontendClient), nil
}

func (q *queueImpl) CreateProducer(p *provider.Params) (messaging.Producer, error) {
	config, err := newSaramaConfigWithAuth(&q.config.Connection.TLS, &q.config.Connection.SASL)
	if err != nil {
		return nil, err
	}
	config.Producer.Return.Successes = true
	p.Logger.Info("Creating async wf producer", tag.KafkaTopicName(q.config.Topic))
	return newProducer(q.config.Topic, q.config.Connection.Brokers, config, p.MetricsClient, p.Logger)
}

func newProducer(topic string, brokers []string, saramaConfig *sarama.Config, metricsClient metrics.Client, logger log.Logger) (messaging.Producer, error) {
	p, err := sarama.NewSyncProducer(brokers, saramaConfig)
	if err != nil {
		return nil, err
	}

	withMetricsOpt := messaging.WithMetricTags(metrics.TopicTag(topic))
	return messaging.NewMetricProducer(kafka.NewKafkaProducer(topic, p, logger), metricsClient, withMetricsOpt), nil
}
