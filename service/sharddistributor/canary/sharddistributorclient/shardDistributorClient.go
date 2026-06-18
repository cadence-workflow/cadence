package sharddistributorclient

import (
	sharddistributorv1 "github.com/cadence-workflow/shard-manager/.gen/proto/sharddistributor/v1"
	"github.com/cadence-workflow/shard-manager/client/sharddistributor"
	smgrpc "github.com/cadence-workflow/shard-manager/client/wrappers/grpc"
	smtimeoutwrapper "github.com/cadence-workflow/shard-manager/client/wrappers/timeout"
	"go.uber.org/fx"
)

type Params struct {
	fx.In

	YarpcClient sharddistributorv1.ShardDistributorAPIYARPCClient
}

func NewShardDistributorClient(p Params) (sharddistributor.Client, error) {
	shardDistributorClient := smgrpc.NewShardDistributorClient(p.YarpcClient)
	shardDistributorClient = smtimeoutwrapper.NewShardDistributorClient(shardDistributorClient, smtimeoutwrapper.ShardDistributorExecutorDefaultTimeout)
	return shardDistributorClient, nil
}
