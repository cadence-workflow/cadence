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
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/service/sharddistributor/client/spectatorclient"
)

func TestShardDistributorResolver_Lookup_modeHashRing(t *testing.T) {
	resolver, ring, _ := newShardDistributorResolver(t)
	resolver.shardDistributionMode = func(...dynamicproperties.FilterOption) string {
		return string(modeKeyHashRing)
	}

	ring.EXPECT().Lookup("test-key").Return(HostInfo{addr: "test-addr"}, nil)

	host, err := resolver.Lookup("test-key")
	assert.NoError(t, err)
	assert.Equal(t, "test-addr", host.addr)
}

func TestShardDistributorResolver_Lookup_modeShardDistributor(t *testing.T) {
	resolver, _, shardDistributorMock := newShardDistributorResolver(t)
	resolver.shardDistributionMode = func(...dynamicproperties.FilterOption) string {
		return string(modeKeyShardDistributor)
	}

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

func TestShardDistributorResolver_Lookup_modeHashRingShadowShardDistributor(t *testing.T) {
	resolver, ring, shardDistributorMock := newShardDistributorResolver(t)
	resolver.shardDistributionMode = func(...dynamicproperties.FilterOption) string {
		return string(modeKeyHashRingShadowShardDistributor)
	}

	cases := []struct {
		name                   string
		hashRingAddr           string
		hashRingError          error
		shardDistributorHostIP string
		shardDistributorError  error
		expectedLog            string
	}{
		{
			name:                   "hash ring and shard distributor agree",
			hashRingAddr:           "127.0.0.1:7933",
			shardDistributorHostIP: "127.0.0.1",
		},
		{
			name:                   "hash ring and shard distributor disagree",
			hashRingAddr:           "127.0.0.1:7933",
			shardDistributorHostIP: "127.0.0.2",
			expectedLog:            "Shadow lookup mismatch",
		},
		{
			name:                  "shard distributor error",
			hashRingAddr:          "127.0.0.1:7933",
			shardDistributorError: assert.AnError,
			expectedLog:           "Failed to lookup in shard distributor shadow",
		},
		{
			name:                   "hash ring error, fallback to shard distributor",
			hashRingError:          assert.AnError,
			shardDistributorHostIP: "127.0.0.2",
		},
		{
			name:                  "both hash ring and shard distributor error",
			hashRingError:         assert.AnError,
			shardDistributorError: assert.AnError,
			expectedLog:           "Failed to lookup in shard distributor shadow",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logger, logs := testlogger.NewObserved(t)
			resolver.logger = logger

			ring.EXPECT().Lookup("test-key").Return(NewDetailedHostInfo(
				tc.hashRingAddr,
				"test-owner",
				PortMap{PortTchannel: 7933, PortGRPC: 7833},
			), tc.hashRingError)

			// Shard distributor is always called in shadow mode for comparison/fallback
			shardDistributorMock.EXPECT().GetShardOwner(gomock.Any(), "test-key").
				Return(&spectatorclient.ShardOwner{
					ExecutorID: "test-owner",
					Metadata: map[string]string{
						"hostIP":   tc.shardDistributorHostIP,
						"tchannel": "7933",
						"grpc":     "7833",
					},
				}, tc.shardDistributorError)

			host, err := resolver.Lookup("test-key")

			// Determine expected behavior based on new fallback logic
			if tc.hashRingError == nil {
				// Hash ring succeeded, return hash ring result
				assert.NoError(t, err)
				assert.Equal(t, tc.hashRingAddr, host.addr)
			} else if tc.shardDistributorError == nil {
				// Hash ring failed, fallback to shard distributor
				assert.NoError(t, err)
				assert.Equal(t, tc.shardDistributorHostIP+":7933", host.addr)
			} else {
				// Both failed
				assert.Equal(t, tc.shardDistributorError, err)
			}

			if tc.expectedLog != "" {
				if tc.expectedLog == "Shadow lookup mismatch" {
					// logDifferencesInHostInfo logs separately for tchannel and grpc ports
					assert.Equal(t, 2, logs.Len())
				} else {
					// Error cases only log once
					assert.Equal(t, 1, logs.Len())
					assert.Equal(t, 1, logs.FilterMessage(tc.expectedLog).Len())
				}
			} else {
				assert.Equal(t, 0, logs.Len())
			}
		})
	}
}

func TestShardDistributorResolver_Lookup_modeShardDistributorShadowHashRing(t *testing.T) {
	resolver, ring, shardDistributorMock := newShardDistributorResolver(t)
	resolver.shardDistributionMode = func(...dynamicproperties.FilterOption) string {
		return string(modeKeyShardDistributorShadowHashRing)
	}

	cases := []struct {
		name                   string
		shardDistributorHostIP string
		shardDistributorError  error
		hashRingAddr           string
		hashRingError          error
		expectedLog            string
	}{
		{
			name:                   "shard distributor and hash ring agree",
			shardDistributorHostIP: "127.0.0.1",
			hashRingAddr:           "127.0.0.1:7933",
		},
		{
			name:                   "shard distributor and hash ring disagree",
			shardDistributorHostIP: "127.0.0.1",
			hashRingAddr:           "127.0.0.2:7933",
			expectedLog:            "Shadow lookup mismatch",
		},
		{
			name:                   "hash ring error",
			shardDistributorHostIP: "127.0.0.1",
			hashRingError:          assert.AnError,
			expectedLog:            "Failed to lookup in hash ring shadow",
		},
		{
			name:                  "shard distributor error, fallback to hash ring",
			shardDistributorError: assert.AnError,
			hashRingAddr:          "127.0.0.2:7933",
		},
		{
			name:                  "both shard distributor and hash ring error",
			shardDistributorError: assert.AnError,
			hashRingError:         assert.AnError,
			expectedLog:           "Failed to lookup in hash ring shadow",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logger, logs := testlogger.NewObserved(t)
			resolver.logger = logger

			shardDistributorMock.EXPECT().GetShardOwner(gomock.Any(), "test-key").
				Return(&spectatorclient.ShardOwner{
					ExecutorID: "test-owner",
					Metadata: map[string]string{
						"hostIP":   tc.shardDistributorHostIP,
						"tchannel": "7933",
						"grpc":     "7833",
					},
				}, tc.shardDistributorError)

			// Hash ring is always called in shadow mode for comparison/fallback
			ring.EXPECT().Lookup("test-key").Return(NewDetailedHostInfo(
				tc.hashRingAddr,
				"test-owner",
				PortMap{PortTchannel: 7933, PortGRPC: 7833},
			), tc.hashRingError)

			host, err := resolver.Lookup("test-key")

			// Determine expected behavior based on new fallback logic
			if tc.shardDistributorError == nil {
				// Shard distributor succeeded, return shard distributor result
				assert.NoError(t, err)
				assert.Equal(t, tc.shardDistributorHostIP+":7933", host.addr)
			} else if tc.hashRingError == nil {
				// Shard distributor failed, fallback to hash ring
				assert.NoError(t, err)
				assert.Equal(t, tc.hashRingAddr, host.addr)
			} else {
				// Both failed
				assert.Equal(t, tc.hashRingError, err)
			}

			if tc.expectedLog != "" {
				if tc.expectedLog == "Shadow lookup mismatch" {
					// logDifferencesInHostInfo logs separately for tchannel and grpc ports
					assert.Equal(t, 2, logs.Len())
				} else {
					// Error cases only log once
					assert.Equal(t, 1, logs.Len())
					assert.Equal(t, 1, logs.FilterMessage(tc.expectedLog).Len())
				}
			} else {
				assert.Equal(t, 0, logs.Len())
			}
		})
	}
}

func TestShardDistributorResolver_Lookup_UnknownMode(t *testing.T) {
	resolver, ring, _ := newShardDistributorResolver(t)
	resolver.shardDistributionMode = func(...dynamicproperties.FilterOption) string {
		return "unknown"
	}

	ring.EXPECT().Lookup("test-key").Return(HostInfo{addr: "test-addr"}, nil)

	host, err := resolver.Lookup("test-key")
	assert.NoError(t, err)
	assert.Equal(t, "test-addr", host.addr)
}

/* Test all the simple proxies
 */
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

func newShardDistributorResolver(t *testing.T) (*shardDistributorResolver, *MockSingleProvider, *spectatorclient.MockSpectator) {
	ctrl := gomock.NewController(t)
	spectator := spectatorclient.NewMockSpectator(ctrl)
	shardDistributionMode := dynamicproperties.GetStringPropertyFn("")
	ring := NewMockSingleProvider(ctrl)
	logger := log.NewNoop()

	resolver := NewShardDistributorResolver(spectator, shardDistributionMode, ring, logger).(*shardDistributorResolver)

	return resolver, ring, spectator
}
