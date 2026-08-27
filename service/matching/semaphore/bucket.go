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

package semaphore

import (
	"context"
	"fmt"
	"math/rand"
	"sync"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
)

// scanPageSize bounds one page of the ownership rebuild. A bucket holds at most
// bucket_size token rows plus one owner row per hold, so this is usually the whole
// partition in a single page.
const scanPageSize = 1000

// Bucket is the counting authority for one semaphore bucket: it tracks which of the
// bucket's slots are open and which owner holds which slot.
//
// Its in-memory state is only a cache of the bucket's partition, indexed once per
// durable row kind, and the conditional write in persistence is the sole authority. That
// is what makes drift tolerable in both directions. An over-optimistic entry (the
// free-set names a slot someone else holds) is caught by the write, which does not
// apply. An over-pessimistic one (a free slot missing from the free-set) only turns
// grants away that could have succeeded, and the next rebuild puts the slot back.
// Neither direction can hand the same slot to two owners or lose a grant.
//
// Granting and releasing are not implemented here yet, so today the state is built once
// by Start and then changes only through the mutators below.
type Bucket struct {
	id      Identifier
	manager persistence.SemaphoreTokenManager
	logger  log.Logger

	// mu guards the two indexes below. It is never held across a persistence call: two
	// concurrent acquires for the same owner are meant to race down to the conditional
	// write, where the loser is told which token it already holds.
	mu sync.Mutex
	// freeList holds the ids of token rows with no holder and freeIndex maps an id back to
	// its position in freeList. The pair buys an O(1) uniform-random draw (index into the
	// slice) alongside O(1) removal of one specific id (swap with the tail, then shrink).
	// A plain map gives neither: picking the k-th key is a linear walk, and Go's map order
	// is not a uniform distribution.
	freeList  []int
	freeIndex map[int]int
	// held is the owner_id -> token_id reverse index, mirroring the partition's owner rows.
	held map[string]int
}

// NewBucket builds the owner of one bucket. The caller must call Start before using it,
// and must discard the Bucket if Start returns an error.
func NewBucket(
	id Identifier,
	manager persistence.SemaphoreTokenManager,
	logger log.Logger,
) *Bucket {
	return &Bucket{
		id:        id,
		manager:   manager,
		logger:    logger.WithTags(tag.Dynamic("semaphore-bucket", id.String())),
		freeIndex: make(map[int]int),
		held:      make(map[string]int),
	}
}

// Start takes ownership of the bucket by scanning its partition and building the
// free-set and the reverse index from what is actually stored. It must be called exactly
// once, and the Bucket must be discarded if it returns an error.
func (b *Bucket) Start(ctx context.Context) error {
	freeList, freeIndex, held, err := b.scan(ctx)
	if err != nil {
		return fmt.Errorf("rebuild semaphore bucket %v: %w", b.id, err)
	}

	b.mu.Lock()
	b.freeList, b.freeIndex, b.held = freeList, freeIndex, held
	b.mu.Unlock()

	b.logger.Info("Semaphore bucket owner started",
		tag.LifeCycleStarted,
		tag.Dynamic("free-slots", len(freeList)),
		tag.Dynamic("held-slots", len(held)),
	)
	return nil
}

// scan rebuilds both indexes from the bucket's partition, following NextPageToken to the
// end. It builds into locals so a failed scan leaves the live state untouched.
func (b *Bucket) scan(ctx context.Context) ([]int, map[int]int, map[string]int, error) {
	free := make(map[int]struct{})
	held := make(map[string]int)
	var skipped int

	var pageToken []byte
	for {
		resp, err := b.manager.ScanSemaphoreBucket(ctx, &persistence.ScanSemaphoreBucketRequest{
			DomainID:      b.id.DomainID,
			SemaphoreName: b.id.SemaphoreName,
			Bucket:        b.id.Bucket,
			PageSize:      scanPageSize,
			NextPageToken: pageToken,
		})
		if err != nil {
			return nil, nil, nil, err
		}

		for _, row := range resp.Ownerships {
			switch row.Kind {
			case persistence.SemaphoreRowKindToken:
				// Holder is empty exactly when the slot is unheld.
				if row.Holder == "" && row.TokenID > 0 {
					free[row.TokenID] = struct{}{}
				}
			case persistence.SemaphoreRowKindOwner:
				if row.OwnerID != "" && row.HeldToken > 0 {
					held[row.OwnerID] = row.HeldToken
				}
			default:
				// A row kind this version does not know, which means a newer version wrote
				// it. Skipping is safe in both directions: a dropped token row costs one
				// slot until the next rebuild, and a dropped owner row leaves the
				// conditional write to catch the duplicate. Counted rather than logged per
				// row, because a version mismatch makes every row in the bucket match here.
				skipped++
			}
		}

		pageToken = resp.NextPageToken
		if len(pageToken) == 0 {
			break
		}
	}

	if skipped > 0 {
		b.logger.Warn("Skipped semaphore rows of unknown kind during rebuild",
			tag.Dynamic("skipped-rows", skipped))
	}

	// A slot claimed by an owner row is not free, whatever its token row said. The two can
	// disagree because a scan is not a snapshot: a grant landing mid-scan can be missed on
	// the token row and seen on the owner row. Reconciling them here just keeps the two
	// indexes agreeing on the pages we did read; repairing the table itself is a separate
	// job. Doing it after the loop rather than inside keeps it independent of the order
	// the two row kinds come back in.
	for _, tokenID := range held {
		delete(free, tokenID)
	}

	freeList := make([]int, 0, len(free))
	freeIndex := make(map[int]int, len(free))
	for tokenID := range free {
		freeIndex[tokenID] = len(freeList)
		freeList = append(freeList, tokenID)
	}
	return freeList, freeIndex, held, nil
}

// reserve draws a uniform-random free id and takes it out of the free-set.
func (b *Bucket) reserve() (int, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.freeList) == 0 {
		return 0, false
	}
	tokenID := b.freeList[rand.Intn(len(b.freeList))]
	b.removeFreeLocked(tokenID)
	return tokenID, true
}

// unreserve puts a reserved id back, for the case where the write proved the slot is
// still free but this owner cannot use it.
func (b *Bucket) unreserve(tokenID int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.addFreeLocked(tokenID)
}

func (b *Bucket) recordHold(ownerID string, tokenID int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.held[ownerID] = tokenID
	// A held slot is never in the free-set. It normally is not here anyway, since the
	// grant reserved it; the already-held case reports a token this host never drew.
	b.removeFreeLocked(tokenID)
}

// forgetHold drops a stale reverse-index entry, returning its slot to the free-set only
// when the caller has confirmed the slot is unheld.
func (b *Bucket) forgetHold(ownerID string, tokenID int, stillFree bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	// Only drop the entry if it still names the token the caller checked; a concurrent
	// grant may have replaced it in the meantime.
	if current, ok := b.held[ownerID]; ok && current == tokenID {
		delete(b.held, ownerID)
	}
	if stillFree {
		b.addFreeLocked(tokenID)
	}
}

func (b *Bucket) addFreeLocked(tokenID int) {
	if _, ok := b.freeIndex[tokenID]; ok {
		return
	}
	b.freeIndex[tokenID] = len(b.freeList)
	b.freeList = append(b.freeList, tokenID)
}

// removeFreeLocked takes one id out in constant time by moving the tail element into its
// slot. Order in freeList carries no meaning, since picks are random.
func (b *Bucket) removeFreeLocked(tokenID int) {
	i, ok := b.freeIndex[tokenID]
	if !ok {
		return
	}
	last := len(b.freeList) - 1
	moved := b.freeList[last]
	b.freeList[i] = moved
	b.freeIndex[moved] = i
	b.freeList = b.freeList[:last]
	// When the removed id was itself the tail, moved == tokenID and the line above just
	// re-added the entry being removed, so this delete has to come last.
	delete(b.freeIndex, tokenID)
}

// freeCount reports how many slots the bucket believes are open. It is a hint, not the
// truth, and exists for tests and metrics.
func (b *Bucket) freeCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.freeList)
}
