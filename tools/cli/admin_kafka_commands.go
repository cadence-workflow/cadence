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

package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sync/atomic"
	"time"

	"github.com/urfave/cli/v2"
	"go.uber.org/thriftrw/protocol/binary"
	"go.uber.org/thriftrw/wire"

	"github.com/uber/cadence/.gen/go/indexer"
	"github.com/uber/cadence/.gen/go/replicator"
	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/thrift"
	"github.com/uber/cadence/tools/common/commoncli"
)

type (
	filterFn              func(*types.ReplicationTask) bool
	filterFnForVisibility func(*indexer.Message) bool

	kafkaMessageType int

	historyV2Task struct {
		Task         *types.ReplicationTask
		Events       []*types.HistoryEvent
		NewRunEvents []*types.HistoryEvent
	}
)

const (
	kafkaMessageTypeReplicationTask kafkaMessageType = iota
	kafkaMessageTypeVisibilityMsg
)

const (
	bufferSize                       = 8192
	preambleVersion0            byte = 0x59
	malformedMessage                 = "Input was malformed"
	chanBufferSize                   = 10000
	maxRereplicateEventID            = 999999
	defaultResendContextTimeout      = 30 * time.Second
)

var (
	r = regexp.MustCompile(`Partition: .*?, Offset: .*?, Key: .*?`)
)

type writerChannel struct {
	Type                   kafkaMessageType
	ReplicationTaskChannel chan *types.ReplicationTask
	VisibilityMsgChannel   chan *indexer.Message
}

func newWriterChannel(messageType kafkaMessageType) *writerChannel {
	ch := &writerChannel{
		Type: messageType,
	}
	switch messageType {
	case kafkaMessageTypeReplicationTask:
		ch.ReplicationTaskChannel = make(chan *types.ReplicationTask, chanBufferSize)
	case kafkaMessageTypeVisibilityMsg:
		ch.VisibilityMsgChannel = make(chan *indexer.Message, chanBufferSize)
	}
	return ch
}

func (ch *writerChannel) Close() {
	if ch.ReplicationTaskChannel != nil {
		close(ch.ReplicationTaskChannel)
	}
	if ch.VisibilityMsgChannel != nil {
		close(ch.VisibilityMsgChannel)
	}
}

// AdminKafkaParse parses the output of k8read and outputs replication tasks
func AdminKafkaParse(c *cli.Context) error {
	inputFile, err := getInputFile(c.String(FlagInputFile))
	defer inputFile.Close()
	if err != nil {
		return commoncli.Problem("Error in Admin kafka parse: ", err)
	}
	outputFile, err := getOutputFile(c.String(FlagOutputFilename))
	defer outputFile.Close()
	if err != nil {
		return commoncli.Problem("Error in Admin kafka parse: ", err)
	}
	readerCh := make(chan []byte, chanBufferSize)
	writerCh := newWriterChannel(kafkaMessageType(c.Int(FlagMessageType)))
	doneCh := make(chan struct{})
	serializer := persistence.NewPayloadSerializer()

	var skippedCount int32
	skipErrMode := c.Bool(FlagSkipErrorMode)

	go startReader(inputFile, readerCh)
	go startParser(readerCh, writerCh, skipErrMode, &skippedCount)
	go startWriter(outputFile, writerCh, doneCh, &skippedCount, serializer, c)

	<-doneCh

	if skipErrMode {
		fmt.Printf("%v messages were skipped due to errors in parsing", atomic.LoadInt32(&skippedCount))
	}
	return nil
}

func buildFilterFn(workflowID, runID string) filterFn {
	return func(task *types.ReplicationTask) bool {
		if len(workflowID) != 0 || len(runID) != 0 {
			if task.GetHistoryTaskV2Attributes() == nil {
				return false
			}
		}
		if len(workflowID) != 0 && task.GetHistoryTaskV2Attributes().WorkflowID != workflowID {
			return false
		}
		if len(runID) != 0 && task.GetHistoryTaskV2Attributes().RunID != runID {
			return false
		}
		return true
	}
}

func buildFilterFnForVisibility(workflowID, runID string) filterFnForVisibility {
	return func(msg *indexer.Message) bool {
		if len(workflowID) != 0 && msg.GetWorkflowID() != workflowID {
			return false
		}
		if len(runID) != 0 && msg.GetRunID() != runID {
			return false
		}
		return true
	}
}

func getOutputFile(outputFile string) (*os.File, error) {
	if len(outputFile) == 0 {
		return os.Stdout, nil
	}
	f, err := os.Create(outputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file %w", err)
	}

	return f, nil
}

func startReader(file io.Reader, readerCh chan<- []byte) error {
	defer close(readerCh)
	reader := bufio.NewReader(file)

	for {
		buf := make([]byte, bufferSize)
		n, err := reader.Read(buf)
		if err != nil {
			if err != io.EOF {
				return fmt.Errorf("failed to read from reader: %w", err)
			}
			break
		}
		buf = buf[:n]
		readerCh <- buf
	}
	return nil
}

func startParser(readerCh <-chan []byte, writerCh *writerChannel, skipErrors bool, skippedCount *int32) error {
	defer writerCh.Close()

	var buffer []byte
	for data := range readerCh {
		buffer = append(buffer, data...)
		parsedData, nextBuffer, err := splitBuffer(buffer)
		if err != nil {
			if skipErrors {
				atomic.AddInt32(skippedCount, 1)
				continue
			} else {
				return fmt.Errorf("error in splitBuffer: %w", err)
			}
		}
		buffer = nextBuffer
		parse(parsedData, skipErrors, skippedCount, writerCh)
	}
	parse(buffer, skipErrors, skippedCount, writerCh)
	return nil
}

func startWriter(
	outputFile *os.File,
	writerCh *writerChannel,
	doneCh chan struct{},
	skippedCount *int32,
	serializer persistence.PayloadSerializer,
	c *cli.Context,
) error {
	defer close(doneCh)

	skipErrMode := c.Bool(FlagSkipErrorMode)
	headerMode := c.Bool(FlagHeadersMode)

	switch writerCh.Type {
	case kafkaMessageTypeReplicationTask:
		return writeReplicationTask(outputFile, writerCh, skippedCount, skipErrMode, headerMode, serializer, c)
	case kafkaMessageTypeVisibilityMsg:
		return writeVisibilityMessage(outputFile, writerCh, skippedCount, skipErrMode, headerMode, c)
	default:
		return fmt.Errorf("unsupported message type: %v", writerCh.Type)
	}
}

func writeReplicationTask(
	outputFile *os.File,
	writerCh *writerChannel,
	skippedCount *int32,
	skipErrMode bool,
	headerMode bool,
	serializer persistence.PayloadSerializer,
	c *cli.Context,
) error {
	filter := buildFilterFn(c.String(FlagWorkflowID), c.String(FlagRunID))
	for task := range writerCh.ReplicationTaskChannel {
		if filter(task) {
			jsonStr, err := decodeReplicationTask(task, serializer)
			if err != nil {
				if !skipErrMode {
					return fmt.Errorf("malformed message, failed to encode into json: %w", err)
				}
				atomic.AddInt32(skippedCount, 1)
				continue
			}

			var outStr string
			if !headerMode {
				outStr = string(jsonStr)
			} else {
				outStr = fmt.Sprintf(
					"%v, %v, %v",
					task.GetHistoryTaskV2Attributes().DomainID,
					task.GetHistoryTaskV2Attributes().WorkflowID,
					task.GetHistoryTaskV2Attributes().RunID,
				)
			}
			_, err = outputFile.WriteString(fmt.Sprintf("%v\n", outStr))
			if err != nil {
				return fmt.Errorf("failed to write to file: %w", err)
			}
		}
	}
	return nil
}

func writeVisibilityMessage(
	outputFile *os.File,
	writerCh *writerChannel,
	skippedCount *int32,
	skipErrMode bool,
	headerMode bool,
	c *cli.Context,
) error {
	filter := buildFilterFnForVisibility(c.String(FlagWorkflowID), c.String(FlagRunID))

	for msg := range writerCh.VisibilityMsgChannel {
		if filter(msg) {
			jsonStr, err := json.Marshal(msg)
			if err != nil {
				if !skipErrMode {
					return fmt.Errorf("malformed message: failed to encode into json, err: %w", err)
				}
				atomic.AddInt32(skippedCount, 1)
				continue
			}

			var outStr string
			if !headerMode {
				outStr = string(jsonStr)
			} else {
				outStr = fmt.Sprintf(
					"%v, %v, %v, %v, %v",
					msg.GetDomainID(),
					msg.GetWorkflowID(),
					msg.GetRunID(),
					msg.GetMessageType().String(),
					msg.GetVersion(),
				)
			}
			_, err = outputFile.WriteString(fmt.Sprintf("%v\n", outStr))
			if err != nil {
				return fmt.Errorf("failed to write to file: %w", err)
			}
		}
	}
	return nil
}

func splitBuffer(buffer []byte) ([]byte, []byte, error) {
	matches := r.FindAllIndex(buffer, -1)
	if len(matches) == 0 {
		return nil, nil, fmt.Errorf("header not found, did you generate dump with -v")
	}
	splitIndex := matches[len(matches)-1][0]
	return buffer[:splitIndex], buffer[splitIndex:], nil
}

func parse(bytes []byte, skipErrors bool, skippedCount *int32, writerCh *writerChannel) error {
	messages, skippedGetMsgCount, err := getMessages(bytes, skipErrors)
	if err != nil {
		return fmt.Errorf("parsing failed: %w", err)
	}
	switch writerCh.Type {
	case kafkaMessageTypeReplicationTask:
		msgs, skippedDeserializeCount, err := deserializeMessages(messages, skipErrors)
		if err != nil {
			return fmt.Errorf("parsing failed: %w", err)
		}
		atomic.AddInt32(skippedCount, skippedGetMsgCount+skippedDeserializeCount)
		for _, msg := range msgs {
			writerCh.ReplicationTaskChannel <- msg
		}
	case kafkaMessageTypeVisibilityMsg:
		msgs, skippedDeserializeCount, err := deserializeVisibilityMessages(messages, skipErrors)
		if err != nil {
			return fmt.Errorf("parsing failed: %w", err)
		}
		atomic.AddInt32(skippedCount, skippedGetMsgCount+skippedDeserializeCount)
		for _, msg := range msgs {
			writerCh.VisibilityMsgChannel <- msg
		}
	}
	return nil
}

func getMessages(data []byte, skipErrors bool) ([][]byte, int32, error) {
	str := string(data)
	messagesWithHeaders := r.Split(str, -1)
	if len(messagesWithHeaders[0]) != 0 {
		return nil, 0, fmt.Errorf(malformedMessage+"Error: %v", errors.New("got data chunk to handle that does not start with valid header"))
	}
	messagesWithHeaders = messagesWithHeaders[1:]
	var rawMessages [][]byte
	var skipped int32
	for _, m := range messagesWithHeaders {
		if len(m) == 0 {
			return nil, 0, fmt.Errorf(malformedMessage+"Error: %v", errors.New("got empty message between valid headers"))
		}
		curr := []byte(m)
		messageStart := bytes.Index(curr, []byte{preambleVersion0})
		if messageStart == -1 {
			if !skipErrors {
				return nil, 0, fmt.Errorf(malformedMessage+"Error: %v", errors.New("failed to find message preamble"))
			}
			skipped++
			continue
		}
		rawMessages = append(rawMessages, curr[messageStart:])
	}
	return rawMessages, skipped, nil
}

func deserializeMessages(messages [][]byte, skipErrors bool) ([]*types.ReplicationTask, int32, error) {
	var replicationTasks []*types.ReplicationTask
	var skipped int32
	for _, m := range messages {
		var task replicator.ReplicationTask
		err := decode(m, &task)
		if err != nil {
			if !skipErrors {
				return nil, 0, fmt.Errorf(malformedMessage+"Error: %v", err)
			}
			skipped++
			continue
		}
		replicationTasks = append(replicationTasks, thrift.ToReplicationTask(&task))
	}
	return replicationTasks, skipped, nil
}

func decode(message []byte, val *replicator.ReplicationTask) error {
	reader := bytes.NewReader(message[1:])
	wireVal, err := binary.Default.Decode(reader, wire.TStruct)
	if err != nil {
		return err
	}
	return val.FromWire(wireVal)
}

func deserializeVisibilityMessages(messages [][]byte, skipErrors bool) ([]*indexer.Message, int32, error) {
	var visibilityMessages []*indexer.Message
	var skipped int32
	for _, m := range messages {
		var msg indexer.Message
		err := decodeVisibility(m, &msg)
		if err != nil {
			if !skipErrors {
				return nil, 0, fmt.Errorf(malformedMessage+"Error: %v", err)
			}
			skipped++
			continue
		}
		visibilityMessages = append(visibilityMessages, &msg)
	}
	return visibilityMessages, skipped, nil
}

func decodeVisibility(message []byte, val *indexer.Message) error {
	reader := bytes.NewReader(message[1:])
	wireVal, err := binary.Default.Decode(reader, wire.TStruct)
	if err != nil {
		return err
	}
	return val.FromWire(wireVal)
}

// ClustersConfig describes the kafka clusters
type ClustersConfig struct {
	Clusters map[string]config.ClusterConfig
	TLS      config.TLS
}

func doRereplicate(
	ctx context.Context,
	domainID string,
	wid string,
	rid string,
	endEventID *int64,
	endEventVersion *int64,
	sourceCluster string,
	adminClient admin.Client,
) error {
	fmt.Printf("Start rereplication for wid: %v, rid:%v \n", wid, rid)
	if err := adminClient.ResendReplicationTasks(
		ctx,
		&types.ResendReplicationTasksRequest{
			DomainID:      domainID,
			WorkflowID:    wid,
			RunID:         rid,
			RemoteCluster: sourceCluster,
			EndEventID:    endEventID,
			EndVersion:    endEventVersion,
		},
	); err != nil {
		return commoncli.Problem("Failed to resend ndc workflow", err)
	}
	fmt.Printf("Done rereplication for wid: %v, rid:%v \n", wid, rid)
	return nil
}

// AdminRereplicate parses will re-publish replication tasks to topic
func AdminRereplicate(c *cli.Context) error {
	sourceCluster, err := getRequiredOption(c, FlagSourceCluster)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	adminClient, err := getDeps(c).ServerAdminClient(c)
	if err != nil {
		return err
	}
	var endEventID, endVersion *int64
	if c.IsSet(FlagMaxEventID) {
		endEventID = common.Int64Ptr(c.Int64(FlagMaxEventID) + 1)
	}
	if c.IsSet(FlagEndEventVersion) {
		endVersion = common.Int64Ptr(c.Int64(FlagEndEventVersion))
	}
	domainID, err := getRequiredOption(c, FlagDomainID)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	wid, err := getRequiredOption(c, FlagWorkflowID)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	rid, err := getRequiredOption(c, FlagRunID)
	if err != nil {
		return commoncli.Problem("Required flag not found: ", err)
	}
	contextTimeout := defaultResendContextTimeout

	if c.IsSet(FlagContextTimeout) {
		contextTimeout = time.Duration(c.Int(FlagContextTimeout)) * time.Second
	}
	ctx, cancel := context.WithTimeout(c.Context, contextTimeout)
	defer cancel()

	return doRereplicate(
		ctx,
		domainID,
		wid,
		rid,
		endEventID,
		endVersion,
		sourceCluster,
		adminClient,
	)
}

func decodeReplicationTask(
	task *types.ReplicationTask,
	serializer persistence.PayloadSerializer,
) ([]byte, error) {
	switch task.GetTaskType() {
	case types.ReplicationTaskTypeHistoryV2:
		historyV2 := task.GetHistoryTaskV2Attributes()
		events, err := serializer.DeserializeBatchEvents(
			persistence.NewDataBlobFromInternal(historyV2.Events),
		)
		if err != nil {
			return nil, err
		}
		var newRunEvents []*types.HistoryEvent
		if historyV2.NewRunEvents != nil {
			newRunEvents, err = serializer.DeserializeBatchEvents(
				persistence.NewDataBlobFromInternal(historyV2.NewRunEvents),
			)
			if err != nil {
				return nil, err
			}
		}
		historyV2.Events = nil
		historyV2.NewRunEvents = nil
		historyV2Attributes := &historyV2Task{
			Task:         task,
			Events:       events,
			NewRunEvents: newRunEvents,
		}
		return json.Marshal(historyV2Attributes)
	default:
		return json.Marshal(task)
	}
}
