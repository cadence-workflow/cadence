package cassandra

import (
	"context"
	"fmt"
	"time"

	"github.com/uber/cadence/common/persistence"
)

const (
	readSchemaVersionCQL        = `SELECT curr_version from schema_version where keyspace_name=?`
	hasTableCQL                 = `SELECT COUNT(*) FROM system_schema.tables WHERE keyspace_name = ? AND table_name = ?;`
	writeSchemaVersionCQL       = `INSERT into schema_version(keyspace_name, creation_time, curr_version, min_compatible_version) VALUES (?,?,?,?)`
	writeSchemaUpdateHistoryCQL = `INSERT into schema_update_history(year, month, update_time, old_version, new_version, manifest_md5, description) VALUES(?,?,?,?,?,?,?)`

	createSchemaVersionTableCQL = `CREATE TABLE schema_version(keyspace_name text PRIMARY KEY, ` +
		`creation_time timestamp, ` +
		`curr_version text, ` +
		`min_compatible_version text);`

	createSchemaUpdateHistoryTableCQL = `CREATE TABLE schema_update_history(` +
		`year int, ` +
		`month int, ` +
		`update_time timestamp, ` +
		`description text, ` +
		`manifest_md5 text, ` +
		`new_version text, ` +
		`old_version text, ` +
		`PRIMARY KEY ((year, month), update_time));`
)

type schemaDB struct {
	*CDB
	latest persistence.Schema
}

func (s *schemaDB) LatestSchema() persistence.Schema {
	return s.latest
}

func (db *CDB) HasSchemaVersioning(ctx context.Context) (bool, error) {
	query := db.session.Query(hasTableCQL)
	query.Bind(db.cfg.Keyspace, "schema_version")
	iter := query.Iter()
	var count int
	if !iter.Scan(&count) {
		err := iter.Close()
		return false, fmt.Errorf("error checking for schema_version table: %w", err)
	}
	return count > 0, nil
}

func (db *CDB) SetupVersioning(_ context.Context) error {
	if err := db.session.Query(createSchemaVersionTableCQL).Exec(); err != nil {
		return err
	}
	return db.session.Query(createSchemaUpdateHistoryTableCQL).Exec()
}

func (db *CDB) GetSchemaVersion(ctx context.Context) (persistence.Version, error) {
	query := db.session.Query(readSchemaVersionCQL, db.cfg.Keyspace)
	iter := query.Iter()
	var version string
	if !iter.Scan(&version) {
		err := iter.Close()
		return persistence.Version{}, err
	}
	if err := iter.Close(); err != nil {
		return persistence.Version{}, err
	}
	return persistence.ParseVersion(version)
}

func (db *CDB) UpdateSchema(ctx context.Context, update *persistence.SchemaUpdate) error {
	current, err := db.GetSchemaVersion(ctx)
	if err != nil {
		return err
	}
	if !current.IsBefore(update.Version) {
		return fmt.Errorf("unable to update backwards from %s to %s", current, update.Version)
	}
	err = db.applyUpdate(ctx, update)
	if err != nil {
		return fmt.Errorf("unable to apply update: %w", err)
	}

	query := db.session.Query(writeSchemaVersionCQL, db.cfg.Keyspace, time.Now(), update.Version.String(), update.MinCompatibleVersion.String())
	err = query.Exec()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	query = db.session.Query(writeSchemaUpdateHistoryCQL)
	query.Bind(now.Year(), int(now.Month()), now, current.String(), update.Version.String(), update.ManifestMD5, update.Description)
	return query.Exec()
}

func (db *CDB) applyUpdate(ctx context.Context, update *persistence.SchemaUpdate) error {
	for _, ddl := range update.DDLStatements {
		err := db.session.Query(ddl).Exec()
		if err != nil {
			return err
		}
	}
	return nil
}
