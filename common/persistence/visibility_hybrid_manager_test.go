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
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
)

const (
	dualStoreName          = "es,pinot"    // test dual read for es and pinot, with es as primary
	dualStorePinotPrimary  = "pinot,es"    // test dual read for es and pinot, with pinot as primary
	tripleStoreName        = "es,pinot,db" // test triple read for es, pinot and db
	dualReadStoreName      = "es,db"       // test dual read for es and db
	dualReadStoreDBPrimary = "db,es"       // test dual read for es and db with db as primary
	esStoreName            = "es"
	pinotStoreName         = "pinot"
	testStoreName          = "test"
)

func TestNewVisibilityHybridManager(t *testing.T) {
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
				visibilityMgrs := map[string]VisibilityManager{
					dbVisStoreName: test.mockDBVisibilityManager,
					esStoreName:    test.mockESVisibilityManager,
					pinotStoreName: test.mockPinotVisibilityManager,
				}
				NewVisibilityHybridManager(visibilityMgrs, nil, dynamicproperties.GetStringPropertyFn(esStoreName), nil, testStoreName, log.NewNoop())
			})
		})
	}
}

func TestNewVisibilityHybridManager_EmptyVisibilityMgr(t *testing.T) {
	// put this outside because need to use it as an input of the table tests
	assert.NotPanics(t, func() {
		visibilityMgrs := map[string]VisibilityManager{}
		NewVisibilityHybridManager(visibilityMgrs, nil, nil, nil, testStoreName, log.NewNoop())
	})
}

func TestVisibilityHybridManagerClose(t *testing.T) {
	// put this outside because need to use it as an input of the table tests
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		mockDBVisibilityManager              VisibilityManager
		mockPinotVisibilityManager           VisibilityManager
		mockESVisibilityManager              VisibilityManager
		mockDBVisibilityManagerAffordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAffordance func(mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAffordance    func(mockESVisibilityManager *MockVisibilityManager)
	}{
		"Case1-1: success case with DB visibility is not nil": {
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAffordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().Close().Return().Times(1)
			},
		},
		"Case1-2: success case with ES visibility is not nil": {
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().Close().Return().Times(1)
			},
		},
		"Case1-3: success case with pinot visibility is not nil": {
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().Close().Return().Times(1)
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAffordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAffordance(test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAffordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityMgrs := map[string]VisibilityManager{
				dbVisStoreName: test.mockDBVisibilityManager,
				esStoreName:    test.mockESVisibilityManager,
				pinotStoreName: test.mockPinotVisibilityManager,
			}
			visibilityManager := NewVisibilityHybridManager(visibilityMgrs, nil, dynamicproperties.GetStringPropertyFn(esStoreName), nil, testStoreName, log.NewNoop())
			assert.NotPanics(t, func() {
				visibilityManager.Close()
			})
		})
	}
}

func TestVisibilityHybridManagerGetName(t *testing.T) {
	visibilityMgrs := map[string]VisibilityManager{
		dbVisStoreName: NewMockVisibilityManager(gomock.NewController(t)),
	}
	visibilityManager := NewVisibilityHybridManager(visibilityMgrs, nil, dynamicproperties.GetStringPropertyFn(dbVisStoreName), nil, testStoreName, log.NewNoop())
	assert.Equal(t, testStoreName, visibilityManager.GetName())
}

func TestVisibilityHybridRecordWorkflowExecutionStarted(t *testing.T) {
	request := &RecordWorkflowExecutionStartedRequest{}

	// put this outside because need to use it as an input of the table tests
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                              *RecordWorkflowExecutionStartedRequest
		mockDBVisibilityManager              VisibilityManager
		mockESVisibilityManager              VisibilityManager
		mockPinotVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAffordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAffordance func(mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAffordance    func(mockESVisibilityManager *MockVisibilityManager)
		writeVisibilityStoreName             dynamicproperties.StringPropertyFn
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAffordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().RecordWorkflowExecutionStarted(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			writeVisibilityStoreName: dynamicproperties.GetStringPropertyFn(advancedWriteModeOff),
			expectedError:            nil,
		},
		"Case1-2: success case with ES visibility is not nil": {
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionStarted(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			writeVisibilityStoreName: dynamicproperties.GetStringPropertyFn(esStoreName),
			expectedError:            nil,
		},
		"Case1-3: success case with pinot visibility is not nil": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionStarted(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			writeVisibilityStoreName: dynamicproperties.GetStringPropertyFn(pinotStoreName),
		},
		"Case1-4: success case with ES visibility is nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAffordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().RecordWorkflowExecutionStarted(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error")).Times(1)
			},
			writeVisibilityStoreName: dynamicproperties.GetStringPropertyFn(esStoreName),
			expectedError:            fmt.Errorf("error"),
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAffordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAffordance(test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAffordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}
			visibilityMgrs := map[string]VisibilityManager{
				dbVisStoreName: test.mockDBVisibilityManager,
				esStoreName:    test.mockESVisibilityManager,
				pinotStoreName: test.mockPinotVisibilityManager,
			}
			visibilityManager := NewVisibilityHybridManager(visibilityMgrs, nil, test.writeVisibilityStoreName, nil, testStoreName, log.NewNoop())

			err := visibilityManager.RecordWorkflowExecutionStarted(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestVisibilityHybridRecordWorkflowExecutionClosed(t *testing.T) {
	request := &RecordWorkflowExecutionClosedRequest{}

	// put this outside because need to use it as an input of the table tests
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		context                              context.Context
		request                              *RecordWorkflowExecutionClosedRequest
		mockDBVisibilityManager              VisibilityManager
		mockESVisibilityManager              VisibilityManager
		mockPinotVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAffordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAffordance func(mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAffordance    func(mockESVisibilityManager *MockVisibilityManager)
		writeVisibilityStoreName             dynamicproperties.StringPropertyFn
		expectedError                        error
	}{
		"Case0-1: success case with writeVisibilityStoreName is nil - should fall back to db": {
			context:                 context.Background(),
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAffordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			expectedError: nil, // Should succeed by falling back to db
		},
		"Case0-2: error case with ES has errors in dual mode": {
			context:                 context.Background(),
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAffordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error")).AnyTimes()
			},
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error")).AnyTimes()
			},
			writeVisibilityStoreName: dynamicproperties.GetStringPropertyFn(tripleStoreName),
			expectedError:            fmt.Errorf("error"),
		},
		"Case0-3: error case with ES has errors in On mode with Pinot is not nil": {
			context:                 context.Background(),
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error")).AnyTimes()
			},
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error")).AnyTimes()
			},
			writeVisibilityStoreName: dynamicproperties.GetStringPropertyFn(dualStoreName),
			expectedError:            fmt.Errorf("error"),
		},
		"Case0-4: error case with Pinot has errors in On mode": {
			context:                 context.Background(),
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error")).AnyTimes()
			},
			writeVisibilityStoreName: dynamicproperties.GetStringPropertyFn(dualStoreName),
			expectedError:            fmt.Errorf("error"),
		},
		"Case0-5: error case with Pinot has errors in Dual mode": {
			context:                 context.Background(),
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error")).AnyTimes()
			},
			writeVisibilityStoreName: dynamicproperties.GetStringPropertyFn(dualStoreName),
			expectedError:            fmt.Errorf("error"),
		},
		"Case0-6: error case with ES has errors in Dual mode": {
			context:                 context.Background(),
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error")).AnyTimes()
			},
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			writeVisibilityStoreName: dynamicproperties.GetStringPropertyFn(dualStoreName),
			expectedError:            fmt.Errorf("error"),
		},
		"Case1-1: success case with DB visibility is not nil": {
			context:                 context.Background(),
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAffordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			writeVisibilityStoreName: dynamicproperties.GetStringPropertyFn(advancedWriteModeOff),
			expectedError:            nil,
		},
		"Case1-2: success case with ES visibility is not nil": {
			context:                 context.Background(),
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			writeVisibilityStoreName: dynamicproperties.GetStringPropertyFn(esStoreName),
		},
		"Case1-3: success case with pinot visibility is not nil": {
			context:                    context.Background(),
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			writeVisibilityStoreName: dynamicproperties.GetStringPropertyFn(pinotStoreName),
		},
		"Case1-4: success case with dual manager": {
			context:                 context.Background(),
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			writeVisibilityStoreName: dynamicproperties.GetStringPropertyFn(dualStoreName),
		},
		"Case1-5: success case with triple manager when ES and Pinot are not nil": {
			context:                 context.Background(),
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAffordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			writeVisibilityStoreName: dynamicproperties.GetStringPropertyFn(tripleStoreName),
		},
		"Case2-1: choose both when ES is nil, fall back to db": {
			context:                 context.Background(),
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAffordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			writeVisibilityStoreName: dynamicproperties.GetStringPropertyFn(dualStoreName),
			expectedError:            nil,
		},
		"Case2-2: choose both when Pinot is nil": {
			context:                 context.Background(),
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAffordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			writeVisibilityStoreName: dynamicproperties.GetStringPropertyFn(dualStoreName),
			expectedError:            nil,
		},
		"Case3-1: chooseVisibilityModeForAdmin when ES is nil": {
			context:                 context.WithValue(context.Background(), VisibilityAdminDeletionKey("visibilityAdminDelete"), true),
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAffordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
		},
		"Case3-2: chooseVisibilityModeForAdmin when DB is nil": {
			context:                    context.WithValue(context.Background(), VisibilityAdminDeletionKey("visibilityAdminDelete"), true),
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
		},
		"Case3-3: chooseVisibilityModeForAdmin when both are not nil": {
			context:                 context.WithValue(context.Background(), VisibilityAdminDeletionKey("visibilityAdminDelete"), true),
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
		},
		"Case3-3: chooseVisibilityModeForAdmin when triple are not nil": {
			context:                 context.WithValue(context.Background(), VisibilityAdminDeletionKey("visibilityAdminDelete"), true),
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAffordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionClosed(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAffordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAffordance(test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAffordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityMgrs := map[string]VisibilityManager{
				dbVisStoreName: test.mockDBVisibilityManager,
				esStoreName:    test.mockESVisibilityManager,
				pinotStoreName: test.mockPinotVisibilityManager,
			}
			visibilityManager := NewVisibilityHybridManager(visibilityMgrs, nil, test.writeVisibilityStoreName, nil, testStoreName, log.NewNoop())

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
func TestVisibilityHybridChooseVisibilityModeForAdmin(t *testing.T) {
	ctrl := gomock.NewController(t)
	dbManager := NewMockVisibilityManager(ctrl)
	esManager := NewMockVisibilityManager(ctrl)
	pntManager := NewMockVisibilityManager(ctrl)
	visibilityMgrs := map[string]VisibilityManager{
		dbVisStoreName: dbManager,
		esStoreName:    esManager,
		pinotStoreName: pntManager,
	}
	mgr := NewVisibilityHybridManager(visibilityMgrs, nil, nil, nil, testStoreName, log.NewNoop())
	tripleManager := mgr.(*visibilityHybridManager)
	tripleManager.visibilityMgrs[dbVisStoreName] = nil
	tripleManager.visibilityMgrs[esStoreName] = nil
	tripleManager.visibilityMgrs[pinotStoreName] = nil
	assert.Equal(t, "INVALID_ADMIN_MODE", tripleManager.chooseVisibilityModeForAdmin())
}

func TestVisibilityHybridRecordWorkflowExecutionUninitialized(t *testing.T) {
	request := &RecordWorkflowExecutionUninitializedRequest{}

	// put this outside because need to use it as an input of the table tests
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                              *RecordWorkflowExecutionUninitializedRequest
		mockDBVisibilityManager              VisibilityManager
		mockESVisibilityManager              VisibilityManager
		mockPinotVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAffordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAffordance func(mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAffordance    func(mockESVisibilityManager *MockVisibilityManager)
		writeVisibilityStoreName             dynamicproperties.StringPropertyFn
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAffordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().RecordWorkflowExecutionUninitialized(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			writeVisibilityStoreName: dynamicproperties.GetStringPropertyFn(advancedWriteModeOff),
			expectedError:            nil,
		},
		"Case1-2: success case with Pinot visibility is not nil": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionUninitialized(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionUninitialized(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			writeVisibilityStoreName: dynamicproperties.GetStringPropertyFn(dualStoreName),
			expectedError:            nil,
		},
		"Case1-3: success case with ES visibility is not nil": {
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().RecordWorkflowExecutionUninitialized(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().RecordWorkflowExecutionUninitialized(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			writeVisibilityStoreName: dynamicproperties.GetStringPropertyFn(dualStoreName),
			expectedError:            nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAffordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAffordance(test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAffordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityMgrs := map[string]VisibilityManager{
				dbVisStoreName: test.mockDBVisibilityManager,
				esStoreName:    test.mockESVisibilityManager,
				pinotStoreName: test.mockPinotVisibilityManager,
			}
			visibilityManager := NewVisibilityHybridManager(visibilityMgrs, nil, test.writeVisibilityStoreName, nil, testStoreName, log.NewNoop())

			err := visibilityManager.RecordWorkflowExecutionUninitialized(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestVisibilityHybridUpsertWorkflowExecution(t *testing.T) {
	request := &UpsertWorkflowExecutionRequest{}

	// put this outside because need to use it as an input of the table tests
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                              *UpsertWorkflowExecutionRequest
		mockDBVisibilityManager              VisibilityManager
		mockESVisibilityManager              VisibilityManager
		mockPinotVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAffordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAffordance func(mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAffordance    func(mockESVisibilityManager *MockVisibilityManager)
		writeVisibilityStoreName             dynamicproperties.StringPropertyFn
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAffordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().UpsertWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			writeVisibilityStoreName: dynamicproperties.GetStringPropertyFn(advancedWriteModeOff),
			expectedError:            nil,
		},
		"Case1-2: success case with Pinot visibility is not nil": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().UpsertWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			writeVisibilityStoreName: dynamicproperties.GetStringPropertyFn(pinotStoreName),
			expectedError:            nil,
		},
		"Case1-3: success case with ES visibility is not nil": {
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().UpsertWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			writeVisibilityStoreName: dynamicproperties.GetStringPropertyFn(esStoreName),
			expectedError:            nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAffordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAffordance(test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAffordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityMgrs := map[string]VisibilityManager{
				dbVisStoreName: test.mockDBVisibilityManager,
				esStoreName:    test.mockESVisibilityManager,
				pinotStoreName: test.mockPinotVisibilityManager,
			}
			visibilityManager := NewVisibilityHybridManager(visibilityMgrs, nil, test.writeVisibilityStoreName, nil, testStoreName, log.NewNoop())

			err := visibilityManager.UpsertWorkflowExecution(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestVisibilityHybridDeleteWorkflowExecution(t *testing.T) {
	request := &VisibilityDeleteWorkflowExecutionRequest{}

	// put this outside because need to use it as an input of the table tests
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                              *VisibilityDeleteWorkflowExecutionRequest
		mockDBVisibilityManager              VisibilityManager
		mockESVisibilityManager              VisibilityManager
		mockPinotVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAffordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAffordance func(mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAffordance    func(mockESVisibilityManager *MockVisibilityManager)
		writeVisibilityStoreName             dynamicproperties.StringPropertyFn
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAffordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().DeleteWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			writeVisibilityStoreName: dynamicproperties.GetStringPropertyFn(advancedWriteModeOff),
			expectedError:            nil,
		},
		"Case1-2: success case with Pinot visibility is not nil": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().DeleteWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			writeVisibilityStoreName: dynamicproperties.GetStringPropertyFn(pinotStoreName),
			expectedError:            nil,
		},
		"Case1-3: success case with ES visibility is not nil": {
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().DeleteWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			writeVisibilityStoreName: dynamicproperties.GetStringPropertyFn(esStoreName),
			expectedError:            nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAffordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAffordance(test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAffordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityMgrs := map[string]VisibilityManager{
				dbVisStoreName: test.mockDBVisibilityManager,
				esStoreName:    test.mockESVisibilityManager,
				pinotStoreName: test.mockPinotVisibilityManager,
			}
			visibilityManager := NewVisibilityHybridManager(visibilityMgrs, nil, test.writeVisibilityStoreName, nil, testStoreName, log.NewNoop())

			err := visibilityManager.DeleteWorkflowExecution(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestVisibilityHybridDeleteUninitializedWorkflowExecution(t *testing.T) {
	request := &VisibilityDeleteWorkflowExecutionRequest{}

	// put this outside because need to use it as an input of the table tests
	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                              *VisibilityDeleteWorkflowExecutionRequest
		mockDBVisibilityManager              VisibilityManager
		mockESVisibilityManager              VisibilityManager
		mockPinotVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAffordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAffordance func(mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAffordance    func(mockESVisibilityManager *MockVisibilityManager)
		writeVisibilityStoreName             dynamicproperties.StringPropertyFn
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAffordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().DeleteUninitializedWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			writeVisibilityStoreName: dynamicproperties.GetStringPropertyFn(advancedWriteModeOff),
			expectedError:            nil,
		},
		"Case1-2: success case with Pinot visibility is not nil": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().DeleteUninitializedWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			writeVisibilityStoreName: dynamicproperties.GetStringPropertyFn(pinotStoreName),
			expectedError:            nil,
		},
		"Case1-3: success case with ES visibility is not nil": {
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().DeleteUninitializedWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			writeVisibilityStoreName: dynamicproperties.GetStringPropertyFn(esStoreName),
			expectedError:            nil,
		},
		"Case1-4: success case with both are not nil": {
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().DeleteUninitializedWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().DeleteUninitializedWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			writeVisibilityStoreName: dynamicproperties.GetStringPropertyFn(dualStoreName),
			expectedError:            nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAffordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAffordance(test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAffordance(test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityMgrs := map[string]VisibilityManager{
				dbVisStoreName: test.mockDBVisibilityManager,
				esStoreName:    test.mockESVisibilityManager,
				pinotStoreName: test.mockPinotVisibilityManager,
			}
			visibilityManager := NewVisibilityHybridManager(visibilityMgrs, nil, test.writeVisibilityStoreName, nil, testStoreName, log.NewNoop())

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

func TestVisibilityHybridListOpenWorkflowExecutions(t *testing.T) {
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
		mockDBVisibilityManagerAffordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAffordance func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAffordance    func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager)
		readVisibilityStoreName              dynamicproperties.StringPropertyFnWithDomainFilter
		wgCount                              int
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAffordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ListOpenWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(esStoreName),
			wgCount:                 0,
			expectedError:           nil,
		},
		"Case1-2: success case with Pinot visibility is not nil": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListOpenWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(pinotStoreName),
			wgCount:                 0,
			expectedError:           nil,
		},
		"Case2-1: success case with double read": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListOpenWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListOpenWorkflowExecutions(gomock.Any(), gomock.Any()).DoAndReturn(func(
					ctx context.Context, request *ListWorkflowExecutionsRequest) (*ListWorkflowExecutionsResponse, error) {
					wg.Done()
					return nil, nil
				}).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dualStorePinotPrimary),
			wgCount:                 1,
			expectedError:           nil,
		},
		"Case2-2: Pinot nil case with double read": {
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListOpenWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dualStoreName),
			wgCount:                 0,
			expectedError:           nil,
		},
		"Case2-3: Read mode is from ES with double read": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListOpenWorkflowExecutions(gomock.Any(), gomock.Any()).DoAndReturn(func(
					ctx context.Context, request *ListWorkflowExecutionsRequest) (*ListWorkflowExecutionsResponse, error) {
					wg.Done()
					return nil, nil
				}).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListOpenWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dualStoreName),
			wgCount:                 1,
			expectedError:           nil,
		},
		"Case2-4: double read with an error": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListOpenWorkflowExecutions(gomock.Any(), gomock.Any()).DoAndReturn(func(
					ctx context.Context, request *ListWorkflowExecutionsRequest) (*ListWorkflowExecutionsResponse, error) {
					wg.Done()
					return nil, fmt.Errorf("test error")
				}).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListOpenWorkflowExecutions(gomock.Any(), gomock.Any()).
					Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dualStoreName),
			wgCount:                 1,
			expectedError:           nil,
		},
		"Case2-5: double read with panic": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListOpenWorkflowExecutions(gomock.Any(), gomock.Any()).DoAndReturn(func(
					ctx context.Context, request *ListWorkflowExecutionsRequest) (*ListWorkflowExecutionsResponse, error) {
					wg.Done()
					panic("test panic")
				}).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListOpenWorkflowExecutions(gomock.Any(), gomock.Any()).
					Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dualStoreName),
			wgCount:                 1,
			expectedError:           nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			wg := sync.WaitGroup{}
			wg.Add(test.wgCount)

			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAffordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAffordance(&wg, test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAffordance(&wg, test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityMgrs := map[string]VisibilityManager{
				dbVisStoreName: test.mockDBVisibilityManager,
				esStoreName:    test.mockESVisibilityManager,
				pinotStoreName: test.mockPinotVisibilityManager,
			}
			visibilityManager := NewVisibilityHybridManager(visibilityMgrs, test.readVisibilityStoreName, nil, dynamicproperties.GetBoolPropertyFnFilteredByDomain(true), testStoreName, log.NewNoop())

			_, err := visibilityManager.ListOpenWorkflowExecutions(context.Background(), test.request)

			wg.Wait()
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestVisibilityHybridListClosedWorkflowExecutions(t *testing.T) {
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
		mockDBVisibilityManagerAffordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAffordance func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAffordance    func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager)
		readVisibilityStoreName              dynamicproperties.StringPropertyFnWithDomainFilter
		wgCount                              int
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			context:                 context.Background(),
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAffordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dbVisStoreName),
			wgCount:                 0,
			expectedError:           nil,
		},
		"Case1-2: success case with Pinot visibility is not nil": {
			context:                    context.Background(),
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(pinotStoreName),
			wgCount:                 0,
			expectedError:           nil,
		},
		"Case1-3: success case with ES visibility is not nil": {
			context:                 context.Background(),
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(esStoreName),
			wgCount:                 0,
			expectedError:           nil,
		},
		"Case2-1: success case with DB visibility is not nil and read Pinot is true": {
			context:                 context.Background(),
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAffordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(pinotStoreName),
			wgCount:                 0,
			expectedError:           nil,
		},
		"Case2-2: success case with DB visibility is not nil and read modes are false": {
			context:                 context.Background(),
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAffordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain("db"),
			wgCount:                 0,
			expectedError:           nil,
		},
		"Case3-1: read from ES with context key": {
			context:                 context.Background(),
			request:                 request,
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(pinotStoreName),
			wgCount:                 0,
		},
		"Case3-2: read from Pinot with context key": {
			context:                    context.Background(),
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(esStoreName),
			wgCount:                 0,
		},
		"Case4-1: success case with double read": {
			context:                    context.Background(),
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).DoAndReturn(func(
					ctx context.Context, request *ListWorkflowExecutionsRequest) (*ListWorkflowExecutionsResponse, error) {
					wg.Done()
					return nil, nil
				}).Times(1).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dualStorePinotPrimary),
			wgCount:                 1,
			expectedError:           nil,
		},
		"Case4-2: double read with an error": {
			context:                    context.Background(),
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).
					DoAndReturn(func(
						ctx context.Context, request *ListWorkflowExecutionsRequest) (*ListWorkflowExecutionsResponse, error) {
						wg.Done()
						return nil, fmt.Errorf("test error")
					}).Times(1).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dualStoreName),
			wgCount:                 1,
			expectedError:           nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			wg := sync.WaitGroup{}
			wg.Add(test.wgCount)

			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAffordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAffordance(&wg, test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAffordance(&wg, test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityMgrs := map[string]VisibilityManager{
				dbVisStoreName: test.mockDBVisibilityManager,
				esStoreName:    test.mockESVisibilityManager,
				pinotStoreName: test.mockPinotVisibilityManager,
			}
			visibilityManager := NewVisibilityHybridManager(visibilityMgrs, test.readVisibilityStoreName, nil, dynamicproperties.GetBoolPropertyFnFilteredByDomain(true), testStoreName, log.NewNoop())

			_, err := visibilityManager.ListClosedWorkflowExecutions(test.context, test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			wg.Wait()
		})
	}
}

func TestVisibilityHybridListOpenWorkflowExecutionsByType(t *testing.T) {
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
		mockDBVisibilityManagerAffordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAffordance func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAffordance    func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager)
		readVisibilityStoreName              dynamicproperties.StringPropertyFnWithDomainFilter
		wgCount                              int
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAffordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ListOpenWorkflowExecutionsByType(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(esStoreName),
			expectedError:           nil,
			wgCount:                 0,
		},
		"Case1-2: success case with Pinot visibility is not nil": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListOpenWorkflowExecutionsByType(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(pinotStoreName),
			wgCount:                 0,
			expectedError:           nil,
		},
		"Case2-1: success case with double read": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListOpenWorkflowExecutionsByType(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListOpenWorkflowExecutionsByType(gomock.Any(), gomock.Any()).DoAndReturn(func(
					ctx context.Context, request *ListWorkflowExecutionsByTypeRequest) (*ListWorkflowExecutionsResponse, error) {
					wg.Done()
					return nil, nil
				}).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dualStorePinotPrimary),
			wgCount:                 1,
			expectedError:           nil,
		},
		"Case2-2: double read with an error": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListOpenWorkflowExecutionsByType(gomock.Any(), gomock.Any()).
					DoAndReturn(func(
						ctx context.Context, request *ListWorkflowExecutionsByTypeRequest) (*ListWorkflowExecutionsResponse, error) {
						wg.Done()
						return nil, fmt.Errorf("test error")
					}).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListOpenWorkflowExecutionsByType(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dualStoreName),
			wgCount:                 1,
			expectedError:           nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			wg := sync.WaitGroup{}
			wg.Add(test.wgCount)

			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAffordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAffordance(&wg, test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAffordance(&wg, test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityMgrs := map[string]VisibilityManager{
				dbVisStoreName: test.mockDBVisibilityManager,
				esStoreName:    test.mockESVisibilityManager,
				pinotStoreName: test.mockPinotVisibilityManager,
			}
			visibilityManager := NewVisibilityHybridManager(visibilityMgrs, test.readVisibilityStoreName, nil, dynamicproperties.GetBoolPropertyFnFilteredByDomain(true), testStoreName, log.NewNoop())

			_, err := visibilityManager.ListOpenWorkflowExecutionsByType(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			wg.Wait()
		})
	}
}

func TestVisibilityHybridListClosedWorkflowExecutionsByType(t *testing.T) {
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
		mockDBVisibilityManagerAffordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAffordance func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAffordance    func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager)
		readVisibilityStoreName              dynamicproperties.StringPropertyFnWithDomainFilter
		wgCount                              int
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAffordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ListClosedWorkflowExecutionsByType(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dbVisStoreName),
			wgCount:                 0,
			expectedError:           nil,
		},
		"Case1-2: success case with Pinot visibility is not nil": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListClosedWorkflowExecutionsByType(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(pinotStoreName),
			wgCount:                 0,
			expectedError:           nil,
		},
		"Case2-1: success case with double read": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListClosedWorkflowExecutionsByType(gomock.Any(), gomock.Any()).DoAndReturn(func(
					ctx context.Context, request *ListWorkflowExecutionsByTypeRequest) (*ListWorkflowExecutionsResponse, error) {
					wg.Done()
					return nil, nil
				}).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListClosedWorkflowExecutionsByType(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dualStoreName),
			wgCount:                 1,
			expectedError:           nil,
		},
		"Case2-2: double read with an error": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListClosedWorkflowExecutionsByType(gomock.Any(), gomock.Any()).
					Return(nil, nil).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListClosedWorkflowExecutionsByType(gomock.Any(), gomock.Any()).DoAndReturn(func(
					ctx context.Context, request *ListWorkflowExecutionsByTypeRequest) (*ListWorkflowExecutionsResponse, error) {
					wg.Done()
					return nil, nil
				}).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dualStorePinotPrimary),
			wgCount:                 1,
			expectedError:           nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			wg := sync.WaitGroup{}
			wg.Add(test.wgCount)

			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAffordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAffordance(&wg, test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAffordance(&wg, test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityMgrs := map[string]VisibilityManager{
				dbVisStoreName: test.mockDBVisibilityManager,
				esStoreName:    test.mockESVisibilityManager,
				pinotStoreName: test.mockPinotVisibilityManager,
			}
			visibilityManager := NewVisibilityHybridManager(visibilityMgrs, test.readVisibilityStoreName, nil, dynamicproperties.GetBoolPropertyFnFilteredByDomain(true), testStoreName, log.NewNoop())

			_, err := visibilityManager.ListClosedWorkflowExecutionsByType(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			wg.Wait()
		})
	}
}

func TestVisibilityHybridListOpenWorkflowExecutionsByWorkflowID(t *testing.T) {
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
		mockDBVisibilityManagerAffordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAffordance func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAffordance    func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager)
		readVisibilityStoreName              dynamicproperties.StringPropertyFnWithDomainFilter
		wgCount                              int
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAffordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ListOpenWorkflowExecutionsByWorkflowID(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dbVisStoreName),
			wgCount:                 0,
			expectedError:           nil,
		},
		"Case1-2: success case with Pinot visibility is not nil": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListOpenWorkflowExecutionsByWorkflowID(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(pinotStoreName),
			wgCount:                 0,
			expectedError:           nil,
		},
		"Case2-1: success case with double read": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListOpenWorkflowExecutionsByWorkflowID(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListOpenWorkflowExecutionsByWorkflowID(gomock.Any(), gomock.Any()).DoAndReturn(func(
					ctx context.Context, request *ListWorkflowExecutionsByWorkflowIDRequest) (*ListWorkflowExecutionsResponse, error) {
					wg.Done()
					return nil, nil
				}).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dualStorePinotPrimary),
			wgCount:                 1,
			expectedError:           nil,
		},
		"Case2-2: double read with an error": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListOpenWorkflowExecutionsByWorkflowID(gomock.Any(), gomock.Any()).
					DoAndReturn(func(
						ctx context.Context, request *ListWorkflowExecutionsByWorkflowIDRequest) (*ListWorkflowExecutionsResponse, error) {
						wg.Done()
						return nil, fmt.Errorf("test error")
					}).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListOpenWorkflowExecutionsByWorkflowID(gomock.Any(), gomock.Any()).
					Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dualStoreName),
			wgCount:                 1,
			expectedError:           nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			wg := sync.WaitGroup{}
			wg.Add(test.wgCount)

			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAffordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAffordance(&wg, test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAffordance(&wg, test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityMgrs := map[string]VisibilityManager{
				dbVisStoreName: test.mockDBVisibilityManager,
				esStoreName:    test.mockESVisibilityManager,
				pinotStoreName: test.mockPinotVisibilityManager,
			}
			visibilityManager := NewVisibilityHybridManager(visibilityMgrs, test.readVisibilityStoreName, nil, dynamicproperties.GetBoolPropertyFnFilteredByDomain(true), testStoreName, log.NewNoop())

			_, err := visibilityManager.ListOpenWorkflowExecutionsByWorkflowID(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			wg.Wait()
		})
	}
}

func TestVisibilityHybridListClosedWorkflowExecutionsByWorkflowID(t *testing.T) {
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
		mockDBVisibilityManagerAffordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAffordance func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAffordance    func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager)
		readVisibilityStoreName              dynamicproperties.StringPropertyFnWithDomainFilter
		wgCount                              int
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAffordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ListClosedWorkflowExecutionsByWorkflowID(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dbVisStoreName),
			wgCount:                 0,
			expectedError:           nil,
		},
		"Case1-2: success case with Pinot visibility is not nil": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListClosedWorkflowExecutionsByWorkflowID(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(pinotStoreName),
			wgCount:                 0,
			expectedError:           nil,
		},
		"Case2-1: success case with double read": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListClosedWorkflowExecutionsByWorkflowID(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListClosedWorkflowExecutionsByWorkflowID(gomock.Any(), gomock.Any()).DoAndReturn(func(
					ctx context.Context, request *ListWorkflowExecutionsByWorkflowIDRequest) (*ListWorkflowExecutionsResponse, error) {
					wg.Done()
					return nil, nil
				}).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dualStorePinotPrimary),
			wgCount:                 1,
			expectedError:           nil,
		},
		"Case2-2: double read with an error": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListClosedWorkflowExecutionsByWorkflowID(gomock.Any(), gomock.Any()).
					DoAndReturn(func(
						ctx context.Context, request *ListWorkflowExecutionsByWorkflowIDRequest) (*ListWorkflowExecutionsResponse, error) {
						wg.Done()
						return nil, fmt.Errorf("test error")
					}).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListClosedWorkflowExecutionsByWorkflowID(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dualStoreName),
			wgCount:                 1,
			expectedError:           nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			wg := sync.WaitGroup{}
			wg.Add(test.wgCount)

			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAffordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAffordance(&wg, test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAffordance(&wg, test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityMgrs := map[string]VisibilityManager{
				dbVisStoreName: test.mockDBVisibilityManager,
				esStoreName:    test.mockESVisibilityManager,
				pinotStoreName: test.mockPinotVisibilityManager,
			}
			visibilityManager := NewVisibilityHybridManager(visibilityMgrs, test.readVisibilityStoreName, nil, dynamicproperties.GetBoolPropertyFnFilteredByDomain(true), testStoreName, log.NewNoop())

			_, err := visibilityManager.ListClosedWorkflowExecutionsByWorkflowID(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			wg.Wait()
		})
	}
}

func TestVisibilityHybridListClosedWorkflowExecutionsByStatus(t *testing.T) {
	defer goleak.VerifyNone(t)
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
		mockDBVisibilityManagerAffordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAffordance func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAffordance    func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager)
		readVisibilityStoreName              dynamicproperties.StringPropertyFnWithDomainFilter
		wgCount                              int
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAffordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ListClosedWorkflowExecutionsByStatus(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dbVisStoreName),
			wgCount:                 0,
			expectedError:           nil,
		},
		"Case1-2: success case with Pinot visibility is not nil": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListClosedWorkflowExecutionsByStatus(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(pinotStoreName),
			wgCount:                 0,
			expectedError:           nil,
		},
		"Case2-1: success case with double read": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListClosedWorkflowExecutionsByStatus(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListClosedWorkflowExecutionsByStatus(gomock.Any(), gomock.Any()).DoAndReturn(func(
					ctx context.Context, request *ListClosedWorkflowExecutionsByStatusRequest) (*ListWorkflowExecutionsResponse, error) {
					wg.Done()
					return nil, nil
				}).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dualStorePinotPrimary),
			wgCount:                 1,
			expectedError:           nil,
		},
		"Case2-2: double read with an error": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListClosedWorkflowExecutionsByStatus(gomock.Any(), gomock.Any()).
					DoAndReturn(func(
						ctx context.Context, request *ListClosedWorkflowExecutionsByStatusRequest) (*ListWorkflowExecutionsResponse, error) {
						wg.Done()
						return nil, fmt.Errorf("test error")
					}).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListClosedWorkflowExecutionsByStatus(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dualStoreName),
			wgCount:                 1,
			expectedError:           nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			wg := sync.WaitGroup{}
			wg.Add(test.wgCount)

			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAffordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAffordance(&wg, test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAffordance(&wg, test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityMgrs := map[string]VisibilityManager{
				dbVisStoreName: test.mockDBVisibilityManager,
				esStoreName:    test.mockESVisibilityManager,
				pinotStoreName: test.mockPinotVisibilityManager,
			}
			visibilityManager := NewVisibilityHybridManager(visibilityMgrs, test.readVisibilityStoreName, nil, dynamicproperties.GetBoolPropertyFnFilteredByDomain(true), testStoreName, log.NewNoop())

			_, err := visibilityManager.ListClosedWorkflowExecutionsByStatus(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			wg.Wait()
		})
	}
}

func TestVisibilityHybridGetClosedWorkflowExecution(t *testing.T) {
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
		mockDBVisibilityManagerAffordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAffordance func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAffordance    func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager)
		readVisibilityStoreName              dynamicproperties.StringPropertyFnWithDomainFilter
		wgCount                              int
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAffordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().GetClosedWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dbVisStoreName),
			wgCount:                 0,
			expectedError:           nil,
		},
		"Case1-2: success case with Pinot visibility is not nil": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().GetClosedWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(pinotStoreName),
			wgCount:                 0,
			expectedError:           nil,
		},
		"Case2-1: success case with double read": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().GetClosedWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().GetClosedWorkflowExecution(gomock.Any(), gomock.Any()).DoAndReturn(func(
					ctx context.Context, request *GetClosedWorkflowExecutionRequest) (*ListWorkflowExecutionsResponse, error) {
					wg.Done()
					return nil, nil
				}).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dualStorePinotPrimary),
			wgCount:                 1,
			expectedError:           nil,
		},
		"Case2-2: double read with an error": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().GetClosedWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(
						ctx context.Context, request *GetClosedWorkflowExecutionRequest) (*ListWorkflowExecutionsResponse, error) {
						wg.Done()
						return nil, fmt.Errorf("test error")
					}).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().GetClosedWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dualStoreName),
			wgCount:                 1,
			expectedError:           nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			wg := sync.WaitGroup{}
			wg.Add(test.wgCount)

			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAffordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAffordance(&wg, test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAffordance(&wg, test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityMgrs := map[string]VisibilityManager{
				dbVisStoreName: test.mockDBVisibilityManager,
				esStoreName:    test.mockESVisibilityManager,
				pinotStoreName: test.mockPinotVisibilityManager,
			}
			visibilityManager := NewVisibilityHybridManager(visibilityMgrs, test.readVisibilityStoreName, nil, dynamicproperties.GetBoolPropertyFnFilteredByDomain(true), testStoreName, log.NewNoop())

			_, err := visibilityManager.GetClosedWorkflowExecution(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			wg.Wait()
		})
	}
}

func TestVisibilityHybridListWorkflowExecutions(t *testing.T) {
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
		mockDBVisibilityManagerAffordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAffordance func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAffordance    func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager)
		readVisibilityStoreName              dynamicproperties.StringPropertyFnWithDomainFilter
		wgCount                              int
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAffordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ListWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dbVisStoreName),
			wgCount:                 0,
			expectedError:           nil,
		},
		"Case1-2: success case with Pinot visibility is not nil": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(pinotStoreName),
			wgCount:                 0,
			expectedError:           nil,
		},
		"Case2-1: success case with double read": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListWorkflowExecutions(gomock.Any(), gomock.Any()).DoAndReturn(func(
					ctx context.Context, request *ListWorkflowExecutionsByQueryRequest) (*ListWorkflowExecutionsResponse, error) {
					wg.Done()
					return nil, fmt.Errorf("test error")
				}).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dualStorePinotPrimary),
			wgCount:                 1,
			expectedError:           nil,
		},
		"Case2-2: double read with an error": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ListWorkflowExecutions(gomock.Any(), gomock.Any()).
					DoAndReturn(func(
						ctx context.Context, request *ListWorkflowExecutionsByQueryRequest) (*ListWorkflowExecutionsResponse, error) {
						wg.Done()
						return nil, fmt.Errorf("test error")
					}).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ListWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dualStoreName),
			wgCount:                 1,
			expectedError:           nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			wg := sync.WaitGroup{}
			wg.Add(test.wgCount)

			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAffordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAffordance(&wg, test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAffordance(&wg, test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityMgrs := map[string]VisibilityManager{
				dbVisStoreName: test.mockDBVisibilityManager,
				esStoreName:    test.mockESVisibilityManager,
				pinotStoreName: test.mockPinotVisibilityManager,
			}
			visibilityManager := NewVisibilityHybridManager(visibilityMgrs, test.readVisibilityStoreName, nil, dynamicproperties.GetBoolPropertyFnFilteredByDomain(true), testStoreName, log.NewNoop())

			_, err := visibilityManager.ListWorkflowExecutions(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			wg.Wait()
		})
	}
}

func TestVisibilityHybridScanWorkflowExecutions(t *testing.T) {
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
		mockDBVisibilityManagerAffordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAffordance func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAffordance    func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager)
		readVisibilityStoreName              dynamicproperties.StringPropertyFnWithDomainFilter
		wgCount                              int
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAffordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().ScanWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dbVisStoreName),
			wgCount:                 0,
			expectedError:           nil,
		},
		"Case1-2: success case with Pinot visibility is not nil": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ScanWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(pinotStoreName),
			wgCount:                 0,
			expectedError:           nil,
		},
		"Case2-1: success case with double read": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ScanWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ScanWorkflowExecutions(gomock.Any(), gomock.Any()).DoAndReturn(func(
					ctx context.Context, request *ListWorkflowExecutionsByQueryRequest) (*ListWorkflowExecutionsResponse, error) {
					wg.Done()
					return nil, nil
				}).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dualStorePinotPrimary),
			wgCount:                 1,
			expectedError:           nil,
		},
		"Case2-2: double read with an error": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().ScanWorkflowExecutions(gomock.Any(), gomock.Any()).
					DoAndReturn(func(
						ctx context.Context, request *ListWorkflowExecutionsByQueryRequest) (*ListWorkflowExecutionsResponse, error) {
						wg.Done()
						return nil, fmt.Errorf("test error")
					}).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().ScanWorkflowExecutions(gomock.Any(), gomock.Any()).
					Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dualStoreName),
			wgCount:                 1,
			expectedError:           nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			wg := sync.WaitGroup{}
			wg.Add(test.wgCount)

			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAffordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAffordance(&wg, test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAffordance(&wg, test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityMgrs := map[string]VisibilityManager{
				dbVisStoreName: test.mockDBVisibilityManager,
				esStoreName:    test.mockESVisibilityManager,
				pinotStoreName: test.mockPinotVisibilityManager,
			}
			visibilityManager := NewVisibilityHybridManager(visibilityMgrs, test.readVisibilityStoreName, nil, dynamicproperties.GetBoolPropertyFnFilteredByDomain(true), testStoreName, log.NewNoop())

			_, err := visibilityManager.ScanWorkflowExecutions(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			wg.Wait()
		})
	}
}

func TestVisibilityHybridCountWorkflowExecutions(t *testing.T) {
	request := &CountWorkflowExecutionsRequest{
		Domain: "test-domain",
	}

	ctrl := gomock.NewController(t)

	tests := map[string]struct {
		request                              *CountWorkflowExecutionsRequest
		mockDBVisibilityManager              VisibilityManager
		mockESVisibilityManager              VisibilityManager
		mockPinotVisibilityManager           VisibilityManager
		mockDBVisibilityManagerAffordance    func(mockDBVisibilityManager *MockVisibilityManager)
		mockPinotVisibilityManagerAffordance func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager)
		mockESVisibilityManagerAffordance    func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager)
		readVisibilityStoreName              dynamicproperties.StringPropertyFnWithDomainFilter
		wgCount                              int
		expectedError                        error
	}{
		"Case1-1: success case with DB visibility is not nil": {
			request:                 request,
			mockDBVisibilityManager: NewMockVisibilityManager(ctrl),
			mockDBVisibilityManagerAffordance: func(mockDBVisibilityManager *MockVisibilityManager) {
				mockDBVisibilityManager.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dbVisStoreName),
			wgCount:                 0,
			expectedError:           nil,
		},
		"Case1-2: success case with Pinot visibility is not nil": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(pinotStoreName),
			wgCount:                 0,
			expectedError:           nil,
		},
		"Case2-1: success case with double read": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).DoAndReturn(func(
					ctx context.Context, request *CountWorkflowExecutionsRequest) (*ListWorkflowExecutionsResponse, error) {
					wg.Done()
					return nil, nil
				}).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dualStorePinotPrimary),
			wgCount:                 1,
			expectedError:           nil,
		},
		"Case2-2: double read with an error": {
			request:                    request,
			mockPinotVisibilityManager: NewMockVisibilityManager(ctrl),
			mockPinotVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockPinotVisibilityManager *MockVisibilityManager) {
				mockPinotVisibilityManager.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).
					DoAndReturn(func(
						ctx context.Context, request *CountWorkflowExecutionsRequest) (*ListWorkflowExecutionsResponse, error) {
						wg.Done()
						return nil, fmt.Errorf("test error")
					}).Times(1)
			},
			mockESVisibilityManager: NewMockVisibilityManager(ctrl),
			mockESVisibilityManagerAffordance: func(wg *sync.WaitGroup, mockESVisibilityManager *MockVisibilityManager) {
				mockESVisibilityManager.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			},
			readVisibilityStoreName: dynamicproperties.GetStringPropertyFnFilteredByDomain(dualStoreName),
			wgCount:                 1,
			expectedError:           nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			wg := sync.WaitGroup{}
			wg.Add(test.wgCount)

			if test.mockDBVisibilityManager != nil {
				test.mockDBVisibilityManagerAffordance(test.mockDBVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockPinotVisibilityManager != nil {
				test.mockPinotVisibilityManagerAffordance(&wg, test.mockPinotVisibilityManager.(*MockVisibilityManager))
			}
			if test.mockESVisibilityManager != nil {
				test.mockESVisibilityManagerAffordance(&wg, test.mockESVisibilityManager.(*MockVisibilityManager))
			}

			visibilityMgrs := map[string]VisibilityManager{
				dbVisStoreName: test.mockDBVisibilityManager,
				esStoreName:    test.mockESVisibilityManager,
				pinotStoreName: test.mockPinotVisibilityManager,
			}
			visibilityManager := NewVisibilityHybridManager(visibilityMgrs, test.readVisibilityStoreName, nil, dynamicproperties.GetBoolPropertyFnFilteredByDomain(true), testStoreName, log.NewNoop())

			_, err := visibilityManager.CountWorkflowExecutions(context.Background(), test.request)
			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			wg.Wait()
		})
	}
}
