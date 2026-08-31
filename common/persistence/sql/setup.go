package sql

import (
	"context"
	"fmt"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

type sqlSetupDB struct {
	logger log.Logger
	db     string
	crud   sqlplugin.AdminDB
}

func NewSQLSetupDB(logger log.Logger, db string, crud sqlplugin.AdminDB) persistence.SetupDB {
	return &sqlSetupDB{
		logger: logger,
		db:     db,
		crud:   crud,
	}
}

func (s *sqlSetupDB) Close() {
	err := s.crud.Close()
	if err != nil {
		s.logger.Error("Failed to close the SQL setup", tag.Error(err))
	}
}

func (s *sqlSetupDB) IsSetup(_ context.Context) (bool, error) {
	return s.crud.DatabaseExists(s.db)
}

func (s *sqlSetupDB) Setup(_ context.Context, _ map[string]string) error {
	err := s.crud.CreateDatabase(s.db)
	if err != nil {
		return fmt.Errorf("error creating database %s: %w", s.db, err)
	}
	return nil
}

func (s *sqlSetupDB) Teardown(_ context.Context) error {
	err := s.crud.DropDatabase(s.db)
	if err != nil {
		return fmt.Errorf("error tearing down database %s: %w", s.db, err)
	}
	return nil
}
