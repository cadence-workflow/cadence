// Copyright (c) 2021 Uber Technologies Inc.
//
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
	"reflect"
	"testing"

	"github.com/gogo/protobuf/proto"
	fuzz "github.com/google/gofuzz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiv1 "github.com/uber/cadence-idl/go/proto/api/v1"

	sharedv1 "github.com/uber/cadence/.gen/proto/shared/v1"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/testutils"
	"github.com/uber/cadence/common/types/testdata"
)

func TestHostInfo(t *testing.T) {
	for _, item := range []*types.HostInfo{nil, {}, &testdata.HostInfo} {
		assert.Equal(t, item, ToHostInfo(FromHostInfo(item)))
	}
}
func TestMembershipInfo(t *testing.T) {
	for _, item := range []*types.MembershipInfo{nil, {}, &testdata.MembershipInfo} {
		assert.Equal(t, item, ToMembershipInfo(FromMembershipInfo(item)))
	}
}
func TestDomainCacheInfo(t *testing.T) {
	for _, item := range []*types.DomainCacheInfo{nil, {}, &testdata.DomainCacheInfo} {
		assert.Equal(t, item, ToDomainCacheInfo(FromDomainCacheInfo(item)))
	}
}
func TestRingInfo(t *testing.T) {
	for _, item := range []*types.RingInfo{nil, {}, &testdata.RingInfo} {
		assert.Equal(t, item, ToRingInfo(FromRingInfo(item)))
	}
}
func TestTransientDecisionInfo(t *testing.T) {
	for _, item := range []*types.TransientDecisionInfo{nil, {}, &testdata.TransientDecisionInfo} {
		assert.Equal(t, item, ToTransientDecisionInfo(FromTransientDecisionInfo(item)))
	}
}
func TestVersionHistories(t *testing.T) {
	for _, item := range []*types.VersionHistories{nil, {}, &testdata.VersionHistories} {
		assert.Equal(t, item, ToVersionHistories(FromVersionHistories(item)))
	}
}
func TestVersionHistory(t *testing.T) {
	for _, item := range []*types.VersionHistory{nil, {}, &testdata.VersionHistory} {
		assert.Equal(t, item, ToVersionHistory(FromVersionHistory(item)))
	}
}
func TestVersionHistoryItem(t *testing.T) {
	for _, item := range []*types.VersionHistoryItem{nil, {}, &testdata.VersionHistoryItem} {
		assert.Equal(t, item, ToVersionHistoryItem(FromVersionHistoryItem(item)))
	}
}
func TestHostInfoArray(t *testing.T) {
	for _, item := range [][]*types.HostInfo{nil, {}, testdata.HostInfoArray} {
		assert.Equal(t, item, ToHostInfoArray(FromHostInfoArray(item)))
	}
}
func TestVersionHistoryArray(t *testing.T) {
	for _, item := range [][]*types.VersionHistory{nil, {}, testdata.VersionHistoryArray} {
		assert.Equal(t, item, ToVersionHistoryArray(FromVersionHistoryArray(item)))
	}
}
func TestRingInfoArray(t *testing.T) {
	for _, item := range [][]*types.RingInfo{nil, {}, testdata.RingInfoArray} {
		assert.Equal(t, item, ToRingInfoArray(FromRingInfoArray(item)))
	}
}
func TestDomainTaskAttributes(t *testing.T) {
	for _, item := range []*types.DomainTaskAttributes{nil, &testdata.DomainTaskAttributes} {
		assert.Equal(t, item, ToDomainTaskAttributes(FromDomainTaskAttributes(item)))
	}
}
func TestFailoverMarkerAttributes(t *testing.T) {
	for _, item := range []*types.FailoverMarkerAttributes{nil, {}, &testdata.FailoverMarkerAttributes} {
		assert.Equal(t, item, ToFailoverMarkerAttributes(FromFailoverMarkerAttributes(item)))
	}
}
func TestFailoverMarkerToken(t *testing.T) {
	for _, item := range []*types.FailoverMarkerToken{nil, {}, &testdata.FailoverMarkerToken} {
		assert.Equal(t, item, ToFailoverMarkerToken(FromFailoverMarkerToken(item)))
	}
}
func TestHistoryTaskV2Attributes(t *testing.T) {
	for _, item := range []*types.HistoryTaskV2Attributes{nil, {}, &testdata.HistoryTaskV2Attributes} {
		assert.Equal(t, item, ToHistoryTaskV2Attributes(FromHistoryTaskV2Attributes(item)))
	}
}
func TestReplicationMessages(t *testing.T) {
	for _, item := range []*types.ReplicationMessages{nil, {}, &testdata.ReplicationMessages} {
		assert.Equal(t, item, ToReplicationMessages(FromReplicationMessages(item)))
	}
}
func TestReplicationTaskInfo(t *testing.T) {
	for _, item := range []*types.ReplicationTaskInfo{nil, {}, &testdata.ReplicationTaskInfo} {
		assert.Equal(t, item, ToReplicationTaskInfo(FromReplicationTaskInfo(item)))
	}
}
func TestReplicationToken(t *testing.T) {
	for _, item := range []*types.ReplicationToken{nil, {}, &testdata.ReplicationToken} {
		assert.Equal(t, item, ToReplicationToken(FromReplicationToken(item)))
	}
}
func TestSyncActivityTaskAttributes(t *testing.T) {
	for _, item := range []*types.SyncActivityTaskAttributes{nil, {}, &testdata.SyncActivityTaskAttributes} {
		assert.Equal(t, item, ToSyncActivityTaskAttributes(FromSyncActivityTaskAttributes(item)))
	}
}
func TestSyncShardStatus(t *testing.T) {
	for _, item := range []*types.SyncShardStatus{nil, {}, &testdata.SyncShardStatus} {
		assert.Equal(t, item, ToSyncShardStatus(FromSyncShardStatus(item)))
	}
}
func TestSyncShardStatusTaskAttributes(t *testing.T) {
	for _, item := range []*types.SyncShardStatusTaskAttributes{nil, {}, &testdata.SyncShardStatusTaskAttributes} {
		assert.Equal(t, item, ToSyncShardStatusTaskAttributes(FromSyncShardStatusTaskAttributes(item)))
	}
}
func TestReplicationTaskInfoArray(t *testing.T) {
	for _, item := range [][]*types.ReplicationTaskInfo{nil, {}, testdata.ReplicationTaskInfoArray} {
		assert.Equal(t, item, ToReplicationTaskInfoArray(FromReplicationTaskInfoArray(item)))
	}
}
func TestReplicationTaskArray(t *testing.T) {
	for _, item := range [][]*types.ReplicationTask{nil, {}, testdata.ReplicationTaskArray} {
		assert.Equal(t, item, ToReplicationTaskArray(FromReplicationTaskArray(item)))
	}
}
func TestReplicationTokenArray(t *testing.T) {
	for _, item := range [][]*types.ReplicationToken{nil, {}, testdata.ReplicationTokenArray} {
		assert.Equal(t, item, ToReplicationTokenArray(FromReplicationTokenArray(item)))
	}
}
func TestReplicationMessagesMap(t *testing.T) {
	for _, item := range []map[int32]*types.ReplicationMessages{nil, {}, testdata.ReplicationMessagesMap} {
		assert.Equal(t, item, ToReplicationMessagesMap(FromReplicationMessagesMap(item)))
	}
}
func TestReplicationTask(t *testing.T) {
	for _, item := range []*types.ReplicationTask{
		nil,
		{},
		&testdata.ReplicationTask_Domain,
		&testdata.ReplicationTask_Failover,
		&testdata.ReplicationTask_History,
		&testdata.ReplicationTask_SyncActivity,
		&testdata.ReplicationTask_SyncShard,
	} {
		assert.Equal(t, item, ToReplicationTask(FromReplicationTask(item)))
	}
}
func TestFailoverMarkerTokenArray(t *testing.T) {
	for _, item := range [][]*types.FailoverMarkerToken{nil, {}, testdata.FailoverMarkerTokenArray} {
		assert.Equal(t, item, ToFailoverMarkerTokenArray(FromFailoverMarkerTokenArray(item)))
	}
}
func TestVersionHistoryItemArray(t *testing.T) {
	for _, item := range [][]*types.VersionHistoryItem{nil, {}, testdata.VersionHistoryItemArray} {
		assert.Equal(t, item, ToVersionHistoryItemArray(FromVersionHistoryItemArray(item)))
	}
}
func TestEventIDVersionPair(t *testing.T) {
	assert.Nil(t, FromEventIDVersionPair(nil, nil))
	assert.Nil(t, ToEventID(nil))
	assert.Nil(t, ToEventVersion(nil))

	pair := FromEventIDVersionPair(common.Int64Ptr(testdata.EventID1), common.Int64Ptr(testdata.Version1))
	assert.Equal(t, testdata.EventID1, *ToEventID(pair))
	assert.Equal(t, testdata.Version1, *ToEventVersion(pair))
}

func TestCrossClusterTaskInfo(t *testing.T) {
	for _, item := range []*types.CrossClusterTaskInfo{nil, {}, &testdata.CrossClusterTaskInfo} {
		assert.Equal(t, item, ToCrossClusterTaskInfo(FromCrossClusterTaskInfo(item)))
	}
}

func TestCrossClusterTaskRequest(t *testing.T) {
	for _, item := range []*types.CrossClusterTaskRequest{
		nil,
		{},
		&testdata.CrossClusterTaskRequestStartChildExecution,
		&testdata.CrossClusterTaskRequestCancelExecution,
		&testdata.CrossClusterTaskRequestSignalExecution,
	} {
		assert.Equal(t, item, ToCrossClusterTaskRequest(FromCrossClusterTaskRequest(item)))
	}
}

func TestCrossClusterTaskResponse(t *testing.T) {
	for _, item := range []*types.CrossClusterTaskResponse{
		nil,
		{},
		&testdata.CrossClusterTaskResponseStartChildExecution,
		&testdata.CrossClusterTaskResponseCancelExecution,
		&testdata.CrossClusterTaskResponseSignalExecution,
	} {
		assert.Equal(t, item, ToCrossClusterTaskResponse(FromCrossClusterTaskResponse(item)))
	}
}

func TestCrossClusterTaskRequestArray(t *testing.T) {
	for _, item := range [][]*types.CrossClusterTaskRequest{nil, {}, testdata.CrossClusterTaskRequestArray} {
		assert.Equal(t, item, ToCrossClusterTaskRequestArray(FromCrossClusterTaskRequestArray(item)))
	}
}

func TestCrossClusterTaskResponseArray(t *testing.T) {
	for _, item := range [][]*types.CrossClusterTaskResponse{nil, {}, testdata.CrossClusterTaskResponseArray} {
		assert.Equal(t, item, ToCrossClusterTaskResponseArray(FromCrossClusterTaskResponseArray(item)))
	}
}

func TestCrossClusterTaskRequestMap(t *testing.T) {
	for _, item := range []map[int32][]*types.CrossClusterTaskRequest{nil, {}, testdata.CrossClusterTaskRequestMap} {
		assert.Equal(t, item, ToCrossClusterTaskRequestMap(FromCrossClusterTaskRequestMap(item)))
	}
	assert.Equal(
		t,
		map[int32][]*types.CrossClusterTaskRequest{
			0: {},
		},
		ToCrossClusterTaskRequestMap(FromCrossClusterTaskRequestMap(
			map[int32][]*types.CrossClusterTaskRequest{
				0: nil,
			},
		)),
	)
}

func TestGetTaskFailedCauseMap(t *testing.T) {
	for _, item := range []map[int32]types.GetTaskFailedCause{nil, {}, testdata.GetCrossClusterTaskFailedCauseMap} {
		assert.Equal(t, item, ToGetTaskFailedCauseMap(FromGetTaskFailedCauseMap(item)))
	}
}

func TestCrossClusterApplyParentClosePolicyRequestAttributes(t *testing.T) {
	item := testdata.CrossClusterApplyParentClosePolicyRequestAttributes
	assert.Equal(
		t,
		&item,
		ToCrossClusterApplyParentClosePolicyRequestAttributes(
			FromCrossClusterApplyParentClosePolicyRequestAttributes(&item),
		),
	)
}

func TestApplyParentClosePolicyAttributes(t *testing.T) {
	item := testdata.ApplyParentClosePolicyAttributes
	assert.Equal(
		t,
		&item,
		ToApplyParentClosePolicyAttributes(
			FromApplyParentClosePolicyAttributes(&item),
		),
	)
}

func TestApplyParentClosePolicyResult(t *testing.T) {
	item := testdata.ApplyParentClosePolicyResult
	assert.Equal(
		t,
		&item,
		ToApplyParentClosePolicyResult(
			FromApplyParentClosePolicyResult(&item),
		),
	)
}

func TestCrossClusterApplyParentClosePolicyResponse(t *testing.T) {
	item := testdata.CrossClusterApplyParentClosePolicyResponseWithChildren
	assert.Equal(
		t,
		&item,
		ToCrossClusterApplyParentClosePolicyResponseAttributes(
			FromCrossClusterApplyParentClosePolicyResponseAttributes(&item),
		),
	)
}

func TestAny(t *testing.T) {
	t.Run("sanity check", func(t *testing.T) {
		internal := types.Any{
			ValueType: "testing",
			Value:     []byte(`test`),
		}
		rpc := sharedv1.Any{
			ValueType: "testing",
			Value:     []byte(`test`),
		}
		require.Equal(t, &rpc, FromAny(&internal))
		require.Equal(t, &internal, ToAny(&rpc))
	})

	t.Run("round trip nils", func(t *testing.T) {
		// somewhat annoying in fuzzing and there are few possibilities, so tested separately
		assert.Nil(t, FromAny(ToAny(nil)), "nil proto -> internal -> proto => should result in nil")
		assert.Nil(t, ToAny(FromAny(nil)), "nil internal -> proto -> internal => should result in nil")
	})

	t.Run("round trip from internal", func(t *testing.T) {
		testutils.EnsureFuzzCoverage(t, []string{
			"empty data", "filled data",
		}, func(t *testing.T, f *fuzz.Fuzzer) string {
			var orig types.Any
			f.Fuzz(&orig)
			out := ToAny(FromAny(&orig))
			assert.Equal(t, &orig, out, "did not survive round-tripping")
			// report what branch of behavior was fuzzed occurred
			if len(orig.Value) == 0 {
				return "empty data" // ignoring nil vs empty difference
			}
			return "filled data"
		})
	})
	t.Run("round trip from proto", func(t *testing.T) {
		testutils.EnsureFuzzCoverage(t, []string{
			"empty data", "filled data",
		}, func(t *testing.T, f *fuzz.Fuzzer) string {
			// unfortunately:
			// - gofuzz panics when it encounters interface fields (so this cannot be done on oneof fields)
			//   - not directly relevant for Any, but causes issues for fuzzing other types
			// - it populates the XXX_ fields and these can be hard to clear
			// so this is fuzz-filled by hand with specific fields rather than as a whole.
			var orig sharedv1.Any
			f.Fuzz(&orig.ValueType)
			f.Fuzz(&orig.Value)
			out := FromAny(ToAny(&orig))
			assert.Equal(t, &orig, out, "did not survive round-tripping")
			if len(orig.Value) == 0 {
				return "empty data" // ignoring nil vs empty difference
			}
			return "filled data"
		})
	})

	t.Run("can contain proto data", func(t *testing.T) {
		// pushing thrift or proto data through this type is very much expected,
		// so this is both evidence that it's possible and an example for how to do so.
		//
		// note that there is an equivalent test in mapper/thrift/shared_test.go.
		// they are structurally the same, but encoding/decoding details vary a bit.

		// some helpers because it's a bit verbose.
		//
		// note that these work for both types in this test: they can encode and decode *any* proto data,
		// you just have to make sure you pass the same type to both encode and decode via some other
		// source of info (i.e. the ValueType field).
		encode := func(thing proto.Marshaler) []byte {
			data, err := thing.Marshal()
			require.NoErrorf(t, err, "could not Marshal the target type: %T", thing)
			return data
		}
		decode := func(data []byte, target proto.Unmarshaler) {
			err := target.Unmarshal(data)
			require.NoErrorf(t, err, "could not Unmarshal to the target type: %T", target)
		}

		// --- create the original data, a proto object, and encode it by hand
		orig := &apiv1.WorkflowExecution{
			WorkflowId: testdata.WorkflowID,
			RunId:      testdata.RunID,
		}
		internalBytes := encode(orig)

		// --- put that data into the custom Any type
		// proto (unfortunately) maintains a type-registry, which means we can look up the unique name for this type...
		//
		// ...BUT this is potentially unsafe in normal code: proto type names can be changed without breaking binary compatibility.
		// we are unlikely to ever do so, but for this reason, hard-coding is actually a bit safer.
		var typeName = "uber.cadence.api.v1.WorkflowExecution" // current value of proto.MessageName(orig)
		anyVal := &types.Any{
			ValueType: typeName,
			Value:     internalBytes, // store proto bytes in the Any
		}

		// --- convert the whole container to proto (mimics making a call via yarpc)
		protoAny := FromAny(anyVal)      // we map to the rpc type
		networkBytes := encode(protoAny) // yarpc does this
		// ^ this is what's sent over the network.

		// as a side note:
		// the final data is not double-encoded, so this "encode -> wrap -> encode" process is reasonably efficient.
		//
		// Thrift and Proto can efficiently move around binary blobs like this, as it's essentially just a memcpy between
		// the input and the output, and there's no `\0` to `\\0` escaping or base64 encoding or whatever needed.
		//
		// no behavior depends on this, it's just presented here as evidence that this Any-wrapper does not meaningfully
		// change any RPC design concerns: anything you would do with normal RPC can be done through an Any if you need
		// loose typing, the change-stability / performance / etc is entirely unaffected.
		//
		// compare via a sliding window to find the place it overlaps, to prove that this is true:
		found := false
		for i := 0; i <= len(networkBytes)-len(internalBytes); i++ {
			if reflect.DeepEqual(internalBytes, networkBytes[i:i+len(internalBytes)]) {
				found = true
				t.Logf("Found matching bytes at index %v", i) // currently at index 41
			}
		}
		// *should* be true for efficiency's sake, but is not truly necessary for correct behavior
		assert.Truef(t, found, "did not find internal bytes within network bytes, might be paying double-encoding costs:\n\tinternal: %v\n\tnetwork:  %v", internalBytes, networkBytes)

		// --- the network pushes the data to a new location ---

		// --- on the receiving side, we map to internal types like normal
		var outAny sharedv1.Any
		decode(networkBytes, &outAny) // yarpc does this
		outAnyVal := ToAny(&outAny)   // we map to internal types

		// --- and finally decode the any-typed data by hand
		require.Equal(t, typeName, outAnyVal.ValueType, "type name through RPC should match the original type name")
		var outOrig apiv1.WorkflowExecution // selected based on the ValueType contents
		decode(outAnyVal.Value, &outOrig)   // do the actual custom decoding
		assert.NotEmpty(t, outOrig, "sanity check, decoded value should not be empty")
		assert.Equal(t, orig, &outOrig, "final round-tripped Any-contained data should be identical to the original object")
	})
}

func TestFromCrossClusterTaskResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromCrossClusterTaskResponse, ToCrossClusterTaskResponse,
		testutils.WithCustomFuncs(
			CrossClusterTaskResponseFuzzer,
			testutils.CrossClusterTaskTypeFuzzer,
			testutils.CrossClusterTaskFailedCauseFuzzer,
		),
	)
}

// CrossClusterTaskResponseFuzzer ensures only one attribute field is set based on TaskType (oneof constraint)
func CrossClusterTaskResponseFuzzer(r *types.CrossClusterTaskResponse, c fuzz.Continue) {
	c.FuzzNoCustom(r)

	// Ensure TaskType is set to a valid value (0-4) using the enum fuzzer
	if r.TaskType == nil {
		var taskType types.CrossClusterTaskType
		testutils.CrossClusterTaskTypeFuzzer(&taskType, c)
		r.TaskType = &taskType
	} else {
		testutils.CrossClusterTaskTypeFuzzer(r.TaskType, c)
	}

	// Ensure FailedCause is valid if set (0-5) using the enum fuzzer
	if r.FailedCause != nil {
		testutils.CrossClusterTaskFailedCauseFuzzer(r.FailedCause, c)
	}

	// Based on TaskType, clear all attributes except the matching one
	switch *r.TaskType {
	case 0: // StartChildExecution
		r.CancelExecutionAttributes = nil
		r.SignalExecutionAttributes = nil
		r.RecordChildWorkflowExecutionCompleteAttributes = nil
		r.ApplyParentClosePolicyAttributes = nil
	case 1: // CancelExecution
		r.StartChildExecutionAttributes = nil
		r.SignalExecutionAttributes = nil
		r.RecordChildWorkflowExecutionCompleteAttributes = nil
		r.ApplyParentClosePolicyAttributes = nil
	case 2: // SignalExecution
		r.StartChildExecutionAttributes = nil
		r.CancelExecutionAttributes = nil
		r.RecordChildWorkflowExecutionCompleteAttributes = nil
		r.ApplyParentClosePolicyAttributes = nil
	case 3: // RecordChildWorkflowExecutionComplete
		r.StartChildExecutionAttributes = nil
		r.CancelExecutionAttributes = nil
		r.SignalExecutionAttributes = nil
		r.ApplyParentClosePolicyAttributes = nil
	case 4: // ApplyParentClosePolicy
		r.StartChildExecutionAttributes = nil
		r.CancelExecutionAttributes = nil
		r.SignalExecutionAttributes = nil
		r.RecordChildWorkflowExecutionCompleteAttributes = nil
	}
}

func TestHostInfoFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHostInfo, ToHostInfo)
}

func TestMembershipInfoFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromMembershipInfo, ToMembershipInfo)
}

func TestDomainCacheInfoFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromDomainCacheInfo, ToDomainCacheInfo)
}

func TestRingInfoFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromRingInfo, ToRingInfo)
}

func TestVersionHistoriesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromVersionHistories, ToVersionHistories)
}

func TestVersionHistoryFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromVersionHistory, ToVersionHistory)
}

func TestVersionHistoryItemFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromVersionHistoryItem, ToVersionHistoryItem)
}

func TestPersistenceSettingFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromPersistenceSetting, ToPersistenceSetting)
}

func TestPersistenceFeatureFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromPersistenceFeature, ToPersistenceFeature)
}

func TestPersistenceInfoFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromPersistenceInfo, ToPersistenceInfo)
}

func TestCrossClusterTaskTypeFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromCrossClusterTaskType, ToCrossClusterTaskType,
		testutils.WithCustomFuncs(testutils.CrossClusterTaskTypeFuzzer),
	)
}

func TestGetTaskFailedCauseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromGetTaskFailedCause, ToGetTaskFailedCause,
		testutils.WithCustomFuncs(testutils.GetTaskFailedCauseFuzzer),
	)
}

func TestApplyParentClosePolicyStatusFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromApplyParentClosePolicyStatus, ToApplyParentClosePolicyStatus,
		testutils.WithCustomFuncs(testutils.CrossClusterTaskFailedCauseFuzzer),
	)
}

func TestDomainTaskAttributesFuzz(t *testing.T) {
	// Info, Config, and ReplicationConfig are built into the 'Domain' field.
	// This mapping is tested in TestDescribeDomainResponseDomainFuzz
	testutils.RunMapperFuzzTest(t, FromDomainTaskAttributes, ToDomainTaskAttributes,
		testutils.WithCustomFuncs(testutils.DomainOperationFuzzer, testutils.DomainStatusFuzzer),
		testutils.WithExcludedFields("Info", "Config", "ReplicationConfig"),
	)
}

func TestFailoverMarkerAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromFailoverMarkerAttributes, ToFailoverMarkerAttributes)
}

func TestFailoverMarkerTokenFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromFailoverMarkerToken, ToFailoverMarkerToken)
}

func TestHistoryTaskV2AttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryTaskV2Attributes, ToHistoryTaskV2Attributes,
		testutils.WithCustomFuncs(testutils.EncodingTypeFuzzer),
	)
}

func TestSyncActivityTaskAttributesFuzz(t *testing.T) {
	// [BUG] FailureDetails is not round-trip compatible
	testutils.RunMapperFuzzTest(t, FromSyncActivityTaskAttributes, ToSyncActivityTaskAttributes,
		testutils.WithCommonEnumFuzzers(),
		testutils.WithExcludedFields("LastFailureDetails"),
	)
}

func TestSyncShardStatusFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromSyncShardStatus, ToSyncShardStatus)
}

func TestSyncShardStatusTaskAttributesFuzz(t *testing.T) {
	// [BUG] ShardID gets truncated (int64 -> int32 issue)
	testutils.RunMapperFuzzTest(t, FromSyncShardStatusTaskAttributes, ToSyncShardStatusTaskAttributes,
		testutils.WithExcludedFields("ShardID"),
	)
}

func TestReplicationMessagesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromReplicationMessages, ToReplicationMessages,
		testutils.WithCustomFuncs(testutils.ReplicationTaskTypeFuzzer, testutils.DomainOperationFuzzer),
		testutils.WithExcludedFields("ReplicationTasks"), // Tested separately in TestReplicationTaskFuzz
	)
}

func TestReplicationTaskInfoFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromReplicationTaskInfo, ToReplicationTaskInfo)
}

func TestReplicationTokenFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromReplicationToken, ToReplicationToken)
}

func TestCrossClusterTaskInfoFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromCrossClusterTaskInfo, ToCrossClusterTaskInfo,
		testutils.WithCustomFuncs(
			testutils.CrossClusterTaskTypeFuzzer,
			testutils.CrossClusterTaskFailedCauseFuzzer,
		),
	)
}

func TestCrossClusterStartChildExecutionRequestAttributesFuzz(t *testing.T) {
	// [BUG] TargetRunID is not round-trip compatible, when it is set to an empty string, it becomes nil
	testutils.RunMapperFuzzTest(t, FromCrossClusterStartChildExecutionRequestAttributes, ToCrossClusterStartChildExecutionRequestAttributes,
		testutils.WithCommonEnumFuzzers(),
		testutils.WithExcludedFields("InitiatedEventAttributes", "TargetRunID"), // InitiatedEventAttributes is tested in FromStartChildWorkflowExecutionInitiatedEventAttributesFuzz
	)
}

func TestCrossClusterStartChildExecutionResponseAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromCrossClusterStartChildExecutionResponseAttributes, ToCrossClusterStartChildExecutionResponseAttributes)
}

func TestCrossClusterCancelExecutionRequestAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromCrossClusterCancelExecutionRequestAttributes, ToCrossClusterCancelExecutionRequestAttributes)
}

func TestCrossClusterCancelExecutionResponseAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromCrossClusterCancelExecutionResponseAttributes, ToCrossClusterCancelExecutionResponseAttributes)
}

func TestCrossClusterSignalExecutionRequestAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromCrossClusterSignalExecutionRequestAttributes, ToCrossClusterSignalExecutionRequestAttributes)
}

func TestCrossClusterSignalExecutionResponseAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromCrossClusterSignalExecutionResponseAttributes, ToCrossClusterSignalExecutionResponseAttributes)
}

func TestCrossClusterRecordChildWorkflowExecutionCompleteRequestAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromCrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes, ToCrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes,
		testutils.WithCommonEnumFuzzers(),
		testutils.WithExcludedFields("CompletionEvent"), // CompletionEvent uses FromHistoryEvent which is tested in TestHistoryEventFuzz
	)
}

func TestCrossClusterRecordChildWorkflowExecutionCompleteResponseAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromCrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes, ToCrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes)
}

func TestCrossClusterApplyParentClosePolicyResponseAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromCrossClusterApplyParentClosePolicyResponseAttributes, ToCrossClusterApplyParentClosePolicyResponseAttributes,
		testutils.WithCustomFuncs(testutils.CrossClusterTaskFailedCauseFuzzer),
	)
}

func TestApplyParentClosePolicyAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromApplyParentClosePolicyAttributes, ToApplyParentClosePolicyAttributes)
}

func TestApplyParentClosePolicyResultFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromApplyParentClosePolicyResult, ToApplyParentClosePolicyResult,
		testutils.WithCustomFuncs(testutils.CrossClusterTaskFailedCauseFuzzer),
	)
}

func TestCrossClusterTaskRequestFuzz(t *testing.T) {
	// This type has a oneof constraint, which doesn't lend itself well to fuzzing
	// All child attributes are tested separately, so we exclude them here and test only TaskInfo
	testutils.RunMapperFuzzTest(t, FromCrossClusterTaskRequest, ToCrossClusterTaskRequest,
		testutils.WithCustomFuncs(
			testutils.CrossClusterTaskTypeFuzzer,
		),
		testutils.WithExcludedFields(
			"StartChildExecutionAttributes",                  // Tested in TestCrossClusterStartChildExecutionRequestAttributesFuzz
			"CancelExecutionAttributes",                      // Tested in TestCrossClusterCancelExecutionRequestAttributesFuzz
			"SignalExecutionAttributes",                      // Tested in TestCrossClusterSignalExecutionRequestAttributesFuzz
			"RecordChildWorkflowExecutionCompleteAttributes", // Tested in TestCrossClusterRecordChildWorkflowExecutionCompleteRequestAttributesFuzz
			"ApplyParentClosePolicyAttributes",               // Tested in TestCrossClusterApplyParentClosePolicyRequestAttributesFuzz
		),
	)
}

func TestReplicationTaskFuzz(t *testing.T) {
	// This type has a oneof constraint, which doesn't lend itself well to fuzzing
	// All child attributes are tested separately, so we exclude them here and test only TaskInfo
	testutils.RunMapperFuzzTest(t, FromReplicationTask, ToReplicationTask,
		testutils.WithCustomFuncs(
			testutils.ReplicationTaskTypeFuzzer,
		),
		testutils.WithExcludedFields(
			"DomainTaskAttributes",          // Tested in TestDomainTaskAttributesFuzz
			"SyncShardStatusTaskAttributes", // Tested in TestSyncShardStatusTaskAttributesFuzz
			"SyncActivityTaskAttributes",    // Tested in TestSyncActivityTaskAttributesFuzz
			"HistoryTaskV2Attributes",       // Tested in TestHistoryTaskV2AttributesFuzz
			"FailoverMarkerAttributes",      // Tested in TestFailoverMarkerAttributesFuzz
		),
	)
}
