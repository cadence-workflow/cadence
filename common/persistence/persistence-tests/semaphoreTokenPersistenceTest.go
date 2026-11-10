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
	"github.com/uber/cadence/common/types"
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

// TestGrantAndRelease walks a slot through the grant/release lifecycle, from no row
// to held to free and held again, verifying the conditional writes and both index directions.
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

	// token rows are not seeded, so a slot that has never been granted has no row
	_, err = manager.GetSemaphoreOwnershipByToken(ctx, &persistence.GetSemaphoreOwnershipByTokenRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket, TokenID: tokenID,
	})
	var notExists *types.EntityNotExistsError
	s.ErrorAs(err, &notExists)

	// the first grant applies and creates the token row
	grantResp, err := manager.GrantSemaphoreToken(ctx, &persistence.GrantSemaphoreTokenRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket, TokenID: tokenID, OwnerID: owner,
	})
	s.NoError(err)
	s.Equal(persistence.SemaphoreGrantApplied, grantResp.Outcome)

	// re-granting the same slot does not apply
	grantAgain, err := manager.GrantSemaphoreToken(ctx, &persistence.GrantSemaphoreTokenRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket, TokenID: tokenID, OwnerID: "someone-else",
	})
	s.NoError(err)
	s.Equal(persistence.SemaphoreGrantSlotTaken, grantAgain.Outcome)

	// forward read shows the holder
	byToken, err := manager.GetSemaphoreOwnershipByToken(ctx, &persistence.GetSemaphoreOwnershipByTokenRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket, TokenID: tokenID,
	})
	s.NoError(err)
	s.Equal(owner, byToken.Ownership.Holder)

	// reverse read shows the held token
	byOwner, err := manager.GetSemaphoreOwnershipByOwner(ctx, &persistence.GetSemaphoreOwnershipByOwnerRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket, OwnerID: owner,
	})
	s.NoError(err)
	s.Equal(tokenID, byOwner.Ownership.HeldToken)

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
	byToken, err = manager.GetSemaphoreOwnershipByToken(ctx, &persistence.GetSemaphoreOwnershipByTokenRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket, TokenID: tokenID,
	})
	s.NoError(err)
	s.Equal("", byToken.Ownership.Holder)

	// the reverse row is gone
	_, err = manager.GetSemaphoreOwnershipByOwner(ctx, &persistence.GetSemaphoreOwnershipByOwnerRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket, OwnerID: owner,
	})
	s.Error(err)

	// a released slot keeps its row as free, and can be granted again
	nextOwner := "owner-" + uuid.NewString()
	grantReleased, err := manager.GrantSemaphoreToken(ctx, &persistence.GrantSemaphoreTokenRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket, TokenID: tokenID, OwnerID: nextOwner,
	})
	s.NoError(err)
	s.Equal(persistence.SemaphoreGrantApplied, grantReleased.Outcome)
	byToken, err = manager.GetSemaphoreOwnershipByToken(ctx, &persistence.GetSemaphoreOwnershipByTokenRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket, TokenID: tokenID,
	})
	s.NoError(err)
	s.Equal(nextOwner, byToken.Ownership.Holder)
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

	// the owner claims the first token
	grantResp, err := manager.GrantSemaphoreToken(ctx, &persistence.GrantSemaphoreTokenRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket, TokenID: firstToken, OwnerID: owner,
	})
	s.NoError(err)
	s.Equal(persistence.SemaphoreGrantApplied, grantResp.Outcome)
	s.Zero(grantResp.HeldToken)

	// a second grant of a different token to the same owner is rejected by the
	// owner guard, and reports the token the owner already holds.
	grantSecond, err := manager.GrantSemaphoreToken(ctx, &persistence.GrantSemaphoreTokenRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket, TokenID: secondToken, OwnerID: owner,
	})
	s.NoError(err)
	s.Equal(persistence.SemaphoreGrantAlreadyHeld, grantSecond.Outcome)
	s.Equal(firstToken, grantSecond.HeldToken)

	// the refused batch wrote nothing, so the second slot still has no row
	_, err = manager.GetSemaphoreOwnershipByToken(ctx, &persistence.GetSemaphoreOwnershipByTokenRequest{
		DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket, TokenID: secondToken,
	})
	var notExists *types.EntityNotExistsError
	s.ErrorAs(err, &notExists)
}

// TestScanSemaphoreBucket verifies a bucket scan returns both row types,
// paginated.
func (s *SemaphoreTokenPersistenceSuite) TestScanSemaphoreBucket() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	manager, err := s.PersistenceFactory.NewSemaphoreTokenManager()
	s.NoError(err)
	defer manager.Close()

	domainID := uuid.NewString()
	semaphoreName := "sem-" + uuid.NewString()
	bucket := 0

	// grant three slots, which writes a token row and an owner row for each
	numGranted := 3
	for i := 1; i <= numGranted; i++ {
		grantResp, err := manager.GrantSemaphoreToken(ctx, &persistence.GrantSemaphoreTokenRequest{
			DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket, TokenID: i, OwnerID: "owner-" + uuid.NewString(),
		})
		s.NoError(err)
		s.Equal(persistence.SemaphoreGrantApplied, grantResp.Outcome)
	}

	// scan the whole partition: only granted slots have token rows, so numGranted of each
	// type. A page size of 2 makes the scan span several pages.
	pageSize := 2
	total := 0
	byRowType := map[persistence.SemaphoreRowType]int{}
	var nextPageToken []byte
	for {
		scanResp, err := manager.ScanSemaphoreBucket(ctx, &persistence.ScanSemaphoreBucketRequest{
			DomainID:      domainID,
			SemaphoreName: semaphoreName,
			Bucket:        bucket,
			PageSize:      pageSize,
			NextPageToken: nextPageToken,
		})
		s.NoError(err)
		s.NotNil(scanResp)
		total += len(scanResp.Ownerships)
		for _, ownership := range scanResp.Ownerships {
			byRowType[ownership.RowType]++
		}
		if len(scanResp.NextPageToken) == 0 {
			break
		}
		nextPageToken = scanResp.NextPageToken
	}
	s.Equal(2*numGranted, total)
	// A scan is the only read that returns both types interleaved, so it is the only place
	// the stored type column is what tells them apart.
	s.Equal(numGranted, byRowType[persistence.SemaphoreRowTypeToken])
	s.Equal(numGranted, byRowType[persistence.SemaphoreRowTypeOwner])
}
