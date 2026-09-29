package types

// Semaphore is a named concurrency limit in a domain.
type Semaphore struct {
	SemaphoreName string `json:"semaphoreName,omitempty"`
	// Size is the total number of tokens.
	Size int32 `json:"size,omitempty"`
	// BucketSize is the number of tokens in each bucket.
	BucketSize int32 `json:"bucketSize,omitempty"`
}

func (v *Semaphore) GetSemaphoreName() (o string) {
	if v != nil {
		return v.SemaphoreName
	}
	return
}

func (v *Semaphore) GetSize() (o int32) {
	if v != nil {
		return v.Size
	}
	return
}

func (v *Semaphore) GetBucketSize() (o int32) {
	if v != nil {
		return v.BucketSize
	}
	return
}

// CreateSemaphoreRequest is the request to create a semaphore.
type CreateSemaphoreRequest struct {
	Domain        string `json:"domain,omitempty"`
	SemaphoreName string `json:"semaphoreName,omitempty"`
	// Size is the total number of tokens. Must be positive.
	Size int32 `json:"size,omitempty"`
	// BucketSize is optional: zero means the server picks a default.
	BucketSize int32 `json:"bucketSize,omitempty"`
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

func (v *CreateSemaphoreRequest) GetSize() (o int32) {
	if v != nil {
		return v.Size
	}
	return
}

func (v *CreateSemaphoreRequest) GetBucketSize() (o int32) {
	if v != nil {
		return v.BucketSize
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
