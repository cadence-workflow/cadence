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
	"sync"
	"time"

	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/types"
)

const (
	_requestsChannelLimit = 1024
	_intervalJitterCoeff  = 0.1 // 10% jitter
)

// ephemeralAssignmentBatchFn is a function that assigns a batch of shards within a namespace
// and returns a map of shardKey -> GetShardOwnerResponse for each successfully assigned shard.
// Keys absent from the result map indicate an error for that specific shard.
type ephemeralAssignmentBatchFn func(ctx context.Context, namespace string, shardKeys []string) (map[string]*types.GetShardOwnerResponse, error)

// batchRequest is a single caller's request submitted to the shardBatcher.
type batchRequest struct {
	namespace string
	shardKey  string
	// respChan is a buffered channel (cap 1) so the batcher goroutine never blocks
	// when writing the result back.
	respChan chan batchResponse
}

type batchResponse struct {
	resp *types.GetShardOwnerResponse
	err  error
}

// shardBatcher collects GetShardOwner calls for ephemeral namespaces over a
// configurable time window and processes them in a single batch.
//
// Usage:
//
//	b := newShardBatcher(100*time.Millisecond, processFn)
//	b.Start()
//	defer b.Stop()
//	resp, err := b.Submit(ctx, &types.GetShardOwnerRequest{Namespace: namespace, ShardKey: shardKey})
type shardBatcher struct {
	timeSource   clock.TimeSource
	interval     time.Duration
	processBatch ephemeralAssignmentBatchFn

	// requestChan is shared across all goroutines; requests are keyed by namespace
	// inside the loop so a single goroutine can handle multiple namespaces.
	requestChan chan *batchRequest

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func newShardBatcher(timeSource clock.TimeSource, interval time.Duration, processBatch ephemeralAssignmentBatchFn) *shardBatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &shardBatcher{
		timeSource:   timeSource,
		interval:     interval,
		processBatch: processBatch,
		requestChan:  make(chan *batchRequest, _requestsChannelLimit),
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Start launches the background batching loop.
func (b *shardBatcher) Start() {
	b.wg.Add(1)
	go b.loop()
}

// Stop signals the batching loop to shut down and waits for it to finish.
// Any requests that have already been submitted but not yet flushed will be
// drained and processed before the loop exits.
func (b *shardBatcher) Stop() {
	b.cancel()
	b.wg.Wait()
}

// Submit enqueues a single GetShardOwner request and blocks until the batcher
// has processed the batch that contains it, or until ctx is cancelled.
func (b *shardBatcher) Submit(ctx context.Context, request *types.GetShardOwnerRequest) (*types.GetShardOwnerResponse, error) {
	req := &batchRequest{
		namespace: request.Namespace,
		shardKey:  request.ShardKey,
		respChan:  make(chan batchResponse, 1),
	}

	select {
	case b.requestChan <- req:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-b.ctx.Done():
		return nil, b.ctx.Err()
	}

	select {
	case res := <-req.respChan:
		return res.resp, res.err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-b.ctx.Done():
		return nil, b.ctx.Err()
	}
}

func (b *shardBatcher) loop() {
	defer b.wg.Done()

	ticker := b.timeSource.NewTicker(backoff.JitDuration(b.interval, _intervalJitterCoeff))
	defer ticker.Stop()

	// pending maps namespace -> list of batchRequests accumulated since last flush.
	pending := make(map[string][]*batchRequest)

	for {
		select {
		case req := <-b.requestChan:
			pending[req.namespace] = append(pending[req.namespace], req)

		case <-ticker.Chan():
			b.flush(pending)
			pending = make(map[string][]*batchRequest)

		case <-b.ctx.Done():
			// drainAndCancel sends a cancellation response to all requests that arrived
			// after the last tick but before the context was cancelled.
			b.drainAndCancel(pending)
			return
		}
	}
}

// flush processes all accumulated requests namespace by namespace.
func (b *shardBatcher) flush(pending map[string][]*batchRequest) {
	for namespace, reqs := range pending {
		if len(reqs) == 0 {
			continue
		}

		shardKeys := make([]string, len(reqs))
		for i, r := range reqs {
			shardKeys[i] = r.shardKey
		}

		// Use a background context so individual caller cancellations do not
		// abort the whole batch; callers will surface their own context error
		// via the select in Submit.
		ctx, cancel := context.WithTimeout(context.Background(), 5*b.interval)
		results, err := b.processBatch(ctx, namespace, shardKeys)
		cancel()

		for _, req := range reqs {
			var res batchResponse
			if err != nil {
				res = batchResponse{err: err}
			} else {
				res = batchResponse{resp: results[req.shardKey]}
				if res.resp == nil {
					// processBatch is expected to always include an entry for
					// every key it was given; a missing entry is an internal error.
					res = batchResponse{err: &types.InternalServiceError{
						Message: "batch processor returned no result for shard key: " + req.shardKey,
					}}
				}
			}
			// Non-blocking write: respChan has capacity 1 and each req has
			// exactly one writer (this loop) and one reader (Submit).
			req.respChan <- res
		}
	}
}

func (b *shardBatcher) drainAndCancel(pending map[string][]*batchRequest) {
	// First flush whatever was already accumulated.
	b.flush(pending)

	// Then drain the channel itself.
	for {
		select {
		case req := <-b.requestChan:
			req.respChan <- batchResponse{err: b.ctx.Err()}
		default:
			return
		}
	}
}
