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

// createSharedDBConn creates a new database connection in the dbPool if one for the given dsn
// doesn't exist yet. Subsequent calls with the same dsn increment a reference count and return
// the existing connection, ensuring that all callers within a process share a single *sqlx.DB.
// This is important for SQLite because:
//   - File-based: SQLite takes exclusive write locks, so multiple *sqlx.DB to the same file causes "database is locked" errors.
//   - In-memory: each *sqlx.DB is an isolated database; sharing one ensures all services see the same data.
func createSharedDBConn(dsn string, createDBConnFn func() (*sqlx.DB, error)) (*sqlx.DB, error) {
	dbPoolMx.Lock()
	defer dbPoolMx.Unlock()

	if db, ok := dbPool[dsn]; ok {
		dbPoolCounter[dsn]++
		return db, nil
	}

	db, err := createDBConnFn()
	if err != nil {
		return nil, err
	}

	dbPool[dsn] = db
	dbPoolCounter[dsn]++

	return db, nil
}

// closeSharedDBConn decrements the reference count for the given dsn and closes the underlying
// connection only when the count reaches zero.
func closeSharedDBConn(dsn string, closeDBConnFn func() error) error {
	dbPoolMx.Lock()
	defer dbPoolMx.Unlock()

	dbPoolCounter[dsn]--
	if dbPoolCounter[dsn] != 0 {
		return nil
	}

	delete(dbPool, dsn)
	delete(dbPoolCounter, dsn)
	return closeDBConnFn()
}
