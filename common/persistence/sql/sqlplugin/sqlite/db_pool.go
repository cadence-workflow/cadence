package sqlite

import (
	"sync"

	"github.com/jmoiron/sqlx"
)

// SQLite database takes exclusive access to the database file during write operations.
// If another process attempts to perform a write operation, it receives a "database is locked" error.
// To ensure only one database connection is created per database file within the running process,
// we use dbPool to track and reuse database connections.
// When a new database connection is requested, it increments dbPoolCounter and returns the existing sql.DB
// from dbPool. When a connection is requested to be closed, it decrements dbPoolCounter.
// Once the counter reaches 0, the database connection is closed as there are no more references to it.
// Reference: https://github.com/mattn/go-sqlite3/issues/274#issuecomment-232942571
var (
	dbPool        = make(map[string]*sqlx.DB)
	dbPoolCounter = make(map[string]int)
	dbPoolMx      sync.Mutex
)

// createSharedDBConn creates a new database connection in the dbPool if it doesn't exist.
func createSharedDBConn(databaseName string, createDBConnFn func() (*sqlx.DB, error)) (*sqlx.DB, error) {
	dbPoolMx.Lock()
	defer dbPoolMx.Unlock()

	if db, ok := dbPool[databaseName]; ok {
		dbPoolCounter[databaseName]++
		return db, nil
	}

	db, err := createDBConnFn()
	if err != nil {
		return nil, err
	}

	dbPool[databaseName] = db
	dbPoolCounter[databaseName]++

	return db, nil
}

// closeSharedDBConn closes the database connection in the dbPool if it exists.
func closeSharedDBConn(databaseName string, closeDBConnFn func() error) error {
	dbPoolMx.Lock()
	defer dbPoolMx.Unlock()

	dbPoolCounter[databaseName]--
	if dbPoolCounter[databaseName] != 0 {
		return nil
	}

	delete(dbPool, databaseName)
	delete(dbPoolCounter, databaseName)
	return closeDBConnFn()
}
