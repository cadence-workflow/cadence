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

package shard

import (
	"sync"
	"testing"
)

// BenchmarkController_ShardIDs-96         52588224               293.9 ns/op            66 B/op          0 allocs/op
// go test -bench=. --benchtime=10s --benchmem
// goos: linux
// goarch: amd64
// pkg: github.com/uber/cadence/service/history/shard
// cpu: AMD EPYC 7B13
// With the old approach, the benchmark result is:
// BenchmarkController_ShardIDs-96            39314            324629 ns/op          272333 B/op         19 allocs/op
func BenchmarkController_ShardIDs(b *testing.B) {
	numShards := 16384
	historyShards := make(map[int]*historyShardsItem)
	for i := 0; i < numShards; i++ {
		historyShards[i] = &historyShardsItem{shardID: i}
	}
	shardController := &controller{
		historyShards: historyShards,
	}
	var wg sync.WaitGroup
	for i := 0; i < b.N; i++ {
		if i%1000 == 0 { // update is much much less frequent than read
			wg.Add(1)
			go func() {
				defer wg.Done()
				shardController.Lock()
				shardController.updateShardIDSnapshotLocked()
				shardController.Unlock()
			}()
		}
		shardController.ShardIDs()
		shardController.NumShards()
	}
	wg.Wait()
}
