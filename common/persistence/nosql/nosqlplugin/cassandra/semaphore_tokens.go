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

	gogocql "github.com/gocql/gocql"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
	"github.com/uber/cadence/common/types"
)

// Placeholders for key columns that do not apply to a given row type. Non-key columns
// that do not apply are bound to gogocql.UnsetValue instead.
//
// The text values are PROVISIONAL: only freeSentinel is LWT-compared, so only its literal
// must be one the owner_id encoding can never produce.
// TODO: finalize the text sentinels with the owner_id encoding.
const (
	emptyTokenID = -1 // token_id on owner rows (key); negative, never a real slot id

	ownerNoneSentinel = "__NONE__" // owner_id on token rows (key)
	freeSentinel      = "__FREE__" // holder of an unheld token row (LWT-compared)
)

// InsertSemaphoreTokens seeds a bucket with free token rows for the given TokenIDs
// via one conditional (LWT) batch of INSERT ... IF NOT EXISTS.
//
// Callers must pass the bucket's FULL id set, which is fixed at semaphore creation
// and never grows (to resize, create a new semaphore name). So this is only ever a
// fresh insert (no rows exist, all applied) or a re-seed of the same set (every
// IF NOT EXISTS fails, a deliberate no-op that never clobbers a held slot); the
// applied flag is therefore ignored.
//
// Growing a bucket is unsupported: the batch is all-or-nothing, so a partial superset
// would have its existing rows' guards reject the whole batch, silently dropping the
// new ids.
func (db *CDB) InsertSemaphoreTokens(ctx context.Context, rows []*nosqlplugin.SemaphoreOwnershipRow) error {
	if len(rows) == 0 {
		return nil
	}
	batch := db.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	for _, row := range rows {
		batch.Query(templateSeedSemaphoreTokenQuery,
			row.DomainID,
			row.SemaphoreName,
			row.Bucket,
			persistence.SemaphoreRowTypeToken, // forward "token" row
			row.TokenID,
			ownerNoneSentinel,  // owner_id key = __NONE__
			freeSentinel,       // holder = __FREE__, the slot is unheld
			gogocql.UnsetValue, // held_token does not apply to a token row
			row.UpdatedTime,
		)
	}
	_, iter, err := db.session.MapExecuteBatchCAS(batch, make(map[string]interface{}))
	if iter != nil {
		_ = iter.Close()
	}
	return err
}

// GrantSemaphoreToken claims row.TokenID for row.OwnerID with one atomic batch of
// two guarded writes: set the token row's holder to the owner only if it is free
// (IF holder = FREE), and insert the owner row only if it is absent (IF NOT EXISTS).
// The batch is all-or-nothing, so the grant applies only if both guards pass.
//
// The IF NOT EXISTS guard enforces one-token-per-hold: a same-owner_id double-grant
// (racing hosts during a handoff, or a caller bug) cannot overwrite an existing hold.
//
// A grant that does not apply is not an error; the returned Outcome says why.
func (db *CDB) GrantSemaphoreToken(ctx context.Context, row *nosqlplugin.SemaphoreOwnershipRow) (nosqlplugin.SemaphoreGrantResult, error) {
	batch := db.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	batch.Query(templateGrantSemaphoreTokenUpdateQuery,
		row.OwnerID,     // SET holder = owner_id
		row.UpdatedTime, // SET updated_time
		row.DomainID,
		row.SemaphoreName,
		row.Bucket,
		persistence.SemaphoreRowTypeToken,
		row.TokenID,
		ownerNoneSentinel, // token row's owner_id key
		freeSentinel,      // IF holder = FREE
	)
	batch.Query(templateGrantSemaphoreOwnerInsertQuery,
		row.DomainID,
		row.SemaphoreName,
		row.Bucket,
		persistence.SemaphoreRowTypeOwner,
		emptyTokenID,
		row.OwnerID,
		gogocql.UnsetValue, // holder does not apply to an owner row
		row.TokenID,        // held_token
		row.UpdatedTime,
	)
	previous := make(map[string]interface{})
	applied, iter, err := db.session.MapExecuteBatchCAS(batch, previous)
	if err != nil {
		if iter != nil {
			_ = iter.Close()
		}
		return nosqlplugin.SemaphoreGrantResult{}, err
	}
	if applied {
		if iter != nil {
			_ = iter.Close()
		}
		return nosqlplugin.SemaphoreGrantResult{Outcome: persistence.SemaphoreGrantApplied}, nil
	}
	// Not applied: walk the returned rows (first in `previous`, the rest via the
	// iterator) to find the owner row and read the token it already holds.
	heldToken := parseAlreadyHeldTokenFromCAS(previous, iter)
	if iter != nil {
		_ = iter.Close()
	}
	if heldToken > 0 {
		return nosqlplugin.SemaphoreGrantResult{
			Outcome:   persistence.SemaphoreGrantAlreadyHeld,
			HeldToken: heldToken,
		}, nil
	}
	return nosqlplugin.SemaphoreGrantResult{Outcome: persistence.SemaphoreGrantSlotTaken}, nil
}

// parseAlreadyHeldTokenFromCAS inspects the CAS result of a not-applied grant
// batch and returns the token this owner already holds, or 0 if the only conflict
// was the slot already being taken. MapExecuteBatchCAS returns the first
// conflicting row in `previous` and the remaining rows through the iterator;
// either may be the owner row, so we check both.
func parseAlreadyHeldTokenFromCAS(previous map[string]interface{}, iter gocql.Iter) int {
	if heldToken, ok := parseHeldTokenIfOwnerRow(previous); ok {
		return heldToken
	}
	if iter == nil {
		return 0
	}
	row := make(map[string]interface{})
	for iter.MapScan(row) {
		if heldToken, ok := parseHeldTokenIfOwnerRow(row); ok {
			return heldToken
		}
		row = make(map[string]interface{})
	}
	return 0
}

// parseHeldTokenIfOwnerRow returns the held_token of an owner (reverse-index) CAS row.
// ok is false for any other row type, and for an owner row whose held_token is absent or
// not positive: that owner holds no valid token.
func parseHeldTokenIfOwnerRow(row map[string]interface{}) (int, bool) {
	rowType, ok := row["type"].(int)
	if !ok || rowType != int(persistence.SemaphoreRowTypeOwner) {
		return 0, false
	}
	heldToken, ok := row["held_token"].(int)
	if !ok || heldToken <= 0 {
		return 0, false
	}
	return heldToken, true
}

// ReleaseSemaphoreToken frees row.TokenID via one atomic batch: the token row's
// holder is reset to free only if it is still held by row.OwnerID, and the
// matching owner row is deleted. Returns applied == false (not an error) for a
// best-effort no-op (something else already touched the slot).
func (db *CDB) ReleaseSemaphoreToken(ctx context.Context, row *nosqlplugin.SemaphoreOwnershipRow) (bool, error) {
	batch := db.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	batch.Query(templateReleaseSemaphoreTokenUpdateQuery,
		freeSentinel,    // SET holder = FREE
		row.UpdatedTime, // SET updated_time
		row.DomainID,
		row.SemaphoreName,
		row.Bucket,
		persistence.SemaphoreRowTypeToken,
		row.TokenID,
		ownerNoneSentinel,
		row.OwnerID, // IF holder = owner_id
	)
	batch.Query(templateReleaseSemaphoreOwnerDeleteQuery,
		row.DomainID,
		row.SemaphoreName,
		row.Bucket,
		persistence.SemaphoreRowTypeOwner,
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

// SelectSemaphoreOwnershipByToken reads a slot's forward (token) row by token id.
func (db *CDB) SelectSemaphoreOwnershipByToken(ctx context.Context, domainID, semaphoreName string, bucket, tokenID int) (*nosqlplugin.SemaphoreOwnershipRow, error) {
	row := &nosqlplugin.SemaphoreOwnershipRow{}
	query := db.session.Query(templateSelectSemaphoreOwnershipByTokenQuery,
		domainID, semaphoreName, bucket, persistence.SemaphoreRowTypeToken, tokenID,
	).WithContext(ctx)
	if err := scanSemaphoreOwnershipRow(query, row); err != nil {
		return nil, err
	}
	return row, nil
}

// SelectSemaphoreOwnershipByOwner reads a hold's reverse (owner) row by owner id.
func (db *CDB) SelectSemaphoreOwnershipByOwner(ctx context.Context, domainID, semaphoreName string, bucket int, ownerID string) (*nosqlplugin.SemaphoreOwnershipRow, error) {
	row := &nosqlplugin.SemaphoreOwnershipRow{}
	query := db.session.Query(templateSelectSemaphoreOwnershipByOwnerQuery,
		domainID, semaphoreName, bucket, persistence.SemaphoreRowTypeOwner, emptyTokenID, ownerID,
	).WithContext(ctx)
	if err := scanSemaphoreOwnershipRow(query, row); err != nil {
		return nil, err
	}
	return row, nil
}

// SelectSemaphoreOwnershipsByBucket scans a bucket partition (both row types), paginated.
func (db *CDB) SelectSemaphoreOwnershipsByBucket(ctx context.Context, filter *nosqlplugin.SemaphoreOwnershipFilter) ([]*nosqlplugin.SemaphoreOwnershipRow, []byte, error) {
	query := db.session.Query(templateSelectSemaphoreOwnershipsByBucketQuery,
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
			Message: "SelectSemaphoreOwnershipsByBucket operation failed. Not able to create query iterator.",
		}
	}

	var rows []*nosqlplugin.SemaphoreOwnershipRow
	row := &nosqlplugin.SemaphoreOwnershipRow{}
	for iter.Scan(
		&row.DomainID,
		&row.SemaphoreName,
		&row.Bucket,
		&row.RowType,
		&row.TokenID,
		&row.OwnerID,
		&row.Holder,
		&row.HeldToken,
		&row.UpdatedTime,
	) {
		normalizeSemaphoreOwnershipRow(row)
		rows = append(rows, row)
		row = &nosqlplugin.SemaphoreOwnershipRow{}

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

// scanSemaphoreOwnershipRow scans a single-row read (forward or reverse) into row and
// normalizes the sentinel columns to zero values.
func scanSemaphoreOwnershipRow(query gocql.Query, row *nosqlplugin.SemaphoreOwnershipRow) error {
	if err := query.Scan(
		&row.DomainID,
		&row.SemaphoreName,
		&row.Bucket,
		&row.RowType,
		&row.TokenID,
		&row.OwnerID,
		&row.Holder,
		&row.HeldToken,
		&row.UpdatedTime,
	); err != nil {
		return err
	}
	normalizeSemaphoreOwnershipRow(row)
	return nil
}

// normalizeSemaphoreOwnershipRow maps the plugin's internal sentinels back to zero
// values so they never leak past this package: an absent owner_id becomes "",
// an unheld holder becomes "", and a not-applicable token id becomes 0.
//
// Columns bound to gogocql.UnsetValue on write (held_token on token rows, holder
// on owner rows) already read back as the zero value, so they need no mapping.
func normalizeSemaphoreOwnershipRow(row *nosqlplugin.SemaphoreOwnershipRow) {
	if row.OwnerID == ownerNoneSentinel {
		row.OwnerID = ""
	}
	if row.Holder == freeSentinel {
		row.Holder = ""
	}
	if row.TokenID == emptyTokenID {
		row.TokenID = 0
	}
}
