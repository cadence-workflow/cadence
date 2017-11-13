// Code generated by thriftrw-plugin-yarpc
// @generated

package historyserviceserver

import (
	"context"
	"github.com/uber/cadence/.gen/go/history"
	"github.com/uber/cadence/.gen/go/shared"
	"go.uber.org/thriftrw/wire"
	"go.uber.org/yarpc/api/transport"
	"go.uber.org/yarpc/encoding/thrift"
)

// Interface is the server-side interface for the HistoryService service.
type Interface interface {
	DescribeWorkflowExecution(
		ctx context.Context,
		DescribeRequest *history.DescribeWorkflowExecutionRequest,
	) (*shared.DescribeWorkflowExecutionResponse, error)

	GetWorkflowExecutionNextEventID(
		ctx context.Context,
		GetRequest *history.GetWorkflowExecutionNextEventIDRequest,
	) (*history.GetWorkflowExecutionNextEventIDResponse, error)

	RecordActivityTaskHeartbeat(
		ctx context.Context,
		HeartbeatRequest *history.RecordActivityTaskHeartbeatRequest,
	) (*shared.RecordActivityTaskHeartbeatResponse, error)

	RecordActivityTaskStarted(
		ctx context.Context,
		AddRequest *history.RecordActivityTaskStartedRequest,
	) (*history.RecordActivityTaskStartedResponse, error)

	RecordChildExecutionCompleted(
		ctx context.Context,
		CompletionRequest *history.RecordChildExecutionCompletedRequest,
	) error

	RecordDecisionTaskStarted(
		ctx context.Context,
		AddRequest *history.RecordDecisionTaskStartedRequest,
	) (*history.RecordDecisionTaskStartedResponse, error)

	RequestCancelWorkflowExecution(
		ctx context.Context,
		CancelRequest *history.RequestCancelWorkflowExecutionRequest,
	) error

	RespondActivityTaskCanceled(
		ctx context.Context,
		CanceledRequest *history.RespondActivityTaskCanceledRequest,
	) error

	RespondActivityTaskCompleted(
		ctx context.Context,
		CompleteRequest *history.RespondActivityTaskCompletedRequest,
	) error

	RespondActivityTaskFailed(
		ctx context.Context,
		FailRequest *history.RespondActivityTaskFailedRequest,
	) error

	RespondDecisionTaskCompleted(
		ctx context.Context,
		CompleteRequest *history.RespondDecisionTaskCompletedRequest,
	) error

	RespondDecisionTaskFailed(
		ctx context.Context,
		FailedRequest *history.RespondDecisionTaskFailedRequest,
	) error

	ScheduleDecisionTask(
		ctx context.Context,
		ScheduleRequest *history.ScheduleDecisionTaskRequest,
	) error

	SignalWorkflowExecution(
		ctx context.Context,
		SignalRequest *history.SignalWorkflowExecutionRequest,
	) error

	StartWorkflowExecution(
		ctx context.Context,
		StartRequest *history.StartWorkflowExecutionRequest,
	) (*shared.StartWorkflowExecutionResponse, error)

	TerminateWorkflowExecution(
		ctx context.Context,
		TerminateRequest *history.TerminateWorkflowExecutionRequest,
	) error
}

// New prepares an implementation of the HistoryService service for
// registration.
//
// 	handler := HistoryServiceHandler{}
// 	dispatcher.Register(historyserviceserver.New(handler))
func New(impl Interface, opts ...thrift.RegisterOption) []transport.Procedure {
	h := handler{impl}
	service := thrift.Service{
		Name: "HistoryService",
		Methods: []thrift.Method{

			thrift.Method{
				Name: "DescribeWorkflowExecution",
				HandlerSpec: thrift.HandlerSpec{

					Type:  transport.Unary,
					Unary: thrift.UnaryHandler(h.DescribeWorkflowExecution),
				},
				Signature:    "DescribeWorkflowExecution(DescribeRequest *history.DescribeWorkflowExecutionRequest) (*shared.DescribeWorkflowExecutionResponse)",
				ThriftModule: history.ThriftModule,
			},

			thrift.Method{
				Name: "GetWorkflowExecutionNextEventID",
				HandlerSpec: thrift.HandlerSpec{

					Type:  transport.Unary,
					Unary: thrift.UnaryHandler(h.GetWorkflowExecutionNextEventID),
				},
				Signature:    "GetWorkflowExecutionNextEventID(GetRequest *history.GetWorkflowExecutionNextEventIDRequest) (*history.GetWorkflowExecutionNextEventIDResponse)",
				ThriftModule: history.ThriftModule,
			},

			thrift.Method{
				Name: "RecordActivityTaskHeartbeat",
				HandlerSpec: thrift.HandlerSpec{

					Type:  transport.Unary,
					Unary: thrift.UnaryHandler(h.RecordActivityTaskHeartbeat),
				},
				Signature:    "RecordActivityTaskHeartbeat(HeartbeatRequest *history.RecordActivityTaskHeartbeatRequest) (*shared.RecordActivityTaskHeartbeatResponse)",
				ThriftModule: history.ThriftModule,
			},

			thrift.Method{
				Name: "RecordActivityTaskStarted",
				HandlerSpec: thrift.HandlerSpec{

					Type:  transport.Unary,
					Unary: thrift.UnaryHandler(h.RecordActivityTaskStarted),
				},
				Signature:    "RecordActivityTaskStarted(AddRequest *history.RecordActivityTaskStartedRequest) (*history.RecordActivityTaskStartedResponse)",
				ThriftModule: history.ThriftModule,
			},

			thrift.Method{
				Name: "RecordChildExecutionCompleted",
				HandlerSpec: thrift.HandlerSpec{

					Type:  transport.Unary,
					Unary: thrift.UnaryHandler(h.RecordChildExecutionCompleted),
				},
				Signature:    "RecordChildExecutionCompleted(CompletionRequest *history.RecordChildExecutionCompletedRequest)",
				ThriftModule: history.ThriftModule,
			},

			thrift.Method{
				Name: "RecordDecisionTaskStarted",
				HandlerSpec: thrift.HandlerSpec{

					Type:  transport.Unary,
					Unary: thrift.UnaryHandler(h.RecordDecisionTaskStarted),
				},
				Signature:    "RecordDecisionTaskStarted(AddRequest *history.RecordDecisionTaskStartedRequest) (*history.RecordDecisionTaskStartedResponse)",
				ThriftModule: history.ThriftModule,
			},

			thrift.Method{
				Name: "RequestCancelWorkflowExecution",
				HandlerSpec: thrift.HandlerSpec{

					Type:  transport.Unary,
					Unary: thrift.UnaryHandler(h.RequestCancelWorkflowExecution),
				},
				Signature:    "RequestCancelWorkflowExecution(CancelRequest *history.RequestCancelWorkflowExecutionRequest)",
				ThriftModule: history.ThriftModule,
			},

			thrift.Method{
				Name: "RespondActivityTaskCanceled",
				HandlerSpec: thrift.HandlerSpec{

					Type:  transport.Unary,
					Unary: thrift.UnaryHandler(h.RespondActivityTaskCanceled),
				},
				Signature:    "RespondActivityTaskCanceled(CanceledRequest *history.RespondActivityTaskCanceledRequest)",
				ThriftModule: history.ThriftModule,
			},

			thrift.Method{
				Name: "RespondActivityTaskCompleted",
				HandlerSpec: thrift.HandlerSpec{

					Type:  transport.Unary,
					Unary: thrift.UnaryHandler(h.RespondActivityTaskCompleted),
				},
				Signature:    "RespondActivityTaskCompleted(CompleteRequest *history.RespondActivityTaskCompletedRequest)",
				ThriftModule: history.ThriftModule,
			},

			thrift.Method{
				Name: "RespondActivityTaskFailed",
				HandlerSpec: thrift.HandlerSpec{

					Type:  transport.Unary,
					Unary: thrift.UnaryHandler(h.RespondActivityTaskFailed),
				},
				Signature:    "RespondActivityTaskFailed(FailRequest *history.RespondActivityTaskFailedRequest)",
				ThriftModule: history.ThriftModule,
			},

			thrift.Method{
				Name: "RespondDecisionTaskCompleted",
				HandlerSpec: thrift.HandlerSpec{

					Type:  transport.Unary,
					Unary: thrift.UnaryHandler(h.RespondDecisionTaskCompleted),
				},
				Signature:    "RespondDecisionTaskCompleted(CompleteRequest *history.RespondDecisionTaskCompletedRequest)",
				ThriftModule: history.ThriftModule,
			},

			thrift.Method{
				Name: "RespondDecisionTaskFailed",
				HandlerSpec: thrift.HandlerSpec{

					Type:  transport.Unary,
					Unary: thrift.UnaryHandler(h.RespondDecisionTaskFailed),
				},
				Signature:    "RespondDecisionTaskFailed(FailedRequest *history.RespondDecisionTaskFailedRequest)",
				ThriftModule: history.ThriftModule,
			},

			thrift.Method{
				Name: "ScheduleDecisionTask",
				HandlerSpec: thrift.HandlerSpec{

					Type:  transport.Unary,
					Unary: thrift.UnaryHandler(h.ScheduleDecisionTask),
				},
				Signature:    "ScheduleDecisionTask(ScheduleRequest *history.ScheduleDecisionTaskRequest)",
				ThriftModule: history.ThriftModule,
			},

			thrift.Method{
				Name: "SignalWorkflowExecution",
				HandlerSpec: thrift.HandlerSpec{

					Type:  transport.Unary,
					Unary: thrift.UnaryHandler(h.SignalWorkflowExecution),
				},
				Signature:    "SignalWorkflowExecution(SignalRequest *history.SignalWorkflowExecutionRequest)",
				ThriftModule: history.ThriftModule,
			},

			thrift.Method{
				Name: "StartWorkflowExecution",
				HandlerSpec: thrift.HandlerSpec{

					Type:  transport.Unary,
					Unary: thrift.UnaryHandler(h.StartWorkflowExecution),
				},
				Signature:    "StartWorkflowExecution(StartRequest *history.StartWorkflowExecutionRequest) (*shared.StartWorkflowExecutionResponse)",
				ThriftModule: history.ThriftModule,
			},

			thrift.Method{
				Name: "TerminateWorkflowExecution",
				HandlerSpec: thrift.HandlerSpec{

					Type:  transport.Unary,
					Unary: thrift.UnaryHandler(h.TerminateWorkflowExecution),
				},
				Signature:    "TerminateWorkflowExecution(TerminateRequest *history.TerminateWorkflowExecutionRequest)",
				ThriftModule: history.ThriftModule,
			},
		},
	}

	procedures := make([]transport.Procedure, 0, 15)
	procedures = append(procedures, thrift.BuildProcedures(service, opts...)...)
	return procedures
}

type handler struct{ impl Interface }

func (h handler) DescribeWorkflowExecution(ctx context.Context, body wire.Value) (thrift.Response, error) {
	var args history.HistoryService_DescribeWorkflowExecution_Args
	if err := args.FromWire(body); err != nil {
		return thrift.Response{}, err
	}

	success, err := h.impl.DescribeWorkflowExecution(ctx, args.DescribeRequest)

	hadError := err != nil
	result, err := history.HistoryService_DescribeWorkflowExecution_Helper.WrapResponse(success, err)

	var response thrift.Response
	if err == nil {
		response.IsApplicationError = hadError
		response.Body = result
	}
	return response, err
}

func (h handler) GetWorkflowExecutionNextEventID(ctx context.Context, body wire.Value) (thrift.Response, error) {
	var args history.HistoryService_GetWorkflowExecutionNextEventID_Args
	if err := args.FromWire(body); err != nil {
		return thrift.Response{}, err
	}

	success, err := h.impl.GetWorkflowExecutionNextEventID(ctx, args.GetRequest)

	hadError := err != nil
	result, err := history.HistoryService_GetWorkflowExecutionNextEventID_Helper.WrapResponse(success, err)

	var response thrift.Response
	if err == nil {
		response.IsApplicationError = hadError
		response.Body = result
	}
	return response, err
}

func (h handler) RecordActivityTaskHeartbeat(ctx context.Context, body wire.Value) (thrift.Response, error) {
	var args history.HistoryService_RecordActivityTaskHeartbeat_Args
	if err := args.FromWire(body); err != nil {
		return thrift.Response{}, err
	}

	success, err := h.impl.RecordActivityTaskHeartbeat(ctx, args.HeartbeatRequest)

	hadError := err != nil
	result, err := history.HistoryService_RecordActivityTaskHeartbeat_Helper.WrapResponse(success, err)

	var response thrift.Response
	if err == nil {
		response.IsApplicationError = hadError
		response.Body = result
	}
	return response, err
}

func (h handler) RecordActivityTaskStarted(ctx context.Context, body wire.Value) (thrift.Response, error) {
	var args history.HistoryService_RecordActivityTaskStarted_Args
	if err := args.FromWire(body); err != nil {
		return thrift.Response{}, err
	}

	success, err := h.impl.RecordActivityTaskStarted(ctx, args.AddRequest)

	hadError := err != nil
	result, err := history.HistoryService_RecordActivityTaskStarted_Helper.WrapResponse(success, err)

	var response thrift.Response
	if err == nil {
		response.IsApplicationError = hadError
		response.Body = result
	}
	return response, err
}

func (h handler) RecordChildExecutionCompleted(ctx context.Context, body wire.Value) (thrift.Response, error) {
	var args history.HistoryService_RecordChildExecutionCompleted_Args
	if err := args.FromWire(body); err != nil {
		return thrift.Response{}, err
	}

	err := h.impl.RecordChildExecutionCompleted(ctx, args.CompletionRequest)

	hadError := err != nil
	result, err := history.HistoryService_RecordChildExecutionCompleted_Helper.WrapResponse(err)

	var response thrift.Response
	if err == nil {
		response.IsApplicationError = hadError
		response.Body = result
	}
	return response, err
}

func (h handler) RecordDecisionTaskStarted(ctx context.Context, body wire.Value) (thrift.Response, error) {
	var args history.HistoryService_RecordDecisionTaskStarted_Args
	if err := args.FromWire(body); err != nil {
		return thrift.Response{}, err
	}

	success, err := h.impl.RecordDecisionTaskStarted(ctx, args.AddRequest)

	hadError := err != nil
	result, err := history.HistoryService_RecordDecisionTaskStarted_Helper.WrapResponse(success, err)

	var response thrift.Response
	if err == nil {
		response.IsApplicationError = hadError
		response.Body = result
	}
	return response, err
}

func (h handler) RequestCancelWorkflowExecution(ctx context.Context, body wire.Value) (thrift.Response, error) {
	var args history.HistoryService_RequestCancelWorkflowExecution_Args
	if err := args.FromWire(body); err != nil {
		return thrift.Response{}, err
	}

	err := h.impl.RequestCancelWorkflowExecution(ctx, args.CancelRequest)

	hadError := err != nil
	result, err := history.HistoryService_RequestCancelWorkflowExecution_Helper.WrapResponse(err)

	var response thrift.Response
	if err == nil {
		response.IsApplicationError = hadError
		response.Body = result
	}
	return response, err
}

func (h handler) RespondActivityTaskCanceled(ctx context.Context, body wire.Value) (thrift.Response, error) {
	var args history.HistoryService_RespondActivityTaskCanceled_Args
	if err := args.FromWire(body); err != nil {
		return thrift.Response{}, err
	}

	err := h.impl.RespondActivityTaskCanceled(ctx, args.CanceledRequest)

	hadError := err != nil
	result, err := history.HistoryService_RespondActivityTaskCanceled_Helper.WrapResponse(err)

	var response thrift.Response
	if err == nil {
		response.IsApplicationError = hadError
		response.Body = result
	}
	return response, err
}

func (h handler) RespondActivityTaskCompleted(ctx context.Context, body wire.Value) (thrift.Response, error) {
	var args history.HistoryService_RespondActivityTaskCompleted_Args
	if err := args.FromWire(body); err != nil {
		return thrift.Response{}, err
	}

	err := h.impl.RespondActivityTaskCompleted(ctx, args.CompleteRequest)

	hadError := err != nil
	result, err := history.HistoryService_RespondActivityTaskCompleted_Helper.WrapResponse(err)

	var response thrift.Response
	if err == nil {
		response.IsApplicationError = hadError
		response.Body = result
	}
	return response, err
}

func (h handler) RespondActivityTaskFailed(ctx context.Context, body wire.Value) (thrift.Response, error) {
	var args history.HistoryService_RespondActivityTaskFailed_Args
	if err := args.FromWire(body); err != nil {
		return thrift.Response{}, err
	}

	err := h.impl.RespondActivityTaskFailed(ctx, args.FailRequest)

	hadError := err != nil
	result, err := history.HistoryService_RespondActivityTaskFailed_Helper.WrapResponse(err)

	var response thrift.Response
	if err == nil {
		response.IsApplicationError = hadError
		response.Body = result
	}
	return response, err
}

func (h handler) RespondDecisionTaskCompleted(ctx context.Context, body wire.Value) (thrift.Response, error) {
	var args history.HistoryService_RespondDecisionTaskCompleted_Args
	if err := args.FromWire(body); err != nil {
		return thrift.Response{}, err
	}

	err := h.impl.RespondDecisionTaskCompleted(ctx, args.CompleteRequest)

	hadError := err != nil
	result, err := history.HistoryService_RespondDecisionTaskCompleted_Helper.WrapResponse(err)

	var response thrift.Response
	if err == nil {
		response.IsApplicationError = hadError
		response.Body = result
	}
	return response, err
}

func (h handler) RespondDecisionTaskFailed(ctx context.Context, body wire.Value) (thrift.Response, error) {
	var args history.HistoryService_RespondDecisionTaskFailed_Args
	if err := args.FromWire(body); err != nil {
		return thrift.Response{}, err
	}

	err := h.impl.RespondDecisionTaskFailed(ctx, args.FailedRequest)

	hadError := err != nil
	result, err := history.HistoryService_RespondDecisionTaskFailed_Helper.WrapResponse(err)

	var response thrift.Response
	if err == nil {
		response.IsApplicationError = hadError
		response.Body = result
	}
	return response, err
}

func (h handler) ScheduleDecisionTask(ctx context.Context, body wire.Value) (thrift.Response, error) {
	var args history.HistoryService_ScheduleDecisionTask_Args
	if err := args.FromWire(body); err != nil {
		return thrift.Response{}, err
	}

	err := h.impl.ScheduleDecisionTask(ctx, args.ScheduleRequest)

	hadError := err != nil
	result, err := history.HistoryService_ScheduleDecisionTask_Helper.WrapResponse(err)

	var response thrift.Response
	if err == nil {
		response.IsApplicationError = hadError
		response.Body = result
	}
	return response, err
}

func (h handler) SignalWorkflowExecution(ctx context.Context, body wire.Value) (thrift.Response, error) {
	var args history.HistoryService_SignalWorkflowExecution_Args
	if err := args.FromWire(body); err != nil {
		return thrift.Response{}, err
	}

	err := h.impl.SignalWorkflowExecution(ctx, args.SignalRequest)

	hadError := err != nil
	result, err := history.HistoryService_SignalWorkflowExecution_Helper.WrapResponse(err)

	var response thrift.Response
	if err == nil {
		response.IsApplicationError = hadError
		response.Body = result
	}
	return response, err
}

func (h handler) StartWorkflowExecution(ctx context.Context, body wire.Value) (thrift.Response, error) {
	var args history.HistoryService_StartWorkflowExecution_Args
	if err := args.FromWire(body); err != nil {
		return thrift.Response{}, err
	}

	success, err := h.impl.StartWorkflowExecution(ctx, args.StartRequest)

	hadError := err != nil
	result, err := history.HistoryService_StartWorkflowExecution_Helper.WrapResponse(success, err)

	var response thrift.Response
	if err == nil {
		response.IsApplicationError = hadError
		response.Body = result
	}
	return response, err
}

func (h handler) TerminateWorkflowExecution(ctx context.Context, body wire.Value) (thrift.Response, error) {
	var args history.HistoryService_TerminateWorkflowExecution_Args
	if err := args.FromWire(body); err != nil {
		return thrift.Response{}, err
	}

	err := h.impl.TerminateWorkflowExecution(ctx, args.TerminateRequest)

	hadError := err != nil
	result, err := history.HistoryService_TerminateWorkflowExecution_Helper.WrapResponse(err)

	var response thrift.Response
	if err == nil {
		response.IsApplicationError = hadError
		response.Body = result
	}
	return response, err
}
