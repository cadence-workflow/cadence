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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/uber/cadence/.gen/go/indexer"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/collection"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/elasticsearch"
	"github.com/uber/cadence/common/elasticsearch/bulk"
	mocks2 "github.com/uber/cadence/common/elasticsearch/bulk/mocks"
	esMocks "github.com/uber/cadence/common/elasticsearch/mocks"
	"github.com/uber/cadence/common/log/testlogger"
	msgMocks "github.com/uber/cadence/common/messaging/mocks"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/metrics/mocks"
)

type esProcessorSuite struct {
	suite.Suite
	esProcessor       *ESProcessorImpl
	mockBulkProcessor *mocks2.GenericBulkProcessor
	mockESClient      *esMocks.GenericClient
	mockScope         *mocks.Scope
}

var (
	testIndex     = "test-index"
	testType      = elasticsearch.GetESDocType()
	testID        = "test-doc-id"
	testStopWatch = metrics.NoopScope(metrics.ESProcessorScope).StartTimer(metrics.ESProcessorProcessMsgLatency)
	testScope     = metrics.ESProcessorScope
	testMetric    = metrics.ESProcessorProcessMsgLatency
)

func TestESProcessorSuite(t *testing.T) {
	s := new(esProcessorSuite)
	suite.Run(t, s)
}

func (s *esProcessorSuite) SetupSuite() {
}

func (s *esProcessorSuite) SetupTest() {
	config := &Config{
		IndexerConcurrency:       dynamicconfig.GetIntPropertyFn(32),
		ESProcessorNumOfWorkers:  dynamicconfig.GetIntPropertyFn(1),
		ESProcessorBulkActions:   dynamicconfig.GetIntPropertyFn(10),
		ESProcessorBulkSize:      dynamicconfig.GetIntPropertyFn(2 << 20),
		ESProcessorFlushInterval: dynamicconfig.GetDurationPropertyFn(1 * time.Minute),
	}
	s.mockBulkProcessor = &mocks2.GenericBulkProcessor{}
	s.mockScope = &mocks.Scope{}

	p := &ESProcessorImpl{
		config:     config,
		logger:     testlogger.New(s.T()),
		scope:      s.mockScope,
		msgEncoder: defaultEncoder,
	}
	p.mapToKafkaMsg = collection.NewShardedConcurrentTxMap(1024, p.hashFn)
	p.bulkProcessor = s.mockBulkProcessor

	s.esProcessor = p

	s.mockESClient = &esMocks.GenericClient{}
}

func (s *esProcessorSuite) TearDownTest() {
	s.mockBulkProcessor.AssertExpectations(s.T())
	s.mockESClient.AssertExpectations(s.T())
}

func (s *esProcessorSuite) TestNewESProcessorAndStart() {
	config := &Config{
		ESProcessorNumOfWorkers:  dynamicconfig.GetIntPropertyFn(1),
		ESProcessorBulkActions:   dynamicconfig.GetIntPropertyFn(10),
		ESProcessorBulkSize:      dynamicconfig.GetIntPropertyFn(2 << 20),
		ESProcessorFlushInterval: dynamicconfig.GetDurationPropertyFn(1 * time.Minute),
	}
	processorName := "test-bulkProcessor"

	s.mockESClient.On("RunBulkProcessor", mock.Anything, mock.MatchedBy(func(input *bulk.BulkProcessorParameters) bool {
		s.Equal(processorName, input.Name)
		s.Equal(config.ESProcessorNumOfWorkers(), input.NumOfWorkers)
		s.Equal(config.ESProcessorBulkActions(), input.BulkActions)
		s.Equal(config.ESProcessorBulkSize(), input.BulkSize)
		s.Equal(config.ESProcessorFlushInterval(), input.FlushInterval)
		s.NotNil(input.Backoff)
		s.NotNil(input.AfterFunc)
		return true
	})).Return(&mocks2.GenericBulkProcessor{}, nil).Once()
	processor, err := newESProcessor(processorName, config, s.mockESClient, s.esProcessor.logger, metrics.NewNoopMetricsClient())
	s.NoError(err)

	s.NotNil(processor.mapToKafkaMsg)
}

func (s *esProcessorSuite) TestStop() {
	s.mockBulkProcessor.On("Stop").Return(nil).Once()
	s.esProcessor.Stop()
	s.Nil(s.esProcessor.mapToKafkaMsg)
}

func (s *esProcessorSuite) TestAdd() {
	request := &bulk.GenericBulkableAddRequest{RequestType: bulk.BulkableIndexRequest}
	mockKafkaMsg := &msgMocks.Message{}
	key := "test-key"
	s.Equal(0, s.esProcessor.mapToKafkaMsg.Len())

	s.mockBulkProcessor.On("Add", request).Return().Once()
	s.mockScope.On("StartTimer", testMetric).Return(testStopWatch).Once()
	s.esProcessor.Add(request, key, mockKafkaMsg)
	s.Equal(1, s.esProcessor.mapToKafkaMsg.Len())
	mockKafkaMsg.AssertExpectations(s.T())

	// handle duplicate
	mockKafkaMsg.On("Ack").Return(nil).Once()
	s.mockScope.On("StartTimer", testMetric).Return(testStopWatch).Once()
	s.esProcessor.Add(request, key, mockKafkaMsg)
	s.Equal(1, s.esProcessor.mapToKafkaMsg.Len())
	mockKafkaMsg.AssertExpectations(s.T())
}

func (s *esProcessorSuite) TestAdd_ConcurrentAdd() {
	request := &bulk.GenericBulkableAddRequest{RequestType: bulk.BulkableIndexRequest}
	mockKafkaMsg := &msgMocks.Message{}
	key := "test-key"

	addFunc := func(wg *sync.WaitGroup) {
		s.mockScope.On("StartTimer", testMetric).Return(testStopWatch).Once()
		s.esProcessor.Add(request, key, mockKafkaMsg)
		wg.Done()
	}
	duplicates := 5
	wg := &sync.WaitGroup{}
	wg.Add(duplicates)
	s.mockBulkProcessor.On("Add", request).Return().Once()
	mockKafkaMsg.On("Ack").Return(nil).Times(duplicates - 1)
	for i := 0; i < duplicates; i++ {
		addFunc(wg)
	}
	wg.Wait()
	mockKafkaMsg.AssertExpectations(s.T())
}

func (s *esProcessorSuite) TestBulkAfterActionX() {
	version := int64(3)
	testKey := "testKey"
	request := &mocks2.GenericBulkableRequest{}
	request.On("String").Return("")
	request.On("Source").Return([]string{string(`{"delete":{"_id":"testKey"}}`)}, nil)

	requests := []bulk.GenericBulkableRequest{request}

	mSuccess := map[string]*bulk.GenericBulkResponseItem{
		"index": {
			Index:   testIndex,
			Type:    testType,
			ID:      testID,
			Version: version,
			Status:  200,
		},
	}
	response := &bulk.GenericBulkResponse{
		Took:   3,
		Errors: false,
		Items:  []map[string]*bulk.GenericBulkResponseItem{mSuccess},
	}

	mockKafkaMsg := &msgMocks.Message{}
	mapVal := newKafkaMessageWithMetrics(mockKafkaMsg, &testStopWatch)
	s.esProcessor.mapToKafkaMsg.Put(testKey, mapVal)
	mockKafkaMsg.On("Ack").Return(nil).Once()
	s.esProcessor.bulkAfterAction(0, requests, response, nil)
	mockKafkaMsg.AssertExpectations(s.T())
}

func (s *esProcessorSuite) TestBulkAfterAction_Nack() {
	version := int64(3)
	testKey := "testKey"
	request := &mocks2.GenericBulkableRequest{}
	request.On("String").Return("")
	request.On("Source").Return([]string{string(`{"delete":{"_id":"testKey"}}`)}, nil)
	requests := []bulk.GenericBulkableRequest{request}

	mFailed := map[string]*bulk.GenericBulkResponseItem{
		"index": {
			Index:   testIndex,
			Type:    testType,
			ID:      testID,
			Version: version,
			Status:  400,
		},
	}
	response := &bulk.GenericBulkResponse{
		Took:   3,
		Errors: false,
		Items:  []map[string]*bulk.GenericBulkResponseItem{mFailed},
	}

	wid := "test-workflowID"
	rid := "test-runID"
	domainID := "test-domainID"
	payload := s.getEncodedMsg(wid, rid, domainID)

	mockKafkaMsg := &msgMocks.Message{}
	mapVal := newKafkaMessageWithMetrics(mockKafkaMsg, &testStopWatch)
	s.esProcessor.mapToKafkaMsg.Put(testKey, mapVal)
	mockKafkaMsg.On("Nack").Return(nil).Once()
	mockKafkaMsg.On("Value").Return(payload).Once()
	// s.mockBulkProcessor.On("RetrieveKafkaKey", request, mock.Anything, mock.Anything).Return(testKey)
	s.esProcessor.bulkAfterAction(0, requests, response, nil)
	mockKafkaMsg.AssertExpectations(s.T())
}

func (s *esProcessorSuite) TestBulkAfterAction_Error() {
	version := int64(3)
	testKey := "testKey"
	request := &mocks2.GenericBulkableRequest{}
	request.On("String").Return("")
	request.On("Source").Return([]string{string(`{"delete":{"_id":"testKey"}}`)}, nil)
	requests := []bulk.GenericBulkableRequest{request}

	mFailed := map[string]*bulk.GenericBulkResponseItem{
		"index": {
			Index:   testIndex,
			Type:    testType,
			ID:      testID,
			Version: version,
			Status:  400,
		},
	}
	response := &bulk.GenericBulkResponse{
		Took:   3,
		Errors: true,
		Items:  []map[string]*bulk.GenericBulkResponseItem{mFailed},
	}

	wid := "test-workflowID"
	rid := "test-runID"
	domainID := "test-domainID"
	payload := s.getEncodedMsg(wid, rid, domainID)

	mockKafkaMsg := &msgMocks.Message{}
	mapVal := newKafkaMessageWithMetrics(mockKafkaMsg, &testStopWatch)
	s.esProcessor.mapToKafkaMsg.Put(testKey, mapVal)
	mockKafkaMsg.On("Nack").Return(nil).Once()
	mockKafkaMsg.On("Value").Return(payload).Once()
	s.mockScope.On("IncCounter", metrics.ESProcessorFailures).Once()
	s.esProcessor.bulkAfterAction(0, requests, response, &bulk.GenericError{Details: fmt.Errorf("some error")})
}

func (s *esProcessorSuite) TestBulkAfterAction_Error_Nack() {
	version := int64(3)
	testKey := "testKey"
	request := &mocks2.GenericBulkableRequest{}
	request.On("String").Return("")
	request.On("Source").Return([]string{`{"delete":{"_index":"test-index","_id":"testKey"}}`}, nil)
	requests := []bulk.GenericBulkableRequest{request}

	mFailed := map[string]*bulk.GenericBulkResponseItem{
		"delete": {
			Index:   testIndex,
			Type:    testType,
			ID:      testID,
			Version: version,
			Status:  409,
		},
	}
	response := &bulk.GenericBulkResponse{
		Took:   3,
		Errors: true,
		Items:  []map[string]*bulk.GenericBulkResponseItem{mFailed},
	}

	wid := "test-workflowID"
	rid := "test-runID"
	domainID := "test-domainID"
	payload := s.getEncodedMsg(wid, rid, domainID)

	mockKafkaMsg := &msgMocks.Message{}
	mapVal := newKafkaMessageWithMetrics(mockKafkaMsg, &testStopWatch)
	s.esProcessor.mapToKafkaMsg.Put(testKey, mapVal)
	mockKafkaMsg.On("Nack").Return(nil).Once()
	mockKafkaMsg.On("Ack").Return(nil).Once() // Expect Ack to be called
	mockKafkaMsg.On("Value").Return(payload).Once()
	s.mockScope.On("IncCounter", metrics.ESProcessorFailures).Once()
	s.esProcessor.bulkAfterAction(0, requests, response, &bulk.GenericError{Status: 404, Details: fmt.Errorf("some error")})
}

func (s *esProcessorSuite) TestAckKafkaMsg() {
	key := "test-key"
	// no msg in map, nothing called
	s.esProcessor.ackKafkaMsg(key)

	request := &bulk.GenericBulkableAddRequest{}
	mockKafkaMsg := &msgMocks.Message{}
	s.mockScope.On("StartTimer", testMetric).Return(testStopWatch).Once()
	s.mockBulkProcessor.On("Add", request).Return().Once()
	s.esProcessor.Add(request, key, mockKafkaMsg)
	s.Equal(1, s.esProcessor.mapToKafkaMsg.Len())

	mockKafkaMsg.On("Ack").Return(nil).Once()
	s.esProcessor.ackKafkaMsg(key)
	mockKafkaMsg.AssertExpectations(s.T())
	s.Equal(0, s.esProcessor.mapToKafkaMsg.Len())
}

func (s *esProcessorSuite) TestNackKafkaMsg() {
	key := "test-key-nack"
	// no msg in map, nothing called
	s.esProcessor.nackKafkaMsg(key)

	request := &bulk.GenericBulkableAddRequest{}
	mockKafkaMsg := &msgMocks.Message{}
	s.mockBulkProcessor.On("Add", request).Return().Once()
	s.mockScope.On("StartTimer", testMetric).Return(testStopWatch).Once()
	s.esProcessor.Add(request, key, mockKafkaMsg)
	s.Equal(1, s.esProcessor.mapToKafkaMsg.Len())

	mockKafkaMsg.On("Nack").Return(nil).Once()
	s.esProcessor.nackKafkaMsg(key)
	mockKafkaMsg.AssertExpectations(s.T())
	s.Equal(0, s.esProcessor.mapToKafkaMsg.Len())
}

func (s *esProcessorSuite) TestHashFn() {
	s.Equal(uint32(0), s.esProcessor.hashFn(0))
	s.NotEqual(uint32(0), s.esProcessor.hashFn("test"))
}

func (s *esProcessorSuite) getEncodedMsg(wid string, rid string, domainID string) []byte {
	indexMsg := &indexer.Message{
		DomainID:   common.StringPtr(domainID),
		WorkflowID: common.StringPtr(wid),
		RunID:      common.StringPtr(rid),
	}
	payload, err := s.esProcessor.msgEncoder.Encode(indexMsg)
	s.NoError(err)
	return payload
}

func (s *esProcessorSuite) TestGetMsgWithInfo() {
	testKey := "test-key"
	testWid := "test-workflowID"
	testRid := "test-runID"
	testDomainid := "test-domainID"
	payload := s.getEncodedMsg(testWid, testRid, testDomainid)

	mockKafkaMsg := &msgMocks.Message{}
	mockKafkaMsg.On("Value").Return(payload).Once()
	mapVal := newKafkaMessageWithMetrics(mockKafkaMsg, &testStopWatch)
	s.esProcessor.mapToKafkaMsg.Put(testKey, mapVal)
	wid, rid, domainID := s.esProcessor.getMsgWithInfo(testKey)
	s.Equal(testWid, wid)
	s.Equal(testRid, rid)
	s.Equal(testDomainid, domainID)
}

func (s *esProcessorSuite) TestGetMsgInfo_Error() {
	testKey := "test-key"
	mockKafkaMsg := &msgMocks.Message{}
	mockKafkaMsg.On("Value").Return([]byte{}).Once()
	mapVal := newKafkaMessageWithMetrics(mockKafkaMsg, &testStopWatch)
	s.esProcessor.mapToKafkaMsg.Put(testKey, mapVal)
	wid, rid, domainID := s.esProcessor.getMsgWithInfo(testKey)
	s.Equal("", wid)
	s.Equal("", rid)
	s.Equal("", domainID)
}

func (s *esProcessorSuite) TestIsResponseSuccess() {
	for i := 200; i < 300; i++ {
		s.True(isResponseSuccess(i))
	}
	status := []int{409, 404}
	for _, code := range status {
		s.True(isResponseSuccess(code))
	}
	status = []int{100, 199, 300, 400, 500, 408, 429, 503, 507}
	for _, code := range status {
		s.False(isResponseSuccess(code))
	}
}

func (s *esProcessorSuite) TestIsResponseRetriable() {
	status := []int{408, 429, 500, 503, 507}
	for _, code := range status {
		s.True(isResponseRetriable(code))
	}
}

func (s *esProcessorSuite) TestIsErrorRetriable() {
	tests := []struct {
		input    *bulk.GenericError
		expected bool
	}{
		{
			input:    &bulk.GenericError{Status: 400},
			expected: false,
		},
		{
			input:    &bulk.GenericError{Status: 408},
			expected: true,
		},
		{
			input:    &bulk.GenericError{},
			expected: false,
		},
	}
	for _, test := range tests {
		s.Equal(test.expected, isResponseRetriable(test.input.Status))
	}
}

func (s *esProcessorSuite) TestIsDeleteRequest() {
	tests := []struct {
		request   bulk.GenericBulkableRequest
		bIsDelete bool
	}{
		{
			request: bulk.NewBulkIndexRequest().
				ID("request.ID").
				Index("request.Index").
				Version(int64(0)).
				VersionType("request.VersionType").Doc("request.Doc"),
			bIsDelete: false,
		},
		{
			request: bulk.NewBulkDeleteRequest().
				ID("request.ID").
				Index("request.Index"),
			bIsDelete: true,
		},
	}
	for _, test := range tests {
		req, _ := test.request.Source()
		s.Equal(test.bIsDelete, s.esProcessor.isDeleteRequest(req))
	}
}

func (s *esProcessorSuite) TestIsDeleteRequest_Error() {
	request := &MockBulkableRequest{}
	s.mockScope.On("IncCounter", mock.AnythingOfType("int")).Return()
	req, err := request.Source()
	s.False(s.esProcessor.isDeleteRequest(req))
	s.Error(err)
}

// MockBulkableRequest is a mock implementation of the GenericBulkableRequest interface
type MockBulkableRequest struct{}

// String returns a mock string
func (m *MockBulkableRequest) String() string {
	return "mock request"
}

// Source returns an error to simulate a failure
func (m *MockBulkableRequest) Source() ([]string, error) {
	return nil, fmt.Errorf("simulated source error")
}

func (s *esProcessorSuite) TestBulkAfterAction_Nack_Shadow_WithError() {
	version := int64(3)
	testKey := "testKey"
	request := &mocks2.GenericBulkableRequest{}
	request.On("String").Return("")
	request.On("Source").Return([]string{string(`{"delete":{"_id":"testKey"}}`)}, nil)
	requests := []bulk.GenericBulkableRequest{request}

	mFailed := map[string]*bulk.GenericBulkResponseItem{
		"index": {
			Index:   testIndex,
			Type:    testType,
			ID:      testID,
			Version: version,
			Status:  400,
		},
	}
	response := &bulk.GenericBulkResponse{
		Took:   3,
		Errors: false,
		Items:  []map[string]*bulk.GenericBulkResponseItem{mFailed},
	}

	// Mock error to be passed to the after action functions
	mockErr := &bulk.GenericError{
		Status:  500,
		Details: fmt.Errorf("Test error occurred"),
	}

	wid := "test-workflowID"
	rid := "test-runID"
	domainID := "test-domainID"
	payload := s.getEncodedMsg(wid, rid, domainID)

	mockKafkaMsg := &msgMocks.Message{}
	mapVal := newKafkaMessageWithMetrics(mockKafkaMsg, &testStopWatch)
	s.esProcessor.mapToKafkaMsg.Put(testKey, mapVal)

	// Mock Kafka message Nack and Value
	mockKafkaMsg.On("Nack").Return(nil).Once()
	mockKafkaMsg.On("Value").Return(payload).Once()
	s.mockScope.On("IncCounter", mock.AnythingOfType("int")).Return()
	// Execute bulkAfterAction for primary processor with error
	s.esProcessor.bulkAfterAction(0, requests, response, mockErr)
}

func (s *esProcessorSuite) TestBulkAfterAction_Shadow_Fail_WithoutError() {
	version := int64(3)
	testKey := "testKey"
	request := &mocks2.GenericBulkableRequest{}
	request.On("String").Return("")
	request.On("Source").Return([]string{string(`{"delete":{"_id":"testKey"}}`)}, nil)
	requests := []bulk.GenericBulkableRequest{request}

	mFailed := map[string]*bulk.GenericBulkResponseItem{
		"index": {
			Index:   testIndex,
			Type:    testType,
			ID:      testID,
			Version: version,
			Status:  400,
		},
	}
	response := &bulk.GenericBulkResponse{
		Took:   3,
		Errors: false,
		Items:  []map[string]*bulk.GenericBulkResponseItem{mFailed},
	}

	wid := "test-workflowID"
	rid := "test-runID"
	domainID := "test-domainID"
	payload := s.getEncodedMsg(wid, rid, domainID)

	mockKafkaMsg := &msgMocks.Message{}
	mapVal := newKafkaMessageWithMetrics(mockKafkaMsg, &testStopWatch)
	s.esProcessor.mapToKafkaMsg.Put(testKey, mapVal)

	// Mock Kafka message Nack and Value
	mockKafkaMsg.On("Nack").Return(nil).Once()
	mockKafkaMsg.On("Value").Return(payload).Once()
	s.mockScope.On("IncCounter", mock.AnythingOfType("int")).Return()
	// Execute bulkAfterAction for primary processor with error
	s.esProcessor.bulkAfterAction(0, requests, response, nil)
}
