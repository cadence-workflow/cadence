package sql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

type sqlSchemaDB struct {
	logger log.Logger
	db     string
	crud   sqlplugin.AdminDB
	schema persistence.Schema
}

func NewSQLSchemaDB(logger log.Logger, db string, crud sqlplugin.AdminDB, schema persistence.Schema) persistence.SchemaDB {
	return &sqlSchemaDB{
		logger: logger,
		db:     db,
		crud:   crud,
		schema: schema,
	}
}

func (s *sqlSchemaDB) Close() {
	err := s.crud.Close()
	if err != nil {
		s.logger.Error("Failed to close schema DB", tag.Error(err))
	}
}

func (s *sqlSchemaDB) LatestSchema() persistence.Schema {
	return s.schema
}

func (s *sqlSchemaDB) HasSchemaVersioning(_ context.Context) (bool, error) {
	return s.crud.HasSchemaVersionTables()
}

func (s *sqlSchemaDB) SetupVersioning(_ context.Context) error {
	return s.crud.CreateSchemaVersionTables()
}

func (s *sqlSchemaDB) GetSchemaVersion(_ context.Context) (persistence.Version, error) {
	stringVersion, err := s.crud.ReadSchemaVersion(s.db)
	if errors.Is(err, sql.ErrNoRows) {
		return persistence.Version{}, nil
	}
	if err != nil {
		return persistence.Version{}, err
	}
	return persistence.ParseVersion(stringVersion)
}

func (s *sqlSchemaDB) UpdateSchema(ctx context.Context, update *persistence.SchemaUpdate) error {
	currentVersion, err := s.GetSchemaVersion(ctx)
	if err != nil {
		return err
	}
	if !currentVersion.IsBefore(update.Version) {
		return fmt.Errorf("cannot upgrade backwards from %s to %s", currentVersion, update.Version)
	}
	s.logger.Info("updating schema")
	err = s.applyUpdate(ctx, update)
	if err != nil {
		return fmt.Errorf("failed to apply update: %w", err)
	}
	// These two surprisingly aren't transactionally implemented, so it's possible we lose the update log
	e := s.crud.UpdateSchemaVersion(s.db, update.Version.String(), update.MinCompatibleVersion.String())
	if e != nil {
		return fmt.Errorf("error updating schema version: %w", e)
	}
	e = s.crud.WriteSchemaUpdateLog(currentVersion.String(), update.Version.String(), update.ManifestMD5, update.Description)
	if e != nil {
		return fmt.Errorf("error writing schema update log: %w", e)
	}
	s.logger.Info("schema update completed successfully")
	return nil
}

func (s *sqlSchemaDB) applyUpdate(ctx context.Context, update *persistence.SchemaUpdate) error {
	for _, stmt := range update.DDLStatements {
		e := s.crud.ExecSchemaOperationQuery(ctx, stmt)
		if e != nil {
			return fmt.Errorf("error executing DDL statement: %w", e)
		}
	}
	return nil
}
