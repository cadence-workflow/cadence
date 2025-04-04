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

package handler

import (
	"context"
	"fmt"
	"sync"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/constants"
)

func NewHandler(
	logger log.Logger,
	metricsClient metrics.Client,
	matchingRing membership.SingleProvider,
	historyRing membership.SingleProvider,
) Handler {
	handler := &handlerImpl{
		logger:        logger,
		metricsClient: metricsClient,
		matchingRing:  matchingRing,
		historyRing:   historyRing,
	}

	// prevent us from trying to serve requests before shard distributor is started and ready
	handler.startWG.Add(1)
	return handler
}

type handlerImpl struct {
	logger        log.Logger
	metricsClient metrics.Client
	peerProvider  membership.PeerProvider
	startWG       sync.WaitGroup

	matchingRing membership.SingleProvider
	historyRing  membership.SingleProvider
}

func (h *handlerImpl) Start() {
	h.startWG.Done()
}

func (h *handlerImpl) Stop() {
}

func (h *handlerImpl) Health(ctx context.Context) (*types.HealthStatus, error) {
	h.startWG.Wait()
	h.logger.Debug("Shard Distributor service health check endpoint reached.")
	hs := &types.HealthStatus{Ok: true, Msg: "shard distributor good"}
	return hs, nil
}

func (h *handlerImpl) GetShardOwner(ctx context.Context, request *types.GetShardOwnerRequest) (resp *types.GetShardOwnerResponse, retError error) {
	defer func() { log.CapturePanic(recover(), h.logger, &retError) }()

	var owner string
	var err error
	switch request.GetNamespace() {
	case constants.HistoryNamespace:
		owner, err = h.historyRing.LookupRaw(request.GetShardKey())
	case constants.MatchingNamespace:
		owner, err = h.matchingRing.LookupRaw(request.GetShardKey())
	default:
		return nil, &types.NamespaceNotFoundError{Namespace: request.GetNamespace()}
	}

	if err != nil {
		return nil, fmt.Errorf("lookup owner in namespace %s %w", request.GetNamespace(), err)
	}

	resp = &types.GetShardOwnerResponse{
		Owner:     owner,
		Namespace: request.Namespace,
	}

	return resp, nil
}
