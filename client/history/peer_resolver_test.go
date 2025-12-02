// Copyright (c) 2021 Uber Technologies, Inc.
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

package history

import (
	"errors"
	"slices"
	"testing"

	"github.com/dgryski/go-farm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber/ringpop-go/hashring"
	gomock "go.uber.org/mock/gomock"
	"golang.org/x/exp/maps"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/service"
)

func TestPeerResolver(t *testing.T) {
	numShards := 123
	t.Run("FromIDs", func(t *testing.T) {
		controller := gomock.NewController(t)
		serviceResolver := membership.NewMockResolver(controller)
		serviceResolver.EXPECT().Lookup(
			service.History, string(rune(common.DomainIDToHistoryShard("domainID", numShards)))).Return(
			membership.NewDetailedHostInfo(
				"domainHost:123",
				"domainHost_123",
				membership.PortMap{membership.PortTchannel: 1234}),
			nil)
		serviceResolver.EXPECT().Lookup(service.History, string(rune(common.WorkflowIDToHistoryShard("workflowID", numShards)))).Return(
			membership.NewDetailedHostInfo(
				"workflowHost:123",
				"workflow",
				membership.PortMap{membership.PortTchannel: 1235, membership.PortGRPC: 1666}), nil)

		serviceResolver.EXPECT().Lookup(service.History, string(rune(99))).Return(
			membership.NewDetailedHostInfo(
				"shardHost:123",
				"shard_123",
				membership.PortMap{membership.PortTchannel: 1235}),
			nil)

		serviceResolver.EXPECT().Lookup(service.History, string(rune(11))).Return(membership.HostInfo{}, assert.AnError)

		r := NewPeerResolver(numShards, serviceResolver, membership.PortTchannel)

		peer, err := r.FromDomainID("domainID")
		assert.NoError(t, err)
		assert.Equal(t, "domainHost:1234", peer)

		peer, err = r.FromWorkflowID("workflowID")
		assert.NoError(t, err)
		assert.Equal(t, "workflowHost:1235", peer)

		peer, err = r.FromShardID(99)
		assert.NoError(t, err)
		assert.Equal(t, "shardHost:1235", peer)

		_, err = r.FromShardID(11)
		assert.Error(t, err)
	})

	t.Run("FromHostAddress", func(t *testing.T) {
		tests := []struct {
			name      string
			address   string
			mock      func(*membership.MockResolver)
			want      string
			wantError bool
		}{
			{
				name:    "success",
				address: "addressHost:123",
				mock: func(mr *membership.MockResolver) {
					mr.EXPECT().LookupByAddress(service.History, "addressHost:123").Return(
						membership.NewDetailedHostInfo(
							"addressHost:123",
							"address",
							membership.PortMap{membership.PortTchannel: 1235, membership.PortGRPC: 1666}),
						nil,
					)
				},
				want: "addressHost:1235",
			},
			{
				name:    "invalid address",
				address: "invalid address",
				mock: func(mr *membership.MockResolver) {
					mr.EXPECT().LookupByAddress(service.History, "invalid address").Return(
						membership.HostInfo{},
						errors.New("host not found"),
					)
				},
				wantError: true,
			},
			{
				name:    "fail on no port",
				address: "addressHost:123",
				mock: func(mr *membership.MockResolver) {
					mr.EXPECT().LookupByAddress(service.History, "addressHost:123").Return(
						membership.NewDetailedHostInfo(
							"addressHost:123",
							"address",
							membership.PortMap{}),
						nil,
					)
				},
				wantError: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				controller := gomock.NewController(t)
				serviceResolver := membership.NewMockResolver(controller)
				tt.mock(serviceResolver)
				r := NewPeerResolver(numShards, serviceResolver, membership.PortTchannel)
				res, err := r.FromHostAddress(tt.address)
				if tt.wantError {
					assert.True(t, common.IsServiceTransientError(err))
				} else {
					assert.Equal(t, tt.want, res)
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("GetAllPeers", func(t *testing.T) {
		tests := []struct {
			name      string
			mock      func(*membership.MockResolver)
			want      []string
			wantError bool
		}{
			{
				name: "success",
				mock: func(mr *membership.MockResolver) {
					mr.EXPECT().Members(service.History).Return(
						[]membership.HostInfo{
							membership.NewDetailedHostInfo(
								"host1:123",
								"address",
								membership.PortMap{membership.PortTchannel: 1235, membership.PortGRPC: 1666}),
							membership.NewDetailedHostInfo(
								"host2:123",
								"address",
								membership.PortMap{membership.PortTchannel: 1235, membership.PortGRPC: 1666}),
						},
						nil,
					)
				},
				want: []string{"host1:1235", "host2:1235"},
			},
			{
				name: "failed on peer resolve",
				mock: func(mr *membership.MockResolver) {
					mr.EXPECT().Members(service.History).Return(nil, assert.AnError)

				},
				wantError: true,
			},
			{
				name: "failed on no port",
				mock: func(mr *membership.MockResolver) {
					mr.EXPECT().Members(service.History).Return(
						[]membership.HostInfo{
							membership.NewDetailedHostInfo(
								"host1:123",
								"address",
								membership.PortMap{membership.PortTchannel: 1235, membership.PortGRPC: 1666}),
							membership.NewDetailedHostInfo(
								"host2:123",
								"address",
								membership.PortMap{membership.PortGRPC: 1666}),
						},
						nil,
					)
				},
				wantError: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				controller := gomock.NewController(t)
				serviceResolver := membership.NewMockResolver(controller)
				tt.mock(serviceResolver)
				r := NewPeerResolver(numShards, serviceResolver, membership.PortTchannel)
				res, err := r.GetAllPeers()
				if tt.wantError {
					assert.True(t, common.IsServiceTransientError(err))
				} else {
					assert.ElementsMatch(t, tt.want, res)
					assert.NoError(t, err)
				}
			})
		}
	})
	t.Run("GlobalRatelimitPeers", func(t *testing.T) {
		peers := []membership.HostInfo{
			membership.NewDetailedHostInfo(
				"abc:123",
				"abc-id",
				membership.PortMap{membership.PortTchannel: 1235}),
			membership.NewDetailedHostInfo(
				"def:456",
				"def-id",
				membership.PortMap{membership.PortTchannel: 1235}),
			membership.NewDetailedHostInfo(
				"xyz:789",
				"xyz-id",
				membership.PortMap{membership.PortTchannel: 1235}),
		}
		limitKeys := []string{
			"domain-w-async",
			"domain-x-user",
			"domain-x-worker",
			"domain-y-worker",
			"domain-z-visibility",
		}

		t.Run("errs", func(t *testing.T) {
			t.Run("no members", func(t *testing.T) {
				resolver := membership.NewMockResolver(gomock.NewController(t))
				resolver.EXPECT().Members(service.History).Return(nil, errors.New("no members"))

				r := NewPeerResolver(numShards, resolver, membership.PortTchannel)
				_, err := r.GlobalRatelimitPeers([]string{"test"})
				assert.ErrorContains(t, err, "no members")
			})
			t.Run("no host for key", func(t *testing.T) {
				resolver := membership.NewMockResolver(gomock.NewController(t))
				resolver.EXPECT().Members(service.History).Return(peers, nil)
				// seems likely impossible, unless the service is unknown
				resolver.EXPECT().Lookup(service.History, gomock.Any()).Return(membership.HostInfo{}, errors.New("no host"))

				r := NewPeerResolver(numShards, resolver, membership.PortTchannel)
				_, err := r.GlobalRatelimitPeers(limitKeys)
				assert.ErrorContains(t, err, "no host")
			})
			t.Run("no protocol", func(t *testing.T) {
				resolver := membership.NewMockResolver(gomock.NewController(t))
				resolver.EXPECT().Members(service.History).Return(peers, nil)
				resolver.EXPECT().Lookup(service.History, gomock.Any()).Return(peers[0], nil)

				// request GRPC, but only configured for TChannel
				r := NewPeerResolver(numShards, resolver, membership.PortGRPC)
				_, err := r.GlobalRatelimitPeers(limitKeys)
				assert.ErrorContains(t, err, "unable to get address")
			})
		})
		t.Run("success", func(t *testing.T) {
			resolver := membership.NewMockResolver(gomock.NewController(t))
			resolver.EXPECT().Members(service.History).Return(peers, nil).Times(2)

			// use a real hashring to resolve the keys.
			// partly because it's easy and removes the need to mock,
			// and partly because this must remain stable across hashring upgrades.
			ring := hashring.New(farm.Fingerprint32, 100 /* arbitrary, but affects sharding */)
			for _, p := range peers {
				ring.AddMembers(p) // casts type
			}
			resolver.EXPECT().Lookup(service.History, gomock.Any()).DoAndReturn(func(service, key string) (membership.HostInfo, error) {
				peer, ok := ring.Lookup(key)
				require.True(t, ok, "keys should always be findable")
				return find(t, peers, func(info membership.HostInfo) bool {
					return info.GetAddress() == peer
				}), nil
			}).AnyTimes()

			// small sanity check: sharded response should return all inputs
			assertAllKeysPresent := func(t *testing.T, sharded map[Peer][]string, limits []string) {
				responded := maps.Values(sharded)
				assert.ElementsMatchf(t,
					limits,
					slices.Concat(responded...), // flatten the [][]string
					"sharded response does not contain exactly the limits requested:\n\trequested: %v\n\tresponse: %v",
					limits, sharded)
			}

			// shard the keys to hosts
			r := NewPeerResolver(numShards, resolver, membership.PortTchannel)
			res, err := r.GlobalRatelimitPeers(limitKeys)
			require.NoError(t, err)
			assertAllKeysPresent(t, res, limitKeys)
			assert.Equal(t, map[Peer][]string{ // determined experimentally, but needs to be stable
				"abc:1235": {"domain-w-async", "domain-x-worker", "domain-y-worker"},
				"def:1235": {"domain-x-user"},
				"xyz:1235": {"domain-z-visibility"},
			}, res, "")

			// request with another key, response should be the same but with a new key somewhere
			oneMoreKey := append(limitKeys, "domain-other-user")
			res, err = r.GlobalRatelimitPeers(oneMoreKey)
			require.NoError(t, err)
			assertAllKeysPresent(t, res, oneMoreKey)
			assert.Equal(t, map[Peer][]string{
				"abc:1235": {"domain-w-async", "domain-x-worker", "domain-y-worker"},
				"def:1235": {"domain-x-user"},
				"xyz:1235": {"domain-z-visibility", "domain-other-user"},
			}, res, "")
		})
	})
}

// finds a single element that matches, or fails.
// surprisingly "slices" doesn't have any filter or find equivalent, only Index,
// and either way I want to ensure no multi-matches.
func find[T any](t *testing.T, elems []T, equals func(T) bool) T {
	var zero T
	filtered := filter(elems, equals)
	if len(filtered) == 0 {
		t.Fatalf("no matching %T elements found", zero)
		return zero // unreachable
	}
	if len(filtered) > 1 {
		t.Fatalf("found multiple matching %T elements: %v", zero, filtered)
		return zero // unreachable
	}
	return filtered[0]
}

func filter[T any](elems []T, match func(T) bool) (filtered []T) {
	for _, e := range elems {
		if match(e) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
