package historyonboardingfx

import (
	"testing"

	"github.com/cadence-workflow/shard-manager/service/sharddistributor/client/clientcommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
)

func TestNew(t *testing.T) {
	const numShards = 1000
	namespace := clientcommon.NamespaceConfig{Namespace: "cadence-history"}

	tests := []struct {
		name          string
		namespaces    []clientcommon.NamespaceConfig
		percentage    int
		wantEnabled   bool
		wantOnboarded map[int]bool
	}{
		{
			name:          "no namespace ignores the operational percentage",
			percentage:    50,
			wantEnabled:   false,
			wantOnboarded: map[int]bool{0: false, 499: false},
		},
		{
			name:          "namespace configured follows the operational percentage",
			namespaces:    []clientcommon.NamespaceConfig{namespace},
			percentage:    10,
			wantEnabled:   true,
			wantOnboarded: map[int]bool{0: true, 99: true, 100: false},
		},
		{
			name:          "namespace configured at zero percent",
			namespaces:    []clientcommon.NamespaceConfig{namespace},
			percentage:    0,
			wantEnabled:   false,
			wantOnboarded: map[int]bool{0: false},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := dynamicconfig.NewInMemoryClient()
			require.NoError(t, client.UpdateValue(dynamicproperties.HistoryPercentageOnboardedToShardManager, tt.percentage))

			var cfg config.Config
			cfg.ShardDistributorHistoryConfig.Namespaces = tt.namespaces
			cfg.Persistence.NumHistoryShards = numShards

			var gate *history.OnboardingGate
			fxtest.New(t,
				testlogger.Module(t),
				fx.Supply(cfg),
				fx.Provide(
					metrics.NewNoopMetricsClient,
					fx.Annotate(
						func() *dynamicconfig.Collection { return dynamicconfig.NewCollection(client, testlogger.New(t)) },
						fx.ResultTags(`name:"operational-dynamic-config"`),
					),
				),
				Module,
				fx.Populate(&gate),
			).RequireStart().RequireStop()

			assert.Equal(t, tt.wantEnabled, gate.Enabled())
			for shardID, want := range tt.wantOnboarded {
				assert.Equal(t, want, gate.ShardOnboarded(shardID), "shard %d", shardID)
			}
		})
	}
}
