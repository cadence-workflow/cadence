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
	"time"

	"github.com/uber/cadence/common/domain/audit"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
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

// DeleteDomain permanently removes a domain record. This operation:
// - Requires domain to be in DEPRECATED status
// - Cannot be performed on domains with running workflows
// - Is irreversible and removes all domain data
func (wh *WorkflowHandler) DeleteDomain(ctx context.Context, deleteRequest *types.DeleteDomainRequest) (retError error) {
	if wh.isShuttingDown() {
		return validate.ErrShuttingDown
	}
	if err := wh.requestValidator.ValidateDeleteDomainRequest(ctx, deleteRequest); err != nil {
		return err
	}

	domainName := deleteRequest.GetName()
	resp, err := wh.domainHandler.DescribeDomain(ctx, &types.DescribeDomainRequest{Name: &domainName})
	if err != nil {
		return err
	}

	if *resp.DomainInfo.Status != types.DomainStatusDeprecated {
		return &types.BadRequestError{Message: "Domain is not in a deprecated state."}
	}

	workflowList, err := wh.ListWorkflowExecutions(ctx, &types.ListWorkflowExecutionsRequest{
		Domain: domainName,
	})
	if err != nil {
		return err
	}

	if len(workflowList.Executions) != 0 {
		return &types.BadRequestError{Message: "Domain still have workflow execution history."}
	}

	return wh.domainHandler.DeleteDomain(ctx, deleteRequest)
}

// DeprecateDomain is used to update status of a registered domain to DEleTED. Once the domain is deleted
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

// FailoverDomain is used to failover a registered domain to different cluster.
func (wh *WorkflowHandler) FailoverDomain(ctx context.Context, failoverRequest *types.FailoverDomainRequest) (*types.FailoverDomainResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if err := wh.requestValidator.ValidateFailoverDomainRequest(ctx, failoverRequest); err != nil {
		return nil, err
	}

	domainName := failoverRequest.GetDomainName()
	logger := wh.GetLogger().WithTags(
		tag.WorkflowDomainName(domainName),
		tag.OperationName("FailoverDomain"))

	logger.Info(fmt.Sprintf("Failover domain is requested. Request: %#v.", failoverRequest))

	failoverResp, err := wh.domainHandler.FailoverDomain(ctx, failoverRequest)
	if err != nil {
		logger.Error("Failover domain operation failed.",
			tag.Error(err))
		return nil, err
	}

	logger.Info("Failover domain operation succeeded.")
	return failoverResp, nil
}

// ListFailoverHistory returns the failover history for a domain
func (wh *WorkflowHandler) ListFailoverHistory(
	ctx context.Context,
	request *types.ListFailoverHistoryRequest,
) (*types.ListFailoverHistoryResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}

	// Extract domain ID from filters
	if request.Filters == nil || request.Filters.DomainID == nil || *request.Filters.DomainID == "" {
		return nil, &types.BadRequestError{Message: "domain_id is required in filters"}
	}

	domainID := *request.Filters.DomainID

	// Set default page size
	pageSize := 5
	if request.Pagination != nil && request.Pagination.PageSize != nil && *request.Pagination.PageSize > 0 {
		pageSize = int(*request.Pagination.PageSize)
	}

	var nextPageToken []byte
	if request.Pagination != nil {
		nextPageToken = request.Pagination.NextPageToken
	}

	// Read from audit log
	readResp, err := wh.GetDomainManager().ReadDomainAuditLog(ctx, &persistence.ReadDomainAuditLogRequest{
		DomainID:      domainID,
		PageSize:      pageSize,
		NextPageToken: nextPageToken,
	})
	if err != nil {
		return nil, err
	}

	// Reconstruct failover events
	events, err := audit.ReconstructFailoverEvents(readResp.Entries)
	if err != nil {
		return nil, err
	}

	return &types.ListFailoverHistoryResponse{
		FailoverEvents: events,
		NextPageToken:  readResp.NextPageToken,
	}, nil
}

// GetFailoverEvent retrieves detailed information about a specific failover event
func (wh *WorkflowHandler) GetFailoverEvent(
	ctx context.Context,
	request *types.GetFailoverEventRequest,
) (*types.GetFailoverEventResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}

	// Validate request
	if request.DomainID == nil || *request.DomainID == "" ||
		request.FailoverEventID == nil || *request.FailoverEventID == "" ||
		request.CreatedTime == nil {
		return nil, &types.BadRequestError{
			Message: "domain_id, failover_event_id, and created_time are required",
		}
	}

	logger := wh.GetLogger()
	logger.Debug("GetFailoverEvent request received",
		tag.WorkflowDomainID(*request.DomainID),
		tag.Value(*request.FailoverEventID),
		tag.Value(*request.CreatedTime))

	// Convert Unix timestamp to time.Time
	createdTime := time.Unix(0, *request.CreatedTime*int64(time.Millisecond))
	logger.Debug("Converted timestamp for query",
		tag.Timestamp(createdTime),
		tag.Value(createdTime.UnixNano()))

	// Get specific audit log entry
	entryResp, err := wh.GetDomainManager().GetDomainAuditLogEntry(ctx, &persistence.GetDomainAuditLogEntryRequest{
		DomainID:    *request.DomainID,
		EventID:     *request.FailoverEventID,
		CreatedTime: createdTime,
	})
	if err != nil {
		logger.Error("Failed to get domain audit log entry",
			tag.Error(err),
			tag.WorkflowDomainID(*request.DomainID),
			tag.Value(*request.FailoverEventID))
		return nil, err
	}

	if entryResp == nil || entryResp.Entry == nil {
		logger.Warn("GetDomainAuditLogEntry returned nil entry",
			tag.WorkflowDomainID(*request.DomainID),
			tag.Value(*request.FailoverEventID))
		return &types.GetFailoverEventResponse{}, nil
	}

	logger.Debug("Retrieved audit log entry",
		tag.WorkflowDomainID(entryResp.Entry.DomainID),
		tag.Value(entryResp.Entry.EventID),
		tag.Timestamp(entryResp.Entry.CreatedTime),
		tag.Value(len(entryResp.Entry.StateAfter)))

	// Reconstruct cluster failovers from the entry
	return audit.GetFailoverEventDetails(entryResp.Entry)
}
