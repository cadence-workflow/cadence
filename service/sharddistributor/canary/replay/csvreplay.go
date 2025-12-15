package replay

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/uber/cadence/common/clock"
)

const (
	_timestampLayout = "2006-01-02 15:04:05"
)

// LoadProvider returns the current replayed load for a shard ID.
type LoadProvider interface {
	LoadForShard(shardID string) (float64, bool)
	ShardCount() int
	CurrentTimestamp() time.Time
}

// CSVReplay replays per-shard loads over time from a CSV where each row is:
// timestamp, load[0], load[1], ..., load[N-1]
//
// The current row is selected based on wall clock time since process start, mapped onto the
// CSV timeline (optionally sped up by Speed). This makes replays deterministic within a process.
type CSVReplay struct {
	timeSource clock.TimeSource
	speed      float64

	startWall time.Time
	startCSV  time.Time

	timestamps []time.Time
	loads      [][]float64
	shardCount int
}

func NewCSVReplayFromFile(timeSource clock.TimeSource, path string, speed float64) (*CSVReplay, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open csv: %w", err)
	}
	defer f.Close()

	return NewCSVReplayFromReader(timeSource, f, speed)
}

func NewCSVReplayFromReader(timeSource clock.TimeSource, r io.Reader, speed float64) (*CSVReplay, error) {
	if timeSource == nil {
		return nil, fmt.Errorf("timeSource is required")
	}
	if speed <= 0 {
		speed = 1.0
	}

	reader := csv.NewReader(bufio.NewReader(r))
	reader.TrimLeadingSpace = true
	reader.ReuseRecord = true

	var timestamps []time.Time
	var loads [][]float64
	shardCount := -1

	for rowIdx := 0; ; rowIdx++ {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("read csv row %d: %w", rowIdx, err)
		}
		if len(record) < 2 {
			return nil, fmt.Errorf("row %d: expected timestamp + at least 1 shard load", rowIdx)
		}

		ts, err := parseCSVTimestamp(record[0])
		if err != nil {
			return nil, fmt.Errorf("row %d: parse timestamp %q: %w", rowIdx, record[0], err)
		}

		rowShardCount := len(record) - 1
		if shardCount == -1 {
			shardCount = rowShardCount
		}
		if rowShardCount != shardCount {
			return nil, fmt.Errorf("row %d: expected %d shard loads, got %d", rowIdx, shardCount, rowShardCount)
		}

		rowLoads := make([]float64, shardCount)
		for i := 0; i < shardCount; i++ {
			v, err := strconv.ParseFloat(strings.TrimSpace(record[i+1]), 64)
			if err != nil {
				return nil, fmt.Errorf("row %d: parse load col %d: %w", rowIdx, i+1, err)
			}
			rowLoads[i] = v
		}

		timestamps = append(timestamps, ts)
		loads = append(loads, rowLoads)
	}

	if shardCount <= 0 || len(timestamps) == 0 {
		return nil, fmt.Errorf("csv contains no data rows")
	}

	startWall := timeSource.Now().UTC()
	return &CSVReplay{
		timeSource: timeSource,
		speed:      speed,
		startWall:  startWall,
		startCSV:   timestamps[0],
		timestamps: timestamps,
		loads:      loads,
		shardCount: shardCount,
	}, nil
}

func (r *CSVReplay) ShardCount() int {
	return r.shardCount
}

func (r *CSVReplay) LoadForShard(shardID string) (float64, bool) {
	shardIndex, err := strconv.Atoi(shardID)
	if err != nil || shardIndex < 0 || shardIndex >= r.shardCount {
		return 0, false
	}

	rowIdx := r.currentRowIndex(r.timeSource.Now().UTC())
	return r.loads[rowIdx][shardIndex], true
}

func (r *CSVReplay) CurrentTimestamp() time.Time {
	rowIdx := r.currentRowIndex(r.timeSource.Now().UTC())
	return r.timestamps[rowIdx]
}

func (r *CSVReplay) currentRowIndex(now time.Time) int {
	elapsed := now.Sub(r.startWall)
	virtual := r.startCSV.Add(time.Duration(float64(elapsed) * r.speed))

	// Find the last row timestamp <= virtual.
	idx := sort.Search(len(r.timestamps), func(i int) bool {
		return r.timestamps[i].After(virtual)
	}) - 1

	if idx < 0 {
		return 0
	}
	if idx >= len(r.timestamps) {
		return len(r.timestamps) - 1
	}
	return idx
}

func parseCSVTimestamp(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	// Some CSV exporters include a UTF-8 BOM at the beginning of the first field.
	s = strings.TrimPrefix(s, "\ufeff")

	if ts, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return ts.UTC(), nil
	}
	if ts, err := time.ParseInLocation(_timestampLayout, s, time.UTC); err == nil {
		return ts.UTC(), nil
	}

	return time.Time{}, fmt.Errorf("unsupported timestamp format")
}
