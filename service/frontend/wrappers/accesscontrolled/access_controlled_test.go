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
	"github.com/stretchr/testify/mock"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/authorization"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/metrics/mocks"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/frontend/admin"
	"github.com/uber/cadence/service/frontend/api"
)

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

func TestListDomainsFiltersUnauthorizedDomains(t *testing.T) {
	controller := gomock.NewController(t)

	mockAuthorizer := authorization.NewMockAuthorizer(controller)
	mockAPIHandler := api.NewMockHandler(controller)
	mockResource := resource.NewMockResource(controller)
	mockResource.EXPECT().GetMetricsClient().Return(metrics.NewNoopMetricsClient()).AnyTimes()

	request := &types.ListDomainsRequest{PageSize: 10}
	response := &types.ListDomainsResponse{
		Domains: []*types.DescribeDomainResponse{
			listDomainsTestResponse("allowed-domain"),
			listDomainsTestResponse("denied-domain"),
		},
		NextPageToken: []byte("next-page"),
	}
	mockAPIHandler.EXPECT().ListDomains(gomock.Any(), request).Return(response, nil)
	mockAuthorizer.EXPECT().
		Authorize(gomock.Any(), listDomainsAuthAttr("allowed-domain")).
		Return(authorization.Result{Decision: authorization.DecisionAllow}, nil)
	mockAuthorizer.EXPECT().
		Authorize(gomock.Any(), listDomainsAuthAttr("denied-domain")).
		Return(authorization.Result{Decision: authorization.DecisionDeny}, nil)

	handler := &apiHandler{
		handler:    mockAPIHandler,
		authorizer: mockAuthorizer,
		Resource:   mockResource,
	}
	result, err := handler.ListDomains(context.Background(), request)

	assert.NoError(t, err)
	assert.Equal(t, []byte("next-page"), result.NextPageToken)
	assert.Len(t, result.Domains, 1)
	assert.Equal(t, "allowed-domain", result.Domains[0].GetDomainInfo().GetName())
}

func TestListDomainsReturnsEmptyWhenNoDomainsAreAuthorized(t *testing.T) {
	controller := gomock.NewController(t)

	mockAuthorizer := authorization.NewMockAuthorizer(controller)
	mockAPIHandler := api.NewMockHandler(controller)
	mockResource := resource.NewMockResource(controller)
	mockResource.EXPECT().GetMetricsClient().Return(metrics.NewNoopMetricsClient()).AnyTimes()

	request := &types.ListDomainsRequest{PageSize: 10}
	response := &types.ListDomainsResponse{
		Domains: []*types.DescribeDomainResponse{
			listDomainsTestResponse("denied-domain"),
		},
	}
	mockAPIHandler.EXPECT().ListDomains(gomock.Any(), request).Return(response, nil)
	mockAuthorizer.EXPECT().
		Authorize(gomock.Any(), listDomainsAuthAttr("denied-domain")).
		Return(authorization.Result{Decision: authorization.DecisionDeny}, nil)

	handler := &apiHandler{
		handler:    mockAPIHandler,
		authorizer: mockAuthorizer,
		Resource:   mockResource,
	}
	result, err := handler.ListDomains(context.Background(), request)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Domains)
}

func TestListDomainsReturnsEmptyPageWithNextPageTokenWhenNoDomainsAreAuthorized(t *testing.T) {
	controller := gomock.NewController(t)

	mockAuthorizer := authorization.NewMockAuthorizer(controller)
	mockAPIHandler := api.NewMockHandler(controller)
	mockResource := resource.NewMockResource(controller)
	mockResource.EXPECT().GetMetricsClient().Return(metrics.NewNoopMetricsClient()).AnyTimes()

	request := &types.ListDomainsRequest{PageSize: 10}
	response := &types.ListDomainsResponse{
		Domains: []*types.DescribeDomainResponse{
			listDomainsTestResponse("denied-domain"),
		},
		NextPageToken: []byte("next-page"),
	}
	mockAPIHandler.EXPECT().ListDomains(gomock.Any(), request).Return(response, nil)
	mockAuthorizer.EXPECT().
		Authorize(gomock.Any(), listDomainsAuthAttr("denied-domain")).
		Return(authorization.Result{Decision: authorization.DecisionDeny}, nil)

	handler := &apiHandler{
		handler:    mockAPIHandler,
		authorizer: mockAuthorizer,
		Resource:   mockResource,
	}
	result, err := handler.ListDomains(context.Background(), request)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, []byte("next-page"), result.NextPageToken)
	assert.Empty(t, result.Domains)
}

func TestListDomainsPropagatesHandlerError(t *testing.T) {
	controller := gomock.NewController(t)

	mockAuthorizer := authorization.NewMockAuthorizer(controller)
	mockAPIHandler := api.NewMockHandler(controller)
	listDomainsErr := errors.New("list domains failed")

	request := &types.ListDomainsRequest{PageSize: 10}
	mockAPIHandler.EXPECT().ListDomains(gomock.Any(), request).Return(nil, listDomainsErr)

	handler := &apiHandler{
		handler:    mockAPIHandler,
		authorizer: mockAuthorizer,
	}
	result, err := handler.ListDomains(context.Background(), request)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, listDomainsErr)
}

func TestListDomainsPropagatesAuthorizerError(t *testing.T) {
	controller := gomock.NewController(t)

	mockAuthorizer := authorization.NewMockAuthorizer(controller)
	mockAPIHandler := api.NewMockHandler(controller)
	mockResource := resource.NewMockResource(controller)
	mockResource.EXPECT().GetMetricsClient().Return(metrics.NewNoopMetricsClient()).AnyTimes()

	request := &types.ListDomainsRequest{PageSize: 10}
	response := &types.ListDomainsResponse{
		Domains: []*types.DescribeDomainResponse{
			listDomainsTestResponse("allowed-domain"),
			listDomainsTestResponse("error-domain"),
			listDomainsTestResponse("unchecked-domain"),
		},
	}
	authorizerErr := errors.New("authorizer failed")
	mockAPIHandler.EXPECT().ListDomains(gomock.Any(), request).Return(response, nil)
	gomock.InOrder(
		mockAuthorizer.EXPECT().
			Authorize(gomock.Any(), listDomainsAuthAttr("allowed-domain")).
			Return(authorization.Result{Decision: authorization.DecisionAllow}, nil),
		mockAuthorizer.EXPECT().
			Authorize(gomock.Any(), listDomainsAuthAttr("error-domain")).
			Return(authorization.Result{}, authorizerErr),
	)

	handler := &apiHandler{
		handler:    mockAPIHandler,
		authorizer: mockAuthorizer,
		Resource:   mockResource,
	}
	result, err := handler.ListDomains(context.Background(), request)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, authorizerErr)
}

func TestListDomainsReturnsAllWhenAllAuthorized(t *testing.T) {
	controller := gomock.NewController(t)

	mockAuthorizer := authorization.NewMockAuthorizer(controller)
	mockAPIHandler := api.NewMockHandler(controller)
	mockResource := resource.NewMockResource(controller)
	mockResource.EXPECT().GetMetricsClient().Return(metrics.NewNoopMetricsClient()).AnyTimes()

	request := &types.ListDomainsRequest{PageSize: 10}
	response := &types.ListDomainsResponse{
		Domains: []*types.DescribeDomainResponse{
			listDomainsTestResponse("first-domain"),
			listDomainsTestResponse("second-domain"),
		},
	}
	mockAPIHandler.EXPECT().ListDomains(gomock.Any(), request).Return(response, nil)
	mockAuthorizer.EXPECT().
		Authorize(gomock.Any(), listDomainsAuthAttr("first-domain")).
		Return(authorization.Result{Decision: authorization.DecisionAllow}, nil)
	mockAuthorizer.EXPECT().
		Authorize(gomock.Any(), listDomainsAuthAttr("second-domain")).
		Return(authorization.Result{Decision: authorization.DecisionAllow}, nil)

	handler := &apiHandler{
		handler:    mockAPIHandler,
		authorizer: mockAuthorizer,
		Resource:   mockResource,
	}
	result, err := handler.ListDomains(context.Background(), request)

	assert.NoError(t, err)
	assert.Len(t, result.Domains, 2)
	assert.Equal(t, "first-domain", result.Domains[0].GetDomainInfo().GetName())
	assert.Equal(t, "second-domain", result.Domains[1].GetDomainInfo().GetName())
}

func listDomainsTestResponse(name string) *types.DescribeDomainResponse {
	return &types.DescribeDomainResponse{
		DomainInfo: &types.DomainInfo{Name: name},
	}
}

func listDomainsAuthAttr(domain string) gomock.Matcher {
	return gomock.Cond(func(attr *authorization.Attributes) bool {
		return attr.APIName == "ListDomains" &&
			attr.Permission == authorization.PermissionRead &&
			attr.DomainName == domain &&
			attr.RequestBody != nil
	})
}
