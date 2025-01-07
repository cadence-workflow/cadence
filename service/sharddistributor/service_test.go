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
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
	"go.uber.org/yarpc"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/resource"
)

func TestNewService(t *testing.T) {
	ctrl := gomock.NewController(t)
	resourceMock := resource.NewMockResource(ctrl)
	factoryMock := resource.NewMockResourceFactory(ctrl)
	factoryMock.EXPECT().NewResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(resourceMock, nil).AnyTimes()

	service, err := NewService(&resource.Params{}, factoryMock)
	assert.NoError(t, err)
	assert.NotNil(t, service)
}

func TestServiceStartStop(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctrl := gomock.NewController(t)

	testDispatcher := yarpc.NewDispatcher(yarpc.Config{Name: "test"})

	resourceMock := resource.NewMockResource(ctrl)
	resourceMock.EXPECT().GetLogger().Return(log.NewNoop()).AnyTimes()
	resourceMock.EXPECT().GetMembershipResolver().Return(nil).AnyTimes()
	resourceMock.EXPECT().GetMetricsClient().Return(nil).AnyTimes()
	resourceMock.EXPECT().GetDispatcher().Return(testDispatcher).AnyTimes()
	resourceMock.EXPECT().Start().Return()
	resourceMock.EXPECT().Stop().Return()

	service := &Service{
		Resource: resourceMock,
		status:   common.DaemonStatusInitialized,
		stopC:    make(chan struct{}),
	}

	go service.Start()

	time.Sleep(100 * time.Millisecond) // The code assumes the service is started when calling Stop
	assert.Equal(t, int32(common.DaemonStatusStarted), atomic.LoadInt32(&service.status))

	service.Stop()
	assert.Equal(t, int32(common.DaemonStatusStopped), atomic.LoadInt32(&service.status))
}

func TestStartAndStopDoesNotChangeStatusWhenAlreadyStopped(t *testing.T) {

	service := &Service{
		status: common.DaemonStatusStopped,
	}

	service.Start()
	assert.Equal(t, int32(common.DaemonStatusStopped), atomic.LoadInt32(&service.status))

	service.Stop()
	assert.Equal(t, int32(common.DaemonStatusStopped), atomic.LoadInt32(&service.status))
}
