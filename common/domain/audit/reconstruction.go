package audit

import (
	"encoding/json"
	"regexp"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

var (
	// Pattern for default cluster change: /ReplicationConfig/ActiveClusterName
	defaultClusterPattern = regexp.MustCompile(`^/ReplicationConfig/ActiveClusterName$`)

	// Pattern for cluster attribute change: /ReplicationConfig/ActiveClusters/{scope}/ClusterAttributes/{name}/ActiveClusterName
	clusterAttributePattern = regexp.MustCompile(`^/ReplicationConfig/ActiveClusters/([^/]+)/ClusterAttributes/([^/]+)/ActiveClusterName$`)
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
	// Parse JSON patch
	var patch []map[string]interface{}
	if err := json.Unmarshal(diffBytes, &patch); err != nil {
		return nil, err
	}

	var failovers []*types.ClusterFailover

	// Iterate through patch operations
	for _, op := range patch {
		path, ok := op["path"].(string)
		if !ok {
			continue
		}

		value, ok := op["value"].(string)
		if !ok {
			continue
		}

		// Check if this is a default cluster change
		if defaultClusterPattern.MatchString(path) {
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
		}
	}

	return failovers, nil
}

func determineFailoverType(entry *persistence.DomainAuditLogEntry) types.FailoverType {
	// For POC, always return FORCE
	// In production, would parse the diff or use additional metadata
	return types.FailoverTypeForce
}

// GetFailoverEventDetails fetches full details for a specific failover event
func GetFailoverEventDetails(entry *persistence.DomainAuditLogEntry) (*types.GetFailoverEventResponse, error) {
	failovers, err := ReconstructClusterFailovers(entry.StateAfter)
	if err != nil {
		return nil, err
	}

	return &types.GetFailoverEventResponse{
		ClusterFailovers: failovers,
	}, nil
}
