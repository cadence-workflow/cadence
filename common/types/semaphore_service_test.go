package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSemaphoreGetters(t *testing.T) {
	var nilSem *Semaphore
	assert.Equal(t, "", nilSem.GetSemaphoreName())
	assert.Equal(t, int32(0), nilSem.GetSize())
	assert.Equal(t, int32(0), nilSem.GetBucketSize())

	sem := &Semaphore{SemaphoreName: "sem", Size: 10, BucketSize: 5}
	assert.Equal(t, "sem", sem.GetSemaphoreName())
	assert.Equal(t, int32(10), sem.GetSize())
	assert.Equal(t, int32(5), sem.GetBucketSize())
}

func TestCreateSemaphoreRequestGetters(t *testing.T) {
	var nilReq *CreateSemaphoreRequest
	assert.Equal(t, "", nilReq.GetDomain())
	assert.Equal(t, "", nilReq.GetSemaphoreName())
	assert.Equal(t, int32(0), nilReq.GetSize())
	assert.Equal(t, int32(0), nilReq.GetBucketSize())

	req := &CreateSemaphoreRequest{Domain: "domain", SemaphoreName: "sem", Size: 10, BucketSize: 5}
	assert.Equal(t, "domain", req.GetDomain())
	assert.Equal(t, "sem", req.GetSemaphoreName())
	assert.Equal(t, int32(10), req.GetSize())
	assert.Equal(t, int32(5), req.GetBucketSize())
}

func TestCreateSemaphoreResponseGetters(t *testing.T) {
	var nilResp *CreateSemaphoreResponse
	assert.Nil(t, nilResp.GetSemaphore())

	sem := &Semaphore{SemaphoreName: "sem"}
	assert.Equal(t, sem, (&CreateSemaphoreResponse{Semaphore: sem}).GetSemaphore())
}
