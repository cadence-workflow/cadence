package sql

import (
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

type sqlAdmin struct {
	logger log.Logger
	plugin sqlplugin.Plugin
	schema persistence.Schema
	cfg    *config.SQL
}

func (s *sqlAdmin) CreateSetupDB() (persistence.SetupDB, error) {
	// The DB might not exist, so connect without it. This matches the existing sql-tool
	cfgCopy := *s.cfg
	cfgCopy.DatabaseName = ""
	adminDB, err := s.plugin.CreateAdminDB(&cfgCopy)
	if err != nil {
		return nil, err
	}
	return NewSQLSetupDB(s.logger, s.cfg.DatabaseName, adminDB), nil
}

func (s *sqlAdmin) SupportsSchema() bool {
	return true
}

func (s *sqlAdmin) CreateSchemaDB() (persistence.SchemaDB, error) {
	adminDB, err := s.plugin.CreateAdminDB(s.cfg)
	if err != nil {
		return nil, err
	}
	return NewSQLSchemaDB(s.logger, s.cfg.DatabaseName, adminDB, s.schema), nil
}
