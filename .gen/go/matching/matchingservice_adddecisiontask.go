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

// Code generated by thriftrw v1.10.0. DO NOT EDIT.
// @generated

package matching

import (
	"errors"
	"fmt"
	"github.com/uber/cadence/.gen/go/shared"
	"go.uber.org/thriftrw/wire"
	"strings"
)

// MatchingService_AddDecisionTask_Args represents the arguments for the MatchingService.AddDecisionTask function.
//
// The arguments for AddDecisionTask are sent and received over the wire as this struct.
type MatchingService_AddDecisionTask_Args struct {
	AddRequest *AddDecisionTaskRequest `json:"addRequest,omitempty"`
}

// ToWire translates a MatchingService_AddDecisionTask_Args struct into a Thrift-level intermediate
// representation. This intermediate representation may be serialized
// into bytes using a ThriftRW protocol implementation.
//
// An error is returned if the struct or any of its fields failed to
// validate.
//
//   x, err := v.ToWire()
//   if err != nil {
//     return err
//   }
//
//   if err := binaryProtocol.Encode(x, writer); err != nil {
//     return err
//   }
func (v *MatchingService_AddDecisionTask_Args) ToWire() (wire.Value, error) {
	var (
		fields [1]wire.Field
		i      int = 0
		w      wire.Value
		err    error
	)

	if v.AddRequest != nil {
		w, err = v.AddRequest.ToWire()
		if err != nil {
			return w, err
		}
		fields[i] = wire.Field{ID: 1, Value: w}
		i++
	}

	return wire.NewValueStruct(wire.Struct{Fields: fields[:i]}), nil
}

func _AddDecisionTaskRequest_Read(w wire.Value) (*AddDecisionTaskRequest, error) {
	var v AddDecisionTaskRequest
	err := v.FromWire(w)
	return &v, err
}

// FromWire deserializes a MatchingService_AddDecisionTask_Args struct from its Thrift-level
// representation. The Thrift-level representation may be obtained
// from a ThriftRW protocol implementation.
//
// An error is returned if we were unable to build a MatchingService_AddDecisionTask_Args struct
// from the provided intermediate representation.
//
//   x, err := binaryProtocol.Decode(reader, wire.TStruct)
//   if err != nil {
//     return nil, err
//   }
//
//   var v MatchingService_AddDecisionTask_Args
//   if err := v.FromWire(x); err != nil {
//     return nil, err
//   }
//   return &v, nil
func (v *MatchingService_AddDecisionTask_Args) FromWire(w wire.Value) error {
	var err error

	for _, field := range w.GetStruct().Fields {
		switch field.ID {
		case 1:
			if field.Value.Type() == wire.TStruct {
				v.AddRequest, err = _AddDecisionTaskRequest_Read(field.Value)
				if err != nil {
					return err
				}

			}
		}
	}

	return nil
}

// String returns a readable string representation of a MatchingService_AddDecisionTask_Args
// struct.
func (v *MatchingService_AddDecisionTask_Args) String() string {
	if v == nil {
		return "<nil>"
	}

	var fields [1]string
	i := 0
	if v.AddRequest != nil {
		fields[i] = fmt.Sprintf("AddRequest: %v", v.AddRequest)
		i++
	}

	return fmt.Sprintf("MatchingService_AddDecisionTask_Args{%v}", strings.Join(fields[:i], ", "))
}

// Equals returns true if all the fields of this MatchingService_AddDecisionTask_Args match the
// provided MatchingService_AddDecisionTask_Args.
//
// This function performs a deep comparison.
func (v *MatchingService_AddDecisionTask_Args) Equals(rhs *MatchingService_AddDecisionTask_Args) bool {
	if !((v.AddRequest == nil && rhs.AddRequest == nil) || (v.AddRequest != nil && rhs.AddRequest != nil && v.AddRequest.Equals(rhs.AddRequest))) {
		return false
	}

	return true
}

// MethodName returns the name of the Thrift function as specified in
// the IDL, for which this struct represent the arguments.
//
// This will always be "AddDecisionTask" for this struct.
func (v *MatchingService_AddDecisionTask_Args) MethodName() string {
	return "AddDecisionTask"
}

// EnvelopeType returns the kind of value inside this struct.
//
// This will always be Call for this struct.
func (v *MatchingService_AddDecisionTask_Args) EnvelopeType() wire.EnvelopeType {
	return wire.Call
}

// MatchingService_AddDecisionTask_Helper provides functions that aid in handling the
// parameters and return values of the MatchingService.AddDecisionTask
// function.
var MatchingService_AddDecisionTask_Helper = struct {
	// Args accepts the parameters of AddDecisionTask in-order and returns
	// the arguments struct for the function.
	Args func(
		addRequest *AddDecisionTaskRequest,
	) *MatchingService_AddDecisionTask_Args

	// IsException returns true if the given error can be thrown
	// by AddDecisionTask.
	//
	// An error can be thrown by AddDecisionTask only if the
	// corresponding exception type was mentioned in the 'throws'
	// section for it in the Thrift file.
	IsException func(error) bool

	// WrapResponse returns the result struct for AddDecisionTask
	// given the error returned by it. The provided error may
	// be nil if AddDecisionTask did not fail.
	//
	// This allows mapping errors returned by AddDecisionTask into a
	// serializable result struct. WrapResponse returns a
	// non-nil error if the provided error cannot be thrown by
	// AddDecisionTask
	//
	//   err := AddDecisionTask(args)
	//   result, err := MatchingService_AddDecisionTask_Helper.WrapResponse(err)
	//   if err != nil {
	//     return fmt.Errorf("unexpected error from AddDecisionTask: %v", err)
	//   }
	//   serialize(result)
	WrapResponse func(error) (*MatchingService_AddDecisionTask_Result, error)

	// UnwrapResponse takes the result struct for AddDecisionTask
	// and returns the erorr returned by it (if any).
	//
	// The error is non-nil only if AddDecisionTask threw an
	// exception.
	//
	//   result := deserialize(bytes)
	//   err := MatchingService_AddDecisionTask_Helper.UnwrapResponse(result)
	UnwrapResponse func(*MatchingService_AddDecisionTask_Result) error
}{}

func init() {
	MatchingService_AddDecisionTask_Helper.Args = func(
		addRequest *AddDecisionTaskRequest,
	) *MatchingService_AddDecisionTask_Args {
		return &MatchingService_AddDecisionTask_Args{
			AddRequest: addRequest,
		}
	}

	MatchingService_AddDecisionTask_Helper.IsException = func(err error) bool {
		switch err.(type) {
		case *shared.BadRequestError:
			return true
		case *shared.InternalServiceError:
			return true
		case *shared.ServiceBusyError:
			return true
		default:
			return false
		}
	}

	MatchingService_AddDecisionTask_Helper.WrapResponse = func(err error) (*MatchingService_AddDecisionTask_Result, error) {
		if err == nil {
			return &MatchingService_AddDecisionTask_Result{}, nil
		}

		switch e := err.(type) {
		case *shared.BadRequestError:
			if e == nil {
				return nil, errors.New("WrapResponse received non-nil error type with nil value for MatchingService_AddDecisionTask_Result.BadRequestError")
			}
			return &MatchingService_AddDecisionTask_Result{BadRequestError: e}, nil
		case *shared.InternalServiceError:
			if e == nil {
				return nil, errors.New("WrapResponse received non-nil error type with nil value for MatchingService_AddDecisionTask_Result.InternalServiceError")
			}
			return &MatchingService_AddDecisionTask_Result{InternalServiceError: e}, nil
		case *shared.ServiceBusyError:
			if e == nil {
				return nil, errors.New("WrapResponse received non-nil error type with nil value for MatchingService_AddDecisionTask_Result.ServiceBusyError")
			}
			return &MatchingService_AddDecisionTask_Result{ServiceBusyError: e}, nil
		}

		return nil, err
	}
	MatchingService_AddDecisionTask_Helper.UnwrapResponse = func(result *MatchingService_AddDecisionTask_Result) (err error) {
		if result.BadRequestError != nil {
			err = result.BadRequestError
			return
		}
		if result.InternalServiceError != nil {
			err = result.InternalServiceError
			return
		}
		if result.ServiceBusyError != nil {
			err = result.ServiceBusyError
			return
		}
		return
	}

}

// MatchingService_AddDecisionTask_Result represents the result of a MatchingService.AddDecisionTask function call.
//
// The result of a AddDecisionTask execution is sent and received over the wire as this struct.
type MatchingService_AddDecisionTask_Result struct {
	BadRequestError      *shared.BadRequestError      `json:"badRequestError,omitempty"`
	InternalServiceError *shared.InternalServiceError `json:"internalServiceError,omitempty"`
	ServiceBusyError     *shared.ServiceBusyError     `json:"serviceBusyError,omitempty"`
}

// ToWire translates a MatchingService_AddDecisionTask_Result struct into a Thrift-level intermediate
// representation. This intermediate representation may be serialized
// into bytes using a ThriftRW protocol implementation.
//
// An error is returned if the struct or any of its fields failed to
// validate.
//
//   x, err := v.ToWire()
//   if err != nil {
//     return err
//   }
//
//   if err := binaryProtocol.Encode(x, writer); err != nil {
//     return err
//   }
func (v *MatchingService_AddDecisionTask_Result) ToWire() (wire.Value, error) {
	var (
		fields [3]wire.Field
		i      int = 0
		w      wire.Value
		err    error
	)

	if v.BadRequestError != nil {
		w, err = v.BadRequestError.ToWire()
		if err != nil {
			return w, err
		}
		fields[i] = wire.Field{ID: 1, Value: w}
		i++
	}
	if v.InternalServiceError != nil {
		w, err = v.InternalServiceError.ToWire()
		if err != nil {
			return w, err
		}
		fields[i] = wire.Field{ID: 2, Value: w}
		i++
	}
	if v.ServiceBusyError != nil {
		w, err = v.ServiceBusyError.ToWire()
		if err != nil {
			return w, err
		}
		fields[i] = wire.Field{ID: 3, Value: w}
		i++
	}

	if i > 1 {
		return wire.Value{}, fmt.Errorf("MatchingService_AddDecisionTask_Result should have at most one field: got %v fields", i)
	}

	return wire.NewValueStruct(wire.Struct{Fields: fields[:i]}), nil
}

// FromWire deserializes a MatchingService_AddDecisionTask_Result struct from its Thrift-level
// representation. The Thrift-level representation may be obtained
// from a ThriftRW protocol implementation.
//
// An error is returned if we were unable to build a MatchingService_AddDecisionTask_Result struct
// from the provided intermediate representation.
//
//   x, err := binaryProtocol.Decode(reader, wire.TStruct)
//   if err != nil {
//     return nil, err
//   }
//
//   var v MatchingService_AddDecisionTask_Result
//   if err := v.FromWire(x); err != nil {
//     return nil, err
//   }
//   return &v, nil
func (v *MatchingService_AddDecisionTask_Result) FromWire(w wire.Value) error {
	var err error

	for _, field := range w.GetStruct().Fields {
		switch field.ID {
		case 1:
			if field.Value.Type() == wire.TStruct {
				v.BadRequestError, err = _BadRequestError_Read(field.Value)
				if err != nil {
					return err
				}

			}
		case 2:
			if field.Value.Type() == wire.TStruct {
				v.InternalServiceError, err = _InternalServiceError_Read(field.Value)
				if err != nil {
					return err
				}

			}
		case 3:
			if field.Value.Type() == wire.TStruct {
				v.ServiceBusyError, err = _ServiceBusyError_Read(field.Value)
				if err != nil {
					return err
				}

			}
		}
	}

	count := 0
	if v.BadRequestError != nil {
		count++
	}
	if v.InternalServiceError != nil {
		count++
	}
	if v.ServiceBusyError != nil {
		count++
	}
	if count > 1 {
		return fmt.Errorf("MatchingService_AddDecisionTask_Result should have at most one field: got %v fields", count)
	}

	return nil
}

// String returns a readable string representation of a MatchingService_AddDecisionTask_Result
// struct.
func (v *MatchingService_AddDecisionTask_Result) String() string {
	if v == nil {
		return "<nil>"
	}

	var fields [3]string
	i := 0
	if v.BadRequestError != nil {
		fields[i] = fmt.Sprintf("BadRequestError: %v", v.BadRequestError)
		i++
	}
	if v.InternalServiceError != nil {
		fields[i] = fmt.Sprintf("InternalServiceError: %v", v.InternalServiceError)
		i++
	}
	if v.ServiceBusyError != nil {
		fields[i] = fmt.Sprintf("ServiceBusyError: %v", v.ServiceBusyError)
		i++
	}

	return fmt.Sprintf("MatchingService_AddDecisionTask_Result{%v}", strings.Join(fields[:i], ", "))
}

// Equals returns true if all the fields of this MatchingService_AddDecisionTask_Result match the
// provided MatchingService_AddDecisionTask_Result.
//
// This function performs a deep comparison.
func (v *MatchingService_AddDecisionTask_Result) Equals(rhs *MatchingService_AddDecisionTask_Result) bool {
	if !((v.BadRequestError == nil && rhs.BadRequestError == nil) || (v.BadRequestError != nil && rhs.BadRequestError != nil && v.BadRequestError.Equals(rhs.BadRequestError))) {
		return false
	}
	if !((v.InternalServiceError == nil && rhs.InternalServiceError == nil) || (v.InternalServiceError != nil && rhs.InternalServiceError != nil && v.InternalServiceError.Equals(rhs.InternalServiceError))) {
		return false
	}
	if !((v.ServiceBusyError == nil && rhs.ServiceBusyError == nil) || (v.ServiceBusyError != nil && rhs.ServiceBusyError != nil && v.ServiceBusyError.Equals(rhs.ServiceBusyError))) {
		return false
	}

	return true
}

// MethodName returns the name of the Thrift function as specified in
// the IDL, for which this struct represent the result.
//
// This will always be "AddDecisionTask" for this struct.
func (v *MatchingService_AddDecisionTask_Result) MethodName() string {
	return "AddDecisionTask"
}

// EnvelopeType returns the kind of value inside this struct.
//
// This will always be Reply for this struct.
func (v *MatchingService_AddDecisionTask_Result) EnvelopeType() wire.EnvelopeType {
	return wire.Reply
}
