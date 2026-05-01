package etcdtypes

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTimeToTime verifies ToTime returns the original time.Time value.
func TestTimeToTime(t *testing.T) {
	now := time.Now()
	require.Equal(t, now, Time(now).ToTime())
}

func TestTimeToTimePtr_NilInput_ReturnsNil(t *testing.T) {
	var input *time.Time
	result := ToTimePtr(input)
	require.Nil(t, result)
}

func TestTimeToTimePtr(t *testing.T) {
	now := time.Now()
	result := ToTimePtr(&now)
	require.NotNil(t, result)
	require.Equal(t, Time(now), *result)
}

func TestTimeToTimePtr_Nil(t *testing.T) {
	result := ToTimePtr(nil)
	require.Nil(t, result)
}

func TestTimeMarshalJSON(t *testing.T) {
	for name, c := range map[string]struct {
		input        Time
		expectOutput string
		expectErr    string
	}{
		"zero time": {
			input:        Time(time.Time{}),
			expectOutput: `"0001-01-01T00:00:00Z"`,
			expectErr:    "",
		},
		"utc time with nanos": {
			input:        Time(time.Date(2025, 11, 18, 12, 0, 0, 123456789, time.UTC)),
			expectOutput: `"2025-11-18T12:00:00.123456789Z"`,
			expectErr:    "",
		},
		"non-UTC time": {
			input:        Time(time.Date(2025, 11, 18, 1, 2, 3, 456789012, time.FixedZone("X", -2*3600))),
			expectOutput: `"2025-11-18T03:02:03.456789012Z"`,
			expectErr:    "",
		},
	} {
		t.Run(name, func(t *testing.T) {
			output, err := c.input.MarshalJSON()
			require.Equal(t, c.expectOutput, string(output))
			if c.expectErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.expectErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTimeUnmarshalJSON(t *testing.T) {
	for name, c := range map[string]struct {
		input      []byte
		expectTime Time
		expectErr  string
	}{
		"zero time": {
			input:      []byte(`"0001-01-01T00:00:00Z"`),
			expectTime: Time(time.Time{}),
			expectErr:  "",
		},
		"valid RFC3339Nano": {
			input:      []byte(`"2025-11-18T12:00:00.123456789Z"`),
			expectTime: Time(time.Date(2025, 11, 18, 12, 0, 0, 123456789, time.UTC)),
			expectErr:  "",
		},
		"non-UTC time": {
			input:      []byte(`"2025-11-17T23:02:03.456789012Z"`),
			expectTime: Time(time.Date(2025, 11, 17, 23, 2, 3, 456789012, time.UTC)),
			expectErr:  "",
		},
		"invalid format": {
			input:      []byte(`"not-a-time"`),
			expectTime: Time{},
			expectErr:  "cannot parse",
		},
		"empty string": {
			input:      []byte(`""`),
			expectTime: Time{},
			expectErr:  "cannot parse",
		},
	} {
		t.Run(name, func(t *testing.T) {
			var tim Time
			err := json.Unmarshal(c.input, &tim)
			require.Equal(t, c.expectTime, tim)
			if c.expectErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.expectErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestParseTimeSuccess checks valid parsing scenarios.
func TestParseTimeSuccess(t *testing.T) {
	for name, c := range map[string]struct {
		input    string
		expected time.Time
	}{
		"with nanos": {
			input:    "2025-11-18T12:00:00.123456789Z",
			expected: time.Date(2025, 11, 18, 12, 0, 0, 123456789, time.UTC),
		},
		"without nanos": {
			input:    "2000-01-01T00:00:00Z",
			expected: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		"non-UTC time": {
			input:    "2025-11-18T01:02:03.456789012-02:00",
			expected: time.Date(2025, 11, 18, 3, 2, 3, 456789012, time.UTC),
		},
	} {
		t.Run(name, func(t *testing.T) {
			parsed, err := ParseTime(c.input)
			require.NoError(t, err, "unexpected error for input %q", c.input)
			assert.Equal(t, c.expected, parsed)
			assert.Equal(t, c.expected.UnixNano(), parsed.UnixNano(), "ParseTime mismatch got %v want %v", parsed, c.expected)
			assert.Equal(t, time.UTC, parsed.Location(), "ParseTime location mismatch got %v want %v", parsed.Location(), time.UTC)
		})
	}
}

func TestParseTime_Failures(t *testing.T) {
	for name, c := range map[string]struct {
		input string
	}{
		"not-a-time":    {input: "not-a-time"},
		"empty string":  {input: ""},
		"invalid month": {input: "2025-13-01T00:00:00Z"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseTime(c.input)
			assert.Error(t, err, "[%s] expected error for input %q", name, c.input)
		})
	}
}

func TestFormatTime(t *testing.T) {
	for name, c := range map[string]struct {
		input    time.Time
		expected string
	}{
		"non-UTC with nanos": {
			input:    time.Date(2025, 11, 17, 21, 2, 3, 456789012, time.FixedZone("X", -2*3600)),
			expected: "2025-11-17T23:02:03.456789012Z",
		},
		"non-UTC without nanos": {
			input:    time.Date(2000, 1, 1, 0, 0, 0, 0, time.FixedZone("Y", +5*3600)),
			expected: "1999-12-31T19:00:00Z",
		},
		"zero time": {
			input:    time.Time{},
			expected: "0001-01-01T00:00:00Z",
		},
	} {
		t.Run(name, func(t *testing.T) {
			str := FormatTime(c.input)
			assert.Equal(t, c.expected, str)
		})
	}
}

// TestTime_JSONMarshalling ensures Marshal and Unmarshal retains nanosecond precision.
func TestTime_JSONMarshalling(t *testing.T) {
	for name, c := range map[string]struct {
		input time.Time
	}{
		"zero time": {
			input: time.Time{},
		},
		"start of 2025": {
			input: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		"now with nanos": {
			input: time.Now().UTC(),
		},
		"max nanos": {
			input: time.Date(2025, 6, 30, 23, 59, 59, 999999999, time.UTC),
		},
	} {
		t.Run(name, func(t *testing.T) {
			wrapped := Time(c.input)
			b, err := json.Marshal(wrapped)
			require.NoError(t, err, "marshal error")

			var back Time
			require.NoError(t, json.Unmarshal(b, &back), "unmarshal error")
			assert.Equal(t, c.input.UTC().UnixNano(), time.Time(back).UnixNano(), "round-trip lost precision: got %v want %v", time.Time(back), c.input.UTC())
		})
	}
}
