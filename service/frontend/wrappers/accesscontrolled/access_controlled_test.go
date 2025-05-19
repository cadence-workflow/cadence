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

package accesscontrolled

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/authorization"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/metrics/mocks"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/frontend/admin"
)

var errFromRecoveredPanic = errors.New("simulated panic error")

type fakeResource struct {
	logger log.Logger
}

func (r *fakeResource) GetLogger() log.Logger {
	return r.logger
}

func TestIsAuthorized(t *testing.T) {
	testCases := []struct {
		name         string
		mockSetup    func(*authorization.MockAuthorizer, *mocks.Scope)
		isAuthorized bool
		wantErr      bool
	}{
		{
			name: "Succes case",
			mockSetup: func(authorizer *authorization.MockAuthorizer, scope *mocks.Scope) {
				authorizer.EXPECT().Authorize(gomock.Any(), gomock.Any()).Return(authorization.Result{Decision: authorization.DecisionAllow}, nil)
				scope.On("StartTimer", metrics.CadenceAuthorizationLatency).Return(metrics.NewTestStopwatch()).Once()
			},
			isAuthorized: true,
			wantErr:      false,
		},
		{
			name: "Error case - unauthorized",
			mockSetup: func(authorizer *authorization.MockAuthorizer, scope *mocks.Scope) {
				authorizer.EXPECT().Authorize(gomock.Any(), gomock.Any()).Return(authorization.Result{Decision: authorization.DecisionDeny}, nil)
				scope.On("StartTimer", metrics.CadenceAuthorizationLatency).Return(metrics.NewTestStopwatch()).Once()
				scope.On("IncCounter", metrics.CadenceErrUnauthorizedCounter).Return().Once()
			},
			isAuthorized: false,
			wantErr:      false,
		},
		{
			name: "Error case - authorization error",
			mockSetup: func(authorizer *authorization.MockAuthorizer, scope *mocks.Scope) {
				authorizer.EXPECT().Authorize(gomock.Any(), gomock.Any()).Return(authorization.Result{}, errors.New("some random error"))
				scope.On("StartTimer", metrics.CadenceAuthorizationLatency).Return(metrics.NewTestStopwatch()).Once()
				scope.On("IncCounter", metrics.CadenceErrAuthorizeFailedCounter).Return().Once()
			},
			isAuthorized: false,
			wantErr:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			controller := gomock.NewController(t)

			mockAuthorizer := authorization.NewMockAuthorizer(controller)
			mockMetricsScope := &mocks.Scope{}
			tc.mockSetup(mockAuthorizer, mockMetricsScope)

			handler := &apiHandler{authorizer: mockAuthorizer}
			got, err := handler.isAuthorized(context.Background(), &authorization.Attributes{}, mockMetricsScope)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.isAuthorized, got)
			}
		})
	}
}

func TestDescribeCluster(t *testing.T) {
	someErr := errors.New("some random err")
	testCases := []struct {
		name      string
		mockSetup func(*authorization.MockAuthorizer, *admin.MockHandler, *resource.MockResource)
		wantErr   error
	}{
		{
			name: "Success case",
			mockSetup: func(authorizer *authorization.MockAuthorizer, adminHandler *admin.MockHandler, mockResource *resource.MockResource) {
				authorizer.EXPECT().Authorize(gomock.Any(), gomock.Any()).Return(authorization.Result{Decision: authorization.DecisionAllow}, nil)
				adminHandler.EXPECT().DescribeCluster(gomock.Any()).Return(&types.DescribeClusterResponse{}, nil)
			},
			wantErr: nil,
		},
		{
			name: "Error case - unauthorized",
			mockSetup: func(authorizer *authorization.MockAuthorizer, adminHandler *admin.MockHandler, mockResource *resource.MockResource) {
				authorizer.EXPECT().Authorize(gomock.Any(), gomock.Any()).Return(authorization.Result{Decision: authorization.DecisionDeny}, nil)
			},
			wantErr: errUnauthorized,
		},
		{
			name: "Error case - authorization error",
			mockSetup: func(authorizer *authorization.MockAuthorizer, adminHandler *admin.MockHandler, mockResource *resource.MockResource) {
				authorizer.EXPECT().Authorize(gomock.Any(), gomock.Any()).Return(authorization.Result{}, someErr)
			},
			wantErr: someErr,
		},
		{
			name: "Panic case - handler panics",
			mockSetup: func(authorizer *authorization.MockAuthorizer, adminHandler *admin.MockHandler, mockResource *resource.MockResource) {
				authorizer.EXPECT().Authorize(gomock.Any(), gomock.Any()).
					Return(authorization.Result{Decision: authorization.DecisionAllow}, nil)
				mockResource.EXPECT().GetLogger().Return(log.NewNoop())
				adminHandler.EXPECT().DescribeCluster(gomock.Any()).
					DoAndReturn(func(context.Context) (*types.DescribeClusterResponse, error) {
						panic(errFromRecoveredPanic)
					})
			},
			wantErr: errFromRecoveredPanic,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			controller := gomock.NewController(t)

			mockAuthorizer := authorization.NewMockAuthorizer(controller)
			mockAdminHandler := admin.NewMockHandler(controller)
			mockResource := resource.NewMockResource(controller)
			tc.mockSetup(mockAuthorizer, mockAdminHandler, mockResource)

			handler := &adminHandler{
				authorizer: mockAuthorizer,
				handler:    mockAdminHandler,
				Resource:   mockResource,
			}
			_, err := handler.DescribeCluster(context.Background())
			if tc.wantErr != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
