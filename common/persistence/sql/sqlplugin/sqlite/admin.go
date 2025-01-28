package sqlite

import "errors"

// CreateDatabase is not supported by sqlite
// each sqlite file is a database
func (mdb *DB) CreateDatabase(name string) error {
	return errors.New("sqlite doesn't support creating database")
}

// DropDatabase is not supported by sqlite
// each sqlite file is a database
func (mdb *DB) DropDatabase(name string) error {
	return errors.New("sqlite doesn't support dropping database")
}
