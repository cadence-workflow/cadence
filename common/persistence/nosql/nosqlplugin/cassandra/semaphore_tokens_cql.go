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

const (
	templateSeedSemaphoreTokenQuery = `INSERT INTO semaphore_tokens (` +
		`domain_id, semaphore_name, bucket, type, token_id, owner_id, holder, held_token, updated_time) ` +
		`VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?) IF NOT EXISTS`

	// Grant runs the next two statements as one atomic batch.
	// (1) Claim the token row in place, only if it is currently free.
	templateGrantSemaphoreTokenUpdateQuery = `UPDATE semaphore_tokens ` +
		`SET holder = ?, updated_time = ? ` +
		`WHERE domain_id = ? AND semaphore_name = ? AND bucket = ? AND type = ? AND token_id = ? AND owner_id = ? ` +
		`IF holder = ?`

	// (2) Insert the matching owner (reverse-index) row, only if absent.
	// IF NOT EXISTS enforces one-token-per-hold: a same-owner_id double-grant cannot
	// overwrite an existing hold. On failure the CAS result carries the owner row's
	// held_token, which we surface for reuse.
	templateGrantSemaphoreOwnerInsertQuery = `INSERT INTO semaphore_tokens (` +
		`domain_id, semaphore_name, bucket, type, token_id, owner_id, holder, held_token, updated_time) ` +
		`VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?) IF NOT EXISTS`

	// Release runs the next two statements as one atomic batch.
	// (1) Clear the token row in place, only if still held by this owner.
	templateReleaseSemaphoreTokenUpdateQuery = `UPDATE semaphore_tokens ` +
		`SET holder = ?, updated_time = ? ` +
		`WHERE domain_id = ? AND semaphore_name = ? AND bucket = ? AND type = ? AND token_id = ? AND owner_id = ? ` +
		`IF holder = ?`

	// (2) Delete the matching owner row.
	templateReleaseSemaphoreOwnerDeleteQuery = `DELETE FROM semaphore_tokens ` +
		`WHERE domain_id = ? AND semaphore_name = ? AND bucket = ? AND type = ? AND token_id = ? AND owner_id = ?`

	// Forward read: owner_id, the trailing clustering column, is omitted. Every write binds
	// ownerNoneSentinel, so exactly one token row exists per (type, token_id).
	templateSelectSemaphoreOwnershipByTokenQuery = `SELECT ` +
		`domain_id, semaphore_name, bucket, type, token_id, owner_id, holder, held_token, updated_time ` +
		`FROM semaphore_tokens ` +
		`WHERE domain_id = ? AND semaphore_name = ? AND bucket = ? AND type = ? AND token_id = ?`

	// Reverse read
	templateSelectSemaphoreOwnershipByOwnerQuery = `SELECT ` +
		`domain_id, semaphore_name, bucket, type, token_id, owner_id, holder, held_token, updated_time ` +
		`FROM semaphore_tokens ` +
		`WHERE domain_id = ? AND semaphore_name = ? AND bucket = ? AND type = ? AND token_id = ? AND owner_id = ?`

	// Full-partition read: no type predicate, so this deliberately returns BOTH row
	// types - token rows first, then owner rows, per the type clustering order.
	// It rebuilds a bucket's forward and reverse indexes in one pass, for cache
	// warm-up on host start and on ownership transfer
	templateSelectSemaphoreOwnershipsByBucketQuery = `SELECT ` +
		`domain_id, semaphore_name, bucket, type, token_id, owner_id, holder, held_token, updated_time ` +
		`FROM semaphore_tokens ` +
		`WHERE domain_id = ? AND semaphore_name = ? AND bucket = ?`
)
