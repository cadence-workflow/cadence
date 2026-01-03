package configtest

import (
	"testing"

	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/service/sharddistributor/config"
)

type ConfigEntry struct {
	Key   dynamicproperties.Key
	Value interface{}
}

func NewTestMigrationConfig(t *testing.T, configEntries ...ConfigEntry) *config.Config {
	client := dynamicconfig.NewInMemoryClient()
	for _, entry := range configEntries {
		err := client.UpdateValue(entry.Key, entry.Value)
		if err != nil {
			t.Errorf("Failed to update config ")
		}
	}
	dc := dynamicconfig.NewCollection(client, testlogger.New(t))
	migrationConfig := config.NewConfig(dc)
	return migrationConfig
}
