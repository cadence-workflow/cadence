package asyncworkflowqueue

import (
	"context"
	"errors"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/isolationgroup"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/engine"
)

type activeProcessor struct {
	engineFn   func() engine.Engine
	timeSource clock.TimeSource
	scope      metrics.Scope
	logger     log.Logger
}

// NewActiveProcessor returns a Processor that starts workflows in the shard engine.
// engineFn is called on every Process; its result is never cached.
func NewActiveProcessor(engineFn func() engine.Engine, timeSource clock.TimeSource, scope metrics.Scope, logger log.Logger) Processor {
	return &activeProcessor{
		engineFn:   engineFn,
		timeSource: timeSource,
		scope:      scope,
		logger:     logger,
	}
}

func (p *activeProcessor) Process(ctx context.Context, req *Request) error {
	var domainName string
	switch {
	case req.Start != nil:
		domainName = req.Start.GetDomain()
	case req.SignalWithStart != nil:
		domainName = req.SignalWithStart.GetDomain()
	default:
		return &types.InternalServiceError{
			Message: "async workflow queue request has neither start nor signal-with-start payload",
		}
	}
	scope := p.scope.Tagged(metrics.DomainTag(domainName))

	var err error
	if req.Start != nil {
		var histReq *types.HistoryStartWorkflowExecutionRequest
		histReq, err = common.CreateHistoryStartWorkflowRequest(req.DomainID, req.Start, p.timeSource.Now(), partitionConfigFromHeaders(req.Headers))
		if err != nil {
			return err
		}
		sw := scope.StartTimer(metrics.AsyncWorkflowQueueProcessLatency)
		_, err = p.engineFn().StartWorkflowExecution(ctx, histReq)
		sw.Stop()
	} else {
		histReq := &types.HistorySignalWithStartWorkflowExecutionRequest{
			DomainUUID:             req.DomainID,
			SignalWithStartRequest: req.SignalWithStart,
			PartitionConfig:        partitionConfigFromHeaders(req.Headers),
		}
		sw := scope.StartTimer(metrics.AsyncWorkflowQueueProcessLatency)
		_, err = p.engineFn().SignalWithStartWorkflowExecution(ctx, histReq)
		sw.Stop()
	}

	var alreadyStarted *types.WorkflowExecutionAlreadyStartedError
	switch {
	case err == nil:
		scope.IncCounter(metrics.AsyncWorkflowQueueSuccessCounter)
		return nil
	case errors.As(err, &alreadyStarted):
		scope.IncCounter(metrics.AsyncWorkflowQueueAlreadyStartedCounter)
		return nil
	default:
		return err
	}
}

// partitionConfigFromHeaders mirrors the frontend ClientPartitionConfigMiddleware: a non-empty client
// isolation group header becomes the tasklist-isolation partition config, otherwise there is none.
func partitionConfigFromHeaders(headers map[string][]byte) map[string]string {
	zone := string(headers[common.ClientIsolationGroupHeaderName])
	if zone == "" {
		return nil
	}
	return map[string]string{isolationgroup.GroupKey: zone}
}
