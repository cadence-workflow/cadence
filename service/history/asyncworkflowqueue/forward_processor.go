package asyncworkflowqueue

import (
	"context"
	"sort"

	"go.uber.org/yarpc"

	"github.com/uber/cadence/client"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

type forwardProcessor struct {
	clientBean client.Bean
	scope      metrics.Scope
	logger     log.Logger
}

// NewForwardProcessor returns a Processor that forwards requests to the frontend of req.ActiveCluster,
// replaying the yarpc headers captured at accept time.
func NewForwardProcessor(clientBean client.Bean, scope metrics.Scope, logger log.Logger) Processor {
	return &forwardProcessor{
		clientBean: clientBean,
		scope:      scope,
		logger:     logger,
	}
}

func (p *forwardProcessor) Process(ctx context.Context, req *Request) error {
	if req.ActiveCluster == "" {
		return &types.InternalServiceError{
			Message: "async workflow queue forward requires a non-empty active cluster",
		}
	}
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

	fe, err := p.clientBean.GetRemoteFrontendClient(req.ActiveCluster)
	if err != nil {
		return err
	}

	opts := headerOptions(req.Headers)
	if req.Start != nil {
		_, err = fe.StartWorkflowExecutionAsync(ctx, &types.StartWorkflowExecutionAsyncRequest{
			StartWorkflowExecutionRequest: req.Start,
		}, opts...)
	} else {
		_, err = fe.SignalWithStartWorkflowExecutionAsync(ctx, &types.SignalWithStartWorkflowExecutionAsyncRequest{
			SignalWithStartWorkflowExecutionRequest: req.SignalWithStart,
		}, opts...)
	}
	if err != nil {
		return err
	}

	p.scope.Tagged(metrics.DomainTag(domainName)).IncCounter(metrics.AsyncWorkflowQueueForwardedCounter)
	return nil
}

// headerOptions converts captured headers into yarpc options, sorted by key for a deterministic order.
func headerOptions(headers map[string][]byte) []yarpc.CallOption {
	if len(headers) == 0 {
		return nil
	}
	keys := make([]string, 0, len(headers))
	for k := range headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	opts := make([]yarpc.CallOption, 0, len(keys))
	for _, k := range keys {
		opts = append(opts, yarpc.WithHeader(k, string(headers[k])))
	}
	return opts
}
