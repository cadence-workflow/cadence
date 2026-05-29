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

package grpc

import (
	"context"

	adminv1 "github.com/uber/cadence-idl/go/proto/admin/v1"
	apiv1 "github.com/uber/cadence-idl/go/proto/api/v1"
	"go.uber.org/yarpc"

	"github.com/uber/cadence/common/types/mapper/proto"
)

func (g AdminHandler) Register(dispatcher *yarpc.Dispatcher) {
	dispatcher.Register(adminv1.BuildAdminAPIYARPCProcedures(g))
}

func (g APIHandler) Register(dispatcher *yarpc.Dispatcher) {
	dispatcher.Register(apiv1.BuildDomainAPIYARPCProcedures(g))
	dispatcher.Register(apiv1.BuildWorkflowAPIYARPCProcedures(g))
	dispatcher.Register(apiv1.BuildWorkerAPIYARPCProcedures(g))
	dispatcher.Register(apiv1.BuildVisibilityAPIYARPCProcedures(g))
	dispatcher.Register(apiv1.BuildMetaAPIYARPCProcedures(g))
	dispatcher.Register(apiv1.BuildScheduleAPIYARPCProcedures(g))
}

func (g APIHandler) Health(ctx context.Context, request *apiv1.HealthRequest) (*apiv1.HealthResponse, error) {
	response, err := g.h.Health(ctx)
	return proto.FromHealthResponse(response), proto.FromError(err)
}
