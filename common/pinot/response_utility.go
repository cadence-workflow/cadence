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
	"time"

	p "github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

const (
	Memo = "Memo"
	Attr = "Attr"
)

func buildMap(hit []interface{}, columnNames []string) map[string]interface{} {
	systemKeyMap := make(map[string]interface{})

	for i := 0; i < len(columnNames); i++ {
		key := columnNames[i]
		systemKeyMap[key] = hit[i]
	}

	return systemKeyMap
}

// VisibilityRecord is a struct of doc for deserialization
// this is different from InternalVisibilityWorkflowExecutionInfo
// use this to deserialize the systemKeyMap from Pinot response
type VisibilityRecord struct {
	WorkflowID            string
	RunID                 string
	WorkflowType          string
	DomainID              string
	StartTime             int64
	ExecutionTime         int64
	CloseTime             int64
	CloseStatus           int
	HistoryLength         int64
	TaskList              string
	IsCron                bool
	NumClusters           int16
	ClusterAttributeScope string
	ClusterAttributeName  string
	UpdateTime            int64
	ShardID               int16
}

func ConvertSearchResultToVisibilityRecord(hit []interface{}, columnNames []string) (*p.InternalVisibilityWorkflowExecutionInfo, error) {
	if len(hit) != len(columnNames) {
		return nil, fmt.Errorf("length of hit (%v) is not equal with length of columnNames(%v)", len(hit), len(columnNames))
	}

	systemKeyMap := buildMap(hit, columnNames)

	jsonSystemKeyMap, err := json.Marshal(systemKeyMap)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal systemKeyMap")
	}

	attributeMap := make(map[string]interface{})
	var memo *p.DataBlob
	if systemKeyMap[Attr] != nil {
		attrMapStr, ok := systemKeyMap[Attr].(string)
		if !ok {
			return nil, fmt.Errorf(`assertion error. Can't convert systemKeyMap[Attr] to string. Found %T`, systemKeyMap[Attr])
		}

		err = json.Unmarshal([]byte(attrMapStr), &attributeMap)
		if err != nil {
			return nil, fmt.Errorf("unable to Unmarshal searchAttribute map: %s", err.Error())
		}

		if attributeMap[Memo] != nil {
			memo, err = convertMemo(attributeMap[Memo])
			if err != nil {
				return nil, fmt.Errorf("unable to convert memo: %s", err.Error())
			}
		}
		delete(attributeMap, Memo) // cleanup after we get memo from search attribute
	}

	// if memo is empty, set it to nil
	if memo != nil && len(memo.GetData()) == 0 {
		memo = nil
	}

	var source *VisibilityRecord
	err = json.Unmarshal(jsonSystemKeyMap, &source)
	if err != nil {
		return nil, fmt.Errorf("unable to Unmarshal systemKeyMap: %s", err.Error())
	}

	record := &p.InternalVisibilityWorkflowExecutionInfo{
		DomainID:              source.DomainID,
		WorkflowType:          source.WorkflowType,
		WorkflowID:            source.WorkflowID,
		RunID:                 source.RunID,
		TypeName:              source.WorkflowType,
		StartTime:             time.UnixMilli(source.StartTime), // be careful: source.StartTime is in milliseconds
		ExecutionTime:         time.UnixMilli(source.ExecutionTime),
		TaskList:              source.TaskList,
		IsCron:                source.IsCron,
		NumClusters:           source.NumClusters,
		ClusterAttributeScope: source.ClusterAttributeScope,
		ClusterAttributeName:  source.ClusterAttributeName,
		ShardID:               source.ShardID,
		SearchAttributes:      attributeMap,
		Memo:                  memo,
	}
	if source.UpdateTime > 0 {
		record.UpdateTime = time.UnixMilli(source.UpdateTime)
	}
	if source.CloseTime > 0 {
		record.CloseTime = time.UnixMilli(source.CloseTime)
		record.Status = toWorkflowExecutionCloseStatus(source.CloseStatus)
		record.HistoryLength = source.HistoryLength
	}

	return record, nil
}

func convertMemo(memoRaw interface{}) (*p.DataBlob, error) {
	db := p.DataBlob{}

	memoRawStr, ok := memoRaw.(string)
	if !ok {
		return nil, fmt.Errorf("memoRaw is not a String: %T", memoRaw)
	}

	err := json.Unmarshal([]byte(memoRawStr), &db)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal memoRawStr: %s", err.Error())
	}
	return &db, nil
}

func toWorkflowExecutionCloseStatus(status int) *types.WorkflowExecutionCloseStatus {
	if status < 0 {
		return nil
	}
	closeStatus := types.WorkflowExecutionCloseStatus(status)
	return &closeStatus
}
