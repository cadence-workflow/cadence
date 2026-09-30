package thrift

import (
	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

func FromSemaphore(t *types.Semaphore) *shared.Semaphore {
	if t == nil {
		return nil
	}
	return &shared.Semaphore{
		SemaphoreName:  common.StringPtr(t.SemaphoreName),
		Capacity:       common.Int32Ptr(t.Capacity),
		BucketCapacity: common.Int32Ptr(t.BucketCapacity),
	}
}

func ToSemaphore(t *shared.Semaphore) *types.Semaphore {
	if t == nil {
		return nil
	}
	return &types.Semaphore{
		SemaphoreName:  t.GetSemaphoreName(),
		Capacity:       t.GetCapacity(),
		BucketCapacity: t.GetBucketCapacity(),
	}
}

func FromCreateSemaphoreRequest(t *types.CreateSemaphoreRequest) *shared.CreateSemaphoreRequest {
	if t == nil {
		return nil
	}
	return &shared.CreateSemaphoreRequest{
		Domain:         common.StringPtr(t.Domain),
		SemaphoreName:  common.StringPtr(t.SemaphoreName),
		Capacity:       common.Int32Ptr(t.Capacity),
		BucketCapacity: common.Int32Ptr(t.BucketCapacity),
	}
}

func ToCreateSemaphoreRequest(t *shared.CreateSemaphoreRequest) *types.CreateSemaphoreRequest {
	if t == nil {
		return nil
	}
	return &types.CreateSemaphoreRequest{
		Domain:         t.GetDomain(),
		SemaphoreName:  t.GetSemaphoreName(),
		Capacity:       t.GetCapacity(),
		BucketCapacity: t.GetBucketCapacity(),
	}
}

func FromCreateSemaphoreResponse(t *types.CreateSemaphoreResponse) *shared.CreateSemaphoreResponse {
	if t == nil {
		return nil
	}
	return &shared.CreateSemaphoreResponse{
		Semaphore: FromSemaphore(t.Semaphore),
	}
}

func ToCreateSemaphoreResponse(t *shared.CreateSemaphoreResponse) *types.CreateSemaphoreResponse {
	if t == nil {
		return nil
	}
	return &types.CreateSemaphoreResponse{
		Semaphore: ToSemaphore(t.Semaphore),
	}
}
