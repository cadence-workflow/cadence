// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package executorclient

import (
	"context"

	"github.com/uber/cadence/common/types"
)

// noopExecutor is an Executor implementation used when no shard-distributor
// config is provided. It acts as if all task lists are locally assigned
// (local-passthrough behaviour): it never contacts the shard distributor,
// never manages shard processors, and reports itself as not onboarded to SD.
type noopExecutor[SP ShardProcessor] struct{}

// NewNoopExecutor returns an Executor that is a no-op. Use this when the
// shard-distributor config is absent so that the rest of the matching engine
// can operate using the local hash-ring exclusively.
func NewNoopExecutor[SP ShardProcessor]() Executor[SP] {
	return &noopExecutor[SP]{}
}

func (e *noopExecutor[SP]) Start(_ context.Context)         {}
func (e *noopExecutor[SP]) Stop()                           {}
func (e *noopExecutor[SP]) GetNamespace() string            { return "" }
func (e *noopExecutor[SP]) IsOnboardedToSD() bool           { return false }
func (e *noopExecutor[SP]) SetMetadata(_ map[string]string) {}
func (e *noopExecutor[SP]) GetMetadata() map[string]string  { return nil }

// GetShardProcess always returns ErrShardProcessNotFound so that callers
// fall back to local hash-ring ownership checks.
func (e *noopExecutor[SP]) GetShardProcess(_ context.Context, _ string) (SP, error) {
	var zero SP
	return zero, ErrShardProcessNotFound
}

// AssignShardsFromLocalLogic is a no-op: without SD there is no shard
// assignment to manage.
func (e *noopExecutor[SP]) AssignShardsFromLocalLogic(_ context.Context, _ map[string]*types.ShardAssignment) error {
	return nil
}

// RemoveShardsFromLocalLogic is a no-op: without SD there is no shard
// assignment to manage.
func (e *noopExecutor[SP]) RemoveShardsFromLocalLogic(_ []string) error {
	return nil
}
