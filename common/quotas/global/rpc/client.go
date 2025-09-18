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

// Package rpc contains a concurrent RPC client, and handles mapping to/from the
// Any-typed request details so other packages do not have to concern themselves
// with those details.
package rpc

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination client_mock.go -self_package github.com/uber/cadence/common/quotas/global/rpc Client

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/quotas/global/shared"
	"github.com/uber/cadence/common/types"
)

type (
	Calls struct {
		Allowed  int
		Rejected int
	}
)

type (
	// UpdateResult holds all successes and errors encountered, to be distinct
	// from fatal errors that imply unusable results (i.e. a normal error return).
	UpdateResult struct {
		// all successfully-returned weights.
		// this may be populated and is safe to use even if Errors is present,
		// as some shards may have succeeded.
		Weights map[shared.GlobalKey]UpdateEntry
		// Any unexpected errors encountered, or nil if none.
		// Weights is valid even if this is present, though it may be empty.
		//
		// This should always be nil aside from major programming or rollout
		// errors, e.g. failures to serialize or deserialize data (coding errors
		// or incompatible server versions).
		Err error
	}
	UpdateEntry struct {
		Weight  float64
		UsedRPS float64
	}

	Client interface {
		// Update performs concurrent calls to all aggregating peers to send load info
		// and retrieve new per-key weights from the rest of the cluster.
		// This is intended to be called periodically in the background per process,
		// not synchronously or concurrently (per set of keys anyway), as it is fairly
		// high cost to shard keys and send so many requests.
		//
		// Currently, there are no truly fatal errors so this API does not return `error`.
		// Even in the presence of errors, some successful data may have been loaded,
		// and that will be part of the UpdateResult struct.
		//
		// As part of the contract of this call, UpdateResult will not ever contain
		// NaN, Inf, or negative values, as these imply fatal calculation problems.
		// It checks internally and will return an error rather than any value.
		Update(ctx context.Context, period time.Duration, load map[shared.GlobalKey]Calls) UpdateResult
	}

	client struct {
		history  history.Client
		resolver history.PeerResolver
		thisHost string

		logger log.Logger
		scope  metrics.Scope
	}
)

var _ Client = (*client)(nil)

func New(
	historyClient history.Client,
	resolver history.PeerResolver,
	logger log.Logger,
	met metrics.Client,
) Client {
	return &client{
		history:  historyClient,
		resolver: resolver,
		thisHost: uuid.NewString(), // TODO: would descriptive be better?  but it works, unique ensures correctness.
		logger:   logger,
		scope:    met.Scope(metrics.GlobalRatelimiter),
	}
}

func (c *client) Update(ctx context.Context, period time.Duration, load map[shared.GlobalKey]Calls) UpdateResult {
	keys := make([]string, 0, len(load))
	for k := range load {
		keys = append(keys, string(k))
	}
	peers, err := c.resolver.GlobalRatelimitPeers(keys)
	if err != nil {
		// should only happen if peers are unavailable, individual requests are handled other ways
		return UpdateResult{
			Err: fmt.Errorf("unable to shard ratelimit update data: %w", err),
		}
	}

	var mut sync.Mutex
	weights := make(map[shared.GlobalKey]UpdateEntry, len(load)) // should get back most or all keys requested

	var g errgroup.Group
	// could limit max concurrency easily with `g.SetLimit(n)` if desired,
	// but as each goes to a different host this seems fine to just blast out all at once,
	// and it makes timeouts easy because we don't need to reserve room for queued calls.
	for peer, peerKeys := range peers {
		g.Go(func() (err error) {
			defer func() { log.CapturePanic(recover(), c.logger, &err) }()

			result, err := c.updateSinglePeer(ctx, peer, period, filterKeys(peerKeys, load))
			if err != nil {
				return err // does not stop other calls, nor should it
			}

			mut.Lock()
			defer mut.Unlock()
			for k, v := range result {
				if _, ok := weights[k]; ok {
					// should not happen as resolver.GlobalRatelimiterPeers ensures disjoint keys
					// are requested, and only the requested keys are returned.
					return fmt.Errorf("received duplicate key %q from peer %q", k, peer)
				}
				weights[k] = v
			}
			return nil
		})
	}

	err = g.Wait()
	if err != nil {
		// wrap it so it's relatively severe-looking, as these should not happen
		err = fmt.Errorf("potentially fatal error during ratelimit update requests: %w", err)
	}
	return UpdateResult{
		Weights: weights,
		Err:     err,
	}
}

func (c *client) updateSinglePeer(ctx context.Context, peer history.Peer, period time.Duration, load map[shared.GlobalKey]Calls) (map[shared.GlobalKey]UpdateEntry, error) {
	anyValue, err := updateToAny(c.thisHost, period, load)
	if err != nil {
		// serialization errors should never happen
		return nil, &SerializationError{err}
	}

	result, err := c.history.RatelimitUpdate(
		ctx,
		&types.RatelimitUpdateRequest{
			Any: anyValue,
		},
		peer.ToYarpcShardKey(),
	)
	if err != nil {
		// client metrics are fine for monitoring, but it does not log errors.
		// TODO: possibly filter out or aggregate "peer lost" logs?  they're expected during deploys, since calls are not implicitly retried.
		c.logger.Warn(
			"request failure when updating ratelimits",
			tag.Error(&RPCError{err}),
			tag.GlobalRatelimiterPeer(string(peer)),
		)
		return nil, nil // rpc errors are essentially expected, and not an "error"
	}

	resp, err := anyToWeights(result.Any)
	if err != nil {
		// deserialization errors should never happen
		return nil, &SerializationError{err}
	}

	// and check the values.
	// - weights must be 0..1 inclusive
	// - used RPS must be non-negative
	// - NaNs and Infs are always rejected
	//
	// any failure is fatal to the whole request, that host cannot be trusted,
	// and any affected limiters should switch to their fallbacks if the issue
	// persists long enough.
	for key, entry := range resp {
		if msg := shared.SanityCheckFloat(0, entry.Weight, 1, "weight"); msg != "" {
			return nil, fmt.Errorf("bad value for key %q: from host: %q: %w", key, peer, errors.New(msg))
		}
		// no upper bound, but true infs are not allowed
		if msg := shared.SanityCheckFloat(0, entry.UsedRPS, math.Inf(1), "used rps"); msg != "" {
			return nil, fmt.Errorf("bad value for key %q: from host: %q %w", key, peer, errors.New(msg))
		}
	}

	return resp, nil
}

func filterKeys(keys []string, load map[shared.GlobalKey]Calls) map[shared.GlobalKey]Calls {
	result := make(map[shared.GlobalKey]Calls, len(keys))
	for _, k := range keys {
		result[shared.GlobalKey(k)] = load[shared.GlobalKey(k)]
	}
	return result
}
