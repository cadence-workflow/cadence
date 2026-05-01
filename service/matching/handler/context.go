// Copyright (c) 2020 Uber Technologies, Inc.
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

package handler

import (
	"context"
	"errors"
	"sync"

	"github.com/uber/cadence/common"
	cadence_errors "github.com/uber/cadence/common/errors"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

type handlerContext struct {
	context.Context
	scope  metrics.Scope
	logger log.Logger
}

func newHandlerContext(
	ctx context.Context,
	domainName string,
	taskList *types.TaskList,
	metricsClient metrics.Client,
	metricsScope metrics.ScopeIdx,
	logger log.Logger,
) *handlerContext {
	return &handlerContext{
		Context: ctx,
		scope:   common.NewPerTaskListScope(domainName, taskList.GetName(), taskList.GetKind(), metricsClient, metricsScope).Tagged(metrics.GetContextTags(ctx)...),
		logger:  logger.WithTags(tag.WorkflowDomainName(domainName), tag.WorkflowTaskListName(taskList.GetName())),
	}
}

// startProfiling initiates recording of request metrics
func (reqCtx *handlerContext) startProfiling(wg *sync.WaitGroup) metrics.Stopwatch {
	wg.Wait()
	sw := reqCtx.scope.StartTimer(metrics.CadenceLatencyPerTaskList)
	reqCtx.scope.IncCounter(metrics.CadenceRequestsPerTaskList)
	return sw
}

func (reqCtx *handlerContext) handleErr(err error) error {
	if err == nil {
		return nil
	}

	logger := reqCtx.logger.Helper()

	switch {
	case errors.As(err, new(*types.InternalServiceError)):
		reqCtx.scope.IncCounter(metrics.CadenceFailuresPerTaskList)
		logger.Error("Internal service error", tag.Error(err))
		return err
	case errors.As(err, new(*types.BadRequestError)):
		reqCtx.scope.IncCounter(metrics.CadenceErrBadRequestPerTaskListCounter)
		return err
	case errors.As(err, new(*types.EntityNotExistsError)):
		reqCtx.scope.IncCounter(metrics.CadenceErrEntityNotExistsPerTaskListCounter)
		return err
	case errors.As(err, new(*types.WorkflowExecutionAlreadyStartedError)):
		reqCtx.scope.IncCounter(metrics.CadenceErrExecutionAlreadyStartedPerTaskListCounter)
		return err
	case errors.As(err, new(*types.DomainAlreadyExistsError)):
		reqCtx.scope.IncCounter(metrics.CadenceErrDomainAlreadyExistsPerTaskListCounter)
		return err
	case errors.As(err, new(*types.QueryFailedError)):
		reqCtx.scope.IncCounter(metrics.CadenceErrQueryFailedPerTaskListCounter)
		return err
	case errors.As(err, new(*types.LimitExceededError)):
		reqCtx.scope.IncCounter(metrics.CadenceErrLimitExceededPerTaskListCounter)
		return err
	case errors.As(err, new(*types.ServiceBusyError)):
		reqCtx.scope.IncCounter(metrics.CadenceErrServiceBusyPerTaskListCounter)
		return err
	case errors.As(err, new(*types.DomainNotActiveError)):
		reqCtx.scope.IncCounter(metrics.CadenceErrDomainNotActivePerTaskListCounter)
		return err
	case errors.As(err, new(*types.RemoteSyncMatchedError)):
		reqCtx.scope.IncCounter(metrics.CadenceErrRemoteSyncMatchFailedPerTaskListCounter)
		return err
	case errors.As(err, new(*types.StickyWorkerUnavailableError)):
		reqCtx.scope.IncCounter(metrics.CadenceErrStickyWorkerUnavailablePerTaskListCounter)
		return err
	case errors.As(err, new(*types.ReadOnlyPartitionError)):
		reqCtx.scope.IncCounter(metrics.CadenceErrReadOnlyPartitionPerTaskListCounter)
		return err
	case errors.As(err, new(*cadence_errors.TaskListNotOwnedByHostError)):
		reqCtx.scope.IncCounter(metrics.CadenceErrTaskListNotOwnedByHostPerTaskListCounter)
		return err
	default:
		reqCtx.scope.IncCounter(metrics.CadenceFailuresPerTaskList)
		logger.Error("Uncategorized error", tag.Error(err))
		return &types.InternalServiceError{Message: err.Error()}
	}
}
