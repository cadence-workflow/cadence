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

package archiver

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"go.uber.org/cadence/.gen/go/cadence/workflowserviceclient"
	cclient "go.uber.org/cadence/client"

	carchiver "github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/archiver/provider"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/quotas"
	"github.com/uber/cadence/common/types"
)

//go:generate mockgen -package=$GOPACKAGE -destination=client_mock.go -self_package=github.com/uber/cadence/service/worker/archiver github.com/uber/cadence/service/worker/archiver Client

type (
	// ClientRequest is the archive request sent to the archiver client
	ClientRequest struct {
		ArchiveRequest       *ArchiveRequest
		CallerService        string
		AttemptArchiveInline bool
	}

	// ClientResponse is the archive response returned from the archiver client
	ClientResponse struct {
		HistoryArchivedInline bool
	}

	// ArchiveRequest is the request signal sent to the archival workflow
	ArchiveRequest struct {
		DomainID   string
		DomainName string
		WorkflowID string
		RunID      string

		// history archival
		ShardID              int
		BranchToken          []byte
		NextEventID          int64
		CloseFailoverVersion int64
		URI                  string // should be historyURI, but keep the existing name for backward compatibility

		// visibility archival
		WorkflowTypeName   string
		StartTimestamp     int64
		ExecutionTimestamp int64
		CloseTimestamp     int64
		CloseStatus        types.WorkflowExecutionCloseStatus
		HistoryLength      int64
		Memo               *types.Memo
		SearchAttributes   map[string][]byte
		VisibilityURI      string

		// archival targets: history and/or visibility
		Targets []ArchivalTarget
	}

	// Client is used to archive workflow histories
	Client interface {
		Archive(context.Context, *ClientRequest) (*ClientResponse, error)
	}

	client struct {
		metricsScope                metrics.Scope
		logger                      log.Logger
		cadenceClient               cclient.Client
		numWorkflows                dynamicproperties.IntPropertyFn
		rateLimiter                 quotas.Limiter
		inlineHistoryRateLimiter    quotas.Limiter
		inlineVisibilityRateLimiter quotas.Limiter
		archiverProvider            provider.ArchiverProvider
		archivingIncompleteHistory  dynamicproperties.BoolPropertyFn
	}

	// ArchivalTarget is either history or visibility
	ArchivalTarget int
)

const (
	signalTimeout = 300 * time.Millisecond

	tooManyRequestsErrMsg = "too many requests to archival workflow"
)

const (
	// ArchiveTargetHistory is the archive target for workflow history
	ArchiveTargetHistory ArchivalTarget = iota
	// ArchiveTargetVisibility is the archive target for workflow visibility record
	ArchiveTargetVisibility
)

var (
	errInlineArchivalThrottled = errors.New("inline archival throttled")
)

// NewClient creates a new Client
func NewClient(
	metricsClient metrics.Client,
	logger log.Logger,
	publicClient workflowserviceclient.Interface,
	numWorkflows dynamicproperties.IntPropertyFn,
	requestRateLimiter quotas.Limiter,
	inlineHistoryRateLimiter quotas.Limiter,
	inlineVisibilityRateLimiter quotas.Limiter,
	archiverProvider provider.ArchiverProvider,
	archivingIncompleteHistory dynamicproperties.BoolPropertyFn,
) Client {
	return &client{
		metricsScope:                metricsClient.Scope(metrics.ArchiverClientScope),
		logger:                      logger,
		cadenceClient:               cclient.NewClient(publicClient, constants.SystemLocalDomainName, &cclient.Options{}),
		numWorkflows:                numWorkflows,
		rateLimiter:                 requestRateLimiter,
		inlineHistoryRateLimiter:    inlineHistoryRateLimiter,
		inlineVisibilityRateLimiter: inlineVisibilityRateLimiter,
		archiverProvider:            archiverProvider,
		archivingIncompleteHistory:  archivingIncompleteHistory,
	}
}

// Archive starts an archival task
func (c *client) Archive(ctx context.Context, request *ClientRequest) (*ClientResponse, error) {
	scopeWithDomainTag := c.metricsScope.Tagged(metrics.DomainTag(request.ArchiveRequest.DomainName))
	for _, target := range request.ArchiveRequest.Targets {
		switch target {
		case ArchiveTargetHistory:
			scopeWithDomainTag.IncCounter(metrics.ArchiverClientHistoryRequestCountPerDomain)
		case ArchiveTargetVisibility:
			scopeWithDomainTag.IncCounter(metrics.ArchiverClientVisibilityRequestCountPerDomain)
		}
	}
	logger := c.logger.WithTags(
		tag.ArchivalCallerServiceName(request.CallerService),
		tag.ArchivalArchiveAttemptedInline(request.AttemptArchiveInline),
	)
	resp := &ClientResponse{
		HistoryArchivedInline: false,
	}
	if request.AttemptArchiveInline {
		results := []chan error{}
		for _, target := range request.ArchiveRequest.Targets {
			ch := make(chan error)
			results = append(results, ch)
			switch target {
			case ArchiveTargetHistory:
				go c.archiveHistoryInline(ctx, request, logger, ch)
			case ArchiveTargetVisibility:
				go c.archiveVisibilityInline(ctx, request, logger, ch)
			default:
				close(ch)
			}
		}

		targets := []ArchivalTarget{}
		for i, target := range request.ArchiveRequest.Targets {
			if <-results[i] != nil {
				targets = append(targets, target)
			} else if target == ArchiveTargetHistory {
				resp.HistoryArchivedInline = true
			}
		}
		request.ArchiveRequest.Targets = targets
	}
	if len(request.ArchiveRequest.Targets) != 0 {
		if err := c.sendArchiveSignal(ctx, request.ArchiveRequest, logger); err != nil {
			return nil, err
		}
	}
	return resp, nil
}

func (c *client) archiveHistoryInline(ctx context.Context, request *ClientRequest, logger log.Logger, errCh chan error) {
	logger = tagLoggerWithHistoryRequest(logger, request.ArchiveRequest)
	scopeWithDomainTag := c.metricsScope.Tagged(metrics.DomainTag(request.ArchiveRequest.DomainName))
	if !c.inlineHistoryRateLimiter.Allow() {
		scopeWithDomainTag.IncCounter(metrics.ArchiverClientHistoryInlineArchiveThrottledCountPerDomain)
		logger.Debug("inline history archival throttled")
		errCh <- errInlineArchivalThrottled
		return
	}

	var err error
	defer func() {
		if err != nil {
			scopeWithDomainTag.IncCounter(metrics.ArchiverClientHistoryInlineArchiveFailureCountPerDomain)
			logger.Info("failed to perform workflow history archival inline", tag.Error(err))
		}
		errCh <- err
	}()
	scopeWithDomainTag.IncCounter(metrics.ArchiverClientHistoryInlineArchiveAttemptCountPerDomain)
	URI, err := carchiver.NewURI(request.ArchiveRequest.URI)
	if err != nil {
		return
	}

	historyArchiver, err := c.archiverProvider.GetHistoryArchiver(URI.Scheme(), request.CallerService)
	if err != nil {
		return
	}

	allowArchivingIncompleteHistoryOpt := carchiver.GetArchivingIncompleteHistoryOption(c.archivingIncompleteHistory)
	err = historyArchiver.Archive(ctx, URI, &carchiver.ArchiveHistoryRequest{
		ShardID:              request.ArchiveRequest.ShardID,
		DomainID:             request.ArchiveRequest.DomainID,
		DomainName:           request.ArchiveRequest.DomainName,
		WorkflowID:           request.ArchiveRequest.WorkflowID,
		RunID:                request.ArchiveRequest.RunID,
		BranchToken:          request.ArchiveRequest.BranchToken,
		NextEventID:          request.ArchiveRequest.NextEventID,
		CloseFailoverVersion: request.ArchiveRequest.CloseFailoverVersion,
	}, allowArchivingIncompleteHistoryOpt)
}

func (c *client) archiveVisibilityInline(ctx context.Context, request *ClientRequest, logger log.Logger, errCh chan error) {
	logger = tagLoggerWithVisibilityRequest(logger, request.ArchiveRequest)
	scopeWithDomainTag := c.metricsScope.Tagged(metrics.DomainTag(request.ArchiveRequest.DomainName))
	if !c.inlineVisibilityRateLimiter.Allow() {
		scopeWithDomainTag.IncCounter(metrics.ArchiverClientVisibilityInlineArchiveThrottledCountPerDomain)
		logger.Debug("inline visibility archival throttled")
		errCh <- errInlineArchivalThrottled
		return
	}

	var err error
	defer func() {
		if err != nil {
			scopeWithDomainTag.IncCounter(metrics.ArchiverClientVisibilityInlineArchiveFailureCountPerDomain)
			logger.Info("failed to perform visibility archival inline", tag.Error(err))
		}
		errCh <- err
	}()
	scopeWithDomainTag.IncCounter(metrics.ArchiverClientVisibilityInlineArchiveAttemptCountPerDomain)
	URI, err := carchiver.NewURI(request.ArchiveRequest.VisibilityURI)
	if err != nil {
		return
	}

	visibilityArchiver, err := c.archiverProvider.GetVisibilityArchiver(URI.Scheme(), request.CallerService)
	if err != nil {
		return
	}

	err = visibilityArchiver.Archive(ctx, URI, &carchiver.ArchiveVisibilityRequest{
		DomainID:           request.ArchiveRequest.DomainID,
		DomainName:         request.ArchiveRequest.DomainName,
		WorkflowID:         request.ArchiveRequest.WorkflowID,
		RunID:              request.ArchiveRequest.RunID,
		WorkflowTypeName:   request.ArchiveRequest.WorkflowTypeName,
		StartTimestamp:     request.ArchiveRequest.StartTimestamp,
		ExecutionTimestamp: request.ArchiveRequest.ExecutionTimestamp,
		CloseTimestamp:     request.ArchiveRequest.CloseTimestamp,
		CloseStatus:        request.ArchiveRequest.CloseStatus,
		HistoryLength:      request.ArchiveRequest.HistoryLength,
		Memo:               request.ArchiveRequest.Memo,
		SearchAttributes:   convertSearchAttributesToString(request.ArchiveRequest.SearchAttributes),
		HistoryArchivalURI: request.ArchiveRequest.URI,
	})
}

func (c *client) sendArchiveSignal(ctx context.Context, request *ArchiveRequest, taggedLogger log.Logger) error {
	scopeWithDomainTag := c.metricsScope.Tagged(metrics.DomainTag(request.DomainName))
	scopeWithDomainTag.IncCounter(metrics.ArchiverClientSendSignalCountPerDomain)
	if ok := c.rateLimiter.Allow(); !ok {
		c.logger.Error(tooManyRequestsErrMsg)
		scopeWithDomainTag.IncCounter(metrics.CadenceErrServiceBusyCounter)
		return errors.New(tooManyRequestsErrMsg)
	}

	workflowID := fmt.Sprintf("%v-%v", workflowIDPrefix, rand.Intn(c.numWorkflows()))
	workflowOptions := cclient.StartWorkflowOptions{
		ID:                              workflowID,
		TaskList:                        decisionTaskList,
		ExecutionStartToCloseTimeout:    workflowStartToCloseTimeout,
		DecisionTaskStartToCloseTimeout: workflowTaskStartToCloseTimeout,
		WorkflowIDReusePolicy:           cclient.WorkflowIDReusePolicyAllowDuplicate,
	}
	signalCtx, cancel := context.WithTimeout(context.Background(), signalTimeout)
	defer cancel()
	_, err := c.cadenceClient.SignalWithStartWorkflow(signalCtx, workflowID, signalName, *request, workflowOptions, archivalWorkflowFnName, nil)
	if err != nil {
		taggedLogger = taggedLogger.WithTags(
			tag.ArchivalRequestDomainID(request.DomainID),
			tag.ArchivalRequestDomainName(request.DomainName),
			tag.ArchivalRequestWorkflowID(request.WorkflowID),
			tag.ArchivalRequestRunID(request.RunID),
			tag.WorkflowID(workflowID),
			tag.Error(err),
		)
		taggedLogger.Error("failed to send signal to archival system workflow")
		scopeWithDomainTag.IncCounter(metrics.ArchiverClientSendSignalFailureCountPerDomain)
		return err
	}
	return nil
}
