package replay

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/clock"
)

// TestCSVReplay_LoadForShard verifies shard loads advance as time advances.
func TestCSVReplay_LoadForShard(t *testing.T) {
	csvData := "2025-12-02 14:03:46,1.0,2.0\n2025-12-02 14:03:56,3.0,4.0\n"
	tmp, err := os.CreateTemp("", "csvreplay-*.csv")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(tmp.Name()) })
	_, err = tmp.WriteString(csvData)
	require.NoError(t, err)
	require.NoError(t, tmp.Close())

	start := time.Date(2025, 12, 2, 14, 3, 46, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(start)

	replay, err := NewCSVReplayFromFile(ts, tmp.Name(), 1.0)
	require.NoError(t, err)
	require.Equal(t, 2, replay.ShardCount())

	load0, ok := replay.LoadForShard("0")
	require.True(t, ok)
	require.Equal(t, 1.0, load0)

	load1, ok := replay.LoadForShard("1")
	require.True(t, ok)
	require.Equal(t, 2.0, load1)

	ts.Advance(10 * time.Second)

	load0, ok = replay.LoadForShard("0")
	require.True(t, ok)
	require.Equal(t, 3.0, load0)

	load1, ok = replay.LoadForShard("1")
	require.True(t, ok)
	require.Equal(t, 4.0, load1)
}

// TestCSVReplay_LoadForShard_BOMTimestamp verifies replay supports CSVs with a UTF-8 BOM prefix.
func TestCSVReplay_LoadForShard_BOMTimestamp(t *testing.T) {
	csvData := "\ufeff2025-12-02 14:03:46,1.0,2.0\n2025-12-02 14:03:56,3.0,4.0\n"
	tmp, err := os.CreateTemp("", "csvreplay-*.csv")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(tmp.Name()) })
	_, err = tmp.WriteString(csvData)
	require.NoError(t, err)
	require.NoError(t, tmp.Close())

	start := time.Date(2025, 12, 2, 14, 3, 46, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(start)

	replay, err := NewCSVReplayFromFile(ts, tmp.Name(), 1.0)
	require.NoError(t, err)

	load0, ok := replay.LoadForShard("0")
	require.True(t, ok)
	require.Equal(t, 1.0, load0)
}
