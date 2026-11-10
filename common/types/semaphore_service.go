package types

// Semaphore is a named concurrency limit in a domain.
type Semaphore struct {
	SemaphoreName string `json:"semaphoreName,omitempty"`
	// Capacity is the total number of tokens.
	Capacity int32 `json:"capacity,omitempty"`
	// BucketCapacity is the number of tokens in each bucket.
	BucketCapacity int32 `json:"bucketCapacity,omitempty"`
}

func (v *Semaphore) GetSemaphoreName() (o string) {
	if v != nil {
		return v.SemaphoreName
	}
	return
}

func (v *Semaphore) GetCapacity() (o int32) {
	if v != nil {
		return v.Capacity
	}
	return
}

func (v *Semaphore) GetBucketCapacity() (o int32) {
	if v != nil {
		return v.BucketCapacity
	}
	return
}

// CreateSemaphoreRequest is the request to create a semaphore.
type CreateSemaphoreRequest struct {
	Domain        string `json:"domain,omitempty"`
	SemaphoreName string `json:"semaphoreName,omitempty"`
	// Capacity is the total number of tokens. Must be positive.
	Capacity int32 `json:"capacity,omitempty"`
	// BucketCapacity is optional: zero means the server picks a default.
	BucketCapacity int32 `json:"bucketCapacity,omitempty"`
}

func (v *CreateSemaphoreRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

func (v *CreateSemaphoreRequest) GetSemaphoreName() (o string) {
	if v != nil {
		return v.SemaphoreName
	}
	return
}

func (v *CreateSemaphoreRequest) GetCapacity() (o int32) {
	if v != nil {
		return v.Capacity
	}
	return
}

func (v *CreateSemaphoreRequest) GetBucketCapacity() (o int32) {
	if v != nil {
		return v.BucketCapacity
	}
	return
}

// CreateSemaphoreResponse is the response for creating a semaphore.
type CreateSemaphoreResponse struct {
	// Semaphore is the semaphore as stored, with defaults filled in.
	Semaphore *Semaphore `json:"semaphore,omitempty"`
}

func (v *CreateSemaphoreResponse) GetSemaphore() *Semaphore {
	if v != nil {
		return v.Semaphore
	}
	return nil
}
