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
	"bytes"

	"github.com/golang/snappy"
	"go.uber.org/thriftrw/protocol/binary"

	"github.com/uber/cadence/.gen/go/sqlblobs"
)

type snappyThriftDecoder struct{}

func newSnappyThriftDecoder() decoder {
	return &snappyThriftDecoder{}
}

func (d *snappyThriftDecoder) shardInfoFromBlob(data []byte) (*ShardInfo, error) {
	result := &sqlblobs.ShardInfo{}
	if err := snappyThriftRWDecode(data, result); err != nil {
		return nil, err
	}
	return shardInfoFromThrift(result), nil
}

func (d *snappyThriftDecoder) domainInfoFromBlob(data []byte) (*DomainInfo, error) {
	result := &sqlblobs.DomainInfo{}
	if err := snappyThriftRWDecode(data, result); err != nil {
		return nil, err
	}
	return domainInfoFromThrift(result), nil
}

func (d *snappyThriftDecoder) historyTreeInfoFromBlob(data []byte) (*HistoryTreeInfo, error) {
	result := &sqlblobs.HistoryTreeInfo{}
	if err := snappyThriftRWDecode(data, result); err != nil {
		return nil, err
	}
	return historyTreeInfoFromThrift(result), nil
}

func (d *snappyThriftDecoder) workflowExecutionInfoFromBlob(data []byte) (*WorkflowExecutionInfo, error) {
	result := &sqlblobs.WorkflowExecutionInfo{}
	if err := snappyThriftRWDecode(data, result); err != nil {
		return nil, err
	}
	return workflowExecutionInfoFromThrift(result), nil
}

func (d *snappyThriftDecoder) activityInfoFromBlob(data []byte) (*ActivityInfo, error) {
	result := &sqlblobs.ActivityInfo{}
	if err := snappyThriftRWDecode(data, result); err != nil {
		return nil, err
	}
	return activityInfoFromThrift(result), nil
}

func (d *snappyThriftDecoder) childExecutionInfoFromBlob(data []byte) (*ChildExecutionInfo, error) {
	result := &sqlblobs.ChildExecutionInfo{}
	if err := snappyThriftRWDecode(data, result); err != nil {
		return nil, err
	}
	return childExecutionInfoFromThrift(result), nil
}

func (d *snappyThriftDecoder) signalInfoFromBlob(data []byte) (*SignalInfo, error) {
	result := &sqlblobs.SignalInfo{}
	if err := snappyThriftRWDecode(data, result); err != nil {
		return nil, err
	}
	return signalInfoFromThrift(result), nil
}

func (d *snappyThriftDecoder) requestCancelInfoFromBlob(data []byte) (*RequestCancelInfo, error) {
	result := &sqlblobs.RequestCancelInfo{}
	if err := snappyThriftRWDecode(data, result); err != nil {
		return nil, err
	}
	return requestCancelInfoFromThrift(result), nil
}

func (d *snappyThriftDecoder) timerInfoFromBlob(data []byte) (*TimerInfo, error) {
	result := &sqlblobs.TimerInfo{}
	if err := snappyThriftRWDecode(data, result); err != nil {
		return nil, err
	}
	return timerInfoFromThrift(result), nil
}

func (d *snappyThriftDecoder) taskInfoFromBlob(data []byte) (*TaskInfo, error) {
	result := &sqlblobs.TaskInfo{}
	if err := snappyThriftRWDecode(data, result); err != nil {
		return nil, err
	}
	return taskInfoFromThrift(result), nil
}

func (d *snappyThriftDecoder) taskListInfoFromBlob(data []byte) (*TaskListInfo, error) {
	result := &sqlblobs.TaskListInfo{}
	if err := snappyThriftRWDecode(data, result); err != nil {
		return nil, err
	}
	return taskListInfoFromThrift(result), nil
}

func (d *snappyThriftDecoder) transferTaskInfoFromBlob(data []byte) (*TransferTaskInfo, error) {
	result := &sqlblobs.TransferTaskInfo{}
	if err := snappyThriftRWDecode(data, result); err != nil {
		return nil, err
	}
	return transferTaskInfoFromThrift(result), nil
}

func (d *snappyThriftDecoder) crossClusterTaskInfoFromBlob(data []byte) (*CrossClusterTaskInfo, error) {
	result := &sqlblobs.TransferTaskInfo{}
	if err := snappyThriftRWDecode(data, result); err != nil {
		return nil, err
	}
	return crossClusterTaskInfoFromThrift(result), nil
}

func (d *snappyThriftDecoder) timerTaskInfoFromBlob(data []byte) (*TimerTaskInfo, error) {
	result := &sqlblobs.TimerTaskInfo{}
	if err := snappyThriftRWDecode(data, result); err != nil {
		return nil, err
	}
	return timerTaskInfoFromThrift(result), nil
}

func (d *snappyThriftDecoder) replicationTaskInfoFromBlob(data []byte) (*ReplicationTaskInfo, error) {
	result := &sqlblobs.ReplicationTaskInfo{}
	if err := snappyThriftRWDecode(data, result); err != nil {
		return nil, err
	}
	return replicationTaskInfoFromThrift(result), nil
}

func snappyThriftRWDecode(b []byte, result thriftRWType) error {
	decompressed, err := snappy.Decode(nil, b)
	if err != nil {
		return err
	}

	buf := bytes.NewReader(decompressed)
	sr := binary.Default.Reader(buf)
	return result.Decode(sr)
}
