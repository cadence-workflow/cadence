package history

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/.gen/go/sqlblobs"
	historyclient "github.com/uber/cadence/client/history"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/codec"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/types"
)

const testNumHistoryShards = 16

type envelopeOpts struct {
	domain    *string
	requestID *string
}

func newEnvelope(t *testing.T, msgType sqlblobs.AsyncRequestType, workflowID string, opts envelopeOpts) *sqlblobs.AsyncRequestMessage {
	t.Helper()
	return &sqlblobs.AsyncRequestMessage{
		PartitionKey: common.StringPtr(workflowID),
		Type:         &msgType,
		Header:       &shared.Header{Fields: map[string][]byte{"k": []byte("v")}},
		Encoding:     common.StringPtr("thriftrw"),
		Payload:      []byte("opaque-inner-request"),
		Domain:       opts.domain,
		RequestId:    opts.requestID,
	}
}

func TestProducerPublish(t *testing.T) {
	tests := []struct {
		name           string
		message        interface{}
		setupClient    func(t *testing.T, m *historyclient.MockClient)
		wantErr        string
		wantBadRequest bool
	}{
		{
			name: "start request: shard from workflow id, identity from envelope, envelope forwarded intact",
			message: newEnvelope(t, sqlblobs.AsyncRequestTypeStartWorkflowExecutionAsyncRequest, "wf-start",
				envelopeOpts{domain: common.StringPtr("domain-a"), requestID: common.StringPtr("req-start")}),
			setupClient: func(t *testing.T, m *historyclient.MockClient) {
				m.EXPECT().EnqueueAsyncWorkflowMessage(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, req *types.EnqueueAsyncWorkflowMessageRequest, _ ...interface{}) (*types.EnqueueAsyncWorkflowMessageResponse, error) {
						assert.Equal(t, int32(common.WorkflowIDToHistoryShard("wf-start", testNumHistoryShards)), req.ShardID)
						assert.Equal(t, "domain-a", req.DomainName)
						assert.Equal(t, "wf-start", req.WorkflowID)
						assert.Equal(t, "req-start", req.RequestID)
						require.NotNil(t, req.RequestType)
						assert.Equal(t, types.AsyncWorkflowRequestTypeStartWorkflowExecution, *req.RequestType)
						require.NotNil(t, req.Payload)
						require.NotNil(t, req.Payload.EncodingType)
						assert.Equal(t, types.EncodingTypeThriftRW, *req.Payload.EncodingType)

						var envelope sqlblobs.AsyncRequestMessage
						require.NoError(t, codec.NewThriftRWEncoder().Decode(req.Payload.Data, &envelope))
						assert.Equal(t, "wf-start", envelope.GetPartitionKey())
						assert.Equal(t, "domain-a", envelope.GetDomain())
						assert.Equal(t, "req-start", envelope.GetRequestId())
						assert.Equal(t, []byte("v"), envelope.GetHeader().GetFields()["k"])
						assert.Equal(t, []byte("opaque-inner-request"), envelope.GetPayload())
						return &types.EnqueueAsyncWorkflowMessageResponse{MessageID: 1}, nil
					}).Times(1)
			},
		},
		{
			name: "signal-with-start request maps to the signal-with-start type",
			message: newEnvelope(t, sqlblobs.AsyncRequestTypeSignalWithStartWorkflowExecutionAsyncRequest, "wf-signal",
				envelopeOpts{domain: common.StringPtr("domain-b"), requestID: common.StringPtr("req-signal")}),
			setupClient: func(t *testing.T, m *historyclient.MockClient) {
				m.EXPECT().EnqueueAsyncWorkflowMessage(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, req *types.EnqueueAsyncWorkflowMessageRequest, _ ...interface{}) (*types.EnqueueAsyncWorkflowMessageResponse, error) {
						assert.Equal(t, int32(common.WorkflowIDToHistoryShard("wf-signal", testNumHistoryShards)), req.ShardID)
						assert.Equal(t, "domain-b", req.DomainName)
						assert.Equal(t, "req-signal", req.RequestID)
						require.NotNil(t, req.RequestType)
						assert.Equal(t, types.AsyncWorkflowRequestTypeSignalWithStartWorkflowExecution, *req.RequestType)
						return &types.EnqueueAsyncWorkflowMessageResponse{MessageID: 2}, nil
					}).Times(1)
			},
		},
		{
			name: "missing domain is rejected before any RPC",
			message: newEnvelope(t, sqlblobs.AsyncRequestTypeStartWorkflowExecutionAsyncRequest, "wf-nodomain",
				envelopeOpts{requestID: common.StringPtr("req-1")}),
			setupClient:    func(t *testing.T, m *historyclient.MockClient) {},
			wantErr:        "domain",
			wantBadRequest: true,
		},
		{
			name: "missing request id is rejected before any RPC",
			message: newEnvelope(t, sqlblobs.AsyncRequestTypeStartWorkflowExecutionAsyncRequest, "wf-noreq",
				envelopeOpts{domain: common.StringPtr("domain-a")}),
			setupClient:    func(t *testing.T, m *historyclient.MockClient) {},
			wantErr:        "requestId",
			wantBadRequest: true,
		},
		{
			name: "unknown request type is rejected before any RPC",
			message: newEnvelope(t, sqlblobs.AsyncRequestType(42), "wf-badtype",
				envelopeOpts{domain: common.StringPtr("domain-a"), requestID: common.StringPtr("req-1")}),
			setupClient:    func(t *testing.T, m *historyclient.MockClient) {},
			wantErr:        "unsupported async request type",
			wantBadRequest: true,
		},
		{
			name:        "wrong message type is rejected before any RPC",
			message:     "not an envelope",
			setupClient: func(t *testing.T, m *historyclient.MockClient) {},
			wantErr:     "unexpected message type",
		},
		{
			name: "history error is returned unchanged",
			message: newEnvelope(t, sqlblobs.AsyncRequestTypeStartWorkflowExecutionAsyncRequest, "wf-err",
				envelopeOpts{domain: common.StringPtr("domain-a"), requestID: common.StringPtr("req-err")}),
			setupClient: func(t *testing.T, m *historyclient.MockClient) {
				m.EXPECT().EnqueueAsyncWorkflowMessage(gomock.Any(), gomock.Any()).Return(nil, errors.New("shard moved")).Times(1)
			},
			wantErr: "shard moved",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			client := historyclient.NewMockClient(ctrl)
			tc.setupClient(t, client)

			p := newProducer(client, testNumHistoryShards, testlogger.New(t))
			err := p.Publish(context.Background(), tc.message)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				var badRequest *types.BadRequestError
				if tc.wantBadRequest {
					require.ErrorAs(t, err, &badRequest)
				} else {
					assert.NotErrorAs(t, err, &badRequest)
				}
				return
			}
			require.NoError(t, err)
		})
	}
}
