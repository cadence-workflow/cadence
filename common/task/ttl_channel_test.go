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

package task

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTTLChannel(t *testing.T) {
	tests := []struct {
		name       string
		bufferSize int
	}{
		{
			name:       "unbuffered channel",
			bufferSize: 0,
		},
		{
			name:       "small buffer",
			bufferSize: 10,
		},
		{
			name:       "large buffer",
			bufferSize: 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := NewTTLChannel[int](tt.bufferSize)
			require.NotNil(t, ch)
			assert.NotNil(t, ch.Chan())
			assert.Equal(t, tt.bufferSize, ch.Cap())
			assert.Equal(t, 0, ch.Len())
			assert.Equal(t, int32(0), ch.RefCount())
		})
	}
}

func TestTTLChannel_RefCount(t *testing.T) {
	ch := NewTTLChannel[int](10)

	// Initial ref count should be 0
	assert.Equal(t, int32(0), ch.RefCount())

	// Add refs
	ch.IncRef()
	assert.Equal(t, int32(1), ch.RefCount())

	ch.IncRef()
	assert.Equal(t, int32(2), ch.RefCount())

	ch.IncRef()
	assert.Equal(t, int32(3), ch.RefCount())

	// Decrement refs
	ch.DecRef()
	assert.Equal(t, int32(2), ch.RefCount())

	ch.DecRef()
	assert.Equal(t, int32(1), ch.RefCount())

	ch.DecRef()
	assert.Equal(t, int32(0), ch.RefCount())
}

func TestTTLChannel_RefCount_MultipleAddDec(t *testing.T) {
	ch := NewTTLChannel[string](5)

	// Add multiple refs
	for i := 0; i < 100; i++ {
		ch.IncRef()
	}
	assert.Equal(t, int32(100), ch.RefCount())

	// Decrement all refs
	for i := 0; i < 100; i++ {
		ch.DecRef()
	}
	assert.Equal(t, int32(0), ch.RefCount())
}

func TestTTLChannel_LastWriteTime(t *testing.T) {
	ch := NewTTLChannel[int](10)

	// Initial last write time should be zero (Unix epoch)
	assert.Equal(t, time.Unix(0, 0), ch.LastWriteTime())

	// Update last write time
	now := time.Now()
	ch.UpdateLastWriteTime(now)

	// Should return the updated time (truncated to seconds)
	expected := time.Unix(now.Unix(), 0)
	assert.Equal(t, expected, ch.LastWriteTime())

	// Update with a different time
	later := now.Add(1 * time.Hour)
	ch.UpdateLastWriteTime(later)

	expected = time.Unix(later.Unix(), 0)
	assert.Equal(t, expected, ch.LastWriteTime())
}

func TestTTLChannel_Chan(t *testing.T) {
	ch := NewTTLChannel[int](10)

	// Should return the underlying channel
	c := ch.Chan()
	require.NotNil(t, c)

	// Should be able to send and receive
	c <- 42
	val := <-c
	assert.Equal(t, 42, val)
}

func TestTTLChannel_Len(t *testing.T) {
	ch := NewTTLChannel[int](10)

	// Initially empty
	assert.Equal(t, 0, ch.Len())

	// Add items
	ch.Chan() <- 1
	assert.Equal(t, 1, ch.Len())

	ch.Chan() <- 2
	assert.Equal(t, 2, ch.Len())

	ch.Chan() <- 3
	assert.Equal(t, 3, ch.Len())

	// Remove items
	<-ch.Chan()
	assert.Equal(t, 2, ch.Len())

	<-ch.Chan()
	assert.Equal(t, 1, ch.Len())

	<-ch.Chan()
	assert.Equal(t, 0, ch.Len())
}

func TestTTLChannel_Cap(t *testing.T) {
	tests := []struct {
		name       string
		bufferSize int
	}{
		{
			name:       "unbuffered",
			bufferSize: 0,
		},
		{
			name:       "buffered 1",
			bufferSize: 1,
		},
		{
			name:       "buffered 100",
			bufferSize: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := NewTTLChannel[int](tt.bufferSize)
			assert.Equal(t, tt.bufferSize, ch.Cap())
		})
	}
}

func TestTTLChannel_ShouldCleanup(t *testing.T) {
	tests := []struct {
		name           string
		refCount       int32
		channelLen     int
		timeSinceWrite time.Duration
		ttl            time.Duration
		expected       bool
	}{
		{
			name:           "should cleanup - all conditions met",
			refCount:       0,
			channelLen:     0,
			timeSinceWrite: 2 * time.Hour,
			ttl:            1 * time.Hour,
			expected:       true,
		},
		{
			name:           "should not cleanup - has references",
			refCount:       1,
			channelLen:     0,
			timeSinceWrite: 2 * time.Hour,
			ttl:            1 * time.Hour,
			expected:       false,
		},
		{
			name:           "should not cleanup - channel not empty",
			refCount:       0,
			channelLen:     1,
			timeSinceWrite: 2 * time.Hour,
			ttl:            1 * time.Hour,
			expected:       false,
		},
		{
			name:           "should not cleanup - not expired",
			refCount:       0,
			channelLen:     0,
			timeSinceWrite: 30 * time.Minute,
			ttl:            1 * time.Hour,
			expected:       false,
		},
		{
			name:           "should not cleanup - just before TTL boundary",
			refCount:       0,
			channelLen:     0,
			timeSinceWrite: 1*time.Hour - 1*time.Second,
			ttl:            1 * time.Hour,
			expected:       false,
		},
		{
			name:           "should cleanup - slightly past TTL",
			refCount:       0,
			channelLen:     0,
			timeSinceWrite: 1*time.Hour + 1*time.Second,
			ttl:            1 * time.Hour,
			expected:       true,
		},
		{
			name:           "should not cleanup - multiple issues",
			refCount:       2,
			channelLen:     3,
			timeSinceWrite: 30 * time.Minute,
			ttl:            1 * time.Hour,
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := NewTTLChannel[int](10)

			// Set up ref count
			for i := int32(0); i < tt.refCount; i++ {
				ch.IncRef()
			}

			// Set up channel length
			for i := 0; i < tt.channelLen; i++ {
				ch.Chan() <- i
			}

			// Set up last write time
			lastWriteTime := time.Now().Add(-tt.timeSinceWrite)
			ch.UpdateLastWriteTime(lastWriteTime)

			// Check should cleanup
			now := time.Now()
			result := ch.ShouldCleanup(now, tt.ttl)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTTLChannel_ShouldCleanup_EdgeCases(t *testing.T) {
	t.Run("zero TTL", func(t *testing.T) {
		ch := NewTTLChannel[int](10)
		// Use time truncated to seconds to match what gets stored
		now := time.Unix(time.Now().Unix(), 0)
		ch.UpdateLastWriteTime(now)

		// Even with zero TTL, should cleanup only if time has passed
		assert.False(t, ch.ShouldCleanup(now, 0))
		assert.True(t, ch.ShouldCleanup(now.Add(1*time.Second), 0))
	})

	t.Run("never written", func(t *testing.T) {
		ch := NewTTLChannel[int](10)
		now := time.Now()
		ttl := 1 * time.Hour

		// Channel was never written to (lastWriteTime is Unix epoch)
		// Should cleanup since time.Now() - Unix(0) > ttl
		assert.True(t, ch.ShouldCleanup(now, ttl))
	})

	t.Run("negative ref count", func(t *testing.T) {
		ch := NewTTLChannel[int](10)
		now := time.Now()
		ch.UpdateLastWriteTime(now.Add(-2 * time.Hour))

		// Manually set negative ref count (shouldn't happen in practice)
		ch.DecRef()
		assert.Equal(t, int32(-1), ch.RefCount())

		// Should not cleanup with negative ref count
		// (even though it's an error state, better safe than sorry)
		assert.False(t, ch.ShouldCleanup(now, 1*time.Hour))
	})
}

func TestTTLChannel_ConcurrentRefCount(t *testing.T) {
	ch := NewTTLChannel[int](10)
	iterations := 1000

	// Concurrently add refs
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < iterations; j++ {
				ch.IncRef()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have 10 * iterations refs
	expected := int32(10 * iterations)
	assert.Equal(t, expected, ch.RefCount())

	// Concurrently decrement refs
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < iterations; j++ {
				ch.DecRef()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should be back to 0
	assert.Equal(t, int32(0), ch.RefCount())
}

func TestTTLChannel_ConcurrentTimeUpdate(t *testing.T) {
	ch := NewTTLChannel[int](10)
	iterations := 100

	// Concurrently update time
	done := make(chan bool)
	baseTime := time.Now()

	for i := 0; i < 10; i++ {
		go func(offset int) {
			for j := 0; j < iterations; j++ {
				ch.UpdateLastWriteTime(baseTime.Add(time.Duration(offset+j) * time.Second))
			}
			done <- true
		}(i * iterations)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have some valid time (exact value depends on interleaving)
	lastTime := ch.LastWriteTime()
	assert.True(t, !lastTime.Before(baseTime), "last write time should not be before base time")
}
