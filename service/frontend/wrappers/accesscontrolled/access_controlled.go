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

	"github.com/uber/cadence/common/authorization"
	cadencectx "github.com/uber/cadence/common/context"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

var errUnauthorized = &types.AccessDeniedError{Message: "Request unauthorized."}

func (a *adminHandler) isAuthorized(ctx context.Context, attr *authorization.Attributes) (context.Context, bool, error) {
	result, err := a.authorizer.Authorize(ctx, attr)
	if err != nil {
		return ctx, false, err
	}
	isAuth := result.Decision == authorization.DecisionAllow
	if isAuth {
		ctx = enrichContextWithCaller(ctx, result)
	}
	return ctx, isAuth, nil
}

func (a *apiHandler) isAuthorized(
	ctx context.Context,
	attr *authorization.Attributes,
	scope metrics.Scope,
) (context.Context, bool, error) {
	sw := scope.StartTimer(metrics.CadenceAuthorizationLatency)
	defer sw.Stop()

	result, err := a.authorizer.Authorize(ctx, attr)
	if err != nil {
		scope.IncCounter(metrics.CadenceErrAuthorizeFailedCounter)
		return ctx, false, err
	}
	isAuth := result.Decision == authorization.DecisionAllow
	if !isAuth {
		scope.IncCounter(metrics.CadenceErrUnauthorizedCounter)
		return ctx, false, nil
	}
	ctx = enrichContextWithCaller(ctx, result)
	if result.CallerInfo == nil || result.CallerInfo.CallerType == cadencectx.CallerTypeUnknown {
		scope.IncCounter(metrics.CadenceRequestsWithoutCallerType)
		tags := []tag.Tag{
			tag.WorkflowDomainName(attr.DomainName),
			tag.Name(attr.APIName),
		}
		if result.CallerInfo != nil && result.CallerInfo.Subject != "" {
			tags = append(tags, tag.ActorID(result.CallerInfo.Subject))
		}
		a.GetLogger().Debug("request without caller type", tags...)
	}
	return ctx, true, nil
}

func enrichContextWithCaller(ctx context.Context, result authorization.Result) context.Context {
	if result.CallerInfo == nil {
		return ctx
	}
	return cadencectx.WithCallerInfo(ctx, result.CallerInfo)
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
