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
	"time"

	p8s "github.com/m3db/prometheus_client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"
	tallyp8s "github.com/uber-go/tally/prometheus"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/authorization"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/metrics/mocks"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/frontend/admin"
	"github.com/uber/cadence/service/frontend/api"
)

func TestAuthorizationMetricsLabelConsistency(t *testing.T) {
	var registrationErrors []error
	promCfg := &tallyp8s.Configuration{
		OnError:   "none",
		TimerType: "histogram",
	}
	reporter, err := promCfg.NewReporter(tallyp8s.ConfigurationOptions{
		Registry: p8s.NewRegistry(),
		OnError: func(err error) {
			registrationErrors = append(registrationErrors, err)
		},
	})
	require.NoError(t, err)

	rootScope, closer := tally.NewRootScope(tally.ScopeOptions{
		Tags: map[string]string{
			metrics.CadenceServiceTagName: "frontend",
		},
		CachedReporter: reporter,
		Separator:      tallyp8s.DefaultSeparator,
	}, time.Second)
	defer closer.Close()

	frontendMetrics := metrics.NewClient(rootScope, metrics.Frontend, metrics.MigrationConfig{})

	ctrl := gomock.NewController(t)
	mockResource := resource.NewMockResource(ctrl)
	mockResource.EXPECT().GetMetricsClient().Return(frontendMetrics).AnyTimes()

	mockAuthorizer := authorization.NewMockAuthorizer(ctrl)
	mockAuthorizer.EXPECT().Authorize(gomock.Any(), gomock.Any()).Return(authorization.Result{Decision: authorization.DecisionAllow}, nil).Times(2)

	mockHandler := api.NewMockHandler(ctrl)
	mockHandler.EXPECT().RegisterDomain(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	mockHandler.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).Return(&types.DescribeWorkflowExecutionResponse{}, nil).Times(1)

	handler := NewAPIHandler(mockHandler, mockResource, mockAuthorizer, config.Authorization{})

	ctx := context.Background()
	_, err = handler.DescribeWorkflowExecution(ctx, &types.DescribeWorkflowExecutionRequest{Domain: "my-domain"})
	require.NoError(t, err)
	err = handler.RegisterDomain(ctx, &types.RegisterDomainRequest{Name: "my-name"})
	require.NoError(t, err)

	assert.Empty(t, registrationErrors, "Prometheus registration errors must not be emitted")
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
				scope.On("ExponentialHistogram", metrics.CadenceAuthorizationLatencyHistogram, mock.AnythingOfType("time.Duration")).Return().Once()
			},
			isAuthorized: true,
			wantErr:      false,
		},
		{
			name: "Error case - unauthorized",
			mockSetup: func(authorizer *authorization.MockAuthorizer, scope *mocks.Scope) {
				authorizer.EXPECT().Authorize(gomock.Any(), gomock.Any()).Return(authorization.Result{Decision: authorization.DecisionDeny}, nil)
				scope.On("StartTimer", metrics.CadenceAuthorizationLatency).Return(metrics.NewTestStopwatch()).Once()
				scope.On("ExponentialHistogram", metrics.CadenceAuthorizationLatencyHistogram, mock.AnythingOfType("time.Duration")).Return().Once()
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
				scope.On("ExponentialHistogram", metrics.CadenceAuthorizationLatencyHistogram, mock.AnythingOfType("time.Duration")).Return().Once()
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
		mockSetup func(*authorization.MockAuthorizer, *admin.MockHandler)
		wantErr   error
	}{
		{
			name: "Success case",
			mockSetup: func(authorizer *authorization.MockAuthorizer, adminHandler *admin.MockHandler) {
				authorizer.EXPECT().Authorize(gomock.Any(), gomock.Any()).Return(authorization.Result{Decision: authorization.DecisionAllow}, nil)
				adminHandler.EXPECT().DescribeCluster(gomock.Any()).Return(&types.DescribeClusterResponse{}, nil)
			},
			wantErr: nil,
		},
		{
			name: "Error case - unauthorized",
			mockSetup: func(authorizer *authorization.MockAuthorizer, adminHandler *admin.MockHandler) {
				authorizer.EXPECT().Authorize(gomock.Any(), gomock.Any()).Return(authorization.Result{Decision: authorization.DecisionDeny}, nil)
			},
			wantErr: errUnauthorized,
		},
		{
			name: "Error case - authorization error",
			mockSetup: func(authorizer *authorization.MockAuthorizer, adminHandler *admin.MockHandler) {
				authorizer.EXPECT().Authorize(gomock.Any(), gomock.Any()).Return(authorization.Result{}, someErr)
			},
			wantErr: someErr,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			controller := gomock.NewController(t)

			mockAuthorizer := authorization.NewMockAuthorizer(controller)
			mockAdminHandler := admin.NewMockHandler(controller)
			tc.mockSetup(mockAuthorizer, mockAdminHandler)

			handler := &adminHandler{authorizer: mockAuthorizer, handler: mockAdminHandler}
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

func TestListDomainsAuthorization(t *testing.T) {
	someErr := errors.New("some random err")
	nextPageToken := []byte("next-page")
	testCases := []struct {
		name              string
		mockSetup         func(*authorization.MockAuthorizer, *api.MockHandler, *types.ListDomainsRequest)
		wantDomains       []string
		wantNextPageToken []byte
		wantErr           error
	}{
		{
			name: "unauthenticated request",
			mockSetup: func(authorizer *authorization.MockAuthorizer, _ *api.MockHandler, _ *types.ListDomainsRequest) {
				authorizer.EXPECT().
					Authorize(gomock.Any(), listDomainsAuthAttr("")).
					Return(authorization.Result{Decision: authorization.DecisionUnauthenticated}, nil)
			},
			wantErr: errUnauthorized,
		},
		{
			name: "operation denied",
			mockSetup: func(authorizer *authorization.MockAuthorizer, _ *api.MockHandler, _ *types.ListDomainsRequest) {
				authorizer.EXPECT().
					Authorize(gomock.Any(), listDomainsAuthAttr("")).
					Return(authorization.Result{Decision: authorization.DecisionDeny}, nil)
			},
			wantErr: errUnauthorized,
		},
		{
			name: "filters unauthorized domains",
			mockSetup: func(authorizer *authorization.MockAuthorizer, handler *api.MockHandler, request *types.ListDomainsRequest) {
				gomock.InOrder(
					authorizer.EXPECT().Authorize(gomock.Any(), listDomainsAuthAttr("")).Return(authorization.Result{Decision: authorization.DecisionAllow}, nil),
					handler.EXPECT().ListDomains(gomock.Any(), request).Return(listDomainsResponse(nextPageToken, "allowed-domain", "denied-domain"), nil),
					authorizer.EXPECT().Authorize(gomock.Any(), listDomainsAuthAttr("allowed-domain")).Return(authorization.Result{Decision: authorization.DecisionAllow}, nil),
					authorizer.EXPECT().Authorize(gomock.Any(), listDomainsAuthAttr("denied-domain")).Return(authorization.Result{Decision: authorization.DecisionDeny}, nil),
				)
			},
			wantDomains:       []string{"allowed-domain"},
			wantNextPageToken: nextPageToken,
		},
		{
			name: "preserves empty page",
			mockSetup: func(authorizer *authorization.MockAuthorizer, handler *api.MockHandler, request *types.ListDomainsRequest) {
				gomock.InOrder(
					authorizer.EXPECT().Authorize(gomock.Any(), listDomainsAuthAttr("")).Return(authorization.Result{Decision: authorization.DecisionAllow}, nil),
					handler.EXPECT().ListDomains(gomock.Any(), request).Return(listDomainsResponse(nextPageToken), nil),
				)
			},
			wantNextPageToken: nextPageToken,
		},
		{
			name: "preserves pagination when all domains are denied",
			mockSetup: func(authorizer *authorization.MockAuthorizer, handler *api.MockHandler, request *types.ListDomainsRequest) {
				gomock.InOrder(
					authorizer.EXPECT().Authorize(gomock.Any(), listDomainsAuthAttr("")).Return(authorization.Result{Decision: authorization.DecisionAllow}, nil),
					handler.EXPECT().ListDomains(gomock.Any(), request).Return(listDomainsResponse(nextPageToken, "denied-domain"), nil),
					authorizer.EXPECT().Authorize(gomock.Any(), listDomainsAuthAttr("denied-domain")).Return(authorization.Result{Decision: authorization.DecisionDeny}, nil),
				)
			},
			wantNextPageToken: nextPageToken,
		},
		{
			name: "returns all authorized domains",
			mockSetup: func(authorizer *authorization.MockAuthorizer, handler *api.MockHandler, request *types.ListDomainsRequest) {
				gomock.InOrder(
					authorizer.EXPECT().Authorize(gomock.Any(), listDomainsAuthAttr("")).Return(authorization.Result{Decision: authorization.DecisionAllow}, nil),
					handler.EXPECT().ListDomains(gomock.Any(), request).Return(listDomainsResponse(nil, "first-domain", "second-domain"), nil),
					authorizer.EXPECT().Authorize(gomock.Any(), listDomainsAuthAttr("first-domain")).Return(authorization.Result{Decision: authorization.DecisionAllow}, nil),
					authorizer.EXPECT().Authorize(gomock.Any(), listDomainsAuthAttr("second-domain")).Return(authorization.Result{Decision: authorization.DecisionAllow}, nil),
				)
			},
			wantDomains: []string{"first-domain", "second-domain"},
		},
		{
			name: "handler error",
			mockSetup: func(authorizer *authorization.MockAuthorizer, handler *api.MockHandler, request *types.ListDomainsRequest) {
				gomock.InOrder(
					authorizer.EXPECT().Authorize(gomock.Any(), listDomainsAuthAttr("")).Return(authorization.Result{Decision: authorization.DecisionAllow}, nil),
					handler.EXPECT().ListDomains(gomock.Any(), request).Return(nil, someErr),
				)
			},
			wantErr: someErr,
		},
		{
			name: "authorizer error",
			mockSetup: func(authorizer *authorization.MockAuthorizer, handler *api.MockHandler, request *types.ListDomainsRequest) {
				gomock.InOrder(
					authorizer.EXPECT().Authorize(gomock.Any(), listDomainsAuthAttr("")).Return(authorization.Result{Decision: authorization.DecisionAllow}, nil),
					handler.EXPECT().ListDomains(gomock.Any(), request).Return(listDomainsResponse(nil, "error-domain"), nil),
					authorizer.EXPECT().Authorize(gomock.Any(), listDomainsAuthAttr("error-domain")).Return(authorization.Result{}, someErr),
				)
			},
			wantErr: someErr,
		},
		{
			name: "authentication expires while filtering",
			mockSetup: func(authorizer *authorization.MockAuthorizer, handler *api.MockHandler, request *types.ListDomainsRequest) {
				gomock.InOrder(
					authorizer.EXPECT().Authorize(gomock.Any(), listDomainsAuthAttr("")).Return(authorization.Result{Decision: authorization.DecisionAllow}, nil),
					handler.EXPECT().ListDomains(gomock.Any(), request).Return(listDomainsResponse(nil, "domain"), nil),
					authorizer.EXPECT().Authorize(gomock.Any(), listDomainsAuthAttr("domain")).Return(authorization.Result{Decision: authorization.DecisionUnauthenticated}, nil),
				)
			},
			wantErr: errUnauthorized,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockAuthorizer := authorization.NewMockAuthorizer(ctrl)
			mockHandler := api.NewMockHandler(ctrl)
			mockResource := resource.NewMockResource(ctrl)
			mockResource.EXPECT().GetMetricsClient().Return(metrics.NewNoopMetricsClient()).AnyTimes()
			request := &types.ListDomainsRequest{PageSize: 10}
			tc.mockSetup(mockAuthorizer, mockHandler, request)

			handler := NewAPIHandler(mockHandler, mockResource, mockAuthorizer, config.Authorization{})
			response, err := handler.ListDomains(context.Background(), request)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, response)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, response)
			assert.Equal(t, tc.wantNextPageToken, response.NextPageToken)
			assert.Equal(t, tc.wantDomains, domainNames(response.Domains))
		})
	}
}

func listDomainsAuthAttr(domain string) gomock.Matcher {
	return gomock.Cond(func(attr *authorization.Attributes) bool {
		return attr.APIName == "ListDomains" &&
			attr.Permission == authorization.PermissionRead &&
			attr.DomainName == domain &&
			attr.AuthenticationOnly == (domain == "") &&
			attr.RequestBody != nil
	})
}

func listDomainsResponse(nextPageToken []byte, domains ...string) *types.ListDomainsResponse {
	response := &types.ListDomainsResponse{NextPageToken: nextPageToken}
	for _, domain := range domains {
		response.Domains = append(response.Domains, &types.DescribeDomainResponse{
			DomainInfo: &types.DomainInfo{Name: domain},
		})
	}
	return response
}

func domainNames(domains []*types.DescribeDomainResponse) []string {
	if len(domains) == 0 {
		return nil
	}

	names := make([]string, 0, len(domains))
	for _, domain := range domains {
		names = append(names, domain.GetDomainInfo().GetName())
	}
	return names
}
