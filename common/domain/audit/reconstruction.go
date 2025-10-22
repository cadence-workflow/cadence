package audit

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

var (
	// Pattern for default cluster change: /ReplicationConfig/ActiveClusterName
	defaultClusterPattern = regexp.MustCompile(`^/ReplicationConfig/ActiveClusterName$`)

	// Pattern for cluster attribute change: /ReplicationConfig/ActiveClusters/attributeScopes/{scope}/clusterAttributes/{name}/activeClusterName
	// Note: JSON marshaling uses lowercase field names from the proto/struct tags
	clusterAttributePattern = regexp.MustCompile(`^/ReplicationConfig/ActiveClusters/attributeScopes/([^/]+)/clusterAttributes/([^/]+)/activeClusterName$`)
)

// ReconstructFailoverEvents converts audit log entries to FailoverEvent types
func ReconstructFailoverEvents(
	entries []*persistence.DomainAuditLogEntry,
) ([]*types.FailoverEvent, error) {
	events := make([]*types.FailoverEvent, 0, len(entries))

	for _, entry := range entries {
		// Only process failover operations
		if entry.OperationType != persistence.DomainOperationTypeFailover {
			continue
		}

		// Convert to Unix milliseconds
		createdTimeMs := entry.CreatedTime.UnixNano() / int64(1000000)
		eventID := entry.EventID
		failoverType := determineFailoverType(entry)

		event := &types.FailoverEvent{
			ID:           &eventID,
			CreatedTime:  &createdTimeMs,
			FailoverType: &failoverType,
		}

		events = append(events, event)
	}

	return events, nil
}

// ReconstructClusterFailovers extracts cluster failover details from a diff
func ReconstructClusterFailovers(diffBytes []byte) ([]*types.ClusterFailover, error) {
	// Log the raw state_after for debugging
	fmt.Printf("DEBUG: StateAfter content: %s\n", string(diffBytes))

	// Parse JSON patch
	var patch []map[string]interface{}
	if err := json.Unmarshal(diffBytes, &patch); err != nil {
		return nil, fmt.Errorf("failed to parse state_after as JSON: %w", err)
	}

	fmt.Printf("DEBUG: Parsed %d patch operations\n", len(patch))

	var failovers []*types.ClusterFailover

	// Iterate through patch operations
	for i, op := range patch {
		fmt.Printf("DEBUG: Operation %d: %+v\n", i, op)

		path, ok := op["path"].(string)
		if !ok {
			fmt.Printf("DEBUG: Operation %d has no 'path' field or path is not a string\n", i)
			continue
		}

		value, ok := op["value"].(string)
		if !ok {
			fmt.Printf("DEBUG: Operation %d path=%s has no 'value' field or value is not a string\n", i, path)
			continue
		}

		fmt.Printf("DEBUG: Checking path=%s value=%s\n", path, value)

		// Check if this is a default cluster change
		if defaultClusterPattern.MatchString(path) {
			fmt.Printf("DEBUG: Matched default cluster pattern!\n")
			isDefault := true
			failovers = append(failovers, &types.ClusterFailover{
				ToCluster: &types.ActiveClusterInfo{
					ActiveClusterName: value,
				},
				IsDefaultCluster: &isDefault,
			})
			continue
		}

		// Check if this is a cluster attribute change
		matches := clusterAttributePattern.FindStringSubmatch(path)
		if len(matches) == 3 {
			scope := matches[1]
			name := matches[2]
			isDefault := false

			fmt.Printf("DEBUG: Matched cluster attribute pattern! scope=%s name=%s\n", scope, name)
			failovers = append(failovers, &types.ClusterFailover{
				ToCluster: &types.ActiveClusterInfo{
					ActiveClusterName: value,
				},
				ClusterAttribute: &types.ClusterAttribute{
					Scope: scope,
					Name:  name,
				},
				IsDefaultCluster: &isDefault,
			})
		} else {
			fmt.Printf("DEBUG: Path did not match any known patterns: %s\n", path)
		}
	}

	fmt.Printf("DEBUG: Found %d cluster failovers\n", len(failovers))
	return failovers, nil
}

func determineFailoverType(entry *persistence.DomainAuditLogEntry) types.FailoverType {
	// For POC, always return FORCE
	// In production, would parse the diff or use additional metadata
	return types.FailoverTypeForce
}

// GetFailoverEventDetails fetches full details for a specific failover event
func GetFailoverEventDetails(entry *persistence.DomainAuditLogEntry) (*types.GetFailoverEventResponse, error) {
	if entry == nil {
		return nil, &types.BadRequestError{Message: "domain audit log entry is nil"}
	}

	if len(entry.StateAfter) == 0 {
		// Empty state_after - this could indicate the entry was found but has no change data
		// This might happen for non-failover operations or data corruption
		return nil, &types.BadRequestError{
			Message: fmt.Sprintf("state_after is empty for event %s", entry.EventID),
		}
	}

	failovers, err := ReconstructClusterFailovers(entry.StateAfter)
	if err != nil {
		return nil, &types.BadRequestError{
			Message: fmt.Sprintf("failed to reconstruct cluster failovers: %v", err),
		}
	}

	if len(failovers) == 0 {
		// This could indicate the StateAfter doesn't contain any recognizable failover patterns
		return nil, &types.BadRequestError{
			Message: fmt.Sprintf("no cluster failovers found in state_after for event %s (state_after length: %d bytes)",
				entry.EventID, len(entry.StateAfter)),
		}
	}

	return &types.GetFailoverEventResponse{
		ClusterFailovers: failovers,
	}, nil
}
