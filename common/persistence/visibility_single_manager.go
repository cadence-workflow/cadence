// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package persistence

import (
	"context"
	"encoding/json"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
)

type (
	visibilityManagerImpl struct {
		serializer  PayloadSerializer
		persistence VisibilityStore
		logger      log.Logger
		dc          *DynamicConfiguration
	}
)

var _ VisibilityManager = (*visibilityManagerImpl)(nil)

// NewVisibilityManagerImpl returns new VisibilityManager via a VisibilityStore
func NewVisibilityManagerImpl(persistence VisibilityStore, logger log.Logger, dc *DynamicConfiguration) VisibilityManager {
	return &visibilityManagerImpl{
		serializer:  NewPayloadSerializer(),
		persistence: persistence,
		logger:      logger,
		dc:          dc,
	}
}

func (v *visibilityManagerImpl) Close() {
	v.persistence.Close()
}

func (v *visibilityManagerImpl) GetName() string {
	return v.persistence.GetName()
}

func (v *visibilityManagerImpl) RecordWorkflowExecutionStarted(
	ctx context.Context,
	request *RecordWorkflowExecutionStartedRequest,
) error {
	req := &InternalRecordWorkflowExecutionStartedRequest{
		DomainUUID:             request.DomainUUID,
		WorkflowID:             request.Execution.GetWorkflowID(),
		RunID:                  request.Execution.GetRunID(),
		WorkflowTypeName:       request.WorkflowTypeName,
		StartTimestamp:         time.Unix(0, request.StartTimestamp),
		ExecutionTimestamp:     time.Unix(0, request.ExecutionTimestamp),
		WorkflowTimeout:        common.SecondsToDuration(request.WorkflowTimeout),
		TaskID:                 request.TaskID,
		TaskList:               request.TaskList,
		IsCron:                 request.IsCron,
		CronSchedule:           request.CronSchedule,
		NumClusters:            request.NumClusters,
		ClusterAttributeScope:  request.ClusterAttributeScope,
		ClusterAttributeName:   request.ClusterAttributeName,
		Memo:                   v.serializeMemo(request.Memo, request.DomainUUID, request.Execution.GetWorkflowID(), request.Execution.GetRunID()),
		UpdateTimestamp:        time.Unix(0, request.UpdateTimestamp),
		SearchAttributes:       request.SearchAttributes,
		ShardID:                request.ShardID,
		ExecutionStatus:        request.ExecutionStatus,
		ScheduledExecutionTime: time.Unix(0, request.ScheduledExecutionTimestamp),
	}
	return v.persistence.RecordWorkflowExecutionStarted(ctx, req)
}

func (v *visibilityManagerImpl) RecordWorkflowExecutionClosed(
	ctx context.Context,
	request *RecordWorkflowExecutionClosedRequest,
) error {
	req := &InternalRecordWorkflowExecutionClosedRequest{
		DomainUUID:             request.DomainUUID,
		WorkflowID:             request.Execution.GetWorkflowID(),
		RunID:                  request.Execution.GetRunID(),
		WorkflowTypeName:       request.WorkflowTypeName,
		StartTimestamp:         time.Unix(0, request.StartTimestamp),
		ExecutionTimestamp:     time.Unix(0, request.ExecutionTimestamp),
		TaskID:                 request.TaskID,
		Memo:                   v.serializeMemo(request.Memo, request.DomainUUID, request.Execution.GetWorkflowID(), request.Execution.GetRunID()),
		TaskList:               request.TaskList,
		ClusterAttributeScope:  request.ClusterAttributeScope,
		ClusterAttributeName:   request.ClusterAttributeName,
		SearchAttributes:       request.SearchAttributes,
		CloseTimestamp:         time.Unix(0, request.CloseTimestamp),
		Status:                 request.Status,
		HistoryLength:          request.HistoryLength,
		RetentionPeriod:        common.SecondsToDuration(request.RetentionSeconds),
		IsCron:                 request.IsCron,
		CronSchedule:           request.CronSchedule,
		NumClusters:            request.NumClusters,
		UpdateTimestamp:        time.Unix(0, request.UpdateTimestamp),
		ShardID:                request.ShardID,
		ExecutionStatus:        request.ExecutionStatus,
		ScheduledExecutionTime: time.Unix(0, request.ScheduledExecutionTimestamp),
	}
	return v.persistence.RecordWorkflowExecutionClosed(ctx, req)
}

func (v *visibilityManagerImpl) RecordWorkflowExecutionUninitialized(
	ctx context.Context,
	request *RecordWorkflowExecutionUninitializedRequest,
) error {
	req := &InternalRecordWorkflowExecutionUninitializedRequest{
		DomainUUID:       request.DomainUUID,
		WorkflowID:       request.Execution.GetWorkflowID(),
		RunID:            request.Execution.GetRunID(),
		WorkflowTypeName: request.WorkflowTypeName,
		UpdateTimestamp:  time.Unix(0, request.UpdateTimestamp),
		ShardID:          request.ShardID,
	}
	return v.persistence.RecordWorkflowExecutionUninitialized(ctx, req)
}

func (v *visibilityManagerImpl) UpsertWorkflowExecution(
	ctx context.Context,
	request *UpsertWorkflowExecutionRequest,
) error {
	req := &InternalUpsertWorkflowExecutionRequest{
		DomainUUID:                  request.DomainUUID,
		WorkflowID:                  request.Execution.GetWorkflowID(),
		RunID:                       request.Execution.GetRunID(),
		WorkflowTypeName:            request.WorkflowTypeName,
		StartTimestamp:              time.Unix(0, request.StartTimestamp),
		ExecutionTimestamp:          time.Unix(0, request.ExecutionTimestamp),
		TaskID:                      request.TaskID,
		Memo:                        v.serializeMemo(request.Memo, request.DomainUUID, request.Execution.GetWorkflowID(), request.Execution.GetRunID()),
		TaskList:                    request.TaskList,
		IsCron:                      request.IsCron,
		CronSchedule:                request.CronSchedule,
		NumClusters:                 request.NumClusters,
		ClusterAttributeScope:       request.ClusterAttributeScope,
		ClusterAttributeName:        request.ClusterAttributeName,
		UpdateTimestamp:             time.Unix(0, request.UpdateTimestamp),
		SearchAttributes:            request.SearchAttributes,
		ShardID:                     request.ShardID,
		ExecutionStatus:             request.ExecutionStatus,
		ScheduledExecutionTimestamp: request.ScheduledExecutionTimestamp,
	}
	return v.persistence.UpsertWorkflowExecution(ctx, req)
}

func (v *visibilityManagerImpl) ListOpenWorkflowExecutions(
	ctx context.Context,
	request *ListWorkflowExecutionsRequest,
) (*ListWorkflowExecutionsResponse, error) {
	internalResp, err := v.persistence.ListOpenWorkflowExecutions(ctx, v.toInternalListWorkflowExecutionsRequest(request))
	if err != nil {
		return nil, err
	}
	return v.convertInternalListResponse(internalResp), nil
}

func (v *visibilityManagerImpl) ListClosedWorkflowExecutions(
	ctx context.Context,
	request *ListWorkflowExecutionsRequest,
) (*ListWorkflowExecutionsResponse, error) {
	internalResp, err := v.persistence.ListClosedWorkflowExecutions(ctx, v.toInternalListWorkflowExecutionsRequest(request))
	if err != nil {
		return nil, err
	}
	return v.convertInternalListResponse(internalResp), nil
}

func (v *visibilityManagerImpl) ListOpenWorkflowExecutionsByType(
	ctx context.Context,
	request *ListWorkflowExecutionsByTypeRequest,
) (*ListWorkflowExecutionsResponse, error) {
	internalListRequest := v.toInternalListWorkflowExecutionsRequest(&request.ListWorkflowExecutionsRequest)
	internalRequest := &InternalListWorkflowExecutionsByTypeRequest{
		WorkflowTypeName: request.WorkflowTypeName,
	}
	if internalListRequest != nil {
		internalRequest.InternalListWorkflowExecutionsRequest = *internalListRequest
	}
	internalResp, err := v.persistence.ListOpenWorkflowExecutionsByType(ctx, internalRequest)
	if err != nil {
		return nil, err
	}
	return v.convertInternalListResponse(internalResp), nil
}

func (v *visibilityManagerImpl) ListClosedWorkflowExecutionsByType(
	ctx context.Context,
	request *ListWorkflowExecutionsByTypeRequest,
) (*ListWorkflowExecutionsResponse, error) {
	internalListRequest := v.toInternalListWorkflowExecutionsRequest(&request.ListWorkflowExecutionsRequest)
	internalRequest := &InternalListWorkflowExecutionsByTypeRequest{
		WorkflowTypeName: request.WorkflowTypeName,
	}
	if internalListRequest != nil {
		internalRequest.InternalListWorkflowExecutionsRequest = *internalListRequest
	}
	internalResp, err := v.persistence.ListClosedWorkflowExecutionsByType(ctx, internalRequest)
	if err != nil {
		return nil, err
	}
	return v.convertInternalListResponse(internalResp), nil
}

func (v *visibilityManagerImpl) ListOpenWorkflowExecutionsByWorkflowID(
	ctx context.Context,
	request *ListWorkflowExecutionsByWorkflowIDRequest,
) (*ListWorkflowExecutionsResponse, error) {
	internalListRequest := v.toInternalListWorkflowExecutionsRequest(&request.ListWorkflowExecutionsRequest)
	internalRequest := &InternalListWorkflowExecutionsByWorkflowIDRequest{
		WorkflowID: request.WorkflowID,
	}
	if internalListRequest != nil {
		internalRequest.InternalListWorkflowExecutionsRequest = *internalListRequest
	}
	internalResp, err := v.persistence.ListOpenWorkflowExecutionsByWorkflowID(ctx, internalRequest)
	if err != nil {
		return nil, err
	}
	return v.convertInternalListResponse(internalResp), nil
}

func (v *visibilityManagerImpl) ListClosedWorkflowExecutionsByWorkflowID(
	ctx context.Context,
	request *ListWorkflowExecutionsByWorkflowIDRequest,
) (*ListWorkflowExecutionsResponse, error) {
	internalListRequest := v.toInternalListWorkflowExecutionsRequest(&request.ListWorkflowExecutionsRequest)
	internalRequest := &InternalListWorkflowExecutionsByWorkflowIDRequest{
		WorkflowID: request.WorkflowID,
	}
	if internalListRequest != nil {
		internalRequest.InternalListWorkflowExecutionsRequest = *internalListRequest
	}
	internalResp, err := v.persistence.ListClosedWorkflowExecutionsByWorkflowID(ctx, internalRequest)
	if err != nil {
		return nil, err
	}
	return v.convertInternalListResponse(internalResp), nil
}

func (v *visibilityManagerImpl) ListClosedWorkflowExecutionsByStatus(
	ctx context.Context,
	request *ListClosedWorkflowExecutionsByStatusRequest,
) (*ListWorkflowExecutionsResponse, error) {
	internalListRequest := v.toInternalListWorkflowExecutionsRequest(&request.ListWorkflowExecutionsRequest)
	internalRequest := &InternalListClosedWorkflowExecutionsByStatusRequest{
		Status: request.Status,
	}
	if internalListRequest != nil {
		internalRequest.InternalListWorkflowExecutionsRequest = *internalListRequest
	}
	internalResp, err := v.persistence.ListClosedWorkflowExecutionsByStatus(ctx, internalRequest)
	if err != nil {
		return nil, err
	}
	return v.convertInternalListResponse(internalResp), nil
}

func (v *visibilityManagerImpl) GetClosedWorkflowExecution(
	ctx context.Context,
	request *GetClosedWorkflowExecutionRequest,
) (*GetClosedWorkflowExecutionResponse, error) {
	internalReq := &InternalGetClosedWorkflowExecutionRequest{
		DomainUUID: request.DomainUUID,
		Domain:     request.Domain,
		Execution:  request.Execution,
	}
	internalResp, err := v.persistence.GetClosedWorkflowExecution(ctx, internalReq)
	if err != nil {
		return nil, err
	}
	return v.convertInternalGetResponse(internalResp), nil
}

func (v *visibilityManagerImpl) DeleteWorkflowExecution(
	ctx context.Context,
	request *VisibilityDeleteWorkflowExecutionRequest,
) error {
	return v.persistence.DeleteWorkflowExecution(ctx, request)
}

func (v *visibilityManagerImpl) DeleteUninitializedWorkflowExecution(
	ctx context.Context,
	request *VisibilityDeleteWorkflowExecutionRequest,
) error {
	return v.persistence.DeleteUninitializedWorkflowExecution(ctx, request)
}

func (v *visibilityManagerImpl) ListWorkflowExecutions(
	ctx context.Context,
	request *ListWorkflowExecutionsByQueryRequest,
) (*ListWorkflowExecutionsResponse, error) {
	internalResp, err := v.persistence.ListWorkflowExecutions(ctx, request)
	if err != nil {
		return nil, err
	}
	return v.convertInternalListResponse(internalResp), nil
}

func (v *visibilityManagerImpl) ScanWorkflowExecutions(
	ctx context.Context,
	request *ListWorkflowExecutionsByQueryRequest,
) (*ListWorkflowExecutionsResponse, error) {
	internalResp, err := v.persistence.ScanWorkflowExecutions(ctx, request)
	if err != nil {
		return nil, err
	}
	return v.convertInternalListResponse(internalResp), nil
}

func (v *visibilityManagerImpl) CountWorkflowExecutions(
	ctx context.Context,
	request *CountWorkflowExecutionsRequest,
) (*CountWorkflowExecutionsResponse, error) {
	return v.persistence.CountWorkflowExecutions(ctx, request)
}

func (v *visibilityManagerImpl) convertInternalGetResponse(internalResp *InternalGetClosedWorkflowExecutionResponse) *GetClosedWorkflowExecutionResponse {
	if internalResp == nil {
		return nil
	}

	resp := &GetClosedWorkflowExecutionResponse{}
	resp.Execution = v.convertVisibilityWorkflowExecutionInfo(internalResp.Execution)
	return resp
}

func (v *visibilityManagerImpl) convertInternalListResponse(internalResp *InternalListWorkflowExecutionsResponse) *ListWorkflowExecutionsResponse {
	if internalResp == nil {
		return nil
	}

	resp := &ListWorkflowExecutionsResponse{}
	resp.Executions = make([]*types.WorkflowExecutionInfo, len(internalResp.Executions))
	for i, execution := range internalResp.Executions {
		resp.Executions[i] = v.convertVisibilityWorkflowExecutionInfo(execution)
	}

	resp.NextPageToken = internalResp.NextPageToken
	return resp
}

func (v *visibilityManagerImpl) getSearchAttributes(attr map[string]interface{}) (*types.SearchAttributes, error) {
	indexedFields := make(map[string][]byte)
	var err error
	var valBytes []byte
	for k, val := range attr {
		valBytes, err = json.Marshal(val)
		if err != nil {
			v.logger.Error("error when encode search attributes", tag.Value(val))
			continue
		}
		indexedFields[k] = valBytes
	}
	if err != nil {
		return nil, err
	}
	return &types.SearchAttributes{
		IndexedFields: indexedFields,
	}, nil
}

func (v *visibilityManagerImpl) convertVisibilityWorkflowExecutionInfo(execution *InternalVisibilityWorkflowExecutionInfo) *types.WorkflowExecutionInfo {
	// special handling of ExecutionTime for cron or retry
	if execution.ExecutionTime.UnixNano() == 0 {
		execution.ExecutionTime = execution.StartTime
	}

	memo, err := v.serializer.DeserializeVisibilityMemo(execution.Memo)
	if err != nil {
		v.logger.Error("failed to deserialize memo",
			tag.WorkflowID(execution.WorkflowID),
			tag.WorkflowRunID(execution.RunID),
			tag.Error(err))
	}
	searchAttributes, err := v.getSearchAttributes(execution.SearchAttributes)
	if err != nil {
		v.logger.Error("failed to convert search attributes",
			tag.WorkflowID(execution.WorkflowID),
			tag.WorkflowRunID(execution.RunID),
			tag.Error(err))
	}

	convertedExecution := &types.WorkflowExecutionInfo{
		Execution: &types.WorkflowExecution{
			WorkflowID: execution.WorkflowID,
			RunID:      execution.RunID,
		},
		Type: &types.WorkflowType{
			Name: execution.TypeName,
		},
		StartTime:              common.Int64Ptr(execution.StartTime.UnixNano()),
		ExecutionTime:          common.Int64Ptr(execution.ExecutionTime.UnixNano()),
		Memo:                   memo,
		SearchAttributes:       searchAttributes,
		TaskList:               &types.TaskList{Name: execution.TaskList},
		IsCron:                 execution.IsCron,
		CronSchedule:           common.StringPtr(execution.CronSchedule),
		ExecutionStatus:        &execution.ExecutionStatus,
		ScheduledExecutionTime: common.Int64Ptr(execution.ScheduledExecutionTime.UnixNano()),
	}

	// for close records
	if execution.Status != nil {
		convertedExecution.CloseTime = common.Int64Ptr(execution.CloseTime.UnixNano())
		convertedExecution.CloseStatus = execution.Status
		convertedExecution.HistoryLength = execution.HistoryLength
	}

	return convertedExecution
}

func (v *visibilityManagerImpl) toInternalListWorkflowExecutionsRequest(req *ListWorkflowExecutionsRequest) *InternalListWorkflowExecutionsRequest {
	if req == nil {
		return nil
	}
	return &InternalListWorkflowExecutionsRequest{
		DomainUUID:    req.DomainUUID,
		Domain:        req.Domain,
		EarliestTime:  time.Unix(0, req.EarliestTime),
		LatestTime:    time.Unix(0, req.LatestTime),
		PageSize:      req.PageSize,
		NextPageToken: req.NextPageToken,
	}
}

func (v *visibilityManagerImpl) serializeMemo(visibilityMemo *types.Memo, domainID, wID, rID string) *DataBlob {
	memo, err := v.serializer.SerializeVisibilityMemo(visibilityMemo, constants.EncodingType(v.dc.SerializationEncoding()))
	if err != nil {
		v.logger.WithTags(
			tag.WorkflowDomainID(domainID),
			tag.WorkflowID(wID),
			tag.WorkflowRunID(rID),
			tag.Error(err)).
			Error("Unable to encode visibility memo")
	}
	if memo == nil {
		return &DataBlob{}
	}
	return memo
}
