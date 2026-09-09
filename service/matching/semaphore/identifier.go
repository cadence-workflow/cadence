package semaphore

import "fmt"

// Identifier names one semaphore bucket. A semaphore of `size` slots is split into
// ceil(size/bucket_size) buckets.
//
// A bucket is three things at once: one partition of semaphore_tokens, the target of every
// conditional write that grants a slot in it, and the unit one Matching host serves. The first
// two are what make a grant correct; the third only keeps this host's free-set useful.
//
// The struct is used directly as a map key, so keep every field comparable. String() is
// for logs and metrics.
type Identifier struct {
	DomainID      string
	SemaphoreName string
	Bucket        int
}

// NewIdentifier builds a bucket identifier, rejecting the values the persistence layer
// would reject anyway. Catching them here means a misconfigured bucket fails at
// construction rather than on its first grant.
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
