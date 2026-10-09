package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSemaphoreGetters(t *testing.T) {
	var nilSem *Semaphore
	assert.Equal(t, "", nilSem.GetSemaphoreName())
	assert.Equal(t, int32(0), nilSem.GetCapacity())
	assert.Equal(t, int32(0), nilSem.GetBucketCapacity())

	sem := &Semaphore{SemaphoreName: "sem", Capacity: 10, BucketCapacity: 5}
	assert.Equal(t, "sem", sem.GetSemaphoreName())
	assert.Equal(t, int32(10), sem.GetCapacity())
	assert.Equal(t, int32(5), sem.GetBucketCapacity())
}

func TestCreateSemaphoreRequestGetters(t *testing.T) {
	var nilReq *CreateSemaphoreRequest
	assert.Equal(t, "", nilReq.GetDomain())
	assert.Equal(t, "", nilReq.GetSemaphoreName())
	assert.Equal(t, int32(0), nilReq.GetCapacity())
	assert.Equal(t, int32(0), nilReq.GetBucketCapacity())

	req := &CreateSemaphoreRequest{Domain: "domain", SemaphoreName: "sem", Capacity: 10, BucketCapacity: 5}
	assert.Equal(t, "domain", req.GetDomain())
	assert.Equal(t, "sem", req.GetSemaphoreName())
	assert.Equal(t, int32(10), req.GetCapacity())
	assert.Equal(t, int32(5), req.GetBucketCapacity())
}

func TestCreateSemaphoreResponseGetters(t *testing.T) {
	var nilResp *CreateSemaphoreResponse
	assert.Nil(t, nilResp.GetSemaphore())

	sem := &Semaphore{SemaphoreName: "sem"}
	assert.Equal(t, sem, (&CreateSemaphoreResponse{Semaphore: sem}).GetSemaphore())
}
