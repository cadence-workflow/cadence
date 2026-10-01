package api

import (
	"context"
	"testing"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/.gen/go/sqlblobs"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/client"
	dc "github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/types"
	frontendcfg "github.com/uber/cadence/service/frontend/config"
)

func TestAsyncHandlersPublishEnvelopeIdentity(t *testing.T) {
	const (
		domainName = "test-domain"
		workflowID = "test-workflow-id"
	)
	requestID := uuid.New()

	testCases := []struct {
		name     string
		wantType sqlblobs.AsyncRequestType
		call     func(wh *WorkflowHandler) error
	}{
		{
			name:     "StartWorkflowExecutionAsync",
			wantType: sqlblobs.AsyncRequestTypeStartWorkflowExecutionAsyncRequest,
			call: func(wh *WorkflowHandler) error {
				_, err := wh.StartWorkflowExecutionAsync(context.Background(), &types.StartWorkflowExecutionAsyncRequest{
					StartWorkflowExecutionRequest: &types.StartWorkflowExecutionRequest{
						Domain:                              domainName,
						WorkflowID:                          workflowID,
						WorkflowType:                        &types.WorkflowType{Name: "test-workflow-type"},
						TaskList:                            &types.TaskList{Name: "test-task-list"},
						Input:                               []byte("test-input"),
						ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(60),
						TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
						Identity:                            "test-identity",
						RequestID:                           requestID,
					},
				})
				return err
			},
		},
		{
			name:     "SignalWithStartWorkflowExecutionAsync",
			wantType: sqlblobs.AsyncRequestTypeSignalWithStartWorkflowExecutionAsyncRequest,
			call: func(wh *WorkflowHandler) error {
				_, err := wh.SignalWithStartWorkflowExecutionAsync(context.Background(), &types.SignalWithStartWorkflowExecutionAsyncRequest{
					SignalWithStartWorkflowExecutionRequest: &types.SignalWithStartWorkflowExecutionRequest{
						Domain:                              domainName,
						WorkflowID:                          workflowID,
						WorkflowType:                        &types.WorkflowType{Name: "test-workflow-type"},
						TaskList:                            &types.TaskList{Name: "test-task-list"},
						Input:                               []byte("test-input"),
						ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(60),
						TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
						Identity:                            "test-identity",
						RequestID:                           requestID,
						SignalName:                          "test-signal-name",
					},
				})
				return err
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockResource := resource.NewTest(t, mockCtrl, metrics.Frontend)
			mockResource.DomainCache.EXPECT().GetDomainID(gomock.Any()).Return("test-domain-id", nil)
			mockVersionChecker := client.NewMockVersionChecker(mockCtrl)
			mockProducerManager := NewMockProducerManager(mockCtrl)
			mockProducer := messaging.NewMockProducer(mockCtrl)

			cfg := frontendcfg.NewConfig(
				dc.NewCollection(
					dc.NewInMemoryClient(),
					mockResource.GetLogger(),
				),
				numHistoryShards,
				false,
				"hostname",
				mockResource.GetLogger(),
			)
			wh := NewWorkflowHandler(mockResource, cfg, mockVersionChecker, nil)
			wh.producerManager = mockProducerManager

			var published []*sqlblobs.AsyncRequestMessage
			mockProducerManager.EXPECT().GetProducerByDomain(domainName).Return(mockProducer, nil)
			mockProducer.EXPECT().Publish(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, message interface{}) error {
					msg, ok := message.(*sqlblobs.AsyncRequestMessage)
					require.True(t, ok, "published message must be *sqlblobs.AsyncRequestMessage, got %T", message)
					published = append(published, msg)
					return nil
				},
			)

			require.NoError(t, tc.call(wh))

			require.Len(t, published, 1)
			msg := published[0]
			assert.Equal(t, domainName, msg.GetDomain())
			assert.Equal(t, requestID, msg.GetRequestId())
			assert.Equal(t, workflowID, msg.GetPartitionKey())
			assert.Equal(t, tc.wantType, msg.GetType())
			assert.NotEmpty(t, msg.GetPayload())
		})
	}
}
