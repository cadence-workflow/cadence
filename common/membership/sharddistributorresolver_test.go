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

package membership

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/service/sharddistributor/client/spectatorclient"
)

func TestShardDistributorResolver_Lookup(t *testing.T) {
	resolver, _, shardDistributorMock := newShardDistributorResolver(t)

	shardDistributorMock.EXPECT().GetShardOwner(gomock.Any(), "test-key").
		Return(&spectatorclient.ShardOwner{
			ExecutorID: "test-owner",
			Metadata: map[string]string{
				"hostIP":   "127.0.0.1",
				"tchannel": "7933",
				"grpc":     "7833",
			},
		}, nil)

	host, err := resolver.Lookup("test-key")
	assert.NoError(t, err)
	assert.Equal(t, "127.0.0.1:7933", host.addr)
}

func TestShardDistributorResolver_Lookup_ExcludedFallsBackToHashRing(t *testing.T) {
	resolver, ring, _ := newShardDistributorResolver(t)
	// Set percentage to 0 so all task lists are excluded
	resolver.percentageOnboarded = dynamicproperties.GetIntPropertyFn(0)

	ring.EXPECT().Lookup("test-key").Return(HostInfo{addr: "test-addr"}, nil)

	host, err := resolver.Lookup("test-key")
	assert.NoError(t, err)
	assert.Equal(t, "test-addr", host.addr)
}

func TestShardDistributorResolver_Lookup_NilSpectatorFallsBackToHashRing(t *testing.T) {
	ctrl := gomock.NewController(t)
	ring := NewMockSingleProvider(ctrl)
	logger := log.NewNoop()

	resolver := NewShardDistributorResolver(
		nil, // nil spectator
		dynamicproperties.GetBoolPropertyFn(false),
		dynamicproperties.GetIntPropertyFn(100),
		dynamicproperties.GetBoolPropertyFn(false),
		ring,
		logger,
	).(*shardDistributorResolver)

	ring.EXPECT().Lookup("test-key").Return(HostInfo{addr: "hash-ring-addr"}, nil)

	host, err := resolver.Lookup("test-key")
	assert.NoError(t, err)
	assert.Equal(t, "hash-ring-addr", host.addr)
}

func TestShardDistributorResolver_Lookup_EmergencyOffboardingFallsBackToHashRing(t *testing.T) {
	resolver, ring, _ := newShardDistributorResolver(t)
	// Emergency offboarding overrides the onboarding percentage
	resolver.emergencyOffboarding = dynamicproperties.GetBoolPropertyFn(true)
	resolver.percentageOnboarded = dynamicproperties.GetIntPropertyFn(100)

	ring.EXPECT().Lookup("test-key").Return(HostInfo{addr: "hash-ring-addr"}, nil)

	host, err := resolver.Lookup("test-key")
	assert.NoError(t, err)
	assert.Equal(t, "hash-ring-addr", host.addr)
}

func TestShardDistributorResolver_Start(t *testing.T) {
	resolver, ring, _ := newShardDistributorResolver(t)
	ring.EXPECT().Start().Times(1)
	resolver.Start()
}

func TestShardDistributorResolver_Stop(t *testing.T) {
	resolver, ring, _ := newShardDistributorResolver(t)
	ring.EXPECT().Stop().Times(1)
	resolver.Stop()
}

func TestShardDistributorResolver_Subscribe(t *testing.T) {
	resolver, ring, _ := newShardDistributorResolver(t)
	ring.EXPECT().Subscribe("test-name", gomock.Any()).Times(1)
	resolver.Subscribe("test-name", nil)
}

func TestShardDistributorResolver_UnSubscribe(t *testing.T) {
	resolver, ring, _ := newShardDistributorResolver(t)
	ring.EXPECT().Unsubscribe("test-name").Times(1)
	resolver.Unsubscribe("test-name")
}

func TestShardDistributorResolver_Members(t *testing.T) {
	resolver, ring, _ := newShardDistributorResolver(t)
	ring.EXPECT().Members().Return(nil)
	resolver.Members()
}

func TestShardDistributorResolver_MemberCount(t *testing.T) {
	resolver, ring, _ := newShardDistributorResolver(t)
	ring.EXPECT().MemberCount().Return(0)
	resolver.MemberCount()
}

func TestShardDistributorResolver_Refresh(t *testing.T) {
	resolver, ring, _ := newShardDistributorResolver(t)
	ring.EXPECT().Refresh().Times(1)
	resolver.Refresh()
}

func TestShardDistributorResolver_AddressToHost(t *testing.T) {
	resolver, ring, _ := newShardDistributorResolver(t)
	ring.EXPECT().AddressToHost("test").Return(HostInfo{}, nil)
	resolver.AddressToHost("test")
}

func TestShardDistributorResolver_Lookup_ExcludeShortLivedTaskLists(t *testing.T) {
	cases := []struct {
		name                       string
		excludeShortLivedTaskLists bool
		taskListName               string
		expectHashRing             bool
	}{
		{
			name:                       "exclude enabled with UUID tasklist uses hash ring",
			excludeShortLivedTaskLists: true,
			taskListName:               "tasklist-550e8400-e29b-41d4-a716-446655440000",
			expectHashRing:             true,
		},
		{
			name:                       "exclude enabled without UUID tasklist uses shard distributor",
			excludeShortLivedTaskLists: true,
			taskListName:               "my-regular-tasklist",
			expectHashRing:             false,
		},
		{
			name:                       "exclude disabled with UUID tasklist uses shard distributor",
			excludeShortLivedTaskLists: false,
			taskListName:               "tasklist-550e8400-e29b-41d4-a716-446655440000",
			expectHashRing:             false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			spectator := spectatorclient.NewMockSpectator(ctrl)
			ring := NewMockSingleProvider(ctrl)
			logger := log.NewNoop()

			resolver := NewShardDistributorResolver(
				spectator,
				dynamicproperties.GetBoolPropertyFn(tc.excludeShortLivedTaskLists),
				dynamicproperties.GetIntPropertyFn(100),
				dynamicproperties.GetBoolPropertyFn(false),
				ring,
				logger,
			).(*shardDistributorResolver)

			if tc.expectHashRing {
				ring.EXPECT().Lookup(tc.taskListName).Return(HostInfo{addr: "hash-ring-addr"}, nil)
			} else {
				spectator.EXPECT().GetShardOwner(gomock.Any(), tc.taskListName).
					Return(&spectatorclient.ShardOwner{
						ExecutorID: "test-owner",
						Metadata: map[string]string{
							"hostIP":   "127.0.0.1",
							"tchannel": "7933",
							"grpc":     "7833",
						},
					}, nil)
			}

			host, err := resolver.Lookup(tc.taskListName)
			assert.NoError(t, err)

			if tc.expectHashRing {
				assert.Equal(t, "hash-ring-addr", host.addr)
			} else {
				assert.Equal(t, "127.0.0.1:7933", host.addr)
			}
		})
	}
}

func newShardDistributorResolver(t *testing.T) (*shardDistributorResolver, *MockSingleProvider, *spectatorclient.MockSpectator) {
	ctrl := gomock.NewController(t)
	spectator := spectatorclient.NewMockSpectator(ctrl)
	excludeShortLivedTaskLists := dynamicproperties.GetBoolPropertyFn(false)
	percentageOnboarded := dynamicproperties.GetIntPropertyFn(100)
	ring := NewMockSingleProvider(ctrl)
	logger := log.NewNoop()

	resolver := NewShardDistributorResolver(spectator, excludeShortLivedTaskLists, percentageOnboarded, dynamicproperties.GetBoolPropertyFn(false), ring, logger).(*shardDistributorResolver)

	return resolver, ring, spectator
}
