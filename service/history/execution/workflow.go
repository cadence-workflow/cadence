// Copyright (c) 2020 Uber Technologies, Inc.
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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination workflow_mock.go -self_package github.com/uber/cadence/service/history/execution

package execution

import (
	"context"
	"fmt"
	"time"

	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

const (
	// IdentityHistoryService is the service role identity
	IdentityHistoryService = "history-service"
	// WorkflowTerminationIdentity is the component which decides to terminate the workflow
	WorkflowTerminationIdentity = "worker-service"
	// WorkflowTerminationReason is the reason for terminating workflow due to version conflict
	WorkflowTerminationReason = "Terminate Workflow Due To Version Conflict."
)

type (
	// Workflow is the interface for NDC workflow
	Workflow interface {
		GetContext() Context
		GetMutableState() MutableState
		GetReleaseFn() ReleaseFunc
		GetVectorClock() (WorkflowVectorClock, error)
		HappensAfter(that Workflow) (bool, error)
		Revive() error
		SuppressBy(incomingWorkflow Workflow) (TransactionPolicy, error)
		FlushBufferedEvents() error
	}

	workflowImpl struct {
		logger          log.Logger
		clusterMetadata cluster.Metadata

		ctx          context.Context
		context      Context
		mutableState MutableState
		releaseFn    ReleaseFunc
	}

	WorkflowVectorClock struct {
		ActiveClusterSelectionPolicy *types.ActiveClusterSelectionPolicy
		LastWriteVersion             int64
		LastEventTaskID              int64
		StartTimestamp               time.Time
		RunID                        string
	}
)

// NewWorkflow creates a new NDC workflow
func NewWorkflow(
	ctx context.Context,
	clusterMetadata cluster.Metadata,
	context Context,
	mutableState MutableState,
	releaseFn ReleaseFunc,
	logger log.Logger,
) Workflow {

	return &workflowImpl{
		ctx:             ctx,
		clusterMetadata: clusterMetadata,
		logger:          logger,
		context:         context,
		mutableState:    mutableState,
		releaseFn:       releaseFn,
	}
}

func (r *workflowImpl) GetContext() Context {
	return r.context
}

func (r *workflowImpl) GetMutableState() MutableState {
	return r.mutableState
}

func (r *workflowImpl) GetReleaseFn() ReleaseFunc {
	return r.releaseFn
}

func (r *workflowImpl) GetVectorClock() (WorkflowVectorClock, error) {

	lastWriteVersion, err := r.mutableState.GetLastWriteVersion()
	if err != nil {
		return WorkflowVectorClock{}, err
	}

	executionInfo := r.mutableState.GetExecutionInfo()
	return WorkflowVectorClock{
		ActiveClusterSelectionPolicy: executionInfo.ActiveClusterSelectionPolicy,
		LastWriteVersion:             lastWriteVersion,
		LastEventTaskID:              executionInfo.LastEventTaskID,
		StartTimestamp:               executionInfo.StartTimestamp,
		RunID:                        executionInfo.RunID,
	}, nil
}

func (r *workflowImpl) HappensAfter(
	that Workflow,
) (bool, error) {

	thisVectorClock, err := r.GetVectorClock()
	if err != nil {
		return false, err
	}
	thatVectorClock, err := that.GetVectorClock()
	if err != nil {
		return false, err
	}

	return workflowHappensAfter(
		thisVectorClock,
		thatVectorClock,
	), nil
}

func (r *workflowImpl) Revive() error {

	state, _ := r.mutableState.GetWorkflowStateCloseStatus()
	if state != persistence.WorkflowStateZombie {
		return nil
	} else if state == persistence.WorkflowStateCompleted {
		// workflow already finished
		return nil
	}

	// workflow is in zombie state, need to set the state correctly accordingly
	state = persistence.WorkflowStateCreated
	if r.mutableState.HasProcessedOrPendingDecision() {
		state = persistence.WorkflowStateRunning
	}
	return r.mutableState.UpdateWorkflowStateCloseStatus(
		state,
		persistence.WorkflowCloseStatusNone,
	)
}

func (r *workflowImpl) SuppressBy(
	incomingWorkflow Workflow,
) (TransactionPolicy, error) {

	// NOTE: READ BEFORE MODIFICATION
	//
	// if the workflow to be suppressed has last write version being local active
	//  then use active logic to terminate this workflow
	// if the workflow to be suppressed has last write version being remote active
	//  then turn this workflow into a zombie

	currentVectorClock, err := r.GetVectorClock()
	if err != nil {
		return TransactionPolicyActive, err
	}
	incomingVectorClock, err := incomingWorkflow.GetVectorClock()
	if err != nil {
		return TransactionPolicyActive, err
	}

	if workflowHappensAfter(
		currentVectorClock,
		incomingVectorClock,
	) {
		return TransactionPolicyActive, &types.InternalServiceError{
			Message: "nDCWorkflow cannot suppress workflow by older workflow",
		}
	}

	// if workflow is in zombie or finished state, keep as is
	if !r.mutableState.IsWorkflowExecutionRunning() {
		return TransactionPolicyPassive, nil
	}

	lastWriteCluster, err := r.clusterMetadata.ClusterNameForFailoverVersion(currentVectorClock.LastWriteVersion)
	if err != nil {
		return TransactionPolicyActive, err
	}
	currentCluster := r.clusterMetadata.GetCurrentClusterName()

	if currentCluster == lastWriteCluster {
		return TransactionPolicyActive, r.terminateWorkflow(currentVectorClock.LastWriteVersion, incomingVectorClock.LastWriteVersion, WorkflowTerminationReason)
	}
	return TransactionPolicyPassive, r.zombiefyWorkflow()
}

func (r *workflowImpl) FlushBufferedEvents() error {

	if !r.mutableState.IsWorkflowExecutionRunning() {
		return nil
	}

	if !r.mutableState.HasBufferedEvents() {
		return nil
	}

	currentVectorClock, err := r.GetVectorClock()
	if err != nil {
		return err
	}

	lastWriteCluster, err := r.clusterMetadata.ClusterNameForFailoverVersion(currentVectorClock.LastWriteVersion)
	if err != nil {
		// TODO: add a test for this
		return err
	}
	currentCluster := r.clusterMetadata.GetCurrentClusterName()

	if lastWriteCluster != currentCluster {
		// TODO: add a test for this
		return &types.InternalServiceError{
			Message: "nDCWorkflow encounter workflow with buffered events but last write not from current cluster",
		}
	}

	return r.failDecision(currentVectorClock.LastWriteVersion, true)
}

func (r *workflowImpl) failDecision(
	lastWriteVersion int64,
	scheduleNewDecision bool,
) error {

	// do not persist the change right now, NDC requires transaction
	r.logger.Debugf("failDecision calling UpdateCurrentVersion for domain %s, wfID %v, lastWriteVersion %v",
		r.mutableState.GetExecutionInfo().DomainID, r.mutableState.GetExecutionInfo().WorkflowID, lastWriteVersion)
	if err := r.mutableState.UpdateCurrentVersion(lastWriteVersion, true); err != nil {
		return err
	}

	decision, ok := r.mutableState.GetInFlightDecision()
	if !ok {
		return nil
	}

	if err := FailDecision(r.mutableState, decision, types.DecisionTaskFailedCauseFailoverCloseDecision); err != nil {
		return err
	}
	if scheduleNewDecision {
		return ScheduleDecision(r.mutableState)
	}
	return nil
}

func (r *workflowImpl) terminateWorkflow(
	lastWriteVersion int64,
	incomingLastWriteVersion int64,
	terminationReason string,
) error {

	eventBatchFirstEventID := r.GetMutableState().GetNextEventID()
	if err := r.failDecision(lastWriteVersion, false); err != nil {
		return err
	}

	// do not persist the change right now, NDC requires transaction
	r.logger.Debugf("terminateWorkflow calling UpdateCurrentVersion for domain %s, wfID %v, lastWriteVersion %v",
		r.mutableState.GetExecutionInfo().DomainID, r.mutableState.GetExecutionInfo().WorkflowID, lastWriteVersion)
	if err := r.mutableState.UpdateCurrentVersion(lastWriteVersion, true); err != nil {
		return err
	}

	_, err := r.mutableState.AddWorkflowExecutionTerminatedEvent(
		eventBatchFirstEventID,
		terminationReason,
		[]byte(fmt.Sprintf("terminated by version: %v", incomingLastWriteVersion)),
		WorkflowTerminationIdentity,
	)

	return err
}

func (r *workflowImpl) zombiefyWorkflow() error {

	return r.mutableState.GetExecutionInfo().UpdateWorkflowStateCloseStatus(
		persistence.WorkflowStateZombie,
		persistence.WorkflowCloseStatusNone,
	)
}

// Conflict resolution for workflow replication requires determining which workflow event is "newer."
// This decision must be made deterministically across all clusters without coordination,
// ensuring that each cluster independently resolves conflicts to the same final state.
//
// Active-Passive Domains:
//   - These domains do not use active cluster selection policies.
//   - The event with the larger failover version is considered newer, since the failover version is
//     a monotonically increasing logical clock for the domain.
//   - After a failover, any event with a higher failover version wins conflict resolution.
//   - Conflicts between events with the same failover version should not occur, as they originate
//     from the same active cluster. In case of a tie, the replication task event ID is used as a tiebreaker.
//
// Active-Active Domains (Same Selection Policy):
//   - Treated the same as active-passive domains.
//   - The event generated after failover (larger failover version) wins.
//
// Active-Active Domains (Different Selection Policies):
//   - These represent workflows started concurrently in different active clusters.
//   - The event with the larger start timestamp is considered newer.
//   - Clock skew between clusters is expected to be small, making this a reasonable rule.
//   - In case of a tie on start time, the RunID is used as a final tiebreaker.
func workflowHappensAfter(
	thisVectorClock WorkflowVectorClock,
	thatVectorClock WorkflowVectorClock,
) bool {
	if !thisVectorClock.ActiveClusterSelectionPolicy.Equals(thatVectorClock.ActiveClusterSelectionPolicy) {
		if !thisVectorClock.StartTimestamp.Equal(thatVectorClock.StartTimestamp) {
			return thatVectorClock.StartTimestamp.Before(thisVectorClock.StartTimestamp)
		}
		return thisVectorClock.RunID > thatVectorClock.RunID
	}

	if thisVectorClock.LastWriteVersion != thatVectorClock.LastWriteVersion {
		return thisVectorClock.LastWriteVersion > thatVectorClock.LastWriteVersion
	}

	// thisVectorClock.LastWriteVersion == thatVectorClock.LastWriteVersion
	return thisVectorClock.LastEventTaskID > thatVectorClock.LastEventTaskID
}
