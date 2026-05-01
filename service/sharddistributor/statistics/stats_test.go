package statistics

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/clock"
)

func TestCalculateSmoothedLoad(t *testing.T) {
	tests := []struct {
		name      string
		prev      float64
		current   float64
		setupTime func(ts clock.MockedTimeSource) (lastUpdate, now time.Time)
		want      float64
		wantErr   bool
	}{
		{
			name:    "first update returns current",
			prev:    0,
			current: 10.0,
			setupTime: func(ts clock.MockedTimeSource) (time.Time, time.Time) {
				return time.Time{}, ts.Now()
			},
			want:    10.0,
			wantErr: false,
		},
		{
			name:    "now before lastUpdate returns current",
			prev:    100.0,
			current: 10.0,
			setupTime: func(ts clock.MockedTimeSource) (time.Time, time.Time) {
				now := ts.Now()
				return now, now.Add(-1 * time.Second)
			},
			want:    10.0,
			wantErr: false,
		},
		{
			name:    "normal smoothing - 1 tau elapsed",
			prev:    10.0,
			current: 20.0,
			setupTime: func(ts clock.MockedTimeSource) (time.Time, time.Time) {
				lastUpdate := ts.Now()
				ts.Advance(30 * time.Second)
				return lastUpdate, ts.Now()
			},
			// dt = 30s, tau = 30s -> dt/tau = 1
			// alpha = 1 - e^-1 = 1 - 0.367879 = 0.632121
			// result = (1 - 0.632121)*10 + 0.632121*20 = 0.367879*10 + 0.632121*20 = 3.67879 + 12.64242 = 16.32121
			want:    16.32120558828557,
			wantErr: false,
		},
		{
			name:    "very small dt returns close to prev",
			prev:    10.0,
			current: 20.0,
			setupTime: func(ts clock.MockedTimeSource) (time.Time, time.Time) {
				lastUpdate := ts.Now()
				ts.Advance(100 * time.Millisecond)
				return lastUpdate, ts.Now()
			},
			wantErr: false,
		},
		{
			name:    "very large dt returns close to current",
			prev:    10.0,
			current: 20.0,
			setupTime: func(ts clock.MockedTimeSource) (time.Time, time.Time) {
				lastUpdate := ts.Now()
				ts.Advance(1 * time.Hour)
				return lastUpdate, ts.Now()
			},
			want:    20.0,
			wantErr: false,
		},
		{
			name:    "current is NaN returns error",
			prev:    10.0,
			current: math.NaN(),
			setupTime: func(ts clock.MockedTimeSource) (time.Time, time.Time) {
				return ts.Now().Add(-10 * time.Second), ts.Now()
			},
			wantErr: true,
		},
		{
			name:    "prev is NaN returns error",
			prev:    math.NaN(),
			current: 10.0,
			setupTime: func(ts clock.MockedTimeSource) (time.Time, time.Time) {
				return ts.Now().Add(-10 * time.Second), ts.Now()
			},
			wantErr: true,
		},
		{
			name:    "current is Inf returns error",
			prev:    10.0,
			current: math.Inf(1),
			setupTime: func(ts clock.MockedTimeSource) (time.Time, time.Time) {
				return ts.Now().Add(-10 * time.Second), ts.Now()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset time source for each test case to a known value
			ts := clock.NewMockedTimeSourceAt(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
			lastUpdate, now := tt.setupTime(ts)

			got, err := CalculateSmoothedLoad(tt.prev, tt.current, lastUpdate, now)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				switch tt.name {
				case "very small dt returns close to prev":
					assert.InDelta(t, tt.prev, got, 0.1)
				case "very large dt returns close to current":
					assert.InDelta(t, tt.current, got, 0.0001)
				default:
					assert.InDelta(t, tt.want, got, 1e-9)
				}
			}
		})
	}
}
