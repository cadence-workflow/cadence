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
	"errors"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/yarpc"
	"go.uber.org/yarpc/api/peer"
	"go.uber.org/yarpc/api/transport"
	"go.uber.org/yarpc/peer/direct"
	"go.uber.org/yarpc/transport/grpc"
	"go.uber.org/yarpc/transport/tchannel"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/service"
)

func TestCombineOutbounds(t *testing.T) {
	grpc := &grpc.Transport{}
	tchannel := &tchannel.Transport{}

	combined := CombineOutbounds()
	outbounds, err := combined.Build(grpc, tchannel)
	assert.NoError(t, err)
	assert.Empty(t, outbounds.Outbounds)

	combined = CombineOutbounds(fakeOutboundBuilder{err: errors.New("err-A")})
	_, err = combined.Build(grpc, tchannel)
	assert.EqualError(t, err, "err-A")

	combined = CombineOutbounds(
		fakeOutboundBuilder{outbounds: yarpc.Outbounds{"A": {}}},
		fakeOutboundBuilder{outbounds: yarpc.Outbounds{"A": {}}},
	)
	_, err = combined.Build(grpc, tchannel)
	assert.EqualError(t, err, "outbound \"A\" already configured")

	combined = CombineOutbounds(
		fakeOutboundBuilder{err: errors.New("err-A")},
		fakeOutboundBuilder{err: errors.New("err-B")},
	)
	_, err = combined.Build(grpc, tchannel)
	assert.EqualError(t, err, "err-A; err-B")

	combined = CombineOutbounds(
		fakeOutboundBuilder{outbounds: yarpc.Outbounds{"A": {}}},
		fakeOutboundBuilder{outbounds: yarpc.Outbounds{"B": {}, "C": {}}},
	)
	outbounds, err = combined.Build(grpc, tchannel)
	assert.NoError(t, err)
	assert.Equal(t, yarpc.Outbounds{
		"A": {},
		"B": {},
		"C": {},
	}, outbounds.Outbounds)
}

func TestPublicClientOutbound(t *testing.T) {
	makeConfig := func(hostPort string, transport string, enableAuth bool, keyPath string) *config.Config {
		return &config.Config{
			PublicClient:  config.PublicClient{HostPort: hostPort, Transport: transport},
			Authorization: config.Authorization{OAuthAuthorizer: config.OAuthAuthorizer{Enable: enableAuth}},
			ClusterGroupMetadata: &config.ClusterGroupMetadata{
				CurrentClusterName: "cluster-A",
				ClusterGroup: map[string]config.ClusterInformation{
					"cluster-A": {
						AuthorizationProvider: config.AuthorizationProvider{
							PrivateKey: keyPath,
						},
					},
				},
			},
		}
	}

	_, err := newPublicClientOutbound(&config.Config{})
	require.EqualError(t, err, "need to provide an endpoint config for PublicClient")

	builder, err := newPublicClientOutbound(makeConfig("localhost:1234", "tchannel", false, ""))
	require.NoError(t, err)
	require.NotNil(t, builder)
	require.Equal(t, "localhost:1234", builder.address)
	require.Equal(t, nil, builder.authMiddleware)
	require.False(t, builder.isGRPC)

	builder, err = newPublicClientOutbound(makeConfig("localhost:1234", "tchannel", true, "invalid"))
	require.EqualError(t, err, "create AuthProvider: invalid private key path invalid")
	require.False(t, builder.isGRPC)

	builder, err = newPublicClientOutbound(makeConfig("localhost:1234", "grpc", true, tempFile(t, "private-key")))
	require.NoError(t, err)
	require.NotNil(t, builder)
	require.Equal(t, "localhost:1234", builder.address)
	require.NotNil(t, builder.authMiddleware)
	require.True(t, builder.isGRPC)

	grpc := &grpc.Transport{}
	tchannel := &tchannel.Transport{}
	o, err := builder.Build(grpc, tchannel)
	require.NoError(t, err)
	outbounds := o.Outbounds
	assert.Equal(t, outbounds[OutboundPublicClient].ServiceName, service.Frontend)
	assert.NotNil(t, outbounds[OutboundPublicClient].Unary)
}

func TestCrossDCOutbounds(t *testing.T) {
	grpc := &grpc.Transport{}
	tchannel := &tchannel.Transport{}

	clusterGroup := map[string]config.ClusterInformation{
		"cluster-A": {Enabled: true, RPCName: "cadence-frontend", RPCTransport: "invalid"},
	}
	_, err := NewCrossDCOutbounds(clusterGroup, &fakePeerChooserFactory{}).Build(grpc, tchannel)
	assert.EqualError(t, err, "unknown cross DC transport type: invalid")

	clusterGroup = map[string]config.ClusterInformation{
		"cluster-A": {Enabled: true, RPCName: "cadence-frontend", RPCTransport: "grpc", AuthorizationProvider: config.AuthorizationProvider{Enable: true, PrivateKey: "invalid path"}},
	}
	_, err = NewCrossDCOutbounds(clusterGroup, &fakePeerChooserFactory{}).Build(grpc, tchannel)
	assert.EqualError(t, err, "create AuthProvider: invalid private key path invalid path")

	clusterGroup = map[string]config.ClusterInformation{
		"cluster-A": {Enabled: true, RPCName: "cadence-frontend", RPCAddress: "address-A", RPCTransport: "grpc", AuthorizationProvider: config.AuthorizationProvider{Enable: true, PrivateKey: tempFile(t, "key")}},
		"cluster-B": {Enabled: true, RPCName: "cadence-frontend", RPCAddress: "address-B", RPCTransport: "tchannel"},
		"cluster-C": {Enabled: false},
	}
	o, err := NewCrossDCOutbounds(clusterGroup, &fakePeerChooserFactory{}).Build(grpc, tchannel)
	assert.NoError(t, err)
	outbounds := o.Outbounds
	assert.Equal(t, 2, len(outbounds))
	assert.Equal(t, "cadence-frontend", outbounds["cluster-A"].ServiceName)
	assert.Equal(t, "cadence-frontend", outbounds["cluster-B"].ServiceName)
	assert.NotNil(t, outbounds["cluster-A"].Unary)
	assert.NotNil(t, outbounds["cluster-B"].Unary)
}

func TestDirectOutbound(t *testing.T) {
	grpc := &grpc.Transport{}
	tchannel := &tchannel.Transport{}
	logger := testlogger.New(t)
	metricCl := metrics.NewNoopMetricsClient()
	falseFn := func(opts ...dynamicconfig.FilterOption) bool { return false }

	o, err := NewDirectOutboundBuilder("cadence-history", false, nil, NewDirectPeerChooserFactory("cadence-history", logger, metricCl), falseFn).Build(grpc, tchannel)
	assert.NoError(t, err)
	outbounds := o.Outbounds
	assert.Equal(t, "cadence-history", outbounds["cadence-history"].ServiceName)
	assert.NotNil(t, outbounds["cadence-history"].Unary)

	o, err = NewDirectOutboundBuilder("cadence-history", true, nil, NewDirectPeerChooserFactory("cadence-history", logger, metricCl), falseFn).Build(grpc, tchannel)
	assert.NoError(t, err)
	outbounds = o.Outbounds
	assert.Equal(t, "cadence-history", outbounds["cadence-history"].ServiceName)
	assert.NotNil(t, outbounds["cadence-history"].Unary)
}

func TestIsGRPCOutboud(t *testing.T) {
	assert.True(t, IsGRPCOutbound(&transport.OutboundConfig{Outbounds: transport.Outbounds{Unary: (&grpc.Transport{}).NewSingleOutbound("localhost:1234")}}))
	assert.False(t, IsGRPCOutbound(&transport.OutboundConfig{Outbounds: transport.Outbounds{Unary: (&tchannel.Transport{}).NewSingleOutbound("localhost:1234")}}))
	assert.Panics(t, func() {
		IsGRPCOutbound(&transport.OutboundConfig{Outbounds: transport.Outbounds{Unary: fakeOutbound{}}})
	})
}

func tempFile(t *testing.T, content string) string {
	f, err := ioutil.TempFile("", "")
	require.NoError(t, err)

	f.Write([]byte(content))
	require.NoError(t, err)

	err = f.Close()
	require.NoError(t, err)

	return f.Name()
}

type fakeOutboundBuilder struct {
	outbounds yarpc.Outbounds
	err       error
}

func (b fakeOutboundBuilder) Build(*grpc.Transport, *tchannel.Transport) (*Outbounds, error) {
	return &Outbounds{Outbounds: b.outbounds}, b.err
}

type fakePeerChooserFactory struct{}

func (f fakePeerChooserFactory) CreatePeerChooser(transport peer.Transport, opts PeerChooserOptions) (PeerChooser, error) {
	chooser, err := direct.New(direct.Configuration{}, transport)
	if err != nil {
		return nil, err
	}
	return &defaultPeerChooser{actual: chooser}, nil
}
