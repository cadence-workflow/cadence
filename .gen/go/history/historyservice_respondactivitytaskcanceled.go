// Code generated by thriftrw v1.10.0. DO NOT EDIT.
// @generated

package history

import (
	"errors"
	"fmt"
	"github.com/uber/cadence/.gen/go/shared"
	"go.uber.org/thriftrw/wire"
	"strings"
)

// HistoryService_RespondActivityTaskCanceled_Args represents the arguments for the HistoryService.RespondActivityTaskCanceled function.
//
// The arguments for RespondActivityTaskCanceled are sent and received over the wire as this struct.
type HistoryService_RespondActivityTaskCanceled_Args struct {
	CanceledRequest *RespondActivityTaskCanceledRequest `json:"canceledRequest,omitempty"`
}

// ToWire translates a HistoryService_RespondActivityTaskCanceled_Args struct into a Thrift-level intermediate
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
func (v *HistoryService_RespondActivityTaskCanceled_Args) ToWire() (wire.Value, error) {
	var (
		fields [1]wire.Field
		i      int = 0
		w      wire.Value
		err    error
	)

	if v.CanceledRequest != nil {
		w, err = v.CanceledRequest.ToWire()
		if err != nil {
			return w, err
		}
		fields[i] = wire.Field{ID: 1, Value: w}
		i++
	}

	return wire.NewValueStruct(wire.Struct{Fields: fields[:i]}), nil
}

func _RespondActivityTaskCanceledRequest_1_Read(w wire.Value) (*RespondActivityTaskCanceledRequest, error) {
	var v RespondActivityTaskCanceledRequest
	err := v.FromWire(w)
	return &v, err
}

// FromWire deserializes a HistoryService_RespondActivityTaskCanceled_Args struct from its Thrift-level
// representation. The Thrift-level representation may be obtained
// from a ThriftRW protocol implementation.
//
// An error is returned if we were unable to build a HistoryService_RespondActivityTaskCanceled_Args struct
// from the provided intermediate representation.
//
//   x, err := binaryProtocol.Decode(reader, wire.TStruct)
//   if err != nil {
//     return nil, err
//   }
//
//   var v HistoryService_RespondActivityTaskCanceled_Args
//   if err := v.FromWire(x); err != nil {
//     return nil, err
//   }
//   return &v, nil
func (v *HistoryService_RespondActivityTaskCanceled_Args) FromWire(w wire.Value) error {
	var err error

	for _, field := range w.GetStruct().Fields {
		switch field.ID {
		case 1:
			if field.Value.Type() == wire.TStruct {
				v.CanceledRequest, err = _RespondActivityTaskCanceledRequest_1_Read(field.Value)
				if err != nil {
					return err
				}

			}
		}
	}

	return nil
}

// String returns a readable string representation of a HistoryService_RespondActivityTaskCanceled_Args
// struct.
func (v *HistoryService_RespondActivityTaskCanceled_Args) String() string {
	if v == nil {
		return "<nil>"
	}

	var fields [1]string
	i := 0
	if v.CanceledRequest != nil {
		fields[i] = fmt.Sprintf("CanceledRequest: %v", v.CanceledRequest)
		i++
	}

	return fmt.Sprintf("HistoryService_RespondActivityTaskCanceled_Args{%v}", strings.Join(fields[:i], ", "))
}

// Equals returns true if all the fields of this HistoryService_RespondActivityTaskCanceled_Args match the
// provided HistoryService_RespondActivityTaskCanceled_Args.
//
// This function performs a deep comparison.
func (v *HistoryService_RespondActivityTaskCanceled_Args) Equals(rhs *HistoryService_RespondActivityTaskCanceled_Args) bool {
	if !((v.CanceledRequest == nil && rhs.CanceledRequest == nil) || (v.CanceledRequest != nil && rhs.CanceledRequest != nil && v.CanceledRequest.Equals(rhs.CanceledRequest))) {
		return false
	}

	return true
}

// MethodName returns the name of the Thrift function as specified in
// the IDL, for which this struct represent the arguments.
//
// This will always be "RespondActivityTaskCanceled" for this struct.
func (v *HistoryService_RespondActivityTaskCanceled_Args) MethodName() string {
	return "RespondActivityTaskCanceled"
}

// EnvelopeType returns the kind of value inside this struct.
//
// This will always be Call for this struct.
func (v *HistoryService_RespondActivityTaskCanceled_Args) EnvelopeType() wire.EnvelopeType {
	return wire.Call
}

// HistoryService_RespondActivityTaskCanceled_Helper provides functions that aid in handling the
// parameters and return values of the HistoryService.RespondActivityTaskCanceled
// function.
var HistoryService_RespondActivityTaskCanceled_Helper = struct {
	// Args accepts the parameters of RespondActivityTaskCanceled in-order and returns
	// the arguments struct for the function.
	Args func(
		canceledRequest *RespondActivityTaskCanceledRequest,
	) *HistoryService_RespondActivityTaskCanceled_Args

	// IsException returns true if the given error can be thrown
	// by RespondActivityTaskCanceled.
	//
	// An error can be thrown by RespondActivityTaskCanceled only if the
	// corresponding exception type was mentioned in the 'throws'
	// section for it in the Thrift file.
	IsException func(error) bool

	// WrapResponse returns the result struct for RespondActivityTaskCanceled
	// given the error returned by it. The provided error may
	// be nil if RespondActivityTaskCanceled did not fail.
	//
	// This allows mapping errors returned by RespondActivityTaskCanceled into a
	// serializable result struct. WrapResponse returns a
	// non-nil error if the provided error cannot be thrown by
	// RespondActivityTaskCanceled
	//
	//   err := RespondActivityTaskCanceled(args)
	//   result, err := HistoryService_RespondActivityTaskCanceled_Helper.WrapResponse(err)
	//   if err != nil {
	//     return fmt.Errorf("unexpected error from RespondActivityTaskCanceled: %v", err)
	//   }
	//   serialize(result)
	WrapResponse func(error) (*HistoryService_RespondActivityTaskCanceled_Result, error)

	// UnwrapResponse takes the result struct for RespondActivityTaskCanceled
	// and returns the erorr returned by it (if any).
	//
	// The error is non-nil only if RespondActivityTaskCanceled threw an
	// exception.
	//
	//   result := deserialize(bytes)
	//   err := HistoryService_RespondActivityTaskCanceled_Helper.UnwrapResponse(result)
	UnwrapResponse func(*HistoryService_RespondActivityTaskCanceled_Result) error
}{}

func init() {
	HistoryService_RespondActivityTaskCanceled_Helper.Args = func(
		canceledRequest *RespondActivityTaskCanceledRequest,
	) *HistoryService_RespondActivityTaskCanceled_Args {
		return &HistoryService_RespondActivityTaskCanceled_Args{
			CanceledRequest: canceledRequest,
		}
	}

	HistoryService_RespondActivityTaskCanceled_Helper.IsException = func(err error) bool {
		switch err.(type) {
		case *shared.BadRequestError:
			return true
		case *shared.InternalServiceError:
			return true
		case *shared.EntityNotExistsError:
			return true
		case *ShardOwnershipLostError:
			return true
		default:
			return false
		}
	}

	HistoryService_RespondActivityTaskCanceled_Helper.WrapResponse = func(err error) (*HistoryService_RespondActivityTaskCanceled_Result, error) {
		if err == nil {
			return &HistoryService_RespondActivityTaskCanceled_Result{}, nil
		}

		switch e := err.(type) {
		case *shared.BadRequestError:
			if e == nil {
				return nil, errors.New("WrapResponse received non-nil error type with nil value for HistoryService_RespondActivityTaskCanceled_Result.BadRequestError")
			}
			return &HistoryService_RespondActivityTaskCanceled_Result{BadRequestError: e}, nil
		case *shared.InternalServiceError:
			if e == nil {
				return nil, errors.New("WrapResponse received non-nil error type with nil value for HistoryService_RespondActivityTaskCanceled_Result.InternalServiceError")
			}
			return &HistoryService_RespondActivityTaskCanceled_Result{InternalServiceError: e}, nil
		case *shared.EntityNotExistsError:
			if e == nil {
				return nil, errors.New("WrapResponse received non-nil error type with nil value for HistoryService_RespondActivityTaskCanceled_Result.EntityNotExistError")
			}
			return &HistoryService_RespondActivityTaskCanceled_Result{EntityNotExistError: e}, nil
		case *ShardOwnershipLostError:
			if e == nil {
				return nil, errors.New("WrapResponse received non-nil error type with nil value for HistoryService_RespondActivityTaskCanceled_Result.ShardOwnershipLostError")
			}
			return &HistoryService_RespondActivityTaskCanceled_Result{ShardOwnershipLostError: e}, nil
		}

		return nil, err
	}
	HistoryService_RespondActivityTaskCanceled_Helper.UnwrapResponse = func(result *HistoryService_RespondActivityTaskCanceled_Result) (err error) {
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
		if result.ShardOwnershipLostError != nil {
			err = result.ShardOwnershipLostError
			return
		}
		return
	}

}

// HistoryService_RespondActivityTaskCanceled_Result represents the result of a HistoryService.RespondActivityTaskCanceled function call.
//
// The result of a RespondActivityTaskCanceled execution is sent and received over the wire as this struct.
type HistoryService_RespondActivityTaskCanceled_Result struct {
	BadRequestError         *shared.BadRequestError      `json:"badRequestError,omitempty"`
	InternalServiceError    *shared.InternalServiceError `json:"internalServiceError,omitempty"`
	EntityNotExistError     *shared.EntityNotExistsError `json:"entityNotExistError,omitempty"`
	ShardOwnershipLostError *ShardOwnershipLostError     `json:"shardOwnershipLostError,omitempty"`
}

// ToWire translates a HistoryService_RespondActivityTaskCanceled_Result struct into a Thrift-level intermediate
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
func (v *HistoryService_RespondActivityTaskCanceled_Result) ToWire() (wire.Value, error) {
	var (
		fields [4]wire.Field
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
	if v.ShardOwnershipLostError != nil {
		w, err = v.ShardOwnershipLostError.ToWire()
		if err != nil {
			return w, err
		}
		fields[i] = wire.Field{ID: 4, Value: w}
		i++
	}

	if i > 1 {
		return wire.Value{}, fmt.Errorf("HistoryService_RespondActivityTaskCanceled_Result should have at most one field: got %v fields", i)
	}

	return wire.NewValueStruct(wire.Struct{Fields: fields[:i]}), nil
}

// FromWire deserializes a HistoryService_RespondActivityTaskCanceled_Result struct from its Thrift-level
// representation. The Thrift-level representation may be obtained
// from a ThriftRW protocol implementation.
//
// An error is returned if we were unable to build a HistoryService_RespondActivityTaskCanceled_Result struct
// from the provided intermediate representation.
//
//   x, err := binaryProtocol.Decode(reader, wire.TStruct)
//   if err != nil {
//     return nil, err
//   }
//
//   var v HistoryService_RespondActivityTaskCanceled_Result
//   if err := v.FromWire(x); err != nil {
//     return nil, err
//   }
//   return &v, nil
func (v *HistoryService_RespondActivityTaskCanceled_Result) FromWire(w wire.Value) error {
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
				v.ShardOwnershipLostError, err = _ShardOwnershipLostError_Read(field.Value)
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
	if v.ShardOwnershipLostError != nil {
		count++
	}
	if count > 1 {
		return fmt.Errorf("HistoryService_RespondActivityTaskCanceled_Result should have at most one field: got %v fields", count)
	}

	return nil
}

// String returns a readable string representation of a HistoryService_RespondActivityTaskCanceled_Result
// struct.
func (v *HistoryService_RespondActivityTaskCanceled_Result) String() string {
	if v == nil {
		return "<nil>"
	}

	var fields [4]string
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
	if v.ShardOwnershipLostError != nil {
		fields[i] = fmt.Sprintf("ShardOwnershipLostError: %v", v.ShardOwnershipLostError)
		i++
	}

	return fmt.Sprintf("HistoryService_RespondActivityTaskCanceled_Result{%v}", strings.Join(fields[:i], ", "))
}

// Equals returns true if all the fields of this HistoryService_RespondActivityTaskCanceled_Result match the
// provided HistoryService_RespondActivityTaskCanceled_Result.
//
// This function performs a deep comparison.
func (v *HistoryService_RespondActivityTaskCanceled_Result) Equals(rhs *HistoryService_RespondActivityTaskCanceled_Result) bool {
	if !((v.BadRequestError == nil && rhs.BadRequestError == nil) || (v.BadRequestError != nil && rhs.BadRequestError != nil && v.BadRequestError.Equals(rhs.BadRequestError))) {
		return false
	}
	if !((v.InternalServiceError == nil && rhs.InternalServiceError == nil) || (v.InternalServiceError != nil && rhs.InternalServiceError != nil && v.InternalServiceError.Equals(rhs.InternalServiceError))) {
		return false
	}
	if !((v.EntityNotExistError == nil && rhs.EntityNotExistError == nil) || (v.EntityNotExistError != nil && rhs.EntityNotExistError != nil && v.EntityNotExistError.Equals(rhs.EntityNotExistError))) {
		return false
	}
	if !((v.ShardOwnershipLostError == nil && rhs.ShardOwnershipLostError == nil) || (v.ShardOwnershipLostError != nil && rhs.ShardOwnershipLostError != nil && v.ShardOwnershipLostError.Equals(rhs.ShardOwnershipLostError))) {
		return false
	}

	return true
}

// MethodName returns the name of the Thrift function as specified in
// the IDL, for which this struct represent the result.
//
// This will always be "RespondActivityTaskCanceled" for this struct.
func (v *HistoryService_RespondActivityTaskCanceled_Result) MethodName() string {
	return "RespondActivityTaskCanceled"
}

// EnvelopeType returns the kind of value inside this struct.
//
// This will always be Reply for this struct.
func (v *HistoryService_RespondActivityTaskCanceled_Result) EnvelopeType() wire.EnvelopeType {
	return wire.Reply
}
