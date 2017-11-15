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

package health

import (
	"errors"
	"fmt"
	"go.uber.org/thriftrw/wire"
	"strings"
)

type Meta_Health_Args struct{}

func (v *Meta_Health_Args) ToWire() (wire.Value, error) {
	var (
		fields [0]wire.Field
		i      int = 0
	)
	return wire.NewValueStruct(wire.Struct{Fields: fields[:i]}), nil
}

func (v *Meta_Health_Args) FromWire(w wire.Value) error {
	for _, field := range w.GetStruct().Fields {
		switch field.ID {
		}
	}
	return nil
}

func (v *Meta_Health_Args) String() string {
	if v == nil {
		return "<nil>"
	}
	var fields [0]string
	i := 0
	return fmt.Sprintf("Meta_Health_Args{%v}", strings.Join(fields[:i], ", "))
}

func (v *Meta_Health_Args) Equals(rhs *Meta_Health_Args) bool {
	return true
}

func (v *Meta_Health_Args) MethodName() string {
	return "health"
}

func (v *Meta_Health_Args) EnvelopeType() wire.EnvelopeType {
	return wire.Call
}

var Meta_Health_Helper = struct {
	Args           func() *Meta_Health_Args
	IsException    func(error) bool
	WrapResponse   func(*HealthStatus, error) (*Meta_Health_Result, error)
	UnwrapResponse func(*Meta_Health_Result) (*HealthStatus, error)
}{}

func init() {
	Meta_Health_Helper.Args = func() *Meta_Health_Args {
		return &Meta_Health_Args{}
	}
	Meta_Health_Helper.IsException = func(err error) bool {
		switch err.(type) {
		default:
			return false
		}
	}
	Meta_Health_Helper.WrapResponse = func(success *HealthStatus, err error) (*Meta_Health_Result, error) {
		if err == nil {
			return &Meta_Health_Result{Success: success}, nil
		}
		return nil, err
	}
	Meta_Health_Helper.UnwrapResponse = func(result *Meta_Health_Result) (success *HealthStatus, err error) {
		if result.Success != nil {
			success = result.Success
			return
		}
		err = errors.New("expected a non-void result")
		return
	}
}

type Meta_Health_Result struct {
	Success *HealthStatus `json:"success,omitempty"`
}

func (v *Meta_Health_Result) ToWire() (wire.Value, error) {
	var (
		fields [1]wire.Field
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
	if i != 1 {
		return wire.Value{}, fmt.Errorf("Meta_Health_Result should have exactly one field: got %v fields", i)
	}
	return wire.NewValueStruct(wire.Struct{Fields: fields[:i]}), nil
}

func _HealthStatus_Read(w wire.Value) (*HealthStatus, error) {
	var v HealthStatus
	err := v.FromWire(w)
	return &v, err
}

func (v *Meta_Health_Result) FromWire(w wire.Value) error {
	var err error
	for _, field := range w.GetStruct().Fields {
		switch field.ID {
		case 0:
			if field.Value.Type() == wire.TStruct {
				v.Success, err = _HealthStatus_Read(field.Value)
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
	if count != 1 {
		return fmt.Errorf("Meta_Health_Result should have exactly one field: got %v fields", count)
	}
	return nil
}

func (v *Meta_Health_Result) String() string {
	if v == nil {
		return "<nil>"
	}
	var fields [1]string
	i := 0
	if v.Success != nil {
		fields[i] = fmt.Sprintf("Success: %v", v.Success)
		i++
	}
	return fmt.Sprintf("Meta_Health_Result{%v}", strings.Join(fields[:i], ", "))
}

func (v *Meta_Health_Result) Equals(rhs *Meta_Health_Result) bool {
	if !((v.Success == nil && rhs.Success == nil) || (v.Success != nil && rhs.Success != nil && v.Success.Equals(rhs.Success))) {
		return false
	}
	return true
}

func (v *Meta_Health_Result) MethodName() string {
	return "health"
}

func (v *Meta_Health_Result) EnvelopeType() wire.EnvelopeType {
	return wire.Reply
}
