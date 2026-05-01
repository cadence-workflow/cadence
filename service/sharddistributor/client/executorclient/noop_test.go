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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/types"
)

func TestNoopExecutor(t *testing.T) {
	exec := NewNoopExecutor[*MockShardProcessor]()

	t.Run("Start and Stop are no-ops", func(t *testing.T) {
		// should not panic
		exec.Start(context.Background())
		exec.Stop()
	})

	t.Run("GetNamespace returns empty string", func(t *testing.T) {
		assert.Equal(t, "", exec.GetNamespace())
	})

	t.Run("IsOnboardedToSD returns false", func(t *testing.T) {
		assert.False(t, exec.IsOnboardedToSD())
	})

	t.Run("SetMetadata and GetMetadata are no-ops", func(t *testing.T) {
		exec.SetMetadata(map[string]string{"key": "value"})
		assert.Nil(t, exec.GetMetadata())
	})

	t.Run("GetShardProcess returns ErrShardProcessNotFound", func(t *testing.T) {
		sp, err := exec.GetShardProcess(context.Background(), "any-shard")
		assert.Nil(t, sp)
		assert.ErrorIs(t, err, ErrShardProcessNotFound)
	})

	t.Run("AssignShardsFromLocalLogic is a no-op", func(t *testing.T) {
		err := exec.AssignShardsFromLocalLogic(context.Background(), map[string]*types.ShardAssignment{
			"shard-1": {},
		})
		assert.NoError(t, err)
	})

	t.Run("RemoveShardsFromLocalLogic is a no-op", func(t *testing.T) {
		err := exec.RemoveShardsFromLocalLogic([]string{"shard-1"})
		assert.NoError(t, err)
	})
}
