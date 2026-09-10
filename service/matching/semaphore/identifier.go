package semaphore

import "fmt"

// Identifier names what one Manager serves: one bucket of one semaphore. A semaphore of `size`
// slots is split into ceil(size/bucket_size) buckets, and a bucket is one partition of
// semaphore_tokens.
//
// Used directly as a map key, so every field must stay comparable. String() is for logs and
// metrics.
type Identifier struct {
	DomainID      string
	SemaphoreName string
	Bucket        int
}

// NewIdentifier rejects the values persistence would reject anyway, so a misconfigured manager
// fails here rather than on its first grant.
func NewIdentifier(domainID, semaphoreName string, bucket int) (Identifier, error) {
	id := Identifier{DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket}
	if err := id.validate(); err != nil {
		return Identifier{}, err
	}
	return id, nil
}

func (id Identifier) validate() error {
	if id.DomainID == "" {
		return fmt.Errorf("%w: domainID is required", ErrInvalidRequest)
	}
	if id.SemaphoreName == "" {
		return fmt.Errorf("%w: semaphoreName is required", ErrInvalidRequest)
	}
	if id.Bucket < 0 {
		return fmt.Errorf("%w: bucket must not be negative, got %d", ErrInvalidRequest, id.Bucket)
	}
	return nil
}

func (id Identifier) String() string {
	return fmt.Sprintf("%s/%s/%d", id.DomainID, id.SemaphoreName, id.Bucket)
}
