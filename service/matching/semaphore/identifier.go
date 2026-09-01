package semaphore

import "fmt"

// Identifier names one semaphore bucket. A semaphore of `size` slots is split into
// ceil(size/bucket_size) buckets.
//
// A bucket is the unit of three things at once, and the design depends on all three lining
// up: exactly one Matching host owns it, it is one partition of semaphore_tokens, and every
// grant against it is one conditional write to that partition.
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
		return Identifier{}, fmt.Errorf("domainID is required")
	}
	if semaphoreName == "" {
		return Identifier{}, fmt.Errorf("semaphoreName is required")
	}
	if bucket < 0 {
		return Identifier{}, fmt.Errorf("bucket must not be negative, got %d", bucket)
	}
	return Identifier{DomainID: domainID, SemaphoreName: semaphoreName, Bucket: bucket}, nil
}

func (id Identifier) String() string {
	return fmt.Sprintf("%s/%s/%d", id.DomainID, id.SemaphoreName, id.Bucket)
}
