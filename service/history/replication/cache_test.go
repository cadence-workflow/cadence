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
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/types"
)

func TestCache(t *testing.T) {
	var encodingType types.EncodingType

	dataBlob := &types.DataBlob{
		EncodingType: encodingType.Ptr(),
		Data:         []byte("01234567890123456789012345678901234567890123456789"),
	}

	historyTaskV2Attributes := &types.HistoryTaskV2Attributes{
		Events: dataBlob,
	}
	task1 := &types.ReplicationTask{SourceTaskID: 100}
	task2 := &types.ReplicationTask{SourceTaskID: 200, HistoryTaskV2Attributes: historyTaskV2Attributes}
	task3 := &types.ReplicationTask{SourceTaskID: 300}
	task4 := &types.ReplicationTask{SourceTaskID: 400}
	task5 := &types.ReplicationTask{SourceTaskID: 500}

	cache := NewCache(dynamicproperties.GetIntPropertyFn(4), dynamicproperties.GetIntPropertyFn(400), log.NewNoop())

	assert.Equal(t, uint64(0), cache.Size())
	require.NoError(t, cache.Put(task2))
	assert.Equal(t, task2.ByteSize(), cache.Size())
	assert.Equal(t, 1, cache.Count())
	require.NoError(t, cache.Put(task2))
	assert.Equal(t, task2.ByteSize(), cache.Size())
	assert.Equal(t, 1, cache.Count())
	require.NoError(t, cache.Put(task3))
	assert.Equal(t, task2.ByteSize()+task3.ByteSize(), cache.Size())
	assert.Equal(t, 2, cache.Count())
	require.NoError(t, cache.Put(task1))
	assert.Equal(t, task1.ByteSize()+task2.ByteSize()+task3.ByteSize(), cache.Size())
	assert.Equal(t, 3, cache.Count())
	assert.Equal(t, errCacheFull, cache.Put(task4))
	assert.Equal(t, 3, cache.Count())
	assert.Equal(t, task1.ByteSize()+task2.ByteSize()+task3.ByteSize(), cache.Size())

	cache.maxSize = dynamicproperties.GetIntPropertyFn(1000)
	require.NoError(t, cache.Put(task4))
	assert.Equal(t, 4, cache.Count())
	assert.Equal(t, task1.ByteSize()+task2.ByteSize()+task3.ByteSize()+task4.ByteSize(), cache.Size())
	assert.Equal(t, errCacheFull, cache.Put(task5))
	assert.Equal(t, 4, cache.Count())
	assert.Equal(t, task1.ByteSize()+task2.ByteSize()+task3.ByteSize()+task4.ByteSize(), cache.Size())

	assert.Nil(t, cache.Get(0))
	assert.Nil(t, cache.Get(99))
	assert.Equal(t, task1, cache.Get(100))
	assert.Nil(t, cache.Get(101))
	assert.Equal(t, task2, cache.Get(200))
	assert.Equal(t, task3, cache.Get(300))
	assert.Equal(t, task4, cache.Get(400))

	cache.Ack(0)
	assert.Equal(t, 4, cache.Count())
	assert.Equal(t, task1, cache.Get(100))
	assert.Equal(t, task2, cache.Get(200))
	assert.Equal(t, task3, cache.Get(300))
	assert.Equal(t, task4, cache.Get(400))

	cache.Ack(1)
	assert.Equal(t, 4, cache.Count())
	assert.Equal(t, task1, cache.Get(100))
	assert.Equal(t, task2, cache.Get(200))
	assert.Equal(t, task3, cache.Get(300))
	assert.Equal(t, task4, cache.Get(400))

	cache.Ack(99)
	assert.Equal(t, 4, cache.Count())
	assert.Equal(t, task1, cache.Get(100))
	assert.Equal(t, task2, cache.Get(200))
	assert.Equal(t, task3, cache.Get(300))
	assert.Equal(t, task4, cache.Get(400))

	cache.Ack(100)
	assert.Equal(t, 3, cache.Count())
	assert.Nil(t, cache.Get(100))
	assert.Equal(t, task2, cache.Get(200))
	assert.Equal(t, task3, cache.Get(300))
	assert.Equal(t, task4, cache.Get(400))

	assert.Equal(t, errAlreadyAcked, cache.Put(task1))

	cache.Ack(401)
	assert.Equal(t, uint64(0), cache.Size())
	assert.Equal(t, 0, cache.Count())
	assert.Nil(t, cache.Get(100))
	assert.Nil(t, cache.Get(200))
	assert.Nil(t, cache.Get(300))
	assert.Nil(t, cache.Get(400))

	require.NoError(t, cache.Put(task5))
	assert.Equal(t, task5.ByteSize(), cache.Size())
	assert.Equal(t, 1, cache.Count())
	assert.Nil(t, cache.Get(100))
	assert.Nil(t, cache.Get(200))
	assert.Nil(t, cache.Get(300))
	assert.Nil(t, cache.Get(400))
	assert.Equal(t, task5, cache.Get(500))
}

func BenchmarkCache(b *testing.B) {
	cache := NewCache(dynamicproperties.GetIntPropertyFn(10000), dynamicproperties.GetIntPropertyFn(1000000), log.NewNoop())
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
