package proto

import (
	apiv1 "github.com/uber/cadence-idl/go/proto/api/v1"

	"github.com/uber/cadence/common/types"
)

func FromSemaphore(t *types.Semaphore) *apiv1.Semaphore {
	if t == nil {
		return nil
	}
	return &apiv1.Semaphore{
		SemaphoreName: t.SemaphoreName,
		Size_:         t.Size,
		BucketSize:    t.BucketSize,
	}
}

func ToSemaphore(t *apiv1.Semaphore) *types.Semaphore {
	if t == nil {
		return nil
	}
	return &types.Semaphore{
		SemaphoreName: t.SemaphoreName,
		Size:          t.Size_,
		BucketSize:    t.BucketSize,
	}
}

func FromCreateSemaphoreRequest(t *types.CreateSemaphoreRequest) *apiv1.CreateSemaphoreRequest {
	if t == nil {
		return nil
	}
	return &apiv1.CreateSemaphoreRequest{
		Domain:        t.Domain,
		SemaphoreName: t.SemaphoreName,
		Size_:         t.Size,
		BucketSize:    t.BucketSize,
	}
}

func ToCreateSemaphoreRequest(t *apiv1.CreateSemaphoreRequest) *types.CreateSemaphoreRequest {
	if t == nil {
		return nil
	}
	return &types.CreateSemaphoreRequest{
		Domain:        t.Domain,
		SemaphoreName: t.SemaphoreName,
		Size:          t.Size_,
		BucketSize:    t.BucketSize,
	}
}

func FromCreateSemaphoreResponse(t *types.CreateSemaphoreResponse) *apiv1.CreateSemaphoreResponse {
	if t == nil {
		return nil
	}
	return &apiv1.CreateSemaphoreResponse{
		Semaphore: FromSemaphore(t.Semaphore),
	}
}

func ToCreateSemaphoreResponse(t *apiv1.CreateSemaphoreResponse) *types.CreateSemaphoreResponse {
	if t == nil {
		return nil
	}
	return &types.CreateSemaphoreResponse{
		Semaphore: ToSemaphore(t.Semaphore),
	}
}
