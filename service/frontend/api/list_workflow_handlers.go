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

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/frontend/validate"
)

// CountWorkflowExecutions - count number of workflow executions in a domain
func (wh *WorkflowHandler) CountWorkflowExecutions(
	ctx context.Context,
	countRequest *types.CountWorkflowExecutionsRequest,
) (resp *types.CountWorkflowExecutionsResponse, retError error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if err := wh.requestValidator.ValidateCountWorkflowExecutionsRequest(ctx, countRequest); err != nil {
		return nil, err
	}
	validatedQuery, err := wh.visibilityQueryValidator.ValidateQuery(countRequest.GetQuery())
	if err != nil {
		return nil, err
	}

	domain := countRequest.GetDomain()
	domainID, err := wh.GetDomainCache().GetDomainID(domain)
	if err != nil {
		return nil, err
	}

	req := &persistence.CountWorkflowExecutionsRequest{
		DomainUUID: domainID,
		Domain:     domain,
		Query:      validatedQuery,
	}
	persistenceResp, err := wh.GetVisibilityManager().CountWorkflowExecutions(ctx, req)
	if err != nil {
		return nil, err
	}

	resp = &types.CountWorkflowExecutionsResponse{
		Count: persistenceResp.Count,
	}
	return resp, nil
}

// ScanWorkflowExecutions - retrieves info for large amount of workflow executions in a domain without order
func (wh *WorkflowHandler) ScanWorkflowExecutions(
	ctx context.Context,
	listRequest *types.ListWorkflowExecutionsRequest,
) (resp *types.ListWorkflowExecutionsResponse, retError error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if err := wh.requestValidator.ValidateListWorkflowExecutionsRequest(ctx, listRequest); err != nil {
		return nil, err
	}
	validatedQuery, err := wh.visibilityQueryValidator.ValidateQuery(listRequest.GetQuery())
	if err != nil {
		return nil, err
	}

	domain := listRequest.GetDomain()
	domainID, err := wh.GetDomainCache().GetDomainID(domain)
	if err != nil {
		return nil, err
	}

	req := &persistence.ListWorkflowExecutionsByQueryRequest{
		DomainUUID:    domainID,
		Domain:        domain,
		PageSize:      int(listRequest.GetPageSize()),
		NextPageToken: listRequest.NextPageToken,
		Query:         validatedQuery,
	}
	persistenceResp, err := wh.GetVisibilityManager().ScanWorkflowExecutions(ctx, req)
	if err != nil {
		return nil, err
	}

	resp = &types.ListWorkflowExecutionsResponse{}
	resp.Executions = persistenceResp.Executions
	resp.NextPageToken = persistenceResp.NextPageToken
	return resp, nil
}

// ListOpenWorkflowExecutions - retrieves info for open workflow executions in a domain
func (wh *WorkflowHandler) ListOpenWorkflowExecutions(
	ctx context.Context,
	listRequest *types.ListOpenWorkflowExecutionsRequest,
) (resp *types.ListOpenWorkflowExecutionsResponse, retError error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if err := wh.requestValidator.ValidateListOpenWorkflowExecutionsRequest(ctx, listRequest); err != nil {
		return nil, err
	}
	domain := listRequest.GetDomain()
	domainID, err := wh.GetDomainCache().GetDomainID(domain)
	if err != nil {
		return nil, err
	}

	baseReq := persistence.ListWorkflowExecutionsRequest{
		DomainUUID:    domainID,
		Domain:        domain,
		PageSize:      int(listRequest.GetMaximumPageSize()),
		NextPageToken: listRequest.NextPageToken,
		EarliestTime:  listRequest.StartTimeFilter.GetEarliestTime(),
		LatestTime:    listRequest.StartTimeFilter.GetLatestTime(),
	}

	var persistenceResp *persistence.ListWorkflowExecutionsResponse
	if listRequest.ExecutionFilter != nil {
		if wh.config.DisableListVisibilityByFilter(domain) {
			err = validate.ErrNoPermission
		} else {
			persistenceResp, err = wh.GetVisibilityManager().ListOpenWorkflowExecutionsByWorkflowID(
				ctx,
				&persistence.ListWorkflowExecutionsByWorkflowIDRequest{
					ListWorkflowExecutionsRequest: baseReq,
					WorkflowID:                    listRequest.ExecutionFilter.GetWorkflowID(),
				})
		}
		wh.GetLogger().Debug("List open workflow with filter",
			tag.WorkflowDomainName(listRequest.GetDomain()), tag.WorkflowListWorkflowFilterByID)
	} else if listRequest.TypeFilter != nil {
		if wh.config.DisableListVisibilityByFilter(domain) {
			err = validate.ErrNoPermission
		} else {
			persistenceResp, err = wh.GetVisibilityManager().ListOpenWorkflowExecutionsByType(
				ctx,
				&persistence.ListWorkflowExecutionsByTypeRequest{
					ListWorkflowExecutionsRequest: baseReq,
					WorkflowTypeName:              listRequest.TypeFilter.GetName(),
				},
			)
		}
		wh.GetLogger().Debug("List open workflow with filter",
			tag.WorkflowDomainName(listRequest.GetDomain()), tag.WorkflowListWorkflowFilterByType)
	} else {
		persistenceResp, err = wh.GetVisibilityManager().ListOpenWorkflowExecutions(ctx, &baseReq)
	}

	if err != nil {
		return nil, err
	}

	resp = &types.ListOpenWorkflowExecutionsResponse{}
	resp.Executions = persistenceResp.Executions
	resp.NextPageToken = persistenceResp.NextPageToken
	return resp, nil
}

// ListArchivedWorkflowExecutions - retrieves archived info for closed workflow executions in a domain
func (wh *WorkflowHandler) ListArchivedWorkflowExecutions(
	ctx context.Context,
	listRequest *types.ListArchivedWorkflowExecutionsRequest,
) (resp *types.ListArchivedWorkflowExecutionsResponse, retError error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if err := wh.requestValidator.ValidateListArchivedWorkflowExecutionsRequest(ctx, listRequest); err != nil {
		return nil, err
	}
	if !wh.GetArchivalMetadata().GetVisibilityConfig().ClusterConfiguredForArchival() {
		return nil, &types.BadRequestError{Message: "Cluster is not configured for visibility archival"}
	}

	if !wh.GetArchivalMetadata().GetVisibilityConfig().ReadEnabled() {
		return nil, &types.BadRequestError{Message: "Cluster is not configured for reading archived visibility records"}
	}

	entry, err := wh.GetDomainCache().GetDomain(listRequest.GetDomain())
	if err != nil {
		return nil, err
	}

	if entry.GetConfig().VisibilityArchivalStatus != types.ArchivalStatusEnabled {
		return nil, &types.BadRequestError{Message: "Domain is not configured for visibility archival"}
	}

	URI, err := archiver.NewURI(entry.GetConfig().VisibilityArchivalURI)
	if err != nil {
		return nil, err
	}

	visibilityArchiver, err := wh.GetArchiverProvider().GetVisibilityArchiver(URI.Scheme(), service.Frontend)
	if err != nil {
		return nil, err
	}

	archiverRequest := &archiver.QueryVisibilityRequest{
		DomainID:      entry.GetInfo().ID,
		PageSize:      int(listRequest.GetPageSize()),
		NextPageToken: listRequest.NextPageToken,
		Query:         listRequest.GetQuery(),
	}

	archiverResponse, err := visibilityArchiver.Query(ctx, URI, archiverRequest)
	if err != nil {
		return nil, err
	}

	// special handling of ExecutionTime for cron or retry
	for _, execution := range archiverResponse.Executions {
		if execution.GetExecutionTime() == 0 {
			execution.ExecutionTime = common.Int64Ptr(execution.GetStartTime())
		}
	}

	return &types.ListArchivedWorkflowExecutionsResponse{
		Executions:    archiverResponse.Executions,
		NextPageToken: archiverResponse.NextPageToken,
	}, nil
}

// ListClosedWorkflowExecutions - retrieves info for closed workflow executions in a domain
func (wh *WorkflowHandler) ListClosedWorkflowExecutions(
	ctx context.Context,
	listRequest *types.ListClosedWorkflowExecutionsRequest,
) (resp *types.ListClosedWorkflowExecutionsResponse, retError error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if err := wh.requestValidator.ValidateListClosedWorkflowExecutionsRequest(ctx, listRequest); err != nil {
		return nil, err
	}
	domain := listRequest.GetDomain()
	domainID, err := wh.GetDomainCache().GetDomainID(domain)
	if err != nil {
		return nil, err
	}

	baseReq := persistence.ListWorkflowExecutionsRequest{
		DomainUUID:    domainID,
		Domain:        domain,
		PageSize:      int(listRequest.GetMaximumPageSize()),
		NextPageToken: listRequest.NextPageToken,
		EarliestTime:  listRequest.StartTimeFilter.GetEarliestTime(),
		LatestTime:    listRequest.StartTimeFilter.GetLatestTime(),
	}

	var persistenceResp *persistence.ListWorkflowExecutionsResponse
	if listRequest.ExecutionFilter != nil {
		if wh.config.DisableListVisibilityByFilter(domain) {
			err = validate.ErrNoPermission
		} else {
			persistenceResp, err = wh.GetVisibilityManager().ListClosedWorkflowExecutionsByWorkflowID(
				ctx,
				&persistence.ListWorkflowExecutionsByWorkflowIDRequest{
					ListWorkflowExecutionsRequest: baseReq,
					WorkflowID:                    listRequest.ExecutionFilter.GetWorkflowID(),
				},
			)
		}
		wh.GetLogger().Debug("List closed workflow with filter",
			tag.WorkflowDomainName(listRequest.GetDomain()), tag.WorkflowListWorkflowFilterByID)
	} else if listRequest.TypeFilter != nil {
		if wh.config.DisableListVisibilityByFilter(domain) {
			err = validate.ErrNoPermission
		} else {
			persistenceResp, err = wh.GetVisibilityManager().ListClosedWorkflowExecutionsByType(
				ctx,
				&persistence.ListWorkflowExecutionsByTypeRequest{
					ListWorkflowExecutionsRequest: baseReq,
					WorkflowTypeName:              listRequest.TypeFilter.GetName(),
				},
			)
		}
		wh.GetLogger().Debug("List closed workflow with filter",
			tag.WorkflowDomainName(listRequest.GetDomain()), tag.WorkflowListWorkflowFilterByType)
	} else if listRequest.StatusFilter != nil {
		if wh.config.DisableListVisibilityByFilter(domain) {
			err = validate.ErrNoPermission
		} else {
			persistenceResp, err = wh.GetVisibilityManager().ListClosedWorkflowExecutionsByStatus(
				ctx,
				&persistence.ListClosedWorkflowExecutionsByStatusRequest{
					ListWorkflowExecutionsRequest: baseReq,
					Status:                        listRequest.GetStatusFilter(),
				},
			)
		}
		wh.GetLogger().Debug("List closed workflow with filter",
			tag.WorkflowDomainName(listRequest.GetDomain()), tag.WorkflowListWorkflowFilterByStatus)
	} else {
		persistenceResp, err = wh.GetVisibilityManager().ListClosedWorkflowExecutions(ctx, &baseReq)
	}

	if err != nil {
		return nil, err
	}

	resp = &types.ListClosedWorkflowExecutionsResponse{}
	resp.Executions = persistenceResp.Executions
	resp.NextPageToken = persistenceResp.NextPageToken
	return resp, nil
}

// ListWorkflowExecutions - retrieves info for workflow executions in a domain
func (wh *WorkflowHandler) ListWorkflowExecutions(
	ctx context.Context,
	listRequest *types.ListWorkflowExecutionsRequest,
) (resp *types.ListWorkflowExecutionsResponse, retError error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if err := wh.requestValidator.ValidateListWorkflowExecutionsRequest(ctx, listRequest); err != nil {
		return nil, err
	}
	validatedQuery, err := wh.visibilityQueryValidator.ValidateQuery(listRequest.GetQuery())
	if err != nil {
		return nil, err
	}

	domain := listRequest.GetDomain()
	domainID, err := wh.GetDomainCache().GetDomainID(domain)
	if err != nil {
		return nil, err
	}

	req := &persistence.ListWorkflowExecutionsByQueryRequest{
		DomainUUID:    domainID,
		Domain:        domain,
		PageSize:      int(listRequest.GetPageSize()),
		NextPageToken: listRequest.NextPageToken,
		Query:         validatedQuery,
	}
	persistenceResp, err := wh.GetVisibilityManager().ListWorkflowExecutions(ctx, req)
	if err != nil {
		return nil, err
	}

	resp = &types.ListWorkflowExecutionsResponse{}
	resp.Executions = persistenceResp.Executions
	resp.NextPageToken = persistenceResp.NextPageToken
	return resp, nil
}
