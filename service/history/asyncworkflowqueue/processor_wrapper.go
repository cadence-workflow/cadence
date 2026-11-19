package asyncworkflowqueue

import (
	"context"
	"errors"
	"fmt"

	"github.com/uber/cadence/common/activecluster"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
)

// ForwardedFromHeaderName is the envelope header stamped with the name of the cluster that forwarded an
// async workflow request. A request carrying a stamp from another cluster is never forwarded a second
// time, bounding forwarding to one hop.
const ForwardedFromHeaderName = "cadence-async-forwarded-from"

type processorWrapper struct {
	currentCluster   string
	activeClusterMgr activecluster.Manager
	active           Processor
	notActive        Processor
	logger           log.Logger
}

// NewProcessorWrapper returns a Processor that routes to active when the domain is active in
// currentCluster and to notActive (with req.ActiveCluster set) otherwise.
func NewProcessorWrapper(
	currentCluster string,
	activeClusterMgr activecluster.Manager,
	active Processor,
	notActive Processor,
	logger log.Logger,
) Processor {
	return &processorWrapper{
		currentCluster:   currentCluster,
		activeClusterMgr: activeClusterMgr,
		active:           active,
		notActive:        notActive,
		logger:           logger,
	}
}

func (p *processorWrapper) Process(ctx context.Context, req *Request) error {
	var policy *types.ActiveClusterSelectionPolicy
	switch {
	case req.Start != nil:
		policy = req.Start.ActiveClusterSelectionPolicy
	case req.SignalWithStart != nil:
		policy = req.SignalWithStart.ActiveClusterSelectionPolicy
	}

	info, err := p.activeClusterMgr.GetActiveClusterInfoByClusterAttribute(ctx, req.DomainID, policy.GetClusterAttribute())
	if err != nil {
		p.logger.Warn("Failed to get active cluster info, processing async workflow request as active.",
			tag.WorkflowDomainID(req.DomainID), tag.Error(err))
	} else if info.ActiveClusterName != p.currentCluster {
		return p.forward(ctx, req, info.ActiveClusterName)
	}

	err = p.active.Process(ctx, req)
	var dna *types.DomainNotActiveError
	if errors.As(err, &dna) && dna.ActiveCluster != "" && dna.ActiveCluster != p.currentCluster {
		return p.forward(ctx, req, dna.ActiveCluster)
	}
	return err
}

// forward delegates a stamped copy of the request to the not-active processor so the row is forwarded
// at most once. The caller's Request is left untouched: Process makes one attempt, and a caller that
// retries with the same Request must not find this cluster's own stamp from the failed attempt.
// A stamp naming the current cluster can only be ours (the remote frontend captures the replayed
// headers into its own queue row, and a bounced row is refused there), so only a foreign stamp
// counts as a prior hop.
func (p *processorWrapper) forward(ctx context.Context, req *Request, activeCluster string) error {
	if from := string(req.Headers[ForwardedFromHeaderName]); from != "" && from != p.currentCluster {
		p.logger.Warn("async workflow request already forwarded once, not forwarding again",
			tag.WorkflowDomainID(req.DomainID),
			tag.WorkflowID(req.Message.WorkflowID),
			tag.Value(from),
			tag.ClusterName(activeCluster))
		return &types.DomainNotActiveError{
			Message: fmt.Sprintf("async workflow request for domain %s already forwarded from %s; active cluster is now %s",
				req.DomainID, from, activeCluster),
			DomainName:     req.Message.DomainName,
			CurrentCluster: p.currentCluster,
			ActiveCluster:  activeCluster,
		}
	}
	fwd := *req
	fwd.ActiveCluster = activeCluster
	fwd.Headers = make(map[string][]byte, len(req.Headers)+1)
	for k, v := range req.Headers {
		fwd.Headers[k] = v
	}
	fwd.Headers[ForwardedFromHeaderName] = []byte(p.currentCluster)
	return p.notActive.Process(ctx, &fwd)
}
