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

package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/asyncworkflow/queueconfigapi"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/client"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/codec"
	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/domain"
	dc "github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/elasticsearch"
	"github.com/uber/cadence/common/isolationgroup/isolationgroupapi"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/ndc"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/frontend/config"
	"github.com/uber/cadence/service/frontend/validate"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/lookup"
)

const (
	getDomainReplicationMessageBatchSize = 100
	defaultLastMessageID                 = int64(-1)
	endMessageID                         = int64(1<<63 - 1)
)

type (
	// adminHandlerImpl is an implementation for admin service independent of wire protocol
	adminHandlerImpl struct {
		resource.Resource

		numberOfHistoryShards int
		params                *resource.Params
		config                *config.Config
		domainDLQHandler      domain.DLQMessageHandler
		domainFailoverWatcher domain.FailoverWatcher
		eventSerializer       persistence.PayloadSerializer
		esClient              elasticsearch.GenericClient
		throttleRetry         *backoff.ThrottleRetry
		isolationGroups       isolationgroupapi.Handler
		asyncWFQueueConfigs   queueconfigapi.Handler
	}

	workflowQueryTemplate struct {
		name     string
		function func(request *types.AdminMaintainWorkflowRequest) error
	}

	getWorkflowRawHistoryV2Token struct {
		DomainName        string
		WorkflowID        string
		RunID             string
		StartEventID      int64
		StartEventVersion int64
		EndEventID        int64
		EndEventVersion   int64
		PersistenceToken  []byte
		VersionHistories  *types.VersionHistories
	}
)

var (
	adminServiceRetryPolicy = common.CreateAdminServiceRetryPolicy()

	corruptWorkflowErrorList = [3]string{
		execution.ErrMissingWorkflowStartEvent.Error(),
		execution.ErrMissingActivityScheduledEvent.Error(),
		persistence.ErrCorruptedHistory.Error(),
	}
)

// NewHandler creates a thrift service for the cadence admin service
func NewHandler(
	resource resource.Resource,
	params *resource.Params,
	config *config.Config,
	domainHandler domain.Handler,
) Handler {

	domainReplicationTaskExecutor := domain.NewReplicationTaskExecutor(
		resource.GetDomainManager(),
		resource.GetTimeSource(),
		resource.GetLogger(),
	)

	return &adminHandlerImpl{
		Resource:              resource,
		numberOfHistoryShards: params.PersistenceConfig.NumHistoryShards,
		params:                params,
		config:                config,
		domainDLQHandler: domain.NewDLQMessageHandler(
			domainReplicationTaskExecutor,
			resource.GetDomainReplicationQueue(),
			resource.GetLogger(),
			resource.GetMetricsClient(),
			clock.NewRealTimeSource(),
		),
		domainFailoverWatcher: domain.NewFailoverWatcher(
			resource.GetDomainCache(),
			resource.GetDomainManager(),
			resource.GetTimeSource(),
			config.DomainFailoverRefreshInterval,
			config.DomainFailoverRefreshTimerJitterCoefficient,
			resource.GetMetricsClient(),
			resource.GetLogger(),
		),
		eventSerializer: persistence.NewPayloadSerializer(),
		esClient:        params.ESClient,
		throttleRetry: backoff.NewThrottleRetry(
			backoff.WithRetryPolicy(adminServiceRetryPolicy),
			backoff.WithRetryableError(common.IsServiceTransientError),
		),
		isolationGroups:     isolationgroupapi.New(resource.GetLogger(), resource.GetIsolationGroupStore(), domainHandler),
		asyncWFQueueConfigs: queueconfigapi.New(resource.GetLogger(), domainHandler),
	}
}

// Start starts the handler
func (adh *adminHandlerImpl) Start() {
	adh.domainDLQHandler.Start()

	if adh.config.EnableGracefulFailover() {
		adh.domainFailoverWatcher.Start()
	}
}

// Stop stops the handler
func (adh *adminHandlerImpl) Stop() {
	adh.domainDLQHandler.Stop()
	adh.domainFailoverWatcher.Stop()
}

// AddSearchAttribute add search attribute to whitelist
func (adh *adminHandlerImpl) AddSearchAttribute(
	ctx context.Context,
	request *types.AddSearchAttributeRequest,
) (retError error) {

	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &retError) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.AdminAddSearchAttributeScope)
	defer sw.Stop()

	// validate request
	if request == nil {
		return adh.error(validate.ErrRequestNotSet, scope)
	}
	if err := validate.CheckPermission(adh.config, request.SecurityToken); err != nil {
		return adh.error(validate.ErrNoPermission, scope)
	}
	if len(request.GetSearchAttribute()) == 0 {
		return adh.error(&types.BadRequestError{Message: "SearchAttributes are not provided"}, scope)
	}

	searchAttr := request.GetSearchAttribute()
	currentValidAttr, err := adh.params.DynamicConfig.GetMapValue(dc.ValidSearchAttributes, nil)
	if err != nil {
		return adh.error(&types.InternalServiceError{Message: fmt.Sprintf("Failed to get dynamic config, err: %v", err)}, scope)
	}

	for keyName, valueType := range searchAttr {
		if definition.IsSystemIndexedKey(keyName) {
			return adh.error(&types.BadRequestError{Message: fmt.Sprintf("Key [%s] is reserved by system", keyName)}, scope)
		}
		if currValType, exist := currentValidAttr[keyName]; exist {
			if currValType != int(valueType) {
				return adh.error(&types.BadRequestError{Message: fmt.Sprintf("Key [%s] is already whitelisted as a different type", keyName)}, scope)
			}
			adh.GetLogger().Warn("Adding a search attribute that is already existing in dynamicconfig, it's probably a noop if ElasticSearch is already added. Here will re-do it on ElasticSearch.")
		}
		currentValidAttr[keyName] = int(valueType)
	}

	// update dynamic config. Until the DB based dynamic config is implemented, we shouldn't fail the updating.
	err = adh.params.DynamicConfig.UpdateValue(dc.ValidSearchAttributes, currentValidAttr)
	if err != nil {
		adh.GetLogger().Warn("Failed to update dynamicconfig. This is only useful in local dev environment for filebased config. Please ignore this warn if this is in a real Cluster, because your filebased dynamicconfig MUST be updated separately. Configstore dynamic config will also require separate updating via the CLI.")
	}

	// when have valid advance visibility config, update elasticsearch mapping, new added field will not be able to remove or update
	if err := adh.validateConfigForAdvanceVisibility(); err != nil {
		adh.GetLogger().Warn("Skip updating OpenSearch/ElasticSearch mapping since Advance Visibility hasn't been enabled.")
	} else {
		index := adh.params.ESConfig.GetVisibilityIndex()
		for k, v := range searchAttr {
			valueType := convertIndexedValueTypeToESDataType(v)
			if len(valueType) == 0 {
				return adh.error(&types.BadRequestError{Message: fmt.Sprintf("Unknown value type, %v", v)}, scope)
			}
			err := adh.params.ESClient.PutMapping(ctx, index, definition.Attr, k, valueType)
			if adh.esClient.IsNotFoundError(err) {
				err = adh.params.ESClient.CreateIndex(ctx, index)
				if err != nil {
					return adh.error(&types.InternalServiceError{Message: fmt.Sprintf("Failed to create ES index, err: %v", err)}, scope)
				}
				err = adh.params.ESClient.PutMapping(ctx, index, definition.Attr, k, valueType)
			}
			if err != nil {
				return adh.error(&types.InternalServiceError{Message: fmt.Sprintf("Failed to update ES mapping, err: %v", err)}, scope)
			}
		}
	}

	return nil
}

// DescribeWorkflowExecution returns information about the specified workflow execution.
func (adh *adminHandlerImpl) DescribeWorkflowExecution(
	ctx context.Context,
	request *types.AdminDescribeWorkflowExecutionRequest,
) (resp *types.AdminDescribeWorkflowExecutionResponse, retError error) {

	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &retError) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.AdminDescribeWorkflowExecutionScope)
	defer sw.Stop()

	if request == nil {
		return nil, adh.error(validate.ErrRequestNotSet, scope)
	}

	if err := validate.CheckExecution(request.Execution); err != nil {
		return nil, adh.error(err, scope)
	}

	shardID := common.WorkflowIDToHistoryShard(request.Execution.WorkflowID, adh.numberOfHistoryShards)
	shardIDForOutput := strconv.Itoa(shardID)

	historyHost, err := lookup.HistoryServerByShardID(adh.GetMembershipResolver(), shardID)
	if err != nil {
		return nil, adh.error(err, scope)
	}

	domainID, err := adh.GetDomainCache().GetDomainID(request.GetDomain())
	if err != nil {
		return nil, adh.error(err, scope)
	}

	historyAddr := historyHost.GetAddress()
	resp2, err := adh.GetHistoryClient().DescribeMutableState(ctx, &types.DescribeMutableStateRequest{
		DomainUUID: domainID,
		Execution:  request.Execution,
	})
	if err != nil {
		return &types.AdminDescribeWorkflowExecutionResponse{}, err
	}
	return &types.AdminDescribeWorkflowExecutionResponse{
		ShardID:                shardIDForOutput,
		HistoryAddr:            historyAddr,
		MutableStateInDatabase: resp2.MutableStateInDatabase,
		MutableStateInCache:    resp2.MutableStateInCache,
	}, err
}

// RemoveTask returns information about the internal states of a history host
func (adh *adminHandlerImpl) RemoveTask(
	ctx context.Context,
	request *types.RemoveTaskRequest,
) (retError error) {

	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &retError) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.AdminRemoveTaskScope)
	defer sw.Stop()

	if request == nil || request.Type == nil {
		return adh.error(validate.ErrRequestNotSet, scope)
	}
	if err := adh.GetHistoryClient().RemoveTask(ctx, request); err != nil {
		return adh.error(err, scope)
	}
	return nil
}

func (adh *adminHandlerImpl) getCorruptWorkflowQueryTemplates(
	ctx context.Context, request *types.AdminMaintainWorkflowRequest,
) []workflowQueryTemplate {
	client := adh.GetFrontendClient()
	return []workflowQueryTemplate{
		{
			name: "DescribeWorkflowExecution",
			function: func(request *types.AdminMaintainWorkflowRequest) error {
				_, err := client.DescribeWorkflowExecution(ctx, &types.DescribeWorkflowExecutionRequest{
					Domain:    request.Domain,
					Execution: request.Execution,
				})
				return err
			},
		},
		{
			name: "GetWorkflowExecutionHistory",
			function: func(request *types.AdminMaintainWorkflowRequest) error {
				_, err := client.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
					Domain:    request.Domain,
					Execution: request.Execution,
				})
				return err
			},
		},
	}
}

func (adh *adminHandlerImpl) MaintainCorruptWorkflow(
	ctx context.Context,
	request *types.AdminMaintainWorkflowRequest,
) (*types.AdminMaintainWorkflowResponse, error) {
	if request.GetExecution() == nil {
		return nil, types.BadRequestError{Message: "Execution is missing"}
	}

	logger := adh.GetLogger().WithTags(
		tag.WorkflowDomainName(request.Domain),
		tag.WorkflowID(request.GetExecution().GetWorkflowID()),
		tag.WorkflowRunID(request.GetExecution().GetRunID()),
	)

	resp := &types.AdminMaintainWorkflowResponse{
		HistoryDeleted:    false,
		ExecutionsDeleted: false,
		VisibilityDeleted: false,
	}

	queryTemplates := adh.getCorruptWorkflowQueryTemplates(ctx, request)
	for _, template := range queryTemplates {
		functionName := template.name
		queryFunc := template.function
		err := queryFunc(request)
		if err == nil {
			logger.Info(fmt.Sprintf("Query succeeded for function: %s", functionName))
			continue
		}
		if err != nil {
			logger.Info(fmt.Sprintf("%s returned error %#v", functionName, err))
		}

		// check if the error message indicates corrupt workflow
		errorMessage := err.Error()
		for _, corruptMessage := range corruptWorkflowErrorList {
			if errorMessage == corruptMessage {
				logger.Info(fmt.Sprintf("Will delete workflow because (%v) returned corrupted error (%#v)",
					functionName, err))
				resp, err = adh.DeleteWorkflow(ctx, request)
				return resp, nil
			}
		}
	}

	return resp, nil
}

func (adh *adminHandlerImpl) deleteWorkflowFromHistory(
	ctx context.Context,
	logger log.Logger,
	shardIDInt int,
	mutableState persistence.WorkflowMutableState,
) bool {
	historyManager := adh.GetHistoryManager()

	branchInfo := shared.HistoryBranch{}
	thriftrwEncoder := codec.NewThriftRWEncoder()
	branchTokens := [][]byte{mutableState.ExecutionInfo.BranchToken}
	if mutableState.VersionHistories != nil {
		// if VersionHistories is set, then all branch infos are stored in VersionHistories
		branchTokens = [][]byte{}
		for _, versionHistory := range mutableState.VersionHistories.ToInternalType().Histories {
			branchTokens = append(branchTokens, versionHistory.BranchToken)
		}
	}

	deletedFromHistory := len(branchTokens) == 0
	failedToDeleteFromHistory := false
	for _, branchToken := range branchTokens {
		err := thriftrwEncoder.Decode(branchToken, &branchInfo)
		if err != nil {
			logger.Error("Cannot decode thrift object", tag.Error(err))
			continue
		}
		domainName, err := adh.GetDomainCache().GetDomainName(mutableState.ExecutionInfo.DomainID)
		if err != nil {
			logger.Error("Unexpected: Cannot fetch domain name", tag.Error(err))
			continue
		}
		logger.Info(fmt.Sprintf("Deleting history events for %#v", branchInfo))
		err = historyManager.DeleteHistoryBranch(ctx, &persistence.DeleteHistoryBranchRequest{
			BranchToken: branchToken,
			ShardID:     &shardIDInt,
			DomainName:  domainName,
		})
		if err != nil {
			logger.Error("Failed to delete history", tag.Error(err))
			failedToDeleteFromHistory = true
		} else {
			deletedFromHistory = true
		}
	}
	return deletedFromHistory && !failedToDeleteFromHistory
}

func (adh *adminHandlerImpl) deleteWorkflowFromExecutions(
	ctx context.Context,
	logger log.Logger,
	shardIDInt int,
	domainID string,
	workflowID string,
	runID string,
	scope metrics.Scope,
) bool {
	exeStore, err := adh.GetExecutionManager(shardIDInt)
	if err != nil {
		logger.Error(fmt.Sprintf("Cannot get execution manager for shardID(%v): %#v", shardIDInt, err))
		return false
	}
	domainName, err := adh.GetDomainCache().GetDomainName(domainID)
	if err != nil {
		logger.Error("Unexpected: Cannot fetch domain name", tag.Error(err))
		return false
	}
	req := &persistence.DeleteWorkflowExecutionRequest{
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
		DomainName: domainName,
	}

	deletedFromExecutions := false
	err = exeStore.DeleteWorkflowExecution(ctx, req)
	if err != nil {
		logger.Error("Delete mutableState row failed", tag.Error(err))
	} else {
		deletedFromExecutions = true
	}

	deleteCurrentReq := &persistence.DeleteCurrentWorkflowExecutionRequest{
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
		DomainName: domainName,
	}

	err = exeStore.DeleteCurrentWorkflowExecution(ctx, deleteCurrentReq)
	if err != nil {
		logger.Error(fmt.Sprintf("Delete current row failed: %#v", err))
		deletedFromExecutions = false
	}

	if deletedFromExecutions {
		logger.Info(fmt.Sprintf("Deleted executions row successfully %#v", deleteCurrentReq))
	}
	return deletedFromExecutions
}

func (adh *adminHandlerImpl) deleteWorkflowFromVisibility(
	ctx context.Context,
	logger log.Logger,
	domainID string,
	domain string,
	workflowID string,
	runID string,
) bool {
	visibilityManager := adh.Resource.GetVisibilityManager()
	if visibilityManager == nil {
		logger.Info("No visibility manager found")
		return false
	}

	logger.Info("Deleting workflow from visibility store")
	key := persistence.VisibilityAdminDeletionKey("visibilityAdminDelete")
	visCtx := context.WithValue(ctx, key, true)
	err := visibilityManager.DeleteWorkflowExecution(
		visCtx,
		&persistence.VisibilityDeleteWorkflowExecutionRequest{
			DomainID:   domainID,
			Domain:     domain,
			RunID:      runID,
			WorkflowID: workflowID,
			TaskID:     math.MaxInt64,
		},
	)
	if err != nil {
		logger.Error("Cannot delete visibility record", tag.Error(err))
	} else {
		logger.Info("Deleted visibility record successfully")
	}
	return err == nil
}

// DeleteWorkflow delete a workflow execution for admin
func (adh *adminHandlerImpl) DeleteWorkflow(
	ctx context.Context,
	request *types.AdminDeleteWorkflowRequest,
) (*types.AdminDeleteWorkflowResponse, error) {
	logger := adh.GetLogger()
	scope := adh.GetMetricsClient().Scope(metrics.AdminDeleteWorkflowScope).Tagged(metrics.GetContextTags(ctx)...)
	if request.GetExecution() == nil {
		logger.Info(fmt.Sprintf("Bad request: %#v", request))
		return nil, adh.error(validate.ErrRequestNotSet, scope)
	}
	domainName := request.GetDomain()
	workflowID := request.GetExecution().GetWorkflowID()
	runID := request.GetExecution().GetRunID()
	skipErrors := request.GetSkipErrors()

	resp, err := adh.DescribeWorkflowExecution(
		ctx,
		&types.AdminDescribeWorkflowExecutionRequest{
			Domain: domainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: workflowID,
				RunID:      runID,
			},
		})

	if err != nil {
		logger.Error("Describe workflow failed", tag.Error(err))
		if !skipErrors {
			return nil, adh.error(err, scope)
		}
	}

	msStr := resp.GetMutableStateInDatabase()
	ms := persistence.WorkflowMutableState{}
	err = json.Unmarshal([]byte(msStr), &ms)
	if err != nil {
		logger.Error(fmt.Sprintf("DeleteWorkflow failed: Cannot unmarshal mutableState: %#v", err))
		return nil, adh.error(err, scope)
	}
	domainID := ms.ExecutionInfo.DomainID
	logger = logger.WithTags(
		tag.WorkflowDomainID(domainID),
		tag.WorkflowDomainName(domainName),
		tag.WorkflowID(workflowID),
		tag.WorkflowRunID(runID),
	)

	shardID := resp.GetShardID()
	shardIDInt, err := strconv.Atoi(shardID)
	if err != nil {
		logger.Error(fmt.Sprintf("Cannot convert shardID(%v) to int: %#v", shardID, err))
		return nil, adh.error(err, scope)
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	deletedFromHistory := adh.deleteWorkflowFromHistory(ctx, logger, shardIDInt, ms)
	deletedFromExecutions := adh.deleteWorkflowFromExecutions(ctx, logger, shardIDInt, domainID, workflowID, runID, scope)
	deletedFromVisibility := false
	if deletedFromExecutions {
		// Without deleting the executions record, let's not delete the visibility record.
		// If we do that, workflow won't be visible but it will exist in the DB
		deletedFromVisibility = adh.deleteWorkflowFromVisibility(ctx, logger, domainID, domainName, workflowID, runID)
	}

	return &types.AdminDeleteWorkflowResponse{
		HistoryDeleted:    deletedFromHistory,
		ExecutionsDeleted: deletedFromExecutions,
		VisibilityDeleted: deletedFromVisibility,
	}, nil
}

// CloseShard returns information about the internal states of a history host
func (adh *adminHandlerImpl) CloseShard(
	ctx context.Context,
	request *types.CloseShardRequest,
) (retError error) {

	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &retError) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.AdminCloseShardScope)
	defer sw.Stop()

	if request == nil {
		return adh.error(validate.ErrRequestNotSet, scope)
	}
	if err := adh.GetHistoryClient().CloseShard(ctx, request); err != nil {
		return adh.error(err, scope)
	}
	return nil
}

// ResetQueue resets processing queue states
func (adh *adminHandlerImpl) ResetQueue(
	ctx context.Context,
	request *types.ResetQueueRequest,
) (retError error) {

	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &retError) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.AdminResetQueueScope)
	defer sw.Stop()

	if request == nil || request.Type == nil {
		return adh.error(validate.ErrRequestNotSet, scope)
	}
	if request.GetClusterName() == "" {
		return adh.error(validate.ErrClusterNameNotSet, scope)
	}

	if err := adh.GetHistoryClient().ResetQueue(ctx, request); err != nil {
		return adh.error(err, scope)
	}
	return nil
}

// DescribeQueue describes processing queue states
func (adh *adminHandlerImpl) DescribeQueue(
	ctx context.Context,
	request *types.DescribeQueueRequest,
) (resp *types.DescribeQueueResponse, retError error) {

	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &retError) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.AdminDescribeQueueScope)
	defer sw.Stop()

	if request == nil || request.Type == nil {
		return nil, adh.error(validate.ErrRequestNotSet, scope)
	}
	if request.GetClusterName() == "" {
		return nil, adh.error(validate.ErrClusterNameNotSet, scope)
	}

	return adh.GetHistoryClient().DescribeQueue(ctx, request)
}

// DescribeShardDistribution returns information about history shard distribution
func (adh *adminHandlerImpl) DescribeShardDistribution(
	ctx context.Context,
	request *types.DescribeShardDistributionRequest,
) (resp *types.DescribeShardDistributionResponse, retError error) {

	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &retError) }()
	_, sw := adh.startRequestProfile(ctx, metrics.AdminDescribeShardDistributionScope)
	defer sw.Stop()

	resp = &types.DescribeShardDistributionResponse{
		NumberOfShards: int32(adh.numberOfHistoryShards),
		Shards:         make(map[int32]string),
	}

	offset := int(request.PageID * request.PageSize)
	nextPageStart := offset + int(request.PageSize)
	for shardID := offset; shardID < adh.numberOfHistoryShards && shardID < nextPageStart; shardID++ {
		info, err := lookup.HistoryServerByShardID(adh.GetMembershipResolver(), shardID)
		if err != nil {
			resp.Shards[int32(shardID)] = "unknown"
		} else {
			resp.Shards[int32(shardID)] = info.Identity()
		}
	}
	return resp, nil
}

// DescribeHistoryHost returns information about the internal states of a history host
func (adh *adminHandlerImpl) DescribeHistoryHost(
	ctx context.Context,
	request *types.DescribeHistoryHostRequest,
) (resp *types.DescribeHistoryHostResponse, retError error) {

	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &retError) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.AdminDescribeHistoryHostScope)
	defer sw.Stop()

	if request == nil || (request.ShardIDForHost == nil && request.ExecutionForHost == nil && request.HostAddress == nil) {
		return nil, adh.error(validate.ErrRequestNotSet, scope)
	}

	if request.ExecutionForHost != nil {
		if err := validate.CheckExecution(request.ExecutionForHost); err != nil {
			return nil, adh.error(err, scope)
		}
	}

	return adh.GetHistoryClient().DescribeHistoryHost(ctx, request)
}

// GetWorkflowExecutionRawHistoryV2 - retrieves the history of workflow execution
func (adh *adminHandlerImpl) GetWorkflowExecutionRawHistoryV2(
	ctx context.Context,
	request *types.GetWorkflowExecutionRawHistoryV2Request,
) (resp *types.GetWorkflowExecutionRawHistoryV2Response, retError error) {

	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &retError) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.AdminGetWorkflowExecutionRawHistoryV2Scope)
	defer sw.Stop()

	if err := adh.validateGetWorkflowExecutionRawHistoryV2Request(
		request,
	); err != nil {
		return nil, adh.error(err, scope)
	}
	domainID, err := adh.GetDomainCache().GetDomainID(request.GetDomain())
	if err != nil {
		return nil, adh.error(err, scope)
	}
	scope = scope.Tagged(metrics.DomainTag(request.GetDomain()))

	execution := request.Execution
	var pageToken *getWorkflowRawHistoryV2Token
	var targetVersionHistory *persistence.VersionHistory
	if request.NextPageToken == nil {
		response, err := adh.GetHistoryClient().GetMutableState(ctx, &types.GetMutableStateRequest{
			DomainUUID: domainID,
			Execution:  execution,
		})
		if err != nil {
			return nil, adh.error(err, scope)
		}

		versionHistories := persistence.NewVersionHistoriesFromInternalType(
			response.GetVersionHistories(),
		)
		targetVersionHistory, err = adh.setRequestDefaultValueAndGetTargetVersionHistory(
			request,
			versionHistories,
		)
		if err != nil {
			return nil, adh.error(err, scope)
		}

		pageToken = adh.generatePaginationToken(request, versionHistories)
	} else {
		pageToken, err = deserializeRawHistoryToken(request.NextPageToken)
		if err != nil {
			return nil, adh.error(err, scope)
		}
		versionHistories := pageToken.VersionHistories
		if versionHistories == nil {
			return nil, adh.error(&types.BadRequestError{Message: "Invalid version histories."}, scope)
		}
		targetVersionHistory, err = adh.setRequestDefaultValueAndGetTargetVersionHistory(
			request,
			persistence.NewVersionHistoriesFromInternalType(versionHistories),
		)
		if err != nil {
			return nil, adh.error(err, scope)
		}
	}

	if err := adh.validatePaginationToken(
		request,
		pageToken,
	); err != nil {
		return nil, adh.error(err, scope)
	}

	if pageToken.StartEventID+1 == pageToken.EndEventID {
		// API is exclusive-exclusive. Return empty response here.
		return &types.GetWorkflowExecutionRawHistoryV2Response{
			HistoryBatches: []*types.DataBlob{},
			NextPageToken:  nil, // no further pagination
			VersionHistory: targetVersionHistory.ToInternalType(),
		}, nil
	}
	pageSize := int(request.GetMaximumPageSize())
	shardID := common.WorkflowIDToHistoryShard(
		execution.GetWorkflowID(),
		adh.numberOfHistoryShards,
	)

	rawHistoryResponse, err := adh.GetHistoryManager().ReadRawHistoryBranch(ctx, &persistence.ReadHistoryBranchRequest{
		BranchToken: targetVersionHistory.GetBranchToken(),
		// GetWorkflowExecutionRawHistoryV2 is exclusive exclusive.
		// ReadRawHistoryBranch is inclusive exclusive.
		MinEventID:    pageToken.StartEventID + 1,
		MaxEventID:    pageToken.EndEventID,
		PageSize:      pageSize,
		NextPageToken: pageToken.PersistenceToken,
		ShardID:       common.IntPtr(shardID),
		DomainName:    request.GetDomain(),
	})
	if err != nil {
		if _, ok := err.(*types.EntityNotExistsError); ok {
			// when no events can be returned from DB, DB layer will return
			// EntityNotExistsError, this API shall return empty response
			return &types.GetWorkflowExecutionRawHistoryV2Response{
				HistoryBatches: []*types.DataBlob{},
				NextPageToken:  nil, // no further pagination
				VersionHistory: targetVersionHistory.ToInternalType(),
			}, nil
		}
		return nil, err
	}

	pageToken.PersistenceToken = rawHistoryResponse.NextPageToken
	size := rawHistoryResponse.Size
	scope.RecordTimer(metrics.HistorySize, time.Duration(size))

	rawBlobs := rawHistoryResponse.HistoryEventBlobs
	blobs := []*types.DataBlob{}
	for _, blob := range rawBlobs {
		blobs = append(blobs, blob.ToInternal())
	}

	result := &types.GetWorkflowExecutionRawHistoryV2Response{
		HistoryBatches: blobs,
		VersionHistory: targetVersionHistory.ToInternalType(),
	}
	if len(pageToken.PersistenceToken) == 0 {
		result.NextPageToken = nil
	} else {
		result.NextPageToken, err = serializeRawHistoryToken(pageToken)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

// DescribeCluster return information about cadence deployment
func (adh *adminHandlerImpl) DescribeCluster(
	ctx context.Context,
) (resp *types.DescribeClusterResponse, retError error) {

	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &retError) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.AdminDescribeClusterScope)
	defer sw.Stop()

	// expose visibility store backend and if advanced options are available
	ave := types.PersistenceFeature{
		Key:     "advancedVisibilityEnabled",
		Enabled: adh.params.ESConfig != nil,
	}
	visibilityStoreInfo := types.PersistenceInfo{
		Backend:  adh.Resource.GetVisibilityManager().GetName(),
		Features: []*types.PersistenceFeature{&ave},
	}

	// expose history store backend
	historyStoreInfo := types.PersistenceInfo{
		Backend: adh.GetHistoryManager().GetName(),
	}

	membershipInfo := types.MembershipInfo{}
	if monitor := adh.GetMembershipResolver(); monitor != nil {
		currentHost, err := monitor.WhoAmI()
		if err != nil {
			return nil, adh.error(err, scope)
		}

		membershipInfo.CurrentHost = &types.HostInfo{
			Identity: currentHost.Identity(),
		}

		var rings []*types.RingInfo
		for _, role := range service.ListWithRing {
			var servers []*types.HostInfo
			members, err := monitor.Members(role)
			if err != nil {
				return nil, adh.error(err, scope)
			}

			for _, server := range members {
				servers = append(servers, &types.HostInfo{
					Identity: server.Identity(),
				})
				membershipInfo.ReachableMembers = append(membershipInfo.ReachableMembers, server.Identity())
			}

			rings = append(rings, &types.RingInfo{
				Role:        role,
				MemberCount: int32(len(servers)),
				Members:     servers,
			})
		}
		membershipInfo.Rings = rings
	}

	return &types.DescribeClusterResponse{
		SupportedClientVersions: &types.SupportedClientVersions{
			GoSdk:   client.SupportedGoSDKVersion,
			JavaSdk: client.SupportedJavaSDKVersion,
		},
		MembershipInfo: &membershipInfo,
		PersistenceInfo: map[string]*types.PersistenceInfo{
			"visibilityStore": &visibilityStoreInfo,
			"historyStore":    &historyStoreInfo,
		},
	}, nil
}

// GetReplicationMessages returns new replication tasks since the read level provided in the token.
func (adh *adminHandlerImpl) GetReplicationMessages(
	ctx context.Context,
	request *types.GetReplicationMessagesRequest,
) (resp *types.GetReplicationMessagesResponse, err error) {

	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &err) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.AdminGetReplicationMessagesScope)
	defer sw.Stop()

	if request == nil {
		return nil, adh.error(validate.ErrRequestNotSet, scope)
	}
	if request.ClusterName == "" {
		return nil, adh.error(validate.ErrClusterNameNotSet, scope)
	}

	resp, err = adh.GetHistoryRawClient().GetReplicationMessages(ctx, request)
	if err != nil {
		return nil, adh.error(err, scope)
	}
	return resp, nil
}

// GetDomainReplicationMessages returns new domain replication tasks since last retrieved task ID.
func (adh *adminHandlerImpl) GetDomainReplicationMessages(
	ctx context.Context,
	request *types.GetDomainReplicationMessagesRequest,
) (resp *types.GetDomainReplicationMessagesResponse, err error) {

	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &err) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.AdminGetDomainReplicationMessagesScope)
	defer sw.Stop()

	if request == nil {
		return nil, adh.error(validate.ErrRequestNotSet, scope)
	}

	if adh.GetDomainReplicationQueue() == nil {
		return nil, adh.error(errors.New("domain replication queue not enabled for cluster"), scope)
	}

	lastMessageID := defaultLastMessageID
	if request.LastRetrievedMessageID != nil {
		lastMessageID = request.GetLastRetrievedMessageID()
	}

	if lastMessageID == defaultLastMessageID {
		clusterAckLevels, err := adh.GetDomainReplicationQueue().GetAckLevels(ctx)
		if err == nil {
			if ackLevel, ok := clusterAckLevels[request.GetClusterName()]; ok {
				lastMessageID = ackLevel
			}
		}
	}

	replicationTasks, lastMessageID, err := adh.GetDomainReplicationQueue().GetReplicationMessages(
		ctx,
		lastMessageID,
		getDomainReplicationMessageBatchSize,
	)
	if err != nil {
		return nil, adh.error(err, scope)
	}

	lastProcessedMessageID := defaultLastMessageID
	if request.LastProcessedMessageID != nil {
		lastProcessedMessageID = request.GetLastProcessedMessageID()
	}
	if err := adh.GetDomainReplicationQueue().UpdateAckLevel(ctx, lastProcessedMessageID, request.GetClusterName()); err != nil {
		adh.GetLogger().Warn("Failed to update domain replication queue ack level.",
			tag.TaskID(int64(lastProcessedMessageID)),
			tag.ClusterName(request.GetClusterName()))
	}

	return &types.GetDomainReplicationMessagesResponse{
		Messages: &types.ReplicationMessages{
			ReplicationTasks:       replicationTasks,
			LastRetrievedMessageID: lastMessageID,
		},
	}, nil
}

// GetDLQReplicationMessages returns new replication tasks based on the dlq info.
func (adh *adminHandlerImpl) GetDLQReplicationMessages(
	ctx context.Context,
	request *types.GetDLQReplicationMessagesRequest,
) (resp *types.GetDLQReplicationMessagesResponse, err error) {

	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &err) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.AdminGetDLQReplicationMessagesScope)
	defer sw.Stop()

	if request == nil {
		return nil, adh.error(validate.ErrRequestNotSet, scope)
	}
	if len(request.GetTaskInfos()) == 0 {
		return nil, adh.error(validate.ErrEmptyReplicationInfo, scope)
	}

	resp, err = adh.GetHistoryClient().GetDLQReplicationMessages(ctx, request)
	if err != nil {
		return nil, adh.error(err, scope)
	}
	return resp, nil
}

// ReapplyEvents applies stale events to the current workflow and the current run
func (adh *adminHandlerImpl) ReapplyEvents(
	ctx context.Context,
	request *types.ReapplyEventsRequest,
) (err error) {

	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &err) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.AdminReapplyEventsScope)
	defer sw.Stop()

	if request == nil {
		return adh.error(validate.ErrRequestNotSet, scope)
	}
	if request.GetDomainName() == "" {
		return adh.error(validate.ErrDomainNotSet, scope)
	}
	if request.WorkflowExecution == nil {
		return adh.error(validate.ErrExecutionNotSet, scope)
	}
	if request.GetWorkflowExecution().GetWorkflowID() == "" {
		return adh.error(validate.ErrWorkflowIDNotSet, scope)
	}
	if request.GetEvents() == nil {
		return adh.error(validate.ErrWorkflowIDNotSet, scope)
	}
	domainEntry, err := adh.GetDomainCache().GetDomain(request.GetDomainName())
	if err != nil {
		return adh.error(err, scope)
	}

	err = adh.GetHistoryClient().ReapplyEvents(ctx, &types.HistoryReapplyEventsRequest{
		DomainUUID: domainEntry.GetInfo().ID,
		Request:    request,
	})
	if err != nil {
		return adh.error(err, scope)
	}
	return nil
}

// ReadDLQMessages reads messages from DLQ
func (adh *adminHandlerImpl) ReadDLQMessages(
	ctx context.Context,
	request *types.ReadDLQMessagesRequest,
) (resp *types.ReadDLQMessagesResponse, err error) {

	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &err) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.AdminReadDLQMessagesScope)
	defer sw.Stop()

	if request == nil {
		return nil, adh.error(validate.ErrRequestNotSet, scope)
	}

	if request.Type == nil {
		return nil, adh.error(validate.ErrEmptyQueueType, scope)
	}

	if request.GetMaximumPageSize() <= 0 {
		request.MaximumPageSize = common.ReadDLQMessagesPageSize
	}

	if request.InclusiveEndMessageID == nil {
		request.InclusiveEndMessageID = common.Int64Ptr(common.EndMessageID)
	}

	var tasks []*types.ReplicationTask
	var token []byte
	var op func() error
	switch request.GetType() {
	case types.DLQTypeReplication:
		return adh.GetHistoryClient().ReadDLQMessages(ctx, request)
	case types.DLQTypeDomain:
		op = func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				var err error
				tasks, token, err = adh.domainDLQHandler.Read(
					ctx,
					request.GetInclusiveEndMessageID(),
					int(request.GetMaximumPageSize()),
					request.GetNextPageToken())
				return err
			}
		}
	default:
		return nil, &types.BadRequestError{Message: "The DLQ type is not supported."}
	}
	err = adh.throttleRetry.Do(ctx, op)
	if err != nil {
		return nil, adh.error(err, scope)
	}

	return &types.ReadDLQMessagesResponse{
		ReplicationTasks: tasks,
		NextPageToken:    token,
	}, nil
}

// PurgeDLQMessages purge messages from DLQ
func (adh *adminHandlerImpl) PurgeDLQMessages(
	ctx context.Context,
	request *types.PurgeDLQMessagesRequest,
) (err error) {

	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &err) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.AdminPurgeDLQMessagesScope)
	defer sw.Stop()

	if request == nil {
		return adh.error(validate.ErrRequestNotSet, scope)
	}

	if request.Type == nil {
		return adh.error(validate.ErrEmptyQueueType, scope)
	}

	if request.InclusiveEndMessageID == nil {
		request.InclusiveEndMessageID = common.Int64Ptr(endMessageID)
	}

	var op func() error
	switch request.GetType() {
	case types.DLQTypeReplication:
		return adh.GetHistoryClient().PurgeDLQMessages(ctx, request)
	case types.DLQTypeDomain:
		op = func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				return adh.domainDLQHandler.Purge(
					ctx,
					request.GetInclusiveEndMessageID(),
				)
			}
		}
	default:
		return &types.BadRequestError{Message: "The DLQ type is not supported."}
	}
	err = adh.throttleRetry.Do(ctx, op)
	if err != nil {
		return adh.error(err, scope)
	}

	return nil
}

func (adh *adminHandlerImpl) CountDLQMessages(
	ctx context.Context,
	request *types.CountDLQMessagesRequest,
) (resp *types.CountDLQMessagesResponse, err error) {
	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &err) }()

	scope, sw := adh.startRequestProfile(ctx, metrics.AdminCountDLQMessagesScope)
	defer sw.Stop()

	domain, err := adh.domainDLQHandler.Count(ctx, request.ForceFetch)
	if err != nil {
		return nil, adh.error(err, scope)
	}

	history, err := adh.GetHistoryClient().CountDLQMessages(ctx, request)
	if err != nil {
		err = adh.error(err, scope)
	}

	return &types.CountDLQMessagesResponse{
		History: history.Entries,
		Domain:  domain,
	}, err
}

// MergeDLQMessages merges DLQ messages
func (adh *adminHandlerImpl) MergeDLQMessages(
	ctx context.Context,
	request *types.MergeDLQMessagesRequest,
) (resp *types.MergeDLQMessagesResponse, err error) {

	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &err) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.AdminMergeDLQMessagesScope)
	defer sw.Stop()

	if request == nil {
		return nil, adh.error(validate.ErrRequestNotSet, scope)
	}

	if request.Type == nil {
		return nil, adh.error(validate.ErrEmptyQueueType, scope)
	}

	if request.InclusiveEndMessageID == nil {
		request.InclusiveEndMessageID = common.Int64Ptr(endMessageID)
	}

	var token []byte
	var op func() error
	switch request.GetType() {
	case types.DLQTypeReplication:
		return adh.GetHistoryClient().MergeDLQMessages(ctx, request)
	case types.DLQTypeDomain:

		op = func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				var err error
				token, err = adh.domainDLQHandler.Merge(
					ctx,
					request.GetInclusiveEndMessageID(),
					int(request.GetMaximumPageSize()),
					request.GetNextPageToken(),
				)
				return err
			}
		}
	default:
		return nil, &types.BadRequestError{Message: "The DLQ type is not supported."}
	}
	err = adh.throttleRetry.Do(ctx, op)
	if err != nil {
		return nil, adh.error(err, scope)
	}

	return &types.MergeDLQMessagesResponse{
		NextPageToken: token,
	}, nil
}

// RefreshWorkflowTasks re-generates the workflow tasks
func (adh *adminHandlerImpl) RefreshWorkflowTasks(
	ctx context.Context,
	request *types.RefreshWorkflowTasksRequest,
) (err error) {
	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &err) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.AdminRefreshWorkflowTasksScope)
	defer sw.Stop()

	if request == nil {
		return adh.error(validate.ErrRequestNotSet, scope)
	}
	if err := validate.CheckExecution(request.Execution); err != nil {
		return adh.error(err, scope)
	}
	domainEntry, err := adh.GetDomainCache().GetDomain(request.GetDomain())
	if err != nil {
		return adh.error(err, scope)
	}

	err = adh.GetHistoryClient().RefreshWorkflowTasks(ctx, &types.HistoryRefreshWorkflowTasksRequest{
		DomainUIID: domainEntry.GetInfo().ID,
		Request:    request,
	})
	if err != nil {
		return adh.error(err, scope)
	}
	return nil
}

// ResendReplicationTasks requests replication task from remote cluster
func (adh *adminHandlerImpl) ResendReplicationTasks(
	ctx context.Context,
	request *types.ResendReplicationTasksRequest,
) (err error) {
	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &err) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.AdminResendReplicationTasksScope)
	defer sw.Stop()

	if request == nil {
		return adh.error(validate.ErrRequestNotSet, scope)
	}
	resender := ndc.NewHistoryResender(
		adh.GetDomainCache(),
		adh.GetRemoteAdminClient(request.GetRemoteCluster()),
		func(ctx context.Context, request *types.ReplicateEventsV2Request) error {
			return adh.GetHistoryClient().ReplicateEventsV2(ctx, request)
		},
		nil,
		nil,
		adh.GetLogger(),
	)
	return resender.SendSingleWorkflowHistory(
		request.DomainID,
		request.GetWorkflowID(),
		request.GetRunID(),
		request.StartEventID,
		request.StartVersion,
		request.EndEventID,
		request.EndVersion,
	)
}

func (adh *adminHandlerImpl) GetCrossClusterTasks(
	ctx context.Context,
	request *types.GetCrossClusterTasksRequest,
) (resp *types.GetCrossClusterTasksResponse, err error) {

	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &err) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.AdminGetCrossClusterTasksScope)
	defer sw.Stop()

	if request == nil {
		return nil, adh.error(validate.ErrRequestNotSet, scope)
	}
	if request.TargetCluster == "" {
		return nil, adh.error(validate.ErrClusterNameNotSet, scope)
	}

	resp, err = adh.GetHistoryRawClient().GetCrossClusterTasks(ctx, request)
	if err != nil {
		return nil, adh.error(err, scope)
	}
	return resp, nil
}

func (adh *adminHandlerImpl) RespondCrossClusterTasksCompleted(
	ctx context.Context,
	request *types.RespondCrossClusterTasksCompletedRequest,
) (resp *types.RespondCrossClusterTasksCompletedResponse, err error) {

	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &err) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.AdminRespondCrossClusterTasksCompletedScope)
	defer sw.Stop()

	if request == nil {
		return nil, adh.error(validate.ErrRequestNotSet, scope)
	}
	if request.TargetCluster == "" {
		return nil, adh.error(validate.ErrClusterNameNotSet, scope)
	}

	resp, err = adh.GetHistoryClient().RespondCrossClusterTasksCompleted(ctx, request)
	if err != nil {
		return nil, adh.error(err, scope)
	}
	return resp, nil
}

func (adh *adminHandlerImpl) validateGetWorkflowExecutionRawHistoryV2Request(
	request *types.GetWorkflowExecutionRawHistoryV2Request,
) error {

	execution := request.Execution
	if len(execution.GetWorkflowID()) == 0 {
		return &types.BadRequestError{Message: "Invalid WorkflowID."}
	}
	// TODO currently, this API is only going to be used by re-send history events
	// to remote cluster if kafka is lossy again, in the future, this API can be used
	// by CLI and client, then empty runID (meaning the current workflow) should be allowed
	if _, err := uuid.Parse(execution.GetRunID()); err != nil {
		return &types.BadRequestError{Message: "Invalid RunID."}
	}

	pageSize := int(request.GetMaximumPageSize())
	if pageSize <= 0 {
		return &types.BadRequestError{Message: "Invalid PageSize."}
	}

	if (request.StartEventID != nil && request.StartEventVersion == nil) ||
		(request.StartEventID == nil && request.StartEventVersion != nil) {
		return &types.BadRequestError{Message: "Invalid start event id and start event version combination."}
	}

	if (request.EndEventID != nil && request.EndEventVersion == nil) ||
		(request.EndEventID == nil && request.EndEventVersion != nil) {
		return &types.BadRequestError{Message: "Invalid end event id and end event version combination."}
	}
	return nil
}

func (adh *adminHandlerImpl) validateConfigForAdvanceVisibility() error {
	if adh.params.ESConfig == nil || adh.params.ESClient == nil {
		return errors.New("ES related config not found")
	}
	return nil
}

func (adh *adminHandlerImpl) setRequestDefaultValueAndGetTargetVersionHistory(
	request *types.GetWorkflowExecutionRawHistoryV2Request,
	versionHistories *persistence.VersionHistories,
) (*persistence.VersionHistory, error) {

	targetBranch, err := versionHistories.GetCurrentVersionHistory()
	if err != nil {
		return nil, err
	}
	firstItem, err := targetBranch.GetFirstItem()
	if err != nil {
		return nil, err
	}
	lastItem, err := targetBranch.GetLastItem()
	if err != nil {
		return nil, err
	}

	if request.StartEventID == nil || request.StartEventVersion == nil {
		// If start event is not set, get the events from the first event
		// As the API is exclusive-exclusive, use first event id - 1 here
		request.StartEventID = common.Int64Ptr(common.FirstEventID - 1)
		request.StartEventVersion = common.Int64Ptr(firstItem.Version)
	}
	if request.EndEventID == nil || request.EndEventVersion == nil {
		// If end event is not set, get the events until the end event
		// As the API is exclusive-exclusive, use end event id + 1 here
		request.EndEventID = common.Int64Ptr(lastItem.EventID + 1)
		request.EndEventVersion = common.Int64Ptr(lastItem.Version)
	}

	if request.GetStartEventID() < 0 {
		return nil, &types.BadRequestError{Message: "Invalid FirstEventID && NextEventID combination."}
	}

	// get branch based on the end event if end event is defined in the request
	if request.GetEndEventID() == lastItem.EventID+1 &&
		request.GetEndEventVersion() == lastItem.Version {
		// this is a special case, target branch remains the same
	} else {
		endItem := persistence.NewVersionHistoryItem(request.GetEndEventID(), request.GetEndEventVersion())
		_, targetBranch, err = versionHistories.FindFirstVersionHistoryByItem(endItem)
		if err != nil {
			return nil, err
		}
	}

	startItem := persistence.NewVersionHistoryItem(request.GetStartEventID(), request.GetStartEventVersion())
	// If the request start event is defined. The start event may be on a different branch as current branch.
	// We need to find the LCA of the start event and the current branch.
	if request.GetStartEventID() == common.FirstEventID-1 &&
		request.GetStartEventVersion() == firstItem.Version {
		// this is a special case, start event is on the same branch as target branch
	} else {
		if !targetBranch.ContainsItem(startItem) {
			_, startBranch, err := versionHistories.FindFirstVersionHistoryByItem(startItem)
			if err != nil {
				return nil, err
			}
			startItem, err = targetBranch.FindLCAItem(startBranch)
			if err != nil {
				return nil, err
			}
			request.StartEventID = common.Int64Ptr(startItem.EventID)
			request.StartEventVersion = common.Int64Ptr(startItem.Version)
		}
	}

	return targetBranch, nil
}

func (adh *adminHandlerImpl) generatePaginationToken(
	request *types.GetWorkflowExecutionRawHistoryV2Request,
	versionHistories *persistence.VersionHistories,
) *getWorkflowRawHistoryV2Token {

	execution := request.Execution
	return &getWorkflowRawHistoryV2Token{
		DomainName:        request.GetDomain(),
		WorkflowID:        execution.GetWorkflowID(),
		RunID:             execution.GetRunID(),
		StartEventID:      request.GetStartEventID(),
		StartEventVersion: request.GetStartEventVersion(),
		EndEventID:        request.GetEndEventID(),
		EndEventVersion:   request.GetEndEventVersion(),
		VersionHistories:  versionHistories.ToInternalType(),
		PersistenceToken:  nil, // this is the initialized value
	}
}

func (adh *adminHandlerImpl) validatePaginationToken(
	request *types.GetWorkflowExecutionRawHistoryV2Request,
	token *getWorkflowRawHistoryV2Token,
) error {

	execution := request.Execution
	if request.GetDomain() != token.DomainName ||
		execution.GetWorkflowID() != token.WorkflowID ||
		execution.GetRunID() != token.RunID ||
		request.GetStartEventID() != token.StartEventID ||
		request.GetStartEventVersion() != token.StartEventVersion ||
		request.GetEndEventID() != token.EndEventID ||
		request.GetEndEventVersion() != token.EndEventVersion {
		return &types.BadRequestError{Message: "Invalid pagination token."}
	}
	return nil
}

// startRequestProfile initiates recording of request metrics
func (adh *adminHandlerImpl) startRequestProfile(ctx context.Context, scope int) (metrics.Scope, metrics.Stopwatch) {
	metricsScope := adh.GetMetricsClient().Scope(scope).Tagged(metrics.DomainUnknownTag()).Tagged(metrics.GetContextTags(ctx)...)
	sw := metricsScope.StartTimer(metrics.CadenceLatency)
	metricsScope.IncCounter(metrics.CadenceRequests)
	return metricsScope, sw
}

func (adh *adminHandlerImpl) error(err error, scope metrics.Scope) error {
	switch err.(type) {
	case *types.InternalServiceError:
		adh.GetLogger().Error("Internal service error", tag.Error(err))
		scope.IncCounter(metrics.CadenceFailures)
		return err
	case *types.BadRequestError:
		scope.IncCounter(metrics.CadenceErrBadRequestCounter)
		return err
	case *types.ServiceBusyError:
		scope.IncCounter(metrics.CadenceErrServiceBusyCounter)
		return err
	case *types.EntityNotExistsError:
		return err
	default:
		adh.GetLogger().Error("Uncategorized error", tag.Error(err))
		scope.IncCounter(metrics.CadenceFailures)
		return &types.InternalServiceError{Message: err.Error()}
	}
}

func convertIndexedValueTypeToESDataType(valueType types.IndexedValueType) string {
	switch valueType {
	case types.IndexedValueTypeString:
		return "text"
	case types.IndexedValueTypeKeyword:
		return "keyword"
	case types.IndexedValueTypeInt:
		return "long"
	case types.IndexedValueTypeDouble:
		return "double"
	case types.IndexedValueTypeBool:
		return "boolean"
	case types.IndexedValueTypeDatetime:
		return "date"
	default:
		return ""
	}
}

func serializeRawHistoryToken(token *getWorkflowRawHistoryV2Token) ([]byte, error) {
	if token == nil {
		return nil, nil
	}

	bytes, err := json.Marshal(token)
	return bytes, err
}

func deserializeRawHistoryToken(bytes []byte) (*getWorkflowRawHistoryV2Token, error) {
	token := &getWorkflowRawHistoryV2Token{}
	err := json.Unmarshal(bytes, token)
	return token, err
}

func (adh *adminHandlerImpl) GetDynamicConfig(ctx context.Context, request *types.GetDynamicConfigRequest) (_ *types.GetDynamicConfigResponse, retError error) {
	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &retError) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.AdminGetDynamicConfigScope)
	defer sw.Stop()

	if request == nil || request.ConfigName == "" {
		return nil, adh.error(validate.ErrRequestNotSet, scope)
	}

	keyVal, err := dc.GetKeyFromKeyName(request.ConfigName)
	if err != nil {
		return nil, adh.error(err, scope)
	}

	var value interface{}
	if request.Filters == nil {
		value, err = adh.params.DynamicConfig.GetValue(keyVal)
		if err != nil {
			return nil, adh.error(err, scope)
		}
	} else {
		convFilters, err := convertFilterListToMap(request.Filters)
		if err != nil {
			return nil, adh.error(err, scope)
		}
		value, err = adh.params.DynamicConfig.GetValueWithFilters(keyVal, convFilters)
		if err != nil {
			return nil, adh.error(err, scope)
		}
	}

	data, err := json.Marshal(value)
	if err != nil {
		return nil, adh.error(err, scope)
	}

	return &types.GetDynamicConfigResponse{
		Value: &types.DataBlob{
			EncodingType: types.EncodingTypeJSON.Ptr(),
			Data:         data,
		},
	}, nil
}

func (adh *adminHandlerImpl) UpdateDynamicConfig(ctx context.Context, request *types.UpdateDynamicConfigRequest) (retError error) {
	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &retError) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.AdminUpdateDynamicConfigScope)
	defer sw.Stop()

	if request == nil || request.ConfigName == "" {
		return adh.error(validate.ErrRequestNotSet, scope)
	}

	keyVal, err := dc.GetKeyFromKeyName(request.ConfigName)
	if err != nil {
		return adh.error(err, scope)
	}

	return adh.params.DynamicConfig.UpdateValue(keyVal, request.ConfigValues)
}

func (adh *adminHandlerImpl) RestoreDynamicConfig(ctx context.Context, request *types.RestoreDynamicConfigRequest) (retError error) {
	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &retError) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.AdminRestoreDynamicConfigScope)
	defer sw.Stop()

	if request == nil || request.ConfigName == "" {
		return adh.error(validate.ErrRequestNotSet, scope)
	}

	keyVal, err := dc.GetKeyFromKeyName(request.ConfigName)
	if err != nil {
		return adh.error(err, scope)
	}

	var filters map[dc.Filter]interface{}

	if request.Filters == nil {
		filters = nil
	} else {
		filters, err = convertFilterListToMap(request.Filters)
		if err != nil {
			return adh.error(validate.ErrInvalidFilters, scope)
		}
	}
	return adh.params.DynamicConfig.RestoreValue(keyVal, filters)
}

func (adh *adminHandlerImpl) ListDynamicConfig(ctx context.Context, request *types.ListDynamicConfigRequest) (_ *types.ListDynamicConfigResponse, retError error) {
	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &retError) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.AdminListDynamicConfigScope)
	defer sw.Stop()

	if request == nil {
		return nil, adh.error(validate.ErrRequestNotSet, scope)
	}

	keyVal, err := dc.GetKeyFromKeyName(request.ConfigName)
	if err != nil || request.ConfigName == "" {
		entries, err2 := adh.params.DynamicConfig.ListValue(nil)
		if err2 != nil {
			return nil, adh.error(err2, scope)
		}
		return &types.ListDynamicConfigResponse{
			Entries: entries,
		}, nil
	}

	entries, err2 := adh.params.DynamicConfig.ListValue(keyVal)
	if err2 != nil {
		err = adh.error(err2, scope)
		return nil, adh.error(err, scope)
	}

	return &types.ListDynamicConfigResponse{
		Entries: entries,
	}, nil
}

func (adh *adminHandlerImpl) GetGlobalIsolationGroups(ctx context.Context, request *types.GetGlobalIsolationGroupsRequest) (_ *types.GetGlobalIsolationGroupsResponse, retError error) {
	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &retError) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.GetGlobalIsolationGroups)
	defer sw.Stop()

	if request == nil {
		return nil, adh.error(validate.ErrRequestNotSet, scope)
	}

	resp, err := adh.isolationGroups.GetGlobalState(ctx)
	if err != nil {
		return nil, adh.error(err, scope)
	}
	return resp, nil
}

func (adh *adminHandlerImpl) UpdateGlobalIsolationGroups(ctx context.Context, request *types.UpdateGlobalIsolationGroupsRequest) (_ *types.UpdateGlobalIsolationGroupsResponse, retError error) {
	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &retError) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.UpdateGlobalIsolationGroups)
	defer sw.Stop()
	if request == nil {
		return nil, adh.error(validate.ErrRequestNotSet, scope)
	}
	err := adh.isolationGroups.UpdateGlobalState(ctx, *request)
	if err != nil {
		return nil, adh.error(err, scope)
	}
	return &types.UpdateGlobalIsolationGroupsResponse{}, nil
}

func (adh *adminHandlerImpl) GetDomainIsolationGroups(ctx context.Context, request *types.GetDomainIsolationGroupsRequest) (_ *types.GetDomainIsolationGroupsResponse, retError error) {
	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &retError) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.GetDomainIsolationGroups)
	defer sw.Stop()

	if request == nil {
		return nil, adh.error(validate.ErrRequestNotSet, scope)
	}

	resp, err := adh.isolationGroups.GetDomainState(ctx, types.GetDomainIsolationGroupsRequest{Domain: request.Domain})
	if err != nil {
		return nil, adh.error(err, scope)
	}
	return resp, nil
}

func (adh *adminHandlerImpl) UpdateDomainIsolationGroups(ctx context.Context, request *types.UpdateDomainIsolationGroupsRequest) (_ *types.UpdateDomainIsolationGroupsResponse, retError error) {
	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &retError) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.UpdateDomainIsolationGroups)
	defer sw.Stop()
	if request == nil {
		return nil, adh.error(validate.ErrRequestNotSet, scope)
	}
	err := adh.isolationGroups.UpdateDomainState(ctx, *request)
	if err != nil {
		return nil, adh.error(err, scope)
	}
	return &types.UpdateDomainIsolationGroupsResponse{}, nil
}

func (adh *adminHandlerImpl) GetDomainAsyncWorkflowConfiguraton(ctx context.Context, request *types.GetDomainAsyncWorkflowConfiguratonRequest) (_ *types.GetDomainAsyncWorkflowConfiguratonResponse, retError error) {
	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &retError) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.GetDomainAsyncWorkflowConfiguraton)
	defer sw.Stop()
	if request == nil {
		return nil, adh.error(validate.ErrRequestNotSet, scope)
	}
	resp, err := adh.asyncWFQueueConfigs.GetConfiguraton(ctx, request)
	if err != nil {
		return nil, adh.error(err, scope)
	}
	return resp, nil
}

func (adh *adminHandlerImpl) UpdateDomainAsyncWorkflowConfiguraton(ctx context.Context, request *types.UpdateDomainAsyncWorkflowConfiguratonRequest) (_ *types.UpdateDomainAsyncWorkflowConfiguratonResponse, retError error) {
	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &retError) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.UpdateDomainAsyncWorkflowConfiguraton)
	defer sw.Stop()
	if request == nil {
		return nil, adh.error(validate.ErrRequestNotSet, scope)
	}
	resp, err := adh.asyncWFQueueConfigs.UpdateConfiguration(ctx, request)
	if err != nil {
		return nil, adh.error(err, scope)
	}
	return resp, nil
}

func (adh *adminHandlerImpl) UpdateTaskListPartitionConfig(ctx context.Context, request *types.UpdateTaskListPartitionConfigRequest) (_ *types.UpdateTaskListPartitionConfigResponse, retError error) {
	defer func() { log.CapturePanic(recover(), adh.GetLogger(), &retError) }()
	scope, sw := adh.startRequestProfile(ctx, metrics.UpdateTaskListPartitionConfig)
	defer sw.Stop()
	if request == nil {
		return nil, adh.error(validate.ErrRequestNotSet, scope)
	}
	domainID, err := adh.GetDomainCache().GetDomainID(request.Domain)
	if err != nil {
		return nil, adh.error(err, scope)
	}
	if request.TaskList == nil {
		return nil, adh.error(validate.ErrTaskListNotSet, scope)
	}
	if request.TaskList.Kind == nil {
		return nil, adh.error(&types.BadRequestError{Message: "Task list kind not set."}, scope)
	}
	if *request.TaskList.Kind != types.TaskListKindNormal {
		return nil, adh.error(&types.BadRequestError{Message: "Only normal tasklist's partition config can be updated."}, scope)
	}
	if request.TaskListType == nil {
		return nil, adh.error(&types.BadRequestError{Message: "Task list type not set."}, scope)
	}
	if request.PartitionConfig == nil {
		return nil, adh.error(&types.BadRequestError{Message: "Task list partition config is not set in the request."}, scope)
	}
	if len(request.PartitionConfig.WritePartitions) > len(request.PartitionConfig.ReadPartitions) {
		return nil, adh.error(&types.BadRequestError{Message: "The number of write partitions cannot be larger than the number of read partitions."}, scope)
	}
	if len(request.PartitionConfig.WritePartitions) <= 0 {
		return nil, adh.error(&types.BadRequestError{Message: "The number of partitions must be larger than 0."}, scope)
	}
	_, err = adh.GetMatchingClient().UpdateTaskListPartitionConfig(ctx, &types.MatchingUpdateTaskListPartitionConfigRequest{
		DomainUUID:      domainID,
		TaskList:        request.TaskList,
		TaskListType:    request.TaskListType,
		PartitionConfig: request.PartitionConfig,
	})
	if err != nil {
		return nil, err
	}
	return &types.UpdateTaskListPartitionConfigResponse{}, nil
}

func convertFromDataBlob(blob *types.DataBlob) (interface{}, error) {
	switch *blob.EncodingType {
	case types.EncodingTypeJSON:
		var v interface{}
		err := json.Unmarshal(blob.Data, &v)
		return v, err
	default:
		return nil, errors.New("unsupported blob encoding")
	}
}

func convertFilterListToMap(filters []*types.DynamicConfigFilter) (map[dc.Filter]interface{}, error) {
	newFilters := make(map[dc.Filter]interface{})

	for _, filter := range filters {
		val, err := convertFromDataBlob(filter.Value)
		if err != nil {
			return nil, err
		}
		newFilters[dc.ParseFilter(filter.Name)] = val
	}
	return newFilters, nil
}
