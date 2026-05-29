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

package pinot

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/constants"
	p "github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func TestConvertSearchResultToVisibilityRecord(t *testing.T) {
	columnName := []string{"WorkflowID", "RunID", "WorkflowType", "DomainID", "StartTime", "ExecutionTime", "CloseTime", "CloseStatus", "HistoryLength", "TaskList", "IsCron", "NumClusters", "UpdateTime", "Attr", "ClusterAttributeScope", "ClusterAttributeName"}
	closeStatus := types.WorkflowExecutionCloseStatusFailed

	sampleRawMemo := &types.Memo{
		Fields: map[string][]byte{
			`"Service"`: []byte(`"serverName1"`),
		},
	}
	serializer := p.NewPayloadSerializer()
	sampleEncodedMemo, err := serializer.SerializeVisibilityMemo(sampleRawMemo, constants.EncodingTypeThriftRW)
	assert.NoError(t, err)

	errorMapRaw1 := map[string]interface{}{"Memo": 123}
	errorMap1, err := json.Marshal(errorMapRaw1)

	errorMapRaw2 := map[string]interface{}{"Memo": "123"}
	errorMap2, err := json.Marshal(errorMapRaw2)

	tests := map[string]struct {
		inputColumnNames         []string
		inputHit                 []interface{}
		expectedVisibilityRecord *p.InternalVisibilityWorkflowExecutionInfo
		memoCheck                bool
		expectErr                error
	}{
		"Case1: nil result": {
			inputColumnNames:         nil,
			inputHit:                 []interface{}{"wfid", "rid", "wftype", "domainid", testEarliestTime, testEarliestTime, testLatestTime, -1, 1, "tsklst", true, 1, testEarliestTime, "{}", "region", "us-east"},
			expectedVisibilityRecord: nil,
			expectErr:                fmt.Errorf("length of hit (16) is not equal with length of columnNames(0)"),
		},
		"Case2-1: marshal system key error case": {
			inputColumnNames:         columnName,
			inputHit:                 []interface{}{"wfid", "rid", "wftype", "domainid", testEarliestTime, testEarliestTime, testLatestTime, 1, 1, "tsklst", true, 1, testEarliestTime, make(chan int), "region", "us-east"},
			expectedVisibilityRecord: nil,
			expectErr:                fmt.Errorf("unable to marshal systemKeyMap"),
		},
		"Case2-2: unmarshal system key error case": {
			inputColumnNames:         columnName,
			inputHit:                 []interface{}{"wfid", "rid", "wftype", "domainid", testEarliestTime, testEarliestTime, testLatestTime, 1, "1", "tsklst", true, 1, testEarliestTime, `{"CustomStringField": "customA and customB or customC", "CustomDoubleField": 3.14}`, "region", "us-east"},
			expectedVisibilityRecord: nil,
			expectErr:                fmt.Errorf("unable to Unmarshal systemKeyMap: json: cannot unmarshal string into Go struct field VisibilityRecord.HistoryLength of type int64"),
		},
		"Case2-3: Attr to string error case": {
			inputColumnNames:         []string{"Attr"},
			inputHit:                 []interface{}{123},
			expectedVisibilityRecord: nil,
			expectErr:                fmt.Errorf(`assertion error. Can't convert systemKeyMap[Attr] to string. Found int`),
		},
		"Case2-4: Attr unmarshal to map error case": {
			inputColumnNames:         []string{"Attr"},
			inputHit:                 []interface{}{"123"},
			expectedVisibilityRecord: nil,
			expectErr:                fmt.Errorf(`unable to Unmarshal searchAttribute map: json: cannot unmarshal number into Go value of type map[string]interface {}`),
		},
		"Case2-4: Memo to string error case": {
			inputColumnNames:         []string{"Attr"},
			inputHit:                 []interface{}{string(errorMap1)},
			expectedVisibilityRecord: nil,
			expectErr:                fmt.Errorf(`unable to convert memo: memoRaw is not a String: float64`),
		},
		"Case2-5: Memo unmarshal error case": {
			inputColumnNames:         []string{"Attr"},
			inputHit:                 []interface{}{string(errorMap2)},
			expectedVisibilityRecord: nil,
			expectErr:                fmt.Errorf(`unable to convert memo: unable to unmarshal memoRawStr: json: cannot unmarshal number into Go value of type persistence.DataBlob`),
		},
		"Case3-1: closed wf with everything except for an empty Attr": {
			inputColumnNames: columnName,
			inputHit:         []interface{}{"wfid", "rid", "wftype", "domainid", testEarliestTime, testEarliestTime, testLatestTime, -1, 1, "tsklst", true, 1, testEarliestTime, "{}", "region", "us-east"},
			expectedVisibilityRecord: &p.InternalVisibilityWorkflowExecutionInfo{
				DomainID:              "domainid",
				WorkflowType:          "wftype",
				WorkflowID:            "wfid",
				RunID:                 "rid",
				TypeName:              "wftype",
				StartTime:             time.UnixMilli(testEarliestTime),
				ExecutionTime:         time.UnixMilli(testEarliestTime),
				CloseTime:             time.UnixMilli(testLatestTime),
				Status:                nil,
				HistoryLength:         1,
				Memo:                  nil,
				TaskList:              "tsklst",
				IsCron:                true,
				NumClusters:           1,
				ClusterAttributeScope: "region",
				ClusterAttributeName:  "us-east",
				UpdateTime:            time.UnixMilli(testEarliestTime),
				SearchAttributes:      map[string]interface{}{},
				ShardID:               0,
			},
		},
		"Case3-2: closed wf with everything": {
			inputColumnNames: columnName,
			inputHit:         []interface{}{"wfid", "rid", "wftype", "domainid", testEarliestTime, testEarliestTime, testLatestTime, 1, 1, "tsklst", true, 1, testEarliestTime, `{"CustomStringField": "customA and customB or customC", "CustomDoubleField": 3.14}`, "region", "us-east"},
			expectedVisibilityRecord: &p.InternalVisibilityWorkflowExecutionInfo{
				DomainID:              "domainid",
				WorkflowType:          "wftype",
				WorkflowID:            "wfid",
				RunID:                 "rid",
				TypeName:              "wftype",
				StartTime:             time.UnixMilli(testEarliestTime),
				ExecutionTime:         time.UnixMilli(testEarliestTime),
				CloseTime:             time.UnixMilli(testLatestTime),
				Status:                &closeStatus,
				HistoryLength:         1,
				Memo:                  nil,
				TaskList:              "tsklst",
				IsCron:                true,
				NumClusters:           1,
				ClusterAttributeScope: "region",
				ClusterAttributeName:  "us-east",
				UpdateTime:            time.UnixMilli(testEarliestTime),
				SearchAttributes:      map[string]interface{}{"CustomStringField": "customA and customB or customC", "CustomDoubleField": 3.14},
				ShardID:               0,
			},
		},
		"Case4: open wf with everything": {
			inputColumnNames: columnName,
			inputHit: []interface{}{"wfid", "rid", "wftype", "domainid", testEarliestTime, testEarliestTime, -1, -1, -1,
				"tsklst", true, 1, testEarliestTime, `{"CustomStringField": "customA and customB or customC", "CustomDoubleField": 3.14}`, "region", "us-east"},
			expectedVisibilityRecord: &p.InternalVisibilityWorkflowExecutionInfo{
				DomainID:              "domainid",
				WorkflowType:          "wftype",
				WorkflowID:            "wfid",
				RunID:                 "rid",
				TypeName:              "wftype",
				StartTime:             time.UnixMilli(testEarliestTime),
				ExecutionTime:         time.UnixMilli(testEarliestTime),
				Memo:                  nil,
				TaskList:              "tsklst",
				IsCron:                true,
				NumClusters:           1,
				ClusterAttributeScope: "region",
				ClusterAttributeName:  "us-east",
				UpdateTime:            time.UnixMilli(testEarliestTime),
				SearchAttributes:      map[string]interface{}{"CustomStringField": "customA and customB or customC", "CustomDoubleField": 3.14},
				ShardID:               0,
			},
			memoCheck: true,
		},
		"Case5-1: open wf with memo": {
			inputColumnNames: columnName,
			inputHit: []interface{}{"wfid", "rid", "wftype", "domainid", testEarliestTime, testEarliestTime, -1, -1, -1,
				"tsklst", true, 1, testEarliestTime,
				`{"Memo":"{\"Encoding\":\"thriftrw\",\"Data\":\"WQ0ACgsLAAAAAQAAAAkiU2VydmljZSIAAAANInNlcnZlck5hbWUxIgA=\"}"}`, "region", "us-east"},
			expectedVisibilityRecord: &p.InternalVisibilityWorkflowExecutionInfo{
				DomainID:              "domainid",
				WorkflowType:          "wftype",
				WorkflowID:            "wfid",
				RunID:                 "rid",
				TypeName:              "wftype",
				StartTime:             time.UnixMilli(testEarliestTime),
				ExecutionTime:         time.UnixMilli(testEarliestTime),
				Memo:                  sampleEncodedMemo,
				TaskList:              "tsklst",
				IsCron:                true,
				NumClusters:           1,
				ClusterAttributeScope: "region",
				ClusterAttributeName:  "us-east",
				UpdateTime:            time.UnixMilli(testEarliestTime),
				SearchAttributes:      map[string]interface{}{"CustomStringField": "customA and customB or customC", "CustomDoubleField": 3.14},
				ShardID:               0,
			},
			memoCheck: true,
		},
		"Case5-2: open wf with empty memo": {
			inputColumnNames: columnName,
			inputHit: []interface{}{"wfid", "rid", "wftype", "domainid", testEarliestTime, testEarliestTime, -1, -1, -1,
				"tsklst", true, 1, testEarliestTime,
				`{"Memo":"{\"Encoding\":\"\",\"Data\":\"\"}"}`, "region", "us-east"},
			expectedVisibilityRecord: &p.InternalVisibilityWorkflowExecutionInfo{
				DomainID:              "domainid",
				WorkflowType:          "wftype",
				WorkflowID:            "wfid",
				RunID:                 "rid",
				TypeName:              "wftype",
				StartTime:             time.UnixMilli(testEarliestTime),
				ExecutionTime:         time.UnixMilli(testEarliestTime),
				Memo:                  nil,
				TaskList:              "tsklst",
				IsCron:                true,
				NumClusters:           1,
				ClusterAttributeScope: "region",
				ClusterAttributeName:  "us-east",
				UpdateTime:            time.UnixMilli(testEarliestTime),
				SearchAttributes:      map[string]interface{}{"CustomStringField": "customA and customB or customC", "CustomDoubleField": 3.14},
				ShardID:               0,
			},
			memoCheck: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				visibilityRecord, err := ConvertSearchResultToVisibilityRecord(test.inputHit, test.inputColumnNames)
				if !test.memoCheck {
					assert.Equal(t, test.expectedVisibilityRecord, visibilityRecord)
					assert.Equal(t, test.expectErr, err)
				} else {
					assert.Equal(t, test.expectedVisibilityRecord.Memo.GetData(), visibilityRecord.Memo.GetData())
					assert.Equal(t, test.expectedVisibilityRecord.Memo.GetEncoding(), visibilityRecord.Memo.GetEncoding())
				}
			})
		})
	}
}

// This is the process of figuring out how to encode/decode memo for Pinot
func TestDeserializeMemoMockingE2E(t *testing.T) {
	sampleRawMemo := &types.Memo{
		Fields: map[string][]byte{
			"Service": []byte("serverName1"),
		},
	}
	serializer := p.NewPayloadSerializer()
	sampleEncodedMemo, err := serializer.SerializeVisibilityMemo(sampleRawMemo, constants.EncodingTypeThriftRW)
	assert.NoError(t, err)
	// not a human-readable string
	assert.Equal(t, "Y\r\x00\n\v\v\x00\x00\x00\x01\x00\x00\x00\aService\x00\x00\x00\vserverName1\x00", string(sampleEncodedMemo.GetData()))

	marshaledMemo, err := json.Marshal(sampleEncodedMemo)
	assert.NoError(t, err)
	// after marshal, data becomes a human-readable char array
	assert.Equal(t, `{"Encoding":"thriftrw","Data":"WQ0ACgsLAAAAAQAAAAdTZXJ2aWNlAAAAC3NlcnZlck5hbWUxAA=="}`, string(marshaledMemo))

	// must-do step, to give it a type, or we can't convert it to a string in the reading side.
	marshaledMemoStr := string(marshaledMemo)

	// mock the reading side
	unmarshaledRawData := p.DataBlob{}

	// marshaledMemoStr still knows that it is a DataBlob type
	err = json.Unmarshal([]byte(marshaledMemoStr), &unmarshaledRawData)
	assert.NoError(t, err)
	sampleDecodedMemo, err := serializer.DeserializeVisibilityMemo(&unmarshaledRawData)
	assert.NoError(t, err)
	assert.Equal(t, "serverName1", string(sampleDecodedMemo.Fields["Service"]))
}
