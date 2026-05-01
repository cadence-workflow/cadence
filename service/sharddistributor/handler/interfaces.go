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

package handler

import (
	"context"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination interfaces_mock.go
//go:generate gowrap gen -g -p . -i Handler -t ../../templates/grpc.tmpl -o ../wrappers/grpc/grpc_handler_generated.go -v handler=GRPC -v package=sharddistributorv1 -v path=github.com/uber/cadence/.gen/proto/sharddistributor/v1 -v prefix=ShardDistributor
//go:generate gowrap gen -g -p . -i Executor -t ../../templates/grpc.tmpl -o ../wrappers/grpc/grpc_executor_generated.go -v handler=ExecutorGRPC -v package=sharddistributorv1 -v path=github.com/uber/cadence/.gen/proto/sharddistributor/v1 -v prefix=ShardDistributorExecutor
//go:generate gowrap gen -g -p . -i Handler -t ../templates/metered.tmpl -o ../wrappers/metered/api_generated.go -v handler=Metrics
//go:generate gowrap gen -g -p . -i Executor -t ../templates/metered.tmpl -o ../wrappers/metered/executor_generated.go -v handler=ExecutorMetrics

// Handler is the interface for shard distributor handler
type Handler interface {
	common.Daemon

	Health(context.Context) (*types.HealthStatus, error)

	GetShardOwner(context.Context, *types.GetShardOwnerRequest) (*types.GetShardOwnerResponse, error)

	WatchNamespaceState(*types.WatchNamespaceStateRequest, WatchNamespaceStateServer) error
}

type Executor interface {
	Heartbeat(context.Context, *types.ExecutorHeartbeatRequest) (*types.ExecutorHeartbeatResponse, error)
}

type WatchNamespaceStateServer interface {
	Context() context.Context
	Send(*types.WatchNamespaceStateResponse) error
}
