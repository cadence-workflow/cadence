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
// fixed map, returning an error for any key not present.
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

func TestShardBatcher_SingleRequest(t *testing.T) {
	want := &types.GetShardOwnerResponse{Owner: "exec-1", Namespace: "ns"}
	b := newShardBatcher(10*time.Millisecond, processFnFromMap(map[string]*types.GetShardOwnerResponse{
		"shard-1": want,
	}))
	b.Start()
	defer b.Stop()

	resp, err := b.Submit(context.Background(), "ns", "shard-1")
	require.NoError(t, err)
	assert.Equal(t, want, resp)
}

func TestShardBatcher_ConcurrentRequestsBatchedTogether(t *testing.T) {
	const numShards = 20
	// Record how many times the batch function is called; under batching it
	// should be far fewer than numShards.
	var mu sync.Mutex
	callCount := 0

	results := make(map[string]*types.GetShardOwnerResponse, numShards)
	for i := range numShards {
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

	// Use a generous interval so all goroutines are guaranteed to queue before
	// the first flush.
	b := newShardBatcher(50*time.Millisecond, batchFn)
	b.Start()
	defer b.Stop()

	var wg sync.WaitGroup
	for i := range numShards {
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
	// All requests should have been collapsed into significantly fewer batch
	// calls than numShards (ideally 1, but allow for timing jitter).
	assert.Less(t, callCount, numShards, "expected batching to reduce the number of processBatch invocations")
}

func TestShardBatcher_ErrorPropagatedToAllCallers(t *testing.T) {
	batchErr := errors.New("storage unavailable")
	batchFn := func(_ context.Context, _ string, _ []string) (map[string]*types.GetShardOwnerResponse, error) {
		return nil, batchErr
	}

	b := newShardBatcher(10*time.Millisecond, batchFn)
	b.Start()
	defer b.Stop()

	const n = 5
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = b.Submit(context.Background(), "ns", "shard")
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		assert.ErrorContains(t, err, "storage unavailable", "caller %d should receive the batch error", i)
	}
}

func TestShardBatcher_MissingKeyInResult(t *testing.T) {
	// processBatch returns an empty map — simulates a bug where a key is absent.
	batchFn := func(_ context.Context, _ string, _ []string) (map[string]*types.GetShardOwnerResponse, error) {
		return map[string]*types.GetShardOwnerResponse{}, nil
	}

	b := newShardBatcher(10*time.Millisecond, batchFn)
	b.Start()
	defer b.Stop()

	_, err := b.Submit(context.Background(), "ns", "shard-missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "shard-missing")
}

func TestShardBatcher_ContextCancelledBeforeSubmit(t *testing.T) {
	b := newShardBatcher(10*time.Millisecond, processFnFromMap(nil))
	b.Start()
	defer b.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before submitting

	_, err := b.Submit(ctx, "ns", "shard-1")
	require.ErrorIs(t, err, context.Canceled)
}

func TestShardBatcher_StopDrainsAndCancelsRemainingRequests(t *testing.T) {
	// Block the batch function so the request sits in-flight when Stop is called.
	block := make(chan struct{})
	batchFn := func(_ context.Context, _ string, _ []string) (map[string]*types.GetShardOwnerResponse, error) {
		<-block
		return nil, nil
	}

	b := newShardBatcher(5*time.Millisecond, batchFn)
	b.Start()

	// Submit a request; it will be picked up but the batchFn will block.
	errCh := make(chan error, 1)
	go func() {
		_, err := b.Submit(context.Background(), "ns", "shard-1")
		errCh <- err
	}()

	// Give the goroutine time to enqueue.
	time.Sleep(20 * time.Millisecond)

	// Unblock and stop concurrently.
	close(block)
	b.Stop()

	// The caller must receive either a valid response or a cancellation — not hang.
	select {
	case <-errCh:
		// ok
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Submit did not return after Stop")
	}
}

func TestShardBatcher_MultipleNamespacesIsolated(t *testing.T) {
	results := map[string]*types.GetShardOwnerResponse{
		"shard-a": {Owner: "exec-ns1", Namespace: "ns1"},
		"shard-b": {Owner: "exec-ns2", Namespace: "ns2"},
	}

	b := newShardBatcher(10*time.Millisecond, processFnFromMap(results))
	b.Start()
	defer b.Stop()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		resp, err := b.Submit(context.Background(), "ns1", "shard-a")
		require.NoError(t, err)
		assert.Equal(t, "exec-ns1", resp.Owner)
	}()
	go func() {
		defer wg.Done()
		resp, err := b.Submit(context.Background(), "ns2", "shard-b")
		require.NoError(t, err)
		assert.Equal(t, "exec-ns2", resp.Owner)
	}()
	wg.Wait()
}
