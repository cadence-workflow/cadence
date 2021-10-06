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
	"io/ioutil"
	"testing"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/yarpc/transport/grpc"
	"go.uber.org/yarpc/transport/tchannel"
)

func TestPublicClientOutbound(t *testing.T) {
	makeConfig := func(hostPort string, enableAuth bool, keyPath string) *config.Config {
		return &config.Config{
			PublicClient:  config.PublicClient{HostPort: hostPort},
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

	builder, err := newPublicClientOutbound(makeConfig("localhost:1234", false, ""))
	require.NoError(t, err)
	require.NotNil(t, builder)
	require.Equal(t, "localhost:1234", builder.address)
	require.Equal(t, nil, builder.authMiddleware)

	builder, err = newPublicClientOutbound(makeConfig("localhost:1234", true, "invalid"))
	require.EqualError(t, err, "create AuthProvider: invalid private key path invalid")

	builder, err = newPublicClientOutbound(makeConfig("localhost:1234", true, tempFile(t, "private-key")))
	require.NoError(t, err)
	require.NotNil(t, builder)
	require.Equal(t, "localhost:1234", builder.address)
	require.NotNil(t, builder.authMiddleware)

	grpc := &grpc.Transport{}
	tchannel := &tchannel.Transport{}
	outbounds, err := builder.Build(grpc, tchannel)
	require.NoError(t, err)
	assert.Equal(t, outbounds[OutboundPublicClient].ServiceName, service.Frontend)
	assert.NotNil(t, outbounds[OutboundPublicClient].Unary)
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
