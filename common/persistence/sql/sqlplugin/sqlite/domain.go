package sqlite

import (
	"context"

	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

const (
	lockDomainMetadataQuery = `SELECT notification_version FROM domain_metadata`
)

// LockDomainMetadata acquires a write lock on a single row in domain_metadata table
func (mdb *DB) LockDomainMetadata(ctx context.Context) error {
	var row sqlplugin.DomainMetadataRow
	err := mdb.driver.GetContext(ctx, sqlplugin.DbDefaultShard, &row.NotificationVersion, lockDomainMetadataQuery)
	return err
}
