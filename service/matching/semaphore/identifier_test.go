package semaphore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests that NewIdentifier rejects the values persistence would reject anyway, so a
// misconfigured bucket fails here rather than on its first grant. Bucket 0 is valid.
func TestNewIdentifier(t *testing.T) {
	tests := []struct {
		name          string
		domainID      string
		semaphoreName string
		bucket        int
		wantErr       bool
		wantString    string
	}{
		{
			name:          "valid",
			domainID:      "domain-1",
			semaphoreName: "sem-1",
			bucket:        2,
			wantString:    "domain-1/sem-1/2",
		},
		{
			name:          "bucket zero is valid",
			domainID:      "domain-1",
			semaphoreName: "sem-1",
			bucket:        0,
			wantString:    "domain-1/sem-1/0",
		},
		{name: "empty domain id", semaphoreName: "sem-1", wantErr: true},
		{name: "empty semaphore name", domainID: "domain-1", wantErr: true},
		{name: "negative bucket", domainID: "domain-1", semaphoreName: "sem-1", bucket: -1, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, err := NewIdentifier(tc.domainID, tc.semaphoreName, tc.bucket)
			if tc.wantErr {
				assert.ErrorIs(t, err, ErrInvalidRequest, "a bad identifier is never worth retrying")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantString, id.String())
		})
	}
}

// Tests that Identifier works as a map key. Buckets are looked up by value, so the struct has to
// stay comparable -- adding a slice or map field would break this at compile time, which is the
// point.
func TestIdentifierIsUsableAsAMapKey(t *testing.T) {
	a, err := NewIdentifier("domain-1", "sem-1", 0)
	require.NoError(t, err)
	b, err := NewIdentifier("domain-1", "sem-1", 1)
	require.NoError(t, err)

	buckets := map[Identifier]string{a: "first", b: "second"}
	same, err := NewIdentifier("domain-1", "sem-1", 0)
	require.NoError(t, err)
	assert.Equal(t, "first", buckets[same])
	assert.Len(t, buckets, 2)
}
