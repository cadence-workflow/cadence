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

package poller

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/types"
)

func TestNewPollerHistory(t *testing.T) {
	p := NewPollerHistory(nil, nil)
	assert.NotNil(t, p)
	assert.NotNil(t, p.(*history).historyCache)
}

func TestUpdatePollerInfo(t *testing.T) {
	updated := false
	updateFn := func() {
		updated = true
	}
	mockCtrl := gomock.NewController(t)
	mockCache := cache.NewMockCache(mockCtrl)
	mockCache.EXPECT().Put(Identity("test"), &Info{IsolationGroup: "dca1"}).Return(nil)
	p := &history{
		onHistoryUpdatedFunc: updateFn,
		historyCache:         mockCache,
	}
	p.UpdatePollerInfo(Identity("test"), Info{IsolationGroup: "dca1"})
	assert.True(t, updated)
}

func TestHistory_HasPollerAfter(t *testing.T) {
	t.Run("empty historyCache", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		mockCache := cache.NewMockCache(mockCtrl)
		mockCache.EXPECT().Size().Return(0)

		p := &history{
			historyCache: mockCache,
		}
		assert.False(t, p.HasPollerAfter(time.Now()))
	})
	t.Run("no poller after", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		mockCache := cache.NewMockCache(mockCtrl)
		mockCache.EXPECT().Size().Return(2)
		mockIter := cache.NewMockIterator(mockCtrl)
		mockEntry := cache.NewMockEntry(mockCtrl)

		now := time.Now()

		mockCache.EXPECT().Iterator().Return(mockIter)
		gomock.InOrder(
			mockIter.EXPECT().HasNext().Return(true),
			mockIter.EXPECT().Next().Return(mockEntry),
			mockEntry.EXPECT().CreateTime().Return(now.Add(-time.Second)),
			mockIter.EXPECT().HasNext().Return(true),
			mockIter.EXPECT().Next().Return(mockEntry),
			mockEntry.EXPECT().CreateTime().Return(now.Add(-2*time.Second)),
			mockIter.EXPECT().HasNext().Return(false),
			mockIter.EXPECT().Close(),
		)

		p := &history{
			historyCache: mockCache,
		}
		assert.False(t, p.HasPollerAfter(now))
	})
	t.Run("has poller after", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		mockCache := cache.NewMockCache(mockCtrl)
		mockCache.EXPECT().Size().Return(2)
		mockIter := cache.NewMockIterator(mockCtrl)
		mockEntry := cache.NewMockEntry(mockCtrl)

		now := time.Now()

		mockCache.EXPECT().Iterator().Return(mockIter)
		gomock.InOrder(
			mockIter.EXPECT().HasNext().Return(true),
			mockIter.EXPECT().Next().Return(mockEntry),
			mockEntry.EXPECT().CreateTime().Return(now.Add(-time.Second)),
			mockIter.EXPECT().HasNext().Return(true),
			mockIter.EXPECT().Next().Return(mockEntry),
			mockEntry.EXPECT().CreateTime().Return(now.Add(time.Second)),
			mockIter.EXPECT().Close(),
		)

		p := &history{
			historyCache: mockCache,
		}
		assert.True(t, p.HasPollerAfter(now))
	})
}

func TestGetPollerCount(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockCache := cache.NewMockCache(mockCtrl)
	mockCache.EXPECT().Size().Return(10)
	p := &history{
		historyCache: mockCache,
	}
	assert.Equal(t, 10, p.GetPollerCount())
}

func TestGetPollerInfo(t *testing.T) {
	t.Run("with_time_filter", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		mockCache := cache.NewMockCache(mockCtrl)
		mockIter := cache.NewMockIterator(mockCtrl)
		mockEntry := cache.NewMockEntry(mockCtrl)

		mockCache.EXPECT().Size().Return(2)

		mockCache.EXPECT().Iterator().Return(mockIter)
		gomock.InOrder(
			mockIter.EXPECT().HasNext().Return(true),
			mockIter.EXPECT().Next().Return(mockEntry),
			mockEntry.EXPECT().Key().Return(Identity("test0")),
			mockEntry.EXPECT().Value().Return(&Info{IsolationGroup: "dca1", RatePerSecond: 1.0}),
			mockEntry.EXPECT().CreateTime().Return(time.UnixMilli(1000)),
			mockIter.EXPECT().HasNext().Return(true),
			mockIter.EXPECT().Next().Return(mockEntry),
			mockEntry.EXPECT().Key().Return(Identity("test1")),
			mockEntry.EXPECT().Value().Return(&Info{IsolationGroup: "dca2", RatePerSecond: 2.0}),
			mockEntry.EXPECT().CreateTime().Return(time.UnixMilli(0)),
			mockIter.EXPECT().HasNext().Return(false),
			mockIter.EXPECT().Close(),
		)
		p := &history{
			historyCache: mockCache,
		}
		info := p.GetPollerInfo(time.UnixMilli(500))
		assert.Equal(t, []*types.PollerInfo{
			{
				Identity:       "test0",
				LastAccessTime: common.Ptr(time.UnixMilli(1000).UnixNano()),
				RatePerSecond:  1.0,
			},
		}, info)
	})
	t.Run("no_time_filter", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		mockCache := cache.NewMockCache(mockCtrl)
		mockIter := cache.NewMockIterator(mockCtrl)
		mockEntry := cache.NewMockEntry(mockCtrl)
		mockCache.EXPECT().Size().Return(2)
		mockCache.EXPECT().Iterator().Return(mockIter)
		gomock.InOrder(
			mockIter.EXPECT().HasNext().Return(true),
			mockIter.EXPECT().Next().Return(mockEntry),
			mockEntry.EXPECT().Key().Return(Identity("test0")),
			mockEntry.EXPECT().Value().Return(&Info{IsolationGroup: "dca1", RatePerSecond: 1.0}),
			mockEntry.EXPECT().CreateTime().Return(time.UnixMilli(1000)),
			mockIter.EXPECT().HasNext().Return(true),
			mockIter.EXPECT().Next().Return(mockEntry),
			mockEntry.EXPECT().Key().Return(Identity("test1")),
			mockEntry.EXPECT().Value().Return(&Info{IsolationGroup: "dca2", RatePerSecond: 2.0}),
			mockEntry.EXPECT().CreateTime().Return(time.UnixMilli(0)),
			mockIter.EXPECT().HasNext().Return(false),
			mockIter.EXPECT().Close(),
		)
		p := &history{
			historyCache: mockCache,
		}
		info := p.GetPollerInfo(time.Time{})
		assert.Equal(t, []*types.PollerInfo{
			{
				Identity:       "test0",
				LastAccessTime: common.Ptr(time.UnixMilli(1000).UnixNano()),
				RatePerSecond:  1.0,
			},
			{
				Identity:       "test1",
				LastAccessTime: common.Ptr(time.UnixMilli(0).UnixNano()),
				RatePerSecond:  2.0,
			},
		}, info)
	})
}

func TestGetPollerIsolationGroups(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockCache := cache.NewMockCache(mockCtrl)
	mockIter := cache.NewMockIterator(mockCtrl)
	mockEntry := cache.NewMockEntry(mockCtrl)

	mockCache.EXPECT().Iterator().Return(mockIter)
	gomock.InOrder(
		mockIter.EXPECT().HasNext().Return(true),
		mockIter.EXPECT().Next().Return(mockEntry),
		mockEntry.EXPECT().Value().Return(&Info{IsolationGroup: "dca1", RatePerSecond: 1.0}),
		mockEntry.EXPECT().CreateTime().Return(time.UnixMilli(1000)),
		mockIter.EXPECT().HasNext().Return(true),
		mockIter.EXPECT().Next().Return(mockEntry),
		mockEntry.EXPECT().Value().Return(&Info{IsolationGroup: "dca2", RatePerSecond: 2.0}),
		mockEntry.EXPECT().CreateTime().Return(time.UnixMilli(0)),
		mockIter.EXPECT().HasNext().Return(false),
		mockIter.EXPECT().Close(),
	)
	p := &history{
		historyCache: mockCache,
	}
	groups := p.GetPollerIsolationGroups(time.UnixMilli(500))
	assert.Equal(t, map[string]int{"dca1": 1}, groups)
}
