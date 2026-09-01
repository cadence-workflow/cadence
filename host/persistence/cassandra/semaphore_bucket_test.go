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

// grantDirect claims a slot through persistence, behind the bucket's back. It stands in
// for the writes another host makes while a bucket is changing hands, which is the only
// way the bucket's in-memory state goes stale in production.
func (f *semaphoreBucketFixture) grantDirect(ctx context.Context, t *testing.T, tokenID int, ownerID string) {
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
		got, err := bucket.Grant(ctx, owner)
		require.NoError(t, err)
		require.Equal(t, semaphore.GrantOutcomeAcquired, got.Outcome)
		require.NotContains(t, granted, got.TokenID, "a slot was handed out twice")
		granted[got.TokenID] = owner
		f.assertHeldInDB(ctx, t, got.TokenID, owner)
	}
	assert.Len(t, granted, slots, "every seeded slot must be grantable exactly once")

	// The bucket is full, so the next acquire has nothing to give.
	full, err := bucket.Grant(ctx, "owner-"+uuid.NewString())
	require.NoError(t, err)
	assert.Equal(t, semaphore.GrantOutcomeNoSlot, full.Outcome)

	// A retry returns the token the owner already has and claims nothing new.
	for tokenID, owner := range granted {
		repeat, err := bucket.Grant(ctx, owner)
		require.NoError(t, err)
		assert.Equal(t, semaphore.GrantOutcomeAlreadyHeld, repeat.Outcome)
		assert.Equal(t, tokenID, repeat.TokenID)
	}

	// A fresh owner rebuilding from the partition must see the same picture, which is
	// what a bucket handoff does.
	rebuilt := f.startBucket(ctx, t)
	after, err := rebuilt.Grant(ctx, "owner-"+uuid.NewString())
	require.NoError(t, err)
	assert.Equal(t, semaphore.GrantOutcomeNoSlot, after.Outcome,
		"a rebuild must find the bucket full")
	for tokenID, owner := range granted {
		repeat, err := rebuilt.Grant(ctx, owner)
		require.NoError(t, err)
		assert.Equal(t, semaphore.GrantOutcomeAlreadyHeld, repeat.Outcome,
			"the rebuilt reverse index must know the existing holds")
		assert.Equal(t, tokenID, repeat.TokenID)
	}
}

// TestCassandraSemaphoreBucketStaleFreeSet drives the slot-taken retry: the bucket
// still lists slots that another writer has since claimed, so its first picks fail and it
// has to work its way onto the one slot that really is open.
func TestCassandraSemaphoreBucketStaleFreeSet(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), semaphoreBucketTestTimeout)
	defer cancel()

	f := newSemaphoreBucketFixture(ctx, t, 3)
	bucket := f.startBucket(ctx, t)

	// After the rebuild, so the bucket's free-set keeps listing both.
	f.grantDirect(ctx, t, 1, "owner-x")
	f.grantDirect(ctx, t, 2, "owner-y")

	owner := "owner-" + uuid.NewString()
	got, err := bucket.Grant(ctx, owner)
	require.NoError(t, err)
	assert.Equal(t, semaphore.GrantOutcomeAcquired, got.Outcome)
	assert.Equal(t, 3, got.TokenID, "the only unclaimed slot is the one that can be granted")
	f.assertHeldInDB(ctx, t, 3, owner)
}

// TestCassandraSemaphoreBucketAlreadyHeldBackstop drives the other failure of the
// conditional write. The bucket's reverse index never learns about a hold claimed behind
// its back, so the write is the only thing standing between one owner and a second slot.
func TestCassandraSemaphoreBucketAlreadyHeldBackstop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), semaphoreBucketTestTimeout)
	defer cancel()

	f := newSemaphoreBucketFixture(ctx, t, 3)
	bucket := f.startBucket(ctx, t)

	owner := "owner-" + uuid.NewString()
	f.grantDirect(ctx, t, 2, owner)

	got, err := bucket.Grant(ctx, owner)
	require.NoError(t, err)
	assert.Equal(t, semaphore.GrantOutcomeAlreadyHeld, got.Outcome,
		"a cold reverse index must not let one owner take a second slot")
	assert.Equal(t, 2, got.TokenID, "the owner keeps the token it already had")
	f.assertHeldInDB(ctx, t, 2, owner)

	// The two remaining slots must both still be grantable. This is what the asymmetric
	// un-reserve buys: had the already-held branch dropped the id it reserved instead of
	// returning it, one of these would be stranded until the next rebuild.
	for _, tokenID := range []int{1, 3} {
		other := "owner-" + uuid.NewString()
		next, err := bucket.Grant(ctx, other)
		require.NoError(t, err)
		require.Equal(t, semaphore.GrantOutcomeAcquired, next.Outcome,
			"slot %d should still be available", tokenID)
		f.assertHeldInDB(ctx, t, next.TokenID, other)
	}

	full, err := bucket.Grant(ctx, "owner-"+uuid.NewString())
	require.NoError(t, err)
	assert.Equal(t, semaphore.GrantOutcomeNoSlot, full.Outcome)
}
