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

package cadence

import (
	"errors"
	"fmt"
	"github.com/uber/cadence/.gen/go/shared"
	"go.uber.org/thriftrw/wire"
	"strings"
)

type WorkflowService_RequestCancelWorkflowExecution_Args struct {
	CancelRequest *shared.RequestCancelWorkflowExecutionRequest `json:"cancelRequest,omitempty"`
}

func (v *WorkflowService_RequestCancelWorkflowExecution_Args) ToWire() (wire.Value, error) {
	var (
		fields [1]wire.Field
		i      int = 0
		w      wire.Value
		err    error
	)
	if v.CancelRequest != nil {
		w, err = v.CancelRequest.ToWire()
		if err != nil {
			return w, err
		}
		fields[i] = wire.Field{ID: 1, Value: w}
		i++
	}
	return wire.NewValueStruct(wire.Struct{Fields: fields[:i]}), nil
}

func _RequestCancelWorkflowExecutionRequest_Read(w wire.Value) (*shared.RequestCancelWorkflowExecutionRequest, error) {
	var v shared.RequestCancelWorkflowExecutionRequest
	err := v.FromWire(w)
	return &v, err
}

func (v *WorkflowService_RequestCancelWorkflowExecution_Args) FromWire(w wire.Value) error {
	var err error
	for _, field := range w.GetStruct().Fields {
		switch field.ID {
		case 1:
			if field.Value.Type() == wire.TStruct {
				v.CancelRequest, err = _RequestCancelWorkflowExecutionRequest_Read(field.Value)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (v *WorkflowService_RequestCancelWorkflowExecution_Args) String() string {
	if v == nil {
		return "<nil>"
	}
	var fields [1]string
	i := 0
	if v.CancelRequest != nil {
		fields[i] = fmt.Sprintf("CancelRequest: %v", v.CancelRequest)
		i++
	}
	return fmt.Sprintf("WorkflowService_RequestCancelWorkflowExecution_Args{%v}", strings.Join(fields[:i], ", "))
}

func (v *WorkflowService_RequestCancelWorkflowExecution_Args) Equals(rhs *WorkflowService_RequestCancelWorkflowExecution_Args) bool {
	if !((v.CancelRequest == nil && rhs.CancelRequest == nil) || (v.CancelRequest != nil && rhs.CancelRequest != nil && v.CancelRequest.Equals(rhs.CancelRequest))) {
		return false
	}
	return true
}

func (v *WorkflowService_RequestCancelWorkflowExecution_Args) MethodName() string {
	return "RequestCancelWorkflowExecution"
}

func (v *WorkflowService_RequestCancelWorkflowExecution_Args) EnvelopeType() wire.EnvelopeType {
	return wire.Call
}

var WorkflowService_RequestCancelWorkflowExecution_Helper = struct {
	Args           func(cancelRequest *shared.RequestCancelWorkflowExecutionRequest) *WorkflowService_RequestCancelWorkflowExecution_Args
	IsException    func(error) bool
	WrapResponse   func(error) (*WorkflowService_RequestCancelWorkflowExecution_Result, error)
	UnwrapResponse func(*WorkflowService_RequestCancelWorkflowExecution_Result) error
}{}

func init() {
	WorkflowService_RequestCancelWorkflowExecution_Helper.Args = func(cancelRequest *shared.RequestCancelWorkflowExecutionRequest) *WorkflowService_RequestCancelWorkflowExecution_Args {
		return &WorkflowService_RequestCancelWorkflowExecution_Args{CancelRequest: cancelRequest}
	}
	WorkflowService_RequestCancelWorkflowExecution_Helper.IsException = func(err error) bool {
		switch err.(type) {
		case *shared.BadRequestError:
			return true
		case *shared.InternalServiceError:
			return true
		case *shared.EntityNotExistsError:
			return true
		case *shared.CancellationAlreadyRequestedError:
			return true
		case *shared.ServiceBusyError:
			return true
		default:
			return false
		}
	}
	WorkflowService_RequestCancelWorkflowExecution_Helper.WrapResponse = func(err error) (*WorkflowService_RequestCancelWorkflowExecution_Result, error) {
		if err == nil {
			return &WorkflowService_RequestCancelWorkflowExecution_Result{}, nil
		}
		switch e := err.(type) {
		case *shared.BadRequestError:
			if e == nil {
				return nil, errors.New("WrapResponse received non-nil error type with nil value for WorkflowService_RequestCancelWorkflowExecution_Result.BadRequestError")
			}
			return &WorkflowService_RequestCancelWorkflowExecution_Result{BadRequestError: e}, nil
		case *shared.InternalServiceError:
			if e == nil {
				return nil, errors.New("WrapResponse received non-nil error type with nil value for WorkflowService_RequestCancelWorkflowExecution_Result.InternalServiceError")
			}
			return &WorkflowService_RequestCancelWorkflowExecution_Result{InternalServiceError: e}, nil
		case *shared.EntityNotExistsError:
			if e == nil {
				return nil, errors.New("WrapResponse received non-nil error type with nil value for WorkflowService_RequestCancelWorkflowExecution_Result.EntityNotExistError")
			}
			return &WorkflowService_RequestCancelWorkflowExecution_Result{EntityNotExistError: e}, nil
		case *shared.CancellationAlreadyRequestedError:
			if e == nil {
				return nil, errors.New("WrapResponse received non-nil error type with nil value for WorkflowService_RequestCancelWorkflowExecution_Result.CancellationAlreadyRequestedError")
			}
			return &WorkflowService_RequestCancelWorkflowExecution_Result{CancellationAlreadyRequestedError: e}, nil
		case *shared.ServiceBusyError:
			if e == nil {
				return nil, errors.New("WrapResponse received non-nil error type with nil value for WorkflowService_RequestCancelWorkflowExecution_Result.ServiceBusyError")
			}
			return &WorkflowService_RequestCancelWorkflowExecution_Result{ServiceBusyError: e}, nil
		}
		return nil, err
	}
	WorkflowService_RequestCancelWorkflowExecution_Helper.UnwrapResponse = func(result *WorkflowService_RequestCancelWorkflowExecution_Result) (err error) {
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
		if result.CancellationAlreadyRequestedError != nil {
			err = result.CancellationAlreadyRequestedError
			return
		}
		if result.ServiceBusyError != nil {
			err = result.ServiceBusyError
			return
		}
		return
	}
}

type WorkflowService_RequestCancelWorkflowExecution_Result struct {
	BadRequestError                   *shared.BadRequestError                   `json:"badRequestError,omitempty"`
	InternalServiceError              *shared.InternalServiceError              `json:"internalServiceError,omitempty"`
	EntityNotExistError               *shared.EntityNotExistsError              `json:"entityNotExistError,omitempty"`
	CancellationAlreadyRequestedError *shared.CancellationAlreadyRequestedError `json:"cancellationAlreadyRequestedError,omitempty"`
	ServiceBusyError                  *shared.ServiceBusyError                  `json:"serviceBusyError,omitempty"`
}

func (v *WorkflowService_RequestCancelWorkflowExecution_Result) ToWire() (wire.Value, error) {
	var (
		fields [5]wire.Field
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
	if v.EntityNotExistError != nil {
		w, err = v.EntityNotExistError.ToWire()
		if err != nil {
			return w, err
		}
		fields[i] = wire.Field{ID: 3, Value: w}
		i++
	}
	if v.CancellationAlreadyRequestedError != nil {
		w, err = v.CancellationAlreadyRequestedError.ToWire()
		if err != nil {
			return w, err
		}
		fields[i] = wire.Field{ID: 4, Value: w}
		i++
	}
	if v.ServiceBusyError != nil {
		w, err = v.ServiceBusyError.ToWire()
		if err != nil {
			return w, err
		}
		fields[i] = wire.Field{ID: 5, Value: w}
		i++
	}
	if i > 1 {
		return wire.Value{}, fmt.Errorf("WorkflowService_RequestCancelWorkflowExecution_Result should have at most one field: got %v fields", i)
	}
	return wire.NewValueStruct(wire.Struct{Fields: fields[:i]}), nil
}

func _CancellationAlreadyRequestedError_Read(w wire.Value) (*shared.CancellationAlreadyRequestedError, error) {
	var v shared.CancellationAlreadyRequestedError
	err := v.FromWire(w)
	return &v, err
}

func (v *WorkflowService_RequestCancelWorkflowExecution_Result) FromWire(w wire.Value) error {
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
				v.EntityNotExistError, err = _EntityNotExistsError_Read(field.Value)
				if err != nil {
					return err
				}
			}
		case 4:
			if field.Value.Type() == wire.TStruct {
				v.CancellationAlreadyRequestedError, err = _CancellationAlreadyRequestedError_Read(field.Value)
				if err != nil {
					return err
				}
			}
		case 5:
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
	if v.EntityNotExistError != nil {
		count++
	}
	if v.CancellationAlreadyRequestedError != nil {
		count++
	}
	if v.ServiceBusyError != nil {
		count++
	}
	if count > 1 {
		return fmt.Errorf("WorkflowService_RequestCancelWorkflowExecution_Result should have at most one field: got %v fields", count)
	}
	return nil
}

func (v *WorkflowService_RequestCancelWorkflowExecution_Result) String() string {
	if v == nil {
		return "<nil>"
	}
	var fields [5]string
	i := 0
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
	if v.CancellationAlreadyRequestedError != nil {
		fields[i] = fmt.Sprintf("CancellationAlreadyRequestedError: %v", v.CancellationAlreadyRequestedError)
		i++
	}
	if v.ServiceBusyError != nil {
		fields[i] = fmt.Sprintf("ServiceBusyError: %v", v.ServiceBusyError)
		i++
	}
	return fmt.Sprintf("WorkflowService_RequestCancelWorkflowExecution_Result{%v}", strings.Join(fields[:i], ", "))
}

func (v *WorkflowService_RequestCancelWorkflowExecution_Result) Equals(rhs *WorkflowService_RequestCancelWorkflowExecution_Result) bool {
	if !((v.BadRequestError == nil && rhs.BadRequestError == nil) || (v.BadRequestError != nil && rhs.BadRequestError != nil && v.BadRequestError.Equals(rhs.BadRequestError))) {
		return false
	}
	if !((v.InternalServiceError == nil && rhs.InternalServiceError == nil) || (v.InternalServiceError != nil && rhs.InternalServiceError != nil && v.InternalServiceError.Equals(rhs.InternalServiceError))) {
		return false
	}
	if !((v.EntityNotExistError == nil && rhs.EntityNotExistError == nil) || (v.EntityNotExistError != nil && rhs.EntityNotExistError != nil && v.EntityNotExistError.Equals(rhs.EntityNotExistError))) {
		return false
	}
	if !((v.CancellationAlreadyRequestedError == nil && rhs.CancellationAlreadyRequestedError == nil) || (v.CancellationAlreadyRequestedError != nil && rhs.CancellationAlreadyRequestedError != nil && v.CancellationAlreadyRequestedError.Equals(rhs.CancellationAlreadyRequestedError))) {
		return false
	}
	if !((v.ServiceBusyError == nil && rhs.ServiceBusyError == nil) || (v.ServiceBusyError != nil && rhs.ServiceBusyError != nil && v.ServiceBusyError.Equals(rhs.ServiceBusyError))) {
		return false
	}
	return true
}

func (v *WorkflowService_RequestCancelWorkflowExecution_Result) MethodName() string {
	return "RequestCancelWorkflowExecution"
}

func (v *WorkflowService_RequestCancelWorkflowExecution_Result) EnvelopeType() wire.EnvelopeType {
	return wire.Reply
}
