package semaphore

import (
	"fmt"

	"github.com/dgryski/go-farm"
)

// A semaphore is split into buckets, and each bucket is served by one Matching host. A request to
// acquire or release a semaphore finds its Matching host in two steps:
//
//  1. OwnerIDToBucket picks the bucket from the request's owner_id.
//  2. RingKey names that bucket on the membership ring, which picks the Matching host.

// OwnerIDToBucket returns the bucket that serves the given owner_id.
//
// The acquire and the later release for one owner_id each compute this separately, so they must
// get the same answer. Never change the hash or add a seed: a release sent to a different bucket
// cannot free the slot its acquire took. For the same reason, numBuckets must come from NumBuckets
// on the semaphore's stored size and bucket_size, which never change after creation.
func OwnerIDToBucket(ownerID string, numBuckets int) (int, error) {
	if numBuckets < 1 {
		return 0, fmt.Errorf("numBuckets must be positive, got %d", numBuckets)
	}
	return int(farm.Fingerprint32([]byte(ownerID)) % uint32(numBuckets)), nil
}

// RingKey returns the key used to look up a bucket's Matching host on the membership ring.
//
// The caller uses it to choose a host, and that host uses it to check it still owns the bucket.
// Both must build the same key, or the host refuses every request.
func RingKey(domainID, semaphoreName string, bucket int) string {
	return fmt.Sprintf("%s_%s_%d", domainID, semaphoreName, bucket)
}
