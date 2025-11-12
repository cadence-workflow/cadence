package etcd

import (
	"go.uber.org/fx"

	"github.com/uber/cadence/service/sharddistributor/store/etcd/executorstore"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/leaderstore"
)

var Module = fx.Module("etcd",
	fx.Provide(executorstore.NewStore),
	fx.Provide(leaderstore.NewLeaderStore),
)
