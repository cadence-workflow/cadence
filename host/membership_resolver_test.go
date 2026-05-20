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

package host

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/service"
)

func TestSimpleResolver_SubscribeAndNotify(t *testing.T) {
	hosts := map[string][]membership.HostInfo{
		service.History: {newTestHost(7201)},
	}
	resolver := NewSimpleResolver(service.History, hosts, newTestHost(7201))
	sr := resolver.(*simpleResolver)

	ch := make(chan *membership.ChangedEvent, 1)
	err := resolver.Subscribe(service.History, "test-listener", ch)
	require.NoError(t, err)

	event := &membership.ChangedEvent{
		HostsAdded:   []string{"127.0.0.1_7202"},
		HostsRemoved: []string{},
	}
	sr.NotifySubscribers(service.History, event)

	received := <-ch
	require.Equal(t, event, received)
}

func TestSimpleResolver_Unsubscribe(t *testing.T) {
	hosts := map[string][]membership.HostInfo{
		service.History: {newTestHost(7201)},
	}
	resolver := NewSimpleResolver(service.History, hosts, newTestHost(7201))
	sr := resolver.(*simpleResolver)

	ch := make(chan *membership.ChangedEvent, 1)
	require.NoError(t, resolver.Subscribe(service.History, "test-listener", ch))
	require.NoError(t, resolver.Unsubscribe(service.History, "test-listener"))

	sr.NotifySubscribers(service.History, &membership.ChangedEvent{
		HostsAdded: []string{"127.0.0.1_7202"},
	})

	require.Len(t, ch, 0) // nothing received
}

func TestSimpleResolver_NotifySubscribers_NonBlocking(t *testing.T) {
	hosts := map[string][]membership.HostInfo{
		service.History: {newTestHost(7201)},
	}
	resolver := NewSimpleResolver(service.History, hosts, newTestHost(7201))
	sr := resolver.(*simpleResolver)

	// unbuffered channel — should not block
	ch := make(chan *membership.ChangedEvent)
	require.NoError(t, resolver.Subscribe(service.History, "test-listener", ch))

	// should not panic or block
	sr.NotifySubscribers(service.History, &membership.ChangedEvent{
		HostsAdded: []string{"127.0.0.1_7202"},
	})
}

func TestSimpleResolver_SharedHashring(t *testing.T) {
	ring := newSimpleHashring([]membership.HostInfo{newTestHost(7201)})
	rings := map[string]*simpleHashring{
		service.History: ring,
	}

	r1 := NewSimpleResolverWithHashrings(service.History, rings, newTestHost(7201))
	r2 := NewSimpleResolverWithHashrings(service.History, rings, newTestHost(7202))

	// Mutate via the shared ring
	ring.AddHost(newTestHost(7203))

	// Both resolvers see the new host
	members1, err := r1.Members(service.History)
	require.NoError(t, err)
	require.Len(t, members1, 2)

	members2, err := r2.Members(service.History)
	require.NoError(t, err)
	require.Len(t, members2, 2)
}
