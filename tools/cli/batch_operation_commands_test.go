package cli

import (
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/batch"
	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common/types"
)

func newBatchOperationCLIContext(app *cli.App, flags map[string]string) *cli.Context {
	set := flag.NewFlagSet("test", 0)
	set.String(FlagDomain, "test-domain", "")
	set.String(FlagTransport, grpcTransport, "")
	set.Int(FlagRPS, 50, "")
	set.Int(FlagConcurrency, 5, "")
	set.Int(FlagPageSize, 1000, "")
	set.Bool(FlagYes, true, "")
	for k, v := range flags {
		set.String(k, v, k)
	}
	return cli.NewContext(app, set, nil)
}

func TestBatchOperationCLI_StartTerminate(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockFrontendClient := frontend.NewMockClient(mockCtrl)
	mockBatchClient := batch.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{serverFrontendClient: mockFrontendClient, serverBatchClient: mockBatchClient})

	mockFrontendClient.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).
		Return(&types.CountWorkflowExecutionsResponse{Count: 3}, nil)
	mockBatchClient.EXPECT().StartBatchOperation(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ interface{}, req *types.StartBatchOperationRequest, _ ...interface{}) (*types.StartBatchOperationResponse, error) {
			assert.Equal(t, "test-domain", req.Domain)
			assert.Equal(t, "TaskList = 'buggy'", req.Query)
			assert.Equal(t, "cleanup", req.Reason)
			require.NotNil(t, req.Operation)
			assert.Equal(t, types.BatchOperationTypeTerminate, req.Operation.GetOperationType())
			assert.NotEmpty(t, req.BatchID)
			return &types.StartBatchOperationResponse{BatchID: req.BatchID}, nil
		})

	c := newBatchOperationCLIContext(app, map[string]string{
		FlagListQuery: "TaskList = 'buggy'",
		FlagReason:    "cleanup",
	})
	err := StartBatchTerminate(c)
	assert.NoError(t, err)
}

func TestBatchOperationCLI_StartSignal(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockFrontendClient := frontend.NewMockClient(mockCtrl)
	mockBatchClient := batch.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{serverFrontendClient: mockFrontendClient, serverBatchClient: mockBatchClient})

	mockFrontendClient.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).
		Return(&types.CountWorkflowExecutionsResponse{Count: 1}, nil)
	mockBatchClient.EXPECT().StartBatchOperation(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ interface{}, req *types.StartBatchOperationRequest, _ ...interface{}) (*types.StartBatchOperationResponse, error) {
			require.NotNil(t, req.Operation)
			assert.Equal(t, types.BatchOperationTypeSignal, req.Operation.GetOperationType())
			assert.Equal(t, "approval", req.Operation.GetSignalBatchOperation().GetSignalName())
			assert.Equal(t, "yes", string(req.Operation.GetSignalBatchOperation().GetInput()))
			return &types.StartBatchOperationResponse{BatchID: req.BatchID}, nil
		})

	c := newBatchOperationCLIContext(app, map[string]string{
		FlagListQuery:  "UserID = 5",
		FlagReason:     "approve",
		FlagSignalName: "approval",
		FlagInput:      "yes",
	})
	err := StartBatchSignal(c)
	assert.NoError(t, err)
}

func TestBatchOperationCLI_StartSignal_MissingSignalName(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockFrontendClient := frontend.NewMockClient(mockCtrl)
	mockBatchClient := batch.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{serverFrontendClient: mockFrontendClient, serverBatchClient: mockBatchClient})

	c := newBatchOperationCLIContext(app, map[string]string{
		FlagListQuery: "UserID = 5",
		FlagReason:    "approve",
	})
	err := StartBatchSignal(c)
	assert.Error(t, err)
}

func TestBatchOperationCLI_StartReplicate(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockFrontendClient := frontend.NewMockClient(mockCtrl)
	mockBatchClient := batch.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{serverFrontendClient: mockFrontendClient, serverBatchClient: mockBatchClient})

	mockFrontendClient.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).
		Return(&types.CountWorkflowExecutionsResponse{Count: 2}, nil)
	mockBatchClient.EXPECT().StartBatchOperation(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ interface{}, req *types.StartBatchOperationRequest, _ ...interface{}) (*types.StartBatchOperationResponse, error) {
			require.NotNil(t, req.Operation)
			assert.Equal(t, types.BatchOperationTypeReplicate, req.Operation.GetOperationType())
			assert.Equal(t, "cluster-a", req.Operation.GetReplicateBatchOperation().GetSourceCluster())
			assert.Equal(t, "cluster-b", req.Operation.GetReplicateBatchOperation().GetTargetCluster())
			return &types.StartBatchOperationResponse{BatchID: req.BatchID}, nil
		})

	c := newBatchOperationCLIContext(app, map[string]string{
		FlagListQuery:     "WorkflowType = 'buggy'",
		FlagReason:        "resend",
		FlagSourceCluster: "cluster-a",
		FlagTargetCluster: "cluster-b",
	})
	err := StartBatchReplicate(c)
	assert.NoError(t, err)
}

func TestBatchOperationCLI_StartTerminate_UsesProvidedBatchID(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockFrontendClient := frontend.NewMockClient(mockCtrl)
	mockBatchClient := batch.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{serverFrontendClient: mockFrontendClient, serverBatchClient: mockBatchClient})

	mockFrontendClient.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).
		Return(&types.CountWorkflowExecutionsResponse{Count: 1}, nil)
	mockBatchClient.EXPECT().StartBatchOperation(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ interface{}, req *types.StartBatchOperationRequest, _ ...interface{}) (*types.StartBatchOperationResponse, error) {
			assert.Equal(t, "my-batch-id", req.BatchID)
			return &types.StartBatchOperationResponse{BatchID: req.BatchID}, nil
		})

	c := newBatchOperationCLIContext(app, map[string]string{
		FlagListQuery: "TaskList = 'buggy'",
		FlagReason:    "cleanup",
		FlagBatchID:   "my-batch-id",
	})
	err := StartBatchTerminate(c)
	assert.NoError(t, err)
}
