// Copyright (c) 2017-2020 Uber Technologies, Inc.
// Portions of the Software are attributed to Copyright (c) 2020 Temporal Technologies Inc.
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

package execution

import (
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

type (
	// TransactionPolicy is the policy used for updating workflow execution
	TransactionPolicy int
)

const (
	// TransactionPolicyActive updates workflow execution as active
	TransactionPolicyActive TransactionPolicy = 0
	// TransactionPolicyPassive updates workflow execution as passive
	TransactionPolicyPassive TransactionPolicy = 1
)

// Ptr returns a pointer to the current transaction policy
func (policy TransactionPolicy) Ptr() *TransactionPolicy {
	return &policy
}

func convertSyncActivityInfos(
	executionInfo *persistence.WorkflowExecutionInfo,
	activityInfos map[int64]*persistence.ActivityInfo,
	inputs map[int64]struct{},
) []persistence.Task {
	outputs := make([]persistence.Task, 0, len(inputs))
	for item := range inputs {
		activityInfo, ok := activityInfos[item]
		if ok {
			// the visibility timestamp will be set in shard context
			outputs = append(outputs, &persistence.SyncActivityTask{
				WorkflowIdentifier: persistence.WorkflowIdentifier{
					DomainID:   executionInfo.DomainID,
					WorkflowID: executionInfo.WorkflowID,
					RunID:      executionInfo.RunID,
				},
				TaskData: persistence.TaskData{
					Version: activityInfo.Version,
				},
				ScheduledID: activityInfo.ScheduleID,
			})
		}
	}
	return outputs
}

func convertWorkflowRequests(inputs map[persistence.WorkflowRequest]struct{}) []*persistence.WorkflowRequest {
	outputs := make([]*persistence.WorkflowRequest, 0, len(inputs))
	for key := range inputs {
		outputs = append(outputs, &key)
	}
	return outputs
}

// FailDecision fails the current decision task
func FailDecision(
	mutableState MutableState,
	decision *DecisionInfo,
	decisionFailureCause types.DecisionTaskFailedCause,
) error {

	if _, err := mutableState.AddDecisionTaskFailedEvent(
		decision.ScheduleID,
		decision.StartedID,
		decisionFailureCause,
		nil,
		IdentityHistoryService,
		"",
		"",
		"",
		"",
		0,
		"",
	); err != nil {
		return err
	}

	return mutableState.FlushBufferedEvents()
}

// ScheduleDecision schedules a new decision task
func ScheduleDecision(
	mutableState MutableState,
) error {

	if mutableState.HasPendingDecision() {
		return nil
	}

	_, err := mutableState.AddDecisionTaskScheduledEvent(false)
	if err != nil {
		return &types.InternalServiceError{Message: "Failed to add decision scheduled event."}
	}
	return nil
}

// FindAutoResetPoint returns the auto reset point
func FindAutoResetPoint(
	timeSource clock.TimeSource,
	badBinaries *types.BadBinaries,
	autoResetPoints *types.ResetPoints,
) (string, *types.ResetPointInfo) {
	if badBinaries == nil || badBinaries.Binaries == nil || autoResetPoints == nil || autoResetPoints.Points == nil {
		return "", nil
	}
	nowNano := timeSource.Now().UnixNano()
	for _, p := range autoResetPoints.Points {
		bin, ok := badBinaries.Binaries[p.GetBinaryChecksum()]
		if ok && p.GetResettable() {
			if p.GetExpiringTimeNano() > 0 && nowNano > p.GetExpiringTimeNano() {
				// reset point has expired and we may already deleted the history
				continue
			}
			return bin.GetReason(), p
		}
	}
	return "", nil
}

// GetChildExecutionDomainName gets domain name for the child workflow
// NOTE: DomainName in ChildExecutionInfo is being deprecated, and
// we should always use DomainID field instead.
// this function exists for backward compatibility reason
func GetChildExecutionDomainName(
	childInfo *persistence.ChildExecutionInfo,
	domainCache cache.DomainCache,
	parentDomainEntry *cache.DomainCacheEntry,
) (string, error) {
	if childInfo.DomainID != "" {
		return domainCache.GetDomainName(childInfo.DomainID)
	}

	if childInfo.DomainNameDEPRECATED != "" {
		return childInfo.DomainNameDEPRECATED, nil
	}

	return parentDomainEntry.GetInfo().Name, nil
}

// GetChildExecutionDomainID gets domainID for the child workflow
// NOTE: DomainName in ChildExecutionInfo is being deprecated, and
// we should always use DomainID field instead.
// this function exists for backward compatibility reason
func GetChildExecutionDomainID(
	childInfo *persistence.ChildExecutionInfo,
	domainCache cache.DomainCache,
	parentDomainEntry *cache.DomainCacheEntry,
) (string, error) {
	if childInfo.DomainID != "" {
		return childInfo.DomainID, nil
	}

	if childInfo.DomainNameDEPRECATED != "" {
		return domainCache.GetDomainID(childInfo.DomainNameDEPRECATED)
	}

	return parentDomainEntry.GetInfo().ID, nil
}

func trimBinaryChecksums(recentBinaryChecksums []string, currResetPoints []*types.ResetPointInfo, maxResetPoints int) ([]string, []*types.ResetPointInfo) {
	numResetPoints := len(currResetPoints)
	if numResetPoints >= maxResetPoints {
		// If exceeding the max limit, do rotation by taking the oldest ones out.
		// startIndex plus one here because it needs to make space for the new binary checksum for the current run
		startIndex := numResetPoints - maxResetPoints + 1
		currResetPoints = currResetPoints[startIndex:]
		recentBinaryChecksums = recentBinaryChecksums[startIndex:]
	}

	return recentBinaryChecksums, currResetPoints
}
