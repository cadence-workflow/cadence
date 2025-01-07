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

package locks

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestBasicLocking(t *testing.T) {
	lock := NewMutex()
	err1 := lock.Lock(context.Background())
	require.Nil(t, err1)
	lock.Unlock()
}

func TestExpiredContext(t *testing.T) {
	lock := NewMutex()
	err1 := lock.Lock(context.Background())
	require.Nil(t, err1)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	cancel()
	err2 := lock.Lock(ctx)
	require.NotNil(t, err2)
	require.Equal(t, err2, ctx.Err())

	lock.Unlock()

	err3 := lock.Lock(context.Background())
	require.Nil(t, err3)
	lock.Unlock()
}

func BenchmarkLock(b *testing.B) {
	l := NewMutex()
	ctx := context.Background()
	for n := 0; n < b.N; n++ {
		_ = l.Lock(ctx)
		l.Unlock()
	}
}

func BenchmarkStdlibLock(b *testing.B) {
	var m sync.Mutex
	for n := 0; n < b.N; n++ {
		m.Lock()
		m.Unlock()
	}
}

func BenchmarkParallelLock(b *testing.B) {
	l := NewMutex()
	ctx := context.Background()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = l.Lock(ctx)
			l.Unlock()
		}
	})
}

func BenchmarkParallelStdlibLock(b *testing.B) {
	var m sync.Mutex
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m.Lock()
			m.Unlock()
		}
	})
}

func TestShouldBeFatal(t *testing.T) {
	t.Run("stdlib", func(t *testing.T) {
		fatalTest(t)
		var m sync.Mutex
		m.Unlock()
	})
	t.Run("custom", func(t *testing.T) {
		fatalTest(t)
		NewMutex().Unlock()
	})
}

func fatalTest(t *testing.T) {
	t.Helper()
	if os.Getenv("RUN_FATAL_TESTS") != "true" {
		t.Skipf(
			"This test intentionally crashes the process, and cannot be tested normally.\n"+
				"Check output manually when running this test on its own:\n"+
				"\tRUN_FATAL_TESTS=true go test -test.run %q %v",
			t.Name(), "github.com/uber/cadence/common/locks",
		)
	} else {
		t.Cleanup(func() {
			t.Fatal("process should have crashed, and this should be unreachable")
		})
	}
}

func TestConcurrently(t *testing.T) {
	defer goleak.VerifyNone(t)

	const (
		tryDuration = time.Millisecond // long enough to ensure at least one acquire
		tries       = 1000
	)

	// attempt to cancel all attempts within a reasonable amount of time
	ctx, cancel := context.WithTimeout(context.Background(), tryDuration*tries*10)
	defer cancel()
	success, err := int32(0), int32(0)

	m := NewMutex()
	var wg sync.WaitGroup
	for i := 0; i < tries; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(ctx, tryDuration)
			defer cancel()
			if m.Lock(ctx) == nil {
				atomic.AddInt32(&success, 1)
				time.Sleep(tryDuration / 10) // sleep for a fraction of the time, to guarantee some timeouts
				m.Unlock()
			} else {
				atomic.AddInt32(&err, 1)
			}
		}()
	}
	wg.Wait()

	t.Logf("success: %v, err: %v", success, err)
	assert.Greater(t, success, int32(1), "should have successfully acquired more than once, as once could be a fluke")
	assert.NotZero(t, err, "should have timed out at least once")
}
