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

package persistence

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/types"
)

func TestNewVisibilityManagerImpl(t *testing.T) {
	tests := map[string]struct {
		name string
	}{
		"Case1: success case": {
			name: "TestNewVisibilityManagerImpl",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockVisibilityStore := NewMockVisibilityStore(ctrl)
			assert.NotPanics(t, func() {
				NewVisibilityManagerImpl(mockVisibilityStore, log.NewNoop(), &DynamicConfiguration{
					SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
				})
			})
		})
	}
}

func TestClose(t *testing.T) {
	tests := map[string]struct {
		name string
	}{
		"Case1: success case": {
			name: "TestNewVisibilityManagerImpl",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockVisibilityStore := NewMockVisibilityStore(ctrl)
			mockVisibilityStore.EXPECT().Close().Return().Times(1)
			visibilityManager := NewVisibilityManagerImpl(mockVisibilityStore, log.NewNoop(), &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})
			assert.NotPanics(t, func() {
				visibilityManager.Close()
			})
		})
	}
}

func TestGetName(t *testing.T) {
	tests := map[string]struct {
		name string
	}{
		"Case1: success case": {
			name: "TestNewVisibilityManagerImpl",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockVisibilityStore := NewMockVisibilityStore(ctrl)
			mockVisibilityStore.EXPECT().GetName().Return(testTableName).Times(1)
			visibilityManager := NewVisibilityManagerImpl(mockVisibilityStore, log.NewNoop(), &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})

			assert.NotPanics(t, func() {
				visibilityManager.GetName()
			})
		})
	}
}

func TestRecordWorkflowExecutionStarted(t *testing.T) {
	errorRequest := &RecordWorkflowExecutionStartedRequest{
		Execution: types.WorkflowExecution{
			WorkflowID: "err-wid",
			RunID:      "err-rid",
		},
		Memo: &types.Memo{
			Fields: map[string][]byte{
				"Service": []byte("servername1"),
			},
		},
		SearchAttributes: map[string][]byte{
			"CustomStringField": []byte("test string"),
			"CustomTimeField":   []byte("2020-01-01T00:00:00Z"),
		},
	}

	request := &RecordWorkflowExecutionStartedRequest{
		Execution: types.WorkflowExecution{
			WorkflowID: "wid",
			RunID:      "rid",
		},
		Memo: &types.Memo{},
		SearchAttributes: map[string][]byte{
			"CustomStringField": []byte("test string"),
			"CustomTimeField":   []byte("2020-01-01T00:00:00Z"),
		},
	}

	tests := map[string]struct {
		request                   *RecordWorkflowExecutionStartedRequest
		visibilityStoreAffordance func(mockVisibilityStore *MockVisibilityStore)
		expectedError             error
	}{
		"Case1: error case": {
			request: errorRequest,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().RecordWorkflowExecutionStarted(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error")).Times(1)
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			request: request,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().RecordWorkflowExecutionStarted(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockVisibilityStore := NewMockVisibilityStore(ctrl)
			visibilityManager := NewVisibilityManagerImpl(mockVisibilityStore, log.NewNoop(), &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})

			test.visibilityStoreAffordance(mockVisibilityStore)

			err := visibilityManager.RecordWorkflowExecutionStarted(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRecordWorkflowExecutionClosed(t *testing.T) {
	errorRequest := &RecordWorkflowExecutionClosedRequest{
		Execution: types.WorkflowExecution{
			WorkflowID: "err-wid",
			RunID:      "err-rid",
		},
		Memo: &types.Memo{},
		SearchAttributes: map[string][]byte{
			"CustomStringField": []byte("test string"),
			"CustomTimeField":   []byte("2020-01-01T00:00:00Z"),
		},
	}

	request := &RecordWorkflowExecutionClosedRequest{
		Execution: types.WorkflowExecution{
			WorkflowID: "wid",
			RunID:      "rid",
		},
		Memo: &types.Memo{},
		SearchAttributes: map[string][]byte{
			"CustomStringField": []byte("test string"),
			"CustomTimeField":   []byte("2020-01-01T00:00:00Z"),
		},
	}

	tests := map[string]struct {
		request                   *RecordWorkflowExecutionClosedRequest
		visibilityStoreAffordance func(mockVisibilityStore *MockVisibilityStore)
		expectedError             error
	}{
		"Case1: error case": {
			request: errorRequest,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error")).Times(1)
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			request: request,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockVisibilityStore := NewMockVisibilityStore(ctrl)
			visibilityManager := NewVisibilityManagerImpl(mockVisibilityStore, log.NewNoop(), &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})

			test.visibilityStoreAffordance(mockVisibilityStore)

			err := visibilityManager.RecordWorkflowExecutionClosed(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRecordWorkflowExecutionUninitialized(t *testing.T) {
	errorRequest := &RecordWorkflowExecutionUninitializedRequest{
		Execution: types.WorkflowExecution{
			WorkflowID: "err-wid",
			RunID:      "err-rid",
		},
	}

	request := &RecordWorkflowExecutionUninitializedRequest{
		Execution: types.WorkflowExecution{
			WorkflowID: "wid",
			RunID:      "rid",
		},
	}

	tests := map[string]struct {
		request                   *RecordWorkflowExecutionUninitializedRequest
		visibilityStoreAffordance func(mockVisibilityStore *MockVisibilityStore)
		expectedError             error
	}{
		"Case1: error case": {
			request: errorRequest,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().RecordWorkflowExecutionUninitialized(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error")).Times(1)
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			request: request,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().RecordWorkflowExecutionUninitialized(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockVisibilityStore := NewMockVisibilityStore(ctrl)
			visibilityManager := NewVisibilityManagerImpl(mockVisibilityStore, log.NewNoop(), &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})

			test.visibilityStoreAffordance(mockVisibilityStore)

			err := visibilityManager.RecordWorkflowExecutionUninitialized(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUpsertWorkflowExecution(t *testing.T) {
	errorRequest := &UpsertWorkflowExecutionRequest{
		Execution: types.WorkflowExecution{
			WorkflowID: "err-wid",
			RunID:      "err-rid",
		},
		Memo: &types.Memo{},
		SearchAttributes: map[string][]byte{
			"CustomStringField": []byte("test string"),
			"CustomTimeField":   []byte("2020-01-01T00:00:00Z"),
		},
	}

	request := &UpsertWorkflowExecutionRequest{
		Execution: types.WorkflowExecution{
			WorkflowID: "wid",
			RunID:      "rid",
		},
		Memo: &types.Memo{},
		SearchAttributes: map[string][]byte{
			"CustomStringField": []byte("test string"),
			"CustomTimeField":   []byte("2020-01-01T00:00:00Z"),
		},
	}

	tests := map[string]struct {
		request                   *UpsertWorkflowExecutionRequest
		visibilityStoreAffordance func(mockVisibilityStore *MockVisibilityStore)
		expectedError             error
	}{
		"Case1: error case": {
			request: errorRequest,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().UpsertWorkflowExecution(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error")).Times(1)
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			request: request,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().UpsertWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockVisibilityStore := NewMockVisibilityStore(ctrl)
			visibilityManager := NewVisibilityManagerImpl(mockVisibilityStore, log.NewNoop(), &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})

			test.visibilityStoreAffordance(mockVisibilityStore)

			err := visibilityManager.UpsertWorkflowExecution(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDeleteWorkflowExecution(t *testing.T) {
	errorRequest := &VisibilityDeleteWorkflowExecutionRequest{}

	request := &VisibilityDeleteWorkflowExecutionRequest{}

	tests := map[string]struct {
		request                   *VisibilityDeleteWorkflowExecutionRequest
		visibilityStoreAffordance func(mockVisibilityStore *MockVisibilityStore)
		expectedError             error
	}{
		"Case1: error case": {
			request: errorRequest,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().DeleteWorkflowExecution(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error")).Times(1)
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			request: request,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().DeleteWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockVisibilityStore := NewMockVisibilityStore(ctrl)
			visibilityManager := NewVisibilityManagerImpl(mockVisibilityStore, log.NewNoop(), &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})

			test.visibilityStoreAffordance(mockVisibilityStore)

			err := visibilityManager.DeleteWorkflowExecution(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDeleteUninitializedWorkflowExecution(t *testing.T) {
	errorRequest := &VisibilityDeleteWorkflowExecutionRequest{}

	request := &VisibilityDeleteWorkflowExecutionRequest{}

	tests := map[string]struct {
		request                   *VisibilityDeleteWorkflowExecutionRequest
		visibilityStoreAffordance func(mockVisibilityStore *MockVisibilityStore)
		expectedError             error
	}{
		"Case1: error case": {
			request: errorRequest,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().DeleteUninitializedWorkflowExecution(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error")).Times(1)
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			request: request,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().DeleteUninitializedWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockVisibilityStore := NewMockVisibilityStore(ctrl)
			visibilityManager := NewVisibilityManagerImpl(mockVisibilityStore, log.NewNoop(), &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})

			test.visibilityStoreAffordance(mockVisibilityStore)

			err := visibilityManager.DeleteUninitializedWorkflowExecution(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestListOpenWorkflowExecutions(t *testing.T) {
	errorRequest := &ListWorkflowExecutionsRequest{}

	request := &ListWorkflowExecutionsRequest{}

	tests := map[string]struct {
		request                   *ListWorkflowExecutionsRequest
		visibilityStoreAffordance func(mockVisibilityStore *MockVisibilityStore)
		expectedError             error
	}{
		"Case1: error case": {
			request: errorRequest,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().ListOpenWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error")).Times(1)
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			request: request,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().ListOpenWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockVisibilityStore := NewMockVisibilityStore(ctrl)
			visibilityManager := NewVisibilityManagerImpl(mockVisibilityStore, log.NewNoop(), &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})

			test.visibilityStoreAffordance(mockVisibilityStore)

			_, err := visibilityManager.ListOpenWorkflowExecutions(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestListClosedWorkflowExecutions(t *testing.T) {
	errorRequest := &ListWorkflowExecutionsRequest{}

	request := &ListWorkflowExecutionsRequest{}

	tests := map[string]struct {
		request                   *ListWorkflowExecutionsRequest
		visibilityStoreAffordance func(mockVisibilityStore *MockVisibilityStore)
		expectedError             error
	}{
		"Case1: error case": {
			request: errorRequest,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error")).Times(1)
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			request: request,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockVisibilityStore := NewMockVisibilityStore(ctrl)
			visibilityManager := NewVisibilityManagerImpl(mockVisibilityStore, log.NewNoop(), &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})

			test.visibilityStoreAffordance(mockVisibilityStore)

			_, err := visibilityManager.ListClosedWorkflowExecutions(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestListOpenWorkflowExecutionsByType(t *testing.T) {
	errorRequest := &ListWorkflowExecutionsByTypeRequest{}

	request := &ListWorkflowExecutionsByTypeRequest{}

	tests := map[string]struct {
		request                   *ListWorkflowExecutionsByTypeRequest
		visibilityStoreAffordance func(mockVisibilityStore *MockVisibilityStore)
		expectedError             error
	}{
		"Case1: error case": {
			request: errorRequest,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().ListOpenWorkflowExecutionsByType(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error")).Times(1)
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			request: request,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().ListOpenWorkflowExecutionsByType(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockVisibilityStore := NewMockVisibilityStore(ctrl)
			visibilityManager := NewVisibilityManagerImpl(mockVisibilityStore, log.NewNoop(), &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})

			test.visibilityStoreAffordance(mockVisibilityStore)

			_, err := visibilityManager.ListOpenWorkflowExecutionsByType(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTestListClosedWorkflowExecutionsByType(t *testing.T) {
	errorRequest := &ListWorkflowExecutionsByTypeRequest{}

	request := &ListWorkflowExecutionsByTypeRequest{}

	tests := map[string]struct {
		request                   *ListWorkflowExecutionsByTypeRequest
		visibilityStoreAffordance func(mockVisibilityStore *MockVisibilityStore)
		expectedError             error
	}{
		"Case1: error case": {
			request: errorRequest,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().ListClosedWorkflowExecutionsByType(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error")).Times(1)
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			request: request,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().ListClosedWorkflowExecutionsByType(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockVisibilityStore := NewMockVisibilityStore(ctrl)
			visibilityManager := NewVisibilityManagerImpl(mockVisibilityStore, log.NewNoop(), &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})

			test.visibilityStoreAffordance(mockVisibilityStore)

			_, err := visibilityManager.ListClosedWorkflowExecutionsByType(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestListOpenWorkflowExecutionsByWorkflowID(t *testing.T) {
	errorRequest := &ListWorkflowExecutionsByWorkflowIDRequest{}

	request := &ListWorkflowExecutionsByWorkflowIDRequest{}

	tests := map[string]struct {
		request                   *ListWorkflowExecutionsByWorkflowIDRequest
		visibilityStoreAffordance func(mockVisibilityStore *MockVisibilityStore)
		expectedError             error
	}{
		"Case1: error case": {
			request: errorRequest,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().ListOpenWorkflowExecutionsByWorkflowID(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error")).Times(1)
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			request: request,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().ListOpenWorkflowExecutionsByWorkflowID(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockVisibilityStore := NewMockVisibilityStore(ctrl)
			visibilityManager := NewVisibilityManagerImpl(mockVisibilityStore, log.NewNoop(), &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})

			test.visibilityStoreAffordance(mockVisibilityStore)

			_, err := visibilityManager.ListOpenWorkflowExecutionsByWorkflowID(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestListClosedWorkflowExecutionsByWorkflowID(t *testing.T) {
	errorRequest := &ListWorkflowExecutionsByWorkflowIDRequest{}

	request := &ListWorkflowExecutionsByWorkflowIDRequest{}

	tests := map[string]struct {
		request                   *ListWorkflowExecutionsByWorkflowIDRequest
		visibilityStoreAffordance func(mockVisibilityStore *MockVisibilityStore)
		expectedError             error
	}{
		"Case1: error case": {
			request: errorRequest,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().ListClosedWorkflowExecutionsByWorkflowID(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error")).Times(1)
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			request: request,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().ListClosedWorkflowExecutionsByWorkflowID(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockVisibilityStore := NewMockVisibilityStore(ctrl)
			visibilityManager := NewVisibilityManagerImpl(mockVisibilityStore, log.NewNoop(), &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})

			test.visibilityStoreAffordance(mockVisibilityStore)

			_, err := visibilityManager.ListClosedWorkflowExecutionsByWorkflowID(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestListClosedWorkflowExecutionsByStatus(t *testing.T) {
	errorRequest := &ListClosedWorkflowExecutionsByStatusRequest{}

	request := &ListClosedWorkflowExecutionsByStatusRequest{}

	tests := map[string]struct {
		request                   *ListClosedWorkflowExecutionsByStatusRequest
		visibilityStoreAffordance func(mockVisibilityStore *MockVisibilityStore)
		expectedError             error
	}{
		"Case1: error case": {
			request: errorRequest,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().ListClosedWorkflowExecutionsByStatus(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error")).Times(1)
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			request: request,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().ListClosedWorkflowExecutionsByStatus(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockVisibilityStore := NewMockVisibilityStore(ctrl)
			visibilityManager := NewVisibilityManagerImpl(mockVisibilityStore, log.NewNoop(), &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})

			test.visibilityStoreAffordance(mockVisibilityStore)

			_, err := visibilityManager.ListClosedWorkflowExecutionsByStatus(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetClosedWorkflowExecution(t *testing.T) {
	errorRequest := &GetClosedWorkflowExecutionRequest{}

	request := &GetClosedWorkflowExecutionRequest{}

	tests := map[string]struct {
		request                   *GetClosedWorkflowExecutionRequest
		visibilityStoreAffordance func(mockVisibilityStore *MockVisibilityStore)
		expectedError             error
	}{
		"Case1: error case": {
			request: errorRequest,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().GetClosedWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error")).Times(1)
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			request: request,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().GetClosedWorkflowExecution(gomock.Any(), gomock.Any()).Return(
					&InternalGetClosedWorkflowExecutionResponse{
						Execution: &InternalVisibilityWorkflowExecutionInfo{
							Memo: &DataBlob{
								Data: []byte("test string"),
							},
						},
					}, nil).Times(1)
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockVisibilityStore := NewMockVisibilityStore(ctrl)
			visibilityManager := NewVisibilityManagerImpl(mockVisibilityStore, log.NewNoop(), &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})

			test.visibilityStoreAffordance(mockVisibilityStore)

			_, err := visibilityManager.GetClosedWorkflowExecution(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestListWorkflowExecutions(t *testing.T) {
	errorRequest := &ListWorkflowExecutionsByQueryRequest{}

	request := &ListWorkflowExecutionsByQueryRequest{}

	testStatus := types.WorkflowExecutionCloseStatus(2)

	tests := map[string]struct {
		request                   *ListWorkflowExecutionsByQueryRequest
		visibilityStoreAffordance func(mockVisibilityStore *MockVisibilityStore)
		expectedError             error
	}{
		"Case1: error case": {
			request: errorRequest,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().ListWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error")).Times(1)
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			request: request,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().ListWorkflowExecutions(gomock.Any(), gomock.Any()).Return(
					&InternalListWorkflowExecutionsResponse{
						Executions: []*InternalVisibilityWorkflowExecutionInfo{
							{
								Status: &testStatus,
								Memo: &DataBlob{
									Data: []byte("test string"),
								},
								SearchAttributes: map[string]interface{}{
									"CustomStringField": []byte("test string"),
									"CustomTimeField":   []byte("2020-01-01T00:00:00Z"),
								},
							},
						}}, nil).Times(1)
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockVisibilityStore := NewMockVisibilityStore(ctrl)
			visibilityManager := NewVisibilityManagerImpl(mockVisibilityStore, log.NewNoop(), &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})

			test.visibilityStoreAffordance(mockVisibilityStore)

			_, err := visibilityManager.ListWorkflowExecutions(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestScanWorkflowExecutions(t *testing.T) {
	errorRequest := &ListWorkflowExecutionsByQueryRequest{}

	request := &ListWorkflowExecutionsByQueryRequest{}

	tests := map[string]struct {
		request                   *ListWorkflowExecutionsByQueryRequest
		visibilityStoreAffordance func(mockVisibilityStore *MockVisibilityStore)
		expectedError             error
	}{
		"Case1: error case": {
			request: errorRequest,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().ScanWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error")).Times(1)
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			request: request,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().ScanWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockVisibilityStore := NewMockVisibilityStore(ctrl)
			visibilityManager := NewVisibilityManagerImpl(mockVisibilityStore, log.NewNoop(), &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})

			test.visibilityStoreAffordance(mockVisibilityStore)

			_, err := visibilityManager.ScanWorkflowExecutions(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCountWorkflowExecutions(t *testing.T) {
	errorRequest := &CountWorkflowExecutionsRequest{}

	request := &CountWorkflowExecutionsRequest{}

	tests := map[string]struct {
		request                   *CountWorkflowExecutionsRequest
		visibilityStoreAffordance func(mockVisibilityStore *MockVisibilityStore)
		expectedError             error
	}{
		"Case1: error case": {
			request: errorRequest,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error")).Times(1)
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			request: request,
			visibilityStoreAffordance: func(mockVisibilityStore *MockVisibilityStore) {
				mockVisibilityStore.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockVisibilityStore := NewMockVisibilityStore(ctrl)
			visibilityManager := NewVisibilityManagerImpl(mockVisibilityStore, log.NewNoop(), &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})

			test.visibilityStoreAffordance(mockVisibilityStore)

			_, err := visibilityManager.CountWorkflowExecutions(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetSearchAttributes(t *testing.T) {
	tests := map[string]struct {
		input          *map[string]interface{}
		expectedOutput *types.SearchAttributes
		expectedError  error
	}{
		"Case1: error case": {
			input: &map[string]interface{}{
				"CustomStringField": make(chan int),
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case2: normal case": {
			input:         &map[string]interface{}{},
			expectedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockVisibilityStore := NewMockVisibilityStore(ctrl)
			visibilityManager := NewVisibilityManagerImpl(mockVisibilityStore, log.NewNoop(), &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})
			visibilityManagerImpl := visibilityManager.(*visibilityManagerImpl)

			actualOutput, actualErr := visibilityManagerImpl.getSearchAttributes(*test.input)
			if test.expectedError != nil {
				assert.Error(t, actualErr)
			} else {
				assert.NoError(t, actualErr)
				assert.NotNil(t, actualOutput)
			}
		})
	}
}

func TestConvertVisibilityWorkflowExecutionInfo(t *testing.T) {
	testExecutionTime := int64(0)
	testStartTime := time.Now()
	testResultTime := testStartTime.UnixNano()

	tests := map[string]struct {
		input          *InternalVisibilityWorkflowExecutionInfo
		expectedOutput *types.WorkflowExecutionInfo
	}{
		"Case1: error case": {
			input: &InternalVisibilityWorkflowExecutionInfo{
				SearchAttributes: map[string]interface{}{
					"CustomStringField": make(chan int),
				},
				ExecutionTime: time.UnixMilli(int64(testExecutionTime)),
				StartTime:     testStartTime,
			},
			expectedOutput: &types.WorkflowExecutionInfo{
				ExecutionTime: &testResultTime,
			},
		},
		"Case2: normal case with Execution Time is 0": {
			input: &InternalVisibilityWorkflowExecutionInfo{
				ExecutionTime: time.UnixMilli(int64(testExecutionTime)),
				StartTime:     testStartTime,
			},
			expectedOutput: &types.WorkflowExecutionInfo{
				ExecutionTime: &testResultTime,
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockVisibilityStore := NewMockVisibilityStore(ctrl)
			visibilityManager := NewVisibilityManagerImpl(mockVisibilityStore, log.NewNoop(), &DynamicConfiguration{
				SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			})
			visibilityManagerImpl := visibilityManager.(*visibilityManagerImpl)

			actualOutput := visibilityManagerImpl.convertVisibilityWorkflowExecutionInfo(test.input)
			assert.Equal(t, *test.expectedOutput.ExecutionTime, *actualOutput.ExecutionTime)
		})
	}
}

func TestToInternalListWorkflowExecutionsRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockVisibilityStore := NewMockVisibilityStore(ctrl)
	visibilityManager := NewVisibilityManagerImpl(mockVisibilityStore, log.NewNoop(), &DynamicConfiguration{
		SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
	})
	visibilityManagerImpl := visibilityManager.(*visibilityManagerImpl)

	assert.Nil(t, visibilityManagerImpl.toInternalListWorkflowExecutionsRequest(nil))
}

func TestSerializeMemo(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockVisibilityStore := NewMockVisibilityStore(ctrl)
	mockPayloadSerializer := NewMockPayloadSerializer(ctrl)
	mockPayloadSerializer.EXPECT().SerializeVisibilityMemo(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error")).Times(1)
	visibilityManager := NewVisibilityManagerImpl(mockVisibilityStore, log.NewNoop(), &DynamicConfiguration{
		SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
	})
	visibilityManagerImpl := visibilityManager.(*visibilityManagerImpl)
	visibilityManagerImpl.serializer = mockPayloadSerializer
	assert.NotPanics(t, func() {
		visibilityManagerImpl.serializeMemo(nil, "testDomain", "testWorkflowID", "testRunID")
	})
}
