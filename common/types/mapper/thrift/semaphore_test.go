package thrift

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/testutils"
	"github.com/uber/cadence/common/types/testdata"
)

func TestSemaphore(t *testing.T) {
	for _, item := range []*types.Semaphore{nil, {}, &testdata.Semaphore} {
		assert.Equal(t, item, ToSemaphore(FromSemaphore(item)))
	}
}

func TestCreateSemaphoreRequest(t *testing.T) {
	for _, item := range []*types.CreateSemaphoreRequest{nil, {}, &testdata.CreateSemaphoreRequest} {
		assert.Equal(t, item, ToCreateSemaphoreRequest(FromCreateSemaphoreRequest(item)))
	}
}

func TestCreateSemaphoreResponse(t *testing.T) {
	for _, item := range []*types.CreateSemaphoreResponse{nil, {}, &testdata.CreateSemaphoreResponse} {
		assert.Equal(t, item, ToCreateSemaphoreResponse(FromCreateSemaphoreResponse(item)))
	}
}

func TestSemaphoreFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromSemaphore, ToSemaphore)
}

func TestCreateSemaphoreRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromCreateSemaphoreRequest, ToCreateSemaphoreRequest)
}

func TestCreateSemaphoreResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromCreateSemaphoreResponse, ToCreateSemaphoreResponse)
}
