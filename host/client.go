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
	adminv1 "github.com/uber/cadence-idl/go/proto/admin/v1"
	apiv1 "github.com/uber/cadence-idl/go/proto/api/v1"
	"go.uber.org/yarpc"

	historyv1 "github.com/uber/cadence/.gen/proto/history/v1"
	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/client/wrappers/grpc"
	"github.com/uber/cadence/common/service"
)

// AdminClient is the interface exposed by admin service client
type AdminClient interface {
	admin.Client
}

// FrontendClient is the interface exposed by frontend service client
type FrontendClient interface {
	frontend.Client
}

// HistoryClient is the interface exposed by history service client
type HistoryClient interface {
	history.Client
}

type MatchingClient interface {
	matching.Client
}

// NewAdminClient creates a client to cadence admin client
func NewAdminClient(d *yarpc.Dispatcher) AdminClient {
	return grpc.NewAdminClient(adminv1.NewAdminAPIYARPCClient(d.ClientConfig(testOutboundName(service.Frontend))))
}

// NewFrontendClient creates a client to cadence frontend client
func NewFrontendClient(d *yarpc.Dispatcher) FrontendClient {
	config := d.ClientConfig(testOutboundName(service.Frontend))
	return grpc.NewFrontendClient(
		apiv1.NewDomainAPIYARPCClient(config),
		apiv1.NewWorkflowAPIYARPCClient(config),
		apiv1.NewWorkerAPIYARPCClient(config),
		apiv1.NewVisibilityAPIYARPCClient(config),
	)
}

// NewHistoryClient creates a client to cadence history service client
func NewHistoryClient(d *yarpc.Dispatcher) HistoryClient {
	return grpc.NewHistoryClient(historyv1.NewHistoryAPIYARPCClient(d.ClientConfig(testOutboundName(service.History))))
}
