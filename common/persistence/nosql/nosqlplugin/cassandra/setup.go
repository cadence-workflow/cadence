package cassandra

import (
	"context"
	"fmt"
	"strconv"
)

const (
	hasKeyspaceCQL    = `SELECT COUNT(*) FROM system_schema.keyspaces WHERE keyspace_name = ?;`
	createKeyspaceCQL = `CREATE KEYSPACE IF NOT EXISTS %v ` +
		`WITH replication = { 'class' : 'SimpleStrategy', 'replication_factor' : %v};`

	createNTSKeyspaceCQL = `CREATE KEYSPACE IF NOT EXISTS %v ` +
		`WITH replication = { 'class' : 'NetworkTopologyStrategy', '%v' : %v};`
	dropKeyspaceCQL = "DROP KEYSPACE IF EXISTS %v"
)

func (db *CDB) IsSetup(ctx context.Context) (bool, error) {
	query := db.session.Query(hasKeyspaceCQL)
	query.Bind(db.cfg.Keyspace)
	iter := query.Iter()
	var count int
	if !iter.Scan(&count) {
		err := iter.Close()
		return false, fmt.Errorf("error checking for keyspace: %w", err)
	}
	return count > 0, nil
}

func (db *CDB) Setup(ctx context.Context, options map[string]string) error {
	replicationFactorString, ok := options["replication_factor"]
	if !ok {
		replicationFactorString = "3"
	}
	replicationFactor, err := strconv.Atoi(replicationFactorString)
	if err != nil {
		return fmt.Errorf("error parsing replication factor: %w", err)
	}
	var cql string
	if db.cfg.Datacenter != "" {
		// Our current tooling supports only a single dc, we could maybe accept more from the options but
		// maybe it's better that users mange their keyspace on their own for sophisticated setups
		cql = fmt.Sprintf(createNTSKeyspaceCQL, db.cfg.Keyspace, db.cfg.Datacenter, replicationFactor)
	} else {
		cql = fmt.Sprintf(createKeyspaceCQL, db.cfg.Keyspace, replicationFactor)
	}
	return db.session.Query(cql).Exec()
}

func (db *CDB) Teardown(ctx context.Context) error {
	return db.session.Query(fmt.Sprintf(dropKeyspaceCQL, db.cfg.Keyspace)).Exec()
}
