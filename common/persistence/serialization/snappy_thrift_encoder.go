// The MIT License (MIT)
//
// Copyright (c) 2017-2020 Uber Technologies Inc.
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

package serialization

import (
	"bytes"

	"github.com/golang/snappy"
	"go.uber.org/thriftrw/protocol/binary"

	"github.com/uber/cadence/common/constants"
)

type snappyThriftEncoder struct{}

func newSnappyThriftEncoder() encoder {
	return &snappyThriftEncoder{}
}

func (e *snappyThriftEncoder) shardInfoToBlob(info *ShardInfo) ([]byte, error) {
	return snappyThriftRWEncode(shardInfoToThrift(info))
}

func (e *snappyThriftEncoder) domainInfoToBlob(info *DomainInfo) ([]byte, error) {
	return snappyThriftRWEncode(domainInfoToThrift(info))
}

func (e *snappyThriftEncoder) historyTreeInfoToBlob(info *HistoryTreeInfo) ([]byte, error) {
	return snappyThriftRWEncode(historyTreeInfoToThrift(info))
}

func (e *snappyThriftEncoder) workflowExecutionInfoToBlob(info *WorkflowExecutionInfo) ([]byte, error) {
	return snappyThriftRWEncode(workflowExecutionInfoToThrift(info))
}

func (e *snappyThriftEncoder) activityInfoToBlob(info *ActivityInfo) ([]byte, error) {
	return snappyThriftRWEncode(activityInfoToThrift(info))
}

func (e *snappyThriftEncoder) childExecutionInfoToBlob(info *ChildExecutionInfo) ([]byte, error) {
	return snappyThriftRWEncode(childExecutionInfoToThrift(info))
}

func (e *snappyThriftEncoder) signalInfoToBlob(info *SignalInfo) ([]byte, error) {
	return snappyThriftRWEncode(signalInfoToThrift(info))
}

func (e *snappyThriftEncoder) requestCancelInfoToBlob(info *RequestCancelInfo) ([]byte, error) {
	return snappyThriftRWEncode(requestCancelInfoToThrift(info))
}

func (e *snappyThriftEncoder) timerInfoToBlob(info *TimerInfo) ([]byte, error) {
	return snappyThriftRWEncode(timerInfoToThrift(info))
}

func (e *snappyThriftEncoder) taskInfoToBlob(info *TaskInfo) ([]byte, error) {
	return snappyThriftRWEncode(taskInfoToThrift(info))
}

func (e *snappyThriftEncoder) taskListInfoToBlob(info *TaskListInfo) ([]byte, error) {
	return snappyThriftRWEncode(taskListInfoToThrift(info))
}

func (e *snappyThriftEncoder) transferTaskInfoToBlob(info *TransferTaskInfo) ([]byte, error) {
	return snappyThriftRWEncode(transferTaskInfoToThrift(info))
}

func (e *snappyThriftEncoder) crossClusterTaskInfoToBlob(info *CrossClusterTaskInfo) ([]byte, error) {
	return snappyThriftRWEncode(crossClusterTaskInfoToThrift(info))
}

func (e *snappyThriftEncoder) timerTaskInfoToBlob(info *TimerTaskInfo) ([]byte, error) {
	return snappyThriftRWEncode(timerTaskInfoToThrift(info))
}

func (e *snappyThriftEncoder) replicationTaskInfoToBlob(info *ReplicationTaskInfo) ([]byte, error) {
	return snappyThriftRWEncode(replicationTaskInfoToThrift(info))
}

func (e *snappyThriftEncoder) encodingType() constants.EncodingType {
	return constants.EncodingTypeThriftRWSnappy
}

func snappyThriftRWEncode(t thriftRWType) ([]byte, error) {
	var b bytes.Buffer
	sw := binary.Default.Writer(&b)
	defer sw.Close()
	if err := t.Encode(sw); err != nil {
		return nil, err
	}

	return snappy.Encode(nil, b.Bytes()), nil
}
