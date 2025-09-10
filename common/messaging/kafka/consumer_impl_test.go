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
	"context"
	"testing"

	"github.com/Shopify/sarama"
	"github.com/Shopify/sarama/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/uber-go/tally"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
)

func TestNewConsumer(t *testing.T) {
	mockProducer := mocks.NewSyncProducer(t, nil)
	group := "tests"
	mockBroker := initMockBroker(t, group)
	defer mockBroker.Close()
	brokerAddr := []string{mockBroker.Addr()}
	kafkaConfig := &config.KafkaConfig{
		Clusters: map[string]config.ClusterConfig{
			"test-cluster": {
				Brokers: brokerAddr,
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
	}
	topic := "test-topic"
	consumerName := "test-consumer"
	metricsClient := metrics.NewClient(tally.NoopScope, metrics.History)
	logger := testlogger.New(t)
	kafkaProducer := NewKafkaProducer(topic, mockProducer, logger)
	clusterName := kafkaConfig.GetKafkaClusterForTopic(topic)
	brokers := kafkaConfig.GetBrokersForKafkaCluster(clusterName)
	consumer, err := NewKafkaConsumer(kafkaProducer, brokers, topic, consumerName,
		nil, metricsClient, logger)
	assert.NoError(t, err, "An error was not expected but got %v", err)
	assert.NotNil(t, consumer, "Expected consumer but got nil")

	err = consumer.Start()
	assert.NoError(t, err)

	consumer.Stop()
}

func TestNewConsumerHandlerImpl(t *testing.T) {
	topic := "test-topic"
	metricsClient := metrics.NewClient(tally.NoopScope, metrics.History)
	logger := testlogger.New(t)
	mockProducer := mocks.NewSyncProducer(t, nil)
	kafkaProducer := NewKafkaProducer(topic, mockProducer, logger)
	msgChan := make(chan messaging.Message, 1)
	consumerHandler := newConsumerHandlerImpl(kafkaProducer, topic, msgChan, metricsClient, logger)

	assert.NotNil(t, consumerHandler)
	assert.Equal(t, kafkaProducer, consumerHandler.dlqProducer)
	assert.Equal(t, topic, consumerHandler.topic)
	// Close the channel at the end of the test
	close(msgChan)
}

// test multiple methods related to messageImpl since setup is repeated
func TestMessageImpl(t *testing.T) {
	topic := "test-topic"
	metricsClient := metrics.NewClient(tally.NoopScope, metrics.History)
	logger := testlogger.New(t)
	mockProducer := mocks.NewSyncProducer(t, nil)
	kafkaProducer := NewKafkaProducer(topic, mockProducer, logger)
	msgChan := make(chan messaging.Message, 1)
	consumerHandler := newConsumerHandlerImpl(kafkaProducer, topic, msgChan, metricsClient, logger)
	consumerGroupSession := NewMockConsumerGroupSession(int32(1))
	partition := int32(100)
	offset := int64(0)
	msgImpl := &messageImpl{
		saramaMsg: &sarama.ConsumerMessage{
			Topic:     topic,
			Partition: partition,
			Offset:    offset,
		},
		session: consumerGroupSession,
		handler: consumerHandler,
		logger:  logger,
	}

	// Ack message that is from a previous session
	err := msgImpl.handler.Setup(NewMockConsumerGroupSession(int32(2)))
	assert.NoError(t, err)
	err = msgImpl.Ack()
	assert.NoError(t, err)

	// normal case
	err = msgImpl.handler.Setup(NewMockConsumerGroupSession(int32(1)))
	assert.NoError(t, err)
	msgImpl.handler.manager.AddMessage(partition, offset)
	err = msgImpl.Ack()
	assert.NoError(t, err)

	// Nack message that is from a previous session
	err = msgImpl.handler.Setup(NewMockConsumerGroupSession(int32(2)))
	assert.NoError(t, err)
	err = msgImpl.Nack()
	assert.NoError(t, err)

	// normal case
	err = msgImpl.handler.Setup(NewMockConsumerGroupSession(int32(1)))
	assert.NoError(t, err)
	mockProducer.ExpectSendMessageAndSucceed()
	msgImpl.handler.manager.AddMessage(partition, offset)
	err = msgImpl.Nack()
	assert.NoError(t, err)

	close(msgChan)
}

// MockConsumerGroupSession implements sarama.ConsumerGroupSession for testing purposes.
type MockConsumerGroupSession struct {
	claims       map[string][]int32
	memberID     string
	generationID int32
	offsets      map[string]map[int32]int64
	commitCalled bool
	context      context.Context
}

func NewMockConsumerGroupSession(generationID int32) *MockConsumerGroupSession {
	return &MockConsumerGroupSession{
		claims:       map[string][]int32{},
		memberID:     "test-member",
		generationID: generationID,
		offsets:      map[string]map[int32]int64{},
		commitCalled: false,
	}
}

func (m *MockConsumerGroupSession) Claims() map[string][]int32 {
	return m.claims
}

func (m *MockConsumerGroupSession) MemberID() string {
	return m.memberID
}

func (m *MockConsumerGroupSession) GenerationID() int32 {
	return m.generationID
}

func (m *MockConsumerGroupSession) MarkOffset(topic string, partition int32, offset int64, metadata string) {
	// not needed for testing
}

func (m *MockConsumerGroupSession) Commit() {
	// not needed for testing
}

func (m *MockConsumerGroupSession) ResetOffset(topic string, partition int32, offset int64, metadata string) {
	// not needed for testing
}

func (m *MockConsumerGroupSession) MarkMessage(msg *sarama.ConsumerMessage, metadata string) {
	// not needed for testing
}

func (m *MockConsumerGroupSession) Context() context.Context {
	// Return a context, you can use context.Background() or a custom context if needed
	return m.context
}

func initMockBroker(t *testing.T, group string) *sarama.MockBroker {
	topics := []string{"test-topic"}
	mockBroker := sarama.NewMockBroker(t, 0)

	mockBroker.SetHandlerByMap(map[string]sarama.MockResponse{
		"MetadataRequest": sarama.NewMockMetadataResponse(t).
			SetBroker(mockBroker.Addr(), mockBroker.BrokerID()).
			SetLeader(topics[0], 0, mockBroker.BrokerID()).
			SetController(mockBroker.BrokerID()),
		"FindCoordinatorRequest": sarama.NewMockFindCoordinatorResponse(t).
			SetCoordinator(sarama.CoordinatorGroup, group, mockBroker),
	})
	return mockBroker
}
