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
	"encoding/json"
	"fmt"
	"time"

	"github.com/golang/snappy"

	"github.com/uber/cadence/.gen/go/config"
	"github.com/uber/cadence/.gen/go/history"
	"github.com/uber/cadence/.gen/go/replicator"
	workflow "github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/.gen/go/sqlblobs"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/checksum"
	"github.com/uber/cadence/common/codec"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/thrift"
)

//go:generate mockgen -package $GOPACKAGE -destination serializer_mock.go -self_package github.com/uber/cadence/common/persistence github.com/uber/cadence/common/persistence PayloadSerializer

type (
	// PayloadSerializer is used by persistence to serialize/deserialize history event(s) and others
	// It will only be used inside persistence, so that serialize/deserialize is transparent for application
	PayloadSerializer interface {
		// serialize/deserialize history events
		SerializeBatchEvents(batch []*types.HistoryEvent, encodingType constants.EncodingType) (*DataBlob, error)
		DeserializeBatchEvents(data *DataBlob) ([]*types.HistoryEvent, error)

		// serialize/deserialize a single history event
		SerializeEvent(event *types.HistoryEvent, encodingType constants.EncodingType) (*DataBlob, error)
		DeserializeEvent(data *DataBlob) (*types.HistoryEvent, error)

		// serialize/deserialize visibility memo fields
		SerializeVisibilityMemo(memo *types.Memo, encodingType constants.EncodingType) (*DataBlob, error)
		DeserializeVisibilityMemo(data *DataBlob) (*types.Memo, error)

		// serialize/deserialize reset points
		SerializeResetPoints(event *types.ResetPoints, encodingType constants.EncodingType) (*DataBlob, error)
		DeserializeResetPoints(data *DataBlob) (*types.ResetPoints, error)

		// serialize/deserialize bad binaries
		SerializeBadBinaries(event *types.BadBinaries, encodingType constants.EncodingType) (*DataBlob, error)
		DeserializeBadBinaries(data *DataBlob) (*types.BadBinaries, error)

		// serialize/deserialize version histories
		SerializeVersionHistories(histories *types.VersionHistories, encodingType constants.EncodingType) (*DataBlob, error)
		DeserializeVersionHistories(data *DataBlob) (*types.VersionHistories, error)

		// serialize/deserialize pending failover markers
		SerializePendingFailoverMarkers(markers []*types.FailoverMarkerAttributes, encodingType constants.EncodingType) (*DataBlob, error)
		DeserializePendingFailoverMarkers(data *DataBlob) ([]*types.FailoverMarkerAttributes, error)

		// serialize/deserialize processing queue statesss
		SerializeProcessingQueueStates(states *types.ProcessingQueueStates, encodingType constants.EncodingType) (*DataBlob, error)
		DeserializeProcessingQueueStates(data *DataBlob) (*types.ProcessingQueueStates, error)

		// serialize/deserialize DynamicConfigBlob
		SerializeDynamicConfigBlob(blob *types.DynamicConfigBlob, encodingType constants.EncodingType) (*DataBlob, error)
		DeserializeDynamicConfigBlob(data *DataBlob) (*types.DynamicConfigBlob, error)

		// serialize/deserialize IsolationGroupConfiguration
		SerializeIsolationGroups(event *types.IsolationGroupConfiguration, encodingType constants.EncodingType) (*DataBlob, error)
		DeserializeIsolationGroups(data *DataBlob) (*types.IsolationGroupConfiguration, error)

		// serialize/deserialize async workflow configuration
		SerializeAsyncWorkflowsConfig(config *types.AsyncWorkflowConfiguration, encodingType constants.EncodingType) (*DataBlob, error)
		DeserializeAsyncWorkflowsConfig(data *DataBlob) (*types.AsyncWorkflowConfiguration, error)

		// serialize/deserialize checksum
		SerializeChecksum(sum checksum.Checksum, encodingType constants.EncodingType) (*DataBlob, error)
		DeserializeChecksum(data *DataBlob) (checksum.Checksum, error)

		// serialize/deserialize active clusters config
		SerializeActiveClusters(activeClusters *types.ActiveClusters, encodingType constants.EncodingType) (*DataBlob, error)
		DeserializeActiveClusters(data *DataBlob) (*types.ActiveClusters, error)

		// serialize/deserialize active cluster selection policy
		SerializeActiveClusterSelectionPolicy(policy *types.ActiveClusterSelectionPolicy, encodingType constants.EncodingType) (*DataBlob, error)
		DeserializeActiveClusterSelectionPolicy(data *DataBlob) (*types.ActiveClusterSelectionPolicy, error)

		// serialize/deserialize workflow timer tasks
		SerializeWorkflowTimerTasks(tasks []*WorkflowTimerTaskInfo, encodingType constants.EncodingType) (*DataBlob, error)
		DeserializeWorkflowTimerTasks(data *DataBlob) ([]*WorkflowTimerTaskInfo, error)
	}

	// CadenceSerializationError is an error type for cadence serialization
	CadenceSerializationError struct {
		msg string
	}

	// CadenceDeserializationError is an error type for cadence deserialization
	CadenceDeserializationError struct {
		msg string
	}

	// UnknownEncodingTypeError is an error type for unknown or unsupported encoding type
	UnknownEncodingTypeError struct {
		encodingType constants.EncodingType
	}

	serializerImpl struct {
		thriftrwEncoder codec.BinaryEncoder
	}
)

// NewPayloadSerializer returns a PayloadSerializer
func NewPayloadSerializer() PayloadSerializer {
	return &serializerImpl{
		thriftrwEncoder: codec.NewThriftRWEncoder(),
	}
}

func (t *serializerImpl) SerializeBatchEvents(events []*types.HistoryEvent, encodingType constants.EncodingType) (*DataBlob, error) {
	return t.serialize(events, encodingType)
}

func (t *serializerImpl) DeserializeBatchEvents(data *DataBlob) ([]*types.HistoryEvent, error) {
	if data == nil {
		return nil, nil
	}
	var events []*types.HistoryEvent
	if data != nil && len(data.Data) == 0 {
		return events, nil
	}
	err := t.deserialize(data, &events)
	return events, err
}

func (t *serializerImpl) SerializeEvent(event *types.HistoryEvent, encodingType constants.EncodingType) (*DataBlob, error) {
	if event == nil {
		return nil, nil
	}
	return t.serialize(event, encodingType)
}

func (t *serializerImpl) DeserializeEvent(data *DataBlob) (*types.HistoryEvent, error) {
	if data == nil {
		return nil, nil
	}
	var event types.HistoryEvent
	err := t.deserialize(data, &event)
	return &event, err
}

func (t *serializerImpl) SerializeResetPoints(rp *types.ResetPoints, encodingType constants.EncodingType) (*DataBlob, error) {
	if rp == nil {
		rp = &types.ResetPoints{}
	}
	return t.serialize(rp, encodingType)
}

func (t *serializerImpl) DeserializeResetPoints(data *DataBlob) (*types.ResetPoints, error) {
	var rp types.ResetPoints
	err := t.deserialize(data, &rp)
	return &rp, err
}

func (t *serializerImpl) SerializeBadBinaries(bb *types.BadBinaries, encodingType constants.EncodingType) (*DataBlob, error) {
	if bb == nil {
		bb = &types.BadBinaries{}
	}
	return t.serialize(bb, encodingType)
}

func (t *serializerImpl) DeserializeBadBinaries(data *DataBlob) (*types.BadBinaries, error) {
	var bb types.BadBinaries
	err := t.deserialize(data, &bb)
	return &bb, err
}

func (t *serializerImpl) SerializeVisibilityMemo(memo *types.Memo, encodingType constants.EncodingType) (*DataBlob, error) {
	if memo == nil {
		// Return nil here to be consistent with Event
		// This check is not duplicate as check in following serialize
		return nil, nil
	}
	return t.serialize(memo, encodingType)
}

func (t *serializerImpl) DeserializeVisibilityMemo(data *DataBlob) (*types.Memo, error) {
	var memo types.Memo
	err := t.deserialize(data, &memo)
	return &memo, err
}

func (t *serializerImpl) SerializeVersionHistories(histories *types.VersionHistories, encodingType constants.EncodingType) (*DataBlob, error) {
	if histories == nil {
		return nil, nil
	}
	return t.serialize(histories, encodingType)
}

func (t *serializerImpl) DeserializeVersionHistories(data *DataBlob) (*types.VersionHistories, error) {
	var histories types.VersionHistories
	err := t.deserialize(data, &histories)
	return &histories, err
}

func (t *serializerImpl) SerializePendingFailoverMarkers(markers []*types.FailoverMarkerAttributes, encodingType constants.EncodingType) (*DataBlob, error) {
	if markers == nil {
		return nil, nil
	}
	return t.serialize(markers, encodingType)
}

func (t *serializerImpl) DeserializePendingFailoverMarkers(data *DataBlob) ([]*types.FailoverMarkerAttributes, error) {
	if data == nil {
		return nil, nil
	}
	var markers []*types.FailoverMarkerAttributes
	if data != nil && len(data.Data) == 0 {
		return markers, nil
	}
	err := t.deserialize(data, &markers)
	return markers, err
}

func (t *serializerImpl) SerializeProcessingQueueStates(states *types.ProcessingQueueStates, encodingType constants.EncodingType) (*DataBlob, error) {
	if states == nil {
		return nil, nil
	}
	return t.serialize(states, encodingType)
}

func (t *serializerImpl) DeserializeProcessingQueueStates(data *DataBlob) (*types.ProcessingQueueStates, error) {
	if data == nil {
		return nil, nil
	}

	var states types.ProcessingQueueStates
	if data != nil && len(data.Data) == 0 {
		return &states, nil
	}
	err := t.deserialize(data, &states)
	return &states, err
}

func (t *serializerImpl) SerializeDynamicConfigBlob(blob *types.DynamicConfigBlob, encodingType constants.EncodingType) (*DataBlob, error) {
	if blob == nil {
		return nil, nil
	}
	return t.serialize(blob, encodingType)
}

func (t *serializerImpl) DeserializeDynamicConfigBlob(data *DataBlob) (*types.DynamicConfigBlob, error) {
	if data == nil {
		return nil, nil
	}

	var blob types.DynamicConfigBlob
	if len(data.Data) == 0 {
		return &blob, nil
	}

	err := t.deserialize(data, &blob)
	return &blob, err
}

func (t *serializerImpl) SerializeIsolationGroups(c *types.IsolationGroupConfiguration, encodingType constants.EncodingType) (*DataBlob, error) {
	if c == nil {
		return nil, nil
	}
	return t.serialize(c, encodingType)
}

func (t *serializerImpl) DeserializeIsolationGroups(data *DataBlob) (*types.IsolationGroupConfiguration, error) {
	if data == nil {
		return nil, nil
	}

	var cfg types.IsolationGroupConfiguration
	if len(data.Data) == 0 {
		return &cfg, nil
	}

	err := t.deserialize(data, &cfg)
	return &cfg, err
}

func (t *serializerImpl) SerializeAsyncWorkflowsConfig(c *types.AsyncWorkflowConfiguration, encodingType constants.EncodingType) (*DataBlob, error) {
	if c == nil {
		return nil, nil
	}
	return t.serialize(c, encodingType)
}

func (t *serializerImpl) DeserializeAsyncWorkflowsConfig(data *DataBlob) (*types.AsyncWorkflowConfiguration, error) {
	if data == nil {
		return nil, nil
	}

	var cfg types.AsyncWorkflowConfiguration
	if len(data.Data) == 0 {
		return &cfg, nil
	}

	err := t.deserialize(data, &cfg)
	return &cfg, err
}

func (t *serializerImpl) SerializeChecksum(sum checksum.Checksum, encodingType constants.EncodingType) (*DataBlob, error) {
	if len(sum.Value) == 0 {
		return nil, nil
	}
	return t.serialize(sum, encodingType)
}

func (t *serializerImpl) DeserializeChecksum(data *DataBlob) (checksum.Checksum, error) {
	if data == nil {
		return checksum.Checksum{}, nil
	}

	var sum checksum.Checksum
	if len(data.Data) == 0 {
		return sum, nil
	}

	err := t.deserialize(data, &sum)
	if err != nil {
		return checksum.Checksum{}, err
	}
	return sum, err
}

func (t *serializerImpl) SerializeActiveClusters(activeClusters *types.ActiveClusters, encodingType constants.EncodingType) (*DataBlob, error) {
	if activeClusters == nil {
		return nil, nil
	}
	return t.serialize(activeClusters, encodingType)
}

func (t *serializerImpl) DeserializeActiveClusters(data *DataBlob) (*types.ActiveClusters, error) {
	if data == nil {
		return nil, nil
	}

	var activeClusters types.ActiveClusters
	if len(data.Data) == 0 {
		return &activeClusters, nil
	}
	err := t.deserialize(data, &activeClusters)
	return &activeClusters, err
}

func (t *serializerImpl) SerializeActiveClusterSelectionPolicy(policy *types.ActiveClusterSelectionPolicy, encodingType constants.EncodingType) (*DataBlob, error) {
	if policy == nil {
		return nil, nil
	}
	return t.serialize(policy, encodingType)
}

func (t *serializerImpl) DeserializeActiveClusterSelectionPolicy(data *DataBlob) (*types.ActiveClusterSelectionPolicy, error) {
	if data == nil {
		return nil, nil
	}

	var policy types.ActiveClusterSelectionPolicy
	err := t.deserialize(data, &policy)
	return &policy, err
}

func (t *serializerImpl) SerializeWorkflowTimerTasks(tasks []*WorkflowTimerTaskInfo, encodingType constants.EncodingType) (*DataBlob, error) {
	if len(tasks) == 0 {
		return nil, nil
	}
	return t.serialize(tasks, encodingType)
}

func (t *serializerImpl) DeserializeWorkflowTimerTasks(data *DataBlob) ([]*WorkflowTimerTaskInfo, error) {
	if data == nil {
		return nil, nil
	}
	var tasks []*WorkflowTimerTaskInfo
	if len(data.Data) == 0 {
		return tasks, nil
	}
	err := t.deserialize(data, &tasks)
	return tasks, err
}

func (t *serializerImpl) serialize(input interface{}, encodingType constants.EncodingType) (*DataBlob, error) {
	if input == nil {
		return nil, nil
	}

	var data []byte
	var err error

	switch encodingType {
	case constants.EncodingTypeThriftRW:
		data, err = t.thriftrwEncode(input)
	case constants.EncodingTypeThriftRWSnappy:
		data, err = t.thriftrwsnappyEncode(input)
	case constants.EncodingTypeJSON, constants.EncodingTypeUnknown, constants.EncodingTypeEmpty: // For backward-compatibility
		encodingType = constants.EncodingTypeJSON
		data, err = json.Marshal(input)
	default:
		return nil, NewUnknownEncodingTypeError(encodingType)
	}

	if err != nil {
		return nil, NewCadenceSerializationError(err.Error())
	}
	return NewDataBlob(data, encodingType), nil
}

func (t *serializerImpl) thriftrwEncode(input interface{}) ([]byte, error) {

	switch input := input.(type) {
	case []*types.HistoryEvent:
		return t.thriftrwEncoder.Encode(&workflow.History{Events: thrift.FromHistoryEventArray(input)})
	case *types.HistoryEvent:
		return t.thriftrwEncoder.Encode(thrift.FromHistoryEvent(input))
	case *types.Memo:
		return t.thriftrwEncoder.Encode(thrift.FromMemo(input))
	case *types.ResetPoints:
		return t.thriftrwEncoder.Encode(thrift.FromResetPoints(input))
	case *types.BadBinaries:
		return t.thriftrwEncoder.Encode(thrift.FromBadBinaries(input))
	case *types.VersionHistories:
		return t.thriftrwEncoder.Encode(thrift.FromVersionHistories(input))
	case []*types.FailoverMarkerAttributes:
		return t.thriftrwEncoder.Encode(&replicator.FailoverMarkers{FailoverMarkers: thrift.FromFailoverMarkerAttributesArray(input)})
	case *types.ProcessingQueueStates:
		return t.thriftrwEncoder.Encode(thrift.FromProcessingQueueStates(input))
	case *types.DynamicConfigBlob:
		return t.thriftrwEncoder.Encode(thrift.FromDynamicConfigBlob(input))
	case *types.IsolationGroupConfiguration:
		return t.thriftrwEncoder.Encode(thrift.FromIsolationGroupConfig(input))
	case *types.AsyncWorkflowConfiguration:
		return t.thriftrwEncoder.Encode(thrift.FromDomainAsyncWorkflowConfiguraton(input))
	case *types.ActiveClusters:
		return t.thriftrwEncoder.Encode(thrift.FromActiveClusters(input))
	case *types.ActiveClusterSelectionPolicy:
		return t.thriftrwEncoder.Encode(thrift.FromActiveClusterSelectionPolicy(input))
	case []*WorkflowTimerTaskInfo:
		return t.thriftrwEncoder.Encode(workflowTimerTaskInfoToThrift(input))
	default:
		return nil, nil
	}
}

func (t *serializerImpl) deserialize(data *DataBlob, target interface{}) error {
	if data == nil {
		return nil
	}
	if len(data.Data) == 0 {
		return NewCadenceDeserializationError("DeserializeEvent empty data")
	}
	var err error

	switch data.GetEncoding() {
	case constants.EncodingTypeThriftRW:
		err = t.thriftrwDecode(data.Data, target)
	case constants.EncodingTypeThriftRWSnappy:
		err = t.thriftrwsnappyDecode(data.Data, target)
	case constants.EncodingTypeJSON, constants.EncodingTypeUnknown, constants.EncodingTypeEmpty: // For backward-compatibility
		err = json.Unmarshal(data.Data, target)
	default:
		return NewUnknownEncodingTypeError(data.GetEncoding())
	}

	if err != nil {
		return NewCadenceDeserializationError(fmt.Sprintf("DeserializeBatchEvents encoding: \"%v\", error: %v", data.Encoding, err.Error()))
	}
	return nil
}

func (t *serializerImpl) thriftrwDecode(data []byte, target interface{}) error {
	switch target := target.(type) {
	case *[]*types.HistoryEvent:
		thriftTarget := workflow.History{}
		if err := t.thriftrwEncoder.Decode(data, &thriftTarget); err != nil {
			return err
		}
		*target = thrift.ToHistoryEventArray(thriftTarget.GetEvents())
		return nil
	case *types.HistoryEvent:
		thriftTarget := workflow.HistoryEvent{}
		if err := t.thriftrwEncoder.Decode(data, &thriftTarget); err != nil {
			return err
		}
		*target = *thrift.ToHistoryEvent(&thriftTarget)
		return nil
	case *types.Memo:
		thriftTarget := workflow.Memo{}
		if err := t.thriftrwEncoder.Decode(data, &thriftTarget); err != nil {
			return err
		}
		*target = *thrift.ToMemo(&thriftTarget)
		return nil
	case *types.ResetPoints:
		thriftTarget := workflow.ResetPoints{}
		if err := t.thriftrwEncoder.Decode(data, &thriftTarget); err != nil {
			return err
		}
		*target = *thrift.ToResetPoints(&thriftTarget)
		return nil
	case *types.BadBinaries:
		thriftTarget := workflow.BadBinaries{}
		if err := t.thriftrwEncoder.Decode(data, &thriftTarget); err != nil {
			return err
		}
		*target = *thrift.ToBadBinaries(&thriftTarget)
		return nil
	case *types.VersionHistories:
		thriftTarget := workflow.VersionHistories{}
		if err := t.thriftrwEncoder.Decode(data, &thriftTarget); err != nil {
			return err
		}
		*target = *thrift.ToVersionHistories(&thriftTarget)
		return nil
	case *[]*types.FailoverMarkerAttributes:
		thriftTarget := replicator.FailoverMarkers{}
		if err := t.thriftrwEncoder.Decode(data, &thriftTarget); err != nil {
			return err
		}
		*target = thrift.ToFailoverMarkerAttributesArray(thriftTarget.GetFailoverMarkers())
		return nil
	case *types.ProcessingQueueStates:
		thriftTarget := history.ProcessingQueueStates{}
		if err := t.thriftrwEncoder.Decode(data, &thriftTarget); err != nil {
			return err
		}
		*target = *thrift.ToProcessingQueueStates(&thriftTarget)
		return nil
	case *types.DynamicConfigBlob:
		thriftTarget := config.DynamicConfigBlob{}
		if err := t.thriftrwEncoder.Decode(data, &thriftTarget); err != nil {
			return err
		}
		*target = *thrift.ToDynamicConfigBlob(&thriftTarget)
		return nil
	case *types.IsolationGroupConfiguration:
		thriftTarget := workflow.IsolationGroupConfiguration{}
		if err := t.thriftrwEncoder.Decode(data, &thriftTarget); err != nil {
			return err
		}
		*target = *thrift.ToIsolationGroupConfig(&thriftTarget)
		return nil
	case *types.AsyncWorkflowConfiguration:
		thriftTarget := workflow.AsyncWorkflowConfiguration{}
		if err := t.thriftrwEncoder.Decode(data, &thriftTarget); err != nil {
			return err
		}
		*target = *thrift.ToDomainAsyncWorkflowConfiguraton(&thriftTarget)
		return nil
	case *types.ActiveClusters:
		thriftTarget := workflow.ActiveClusters{}
		if err := t.thriftrwEncoder.Decode(data, &thriftTarget); err != nil {
			return err
		}
		*target = *thrift.ToActiveClusters(&thriftTarget)
		return nil
	case *types.ActiveClusterSelectionPolicy:
		thriftTarget := workflow.ActiveClusterSelectionPolicy{}
		if err := t.thriftrwEncoder.Decode(data, &thriftTarget); err != nil {
			return err
		}
		*target = *thrift.ToActiveClusterSelectionPolicy(&thriftTarget)
		return nil
	case *[]*WorkflowTimerTaskInfo:
		thriftTarget := sqlblobs.WorkflowTimerTaskInfo{}
		if err := t.thriftrwEncoder.Decode(data, &thriftTarget); err != nil {
			return err
		}
		*target = workflowTimerTaskInfoFromThrift(&thriftTarget)
		return nil
	default:
		return nil
	}
}

func (t *serializerImpl) thriftrwsnappyEncode(input interface{}) ([]byte, error) {
	data, err := t.thriftrwEncode(input)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}

	return snappy.Encode(nil, data), nil
}

func (t *serializerImpl) thriftrwsnappyDecode(data []byte, target interface{}) error {
	decompressed, err := snappy.Decode(nil, data)
	if err != nil {
		return err
	}

	return t.thriftrwDecode(decompressed, target)
}

// NewUnknownEncodingTypeError returns a new instance of encoding type error
func NewUnknownEncodingTypeError(encodingType constants.EncodingType) error {
	return &UnknownEncodingTypeError{encodingType: encodingType}
}

func (e *UnknownEncodingTypeError) Error() string {
	return fmt.Sprintf("unknown or unsupported encoding type %v", e.encodingType)
}

// NewCadenceSerializationError returns a CadenceSerializationError
func NewCadenceSerializationError(msg string) *CadenceSerializationError {
	return &CadenceSerializationError{msg: msg}
}

func (e *CadenceSerializationError) Error() string {
	return fmt.Sprintf("cadence serialization error: %v", e.msg)
}

// NewCadenceDeserializationError returns a CadenceDeserializationError
func NewCadenceDeserializationError(msg string) *CadenceDeserializationError {
	return &CadenceDeserializationError{msg: msg}
}

func (e *CadenceDeserializationError) Error() string {
	return fmt.Sprintf("cadence deserialization error: %v", e.msg)
}

// workflowTimerTaskInfoToThrift converts persistence WorkflowTimerTaskInfo slice to thrift wrapper type
func workflowTimerTaskInfoToThrift(tasks []*WorkflowTimerTaskInfo) *sqlblobs.WorkflowTimerTaskInfo {
	refs := make([]*sqlblobs.TimerReference, 0, len(tasks))
	for _, task := range tasks {
		if task == nil {
			continue
		}
		refs = append(refs, &sqlblobs.TimerReference{
			TaskID:              common.Int64Ptr(task.TaskID),
			VisibilityTimestamp: common.Int64Ptr(task.VisibilityTimestamp.UnixNano()),
			TimeoutType:         common.Int16Ptr(int16(task.TimeoutType)),
		})
	}
	return &sqlblobs.WorkflowTimerTaskInfo{
		References: refs,
	}
}

// workflowTimerTaskInfoFromThrift converts thrift wrapper type to persistence WorkflowTimerTaskInfo slice
func workflowTimerTaskInfoFromThrift(info *sqlblobs.WorkflowTimerTaskInfo) []*WorkflowTimerTaskInfo {
	if info == nil || info.References == nil {
		return nil
	}
	tasks := make([]*WorkflowTimerTaskInfo, 0, len(info.References))
	for _, ref := range info.References {
		if ref == nil {
			continue
		}
		tasks = append(tasks, &WorkflowTimerTaskInfo{
			TaskID:              ref.GetTaskID(),
			VisibilityTimestamp: time.Unix(0, ref.GetVisibilityTimestamp()),
			TimeoutType:         int(ref.GetTimeoutType()),
		})
	}
	return tasks
}
