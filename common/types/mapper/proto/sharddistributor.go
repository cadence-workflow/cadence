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
	}
}

func FromShardDistributorExecutorHeartbeatRequest(t *types.ExecutorHeartbeatRequest) *sharddistributorv1.HeartbeatRequest {
	if t == nil {
		return nil
	}

	// Convert the ExecutorStatus enum
	var status sharddistributorv1.ExecutorStatus
	status = toExecutorStatus(t.GetStatus())

	// Convert the ShardStatusReports
	var shardStatusReports map[string]*sharddistributorv1.ShardStatusReport
	if t.GetShardStatusReports() != nil {
		shardStatusReports = make(map[string]*sharddistributorv1.ShardStatusReport)

		for shardKey, shardStatusReport := range t.GetShardStatusReports() {

			var status sharddistributorv1.ShardStatus
			status = toShardStatus(shardStatusReport.GetStatus())

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
	}
}

func ToShardDistributorExecutorHeartbeatRequest(t *sharddistributorv1.HeartbeatRequest) *types.ExecutorHeartbeatRequest {
	if t == nil {
		return nil
	}

	// Convert the ExecutorStatus enum
	status := getExecutorStatusFromProto(t.GetStatus())

	// Convert the ShardStatusReports
	var shardStatusReports map[string]*types.ShardStatusReport
	if t.GetShardStatusReports() != nil {
		shardStatusReports = make(map[string]*types.ShardStatusReport)

		for shardKey, shardStatusReport := range t.GetShardStatusReports() {

			var status types.ShardStatus
			status = getShardStatusFromProto(shardStatusReport.GetStatus())
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
	}
}

func FromShardDistributorExecutorHeartbeatResponse(t *types.ExecutorHeartbeatResponse) *sharddistributorv1.HeartbeatResponse {
	if t == nil {
		return nil
	}

	// Convert the ShardAssignments
	var shardAssignments map[string]*sharddistributorv1.ShardAssignment
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
	}
}

func ToShardDistributorExecutorHeartbeatResponse(t *sharddistributorv1.HeartbeatResponse) *types.ExecutorHeartbeatResponse {
	if t == nil {
		return nil
	}

	// Convert the ShardAssignments
	var shardAssignments map[string]*types.ShardAssignment
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

	return &types.ExecutorHeartbeatResponse{
		ShardAssignments: shardAssignments,
	}
}

func FromShardDistributorExecutorAssignShardRequest(t *types.ExecutorAssignShardRequest) *sharddistributorv1.AssignShardRequest {
	if t == nil {
		return nil
	}
	executorstatus := toExecutorStatus(t.GetExecutorStatus())
	shardstatus := toShardStatus(t.GetShardStatus())
	return &sharddistributorv1.AssignShardRequest{
		Namespace:      t.GetNamespace(),
		ExecutorId:     t.GetExecutorID(),
		ExecutorStatus: executorstatus,
		ShardId:        t.GetShardID(),
		ShardStatus:    shardstatus,
	}

}

func ToShardDistributorExecutorAssignShardRequest(t *sharddistributorv1.AssignShardRequest) *types.ExecutorAssignShardRequest {
	if t == nil {
		return nil
	}

	executorStatus := getExecutorStatusFromProto(t.GetExecutorStatus())
	shardStatus := getShardStatusFromProto(t.GetShardStatus())

	return &types.ExecutorAssignShardRequest{
		Namespace:      t.GetNamespace(),
		ExecutorID:     t.GetExecutorId(),
		ExecutorStatus: executorStatus,
		ShardID:        t.GetShardId(),
		ShardStatus:    shardStatus,
	}
}

func FromShardDistributorExecutorAssignShardResponse(t *types.ExecutorAssignShardResponse) *sharddistributorv1.AssignShardResponse {
	if t == nil {
		return nil
	}
	var shardAssignments map[string]*sharddistributorv1.ShardAssignment
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

	return &sharddistributorv1.AssignShardResponse{
		ShardAssignments: shardAssignments,
		MigrationPhase:   toMigrationPhase(t.GetMigrationPhase()),
	}
}

func ToShardDistributorExecutorAssignShardResponse(t *sharddistributorv1.AssignShardResponse) *types.ExecutorAssignShardResponse {
	if t == nil {
		return nil
	}
	var shardAssignments map[string]*types.ShardAssignment
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

	return &types.ExecutorAssignShardResponse{
		ShardAssignments: shardAssignments,
		MigrationPhase:   getMigrationPhaseFromProto(t.GetMigrationPhase()),
	}
}

func getExecutorStatusFromProto(protoStatus sharddistributorv1.ExecutorStatus) types.ExecutorStatus {
	var status types.ExecutorStatus
	switch protoStatus {
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
	return status
}

func getShardStatusFromProto(protoStatus sharddistributorv1.ShardStatus) types.ShardStatus {
	var status types.ShardStatus
	switch protoStatus {
	case sharddistributorv1.ShardStatus_SHARD_STATUS_INVALID:
		status = types.ShardStatusINVALID
	case sharddistributorv1.ShardStatus_SHARD_STATUS_READY:
		status = types.ShardStatusREADY
	case sharddistributorv1.ShardStatus_SHARD_STATUS_DONE:
		status = types.ShardStatusDONE
	default:
		status = types.ShardStatusINVALID
	}
	return status
}

func getMigrationPhaseFromProto(protoMigrationPhase sharddistributorv1.MigrationPhase) types.MigrationPhase {
	var phase types.MigrationPhase
	switch protoMigrationPhase {
	case sharddistributorv1.MigrationPhase_MIGRATION_PHASE_INVALID:
		phase = types.MigrationPhaseINVALID
	case sharddistributorv1.MigrationPhase_MIGRATION_PHASE_LOCAL_PASSTHROUGH:
		phase = types.MigrationPhaseLOCALPASSTHROUGH
	case sharddistributorv1.MigrationPhase_MIGRATION_PHASE_LOCAL_PASSTHROUGH_SHADOW:
		phase = types.MigrationPhaseLOCALPASSTHROUGHSHADOW
	case sharddistributorv1.MigrationPhase_MIGRATION_PHASE_DISTRIBUTED_PASSTHROUGH:
		phase = types.MigrationPhaseDISTRIBUTEDPASSTHROUGH
	case sharddistributorv1.MigrationPhase_MIGRATION_PHASE_ONBOARDED:
		phase = types.MigrationPhasePHASEONBOARDED
	default:
		phase = types.MigrationPhaseINVALID
	}
	return phase
}

func toExecutorStatus(statusSD types.ExecutorStatus) sharddistributorv1.ExecutorStatus {
	var status sharddistributorv1.ExecutorStatus
	switch statusSD {
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
	return status
}

func toShardStatus(statusSD types.ShardStatus) sharddistributorv1.ShardStatus {
	var status sharddistributorv1.ShardStatus
	switch statusSD {
	case types.ShardStatusINVALID:
		status = sharddistributorv1.ShardStatus_SHARD_STATUS_INVALID
	case types.ShardStatusREADY:
		status = sharddistributorv1.ShardStatus_SHARD_STATUS_READY
	case types.ShardStatusDONE:
		status = sharddistributorv1.ShardStatus_SHARD_STATUS_DONE
	default:
		status = sharddistributorv1.ShardStatus_SHARD_STATUS_INVALID
	}
	return status
}

func toMigrationPhase(phaseSD types.MigrationPhase) sharddistributorv1.MigrationPhase {
	var phase sharddistributorv1.MigrationPhase
	switch phaseSD {
	case types.MigrationPhaseINVALID:
		phase = sharddistributorv1.MigrationPhase_MIGRATION_PHASE_INVALID
	case types.MigrationPhaseLOCALPASSTHROUGH:
		phase = sharddistributorv1.MigrationPhase_MIGRATION_PHASE_LOCAL_PASSTHROUGH_SHADOW
	case types.MigrationPhaseDISTRIBUTEDPASSTHROUGH:
		phase = sharddistributorv1.MigrationPhase_MIGRATION_PHASE_DISTRIBUTED_PASSTHROUGH
	case types.MigrationPhasePHASEONBOARDED:
		phase = sharddistributorv1.MigrationPhase_MIGRATION_PHASE_ONBOARDED
	default:
		phase = sharddistributorv1.MigrationPhase_MIGRATION_PHASE_INVALID
	}
	return phase
}
