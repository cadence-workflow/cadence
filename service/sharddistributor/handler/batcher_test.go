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

	"github.com/uber/cadence/common/types"
)

// processFnFromMap returns an ephemeralBatchFn that resolves shard keys from a
// fixed map, returning an empty map (and no error) for any key not present.
func processFnFromMap(results map[string]*types.GetShardOwnerResponse) ephemeralBatchFn {
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

func TestShardBatcher_Submit(t *testing.T) {
	tests := []struct {
		name      string
		batchFn   ephemeralBatchFn
		namespace string
		shardKey  string
		// ctxFn builds the context used for the Submit call; defaults to context.Background().
		ctxFn       func() context.Context
		wantErr     bool
		wantErrIs   error
		wantErrMsg  string
		wantOwner   string
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
			b := newShardBatcher(10*time.Millisecond, tc.batchFn)
			b.Start()
			defer b.Stop()

			ctx := context.Background()
			if tc.ctxFn != nil {
				ctx = tc.ctxFn()
			}

			resp, err := b.Submit(ctx, tc.namespace, tc.shardKey)

			if tc.wantErr {
				require.Error(t, err)
				if tc.wantErrIs != nil {
					assert.ErrorIs(t, err, tc.wantErrIs)
				}
				if tc.wantErrMsg != "" {
					assert.ErrorContains(t, err, tc.wantErrMsg)
				}
				return
			}

			require.NoError(t, err)
			if tc.wantOwner != "" {
				assert.Equal(t, tc.wantOwner, resp.Owner)
			}
		})
	}
}

func TestShardBatcher_MultipleNamespacesIsolated(t *testing.T) {
	tests := []struct {
		name      string
		results   map[string]*types.GetShardOwnerResponse
		requests  []struct {
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
			b := newShardBatcher(10*time.Millisecond, processFnFromMap(tc.results))
			b.Start()
			defer b.Stop()

			var wg sync.WaitGroup
			for _, req := range tc.requests {
				wg.Add(1)
				go func(namespace, shardKey, wantOwner string) {
					defer wg.Done()
					resp, err := b.Submit(context.Background(), namespace, shardKey)
					require.NoError(t, err)
					assert.Equal(t, wantOwner, resp.Owner)
				}(req.namespace, req.shardKey, req.wantOwner)
			}
			wg.Wait()
		})
	}
}

func TestShardBatcher_ErrorPropagatedToAllCallers(t *testing.T) {
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

			b := newShardBatcher(10*time.Millisecond, batchFn)
			b.Start()
			defer b.Stop()

			errs := make([]error, tc.numCallers)
			var wg sync.WaitGroup
			for i := range tc.numCallers {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					_, errs[i] = b.Submit(context.Background(), "ns", "shard")
				}(i)
			}
			wg.Wait()

			for i, err := range errs {
				assert.ErrorContains(t, err, tc.batchErr.Error(), "caller %d should receive the batch error", i)
			}
		})
	}
}

func TestShardBatcher_ConcurrentRequestsBatchedTogether(t *testing.T) {
	tests := []struct {
		name      string
		numShards int
		// interval is kept generous so all goroutines queue before the first flush.
		interval time.Duration
	}{
		{
			name:      "20 concurrent shards collapsed into fewer batch calls",
			numShards: 20,
			interval:  50 * time.Millisecond,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			callCount := 0

			results := make(map[string]*types.GetShardOwnerResponse, tc.numShards)
			for i := range tc.numShards {
				key := "shard-" + string(rune('A'+i))
				results[key] = &types.GetShardOwnerResponse{Owner: "exec-1", Namespace: "ns"}
			}

			batchFn := func(_ context.Context, _ string, shardKeys []string) (map[string]*types.GetShardOwnerResponse, error) {
				mu.Lock()
				callCount++
				mu.Unlock()
				out := make(map[string]*types.GetShardOwnerResponse, len(shardKeys))
				for _, k := range shardKeys {
					if v, ok := results[k]; ok {
						out[k] = v
					}
				}
				return out, nil
			}

			b := newShardBatcher(tc.interval, batchFn)
			b.Start()
			defer b.Stop()

			var wg sync.WaitGroup
			for i := range tc.numShards {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					key := "shard-" + string(rune('A'+i))
					resp, err := b.Submit(context.Background(), "ns", key)
					require.NoError(t, err)
					assert.Equal(t, "exec-1", resp.Owner)
				}(i)
			}
			wg.Wait()

			mu.Lock()
			defer mu.Unlock()
			assert.Less(t, callCount, tc.numShards, "expected batching to reduce the number of processBatch invocations")
		})
	}
}

func TestShardBatcher_StopDrainsAndCancelsRemainingRequests(t *testing.T) {
	tests := []struct {
		name          string
		enqueueDelay  time.Duration
		stopTimeout   time.Duration
	}{
		{
			name:         "in-flight request resolves after Stop",
			enqueueDelay: 20 * time.Millisecond,
			stopTimeout:  500 * time.Millisecond,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			block := make(chan struct{})
			batchFn := func(_ context.Context, _ string, _ []string) (map[string]*types.GetShardOwnerResponse, error) {
				<-block
				return nil, nil
			}

			b := newShardBatcher(5*time.Millisecond, batchFn)
			b.Start()

			errCh := make(chan error, 1)
			go func() {
				_, err := b.Submit(context.Background(), "ns", "shard-1")
				errCh <- err
			}()

			// Give the goroutine time to enqueue.
			time.Sleep(tc.enqueueDelay)

			// Unblock and stop concurrently.
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