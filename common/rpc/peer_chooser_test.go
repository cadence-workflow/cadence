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

package rpc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/yarpc/api/peer"
	"go.uber.org/yarpc/api/transport"
	"go.uber.org/yarpc/transport/grpc"

	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
)

type (
	fakePeerTransport struct{}
	fakePeer          struct{}
)

func (t *fakePeerTransport) RetainPeer(peer.Identifier, peer.Subscriber) (peer.Peer, error) {
	return &fakePeer{}, nil
}
func (t *fakePeerTransport) ReleasePeer(peer.Identifier, peer.Subscriber) error {
	return nil
}

func (p *fakePeer) Identifier() string  { return "fakePeer" }
func (p *fakePeer) Status() peer.Status { return peer.Status{ConnectionStatus: peer.Available} }
func (p *fakePeer) StartRequest()       {}
func (p *fakePeer) EndRequest()         {}

func TestDNSPeerChooserFactory(t *testing.T) {
	defer goleak.VerifyNone(t)

	logger := log.NewNoop()
	ctx := context.Background()
	interval := 10 * time.Millisecond

	factory := NewDNSPeerChooserFactory(interval, logger)
	peerTransport := &fakePeerTransport{}

	// Ensure invalid address returns error
	_, err := factory.CreatePeerChooser(peerTransport, PeerChooserOptions{Address: "invalid address"})
	assert.EqualError(t, err, "incorrect DNS:Port format")

	chooser, err := factory.CreatePeerChooser(peerTransport, PeerChooserOptions{Address: "localhost:1234"})
	require.NoError(t, err)

	require.NoError(t, chooser.Start())
	defer chooser.Stop()

	require.True(t, chooser.IsRunning())

	// Wait for refresh
	time.Sleep(interval + 50*time.Millisecond)

	peer, _, err := chooser.Choose(ctx, &transport.Request{})
	require.NoError(t, err)
	require.NotNil(t, peer)
	assert.Equal(t, "fakePeer", peer.Identifier())
}

func TestDirectPeerChooserFactory(t *testing.T) {
	logger := testlogger.New(t)
	metricCl := metrics.NewNoopMetricsClient()
	serviceName := "service"
	pcf := NewDirectPeerChooserFactory(serviceName, logger, metricCl)
	directConnRetainFn := func(opts ...dynamicconfig.FilterOption) bool { return false }
	grpcTransport := grpc.NewTransport()
	chooser, err := pcf.CreatePeerChooser(grpcTransport, PeerChooserOptions{
		ServiceName:                            serviceName,
		EnableConnectionRetainingDirectChooser: directConnRetainFn,
	})
	if err != nil {
		t.Fatalf("Failed to create direct peer chooser: %v", err)
	}
	if chooser == nil {
		t.Fatal("Failed to create direct peer chooser: nil")
	}

	if _, dc := chooser.(*directPeerChooser); !dc {
		t.Fatalf("Want chooser be of type (*directPeerChooser), got %d", chooser)
	}
}
