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

package queuev2

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/shard"
)

func testImmediateOptions(overrides ...func(*cachedQueueReaderOptions)) *cachedQueueReaderOptions {
	opts := &cachedQueueReaderOptions{
		Mode:                 dynamicproperties.GetStringPropertyFnFilteredByShardID("enabled"),
		MaxSize:              dynamicproperties.GetIntPropertyFn(100),
		ShadowSampleInterval: dynamicproperties.GetDurationPropertyFn(0),
	}
	for _, o := range overrides {
		if o == nil {
			continue
		}
		o(opts)
	}
	return opts
}

// setupImmediateCachedReader builds a transfer cached reader backed by a real in-memory queue
// and a mocked base reader.
func setupImmediateCachedReader(
	t *testing.T,
	ctrl *gomock.Controller,
	overrides ...func(*cachedQueueReaderOptions),
) (*cachedImmediateQueueReader, *MockQueueReader) {
	t.Helper()
	mockShard := shard.NewMockContext(ctrl)
	mockShard.EXPECT().GetRangeID().Return(int64(0)).AnyTimes()
	mockShard.EXPECT().GetConfig().Return(&config.Config{RangeSizeBits: 20}).AnyTimes()
	mockShard.EXPECT().GetShardID().Return(0).AnyTimes()

	mockBase := NewMockQueueReader(ctrl)
	r := newCachedImmediateQueueReaderWithOptions(
		mockBase,
		newInMemQueue(),
		mockShard,
		clock.NewMockedTimeSource(),
		testlogger.New(t),
		metrics.NoopScope,
		testImmediateOptions(overrides...),
	)
	return r, mockBase
}

// TestCachedImmediateQueueReader_StartStop verifies the lifecycle only flips state: there is no
// background goroutine (unlike the scheduled reader), so goleak must see nothing leak, and both
// Start and Stop are idempotent.
func TestCachedImmediateQueueReader_StartStop(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctrl := gomock.NewController(t)
	r, _ := setupImmediateCachedReader(t, ctrl)

	require.Equal(t, common.DaemonStatusInitialized, r.status)

	r.Start()
	r.Start() // idempotent
	require.Equal(t, common.DaemonStatusStarted, r.status)

	r.Stop()
	r.Stop() // idempotent
	require.Equal(t, common.DaemonStatusStopped, r.status)
}
