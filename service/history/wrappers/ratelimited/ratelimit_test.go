// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

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

package ratelimited

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/service/history/workflowcache"
)

func TestAllowWfID(t *testing.T) {
	tests := []struct {
		workflowIDCacheAllow bool
		expected             bool
	}{
		{
			workflowIDCacheAllow: true,
			expected:             true,
		},
		{
			workflowIDCacheAllow: false,
			expected:             false,
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("workflowIDCacheAllow: %t", tt.workflowIDCacheAllow), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			workflowIDCacheMock := workflowcache.NewMockWFCache(ctrl)
			workflowIDCacheMock.EXPECT().AllowExternal(testDomainID, testWorkflowID).Return(tt.workflowIDCacheAllow).Times(1)

			h := &historyHandler{
				workflowIDCache: workflowIDCacheMock,
				logger:          log.NewNoop(),
			}

			got := h.allowWfID(testDomainID, testWorkflowID)

			assert.Equal(t, tt.expected, got)
		})
	}
}
