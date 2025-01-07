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

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
)

var testServices = []string{"test-worker", "test-services"}

func TestSubscribeIsCalledOnPeerProvider(t *testing.T) {
	r, pp := newTestResolver(t)
	_, err := r.getRing("test-worker")
	assert.NoError(t, err)

	// After membership is started, we expect start, subscribe and GetMembers on PeerProvider
	pp.EXPECT().Start().Times(1)
	pp.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Times(len(testServices))
	pp.EXPECT().GetMembers(gomock.Any()).Times(len(testServices))

	r.Start()
}

func TestNewCreatesAllRings(t *testing.T) {
	a, _ := newTestResolver(t)
	assert.Equal(t, len(testServices), len(a.rings))

}

func TestMethodsAreRoutedToARing(t *testing.T) {
	var changeCh = make(chan *ChangedEvent)
	a, pp := newTestResolver(t)

	// add members to this ring
	hosts := []HostInfo{}
	for _, addr := range []string{"127", "128"} {
		hosts = append(hosts, NewHostInfo(addr))
	}

	pp.EXPECT().GetMembers("test-worker").Return(hosts, nil).Times(1)
	pp.EXPECT().WhoAmI().AnyTimes()

	r, err := a.getRing("test-worker")
	r.refresh()

	assert.NoError(t, err)

	hi, err := r.Lookup("key")
	assert.NoError(t, err)
	assert.Equal(t, "127", hi.GetAddress())

	// the same ring will be picked here
	_, err = a.Lookup("WRONG-RING-NAME", "key")
	assert.Error(t, err)

	members, err := a.Members("test-worker")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(members))

	nomembers, err := a.Members("WRONG-RING-NAME")
	assert.Error(t, err)
	assert.Equal(t, 0, len(nomembers))

	memcount, err := a.MemberCount("test-worker")
	assert.NoError(t, err)
	assert.Equal(t, 2, memcount)

	nomemcount, err := a.MemberCount("WRONG-RING-NAME")
	assert.Error(t, err)
	assert.Equal(t, 0, nomemcount)

	serr := a.Subscribe("test-worker", "sub1", changeCh)
	assert.NoError(t, serr)

	serr = a.Subscribe("WRONG-RING-NAME", "sub1", changeCh)
	assert.Error(t, serr)

	serr = a.Unsubscribe("test-worker", "sub1")
	assert.NoError(t, serr)

	serr = a.Unsubscribe("WRONG-RING-NAME", "sub1")
	assert.Error(t, serr)

}

func TestNonExistingRingReturnsError(t *testing.T) {
	a, _ := newTestResolver(t)
	_, err := a.getRing("non-existing")
	assert.Error(t, err)
}

func TestCallsAreForwardedToProvider(t *testing.T) {
	a, mockedPeer := newTestResolver(t)

	mockedPeer.EXPECT().WhoAmI().Times(1)
	mockedPeer.EXPECT().SelfEvict().Times(1)
	mockedPeer.EXPECT().Stop().Times(1)

	a.status = common.DaemonStatusStarted
	a.WhoAmI()
	a.EvictSelf()
	a.Stop()

}

func newTestResolver(t *testing.T) (*MultiringResolver, *MockPeerProvider) {

	ctrl := gomock.NewController(t)
	pp := NewMockPeerProvider(ctrl)
	return NewMultiringResolver(
		testServices,
		pp,
		log.NewNoop(),
		metrics.NewNoopMetricsClient(),
	), pp
}
