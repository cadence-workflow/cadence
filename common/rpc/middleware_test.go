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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/yarpc/api/transport"
	"go.uber.org/yarpc/yarpctest"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/isolationgroup"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

func TestAuthOubboundMiddleware(t *testing.T) {
	m := authOutboundMiddleware{}
	_, err := m.Call(context.Background(), &transport.Request{}, &fakeOutbound{verify: func(request *transport.Request) {
		assert.Empty(t, request.Headers)
	}})
	assert.NoError(t, err)

	m = authOutboundMiddleware{fakeAuthProvider{err: assert.AnError}}
	_, err = m.Call(context.Background(), &transport.Request{}, &fakeOutbound{})
	assert.Error(t, err)

	m = authOutboundMiddleware{fakeAuthProvider{token: []byte("token")}}
	_, err = m.Call(context.Background(), &transport.Request{}, &fakeOutbound{verify: func(request *transport.Request) {
		assert.Equal(t, "token", request.Headers.Items()[common.AuthorizationTokenHeaderName])
	}})
	assert.NoError(t, err)
}

func TestResponseInfoMiddleware(t *testing.T) {
	m := ResponseInfoMiddleware{}
	ctx, responseInfo := ContextWithResponseInfo(context.Background())
	body := ioutil.NopCloser(bytes.NewReader([]byte{1, 2, 3, 4, 5}))
	response, err := m.Call(ctx, &transport.Request{}, &fakeOutbound{response: &transport.Response{Body: body}})
	assert.NoError(t, err)
	ioutil.ReadAll(response.Body)
	assert.Equal(t, 5, responseInfo.Size)
}

func TestResponseInfoMiddleware_Error(t *testing.T) {
	m := ResponseInfoMiddleware{}
	ctx, responseInfo := ContextWithResponseInfo(context.Background())
	_, err := m.Call(ctx, &transport.Request{}, &fakeOutbound{err: fmt.Errorf("test")})
	assert.Error(t, err)
	assert.Equal(t, 0, responseInfo.Size)
}

func TestInboundMetricsMiddleware(t *testing.T) {
	m := InboundMetricsMiddleware{}
	h := &fakeHandler{}
	err := m.Handle(context.Background(), &transport.Request{Transport: "grpc", Caller: "x-caller"}, nil, h)
	assert.NoError(t, err)
	assert.ElementsMatch(t, metrics.GetContextTags(h.ctx), []metrics.Tag{
		metrics.TransportTag("grpc"),
		metrics.CallerTag("x-caller"),
	})
}

func TestOverrideCallerMiddleware(t *testing.T) {
	m := overrideCallerMiddleware{"x-caller"}
	_, err := m.Call(context.Background(), &transport.Request{Caller: "service"}, &fakeOutbound{verify: func(r *transport.Request) {
		assert.Equal(t, "x-caller", r.Caller)
	}})
	assert.NoError(t, err)
}

func TestHeaderForwardingMiddleware(t *testing.T) {
	inboundHeaders := map[string]string{
		"key-a": "inbound-value-a",
		"key-b": "inbound-value-b",
		"key-x": "inbound-value-x",
	}
	outboundHeaders := map[string]string{
		"key-b": "outbound-value-b",
		"key-c": "outbound-value-c",
	}
	combinedHeaders := map[string]string{
		"key-a": "inbound-value-a",
		"key-b": "outbound-value-b",
		"key-c": "outbound-value-c",
		"key-x": "inbound-value-x",
	}
	combinedHeadersWithoutX := map[string]string{
		"key-a": "inbound-value-a",
		"key-b": "outbound-value-b",
		"key-c": "outbound-value-c",
	}

	ctxWithInbound := yarpctest.ContextWithCall(context.Background(), &yarpctest.Call{Headers: inboundHeaders})
	makeRequest := func() *transport.Request {
		// request and headers are mutated by Call, so we must not share data
		return &transport.Request{Headers: transport.HeadersFromMap(outboundHeaders)}
	}

	t.Run("default rules", func(t *testing.T) {
		t.Parallel()
		m := HeaderForwardingMiddleware{
			Rules: []config.HeaderRule{
				// default
				{
					Add:   true,
					Match: regexp.MustCompile(""),
				},
			},
		}
		t.Run("no inbound makes no changes", func(t *testing.T) {
			t.Parallel()
			// No ongoing inbound call -> keep existing outbound headers
			_, err := m.Call(context.Background(), makeRequest(), &fakeOutbound{verify: func(r *transport.Request) {
				assert.Equal(t, outboundHeaders, r.Headers.Items())
			}})
			assert.NoError(t, err)
		})

		t.Run("inbound headers merge missing", func(t *testing.T) {
			t.Parallel()
			// With ongoing inbound call -> forward inbound headers not present in the request
			_, err := m.Call(ctxWithInbound, makeRequest(), &fakeOutbound{verify: func(r *transport.Request) {
				assert.Equal(t, combinedHeaders, r.Headers.Items())
			}})
			assert.NoError(t, err)
		})
	})
	t.Run("can exclude inbound headers", func(t *testing.T) {
		t.Parallel()
		m := HeaderForwardingMiddleware{
			Rules: []config.HeaderRule{
				// add by default, earlier tests ensure this works
				{
					Add:   true,
					Match: regexp.MustCompile(""),
				},
				// remove X
				{
					Add:   false,
					Match: regexp.MustCompile("(?i)Key-x"),
				},
			},
		}
		_, err := m.Call(ctxWithInbound, makeRequest(), &fakeOutbound{verify: func(r *transport.Request) {
			assert.Equal(t, combinedHeadersWithoutX, r.Headers.Items())
		}})
		assert.NoError(t, err)
	})
}

func TestForwardPartitionConfigMiddleware(t *testing.T) {
	t.Run("inbound middleware", func(t *testing.T) {
		partitionConfig := map[string]string{"isolation-group": "xyz"}
		blob, err := json.Marshal(partitionConfig)
		require.NoError(t, err)
		testCases := []struct {
			message                 string
			headers                 transport.Headers
			ctx                     context.Context
			expectedPartitionConfig map[string]string
			expectedIsolationGroup  string
		}{
			{
				message: "it injects partition config into context",
				headers: transport.NewHeaders().
					With(common.PartitionConfigHeaderName, string(blob)).
					With(common.IsolationGroupHeaderName, "abc").
					With(common.AutoforwardingClusterHeaderName, "cluster0"),
				ctx:                     context.Background(),
				expectedPartitionConfig: partitionConfig,
				expectedIsolationGroup:  "abc",
			},
			{
				message: "it overwrites the existing partition config in the context",
				headers: transport.NewHeaders().
					With(common.PartitionConfigHeaderName, string(blob)).
					With(common.IsolationGroupHeaderName, "abc").
					With(common.AutoforwardingClusterHeaderName, "cluster0"),
				ctx:                     isolationgroup.ContextWithIsolationGroup(isolationgroup.ContextWithConfig(context.Background(), map[string]string{"z": "x"}), "fff"),
				expectedPartitionConfig: partitionConfig,
				expectedIsolationGroup:  "abc",
			},
			{
				message: "it overwrites the existing partition config in the context with nil config",
				headers: transport.NewHeaders().
					With(common.AutoforwardingClusterHeaderName, "cluster0"),
				ctx:                     isolationgroup.ContextWithIsolationGroup(isolationgroup.ContextWithConfig(context.Background(), map[string]string{"z": "x"}), "fff"),
				expectedPartitionConfig: nil,
				expectedIsolationGroup:  "",
			},
			{
				message: "it injects partition config into context only if the request is an auto-fowarding request",
				headers: transport.NewHeaders().
					With(common.PartitionConfigHeaderName, string(blob)).
					With(common.IsolationGroupHeaderName, "abc"),
				ctx:                     context.Background(),
				expectedPartitionConfig: nil,
				expectedIsolationGroup:  "",
			},
		}

		for _, tt := range testCases {
			t.Run(tt.message, func(t *testing.T) {
				m := &ForwardPartitionConfigMiddleware{}
				h := &fakeHandler{}
				err := m.Handle(tt.ctx, &transport.Request{Headers: tt.headers}, nil, h)
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedPartitionConfig, isolationgroup.ConfigFromContext(h.ctx))
				assert.Equal(t, tt.expectedIsolationGroup, isolationgroup.IsolationGroupFromContext(h.ctx))
			})
		}
	})

	t.Run("outbound middleware", func(t *testing.T) {
		partitionConfig := map[string]string{"isolation-group": "xyz"}
		blob, err := json.Marshal(partitionConfig)
		require.NoError(t, err)
		testCases := []struct {
			message                           string
			headers                           transport.Headers
			ctx                               context.Context
			expectedSerializedPartitionConfig string
			expectedIsolationGroup            string
		}{
			{
				message: "it retrieves partition config from the context and sets it in the request header",
				headers: transport.NewHeaders().
					With(common.AutoforwardingClusterHeaderName, "cluster0"),
				ctx:                               isolationgroup.ContextWithIsolationGroup(isolationgroup.ContextWithConfig(context.Background(), partitionConfig), "abc"),
				expectedSerializedPartitionConfig: string(blob),
				expectedIsolationGroup:            "abc",
			},
			{
				message: "it retrieves partition config from the context and overwrites the existing request header",
				headers: transport.NewHeaders().
					With(common.IsolationGroupHeaderName, "lll").
					With(common.PartitionConfigHeaderName, "asdfasf").
					With(common.AutoforwardingClusterHeaderName, "cluster0"),
				ctx:                               isolationgroup.ContextWithIsolationGroup(isolationgroup.ContextWithConfig(context.Background(), partitionConfig), "abc"),
				expectedSerializedPartitionConfig: string(blob),
				expectedIsolationGroup:            "abc",
			},
			{
				message: "it deletes the existing request header if we cannot retrieve partition config from the context",
				headers: transport.NewHeaders().
					With(common.IsolationGroupHeaderName, "lll").
					With(common.PartitionConfigHeaderName, "asdfasf").
					With(common.AutoforwardingClusterHeaderName, "cluster0"),
				ctx:                               context.Background(),
				expectedSerializedPartitionConfig: "",
				expectedIsolationGroup:            "",
			},
			{
				message: "it retrieves partition config from the context and sets it in the request header only if the request is an auto-forwarding request",
				headers: transport.NewHeaders().
					With(common.IsolationGroupHeaderName, "lll").
					With(common.PartitionConfigHeaderName, "asdfasf"),
				ctx:                               isolationgroup.ContextWithIsolationGroup(isolationgroup.ContextWithConfig(context.Background(), partitionConfig), "abc"),
				expectedSerializedPartitionConfig: "asdfasf",
				expectedIsolationGroup:            "lll",
			},
		}

		for _, tt := range testCases {
			t.Run(tt.message, func(t *testing.T) {
				m := &ForwardPartitionConfigMiddleware{}
				o := &fakeOutbound{
					verify: func(r *transport.Request) {
						assert.Equal(t, tt.expectedIsolationGroup, r.Headers.Items()[common.IsolationGroupHeaderName])
						assert.Equal(t, tt.expectedSerializedPartitionConfig, r.Headers.Items()[common.PartitionConfigHeaderName])
					},
				}
				_, err := m.Call(tt.ctx, &transport.Request{Headers: tt.headers}, o)
				assert.NoError(t, err)
			})
		}
	})
}

func TestClientPartitionConfigMiddleware(t *testing.T) {
	t.Run("it sets the partition config", func(t *testing.T) {
		m := &ClientPartitionConfigMiddleware{}
		h := &fakeHandler{}
		headers := transport.NewHeaders().
			With(common.ClientIsolationGroupHeaderName, "dca1")
		err := m.Handle(context.Background(), &transport.Request{Headers: headers}, nil, h)
		assert.NoError(t, err)
		assert.Equal(t, map[string]string{isolationgroup.GroupKey: "dca1"}, isolationgroup.ConfigFromContext(h.ctx))
		assert.Equal(t, "dca1", isolationgroup.IsolationGroupFromContext(h.ctx))
	})

	t.Run("noop when header is empty", func(t *testing.T) {
		m := &ClientPartitionConfigMiddleware{}
		h := &fakeHandler{}
		headers := transport.NewHeaders()
		ctx := context.Background()
		err := m.Handle(ctx, &transport.Request{Headers: headers}, nil, h)
		assert.NoError(t, err)
		assert.Nil(t, isolationgroup.ConfigFromContext(h.ctx))
		assert.Equal(t, "", isolationgroup.IsolationGroupFromContext(h.ctx))
		assert.Equal(t, ctx, h.ctx)
	})
}

func TestCallerInfoMiddleware(t *testing.T) {
	t.Run("extracts caller type from header", func(t *testing.T) {
		m := &CallerInfoMiddleware{}
		h := &fakeHandler{}
		headers := transport.NewHeaders().With(types.CallerTypeHeaderName, "cli")
		err := m.Handle(context.Background(), &transport.Request{Headers: headers}, nil, h)
		assert.NoError(t, err)

		callerInfo := types.GetCallerInfoFromContext(h.ctx)
		assert.Equal(t, types.CallerTypeCLI, callerInfo.GetCallerType())
	})

	t.Run("sets unknown caller type when header is missing", func(t *testing.T) {
		m := &CallerInfoMiddleware{}
		h := &fakeHandler{}
		headers := transport.NewHeaders()
		err := m.Handle(context.Background(), &transport.Request{Headers: headers}, nil, h)
		assert.NoError(t, err)

		callerInfo := types.GetCallerInfoFromContext(h.ctx)
		assert.Equal(t, types.CallerTypeUnknown, callerInfo.GetCallerType())
	})

	t.Run("extracts different caller types", func(t *testing.T) {
		tests := []struct {
			name           string
			headerValue    string
			expectedCaller types.CallerType
		}{
			{"CLI", "cli", types.CallerTypeCLI},
			{"UI", "ui", types.CallerTypeUI},
			{"SDK", "sdk", types.CallerTypeSDK},
			{"Internal", "internal", types.CallerTypeInternal},
			{"Empty", "", types.CallerTypeUnknown},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				m := &CallerInfoMiddleware{}
				h := &fakeHandler{}
				headers := transport.NewHeaders().With(types.CallerTypeHeaderName, tt.headerValue)
				err := m.Handle(context.Background(), &transport.Request{Headers: headers}, nil, h)
				assert.NoError(t, err)

				callerInfo := types.GetCallerInfoFromContext(h.ctx)
				assert.Equal(t, tt.expectedCaller, callerInfo.GetCallerType())
			})
		}
	})
}

func TestQueryConsistencyLevelInboundMiddleware(t *testing.T) {
	tests := []struct {
		name          string
		headerValue   string
		hasHeader     bool
		expectedLevel types.QueryConsistencyLevel
		expectError   bool
	}{
		{
			name:          "parses EVENTUAL consistency level",
			headerValue:   "EVENTUAL",
			hasHeader:     true,
			expectedLevel: types.QueryConsistencyLevelEventual,
			expectError:   false,
		},
		{
			name:          "parses STRONG consistency level",
			headerValue:   "STRONG",
			hasHeader:     true,
			expectedLevel: types.QueryConsistencyLevelStrong,
			expectError:   false,
		},
		{
			name:          "parses lowercase eventual",
			headerValue:   "eventual",
			hasHeader:     true,
			expectedLevel: types.QueryConsistencyLevelEventual,
			expectError:   false,
		},
		{
			name:          "parses lowercase strong",
			headerValue:   "strong",
			hasHeader:     true,
			expectedLevel: types.QueryConsistencyLevelStrong,
			expectError:   false,
		},
		{
			name:          "parses numeric value 0",
			headerValue:   "0",
			hasHeader:     true,
			expectedLevel: types.QueryConsistencyLevelEventual,
			expectError:   false,
		},
		{
			name:          "parses numeric value 1",
			headerValue:   "1",
			hasHeader:     true,
			expectedLevel: types.QueryConsistencyLevelStrong,
			expectError:   false,
		},
		{
			name:        "returns error for invalid value",
			headerValue: "INVALID",
			hasHeader:   true,
			expectError: true,
		},
		{
			name:        "no header - noop",
			hasHeader:   false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &QueryConsistencyLevelInboundMiddleware{}
			h := &fakeHandler{}
			headers := transport.NewHeaders()
			if tt.hasHeader {
				headers = headers.With(common.QueryConsistencyLevelHeaderName, tt.headerValue)
			}

			err := m.Handle(context.Background(), &transport.Request{Headers: headers}, nil, h)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "failed to parse query consistency level")
				return
			}

			assert.NoError(t, err)
			if tt.hasHeader {
				level, ok := h.ctx.Value(common.QueryConsistencyLevelHeaderName).(types.QueryConsistencyLevel)
				assert.True(t, ok, "expected query consistency level to be set in context")
				assert.Equal(t, tt.expectedLevel, level)
			} else {
				// When no header is present, context value should not be set
				assert.Nil(t, h.ctx.Value(common.QueryConsistencyLevelHeaderName))
			}
		})
	}
}

type fakeHandler struct {
	ctx context.Context
}

func (h *fakeHandler) Handle(ctx context.Context, req *transport.Request, resw transport.ResponseWriter) error {
	h.ctx = ctx
	return nil
}

type fakeOutbound struct {
	verify   func(*transport.Request)
	response *transport.Response
	err      error
}

func (o fakeOutbound) Call(ctx context.Context, request *transport.Request) (*transport.Response, error) {
	if o.verify != nil {
		o.verify(request)
	}
	return o.response, o.err
}
func (o fakeOutbound) Start() error                      { return nil }
func (o fakeOutbound) Stop() error                       { return nil }
func (o fakeOutbound) IsRunning() bool                   { return true }
func (o fakeOutbound) Transports() []transport.Transport { return nil }

type fakeAuthProvider struct {
	token []byte
	err   error
}

func (p fakeAuthProvider) GetAuthToken() ([]byte, error) {
	return p.token, p.err
}
