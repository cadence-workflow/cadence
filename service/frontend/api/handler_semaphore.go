package api

import (
	"context"
	"errors"
	"fmt"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
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
	if !common.IsValidIDLength(
		semaphoreName,
		wh.GetMetricsClient().Scope(metrics.FrontendCreateSemaphoreScope),
		wh.config.MaxIDLengthWarnLimit(),
		wh.config.SemaphoreNameMaxLength(domainName),
		metrics.CadenceErrSemaphoreNameExceededWarnLimit,
		domainName,
		wh.GetLogger(),
		tag.IDTypeSemaphoreName) {
		return nil, &types.BadRequestError{Message: "SemaphoreName exceeds length limit."}
	}
	capacity := request.GetCapacity()
	if capacity <= 0 {
		return nil, &types.BadRequestError{Message: fmt.Sprintf("Capacity must be positive, got %d.", capacity)}
	}
	bucketCapacity := request.GetBucketCapacity()
	if bucketCapacity < 0 || bucketCapacity > persistence.MaxSemaphoreBucketSize {
		return nil, &types.BadRequestError{Message: fmt.Sprintf(
			"BucketCapacity must be between 0 and %d, got %d.", persistence.MaxSemaphoreBucketSize, bucketCapacity,
		)}
	}

	domainID, err := wh.GetDomainCache().GetDomainID(domainName)
	if err != nil {
		return nil, err
	}

	// The API calls them capacity and bucket_capacity; persistence stores them as size and bucket_size.
	resp, err := wh.GetSemaphoreMetadataManager().CreateSemaphore(ctx, &persistence.CreateSemaphoreRequest{
		DomainID:      domainID,
		SemaphoreName: semaphoreName,
		Size:          int(capacity),
		BucketSize:    int(bucketCapacity),
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
		SemaphoreName:  s.SemaphoreName,
		Capacity:       int32(s.Size),
		BucketCapacity: int32(s.BucketSize),
	}
}
