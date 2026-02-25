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
	apiv1 "github.com/uber/cadence-idl/go/proto/api/v1"
	"go.uber.org/yarpc/yarpctest"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/client"
	"github.com/uber/cadence/common/types"
)

func TestGetRequestedConsistencyLevelFromContext(t *testing.T) {
	tests := []struct {
		name         string
		featureFlags apiv1.FeatureFlags
		expected     types.QueryConsistencyLevel
	}{
		{
			name:         "empty feature flags",
			featureFlags: apiv1.FeatureFlags{},
			expected:     types.QueryConsistencyLevelEventual,
		},
		{
			name:         "auto forwarding disabled",
			featureFlags: apiv1.FeatureFlags{AutoforwardingEnabled: false},
			expected:     types.QueryConsistencyLevelEventual,
		},
		{
			name:         "autoforwarding enabled",
			featureFlags: apiv1.FeatureFlags{AutoforwardingEnabled: true},
			expected:     types.QueryConsistencyLevelStrong,
		},
		{
			name: "no autoforwarding field",
			featureFlags: apiv1.FeatureFlags{
				WorkflowExecutionAlreadyCompletedErrorEnabled: true,
			},
			expected: types.QueryConsistencyLevelEventual,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := yarpctest.ContextWithCall(context.Background(), &yarpctest.Call{
				Headers: map[string]string{common.ClientFeatureFlagsHeaderName: client.FeatureFlagsHeader(tt.featureFlags)},
			})

			result := getRequestedConsistencyLevelFromContext(ctx)
			assert.Equal(t, tt.expected, result)
		})
	}
}
