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

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
)

func TestNewPinotVisibilityTripleManager(t *testing.T) {
	// put this outside because need to use it as an input of the table tests
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		mockDBVisibilityManager    VisibilityManager
		mockESVisibilityManager    VisibilityManager
		mockPinotVisibilityManager VisibilityManager
	}{
		"Case1: nil case": {
			mockDBVisibilityManager:    nil,
			mockESVisibilityManager:    nil,
			mockPinotVisibilityManager: nil,
		},
		"Case2: success case": {
			mockDBVisibilityManager:    NewMockVisibilityManager(ctrl),
			mockESVisibilityManager:    NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				NewPinotVisibilityTripleManager(test.mockDBVisibilityManager, test.mockPinotVisibilityManager,
					test.mockESVisibilityManager, nil, nil,
					nil, nil, log.NewNoop())
			})
		})
	}
}

func TestPinotTripleManagerClose(t *testing.T) {
	// put this outside because need to use it as an input of the table tests
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		mockDBVisibilityManager              VisibilityManager
		mockPinotVisibilityManager           VisibilityManager
		mockESVisibilityManager              VisibilityManager
		mockDBVisibilityManagerAccordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAccordance func(mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance    func(mockESVisibilityManager *MockVisibilityManager)
	}{
		"Case1-1: success case with DB visibility is not nil": {
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().Close().Return().Times(1)
			},
		},
		"Case1-2: success case with ES visibility is not nil": {
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().Close().Return().Times(1)
			},
		},
		"Case1-3: success case with pinot visibility is not nil": {
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().Close().Return().Times(1)
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAccordance(test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityManager := NewPinotVisibilityTripleManager(test.mockDBVisibilityManager, test.mockPinotVisibilityManager,
				test.mockESVisibilityManager, nil, nil,
				nil, nil, log.NewNoop())
			assert.NotPanics(t, func() {
				visibilityManager.Close()
			})
		})
	}
}

func TestPinotTripleManagerGetName(t *testing.T) {
	// put this outside because need to use it as an input of the table tests
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		mockDBVisibilityManager              VisibilityManager
		mockESVisibilityManager              VisibilityManager
		mockPinotVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAccordance func(mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance    func(mockESVisibilityManager *MockVisibilityManager)
	}{
		"Case1-1: success case with DB visibility is not nil": {
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().GetName().Return(testTableName).Times(1)

			},
		},
		"Case1-2: success case with ES visibility is not nil": {
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().GetName().Return(testTableName).Times(1)
			},
		},
		"Case1-3: success case with pinot visibility is not nil": {
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().GetName().Return(testTableName).Times(1)
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAccordance(test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}
			visibilityManager := NewPinotVisibilityTripleManager(test.mockDBVisibilityManager, test.mockPinotVisibilityManager,
				test.mockESVisibilityManager, nil, nil,
				nil, nil, log.NewNoop())

			assert.NotPanics(t, func() {
				visibilityManager.GetName()
			})
		})
	}
}

func TestPinotTripleRecordWorkflowExecutionStarted(t *testing.T) {
	request := &RecordWorkflowExecutionStartedRequest{}

	// put this outside because need to use it as an input of the table tests
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                              *RecordWorkflowExecutionStartedRequest
		mockDBVisibilityManager              VisibilityManager
		mockESVisibilityManager              VisibilityManager
		mockPinotVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAccordance func(mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance    func(mockESVisibilityManager *MockVisibilityManager)
		advancedVisibilityWritingMode        dynamicconfig.StringPropertyFn
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().RecordWorkflowExecutionStarted(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeOff),
			expectedError:                 nil,
		},
		"Case1-2: success case with ES visibility is not nil": {
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionStarted(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeOn),
			expectedError:                 nil,
		},
		"Case1-3: success case with pinot visibility is not nil": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionStarted(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeOn),
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAccordance(test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}
			visibilityManager := NewPinotVisibilityTripleManager(test.mockDBVisibilityManager, test.mockPinotVisibilityManager,
				test.mockESVisibilityManager, nil, nil,
				test.advancedVisibilityWritingMode, nil, log.NewNoop())

			err := visibilityManager.RecordWorkflowExecutionStarted(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPinotTripleRecordWorkflowExecutionClosed(t *testing.T) {
	request := &RecordWorkflowExecutionClosedRequest{}

	// put this outside because need to use it as an input of the table tests
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		context                              context.Context
		request                              *RecordWorkflowExecutionClosedRequest
		mockDBVisibilityManager              VisibilityManager
		mockESVisibilityManager              VisibilityManager
		mockPinotVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAccordance func(mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance    func(mockESVisibilityManager *MockVisibilityManager)
		advancedVisibilityWritingMode        dynamicconfig.StringPropertyFn
		expectedError                        error
	}{
		"Case0-1: error case with advancedVisibilityWritingMode is nil": {
			context:                 context.Background(),
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			expectedError: fmt.Errorf("error"),
		},
		"Case0-2: error case with ES has errors in dual mode": {
			context:                 context.Background(),
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error")).AnyTimes()
			},
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error")).AnyTimes()
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeDual),
			expectedError:                 fmt.Errorf("error"),
		},
		"Case0-3: error case with ES has errors in On mode with Pinot is not nil": {
			context:                 context.Background(),
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error")).AnyTimes()
			},
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error")).AnyTimes()
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeOn),
			expectedError:                 fmt.Errorf("error"),
		},
		"Case0-4: error case with Pinot has errors in On mode": {
			context:                 context.Background(),
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error")).AnyTimes()
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeOn),
			expectedError:                 fmt.Errorf("error"),
		},
		"Case0-5: error case with Pinot has errors in Triple mode": {
			context:                 context.Background(),
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error")).AnyTimes()
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeTriple),
			expectedError:                 fmt.Errorf("error"),
		},
		"Case0-6: error case with ES has errors in Triple mode": {
			context:                 context.Background(),
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error")).AnyTimes()
			},
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeTriple),
			expectedError:                 fmt.Errorf("error"),
		},
		"Case1-1: success case with DB visibility is not nil": {
			context:                 context.Background(),
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeOff),
			expectedError:                 nil,
		},
		"Case1-2: success case with ES visibility is not nil": {
			context:                 context.Background(),
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeOn),
			expectedError:                 nil,
		},
		"Case1-3: success case with pinot visibility is not nil": {
			context:                    context.Background(),
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeOn),
		},
		"Case1-4: success case with dual manager": {
			context:                 context.Background(),
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeDual),
			expectedError:                 nil,
		},
		"Case1-5: success case with dual manager when ES and Pinot are not nil": {
			context:                 context.Background(),
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeDual),
			expectedError:                 nil,
		},
		"Case1-6: success case with triple write when ES and Pinot are not nil": {
			context:                 context.Background(),
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeTriple),
			expectedError:                 nil,
		},
		"Case1-7: success case with triple write when ES and Pinot are nil": {
			context:                 context.Background(),
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeTriple),
			expectedError:                 nil,
		},
		"Case1-8: success case with triple write when db is nil": {
			context:                    context.Background(),
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeTriple),
			expectedError:                 nil,
		},
		"Case2-1: choose DB visibility manager when it is nil": {
			context:                    context.Background(),
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeOff),
		},
		"Case2-2: choose ES visibility manager when it is nil": {
			context:                 context.Background(),
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeOn),
		},
		"Case2-3: choose both when ES is nil": {
			context:                 context.Background(),
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeDual),
		},
		"Case2-4: choose both when DB is nil": {
			context:                    context.Background(),
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeDual),
		},
		"Case3-1: chooseVisibilityModeForAdmin when ES is nil": {
			context:                 context.WithValue(context.Background(), VisibilityAdminDeletionKey("visibilityAdminDelete"), true),
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
		},
		"Case3-2: chooseVisibilityModeForAdmin when DB is nil": {
			context:                    context.WithValue(context.Background(), VisibilityAdminDeletionKey("visibilityAdminDelete"), true),
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
		},
		"Case3-3: chooseVisibilityModeForAdmin when both are not nil": {
			context:                 context.WithValue(context.Background(), VisibilityAdminDeletionKey("visibilityAdminDelete"), true),
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
		},
		"Case3-3: chooseVisibilityModeForAdmin when triple are not nil": {
			context:                 context.WithValue(context.Background(), VisibilityAdminDeletionKey("visibilityAdminDelete"), true),
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAccordance(test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityManager := NewPinotVisibilityTripleManager(test.mockDBVisibilityManager, test.mockPinotVisibilityManager,
				test.mockESVisibilityManager, nil, nil,
				test.advancedVisibilityWritingMode, nil, log.NewNoop())

			err := visibilityManager.RecordWorkflowExecutionClosed(test.context, test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// test an edge case
func TestPinotTripleChooseVisibilityModeForAdmin(t *testing.T) {
	ctrl := gomock.NewController(t)
	dbManager := NewMockVisibilityManager(ctrl)
	esManager := NewMockVisibilityManager(ctrl)
	pntManager := NewMockVisibilityManager(ctrl)
	mgr := NewPinotVisibilityTripleManager(dbManager, pntManager, esManager, nil, nil,
		nil, nil, log.NewNoop())
	tripleManager := mgr.(*pinotVisibilityTripleManager)
	tripleManager.dbVisibilityManager = nil
	tripleManager.pinotVisibilityManager = nil
	tripleManager.esVisibilityManager = nil
	assert.Equal(t, "INVALID_ADMIN_MODE", tripleManager.chooseVisibilityModeForAdmin())
}

func TestPinotTripleRecordWorkflowExecutionUninitialized(t *testing.T) {
	request := &RecordWorkflowExecutionUninitializedRequest{}

	// put this outside because need to use it as an input of the table tests
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                              *RecordWorkflowExecutionUninitializedRequest
		mockDBVisibilityManager              VisibilityManager
		mockESVisibilityManager              VisibilityManager
		mockPinotVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAccordance func(mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance    func(mockESVisibilityManager *MockVisibilityManager)
		advancedVisibilityWritingMode        dynamicconfig.StringPropertyFn
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().RecordWorkflowExecutionUninitialized(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeOff),
			expectedError:                 nil,
		},
		"Case1-2: success case with Pinot visibility is not nil": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionUninitialized(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeOn),
			expectedError:                 nil,
		},
		"Case1-3: success case with ES visibility is not nil": {
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionUninitialized(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeOn),
			expectedError:                 nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAccordance(test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityManager := NewPinotVisibilityTripleManager(test.mockDBVisibilityManager, test.mockPinotVisibilityManager,
				test.mockESVisibilityManager, nil, nil,
				test.advancedVisibilityWritingMode, nil, log.NewNoop())

			err := visibilityManager.RecordWorkflowExecutionUninitialized(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPinotTripleUpsertWorkflowExecution(t *testing.T) {
	request := &UpsertWorkflowExecutionRequest{}

	// put this outside because need to use it as an input of the table tests
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                              *UpsertWorkflowExecutionRequest
		mockDBVisibilityManager              VisibilityManager
		mockESVisibilityManager              VisibilityManager
		mockPinotVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAccordance func(mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance    func(mockESVisibilityManager *MockVisibilityManager)
		advancedVisibilityWritingMode        dynamicconfig.StringPropertyFn
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().UpsertWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeOff),
			expectedError:                 nil,
		},
		"Case1-2: success case with Pinot visibility is not nil": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().UpsertWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeOn),
			expectedError:                 nil,
		},
		"Case1-3: success case with ES visibility is not nil": {
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().UpsertWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeOn),
			expectedError:                 nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAccordance(test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityManager := NewPinotVisibilityTripleManager(test.mockDBVisibilityManager, test.mockPinotVisibilityManager,
				test.mockESVisibilityManager, nil, nil,
				test.advancedVisibilityWritingMode, nil, log.NewNoop())

			err := visibilityManager.UpsertWorkflowExecution(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPinotTripleDeleteWorkflowExecution(t *testing.T) {
	request := &VisibilityDeleteWorkflowExecutionRequest{}

	// put this outside because need to use it as an input of the table tests
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                              *VisibilityDeleteWorkflowExecutionRequest
		mockDBVisibilityManager              VisibilityManager
		mockESVisibilityManager              VisibilityManager
		mockPinotVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAccordance func(mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance    func(mockESVisibilityManager *MockVisibilityManager)
		advancedVisibilityWritingMode        dynamicconfig.StringPropertyFn
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().DeleteWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeOff),
			expectedError:                 nil,
		},
		"Case1-2: success case with Pinot visibility is not nil": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().DeleteWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeOn),
			expectedError:                 nil,
		},
		"Case1-3: success case with ES visibility is not nil": {
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().DeleteWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeOn),
			expectedError:                 nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAccordance(test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityManager := NewPinotVisibilityTripleManager(test.mockDBVisibilityManager, test.mockPinotVisibilityManager,
				test.mockESVisibilityManager, nil, nil,
				test.advancedVisibilityWritingMode, nil, log.NewNoop())

			err := visibilityManager.DeleteWorkflowExecution(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPinotTripleDeleteUninitializedWorkflowExecution(t *testing.T) {
	request := &VisibilityDeleteWorkflowExecutionRequest{}

	// put this outside because need to use it as an input of the table tests
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                              *VisibilityDeleteWorkflowExecutionRequest
		mockDBVisibilityManager              VisibilityManager
		mockESVisibilityManager              VisibilityManager
		mockPinotVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAccordance func(mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance    func(mockESVisibilityManager *MockVisibilityManager)
		advancedVisibilityWritingMode        dynamicconfig.StringPropertyFn
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().DeleteUninitializedWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeOff),
			expectedError:                 nil,
		},
		"Case1-2: success case with Pinot visibility is not nil": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().DeleteUninitializedWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeOn),
			expectedError:                 nil,
		},
		"Case1-3: success case with ES visibility is not nil": {
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().DeleteUninitializedWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeOn),
			expectedError:                 nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAccordance(test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityManager := NewPinotVisibilityTripleManager(test.mockDBVisibilityManager, test.mockPinotVisibilityManager,
				test.mockESVisibilityManager, nil, nil,
				test.advancedVisibilityWritingMode, nil, log.NewNoop())

			err := visibilityManager.DeleteUninitializedWorkflowExecution(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFilterAttrPrefix(t *testing.T) {
	tests := map[string]struct {
		expectedInput  string
		expectedOutput string
	}{
		"Case1: empty input": {
			expectedInput:  "",
			expectedOutput: "",
		},
		"Case2: filtered input": {
			expectedInput:  "`Attr.CustomIntField` = 12",
			expectedOutput: "CustomIntField = 12",
		},
		"Case3: complex input": {
			expectedInput:  "WorkflowID = 'test-wf' and (`Attr.CustomIntField` = 12 or `Attr.CustomStringField` = 'a-b-c' and WorkflowType = 'wf-type')",
			expectedOutput: "WorkflowID = 'test-wf' and (CustomIntField = 12 or CustomStringField = 'a-b-c' and WorkflowType = 'wf-type')",
		},
		"Case4: false positive case": {
			expectedInput:  "`Attr.CustomStringField` = '`Attr.ABCtesting'",
			expectedOutput: "CustomStringField = 'ABCtesting'", // this is supposed to be CustomStringField = '`Attr.ABCtesting'
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				actualOutput := filterAttrPrefix(test.expectedInput)
				assert.Equal(t, test.expectedOutput, actualOutput)
			})
		})
	}
}

func TestPinotTripleListOpenWorkflowExecutions(t *testing.T) {
	request := &ListWorkflowExecutionsRequest{
		Domain: "test-domain",
	}

	// put this outside because need to use it as an input of the table tests
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                              *ListWorkflowExecutionsRequest
		mockDBVisibilityManager              VisibilityManager
		mockESVisibilityManager              VisibilityManager
		mockPinotVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAccordance func(mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance    func(mockESVisibilityManager *MockVisibilityManager)
		readModeIsFromES                     dynamicconfig.BoolPropertyFnWithDomainFilter
		readModeIsFromPinot                  dynamicconfig.BoolPropertyFnWithDomainFilter
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ListOpenWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetBoolPropertyFnFilteredByDomain(true),
			expectedError:    nil,
		},
		"Case1-2: success case with Pinot visibility is not nil": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListOpenWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES:    dynamicconfig.GetBoolPropertyFnFilteredByDomain(false),
			readModeIsFromPinot: dynamicconfig.GetBoolPropertyFnFilteredByDomain(true),
			expectedError:       nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAccordance(test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityManager := NewPinotVisibilityTripleManager(test.mockDBVisibilityManager, test.mockPinotVisibilityManager,
				test.mockESVisibilityManager, test.readModeIsFromPinot, test.readModeIsFromES,
				nil, dynamicconfig.GetBoolPropertyFnFilteredByDomain(true), log.NewNoop())

			_, err := visibilityManager.ListOpenWorkflowExecutions(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPinotTripleListClosedWorkflowExecutions(t *testing.T) {
	request := &ListWorkflowExecutionsRequest{
		Domain: "test-domain",
	}

	// put this outside because need to use it as an input of the table tests
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		context                              context.Context
		request                              *ListWorkflowExecutionsRequest
		mockDBVisibilityManager              VisibilityManager
		mockESVisibilityManager              VisibilityManager
		mockPinotVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAccordance func(mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance    func(mockESVisibilityManager *MockVisibilityManager)
		readModeIsFromES                     dynamicconfig.BoolPropertyFnWithDomainFilter
		readModeIsFromPinot                  dynamicconfig.BoolPropertyFnWithDomainFilter
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			context:                 context.Background(),
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetBoolPropertyFnFilteredByDomain(true),
			expectedError:    nil,
		},
		"Case1-2: success case with Pinot visibility is not nil": {
			context:                    context.Background(),
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES:    dynamicconfig.GetBoolPropertyFnFilteredByDomain(false),
			readModeIsFromPinot: dynamicconfig.GetBoolPropertyFnFilteredByDomain(true),
			expectedError:       nil,
		},
		"Case1-3: success case with ES visibility is not nil": {
			context:                 context.Background(),
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES:    dynamicconfig.GetBoolPropertyFnFilteredByDomain(true),
			readModeIsFromPinot: dynamicconfig.GetBoolPropertyFnFilteredByDomain(false),
			expectedError:       nil,
		},
		"Case2-1: success case with DB visibility is not nil and read Pinot is true": {
			context:                 context.Background(),
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES:    dynamicconfig.GetBoolPropertyFnFilteredByDomain(false),
			readModeIsFromPinot: dynamicconfig.GetBoolPropertyFnFilteredByDomain(true),
			expectedError:       nil,
		},
		"Case2-2: success case with Pinot visibility is not nil and read mod is false": {
			context:                    context.Background(),
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES:    dynamicconfig.GetBoolPropertyFnFilteredByDomain(false),
			readModeIsFromPinot: dynamicconfig.GetBoolPropertyFnFilteredByDomain(false),
			expectedError:       nil,
		},
		"Case2-3: success case with DB visibility is not nil and read modes are false": {
			context:                 context.Background(),
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES:    dynamicconfig.GetBoolPropertyFnFilteredByDomain(false),
			readModeIsFromPinot: dynamicconfig.GetBoolPropertyFnFilteredByDomain(false),
			expectedError:       nil,
		},
		"Case3-1: read from ES with context key": {
			context:                 context.WithValue(context.Background(), ContextKey, VisibilityOverridePrimary),
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
		},
		"Case3-2: read from Pinot with context key": {
			context:                    context.WithValue(context.Background(), ContextKey, VisibilityOverrideSecondary),
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAccordance(test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityManager := NewPinotVisibilityTripleManager(test.mockDBVisibilityManager, test.mockPinotVisibilityManager,
				test.mockESVisibilityManager, test.readModeIsFromPinot, test.readModeIsFromES,
				nil, dynamicconfig.GetBoolPropertyFnFilteredByDomain(true), log.NewNoop())

			_, err := visibilityManager.ListClosedWorkflowExecutions(test.context, test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPinotTripleListOpenWorkflowExecutionsByType(t *testing.T) {
	request := &ListWorkflowExecutionsByTypeRequest{
		ListWorkflowExecutionsRequest: ListWorkflowExecutionsRequest{
			Domain: "test-domain",
		},
	}

	// put this outside because need to use it as an input of the table tests
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                              *ListWorkflowExecutionsByTypeRequest
		mockDBVisibilityManager              VisibilityManager
		mockESVisibilityManager              VisibilityManager
		mockPinotVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAccordance func(mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance    func(mockESVisibilityManager *MockVisibilityManager)
		readModeIsFromES                     dynamicconfig.BoolPropertyFnWithDomainFilter
		readModeIsFromPinot                  dynamicconfig.BoolPropertyFnWithDomainFilter
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ListOpenWorkflowExecutionsByType(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetBoolPropertyFnFilteredByDomain(true),
			expectedError:    nil,
		},
		"Case1-2: success case with Pinot visibility is not nil": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListOpenWorkflowExecutionsByType(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES:    dynamicconfig.GetBoolPropertyFnFilteredByDomain(false),
			readModeIsFromPinot: dynamicconfig.GetBoolPropertyFnFilteredByDomain(true),
			expectedError:       nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAccordance(test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityManager := NewPinotVisibilityTripleManager(test.mockDBVisibilityManager, test.mockPinotVisibilityManager,
				test.mockESVisibilityManager, test.readModeIsFromPinot, test.readModeIsFromES,
				nil, dynamicconfig.GetBoolPropertyFnFilteredByDomain(true), log.NewNoop())

			_, err := visibilityManager.ListOpenWorkflowExecutionsByType(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPinotTripleListClosedWorkflowExecutionsByType(t *testing.T) {
	request := &ListWorkflowExecutionsByTypeRequest{
		ListWorkflowExecutionsRequest: ListWorkflowExecutionsRequest{
			Domain: "test-domain",
		},
	}

	// put this outside because need to use it as an input of the table tests
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                              *ListWorkflowExecutionsByTypeRequest
		mockDBVisibilityManager              VisibilityManager
		mockESVisibilityManager              VisibilityManager
		mockPinotVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAccordance func(mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance    func(mockESVisibilityManager *MockVisibilityManager)
		readModeIsFromES                     dynamicconfig.BoolPropertyFnWithDomainFilter
		readModeIsFromPinot                  dynamicconfig.BoolPropertyFnWithDomainFilter
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ListClosedWorkflowExecutionsByType(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetBoolPropertyFnFilteredByDomain(true),
			expectedError:    nil,
		},
		"Case1-2: success case with Pinot visibility is not nil": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListClosedWorkflowExecutionsByType(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES:    dynamicconfig.GetBoolPropertyFnFilteredByDomain(false),
			readModeIsFromPinot: dynamicconfig.GetBoolPropertyFnFilteredByDomain(true),
			expectedError:       nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAccordance(test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityManager := NewPinotVisibilityTripleManager(test.mockDBVisibilityManager, test.mockPinotVisibilityManager,
				test.mockESVisibilityManager, test.readModeIsFromPinot, test.readModeIsFromES,
				nil, dynamicconfig.GetBoolPropertyFnFilteredByDomain(true), log.NewNoop())

			_, err := visibilityManager.ListClosedWorkflowExecutionsByType(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPinotTripleListOpenWorkflowExecutionsByWorkflowID(t *testing.T) {
	request := &ListWorkflowExecutionsByWorkflowIDRequest{
		ListWorkflowExecutionsRequest: ListWorkflowExecutionsRequest{
			Domain: "test-domain",
		},
	}

	// put this outside because need to use it as an input of the table tests
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                              *ListWorkflowExecutionsByWorkflowIDRequest
		mockDBVisibilityManager              VisibilityManager
		mockESVisibilityManager              VisibilityManager
		mockPinotVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAccordance func(mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance    func(mockESVisibilityManager *MockVisibilityManager)
		readModeIsFromES                     dynamicconfig.BoolPropertyFnWithDomainFilter
		readModeIsFromPinot                  dynamicconfig.BoolPropertyFnWithDomainFilter
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ListOpenWorkflowExecutionsByWorkflowID(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetBoolPropertyFnFilteredByDomain(true),
			expectedError:    nil,
		},
		"Case1-2: success case with Pinot visibility is not nil": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListOpenWorkflowExecutionsByWorkflowID(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES:    dynamicconfig.GetBoolPropertyFnFilteredByDomain(false),
			readModeIsFromPinot: dynamicconfig.GetBoolPropertyFnFilteredByDomain(true),
			expectedError:       nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAccordance(test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityManager := NewPinotVisibilityTripleManager(test.mockDBVisibilityManager, test.mockPinotVisibilityManager,
				test.mockESVisibilityManager, test.readModeIsFromPinot, test.readModeIsFromES,
				nil, dynamicconfig.GetBoolPropertyFnFilteredByDomain(true), log.NewNoop())

			_, err := visibilityManager.ListOpenWorkflowExecutionsByWorkflowID(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPinotTripleListClosedWorkflowExecutionsByWorkflowID(t *testing.T) {
	request := &ListWorkflowExecutionsByWorkflowIDRequest{
		ListWorkflowExecutionsRequest: ListWorkflowExecutionsRequest{
			Domain: "test-domain",
		},
	}

	// put this outside because need to use it as an input of the table tests
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                              *ListWorkflowExecutionsByWorkflowIDRequest
		mockDBVisibilityManager              VisibilityManager
		mockESVisibilityManager              VisibilityManager
		mockPinotVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAccordance func(mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance    func(mockESVisibilityManager *MockVisibilityManager)
		readModeIsFromES                     dynamicconfig.BoolPropertyFnWithDomainFilter
		readModeIsFromPinot                  dynamicconfig.BoolPropertyFnWithDomainFilter
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ListClosedWorkflowExecutionsByWorkflowID(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetBoolPropertyFnFilteredByDomain(true),
			expectedError:    nil,
		},
		"Case1-2: success case with Pinot visibility is not nil": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListClosedWorkflowExecutionsByWorkflowID(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES:    dynamicconfig.GetBoolPropertyFnFilteredByDomain(false),
			readModeIsFromPinot: dynamicconfig.GetBoolPropertyFnFilteredByDomain(true),
			expectedError:       nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAccordance(test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityManager := NewPinotVisibilityTripleManager(test.mockDBVisibilityManager, test.mockPinotVisibilityManager,
				test.mockESVisibilityManager, test.readModeIsFromPinot, test.readModeIsFromES,
				nil, dynamicconfig.GetBoolPropertyFnFilteredByDomain(false), log.NewNoop())

			_, err := visibilityManager.ListClosedWorkflowExecutionsByWorkflowID(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPinotTripleListClosedWorkflowExecutionsByStatus(t *testing.T) {
	request := &ListClosedWorkflowExecutionsByStatusRequest{
		ListWorkflowExecutionsRequest: ListWorkflowExecutionsRequest{
			Domain: "test-domain",
		},
	}

	// put this outside because need to use it as an input of the table tests
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                              *ListClosedWorkflowExecutionsByStatusRequest
		mockDBVisibilityManager              VisibilityManager
		mockESVisibilityManager              VisibilityManager
		mockPinotVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAccordance func(mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance    func(mockESVisibilityManager *MockVisibilityManager)
		readModeIsFromES                     dynamicconfig.BoolPropertyFnWithDomainFilter
		readModeIsFromPinot                  dynamicconfig.BoolPropertyFnWithDomainFilter
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ListClosedWorkflowExecutionsByStatus(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetBoolPropertyFnFilteredByDomain(true),
			expectedError:    nil,
		},
		"Case1-2: success case with Pinot visibility is not nil": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListClosedWorkflowExecutionsByStatus(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES:    dynamicconfig.GetBoolPropertyFnFilteredByDomain(false),
			readModeIsFromPinot: dynamicconfig.GetBoolPropertyFnFilteredByDomain(true),
			expectedError:       nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAccordance(test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityManager := NewPinotVisibilityTripleManager(test.mockDBVisibilityManager, test.mockPinotVisibilityManager,
				test.mockESVisibilityManager, test.readModeIsFromPinot, test.readModeIsFromES,
				nil, dynamicconfig.GetBoolPropertyFnFilteredByDomain(true), log.NewNoop())

			_, err := visibilityManager.ListClosedWorkflowExecutionsByStatus(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPinotTripleGetClosedWorkflowExecution(t *testing.T) {
	request := &GetClosedWorkflowExecutionRequest{
		Domain: "test-domain",
	}

	// put this outside because need to use it as an input of the table tests
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                              *GetClosedWorkflowExecutionRequest
		mockDBVisibilityManager              VisibilityManager
		mockESVisibilityManager              VisibilityManager
		mockPinotVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAccordance func(mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance    func(mockESVisibilityManager *MockVisibilityManager)
		readModeIsFromES                     dynamicconfig.BoolPropertyFnWithDomainFilter
		readModeIsFromPinot                  dynamicconfig.BoolPropertyFnWithDomainFilter
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().GetClosedWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetBoolPropertyFnFilteredByDomain(true),
			expectedError:    nil,
		},
		"Case1-2: success case with Pinot visibility is not nil": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().GetClosedWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES:    dynamicconfig.GetBoolPropertyFnFilteredByDomain(false),
			readModeIsFromPinot: dynamicconfig.GetBoolPropertyFnFilteredByDomain(true),
			expectedError:       nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAccordance(test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityManager := NewPinotVisibilityTripleManager(test.mockDBVisibilityManager, test.mockPinotVisibilityManager,
				test.mockESVisibilityManager, test.readModeIsFromPinot, test.readModeIsFromES,
				nil, dynamicconfig.GetBoolPropertyFnFilteredByDomain(true), log.NewNoop())

			_, err := visibilityManager.GetClosedWorkflowExecution(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPinotTripleListWorkflowExecutions(t *testing.T) {
	request := &ListWorkflowExecutionsByQueryRequest{
		Domain: "test-domain",
	}

	// put this outside because need to use it as an input of the table tests
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                              *ListWorkflowExecutionsByQueryRequest
		mockDBVisibilityManager              VisibilityManager
		mockESVisibilityManager              VisibilityManager
		mockPinotVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAccordance func(mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance    func(mockESVisibilityManager *MockVisibilityManager)
		readModeIsFromES                     dynamicconfig.BoolPropertyFnWithDomainFilter
		readModeIsFromPinot                  dynamicconfig.BoolPropertyFnWithDomainFilter
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ListWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetBoolPropertyFnFilteredByDomain(true),
			expectedError:    nil,
		},
		"Case1-2: success case with Pinot visibility is not nil": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES:    dynamicconfig.GetBoolPropertyFnFilteredByDomain(false),
			readModeIsFromPinot: dynamicconfig.GetBoolPropertyFnFilteredByDomain(true),
			expectedError:       nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAccordance(test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityManager := NewPinotVisibilityTripleManager(test.mockDBVisibilityManager, test.mockPinotVisibilityManager,
				test.mockESVisibilityManager, test.readModeIsFromPinot, test.readModeIsFromES,
				nil, dynamicconfig.GetBoolPropertyFnFilteredByDomain(true), log.NewNoop())

			_, err := visibilityManager.ListWorkflowExecutions(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPinotTripleScanWorkflowExecutions(t *testing.T) {
	request := &ListWorkflowExecutionsByQueryRequest{
		Domain: "test-domain",
	}

	// put this outside because need to use it as an input of the table tests
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                              *ListWorkflowExecutionsByQueryRequest
		mockDBVisibilityManager              VisibilityManager
		mockESVisibilityManager              VisibilityManager
		mockPinotVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAccordance func(mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance    func(mockESVisibilityManager *MockVisibilityManager)
		readModeIsFromES                     dynamicconfig.BoolPropertyFnWithDomainFilter
		readModeIsFromPinot                  dynamicconfig.BoolPropertyFnWithDomainFilter
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ScanWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetBoolPropertyFnFilteredByDomain(true),
			expectedError:    nil,
		},
		"Case1-2: success case with Pinot visibility is not nil": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ScanWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES:    dynamicconfig.GetBoolPropertyFnFilteredByDomain(false),
			readModeIsFromPinot: dynamicconfig.GetBoolPropertyFnFilteredByDomain(true),
			expectedError:       nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAccordance(test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityManager := NewPinotVisibilityTripleManager(test.mockDBVisibilityManager, test.mockPinotVisibilityManager,
				test.mockESVisibilityManager, test.readModeIsFromPinot, test.readModeIsFromES,
				nil, dynamicconfig.GetBoolPropertyFnFilteredByDomain(true), log.NewNoop())

			_, err := visibilityManager.ScanWorkflowExecutions(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPinotTripleCountWorkflowExecutions(t *testing.T) {
	request := &CountWorkflowExecutionsRequest{
		Domain: "test-domain",
	}

	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                              *CountWorkflowExecutionsRequest
		mockDBVisibilityManager              VisibilityManager
		mockESVisibilityManager              VisibilityManager
		mockPinotVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAccordance func(mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance    func(mockESVisibilityManager *MockVisibilityManager)
		readModeIsFromES                     dynamicconfig.BoolPropertyFnWithDomainFilter
		readModeIsFromPinot                  dynamicconfig.BoolPropertyFnWithDomainFilter
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetBoolPropertyFnFilteredByDomain(true),
			expectedError:    nil,
		},
		"Case1-2: success case with Pinot visibility is not nil": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAccordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES:    dynamicconfig.GetBoolPropertyFnFilteredByDomain(false),
			readModeIsFromPinot: dynamicconfig.GetBoolPropertyFnFilteredByDomain(true),
			expectedError:       nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAccordance(test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityManager := NewPinotVisibilityTripleManager(test.mockDBVisibilityManager, test.mockPinotVisibilityManager,
				test.mockESVisibilityManager, test.readModeIsFromPinot, test.readModeIsFromES,
				nil, dynamicconfig.GetBoolPropertyFnFilteredByDomain(true), log.NewNoop())

			_, err := visibilityManager.CountWorkflowExecutions(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
