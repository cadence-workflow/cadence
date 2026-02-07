// Copyright (c) 2017 Uber Technologies, Inc.
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

package clusterredirection

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

func TestGetRequestedConsistencyLevelFromContext(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected types.QueryConsistencyLevel
	}{
		{
			name:     "no value in context returns eventual",
			ctx:      context.Background(),
			expected: types.QueryConsistencyLevelEventual,
		},
		{
			name:     "eventual level in context",
			ctx:      context.WithValue(context.Background(), common.QueryConsistencyLevelHeaderName, types.QueryConsistencyLevelEventual),
			expected: types.QueryConsistencyLevelEventual,
		},
		{
			name:     "strong level in context",
			ctx:      context.WithValue(context.Background(), common.QueryConsistencyLevelHeaderName, types.QueryConsistencyLevelStrong),
			expected: types.QueryConsistencyLevelStrong,
		},
		{
			name:     "wrong type in context returns eventual",
			ctx:      context.WithValue(context.Background(), common.QueryConsistencyLevelHeaderName, "strong"),
			expected: types.QueryConsistencyLevelEventual,
		},
		{
			name:     "nil context value returns eventual",
			ctx:      context.WithValue(context.Background(), common.QueryConsistencyLevelHeaderName, nil),
			expected: types.QueryConsistencyLevelEventual,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getRequestedConsistencyLevelFromContext(tt.ctx)
			assert.Equal(t, tt.expected, result)
		})
	}
}
