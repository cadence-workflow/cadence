// Copyright (c) 2021 Uber Technologies Inc.
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
	"strconv"
	"time"

	gogo "github.com/gogo/protobuf/types"

	"github.com/uber/cadence/common"
)

func fromDoubleValue(v *float64) *gogo.DoubleValue {
	if v == nil {
		return nil
	}
	return &gogo.DoubleValue{Value: *v}
}

func toDoubleValue(v *gogo.DoubleValue) *float64 {
	if v == nil {
		return nil
	}
	return common.Float64Ptr(v.Value)
}

func fromInt64Value(v *int64) *gogo.Int64Value {
	if v == nil {
		return nil
	}
	return &gogo.Int64Value{Value: *v}
}

func toInt64Value(v *gogo.Int64Value) *int64 {
	if v == nil {
		return nil
	}
	return common.Int64Ptr(v.Value)
}

func unixNanoToTime(t *int64) *gogo.Timestamp {
	if t == nil {
		return nil
	}
	time, err := gogo.TimestampProto(time.Unix(0, *t))
	if err != nil {
		panic(err)
	}
	return time
}

func timeToUnixNano(t *gogo.Timestamp) *int64 {
	if t == nil {
		return nil
	}
	timestamp, err := gogo.TimestampFromProto(t)
	if err != nil {
		panic(err)
	}
	return common.Int64Ptr(timestamp.UnixNano())
}

func timeToTimestamp(t *time.Time) *gogo.Timestamp {
	if t == nil {
		return nil
	}

	nanos := t.UnixNano()
	return unixNanoToTime(&nanos)
}

func timestampToTime(t *gogo.Timestamp) *time.Time {
	if t == nil {
		return nil
	}

	time := time.Unix(t.GetSeconds(), int64(t.GetNanos()))
	return &time
}

func daysToDuration(d *int32) *gogo.Duration {
	if d == nil {
		return nil
	}
	return gogo.DurationProto(common.DaysToDuration(*d))
}

func durationToDays(d *gogo.Duration) *int32 {
	if d == nil {
		return nil
	}
	duration, err := gogo.DurationFromProto(d)
	if err != nil {
		panic(err)
	}
	return common.Int32Ptr(common.DurationToDays(duration))
}

func secondsToDuration(d *int32) *gogo.Duration {
	if d == nil {
		return nil
	}
	return gogo.DurationProto(common.SecondsToDuration(int64(*d)))
}

func durationToSeconds(d *gogo.Duration) *int32 {
	if d == nil {
		return nil
	}
	duration, err := gogo.DurationFromProto(d)
	if err != nil {
		panic(err)
	}
	return common.Int32Ptr(int32(common.DurationToSeconds(duration)))
}

func int32To64(v *int32) *int64 {
	if v == nil {
		return nil
	}
	return common.Int64Ptr(int64(*v))
}

func int64To32(v *int64) *int32 {
	if v == nil {
		return nil
	}
	return common.Int32Ptr(int32(*v))
}

func stringToInt32(s string) int32 {
	i, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	return int32(i)
}

func int32ToString(i int32) string {
	s := strconv.Itoa(int(i))
	return s
}

type fieldSet map[string]struct{}

func newFieldSet(mask *gogo.FieldMask) fieldSet {
	if mask == nil {
		return nil
	}
	fs := map[string]struct{}{}
	for _, field := range mask.Paths {
		fs[field] = struct{}{}
	}
	return fs
}

func (fs fieldSet) isSet(field string) bool {
	if fs == nil {
		return true
	}
	_, ok := fs[field]
	return ok
}

func newFieldMask(fields []string) *gogo.FieldMask {
	return &gogo.FieldMask{Paths: fields}
}
