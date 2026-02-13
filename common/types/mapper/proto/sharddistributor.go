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

package proto

import (
	sharddistributorv1 "github.com/uber/cadence/.gen/proto/sharddistributor/v1"
	"github.com/uber/cadence/common/types"
)

// FromShardDistributorGetShardOwnerRequest converts a types.GetShardOwnerRequest to a sharddistributor.GetShardOwnerRequest
func FromShardDistributorGetShardOwnerRequest(t *types.GetShardOwnerRequest) *sharddistributorv1.GetShardOwnerRequest {
	if t == nil {
		return nil
	}
	return &sharddistributorv1.GetShardOwnerRequest{
		ShardKey:  t.GetShardKey(),
		Namespace: t.GetNamespace(),
	}
}

// ToShardDistributorGetShardOwnerRequest converts a sharddistributor.GetShardOwnerRequest to a types.GetShardOwnerRequest
func ToShardDistributorGetShardOwnerRequest(t *sharddistributorv1.GetShardOwnerRequest) *types.GetShardOwnerRequest {
	if t == nil {
		return nil
	}
	return &types.GetShardOwnerRequest{
		ShardKey:  t.GetShardKey(),
		Namespace: t.GetNamespace(),
	}
}

// FromShardDistributorGetShardOwnerResponse converts a types.GetShardOwnerResponse to a sharddistributor.GetShardOwnerResponse
func FromShardDistributorGetShardOwnerResponse(t *types.GetShardOwnerResponse) *sharddistributorv1.GetShardOwnerResponse {
	if t == nil {
		return nil
	}
	return &sharddistributorv1.GetShardOwnerResponse{
		Owner:     t.GetOwner(),
		Namespace: t.GetNamespace(),
		Metadata:  t.GetMetadata(),
	}
}

// ToShardDistributorGetShardOwnerResponse converts a sharddistributor.GetShardOwnerResponse to a types.GetShardOwnerResponse
func ToShardDistributorGetShardOwnerResponse(t *sharddistributorv1.GetShardOwnerResponse) *types.GetShardOwnerResponse {
	if t == nil {
		return nil
	}
	return &types.GetShardOwnerResponse{
		Owner:     t.GetOwner(),
		Namespace: t.GetNamespace(),
		Metadata:  t.GetMetadata(),
	}
}

func FromShardDistributorExecutorHeartbeatRequest(t *types.ExecutorHeartbeatRequest) *sharddistributorv1.HeartbeatRequest {
	if t == nil {
		return nil
	}

	// Convert the ExecutorStatus enum
	var status sharddistributorv1.ExecutorStatus
	switch t.GetStatus() {
	case types.ExecutorStatusINVALID:
		status = sharddistributorv1.ExecutorStatus_EXECUTOR_STATUS_INVALID
	case types.ExecutorStatusACTIVE:
		status = sharddistributorv1.ExecutorStatus_EXECUTOR_STATUS_ACTIVE
	case types.ExecutorStatusDRAINING:
		status = sharddistributorv1.ExecutorStatus_EXECUTOR_STATUS_DRAINING
	case types.ExecutorStatusDRAINED:
		status = sharddistributorv1.ExecutorStatus_EXECUTOR_STATUS_DRAINED
	default:
		status = sharddistributorv1.ExecutorStatus_EXECUTOR_STATUS_INVALID
	}

	// Convert the ShardStatusReports
	var shardStatusReports map[string]*sharddistributorv1.ShardStatusReport
	if t.GetShardStatusReports() != nil {
		shardStatusReports = make(map[string]*sharddistributorv1.ShardStatusReport)

		for shardKey, shardStatusReport := range t.GetShardStatusReports() {

			var status sharddistributorv1.ShardStatus
			switch shardStatusReport.GetStatus() {
			case types.ShardStatusINVALID:
				status = sharddistributorv1.ShardStatus_SHARD_STATUS_INVALID
			case types.ShardStatusREADY:
				status = sharddistributorv1.ShardStatus_SHARD_STATUS_READY
			case types.ShardStatusDONE:
				status = sharddistributorv1.ShardStatus_SHARD_STATUS_DONE
			default:
				status = sharddistributorv1.ShardStatus_SHARD_STATUS_INVALID
			}

			shardStatusReports[shardKey] = &sharddistributorv1.ShardStatusReport{
				Status:    status,
				ShardLoad: shardStatusReport.GetShardLoad(),
			}
		}
	}
	return &sharddistributorv1.HeartbeatRequest{
		Namespace:          t.GetNamespace(),
		ExecutorId:         t.GetExecutorID(),
		Status:             status,
		ShardStatusReports: shardStatusReports,
		Metadata:           t.GetMetadata(),
	}
}

func ToShardDistributorExecutorHeartbeatRequest(t *sharddistributorv1.HeartbeatRequest) *types.ExecutorHeartbeatRequest {
	if t == nil {
		return nil
	}

	// Convert the ExecutorStatus enum
	var status types.ExecutorStatus
	switch t.GetStatus() {
	case sharddistributorv1.ExecutorStatus_EXECUTOR_STATUS_INVALID:
		status = types.ExecutorStatusINVALID
	case sharddistributorv1.ExecutorStatus_EXECUTOR_STATUS_ACTIVE:
		status = types.ExecutorStatusACTIVE
	case sharddistributorv1.ExecutorStatus_EXECUTOR_STATUS_DRAINING:
		status = types.ExecutorStatusDRAINING
	case sharddistributorv1.ExecutorStatus_EXECUTOR_STATUS_DRAINED:
		status = types.ExecutorStatusDRAINED
	default:
		status = types.ExecutorStatusINVALID
	}

	// Convert the ShardStatusReports
	var shardStatusReports map[string]*types.ShardStatusReport
	if t.GetShardStatusReports() != nil {
		shardStatusReports = make(map[string]*types.ShardStatusReport)

		for shardKey, shardStatusReport := range t.GetShardStatusReports() {

			var status types.ShardStatus
			switch shardStatusReport.GetStatus() {
			case sharddistributorv1.ShardStatus_SHARD_STATUS_INVALID:
				status = types.ShardStatusINVALID
			case sharddistributorv1.ShardStatus_SHARD_STATUS_READY:
				status = types.ShardStatusREADY
			case sharddistributorv1.ShardStatus_SHARD_STATUS_DONE:
				status = types.ShardStatusDONE
			}

			shardStatusReports[shardKey] = &types.ShardStatusReport{
				Status:    status,
				ShardLoad: shardStatusReport.GetShardLoad(),
			}
		}
	}

	return &types.ExecutorHeartbeatRequest{
		Namespace:          t.GetNamespace(),
		ExecutorID:         t.GetExecutorId(),
		Status:             status,
		ShardStatusReports: shardStatusReports,
		Metadata:           t.GetMetadata(),
	}
}

func FromShardDistributorExecutorHeartbeatResponse(t *types.ExecutorHeartbeatResponse) *sharddistributorv1.HeartbeatResponse {
	if t == nil {
		return nil
	}

	// Convert the ShardAssignments
	var shardAssignments map[string]*sharddistributorv1.ShardAssignment
	migrationMode := toMigrationMode(t.GetMigrationMode())
	if t.GetShardAssignments() != nil {
		shardAssignments = make(map[string]*sharddistributorv1.ShardAssignment)

		for shardKey, shardAssignment := range t.GetShardAssignments() {
			var status sharddistributorv1.AssignmentStatus
			switch shardAssignment.GetStatus() {
			case types.AssignmentStatusINVALID:
				status = sharddistributorv1.AssignmentStatus_ASSIGNMENT_STATUS_INVALID
			case types.AssignmentStatusREADY:
				status = sharddistributorv1.AssignmentStatus_ASSIGNMENT_STATUS_READY
			}
			shardAssignments[shardKey] = &sharddistributorv1.ShardAssignment{
				Status: status,
			}
		}
	}

	return &sharddistributorv1.HeartbeatResponse{
		ShardAssignments: shardAssignments,
		MigrationMode:    migrationMode,
	}
}

func ToShardDistributorExecutorHeartbeatResponse(t *sharddistributorv1.HeartbeatResponse) *types.ExecutorHeartbeatResponse {
	if t == nil {
		return nil
	}

	// Convert the ShardAssignments
	var shardAssignments map[string]*types.ShardAssignment
	var migrationMode types.MigrationMode
	if t.GetShardAssignments() != nil {
		shardAssignments = make(map[string]*types.ShardAssignment)

		for shardKey, shardAssignment := range t.GetShardAssignments() {
			var status types.AssignmentStatus
			switch shardAssignment.GetStatus() {
			case sharddistributorv1.AssignmentStatus_ASSIGNMENT_STATUS_INVALID:
				status = types.AssignmentStatusINVALID
			case sharddistributorv1.AssignmentStatus_ASSIGNMENT_STATUS_READY:
				status = types.AssignmentStatusREADY
			}
			shardAssignments[shardKey] = &types.ShardAssignment{
				Status: status,
			}
		}
	}
	migrationMode = getMigrationModeFromProto(t.GetMigrationMode())

	return &types.ExecutorHeartbeatResponse{
		ShardAssignments: shardAssignments,
		MigrationMode:    migrationMode,
	}
}

func getMigrationModeFromProto(protoMigrationMode sharddistributorv1.MigrationMode) types.MigrationMode {
	var mode types.MigrationMode
	switch protoMigrationMode {
	case sharddistributorv1.MigrationMode_MIGRATION_MODE_LOCAL_PASSTHROUGH:
		mode = types.MigrationModeLOCALPASSTHROUGH
	case sharddistributorv1.MigrationMode_MIGRATION_MODE_LOCAL_PASSTHROUGH_SHADOW:
		mode = types.MigrationModeLOCALPASSTHROUGHSHADOW
	case sharddistributorv1.MigrationMode_MIGRATION_MODE_DISTRIBUTED_PASSTHROUGH:
		mode = types.MigrationModeDISTRIBUTEDPASSTHROUGH
	case sharddistributorv1.MigrationMode_MIGRATION_MODE_ONBOARDED:
		mode = types.MigrationModeONBOARDED
	default:
		mode = types.MigrationModeINVALID
	}
	return mode
}

func toMigrationMode(modeSD types.MigrationMode) sharddistributorv1.MigrationMode {
	var mode sharddistributorv1.MigrationMode
	switch modeSD {
	case types.MigrationModeINVALID:
		mode = sharddistributorv1.MigrationMode_MIGRATION_MODE_INVALID
	case types.MigrationModeLOCALPASSTHROUGH:
		mode = sharddistributorv1.MigrationMode_MIGRATION_MODE_LOCAL_PASSTHROUGH
	case types.MigrationModeLOCALPASSTHROUGHSHADOW:
		mode = sharddistributorv1.MigrationMode_MIGRATION_MODE_LOCAL_PASSTHROUGH_SHADOW
	case types.MigrationModeDISTRIBUTEDPASSTHROUGH:
		mode = sharddistributorv1.MigrationMode_MIGRATION_MODE_DISTRIBUTED_PASSTHROUGH
	case types.MigrationModeONBOARDED:
		mode = sharddistributorv1.MigrationMode_MIGRATION_MODE_ONBOARDED
	default:
		mode = sharddistributorv1.MigrationMode_MIGRATION_MODE_INVALID
	}
	return mode
}

// FromShardDistributorWatchNamespaceStateRequest converts a types.WatchNamespaceStateRequest to a sharddistributor.WatchNamespaceStateRequest
func FromShardDistributorWatchNamespaceStateRequest(t *types.WatchNamespaceStateRequest) *sharddistributorv1.WatchNamespaceStateRequest {
	if t == nil {
		return nil
	}
	return &sharddistributorv1.WatchNamespaceStateRequest{
		Namespace: t.GetNamespace(),
	}
}

// ToShardDistributorWatchNamespaceStateRequest converts a sharddistributor.WatchNamespaceStateRequest to a types.WatchNamespaceStateRequest
func ToShardDistributorWatchNamespaceStateRequest(t *sharddistributorv1.WatchNamespaceStateRequest) *types.WatchNamespaceStateRequest {
	if t == nil {
		return nil
	}
	return &types.WatchNamespaceStateRequest{
		Namespace: t.GetNamespace(),
	}
}

// FromShardDistributorWatchNamespaceStateResponse converts a types.WatchNamespaceStateResponse to a sharddistributor.WatchNamespaceStateResponse
func FromShardDistributorWatchNamespaceStateResponse(t *types.WatchNamespaceStateResponse) *sharddistributorv1.WatchNamespaceStateResponse {
	if t == nil {
		return nil
	}

	var executors []*sharddistributorv1.ExecutorInfo

	for _, executor := range t.GetExecutors() {
		// Convert the Shards
		shards := make([]*sharddistributorv1.Shard, 0, len(executor.GetAssignedShards()))
		for _, shard := range executor.GetAssignedShards() {
			shards = append(shards, &sharddistributorv1.Shard{
				ShardKey: shard.GetShardKey(),
			})
		}
		executors = append(executors, &sharddistributorv1.ExecutorInfo{
			ExecutorId: executor.GetExecutorID(),
			Metadata:   executor.GetMetadata(),
			Shards:     shards,
		})
	}

	return &sharddistributorv1.WatchNamespaceStateResponse{
		Executors: executors,
	}
}

// ToShardDistributorWatchNamespaceStateResponse converts a sharddistributor.WatchNamespaceStateResponse to a types.WatchNamespaceStateResponse
func ToShardDistributorWatchNamespaceStateResponse(t *sharddistributorv1.WatchNamespaceStateResponse) *types.WatchNamespaceStateResponse {
	if t == nil {
		return nil
	}

	var executors []*types.ExecutorShardAssignment
	if t.GetExecutors() != nil {
		executors = make([]*types.ExecutorShardAssignment, 0, len(t.GetExecutors()))
		for _, executor := range t.GetExecutors() {
			// Convert the Shards
			shards := make([]*types.Shard, 0, len(executor.GetShards()))
			for _, shard := range executor.GetShards() {
				shards = append(shards, &types.Shard{
					ShardKey: shard.GetShardKey(),
				})
			}

			executors = append(executors, &types.ExecutorShardAssignment{
				ExecutorID:     executor.GetExecutorId(),
				Metadata:       executor.GetMetadata(),
				AssignedShards: shards,
			})
		}
	}

	return &types.WatchNamespaceStateResponse{
		Executors: executors,
	}
}
