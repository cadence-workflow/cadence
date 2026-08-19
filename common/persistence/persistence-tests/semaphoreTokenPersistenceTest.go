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

package persistencetests

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/persistence"
)

type (
	SemaphoreTokenPersistenceSuite struct {
		*TestBase
		*require.Assertions
	}
)

func (s *SemaphoreTokenPersistenceSuite) SetupSuite() {
	if testing.Verbose() {
		log.SetOutput(os.Stdout)
	}
}

func (s *SemaphoreTokenPersistenceSuite) SetupTest() {
	s.Assertions = require.New(s.T())
}

func (s *SemaphoreTokenPersistenceSuite) TearDownSuite() {
	s.TearDownWorkflowStore()
}

// TestGrantAndRelease seeds a bucket, then walks a slot through the grant/release
// lifecycle, verifying the conditional writes and both index directions.
func (s *SemaphoreTokenPersistenceSuite) TestGrantAndRelease() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	manager, err := s.PersistenceFactory.NewSemaphoreTokenManager()
	s.NoError(err)
	s.NotNil(manager)
	defer manager.Close()

	domainID := uuid.NewString()
	semaphoreName := "sem-" + uuid.NewString()
	bucket := 0
	tokenID := 1
	owner := "owner-" + uuid.NewString()

	// seed a single free slot
	s.NoError(manager.SeedSemaphoreTokens(ctx, &persistence.SeedSemaphoreTokensRequest{
		DomainID:      domainID,
		SemaphoreName: semaphoreName,
		Bucket:        bucket,
		TokenIDs:      []int{tokenID},
	}))

	// the slot starts free
	byID, err := manager.GetSemaphoreTokenByID(ctx, &persistence.GetSemaphoreTokenByIDRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket, TokenID: tokenID,
	})
	s.NoError(err)
	s.Equal("", byID.Token.Holder)

	// grant applies
	grantResp, err := manager.GrantSemaphoreToken(ctx, &persistence.GrantSemaphoreTokenRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket, TokenID: tokenID, OwnerID: owner,
	})
	s.NoError(err)
	s.True(grantResp.Applied)

	// re-granting the same slot does not apply
	grantAgain, err := manager.GrantSemaphoreToken(ctx, &persistence.GrantSemaphoreTokenRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket, TokenID: tokenID, OwnerID: "someone-else",
	})
	s.NoError(err)
	s.False(grantAgain.Applied)

	// forward read shows the holder
	byID, err = manager.GetSemaphoreTokenByID(ctx, &persistence.GetSemaphoreTokenByIDRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket, TokenID: tokenID,
	})
	s.NoError(err)
	s.Equal(owner, byID.Token.Holder)

	// reverse read shows the held token
	byOwner, err := manager.GetSemaphoreTokenByOwner(ctx, &persistence.GetSemaphoreTokenByOwnerRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket, OwnerID: owner,
	})
	s.NoError(err)
	s.Equal(tokenID, byOwner.Token.HeldToken)

	// a release by the wrong owner does not apply
	wrongRelease, err := manager.ReleaseSemaphoreToken(ctx, &persistence.ReleaseSemaphoreTokenRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket, TokenID: tokenID, OwnerID: "not-the-owner",
	})
	s.NoError(err)
	s.False(wrongRelease.Applied)

	// the real owner's release applies
	release, err := manager.ReleaseSemaphoreToken(ctx, &persistence.ReleaseSemaphoreTokenRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket, TokenID: tokenID, OwnerID: owner,
	})
	s.NoError(err)
	s.True(release.Applied)

	// the slot is free again
	byID, err = manager.GetSemaphoreTokenByID(ctx, &persistence.GetSemaphoreTokenByIDRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket, TokenID: tokenID,
	})
	s.NoError(err)
	s.Equal("", byID.Token.Holder)

	// the reverse row is gone
	_, err = manager.GetSemaphoreTokenByOwner(ctx, &persistence.GetSemaphoreTokenByOwnerRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket, OwnerID: owner,
	})
	s.Error(err)
}

// TestGrantSameOwnerDifferentTokenIsRejected verifies the IF NOT EXISTS owner
// guard: once an owner holds a token, a second grant of a different token to the
// same owner_id does not apply and surfaces the already-held token for reuse.
func (s *SemaphoreTokenPersistenceSuite) TestGrantSameOwnerDifferentTokenIsRejected() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	manager, err := s.PersistenceFactory.NewSemaphoreTokenManager()
	s.NoError(err)
	defer manager.Close()

	domainID := uuid.NewString()
	semaphoreName := "sem-" + uuid.NewString()
	bucket := 0
	firstToken := 1
	secondToken := 2
	owner := "owner-" + uuid.NewString()

	// seed two free slots
	s.NoError(manager.SeedSemaphoreTokens(ctx, &persistence.SeedSemaphoreTokensRequest{
		DomainID:      domainID,
		SemaphoreName: semaphoreName,
		Bucket:        bucket,
		TokenIDs:      []int{firstToken, secondToken},
	}))

	// the owner claims the first token
	grantResp, err := manager.GrantSemaphoreToken(ctx, &persistence.GrantSemaphoreTokenRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket, TokenID: firstToken, OwnerID: owner,
	})
	s.NoError(err)
	s.True(grantResp.Applied)
	s.Zero(grantResp.AlreadyHeldToken)

	// a second grant of a different token to the same owner is rejected by the
	// owner guard, and reports the token the owner already holds.
	grantSecond, err := manager.GrantSemaphoreToken(ctx, &persistence.GrantSemaphoreTokenRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket, TokenID: secondToken, OwnerID: owner,
	})
	s.NoError(err)
	s.False(grantSecond.Applied)
	s.Equal(firstToken, grantSecond.AlreadyHeldToken)

	// the second slot was never claimed and is still free
	byID, err := manager.GetSemaphoreTokenByID(ctx, &persistence.GetSemaphoreTokenByIDRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket, TokenID: secondToken,
	})
	s.NoError(err)
	s.Equal("", byID.Token.Holder)
}

// TestSeedIsIdempotent verifies that re-seeding a bucket never clobbers a held slot.
func (s *SemaphoreTokenPersistenceSuite) TestSeedIsIdempotent() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	manager, err := s.PersistenceFactory.NewSemaphoreTokenManager()
	s.NoError(err)
	defer manager.Close()

	domainID := uuid.NewString()
	semaphoreName := "sem-" + uuid.NewString()
	bucket := 0
	tokenID := 1
	owner := "owner-" + uuid.NewString()

	seed := &persistence.SeedSemaphoreTokensRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket, TokenIDs: []int{tokenID},
	}
	s.NoError(manager.SeedSemaphoreTokens(ctx, seed))

	grantResp, err := manager.GrantSemaphoreToken(ctx, &persistence.GrantSemaphoreTokenRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket, TokenID: tokenID, OwnerID: owner,
	})
	s.NoError(err)
	s.True(grantResp.Applied)

	// re-seed: must not reset the held slot back to free
	s.NoError(manager.SeedSemaphoreTokens(ctx, seed))

	byID, err := manager.GetSemaphoreTokenByID(ctx, &persistence.GetSemaphoreTokenByIDRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket, TokenID: tokenID,
	})
	s.NoError(err)
	s.Equal(owner, byID.Token.Holder)
}

// TestListSemaphoreTokensByBucket verifies a bucket scan returns both row kinds,
// paginated.
func (s *SemaphoreTokenPersistenceSuite) TestListSemaphoreTokensByBucket() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	manager, err := s.PersistenceFactory.NewSemaphoreTokenManager()
	s.NoError(err)
	defer manager.Close()

	domainID := uuid.NewString()
	semaphoreName := "sem-" + uuid.NewString()
	bucket := 0
	numTokens := 5

	tokenIDs := make([]int, 0, numTokens)
	for i := 1; i <= numTokens; i++ {
		tokenIDs = append(tokenIDs, i)
	}
	s.NoError(manager.SeedSemaphoreTokens(ctx, &persistence.SeedSemaphoreTokensRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket, TokenIDs: tokenIDs,
	}))

	// grant a couple, which adds reverse (owner) rows to the partition
	numGranted := 2
	for i := 1; i <= numGranted; i++ {
		grantResp, err := manager.GrantSemaphoreToken(ctx, &persistence.GrantSemaphoreTokenRequest{
			DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket, TokenID: i, OwnerID: "owner-" + uuid.NewString(),
		})
		s.NoError(err)
		s.True(grantResp.Applied)
	}

	// scan the whole partition: numTokens token rows + numGranted owner rows
	pageSize := 3
	total := 0
	var nextPageToken []byte
	for {
		listResp, err := manager.ListSemaphoreTokensByBucket(ctx, &persistence.ListSemaphoreTokensByBucketRequest{
			DomainID:      domainID,
			SemaphoreName: semaphoreName,
			Bucket:        bucket,
			PageSize:      pageSize,
			NextPageToken: nextPageToken,
		})
		s.NoError(err)
		s.NotNil(listResp)
		total += len(listResp.Tokens)
		if len(listResp.NextPageToken) == 0 {
			break
		}
		nextPageToken = listResp.NextPageToken
	}
	s.Equal(numTokens+numGranted, total)
}
