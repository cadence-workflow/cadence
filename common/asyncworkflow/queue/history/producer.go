package history

import (
	"context"
	"fmt"

	"github.com/uber/cadence/.gen/go/sqlblobs"
	historyclient "github.com/uber/cadence/client/history"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/codec"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
)

type producerImpl struct {
	historyClient    historyclient.Client
	numHistoryShards int
	encoder          *codec.ThriftRWEncoder
	logger           log.Logger
}

func newProducer(historyClient historyclient.Client, numHistoryShards int, logger log.Logger) *producerImpl {
	return &producerImpl{
		historyClient:    historyClient,
		numHistoryShards: numHistoryShards,
		encoder:          codec.NewThriftRWEncoder(),
		logger:           logger,
	}
}

func (p *producerImpl) Publish(ctx context.Context, message interface{}) error {
	msg, ok := message.(*sqlblobs.AsyncRequestMessage)
	if !ok {
		return fmt.Errorf("unexpected message type %T, expected *sqlblobs.AsyncRequestMessage", message)
	}

	workflowID := msg.GetPartitionKey()
	if msg.GetDomain() == "" {
		return &types.BadRequestError{Message: "async request envelope is missing domain"}
	}
	if msg.GetRequestId() == "" {
		return &types.BadRequestError{Message: "async request envelope is missing requestId"}
	}

	requestType, err := asyncRequestType(msg.GetType())
	if err != nil {
		return err
	}
	payload, err := p.encoder.Encode(msg)
	if err != nil {
		return err
	}

	shardID := common.WorkflowIDToHistoryShard(workflowID, p.numHistoryShards)
	_, err = p.historyClient.EnqueueAsyncWorkflowMessage(ctx, &types.EnqueueAsyncWorkflowMessageRequest{
		ShardID:     int32(shardID),
		DomainName:  msg.GetDomain(),
		WorkflowID:  workflowID,
		RequestID:   msg.GetRequestId(),
		RequestType: requestType.Ptr(),
		Payload: &types.DataBlob{
			EncodingType: types.EncodingTypeThriftRW.Ptr(),
			Data:         payload,
		},
	})
	if err != nil {
		p.logger.Warn(
			"Failed to enqueue async workflow message",
			tag.ShardID(shardID),
			tag.WorkflowID(workflowID),
			tag.Error(err),
		)
		return err
	}
	return nil
}

func asyncRequestType(t sqlblobs.AsyncRequestType) (types.AsyncWorkflowRequestType, error) {
	switch t {
	case sqlblobs.AsyncRequestTypeStartWorkflowExecutionAsyncRequest:
		return types.AsyncWorkflowRequestTypeStartWorkflowExecution, nil
	case sqlblobs.AsyncRequestTypeSignalWithStartWorkflowExecutionAsyncRequest:
		return types.AsyncWorkflowRequestTypeSignalWithStartWorkflowExecution, nil
	default:
		return 0, &types.BadRequestError{Message: fmt.Sprintf("unsupported async request type %v", t)}
	}
}
