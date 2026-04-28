// Copyright (c) 2022 Uber Technologies Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package proto

import (
	"testing"
	"time"

	gogo "github.com/gogo/protobuf/types"
	fuzz "github.com/google/gofuzz"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/types/mapper/testutils"
)

func TestTimeToTimestamp(t *testing.T) {
	testTime := time.Unix(10, 10)
	timestamp := timeToTimestamp(&testTime)
	assert.Equal(t, gogo.Timestamp{Seconds: 10, Nanos: 10}, *timestamp)
}

func TestTimeToTimestampNil(t *testing.T) {
	result := timeToTimestamp(nil)
	assert.Nil(t, result)
}

// SafeUnixNanoFuzzer constrains unix nanoseconds to [0, MaxSafeTimestampSeconds * 1e9)
// to prevent UnixNano() panicking for dates outside [year 1678, year 2262].
func SafeUnixNanoFuzzer(n *int64, c fuzz.Continue) {
	*n = c.Int63n(int64(testutils.MaxSafeTimestampSeconds) * int64(testutils.NanosecondsPerSecond))
}

// LocalTimeFuzzer generates local-timezone times because timestampToTime returns
// time.Unix() (local TZ), so UTC input would fail reflect.DeepEqual after round-trip.
func LocalTimeFuzzer(t *time.Time, c fuzz.Continue) {
	*t = time.Unix(c.Int63n(testutils.MaxSafeTimestampSeconds), c.Int63n(int64(testutils.NanosecondsPerSecond)))
}

// SafeDaysFuzzer constrains days to [-100000, 100000) to prevent int64 overflow:
// DaysToDuration multiplies by 86400e9 ns/day, which overflows for large int32 values.
func SafeDaysFuzzer(d *int32, c fuzz.Continue) {
	const maxSafeDays = 100000
	*d = int32(c.Intn(2*maxSafeDays)) - maxSafeDays
}

func TestFromDoubleValueFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, fromDoubleValue, toDoubleValue)
}

func TestFromInt64ValueFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, fromInt64Value, toInt64Value)
}

func TestUnixNanoToTimeFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, unixNanoToTime, timeToUnixNano,
		testutils.WithCustomFuncs(SafeUnixNanoFuzzer),
	)
}

func TestTimeToTimestampFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, timeToTimestamp, timestampToTime,
		testutils.WithCustomFuncs(LocalTimeFuzzer),
	)
}

func TestDurationToDurationProtoFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, durationToDurationProto, durationProtoToDuration)
}

func TestDaysToDurationFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, daysToDuration, durationToDays,
		testutils.WithCustomFuncs(SafeDaysFuzzer),
	)
}

func TestSecondsToDurationFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, secondsToDuration, durationToSeconds)
}

func TestInt32To64Fuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, int32To64, int64To32)
}

func TestInt32ToStringFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, int32ToString, stringToInt32)
}
