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

package serialization

import (
	"testing"
	"time"

	"github.com/golang/snappy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/persistence"
)

func TestSnappyThriftDecoderRoundTrip(t *testing.T) {
	dc := &persistence.DynamicConfiguration{
		SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRWSnappy)),
	}
	parser, err := NewParser(dc)
	require.NoError(t, err)

	testCases := []struct {
		name string
		data interface{}
	}{
		{name: "ShardInfo", data: shardInfoTestData},
		{name: "DomainInfo", data: domainInfoTestData},
		{name: "HistoryTreeInfo", data: historyTreeInfoTestData},
		{name: "WorkflowExecutionInfo", data: workflowExecutionInfoTestData},
		{name: "ActivityInfo", data: activityInfoTestData},
		{name: "ChildExecutionInfo", data: childExecutionInfoTestData},
		{name: "SignalInfo", data: signalInfoTestData},
		{name: "RequestCancelInfo", data: requestCancelInfoTestData},
		{name: "TimerInfo", data: timerInfoTestData},
		{name: "TaskInfo", data: taskInfoTestData},
		{name: "TaskListInfo", data: taskListInfoTestData},
		{name: "TransferTaskInfo", data: transferTaskInfoTestData},
		{name: "TimerTaskInfo", data: timerTaskInfoTestData},
		{name: "ReplicationTaskInfo", data: replicationTaskInfoTestData},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			blob := encodeWithParser(t, parser, tc.data)
			decoded := decodeWithParser(t, parser, blob, tc.data)
			assert.Equal(t, tc.data, decoded)
		})
	}
}

func TestSnappyThriftDecoderErrorHandling(t *testing.T) {
	decoder := newSnappyThriftDecoder()

	testCases := []struct {
		name        string
		data        []byte
		decodeFunc  func([]byte) (interface{}, error)
		expectError bool
	}{
		{
			name: "Invalid snappy data for ShardInfo",
			data: []byte("invalid snappy data"),
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.shardInfoFromBlob(data)
			},
			expectError: true,
		},
		{
			name: "Empty data for DomainInfo",
			data: []byte{},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.domainInfoFromBlob(data)
			},
			expectError: true,
		},
		{
			name: "Corrupted snappy data for ActivityInfo",
			data: []byte{0xff, 0xff, 0xff, 0xff},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.activityInfoFromBlob(data)
			},
			expectError: true,
		},
		{
			name: "Valid snappy but invalid thrift for WorkflowExecutionInfo",
			data: func() []byte {
				// Create valid snappy compressed data but with invalid thrift content
				invalidThrift := []byte("not thrift data")
				compressed := snappy.Encode(nil, invalidThrift)
				return compressed
			}(),
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.workflowExecutionInfoFromBlob(data)
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.decodeFunc(tc.data)
			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestSnappyThriftRWDecode(t *testing.T) {
	// Test the low-level snappyThriftRWDecode function directly

	t.Run("Valid data", func(t *testing.T) {
		// Create a simple thrift struct and encode it
		encoder := newSnappyThriftEncoder()
		shardInfo := &ShardInfo{
			StolenSinceRenew:    1,
			UpdatedAt:           time.Now(),
			ReplicationAckLevel: 2,
			TransferAckLevel:    3,
		}

		encoded, err := encoder.shardInfoToBlob(shardInfo)
		require.NoError(t, err)

		// Now decode it back
		decoder := newSnappyThriftDecoder()
		decoded, err := decoder.shardInfoFromBlob(encoded)
		require.NoError(t, err)

		assert.Equal(t, shardInfo.StolenSinceRenew, decoded.StolenSinceRenew)
		assert.Equal(t, shardInfo.ReplicationAckLevel, decoded.ReplicationAckLevel)
		assert.Equal(t, shardInfo.TransferAckLevel, decoded.TransferAckLevel)
	})

	t.Run("Invalid snappy compression", func(t *testing.T) {
		decoder := newSnappyThriftDecoder()

		// Test with invalid snappy data
		invalidData := []byte("this is not snappy compressed data")
		_, err := decoder.shardInfoFromBlob(invalidData)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "snappy")
	})

	t.Run("Valid snappy but invalid thrift", func(t *testing.T) {
		decoder := newSnappyThriftDecoder()

		// Create valid snappy data but with invalid thrift content
		invalidThrift := []byte("not a valid thrift message")
		compressed := snappy.Encode(nil, invalidThrift)

		_, err := decoder.shardInfoFromBlob(compressed)
		assert.Error(t, err)
	})
}

func TestSnappyThriftDecoderInterface(t *testing.T) {
	// Verify that snappyThriftDecoder implements the decoder interface
	var _ decoder = (*snappyThriftDecoder)(nil)

	// Test that newSnappyThriftDecoder returns a valid decoder
	decoder := newSnappyThriftDecoder()
	assert.NotNil(t, decoder)
	assert.IsType(t, &snappyThriftDecoder{}, decoder)
}

func TestSnappyThriftDecoderNilHandling(t *testing.T) {
	decoder := newSnappyThriftDecoder()

	// Test each method with nil data
	testCases := []struct {
		name       string
		decodeFunc func() (interface{}, error)
	}{
		{
			name: "shardInfoFromBlob with nil",
			decodeFunc: func() (interface{}, error) {
				return decoder.shardInfoFromBlob(nil)
			},
		},
		{
			name: "domainInfoFromBlob with nil",
			decodeFunc: func() (interface{}, error) {
				return decoder.domainInfoFromBlob(nil)
			},
		},
		{
			name: "activityInfoFromBlob with nil",
			decodeFunc: func() (interface{}, error) {
				return decoder.activityInfoFromBlob(nil)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.decodeFunc()
			assert.Error(t, err)
			assert.Nil(t, result)
		})
	}
}

// Helper functions for encoding and decoding with parser
func encodeWithParser(t *testing.T, parser Parser, data interface{}) []byte {
	var blob []byte
	var err error

	switch v := data.(type) {
	case *ShardInfo:
		db, e := parser.ShardInfoToBlob(v)
		require.NoError(t, e)
		blob = db.Data
	case *DomainInfo:
		db, e := parser.DomainInfoToBlob(v)
		require.NoError(t, e)
		blob = db.Data
	case *HistoryTreeInfo:
		db, e := parser.HistoryTreeInfoToBlob(v)
		require.NoError(t, e)
		blob = db.Data
	case *WorkflowExecutionInfo:
		db, e := parser.WorkflowExecutionInfoToBlob(v)
		require.NoError(t, e)
		blob = db.Data
	case *ActivityInfo:
		db, e := parser.ActivityInfoToBlob(v)
		require.NoError(t, e)
		blob = db.Data
	case *ChildExecutionInfo:
		db, e := parser.ChildExecutionInfoToBlob(v)
		require.NoError(t, e)
		blob = db.Data
	case *SignalInfo:
		db, e := parser.SignalInfoToBlob(v)
		require.NoError(t, e)
		blob = db.Data
	case *RequestCancelInfo:
		db, e := parser.RequestCancelInfoToBlob(v)
		require.NoError(t, e)
		blob = db.Data
	case *TimerInfo:
		db, e := parser.TimerInfoToBlob(v)
		require.NoError(t, e)
		blob = db.Data
	case *TaskInfo:
		db, e := parser.TaskInfoToBlob(v)
		require.NoError(t, e)
		blob = db.Data
	case *TaskListInfo:
		db, e := parser.TaskListInfoToBlob(v)
		require.NoError(t, e)
		blob = db.Data
	case *TransferTaskInfo:
		db, e := parser.TransferTaskInfoToBlob(v)
		require.NoError(t, e)
		blob = db.Data
	case *TimerTaskInfo:
		db, e := parser.TimerTaskInfoToBlob(v)
		require.NoError(t, e)
		blob = db.Data
	case *ReplicationTaskInfo:
		db, e := parser.ReplicationTaskInfoToBlob(v)
		require.NoError(t, e)
		blob = db.Data
	default:
		t.Fatalf("Unknown type %T", v)
	}

	require.NoError(t, err)
	require.NotEmpty(t, blob)
	return blob
}

func decodeWithParser(t *testing.T, parser Parser, blob []byte, data interface{}) interface{} {
	var result interface{}
	var err error

	encoding := string(constants.EncodingTypeThriftRWSnappy)

	switch data.(type) {
	case *ShardInfo:
		result, err = parser.ShardInfoFromBlob(blob, encoding)
	case *DomainInfo:
		result, err = parser.DomainInfoFromBlob(blob, encoding)
	case *HistoryTreeInfo:
		result, err = parser.HistoryTreeInfoFromBlob(blob, encoding)
	case *WorkflowExecutionInfo:
		result, err = parser.WorkflowExecutionInfoFromBlob(blob, encoding)
	case *ActivityInfo:
		result, err = parser.ActivityInfoFromBlob(blob, encoding)
	case *ChildExecutionInfo:
		result, err = parser.ChildExecutionInfoFromBlob(blob, encoding)
	case *SignalInfo:
		result, err = parser.SignalInfoFromBlob(blob, encoding)
	case *RequestCancelInfo:
		result, err = parser.RequestCancelInfoFromBlob(blob, encoding)
	case *TimerInfo:
		result, err = parser.TimerInfoFromBlob(blob, encoding)
	case *TaskInfo:
		result, err = parser.TaskInfoFromBlob(blob, encoding)
	case *TaskListInfo:
		result, err = parser.TaskListInfoFromBlob(blob, encoding)
	case *TransferTaskInfo:
		result, err = parser.TransferTaskInfoFromBlob(blob, encoding)
	case *TimerTaskInfo:
		result, err = parser.TimerTaskInfoFromBlob(blob, encoding)
	case *ReplicationTaskInfo:
		result, err = parser.ReplicationTaskInfoFromBlob(blob, encoding)
	default:
		t.Fatalf("Unknown type %T", data)
	}

	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}
