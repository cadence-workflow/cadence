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

package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

func TestValidateFailoverDomainRequest(t *testing.T) {
	testCases := []struct {
		name          string
		req           *types.FailoverDomainRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "valid request",
			req: &types.FailoverDomainRequest{
				DomainName:              "test-domain",
				DomainActiveClusterName: common.StringPtr("cluster2"),
				Reason:                  "Planned maintenance on cluster1",
			},
			expectError: false,
		},
		{
			name:          "nil request",
			req:           nil,
			expectError:   true,
			expectedError: "Request is nil.",
		},
		{
			name: "empty domain name",
			req: &types.FailoverDomainRequest{
				DomainActiveClusterName: common.StringPtr("cluster2"),
				Reason:                  "Planned maintenance",
			},
			expectError:   true,
			expectedError: "Domain not set on request.",
		},
		{
			name: "missing cluster info",
			req: &types.FailoverDomainRequest{
				DomainName: "test-domain",
				Reason:     "Planned maintenance",
			},
			expectError:   true,
			expectedError: "DomainActiveClusterName or ActiveClusters must be provided to failover the domain",
		},
		{
			name: "missing reason",
			req: &types.FailoverDomainRequest{
				DomainName:              "test-domain",
				DomainActiveClusterName: common.StringPtr("cluster2"),
			},
			expectError:   true,
			expectedError: "Reason must be provided for domain failover",
		},
		{
			name: "empty reason",
			req: &types.FailoverDomainRequest{
				DomainName:              "test-domain",
				DomainActiveClusterName: common.StringPtr("cluster2"),
				Reason:                  "",
			},
			expectError:   true,
			expectedError: "Reason must be provided for domain failover",
		},
		// TODO: Fix ActiveClusters test case structure
		// {
		// 	name: "valid request with active clusters",
		// 	req: &types.FailoverDomainRequest{
		// 		DomainName: "test-domain",
		// 		ActiveClusters: &types.ActiveClusters{
		// 			AttributeScopes: []string{"cluster1", "cluster2"},
		// 		},
		// 		Reason: "Load balancing across clusters",
		// 	},
		// 	expectError: false,
		// },
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			v, _ := setupMocksForRequestValidator(t)

			err := v.ValidateFailoverDomainRequest(context.Background(), tc.req)
			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
