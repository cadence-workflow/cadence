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
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

func TestFailoverDomainValidation_WithoutReason_ExpectError(t *testing.T) {
	// This test verifies that when a failover is attempted without providing a reason,
	// the validation fails and returns an appropriate error.

	v, _ := setupMocksForRequestValidator(t)

	// Test case 1: Failover request WITHOUT a reason
	failoverRequestWithoutReason := &types.FailoverDomainRequest{
		DomainName:              "test-domain",
		DomainActiveClusterName: common.StringPtr("cluster2"),
		// Reason is intentionally missing
	}

	err := v.ValidateFailoverDomainRequest(context.Background(), failoverRequestWithoutReason)

	// Verify we get the expected error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Reason must be provided for domain failover")

	// Test case 2: Failover request with empty reason
	failoverRequestEmptyReason := &types.FailoverDomainRequest{
		DomainName:              "test-domain",
		DomainActiveClusterName: common.StringPtr("cluster2"),
		Reason:                  "", // Empty reason
	}

	err = v.ValidateFailoverDomainRequest(context.Background(), failoverRequestEmptyReason)

	// Verify we get the expected error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Reason must be provided for domain failover")
}

func TestFailoverDomainValidation_WithReason_ExpectSuccess(t *testing.T) {
	// This test verifies that when a failover is attempted with a valid reason,
	// the validation succeeds.

	v, _ := setupMocksForRequestValidator(t)

	// Create a failover request WITH a reason
	failoverRequestWithReason := &types.FailoverDomainRequest{
		DomainName:              "test-domain",
		DomainActiveClusterName: common.StringPtr("cluster2"),
		Reason:                  "Planned maintenance on cluster1 - upgrading to v1.2.3",
	}

	err := v.ValidateFailoverDomainRequest(context.Background(), failoverRequestWithReason)

	// Verify validation passes
	require.NoError(t, err)
}

func TestFailoverDomainReason_StoredInDomainData(t *testing.T) {
	// This test verifies that the reason provided during failover
	// is properly converted to domain data for storage.

	failoverRequest := &types.FailoverDomainRequest{
		DomainName:              "test-domain",
		DomainActiveClusterName: common.StringPtr("cluster2"),
		Reason:                  "Emergency failover due to cluster1 outage",
	}

	// Call ToUpdateDomainRequest and verify reason is in data
	updateReq := failoverRequest.ToUpdateDomainRequest()

	assert.NotNil(t, updateReq)
	assert.Equal(t, "test-domain", updateReq.Name)
	assert.Equal(t, common.StringPtr("cluster2"), updateReq.ActiveClusterName)
	assert.NotNil(t, updateReq.Data)
	assert.Equal(t, "Emergency failover due to cluster1 outage", updateReq.Data["FailoverReason"])
}
