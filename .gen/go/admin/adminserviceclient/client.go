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

// Code generated by thriftrw-plugin-yarpc
// @generated

package adminserviceclient

import (
	"context"
	"github.com/uber/cadence/.gen/go/admin"
	"go.uber.org/thriftrw/wire"
	"go.uber.org/yarpc"
	"go.uber.org/yarpc/api/transport"
	"go.uber.org/yarpc/encoding/thrift"
	"reflect"
)

// Interface is a client for the AdminService service.
type Interface interface {
	DescribeHistoryHost(
		ctx context.Context,
		Request *admin.DescribeHistoryHostRequest,
		opts ...yarpc.CallOption,
	) (*admin.DescribeHistoryHostResponse, error)

	DescribeWorkflowExecution(
		ctx context.Context,
		Request *admin.DescribeWorkflowExecutionRequest,
		opts ...yarpc.CallOption,
	) (*admin.DescribeWorkflowExecutionResponse, error)
}

// New builds a new client for the AdminService service.
//
// 	client := adminserviceclient.New(dispatcher.ClientConfig("adminservice"))
func New(c transport.ClientConfig, opts ...thrift.ClientOption) Interface {
	return client{
		c: thrift.New(thrift.Config{
			Service:      "AdminService",
			ClientConfig: c,
		}, opts...),
	}
}

func init() {
	yarpc.RegisterClientBuilder(
		func(c transport.ClientConfig, f reflect.StructField) Interface {
			return New(c, thrift.ClientBuilderOptions(c, f)...)
		},
	)
}

type client struct {
	c thrift.Client
}

func (c client) DescribeHistoryHost(
	ctx context.Context,
	_Request *admin.DescribeHistoryHostRequest,
	opts ...yarpc.CallOption,
) (success *admin.DescribeHistoryHostResponse, err error) {

	args := admin.AdminService_DescribeHistoryHost_Helper.Args(_Request)

	var body wire.Value
	body, err = c.c.Call(ctx, args, opts...)
	if err != nil {
		return
	}

	var result admin.AdminService_DescribeHistoryHost_Result
	if err = result.FromWire(body); err != nil {
		return
	}

	success, err = admin.AdminService_DescribeHistoryHost_Helper.UnwrapResponse(&result)
	return
}

func (c client) DescribeWorkflowExecution(
	ctx context.Context,
	_Request *admin.DescribeWorkflowExecutionRequest,
	opts ...yarpc.CallOption,
) (success *admin.DescribeWorkflowExecutionResponse, err error) {

	args := admin.AdminService_DescribeWorkflowExecution_Helper.Args(_Request)

	var body wire.Value
	body, err = c.c.Call(ctx, args, opts...)
	if err != nil {
		return
	}

	var result admin.AdminService_DescribeWorkflowExecution_Result
	if err = result.FromWire(body); err != nil {
		return
	}

	success, err = admin.AdminService_DescribeWorkflowExecution_Helper.UnwrapResponse(&result)
	return
}
