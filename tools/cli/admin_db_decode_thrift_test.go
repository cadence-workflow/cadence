// The MIT License (MIT)
//
// Copyright (c) 2020 Uber Technologies, Inc.
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

package cli

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/uber/cadence/.gen/go/history"
	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/.gen/go/sqlblobs"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/codec"
)

func TestThriftDecodeHelper(t *testing.T) {
	tests := []struct {
		desc      string
		input     string
		encoding  string
		wantObjFn func(*testing.T) codec.ThriftObject
		wantErr   bool
	}{
		{
			desc:     "invalid base64 input",
			input:    "not-a-valid-base64",
			encoding: "base64",
			wantErr:  true,
		},
		{
			desc:     "invalid hex input",
			input:    "not-a-valid-hex",
			encoding: "hex",
			wantErr:  true,
		},
		{
			desc:      "VersionHistories hex with '0x' prefix",
			input:     "0x5908000a000000000f00140c000000010b000a0000004c590b000a000000067472656569640b0014000000086272616e636869640f001e0c000000010b000a000000086272616e636869640a001400000000000000010a001e000000000000006400000000",
			encoding:  "hex",
			wantObjFn: generateTestVersionHistories,
		},
		{
			desc:      "VersionHistories hex without '0x' prefix",
			input:     "5908000a000000000f00140c000000010b000a0000004c590b000a000000067472656569640b0014000000086272616e636869640f001e0c000000010b000a000000086272616e636869640a001400000000000000010a001e000000000000006400000000",
			encoding:  "hex",
			wantObjFn: generateTestVersionHistories,
		},
		{
			desc:      "VersionHistories base64",
			input:     "WQgACgAAAAAPABQMAAAAAQsACgAAAExZCwAKAAAABnRyZWVpZAsAFAAAAAhicmFuY2hpZA8AHgwAAAABCwAKAAAACGJyYW5jaGlkCgAUAAAAAAAAAAEKAB4AAAAAAAAAZAAAAAA=",
			encoding:  "base64",
			wantObjFn: generateTestVersionHistories,
		},
		{
			desc:      "ResetPoints hex",
			input:     "590f000a0c000000010b000a00000008636865636b73756d0b00140000000572756e69640a001e00000000000000010a002800000000000000010a0032000000000000000102003c010000",
			encoding:  "hex",
			wantObjFn: generateTestResetPoints,
		},
		{
			desc:      "ProcessingQueueStates hex",
			input:     "590d000a0b0f0000000100000008636c7573746572310c0000000208000a000000000a001400000000000003e80a001e00000000000007d00c00280f000a0b0000000100000006646f6d61696e000008000a000000010a0014000000000000012c0a001e00000000000001900c00280f000a0b0000000100000006646f6d61696e000000",
			encoding:  "hex",
			wantObjFn: generateTestProcessingQueueStates,
		},
		{
			desc:      "DomainInfo hex",
			input:     "590b000a000000046e616d650b000c0000000b6465736372697074696f6e0b000e000000056f776e65720800100000000306001200070d00260b0b0000000100000007646174616b6579000000096461746176616c756500",
			encoding:  "hex",
			wantObjFn: generateTestDomainInfo,
		},
		{
			desc:      "HistoryTreeInfo hex",
			input:     "590a000a0000000218711a000f000c0c000000010b000a000000086272616e636869640a001400000000000000010a001e0000000000000064000b000e00000009736f6d6520696e666f00",
			encoding:  "hex",
			wantObjFn: generateTestHistoryTreeInfo,
		},
		{
			desc:      "TimerInfo hex",
			input:     "590a000a00000000000000010a000c00000000000000010a000e00000000000003e80a0010000000000000000500",
			encoding:  "hex",
			wantObjFn: generateTestTimerInfo,
		},
		{
			desc:      "Active clusters config",
			input:     "0x590d000b0b0c00000001000000086c6f636174696f6e0d000a0b0c00000001000000066c6f6e646f6e0b000a00000008636c7573746572320a00140000000000000002000000",
			encoding:  "hex",
			wantObjFn: generateTestActiveClustersConfig,
		},
		{
			desc:      "Active cluster selection policy",
			input:     "5908000a000000000b001400000007726567696f6e300b001e000000000b00280000000000",
			encoding:  "hex",
			wantObjFn: generateTestActiveClusterSelectionPolicy,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			data, err := decodeUserInput(tc.input, tc.encoding)
			if (err != nil) != tc.wantErr {
				t.Fatalf("decodeUserInput() error: %v, wantErr: %v", err, tc.wantErr)
			}
			if err != nil {
				return
			}

			gotObj, decodeErr := decodeThriftPayload(data)
			if (decodeErr != nil) != tc.wantErr {
				t.Fatalf("decodeUserInput() error: %v, wantErr: %v", decodeErr, tc.wantErr)
			}
			if decodeErr != nil {
				return
			}

			want := tc.wantObjFn(t)

			if diff := cmp.Diff(want, gotObj); diff != "" {
				t.Fatalf("Object mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func mustThriftEncode(t *testing.T, obj codec.ThriftObject) []byte {
	t.Helper()
	data, err := codec.NewThriftRWEncoder().Encode(obj)
	if err != nil {
		t.Fatalf("Failed to encode thrift obj, err: %v", err)
	}
	return data
}

func generateTestVersionHistories(t *testing.T) codec.ThriftObject {
	t.Helper()
	historyBranch := &shared.HistoryBranch{
		TreeID:   common.StringPtr("treeid"),
		BranchID: common.StringPtr("branchid"),
		Ancestors: []*shared.HistoryBranchRange{
			{
				BranchID:    common.StringPtr("branchid"),
				BeginNodeID: common.Int64Ptr(1),
				EndNodeID:   common.Int64Ptr(100),
			},
		},
	}
	return &shared.VersionHistories{
		CurrentVersionHistoryIndex: common.Int32Ptr(0),
		Histories: []*shared.VersionHistory{
			{
				BranchToken: mustThriftEncode(t, historyBranch),
			},
		},
	}
}

func generateTestResetPoints(t *testing.T) codec.ThriftObject {
	t.Helper()
	return &shared.ResetPoints{
		Points: []*shared.ResetPointInfo{
			{
				BinaryChecksum:           common.StringPtr("checksum"),
				RunId:                    common.StringPtr("runid"),
				FirstDecisionCompletedId: common.Int64Ptr(1),
				CreatedTimeNano:          common.Int64Ptr(1),
				Resettable:               common.BoolPtr(true),
				ExpiringTimeNano:         common.Int64Ptr(1),
			},
		},
	}
}

func generateTestProcessingQueueStates(t *testing.T) codec.ThriftObject {
	t.Helper()
	return &history.ProcessingQueueStates{
		StatesByCluster: map[string][]*history.ProcessingQueueState{
			"cluster1": {
				{
					Level:    common.Int32Ptr(0),
					AckLevel: common.Int64Ptr(1000),
					MaxLevel: common.Int64Ptr(2000),
					DomainFilter: &history.DomainFilter{
						DomainIDs: []string{"domain"},
					},
				},
				{
					Level:    common.Int32Ptr(1),
					AckLevel: common.Int64Ptr(300),
					MaxLevel: common.Int64Ptr(400),
					DomainFilter: &history.DomainFilter{
						DomainIDs: []string{"domain"},
					},
				},
			},
		},
	}
}

func generateTestDomainInfo(t *testing.T) codec.ThriftObject {
	t.Helper()
	return &sqlblobs.DomainInfo{
		Name:          common.StringPtr("name"),
		Owner:         common.StringPtr("owner"),
		Status:        common.Int32Ptr(3),
		RetentionDays: common.Int16Ptr(7),
		Description:   common.StringPtr("description"),
		Data: map[string]string{
			"datakey": "datavalue",
		},
	}
}

func generateTestHistoryTreeInfo(t *testing.T) codec.ThriftObject {
	t.Helper()
	return &sqlblobs.HistoryTreeInfo{
		CreatedTimeNanos: common.Int64Ptr(9000000000),
		Ancestors: []*shared.HistoryBranchRange{
			{
				BranchID:    common.StringPtr("branchid"),
				BeginNodeID: common.Int64Ptr(1),
				EndNodeID:   common.Int64Ptr(100),
			},
		},
		Info: common.StringPtr("some info"),
	}
}

func generateTestTimerInfo(t *testing.T) codec.ThriftObject {
	t.Helper()
	return &sqlblobs.TimerInfo{
		Version:         common.Int64Ptr(1),
		StartedID:       common.Int64Ptr(1),
		ExpiryTimeNanos: common.Int64Ptr(1000),
		TaskID:          common.Int64Ptr(5),
	}
}

func generateTestActiveClusterSelectionPolicy(t *testing.T) codec.ThriftObject {
	t.Helper()
	return &shared.ActiveClusterSelectionPolicy{
		Strategy:           shared.ActiveClusterSelectionStrategyRegionSticky.Ptr(),
		StickyRegion:       common.StringPtr("region0"),
		ExternalEntityType: common.StringPtr(""),
		ExternalEntityKey:  common.StringPtr(""),
	}
}

func generateTestActiveClustersConfig(t *testing.T) codec.ThriftObject {
	t.Helper()
	return &shared.ActiveClusters{
		ActiveClustersByClusterAttribute: map[string]*shared.ClusterAttributeScope{
			"location": &shared.ClusterAttributeScope{
				ClusterAttributes: map[string]*shared.ActiveClusterInfo{
					"london": {
						ActiveClusterName: common.Ptr("cluster2"),
						FailoverVersion:   common.Int64Ptr(2),
					},
				},
			},
		},
	}
}
