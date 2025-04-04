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
	"encoding/json"
	"io"

	"go.uber.org/cadence/worker"
	"go.uber.org/yarpc"
	"go.uber.org/yarpc/api/transport"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/isolationgroup"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
)

type authOutboundMiddleware struct {
	authProvider worker.AuthorizationProvider
}

func (m *authOutboundMiddleware) Call(ctx context.Context, request *transport.Request, out transport.UnaryOutbound) (*transport.Response, error) {
	if m.authProvider == nil {
		return out.Call(ctx, request)
	}

	token, err := m.authProvider.GetAuthToken()
	if err != nil {
		return nil, err
	}
	request.Headers = request.Headers.
		With(common.AuthorizationTokenHeaderName, string(token))

	return out.Call(ctx, request)
}

type contextKey string

const _responseInfoContextKey = contextKey("response-info")

// ContextWithResponseInfo will create a child context that has ResponseInfo set as value.
// This value will get filled after the call is made and can be used later to retrieve some info of interest.
func ContextWithResponseInfo(parent context.Context) (context.Context, *ResponseInfo) {
	responseInfo := &ResponseInfo{}
	return context.WithValue(parent, _responseInfoContextKey, responseInfo), responseInfo
}

// ResponseInfo structure is filled with data after the RPC call.
// It can be obtained with rpc.ContextWithResponseInfo function.
type ResponseInfo struct {
	Size int
}

type countingReadCloser struct {
	reader    io.ReadCloser
	bytesRead *int
}

func (r *countingReadCloser) Read(p []byte) (n int, err error) {
	n, err = r.reader.Read(p)
	*r.bytesRead += n
	return n, err
}

func (r *countingReadCloser) Close() (err error) {
	return r.reader.Close()
}

// ResponseInfoMiddleware populates context with ResponseInfo structure which contains info about response that was received.
// In particular, it counts the size of the response in bytes. Such information can be useful down the line, where payload are deserialized and no longer have their size.
type ResponseInfoMiddleware struct{}

func (m *ResponseInfoMiddleware) Call(ctx context.Context, request *transport.Request, out transport.UnaryOutbound) (*transport.Response, error) {
	response, err := out.Call(ctx, request)

	if value := ctx.Value(_responseInfoContextKey); value != nil {
		if responseInfo, ok := value.(*ResponseInfo); ok && response != nil {
			// We can not use response.BodySize here, because it is not set on all transports.
			// Instead wrap body reader with counter, that increments responseInfo.Size as it is read.
			response.Body = &countingReadCloser{reader: response.Body, bytesRead: &responseInfo.Size}
		}
	}

	return response, err
}

// InboundMetricsMiddleware tags context with additional metric tags from incoming request.
type InboundMetricsMiddleware struct{}

func (m *InboundMetricsMiddleware) Handle(ctx context.Context, req *transport.Request, resw transport.ResponseWriter, h transport.UnaryHandler) error {
	ctx = metrics.TagContext(ctx,
		metrics.CallerTag(req.Caller),
		metrics.TransportTag(req.Transport),
	)
	return h.Handle(ctx, req, resw)
}

// ComparatorYarpcKey is the const for yarpc key
const ComparatorYarpcKey = "cadence-visibility-override"

// PinotComparatorMiddleware checks the header of a grpc request, and then override the context accordingly
// note: for Pinot Migration only (Jan. 2024)
type PinotComparatorMiddleware struct{}

func (m *PinotComparatorMiddleware) Handle(ctx context.Context, req *transport.Request, resw transport.ResponseWriter, h transport.UnaryHandler) error {
	yarpcKey, ok := req.Headers.Get(ComparatorYarpcKey)
	if ok {
		contextWithVisibilityOverride(ctx, yarpcKey) //original key can not be passed, need get new context with key
	}

	return h.Handle(ctx, req, resw)
}

// ContextWithOverride adds a value in ctx
func contextWithVisibilityOverride(ctx context.Context, value string) context.Context {
	return context.WithValue(ctx, persistence.ContextKey, value)
}

type overrideCallerMiddleware struct {
	caller string
}

func (m *overrideCallerMiddleware) Call(ctx context.Context, request *transport.Request, out transport.UnaryOutbound) (*transport.Response, error) {
	request.Caller = m.caller
	return out.Call(ctx, request)
}

// HeaderForwardingMiddleware forwards headers from current inbound RPC call that is being handled to new outbound calls being made.
// As this does NOT differentiate between transports or purposes, it generally assumes we are not acting as a true proxy,
// so things like content lengths and encodings should not be forwarded - they will be provided by the outbound RPC library as needed.
//
// Duplicated headers retain the first value only, matching how browsers and Go (afaict) generally behave.
//
// This uses overly-simplified rules for choosing which headers are forwarded and which are not, intended to be lightly configurable.
// For a more in-depth logic review if it becomes needed, check:
//   - How Go's ReverseProxy deals with headers, e.g. per-protocol and a list of exclusions: https://cs.opensource.google/go/go/+/refs/tags/go1.20.1:src/net/http/httputil/reverseproxy.go;l=332
//   - HTTP's spec for headers, namely how duplicates and Connection work: https://www.rfc-editor.org/rfc/rfc9110.html#name-header-fields
//   - Many browsers prefer first-value-wins for unexpected duplicates: https://bugzilla.mozilla.org/show_bug.cgi?id=376756
//   - But there are MANY map-like implementations that choose last-value wins, and this mismatch is a source of frequent security problems.
//   - YARPC's `With` only retains the last call's value: https://github.com/yarpc/yarpc-go/blob/8ccd79a2ca696150213faac1d35011c5be52e5fb/api/transport/header.go#L69-L77
//   - Go's MIMEHeader's Get (used by YARPC) only returns the first value, and does not join duplicates: https://pkg.go.dev/net/textproto#MIMEHeader.Get
//
// There is likely no correct choice, as it depends on the recipients' behavior.
// If we need to support more complex logic, it's likely worth jumping to a fully-controllable thing.
// Middle-grounds will probably just need to be changed again later.
type HeaderForwardingMiddleware struct {
	// Rules are applied in order to add or remove headers by regex.
	//
	// There are no default rules, so by default no headers are copied.
	// To include headers by default, Add with a permissive regex and then remove specific ones.
	Rules []config.HeaderRule
}

func (m *HeaderForwardingMiddleware) Call(ctx context.Context, request *transport.Request, out transport.UnaryOutbound) (*transport.Response, error) {
	if inboundCall := yarpc.CallFromContext(ctx); inboundCall != nil {
		outboundHeaders := request.Headers
		pending := make(map[string][]string, len(inboundCall.HeaderNames()))
		names := inboundCall.HeaderNames()
		for _, rule := range m.Rules {
			for _, key := range names {
				if !rule.Match.MatchString(key) {
					continue
				}
				if _, ok := outboundHeaders.Get(key); ok {
					continue // do not overwrite existing headers
				}
				if rule.Add {
					pending[key] = append(pending[key], inboundCall.Header(key))
				} else {
					delete(pending, key)
				}
			}
		}
		for k, vs := range pending {
			// yarpc's Headers.With keeps the LAST value, but we (and browsers) prefer the FIRST,
			// and we do not canonicalize duplicates.
			request.Headers = request.Headers.With(k, vs[0])
		}
	}

	return out.Call(ctx, request)
}

// ForwardPartitionConfigMiddleware forwards the partition config to remote cluster
// The middleware should always be applied after any other middleware that inject partition config into the context
// so that it can overwrites the partition config into the context
// The purpose of this middleware is to make sure the partition config doesn't change when a request is forwarded from
// passive cluster to the active cluster
type ForwardPartitionConfigMiddleware struct{}

func (m *ForwardPartitionConfigMiddleware) Handle(ctx context.Context, req *transport.Request, resw transport.ResponseWriter, h transport.UnaryHandler) error {
	if _, ok := req.Headers.Get(common.AutoforwardingClusterHeaderName); ok {
		var partitionConfig map[string]string
		if blob, ok := req.Headers.Get(common.PartitionConfigHeaderName); ok && len(blob) > 0 {
			if err := json.Unmarshal([]byte(blob), &partitionConfig); err != nil {
				return err
			}
		}
		ctx = isolationgroup.ContextWithConfig(ctx, partitionConfig)
		isolationGroup, _ := req.Headers.Get(common.IsolationGroupHeaderName)
		ctx = isolationgroup.ContextWithIsolationGroup(ctx, isolationGroup)
	}
	return h.Handle(ctx, req, resw)
}

func (m *ForwardPartitionConfigMiddleware) Call(ctx context.Context, request *transport.Request, out transport.UnaryOutbound) (*transport.Response, error) {
	if _, ok := request.Headers.Get(common.AutoforwardingClusterHeaderName); ok {
		partitionConfig := isolationgroup.ConfigFromContext(ctx)
		if len(partitionConfig) > 0 {
			blob, err := json.Marshal(partitionConfig)
			if err != nil {
				return nil, err
			}
			request.Headers = request.Headers.With(common.PartitionConfigHeaderName, string(blob))
		} else {
			request.Headers.Del(common.PartitionConfigHeaderName)
		}
		isolationGroup := isolationgroup.IsolationGroupFromContext(ctx)
		if isolationGroup != "" {
			request.Headers = request.Headers.With(common.IsolationGroupHeaderName, isolationGroup)
		} else {
			request.Headers.Del(common.IsolationGroupHeaderName)
		}
	}
	return out.Call(ctx, request)
}

// ClientPartitionConfigMiddleware stores the partition config and isolation group of the request into the context
// It reads a header from client request and uses it as the isolation group
type ClientPartitionConfigMiddleware struct{}

func (m *ClientPartitionConfigMiddleware) Handle(ctx context.Context, req *transport.Request, resw transport.ResponseWriter, h transport.UnaryHandler) error {
	zone, _ := req.Headers.Get(common.ClientIsolationGroupHeaderName)
	if zone != "" {
		ctx = isolationgroup.ContextWithConfig(ctx, map[string]string{
			isolationgroup.GroupKey: zone,
		})
		ctx = isolationgroup.ContextWithIsolationGroup(ctx, zone)
	}
	return h.Handle(ctx, req, resw)
}
