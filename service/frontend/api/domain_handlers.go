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

package api

import (
	"context"
	"fmt"

	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/frontend/validate"
)

// RegisterDomain creates a new domain which can be used as a container for all resources.  Domain is a top level
// entity within Cadence, used as a container for all resources like workflow executions, tasklists, etc.  Domain
// acts as a sandbox and provides isolation for all resources within the domain.  All resources belongs to exactly one
// domain.
func (wh *WorkflowHandler) RegisterDomain(ctx context.Context, registerRequest *types.RegisterDomainRequest) (retError error) {
	if wh.isShuttingDown() {
		return validate.ErrShuttingDown
	}
	if err := wh.requestValidator.ValidateRegisterDomainRequest(ctx, registerRequest); err != nil {
		return err
	}
	return wh.domainHandler.RegisterDomain(ctx, registerRequest)
}

// ListDomains returns the information and configuration for a registered domain.
func (wh *WorkflowHandler) ListDomains(
	ctx context.Context,
	listRequest *types.ListDomainsRequest,
) (response *types.ListDomainsResponse, retError error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}

	if listRequest == nil {
		return nil, validate.ErrRequestNotSet
	}

	return wh.domainHandler.ListDomains(ctx, listRequest)
}

// DescribeDomain returns the information and configuration for a registered domain.
func (wh *WorkflowHandler) DescribeDomain(
	ctx context.Context,
	describeRequest *types.DescribeDomainRequest,
) (response *types.DescribeDomainResponse, retError error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if err := wh.requestValidator.ValidateDescribeDomainRequest(ctx, describeRequest); err != nil {
		return nil, err
	}
	resp, err := wh.domainHandler.DescribeDomain(ctx, describeRequest)
	if err != nil {
		return nil, err
	}

	if resp.GetFailoverInfo() != nil && resp.GetFailoverInfo().GetFailoverExpireTimestamp() > 0 {
		// fetch ongoing failover info from history service
		failoverResp, err := wh.GetHistoryClient().GetFailoverInfo(ctx, &types.GetFailoverInfoRequest{
			DomainID: resp.GetDomainInfo().UUID,
		})
		if err != nil {
			// despite the error from history, return describe domain response
			wh.GetLogger().Error(
				fmt.Sprintf("Failed to get failover info for domain %s", resp.DomainInfo.GetName()),
				tag.Error(err),
			)
			return resp, nil
		}
		resp.FailoverInfo.CompletedShardCount = failoverResp.GetCompletedShardCount()
		resp.FailoverInfo.PendingShards = failoverResp.GetPendingShards()
	}
	return resp, nil
}

// UpdateDomain is used to update the information and configuration for a registered domain.
func (wh *WorkflowHandler) UpdateDomain(
	ctx context.Context,
	updateRequest *types.UpdateDomainRequest,
) (resp *types.UpdateDomainResponse, retError error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if err := wh.requestValidator.ValidateUpdateDomainRequest(ctx, updateRequest); err != nil {
		return nil, err
	}

	domainName := updateRequest.GetName()
	logger := wh.GetLogger().WithTags(
		tag.WorkflowDomainName(domainName),
		tag.OperationName("DomainUpdate"))

	isFailover := isFailoverRequest(updateRequest)
	isGraceFailover := isGraceFailoverRequest(updateRequest)
	logger.Info(fmt.Sprintf(
		"Domain Update requested. isFailover: %v, isGraceFailover: %v, Request: %#v.",
		isFailover,
		isGraceFailover,
		updateRequest))

	if isGraceFailover {
		if err := wh.checkOngoingFailover(
			ctx,
			&updateRequest.Name,
		); err != nil {
			logger.Error("Graceful domain failover request failed. Not able to check ongoing failovers.",
				tag.Error(err))
			return nil, err
		}
	}

	// TODO: call remote clusters to verify domain data
	resp, err := wh.domainHandler.UpdateDomain(ctx, updateRequest)
	if err != nil {
		logger.Error("Domain update operation failed.",
			tag.Error(err))
		return nil, err
	}
	logger.Info("Domain update operation succeeded.")
	return resp, nil
}

// DeprecateDomain us used to update status of a registered domain to DEPRECATED. Once the domain is deprecated
// it cannot be used to start new workflow executions.  Existing workflow executions will continue to run on
// deprecated domains.
func (wh *WorkflowHandler) DeprecateDomain(ctx context.Context, deprecateRequest *types.DeprecateDomainRequest) (retError error) {
	if wh.isShuttingDown() {
		return validate.ErrShuttingDown
	}
	if err := wh.requestValidator.ValidateDeprecateDomainRequest(ctx, deprecateRequest); err != nil {
		return err
	}
	return wh.domainHandler.DeprecateDomain(ctx, deprecateRequest)
}
