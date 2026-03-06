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
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/types"
)

// processFnFromMap returns an ephemeralAssignmentBatchFn that resolves shard keys from a
// fixed map, returning an empty map (and no error) for any key not present.
func processFnFromMap(results map[string]*types.GetShardOwnerResponse) ephemeralAssignmentBatchFn {
	return func(_ context.Context, _ string, shardKeys []string) (map[string]*types.GetShardOwnerResponse, error) {
		out := make(map[string]*types.GetShardOwnerResponse, len(shardKeys))
		for _, k := range shardKeys {
			if v, ok := results[k]; ok {
				out[k] = v
			}
		}
		return out, nil
	}
}

// advanceUntilDone repeatedly advances the mocked clock by step until done is closed.
// Each advance fires one ticker cycle, draining whatever requests have been enqueued
// since the previous tick. This avoids races between goroutine scheduling and a
// single Advance call.
func advanceUntilDone(ts clock.MockedTimeSource, done <-chan struct{}, step time.Duration) {
	for {
		select {
		case <-done:
			return
		default:
			ts.Advance(step)
		}
	}
}

func TestShardBatcher_Submit(t *testing.T) {
	defer goleak.VerifyNone(t)
	tests := []struct {
		name      string
		batchFn   ephemeralAssignmentBatchFn
		namespace string
		shardKey  string
		// ctxFn builds the context used for the Submit call; defaults to context.Background().
		ctxFn      func() context.Context
		wantErr    bool
		wantErrIs  error
		wantErrMsg string
		wantOwner  string
	}{
		{
			name: "single request resolved from map",
			batchFn: processFnFromMap(map[string]*types.GetShardOwnerResponse{
				"shard-1": {Owner: "exec-1", Namespace: "ns"},
			}),
			namespace: "ns",
			shardKey:  "shard-1",
			wantOwner: "exec-1",
		},
		{
			name: "batch function returns error - propagated to caller",
			batchFn: func(_ context.Context, _ string, _ []string) (map[string]*types.GetShardOwnerResponse, error) {
				return nil, errors.New("storage unavailable")
			},
			namespace:  "ns",
			shardKey:   "shard-1",
			wantErr:    true,
			wantErrMsg: "storage unavailable",
		},
		{
			name: "key absent from batch result returns internal error",
			batchFn: func(_ context.Context, _ string, _ []string) (map[string]*types.GetShardOwnerResponse, error) {
				return map[string]*types.GetShardOwnerResponse{}, nil
			},
			namespace:  "ns",
			shardKey:   "shard-missing",
			wantErr:    true,
			wantErrMsg: "shard-missing",
		},
		{
			name:      "context cancelled before submit",
			batchFn:   processFnFromMap(nil),
			namespace: "ns",
			shardKey:  "shard-1",
			ctxFn: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			wantErr:   true,
			wantErrIs: context.Canceled,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := clock.NewMockedTimeSource()
			b := newShardBatcher(ts, 10*time.Millisecond, tc.batchFn)
			b.Start()
			defer b.Stop()

			ctx := context.Background()
			if tc.ctxFn != nil {
				ctx = tc.ctxFn()
			}

			type result struct {
				resp *types.GetShardOwnerResponse
				err  error
			}
			done := make(chan struct{})
			ch := make(chan result, 1)
			go func() {
				resp, err := b.Submit(ctx, &types.GetShardOwnerRequest{Namespace: tc.namespace, ShardKey: tc.shardKey})
				ch <- result{resp, err}
				close(done)
			}()

			// Keep ticking until the goroutine finishes. Advancing by 2x the interval
			// per step accounts for jitter and ensures the request is flushed even if
			// it races with the first tick.
			ts.BlockUntil(1)
			advanceUntilDone(ts, done, 20*time.Millisecond)

			got := <-ch
			if tc.wantErr {
				require.Error(t, got.err)
				if tc.wantErrIs != nil {
					assert.ErrorIs(t, got.err, tc.wantErrIs)
				}
				if tc.wantErrMsg != "" {
					assert.ErrorContains(t, got.err, tc.wantErrMsg)
				}
				return
			}

			require.NoError(t, got.err)
			if tc.wantOwner != "" {
				assert.Equal(t, tc.wantOwner, got.resp.Owner)
			}
		})
	}
}

func TestShardBatcher_MultipleNamespacesIsolated(t *testing.T) {
	defer goleak.VerifyNone(t)
	tests := []struct {
		name     string
		results  map[string]*types.GetShardOwnerResponse
		requests []struct {
			namespace string
			shardKey  string
			wantOwner string
		}
	}{
		{
			name: "two namespaces resolved independently",
			results: map[string]*types.GetShardOwnerResponse{
				"shard-a": {Owner: "exec-ns1", Namespace: "ns1"},
				"shard-b": {Owner: "exec-ns2", Namespace: "ns2"},
			},
			requests: []struct {
				namespace string
				shardKey  string
				wantOwner string
			}{
				{namespace: "ns1", shardKey: "shard-a", wantOwner: "exec-ns1"},
				{namespace: "ns2", shardKey: "shard-b", wantOwner: "exec-ns2"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := clock.NewMockedTimeSource()
			b := newShardBatcher(ts, 10*time.Millisecond, processFnFromMap(tc.results))
			b.Start()
			defer b.Stop()

			type result struct {
				owner string
				err   error
			}
			got := make([]result, len(tc.requests))
			var wg sync.WaitGroup
			for i, req := range tc.requests {
				wg.Add(1)
				go func(i int, namespace, shardKey string) {
					defer wg.Done()
					resp, err := b.Submit(context.Background(), &types.GetShardOwnerRequest{Namespace: namespace, ShardKey: shardKey})
					if err == nil {
						got[i] = result{owner: resp.Owner}
					} else {
						got[i] = result{err: err}
					}
				}(i, req.namespace, req.shardKey)
			}

			done := make(chan struct{})
			go func() { wg.Wait(); close(done) }()

			ts.BlockUntil(1)
			advanceUntilDone(ts, done, 20*time.Millisecond)

			for i, req := range tc.requests {
				require.NoError(t, got[i].err)
				assert.Equal(t, req.wantOwner, got[i].owner)
			}
		})
	}
}

func TestShardBatcher_ErrorPropagatedToAllCallers(t *testing.T) {
	defer goleak.VerifyNone(t)
	tests := []struct {
		name       string
		batchErr   error
		numCallers int
	}{
		{
			name:       "error propagated to all concurrent callers",
			batchErr:   errors.New("storage unavailable"),
			numCallers: 5,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			batchFn := func(_ context.Context, _ string, _ []string) (map[string]*types.GetShardOwnerResponse, error) {
				return nil, tc.batchErr
			}

			ts := clock.NewMockedTimeSource()
			b := newShardBatcher(ts, 10*time.Millisecond, batchFn)
			b.Start()
			defer b.Stop()

			errs := make([]error, tc.numCallers)
			var wg sync.WaitGroup
			for i := range tc.numCallers {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					_, errs[i] = b.Submit(context.Background(), &types.GetShardOwnerRequest{Namespace: "ns", ShardKey: "shard"})
				}(i)
			}

			done := make(chan struct{})
			go func() { wg.Wait(); close(done) }()

			ts.BlockUntil(1)
			advanceUntilDone(ts, done, 20*time.Millisecond)

			for i, err := range errs {
				assert.ErrorContains(t, err, tc.batchErr.Error(), "caller %d should receive the batch error", i)
			}
		})
	}
}

func TestShardBatcher_ConcurrentRequestsBatchedTogether(t *testing.T) {
	defer goleak.VerifyNone(t)
	tests := []struct {
		name      string
		numShards int
		interval  time.Duration
	}{
		{
			name:      "20 concurrent shards collapsed into fewer batch calls",
			numShards: 20,
			interval:  50 * time.Millisecond,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results := make(map[string]*types.GetShardOwnerResponse, tc.numShards)
			for i := range tc.numShards {
				key := "shard-" + string(rune('A'+i))
				results[key] = &types.GetShardOwnerResponse{Owner: "exec-1", Namespace: "ns"}
			}

			// maxBatchSize tracks the largest single batch seen. Batching is confirmed
			// when at least one flush call receives more than one shard key.
			var mu sync.Mutex
			maxBatchSize := 0
			batchFn := func(_ context.Context, _ string, shardKeys []string) (map[string]*types.GetShardOwnerResponse, error) {
				mu.Lock()
				if len(shardKeys) > maxBatchSize {
					maxBatchSize = len(shardKeys)
				}
				mu.Unlock()
				out := make(map[string]*types.GetShardOwnerResponse, len(shardKeys))
				for _, k := range shardKeys {
					if v, ok := results[k]; ok {
						out[k] = v
					}
				}
				return out, nil
			}

			ts := clock.NewMockedTimeSource()
			b := newShardBatcher(ts, tc.interval, batchFn)
			b.Start()
			defer b.Stop()

			var wg sync.WaitGroup
			for i := range tc.numShards {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					key := "shard-" + string(rune('A'+i))
					resp, err := b.Submit(context.Background(), &types.GetShardOwnerRequest{Namespace: "ns", ShardKey: key})
					require.NoError(t, err)
					assert.Equal(t, "exec-1", resp.Owner)
				}(i)
			}

			done := make(chan struct{})
			go func() { wg.Wait(); close(done) }()

			// Keep ticking until all callers finish. Each advance drains whatever
			// has been enqueued since the previous tick.
			ts.BlockUntil(1)
			advanceUntilDone(ts, done, 2*tc.interval)

			mu.Lock()
			defer mu.Unlock()
			assert.Greater(t, maxBatchSize, 1, "expected at least one batch call to receive more than one shard")
		})
	}
}

func TestShardBatcher_StopDrainsAndCancelsRemainingRequests(t *testing.T) {
	defer goleak.VerifyNone(t)
	tests := []struct {
		name        string
		stopTimeout time.Duration
	}{
		{
			name:        "in-flight request resolves after Stop",
			stopTimeout: 500 * time.Millisecond,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			block := make(chan struct{})
			batchFn := func(_ context.Context, _ string, _ []string) (map[string]*types.GetShardOwnerResponse, error) {
				<-block
				return nil, nil
			}

			ts := clock.NewMockedTimeSource()
			b := newShardBatcher(ts, 5*time.Millisecond, batchFn)
			b.Start()

			errCh := make(chan error, 1)
			done := make(chan struct{})
			go func() {
				_, err := b.Submit(context.Background(), &types.GetShardOwnerRequest{Namespace: "ns", ShardKey: "shard-1"})
				errCh <- err
				close(done)
			}()

			// Keep ticking until the flush is triggered and the goroutine unblocks.
			ts.BlockUntil(1)
			go advanceUntilDone(ts, done, 10*time.Millisecond)

			// Unblock the batchFn and stop the batcher.
			close(block)
			b.Stop()

			select {
			case <-errCh:
				// ok — caller received either a result or a cancellation error
			case <-time.After(tc.stopTimeout):
				t.Fatal("Submit did not return after Stop")
			}
		})
	}
}
