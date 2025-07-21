// Copyright (c) 2021 Uber Technologies Inc.
//
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

package proto

import (
	apiv1 "github.com/uber/cadence-idl/go/proto/api/v1"
	"go.uber.org/yarpc/encoding/protobuf"
	"go.uber.org/yarpc/yarpcerrors"

	sharddistributorv1 "github.com/uber/cadence/.gen/proto/sharddistributor/v1"
	sharedv1 "github.com/uber/cadence/.gen/proto/shared/v1"
	cadence_errors "github.com/uber/cadence/common/errors"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/errorutils"
)

func FromError(err error) error {
	if err == nil {
		return protobuf.NewError(yarpcerrors.CodeOK, "")
	}

	var (
		ok       bool
		typedErr error
	)
	if ok, typedErr = errorutils.ConvertError(err, fromAccessDeniedError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, fromInternalServiceError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, fromEntityNotExistsError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, fromWorkflowExecutionAlreadyCompletedError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, fromBadRequestError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, fromQueryFailedError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, fromShardOwnershipLostError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, fromTaskListNotOwnedByHostError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, fromCurrentBranchChangedError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, fromRetryTaskV2Error); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, fromCancellationAlreadyRequestedError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, fromDomainAlreadyExistsError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, fromEventAlreadyStartedError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, fromWorkflowExecutionAlreadyStartedError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, fromClientVersionNotSupportedErr); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, fromFeatureNotEnabledErr); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, fromDomainNotActive); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, fromInternalDataInconsistencyErr); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, fromLimitExceededErr); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, fromServiceBusyErr); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, fromRemoteSyncMatchedErr); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, fromStickyWorkerUnavailableErr); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, fromNamespaceNotFoundErr); ok {
		return typedErr
	}

	return protobuf.NewError(yarpcerrors.CodeUnknown, err.Error())
}

func ToError(err error) error {
	status := yarpcerrors.FromError(err)
	if status == nil || status.Code() == yarpcerrors.CodeOK {
		return nil
	}

	switch status.Code() {
	case yarpcerrors.CodePermissionDenied:
		return &types.AccessDeniedError{
			Message: status.Message(),
		}
	case yarpcerrors.CodeInternal:
		return &types.InternalServiceError{
			Message: status.Message(),
		}
	case yarpcerrors.CodeNotFound:
		switch details := getErrorDetails(err).(type) {
		case *apiv1.EntityNotExistsError:
			return &types.EntityNotExistsError{
				Message:        status.Message(),
				CurrentCluster: details.CurrentCluster,
				ActiveCluster:  details.ActiveCluster,
				ActiveClusters: details.ActiveClusters,
			}
		case *apiv1.WorkflowExecutionAlreadyCompletedError:
			return &types.WorkflowExecutionAlreadyCompletedError{
				Message: status.Message(),
			}
		case *sharddistributorv1.NamespaceNotFoundError:
			return &types.NamespaceNotFoundError{
				Namespace: details.Namespace,
			}
		}
	case yarpcerrors.CodeInvalidArgument:
		switch getErrorDetails(err).(type) {
		case nil:
			return &types.BadRequestError{
				Message: status.Message(),
			}
		case *apiv1.QueryFailedError:
			return &types.QueryFailedError{
				Message: status.Message(),
			}
		}
	case yarpcerrors.CodeAborted:
		switch details := getErrorDetails(err).(type) {
		case *sharedv1.ShardOwnershipLostError:
			return &types.ShardOwnershipLostError{
				Message: status.Message(),
				Owner:   details.Owner,
			}
		case *sharedv1.TaskListNotOwnedByHostError:
			return &cadence_errors.TaskListNotOwnedByHostError{
				OwnedByIdentity: details.OwnedByIdentity,
				MyIdentity:      details.MyIdentity,
				TasklistName:    details.TaskListName,
			}
		case *sharedv1.CurrentBranchChangedError:
			return &types.CurrentBranchChangedError{
				Message:            status.Message(),
				CurrentBranchToken: details.CurrentBranchToken,
			}
		case *sharedv1.RetryTaskV2Error:
			return &types.RetryTaskV2Error{
				Message:           status.Message(),
				DomainID:          details.DomainId,
				WorkflowID:        ToWorkflowID(details.WorkflowExecution),
				RunID:             ToRunID(details.WorkflowExecution),
				StartEventID:      ToEventID(details.StartEvent),
				StartEventVersion: ToEventVersion(details.StartEvent),
				EndEventID:        ToEventID(details.EndEvent),
				EndEventVersion:   ToEventVersion(details.EndEvent),
			}
		}
	case yarpcerrors.CodeAlreadyExists:
		switch details := getErrorDetails(err).(type) {
		case *apiv1.CancellationAlreadyRequestedError:
			return &types.CancellationAlreadyRequestedError{
				Message: status.Message(),
			}
		case *apiv1.DomainAlreadyExistsError:
			return &types.DomainAlreadyExistsError{
				Message: status.Message(),
			}
		case *sharedv1.EventAlreadyStartedError:
			return &types.EventAlreadyStartedError{
				Message: status.Message(),
			}
		case *apiv1.WorkflowExecutionAlreadyStartedError:
			return &types.WorkflowExecutionAlreadyStartedError{
				Message:        status.Message(),
				StartRequestID: details.StartRequestId,
				RunID:          details.RunId,
			}
		}
	case yarpcerrors.CodeDataLoss:
		return &types.InternalDataInconsistencyError{
			Message: status.Message(),
		}
	case yarpcerrors.CodeFailedPrecondition:
		switch details := getErrorDetails(err).(type) {
		case *apiv1.ClientVersionNotSupportedError:
			return &types.ClientVersionNotSupportedError{
				FeatureVersion:    details.FeatureVersion,
				ClientImpl:        details.ClientImpl,
				SupportedVersions: details.SupportedVersions,
			}
		case *apiv1.FeatureNotEnabledError:
			return &types.FeatureNotEnabledError{
				FeatureFlag: details.FeatureFlag,
			}
		case *apiv1.DomainNotActiveError:
			return &types.DomainNotActiveError{
				Message:        status.Message(),
				DomainName:     details.Domain,
				CurrentCluster: details.CurrentCluster,
				ActiveCluster:  details.ActiveCluster,
				ActiveClusters: details.ActiveClusters,
			}
		}
	case yarpcerrors.CodeResourceExhausted:
		switch details := getErrorDetails(err).(type) {
		case *apiv1.LimitExceededError:
			return &types.LimitExceededError{
				Message: status.Message(),
			}
		case *apiv1.ServiceBusyError:
			return &types.ServiceBusyError{
				Message: status.Message(),
				Reason:  details.Reason,
			}
		}
	case yarpcerrors.CodeUnavailable:
		switch getErrorDetails(err).(type) {
		case *sharedv1.RemoteSyncMatchedError:
			return &types.RemoteSyncMatchedError{
				Message: status.Message(),
			}
		case *apiv1.StickyWorkerUnavailableError:
			return &types.StickyWorkerUnavailableError{
				Message: status.Message(),
			}
		}
	}

	// If error does not match anything, return raw yarpc status error
	// There are some code that casts error to yarpc status to check for deadline exceeded status
	return status
}

func getErrorDetails(err error) interface{} {
	details := protobuf.GetErrorDetails(err)
	if len(details) > 0 {
		return details[0]
	}
	return nil
}

func fromAccessDeniedError(e *types.AccessDeniedError) error {
	return protobuf.NewError(yarpcerrors.CodePermissionDenied, e.Message)
}

func fromInternalServiceError(e *types.InternalServiceError) error {
	return protobuf.NewError(yarpcerrors.CodeInternal, e.Message)
}

func fromEntityNotExistsError(e *types.EntityNotExistsError) error {
	return protobuf.NewError(yarpcerrors.CodeNotFound, e.Message, protobuf.WithErrorDetails(&apiv1.EntityNotExistsError{
		CurrentCluster: e.CurrentCluster,
		ActiveCluster:  e.ActiveCluster,
		ActiveClusters: e.ActiveClusters,
	}))
}

func fromWorkflowExecutionAlreadyCompletedError(e *types.WorkflowExecutionAlreadyCompletedError) error {
	return protobuf.NewError(yarpcerrors.CodeNotFound, e.Message, protobuf.WithErrorDetails(&apiv1.WorkflowExecutionAlreadyCompletedError{}))
}

func fromBadRequestError(e *types.BadRequestError) error {
	return protobuf.NewError(yarpcerrors.CodeInvalidArgument, e.Message)
}

func fromQueryFailedError(e *types.QueryFailedError) error {
	return protobuf.NewError(yarpcerrors.CodeInvalidArgument, e.Message, protobuf.WithErrorDetails(&apiv1.QueryFailedError{}))
}

func fromShardOwnershipLostError(e *types.ShardOwnershipLostError) error {
	return protobuf.NewError(yarpcerrors.CodeAborted, e.Message, protobuf.WithErrorDetails(&sharedv1.ShardOwnershipLostError{
		Owner: e.Owner,
	}))
}

func fromTaskListNotOwnedByHostError(e *cadence_errors.TaskListNotOwnedByHostError) error {
	return protobuf.NewError(yarpcerrors.CodeAborted, e.Error(), protobuf.WithErrorDetails(&sharedv1.TaskListNotOwnedByHostError{
		OwnedByIdentity: e.OwnedByIdentity,
		MyIdentity:      e.MyIdentity,
		TaskListName:    e.TasklistName,
	}))
}

func fromCurrentBranchChangedError(e *types.CurrentBranchChangedError) error {
	return protobuf.NewError(yarpcerrors.CodeAborted, e.Message, protobuf.WithErrorDetails(&sharedv1.CurrentBranchChangedError{
		CurrentBranchToken: e.GetCurrentBranchToken(),
	}))
}

func fromRetryTaskV2Error(e *types.RetryTaskV2Error) error {
	return protobuf.NewError(yarpcerrors.CodeAborted, e.Message, protobuf.WithErrorDetails(&sharedv1.RetryTaskV2Error{
		DomainId:          e.DomainID,
		WorkflowExecution: FromWorkflowRunPair(e.WorkflowID, e.RunID),
		StartEvent:        FromEventIDVersionPair(e.StartEventID, e.StartEventVersion),
		EndEvent:          FromEventIDVersionPair(e.EndEventID, e.EndEventVersion),
	}))
}

func fromCancellationAlreadyRequestedError(e *types.CancellationAlreadyRequestedError) error {
	return protobuf.NewError(yarpcerrors.CodeAlreadyExists, e.Message, protobuf.WithErrorDetails(&apiv1.CancellationAlreadyRequestedError{}))
}

func fromDomainAlreadyExistsError(e *types.DomainAlreadyExistsError) error {
	return protobuf.NewError(yarpcerrors.CodeAlreadyExists, e.Message, protobuf.WithErrorDetails(&apiv1.DomainAlreadyExistsError{}))
}

func fromEventAlreadyStartedError(e *types.EventAlreadyStartedError) error {
	return protobuf.NewError(yarpcerrors.CodeAlreadyExists, e.Message, protobuf.WithErrorDetails(&sharedv1.EventAlreadyStartedError{}))
}

func fromWorkflowExecutionAlreadyStartedError(e *types.WorkflowExecutionAlreadyStartedError) error {
	return protobuf.NewError(yarpcerrors.CodeAlreadyExists, e.Message, protobuf.WithErrorDetails(&apiv1.WorkflowExecutionAlreadyStartedError{
		StartRequestId: e.StartRequestID,
		RunId:          e.RunID,
	}))
}

func fromClientVersionNotSupportedErr(e *types.ClientVersionNotSupportedError) error {
	return protobuf.NewError(yarpcerrors.CodeFailedPrecondition, "Client version not supported", protobuf.WithErrorDetails(&apiv1.ClientVersionNotSupportedError{
		FeatureVersion:    e.FeatureVersion,
		ClientImpl:        e.ClientImpl,
		SupportedVersions: e.SupportedVersions,
	}))
}

func fromFeatureNotEnabledErr(e *types.FeatureNotEnabledError) error {
	return protobuf.NewError(yarpcerrors.CodeFailedPrecondition, "Feature flag not enabled", protobuf.WithErrorDetails(&apiv1.FeatureNotEnabledError{
		FeatureFlag: e.FeatureFlag,
	}))
}

func fromDomainNotActive(e *types.DomainNotActiveError) error {
	return protobuf.NewError(yarpcerrors.CodeFailedPrecondition, e.Message, protobuf.WithErrorDetails(&apiv1.DomainNotActiveError{
		Domain:         e.DomainName,
		CurrentCluster: e.CurrentCluster,
		ActiveCluster:  e.ActiveCluster,
		ActiveClusters: e.ActiveClusters,
	}))
}

func fromInternalDataInconsistencyErr(e *types.InternalDataInconsistencyError) error {
	return protobuf.NewError(yarpcerrors.CodeDataLoss, e.Message, protobuf.WithErrorDetails(&sharedv1.InternalDataInconsistencyError{}))
}

func fromLimitExceededErr(e *types.LimitExceededError) error {
	return protobuf.NewError(yarpcerrors.CodeResourceExhausted, e.Message, protobuf.WithErrorDetails(&apiv1.LimitExceededError{}))
}

func fromServiceBusyErr(e *types.ServiceBusyError) error {
	return protobuf.NewError(yarpcerrors.CodeResourceExhausted, e.Message, protobuf.WithErrorDetails(&apiv1.ServiceBusyError{
		Reason: e.Reason,
	}))
}

func fromRemoteSyncMatchedErr(e *types.RemoteSyncMatchedError) error {
	return protobuf.NewError(yarpcerrors.CodeUnavailable, e.Message, protobuf.WithErrorDetails(&sharedv1.RemoteSyncMatchedError{}))
}

func fromStickyWorkerUnavailableErr(e *types.StickyWorkerUnavailableError) error {
	return protobuf.NewError(yarpcerrors.CodeUnavailable, e.Message, protobuf.WithErrorDetails(&apiv1.StickyWorkerUnavailableError{}))
}

func fromNamespaceNotFoundErr(e *types.NamespaceNotFoundError) error {
	return protobuf.NewError(yarpcerrors.CodeNotFound, e.Error(), protobuf.WithErrorDetails(&sharddistributorv1.NamespaceNotFoundError{
		Namespace: e.Namespace,
	}))
}
