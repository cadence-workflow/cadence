// Copyright (c) 2022 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package permember

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/membership"
)

func Test_PerMember(t *testing.T) {
	ctrl := gomock.NewController(t)
	resolver := membership.NewMockResolver(ctrl)
	resolver.EXPECT().MemberCount("A").Return(10, nil).MinTimes(1)
	resolver.EXPECT().MemberCount("X").Return(0, assert.AnError).MinTimes(1)
	resolver.EXPECT().MemberCount("Y").Return(0, nil).MinTimes(1)

	// Invalid service - fallback to instanceRPS
	assert.Equal(t, 3.0, PerMember("X", 20.0, 3.0, resolver))

	// Invalid member count - fallback to instanceRPS
	assert.Equal(t, 3.0, PerMember("Y", 20.0, 3.0, resolver))

	// GlobalRPS not provided - fallback to instanceRPS
	assert.Equal(t, 3.0, PerMember("A", 0, 3.0, resolver))

	// Calculate average per member RPS (prefer averaged global - lower)
	assert.Equal(t, 2.0, PerMember("A", 20.0, 3.0, resolver))

	// Calculate average per member RPS (prefer instanceRPS - lower)
	assert.Equal(t, 3.0, PerMember("A", 100.0, 3.0, resolver))
}

func Test_PerMemberFactory(t *testing.T) {
	ctrl := gomock.NewController(t)
	resolver := membership.NewMockResolver(ctrl)
	resolver.EXPECT().MemberCount("A").Return(10, nil).MinTimes(1)

	factory := NewPerMemberDynamicRateLimiterFactory(
		"A",
		func(string) int { return 20 },
		func(string) int { return 3 },
		resolver,
	)

	limiter := factory.GetLimiter("TestDomainName")

	// The limit is 20 and there are 10 instances, so the per member limit is 2
	assert.Equal(t, true, limiter.Allow())
	assert.Equal(t, true, limiter.Allow())
	assert.Equal(t, false, limiter.Allow())
}
