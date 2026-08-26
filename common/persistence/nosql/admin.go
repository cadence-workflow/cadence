package nosql

import (
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

type nosqlAdmin struct {
	logger log.Logger
	plugin nosqlplugin.Plugin
	dbType persistence.DBType
	cfg    *config.NoSQL
}

func (n *nosqlAdmin) CreateSetupDB() (persistence.SetupDB, error) {
	return n.plugin.SetupDB(n.cfg, n.logger, &persistence.DynamicConfiguration{})
}

func (n *nosqlAdmin) SupportsSchema() bool {
	return true
}

func (n *nosqlAdmin) CreateSchemaDB() (persistence.SchemaDB, error) {
	return n.plugin.SchemaDB(n.dbType, n.cfg, n.logger, &persistence.DynamicConfiguration{})
}
