package batch

import (
	"context"

	"go.uber.org/yarpc"

	"github.com/uber/cadence/common/types"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination interface_mock.go -package batch github.com/uber/cadence/client/batch Client
//go:generate gowrap gen -g -p . -i Client -t ../templates/grpc.tmpl -o ../wrappers/grpc/batch_generated.go -v client=Batch -v package=apiv1 -v path=github.com/uber/cadence-idl/go/proto/api/v1 -v prefix=

// Client is the interface exposed by the batch operations API. Only
// StartBatchOperation is implemented so far; add the rest as they land.
type Client interface {
	StartBatchOperation(ctx context.Context, request *types.StartBatchOperationRequest, opts ...yarpc.CallOption) (*types.StartBatchOperationResponse, error)
}
