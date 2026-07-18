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

// Package taskvalidator provides a Work in Progress service for workflow validations.
package taskvalidator

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"

	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/entity"
	"github.com/uber/cadence/common/reconciliation/invariant"
	"github.com/uber/cadence/common/types"
)

// Checker interface for initiating the validation process.
type Checker interface {
	WorkflowCheckforValidation(ctx context.Context, workflowID string, domainID string, domainName string, runID string) error
}

// staleChecker interface definition.
type staleChecker interface {
	CheckAge(response *persistence.GetWorkflowExecutionResponse) (bool, error)
}

// checkerImpl is the implementation of the Checker interface.
type checkerImpl struct {
	logger        *zap.Logger
	metricsClient metrics.Client
	dc            cache.DomainCache
	pr            persistence.Retryer
	staleCheck    staleChecker
}

// NewWfChecker creates a new instance of a workflow validation checker.
// It requires a logger, metrics client, domain cache, persistence retryer,
// and a stale checker implementation to function.
func NewWfChecker(logger *zap.Logger, metrics metrics.Client, domainCache cache.DomainCache, executionManager persistence.ExecutionManager, historymanager persistence.HistoryManager) (Checker, error) {
	// Create the persistence retryer
	retryPolicy := backoff.NewExponentialRetryPolicy(100 * time.Millisecond) // Adjust as needed
	pr := persistence.NewPersistenceRetryer(executionManager, historymanager, retryPolicy)

	// Create the stale check instance
	staleCheckInstance := invariant.NewStaleWorkflow(pr, domainCache, logger)
	staleCheck, _ := staleCheckInstance.(staleChecker)

	// Return the checker implementation
	return &checkerImpl{
		logger:        logger,
		metricsClient: metrics,
		dc:            domainCache,
		pr:            pr,
		staleCheck:    staleCheck,
	}, nil
}

// WorkflowCheckforValidation performs workflow validation.
func (w *checkerImpl) WorkflowCheckforValidation(ctx context.Context, workflowID string, domainID string, domainName string, runID string) error {
	w.logger.Info("WorkflowCheckforValidation",
		zap.String("WorkflowID", workflowID),
		zap.String("RunID", runID),
		zap.String("DomainID", domainID),
		zap.String("DomainName", domainName))

	workflowResp, err := w.pr.GetWorkflowExecution(ctx, &persistence.GetWorkflowExecutionRequest{
		DomainID:   domainID,
		DomainName: domainName,
		Execution: types.WorkflowExecution{
			WorkflowID: workflowID,
			RunID:      runID,
		},
	})
	if err != nil {
		w.logger.Error("Error getting workflow execution", zap.Error(err))
		return err
	}

	// Create an instance of ConcreteExecution
	concreteExecution := &entity.ConcreteExecution{
		Execution: entity.Execution{
			DomainID:   workflowResp.State.ExecutionInfo.DomainID,
			WorkflowID: workflowResp.State.ExecutionInfo.WorkflowID,
			RunID:      workflowResp.State.ExecutionInfo.RunID,
			State:      workflowResp.State.ExecutionInfo.State,
		},
	}

	if w.isDomainDeprecated(domainID) {
		invariant.DeleteExecution(ctx, concreteExecution, w.pr, w.dc)
		return nil
	}

	if w.staleCheck != nil {
		stale, err := w.staleWorkflowCheck(workflowResp)
		if err != nil {
			w.logger.Error("Error checking if the execution is stale", zap.Error(err))
			return err
		}
		if stale {
			invariant.DeleteExecution(ctx, concreteExecution, w.pr, w.dc)
			return nil
		}
	}

	w.metricsClient.Scope(metrics.TaskValidatorScope, metrics.DomainTag(domainName)).IncCounter(metrics.ValidatedWorkflowCount)
	return nil
}

// isDomainDeprecated checks if a domain is deprecated.
func (w *checkerImpl) isDomainDeprecated(domainID string) bool {
	domain, err := w.dc.GetDomainByID(domainID)
	if err != nil {
		w.logger.Error("Error getting domain by ID", zap.Error(err))
		return false
	}
	return domain.IsDeprecatedOrDeleted()
}

// staleWorkflowCheck checks if a workflow is stale and returns any errors encountered.
func (w *checkerImpl) staleWorkflowCheck(workflowResp *persistence.GetWorkflowExecutionResponse) (bool, error) {
	if workflowResp == nil || workflowResp.State == nil || workflowResp.State.ExecutionInfo == nil {
		w.logger.Error("Invalid workflow execution response received")
		return false, errors.New("invalid workflow execution response")
	}
	pastExpiration, err := w.staleCheck.CheckAge(workflowResp)
	if err != nil {
		w.logger.Error("Error during stale workflow check", zap.Error(err))
		return false, err
	}
	return pastExpiration, nil
}
