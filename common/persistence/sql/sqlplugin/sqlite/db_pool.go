package sqlite

import (
	"sync"

	"github.com/jmoiron/sqlx"
)

// SQLite database takes exclusive access to the database file during write operation.
// If another process tries to do another write operation, it returns the error database to be locked.
// To be sure that only database connection is created per one database file within the running process,
// we use dbPool to keep track of the database connections and reuse them.
// Each time when a new db connection creation is called, it will increment dbPoolCounter and returns already stored sql.DB
// from dbPool. If the connection is requested to be closed, it will decrement dbPoolCounter.
// When the counter reaches 0, it will close the database connection, as there are no more references to it.
// Reference: https://github.com/mattn/go-sqlite3/issues/274#issuecomment-232942571
var (
	dbPool        = make(map[string]*sqlx.DB)
	dbPoolCounter = make(map[string]int)
	dbPoolMx      sync.Mutex
)

// createPolledDBConn creates a new database connection in the dbPool if it doesn't exist.
func createPolledDBConn(databaseName string, createDBConnFn func() (*sqlx.DB, error)) (*sqlx.DB, error) {
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

// createPolledDBConn closes the database connection in the dbPool if it exists.
func closePolledDBConn(databaseName string, closeFn func() error) error {
	dbPoolMx.Lock()
	defer dbPoolMx.Unlock()

	dbPoolCounter[databaseName]--
	if dbPoolCounter[databaseName] != 0 {
		return nil
	}

	delete(dbPool, databaseName)
	delete(dbPoolCounter, databaseName)
	return closeFn()
}
