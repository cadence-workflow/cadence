// Copyright (c) 2025 Uber Technologies, Inc.
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

package audit

import (
	"context"
	"encoding/json"

	"github.com/wI2L/jsondiff"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

// ComputeDomainDiff computes the JSON diff between old and new domain states
// Returns the diff as a JSON patch (RFC6902) in byte format
// This handles both active-passive and active-active (cluster attributes) changes
func ComputeDomainDiff(
	oldDomain *persistence.GetDomainResponse,
	newDomain *persistence.GetDomainResponse,
) ([]byte, error) {
	// Convert to JSON-serializable format
	oldJSON, err := json.Marshal(oldDomain)
	if err != nil {
		return nil, err
	}

	newJSON, err := json.Marshal(newDomain)
	if err != nil {
		return nil, err
	}

	// Unmarshal to interface{} for jsondiff
	var oldData, newData interface{}
	if err := json.Unmarshal(oldJSON, &oldData); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(newJSON, &newData); err != nil {
		return nil, err
	}

	// Compute diff using jsondiff
	patch, err := jsondiff.Compare(oldData, newData)
	if err != nil {
		return nil, err
	}

	// Serialize patch to JSON bytes
	return json.Marshal(patch)
}

// DetermineOperationType determines the operation type from the update request
func DetermineOperationType(request *types.UpdateDomainRequest) persistence.DomainOperationType {
	// Check if this is a failover (active cluster change)
	if request.ActiveClusterName != nil {
		return persistence.DomainOperationTypeFailover
	}

	// Check for active clusters (active-active failover)
	if request.ActiveClusters != nil {
		return persistence.DomainOperationTypeFailover
	}

	// Otherwise it's a regular update
	return persistence.DomainOperationTypeUpdate
}

// ExtractIdentity extracts identity information from the context
// For POC, return placeholder values
// TODO: Implement proper identity extraction from context
func ExtractIdentity(ctx context.Context) (identity, identityType string) {
	// Placeholder for POC
	return "unknown", "system"
}
