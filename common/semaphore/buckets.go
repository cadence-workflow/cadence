package semaphore

import (
	"errors"
	"fmt"
)

// ErrBucketOutOfRange means the bucket number is past the semaphore's last bucket, or negative.
var ErrBucketOutOfRange = errors.New("bucket out of range")

// TokenRange is the consecutive token ids a bucket owns, for example {First: 101, Count: 100}
// is tokens 101 to 200. The zero value holds no tokens.
type TokenRange struct {
	First int
	Count int
}

// Contains reports whether tokenID is in the range.
func (r TokenRange) Contains(tokenID int) bool {
	return tokenID >= r.First && tokenID < r.First+r.Count
}

// Last returns the range's last token id. Meaningless for an empty range.
func (r TokenRange) Last() int {
	return r.First + r.Count - 1
}

// NumBuckets returns how many buckets a semaphore is split into: ceil(size / bucketSize).
func NumBuckets(size, bucketSize int) (int, error) {
	if size < 1 {
		return 0, fmt.Errorf("size must be positive, got %d", size)
	}
	if bucketSize < 1 {
		return 0, fmt.Errorf("bucketSize must be positive, got %d", bucketSize)
	}
	return (size + bucketSize - 1) / bucketSize, nil
}

// BucketTokenRange returns the token ids owned by bucket: bucketSize ids starting at
// bucket*bucketSize+1, with the last bucket ending at size.
func BucketTokenRange(size, bucketSize, bucket int) (TokenRange, error) {
	numBuckets, err := NumBuckets(size, bucketSize)
	if err != nil {
		return TokenRange{}, err
	}
	if bucket < 0 || bucket >= numBuckets {
		return TokenRange{}, fmt.Errorf("bucket %d is not in [0, %d): %w", bucket, numBuckets, ErrBucketOutOfRange)
	}
	return TokenRange{
		First: bucket*bucketSize + 1,
		// The last bucket holds what is left, which may be less than bucketSize.
		Count: min(bucketSize, size-bucket*bucketSize),
	}, nil
}
