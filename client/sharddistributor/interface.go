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

package sharddistributor

import (
	"context"

	"go.uber.org/yarpc"

	"github.com/uber/cadence/common/types"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination interface_mock.go -self_package github.com/uber/cadence/client/sharddistributor
//go:generate gowrap gen -g -p . -i Client -t ../templates/retry.tmpl -o ../wrappers/retryable/sharddistributor_generated.go -v client=ShardDistributor
//go:generate gowrap gen -g -p . -i Client -t ../templates/metered.tmpl -o ../wrappers/metered/sharddistributor_generated.go -v client=ShardDistributor
//go:generate gowrap gen -g -p . -i Client -t ../templates/errorinjectors.tmpl -o ../wrappers/errorinjectors/sharddistributor_generated.go -v client=ShardDistributor
//go:generate gowrap gen -g -p . -i Client -t ../templates/grpc.tmpl -o ../wrappers/grpc/sharddistributor_generated.go -v client=ShardDistributor -v package=sharddistributorv1 -v path=github.com/uber/cadence/.gen/proto/sharddistributor/v1 -v prefix=ShardDistributor
//go:generate gowrap gen -g -p . -i Client -t ../templates/timeout.tmpl -o ../wrappers/timeout/sharddistributor_generated.go -v client=ShardDistributor

type Client interface {
	GetShardOwner(context.Context, *types.GetShardOwnerRequest, ...yarpc.CallOption) (*types.GetShardOwnerResponse, error)
	WatchNamespaceState(context.Context, *types.WatchNamespaceStateRequest, ...yarpc.CallOption) (WatchNamespaceStateClient, error)
}

type WatchNamespaceStateClient interface {
	Context() context.Context
	Recv(...yarpc.StreamOption) (*types.WatchNamespaceStateResponse, error)
	CloseSend(...yarpc.StreamOption) error
}
