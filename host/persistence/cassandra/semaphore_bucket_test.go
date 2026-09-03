package cassandra

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/matching/semaphore"
)

const semaphoreBucketTestTimeout = 30 * time.Second

// semaphoreBucketFixture is one seeded bucket plus the manager behind it. Each test gets
// its own domain and semaphore name, so they share a keyspace without colliding.
type semaphoreBucketFixture struct {
	manager persistence.SemaphoreTokenManager
	id      semaphore.Identifier
}

func newSemaphoreBucketFixture(ctx context.Context, t *testing.T, slots int) *semaphoreBucketFixture {
	t.Helper()
	base := CassandraTestBase(t)
	t.Cleanup(base.TearDownWorkflowStore)

	manager, err := base.PersistenceFactory.NewSemaphoreTokenManager()
	require.NoError(t, err)
	t.Cleanup(manager.Close)

	id, err := semaphore.NewIdentifier(uuid.NewString(), "sem-"+uuid.NewString(), 0)
	require.NoError(t, err)

	tokens := make([]int, 0, slots)
	for i := 1; i <= slots; i++ {
		tokens = append(tokens, i)
	}
	require.NoError(t, manager.SeedSemaphoreTokens(ctx, &persistence.SeedSemaphoreTokensRequest{
		DomainID:      id.DomainID,
		SemaphoreName: id.SemaphoreName,
		Bucket:        id.Bucket,
		TokenIDs:      tokens,
	}))

	return &semaphoreBucketFixture{manager: manager, id: id}
}

func (f *semaphoreBucketFixture) startBucket(ctx context.Context, t *testing.T) *semaphore.Bucket {
	t.Helper()
	b := semaphore.NewBucket(f.id, f.manager, testlogger.New(t))
	require.NoError(t, b.Start(ctx))
	t.Cleanup(b.Stop)
	return b
}

// grantBypassingBucket writes a grant that the bucket's in-memory indexes never see. Its
// purpose is to produce a stale cache, so these tests can check that the conditional write,
// not the cache, is what decides a grant. Nothing in the Bucket API can produce one.
func (f *semaphoreBucketFixture) grantBypassingBucket(ctx context.Context, t *testing.T, tokenID int, ownerID string) {
	t.Helper()
	resp, err := f.manager.GrantSemaphoreToken(ctx, &persistence.GrantSemaphoreTokenRequest{
		DomainID:      f.id.DomainID,
		SemaphoreName: f.id.SemaphoreName,
		Bucket:        f.id.Bucket,
		TokenID:       tokenID,
		OwnerID:       ownerID,
	})
	require.NoError(t, err)
	require.Equal(t, persistence.SemaphoreGrantApplied, resp.Outcome)
}

// assertHeldInDB checks both index directions agree that ownerID holds tokenID.
func (f *semaphoreBucketFixture) assertHeldInDB(ctx context.Context, t *testing.T, tokenID int, ownerID string) {
	t.Helper()
	byToken, err := f.manager.GetSemaphoreOwnershipByToken(ctx, &persistence.GetSemaphoreOwnershipByTokenRequest{
		DomainID:      f.id.DomainID,
		SemaphoreName: f.id.SemaphoreName,
		Bucket:        f.id.Bucket,
		TokenID:       tokenID,
	})
	require.NoError(t, err)
	assert.Equal(t, ownerID, byToken.Ownership.Holder, "forward row must name the owner")

	byOwner, err := f.manager.GetSemaphoreOwnershipByOwner(ctx, &persistence.GetSemaphoreOwnershipByOwnerRequest{
		DomainID:      f.id.DomainID,
		SemaphoreName: f.id.SemaphoreName,
		Bucket:        f.id.Bucket,
		OwnerID:       ownerID,
	})
	require.NoError(t, err)
	assert.Equal(t, tokenID, byOwner.Ownership.HeldToken, "reverse row must name the token")
}

// TestCassandraSemaphoreBucketGrant fills a bucket one grant at a time and checks that
// what the bucket believes and what the table holds stay in step.
func TestCassandraSemaphoreBucketGrant(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), semaphoreBucketTestTimeout)
	defer cancel()

	const slots = 5
	f := newSemaphoreBucketFixture(ctx, t, slots)
	bucket := f.startBucket(ctx, t)

	granted := make(map[int]string, slots)
	for i := 0; i < slots; i++ {
		owner := "owner-" + uuid.NewString()
		got, err := bucket.Acquire(ctx, owner)
		require.NoError(t, err)
		require.Equal(t, semaphore.AcquireOutcomeAcquired, got.Outcome)
		require.NotContains(t, granted, got.TokenID, "a slot was handed out twice")
		granted[got.TokenID] = owner
		f.assertHeldInDB(ctx, t, got.TokenID, owner)
	}
	assert.Len(t, granted, slots, "every seeded slot must be grantable exactly once")

	// The bucket is full, so the next acquire has nothing to give.
	full, err := bucket.Acquire(ctx, "owner-"+uuid.NewString())
	require.NoError(t, err)
	assert.Equal(t, semaphore.AcquireOutcomeNoSlot, full.Outcome)

	// A retry returns the token the owner already has and claims nothing new.
	for tokenID, owner := range granted {
		repeat, err := bucket.Acquire(ctx, owner)
		require.NoError(t, err)
		assert.Equal(t, semaphore.AcquireOutcomeAlreadyHeld, repeat.Outcome)
		assert.Equal(t, tokenID, repeat.TokenID)
	}

	// A second owner loading the same partition must see the same picture, which is what
	// a bucket handoff does.
	reloaded := f.startBucket(ctx, t)
	after, err := reloaded.Acquire(ctx, "owner-"+uuid.NewString())
	require.NoError(t, err)
	assert.Equal(t, semaphore.AcquireOutcomeNoSlot, after.Outcome,
		"a fresh load must find the bucket full")
	for tokenID, owner := range granted {
		repeat, err := reloaded.Acquire(ctx, owner)
		require.NoError(t, err)
		assert.Equal(t, semaphore.AcquireOutcomeAlreadyHeld, repeat.Outcome,
			"a freshly loaded reverse index must know the existing holds")
		assert.Equal(t, tokenID, repeat.TokenID)
	}
}

// TestCassandraSemaphoreBucketStaleFreeSet grants against a free-set that is out of date.
// Two of the three slots are claimed after the bucket loads, so it still offers all three,
// and only the third can actually be granted. Slots are drawn at random, so the number of
// wrong draws varies; each one costs a conditional write that fails and drops that id.
func TestCassandraSemaphoreBucketStaleFreeSet(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), semaphoreBucketTestTimeout)
	defer cancel()

	f := newSemaphoreBucketFixture(ctx, t, 3)
	bucket := f.startBucket(ctx, t)

	// After the load, so the bucket's free-set keeps listing both.
	f.grantBypassingBucket(ctx, t, 1, "owner-x")
	f.grantBypassingBucket(ctx, t, 2, "owner-y")

	owner := "owner-" + uuid.NewString()
	got, err := bucket.Acquire(ctx, owner)
	require.NoError(t, err)
	assert.Equal(t, semaphore.AcquireOutcomeAcquired, got.Outcome)
	assert.Equal(t, 3, got.TokenID, "the only unclaimed slot is the one that can be granted")
	f.assertHeldInDB(ctx, t, 3, owner)
}

// TestCassandraSemaphoreBucketAlreadyHeldBackstop checks that an owner cannot take a second
// slot when the bucket's reverse index has no record of the first -- what every bucket looks
// like just after a handoff. Nothing on this host refuses the grant, so the owner-row guard
// in the conditional write is the only thing that does.
func TestCassandraSemaphoreBucketAlreadyHeldBackstop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), semaphoreBucketTestTimeout)
	defer cancel()

	f := newSemaphoreBucketFixture(ctx, t, 3)
	bucket := f.startBucket(ctx, t)

	// Grant slot 2 behind the bucket's back, so its indexes never learn of the hold: they
	// still list 2 as free and have no entry for this owner.
	owner := "owner-" + uuid.NewString()
	f.grantBypassingBucket(ctx, t, 2, owner)

	got, err := bucket.Acquire(ctx, owner)
	require.NoError(t, err)
	assert.Equal(t, semaphore.AcquireOutcomeAlreadyHeld, got.Outcome,
		"a cold reverse index must not let one owner take a second slot")
	assert.Equal(t, 2, got.TokenID, "the owner keeps the token it already had")
	f.assertHeldInDB(ctx, t, 2, owner)

	// The already-held branch puts back the id it drew, unlike slot-taken which keeps it out,
	// so the free-set is exactly {1, 3} whichever id the call above happened to draw. Keeping
	// it out instead would leave one slot and the second acquire would find none. A draw of 2
	// hides that, so the unit test is the strict check.
	granted := map[int]bool{}
	for range 2 {
		other := "owner-" + uuid.NewString()
		next, err := bucket.Acquire(ctx, other)
		require.NoError(t, err)
		require.Equal(t, semaphore.AcquireOutcomeAcquired, next.Outcome)
		f.assertHeldInDB(ctx, t, next.TokenID, other)
		granted[next.TokenID] = true
	}
	assert.Equal(t, map[int]bool{1: true, 3: true}, granted, "both untouched slots must come out")

	// Every slot is held now, so no token comes back. Which no-token answer it is depends on
	// the draws above: if one drew slot 2 the write dropped it and the free-set is empty, so
	// NoSlot; if not, this call draws 2 and the write proving it taken makes it Contended.
	// Both are correct, so assert only that no token came out.
	full, err := bucket.Acquire(ctx, "owner-"+uuid.NewString())
	require.NoError(t, err)
	assert.Contains(t,
		[]semaphore.AcquireOutcome{semaphore.AcquireOutcomeNoSlot, semaphore.AcquireOutcomeContended},
		full.Outcome, "a full bucket hands out no token")
	assert.Zero(t, full.TokenID)
}
