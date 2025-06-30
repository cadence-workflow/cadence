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

package sharddistributor

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/rpc"
)

func TestLegacyServiceStartStop(t *testing.T) {
	defer goleak.VerifyNone(t)

	testDispatcher := yarpc.NewDispatcher(yarpc.Config{Name: "test"})
	ctrl := gomock.NewController(t)
	factory := rpc.NewMockFactory(ctrl)
	factory.EXPECT().GetDispatcher().Return(testDispatcher)

	params := &resource.Params{
		Logger:        testlogger.New(t),
		MetricsClient: metrics.NewNoopMetricsClient(),
		RPCFactory:    factory,
		TimeSource:    clock.NewRealTimeSource(),
	}

	resourceMock := resource.NewMockResource(ctrl)
	resourceMock.EXPECT().Start()
	resourceMock.EXPECT().Stop()
	factoryMock := resource.NewMockResourceFactory(ctrl)
	factoryMock.EXPECT().NewResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(resourceMock, nil).AnyTimes()

	service, err := NewService(params, factoryMock)
	require.NoError(t, err)

	doneWG := sync.WaitGroup{}
	doneWG.Add(1)

	go func() {
		defer doneWG.Done()
		service.Start()
	}()

	time.Sleep(time.Millisecond) // The code assumes the service is started when calling Stop
	assert.Equal(t, common.DaemonStatusStarted, atomic.LoadInt32(&service.status))

	service.Stop()
	success := common.AwaitWaitGroup(&doneWG, time.Second)
	assert.True(t, success)
}
