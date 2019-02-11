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

package messaging

import (
	"strings"

	"github.com/Shopify/sarama"
	"github.com/uber-common/bark"
	uberKafkaClient "github.com/uber-go/kafka-client"
	uberKafka "github.com/uber-go/kafka-client/kafka"
	"github.com/uber-go/tally"
	"github.com/uber/cadence/common/metrics"
	"go.uber.org/zap"
)

type (
	// This is a default implementation of Client interface which makes use of uber-go/kafka-client as consumer
	kafkaClient struct {
		config        *KafkaConfig
		client        uberKafkaClient.Client
		metricsClient metrics.Client
		logger        bark.Logger
	}
)

var _ Client = (*kafkaClient)(nil)

// NewKafkaClient is used to create an instance of KafkaClient
func NewKafkaClient(kc *KafkaConfig, metricsClient metrics.Client, zLogger *zap.Logger, logger bark.Logger, metricScope tally.Scope,
	checkCluster, checkApp bool) Client {
	kc.Validate(checkCluster, checkApp)

	// mapping from cluster name to list of broker ip addresses
	brokers := map[string][]string{}
	for cluster, cfg := range kc.Clusters {
		brokers[cluster] = cfg.Brokers
		for i := range brokers[cluster] {
			if !strings.Contains(cfg.Brokers[i], ":") {
				cfg.Brokers[i] += ":9092"
			}
		}
	}

	// mapping from topic name to cluster that has that topic
	topicClusterAssignment := map[string][]string{}
	for topic, cfg := range kc.Topics {
		topicClusterAssignment[topic] = []string{cfg.Cluster}
	}

	client := uberKafkaClient.New(uberKafka.NewStaticNameResolver(topicClusterAssignment, brokers), zLogger, metricScope)

	return &kafkaClient{
		config:        kc,
		client:        client,
		metricsClient: metricsClient,
		logger:        logger,
	}
}

// NewConsumer is used to create a Kafka consumer
func (c *kafkaClient) NewConsumer(app, consumerName string, concurrency int) (Consumer, error) {
	topics := c.config.getTopicsForApplication(app)
	kafkaClusterNameForTopic := c.config.getKafkaClusterForTopic(topics.Topic)
	kafkaClusterNameForDLQTopic := c.config.getKafkaClusterForTopic(topics.DLQTopic)

	topic := createUberKafkaTopic(topics.Topic, kafkaClusterNameForTopic)
	dlq := createUberKafkaTopic(topics.DLQTopic, kafkaClusterNameForDLQTopic)

	return c.newConsumerHelper(topic, dlq, consumerName, concurrency)
}

// NewConsumerWithClusterName is used to create a Kafka consumer for consuming replication tasks
func (c *kafkaClient) NewConsumerWithClusterName(currentCluster, sourceCluster, consumerName string, concurrency int) (Consumer, error) {
	currentTopics := c.config.getTopicsForCadenceCluster(currentCluster)
	sourceTopics := c.config.getTopicsForCadenceCluster(sourceCluster)
	kafkaClusterNameForTopic := c.config.getKafkaClusterForTopic(sourceTopics.Topic)
	kafkaClusterNameForDLQTopic := c.config.getKafkaClusterForTopic(currentTopics.DLQTopic)

	topic := createUberKafkaTopic(sourceTopics.Topic, kafkaClusterNameForTopic)
	dlq := createUberKafkaTopic(currentTopics.DLQTopic, kafkaClusterNameForDLQTopic)

	return c.newConsumerHelper(topic, dlq, consumerName, concurrency)
}

func createUberKafkaTopic(name, cluster string) *uberKafka.Topic {
	return &uberKafka.Topic{
		Name:    name,
		Cluster: cluster,
	}
}

func (c *kafkaClient) newConsumerHelper(topic, dlq *uberKafka.Topic, consumerName string, concurrency int) (Consumer, error) {
	topicList := uberKafka.ConsumerTopicList{
		uberKafka.ConsumerTopic{
			Topic: *topic,
			DLQ:   *dlq,
		},
	}
	consumerConfig := uberKafka.NewConsumerConfig(consumerName, topicList)
	consumerConfig.Concurrency = concurrency
	consumerConfig.Offsets.Initial.Offset = uberKafka.OffsetOldest

	uConsumer, err := c.client.NewConsumer(consumerConfig)
	if err != nil {
		return nil, err
	}
	return newKafkaConsumer(uConsumer, c.logger), nil
}

// NewProducer is used to create a Kafka producer
func (c *kafkaClient) NewProducer(app string) (Producer, error) {
	topics := c.config.getTopicsForApplication(app)
	return c.newProducerHelper(topics.Topic)
}

// NewProducerWithClusterName is used to create a Kafka producer for shipping replication tasks
func (c *kafkaClient) NewProducerWithClusterName(sourceCluster string) (Producer, error) {
	topics := c.config.getTopicsForCadenceCluster(sourceCluster)
	return c.newProducerHelper(topics.Topic)
}

func (c *kafkaClient) newProducerHelper(topic string) (Producer, error) {
	kafkaClusterName := c.config.getKafkaClusterForTopic(topic)
	brokers := c.config.getBrokersForKafkaCluster(kafkaClusterName)

	producer, err := sarama.NewSyncProducer(brokers, nil)
	if err != nil {
		return nil, err
	}

	if c.metricsClient != nil {
		c.logger.Info("Create producer with metricsClient")
		return NewMetricProducer(NewKafkaProducer(topic, producer, c.logger), c.metricsClient), nil
	}
	return NewKafkaProducer(topic, producer, c.logger), nil
}
