package semaphore

import (
	"context"
	"errors"
	"fmt"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

// scanPageSize is one larger than a full bucket -- one token row per token plus one owner row
// per owner -- so the startup scan takes a single round trip. An optimization only: paging
// runs to the end whatever the size.
const scanPageSize = 2*persistence.MaxSemaphoreBucketSize + 1

// bucketDB makes one bucket's persistence calls, with the bucket's identity filled in. Each method
// is one call, and its name says the table: Metadata for semaphore_metadata, Token for
// semaphore_tokens.
type bucketDB struct {
	id       Identifier
	metadata persistence.SemaphoreMetadataManager
	tokens   persistence.SemaphoreTokenManager
}

// getMetadata reads the semaphore's row from semaphore_metadata.
func (db *bucketDB) getMetadata(ctx context.Context) (*persistence.SemaphoreMetadata, error) {
	resp, err := db.metadata.GetSemaphore(ctx, &persistence.GetSemaphoreRequest{
		DomainID:      db.id.DomainID,
		SemaphoreName: db.id.SemaphoreName,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Semaphore == nil {
		return nil, &types.InternalServiceError{Message: fmt.Sprintf("no semaphore returned for domain %v, semaphore %v", db.id.DomainID, db.id.SemaphoreName)}
	}
	return resp.Semaphore, nil
}

// scanTokenRows pages through the bucket's partition of semaphore_tokens and returns every row,
// token rows and owner rows.
func (db *bucketDB) scanTokenRows(ctx context.Context) ([]*persistence.SemaphoreOwnership, error) {
	var rows []*persistence.SemaphoreOwnership
	var pageToken []byte
	for {
		resp, err := db.tokens.ScanSemaphoreBucket(ctx, &persistence.ScanSemaphoreBucketRequest{
			DomainID:      db.id.DomainID,
			SemaphoreName: db.id.SemaphoreName,
			Bucket:        db.id.Bucket,
			PageSize:      scanPageSize,
			NextPageToken: pageToken,
		})
		if err != nil {
			return nil, err
		}
		rows = append(rows, resp.Ownerships...)

		pageToken = resp.NextPageToken
		if len(pageToken) == 0 {
			return rows, nil
		}
	}
}

// grantToken gives tokenID to ownerID with the conditional write to semaphore_tokens. A write
// that does not apply comes back as an outcome, not an error.
func (db *bucketDB) grantToken(ctx context.Context, tokenID int, ownerID string) (*persistence.GrantSemaphoreTokenResponse, error) {
	return db.tokens.GrantSemaphoreToken(ctx, &persistence.GrantSemaphoreTokenRequest{
		DomainID:      db.id.DomainID,
		SemaphoreName: db.id.SemaphoreName,
		Bucket:        db.id.Bucket,
		TokenID:       tokenID,
		OwnerID:       ownerID,
	})
}

// getTokenRow reads tokenID's token row from semaphore_tokens. found is false when the token has
// no row. When found, row is never nil.
func (db *bucketDB) getTokenRow(ctx context.Context, tokenID int) (row *persistence.SemaphoreOwnership, found bool, err error) {
	resp, err := db.tokens.GetSemaphoreOwnershipByToken(ctx, &persistence.GetSemaphoreOwnershipByTokenRequest{
		DomainID:      db.id.DomainID,
		SemaphoreName: db.id.SemaphoreName,
		Bucket:        db.id.Bucket,
		TokenID:       tokenID,
	})
	if err != nil {
		var notExists *types.EntityNotExistsError
		if errors.As(err, &notExists) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if resp == nil || resp.Ownership == nil {
		return nil, false, &types.InternalServiceError{Message: fmt.Sprintf("no token row returned for token %d of bucket %v", tokenID, db.id)}
	}
	return resp.Ownership, true, nil
}
