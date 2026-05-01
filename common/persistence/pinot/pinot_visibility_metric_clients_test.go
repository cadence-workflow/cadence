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

package pinotvisibility

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/.gen/go/indexer"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	metricsClientMocks "github.com/uber/cadence/common/metrics/mocks"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
	p "github.com/uber/cadence/common/persistence"
	pnt "github.com/uber/cadence/common/pinot"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/common/types"
)

var (
	testStopwatch = metrics.NoopScope.StartTimer(metrics.PinotLatency)
)

func TestMetricClientRecordWorkflowExecutionStarted(t *testing.T) {
	// test non-empty request fields match
	errorRequest := &p.RecordWorkflowExecutionStartedRequest{
		WorkflowTypeName: "errorWorkflowTypeName",
		Memo: &types.Memo{
			map[string][]byte{
				"CustomStringField": []byte("test string"),
			},
		},
	}

	request := &p.RecordWorkflowExecutionStartedRequest{
		WorkflowTypeName: "wtn",
		Memo: &types.Memo{
			map[string][]byte{
				"CustomStringField": []byte("test string"),
			},
		},
	}

	tests := map[string]struct {
		request                *p.RecordWorkflowExecutionStartedRequest
		producerMockAffordance func(mockProducer *mocks.KafkaProducer)
		scopeMockAffordance    func(mockScope *metricsClientMocks.Scope)
		expectedError          error
	}{
		"Case1: error case": {
			request: errorRequest,
			producerMockAffordance: func(mockProducer *mocks.KafkaProducer) {
				mockProducer.On("Publish", mock.Anything, mock.MatchedBy(func(input *indexer.PinotMessage) bool {
					return true
				})).Return(&types.BadRequestError{}).Once()
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Times(3)
			},
			expectedError: &types.BadRequestError{},
		},
		"Case2: normal case": {
			request: request,
			producerMockAffordance: func(mockProducer *mocks.KafkaProducer) {
				mockProducer.On("Publish", mock.Anything, mock.MatchedBy(func(input *indexer.PinotMessage) bool {
					return true
				})).Return(nil).Once()
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Once()
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// create mock clients
			ctrl := gomock.NewController(t)
			mockPinotClient := pnt.NewMockGenericClient(ctrl)
			mockProducer := &mocks.KafkaProducer{}
			mockMetricClient := &metricsClientMocks.Client{}
			mockScope := &metricsClientMocks.Scope{}

			// create metricClient
			logger := log.NewNoop()
			mgr := NewPinotVisibilityStore(mockPinotClient, &service.Config{
				ValidSearchAttributes:  dynamicproperties.GetMapPropertyFn(definition.GetDefaultIndexedKeys()),
				ESIndexMaxResultWindow: dynamicproperties.GetIntPropertyFn(3),
			}, mockProducer, testlogger.New(t))
			visibilityStore := mgr.(*pinotVisibilityStore)
			pinotVisibilityManager := p.NewVisibilityManagerImpl(visibilityStore, logger, &persistence.DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})
			visibilityMgr := NewPinotVisibilityMetricsClient(pinotVisibilityManager, mockMetricClient, logger)
			metricsClient := visibilityMgr.(*pinotVisibilityMetricsClient)

			// mock behaviors
			mockMetricClient.On("Scope", mock.Anything, mock.Anything).Return(mockScope).Once()
			mockScope.On("StartTimer", mock.Anything, mock.Anything).Return(testStopwatch).Once()
			test.producerMockAffordance(mockProducer)
			test.scopeMockAffordance(mockScope)

			err := metricsClient.RecordWorkflowExecutionStarted(context.Background(), test.request)
			assert.Equal(t, err, test.expectedError)
		})
	}
}

func TestMetricClientRecordWorkflowExecutionClosed(t *testing.T) {
	// test non-empty request fields match
	errorRequest := &p.RecordWorkflowExecutionClosedRequest{
		WorkflowTypeName: "errorWorkflowTypeName",
		Memo: &types.Memo{
			map[string][]byte{
				"CustomStringField": []byte("test string"),
			},
		},
	}

	request := &p.RecordWorkflowExecutionClosedRequest{
		WorkflowTypeName: "wtn",
		Memo: &types.Memo{
			map[string][]byte{
				"CustomStringField": []byte("test string"),
			},
		},
	}

	tests := map[string]struct {
		request                *p.RecordWorkflowExecutionClosedRequest
		producerMockAffordance func(mockProducer *mocks.KafkaProducer)
		scopeMockAffordance    func(mockScope *metricsClientMocks.Scope)
		expectedError          error
	}{
		"Case1: error case": {
			request: errorRequest,
			producerMockAffordance: func(mockProducer *mocks.KafkaProducer) {
				mockProducer.On("Publish", mock.Anything, mock.MatchedBy(func(input *indexer.PinotMessage) bool {
					return true
				})).Return(&types.ServiceBusyError{}).Once()
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Times(3)
			},
			expectedError: &types.ServiceBusyError{},
		},
		"Case2: normal case": {
			request: request,
			producerMockAffordance: func(mockProducer *mocks.KafkaProducer) {
				mockProducer.On("Publish", mock.Anything, mock.MatchedBy(func(input *indexer.PinotMessage) bool {
					return true
				})).Return(nil).Once()
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Once()
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// create mock clients
			ctrl := gomock.NewController(t)
			mockPinotClient := pnt.NewMockGenericClient(ctrl)
			mockProducer := &mocks.KafkaProducer{}
			mockMetricClient := &metricsClientMocks.Client{}
			mockScope := &metricsClientMocks.Scope{}

			// create metricClient
			logger := log.NewNoop()
			mgr := NewPinotVisibilityStore(mockPinotClient, &service.Config{
				ValidSearchAttributes:  dynamicproperties.GetMapPropertyFn(definition.GetDefaultIndexedKeys()),
				ESIndexMaxResultWindow: dynamicproperties.GetIntPropertyFn(3),
			}, mockProducer, testlogger.New(t))
			visibilityStore := mgr.(*pinotVisibilityStore)
			pinotVisibilityManager := p.NewVisibilityManagerImpl(visibilityStore, logger, &persistence.DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})
			visibilityMgr := NewPinotVisibilityMetricsClient(pinotVisibilityManager, mockMetricClient, logger)
			metricsClient := visibilityMgr.(*pinotVisibilityMetricsClient)

			// mock behaviors
			mockMetricClient.On("Scope", mock.Anything, mock.Anything).Return(mockScope).Once()
			mockScope.On("StartTimer", mock.Anything, mock.Anything).Return(testStopwatch).Once()
			test.producerMockAffordance(mockProducer)
			test.scopeMockAffordance(mockScope)

			err := metricsClient.RecordWorkflowExecutionClosed(context.Background(), test.request)
			assert.Equal(t, err, test.expectedError)
		})
	}
}

func TestMetricClientRecordWorkflowExecutionUninitialized(t *testing.T) {
	// test non-empty request fields match
	errorRequest := &p.RecordWorkflowExecutionUninitializedRequest{
		WorkflowTypeName: "errorWorkflowTypeName",
	}

	request := &p.RecordWorkflowExecutionUninitializedRequest{
		WorkflowTypeName: "wtn",
	}

	tests := map[string]struct {
		request                *p.RecordWorkflowExecutionUninitializedRequest
		producerMockAffordance func(mockProducer *mocks.KafkaProducer)
		scopeMockAffordance    func(mockScope *metricsClientMocks.Scope)
		expectedError          error
	}{
		"Case1: error case": {
			request: errorRequest,
			producerMockAffordance: func(mockProducer *mocks.KafkaProducer) {
				mockProducer.On("Publish", mock.Anything, mock.MatchedBy(func(input *indexer.PinotMessage) bool {
					return true
				})).Return(fmt.Errorf("error")).Once()
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Times(3)
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			request: request,
			producerMockAffordance: func(mockProducer *mocks.KafkaProducer) {
				mockProducer.On("Publish", mock.Anything, mock.MatchedBy(func(input *indexer.PinotMessage) bool {
					return true
				})).Return(nil).Once()
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Once()
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// create mock clients
			ctrl := gomock.NewController(t)
			mockPinotClient := pnt.NewMockGenericClient(ctrl)
			mockProducer := &mocks.KafkaProducer{}
			mockMetricClient := &metricsClientMocks.Client{}
			mockScope := &metricsClientMocks.Scope{}

			// create metricClient
			logger := log.NewNoop()
			mgr := NewPinotVisibilityStore(mockPinotClient, &service.Config{
				ValidSearchAttributes:  dynamicproperties.GetMapPropertyFn(definition.GetDefaultIndexedKeys()),
				ESIndexMaxResultWindow: dynamicproperties.GetIntPropertyFn(3),
			}, mockProducer, testlogger.New(t))
			visibilityStore := mgr.(*pinotVisibilityStore)
			pinotVisibilityManager := p.NewVisibilityManagerImpl(visibilityStore, logger, &persistence.DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})
			visibilityMgr := NewPinotVisibilityMetricsClient(pinotVisibilityManager, mockMetricClient, logger)
			metricsClient := visibilityMgr.(*pinotVisibilityMetricsClient)

			// mock behaviors
			mockMetricClient.On("Scope", mock.Anything, mock.Anything).Return(mockScope).Once()
			mockScope.On("StartTimer", mock.Anything, mock.Anything).Return(testStopwatch).Once()
			test.producerMockAffordance(mockProducer)
			test.scopeMockAffordance(mockScope)

			err := metricsClient.RecordWorkflowExecutionUninitialized(context.Background(), test.request)
			assert.Equal(t, err, test.expectedError)
		})
	}
}

func TestMetricClientUpsertWorkflowExecution(t *testing.T) {
	// test non-empty request fields match
	errorRequest := &p.UpsertWorkflowExecutionRequest{
		WorkflowTypeName: "errorWorkflowTypeName",
	}

	request := &p.UpsertWorkflowExecutionRequest{
		WorkflowTypeName: "wtn",
	}

	tests := map[string]struct {
		request                *p.UpsertWorkflowExecutionRequest
		producerMockAffordance func(mockProducer *mocks.KafkaProducer)
		scopeMockAffordance    func(mockScope *metricsClientMocks.Scope)
		expectedError          error
	}{
		"Case1: error case": {
			request: errorRequest,
			producerMockAffordance: func(mockProducer *mocks.KafkaProducer) {
				mockProducer.On("Publish", mock.Anything, mock.MatchedBy(func(input *indexer.PinotMessage) bool {
					return true
				})).Return(fmt.Errorf("error")).Once()
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Times(3)
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			request: request,
			producerMockAffordance: func(mockProducer *mocks.KafkaProducer) {
				mockProducer.On("Publish", mock.Anything, mock.MatchedBy(func(input *indexer.PinotMessage) bool {
					return true
				})).Return(nil).Once()
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Once()
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// create mock clients
			ctrl := gomock.NewController(t)
			mockPinotClient := pnt.NewMockGenericClient(ctrl)
			mockProducer := &mocks.KafkaProducer{}
			mockMetricClient := &metricsClientMocks.Client{}
			mockScope := &metricsClientMocks.Scope{}

			// create metricClient
			logger := log.NewNoop()
			mgr := NewPinotVisibilityStore(mockPinotClient, &service.Config{
				ValidSearchAttributes:  dynamicproperties.GetMapPropertyFn(definition.GetDefaultIndexedKeys()),
				ESIndexMaxResultWindow: dynamicproperties.GetIntPropertyFn(3),
			}, mockProducer, testlogger.New(t))
			visibilityStore := mgr.(*pinotVisibilityStore)
			pinotVisibilityManager := p.NewVisibilityManagerImpl(visibilityStore, logger, &persistence.DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})
			visibilityMgr := NewPinotVisibilityMetricsClient(pinotVisibilityManager, mockMetricClient, logger)
			metricsClient := visibilityMgr.(*pinotVisibilityMetricsClient)

			// mock behaviors
			mockMetricClient.On("Scope", mock.Anything, mock.Anything).Return(mockScope).Once()
			mockScope.On("StartTimer", mock.Anything, mock.Anything).Return(testStopwatch).Once()
			test.producerMockAffordance(mockProducer)
			test.scopeMockAffordance(mockScope)

			err := metricsClient.UpsertWorkflowExecution(context.Background(), test.request)
			assert.Equal(t, err, test.expectedError)
		})
	}
}

func TestMetricClientListOpenWorkflowExecutions(t *testing.T) {
	// test non-empty request fields match
	errorRequest := &p.ListWorkflowExecutionsRequest{
		Domain: "badDomainID",
	}

	request := &p.ListWorkflowExecutionsRequest{
		Domain: DomainID,
	}

	tests := map[string]struct {
		request                   *p.ListWorkflowExecutionsRequest
		pinotClientMockAffordance func(mockPinotClient *pnt.MockGenericClient)
		scopeMockAffordance       func(mockScope *metricsClientMocks.Scope)
		expectedError             error
	}{
		"Case1: error case": {
			request: errorRequest,
			pinotClientMockAffordance: func(mockPinotClient *pnt.MockGenericClient) {
				mockPinotClient.EXPECT().GetTableName().Return(testTableName).Times(1)
				mockPinotClient.EXPECT().Search(gomock.Any()).Return(nil, fmt.Errorf("error")).Times(1)
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Times(3)
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			request: request,
			pinotClientMockAffordance: func(mockPinotClient *pnt.MockGenericClient) {
				mockPinotClient.EXPECT().GetTableName().Return(testTableName).Times(1)
				mockPinotClient.EXPECT().Search(gomock.Any()).Return(nil, nil).Times(1)
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Once()
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// create mock clients
			ctrl := gomock.NewController(t)
			mockPinotClient := pnt.NewMockGenericClient(ctrl)
			mockProducer := &mocks.KafkaProducer{}
			mockMetricClient := &metricsClientMocks.Client{}
			mockScope := &metricsClientMocks.Scope{}

			// create metricClient
			logger := log.NewNoop()
			mgr := NewPinotVisibilityStore(mockPinotClient, &service.Config{
				ValidSearchAttributes:  dynamicproperties.GetMapPropertyFn(definition.GetDefaultIndexedKeys()),
				ESIndexMaxResultWindow: dynamicproperties.GetIntPropertyFn(3),
			}, mockProducer, testlogger.New(t))
			visibilityStore := mgr.(*pinotVisibilityStore)
			pinotVisibilityManager := p.NewVisibilityManagerImpl(visibilityStore, logger, &persistence.DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})
			visibilityMgr := NewPinotVisibilityMetricsClient(pinotVisibilityManager, mockMetricClient, logger)
			metricsClient := visibilityMgr.(*pinotVisibilityMetricsClient)

			// mock behaviors
			mockMetricClient.On("Scope", mock.Anything, mock.Anything).Return(mockScope).Once()
			mockScope.On("StartTimer", mock.Anything, mock.Anything).Return(testStopwatch).Once()
			test.pinotClientMockAffordance(mockPinotClient)
			test.scopeMockAffordance(mockScope)

			_, err := metricsClient.ListOpenWorkflowExecutions(context.Background(), test.request)
			assert.Equal(t, test.expectedError, err)
		})
	}
}

func TestMetricClientListClosedWorkflowExecutions(t *testing.T) {
	// test non-empty request fields match
	errorRequest := &p.ListWorkflowExecutionsRequest{
		Domain: "badDomainId",
	}

	request := &p.ListWorkflowExecutionsRequest{
		Domain: DomainID,
	}

	tests := map[string]struct {
		request                   *p.ListWorkflowExecutionsRequest
		pinotClientMockAffordance func(mockPinotClient *pnt.MockGenericClient)
		scopeMockAffordance       func(mockScope *metricsClientMocks.Scope)
		expectedError             error
	}{
		"Case1: error case": {
			request: errorRequest,
			pinotClientMockAffordance: func(mockPinotClient *pnt.MockGenericClient) {
				mockPinotClient.EXPECT().GetTableName().Return(testTableName).Times(1)
				mockPinotClient.EXPECT().Search(gomock.Any()).Return(nil, fmt.Errorf("error")).Times(1)
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Times(3)
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			request: request,
			pinotClientMockAffordance: func(mockPinotClient *pnt.MockGenericClient) {
				mockPinotClient.EXPECT().GetTableName().Return(testTableName).Times(1)
				mockPinotClient.EXPECT().Search(gomock.Any()).Return(nil, nil).Times(1)
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Once()
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// create mock clients
			ctrl := gomock.NewController(t)
			mockPinotClient := pnt.NewMockGenericClient(ctrl)
			mockProducer := &mocks.KafkaProducer{}
			mockMetricClient := &metricsClientMocks.Client{}
			mockScope := &metricsClientMocks.Scope{}

			// create metricClient
			logger := log.NewNoop()
			mgr := NewPinotVisibilityStore(mockPinotClient, &service.Config{
				ValidSearchAttributes:  dynamicproperties.GetMapPropertyFn(definition.GetDefaultIndexedKeys()),
				ESIndexMaxResultWindow: dynamicproperties.GetIntPropertyFn(3),
			}, mockProducer, testlogger.New(t))
			visibilityStore := mgr.(*pinotVisibilityStore)
			pinotVisibilityManager := p.NewVisibilityManagerImpl(visibilityStore, logger, &persistence.DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})
			visibilityMgr := NewPinotVisibilityMetricsClient(pinotVisibilityManager, mockMetricClient, logger)
			metricsClient := visibilityMgr.(*pinotVisibilityMetricsClient)

			// mock behaviors
			mockMetricClient.On("Scope", mock.Anything, mock.Anything).Return(mockScope).Once()
			mockScope.On("StartTimer", mock.Anything, mock.Anything).Return(testStopwatch).Once()
			test.pinotClientMockAffordance(mockPinotClient)
			test.scopeMockAffordance(mockScope)

			_, err := metricsClient.ListClosedWorkflowExecutions(context.Background(), test.request)
			assert.Equal(t, test.expectedError, err)
		})
	}
}

func TestMetricClientListOpenWorkflowExecutionsByType(t *testing.T) {
	// test non-empty request fields match
	errorRequest := &p.ListWorkflowExecutionsByTypeRequest{
		ListWorkflowExecutionsRequest: p.ListWorkflowExecutionsRequest{
			Domain: "badDomainID",
		},
		WorkflowTypeName: "",
	}

	request := &p.ListWorkflowExecutionsByTypeRequest{}

	tests := map[string]struct {
		request                   *p.ListWorkflowExecutionsByTypeRequest
		pinotClientMockAffordance func(mockPinotClient *pnt.MockGenericClient)
		scopeMockAffordance       func(mockScope *metricsClientMocks.Scope)
		expectedError             error
	}{
		"Case1: error case": {
			request: errorRequest,
			pinotClientMockAffordance: func(mockPinotClient *pnt.MockGenericClient) {
				mockPinotClient.EXPECT().GetTableName().Return(testTableName).Times(1)
				mockPinotClient.EXPECT().Search(gomock.Any()).Return(nil, fmt.Errorf("error")).Times(1)
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Times(3)
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			request: request,
			pinotClientMockAffordance: func(mockPinotClient *pnt.MockGenericClient) {
				mockPinotClient.EXPECT().GetTableName().Return(testTableName).Times(1)
				mockPinotClient.EXPECT().Search(gomock.Any()).Return(nil, nil).Times(1)
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Once()
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// create mock clients
			ctrl := gomock.NewController(t)
			mockPinotClient := pnt.NewMockGenericClient(ctrl)
			mockProducer := &mocks.KafkaProducer{}
			mockMetricClient := &metricsClientMocks.Client{}
			mockScope := &metricsClientMocks.Scope{}

			// create metricClient
			logger := log.NewNoop()
			mgr := NewPinotVisibilityStore(mockPinotClient, &service.Config{
				ValidSearchAttributes:  dynamicproperties.GetMapPropertyFn(definition.GetDefaultIndexedKeys()),
				ESIndexMaxResultWindow: dynamicproperties.GetIntPropertyFn(3),
			}, mockProducer, testlogger.New(t))
			visibilityStore := mgr.(*pinotVisibilityStore)
			pinotVisibilityManager := p.NewVisibilityManagerImpl(visibilityStore, logger, &persistence.DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})
			visibilityMgr := NewPinotVisibilityMetricsClient(pinotVisibilityManager, mockMetricClient, logger)
			metricsClient := visibilityMgr.(*pinotVisibilityMetricsClient)

			// mock behaviors
			mockMetricClient.On("Scope", mock.Anything, mock.Anything).Return(mockScope).Once()
			mockScope.On("StartTimer", mock.Anything, mock.Anything).Return(testStopwatch).Once()
			test.pinotClientMockAffordance(mockPinotClient)
			test.scopeMockAffordance(mockScope)

			_, err := metricsClient.ListOpenWorkflowExecutionsByType(context.Background(), test.request)
			assert.Equal(t, test.expectedError, err)
		})
	}
}

func TestMetricClientListClosedWorkflowExecutionsByType(t *testing.T) {
	// test non-empty request fields match
	errorRequest := &p.ListWorkflowExecutionsByTypeRequest{
		ListWorkflowExecutionsRequest: p.ListWorkflowExecutionsRequest{
			Domain: "badDomainID",
		},
		WorkflowTypeName: "",
	}

	request := &p.ListWorkflowExecutionsByTypeRequest{}

	tests := map[string]struct {
		request                   *p.ListWorkflowExecutionsByTypeRequest
		pinotClientMockAffordance func(mockPinotClient *pnt.MockGenericClient)
		scopeMockAffordance       func(mockScope *metricsClientMocks.Scope)
		expectedError             error
	}{
		"Case1: error case": {
			request: errorRequest,
			pinotClientMockAffordance: func(mockPinotClient *pnt.MockGenericClient) {
				mockPinotClient.EXPECT().GetTableName().Return(testTableName).Times(1)
				mockPinotClient.EXPECT().Search(gomock.Any()).Return(nil, fmt.Errorf("error")).Times(1)
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Times(3)
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			request: request,
			pinotClientMockAffordance: func(mockPinotClient *pnt.MockGenericClient) {
				mockPinotClient.EXPECT().GetTableName().Return(testTableName).Times(1)
				mockPinotClient.EXPECT().Search(gomock.Any()).Return(nil, nil).Times(1)
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Once()
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// create mock clients
			ctrl := gomock.NewController(t)
			mockPinotClient := pnt.NewMockGenericClient(ctrl)
			mockProducer := &mocks.KafkaProducer{}
			mockMetricClient := &metricsClientMocks.Client{}
			mockScope := &metricsClientMocks.Scope{}

			// create metricClient
			logger := log.NewNoop()
			mgr := NewPinotVisibilityStore(mockPinotClient, &service.Config{
				ValidSearchAttributes:  dynamicproperties.GetMapPropertyFn(definition.GetDefaultIndexedKeys()),
				ESIndexMaxResultWindow: dynamicproperties.GetIntPropertyFn(3),
			}, mockProducer, testlogger.New(t))
			visibilityStore := mgr.(*pinotVisibilityStore)
			pinotVisibilityManager := p.NewVisibilityManagerImpl(visibilityStore, logger, &persistence.DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})
			visibilityMgr := NewPinotVisibilityMetricsClient(pinotVisibilityManager, mockMetricClient, logger)
			metricsClient := visibilityMgr.(*pinotVisibilityMetricsClient)

			// mock behaviors
			mockMetricClient.On("Scope", mock.Anything, mock.Anything).Return(mockScope).Once()
			mockScope.On("StartTimer", mock.Anything, mock.Anything).Return(testStopwatch).Once()
			test.pinotClientMockAffordance(mockPinotClient)
			test.scopeMockAffordance(mockScope)

			_, err := metricsClient.ListClosedWorkflowExecutionsByType(context.Background(), test.request)
			assert.Equal(t, test.expectedError, err)
		})
	}
}

func TestMetricClientListOpenWorkflowExecutionsByWorkflowID(t *testing.T) {
	// test non-empty request fields match
	errorRequest := &p.ListWorkflowExecutionsByWorkflowIDRequest{
		ListWorkflowExecutionsRequest: p.ListWorkflowExecutionsRequest{
			Domain: "badDomainID",
		},
	}

	request := &p.ListWorkflowExecutionsByWorkflowIDRequest{}

	tests := map[string]struct {
		request                   *p.ListWorkflowExecutionsByWorkflowIDRequest
		pinotClientMockAffordance func(mockPinotClient *pnt.MockGenericClient)
		scopeMockAffordance       func(mockScope *metricsClientMocks.Scope)
		expectedError             error
	}{
		"Case1: error case": {
			request: errorRequest,
			pinotClientMockAffordance: func(mockPinotClient *pnt.MockGenericClient) {
				mockPinotClient.EXPECT().GetTableName().Return(testTableName).Times(1)
				mockPinotClient.EXPECT().Search(gomock.Any()).Return(nil, fmt.Errorf("error")).Times(1)
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Times(3)
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			request: request,
			pinotClientMockAffordance: func(mockPinotClient *pnt.MockGenericClient) {
				mockPinotClient.EXPECT().GetTableName().Return(testTableName).Times(1)
				mockPinotClient.EXPECT().Search(gomock.Any()).Return(nil, nil).Times(1)
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Once()
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// create mock clients
			ctrl := gomock.NewController(t)
			mockPinotClient := pnt.NewMockGenericClient(ctrl)
			mockProducer := &mocks.KafkaProducer{}
			mockMetricClient := &metricsClientMocks.Client{}
			mockScope := &metricsClientMocks.Scope{}

			// create metricClient
			logger := log.NewNoop()
			mgr := NewPinotVisibilityStore(mockPinotClient, &service.Config{
				ValidSearchAttributes:  dynamicproperties.GetMapPropertyFn(definition.GetDefaultIndexedKeys()),
				ESIndexMaxResultWindow: dynamicproperties.GetIntPropertyFn(3),
			}, mockProducer, testlogger.New(t))
			visibilityStore := mgr.(*pinotVisibilityStore)
			pinotVisibilityManager := p.NewVisibilityManagerImpl(visibilityStore, logger, &persistence.DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})
			visibilityMgr := NewPinotVisibilityMetricsClient(pinotVisibilityManager, mockMetricClient, logger)
			metricsClient := visibilityMgr.(*pinotVisibilityMetricsClient)

			// mock behaviors
			mockMetricClient.On("Scope", mock.Anything, mock.Anything).Return(mockScope).Once()
			mockScope.On("StartTimer", mock.Anything, mock.Anything).Return(testStopwatch).Once()
			test.pinotClientMockAffordance(mockPinotClient)
			test.scopeMockAffordance(mockScope)

			_, err := metricsClient.ListOpenWorkflowExecutionsByWorkflowID(context.Background(), test.request)
			assert.Equal(t, test.expectedError, err)
		})
	}
}

func TestMetricClientListClosedWorkflowExecutionsByWorkflowID(t *testing.T) {
	// test non-empty request fields match
	errorRequest := &p.ListWorkflowExecutionsByWorkflowIDRequest{
		ListWorkflowExecutionsRequest: p.ListWorkflowExecutionsRequest{
			Domain: "badDomainID",
		},
	}

	request := &p.ListWorkflowExecutionsByWorkflowIDRequest{}

	tests := map[string]struct {
		request                   *p.ListWorkflowExecutionsByWorkflowIDRequest
		pinotClientMockAffordance func(mockPinotClient *pnt.MockGenericClient)
		scopeMockAffordance       func(mockScope *metricsClientMocks.Scope)
		expectedError             error
	}{
		"Case1: error case": {
			request: errorRequest,
			pinotClientMockAffordance: func(mockPinotClient *pnt.MockGenericClient) {
				mockPinotClient.EXPECT().GetTableName().Return(testTableName).Times(1)
				mockPinotClient.EXPECT().Search(gomock.Any()).Return(nil, fmt.Errorf("error")).Times(1)
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Times(3)
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			request: request,
			pinotClientMockAffordance: func(mockPinotClient *pnt.MockGenericClient) {
				mockPinotClient.EXPECT().GetTableName().Return(testTableName).Times(1)
				mockPinotClient.EXPECT().Search(gomock.Any()).Return(nil, nil).Times(1)
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Once()
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// create mock clients
			ctrl := gomock.NewController(t)
			mockPinotClient := pnt.NewMockGenericClient(ctrl)
			mockProducer := &mocks.KafkaProducer{}
			mockMetricClient := &metricsClientMocks.Client{}
			mockScope := &metricsClientMocks.Scope{}

			// create metricClient
			logger := log.NewNoop()
			mgr := NewPinotVisibilityStore(mockPinotClient, &service.Config{
				ValidSearchAttributes:  dynamicproperties.GetMapPropertyFn(definition.GetDefaultIndexedKeys()),
				ESIndexMaxResultWindow: dynamicproperties.GetIntPropertyFn(3),
			}, mockProducer, testlogger.New(t))
			visibilityStore := mgr.(*pinotVisibilityStore)
			pinotVisibilityManager := p.NewVisibilityManagerImpl(visibilityStore, logger, &persistence.DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})
			visibilityMgr := NewPinotVisibilityMetricsClient(pinotVisibilityManager, mockMetricClient, logger)
			metricsClient := visibilityMgr.(*pinotVisibilityMetricsClient)

			// mock behaviors
			mockMetricClient.On("Scope", mock.Anything, mock.Anything).Return(mockScope).Once()
			mockScope.On("StartTimer", mock.Anything, mock.Anything).Return(testStopwatch).Once()
			test.pinotClientMockAffordance(mockPinotClient)
			test.scopeMockAffordance(mockScope)

			_, err := metricsClient.ListClosedWorkflowExecutionsByWorkflowID(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Equal(t, test.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMetricClientListClosedWorkflowExecutionsByStatus(t *testing.T) {
	// test non-empty request fields match
	errorRequest := &p.ListClosedWorkflowExecutionsByStatusRequest{
		ListWorkflowExecutionsRequest: p.ListWorkflowExecutionsRequest{
			Domain: "badDomainID",
		},
	}

	request := &p.ListClosedWorkflowExecutionsByStatusRequest{}

	tests := map[string]struct {
		request                   *p.ListClosedWorkflowExecutionsByStatusRequest
		pinotClientMockAffordance func(mockPinotClient *pnt.MockGenericClient)
		scopeMockAffordance       func(mockScope *metricsClientMocks.Scope)
		expectedError             error
	}{
		"Case1: error case": {
			request: errorRequest,
			pinotClientMockAffordance: func(mockPinotClient *pnt.MockGenericClient) {
				mockPinotClient.EXPECT().GetTableName().Return(testTableName).Times(1)
				mockPinotClient.EXPECT().Search(gomock.Any()).Return(nil, fmt.Errorf("error")).Times(1)
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Times(3)
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			request: request,
			pinotClientMockAffordance: func(mockPinotClient *pnt.MockGenericClient) {
				mockPinotClient.EXPECT().GetTableName().Return(testTableName).Times(1)
				mockPinotClient.EXPECT().Search(gomock.Any()).Return(nil, nil).Times(1)
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Once()
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// create mock clients
			ctrl := gomock.NewController(t)
			mockPinotClient := pnt.NewMockGenericClient(ctrl)
			mockProducer := &mocks.KafkaProducer{}
			mockMetricClient := &metricsClientMocks.Client{}
			mockScope := &metricsClientMocks.Scope{}

			// create metricClient
			logger := log.NewNoop()
			mgr := NewPinotVisibilityStore(mockPinotClient, &service.Config{
				ValidSearchAttributes:  dynamicproperties.GetMapPropertyFn(definition.GetDefaultIndexedKeys()),
				ESIndexMaxResultWindow: dynamicproperties.GetIntPropertyFn(3),
			}, mockProducer, testlogger.New(t))
			visibilityStore := mgr.(*pinotVisibilityStore)
			pinotVisibilityManager := p.NewVisibilityManagerImpl(visibilityStore, logger, &persistence.DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})
			visibilityMgr := NewPinotVisibilityMetricsClient(pinotVisibilityManager, mockMetricClient, logger)
			metricsClient := visibilityMgr.(*pinotVisibilityMetricsClient)

			// mock behaviors
			mockMetricClient.On("Scope", mock.Anything, mock.Anything).Return(mockScope).Once()
			mockScope.On("StartTimer", mock.Anything, mock.Anything).Return(testStopwatch).Once()
			test.pinotClientMockAffordance(mockPinotClient)
			test.scopeMockAffordance(mockScope)

			_, err := metricsClient.ListClosedWorkflowExecutionsByStatus(context.Background(), test.request)
			assert.Equal(t, test.expectedError, err)
		})
	}
}

func TestMetricClientGetClosedWorkflowExecution(t *testing.T) {
	// test non-empty request fields match
	errorRequest := &p.GetClosedWorkflowExecutionRequest{}

	request := &p.GetClosedWorkflowExecutionRequest{}

	tests := map[string]struct {
		request                   *p.GetClosedWorkflowExecutionRequest
		pinotClientMockAffordance func(mockPinotClient *pnt.MockGenericClient)
		scopeMockAffordance       func(mockScope *metricsClientMocks.Scope)
		expectedError             error
	}{
		"Case1: error case": {
			request: errorRequest,
			pinotClientMockAffordance: func(mockPinotClient *pnt.MockGenericClient) {
				mockPinotClient.EXPECT().GetTableName().Return(testTableName).Times(1)
				mockPinotClient.EXPECT().Search(gomock.Any()).Return(nil, fmt.Errorf("error")).Times(1)
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Times(3)
			},
			expectedError: fmt.Errorf("Pinot GetClosedWorkflowExecution failed, error"),
		},
		"Case2: normal case": {
			request: request,
			pinotClientMockAffordance: func(mockPinotClient *pnt.MockGenericClient) {
				mockPinotClient.EXPECT().GetTableName().Return(testTableName).Times(1)
				mockPinotClient.EXPECT().Search(gomock.Any()).Return(&pnt.SearchResponse{
					Executions: []*p.InternalVisibilityWorkflowExecutionInfo{
						{
							DomainID: DomainID,
						},
					},
				}, nil).Times(1)
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Once()
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// create mock clients
			ctrl := gomock.NewController(t)
			mockPinotClient := pnt.NewMockGenericClient(ctrl)
			mockProducer := &mocks.KafkaProducer{}
			mockMetricClient := &metricsClientMocks.Client{}
			mockScope := &metricsClientMocks.Scope{}

			// create metricClient
			logger := log.NewNoop()
			mgr := NewPinotVisibilityStore(mockPinotClient, &service.Config{
				ValidSearchAttributes:  dynamicproperties.GetMapPropertyFn(definition.GetDefaultIndexedKeys()),
				ESIndexMaxResultWindow: dynamicproperties.GetIntPropertyFn(3),
			}, mockProducer, testlogger.New(t))
			visibilityStore := mgr.(*pinotVisibilityStore)
			pinotVisibilityManager := p.NewVisibilityManagerImpl(visibilityStore, logger, &persistence.DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})
			visibilityMgr := NewPinotVisibilityMetricsClient(pinotVisibilityManager, mockMetricClient, logger)
			metricsClient := visibilityMgr.(*pinotVisibilityMetricsClient)

			// mock behaviors
			mockMetricClient.On("Scope", mock.Anything, mock.Anything).Return(mockScope).Once()
			mockScope.On("StartTimer", mock.Anything, mock.Anything).Return(testStopwatch).Once()
			test.pinotClientMockAffordance(mockPinotClient)
			test.scopeMockAffordance(mockScope)

			_, err := metricsClient.GetClosedWorkflowExecution(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Equal(t, test.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMetricClientListWorkflowExecutions(t *testing.T) {
	errorRequest := &p.ListWorkflowExecutionsByQueryRequest{}
	request := &p.ListWorkflowExecutionsByQueryRequest{}

	tests := map[string]struct {
		request                   *p.ListWorkflowExecutionsByQueryRequest
		pinotClientMockAffordance func(mockPinotClient *pnt.MockGenericClient)
		scopeMockAffordance       func(mockScope *metricsClientMocks.Scope)
		expectedError             error
	}{
		"Case1: error case": {
			request: errorRequest,
			pinotClientMockAffordance: func(mockPinotClient *pnt.MockGenericClient) {
				mockPinotClient.EXPECT().GetTableName().Return(testTableName).Times(1)
				mockPinotClient.EXPECT().Search(gomock.Any()).Return(nil, fmt.Errorf("error")).Times(1)
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Times(3)
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			request: request,
			pinotClientMockAffordance: func(mockPinotClient *pnt.MockGenericClient) {
				mockPinotClient.EXPECT().GetTableName().Return(testTableName).Times(1)
				mockPinotClient.EXPECT().Search(gomock.Any()).Return(nil, nil).Times(1)
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Once()
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// create mock clients
			ctrl := gomock.NewController(t)
			mockPinotClient := pnt.NewMockGenericClient(ctrl)
			mockProducer := &mocks.KafkaProducer{}
			mockMetricClient := &metricsClientMocks.Client{}
			mockScope := &metricsClientMocks.Scope{}

			// create metricClient
			logger := log.NewNoop()
			mgr := NewPinotVisibilityStore(mockPinotClient, &service.Config{
				ValidSearchAttributes:  dynamicproperties.GetMapPropertyFn(definition.GetDefaultIndexedKeys()),
				ESIndexMaxResultWindow: dynamicproperties.GetIntPropertyFn(3),
			}, mockProducer, testlogger.New(t))
			visibilityStore := mgr.(*pinotVisibilityStore)
			pinotVisibilityManager := p.NewVisibilityManagerImpl(visibilityStore, logger, &persistence.DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})
			visibilityMgr := NewPinotVisibilityMetricsClient(pinotVisibilityManager, mockMetricClient, logger)
			metricsClient := visibilityMgr.(*pinotVisibilityMetricsClient)

			// mock behaviors
			mockMetricClient.On("Scope", mock.Anything, mock.Anything).Return(mockScope).Once()
			mockScope.On("StartTimer", mock.Anything, mock.Anything).Return(testStopwatch).Once()
			test.pinotClientMockAffordance(mockPinotClient)
			test.scopeMockAffordance(mockScope)

			_, err := metricsClient.ListWorkflowExecutions(context.Background(), test.request)
			assert.Equal(t, test.expectedError, err)
		})
	}
}

func TestMetricClientScanWorkflowExecutions(t *testing.T) {
	errorRequest := &p.ListWorkflowExecutionsByQueryRequest{}
	request := &p.ListWorkflowExecutionsByQueryRequest{}

	tests := map[string]struct {
		request                   *p.ListWorkflowExecutionsByQueryRequest
		pinotClientMockAffordance func(mockPinotClient *pnt.MockGenericClient)
		scopeMockAffordance       func(mockScope *metricsClientMocks.Scope)
		expectedError             error
	}{
		"Case1: error case": {
			request: errorRequest,
			pinotClientMockAffordance: func(mockPinotClient *pnt.MockGenericClient) {
				mockPinotClient.EXPECT().GetTableName().Return(testTableName).Times(1)
				mockPinotClient.EXPECT().Search(gomock.Any()).Return(nil, fmt.Errorf("error")).Times(1)
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Times(3)
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			request: request,
			pinotClientMockAffordance: func(mockPinotClient *pnt.MockGenericClient) {
				mockPinotClient.EXPECT().GetTableName().Return(testTableName).Times(1)
				mockPinotClient.EXPECT().Search(gomock.Any()).Return(nil, nil).Times(1)
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Once()
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// create mock clients
			ctrl := gomock.NewController(t)
			mockPinotClient := pnt.NewMockGenericClient(ctrl)
			mockProducer := &mocks.KafkaProducer{}
			mockMetricClient := &metricsClientMocks.Client{}
			mockScope := &metricsClientMocks.Scope{}

			// create metricClient
			logger := log.NewNoop()
			mgr := NewPinotVisibilityStore(mockPinotClient, &service.Config{
				ValidSearchAttributes:  dynamicproperties.GetMapPropertyFn(definition.GetDefaultIndexedKeys()),
				ESIndexMaxResultWindow: dynamicproperties.GetIntPropertyFn(3),
			}, mockProducer, testlogger.New(t))
			visibilityStore := mgr.(*pinotVisibilityStore)
			pinotVisibilityManager := p.NewVisibilityManagerImpl(visibilityStore, logger, &persistence.DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})
			visibilityMgr := NewPinotVisibilityMetricsClient(pinotVisibilityManager, mockMetricClient, logger)
			metricsClient := visibilityMgr.(*pinotVisibilityMetricsClient)

			// mock behaviors
			mockMetricClient.On("Scope", mock.Anything, mock.Anything).Return(mockScope).Once()
			mockScope.On("StartTimer", mock.Anything, mock.Anything).Return(testStopwatch).Once()
			test.pinotClientMockAffordance(mockPinotClient)
			test.scopeMockAffordance(mockScope)

			_, err := metricsClient.ScanWorkflowExecutions(context.Background(), test.request)
			assert.Equal(t, test.expectedError, err)
		})
	}
}

func TestMetricClientCountWorkflowExecutions(t *testing.T) {
	errorRequest := &p.CountWorkflowExecutionsRequest{}
	request := &p.CountWorkflowExecutionsRequest{}

	tests := map[string]struct {
		request                   *p.CountWorkflowExecutionsRequest
		pinotClientMockAffordance func(mockPinotClient *pnt.MockGenericClient)
		scopeMockAffordance       func(mockScope *metricsClientMocks.Scope)
		expectedError             error
	}{
		"Case1: error case": {
			request: errorRequest,
			pinotClientMockAffordance: func(mockPinotClient *pnt.MockGenericClient) {
				mockPinotClient.EXPECT().GetTableName().Return(testTableName).Times(1)
				mockPinotClient.EXPECT().CountByQuery(gomock.Any()).Return(int64(0), fmt.Errorf("error")).Times(1)
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Times(3)
			},
			expectedError: fmt.Errorf("CountClosedWorkflowExecutions failed, error"),
		},
		"Case2: normal case": {
			request: request,
			pinotClientMockAffordance: func(mockPinotClient *pnt.MockGenericClient) {
				mockPinotClient.EXPECT().GetTableName().Return(testTableName).Times(1)
				mockPinotClient.EXPECT().CountByQuery(gomock.Any()).Return(int64(1), nil).Times(1)
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Once()
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// create mock clients
			ctrl := gomock.NewController(t)
			mockPinotClient := pnt.NewMockGenericClient(ctrl)
			mockProducer := &mocks.KafkaProducer{}
			mockMetricClient := &metricsClientMocks.Client{}
			mockScope := &metricsClientMocks.Scope{}

			// create metricClient
			logger := log.NewNoop()
			mgr := NewPinotVisibilityStore(mockPinotClient, &service.Config{
				ValidSearchAttributes:  dynamicproperties.GetMapPropertyFn(definition.GetDefaultIndexedKeys()),
				ESIndexMaxResultWindow: dynamicproperties.GetIntPropertyFn(3),
			}, mockProducer, testlogger.New(t))
			visibilityStore := mgr.(*pinotVisibilityStore)
			pinotVisibilityManager := p.NewVisibilityManagerImpl(visibilityStore, logger, &persistence.DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})
			visibilityMgr := NewPinotVisibilityMetricsClient(pinotVisibilityManager, mockMetricClient, logger)
			metricsClient := visibilityMgr.(*pinotVisibilityMetricsClient)

			// mock behaviors
			mockMetricClient.On("Scope", mock.Anything, mock.Anything).Return(mockScope).Once()
			mockScope.On("StartTimer", mock.Anything, mock.Anything).Return(testStopwatch).Once()
			test.pinotClientMockAffordance(mockPinotClient)
			test.scopeMockAffordance(mockScope)

			_, err := metricsClient.CountWorkflowExecutions(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Equal(t, test.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMetricClientDeleteWorkflowExecution(t *testing.T) {
	// test non-empty request fields match
	errorRequest := &p.VisibilityDeleteWorkflowExecutionRequest{}

	request := &p.VisibilityDeleteWorkflowExecutionRequest{}

	tests := map[string]struct {
		request                *p.VisibilityDeleteWorkflowExecutionRequest
		producerMockAffordance func(mockProducer *mocks.KafkaProducer)
		scopeMockAffordance    func(mockScope *metricsClientMocks.Scope)
		expectedError          error
	}{
		"Case1: error case": {
			request: errorRequest,
			producerMockAffordance: func(mockProducer *mocks.KafkaProducer) {
				mockProducer.On("Publish", mock.Anything, mock.MatchedBy(func(input *indexer.PinotMessage) bool {
					return true
				})).Return(fmt.Errorf("error")).Once()
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Times(3)
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			request: request,
			producerMockAffordance: func(mockProducer *mocks.KafkaProducer) {
				mockProducer.On("Publish", mock.Anything, mock.MatchedBy(func(input *indexer.PinotMessage) bool {
					return true
				})).Return(nil).Once()
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Once()
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// create mock clients
			ctrl := gomock.NewController(t)
			mockPinotClient := pnt.NewMockGenericClient(ctrl)
			mockProducer := &mocks.KafkaProducer{}
			mockMetricClient := &metricsClientMocks.Client{}
			mockScope := &metricsClientMocks.Scope{}

			// create metricClient
			logger := log.NewNoop()
			mgr := NewPinotVisibilityStore(mockPinotClient, &service.Config{
				ValidSearchAttributes:  dynamicproperties.GetMapPropertyFn(definition.GetDefaultIndexedKeys()),
				ESIndexMaxResultWindow: dynamicproperties.GetIntPropertyFn(3),
			}, mockProducer, testlogger.New(t))
			visibilityStore := mgr.(*pinotVisibilityStore)
			pinotVisibilityManager := p.NewVisibilityManagerImpl(visibilityStore, logger, &persistence.DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})
			visibilityMgr := NewPinotVisibilityMetricsClient(pinotVisibilityManager, mockMetricClient, logger)
			metricsClient := visibilityMgr.(*pinotVisibilityMetricsClient)

			// mock behaviors
			mockMetricClient.On("Scope", mock.Anything, mock.Anything).Return(mockScope).Once()
			mockScope.On("StartTimer", mock.Anything, mock.Anything).Return(testStopwatch).Once()
			test.producerMockAffordance(mockProducer)
			test.scopeMockAffordance(mockScope)

			err := metricsClient.DeleteWorkflowExecution(context.Background(), test.request)
			assert.Equal(t, err, test.expectedError)
		})
	}
}

func TestMetricClientDeleteUninitializedWorkflowExecution(t *testing.T) {
	// test non-empty request fields match
	errorRequest := &p.VisibilityDeleteWorkflowExecutionRequest{}

	request := &p.VisibilityDeleteWorkflowExecutionRequest{}

	tests := map[string]struct {
		request                *p.VisibilityDeleteWorkflowExecutionRequest
		producerMockAffordance func(mockProducer *mocks.KafkaProducer)
		scopeMockAffordance    func(mockScope *metricsClientMocks.Scope)
		expectedError          error
	}{
		"Case1: error case": {
			request: errorRequest,
			producerMockAffordance: func(mockProducer *mocks.KafkaProducer) {
				mockProducer.On("Publish", mock.Anything, mock.MatchedBy(func(input *indexer.PinotMessage) bool {
					return true
				})).Return(fmt.Errorf("error")).Once()
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Times(3)
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			request: request,
			producerMockAffordance: func(mockProducer *mocks.KafkaProducer) {
				mockProducer.On("Publish", mock.Anything, mock.MatchedBy(func(input *indexer.PinotMessage) bool {
					return true
				})).Return(nil).Once()
			},
			scopeMockAffordance: func(mockScope *metricsClientMocks.Scope) {
				mockScope.On("IncCounter", mock.Anything, mock.Anything, mock.Anything).Return().Once()
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// create mock clients
			ctrl := gomock.NewController(t)
			mockPinotClient := pnt.NewMockGenericClient(ctrl)
			mockProducer := &mocks.KafkaProducer{}
			mockMetricClient := &metricsClientMocks.Client{}
			mockScope := &metricsClientMocks.Scope{}

			// create metricClient
			logger := log.NewNoop()
			mgr := NewPinotVisibilityStore(mockPinotClient, &service.Config{
				ValidSearchAttributes:  dynamicproperties.GetMapPropertyFn(definition.GetDefaultIndexedKeys()),
				ESIndexMaxResultWindow: dynamicproperties.GetIntPropertyFn(3),
			}, mockProducer, testlogger.New(t))
			visibilityStore := mgr.(*pinotVisibilityStore)
			pinotVisibilityManager := p.NewVisibilityManagerImpl(visibilityStore, logger, &persistence.DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})
			visibilityMgr := NewPinotVisibilityMetricsClient(pinotVisibilityManager, mockMetricClient, logger)
			metricsClient := visibilityMgr.(*pinotVisibilityMetricsClient)

			// mock behaviors
			mockMetricClient.On("Scope", mock.Anything, mock.Anything).Return(mockScope).Once()
			mockScope.On("StartTimer", mock.Anything, mock.Anything).Return(testStopwatch).Once()
			test.producerMockAffordance(mockProducer)
			test.scopeMockAffordance(mockScope)

			err := metricsClient.DeleteUninitializedWorkflowExecution(context.Background(), test.request)
			assert.Equal(t, err, test.expectedError)
		})
	}
}

func TestMetricClientClose(t *testing.T) {
	// create mock clients
	ctrl := gomock.NewController(t)
	mockPinotClient := pnt.NewMockGenericClient(ctrl)
	mockProducer := &mocks.KafkaProducer{}
	mockMetricClient := &metricsClientMocks.Client{}

	// create metricClient
	logger := log.NewNoop()
	mgr := NewPinotVisibilityStore(mockPinotClient, &service.Config{
		ValidSearchAttributes:  dynamicproperties.GetMapPropertyFn(definition.GetDefaultIndexedKeys()),
		ESIndexMaxResultWindow: dynamicproperties.GetIntPropertyFn(3),
	}, mockProducer, testlogger.New(t))
	visibilityStore := mgr.(*pinotVisibilityStore)
	pinotVisibilityManager := p.NewVisibilityManagerImpl(visibilityStore, logger, &persistence.DynamicConfiguration{
		SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
	})
	visibilityMgr := NewPinotVisibilityMetricsClient(pinotVisibilityManager, mockMetricClient, logger)
	metricsClient := visibilityMgr.(*pinotVisibilityMetricsClient)

	assert.NotPanics(t, func() {
		metricsClient.Close()

	})
}

func TestMetricClientGetName(t *testing.T) {
	// create mock clients
	ctrl := gomock.NewController(t)
	mockPinotClient := pnt.NewMockGenericClient(ctrl)
	mockProducer := &mocks.KafkaProducer{}
	mockMetricClient := &metricsClientMocks.Client{}

	// create metricClient
	logger := log.NewNoop()
	mgr := NewPinotVisibilityStore(mockPinotClient, &service.Config{
		ValidSearchAttributes:  dynamicproperties.GetMapPropertyFn(definition.GetDefaultIndexedKeys()),
		ESIndexMaxResultWindow: dynamicproperties.GetIntPropertyFn(3),
	}, mockProducer, testlogger.New(t))
	visibilityStore := mgr.(*pinotVisibilityStore)
	pinotVisibilityManager := p.NewVisibilityManagerImpl(visibilityStore, logger, &persistence.DynamicConfiguration{
		SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
	})
	visibilityMgr := NewPinotVisibilityMetricsClient(pinotVisibilityManager, mockMetricClient, logger)
	metricsClient := visibilityMgr.(*pinotVisibilityMetricsClient)

	assert.NotPanics(t, func() {
		metricsClient.GetName()

	})
}
