// Copyright (c) 2024 Uber Technologies, Inc.
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
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package domaindeprecation

import (
	"time"

	"go.uber.org/cadence"
	"go.uber.org/cadence/workflow"
	"go.uber.org/zap"
)

const (
	domainDeprecationWorkflowTypeName = "domain-deprecation-workflow"
	domainDeprecationTaskListName     = "domain-deprecation-tasklist"

	disableArchivalActivity    = "disableArchival"
	deprecateDomainActivity    = "deprecateDomain"
	listWorkflowsActivity      = "listWorkflowsActivity"
	terminateWorkflowsActivity = "terminateWorkflowsActivity"
)

var (
	retryPolicy = cadence.RetryPolicy{
		InitialInterval:    10 * time.Second,
		BackoffCoefficient: 1.7,
		MaximumInterval:    5 * time.Minute,
		ExpirationInterval: 10 * time.Minute,
		NonRetriableErrorReasons: []string{
			ErrDomainDoesNotExistNonRetryable,
		},
	}

	activityOptions = workflow.ActivityOptions{
		ScheduleToStartTimeout: 5 * time.Minute,
		StartToCloseTimeout:    5 * time.Minute,
		HeartbeatTimeout:       5 * time.Minute,
		RetryPolicy:            &retryPolicy,
	}
)

// DomainDeprecationWorkflow is the workflow that handles domain deprecation process
func (w *domainDeprecator) DomainDeprecationWorkflow(ctx workflow.Context, domainName string) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting domain deprecation workflow", zap.String("domain", domainName))

	domainParams := DomainActivityParams{
		DomainName: domainName,
	}

	// Step 1: Activity disables archival
	err := workflow.ExecuteActivity(
		workflow.WithActivityOptions(ctx, activityOptions),
		w.DisableArchivalActivity,
		domainParams,
	).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Step 2: Deprecate a domain
	err = workflow.ExecuteActivity(
		workflow.WithActivityOptions(ctx, activityOptions),
		w.DeprecateDomainActivity,
		domainParams,
	).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Step 3: List open workflows of the domain
	var workflowDetails []WorkflowDetails
	err = workflow.ExecuteActivity(
		workflow.WithActivityOptions(ctx, activityOptions),
		w.ListOpenWorkflowsActivity,
		domainParams,
	).Get(ctx, &workflowDetails)
	if err != nil {
		return err
	}

	// Step 4: Terminate open workflows of the domain
	if len(workflowDetails) > 0 {
		err = workflow.ExecuteActivity(
			workflow.WithActivityOptions(ctx, activityOptions),
			w.TerminateWorkflowsActivity,
			TerminateWorkflowsActivityParams{
				DomainName:      domainName,
				WorkflowDetails: workflowDetails,
			},
		).Get(ctx, nil)
	}
	if err != nil {
		return err
	}

	logger.Info("Domain deprecation workflow completed successfully", zap.String("domain", domainName))
	return nil
}
