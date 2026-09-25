package errors

import "fmt"

var _ error = &SemaphoreNotOwnedByHostError{}

// SemaphoreNotOwnedByHostError says the ring puts this bucket on another host. The caller should
// go to OwnedByIdentity rather than wait, since nothing about this host will change.
type SemaphoreNotOwnedByHostError struct {
	OwnedByIdentity string
	MyIdentity      string
	// BucketID names the bucket in full, as "<domainID>/<semaphoreName>/<bucket>".
	BucketID string
}

func (m *SemaphoreNotOwnedByHostError) Error() string {
	return fmt.Sprintf("semaphore bucket is not owned by this host: OwnedBy: %s, Me: %s, BucketID: %s",
		m.OwnedByIdentity, m.MyIdentity, m.BucketID)
}

func NewSemaphoreNotOwnedByHostError(ownedByIdentity string, myIdentity string, bucketID string) *SemaphoreNotOwnedByHostError {
	return &SemaphoreNotOwnedByHostError{
		OwnedByIdentity: ownedByIdentity,
		MyIdentity:      myIdentity,
		BucketID:        bucketID,
	}
}
