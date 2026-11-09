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

func TestRingKey(t *testing.T) {
	assert.Equal(t, "domain-1_sem-1_0", RingKey("domain-1", "sem-1", 0))
	assert.Equal(t, "domain-1_sem-1_12", RingKey("domain-1", "sem-1", 12))
}
