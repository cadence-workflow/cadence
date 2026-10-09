package engineimpl

import (
	"context"
	"fmt"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

// EnqueueAsyncWorkflowMessage writes one accepted async workflow request onto this shard's queue.
// The shard context allocates the message id and writes the row under the shard lock, so message ids
// become visible to the consumer in allocation order.
func (e *historyEngineImpl) EnqueueAsyncWorkflowMessage(
	ctx context.Context,
	request *types.EnqueueAsyncWorkflowMessageRequest,
) (*types.EnqueueAsyncWorkflowMessageResponse, error) {
	shardID := e.shard.GetShardID()
	if int(request.GetShardID()) != shardID {
		return nil, &types.BadRequestError{Message: fmt.Sprintf("request shard %d does not match shard %d", request.GetShardID(), shardID)}
	}
	if expected := common.WorkflowIDToHistoryShard(request.GetWorkflowID(), e.config.NumberOfShards); expected != shardID {
		return nil, &types.BadRequestError{Message: fmt.Sprintf("workflow %q belongs to shard %d, not shard %d", request.GetWorkflowID(), expected, shardID)}
	}

	payload := request.GetPayload()
	if payload == nil || len(payload.Data) == 0 {
		return nil, &types.BadRequestError{Message: "payload is required"}
	}
	payloadEncoding, err := asyncWorkflowPayloadEncoding(payload.GetEncodingType())
	if err != nil {
		return nil, err
	}
	requestType, err := asyncWorkflowRequestType(request.RequestType)
	if err != nil {
		return nil, err
	}

	enqueueRequest := &persistence.EnqueueAsyncWorkflowMessageRequest{
		ShardID:         shardID,
		SourceCluster:   e.shard.GetClusterMetadata().GetCurrentClusterName(),
		DomainName:      request.GetDomainName(),
		WorkflowID:      request.GetWorkflowID(),
		RequestID:       request.GetRequestID(),
		RequestType:     requestType,
		Payload:         payload.Data,
		PayloadEncoding: payloadEncoding,
	}
	messageID, err := e.shard.EnqueueAsyncWorkflowMessage(ctx, enqueueRequest)
	if err != nil {
		return nil, err
	}

	return &types.EnqueueAsyncWorkflowMessageResponse{MessageID: messageID}, nil
}

func asyncWorkflowPayloadEncoding(encoding types.EncodingType) (constants.EncodingType, error) {
	switch encoding {
	case types.EncodingTypeThriftRW:
		return constants.EncodingTypeThriftRW, nil
	case types.EncodingTypeJSON:
		return constants.EncodingTypeJSON, nil
	default:
		return "", &types.BadRequestError{Message: fmt.Sprintf("unsupported payload encoding: %s", encoding.String())}
	}
}

func asyncWorkflowRequestType(requestType *types.AsyncWorkflowRequestType) (persistence.AsyncWorkflowRequestType, error) {
	if requestType == nil {
		return 0, &types.BadRequestError{Message: "request type is required"}
	}

	switch *requestType {
	case types.AsyncWorkflowRequestTypeStartWorkflowExecution:
		return persistence.AsyncWorkflowRequestTypeStartWorkflow, nil
	case types.AsyncWorkflowRequestTypeSignalWithStartWorkflowExecution:
		return persistence.AsyncWorkflowRequestTypeSignalWithStartWorkflow, nil
	default:
		return 0, &types.BadRequestError{Message: fmt.Sprintf("unsupported request type: %s", requestType.String())}
	}
}
