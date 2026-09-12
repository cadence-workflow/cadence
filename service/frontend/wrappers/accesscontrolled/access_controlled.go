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
	"time"

	"github.com/uber/cadence/common/authorization"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

var errUnauthorized = &types.AccessDeniedError{Message: "Request unauthorized."}

func (a *adminHandler) isAuthorized(ctx context.Context, attr *authorization.Attributes) (bool, error) {
	result, err := a.authorizer.Authorize(ctx, attr)
	if err != nil {
		return false, err
	}
	isAuth := result.Decision == authorization.DecisionAllow
	return isAuth, nil
}

func (a *apiHandler) isAuthorized(
	ctx context.Context,
	attr *authorization.Attributes,
	scope metrics.Scope,
) (bool, error) {
	return a.checkAccess(ctx, a.authorizer, attr, scope)
}

func (a *apiHandler) isAuthenticated(
	ctx context.Context,
	attr *authorization.Attributes,
	scope metrics.Scope,
) (bool, error) {
	return a.checkAccess(ctx, a.authenticator, attr, scope)
}

func (a *apiHandler) checkAccess(
	ctx context.Context,
	authorizer authorization.Authorizer,
	attr *authorization.Attributes,
	scope metrics.Scope,
) (bool, error) {
	authStart := time.Now()
	sw := scope.StartTimer(metrics.CadenceAuthorizationLatency)
	defer func() {
		sw.Stop()
		scope.ExponentialHistogram(metrics.CadenceAuthorizationLatencyHistogram, time.Since(authStart))
	}()

	result, err := authorizer.Authorize(ctx, attr)
	if err != nil {
		scope.IncCounter(metrics.CadenceErrAuthorizeFailedCounter)
		return false, err
	}
	isAuthorized := result.Decision == authorization.DecisionAllow
	if !isAuthorized {
		scope.IncCounter(metrics.CadenceErrUnauthorizedCounter)
	}
	return isAuthorized, nil
}

// authorizeFilter checks individual resources without recording request authorization metrics.
func (a *apiHandler) authorizeFilter(ctx context.Context, attr *authorization.Attributes) (bool, error) {
	result, err := a.authorizer.Authorize(ctx, attr)
	if err != nil {
		return false, err
	}
	return result.Decision == authorization.DecisionAllow, nil
}

func (a *apiHandler) listAuthorizedDomains(
	ctx context.Context,
	listRequest *types.ListDomainsRequest,
) (*types.ListDomainsResponse, error) {
	response, err := a.handler.ListDomains(ctx, listRequest)
	if err != nil || response == nil {
		return response, err
	}

	requestBody := authorization.NewFilteredRequestBody(listRequest)
	authorizedDomains := make([]*types.DescribeDomainResponse, 0, len(response.GetDomains()))
	for _, domain := range response.GetDomains() {
		// A denied domain is filtered out rather than failing the whole request: the
		// caller's credentials were already validated before the list was fetched.
		isAuthorized, err := a.authorizeFilter(ctx, &authorization.Attributes{
			APIName:     "ListDomains",
			Permission:  authorization.PermissionRead,
			RequestBody: requestBody,
			DomainName:  domain.GetDomainInfo().GetName(),
		})
		if err != nil {
			return nil, err
		}

		if isAuthorized {
			authorizedDomains = append(authorizedDomains, domain)
		}
	}

	response.Domains = authorizedDomains
	return response, nil
}

// getMetricsScopeWithDomain return metrics scope with domain tag
func (a *apiHandler) getMetricsScopeWithDomain(
	scope metrics.ScopeIdx,
	domain string,
) metrics.Scope {
	if domain != "" {
		return a.GetMetricsClient().Scope(scope).Tagged(metrics.DomainTag(domain))
	}
	return a.GetMetricsClient().Scope(scope).Tagged(metrics.DomainUnknownTag())
}
