package testdata

import "github.com/uber/cadence/common/types"

var (
	Semaphore = types.Semaphore{
		SemaphoreName: "my-semaphore",
		Size:          1000,
		BucketSize:    100,
	}

	CreateSemaphoreRequest = types.CreateSemaphoreRequest{
		Domain:        DomainName,
		SemaphoreName: "my-semaphore",
		Size:          1000,
		BucketSize:    100,
	}

	CreateSemaphoreResponse = types.CreateSemaphoreResponse{
		Semaphore: &Semaphore,
	}
)
