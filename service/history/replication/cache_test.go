// The MIT License (MIT)
//
// Copyright (c) 2022 Uber Technologies Inc.
//
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

package replication

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/types"
)

func TestCache(t *testing.T) {
	task1 := &types.ReplicationTask{SourceTaskID: 100}
	task2 := &types.ReplicationTask{SourceTaskID: 200}
	task3 := &types.ReplicationTask{SourceTaskID: 300}
	task4 := &types.ReplicationTask{SourceTaskID: 400}

	cache := NewCache(dynamicproperties.GetIntPropertyFn(3))

	assert.Equal(t, 0, cache.Size())
	require.NoError(t, cache.Put(task2))
	assert.Equal(t, 1, cache.Size())
	require.NoError(t, cache.Put(task2))
	assert.Equal(t, 1, cache.Size())
	require.NoError(t, cache.Put(task3))
	assert.Equal(t, 2, cache.Size())
	require.NoError(t, cache.Put(task1))
	assert.Equal(t, 3, cache.Size())
	assert.Equal(t, errCacheFull, cache.Put(task4))
	assert.Equal(t, 3, cache.Size())

	assert.Nil(t, cache.Get(0))
	assert.Nil(t, cache.Get(99))
	assert.Equal(t, task1, cache.Get(100))
	assert.Nil(t, cache.Get(101))
	assert.Equal(t, task2, cache.Get(200))
	assert.Equal(t, task3, cache.Get(300))

	cache.Ack(0)
	assert.Equal(t, 3, cache.Size())
	assert.Equal(t, task1, cache.Get(100))
	assert.Equal(t, task2, cache.Get(200))
	assert.Equal(t, task3, cache.Get(300))

	cache.Ack(1)
	assert.Equal(t, 3, cache.Size())
	assert.Equal(t, task1, cache.Get(100))
	assert.Equal(t, task2, cache.Get(200))
	assert.Equal(t, task3, cache.Get(300))

	cache.Ack(99)
	assert.Equal(t, 3, cache.Size())
	assert.Equal(t, task1, cache.Get(100))
	assert.Equal(t, task2, cache.Get(200))
	assert.Equal(t, task3, cache.Get(300))

	cache.Ack(100)
	assert.Equal(t, 2, cache.Size())
	assert.Nil(t, cache.Get(100))
	assert.Equal(t, task2, cache.Get(200))
	assert.Equal(t, task3, cache.Get(300))

	assert.Equal(t, errAlreadyAcked, cache.Put(task1))

	cache.Ack(301)
	assert.Equal(t, 0, cache.Size())
	assert.Nil(t, cache.Get(100))
	assert.Nil(t, cache.Get(200))
	assert.Nil(t, cache.Get(300))

	require.NoError(t, cache.Put(task4))
	assert.Equal(t, 1, cache.Size())
	assert.Nil(t, cache.Get(100))
	assert.Nil(t, cache.Get(200))
	assert.Nil(t, cache.Get(300))
	assert.Equal(t, task4, cache.Get(400))
}

func BenchmarkCache(b *testing.B) {
	cache := NewCache(dynamicproperties.GetIntPropertyFn(10000))
	for i := 0; i < 5000; i++ {
		cache.Put(&types.ReplicationTask{SourceTaskID: int64(i * 100)})
	}

	for n := 0; n < b.N; n++ {
		readLevel := int64((n) * 100)
		cache.Get(readLevel)
		cache.Put(&types.ReplicationTask{SourceTaskID: int64(n * 100)})
		cache.Ack(readLevel)
	}
}
