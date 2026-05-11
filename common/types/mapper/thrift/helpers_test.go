// Copyright (c) 2017-2022 Uber Technologies Inc.

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

package thrift

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTimeToNano(t *testing.T) {
	unixTime := time.Unix(1, 1)
	result := timeToNano(&unixTime)
	assert.Equal(t, int64(1000000001), *result)
}

func TestTimeToNanoNil(t *testing.T) {
	result := timeToNano(nil)
	assert.Nil(t, result)
}

func TestNanoToTime(t *testing.T) {
	nanos := int64(1000000001)
	result := nanoToTime(&nanos)
	assert.True(t, time.Unix(1, 1).Equal(*result))
}

func TestNanoToTimeNil(t *testing.T) {
	result := nanoToTime(nil)
	assert.Nil(t, result)
}

func TestTimeValToNano(t *testing.T) {
	ts := time.Unix(1, 500000000).UTC()
	result := timeValToNano(ts)
	assert.Equal(t, int64(1500000000), *result)
}

func TestTimeValToNanoZero(t *testing.T) {
	result := timeValToNano(time.Time{})
	assert.Nil(t, result)
}

func TestNanoToTimeVal(t *testing.T) {
	nanos := int64(1000000001)
	result := nanoToTimeVal(&nanos)
	assert.True(t, time.Unix(1, 1).UTC().Equal(result))
}

func TestNanoToTimeValNil(t *testing.T) {
	result := nanoToTimeVal(nil)
	assert.True(t, result.IsZero())
}

func TestDurationToSecondsI32(t *testing.T) {
	result := durationToSeconds(90 * time.Second)
	assert.Equal(t, int32(90), *result)
}

func TestDurationToSecondsI32Zero(t *testing.T) {
	result := durationToSeconds(0)
	assert.Nil(t, result)
}

func TestSecondsI32ToDuration(t *testing.T) {
	s := int32(90)
	result := secondsToDuration(&s)
	assert.Equal(t, 90*time.Second, result)
}

func TestSecondsI32ToDurationNil(t *testing.T) {
	result := secondsToDuration(nil)
	assert.Equal(t, time.Duration(0), result)
}
