package sharddistributorexecutorclient

import (
	"go.uber.org/fx"

	sharddistributorv1 "github.com/uber/cadence/.gen/proto/sharddistributor/v1"
	"github.com/uber/cadence/client/wrappers/grpc"
	timeoutwrapper "github.com/uber/cadence/client/wrappers/timeout"
	"github.com/uber/cadence/service/sharddistributor/client/executorclient"
)

// Params contains the dependencies needed to create a shard distributor client
type Params struct {
	fx.In

	YarpcClient sharddistributorv1.ShardDistributorExecutorAPIYARPCClient
}

// NewShardDistributorExecutorClient creates a new shard distributor executor client with GRPC and timeout wrappers
func NewShardDistributorExecutorClient(p Params) (executorclient.Client, error) {
	shardDistributorExecutorClient := grpc.NewShardDistributorExecutorClient(p.YarpcClient)
	shardDistributorExecutorClient = timeoutwrapper.NewShardDistributorExecutorClient(shardDistributorExecutorClient, timeoutwrapper.ShardDistributorExecutorDefaultTimeout)
	return shardDistributorExecutorClient, nil
}
