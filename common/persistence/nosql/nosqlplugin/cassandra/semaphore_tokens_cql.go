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

	// Grant: conditional in-place UPDATE of the token row (claim only if free) ...
	templateGrantSemaphoreTokenUpdateQuery = `UPDATE semaphore_tokens ` +
		`SET holder = ?, updated_time = ? ` +
		`WHERE domain_id = ? AND semaphore_name = ? AND bucket = ? AND type = ? AND token_id = ? AND owner_id = ? ` +
		`IF holder = ?`

	// ... plus the matching owner (reverse-index) row INSERT, in the same batch.
	// IF NOT EXISTS makes this a second condition on the batch: it enforces
	// one-token-per-hold (owner_id), so a same-owner_id double-grant cannot
	// overwrite an existing hold. When it fails, the CAS result carries the owner
	// row's current held_token, which we surface for reuse.
	templateGrantSemaphoreOwnerInsertQuery = `INSERT INTO semaphore_tokens (` +
		`domain_id, semaphore_name, bucket, type, token_id, owner_id, holder, held_token, updated_time) ` +
		`VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?) IF NOT EXISTS`

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
