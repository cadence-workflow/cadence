package historyonboardingfx

import (
	"go.uber.org/fx"

	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
)

// Module provides the history shard onboarding gate for fx application
var Module = fx.Module("historyonboardingfx",
	fx.Provide(New),
)

// Params are the dependencies for creating the history shard onboarding gate
type Params struct {
	fx.In

	Cfg                      config.Config
	OperationalDynamicConfig *dynamicconfig.Collection `name:"operational-dynamic-config"`
	MetricsClient            metrics.Client
	Logger                   log.Logger
}

// New creates the history shard onboarding gate
// Without a shard-distributor-history namespace, the percentage is pinned to 0
func New(p Params) *history.OnboardingGate {
	percentageOnboarded := p.OperationalDynamicConfig.GetIntProperty(dynamicproperties.HistoryPercentageOnboardedToShardManager)
	if len(p.Cfg.ShardDistributorHistoryConfig.Namespaces) == 0 {
		p.Logger.Info("history shard-manager onboarding not configured; treating onboarding percentage as 0")
		percentageOnboarded = dynamicproperties.GetIntPropertyFn(0)
	}
	return history.NewOnboardingGate(p.MetricsClient, percentageOnboarded, p.Cfg.Persistence.NumHistoryShards)
}
