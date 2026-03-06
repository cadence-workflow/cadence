// Copyright (c) 2026 Uber Technologies, Inc.
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

package scheduler

import (
	"context"

	"go.uber.org/cadence/.gen/go/cadence/workflowserviceclient"
	"go.uber.org/cadence/worker"
	"go.uber.org/cadence/workflow"

	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
)

type (
	// BootstrapParams contains the parameters needed to create a scheduler worker.
	BootstrapParams struct {
		ServiceClient  workflowserviceclient.Interface
		FrontendClient frontend.Client
		Logger         log.Logger
	}

	// Scheduler is the background worker that polls the scheduler task list
	// and executes schedule workflows.
	Scheduler struct {
		logger log.Logger
		worker worker.Worker
	}
)

// New creates a new Scheduler worker instance.
// It registers the scheduler workflow and creates a Cadence SDK worker
// for the scheduler task list in the system domain.
func New(params *BootstrapParams) *Scheduler {
	logger := params.Logger.WithTags(tag.ComponentScheduler)

	actCtx := context.WithValue(context.Background(), schedulerContextKey, schedulerContext{
		FrontendClient: params.FrontendClient,
	})
	wo := worker.Options{
		BackgroundActivityContext: actCtx,
	}
	w := worker.New(params.ServiceClient, constants.SystemLocalDomainName, TaskListName, wo)
	w.RegisterWorkflowWithOptions(SchedulerWorkflow, workflow.RegisterOptions{Name: WorkflowTypeName})

	return &Scheduler{
		logger: logger,
		worker: w,
	}
}

// Start begins polling the scheduler task list.
func (s *Scheduler) Start() error {
	s.logger.Info("scheduler worker starting")
	if err := s.worker.Start(); err != nil {
		s.worker.Stop()
		return err
	}
	s.logger.Info("scheduler worker started")
	return nil
}

// Stop stops the scheduler worker.
func (s *Scheduler) Stop() {
	s.logger.Info("scheduler worker stopping")
	s.worker.Stop()
	s.logger.Info("scheduler worker stopped")
}
