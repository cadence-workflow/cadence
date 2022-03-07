// Copyright (c) 2022 Uber Technologies, Inc.
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

package watchdog

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/cadence/.gen/go/cadence/workflowserviceclient"
	cclient "go.uber.org/cadence/client"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/types"
)

type (

	// Client is used to send request to processor workflow
	Client interface {
		ReportCorruptWorkflow(domainName string, workflowID string, runID string) error
	}

	clientImpl struct {
		logger        log.Logger
		cadenceClient cclient.Client
		processed     cache.Cache
	}
)

var _ Client = (*clientImpl)(nil)

const (
	SignalTimeout = 400 * time.Millisecond
)

// NewClient creates a new Client
func NewClient(
	logger log.Logger,
	publicClient workflowserviceclient.Interface,
) Client {
	cacheOpts := &cache.Options{
		InitialCapacity: 100,
		MaxCount:        1000,
		TTL:             24 * time.Hour,
		Pin:             false,
	}
	return &clientImpl{
		logger:        logger,
		cadenceClient: cclient.NewClient(publicClient, common.SystemLocalDomainName, &cclient.Options{}),
		processed:     cache.New(cacheOpts),
	}
}

func (c *clientImpl) getProcessedID(workflowID string, runID string) string {
	return fmt.Sprintf("%s-%s", workflowID, runID)
}

func (c *clientImpl) ReportCorruptWorkflow(
	domainName string,
	workflowID string,
	runID string,
) error {
	if c.processed.Get(c.getProcessedID(workflowID, runID)) != nil {
		// We already processed this workflow before and couldn't decide if we should delete
		return nil
	}

	// By putting the workflow into the cache we avoid spamming the workflow
	c.processed.Put(c.getProcessedID(workflowID, runID), struct{}{})

	signalCtx, cancel := context.WithTimeout(context.Background(), SignalTimeout)
	defer cancel()
	request := CorruptWFRequest{
		DomainName: domainName,
		Workflow: types.WorkflowExecution{
			WorkflowID: workflowID,
			RunID:      runID,
		},
	}
	return c.cadenceClient.SignalWorkflow(signalCtx, WatchdogWFID, "", CorruptWorkflowWatchdogChannelName, request)
}
