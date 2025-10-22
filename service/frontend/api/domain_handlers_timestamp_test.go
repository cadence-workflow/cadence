package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestTimestampConversion verifies that the timestamp conversion logic
// used in GetFailoverEvent maintains precision correctly
func TestTimestampConversion(t *testing.T) {
	// Simulate a timestamp stored in Cassandra (millisecond precision)
	originalTime := time.Date(2025, 10, 22, 18, 29, 23, 874000000, time.UTC)

	// Step 1: Convert to milliseconds (as done in ListFailoverHistory)
	createdTimeMs := originalTime.UnixNano() / int64(time.Millisecond)
	t.Logf("Original time: %v", originalTime)
	t.Logf("Original nanos: %d", originalTime.UnixNano())
	t.Logf("Converted to ms: %d", createdTimeMs)

	// Step 2: Convert back to time.Time (as done in GetFailoverEvent handler)
	reconstructedTime := time.Unix(0, createdTimeMs*int64(time.Millisecond))
	t.Logf("Reconstructed time: %v", reconstructedTime)
	t.Logf("Reconstructed nanos: %d", reconstructedTime.UnixNano())

	// Verify they are equal
	assert.Equal(t, originalTime.UnixNano(), reconstructedTime.UnixNano(),
		"Timestamps should be equal after round-trip conversion")
	assert.True(t, originalTime.Equal(reconstructedTime),
		"time.Time values should be equal")

	// Test with the exact timestamp from the issue
	// "2025-10-22T18:29:23.874Z"
	issueTime, err := time.Parse(time.RFC3339, "2025-10-22T18:29:23.874Z")
	assert.NoError(t, err)
	t.Logf("Issue timestamp: %v (nanos: %d)", issueTime, issueTime.UnixNano())

	// Convert to ms and back
	issueMs := issueTime.UnixNano() / int64(time.Millisecond)
	issueReconstructed := time.Unix(0, issueMs*int64(time.Millisecond))
	t.Logf("Issue reconstructed: %v (nanos: %d)", issueReconstructed, issueReconstructed.UnixNano())

	assert.True(t, issueTime.Equal(issueReconstructed),
		"Issue timestamp should survive round-trip")
}

// TestTimestampMillisecondPrecision verifies that timestamps maintain only millisecond precision
func TestTimestampMillisecondPrecision(t *testing.T) {
	// Create a time with sub-millisecond precision
	timeWithNanos := time.Date(2025, 10, 22, 18, 29, 23, 874123456, time.UTC)

	// Convert to ms and back (simulating Cassandra storage)
	ms := timeWithNanos.UnixNano() / int64(time.Millisecond)
	timeFromMs := time.Unix(0, ms*int64(time.Millisecond))

	t.Logf("Original: %v (nanos: %d)", timeWithNanos, timeWithNanos.UnixNano())
	t.Logf("After ms conversion: %v (nanos: %d)", timeFromMs, timeFromMs.UnixNano())

	// The millisecond portion should be preserved
	assert.Equal(t, int64(874), ms%1000, "Milliseconds should be 874")

	// Sub-millisecond precision should be lost
	assert.Equal(t, int64(0), timeFromMs.Nanosecond()%1000000,
		"Sub-millisecond precision should be lost")
}
