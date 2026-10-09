package semaphore

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNumBuckets(t *testing.T) {
	tests := []struct {
		name          string
		semaphoreSize int
		bucketSize    int
		want          int
		wantErr       bool
	}{
		{name: "smaller than one bucket", semaphoreSize: 5, bucketSize: 100, want: 1},
		{name: "exact multiple", semaphoreSize: 200, bucketSize: 100, want: 2},
		{name: "rounds up", semaphoreSize: 201, bucketSize: 100, want: 3},
		{name: "bucket size one", semaphoreSize: 7, bucketSize: 1, want: 7},
		{name: "zero semaphore size", semaphoreSize: 0, bucketSize: 100, wantErr: true},
		{name: "zero bucket size", semaphoreSize: 10, bucketSize: 0, wantErr: true},
		{name: "negative bucket size", semaphoreSize: 10, bucketSize: -1, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NumBuckets(tt.semaphoreSize, tt.bucketSize)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBucketTokenRange(t *testing.T) {
	tests := []struct {
		name                    string
		semaphoreSize           int
		bucketSize              int
		bucket                  int
		wantFirstToken          int
		wantLastToken           int
		wantErr                 bool
		wantErrBucketOutOfRange bool
	}{
		{name: "one bucket smaller than bucket size", semaphoreSize: 5, bucketSize: 100, bucket: 0, wantFirstToken: 1, wantLastToken: 5},
		{name: "first of two full buckets", semaphoreSize: 200, bucketSize: 100, bucket: 0, wantFirstToken: 1, wantLastToken: 100},
		{name: "second of two full buckets", semaphoreSize: 200, bucketSize: 100, bucket: 1, wantFirstToken: 101, wantLastToken: 200},
		{name: "last bucket is partial", semaphoreSize: 201, bucketSize: 100, bucket: 2, wantFirstToken: 201, wantLastToken: 201},
		{name: "bucket size one", semaphoreSize: 7, bucketSize: 1, bucket: 6, wantFirstToken: 7, wantLastToken: 7},
		{name: "bucket past the last one", semaphoreSize: 200, bucketSize: 100, bucket: 2, wantErr: true, wantErrBucketOutOfRange: true},
		{name: "negative bucket", semaphoreSize: 200, bucketSize: 100, bucket: -1, wantErr: true, wantErrBucketOutOfRange: true},
		{name: "zero semaphore size", semaphoreSize: 0, bucketSize: 100, bucket: 0, wantErr: true},
		{name: "zero bucket size", semaphoreSize: 10, bucketSize: 0, bucket: 0, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := BucketTokenRange(tt.semaphoreSize, tt.bucketSize, tt.bucket)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tt.wantErrBucketOutOfRange, errors.Is(err, ErrBucketOutOfRange))
				assert.Equal(t, TokenRange{}, r, "an error carries no range")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantFirstToken, r.First)
			assert.Equal(t, tt.wantLastToken, r.Last())
		})
	}
}

// Tests that every token from 1 to the semaphore's size belongs to exactly one bucket, across all
// the buckets NumBuckets reports. An overlap would let two owners hold the same token id in
// different buckets, admitting more holders than the semaphore's size; a gap would leave a token no
// host ever grants.
func TestBucketTokenRangeCoversEveryTokenOnce(t *testing.T) {
	for _, tc := range []struct{ semaphoreSize, bucketSize int }{{1, 1}, {7, 3}, {200, 100}, {201, 100}, {1000, 250}} {
		numBuckets, err := NumBuckets(tc.semaphoreSize, tc.bucketSize)
		require.NoError(t, err)
		seen := make(map[int]int, tc.semaphoreSize)
		for b := 0; b < numBuckets; b++ {
			r, err := BucketTokenRange(tc.semaphoreSize, tc.bucketSize, b)
			require.NoError(t, err)
			for id := r.First; r.Contains(id); id++ {
				seen[id]++
			}
		}
		assert.Len(t, seen, tc.semaphoreSize, "semaphoreSize %d, bucketSize %d", tc.semaphoreSize, tc.bucketSize)
		for id := 1; id <= tc.semaphoreSize; id++ {
			assert.Equal(t, 1, seen[id], "token %d, semaphoreSize %d, bucketSize %d", id, tc.semaphoreSize, tc.bucketSize)
		}
	}
}
