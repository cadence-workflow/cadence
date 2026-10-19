//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination handler_mock.go

package semaphore

import (
	"context"
	"errors"
	"fmt"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/domain"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

type (
	// Handler is the semaphore operation handler
	Handler interface {
		CreateSemaphore(
			ctx context.Context,
			createRequest *types.CreateSemaphoreRequest,
		) (*types.CreateSemaphoreResponse, error)
	}

	// handlerImpl is the semaphore operation handler implementation
	handlerImpl struct {
		semaphoreMetadataManager persistence.SemaphoreMetadataManager
		domainCache              cache.DomainCache
		semaphoreReplicator      domain.SemaphoreReplicator
	}
)

var _ Handler = (*handlerImpl)(nil)

// NewHandler create a new semaphore handler
func NewHandler(
	semaphoreMetadataManager persistence.SemaphoreMetadataManager,
	domainCache cache.DomainCache,
	semaphoreReplicator domain.SemaphoreReplicator,
) Handler {
	return &handlerImpl{
		semaphoreMetadataManager: semaphoreMetadataManager,
		domainCache:              domainCache,
		semaphoreReplicator:      semaphoreReplicator,
	}
}

// CreateSemaphore creates a semaphore and replicates it for a global domain.
func (h *handlerImpl) CreateSemaphore(
	ctx context.Context,
	createRequest *types.CreateSemaphoreRequest,
) (*types.CreateSemaphoreResponse, error) {
	domainName := createRequest.GetDomain()
	semaphoreName := createRequest.GetSemaphoreName()
	capacity := createRequest.GetCapacity()
	bucketCapacity := createRequest.GetBucketCapacity()

	domainEntry, err := h.domainCache.GetDomain(domainName)
	if err != nil {
		return nil, err
	}
	domainID := domainEntry.GetInfo().ID

	// The API calls them capacity and bucket_capacity; persistence stores them as size and bucket_size.
	resp, err := h.semaphoreMetadataManager.CreateSemaphore(ctx, &persistence.CreateSemaphoreRequest{
		DomainID:      domainID,
		SemaphoreName: semaphoreName,
		Size:          int(capacity),
		BucketSize:    int(bucketCapacity),
	})
	var semaphore *persistence.SemaphoreMetadata
	if err != nil {
		// ConditionFailedError means the semaphore already exists. Any other error is a real failure.
		if !errors.As(err, new(*persistence.ConditionFailedError)) {
			return nil, err
		}

		// A repeat create with the same settings succeeds and publishes again, so a caller can
		// retry a create whose replication publish failed.
		getResp, getErr := h.semaphoreMetadataManager.GetSemaphore(ctx, &persistence.GetSemaphoreRequest{
			DomainID:      domainID,
			SemaphoreName: semaphoreName,
		})
		if getErr != nil {
			return nil, getErr
		}
		if !existingSemaphoreMatchesCreateRequest(getResp.Semaphore, createRequest) {
			return nil, &types.BadRequestError{Message: fmt.Sprintf(
				"semaphore %q already exists in domain %q", semaphoreName, domainName,
			)}
		}
		semaphore = getResp.Semaphore
	} else {
		semaphore = resp.Semaphore
	}

	// Only a global domain has other clusters to replicate to.
	if domainEntry.IsGlobalDomain() {
		if err := h.semaphoreReplicator.HandleTransmissionTask(ctx, semaphore); err != nil {
			return nil, err
		}
	}
	return &types.CreateSemaphoreResponse{Semaphore: toSemaphore(semaphore)}, nil
}

// existingSemaphoreMatchesCreateRequest reports whether the stored semaphore has the requested capacities.
func existingSemaphoreMatchesCreateRequest(
	existing *persistence.SemaphoreMetadata,
	createRequest *types.CreateSemaphoreRequest,
) bool {
	// Bucket capacity is optional. When it is 0, the persistence layer stores
	// DefaultSemaphoreBucketSize instead, so compare with that.
	wantBucketSize := int(createRequest.GetBucketCapacity())
	if wantBucketSize == 0 {
		wantBucketSize = persistence.DefaultSemaphoreBucketSize
	}
	return existing.Size == int(createRequest.GetCapacity()) && existing.BucketSize == wantBucketSize
}

func toSemaphore(s *persistence.SemaphoreMetadata) *types.Semaphore {
	return &types.Semaphore{
		SemaphoreName:  s.SemaphoreName,
		Capacity:       int32(s.Size),
		BucketCapacity: int32(s.BucketSize),
	}
}
