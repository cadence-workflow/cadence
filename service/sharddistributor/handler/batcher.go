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

	"github.com/uber/cadence/common/types"
)

// ephemeralBatchFn is a function that assigns a batch of shards within a namespace
// and returns a map of shardKey -> GetShardOwnerResponse for each successfully assigned shard.
// Keys absent from the result map indicate an error for that specific shard.
type ephemeralBatchFn func(ctx context.Context, namespace string, shardKeys []string) (map[string]*types.GetShardOwnerResponse, error)

// batchRequest is a single caller's request submitted to the shardBatcher.
type batchRequest struct {
	namespace string
	shardKey  string
	// respChan is a buffered channel (cap 1) so the batcher goroutine never blocks
	// when writing the result back.
	respChan chan batchResponse
}

// batchResponse carries the result for a single batchRequest.
type batchResponse struct {
	resp *types.GetShardOwnerResponse
	err  error
}

// shardBatcher collects GetShardOwner calls for ephemeral namespaces over a
// configurable time window and processes them in a single batch, reducing the
// number of storage round-trips under high concurrency.
//
// Usage:
//
//	b := newShardBatcher(100*time.Millisecond, processFn)
//	b.Start()
//	defer b.Stop()
//	resp, err := b.Submit(ctx, namespace, shardKey)
type shardBatcher struct {
	interval     time.Duration
	processBatch ephemeralBatchFn

	// requestChan is shared across all goroutines; requests are keyed by namespace
	// inside the loop so a single goroutine can handle multiple namespaces.
	requestChan chan *batchRequest

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// newShardBatcher creates a new shardBatcher that will flush pending requests
// every interval using the provided processBatch function.
func newShardBatcher(interval time.Duration, processBatch ephemeralBatchFn) *shardBatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &shardBatcher{
		interval:     interval,
		processBatch: processBatch,
		// Generous buffer so callers are not blocked when submitting requests.
		requestChan: make(chan *batchRequest, 1024),
		ctx:         ctx,
		cancel:      cancel,
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
func (b *shardBatcher) Submit(ctx context.Context, namespace, shardKey string) (*types.GetShardOwnerResponse, error) {
	req := &batchRequest{
		namespace: namespace,
		shardKey:  shardKey,
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

// loop is the single background goroutine that drives micro-batching.
// It collects requests during each interval tick and flushes them together.
func (b *shardBatcher) loop() {
	defer b.wg.Done()

	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()

	// pending maps namespace -> list of batchRequests accumulated since last flush.
	pending := make(map[string][]*batchRequest)

	for {
		select {
		case req := <-b.requestChan:
			pending[req.namespace] = append(pending[req.namespace], req)

		case <-ticker.C:
			b.flush(pending)
			// Re-initialise; flush takes ownership of the old map entries.
			pending = make(map[string][]*batchRequest)

		case <-b.ctx.Done():
			// Drain any remaining requests in the channel before exiting so
			// callers that already submitted get an orderly cancellation response.
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
		results, err := b.processBatch(context.Background(), namespace, shardKeys)

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

// drainAndCancel sends a cancellation response to all requests that arrived
// after the last tick but before the context was cancelled.
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
