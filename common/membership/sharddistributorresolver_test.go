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

	"github.com/uber/cadence/client/sharddistributor"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/types"
)

func TestShardDistributorResolver_Lookup_modeHashRing(t *testing.T) {
	resolver, ring, _ := newShardDistributorResolver(t)
	resolver.shardDistributionMode = func(...dynamicconfig.FilterOption) string {
		return string(modeKeyHashRing)
	}

	ring.EXPECT().LookupRaw("test-key").Return("test-owner", nil)
	ring.EXPECT().AddressToHost("test-owner").Return(HostInfo{addr: "test-addr"}, nil)

	host, err := resolver.Lookup("test-key")
	assert.NoError(t, err)
	assert.Equal(t, "test-addr", host.addr)
}

func TestShardDistributorResolver_Lookup_modeShardDistributor(t *testing.T) {
	resolver, ring, shardDistributorMock := newShardDistributorResolver(t)
	resolver.shardDistributionMode = func(...dynamicconfig.FilterOption) string {
		return string(modeKeyShardDistributor)
	}

	shardDistributorMock.EXPECT().GetShardOwner(gomock.Any(),
		&types.GetShardOwnerRequest{ShardKey: "test-key", Namespace: "test-namespace"}).
		Return(&types.GetShardOwnerResponse{Owner: "test-owner"}, nil)
	ring.EXPECT().AddressToHost("test-owner").Return(HostInfo{addr: "test-addr"}, nil)

	host, err := resolver.Lookup("test-key")
	assert.NoError(t, err)
	assert.Equal(t, "test-addr", host.addr)
}

func TestShardDistributorResolver_Lookup_modeHashRingShadowShardDistributor(t *testing.T) {
	resolver, ring, shardDistributorMock := newShardDistributorResolver(t)
	resolver.shardDistributionMode = func(...dynamicconfig.FilterOption) string {
		return string(modeKeyHashRingShadowShardDistributor)
	}

	cases := []struct {
		name                  string
		hashRingOwner         string
		hashRingError         error
		shardDistributorOwner string
		shardDistributorError error
		expectedLog           string
	}{
		{
			name:                  "hash ring and shard distributor agree",
			hashRingOwner:         "test-owner",
			shardDistributorOwner: "test-owner",
		},
		{
			name:                  "hash ring and shard distributor disagree",
			hashRingOwner:         "test-owner",
			shardDistributorOwner: "test-owner-2",
			expectedLog:           "Shadow lookup mismatch",
		},
		{
			name:                  "shard distributor error",
			hashRingOwner:         "test-owner",
			shardDistributorError: assert.AnError,
			expectedLog:           "Failed to lookup in shard distributor shadow",
		},
		{
			name:          "hash ring error",
			hashRingError: assert.AnError,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logger, logs := testlogger.NewObserved(t)
			resolver.logger = logger

			ring.EXPECT().LookupRaw("test-key").Return(tc.hashRingOwner, tc.hashRingError)
			// If the hash ring lookup fails, we should just bail out and not call the shard distributor
			if tc.hashRingError == nil {
				shardDistributorMock.EXPECT().GetShardOwner(gomock.Any(),
					&types.GetShardOwnerRequest{ShardKey: "test-key", Namespace: "test-namespace"}).
					Return(&types.GetShardOwnerResponse{Owner: tc.shardDistributorOwner}, tc.shardDistributorError)

				ring.EXPECT().AddressToHost("test-owner").Return(HostInfo{addr: "test-addr"}, nil)
			}

			host, err := resolver.Lookup("test-key")
			assert.Equal(t, err, tc.hashRingError)

			if tc.hashRingError == nil {
				assert.Equal(t, "test-addr", host.addr)
			}

			if tc.expectedLog != "" {
				assert.Equal(t, 1, logs.Len())
				assert.Equal(t, 1, logs.FilterMessage(tc.expectedLog).Len())
			} else {
				assert.Equal(t, 0, logs.Len())
			}
		})
	}
}

func TestShardDistributorResolver_Lookup_modeShardDistributorShadowHashRing(t *testing.T) {
	resolver, ring, shardDistributorMock := newShardDistributorResolver(t)
	resolver.shardDistributionMode = func(...dynamicconfig.FilterOption) string {
		return string(modeKeyShardDistributorShadowHashRing)
	}

	cases := []struct {
		name                  string
		shardDistributorOwner string
		shardDistributorError error
		hashRingOwner         string
		hashRingError         error
		expectedLog           string
	}{
		{
			name:                  "shard distributor and hash ring agree",
			shardDistributorOwner: "test-owner",
			hashRingOwner:         "test-owner",
		},
		{
			name:                  "shard distributor and hash ring disagree",
			shardDistributorOwner: "test-owner",
			hashRingOwner:         "test-owner-2",
			expectedLog:           "Shadow lookup mismatch",
		},
		{
			name:                  "hash ring error",
			shardDistributorOwner: "test-owner",
			hashRingError:         assert.AnError,
			expectedLog:           "Failed to lookup in hash ring shadow",
		},
		{
			name:                  "shard distributor error",
			shardDistributorError: assert.AnError,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logger, logs := testlogger.NewObserved(t)
			resolver.logger = logger

			shardDistributorMock.EXPECT().GetShardOwner(gomock.Any(),
				&types.GetShardOwnerRequest{ShardKey: "test-key", Namespace: "test-namespace"}).
				Return(&types.GetShardOwnerResponse{Owner: tc.shardDistributorOwner}, tc.shardDistributorError)

			// If the hash ring lookup fails, we should just bail out and not call the shard distributor
			if tc.shardDistributorError == nil {
				ring.EXPECT().LookupRaw("test-key").Return(tc.hashRingOwner, tc.hashRingError)
				ring.EXPECT().AddressToHost("test-owner").Return(HostInfo{addr: "test-addr"}, nil)
			}

			host, err := resolver.Lookup("test-key")
			assert.Equal(t, err, tc.shardDistributorError)

			if tc.shardDistributorError == nil {
				assert.Equal(t, "test-addr", host.addr)
			}

			if tc.expectedLog != "" {
				assert.Equal(t, 1, logs.Len())
				assert.Equal(t, 1, logs.FilterMessage(tc.expectedLog).Len())
			} else {
				assert.Equal(t, 0, logs.Len())
			}
		})
	}
}

func TestShardDistributorResolver_Lookup_UnknownMode(t *testing.T) {
	resolver, ring, _ := newShardDistributorResolver(t)
	resolver.shardDistributionMode = func(...dynamicconfig.FilterOption) string {
		return "unknown"
	}

	ring.EXPECT().LookupRaw("test-key").Return("test-owner", nil)
	ring.EXPECT().AddressToHost("test-owner").Return(HostInfo{addr: "test-addr"}, nil)

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

func newShardDistributorResolver(t *testing.T) (*shardDistributorResolver, *MockSingleProvider, *sharddistributor.MockClient) {
	ctrl := gomock.NewController(t)
	namespace := "test-namespace"
	client := sharddistributor.NewMockClient(ctrl)
	shardDistributionMode := dynamicconfig.GetStringPropertyFn("")
	ring := NewMockSingleProvider(ctrl)
	logger := log.NewNoop()

	resolver := NewShardDistributorResolver(namespace, client, shardDistributionMode, ring, logger).(*shardDistributorResolver)

	return resolver, ring, client
}
