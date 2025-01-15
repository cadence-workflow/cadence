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

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
)

func TestNewVisibilityDualManager(t *testing.T) {
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		mockDBVisibilityManager VisibilityManager
		mockESVisibilityManager VisibilityManager
	}{
		"Case1: nil case": {
			mockDBVisibilityManager: nil,
			mockESVisibilityManager: nil,
		},
		"Case2: success case": {
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				NewVisibilityDualManager(test.mockDBVisibilityManager, test.mockESVisibilityManager, nil, nil, log.NewNoop())
			})
		})
	}
}

func TestDualManagerClose(t *testing.T) {
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		mockDBVisibilityManager           VisibilityManager
		mockESVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance func(mockDBVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance func(mockESVisibilityManager *MockVisibilityManager)
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
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}
			visibilityManager := NewVisibilityDualManager(test.mockDBVisibilityManager, test.mockESVisibilityManager, nil, nil, log.NewNoop())
			assert.NotPanics(t, func() {
				visibilityManager.Close()
			})
		})
	}
}

func TestDualManagerGetName(t *testing.T) {
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		mockDBVisibilityManager           VisibilityManager
		mockESVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance func(mockDBVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance func(mockESVisibilityManager *MockVisibilityManager)
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
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}
			visibilityManager := NewVisibilityDualManager(test.mockDBVisibilityManager, test.mockESVisibilityManager, nil, nil, log.NewNoop())

			assert.NotPanics(t, func() {
				visibilityManager.GetName()
			})
		})
	}
}

func TestDualRecordWorkflowExecutionStarted(t *testing.T) {
	request := &RecordWorkflowExecutionStartedRequest{}

	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                           *RecordWorkflowExecutionStartedRequest
		mockDBVisibilityManager           VisibilityManager
		mockESVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance func(mockDBVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance func(mockESVisibilityManager *MockVisibilityManager)
		advancedVisibilityWritingMode     dynamicconfig.StringPropertyFn
		expectedError                     error
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
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}
			visibilityManager := NewVisibilityDualManager(test.mockDBVisibilityManager,
				test.mockESVisibilityManager, nil, test.advancedVisibilityWritingMode, log.NewNoop())

			err := visibilityManager.RecordWorkflowExecutionStarted(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDualRecordWorkflowExecutionClosed(t *testing.T) {
	request := &RecordWorkflowExecutionClosedRequest{}

	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		context                           context.Context
		request                           *RecordWorkflowExecutionClosedRequest
		mockDBVisibilityManager           VisibilityManager
		mockESVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance func(mockDBVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance func(mockESVisibilityManager *MockVisibilityManager)
		advancedVisibilityWritingMode     dynamicconfig.StringPropertyFn
		expectedError                     error
	}{
		"Case0-1: error case with advancedVisibilityWritingMode is nil": {
			context:                 context.Background(),
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
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
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeDual),
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
		"Case1-3: success case with dual manager": {
			context:                 context.Background(),
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			advancedVisibilityWritingMode: dynamicconfig.GetStringPropertyFn(common.AdvancedVisibilityWritingModeDual),
			expectedError:                 nil,
		},
		"Case2-1: choose DB visibility manager when it is nil": {
			context:                 context.Background(),
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
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
			context:                 context.Background(),
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
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
			context:                 context.WithValue(context.Background(), VisibilityAdminDeletionKey("visibilityAdminDelete"), true),
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
		},
		"Case3-3: chooseVisibilityModeForAdmin when both are not nil": {
			context:                 context.WithValue(context.Background(), VisibilityAdminDeletionKey("visibilityAdminDelete"), true),
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}
			visibilityManager := NewVisibilityDualManager(test.mockDBVisibilityManager,
				test.mockESVisibilityManager, nil, test.advancedVisibilityWritingMode, log.NewNoop())

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
func TestChooseVisibilityModeForAdmin(t *testing.T) {
	ctrl := gomock.NewController(t)
	dbManager := NewMockVisibilityManager(ctrl)
	esManager := NewMockVisibilityManager(ctrl)
	mgr := NewVisibilityDualManager(dbManager, esManager, nil, nil, log.NewNoop())
	dualManager := mgr.(*visibilityDualManager)
	dualManager.dbVisibilityManager = nil
	dualManager.advancedVisibilityManager = nil
	assert.Equal(t, "INVALID_ADMIN_MODE", dualManager.chooseVisibilityModeForAdmin())
}

func TestDualRecordWorkflowExecutionUninitialized(t *testing.T) {
	request := &RecordWorkflowExecutionUninitializedRequest{}

	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                           *RecordWorkflowExecutionUninitializedRequest
		mockDBVisibilityManager           VisibilityManager
		mockESVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance func(mockDBVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance func(mockESVisibilityManager *MockVisibilityManager)
		advancedVisibilityWritingMode     dynamicconfig.StringPropertyFn
		expectedError                     error
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
		"Case1-2: success case with ES visibility is not nil": {
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
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}
			visibilityManager := NewVisibilityDualManager(test.mockDBVisibilityManager,
				test.mockESVisibilityManager, nil, test.advancedVisibilityWritingMode, log.NewNoop())

			err := visibilityManager.RecordWorkflowExecutionUninitialized(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDualUpsertWorkflowExecution(t *testing.T) {
	request := &UpsertWorkflowExecutionRequest{}

	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                           *UpsertWorkflowExecutionRequest
		mockDBVisibilityManager           VisibilityManager
		mockESVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance func(mockDBVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance func(mockESVisibilityManager *MockVisibilityManager)
		advancedVisibilityWritingMode     dynamicconfig.StringPropertyFn
		expectedError                     error
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
		"Case1-2: success case with ES visibility is not nil": {
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
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}
			visibilityManager := NewVisibilityDualManager(test.mockDBVisibilityManager,
				test.mockESVisibilityManager, nil, test.advancedVisibilityWritingMode, log.NewNoop())

			err := visibilityManager.UpsertWorkflowExecution(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDualDeleteWorkflowExecution(t *testing.T) {
	request := &VisibilityDeleteWorkflowExecutionRequest{}

	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                           *VisibilityDeleteWorkflowExecutionRequest
		mockDBVisibilityManager           VisibilityManager
		mockESVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance func(mockDBVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance func(mockESVisibilityManager *MockVisibilityManager)
		advancedVisibilityWritingMode     dynamicconfig.StringPropertyFn
		expectedError                     error
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
		"Case1-2: success case with ES visibility is not nil": {
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
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}
			visibilityManager := NewVisibilityDualManager(test.mockDBVisibilityManager,
				test.mockESVisibilityManager, nil, test.advancedVisibilityWritingMode, log.NewNoop())

			err := visibilityManager.DeleteWorkflowExecution(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDualDeleteUninitializedWorkflowExecution(t *testing.T) {
	request := &VisibilityDeleteWorkflowExecutionRequest{}

	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                           *VisibilityDeleteWorkflowExecutionRequest
		mockDBVisibilityManager           VisibilityManager
		mockESVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance func(mockDBVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance func(mockESVisibilityManager *MockVisibilityManager)
		advancedVisibilityWritingMode     dynamicconfig.StringPropertyFn
		expectedError                     error
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
		"Case1-2: success case with ES visibility is not nil": {
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
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}
			visibilityManager := NewVisibilityDualManager(test.mockDBVisibilityManager,
				test.mockESVisibilityManager, nil, test.advancedVisibilityWritingMode, log.NewNoop())

			err := visibilityManager.DeleteUninitializedWorkflowExecution(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDualListOpenWorkflowExecutions(t *testing.T) {
	request := &ListWorkflowExecutionsRequest{
		Domain: "test-domain",
	}

	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                           *ListWorkflowExecutionsRequest
		mockDBVisibilityManager           VisibilityManager
		mockESVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance func(mockDBVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance func(mockESVisibilityManager *MockVisibilityManager)
		readModeIsFromES                  dynamicconfig.StringPropertyFnWithDomainFilter
		expectedError                     error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ListOpenWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetStringPropertyFnFilteredByDomain("es"),
			expectedError:    nil,
		},
		"Case1-2: success case with ES visibility is not nil": {
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListOpenWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetStringPropertyFnFilteredByDomain("es"),
			expectedError:    nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}
			visibilityManager := NewVisibilityDualManager(test.mockDBVisibilityManager,
				test.mockESVisibilityManager, test.readModeIsFromES, nil, log.NewNoop())

			_, err := visibilityManager.ListOpenWorkflowExecutions(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDualListClosedWorkflowExecutions(t *testing.T) {
	request := &ListWorkflowExecutionsRequest{
		Domain: "test-domain",
	}

	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                           *ListWorkflowExecutionsRequest
		mockDBVisibilityManager           VisibilityManager
		mockESVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance func(mockDBVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance func(mockESVisibilityManager *MockVisibilityManager)
		readModeIsFromES                  dynamicconfig.StringPropertyFnWithDomainFilter
		expectedError                     error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetStringPropertyFnFilteredByDomain("es"),
			expectedError:    nil,
		},
		"Case1-2: success case with ES visibility is not nil": {
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetStringPropertyFnFilteredByDomain("es"),
			expectedError:    nil,
		},
		"Case2-1: success case with DB visibility is not nil and read mod is false": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetStringPropertyFnFilteredByDomain("db"),
			expectedError:    nil,
		},
		"Case2-2: success case with ES visibility is not nil and read mod is false": {
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetStringPropertyFnFilteredByDomain("db"),
			expectedError:    nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}
			visibilityManager := NewVisibilityDualManager(test.mockDBVisibilityManager,
				test.mockESVisibilityManager, test.readModeIsFromES, nil, log.NewNoop())

			_, err := visibilityManager.ListClosedWorkflowExecutions(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDualListOpenWorkflowExecutionsByType(t *testing.T) {
	request := &ListWorkflowExecutionsByTypeRequest{
		ListWorkflowExecutionsRequest: ListWorkflowExecutionsRequest{
			Domain: "test-domain",
		},
	}

	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                           *ListWorkflowExecutionsByTypeRequest
		mockDBVisibilityManager           VisibilityManager
		mockESVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance func(mockDBVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance func(mockESVisibilityManager *MockVisibilityManager)
		readModeIsFromES                  dynamicconfig.StringPropertyFnWithDomainFilter
		expectedError                     error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ListOpenWorkflowExecutionsByType(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetStringPropertyFnFilteredByDomain("es"),
			expectedError:    nil,
		},
		"Case1-2: success case with ES visibility is not nil": {
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListOpenWorkflowExecutionsByType(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetStringPropertyFnFilteredByDomain("es"),
			expectedError:    nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}
			visibilityManager := NewVisibilityDualManager(test.mockDBVisibilityManager,
				test.mockESVisibilityManager, test.readModeIsFromES, nil, log.NewNoop())

			_, err := visibilityManager.ListOpenWorkflowExecutionsByType(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDualListClosedWorkflowExecutionsByType(t *testing.T) {
	request := &ListWorkflowExecutionsByTypeRequest{
		ListWorkflowExecutionsRequest: ListWorkflowExecutionsRequest{
			Domain: "test-domain",
		},
	}

	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                           *ListWorkflowExecutionsByTypeRequest
		mockDBVisibilityManager           VisibilityManager
		mockESVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance func(mockDBVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance func(mockESVisibilityManager *MockVisibilityManager)
		readModeIsFromES                  dynamicconfig.StringPropertyFnWithDomainFilter
		expectedError                     error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ListClosedWorkflowExecutionsByType(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetStringPropertyFnFilteredByDomain("es"),
			expectedError:    nil,
		},
		"Case1-2: success case with ES visibility is not nil": {
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListClosedWorkflowExecutionsByType(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetStringPropertyFnFilteredByDomain("es"),
			expectedError:    nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}
			visibilityManager := NewVisibilityDualManager(test.mockDBVisibilityManager,
				test.mockESVisibilityManager, test.readModeIsFromES, nil, log.NewNoop())

			_, err := visibilityManager.ListClosedWorkflowExecutionsByType(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDualListOpenWorkflowExecutionsByWorkflowID(t *testing.T) {
	request := &ListWorkflowExecutionsByWorkflowIDRequest{
		ListWorkflowExecutionsRequest: ListWorkflowExecutionsRequest{
			Domain: "test-domain",
		},
	}

	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                           *ListWorkflowExecutionsByWorkflowIDRequest
		mockDBVisibilityManager           VisibilityManager
		mockESVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance func(mockDBVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance func(mockESVisibilityManager *MockVisibilityManager)
		readModeIsFromES                  dynamicconfig.StringPropertyFnWithDomainFilter
		expectedError                     error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ListOpenWorkflowExecutionsByWorkflowID(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetStringPropertyFnFilteredByDomain("es"),
			expectedError:    nil,
		},
		"Case1-2: success case with ES visibility is not nil": {
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListOpenWorkflowExecutionsByWorkflowID(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetStringPropertyFnFilteredByDomain("es"),
			expectedError:    nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}
			visibilityManager := NewVisibilityDualManager(test.mockDBVisibilityManager,
				test.mockESVisibilityManager, test.readModeIsFromES, nil, log.NewNoop())

			_, err := visibilityManager.ListOpenWorkflowExecutionsByWorkflowID(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDualListClosedWorkflowExecutionsByWorkflowID(t *testing.T) {
	request := &ListWorkflowExecutionsByWorkflowIDRequest{
		ListWorkflowExecutionsRequest: ListWorkflowExecutionsRequest{
			Domain: "test-domain",
		},
	}

	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                           *ListWorkflowExecutionsByWorkflowIDRequest
		mockDBVisibilityManager           VisibilityManager
		mockESVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance func(mockDBVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance func(mockESVisibilityManager *MockVisibilityManager)
		readModeIsFromES                  dynamicconfig.StringPropertyFnWithDomainFilter
		expectedError                     error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ListClosedWorkflowExecutionsByWorkflowID(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetStringPropertyFnFilteredByDomain("es"),
			expectedError:    nil,
		},
		"Case1-2: success case with ES visibility is not nil": {
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListClosedWorkflowExecutionsByWorkflowID(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetStringPropertyFnFilteredByDomain("es"),
			expectedError:    nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}
			visibilityManager := NewVisibilityDualManager(test.mockDBVisibilityManager,
				test.mockESVisibilityManager, test.readModeIsFromES, nil, log.NewNoop())

			_, err := visibilityManager.ListClosedWorkflowExecutionsByWorkflowID(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDualListClosedWorkflowExecutionsByStatus(t *testing.T) {
	request := &ListClosedWorkflowExecutionsByStatusRequest{
		ListWorkflowExecutionsRequest: ListWorkflowExecutionsRequest{
			Domain: "test-domain",
		},
	}

	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                           *ListClosedWorkflowExecutionsByStatusRequest
		mockDBVisibilityManager           VisibilityManager
		mockESVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance func(mockDBVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance func(mockESVisibilityManager *MockVisibilityManager)
		readModeIsFromES                  dynamicconfig.StringPropertyFnWithDomainFilter
		expectedError                     error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ListClosedWorkflowExecutionsByStatus(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetStringPropertyFnFilteredByDomain("es"),
			expectedError:    nil,
		},
		"Case1-2: success case with ES visibility is not nil": {
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListClosedWorkflowExecutionsByStatus(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetStringPropertyFnFilteredByDomain("es"),
			expectedError:    nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}
			visibilityManager := NewVisibilityDualManager(test.mockDBVisibilityManager,
				test.mockESVisibilityManager, test.readModeIsFromES, nil, log.NewNoop())

			_, err := visibilityManager.ListClosedWorkflowExecutionsByStatus(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDualGetClosedWorkflowExecution(t *testing.T) {
	request := &GetClosedWorkflowExecutionRequest{
		Domain: "test-domain",
	}

	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                           *GetClosedWorkflowExecutionRequest
		mockDBVisibilityManager           VisibilityManager
		mockESVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance func(mockDBVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance func(mockESVisibilityManager *MockVisibilityManager)
		readModeIsFromES                  dynamicconfig.StringPropertyFnWithDomainFilter
		expectedError                     error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().GetClosedWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetStringPropertyFnFilteredByDomain("es"),
			expectedError:    nil,
		},
		"Case1-2: success case with ES visibility is not nil": {
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().GetClosedWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetStringPropertyFnFilteredByDomain("es"),
			expectedError:    nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}
			visibilityManager := NewVisibilityDualManager(test.mockDBVisibilityManager,
				test.mockESVisibilityManager, test.readModeIsFromES, nil, log.NewNoop())

			_, err := visibilityManager.GetClosedWorkflowExecution(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDualListWorkflowExecutions(t *testing.T) {
	request := &ListWorkflowExecutionsByQueryRequest{
		Domain: "test-domain",
	}

	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                           *ListWorkflowExecutionsByQueryRequest
		mockDBVisibilityManager           VisibilityManager
		mockESVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance func(mockDBVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance func(mockESVisibilityManager *MockVisibilityManager)
		readModeIsFromES                  dynamicconfig.StringPropertyFnWithDomainFilter
		expectedError                     error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ListWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetStringPropertyFnFilteredByDomain("es"),
			expectedError:    nil,
		},
		"Case1-2: success case with ES visibility is not nil": {
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetStringPropertyFnFilteredByDomain("es"),
			expectedError:    nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}
			visibilityManager := NewVisibilityDualManager(test.mockDBVisibilityManager,
				test.mockESVisibilityManager, test.readModeIsFromES, nil, log.NewNoop())

			_, err := visibilityManager.ListWorkflowExecutions(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDualScanWorkflowExecutions(t *testing.T) {
	request := &ListWorkflowExecutionsByQueryRequest{
		Domain: "test-domain",
	}

	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                           *ListWorkflowExecutionsByQueryRequest
		mockDBVisibilityManager           VisibilityManager
		mockESVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance func(mockDBVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance func(mockESVisibilityManager *MockVisibilityManager)
		readModeIsFromES                  dynamicconfig.StringPropertyFnWithDomainFilter
		expectedError                     error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ScanWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetStringPropertyFnFilteredByDomain("es"),
			expectedError:    nil,
		},
		"Case1-2: success case with ES visibility is not nil": {
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ScanWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetStringPropertyFnFilteredByDomain("es"),
			expectedError:    nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}
			visibilityManager := NewVisibilityDualManager(test.mockDBVisibilityManager,
				test.mockESVisibilityManager, test.readModeIsFromES, nil, log.NewNoop())

			_, err := visibilityManager.ScanWorkflowExecutions(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDualCountWorkflowExecutions(t *testing.T) {
	request := &CountWorkflowExecutionsRequest{
		Domain: "test-domain",
	}

	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                           *CountWorkflowExecutionsRequest
		mockDBVisibilityManager           VisibilityManager
		mockESVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAccordance func(mockDBVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAccordance func(mockESVisibilityManager *MockVisibilityManager)
		readModeIsFromES                  dynamicconfig.StringPropertyFnWithDomainFilter
		expectedError                     error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAccordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetStringPropertyFnFilteredByDomain("es"),
			expectedError:    nil,
		},
		"Case1-2: success case with ES visibility is not nil": {
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAccordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readModeIsFromES: dynamicconfig.GetStringPropertyFnFilteredByDomain("es"),
			expectedError:    nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAccordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAccordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}
			visibilityManager := NewVisibilityDualManager(test.mockDBVisibilityManager,
				test.mockESVisibilityManager, test.readModeIsFromES, nil, log.NewNoop())

			_, err := visibilityManager.CountWorkflowExecutions(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
