package testdata

import "github.com/uber/cadence/common/types"

var (
	Semaphore = types.Semaphore{
		SemaphoreName:  "my-semaphore",
		Capacity:       1000,
		BucketCapacity: 100,
	}

	CreateSemaphoreRequest = types.CreateSemaphoreRequest{
		Domain:         DomainName,
		SemaphoreName:  "my-semaphore",
		Capacity:       1000,
		BucketCapacity: 100,
	}

	CreateSemaphoreResponse = types.CreateSemaphoreResponse{
		Semaphore: &Semaphore,
	}
)
