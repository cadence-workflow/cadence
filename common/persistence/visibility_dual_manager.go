// Copyright (c) 2021 Uber Technologies, Inc.
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
	"fmt"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
)

type (
	visibilityDualManager struct {
		logger                    log.Logger
		dbVisibilityManager       VisibilityManager
		advancedVisibilityManager VisibilityManager
		readVisibilityStoreName   dynamicconfig.StringPropertyFnWithDomainFilter
		writeMode                 dynamicconfig.StringPropertyFn
	}
)

var _ VisibilityManager = (*visibilityDualManager)(nil)

// NewVisibilityDualManager create a visibility manager that operate on DB or ElasticSearch based on dynamic config.
func NewVisibilityDualManager(
	dbVisibilityManager VisibilityManager, // one of the VisibilityManager can be nil
	advancedVisibilityManager VisibilityManager, // one of the VisibilityManager can be nil
	readVisibilityStoreName dynamicconfig.StringPropertyFnWithDomainFilter,
	visWritingMode dynamicconfig.StringPropertyFn,
	logger log.Logger,
) VisibilityManager {
	if dbVisibilityManager == nil && advancedVisibilityManager == nil {
		logger.Fatal("require one of dbVisibilityManager or advancedVisibilityManager, such as es or pinot")
		return nil
	}
	return &visibilityDualManager{
		dbVisibilityManager:       dbVisibilityManager,
		advancedVisibilityManager: advancedVisibilityManager,
		readVisibilityStoreName:   readVisibilityStoreName,
		writeMode:                 visWritingMode,
		logger:                    logger,
	}
}

func (v *visibilityDualManager) Close() {
	if v.dbVisibilityManager != nil {
		v.dbVisibilityManager.Close()
	}
	if v.advancedVisibilityManager != nil {
		v.advancedVisibilityManager.Close()
	}
}

func (v *visibilityDualManager) GetName() string {
	if v.advancedVisibilityManager != nil {
		return v.advancedVisibilityManager.GetName()
	}
	return v.dbVisibilityManager.GetName()
}

func (v *visibilityDualManager) RecordWorkflowExecutionStarted(
	ctx context.Context,
	request *RecordWorkflowExecutionStartedRequest,
) error {
	return v.chooseVisibilityManagerForWrite(
		ctx,
		func() error {
			return v.dbVisibilityManager.RecordWorkflowExecutionStarted(ctx, request)
		},
		func() error {
			return v.advancedVisibilityManager.RecordWorkflowExecutionStarted(ctx, request)
		},
	)
}

func (v *visibilityDualManager) RecordWorkflowExecutionClosed(
	ctx context.Context,
	request *RecordWorkflowExecutionClosedRequest,
) error {
	return v.chooseVisibilityManagerForWrite(
		ctx,
		func() error {
			return v.dbVisibilityManager.RecordWorkflowExecutionClosed(ctx, request)
		},
		func() error {
			return v.advancedVisibilityManager.RecordWorkflowExecutionClosed(ctx, request)
		},
	)
}

func (v *visibilityDualManager) RecordWorkflowExecutionUninitialized(
	ctx context.Context,
	request *RecordWorkflowExecutionUninitializedRequest,
) error {
	return v.chooseVisibilityManagerForWrite(
		ctx,
		func() error {
			return v.dbVisibilityManager.RecordWorkflowExecutionUninitialized(ctx, request)
		},
		func() error {
			return v.advancedVisibilityManager.RecordWorkflowExecutionUninitialized(ctx, request)
		},
	)
}

func (v *visibilityDualManager) DeleteWorkflowExecution(
	ctx context.Context,
	request *VisibilityDeleteWorkflowExecutionRequest,
) error {
	return v.chooseVisibilityManagerForWrite(
		ctx,
		func() error {
			return v.dbVisibilityManager.DeleteWorkflowExecution(ctx, request)
		},
		func() error {
			return v.advancedVisibilityManager.DeleteWorkflowExecution(ctx, request)
		},
	)
}

func (v *visibilityDualManager) DeleteUninitializedWorkflowExecution(
	ctx context.Context,
	request *VisibilityDeleteWorkflowExecutionRequest,
) error {
	return v.chooseVisibilityManagerForWrite(
		ctx,
		func() error {
			return v.dbVisibilityManager.DeleteUninitializedWorkflowExecution(ctx, request)
		},
		func() error {
			return v.advancedVisibilityManager.DeleteUninitializedWorkflowExecution(ctx, request)
		},
	)
}

func (v *visibilityDualManager) UpsertWorkflowExecution(
	ctx context.Context,
	request *UpsertWorkflowExecutionRequest,
) error {
	return v.chooseVisibilityManagerForWrite(
		ctx,
		func() error {
			return v.dbVisibilityManager.UpsertWorkflowExecution(ctx, request)
		},
		func() error {
			return v.advancedVisibilityManager.UpsertWorkflowExecution(ctx, request)
		},
	)
}

func (v *visibilityDualManager) chooseVisibilityModeForAdmin() string {
	switch {
	case v.dbVisibilityManager != nil && v.advancedVisibilityManager != nil:
		return common.AdvancedVisibilityWritingModeDual
	case v.advancedVisibilityManager != nil:
		return common.AdvancedVisibilityWritingModeOn
	case v.dbVisibilityManager != nil:
		return common.AdvancedVisibilityWritingModeOff
	default:
		return "INVALID_ADMIN_MODE"
	}
}

func (v *visibilityDualManager) chooseVisibilityManagerForWrite(ctx context.Context, dbVisFunc, advancedVisFunc func() error) error {
	var writeMode string
	if v.writeMode != nil {
		writeMode = v.writeMode()
	} else {
		key := VisibilityAdminDeletionKey("visibilityAdminDelete")
		if value := ctx.Value(key); value != nil && value.(bool) {
			writeMode = v.chooseVisibilityModeForAdmin()
		}
	}

	switch writeMode {
	case common.AdvancedVisibilityWritingModeOff:
		if v.dbVisibilityManager != nil {
			return dbVisFunc()
		}
		v.logger.Warn("basic visibility is not available to write, fall back to advanced visibility")
		return advancedVisFunc()
	case common.AdvancedVisibilityWritingModeOn:
		if v.advancedVisibilityManager != nil {
			return advancedVisFunc()
		}
		v.logger.Warn("advanced visibility is not available to write, fall back to basic visibility")
		return dbVisFunc()
	case common.AdvancedVisibilityWritingModeDual:
		if v.advancedVisibilityManager != nil {
			if err := advancedVisFunc(); err != nil {
				return err
			}
			if v.dbVisibilityManager != nil {
				return dbVisFunc()
			}
			v.logger.Warn("basic visibility is not available to write")
			return nil
		}
		v.logger.Warn("advanced visibility is not available to write")
		return dbVisFunc()
	default:
		return &types.InternalServiceError{
			Message: fmt.Sprintf("Unknown visibility writing mode: %s", writeMode),
		}
	}
}

func (v *visibilityDualManager) ListOpenWorkflowExecutions(
	ctx context.Context,
	request *ListWorkflowExecutionsRequest,
) (*ListWorkflowExecutionsResponse, error) {
	manager := v.chooseVisibilityManagerForRead(request.Domain)
	return manager.ListOpenWorkflowExecutions(ctx, request)
}

func (v *visibilityDualManager) ListClosedWorkflowExecutions(
	ctx context.Context,
	request *ListWorkflowExecutionsRequest,
) (*ListWorkflowExecutionsResponse, error) {
	manager := v.chooseVisibilityManagerForRead(request.Domain)
	return manager.ListClosedWorkflowExecutions(ctx, request)
}

func (v *visibilityDualManager) ListOpenWorkflowExecutionsByType(
	ctx context.Context,
	request *ListWorkflowExecutionsByTypeRequest,
) (*ListWorkflowExecutionsResponse, error) {
	manager := v.chooseVisibilityManagerForRead(request.Domain)
	return manager.ListOpenWorkflowExecutionsByType(ctx, request)
}

func (v *visibilityDualManager) ListClosedWorkflowExecutionsByType(
	ctx context.Context,
	request *ListWorkflowExecutionsByTypeRequest,
) (*ListWorkflowExecutionsResponse, error) {
	manager := v.chooseVisibilityManagerForRead(request.Domain)
	return manager.ListClosedWorkflowExecutionsByType(ctx, request)
}

func (v *visibilityDualManager) ListOpenWorkflowExecutionsByWorkflowID(
	ctx context.Context,
	request *ListWorkflowExecutionsByWorkflowIDRequest,
) (*ListWorkflowExecutionsResponse, error) {
	manager := v.chooseVisibilityManagerForRead(request.Domain)
	return manager.ListOpenWorkflowExecutionsByWorkflowID(ctx, request)
}

func (v *visibilityDualManager) ListClosedWorkflowExecutionsByWorkflowID(
	ctx context.Context,
	request *ListWorkflowExecutionsByWorkflowIDRequest,
) (*ListWorkflowExecutionsResponse, error) {
	manager := v.chooseVisibilityManagerForRead(request.Domain)
	return manager.ListClosedWorkflowExecutionsByWorkflowID(ctx, request)
}

func (v *visibilityDualManager) ListClosedWorkflowExecutionsByStatus(
	ctx context.Context,
	request *ListClosedWorkflowExecutionsByStatusRequest,
) (*ListWorkflowExecutionsResponse, error) {
	manager := v.chooseVisibilityManagerForRead(request.Domain)
	return manager.ListClosedWorkflowExecutionsByStatus(ctx, request)
}

func (v *visibilityDualManager) GetClosedWorkflowExecution(
	ctx context.Context,
	request *GetClosedWorkflowExecutionRequest,
) (*GetClosedWorkflowExecutionResponse, error) {
	manager := v.chooseVisibilityManagerForRead(request.Domain)
	return manager.GetClosedWorkflowExecution(ctx, request)
}

func (v *visibilityDualManager) ListWorkflowExecutions(
	ctx context.Context,
	request *ListWorkflowExecutionsByQueryRequest,
) (*ListWorkflowExecutionsResponse, error) {
	manager := v.chooseVisibilityManagerForRead(request.Domain)
	return manager.ListWorkflowExecutions(ctx, request)
}

func (v *visibilityDualManager) ScanWorkflowExecutions(
	ctx context.Context,
	request *ListWorkflowExecutionsByQueryRequest,
) (*ListWorkflowExecutionsResponse, error) {
	manager := v.chooseVisibilityManagerForRead(request.Domain)
	return manager.ScanWorkflowExecutions(ctx, request)
}

func (v *visibilityDualManager) CountWorkflowExecutions(
	ctx context.Context,
	request *CountWorkflowExecutionsRequest,
) (*CountWorkflowExecutionsResponse, error) {
	manager := v.chooseVisibilityManagerForRead(request.Domain)
	return manager.CountWorkflowExecutions(ctx, request)
}

func (v *visibilityDualManager) chooseVisibilityManagerForRead(domain string) VisibilityManager {
	var visibilityMgr VisibilityManager
	if v.readVisibilityStoreName(domain) != dbVisStoreName {
		if v.advancedVisibilityManager != nil {
			visibilityMgr = v.advancedVisibilityManager
		} else {
			visibilityMgr = v.dbVisibilityManager
			v.logger.Warn("domain is configured to read from advanced visibility(ElasticSearch/Pinot based) but it's not available, fall back to basic visibility",
				tag.WorkflowDomainName(domain))
		}
	} else {
		if v.dbVisibilityManager != nil {
			visibilityMgr = v.dbVisibilityManager
		} else {
			visibilityMgr = v.advancedVisibilityManager
			v.logger.Warn("domain is configured to read from basic visibility but it's not available, fall back to advanced visibility",
				tag.WorkflowDomainName(domain))
		}
	}
	return visibilityMgr
}
