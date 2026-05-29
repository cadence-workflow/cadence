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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination request_validator_mock.go -self_package github.com/uber/cadence/service/frontend/api requestValidator

package api

import (
	"context"
	"fmt"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/frontend/config"
	"github.com/uber/cadence/service/frontend/validate"
)

type (
	RequestValidator interface {
		ValidateRefreshWorkflowTasksRequest(context.Context, *types.RefreshWorkflowTasksRequest) error
		ValidateDescribeTaskListRequest(context.Context, *types.DescribeTaskListRequest) error
		ValidateListTaskListPartitionsRequest(context.Context, *types.ListTaskListPartitionsRequest) error
		ValidateGetTaskListsByDomainRequest(context.Context, *types.GetTaskListsByDomainRequest) error
		ValidateResetStickyTaskListRequest(context.Context, *types.ResetStickyTaskListRequest) error
		ValidateCountWorkflowExecutionsRequest(context.Context, *types.CountWorkflowExecutionsRequest) error
		ValidateListWorkflowExecutionsRequest(context.Context, *types.ListWorkflowExecutionsRequest) error
		ValidateListOpenWorkflowExecutionsRequest(context.Context, *types.ListOpenWorkflowExecutionsRequest) error
		ValidateListArchivedWorkflowExecutionsRequest(context.Context, *types.ListArchivedWorkflowExecutionsRequest) error
		ValidateListClosedWorkflowExecutionsRequest(context.Context, *types.ListClosedWorkflowExecutionsRequest) error
		ValidateRegisterDomainRequest(context.Context, *types.RegisterDomainRequest) error
		ValidateDescribeDomainRequest(context.Context, *types.DescribeDomainRequest) error
		ValidateUpdateDomainRequest(context.Context, *types.UpdateDomainRequest) error
		ValidateDeleteDomainRequest(context.Context, *types.DeleteDomainRequest) error
		ValidateDeprecateDomainRequest(context.Context, *types.DeprecateDomainRequest) error
		ValidateFailoverDomainRequest(context.Context, *types.FailoverDomainRequest) error
	}

	requestValidatorImpl struct {
		logger        log.Logger
		metricsClient metrics.Client
		config        *config.Config
	}
)

func NewRequestValidator(logger log.Logger, metricsClient metrics.Client, config *config.Config) RequestValidator {
	return &requestValidatorImpl{
		logger:        logger,
		metricsClient: metricsClient,
		config:        config,
	}
}

func (v *requestValidatorImpl) validateTaskList(t *types.TaskList, scope metrics.Scope, domain string) error {
	if t == nil || t.GetName() == "" {
		return validate.ErrTaskListNotSet
	}
	if !common.IsValidIDLength(
		t.GetName(),
		scope,
		v.config.MaxIDLengthWarnLimit(),
		v.config.TaskListNameMaxLength(domain),
		metrics.CadenceErrTaskListNameExceededWarnLimit,
		domain,
		v.logger,
		tag.IDTypeTaskListName) {
		return validate.ErrTaskListTooLong
	}
	return nil
}

func checkRequiredDomainDataKVs(requiredDomainDataKeys map[string]interface{}, domainData map[string]string) error {
	// check requiredDomainDataKeys
	for k := range requiredDomainDataKeys {
		_, ok := domainData[k]
		if !ok {
			return fmt.Errorf("domain data error, missing required key %v . All required keys: %v", k, requiredDomainDataKeys)
		}
	}
	return nil
}

func checkFailOverPermission(config *config.Config, domainName string) error {
	if config.Lockdown(domainName) {
		return validate.ErrDomainInLockdown
	}
	return nil
}

func (v *requestValidatorImpl) isListRequestPageSizeTooLarge(pageSize int32, domain string) bool {
	return common.IsAdvancedVisibilityReadingEnabled(v.config.ReadVisibilityStoreName(domain) != "db", v.config.IsAdvancedVisConfigExist) &&
		pageSize > int32(v.config.ESIndexMaxResultWindow())
}

func (v *requestValidatorImpl) ValidateRefreshWorkflowTasksRequest(ctx context.Context, req *types.RefreshWorkflowTasksRequest) error {
	if req == nil {
		return validate.ErrRequestNotSet
	}
	return validate.CheckExecution(req.Execution)
}

func (v *requestValidatorImpl) ValidateDescribeTaskListRequest(ctx context.Context, request *types.DescribeTaskListRequest) error {
	if request == nil {
		return validate.ErrRequestNotSet
	}
	if request.GetDomain() == "" {
		return validate.ErrDomainNotSet
	}
	if request.TaskListType == nil {
		return validate.ErrTaskListTypeNotSet
	}
	scope := getMetricsScopeWithDomain(metrics.FrontendDescribeTaskListScope, request, v.metricsClient).Tagged(metrics.GetContextTags(ctx)...)
	return v.validateTaskList(request.TaskList, scope, request.GetDomain())
}

func (v *requestValidatorImpl) ValidateListTaskListPartitionsRequest(ctx context.Context, request *types.ListTaskListPartitionsRequest) error {
	if request == nil {
		return validate.ErrRequestNotSet
	}
	if request.GetDomain() == "" {
		return validate.ErrDomainNotSet
	}
	scope := getMetricsScopeWithDomain(metrics.FrontendListTaskListPartitionsScope, request, v.metricsClient).Tagged(metrics.GetContextTags(ctx)...)
	return v.validateTaskList(request.TaskList, scope, request.GetDomain())
}

func (v *requestValidatorImpl) ValidateGetTaskListsByDomainRequest(ctx context.Context, request *types.GetTaskListsByDomainRequest) error {
	if request == nil {
		return validate.ErrRequestNotSet
	}
	if request.GetDomain() == "" {
		return validate.ErrDomainNotSet
	}
	return nil
}

func (v *requestValidatorImpl) ValidateResetStickyTaskListRequest(ctx context.Context, resetRequest *types.ResetStickyTaskListRequest) error {
	if resetRequest == nil {
		return validate.ErrRequestNotSet
	}
	domainName := resetRequest.GetDomain()
	if domainName == "" {
		return validate.ErrDomainNotSet
	}
	wfExecution := resetRequest.GetExecution()
	return validate.CheckExecution(wfExecution)
}

func (v *requestValidatorImpl) ValidateCountWorkflowExecutionsRequest(ctx context.Context, countRequest *types.CountWorkflowExecutionsRequest) error {
	if countRequest == nil {
		return validate.ErrRequestNotSet
	}
	if countRequest.GetDomain() == "" {
		return validate.ErrDomainNotSet
	}
	return nil
}

func (v *requestValidatorImpl) ValidateListWorkflowExecutionsRequest(ctx context.Context, listRequest *types.ListWorkflowExecutionsRequest) error {
	if listRequest == nil {
		return validate.ErrRequestNotSet
	}
	if listRequest.GetDomain() == "" {
		return validate.ErrDomainNotSet
	}
	if listRequest.GetPageSize() <= 0 {
		listRequest.PageSize = int32(v.config.VisibilityMaxPageSize(listRequest.GetDomain()))
	}
	if v.isListRequestPageSizeTooLarge(listRequest.GetPageSize(), listRequest.GetDomain()) {
		return &types.BadRequestError{Message: fmt.Sprintf("Pagesize is larger than allow %d", v.config.ESIndexMaxResultWindow())}
	}
	return nil
}

func (v *requestValidatorImpl) ValidateListOpenWorkflowExecutionsRequest(ctx context.Context, listRequest *types.ListOpenWorkflowExecutionsRequest) error {
	if listRequest == nil {
		return validate.ErrRequestNotSet
	}
	if listRequest.GetDomain() == "" {
		return validate.ErrDomainNotSet
	}
	if listRequest.StartTimeFilter == nil {
		return &types.BadRequestError{Message: "StartTimeFilter is required"}
	}
	if listRequest.StartTimeFilter.EarliestTime == nil {
		return &types.BadRequestError{Message: "EarliestTime in StartTimeFilter is required"}
	}
	if listRequest.StartTimeFilter.LatestTime == nil {
		return &types.BadRequestError{Message: "LatestTime in StartTimeFilter is required"}
	}
	if listRequest.StartTimeFilter.GetEarliestTime() > listRequest.StartTimeFilter.GetLatestTime() {
		return &types.BadRequestError{Message: "EarliestTime in StartTimeFilter should not be larger than LatestTime"}
	}
	if listRequest.ExecutionFilter != nil && listRequest.TypeFilter != nil {
		return &types.BadRequestError{Message: "Only one of ExecutionFilter or TypeFilter is allowed"}
	}
	if listRequest.GetMaximumPageSize() <= 0 {
		listRequest.MaximumPageSize = int32(v.config.VisibilityMaxPageSize(listRequest.GetDomain()))
	}
	if v.isListRequestPageSizeTooLarge(listRequest.GetMaximumPageSize(), listRequest.GetDomain()) {
		return &types.BadRequestError{Message: fmt.Sprintf("Pagesize is larger than allow %d", v.config.ESIndexMaxResultWindow())}
	}
	return nil
}

func (v *requestValidatorImpl) ValidateListArchivedWorkflowExecutionsRequest(ctx context.Context, listRequest *types.ListArchivedWorkflowExecutionsRequest) error {
	if listRequest == nil {
		return validate.ErrRequestNotSet
	}
	if listRequest.GetDomain() == "" {
		return validate.ErrDomainNotSet
	}
	if listRequest.GetPageSize() <= 0 {
		listRequest.PageSize = int32(v.config.VisibilityMaxPageSize(listRequest.GetDomain()))
	}
	maxPageSize := v.config.VisibilityArchivalQueryMaxPageSize()
	if int(listRequest.GetPageSize()) > maxPageSize {
		return &types.BadRequestError{Message: fmt.Sprintf("Pagesize is larger than allowed %d", maxPageSize)}
	}
	return nil
}

func (v *requestValidatorImpl) ValidateListClosedWorkflowExecutionsRequest(ctx context.Context, listRequest *types.ListClosedWorkflowExecutionsRequest) error {
	if listRequest == nil {
		return validate.ErrRequestNotSet
	}
	if listRequest.GetDomain() == "" {
		return validate.ErrDomainNotSet
	}
	if listRequest.StartTimeFilter == nil {
		return &types.BadRequestError{Message: "StartTimeFilter is required"}
	}
	if listRequest.StartTimeFilter.EarliestTime == nil {
		return &types.BadRequestError{Message: "EarliestTime in StartTimeFilter is required"}
	}
	if listRequest.StartTimeFilter.LatestTime == nil {
		return &types.BadRequestError{Message: "LatestTime in StartTimeFilter is required"}
	}
	if listRequest.StartTimeFilter.GetEarliestTime() > listRequest.StartTimeFilter.GetLatestTime() {
		return &types.BadRequestError{Message: "EarliestTime in StartTimeFilter should not be larger than LatestTime"}
	}
	filterCount := 0
	if listRequest.ExecutionFilter != nil {
		filterCount++
	}
	if listRequest.TypeFilter != nil {
		filterCount++
	}
	if listRequest.StatusFilter != nil {
		filterCount++
	}
	if filterCount > 1 {
		return &types.BadRequestError{Message: "Only one of ExecutionFilter, TypeFilter or StatusFilter is allowed"}
	} // If ExecutionFilter is provided with one of TypeFilter or StatusFilter, use ExecutionFilter and ignore other filter
	if listRequest.GetMaximumPageSize() <= 0 {
		listRequest.MaximumPageSize = int32(v.config.VisibilityMaxPageSize(listRequest.GetDomain()))
	}
	if v.isListRequestPageSizeTooLarge(listRequest.GetMaximumPageSize(), listRequest.GetDomain()) {
		return &types.BadRequestError{Message: fmt.Sprintf("Pagesize is larger than allow %d", v.config.ESIndexMaxResultWindow())}
	}
	return nil
}

func (v *requestValidatorImpl) ValidateRegisterDomainRequest(ctx context.Context, registerRequest *types.RegisterDomainRequest) error {
	if registerRequest == nil {
		return validate.ErrRequestNotSet
	}
	if registerRequest.GetName() == "" {
		return validate.ErrDomainNotSet
	}
	domain := registerRequest.GetName()
	scope := v.metricsClient.Scope(metrics.FrontendRegisterDomainScope).Tagged(metrics.DomainTag(domain)).Tagged(metrics.GetContextTags(ctx)...)
	if !common.IsValidIDLength(
		domain,
		scope,
		v.config.MaxIDLengthWarnLimit(),
		v.config.DomainNameMaxLength(domain),
		metrics.CadenceErrTaskListNameExceededWarnLimit,
		domain,
		v.logger,
		tag.IDTypeDomainName) {
		return validate.ErrDomainTooLong
	}
	if registerRequest.GetWorkflowExecutionRetentionPeriodInDays() > int32(v.config.DomainConfig.MaxRetentionDays()) {
		return validate.ErrInvalidRetention
	}
	if err := checkRequiredDomainDataKVs(v.config.DomainConfig.RequiredDomainDataKeys(), registerRequest.GetData()); err != nil {
		return err
	}
	return validate.CheckPermission(v.config, registerRequest.SecurityToken)
}

func (v *requestValidatorImpl) ValidateDescribeDomainRequest(ctx context.Context, describeRequest *types.DescribeDomainRequest) error {
	if describeRequest == nil {
		return validate.ErrRequestNotSet
	}
	if describeRequest.GetName() == "" && describeRequest.GetUUID() == "" {
		return validate.ErrDomainNotSet
	}
	return nil
}

func (v *requestValidatorImpl) ValidateUpdateDomainRequest(ctx context.Context, updateRequest *types.UpdateDomainRequest) error {
	if updateRequest == nil {
		return validate.ErrRequestNotSet
	}
	if updateRequest.GetName() == "" {
		return validate.ErrDomainNotSet
	}
	if updateRequest.WorkflowExecutionRetentionPeriodInDays != nil && *updateRequest.WorkflowExecutionRetentionPeriodInDays > int32(v.config.DomainConfig.MaxRetentionDays()) {
		return validate.ErrInvalidRetention
	}
	isFailover := isFailoverRequest(updateRequest)
	// don't require permission for failover request
	if isFailover {
		// reject the failover if the cluster is in lockdown
		if err := checkFailOverPermission(v.config, updateRequest.GetName()); err != nil {
			return err
		}
	} else {
		if err := validate.CheckPermission(v.config, updateRequest.SecurityToken); err != nil {
			return err
		}
	}
	return nil
}

func (v *requestValidatorImpl) ValidateDeleteDomainRequest(ctx context.Context, deleteRequest *types.DeleteDomainRequest) error {
	if deleteRequest == nil {
		return validate.ErrRequestNotSet
	}
	if deleteRequest.GetName() == "" {
		return validate.ErrDomainNotSet
	}
	return validate.CheckPermission(v.config, deleteRequest.SecurityToken)
}

func (v *requestValidatorImpl) ValidateDeprecateDomainRequest(ctx context.Context, deprecateRequest *types.DeprecateDomainRequest) error {
	if deprecateRequest == nil {
		return validate.ErrRequestNotSet
	}
	if deprecateRequest.GetName() == "" {
		return validate.ErrDomainNotSet
	}
	return validate.CheckPermission(v.config, deprecateRequest.SecurityToken)
}

func (v *requestValidatorImpl) ValidateFailoverDomainRequest(ctx context.Context, failoverDomainRequest *types.FailoverDomainRequest) error {
	if failoverDomainRequest == nil {
		return validate.ErrRequestNotSet
	}
	if failoverDomainRequest.GetDomainName() == "" {
		return validate.ErrDomainNotSet
	}

	if failoverDomainRequest.DomainActiveClusterName == nil && failoverDomainRequest.ActiveClusters == nil {
		return &types.BadRequestError{Message: "DomainActiveClusterName or ActiveClusters must be provided to failover the domain"}
	}

	// Security token is not required for failover request - reject the failover if the cluster is in lockdown
	return checkFailOverPermission(v.config, failoverDomainRequest.GetDomainName())
}
