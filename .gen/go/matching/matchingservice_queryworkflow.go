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

// Code generated by thriftrw v1.6.0. DO NOT EDIT.
// @generated

package matching

import (
	"errors"
	"fmt"
	"github.com/uber/cadence/.gen/go/shared"
	"go.uber.org/thriftrw/wire"
	"strings"
)

type MatchingService_QueryWorkflow_Args struct {
	QueryRequest *QueryWorkflowRequest `json:"queryRequest,omitempty"`
}

func (v *MatchingService_QueryWorkflow_Args) ToWire() (wire.Value, error) {
	var (
		fields [1]wire.Field
		i      int = 0
		w      wire.Value
		err    error
	)
	if v.QueryRequest != nil {
		w, err = v.QueryRequest.ToWire()
		if err != nil {
			return w, err
		}
		fields[i] = wire.Field{ID: 1, Value: w}
		i++
	}
	return wire.NewValueStruct(wire.Struct{Fields: fields[:i]}), nil
}

func _QueryWorkflowRequest_1_Read(w wire.Value) (*QueryWorkflowRequest, error) {
	var v QueryWorkflowRequest
	err := v.FromWire(w)
	return &v, err
}

func (v *MatchingService_QueryWorkflow_Args) FromWire(w wire.Value) error {
	var err error
	for _, field := range w.GetStruct().Fields {
		switch field.ID {
		case 1:
			if field.Value.Type() == wire.TStruct {
				v.QueryRequest, err = _QueryWorkflowRequest_1_Read(field.Value)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (v *MatchingService_QueryWorkflow_Args) String() string {
	if v == nil {
		return "<nil>"
	}
	var fields [1]string
	i := 0
	if v.QueryRequest != nil {
		fields[i] = fmt.Sprintf("QueryRequest: %v", v.QueryRequest)
		i++
	}
	return fmt.Sprintf("MatchingService_QueryWorkflow_Args{%v}", strings.Join(fields[:i], ", "))
}

func (v *MatchingService_QueryWorkflow_Args) Equals(rhs *MatchingService_QueryWorkflow_Args) bool {
	if !((v.QueryRequest == nil && rhs.QueryRequest == nil) || (v.QueryRequest != nil && rhs.QueryRequest != nil && v.QueryRequest.Equals(rhs.QueryRequest))) {
		return false
	}
	return true
}

func (v *MatchingService_QueryWorkflow_Args) MethodName() string {
	return "QueryWorkflow"
}

func (v *MatchingService_QueryWorkflow_Args) EnvelopeType() wire.EnvelopeType {
	return wire.Call
}

var MatchingService_QueryWorkflow_Helper = struct {
	Args           func(queryRequest *QueryWorkflowRequest) *MatchingService_QueryWorkflow_Args
	IsException    func(error) bool
	WrapResponse   func(*shared.QueryWorkflowResponse, error) (*MatchingService_QueryWorkflow_Result, error)
	UnwrapResponse func(*MatchingService_QueryWorkflow_Result) (*shared.QueryWorkflowResponse, error)
}{}

func init() {
	MatchingService_QueryWorkflow_Helper.Args = func(queryRequest *QueryWorkflowRequest) *MatchingService_QueryWorkflow_Args {
		return &MatchingService_QueryWorkflow_Args{QueryRequest: queryRequest}
	}
	MatchingService_QueryWorkflow_Helper.IsException = func(err error) bool {
		switch err.(type) {
		case *shared.BadRequestError:
			return true
		case *shared.InternalServiceError:
			return true
		case *shared.EntityNotExistsError:
			return true
		case *shared.QueryFailedError:
			return true
		default:
			return false
		}
	}
	MatchingService_QueryWorkflow_Helper.WrapResponse = func(success *shared.QueryWorkflowResponse, err error) (*MatchingService_QueryWorkflow_Result, error) {
		if err == nil {
			return &MatchingService_QueryWorkflow_Result{Success: success}, nil
		}
		switch e := err.(type) {
		case *shared.BadRequestError:
			if e == nil {
				return nil, errors.New("WrapResponse received non-nil error type with nil value for MatchingService_QueryWorkflow_Result.BadRequestError")
			}
			return &MatchingService_QueryWorkflow_Result{BadRequestError: e}, nil
		case *shared.InternalServiceError:
			if e == nil {
				return nil, errors.New("WrapResponse received non-nil error type with nil value for MatchingService_QueryWorkflow_Result.InternalServiceError")
			}
			return &MatchingService_QueryWorkflow_Result{InternalServiceError: e}, nil
		case *shared.EntityNotExistsError:
			if e == nil {
				return nil, errors.New("WrapResponse received non-nil error type with nil value for MatchingService_QueryWorkflow_Result.EntityNotExistError")
			}
			return &MatchingService_QueryWorkflow_Result{EntityNotExistError: e}, nil
		case *shared.QueryFailedError:
			if e == nil {
				return nil, errors.New("WrapResponse received non-nil error type with nil value for MatchingService_QueryWorkflow_Result.QueryFailedError")
			}
			return &MatchingService_QueryWorkflow_Result{QueryFailedError: e}, nil
		}
		return nil, err
	}
	MatchingService_QueryWorkflow_Helper.UnwrapResponse = func(result *MatchingService_QueryWorkflow_Result) (success *shared.QueryWorkflowResponse, err error) {
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
		if result.QueryFailedError != nil {
			err = result.QueryFailedError
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

type MatchingService_QueryWorkflow_Result struct {
	Success              *shared.QueryWorkflowResponse `json:"success,omitempty"`
	BadRequestError      *shared.BadRequestError       `json:"badRequestError,omitempty"`
	InternalServiceError *shared.InternalServiceError  `json:"internalServiceError,omitempty"`
	EntityNotExistError  *shared.EntityNotExistsError  `json:"entityNotExistError,omitempty"`
	QueryFailedError     *shared.QueryFailedError      `json:"queryFailedError,omitempty"`
}

func (v *MatchingService_QueryWorkflow_Result) ToWire() (wire.Value, error) {
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
	if v.QueryFailedError != nil {
		w, err = v.QueryFailedError.ToWire()
		if err != nil {
			return w, err
		}
		fields[i] = wire.Field{ID: 4, Value: w}
		i++
	}
	if i != 1 {
		return wire.Value{}, fmt.Errorf("MatchingService_QueryWorkflow_Result should have exactly one field: got %v fields", i)
	}
	return wire.NewValueStruct(wire.Struct{Fields: fields[:i]}), nil
}

func _QueryWorkflowResponse_Read(w wire.Value) (*shared.QueryWorkflowResponse, error) {
	var v shared.QueryWorkflowResponse
	err := v.FromWire(w)
	return &v, err
}

func _EntityNotExistsError_Read(w wire.Value) (*shared.EntityNotExistsError, error) {
	var v shared.EntityNotExistsError
	err := v.FromWire(w)
	return &v, err
}

func _QueryFailedError_Read(w wire.Value) (*shared.QueryFailedError, error) {
	var v shared.QueryFailedError
	err := v.FromWire(w)
	return &v, err
}

func (v *MatchingService_QueryWorkflow_Result) FromWire(w wire.Value) error {
	var err error
	for _, field := range w.GetStruct().Fields {
		switch field.ID {
		case 0:
			if field.Value.Type() == wire.TStruct {
				v.Success, err = _QueryWorkflowResponse_Read(field.Value)
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
				v.QueryFailedError, err = _QueryFailedError_Read(field.Value)
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
	if v.QueryFailedError != nil {
		count++
	}
	if count != 1 {
		return fmt.Errorf("MatchingService_QueryWorkflow_Result should have exactly one field: got %v fields", count)
	}
	return nil
}

func (v *MatchingService_QueryWorkflow_Result) String() string {
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
	if v.QueryFailedError != nil {
		fields[i] = fmt.Sprintf("QueryFailedError: %v", v.QueryFailedError)
		i++
	}
	return fmt.Sprintf("MatchingService_QueryWorkflow_Result{%v}", strings.Join(fields[:i], ", "))
}

func (v *MatchingService_QueryWorkflow_Result) Equals(rhs *MatchingService_QueryWorkflow_Result) bool {
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
	if !((v.QueryFailedError == nil && rhs.QueryFailedError == nil) || (v.QueryFailedError != nil && rhs.QueryFailedError != nil && v.QueryFailedError.Equals(rhs.QueryFailedError))) {
		return false
	}
	return true
}

func (v *MatchingService_QueryWorkflow_Result) MethodName() string {
	return "QueryWorkflow"
}

func (v *MatchingService_QueryWorkflow_Result) EnvelopeType() wire.EnvelopeType {
	return wire.Reply
}
