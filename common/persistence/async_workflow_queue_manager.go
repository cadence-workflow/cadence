package persistence

import (
	"context"
	"fmt"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/types"
)

type asyncWorkflowQueueManagerImpl struct {
	persistence AsyncWorkflowQueueStore
	logger      log.Logger
	timeSrc     clock.TimeSource
}

// NewAsyncWorkflowQueueManager returns a new AsyncWorkflowQueueManager
func NewAsyncWorkflowQueueManager(persistence AsyncWorkflowQueueStore, logger log.Logger) AsyncWorkflowQueueManager {
	return &asyncWorkflowQueueManagerImpl{
		persistence: persistence,
		logger:      logger,
		timeSrc:     clock.NewRealTimeSource(),
	}
}

func (m *asyncWorkflowQueueManagerImpl) GetName() string {
	return m.persistence.GetName()
}

func (m *asyncWorkflowQueueManagerImpl) Close() {
	m.persistence.Close()
}

func (m *asyncWorkflowQueueManagerImpl) EnqueueAsyncWorkflowMessage(
	ctx context.Context,
	request *EnqueueAsyncWorkflowMessageRequest,
) error {
	if err := validateAsyncWorkflowMessageFields(
		request.ShardID, request.SourceCluster, request.MessageID,
		request.DomainName, request.WorkflowID, request.RequestID,
		request.RequestType, request.Payload, request.PayloadEncoding,
	); err != nil {
		return err
	}
	return m.persistence.EnqueueAsyncWorkflowMessage(ctx, &InternalEnqueueAsyncWorkflowMessageRequest{
		EnqueueAsyncWorkflowMessageRequest: request,
		CurrentTimeStamp:                   m.timeSrc.Now().UTC(),
	})
}

func (m *asyncWorkflowQueueManagerImpl) ReadAsyncWorkflowMessages(
	ctx context.Context,
	request *ReadAsyncWorkflowMessagesRequest,
) (*ReadAsyncWorkflowMessagesResponse, error) {
	if err := validateAsyncWorkflowRead(request.ShardID, request.SourceCluster, request.PageSize); err != nil {
		return nil, err
	}
	return m.persistence.ReadAsyncWorkflowMessages(ctx, request)
}

func (m *asyncWorkflowQueueManagerImpl) GetAsyncWorkflowAckLevels(
	ctx context.Context,
	request *GetAsyncWorkflowAckLevelsRequest,
) (*GetAsyncWorkflowAckLevelsResponse, error) {
	if err := validateAsyncWorkflowShardID(request.ShardID); err != nil {
		return nil, err
	}
	return m.persistence.GetAsyncWorkflowAckLevels(ctx, request)
}

func (m *asyncWorkflowQueueManagerImpl) UpdateAsyncWorkflowAckLevel(
	ctx context.Context,
	request *UpdateAsyncWorkflowAckLevelRequest,
) error {
	if err := validateAsyncWorkflowKey(request.ShardID, request.SourceCluster); err != nil {
		return err
	}
	if request.AckLevel < 0 {
		return asyncWorkflowBadRequest("AckLevel must be non-negative, got %d", request.AckLevel)
	}
	return m.persistence.UpdateAsyncWorkflowAckLevel(ctx, &InternalUpdateAsyncWorkflowAckLevelRequest{
		UpdateAsyncWorkflowAckLevelRequest: request,
		CurrentTimeStamp:                   m.timeSrc.Now().UTC(),
	})
}

func (m *asyncWorkflowQueueManagerImpl) RangeDeleteAsyncWorkflowMessages(
	ctx context.Context,
	request *RangeDeleteAsyncWorkflowMessagesRequest,
) error {
	if err := validateAsyncWorkflowRangeDelete(request.ShardID, request.SourceCluster, request.InclusiveMaxMessageID); err != nil {
		return err
	}
	return m.persistence.RangeDeleteAsyncWorkflowMessages(ctx, request)
}

func (m *asyncWorkflowQueueManagerImpl) EnqueueAsyncWorkflowMessageToDLQ(
	ctx context.Context,
	request *EnqueueAsyncWorkflowMessageToDLQRequest,
) error {
	if request.Message == nil {
		return asyncWorkflowBadRequest("Message must not be nil")
	}
	if request.Reason == "" {
		return asyncWorkflowBadRequest("Reason must not be empty")
	}
	msg := request.Message
	if err := validateAsyncWorkflowMessageFields(
		msg.ShardID, msg.SourceCluster, msg.MessageID,
		msg.DomainName, msg.WorkflowID, msg.RequestID,
		msg.RequestType, msg.Payload, msg.PayloadEncoding,
	); err != nil {
		return err
	}
	return m.persistence.EnqueueAsyncWorkflowMessageToDLQ(ctx, &InternalEnqueueAsyncWorkflowMessageToDLQRequest{
		EnqueueAsyncWorkflowMessageToDLQRequest: request,
		CurrentTimeStamp:                        m.timeSrc.Now().UTC(),
	})
}

func (m *asyncWorkflowQueueManagerImpl) ReadAsyncWorkflowMessagesFromDLQ(
	ctx context.Context,
	request *ReadAsyncWorkflowMessagesFromDLQRequest,
) (*ReadAsyncWorkflowMessagesFromDLQResponse, error) {
	if err := validateAsyncWorkflowRead(request.ShardID, request.SourceCluster, request.PageSize); err != nil {
		return nil, err
	}
	return m.persistence.ReadAsyncWorkflowMessagesFromDLQ(ctx, request)
}

func (m *asyncWorkflowQueueManagerImpl) RangeDeleteAsyncWorkflowMessagesFromDLQ(
	ctx context.Context,
	request *RangeDeleteAsyncWorkflowMessagesFromDLQRequest,
) error {
	if err := validateAsyncWorkflowRangeDelete(request.ShardID, request.SourceCluster, request.InclusiveMaxMessageID); err != nil {
		return err
	}
	return m.persistence.RangeDeleteAsyncWorkflowMessagesFromDLQ(ctx, request)
}

func asyncWorkflowBadRequest(format string, args ...interface{}) error {
	return &types.BadRequestError{Message: fmt.Sprintf(format, args...)}
}

func validateAsyncWorkflowShardID(shardID int) error {
	if shardID < 0 {
		return asyncWorkflowBadRequest("ShardID must be non-negative, got %d", shardID)
	}
	return nil
}

func validateAsyncWorkflowKey(shardID int, sourceCluster string) error {
	if err := validateAsyncWorkflowShardID(shardID); err != nil {
		return err
	}
	if sourceCluster == "" {
		return asyncWorkflowBadRequest("SourceCluster must not be empty")
	}
	return nil
}

func validateAsyncWorkflowRead(shardID int, sourceCluster string, pageSize int) error {
	if err := validateAsyncWorkflowKey(shardID, sourceCluster); err != nil {
		return err
	}
	if pageSize <= 0 {
		return asyncWorkflowBadRequest("PageSize must be positive, got %d", pageSize)
	}
	return nil
}

func validateAsyncWorkflowRangeDelete(shardID int, sourceCluster string, inclusiveMaxMessageID int64) error {
	if err := validateAsyncWorkflowKey(shardID, sourceCluster); err != nil {
		return err
	}
	if inclusiveMaxMessageID < 0 {
		return asyncWorkflowBadRequest("InclusiveMaxMessageID must be non-negative, got %d", inclusiveMaxMessageID)
	}
	return nil
}

// validateAsyncWorkflowMessageFields is shared by the enqueue and DLQ-enqueue paths so they cannot drift.
func validateAsyncWorkflowMessageFields(
	shardID int,
	sourceCluster string,
	messageID int64,
	domainName, workflowID, requestID string,
	requestType AsyncWorkflowRequestType,
	payload []byte,
	payloadEncoding constants.EncodingType,
) error {
	if err := validateAsyncWorkflowKey(shardID, sourceCluster); err != nil {
		return err
	}
	if messageID <= 0 {
		return asyncWorkflowBadRequest("MessageID must be positive, got %d", messageID)
	}
	if domainName == "" {
		return asyncWorkflowBadRequest("DomainName must not be empty")
	}
	if workflowID == "" {
		return asyncWorkflowBadRequest("WorkflowID must not be empty")
	}
	if requestID == "" {
		return asyncWorkflowBadRequest("RequestID must not be empty")
	}
	if len(payload) == 0 {
		return asyncWorkflowBadRequest("Payload must not be empty")
	}
	if payloadEncoding == "" {
		return asyncWorkflowBadRequest("PayloadEncoding must not be empty")
	}
	switch requestType {
	case AsyncWorkflowRequestTypeStartWorkflow, AsyncWorkflowRequestTypeSignalWithStartWorkflow:
	default:
		return asyncWorkflowBadRequest("RequestType %d is not a valid async workflow request type", requestType)
	}
	return nil
}
