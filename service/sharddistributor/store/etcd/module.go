package etcd

import (
	"go.uber.org/fx"

	"github.com/uber/cadence/service/sharddistributor/store/etcd/executorstore"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/leaderstore"
)

var Module = fx.Module("etcd",
	executorstore.Module,
	fx.Provide(leaderstore.NewLeaderStore),
)
