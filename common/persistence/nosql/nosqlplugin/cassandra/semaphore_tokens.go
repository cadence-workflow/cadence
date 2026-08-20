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

// Sentinels stored in columns that do not apply to a given row kind.
//
// The int sentinels are negative so they can never collide with a real slot id (>= 1).
//
// The text sentinels are PROVISIONAL. freeSentinel is the only value ever LWT-compared,
// so its final literal must be one the owner_id encoding can never produce;
// ownerNoneSentinel's value does not matter (the row type already separates token and
// owner rows).
// TODO: finalize freeSentinel/ownerNoneSentinel with the owner_id encoding.
const (
	emptyTokenID   = -1 // token_id on owner rows
	emptyHeldToken = -1 // held_token on token rows

	ownerNoneSentinel = "__NONE__" // owner_id on token rows; holder on owner rows
	freeSentinel      = "__FREE__" // holder of an unheld token row
)

// InsertSemaphoreTokens seeds a bucket with free token rows for the given
// TokenIDs, using a single conditional (LWT) batch of INSERT ... IF NOT EXISTS.
//
// Contract: callers must supply a bucket's FULL id set. A bucket's id range is
// fixed at semaphore creation and never grows (to change size/bucket_size you
// create a new semaphore name), so seeding is only ever one of two cases:
//   - fresh bucket: no rows exist, so all rows are inserted;
//   - re-seed of the same set: every row exists, so the batch is a deliberate
//     no-op that never clobbers an already-held slot.
// The applied flag is intentionally ignored: "not applied" is the expected
// outcome of a same-set re-seed, not an error.
//
// Growing a bucket is unsupported by design, and this relies on it: a conditional
// batch is all-or-nothing, so a partial superset (some ids already present) would
// have its existing rows' guards reject the WHOLE batch, silently dropping the
// brand-new ids.
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

// GrantSemaphoreToken claims row.TokenID for row.OwnerID with one atomic batch of
// two guarded writes: set the token row's holder to the owner only if it is free
// (IF holder = FREE), and insert the owner row only if it is absent (IF NOT EXISTS).
// The batch is all-or-nothing, so the grant applies only if both guards pass.
//
// The IF NOT EXISTS guard enforces one-token-per-hold: a same-owner_id double-grant
// (racing hosts during a handoff, or a caller bug) cannot overwrite an existing hold.
//
// Returns Applied == false (not an error) when the grant did not apply:
//   - AlreadyHeldToken > 0: this owner already holds that token (reuse it);
//   - AlreadyHeldToken == 0: the slot is taken by someone else (retry another).
func (db *CDB) GrantSemaphoreToken(ctx context.Context, row *nosqlplugin.SemaphoreTokenRow) (nosqlplugin.SemaphoreGrantResult, error) {
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
		return nosqlplugin.SemaphoreGrantResult{Applied: true}, nil
	}
	// Not applied: walk the returned rows (first in `previous`, the rest via the
	// iterator) to find the owner row and read the token it already holds.
	heldToken := parseAlreadyHeldTokenFromCAS(previous, iter)
	if iter != nil {
		_ = iter.Close()
	}
	return nosqlplugin.SemaphoreGrantResult{Applied: false, AlreadyHeldToken: heldToken}, nil
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

// parseHeldTokenIfOwnerRow is a helper for parseAlreadyHeldTokenFromCAS: it
// returns the held_token of the given CAS row when it is an owner (reverse-index)
// row, normalized to 0 when absent; ok is false for any other row kind.
func parseHeldTokenIfOwnerRow(row map[string]interface{}) (int, bool) {
	rowType, ok := row["type"].(int)
	if !ok || rowType != rowTypeSemaphoreOwner {
		return 0, false
	}
	heldToken, _ := row["held_token"].(int)
	if heldToken == emptyHeldToken {
		heldToken = 0
	}
	return heldToken, true
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
