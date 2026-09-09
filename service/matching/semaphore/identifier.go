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
	if domainID == "" {
		return Identifier{}, fmt.Errorf("%w: domainID is required", ErrInvalidRequest)
	}
	if semaphoreName == "" {
		return Identifier{}, fmt.Errorf("%w: semaphoreName is required", ErrInvalidRequest)
	}
	if bucket < 0 {
		return Identifier{}, fmt.Errorf("%w: bucket must not be negative, got %d", ErrInvalidRequest, bucket)
	}
	return Identifier{DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket}, nil
}

func (id Identifier) String() string {
	return fmt.Sprintf("%s/%s/%d", id.DomainID, id.SemaphoreName, id.Bucket)
}
