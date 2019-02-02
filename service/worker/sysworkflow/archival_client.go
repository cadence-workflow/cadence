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

package sysworkflow

import (
	"context"
	"errors"
	"fmt"
	"github.com/uber-common/bark"
	"github.com/uber/cadence/client/public"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/service/dynamicconfig"
	"go.uber.org/cadence/client"
	"math/rand"
)

type (
	// ArchiveRequest is request to Archive
	ArchiveRequest struct {
		DomainID          string
		WorkflowID        string
		RunID             string
		EventStoreVersion int32
		BranchToken       []byte
		LastFirstEventID  int64
	}

	// BackfillRequest is request to Backfill
	BackfillRequest struct{}

	// ArchivalClient is used to archive workflow histories
	ArchivalClient interface {
		Archive(*ArchiveRequest) error
		Backfill(*BackfillRequest) error
	}

	archivalClient struct {
		cadenceClient client.Client
		numSWFn       dynamicconfig.IntPropertyFn
		logger        bark.Logger
		metricsClient metrics.Client
	}

	signal struct {
		RequestType    requestType
		ArchiveRequest *ArchiveRequest
		BackillRequest *BackfillRequest
	}
)

// NewArchivalClient creates a new ArchivalClient
func NewArchivalClient(
	publicClient public.Client,
	numSWFn dynamicconfig.IntPropertyFn,
	logger bark.Logger,
	metricsClient metrics.Client,
) ArchivalClient {
	return &archivalClient{
		cadenceClient: client.NewClient(publicClient, SystemDomainName, &client.Options{}),
		numSWFn:       numSWFn,
		logger:        logger,
		metricsClient: metricsClient,
	}
}

// Archive starts an archival task
func (c *archivalClient) Archive(request *ArchiveRequest) error {
	workflowID := fmt.Sprintf("%v-%v", workflowIDPrefix, rand.Intn(c.numSWFn()))
	workflowOptions := client.StartWorkflowOptions{
		ID:                              workflowID,
		TaskList:                        decisionTaskList,
		ExecutionStartToCloseTimeout:    workflowStartToCloseTimeout,
		DecisionTaskStartToCloseTimeout: decisionTaskStartToCloseTimeout,
		WorkflowIDReusePolicy:           client.WorkflowIDReusePolicyAllowDuplicate,
	}
	signal := signal{
		RequestType:    archivalRequest,
		ArchiveRequest: request,
	}
	// TODO: I should be passing logger and metrics to workflow here
	_, err := c.cadenceClient.SignalWithStartWorkflow(
		context.Background(),
		workflowID,
		signalName,
		signal,
		workflowOptions,
		systemWorkflowFnName,
	)
	return err
}

// Backfill starts a backfill task
func (c *archivalClient) Backfill(request *BackfillRequest) error {
	// TODO: implement this once backfill is supported
	return errors.New("not implemented")
}
