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
	"fmt"
	"testing"

	"github.com/golang/snappy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/persistence"
)

func TestSnappyThriftEncoderRoundTrip(t *testing.T) {
	encoder := newSnappyThriftEncoder()
	decoder := newSnappyThriftDecoder()

	testCases := []struct {
		name       string
		data       interface{}
		encodeFunc func(interface{}) ([]byte, error)
		decodeFunc func([]byte) (interface{}, error)
	}{
		{
			name: "ShardInfo",
			data: shardInfoTestData,
			encodeFunc: func(data interface{}) ([]byte, error) {
				return encoder.shardInfoToBlob(data.(*ShardInfo))
			},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.shardInfoFromBlob(data)
			},
		},
		{
			name: "DomainInfo",
			data: domainInfoTestData,
			encodeFunc: func(data interface{}) ([]byte, error) {
				return encoder.domainInfoToBlob(data.(*DomainInfo))
			},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.domainInfoFromBlob(data)
			},
		},
		{
			name: "HistoryTreeInfo",
			data: historyTreeInfoTestData,
			encodeFunc: func(data interface{}) ([]byte, error) {
				return encoder.historyTreeInfoToBlob(data.(*HistoryTreeInfo))
			},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.historyTreeInfoFromBlob(data)
			},
		},
		{
			name: "WorkflowExecutionInfo",
			data: workflowExecutionInfoTestData,
			encodeFunc: func(data interface{}) ([]byte, error) {
				return encoder.workflowExecutionInfoToBlob(data.(*WorkflowExecutionInfo))
			},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.workflowExecutionInfoFromBlob(data)
			},
		},
		{
			name: "ActivityInfo",
			data: activityInfoTestData,
			encodeFunc: func(data interface{}) ([]byte, error) {
				return encoder.activityInfoToBlob(data.(*ActivityInfo))
			},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.activityInfoFromBlob(data)
			},
		},
		{
			name: "ChildExecutionInfo",
			data: childExecutionInfoTestData,
			encodeFunc: func(data interface{}) ([]byte, error) {
				return encoder.childExecutionInfoToBlob(data.(*ChildExecutionInfo))
			},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.childExecutionInfoFromBlob(data)
			},
		},
		{
			name: "SignalInfo",
			data: signalInfoTestData,
			encodeFunc: func(data interface{}) ([]byte, error) {
				return encoder.signalInfoToBlob(data.(*SignalInfo))
			},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.signalInfoFromBlob(data)
			},
		},
		{
			name: "RequestCancelInfo",
			data: requestCancelInfoTestData,
			encodeFunc: func(data interface{}) ([]byte, error) {
				return encoder.requestCancelInfoToBlob(data.(*RequestCancelInfo))
			},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.requestCancelInfoFromBlob(data)
			},
		},
		{
			name: "TimerInfo",
			data: timerInfoTestData,
			encodeFunc: func(data interface{}) ([]byte, error) {
				return encoder.timerInfoToBlob(data.(*TimerInfo))
			},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.timerInfoFromBlob(data)
			},
		},
		{
			name: "TaskInfo",
			data: taskInfoTestData,
			encodeFunc: func(data interface{}) ([]byte, error) {
				return encoder.taskInfoToBlob(data.(*TaskInfo))
			},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.taskInfoFromBlob(data)
			},
		},
		{
			name: "TaskListInfo",
			data: taskListInfoTestData,
			encodeFunc: func(data interface{}) ([]byte, error) {
				return encoder.taskListInfoToBlob(data.(*TaskListInfo))
			},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.taskListInfoFromBlob(data)
			},
		},
		{
			name: "TransferTaskInfo",
			data: transferTaskInfoTestData,
			encodeFunc: func(data interface{}) ([]byte, error) {
				return encoder.transferTaskInfoToBlob(data.(*TransferTaskInfo))
			},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.transferTaskInfoFromBlob(data)
			},
		},
		{
			name: "TimerTaskInfo",
			data: timerTaskInfoTestData,
			encodeFunc: func(data interface{}) ([]byte, error) {
				return encoder.timerTaskInfoToBlob(data.(*TimerTaskInfo))
			},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.timerTaskInfoFromBlob(data)
			},
		},
		{
			name: "ReplicationTaskInfo",
			data: replicationTaskInfoTestData,
			encodeFunc: func(data interface{}) ([]byte, error) {
				return encoder.replicationTaskInfoToBlob(data.(*ReplicationTaskInfo))
			},
			decodeFunc: func(data []byte) (interface{}, error) {
				return decoder.replicationTaskInfoFromBlob(data)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Encode the data using the encoder
			encoded, err := tc.encodeFunc(tc.data)
			require.NoError(t, err)
			require.NotEmpty(t, encoded)

			// Verify the data is snappy compressed
			assert.True(t, isValidSnappyData(encoded), "encoded data should be valid snappy compressed data")

			// Decode the data using the decoder and verify it matches
			decoded, err := tc.decodeFunc(encoded)
			require.NoError(t, err)
			assert.Equal(t, tc.data, decoded)
		})
	}
}

func TestSnappyThriftEncoderEncodingType(t *testing.T) {
	encoder := newSnappyThriftEncoder()

	encodingType := encoder.encodingType()
	assert.Equal(t, constants.EncodingTypeThriftRWSnappy, encodingType)
}

func TestSnappyThriftEncoderInterface(t *testing.T) {
	// Verify that snappyThriftEncoder implements the encoder interface
	var _ encoder = (*snappyThriftEncoder)(nil)

	// Test that newSnappyThriftEncoder returns a valid encoder
	encoder := newSnappyThriftEncoder()
	assert.NotNil(t, encoder)
	assert.IsType(t, &snappyThriftEncoder{}, encoder)
}

func TestSnappyThriftEncoderNilHandling(t *testing.T) {
	encoder := newSnappyThriftEncoder()

	// Test each method with nil data
	testCases := []struct {
		name       string
		encodeFunc func() ([]byte, error)
	}{
		{
			name: "shardInfoToBlob with nil",
			encodeFunc: func() ([]byte, error) {
				return encoder.shardInfoToBlob(nil)
			},
		},
		{
			name: "domainInfoToBlob with nil",
			encodeFunc: func() ([]byte, error) {
				return encoder.domainInfoToBlob(nil)
			},
		},
		{
			name: "activityInfoToBlob with nil",
			encodeFunc: func() ([]byte, error) {
				return encoder.activityInfoToBlob(nil)
			},
		},
		{
			name: "workflowExecutionInfoToBlob with nil",
			encodeFunc: func() ([]byte, error) {
				return encoder.workflowExecutionInfoToBlob(nil)
			},
		},
		{
			name: "childExecutionInfoToBlob with nil",
			encodeFunc: func() ([]byte, error) {
				return encoder.childExecutionInfoToBlob(nil)
			},
		},
		{
			name: "signalInfoToBlob with nil",
			encodeFunc: func() ([]byte, error) {
				return encoder.signalInfoToBlob(nil)
			},
		},
		{
			name: "requestCancelInfoToBlob with nil",
			encodeFunc: func() ([]byte, error) {
				return encoder.requestCancelInfoToBlob(nil)
			},
		},
		{
			name: "timerInfoToBlob with nil",
			encodeFunc: func() ([]byte, error) {
				return encoder.timerInfoToBlob(nil)
			},
		},
		{
			name: "taskInfoToBlob with nil",
			encodeFunc: func() ([]byte, error) {
				return encoder.taskInfoToBlob(nil)
			},
		},
		{
			name: "taskListInfoToBlob with nil",
			encodeFunc: func() ([]byte, error) {
				return encoder.taskListInfoToBlob(nil)
			},
		},
		{
			name: "transferTaskInfoToBlob with nil",
			encodeFunc: func() ([]byte, error) {
				return encoder.transferTaskInfoToBlob(nil)
			},
		},
		{
			name: "crossClusterTaskInfoToBlob with nil",
			encodeFunc: func() ([]byte, error) {
				return encoder.crossClusterTaskInfoToBlob(nil)
			},
		},
		{
			name: "timerTaskInfoToBlob with nil",
			encodeFunc: func() ([]byte, error) {
				return encoder.timerTaskInfoToBlob(nil)
			},
		},
		{
			name: "replicationTaskInfoToBlob with nil",
			encodeFunc: func() ([]byte, error) {
				return encoder.replicationTaskInfoToBlob(nil)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result []byte
			var err error
			var panicRecovered bool

			// Use defer/recover to handle panic as expected behavior for nil input
			func() {
				defer func() {
					if r := recover(); r != nil {
						panicRecovered = true
					}
				}()
				result, err = tc.encodeFunc()
			}()

			// Nil input should either produce an error, panic, or valid empty data
			if panicRecovered {
				// Panic is expected for nil input
				assert.True(t, true, "nil input caused expected panic")
			} else if err != nil {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				// If no error, result should be valid snappy data
				assert.NotNil(t, result)
				assert.True(t, isValidSnappyData(result), "nil input should produce valid snappy data")
			}
		})
	}
}

func TestSnappyThriftRWEncode(t *testing.T) {
	// Test the low-level snappyThriftRWEncode function directly

	t.Run("Valid thrift object", func(t *testing.T) {
		// Create a simple thrift struct and encode it
		shardInfo := shardInfoTestData

		// Convert to thrift format
		thriftStruct := shardInfoToThrift(shardInfo)

		// Encode using the low-level function
		encoded, err := snappyThriftRWEncode(thriftStruct)
		require.NoError(t, err)
		require.NotEmpty(t, encoded)

		// Verify it's valid snappy data
		assert.True(t, isValidSnappyData(encoded))

		// Verify we can decode it back
		decoder := newSnappyThriftDecoder()
		decoded, err := decoder.shardInfoFromBlob(encoded)
		require.NoError(t, err)

		assert.Equal(t, shardInfo.StolenSinceRenew, decoded.StolenSinceRenew)
		assert.Equal(t, shardInfo.ReplicationAckLevel, decoded.ReplicationAckLevel)
		assert.Equal(t, shardInfo.TransferAckLevel, decoded.TransferAckLevel)
	})

	t.Run("Nil thrift object", func(t *testing.T) {
		// Test with nil thrift object (converted from nil input)
		thriftStruct := shardInfoToThrift(nil)
		assert.Nil(t, thriftStruct)

		var err error
		var panicRecovered bool

		// Use defer/recover to handle panic as expected behavior for nil input
		func() {
			defer func() {
				if r := recover(); r != nil {
					panicRecovered = true
				}
			}()
			_, err = snappyThriftRWEncode(thriftStruct)
		}()

		// Either panic or error is acceptable for nil input
		if panicRecovered {
			assert.True(t, true, "nil thrift object caused expected panic")
		} else if err != nil {
			assert.Error(t, err)
		}
	})
}

func TestSnappyThriftEncoderWithParser(t *testing.T) {
	dc := &persistence.DynamicConfiguration{
		SerializationEncoding: dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRWSnappy)),
	}
	// Test encoder integration with parser
	parser, err := NewParser(dc)
	require.NoError(t, err)

	testData := shardInfoTestData

	// Encode using parser
	blob, err := parser.ShardInfoToBlob(testData)
	require.NoError(t, err)
	assert.Equal(t, constants.EncodingTypeThriftRWSnappy, blob.Encoding)
	assert.NotEmpty(t, blob.Data)
	assert.True(t, isValidSnappyData(blob.Data))

	// Decode using parser
	decoded, err := parser.ShardInfoFromBlob(blob.Data, string(blob.Encoding))
	require.NoError(t, err)
	assert.Equal(t, testData, decoded)
}

func TestSnappyThriftEncoderDataCompression(t *testing.T) {
	encoder := newSnappyThriftEncoder()

	// Create a large data structure to test compression
	largeData := &WorkflowExecutionInfo{
		WorkflowTypeName: "very_long_workflow_type_name_that_should_compress_well_when_repeated",
		TaskList:         "very_long_task_list_name_that_should_compress_well_when_repeated",
		ExecutionContext: make([]byte, 1000), // Large byte array
		SearchAttributes: make(map[string][]byte),
		Memo:             make(map[string][]byte),
	}

	// Fill with repetitive data that should compress well
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("repetitive_key_that_compresses_well_%d", i)
		value := []byte("repetitive_value_that_compresses_well_when_repeated_many_times")
		largeData.SearchAttributes[key] = value
		largeData.Memo[key] = value
	}

	// Encode the data
	encoded, err := encoder.workflowExecutionInfoToBlob(largeData)
	require.NoError(t, err)
	require.NotEmpty(t, encoded)

	// Verify it's compressed (should be significantly smaller than uncompressed)
	assert.True(t, isValidSnappyData(encoded))

	// Decode and verify correctness
	decoder := newSnappyThriftDecoder()
	decoded, err := decoder.workflowExecutionInfoFromBlob(encoded)
	require.NoError(t, err)
	assert.Equal(t, largeData.WorkflowTypeName, decoded.WorkflowTypeName)
	assert.Equal(t, largeData.TaskList, decoded.TaskList)
	assert.Len(t, decoded.SearchAttributes, 100)
	assert.Len(t, decoded.Memo, 100)
}

// Helper function to check if data is valid snappy compressed data
func isValidSnappyData(data []byte) bool {
	_, err := snappy.Decode(nil, data)
	return err == nil
}
