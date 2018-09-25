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
	"sync/atomic"

	workflow "github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/codec"
)

type (
	// HistorySerializer is used by persistence to serialize/deserialize history event(s)
	// It will only be used inside persistence, so that serialize/deserialize is transparent for application
	HistorySerializer interface {
		// serialize/deserialize history events
		SerializeBatchEvents(batch *workflow.History, encodingType common.EncodingType) (*DataBlob, error)
		DeserializeBatchEvents(data *DataBlob) (*workflow.History, error)

		// serialize/deserialize a single history event
		SerializeEvent(event *workflow.HistoryEvent, encodingType common.EncodingType) (*DataBlob, error)
		DeserializeEvent(data *DataBlob) (*workflow.HistoryEvent, error)
	}

	// InconsistentDataHeaderError is an error that is returned when HistorySerializer returns inconsistent header
	InconsistentDataHeaderError struct {
		header1 map[string]string
		header2 map[string]string
	}

	// HistorySerializationError is an error type that's
	// returned on a history serialization failure
	HistorySerializationError struct {
		msg string
	}

	// HistoryDeserializationError is an error type that's
	// returned on a history deserialization failure
	HistoryDeserializationError struct {
		msg string
	}

	// UnknownEncodingTypeError is an error type that's
	// returned when the encoding type provided as input
	// is unknown or unsupported
	UnknownEncodingTypeError struct {
		encodingType common.EncodingType
	}

	// HistoryVersionCompatibilityError is an error type
	// that's returned when history serialization or
	// deserialization cannot proceed due to version
	// incompatibility
	HistoryVersionCompatibilityError struct {
		requiredVersion  int
		supportedVersion int
	}

	serializerImpl struct {
		thriftrwEncoder codec.BinaryEncoder
	}
)

const (
	// DefaultEncodingType is the default encoding/decoding format for persisted history
	DefaultEncodingType = common.EncodingTypeJSON
)

// version checking will be used when we introduce backward incompatible change
var defaultHistoryVersion = int32(1)
var maxSupportedHistoryVersion = int32(1)

// NewHistorySerializer returns a HistorySerializer
func NewHistorySerializer() HistorySerializer {
	return &serializerImpl{
		thriftrwEncoder: codec.NewThriftRWEncoder(),
	}
}

func (t *serializerImpl) SerializeBatchEvents(batch *workflow.History, encodingType common.EncodingType) (*DataBlob, error) {
	switch encodingType {
	case common.EncodingTypeThriftRW:
		history := &workflow.History{
			Events: batch.Events,
		}
		data, err := t.thriftrwEncoder.Encode(history)
		if err != nil {
			return nil, &HistorySerializationError{msg: err.Error()}
		}
		return NewDataBlob(data, encodingType, defaultHistoryVersion), nil
	default:
		fallthrough
	case common.EncodingTypeJSON:
		data, err := json.Marshal(batch.Events)
		if err != nil {
			return nil, &HistorySerializationError{msg: err.Error()}
		}
		return NewDataBlob(data, encodingType, defaultHistoryVersion), nil
	}
}

func (t *serializerImpl) DeserializeBatchEvents(data *DataBlob) (*workflow.History, error) {
	if data.GetVersion() > GetMaxSupportedHistoryVersion() {
		err := NewHistoryVersionCompatibilityError(data.GetVersion(), GetMaxSupportedHistoryVersion())
		return nil, &HistoryDeserializationError{msg: err.Error()}
	}
	switch data.GetEncoding() {
	case common.EncodingTypeJSON:
		var events []*workflow.HistoryEvent
		err := json.Unmarshal(data.Data, &events)
		if err != nil {
			return nil, &HistoryDeserializationError{msg: err.Error()}
		}
		return &workflow.History{Events: events}, nil
	case common.EncodingTypeThriftRW:
		var history workflow.History
		err := t.thriftrwEncoder.Decode(data.Data, &history)
		if err != nil {
			return nil, &HistoryDeserializationError{msg: err.Error()}
		}
		return &history, nil
	default:
		return nil, NewUnknownEncodingTypeError(data.GetEncoding())
	}
}

func (t *serializerImpl) SerializeEvent(event *workflow.HistoryEvent, encodingType common.EncodingType) (*DataBlob, error) {
	switch encodingType {
	case common.EncodingTypeThriftRW:
		data, err := t.thriftrwEncoder.Encode(event)
		if err != nil {
			return nil, &HistorySerializationError{msg: err.Error()}
		}
		return NewDataBlob(data, encodingType, defaultHistoryVersion), nil
	default:
		fallthrough
	case common.EncodingTypeJSON:
		data, err := json.Marshal(event)
		if err != nil {
			return nil, &HistorySerializationError{msg: err.Error()}
		}
		return NewDataBlob(data, encodingType, defaultHistoryVersion), nil
	}
}

func (t *serializerImpl) DeserializeEvent(data *DataBlob) (*workflow.HistoryEvent, error) {
	if data.GetVersion() > GetMaxSupportedHistoryVersion() {
		err := NewHistoryVersionCompatibilityError(data.GetVersion(), GetMaxSupportedHistoryVersion())
		return nil, &HistoryDeserializationError{msg: err.Error()}
	}
	var event workflow.HistoryEvent
	switch data.GetEncoding() {
	case common.EncodingTypeJSON:
		err := json.Unmarshal(data.Data, &event)
		if err != nil {
			return nil, &HistoryDeserializationError{msg: err.Error()}
		}
		return &event, nil
	case common.EncodingTypeThriftRW:
		err := t.thriftrwEncoder.Decode(data.Data, &event)
		if err != nil {
			return nil, &HistoryDeserializationError{msg: err.Error()}
		}
		return &event, nil
	default:
		return nil, NewUnknownEncodingTypeError(data.GetEncoding())
	}
}

// NewUnknownEncodingTypeError returns a new instance of encoding type error
func NewUnknownEncodingTypeError(encodingType common.EncodingType) error {
	return &UnknownEncodingTypeError{encodingType: encodingType}
}

func (e *UnknownEncodingTypeError) Error() string {
	return fmt.Sprintf("unknown or unsupported encoding type %v", e.encodingType)
}

// NewHistoryVersionCompatibilityError returns a new instance of compatibility error type
func NewHistoryVersionCompatibilityError(required int, supported int) error {
	return &HistoryVersionCompatibilityError{
		requiredVersion:  required,
		supportedVersion: supported,
	}
}

func (e *HistoryVersionCompatibilityError) Error() string {
	return fmt.Sprintf("incompatible history version;required=%v;maxSupported=%v",
		e.requiredVersion, e.supportedVersion)
}

// NewInconsistentDataHeaderError returns a new InconsistentDataHeaderError
func NewInconsistentDataHeaderError(header1 map[string]string, header2 map[string]string) *InconsistentDataHeaderError {
	return &InconsistentDataHeaderError{
		header1: header1,
		header2: header2,
	}
}
func (e *HistorySerializationError) Error() string {
	return fmt.Sprintf("history serialization error: %v", e.msg)
}

func (e *InconsistentDataHeaderError) Error() string {
	return fmt.Sprintf("history serialization error. Header1: %v; Header2: %v", e.header1, e.header2)
}

func (e *HistoryDeserializationError) Error() string {
	return fmt.Sprintf("history deserialization error: %v", e.msg)
}

// SetMaxSupportedHistoryVersion resets the max supported history version
// this method is only intended for integration test
func SetMaxSupportedHistoryVersion(version int) {
	atomic.StoreInt32(&maxSupportedHistoryVersion, int32(version))
}

// GetMaxSupportedHistoryVersion returns the max supported version
func GetMaxSupportedHistoryVersion() int {
	return int(atomic.LoadInt32(&maxSupportedHistoryVersion))
}

// SetDefaultHistoryVersion resets the default history version
// only intended for integration test
func SetDefaultHistoryVersion(version int) {
	atomic.StoreInt32(&defaultHistoryVersion, int32(version))
}

// GetDefaultHistoryVersion returns the default history version
func GetDefaultHistoryVersion() int {
	return int(atomic.LoadInt32(&defaultHistoryVersion))
}
