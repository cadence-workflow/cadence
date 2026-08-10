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
	"context"
	"errors"
	"fmt"

	"go.uber.org/cadence"

	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/worker/scheduler"
)

// CheckActivePollersActivity checks if there are any active pollers for the domain's task lists
// across all clusters. Returns a non-retryable error if pollers are found, preventing deprecation.
func (w *domainDeprecator) CheckActivePollersActivity(ctx context.Context, params DomainDeprecationParams) error {
	client := w.clientBean.GetFrontendClient()

	domainResp, err := client.DescribeDomain(ctx, &types.DescribeDomainRequest{
		Name: &params.DomainName,
	})
	if err != nil {
		var entityNotExistsError *types.EntityNotExistsError
		if errors.As(err, &entityNotExistsError) {
			return cadence.NewCustomError(ErrDomainDoesNotExistNonRetryable)
		}
		return fmt.Errorf("failed to describe domain: %v", err)
	}

	if domainResp.ReplicationConfiguration == nil || len(domainResp.ReplicationConfiguration.Clusters) == 0 {
		w.logger.Info("No replication configuration found for domain, skipping poller check",
			tag.WorkflowDomainName(params.DomainName))
		return nil
	}

	var activeTaskLists []string
	for _, cluster := range domainResp.ReplicationConfiguration.Clusters {
		clusterName := cluster.GetClusterName()
		frontendClient, err := w.clientBean.GetRemoteFrontendClient(clusterName)
		if err != nil {
			return fmt.Errorf("failed to get frontend client for cluster %s: %v", clusterName, err)
		}

		resp, err := frontendClient.GetTaskListsByDomain(ctx, &types.GetTaskListsByDomainRequest{
			Domain: params.DomainName,
		})
		if err != nil {
			return fmt.Errorf("failed to get task lists for domain %s in cluster %s: %v", params.DomainName, clusterName, err)
		}

		for name, tl := range resp.GetDecisionTaskListMap() {
			if name == scheduler.TaskListName {
				continue
			}
			if len(tl.GetPollers()) > 0 {
				activeTaskLists = append(activeTaskLists, fmt.Sprintf("%s (cluster: %s)", name, clusterName))
			}
		}
		for name, tl := range resp.GetActivityTaskListMap() {
			if name == scheduler.TaskListName {
				continue
			}
			if len(tl.GetPollers()) > 0 {
				activeTaskLists = append(activeTaskLists, fmt.Sprintf("%s (cluster: %s)", name, clusterName))
			}
		}
	}

	if len(activeTaskLists) > 0 {
		w.logger.Warn("Domain has active pollers",
			tag.WorkflowDomainName(params.DomainName),
			tag.Dynamic("active_task_lists", activeTaskLists))
		return cadence.NewCustomError(ErrActivePollersNonRetryable,
			fmt.Sprintf("domain %s has active pollers on task lists: %v, use Force to override", params.DomainName, activeTaskLists))
	}

	w.logger.Info("No active pollers found for domain", tag.WorkflowDomainName(params.DomainName))
	return nil
}

// DisableArchivalActivity disables archival for the domain
func (w *domainDeprecator) DisableArchivalActivity(ctx context.Context, params DomainDeprecationParams) error {
	client := w.clientBean.GetFrontendClient()
	disabled := types.ArchivalStatusDisabled

	describeRequest := &types.DescribeDomainRequest{
		Name: &params.DomainName,
	}
	domainResp, err := client.DescribeDomain(ctx, describeRequest)
	if err != nil {
		var entityNotExistsError *types.EntityNotExistsError
		if errors.As(err, &entityNotExistsError) {
			return cadence.NewCustomError(ErrDomainDoesNotExistNonRetryable)
		}
		return fmt.Errorf("failed to describe domain: %v", err)
	}

	// Check if archival is already disabled
	if *domainResp.Configuration.VisibilityArchivalStatus == disabled &&
		*domainResp.Configuration.HistoryArchivalStatus == disabled {
		w.logger.Info("Archival is already disabled for domain", tag.WorkflowDomainName(params.DomainName))
		return nil
	}

	updateRequest := &types.UpdateDomainRequest{
		Name:                     params.DomainName,
		HistoryArchivalStatus:    &disabled,
		VisibilityArchivalStatus: &disabled,
		SecurityToken:            params.SecurityToken,
	}
	updateResp, err := client.UpdateDomain(ctx, updateRequest)
	if err != nil {
		return fmt.Errorf("failed to update domain: %v", err)
	}

	if *updateResp.Configuration.VisibilityArchivalStatus != disabled ||
		*updateResp.Configuration.HistoryArchivalStatus != disabled {
		return fmt.Errorf("failed to disable archival for domain %s", params.DomainName)
	}

	w.logger.Info("Disabled archival for domain", tag.WorkflowDomainName(params.DomainName))
	return nil
}

// DeprecateDomainActivity deprecates the domain
func (w *domainDeprecator) DeprecateDomainActivity(ctx context.Context, params DomainDeprecationParams) error {
	client := w.clientBean.GetFrontendClient()

	err := client.DeprecateDomain(ctx, &types.DeprecateDomainRequest{
		Name:          params.DomainName,
		SecurityToken: params.SecurityToken,
	})
	if err != nil {
		return fmt.Errorf("failed to deprecate domain: %v", err)
	}

	return nil
}

// CheckOpenWorkflowsActivity checks if there are any open workflows in the domain
func (w *domainDeprecator) CheckOpenWorkflowsActivity(ctx context.Context, params DomainDeprecationParams) (bool, error) {
	client := w.clientBean.GetFrontendClient()

	countRequest := &types.CountWorkflowExecutionsRequest{
		Domain: params.DomainName,
		Query:  "CloseTime = missing",
	}

	countResp, err := client.CountWorkflowExecutions(ctx, countRequest)
	if err != nil {
		return false, fmt.Errorf("failed to count open workflows: %v", err)
	}

	hasOpenWorkflows := countResp.Count > 0
	if hasOpenWorkflows {
		w.logger.Info("Found open workflows in domain",
			tag.WorkflowDomainName(params.DomainName),
			tag.Number(countResp.Count))
	} else {
		w.logger.Info("No open workflows found in domain",
			tag.WorkflowDomainName(params.DomainName))
	}

	return hasOpenWorkflows, nil
}
