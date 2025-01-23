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
	converter   DataConverter
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
		converter:   &converter{},
		driver:      driver,
		originalDBs: xdbs, // this is kept because newDB will be called again when starting a transaction
		numDBShards: numDBShards,
	}

	return db, nil
}

// GetTotalNumDBShards returns the total number of shards
func (mdb *db) GetTotalNumDBShards() int {
	return mdb.numDBShards
}

// BeginTx starts a new transaction and returns a reference to the Tx object
func (mdb *db) BeginTx(ctx context.Context, dbShardID int) (sqlplugin.Tx, error) {
	xtx, err := mdb.driver.BeginTxx(ctx, dbShardID, nil)
	if err != nil {
		return nil, err
	}
	return newDB(mdb.originalDBs, xtx, dbShardID, mdb.numDBShards)
}

// PluginName returns the name of the plugin
func (mdb *db) PluginName() string {
	return PluginName
}

// Close closes the connection to the db
func (mdb *db) Close() error {
	return mdb.driver.Close()
}

// Commit commits a previously started transaction
func (mdb *db) Commit() error {
	return mdb.driver.Commit()
}

// Rollback triggers rollback of a previously started transaction
func (mdb *db) Rollback() error {
	return mdb.driver.Rollback()
}

// SupportsTTL returns weather SQLite supports TTL
func (mdb *db) SupportsTTL() bool {
	return false
}

// MaxAllowedTTL returns the max allowed ttl SQLite supports
func (mdb *db) MaxAllowedTTL() (*time.Duration, error) {
	return nil, sqlplugin.ErrTTLNotSupported
}

// SupportsAsyncTransaction returns weather SQLite supports Asynchronous transaction
func (mdb *db) SupportsAsyncTransaction() bool {
	return false
}
