package api

import (
	"context"
	"errors"
	"fmt"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/frontend/validate"
)

// CreateSemaphore creates a semaphore. It returns BadRequestError if the semaphore already exists.
func (wh *WorkflowHandler) CreateSemaphore(
	ctx context.Context,
	request *types.CreateSemaphoreRequest,
) (*types.CreateSemaphoreResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if request == nil {
		return nil, validate.ErrRequestNotSet
	}

	domainName := request.GetDomain()
	if domainName == "" {
		return nil, validate.ErrDomainNotSet
	}
	if !wh.config.EnableDistributedSemaphore(domainName) {
		return nil, &types.BadRequestError{Message: fmt.Sprintf(
			"Semaphores are not enabled for domain %q. Set dynamic config system.enableDistributedSemaphore=true for this domain to enable them.",
			domainName,
		)}
	}
	semaphoreName := request.GetSemaphoreName()
	if semaphoreName == "" {
		return nil, &types.BadRequestError{Message: "SemaphoreName is not set on request."}
	}
	size := request.GetSize()
	if size <= 0 {
		return nil, &types.BadRequestError{Message: fmt.Sprintf("Size must be positive, got %d.", size)}
	}
	bucketSize := request.GetBucketSize()
	if bucketSize < 0 || bucketSize > persistence.MaxSemaphoreBucketSize {
		return nil, &types.BadRequestError{Message: fmt.Sprintf(
			"BucketSize must be between 0 and %d, got %d.", persistence.MaxSemaphoreBucketSize, bucketSize,
		)}
	}

	domainID, err := wh.GetDomainCache().GetDomainID(domainName)
	if err != nil {
		return nil, err
	}

	resp, err := wh.GetSemaphoreMetadataManager().CreateSemaphore(ctx, &persistence.CreateSemaphoreRequest{
		DomainID:      domainID,
		SemaphoreName: semaphoreName,
		Size:          int(size),
		BucketSize:    int(bucketSize),
	})
	if err != nil {
		if errors.As(err, new(*persistence.ConditionFailedError)) {
			return nil, &types.BadRequestError{Message: fmt.Sprintf(
				"semaphore %q already exists in domain %q", semaphoreName, domainName,
			)}
		}
		return nil, err
	}
	return &types.CreateSemaphoreResponse{Semaphore: toSemaphore(resp.Semaphore)}, nil
}

func toSemaphore(s *persistence.SemaphoreMetadata) *types.Semaphore {
	return &types.Semaphore{
		SemaphoreName: s.SemaphoreName,
		Size:          int32(s.Size),
		BucketSize:    int32(s.BucketSize),
	}
}
