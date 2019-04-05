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

// Code generated by thriftrw v1.18.0. DO NOT EDIT.
// @generated

package admin

import (
	errors "errors"
	fmt "fmt"
	shared "github.com/uber/cadence/.gen/go/shared"
	multierr "go.uber.org/multierr"
	wire "go.uber.org/thriftrw/wire"
	zapcore "go.uber.org/zap/zapcore"
	strings "strings"
)

// AdminService_DescribeWorkflowExecution_Args represents the arguments for the AdminService.DescribeWorkflowExecution function.
//
// The arguments for DescribeWorkflowExecution are sent and received over the wire as this struct.
type AdminService_DescribeWorkflowExecution_Args struct {
	Request *DescribeWorkflowExecutionRequest `json:"request,omitempty"`
}

// ToWire translates a AdminService_DescribeWorkflowExecution_Args struct into a Thrift-level intermediate
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
func (v *AdminService_DescribeWorkflowExecution_Args) ToWire() (wire.Value, error) {
	var (
		fields [1]wire.Field
		i      int = 0
		w      wire.Value
		err    error
	)

	if v.Request != nil {
		w, err = v.Request.ToWire()
		if err != nil {
			return w, err
		}
		fields[i] = wire.Field{ID: 1, Value: w}
		i++
	}

	return wire.NewValueStruct(wire.Struct{Fields: fields[:i]}), nil
}

func _DescribeWorkflowExecutionRequest_Read(w wire.Value) (*DescribeWorkflowExecutionRequest, error) {
	var v DescribeWorkflowExecutionRequest
	err := v.FromWire(w)
	return &v, err
}

// FromWire deserializes a AdminService_DescribeWorkflowExecution_Args struct from its Thrift-level
// representation. The Thrift-level representation may be obtained
// from a ThriftRW protocol implementation.
//
// An error is returned if we were unable to build a AdminService_DescribeWorkflowExecution_Args struct
// from the provided intermediate representation.
//
//   x, err := binaryProtocol.Decode(reader, wire.TStruct)
//   if err != nil {
//     return nil, err
//   }
//
//   var v AdminService_DescribeWorkflowExecution_Args
//   if err := v.FromWire(x); err != nil {
//     return nil, err
//   }
//   return &v, nil
func (v *AdminService_DescribeWorkflowExecution_Args) FromWire(w wire.Value) error {
	var err error

	for _, field := range w.GetStruct().Fields {
		switch field.ID {
		case 1:
			if field.Value.Type() == wire.TStruct {
				v.Request, err = _DescribeWorkflowExecutionRequest_Read(field.Value)
				if err != nil {
					return err
				}

			}
		}
	}

	return nil
}

// String returns a readable string representation of a AdminService_DescribeWorkflowExecution_Args
// struct.
func (v *AdminService_DescribeWorkflowExecution_Args) String() string {
	if v == nil {
		return "<nil>"
	}

	var fields [1]string
	i := 0
	if v.Request != nil {
		fields[i] = fmt.Sprintf("Request: %v", v.Request)
		i++
	}

	return fmt.Sprintf("AdminService_DescribeWorkflowExecution_Args{%v}", strings.Join(fields[:i], ", "))
}

// Equals returns true if all the fields of this AdminService_DescribeWorkflowExecution_Args match the
// provided AdminService_DescribeWorkflowExecution_Args.
//
// This function performs a deep comparison.
func (v *AdminService_DescribeWorkflowExecution_Args) Equals(rhs *AdminService_DescribeWorkflowExecution_Args) bool {
	if v == nil {
		return rhs == nil
	} else if rhs == nil {
		return false
	}
	if !((v.Request == nil && rhs.Request == nil) || (v.Request != nil && rhs.Request != nil && v.Request.Equals(rhs.Request))) {
		return false
	}

	return true
}

// MarshalLogObject implements zapcore.ObjectMarshaler, enabling
// fast logging of AdminService_DescribeWorkflowExecution_Args.
func (v *AdminService_DescribeWorkflowExecution_Args) MarshalLogObject(enc zapcore.ObjectEncoder) (err error) {
	if v == nil {
		return nil
	}
	if v.Request != nil {
		err = multierr.Append(err, enc.AddObject("request", v.Request))
	}
	return err
}

// GetRequest returns the value of Request if it is set or its
// zero value if it is unset.
func (v *AdminService_DescribeWorkflowExecution_Args) GetRequest() (o *DescribeWorkflowExecutionRequest) {
	if v != nil && v.Request != nil {
		return v.Request
	}

	return
}

// IsSetRequest returns true if Request is not nil.
func (v *AdminService_DescribeWorkflowExecution_Args) IsSetRequest() bool {
	return v != nil && v.Request != nil
}

// MethodName returns the name of the Thrift function as specified in
// the IDL, for which this struct represent the arguments.
//
// This will always be "DescribeWorkflowExecution" for this struct.
func (v *AdminService_DescribeWorkflowExecution_Args) MethodName() string {
	return "DescribeWorkflowExecution"
}

// EnvelopeType returns the kind of value inside this struct.
//
// This will always be Call for this struct.
func (v *AdminService_DescribeWorkflowExecution_Args) EnvelopeType() wire.EnvelopeType {
	return wire.Call
}

// AdminService_DescribeWorkflowExecution_Helper provides functions that aid in handling the
// parameters and return values of the AdminService.DescribeWorkflowExecution
// function.
var AdminService_DescribeWorkflowExecution_Helper = struct {
	// Args accepts the parameters of DescribeWorkflowExecution in-order and returns
	// the arguments struct for the function.
	Args func(
		request *DescribeWorkflowExecutionRequest,
	) *AdminService_DescribeWorkflowExecution_Args

	// IsException returns true if the given error can be thrown
	// by DescribeWorkflowExecution.
	//
	// An error can be thrown by DescribeWorkflowExecution only if the
	// corresponding exception type was mentioned in the 'throws'
	// section for it in the Thrift file.
	IsException func(error) bool

	// WrapResponse returns the result struct for DescribeWorkflowExecution
	// given its return value and error.
	//
	// This allows mapping values and errors returned by
	// DescribeWorkflowExecution into a serializable result struct.
	// WrapResponse returns a non-nil error if the provided
	// error cannot be thrown by DescribeWorkflowExecution
	//
	//   value, err := DescribeWorkflowExecution(args)
	//   result, err := AdminService_DescribeWorkflowExecution_Helper.WrapResponse(value, err)
	//   if err != nil {
	//     return fmt.Errorf("unexpected error from DescribeWorkflowExecution: %v", err)
	//   }
	//   serialize(result)
	WrapResponse func(*DescribeWorkflowExecutionResponse, error) (*AdminService_DescribeWorkflowExecution_Result, error)

	// UnwrapResponse takes the result struct for DescribeWorkflowExecution
	// and returns the value or error returned by it.
	//
	// The error is non-nil only if DescribeWorkflowExecution threw an
	// exception.
	//
	//   result := deserialize(bytes)
	//   value, err := AdminService_DescribeWorkflowExecution_Helper.UnwrapResponse(result)
	UnwrapResponse func(*AdminService_DescribeWorkflowExecution_Result) (*DescribeWorkflowExecutionResponse, error)
}{}

func init() {
	AdminService_DescribeWorkflowExecution_Helper.Args = func(
		request *DescribeWorkflowExecutionRequest,
	) *AdminService_DescribeWorkflowExecution_Args {
		return &AdminService_DescribeWorkflowExecution_Args{
			Request: request,
		}
	}

	AdminService_DescribeWorkflowExecution_Helper.IsException = func(err error) bool {
		switch err.(type) {
		case *shared.BadRequestError:
			return true
		case *shared.InternalServiceError:
			return true
		case *shared.EntityNotExistsError:
			return true
		case *shared.AccessDeniedError:
			return true
		default:
			return false
		}
	}

	AdminService_DescribeWorkflowExecution_Helper.WrapResponse = func(success *DescribeWorkflowExecutionResponse, err error) (*AdminService_DescribeWorkflowExecution_Result, error) {
		if err == nil {
			return &AdminService_DescribeWorkflowExecution_Result{Success: success}, nil
		}

		switch e := err.(type) {
		case *shared.BadRequestError:
			if e == nil {
				return nil, errors.New("WrapResponse received non-nil error type with nil value for AdminService_DescribeWorkflowExecution_Result.BadRequestError")
			}
			return &AdminService_DescribeWorkflowExecution_Result{BadRequestError: e}, nil
		case *shared.InternalServiceError:
			if e == nil {
				return nil, errors.New("WrapResponse received non-nil error type with nil value for AdminService_DescribeWorkflowExecution_Result.InternalServiceError")
			}
			return &AdminService_DescribeWorkflowExecution_Result{InternalServiceError: e}, nil
		case *shared.EntityNotExistsError:
			if e == nil {
				return nil, errors.New("WrapResponse received non-nil error type with nil value for AdminService_DescribeWorkflowExecution_Result.EntityNotExistError")
			}
			return &AdminService_DescribeWorkflowExecution_Result{EntityNotExistError: e}, nil
		case *shared.AccessDeniedError:
			if e == nil {
				return nil, errors.New("WrapResponse received non-nil error type with nil value for AdminService_DescribeWorkflowExecution_Result.AccessDeniedError")
			}
			return &AdminService_DescribeWorkflowExecution_Result{AccessDeniedError: e}, nil
		}

		return nil, err
	}
	AdminService_DescribeWorkflowExecution_Helper.UnwrapResponse = func(result *AdminService_DescribeWorkflowExecution_Result) (success *DescribeWorkflowExecutionResponse, err error) {
		if result.BadRequestError != nil {
			err = result.BadRequestError
			return
		}
		if result.InternalServiceError != nil {
			err = result.InternalServiceError
			return
		}
		if result.EntityNotExistError != nil {
			err = result.EntityNotExistError
			return
		}
		if result.AccessDeniedError != nil {
			err = result.AccessDeniedError
			return
		}

		if result.Success != nil {
			success = result.Success
			return
		}

		err = errors.New("expected a non-void result")
		return
	}

}

// AdminService_DescribeWorkflowExecution_Result represents the result of a AdminService.DescribeWorkflowExecution function call.
//
// The result of a DescribeWorkflowExecution execution is sent and received over the wire as this struct.
//
// Success is set only if the function did not throw an exception.
type AdminService_DescribeWorkflowExecution_Result struct {
	// Value returned by DescribeWorkflowExecution after a successful execution.
	Success              *DescribeWorkflowExecutionResponse `json:"success,omitempty"`
	BadRequestError      *shared.BadRequestError            `json:"badRequestError,omitempty"`
	InternalServiceError *shared.InternalServiceError       `json:"internalServiceError,omitempty"`
	EntityNotExistError  *shared.EntityNotExistsError       `json:"entityNotExistError,omitempty"`
	AccessDeniedError    *shared.AccessDeniedError          `json:"accessDeniedError,omitempty"`
}

// ToWire translates a AdminService_DescribeWorkflowExecution_Result struct into a Thrift-level intermediate
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
func (v *AdminService_DescribeWorkflowExecution_Result) ToWire() (wire.Value, error) {
	var (
		fields [5]wire.Field
		i      int = 0
		w      wire.Value
		err    error
	)

	if v.Success != nil {
		w, err = v.Success.ToWire()
		if err != nil {
			return w, err
		}
		fields[i] = wire.Field{ID: 0, Value: w}
		i++
	}
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
	if v.EntityNotExistError != nil {
		w, err = v.EntityNotExistError.ToWire()
		if err != nil {
			return w, err
		}
		fields[i] = wire.Field{ID: 3, Value: w}
		i++
	}
	if v.AccessDeniedError != nil {
		w, err = v.AccessDeniedError.ToWire()
		if err != nil {
			return w, err
		}
		fields[i] = wire.Field{ID: 4, Value: w}
		i++
	}

	if i != 1 {
		return wire.Value{}, fmt.Errorf("AdminService_DescribeWorkflowExecution_Result should have exactly one field: got %v fields", i)
	}

	return wire.NewValueStruct(wire.Struct{Fields: fields[:i]}), nil
}

func _DescribeWorkflowExecutionResponse_Read(w wire.Value) (*DescribeWorkflowExecutionResponse, error) {
	var v DescribeWorkflowExecutionResponse
	err := v.FromWire(w)
	return &v, err
}

func _EntityNotExistsError_Read(w wire.Value) (*shared.EntityNotExistsError, error) {
	var v shared.EntityNotExistsError
	err := v.FromWire(w)
	return &v, err
}

// FromWire deserializes a AdminService_DescribeWorkflowExecution_Result struct from its Thrift-level
// representation. The Thrift-level representation may be obtained
// from a ThriftRW protocol implementation.
//
// An error is returned if we were unable to build a AdminService_DescribeWorkflowExecution_Result struct
// from the provided intermediate representation.
//
//   x, err := binaryProtocol.Decode(reader, wire.TStruct)
//   if err != nil {
//     return nil, err
//   }
//
//   var v AdminService_DescribeWorkflowExecution_Result
//   if err := v.FromWire(x); err != nil {
//     return nil, err
//   }
//   return &v, nil
func (v *AdminService_DescribeWorkflowExecution_Result) FromWire(w wire.Value) error {
	var err error

	for _, field := range w.GetStruct().Fields {
		switch field.ID {
		case 0:
			if field.Value.Type() == wire.TStruct {
				v.Success, err = _DescribeWorkflowExecutionResponse_Read(field.Value)
				if err != nil {
					return err
				}

			}
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
				v.EntityNotExistError, err = _EntityNotExistsError_Read(field.Value)
				if err != nil {
					return err
				}

			}
		case 4:
			if field.Value.Type() == wire.TStruct {
				v.AccessDeniedError, err = _AccessDeniedError_Read(field.Value)
				if err != nil {
					return err
				}

			}
		}
	}

	count := 0
	if v.Success != nil {
		count++
	}
	if v.BadRequestError != nil {
		count++
	}
	if v.InternalServiceError != nil {
		count++
	}
	if v.EntityNotExistError != nil {
		count++
	}
	if v.AccessDeniedError != nil {
		count++
	}
	if count != 1 {
		return fmt.Errorf("AdminService_DescribeWorkflowExecution_Result should have exactly one field: got %v fields", count)
	}

	return nil
}

// String returns a readable string representation of a AdminService_DescribeWorkflowExecution_Result
// struct.
func (v *AdminService_DescribeWorkflowExecution_Result) String() string {
	if v == nil {
		return "<nil>"
	}

	var fields [5]string
	i := 0
	if v.Success != nil {
		fields[i] = fmt.Sprintf("Success: %v", v.Success)
		i++
	}
	if v.BadRequestError != nil {
		fields[i] = fmt.Sprintf("BadRequestError: %v", v.BadRequestError)
		i++
	}
	if v.InternalServiceError != nil {
		fields[i] = fmt.Sprintf("InternalServiceError: %v", v.InternalServiceError)
		i++
	}
	if v.EntityNotExistError != nil {
		fields[i] = fmt.Sprintf("EntityNotExistError: %v", v.EntityNotExistError)
		i++
	}
	if v.AccessDeniedError != nil {
		fields[i] = fmt.Sprintf("AccessDeniedError: %v", v.AccessDeniedError)
		i++
	}

	return fmt.Sprintf("AdminService_DescribeWorkflowExecution_Result{%v}", strings.Join(fields[:i], ", "))
}

// Equals returns true if all the fields of this AdminService_DescribeWorkflowExecution_Result match the
// provided AdminService_DescribeWorkflowExecution_Result.
//
// This function performs a deep comparison.
func (v *AdminService_DescribeWorkflowExecution_Result) Equals(rhs *AdminService_DescribeWorkflowExecution_Result) bool {
	if v == nil {
		return rhs == nil
	} else if rhs == nil {
		return false
	}
	if !((v.Success == nil && rhs.Success == nil) || (v.Success != nil && rhs.Success != nil && v.Success.Equals(rhs.Success))) {
		return false
	}
	if !((v.BadRequestError == nil && rhs.BadRequestError == nil) || (v.BadRequestError != nil && rhs.BadRequestError != nil && v.BadRequestError.Equals(rhs.BadRequestError))) {
		return false
	}
	if !((v.InternalServiceError == nil && rhs.InternalServiceError == nil) || (v.InternalServiceError != nil && rhs.InternalServiceError != nil && v.InternalServiceError.Equals(rhs.InternalServiceError))) {
		return false
	}
	if !((v.EntityNotExistError == nil && rhs.EntityNotExistError == nil) || (v.EntityNotExistError != nil && rhs.EntityNotExistError != nil && v.EntityNotExistError.Equals(rhs.EntityNotExistError))) {
		return false
	}
	if !((v.AccessDeniedError == nil && rhs.AccessDeniedError == nil) || (v.AccessDeniedError != nil && rhs.AccessDeniedError != nil && v.AccessDeniedError.Equals(rhs.AccessDeniedError))) {
		return false
	}

	return true
}

// MarshalLogObject implements zapcore.ObjectMarshaler, enabling
// fast logging of AdminService_DescribeWorkflowExecution_Result.
func (v *AdminService_DescribeWorkflowExecution_Result) MarshalLogObject(enc zapcore.ObjectEncoder) (err error) {
	if v == nil {
		return nil
	}
	if v.Success != nil {
		err = multierr.Append(err, enc.AddObject("success", v.Success))
	}
	if v.BadRequestError != nil {
		err = multierr.Append(err, enc.AddObject("badRequestError", v.BadRequestError))
	}
	if v.InternalServiceError != nil {
		err = multierr.Append(err, enc.AddObject("internalServiceError", v.InternalServiceError))
	}
	if v.EntityNotExistError != nil {
		err = multierr.Append(err, enc.AddObject("entityNotExistError", v.EntityNotExistError))
	}
	if v.AccessDeniedError != nil {
		err = multierr.Append(err, enc.AddObject("accessDeniedError", v.AccessDeniedError))
	}
	return err
}

// GetSuccess returns the value of Success if it is set or its
// zero value if it is unset.
func (v *AdminService_DescribeWorkflowExecution_Result) GetSuccess() (o *DescribeWorkflowExecutionResponse) {
	if v != nil && v.Success != nil {
		return v.Success
	}

	return
}

// IsSetSuccess returns true if Success is not nil.
func (v *AdminService_DescribeWorkflowExecution_Result) IsSetSuccess() bool {
	return v != nil && v.Success != nil
}

// GetBadRequestError returns the value of BadRequestError if it is set or its
// zero value if it is unset.
func (v *AdminService_DescribeWorkflowExecution_Result) GetBadRequestError() (o *shared.BadRequestError) {
	if v != nil && v.BadRequestError != nil {
		return v.BadRequestError
	}

	return
}

// IsSetBadRequestError returns true if BadRequestError is not nil.
func (v *AdminService_DescribeWorkflowExecution_Result) IsSetBadRequestError() bool {
	return v != nil && v.BadRequestError != nil
}

// GetInternalServiceError returns the value of InternalServiceError if it is set or its
// zero value if it is unset.
func (v *AdminService_DescribeWorkflowExecution_Result) GetInternalServiceError() (o *shared.InternalServiceError) {
	if v != nil && v.InternalServiceError != nil {
		return v.InternalServiceError
	}

	return
}

// IsSetInternalServiceError returns true if InternalServiceError is not nil.
func (v *AdminService_DescribeWorkflowExecution_Result) IsSetInternalServiceError() bool {
	return v != nil && v.InternalServiceError != nil
}

// GetEntityNotExistError returns the value of EntityNotExistError if it is set or its
// zero value if it is unset.
func (v *AdminService_DescribeWorkflowExecution_Result) GetEntityNotExistError() (o *shared.EntityNotExistsError) {
	if v != nil && v.EntityNotExistError != nil {
		return v.EntityNotExistError
	}

	return
}

// IsSetEntityNotExistError returns true if EntityNotExistError is not nil.
func (v *AdminService_DescribeWorkflowExecution_Result) IsSetEntityNotExistError() bool {
	return v != nil && v.EntityNotExistError != nil
}

// GetAccessDeniedError returns the value of AccessDeniedError if it is set or its
// zero value if it is unset.
func (v *AdminService_DescribeWorkflowExecution_Result) GetAccessDeniedError() (o *shared.AccessDeniedError) {
	if v != nil && v.AccessDeniedError != nil {
		return v.AccessDeniedError
	}

	return
}

// IsSetAccessDeniedError returns true if AccessDeniedError is not nil.
func (v *AdminService_DescribeWorkflowExecution_Result) IsSetAccessDeniedError() bool {
	return v != nil && v.AccessDeniedError != nil
}

// MethodName returns the name of the Thrift function as specified in
// the IDL, for which this struct represent the result.
//
// This will always be "DescribeWorkflowExecution" for this struct.
func (v *AdminService_DescribeWorkflowExecution_Result) MethodName() string {
	return "DescribeWorkflowExecution"
}

// EnvelopeType returns the kind of value inside this struct.
//
// This will always be Reply for this struct.
func (v *AdminService_DescribeWorkflowExecution_Result) EnvelopeType() wire.EnvelopeType {
	return wire.Reply
}
