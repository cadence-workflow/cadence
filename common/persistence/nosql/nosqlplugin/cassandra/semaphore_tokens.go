// Copyright (c) 2025 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package cassandra

import (
	"context"

	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
	"github.com/uber/cadence/common/types"
)

// Row kinds for the `type` clustering column of semaphore_tokens.
const (
	rowTypeSemaphoreToken = iota // forward index row, keyed by token_id (token_id -> holder)
	rowTypeSemaphoreOwner        // reverse index row, keyed by owner_id (owner_id -> held_token)
)

// Sentinels stored in the columns that do not apply to a given row kind.
//
// The int sentinels are negative / out-of-range (real slot ids are >= 1),
// matching the executions/tasks convention (rowTypeExecutionTaskID=-10,
// taskListTaskID=-12345) rather than 0.
//
// The text sentinels are PROVISIONAL. Their final literals are co-designed with
// the owner_id encoding, which must guarantee a real owner_id can never equal
// freeSentinel (the one LWT-compared value). ownerNoneSentinel is only a
// type-protected placeholder (a token row can never collide with an owner row),
// so its value is not load-bearing.
// TODO(semaphore, Phase 2): finalize freeSentinel/ownerNoneSentinel with the owner_id encoding.
const (
	emptyTokenID   = -1 // token_id on owner rows
	emptyHeldToken = -1 // held_token on token rows

	ownerNoneSentinel = "__NONE__" // owner_id on token rows; holder on owner rows
	freeSentinel      = "__FREE__" // holder of an unheld token row
)

const (
	templateSeedSemaphoreTokenQuery = `INSERT INTO semaphore_tokens (` +
		`domain_id, semaphore_name, bucket, type, token_id, owner_id, holder, held_token, updated_time) ` +
		`VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?) IF NOT EXISTS`

	// Grant: conditional in-place UPDATE of the token row (claim only if free) ...
	templateGrantSemaphoreTokenUpdateQuery = `UPDATE semaphore_tokens ` +
		`SET holder = ?, updated_time = ? ` +
		`WHERE domain_id = ? AND semaphore_name = ? AND bucket = ? AND type = ? AND token_id = ? AND owner_id = ? ` +
		`IF holder = ?`

	// ... plus the matching owner (reverse-index) row INSERT, in the same batch.
	templateGrantSemaphoreOwnerInsertQuery = `INSERT INTO semaphore_tokens (` +
		`domain_id, semaphore_name, bucket, type, token_id, owner_id, holder, held_token, updated_time) ` +
		`VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// Release: guarded in-place UPDATE of the token row (clear only if still held
	// by this owner) ...
	templateReleaseSemaphoreTokenUpdateQuery = `UPDATE semaphore_tokens ` +
		`SET holder = ?, updated_time = ? ` +
		`WHERE domain_id = ? AND semaphore_name = ? AND bucket = ? AND type = ? AND token_id = ? AND owner_id = ? ` +
		`IF holder = ?`

	// ... plus the matching owner row DELETE, in the same batch.
	templateReleaseSemaphoreOwnerDeleteQuery = `DELETE FROM semaphore_tokens ` +
		`WHERE domain_id = ? AND semaphore_name = ? AND bucket = ? AND type = ? AND token_id = ? AND owner_id = ?`

	// Forward read: owner_id is the trailing clustering column, so it is omitted.
	templateSelectSemaphoreTokenByIDQuery = `SELECT ` +
		`domain_id, semaphore_name, bucket, token_id, owner_id, holder, held_token, updated_time ` +
		`FROM semaphore_tokens ` +
		`WHERE domain_id = ? AND semaphore_name = ? AND bucket = ? AND type = ? AND token_id = ?`

	// Reverse read: token_id (a middle clustering column) must be pinned to reach owner_id.
	templateSelectSemaphoreTokenByOwnerQuery = `SELECT ` +
		`domain_id, semaphore_name, bucket, token_id, owner_id, holder, held_token, updated_time ` +
		`FROM semaphore_tokens ` +
		`WHERE domain_id = ? AND semaphore_name = ? AND bucket = ? AND type = ? AND token_id = ? AND owner_id = ?`

	templateSelectSemaphoreTokensByBucketQuery = `SELECT ` +
		`domain_id, semaphore_name, bucket, type, token_id, owner_id, holder, held_token, updated_time ` +
		`FROM semaphore_tokens ` +
		`WHERE domain_id = ? AND semaphore_name = ? AND bucket = ?`
)

// InsertSemaphoreTokens seeds a bucket with free token rows for the given rows'
// TokenIDs, using a single conditional (LWT) batch of INSERT ... IF NOT EXISTS.
//
// Contract: callers must supply a bucket's FULL, IMMUTABLE id set. A bucket's id
// range is fixed at semaphore creation and never grows (to change size/bucket_size
// you create a new semaphore name), so seeding is only ever a fresh insert or a
// re-seed of the exact same set — never a superset. Within those two cases:
//   - fresh bucket: no rows exist, all conditions pass, all rows are inserted;
//   - re-seed of the same set: every row already exists, so every condition fails
//     and the batch is a deliberate no-op that never clobbers an already-held slot.
//
// The applied flag is therefore intentionally ignored: for a same-set re-seed
// "not applied" is the desired outcome, not an error.
//
// This relies on the immutability contract. A conditional batch is all-or-nothing:
// if it were ever called with a partial superset (a subset of the ids already
// present), the existing rows' failed conditions would reject the WHOLE batch, so
// the brand-new ids would be silently NOT inserted. Growing a bucket is unsupported
// by design.
func (db *CDB) InsertSemaphoreTokens(ctx context.Context, rows []*nosqlplugin.SemaphoreTokenRow) error {
	if len(rows) == 0 {
		return nil
	}
	batch := db.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	for _, row := range rows {
		batch.Query(templateSeedSemaphoreTokenQuery,
			row.DomainID,
			row.SemaphoreName,
			row.Bucket,
			rowTypeSemaphoreToken, // type = 0 (forward "token" row)
			row.TokenID,
			ownerNoneSentinel, // owner_id key = __NONE__
			freeSentinel,      // holder = __FREE__, the slot is unheld
			emptyHeldToken,
			row.UpdatedTime,
		)
	}
	_, iter, err := db.session.MapExecuteBatchCAS(batch, make(map[string]interface{}))
	if iter != nil {
		_ = iter.Close()
	}
	return err
}

// GrantSemaphoreToken claims row.TokenID for row.OwnerID via one atomic batch:
// the token row's holder is set to the owner only if it is currently free, and
// the matching owner row is inserted. Returns applied == false (not an error)
// when the slot was not free.
func (db *CDB) GrantSemaphoreToken(ctx context.Context, row *nosqlplugin.SemaphoreTokenRow) (bool, error) {
	batch := db.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	batch.Query(templateGrantSemaphoreTokenUpdateQuery,
		row.OwnerID,     // SET holder = owner_id
		row.UpdatedTime, // SET updated_time
		row.DomainID,
		row.SemaphoreName,
		row.Bucket,
		rowTypeSemaphoreToken,
		row.TokenID,
		ownerNoneSentinel, // token row's owner_id key
		freeSentinel,      // IF holder = FREE
	)
	batch.Query(templateGrantSemaphoreOwnerInsertQuery,
		row.DomainID,
		row.SemaphoreName,
		row.Bucket,
		rowTypeSemaphoreOwner,
		emptyTokenID,
		row.OwnerID,
		ownerNoneSentinel, // owner row's holder placeholder
		row.TokenID,       // held_token
		row.UpdatedTime,
	)
	applied, iter, err := db.session.MapExecuteBatchCAS(batch, make(map[string]interface{}))
	if iter != nil {
		_ = iter.Close()
	}
	if err != nil {
		return false, err
	}
	return applied, nil
}

// ReleaseSemaphoreToken frees row.TokenID via one atomic batch: the token row's
// holder is reset to free only if it is still held by row.OwnerID, and the
// matching owner row is deleted. Returns applied == false (not an error) for a
// best-effort no-op (something else already touched the slot).
func (db *CDB) ReleaseSemaphoreToken(ctx context.Context, row *nosqlplugin.SemaphoreTokenRow) (bool, error) {
	batch := db.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	batch.Query(templateReleaseSemaphoreTokenUpdateQuery,
		freeSentinel,    // SET holder = FREE
		row.UpdatedTime, // SET updated_time
		row.DomainID,
		row.SemaphoreName,
		row.Bucket,
		rowTypeSemaphoreToken,
		row.TokenID,
		ownerNoneSentinel,
		row.OwnerID, // IF holder = owner_id
	)
	batch.Query(templateReleaseSemaphoreOwnerDeleteQuery,
		row.DomainID,
		row.SemaphoreName,
		row.Bucket,
		rowTypeSemaphoreOwner,
		emptyTokenID,
		row.OwnerID,
	)
	applied, iter, err := db.session.MapExecuteBatchCAS(batch, make(map[string]interface{}))
	if iter != nil {
		_ = iter.Close()
	}
	if err != nil {
		return false, err
	}
	return applied, nil
}

// SelectSemaphoreTokenByID reads a slot's forward (token) row by token id.
func (db *CDB) SelectSemaphoreTokenByID(ctx context.Context, domainID, semaphoreName string, bucket, tokenID int) (*nosqlplugin.SemaphoreTokenRow, error) {
	row := &nosqlplugin.SemaphoreTokenRow{}
	query := db.session.Query(templateSelectSemaphoreTokenByIDQuery,
		domainID, semaphoreName, bucket, rowTypeSemaphoreToken, tokenID,
	).WithContext(ctx)
	if err := scanSemaphoreTokenRow(query, row); err != nil {
		return nil, err
	}
	return row, nil
}

// SelectSemaphoreTokenByOwner reads a hold's reverse (owner) row by owner id.
func (db *CDB) SelectSemaphoreTokenByOwner(ctx context.Context, domainID, semaphoreName string, bucket int, ownerID string) (*nosqlplugin.SemaphoreTokenRow, error) {
	row := &nosqlplugin.SemaphoreTokenRow{}
	query := db.session.Query(templateSelectSemaphoreTokenByOwnerQuery,
		domainID, semaphoreName, bucket, rowTypeSemaphoreOwner, emptyTokenID, ownerID,
	).WithContext(ctx)
	if err := scanSemaphoreTokenRow(query, row); err != nil {
		return nil, err
	}
	return row, nil
}

// SelectSemaphoreTokensByBucket scans a bucket partition (both row kinds), paginated.
func (db *CDB) SelectSemaphoreTokensByBucket(ctx context.Context, filter *nosqlplugin.SemaphoreTokenFilter) ([]*nosqlplugin.SemaphoreTokenRow, []byte, error) {
	query := db.session.Query(templateSelectSemaphoreTokensByBucketQuery,
		filter.DomainID, filter.SemaphoreName, filter.Bucket,
	).WithContext(ctx)

	if filter.PageSize > 0 {
		query = query.PageSize(filter.PageSize)
	}
	if len(filter.NextPageToken) > 0 {
		query = query.PageState(filter.NextPageToken)
	}

	iter := query.Iter()
	if iter == nil {
		return nil, nil, &types.InternalServiceError{
			Message: "SelectSemaphoreTokensByBucket operation failed. Not able to create query iterator.",
		}
	}

	var rows []*nosqlplugin.SemaphoreTokenRow
	var rowType int
	row := &nosqlplugin.SemaphoreTokenRow{}
	for iter.Scan(
		&row.DomainID,
		&row.SemaphoreName,
		&row.Bucket,
		&rowType,
		&row.TokenID,
		&row.OwnerID,
		&row.Holder,
		&row.HeldToken,
		&row.UpdatedTime,
	) {
		normalizeSemaphoreTokenRow(row)
		rows = append(rows, row)
		row = &nosqlplugin.SemaphoreTokenRow{}

		if filter.PageSize > 0 && len(rows) >= filter.PageSize {
			break
		}
	}

	nextPageToken := iter.PageState()
	if err := iter.Close(); err != nil {
		return nil, nil, err
	}

	return rows, nextPageToken, nil
}

// scanSemaphoreTokenRow scans a single-row read (forward or reverse) into row and
// normalizes the sentinel columns to zero values.
func scanSemaphoreTokenRow(query gocql.Query, row *nosqlplugin.SemaphoreTokenRow) error {
	if err := query.Scan(
		&row.DomainID,
		&row.SemaphoreName,
		&row.Bucket,
		&row.TokenID,
		&row.OwnerID,
		&row.Holder,
		&row.HeldToken,
		&row.UpdatedTime,
	); err != nil {
		return err
	}
	normalizeSemaphoreTokenRow(row)
	return nil
}

// normalizeSemaphoreTokenRow maps the plugin's internal sentinels back to zero
// values so they never leak past this package: an unheld/absent holder or
// owner_id becomes "", and a not-applicable token id / held token becomes 0.
func normalizeSemaphoreTokenRow(row *nosqlplugin.SemaphoreTokenRow) {
	if row.OwnerID == ownerNoneSentinel {
		row.OwnerID = ""
	}
	if row.Holder == freeSentinel || row.Holder == ownerNoneSentinel {
		row.Holder = ""
	}
	if row.TokenID == emptyTokenID {
		row.TokenID = 0
	}
	if row.HeldToken == emptyHeldToken {
		row.HeldToken = 0
	}
}
