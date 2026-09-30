package semaphore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pins OwnerIDToBucket's output. If this fails, the hash changed, and releases for slots already
// acquired would go to the wrong bucket.
func TestOwnerIDToBucket(t *testing.T) {
	tests := []struct {
		ownerID    string
		numBuckets int
		want       int
	}{
		{"4:wf-1:run-abc:1", 1, 0},
		{"4:wf-1:run-abc:1", 4, 3},
		{"4:wf-1:run-abc:1", 8, 7},
		{"4:wf-1:run-abc:1", 100, 55},
		{"4:wf-2:run-def:1", 100, 3},
		{"4:wf-1:run-abc:2", 100, 7},
		{"8:order:42:3f1c9b2e-8d4a-4e7b-9c1a-2b5d6e7f8a90:17", 4, 1},
		{"8:order:42:3f1c9b2e-8d4a-4e7b-9c1a-2b5d6e7f8a90:17", 8, 1},
		{"8:order:42:3f1c9b2e-8d4a-4e7b-9c1a-2b5d6e7f8a90:17", 100, 25},
	}
	for _, tt := range tests {
		t.Run(tt.ownerID, func(t *testing.T) {
			got, err := OwnerIDToBucket(tt.ownerID, tt.numBuckets)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestOwnerIDToBucketRejectsNonPositiveN(t *testing.T) {
	for _, n := range []int{0, -1} {
		_, err := OwnerIDToBucket("4:wf-1:run-abc:1", n)
		assert.Error(t, err, "numBuckets %d", n)
	}
}

func TestNumBuckets(t *testing.T) {
	tests := []struct {
		name       string
		size       int
		bucketSize int
		want       int
		wantErr    bool
	}{
		{name: "smaller than one bucket", size: 5, bucketSize: 100, want: 1},
		{name: "exact multiple", size: 200, bucketSize: 100, want: 2},
		{name: "rounds up", size: 201, bucketSize: 100, want: 3},
		{name: "bucket size one", size: 7, bucketSize: 1, want: 7},
		{name: "zero size", size: 0, bucketSize: 100, wantErr: true},
		{name: "zero bucket size", size: 10, bucketSize: 0, wantErr: true},
		{name: "negative bucket size", size: 10, bucketSize: -1, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NumBuckets(tt.size, tt.bucketSize)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRingKey(t *testing.T) {
	assert.Equal(t, "domain-1_sem-1_0", RingKey("domain-1", "sem-1", 0))
	assert.Equal(t, "domain-1_sem-1_12", RingKey("domain-1", "sem-1", 12))
}
