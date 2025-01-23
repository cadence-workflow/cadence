package sqlite

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/uber/cadence/common/persistence/sql/sqldriver"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

var (
	_ sqlplugin.AdminDB = (*db)(nil)
	_ sqlplugin.DB      = (*db)(nil)
	_ sqlplugin.Tx      = (*db)(nil)
)

type db struct {
	driver      sqldriver.Driver
	originalDBs []*sqlx.DB
	numDBShards int
}

// newDB returns a new instance of db
func newDB(xdbs []*sqlx.DB, tx *sqlx.Tx, dbShardID int, numDBShards int) (*db, error) {
	driver, err := sqldriver.NewDriver(xdbs, tx, dbShardID)
	if err != nil {
		return nil, err
	}

	db := &db{
		originalDBs: xdbs, // this is kept because newDB will be called again when starting a transaction
		driver:      driver,
		numDBShards: numDBShards,
	}

	return db, nil
}

// GetTotalNumDBShards returns the total number of shards
func (db *db) GetTotalNumDBShards() int {
	return db.numDBShards
}

// BeginTx starts a new transaction and returns a reference to the Tx object
func (db *db) BeginTx(ctx context.Context, dbShardID int) (sqlplugin.Tx, error) {
	xtx, err := db.driver.BeginTxx(ctx, dbShardID, nil)
	if err != nil {
		return nil, err
	}
	return newDB(db.originalDBs, xtx, dbShardID, db.numDBShards)
}

// PluginName returns the name of the plugin
func (db *db) PluginName() string {
	return PluginName
}

// Close closes the connection to the db
func (db *db) Close() error {
	return db.driver.Close()
}

// Commit commits a previously started transaction
func (db *db) Commit() error {
	return db.driver.Commit()
}

// Rollback triggers rollback of a previously started transaction
func (db *db) Rollback() error {
	return db.driver.Rollback()
}

// SupportsTTL returns weather SQLite supports TTL
func (db *db) SupportsTTL() bool {
	return false
}

// MaxAllowedTTL returns the max allowed ttl SQLite supports
func (db *db) MaxAllowedTTL() (*time.Duration, error) {
	return nil, sqlplugin.ErrTTLNotSupported
}

// SupportsAsyncTransaction returns weather SQLite supports Asynchronous transaction
func (db *db) SupportsAsyncTransaction() bool {
	return false
}

// IsDupEntryError verify if the error is a duplicate entry error
func (db *db) IsDupEntryError(err error) bool {
	//TODO implement me
	panic("implement me")
}

// IsNotFoundError verify if the error is a not found error
func (db *db) IsNotFoundError(err error) bool {
	//TODO implement me
	panic("implement me")
}

// IsTimeoutError verify if the error is a timeout error
func (db *db) IsTimeoutError(err error) bool {
	//TODO implement me
	panic("implement me")
}

// IsThrottlingError verify if the error is a throttling error
func (db *db) IsThrottlingError(err error) bool {
	//TODO implement me
	panic("implement me")
}
