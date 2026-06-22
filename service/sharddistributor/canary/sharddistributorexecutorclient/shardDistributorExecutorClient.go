package sharddistributorexecutorclient

import (
	sharddistributorv1 "github.com/cadence-workflow/shard-manager/.gen/proto/sharddistributor/v1"
	smgrpc "github.com/cadence-workflow/shard-manager/client/wrappers/grpc"
	smtimeoutwrapper "github.com/cadence-workflow/shard-manager/client/wrappers/timeout"
	"github.com/cadence-workflow/shard-manager/service/sharddistributor/client/executorclient"
	"go.uber.org/fx"
)

type Params struct {
	fx.In

	YarpcClient sharddistributorv1.ShardDistributorExecutorAPIYARPCClient
}

func NewShardDistributorExecutorClient(p Params) (executorclient.Client, error) {
	client := smgrpc.NewShardDistributorExecutorClient(p.YarpcClient)
	client = smtimeoutwrapper.NewShardDistributorExecutorClient(client, smtimeoutwrapper.ShardDistributorExecutorDefaultTimeout)
	return client, nil
}
