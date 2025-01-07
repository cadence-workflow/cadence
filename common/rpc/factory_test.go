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

package rpc

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"

	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/service"
)

func TestNewFactory(t *testing.T) {
	ctrl := gomock.NewController(t)
	logger := testlogger.New(t)
	serviceName := "service"
	ob := NewMockOutboundsBuilder(ctrl)
	ob.EXPECT().Build(gomock.Any(), gomock.Any()).Return(&Outbounds{}, nil).Times(1)
	grpcMsgSize := 4 * 1024 * 1024
	f := NewFactory(logger, Params{
		ServiceName:     serviceName,
		TChannelAddress: "localhost:0",
		GRPCMaxMsgSize:  grpcMsgSize,
		GRPCAddress:     "localhost:0",
		HTTP: &httpParams{
			Address: "localhost:0",
		},
		OutboundsBuilder: ob,
	})

	if f == nil {
		t.Fatal("NewFactory returned nil")
	}

	assert.NotNil(t, f.GetDispatcher(), "GetDispatcher returned nil")
	assert.NotNil(t, f.GetTChannel(), "GetTChannel returned nil")
	assert.Equal(t, grpcMsgSize, f.GetMaxMessageSize(), "GetMaxMessageSize returned wrong value")
}

func TestStartStop(t *testing.T) {
	membersBySvc := map[string][]membership.HostInfo{
		service.Matching: {
			membership.NewHostInfo("localhost:9191"),
			membership.NewHostInfo("localhost:9192"),
		},
		service.History: {
			membership.NewHostInfo("localhost:8585"),
		},
	}

	tests := []struct {
		desc             string
		wantMembersBySvc map[string][]membership.HostInfo
		mockFn           func(*membership.MockResolver)
		wantStartErr     bool
	}{
		{
			desc:             "success",
			wantMembersBySvc: membersBySvc,
			mockFn: func(peerLister *membership.MockResolver) {
				for _, svc := range servicesToTalkP2P {
					peerLister.EXPECT().Subscribe(svc, factoryComponentName, gomock.Any()).
						DoAndReturn(func(service, name string, notifyChannel chan<- *membership.ChangedEvent) error {
							// Notify the channel once to validate listening logic is working
							notifyChannel <- &membership.ChangedEvent{}
							return nil
						}).Times(1)

					peerLister.EXPECT().Members(svc).Return(membersBySvc[svc], nil).Times(1)
					peerLister.EXPECT().Unsubscribe(svc, factoryComponentName).Return(nil).Times(1)
				}
			},
		},
		{
			desc:         "subscription to membership updates fail",
			wantStartErr: true,
			mockFn: func(peerLister *membership.MockResolver) {
				for i, svc := range servicesToTalkP2P {
					if i == 0 {
						// subscribe will only be called for the first service and after failing, it should not be called for the rest
						peerLister.EXPECT().Subscribe(svc, factoryComponentName, gomock.Any()).Return(errors.New("failed")).Times(1)
					}

					// subscribe will be called for all services during stop
					peerLister.EXPECT().Unsubscribe(svc, factoryComponentName).Return(nil).Times(1)
				}
			},
		},
		{
			desc:             "unsubscirption from membership updates fail",
			wantMembersBySvc: membersBySvc,
			mockFn: func(peerLister *membership.MockResolver) {
				for _, svc := range servicesToTalkP2P {
					peerLister.EXPECT().Subscribe(svc, factoryComponentName, gomock.Any()).
						DoAndReturn(func(service, name string, notifyChannel chan<- *membership.ChangedEvent) error {
							// Notify the channel once to validate listening logic is working
							notifyChannel <- &membership.ChangedEvent{}
							return nil
						}).Times(1)
					peerLister.EXPECT().Members(svc).Return(membersBySvc[svc], nil).Times(1)
					peerLister.EXPECT().Unsubscribe(svc, factoryComponentName).Return(errors.New("failed")).Times(1)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			logger := testlogger.New(t)
			serviceName := "service"
			ob := NewMockOutboundsBuilder(ctrl)
			var mu sync.Mutex
			gotMembers := make(map[string][]membership.HostInfo)
			outbounds := &Outbounds{
				onUpdatePeers: func(svc string, members []membership.HostInfo) {
					mu.Lock()
					defer mu.Unlock()
					gotMembers[svc] = members
				},
			}
			ob.EXPECT().Build(gomock.Any(), gomock.Any()).Return(outbounds, nil).Times(1)
			grpcMsgSize := 4 * 1024 * 1024
			f := NewFactory(logger, Params{
				ServiceName:     serviceName,
				TChannelAddress: "localhost:0",
				GRPCMaxMsgSize:  grpcMsgSize,
				GRPCAddress:     "localhost:0",
				HTTP: &httpParams{
					Address: "localhost:0",
				},
				OutboundsBuilder: ob,
			})

			peerLister := membership.NewMockResolver(ctrl)
			tc.mockFn(peerLister)

			if err := f.Start(peerLister); err != nil {
				if !tc.wantStartErr {
					t.Fatalf("Factory.Start() returned error: %v", err)
				}

				// start failed expectedly. do not proceed with rest of the validations
				f.Stop()
				return
			}

			// Wait for membership changes to be processed
			time.Sleep(100 * time.Millisecond)
			mu.Lock()
			assert.Equal(t, tc.wantMembersBySvc, gotMembers, "UpdatePeers not called with expected members")
			mu.Unlock()

			if err := f.Stop(); err != nil {
				t.Fatalf("Factory.Stop() returned error: %v", err)
			}

			goleak.VerifyNone(t)
		})
	}
}
