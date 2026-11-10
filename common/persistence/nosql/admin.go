package nosql

import (
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

type nosqlAdmin struct {
	logger     log.Logger
	plugin     nosqlplugin.Plugin
	dbType     persistence.DBType
	identifier string
	cfg        *config.NoSQL
}

func (n *nosqlAdmin) PluginName() string {
	return n.cfg.PluginName
}

func (n *nosqlAdmin) DBType() persistence.DBType {
	return n.dbType
}

func (n *nosqlAdmin) Identifier() string {
	return n.identifier
}

func (n *nosqlAdmin) CreateSetupDB() (persistence.SetupDB, error) {
	return n.plugin.SetupDB(n.cfg, n.logger, persistence.NewDefaultDynamicConfiguration())
}

func (n *nosqlAdmin) SupportsSchema() bool {
	return true
}

func (n *nosqlAdmin) CreateSchemaDB() (persistence.SchemaDB, error) {
	return n.plugin.SchemaDB(n.dbType, n.cfg, n.logger, persistence.NewDefaultDynamicConfiguration())
}
