package errors

import "fmt"

var _ error = &SemaphoreNotOwnedByHostError{}

// SemaphoreNotOwnedByHostError says the ring puts this bucket on another host. The caller should
// go to OwnedByIdentity rather than wait, since nothing about this host will change.
type SemaphoreNotOwnedByHostError struct {
	OwnedByIdentity string
	MyIdentity      string
	Bucket          string
}

func (m *SemaphoreNotOwnedByHostError) Error() string {
	return fmt.Sprintf("semaphore bucket is not owned by this host: OwnedBy: %s, Me: %s, Bucket: %s",
		m.OwnedByIdentity, m.MyIdentity, m.Bucket)
}

func NewSemaphoreNotOwnedByHostError(ownedByIdentity string, myIdentity string, bucket string) *SemaphoreNotOwnedByHostError {
	return &SemaphoreNotOwnedByHostError{
		OwnedByIdentity: ownedByIdentity,
		MyIdentity:      myIdentity,
		Bucket:          bucket,
	}
}
