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

package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/thriftrw/protocol/binary"
	"go.uber.org/thriftrw/ptr"

	"github.com/uber/cadence/.gen/go/indexer"
	"github.com/uber/cadence/.gen/go/replicator"
	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/tools/cli/clitest"
	"github.com/uber/cadence/tools/common/commoncli"
)

func TestWriterChannel(t *testing.T) {
	t.Run("replication task type", func(t *testing.T) {
		ch := newWriterChannel(kafkaMessageTypeReplicationTask)

		assert.Equal(t, kafkaMessageType(0), ch.Type)
		assert.Nil(t, ch.VisibilityMsgChannel)
		assert.NotNil(t, ch.ReplicationTaskChannel)
		assert.Equal(t, 10000, cap(ch.ReplicationTaskChannel))
	})

	t.Run("msg visibility type", func(t *testing.T) {
		ch := newWriterChannel(kafkaMessageTypeVisibilityMsg)

		assert.Equal(t, kafkaMessageType(1), ch.Type)
		assert.Nil(t, ch.ReplicationTaskChannel)
		assert.NotNil(t, ch.VisibilityMsgChannel)
		assert.Equal(t, 10000, cap(ch.VisibilityMsgChannel))
	})

	t.Run("replication task type close channel", func(t *testing.T) {
		ch := newWriterChannel(kafkaMessageTypeReplicationTask)
		ch.ReplicationTaskChannel <- nil
		_, ok := <-ch.ReplicationTaskChannel
		assert.True(t, ok)

		ch.Close()
		_, ok = <-ch.ReplicationTaskChannel
		assert.False(t, ok)
	})
	t.Run("msg visibility type close channel", func(t *testing.T) {
		ch := newWriterChannel(kafkaMessageTypeVisibilityMsg)
		ch.VisibilityMsgChannel <- nil
		_, ok := <-ch.VisibilityMsgChannel
		assert.True(t, ok)

		ch.Close()
		_, ok = <-ch.VisibilityMsgChannel
		assert.False(t, ok)
	})
}

func TestBuildFilterFn(t *testing.T) {
	allTasks := []*types.ReplicationTask{
		nil,
		{HistoryTaskV2Attributes: nil},
		{HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{}},
		{HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{RunID: "run-id-1"}},
		{HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{RunID: "run-id-2"}},
		{HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{WorkflowID: "my-workflow-id"}},
		{HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{WorkflowID: "my-workflow-id", RunID: "run-id-1"}},
		{HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{WorkflowID: "my-workflow-id", RunID: "run-id-2"}},
		{HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{WorkflowID: "alien-workflow-id"}},
		{HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{WorkflowID: "alien-workflow-id", RunID: "run-id-1"}},
		{HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{WorkflowID: "alien-workflow-id", RunID: "run-id-2"}},
	}

	tests := []struct {
		name                     string
		filterFn                 filterFn
		expectedFilteredTaskList []*types.ReplicationTask
	}{
		{"empty filter always return true", buildFilterFn("", ""), allTasks},
		{
			"filter with workflowId only",
			buildFilterFn("my-workflow-id", ""),
			[]*types.ReplicationTask{
				{HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{WorkflowID: "my-workflow-id"}},
				{HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{WorkflowID: "my-workflow-id", RunID: "run-id-1"}},
				{HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{WorkflowID: "my-workflow-id", RunID: "run-id-2"}},
			},
		},
		{
			"filter with runId only",
			buildFilterFn("", "run-id-1"),
			[]*types.ReplicationTask{
				{HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{RunID: "run-id-1"}},
				{HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{WorkflowID: "my-workflow-id", RunID: "run-id-1"}},
				{HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{WorkflowID: "alien-workflow-id", RunID: "run-id-1"}},
			},
		},
		{
			"filter with workflow and runId",
			buildFilterFn("my-workflow-id", "run-id-1"),
			[]*types.ReplicationTask{
				{HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{WorkflowID: "my-workflow-id", RunID: "run-id-1"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result []*types.ReplicationTask

			for _, task := range allTasks {
				if tt.filterFn(task) {
					result = append(result, task)
				}
			}

			assert.Equal(t, tt.expectedFilteredTaskList, result)
		})
	}
}

func TestBuildFilterFnForVisibility(t *testing.T) {
	allMessages := []*indexer.Message{
		nil,
		{},
		{RunID: ptr.String("run-id-1")},
		{RunID: ptr.String("run-id-2")},
		{WorkflowID: ptr.String("my-workflow-id")},
		{WorkflowID: ptr.String("my-workflow-id"), RunID: ptr.String("run-id-1")},
		{WorkflowID: ptr.String("my-workflow-id"), RunID: ptr.String("run-id-2")},
		{WorkflowID: ptr.String("alien-workflow-id")},
		{WorkflowID: ptr.String("alien-workflow-id"), RunID: ptr.String("run-id-1")},
		{WorkflowID: ptr.String("alien-workflow-id"), RunID: ptr.String("run-id-2")},
	}

	tests := []struct {
		name                     string
		filterFn                 filterFnForVisibility
		expectedFilteredTaskList []*indexer.Message
	}{
		{"empty filter always return true", buildFilterFnForVisibility("", ""), allMessages},
		{
			"filter with workflowId only",
			buildFilterFnForVisibility("my-workflow-id", ""),
			[]*indexer.Message{
				{WorkflowID: ptr.String("my-workflow-id")},
				{WorkflowID: ptr.String("my-workflow-id"), RunID: ptr.String("run-id-1")},
				{WorkflowID: ptr.String("my-workflow-id"), RunID: ptr.String("run-id-2")},
			},
		},
		{
			"filter with runId only",
			buildFilterFnForVisibility("", "run-id-1"),
			[]*indexer.Message{
				{RunID: ptr.String("run-id-1")},
				{WorkflowID: ptr.String("my-workflow-id"), RunID: ptr.String("run-id-1")},
				{WorkflowID: ptr.String("alien-workflow-id"), RunID: ptr.String("run-id-1")},
			},
		},
		{
			"filter with workflow and runId",
			buildFilterFnForVisibility("my-workflow-id", "run-id-1"),
			[]*indexer.Message{
				{WorkflowID: ptr.String("my-workflow-id"), RunID: ptr.String("run-id-1")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result []*indexer.Message

			for _, task := range allMessages {
				if tt.filterFn(task) {
					result = append(result, task)
				}
			}

			assert.Equal(t, tt.expectedFilteredTaskList, result)
		})
	}
}

func TestGetOutputFile(t *testing.T) {
	t.Run("returns stdout if filename is empty", func(t *testing.T) {
		file, err := getOutputFile("")
		assert.NoError(t, err)
		assert.Equal(t, os.Stdout, file)
	})

	t.Run("creates a file", func(t *testing.T) {
		file, err := getOutputFile("test.txt")
		defer func() {
			if file != nil {
				_ = file.Close()
				_ = os.Remove("test.txt")
			}
		}()

		assert.NoError(t, err)
		assert.NotNil(t, file)

		info, err := file.Stat()

		assert.Equal(t, "test.txt", info.Name())
	})

	t.Run("fails to create a file", func(t *testing.T) {
		file, err := getOutputFile("/non/existent/directory/test.txt")

		assert.EqualError(t, err, "failed to create output file open /non/existent/directory/test.txt: no such file or directory")
		assert.Nil(t, file)
	})
}

type FailingIOReader struct{}

func (*FailingIOReader) Read(_ []byte) (n int, err error) {
	return 0, errors.New("failed to read")
}

func TestStartReader(t *testing.T) {
	t.Run("starts reader successfully", func(t *testing.T) {
		ch := make(chan []byte)
		file := bytes.NewBuffer([]byte{1, 2, 3, 4})

		go func() {
			err := startReader(file, ch)
			assert.NoError(t, err)
		}()

		readBytes := <-ch

		assert.Equal(t, []byte{1, 2, 3, 4}, readBytes)
	})

	t.Run("reader stops successfully at the EOF", func(t *testing.T) {
		ch := make(chan []byte)
		file := bytes.NewBuffer([]byte{1, 2, 3, 4})

		go func() {
			<-ch
		}()

		err := startReader(file, ch)
		assert.NoError(t, err)
	})

	t.Run("returns error if couldn't read", func(t *testing.T) {
		err := startReader(&FailingIOReader{}, make(chan<- []byte))
		assert.EqualError(t, err, "failed to read from reader: failed to read")
	})
}

func TestStartParser(t *testing.T) {
	t.Run("skipErrors ignores invalid data", func(t *testing.T) {
		readerChannel := make(chan []byte)
		writerChannel := newWriterChannel(kafkaMessageTypeReplicationTask)
		skippedMessagesCount := int32(0)

		go func() {
			readerChannel <- []byte{1, 2, 3, 4}
			close(readerChannel)
		}()

		err := startParser(readerChannel, writerChannel, true, &skippedMessagesCount)
		assert.NoError(t, err)
		assert.Equal(t, int32(1), skippedMessagesCount)
	})

	t.Run("without skipErrors the invalid data returns error", func(t *testing.T) {
		readerChannel := make(chan []byte)
		writerChannel := newWriterChannel(kafkaMessageTypeReplicationTask)
		count := int32(0)

		go func() {
			readerChannel <- []byte{1, 2, 3, 4}
			close(readerChannel)
		}()

		err := startParser(readerChannel, writerChannel, false, &count)
		assert.EqualError(t, err, "error in splitBuffer: header not found, did you generate dump with -v")
	})

	t.Run("successfully start parser", func(t *testing.T) {
		readerChannel := make(chan []byte)
		writerChannel := newWriterChannel(kafkaMessageTypeReplicationTask)
		count := int32(0)

		go func() {
			readerChannel <- []byte("Partition: abc, Offset: 0, Key: 123")
			close(readerChannel)
		}()

		go func() {
			err := startParser(readerChannel, writerChannel, false, &count)
			assert.NoError(t, err)
		}()

		result := <-writerChannel.ReplicationTaskChannel
		assert.Equal(t, (*types.ReplicationTask)(nil), result)
	})
}

func TestDeserializeMessage(t *testing.T) {
	t.Run("skipErrors ignores malformed messages", func(t *testing.T) {
		tasks, skipped, err := deserializeMessages([][]byte{{1, 2, 3, 4}}, true)

		assert.Equal(t, []*types.ReplicationTask(nil), tasks)
		assert.Equal(t, int32(1), skipped)
		assert.NoError(t, err)
	})

	t.Run("without skipErrors malformed messages return error", func(t *testing.T) {
		tasks, skipped, err := deserializeMessages([][]byte{{1, 2, 3, 4}}, false)

		assert.Nil(t, tasks)
		assert.Equal(t, int32(0), skipped)
		assert.EqualError(t, err, "Input was malformedError: unexpected EOF")
	})

	t.Run("successful deserialization", func(t *testing.T) {
		wireVal, _ := (&replicator.ReplicationTask{CreationTime: ptr.Int64(123)}).ToWire()

		encoded := bytes.NewBuffer([]byte{})
		_ = binary.Default.Encode(wireVal, encoded)

		tasks, skipped, err := deserializeMessages([][]byte{encoded.Bytes()}, false)

		assert.Equal(t, []*types.ReplicationTask{{}}, tasks)
		assert.Equal(t, int32(0), skipped)
		assert.NoError(t, err)
	})
}

func TestDeserializeVisibilityMessage(t *testing.T) {
	t.Run("skipErrors ignores malformed messages", func(t *testing.T) {
		tasks, skipped, err := deserializeVisibilityMessages([][]byte{{1, 2, 3, 4}}, true)

		assert.Equal(t, []*indexer.Message(nil), tasks)
		assert.Equal(t, int32(1), skipped)
		assert.NoError(t, err)
	})

	t.Run("without skipErrors malformed messages return error", func(t *testing.T) {
		tasks, skipped, err := deserializeVisibilityMessages([][]byte{{1, 2, 3, 4}}, false)

		assert.Nil(t, tasks)
		assert.Equal(t, int32(0), skipped)
		assert.EqualError(t, err, "Input was malformedError: unexpected EOF")
	})

	t.Run("successful deserialization", func(t *testing.T) {
		wireVal, _ := (&indexer.Message{Version: ptr.Int64(123)}).ToWire()

		encoded := bytes.NewBuffer([]byte{})
		_ = binary.Default.Encode(wireVal, encoded)

		tasks, skipped, err := deserializeVisibilityMessages([][]byte{encoded.Bytes()}, false)

		assert.Equal(t, []*indexer.Message{{}}, tasks)
		assert.Equal(t, int32(0), skipped)
		assert.NoError(t, err)
	})
}

func TestDecodeReplicationTask(t *testing.T) {
	t.Run("decode replication task", func(t *testing.T) {
		result, err := decodeReplicationTask(&types.ReplicationTask{TaskType: types.ReplicationTaskTypeHistory.Ptr()}, persistence.NewPayloadSerializer())
		expectedTaskJSONBytes := []byte("{\"taskType\":\"History\"}")
		assert.NoError(t, err)
		assert.Equal(t, expectedTaskJSONBytes, result)
	})

	t.Run("decode replication task type HistoryV2", func(t *testing.T) {
		task := &types.ReplicationTask{
			TaskType: types.ReplicationTaskTypeHistoryV2.Ptr(),
			HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{
				Events: &types.DataBlob{
					EncodingType: nil,
					Data:         nil,
				},
				NewRunEvents: &types.DataBlob{
					EncodingType: nil,
					Data:         nil,
				},
			},
		}
		result, err := decodeReplicationTask(task, persistence.NewPayloadSerializer())
		expectedTaskJSONBytes := []byte("{\"Task\":{\"taskType\":\"HistoryV2\",\"historyTaskV2Attributes\":{}},\"Events\":null,\"NewRunEvents\":null}")
		assert.NoError(t, err)
		assert.Equal(t, expectedTaskJSONBytes, result)
	})

	t.Run("deserialize returns an error", func(t *testing.T) {
		task := &types.ReplicationTask{
			TaskType: types.ReplicationTaskTypeHistoryV2.Ptr(),
			HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{
				Events: &types.DataBlob{EncodingType: nil, Data: []byte{}},
			},
		}

		eventsBlob := persistence.NewDataBlobFromInternal(&types.DataBlob{EncodingType: nil, Data: []byte{}})
		serializerMock := persistence.NewMockPayloadSerializer(gomock.NewController(t))
		serializerMock.EXPECT().DeserializeBatchEvents(eventsBlob).Return(nil, assert.AnError).Times(1)

		result, err := decodeReplicationTask(task, serializerMock)

		assert.Equal(t, err, assert.AnError)
		assert.Nil(t, result)
	})

	t.Run("deserialize returns an error for NewRunEvents", func(t *testing.T) {
		task := &types.ReplicationTask{
			TaskType: types.ReplicationTaskTypeHistoryV2.Ptr(),
			HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{
				Events:       &types.DataBlob{EncodingType: nil, Data: []byte{}},
				NewRunEvents: &types.DataBlob{EncodingType: nil, Data: []byte{}},
			},
		}

		eventsBlob := persistence.NewDataBlobFromInternal(&types.DataBlob{EncodingType: nil, Data: []byte{}})
		newRunEventsBlob := persistence.NewDataBlobFromInternal(&types.DataBlob{EncodingType: nil, Data: []byte{}})
		serializerMock := persistence.NewMockPayloadSerializer(gomock.NewController(t))
		serializerMock.EXPECT().DeserializeBatchEvents(eventsBlob).Return(nil, nil).Times(1)
		serializerMock.EXPECT().DeserializeBatchEvents(newRunEventsBlob).Return(nil, assert.AnError).Times(1)

		result, err := decodeReplicationTask(task, serializerMock)

		assert.Equal(t, err, assert.AnError)
		assert.Nil(t, result)
	})
}

func TestDoRereplicate(t *testing.T) {
	t.Run("successful rereplication", func(t *testing.T) {
		clientMock := admin.NewMockClient(gomock.NewController(t))
		clientMock.EXPECT().ResendReplicationTasks(context.Background(), &types.ResendReplicationTasksRequest{
			DomainID:      "domainID",
			WorkflowID:    "wid",
			RunID:         "rid",
			RemoteCluster: "sourceCluster",
			EndEventID:    ptr.Int64(1),
			EndVersion:    ptr.Int64(1),
		}).Return(nil).Times(1)

		err := doRereplicate(context.Background(), "domainID", "wid", "rid", ptr.Int64(1), ptr.Int64(1), "sourceCluster", clientMock)

		assert.NoError(t, err)
	})

	t.Run("returns err", func(t *testing.T) {
		clientMock := admin.NewMockClient(gomock.NewController(t))
		clientMock.EXPECT().ResendReplicationTasks(context.Background(), &types.ResendReplicationTasksRequest{
			DomainID:      "domainID",
			WorkflowID:    "wid",
			RunID:         "rid",
			RemoteCluster: "sourceCluster",
			EndEventID:    ptr.Int64(1),
			EndVersion:    ptr.Int64(1),
		}).Return(assert.AnError).Times(1)

		err := doRereplicate(context.Background(), "domainID", "wid", "rid", ptr.Int64(1), ptr.Int64(1), "sourceCluster", clientMock)

		assert.EqualError(t, err, "Failed to resend ndc workflow: assert.AnError general error for testing")
	})
}

func TestAdminRereplicate(t *testing.T) {
	tests := []struct {
		name string
		args []clitest.CliArgument
		err  error
	}{
		{
			name: "success",
			args: []clitest.CliArgument{
				clitest.StringArgument(FlagSourceCluster, "sourceCluster"),
				clitest.IntArgument(FlagMaxEventID, 123),
				clitest.IntArgument(FlagEndEventVersion, 321),
				clitest.StringArgument(FlagDomainID, "domainID"),
				clitest.StringArgument(FlagWorkflowID, "workflowID"),
				clitest.StringArgument(FlagRunID, "runID"),
				clitest.StringArgument(FlagContextTimeout, "10s"),
			},
			err: nil,
		},
		{
			name: "source cluster is missing",
			args: []clitest.CliArgument{
				clitest.IntArgument(FlagMaxEventID, 123),
				clitest.IntArgument(FlagEndEventVersion, 321),
				clitest.StringArgument(FlagDomainID, "domainID"),
				clitest.StringArgument(FlagWorkflowID, "workflowID"),
				clitest.StringArgument(FlagRunID, "runID"),
				clitest.StringArgument(FlagContextTimeout, "10s"),
			},
			err: commoncli.Problem("Required flag not found: ", errors.New("option source_cluster is required")),
		},
		{
			name: "domain ID is missing",
			args: []clitest.CliArgument{
				clitest.StringArgument(FlagSourceCluster, "sourceCluster"),
				clitest.IntArgument(FlagMaxEventID, 123),
				clitest.IntArgument(FlagEndEventVersion, 321),
				clitest.StringArgument(FlagWorkflowID, "workflowID"),
				clitest.StringArgument(FlagRunID, "runID"),
				clitest.StringArgument(FlagContextTimeout, "10s"),
			},
			err: commoncli.Problem("Required flag not found: ", errors.New("option domain_id is required")),
		},
		{
			name: "workflow ID is missing",
			args: []clitest.CliArgument{
				clitest.StringArgument(FlagSourceCluster, "sourceCluster"),
				clitest.IntArgument(FlagMaxEventID, 123),
				clitest.IntArgument(FlagEndEventVersion, 321),
				clitest.StringArgument(FlagDomainID, "domainID"),
				clitest.StringArgument(FlagRunID, "runID"),
				clitest.StringArgument(FlagContextTimeout, "10s"),
			},
			err: commoncli.Problem("Required flag not found: ", errors.New("option workflow_id is required")),
		},
		{
			name: "run ID is missing",
			args: []clitest.CliArgument{
				clitest.StringArgument(FlagSourceCluster, "sourceCluster"),
				clitest.IntArgument(FlagMaxEventID, 123),
				clitest.IntArgument(FlagEndEventVersion, 321),
				clitest.StringArgument(FlagDomainID, "domainID"),
				clitest.StringArgument(FlagWorkflowID, "workflowID"),
				clitest.StringArgument(FlagContextTimeout, "10s"),
			},
			err: commoncli.Problem("Required flag not found: ", errors.New("option run_id is required")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testData := newCLITestData(t)
			testData.mockAdminClient.EXPECT().ResendReplicationTasks(gomock.Any(), &types.ResendReplicationTasksRequest{
				DomainID:      "domainID",
				WorkflowID:    "workflowID",
				RunID:         "runID",
				RemoteCluster: "sourceCluster",
				EndEventID:    ptr.Int64(124),
				EndVersion:    ptr.Int64(321),
			}).Return(nil).MaxTimes(1)

			cliContext := clitest.NewCLIContext(t, testData.app, tt.args...)

			err := AdminRereplicate(cliContext)
			assert.Equal(t, tt.err, err)
		})
	}
}

func TestParse(t *testing.T) {
	t.Run("successful parse replication task", func(t *testing.T) {
		wireVal, _ := (&replicator.ReplicationTask{CreationTime: ptr.Int64(123)}).ToWire()
		encoded := bytes.NewBuffer([]byte{})
		_ = binary.Default.Encode(wireVal, encoded)
		data := []byte("Partition: abc, Offset: 0, Key: 123")
		data = append(data, preambleVersion0)
		data = append(data, encoded.Bytes()...)

		skipped := ptr.Int32(0)
		ch := newWriterChannel(kafkaMessageTypeReplicationTask)

		go func() {
			err := parse(data, false, skipped, ch)
			assert.NoError(t, err)
			close(ch.ReplicationTaskChannel)
		}()

		result := <-ch.ReplicationTaskChannel

		assert.NotNil(t, result)
	})

	t.Run("successful parse visibility task", func(t *testing.T) {
		wireVal, _ := (&indexer.Message{Version: ptr.Int64(123)}).ToWire()
		encoded := bytes.NewBuffer([]byte{})
		_ = binary.Default.Encode(wireVal, encoded)
		data := []byte("Partition: abc, Offset: 0, Key: 123")
		data = append(data, preambleVersion0)
		data = append(data, encoded.Bytes()...)

		skipped := ptr.Int32(0)
		ch := newWriterChannel(kafkaMessageTypeVisibilityMsg)

		go func() {
			err := parse(data, false, skipped, ch)
			assert.NoError(t, err)
			close(ch.VisibilityMsgChannel)
		}()

		result := <-ch.VisibilityMsgChannel

		assert.NotNil(t, result)
	})

	t.Run("invalid replication task", func(t *testing.T) {
		data := []byte("Partition: abc, Offset: 0, Key: 123")
		data = append(data, preambleVersion0)
		data = append(data, []byte{1, 2, 3, 4}...)

		skipped := ptr.Int32(0)
		ch := newWriterChannel(kafkaMessageTypeReplicationTask)

		go func() {
			err := parse(data, false, skipped, ch)
			assert.EqualError(t, err, "parsing failed: Input was malformedError: unknown ttype Type(1)")
			close(ch.ReplicationTaskChannel)
		}()

		result := <-ch.ReplicationTaskChannel

		assert.Nil(t, result)
	})

	t.Run("invalid visibility task", func(t *testing.T) {
		data := []byte("Partition: abc, Offset: 0, Key: 123")
		data = append(data, preambleVersion0)
		data = append(data, []byte{1, 2, 3, 4}...)

		skipped := ptr.Int32(0)
		ch := newWriterChannel(kafkaMessageTypeVisibilityMsg)

		go func() {
			err := parse(data, false, skipped, ch)
			assert.EqualError(t, err, "parsing failed: Input was malformedError: unknown ttype Type(1)")
			close(ch.VisibilityMsgChannel)
		}()

		result := <-ch.VisibilityMsgChannel

		assert.Nil(t, result)
	})
}

func TestAdminKafkaParse(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		wireVal, _ := (&replicator.ReplicationTask{CreationTime: ptr.Int64(123)}).ToWire()
		encoded := bytes.NewBuffer([]byte{})
		_ = binary.Default.Encode(wireVal, encoded)
		data := []byte("Partition: abc, Offset: 0, Key: 123")
		data = append(data, preambleVersion0)
		data = append(data, encoded.Bytes()...)

		inputFileName := createTempFileWithContent(t, string(data))
		outputFileName := createTempFileWithContent(t, "")

		testData := newCLITestData(t)

		cliContext := clitest.NewCLIContext(t, testData.app,
			clitest.StringArgument(FlagInputFile, inputFileName),
			clitest.StringArgument(FlagOutputFilename, outputFileName),
		)

		err := AdminKafkaParse(cliContext)

		assert.NoError(t, err)

		result, err := os.ReadFile(outputFileName)

		assert.Equal(t, "{\"creationTime\":123}\n", string(result))
	})

	t.Run("can't open input file", func(t *testing.T) {
		testData := newCLITestData(t)

		cliContext := clitest.NewCLIContext(t, testData.app,
			clitest.StringArgument(FlagInputFile, "/path/do/not/exist/input.txt"),
			clitest.StringArgument(FlagOutputFilename, "output.txt"),
		)

		err := AdminKafkaParse(cliContext)

		assert.EqualError(t, err, "Error in Admin kafka parse: : failed to open input file for reading: /path/do/not/exist/input.txt: open /path/do/not/exist/input.txt: no such file or directory")
	})

	t.Run("can't open output file", func(t *testing.T) {
		testData := newCLITestData(t)

		inputFileName := createTempFileWithContent(t, "")

		cliContext := clitest.NewCLIContext(t, testData.app,
			clitest.StringArgument(FlagInputFile, inputFileName),
			clitest.StringArgument(FlagOutputFilename, "/path/do/not/exist/output.txt"),
		)

		err := AdminKafkaParse(cliContext)

		assert.EqualError(t, err, "Error in Admin kafka parse: : failed to create output file open /path/do/not/exist/output.txt: no such file or directory")
	})
}

func TestWriteVisibilityMessage(t *testing.T) {
	t.Run("successful write", func(t *testing.T) {
		filename := createTempFileWithContent(t, "")
		file, _ := os.OpenFile(filename, os.O_WRONLY, 0600)
		defer file.Close()

		ch := newWriterChannel(kafkaMessageTypeVisibilityMsg)
		skippedCount := ptr.Int32(0)
		cliContext := clitest.NewCLIContext(t, newCLITestData(t).app)

		go func() {
			ch.VisibilityMsgChannel <- &indexer.Message{
				DomainID:   ptr.String("domain_id"),
				WorkflowID: ptr.String("workflow_id"),
				RunID:      ptr.String("run_id"),
			}
			ch.Close()
		}()

		err := writeVisibilityMessage(file, ch, skippedCount, false, false, cliContext)

		assert.NoError(t, err)

		result, _ := os.ReadFile(filename)

		assert.Equal(t, "{\"domainID\":\"domain_id\",\"workflowID\":\"workflow_id\",\"runID\":\"run_id\"}\n", string(result))
	})

	t.Run("successful write with header mode", func(t *testing.T) {
		filename := createTempFileWithContent(t, "")
		file, _ := os.OpenFile(filename, os.O_WRONLY, 0600)
		defer file.Close()

		ch := newWriterChannel(kafkaMessageTypeVisibilityMsg)
		skippedCount := ptr.Int32(0)
		cliContext := clitest.NewCLIContext(t, newCLITestData(t).app)

		go func() {
			ch.VisibilityMsgChannel <- &indexer.Message{
				DomainID:   ptr.String("domain_id"),
				WorkflowID: ptr.String("workflow_id"),
				RunID:      ptr.String("run_id"),
			}
			ch.Close()
		}()

		err := writeVisibilityMessage(file, ch, skippedCount, false, true, cliContext)

		assert.NoError(t, err)

		result, _ := os.ReadFile(filename)

		assert.Equal(t, "domain_id, workflow_id, run_id, Index, 0\n", string(result))
	})

	t.Run("returns error if can't write to the file", func(t *testing.T) {
		filename := createTempFileWithContent(t, "")
		file, _ := os.OpenFile(filename, os.O_WRONLY, 0600)

		ch := newWriterChannel(kafkaMessageTypeVisibilityMsg)
		skippedCount := ptr.Int32(0)
		cliContext := clitest.NewCLIContext(t, newCLITestData(t).app)

		go func() {
			ch.VisibilityMsgChannel <- &indexer.Message{}
			ch.Close()
		}()

		_ = file.Close()

		err := writeVisibilityMessage(file, ch, skippedCount, false, true, cliContext)

		assert.Errorf(t, err, "failed to write to file: write %v: file already closed", filename)
	})
}

func TestWriteReplicationTask(t *testing.T) {
	t.Run("successful write", func(t *testing.T) {
		filename := createTempFileWithContent(t, "")
		file, _ := os.OpenFile(filename, os.O_WRONLY, 0600)
		defer file.Close()

		ch := newWriterChannel(kafkaMessageTypeReplicationTask)
		skippedCount := ptr.Int32(0)
		cliContext := clitest.NewCLIContext(t, newCLITestData(t).app)

		go func() {
			ch.ReplicationTaskChannel <- &types.ReplicationTask{
				SourceTaskID: 123,
				CreationTime: ptr.Int64(123),
			}
			ch.Close()
		}()

		err := writeReplicationTask(file, ch, skippedCount, false, false, persistence.NewPayloadSerializer(), cliContext)

		assert.NoError(t, err)

		result, _ := os.ReadFile(filename)

		assert.Equal(t, "{\"sourceTaskId\":123,\"creationTime\":123}\n", string(result))
	})

	t.Run("successful write with header mode", func(t *testing.T) {
		filename := createTempFileWithContent(t, "")
		file, _ := os.OpenFile(filename, os.O_WRONLY, 0600)
		defer file.Close()

		ch := newWriterChannel(kafkaMessageTypeReplicationTask)
		skippedCount := ptr.Int32(0)
		cliContext := clitest.NewCLIContext(t, newCLITestData(t).app)

		go func() {
			ch.ReplicationTaskChannel <- &types.ReplicationTask{
				SourceTaskID: 123,
				CreationTime: ptr.Int64(123),
				HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{
					DomainID:   "domain_id",
					WorkflowID: "workflow_id",
					RunID:      "run_id",
				},
			}
			ch.Close()
		}()

		err := writeReplicationTask(file, ch, skippedCount, false, true, persistence.NewPayloadSerializer(), cliContext)

		assert.NoError(t, err)

		result, _ := os.ReadFile(filename)

		assert.Equal(t, "domain_id, workflow_id, run_id\n", string(result))
	})

	t.Run("returns error if can't write to the file", func(t *testing.T) {
		filename := createTempFileWithContent(t, "")
		file, _ := os.OpenFile(filename, os.O_WRONLY, 0600)

		ch := newWriterChannel(kafkaMessageTypeReplicationTask)
		skippedCount := ptr.Int32(0)
		cliContext := clitest.NewCLIContext(t, newCLITestData(t).app)

		go func() {
			ch.ReplicationTaskChannel <- &types.ReplicationTask{}
			ch.Close()
		}()

		file.Close()

		err := writeReplicationTask(file, ch, skippedCount, false, false, persistence.NewPayloadSerializer(), cliContext)

		assert.Errorf(t, err, "failed to write to file: write %v: file already closed", filename)
	})
}

func TestStartWriter(t *testing.T) {
	t.Run("successful write to replication task", func(t *testing.T) {
		filename := createTempFileWithContent(t, "")
		file, _ := os.OpenFile(filename, os.O_WRONLY, 0600)
		defer file.Close()

		ch := newWriterChannel(kafkaMessageTypeReplicationTask)
		doneCh := make(chan struct{})
		skippedCount := ptr.Int32(0)
		cliContext := clitest.NewCLIContext(t, newCLITestData(t).app)

		go func() {
			ch.ReplicationTaskChannel <- &types.ReplicationTask{
				SourceTaskID: 123,
				CreationTime: ptr.Int64(123),
			}
			ch.Close()
		}()

		err := startWriter(file, ch, doneCh, skippedCount, persistence.NewPayloadSerializer(), cliContext)

		assert.NoError(t, err)

		result, _ := os.ReadFile(filename)

		assert.Equal(t, "{\"sourceTaskId\":123,\"creationTime\":123}\n", string(result))
	})

	t.Run("successful write with visibility task", func(t *testing.T) {
		filename := createTempFileWithContent(t, "")
		file, _ := os.OpenFile(filename, os.O_WRONLY, 0600)
		defer file.Close()

		ch := newWriterChannel(kafkaMessageTypeVisibilityMsg)
		doneCh := make(chan struct{})
		skippedCount := ptr.Int32(0)
		cliContext := clitest.NewCLIContext(t, newCLITestData(t).app)

		go func() {
			ch.VisibilityMsgChannel <- &indexer.Message{
				DomainID:   ptr.String("domain_id"),
				WorkflowID: ptr.String("workflow_id"),
				RunID:      ptr.String("run_id"),
			}
			ch.Close()
		}()

		err := startWriter(file, ch, doneCh, skippedCount, persistence.NewPayloadSerializer(), cliContext)

		assert.NoError(t, err)

		result, _ := os.ReadFile(filename)

		assert.Equal(t, "{\"domainID\":\"domain_id\",\"workflowID\":\"workflow_id\",\"runID\":\"run_id\"}\n", string(result))
	})

	t.Run("unsupported type", func(t *testing.T) {
		ch := newWriterChannel(kafkaMessageType(321))
		doneCh := make(chan struct{})
		cliContext := clitest.NewCLIContext(t, newCLITestData(t).app)

		err := startWriter(nil, ch, doneCh, nil, persistence.NewPayloadSerializer(), cliContext)

		assert.EqualError(t, err, "unsupported message type: 321")
	})
}
