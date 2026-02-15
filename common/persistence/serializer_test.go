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

package persistence

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/checksum"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/types"
)

type testDef struct {
	name string
	// payloads is a map of payload name to payload. "nil" is a special name for nil payload which expects to return nil with some exceptions
	payloads      map[string]any
	nilHandled    bool
	serializeFn   func(any, constants.EncodingType) (*DataBlob, error)
	deserializeFn func(*DataBlob) (any, error)
}

// key is encoding type, value is whether the encoding type is supported
var encodingTypes = map[constants.EncodingType]bool{
	constants.EncodingTypeEmpty:    true,
	constants.EncodingTypeUnknown:  true,
	constants.EncodingTypeJSON:     true,
	constants.EncodingTypeThriftRW: true,
	constants.EncodingTypeGob:      false,
}

type runnableTest struct {
	testDef
	encoding    constants.EncodingType
	supported   bool
	payloadName string
	payload     any
}

func TestSerializers(t *testing.T) {
	// create serializer once and reuse it for all test cases in parallel to catch concurrency issues
	serializer := NewPayloadSerializer()

	tests := []testDef{
		{
			name: "history event",
			payloads: map[string]any{
				"nil":    (*types.HistoryEvent)(nil),
				"normal": generateTestHistoryEvent(1),
			},
			serializeFn: func(payload any, encoding constants.EncodingType) (*DataBlob, error) {
				return serializer.SerializeEvent(payload.(*types.HistoryEvent), encoding)
			},
			deserializeFn: func(data *DataBlob) (any, error) {
				return serializer.DeserializeEvent(data)
			},
		},
		{
			name: "batch history events",
			payloads: map[string]any{
				"nil":    ([]*types.HistoryEvent)(nil),
				"normal": generateTestHistoryEventBatch(),
			},
			nilHandled: true,
			serializeFn: func(payload any, encoding constants.EncodingType) (*DataBlob, error) {
				return serializer.SerializeBatchEvents(payload.([]*types.HistoryEvent), encoding)
			},
			deserializeFn: func(data *DataBlob) (any, error) {
				return serializer.DeserializeBatchEvents(data)
			},
		},
		{
			name: "visibility memo",
			payloads: map[string]any{
				"nil":    (*types.Memo)(nil),
				"normal": generateVisibilityMemo(),
			},
			serializeFn: func(payload any, encoding constants.EncodingType) (*DataBlob, error) {
				return serializer.SerializeVisibilityMemo(payload.(*types.Memo), encoding)
			},
			deserializeFn: func(data *DataBlob) (any, error) {
				return serializer.DeserializeVisibilityMemo(data)
			},
		},
		{
			name: "version histories",
			payloads: map[string]any{
				"nil":    (*types.VersionHistories)(nil),
				"normal": generateVersionHistories(),
			},
			serializeFn: func(payload any, encoding constants.EncodingType) (*DataBlob, error) {
				return serializer.SerializeVersionHistories(payload.(*types.VersionHistories), encoding)
			},
			deserializeFn: func(data *DataBlob) (any, error) {
				return serializer.DeserializeVersionHistories(data)
			},
		},
		{
			name: "reset points",
			payloads: map[string]any{
				"nil":    (*types.ResetPoints)(nil),
				"normal": generateResetPoints(),
			},
			nilHandled: true,
			serializeFn: func(payload any, encoding constants.EncodingType) (*DataBlob, error) {
				return serializer.SerializeResetPoints(payload.(*types.ResetPoints), encoding)
			},
			deserializeFn: func(data *DataBlob) (any, error) {
				return serializer.DeserializeResetPoints(data)
			},
		},
		{
			name: "bad binaries",
			payloads: map[string]any{
				"nil":    (*types.BadBinaries)(nil),
				"normal": generateBadBinaries(),
			},
			nilHandled: true,
			serializeFn: func(payload any, encoding constants.EncodingType) (*DataBlob, error) {
				return serializer.SerializeBadBinaries(payload.(*types.BadBinaries), encoding)
			},
			deserializeFn: func(data *DataBlob) (any, error) {
				return serializer.DeserializeBadBinaries(data)
			},
		},
		{
			name: "processing queue states",
			payloads: map[string]any{
				"nil":    (*types.ProcessingQueueStates)(nil),
				"normal": generateProcessingQueueStates(),
			},
			serializeFn: func(payload any, encoding constants.EncodingType) (*DataBlob, error) {
				return serializer.SerializeProcessingQueueStates(payload.(*types.ProcessingQueueStates), encoding)
			},
			deserializeFn: func(data *DataBlob) (any, error) {
				return serializer.DeserializeProcessingQueueStates(data)
			},
		},
		{
			name: "dynamic config blob",
			payloads: map[string]any{
				"nil":    (*types.DynamicConfigBlob)(nil),
				"normal": generateDynamicConfigBlob(),
			},
			serializeFn: func(payload any, encoding constants.EncodingType) (*DataBlob, error) {
				return serializer.SerializeDynamicConfigBlob(payload.(*types.DynamicConfigBlob), encoding)
			},
			deserializeFn: func(data *DataBlob) (any, error) {
				return serializer.DeserializeDynamicConfigBlob(data)
			},
		},
		{
			name: "isolation group",
			payloads: map[string]any{
				"nil":    (*types.IsolationGroupConfiguration)(nil),
				"normal": generateIsolationGroupConfiguration(),
			},
			serializeFn: func(payload any, encoding constants.EncodingType) (*DataBlob, error) {
				return serializer.SerializeIsolationGroups(payload.(*types.IsolationGroupConfiguration), encoding)
			},
			deserializeFn: func(data *DataBlob) (any, error) {
				return serializer.DeserializeIsolationGroups(data)
			},
		},
		{
			name: "failover markers",
			payloads: map[string]any{
				"nil":    ([]*types.FailoverMarkerAttributes)(nil),
				"normal": generateFailoverMarkerAttributes(),
			},
			serializeFn: func(payload any, encoding constants.EncodingType) (*DataBlob, error) {
				return serializer.SerializePendingFailoverMarkers(payload.([]*types.FailoverMarkerAttributes), encoding)
			},
			deserializeFn: func(data *DataBlob) (any, error) {
				return serializer.DeserializePendingFailoverMarkers(data)
			},
		},
		{
			name: "async workflow config",
			payloads: map[string]any{
				"nil":    (*types.AsyncWorkflowConfiguration)(nil),
				"normal": generateAsyncWorkflowConfig(),
			},
			serializeFn: func(payload any, encoding constants.EncodingType) (*DataBlob, error) {
				return serializer.SerializeAsyncWorkflowsConfig(payload.(*types.AsyncWorkflowConfiguration), encoding)
			},
			deserializeFn: func(data *DataBlob) (any, error) {
				return serializer.DeserializeAsyncWorkflowsConfig(data)
			},
		},
		{
			name: "checksum",
			payloads: map[string]any{
				"empty":  checksum.Checksum{},
				"normal": generateChecksum(),
			},
			serializeFn: func(payload any, encoding constants.EncodingType) (*DataBlob, error) {
				return serializer.SerializeChecksum(payload.(checksum.Checksum), encoding)
			},
			deserializeFn: func(data *DataBlob) (any, error) {
				return serializer.DeserializeChecksum(data)
			},
		},
		{
			name: "active clusters",
			payloads: map[string]any{
				"nil":    (*types.ActiveClusters)(nil),
				"normal": generateActiveClusters(),
			},
			serializeFn: func(payload any, encoding constants.EncodingType) (*DataBlob, error) {
				return serializer.SerializeActiveClusters(payload.(*types.ActiveClusters), encoding)
			},
			deserializeFn: func(data *DataBlob) (any, error) {
				return serializer.DeserializeActiveClusters(data)
			},
		},
		{
			name: "active cluster selection policy",
			payloads: map[string]any{
				"nil":    (*types.ActiveClusterSelectionPolicy)(nil),
				"normal": generateActiveClusterSelectionPolicy(),
			},
			serializeFn: func(payload any, encoding constants.EncodingType) (*DataBlob, error) {
				return serializer.SerializeActiveClusterSelectionPolicy(payload.(*types.ActiveClusterSelectionPolicy), encoding)
			},
			deserializeFn: func(data *DataBlob) (any, error) {
				return serializer.DeserializeActiveClusterSelectionPolicy(data)
			},
		},
	}

	// generate runnable test cases here so actual test body is not 3 level nested
	var runnableTests []runnableTest
	for _, td := range tests {
		for encoding, supported := range encodingTypes {
			for payloadName, payload := range td.payloads {
				if _, ok := payload.(checksum.Checksum); ok {
					if encoding != constants.EncodingTypeJSON {
						continue
					}
				}
				runnableTests = append(runnableTests, runnableTest{
					testDef:     td,
					encoding:    encoding,
					supported:   supported,
					payloadName: payloadName,
					payload:     payload,
				})
			}
		}
	}

	for _, tc := range runnableTests {
		t.Run(fmt.Sprintf("%s with encoding:%s,payload:%s", tc.name, tc.encoding, tc.payloadName), func(t *testing.T) {
			t.Parallel()

			serialized, err := tc.serializeFn(tc.payload, tc.encoding)
			// expect error if the encoding type is not supported. special case is nil payloads.
			// most of the serialization functions return early for nils but
			// some of them call underlying helper function which checks encoding type and raises error.
			wantErr := !tc.supported && !(tc.payloadName == "nil" && !tc.nilHandled)
			if wantErr != (err != nil) {
				t.Fatalf("Got serialization err: %v, want err?: %v", err, wantErr)
			}
			if err != nil {
				return
			}

			deserialized, err := tc.deserializeFn(serialized)
			if err != nil {
				t.Fatalf("Got serialization err: %v", err)
			}

			if deserialized == nil {
				t.Fatalf("Got nil deserialized payload")
			}

			if tc.payloadName == "nil" {
				return
			}
		})
	}
}

func TestDataBlob_GetData(t *testing.T) {
	tests := map[string]struct {
		in          *DataBlob
		expectedOut []byte
	}{
		"valid data": {
			in:          &DataBlob{Data: []byte("dat")},
			expectedOut: []byte("dat"),
		},
		"empty data": {
			in:          &DataBlob{Data: nil},
			expectedOut: []byte{},
		},
		"empty data 2": {
			in:          nil,
			expectedOut: []byte{},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, td.expectedOut, td.in.GetData())
		})
	}
}

func generateTestHistoryEvent(id int64) *types.HistoryEvent {
	return &types.HistoryEvent{
		ID:        id,
		Timestamp: common.Int64Ptr(time.Now().UnixNano()),
		EventType: types.EventTypeActivityTaskCompleted.Ptr(),
		ActivityTaskCompletedEventAttributes: &types.ActivityTaskCompletedEventAttributes{
			Result:           []byte("result-1-event-1"),
			ScheduledEventID: 4,
			StartedEventID:   5,
			Identity:         "event-1",
		},
	}
}

func generateTestHistoryEventBatch() []*types.HistoryEvent {
	return []*types.HistoryEvent{
		generateTestHistoryEvent(111),
		generateTestHistoryEvent(112),
		generateTestHistoryEvent(113),
	}
}

func generateVisibilityMemo() *types.Memo {
	memoFields := map[string][]byte{
		"TestField": []byte(`Test binary`),
	}
	return &types.Memo{Fields: memoFields}
}

func generateVersionHistories() *types.VersionHistories {
	return &types.VersionHistories{
		Histories: []*types.VersionHistory{
			{
				BranchToken: []byte{1},
				Items: []*types.VersionHistoryItem{
					{
						EventID: 1,
						Version: 0,
					},
					{
						EventID: 2,
						Version: 1,
					},
				},
			},
			{
				BranchToken: []byte{2},
				Items: []*types.VersionHistoryItem{
					{
						EventID: 2,
						Version: 0,
					},
					{
						EventID: 3,
						Version: 1,
					},
				},
			},
		},
	}
}

func generateResetPoints() *types.ResetPoints {
	return &types.ResetPoints{
		Points: []*types.ResetPointInfo{
			{
				BinaryChecksum:           "bad-binary-cs",
				RunID:                    "test-run-id",
				FirstDecisionCompletedID: 123,
				CreatedTimeNano:          common.Int64Ptr(456),
				ExpiringTimeNano:         common.Int64Ptr(789),
				Resettable:               true,
			},
		},
	}
}

func generateBadBinaries() *types.BadBinaries {
	return &types.BadBinaries{
		Binaries: map[string]*types.BadBinaryInfo{
			"bad-binary-cs": {
				CreatedTimeNano: common.Int64Ptr(456),
				Operator:        "test-operattor",
				Reason:          "test-reason",
			},
		},
	}
}

func generateProcessingQueueStates() *types.ProcessingQueueStates {
	return &types.ProcessingQueueStates{
		StatesByCluster: map[string][]*types.ProcessingQueueState{
			"cluster1": {
				&types.ProcessingQueueState{
					Level:    common.Int32Ptr(0),
					AckLevel: common.Int64Ptr(1),
					MaxLevel: common.Int64Ptr(2),
					DomainFilter: &types.DomainFilter{
						DomainIDs:    []string{"domain1", "domain2"},
						ReverseMatch: true,
					},
				},
			},
			"cluster2": {
				&types.ProcessingQueueState{
					Level:    common.Int32Ptr(3),
					AckLevel: common.Int64Ptr(4),
					MaxLevel: common.Int64Ptr(5),
					DomainFilter: &types.DomainFilter{
						DomainIDs:    []string{"domain3", "domain4"},
						ReverseMatch: true,
					},
				},
			},
		},
	}
}

func generateDynamicConfigBlob() *types.DynamicConfigBlob {
	return &types.DynamicConfigBlob{
		SchemaVersion: 1,
		Entries: []*types.DynamicConfigEntry{
			{
				Name: dynamicproperties.TestGetBoolPropertyKey.String(),
				Values: []*types.DynamicConfigValue{
					{
						Value: &types.DataBlob{
							EncodingType: types.EncodingTypeJSON.Ptr(),
							Data:         []byte("false"),
						},
						Filters: nil,
					},
				},
			},
		},
	}
}

func generateIsolationGroupConfiguration() *types.IsolationGroupConfiguration {
	return &types.IsolationGroupConfiguration{
		"zone-1": {
			Name:  "zone-1",
			State: types.IsolationGroupStateDrained,
		},
		"zone-2": {
			Name:  "zone-2",
			State: types.IsolationGroupStateHealthy,
		},
	}
}

func generateFailoverMarkerAttributes() []*types.FailoverMarkerAttributes {
	return []*types.FailoverMarkerAttributes{
		{
			DomainID:        "domain1",
			FailoverVersion: 123,
			CreationTime:    common.Int64Ptr(time.Now().UnixNano()),
		},
		{
			DomainID:        "domain2",
			FailoverVersion: 456,
			CreationTime:    common.Int64Ptr(time.Now().UnixNano()),
		},
	}
}

func generateAsyncWorkflowConfig() *types.AsyncWorkflowConfiguration {
	return &types.AsyncWorkflowConfiguration{
		Enabled:             true,
		PredefinedQueueName: "test-queue",
		QueueType:           "kafka",
		QueueConfig: &types.DataBlob{
			EncodingType: types.EncodingTypeJSON.Ptr(),
			Data:         []byte(`{"key":"value"}`),
		},
	}
}

func generateChecksum() checksum.Checksum {
	return checksum.Checksum{
		Flavor:  checksum.FlavorIEEECRC32OverThriftBinary,
		Version: 1,
		Value:   []byte("test-checksum"),
	}
}

func generateActiveClusters() *types.ActiveClusters {
	return &types.ActiveClusters{
		AttributeScopes: map[string]types.ClusterAttributeScope{
			"region": {
				ClusterAttributes: map[string]types.ActiveClusterInfo{
					"region1": {
						ActiveClusterName: "cluster1",
						FailoverVersion:   2,
					},
					"region2": {
						ActiveClusterName: "cluster2",
						FailoverVersion:   3,
					},
				},
			},
		},
	}
}

func generateActiveClusterSelectionPolicy() *types.ActiveClusterSelectionPolicy {
	return &types.ActiveClusterSelectionPolicy{
		ActiveClusterSelectionStrategy: types.ActiveClusterSelectionStrategyRegionSticky.Ptr(),
		StickyRegion:                   "region1",
		ExternalEntityType:             "externalEntityType1",
		ExternalEntityKey:              "externalEntityKey1",
	}
}
