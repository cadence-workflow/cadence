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

package indexer

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/.gen/go/indexer"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/elasticsearch/bulk"
	mocks2 "github.com/uber/cadence/common/elasticsearch/bulk/mocks"
	esMocks "github.com/uber/cadence/common/elasticsearch/mocks"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
)

func TestNewDualIndexer(t *testing.T) {
	ctrl := gomock.NewController(t)

	config := &Config{
		ESProcessorNumOfWorkers:  dynamicconfig.GetIntPropertyFn(1),
		ESProcessorBulkActions:   dynamicconfig.GetIntPropertyFn(10),
		ESProcessorBulkSize:      dynamicconfig.GetIntPropertyFn(2 << 20),
		ESProcessorFlushInterval: dynamicconfig.GetDurationPropertyFn(1 * time.Minute),
	}
	processorName := "test-bulkProcessor"
	consumerName := "test-bulkProcessor-os-consumer"
	mockESClient := &esMocks.GenericClient{}
	mockESClient.On("RunBulkProcessor", mock.Anything, mock.MatchedBy(func(input *bulk.BulkProcessorParameters) bool {
		return true
	})).Return(&mocks2.GenericBulkProcessor{}, nil).Times(2)

	mockMessagingClient := messaging.NewMockClient(ctrl)
	mockMessagingClient.EXPECT().NewConsumer("visibility", "test-bulkProcessor-consumer").Return(nil, nil).Times(1)
	mockMessagingClient.EXPECT().NewConsumer("visibility", "test-bulkProcessor-os-consumer").Return(nil, nil).Times(1)

	indexer := NewMigrationDualIndexer(config, mockMessagingClient, mockESClient, mockESClient, processorName, processorName, "", consumerName, testlogger.New(t), metrics.NewNoopMetricsClient())
	assert.NotNil(t, indexer)
}

func TestNewIndexer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := &Config{
		ESProcessorNumOfWorkers:  dynamicconfig.GetIntPropertyFn(1),
		ESProcessorBulkActions:   dynamicconfig.GetIntPropertyFn(10),
		ESProcessorBulkSize:      dynamicconfig.GetIntPropertyFn(2 << 20),
		ESProcessorFlushInterval: dynamicconfig.GetDurationPropertyFn(1 * time.Minute),
	}
	processorName := "test-bulkProcessor"
	mockESClient := &esMocks.GenericClient{}
	mockESClient.On("RunBulkProcessor", mock.Anything, mock.MatchedBy(func(input *bulk.BulkProcessorParameters) bool {
		return true
	})).Return(&mocks2.GenericBulkProcessor{}, nil).Times(2)

	mockMessagingClient := messaging.NewMockClient(ctrl)
	mockMessagingClient.EXPECT().NewConsumer("visibility", "test-bulkProcessor-consumer").Return(nil, nil).Times(1)

	indexer := NewIndexer(config, mockMessagingClient, mockESClient, processorName, "", testlogger.New(t), metrics.NewNoopMetricsClient())
	assert.NotNil(t, indexer)
}

// TestIndexerStart tests the Start method of Indexer
func TestIndexerStart(t *testing.T) {
	ctrl := gomock.NewController(t)

	config := &Config{
		ESProcessorNumOfWorkers:  dynamicconfig.GetIntPropertyFn(1),
		ESProcessorBulkActions:   dynamicconfig.GetIntPropertyFn(10),
		ESProcessorBulkSize:      dynamicconfig.GetIntPropertyFn(2 << 20),
		ESProcessorFlushInterval: dynamicconfig.GetDurationPropertyFn(1 * time.Minute),
		IndexerConcurrency:       dynamicconfig.GetIntPropertyFn(1),
	}
	mockConsumer := messaging.NewMockConsumer(ctrl)
	mockConsumer.EXPECT().Start().Return(nil).Times(1)
	messageChan := make(chan messaging.Message)
	mockConsumer.EXPECT().Messages().Return((<-chan messaging.Message)(messageChan)).Times(1)
	mockConsumer.EXPECT().Stop().Return().Times(1)
	mockvisibiltyProcessor := NewMockESProcessor(ctrl)
	mockvisibiltyProcessor.EXPECT().Start().Return().Times(1)
	mockvisibiltyProcessor.EXPECT().Stop().Return().Times(1)

	indexer := &Indexer{
		config:              config,
		esIndexName:         "test-index",
		consumer:            mockConsumer,
		logger:              log.NewNoop(),
		scope:               metrics.NoopScope(metrics.IndexProcessorScope),
		shutdownCh:          make(chan struct{}),
		visibilityProcessor: mockvisibiltyProcessor,
		msgEncoder:          defaultEncoder,
	}
	err := indexer.Start()
	assert.NoError(t, err)
	close(messageChan)

	indexer.Stop()
	defer goleak.VerifyNone(t)
}

// TestIndexerStart_ConsumerError tests the Start method when consumer.Start returns an error
func TestIndexerStart_ConsumerError(t *testing.T) {
	ctrl := gomock.NewController(t)

	config := &Config{
		ESProcessorNumOfWorkers:  dynamicconfig.GetIntPropertyFn(1),
		ESProcessorBulkActions:   dynamicconfig.GetIntPropertyFn(10),
		ESProcessorBulkSize:      dynamicconfig.GetIntPropertyFn(2 << 20),
		ESProcessorFlushInterval: dynamicconfig.GetDurationPropertyFn(1 * time.Minute),
		IndexerConcurrency:       dynamicconfig.GetIntPropertyFn(1),
	}
	mockConsumer := messaging.NewMockConsumer(ctrl)
	mockConsumer.EXPECT().Start().Return(fmt.Errorf("some error")).Times(1)
	mockvisibiltyProcessor := NewMockESProcessor(ctrl)

	indexer := &Indexer{
		config:              config,
		esIndexName:         "test-index",
		consumer:            mockConsumer,
		logger:              log.NewNoop(),
		scope:               metrics.NoopScope(metrics.IndexProcessorScope),
		shutdownCh:          make(chan struct{}),
		visibilityProcessor: mockvisibiltyProcessor,
		msgEncoder:          defaultEncoder,
	}
	err := indexer.Start()
	assert.ErrorContains(t, err, "some error")

}

func TestIndexerStop(t *testing.T) {
	ctrl := gomock.NewController(t)

	config := &Config{
		ESProcessorNumOfWorkers:  dynamicconfig.GetIntPropertyFn(1),
		ESProcessorBulkActions:   dynamicconfig.GetIntPropertyFn(10),
		ESProcessorBulkSize:      dynamicconfig.GetIntPropertyFn(2 << 20),
		ESProcessorFlushInterval: dynamicconfig.GetDurationPropertyFn(1 * time.Minute),
		IndexerConcurrency:       dynamicconfig.GetIntPropertyFn(1),
	}

	// Mock the messaging consumer
	mockConsumer := messaging.NewMockConsumer(ctrl)
	messageChan := make(chan messaging.Message)
	mockConsumer.EXPECT().Messages().Return((<-chan messaging.Message)(messageChan)).AnyTimes()
	// No specific expectations for Start or Stop since they're not called in Stop()

	// Mock the visibility processor
	mockVisibilityProcessor := NewMockESProcessor(ctrl)
	// Create the Indexer instance with mocks
	indexer := &Indexer{
		config:              config,
		esIndexName:         "test-index",
		consumer:            mockConsumer,
		logger:              log.NewNoop(),
		scope:               metrics.NoopScope(metrics.IndexProcessorScope),
		shutdownCh:          make(chan struct{}),
		visibilityProcessor: mockVisibilityProcessor,
		msgEncoder:          defaultEncoder,
	}

	// Simulate that the indexer was started
	atomic.StoreInt32(&indexer.isStarted, 1)

	// Simulate the processorPump goroutine that waits on shutdownCh
	indexer.shutdownWG.Add(1)
	go func() {
		defer indexer.shutdownWG.Done()
		<-indexer.shutdownCh
	}()

	// Call Stop and verify behavior
	indexer.Stop()

	// Verify that shutdownCh is closed
	select {
	case <-indexer.shutdownCh:
		// Expected: shutdownCh should be closed
	default:
		t.Error("shutdownCh is not closed")
	}

	// Verify that the WaitGroup has completed
	success := common.AwaitWaitGroup(&indexer.shutdownWG, time.Second)
	assert.True(t, success)

	// Verify that isStopped flag is set
	assert.Equal(t, int32(1), atomic.LoadInt32(&indexer.isStopped))

	// Call Stop again to ensure idempotency
	indexer.Stop()
	defer goleak.VerifyNone(t)
}

func TestIsValidFieldToES(t *testing.T) {
	tests := map[string]struct {
		testIndexer *Indexer
		field       string
		expectedRes bool
	}{
		"not EnableQueryAttributeValidation": {
			testIndexer: &Indexer{
				config: &Config{
					EnableQueryAttributeValidation: dynamicconfig.GetBoolPropertyFn(false),
				},
			},
			field:       "someField",
			expectedRes: true,
		},
		"field is valid": {
			testIndexer: &Indexer{
				config: &Config{
					EnableQueryAttributeValidation: dynamicconfig.GetBoolPropertyFn(true),
					ValidSearchAttributes:          dynamicconfig.GetMapPropertyFn(map[string]interface{}{"someField": "ok"}),
				},
			},
			field:       "someField",
			expectedRes: true,
		},
		"field is not valid, but meet definition": {
			testIndexer: &Indexer{
				config: &Config{
					EnableQueryAttributeValidation: dynamicconfig.GetBoolPropertyFn(true),
					ValidSearchAttributes:          dynamicconfig.GetMapPropertyFn(map[string]interface{}{}),
				},
			},
			field:       definition.Memo,
			expectedRes: true,
		},
		"false": {
			testIndexer: &Indexer{
				config: &Config{
					EnableQueryAttributeValidation: dynamicconfig.GetBoolPropertyFn(true),
					ValidSearchAttributes:          dynamicconfig.GetMapPropertyFn(map[string]interface{}{}),
				},
			},
			field:       "stuff",
			expectedRes: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			res := tc.testIndexer.isValidFieldToES(tc.field)
			assert.Equal(t, tc.expectedRes, res)
		})
	}
}

func TestFulfillDoc_AllFieldsPresent(t *testing.T) {
	domainID := "domain1"
	workflowID := "workflow1"
	runID := "run1"
	keyToKafkaMsg := "kafka-key-1"

	doc := map[string]interface{}{}
	msg := &indexer.Message{
		DomainID:   &domainID,
		WorkflowID: &workflowID,
		RunID:      &runID,
	}

	expectedDoc := map[string]interface{}{
		definition.DomainID:   domainID,
		definition.WorkflowID: workflowID,
		definition.RunID:      runID,
		definition.KafkaKey:   keyToKafkaMsg,
	}

	fulfillDoc(doc, msg, keyToKafkaMsg)
	assert.Equal(t, expectedDoc, doc, "fulfillDoc() result mismatch")
}

func TestDumpFieldsToMap(t *testing.T) {
	testIndexer := &Indexer{
		config: &Config{
			EnableQueryAttributeValidation: dynamicconfig.GetBoolPropertyFn(true),
			ValidSearchAttributes:          dynamicconfig.GetMapPropertyFn(map[string]interface{}{}),
		},
		logger: log.NewNoop(),
		scope:  metrics.NoopScope(metrics.Worker),
	}

	stringPtr := "string"

	tests := map[string]struct {
		fields   map[string]*indexer.Field
		expected map[string]interface{}
	}{
		"empty fields": {
			fields: map[string]*indexer.Field{},
			expected: map[string]interface{}{
				"Attr": map[string]interface{}{},
			},
		},
		"different fields": {
			fields: map[string]*indexer.Field{
				"invalid": {
					StringData: &stringPtr,
				},
			},
			expected: map[string]interface{}{
				"Attr": map[string]interface{}{},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			res := testIndexer.dumpFieldsToMap(tc.fields, "id")
			assert.Equal(t, tc.expected, res)
		})
	}
}
