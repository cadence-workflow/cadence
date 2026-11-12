package cli

import (
	"flag"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/batch"
	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common/types"
)

func newBatchOperationCLIContext(app *cli.App, overrides map[string]string) *cli.Context {
	set := flag.NewFlagSet("test", 0)

	domain := "test-domain"
	if v, ok := overrides[FlagDomain]; ok {
		domain = v
	}
	set.String(FlagDomain, domain, "")

	set.String(FlagTransport, grpcTransport, "")
	if v, ok := overrides[FlagTransport]; ok {
		set.Set(FlagTransport, v)
	}

	yes := true
	if v, ok := overrides[FlagYes]; ok {
		yes = v == "true"
	}
	set.Bool(FlagYes, yes, "")

	set.Int(FlagRPS, 50, "")
	set.Int(FlagConcurrency, 5, "")
	set.Int(FlagPageSize, 1000, "")

	handled := map[string]bool{FlagDomain: true, FlagTransport: true, FlagYes: true}
	for k, v := range overrides {
		if handled[k] {
			continue
		}
		set.String(k, v, "")
	}
	return cli.NewContext(app, set, nil)
}

func TestStartBatchOperationCLI(t *testing.T) {
	tests := map[string]struct {
		action   func(*cli.Context) error
		flags    map[string]string
		input    string // simulates stdin for the confirmation prompt; defaults to "" (FlagYes true skips it)
		mockFn   func(*frontend.MockClient, *batch.MockClient, **types.StartBatchOperationRequest)
		wantErr  bool
		validate func(*testing.T, *types.StartBatchOperationRequest)
	}{
		"missing domain": {
			action:  StartBatchTerminate,
			flags:   map[string]string{FlagDomain: "", FlagListQuery: "q", FlagReason: "r"},
			mockFn:  func(f *frontend.MockClient, b *batch.MockClient, captured **types.StartBatchOperationRequest) {},
			wantErr: true,
		},
		"missing query": {
			action:  StartBatchTerminate,
			flags:   map[string]string{FlagListQuery: "", FlagReason: "r"},
			mockFn:  func(f *frontend.MockClient, b *batch.MockClient, captured **types.StartBatchOperationRequest) {},
			wantErr: true,
		},
		"missing reason": {
			action:  StartBatchTerminate,
			flags:   map[string]string{FlagListQuery: "q", FlagReason: ""},
			mockFn:  func(f *frontend.MockClient, b *batch.MockClient, captured **types.StartBatchOperationRequest) {},
			wantErr: true,
		},
		"signal missing signal name": {
			action: StartBatchSignal,
			flags: map[string]string{
				FlagListQuery: "UserID = 5",
				FlagReason:    "approve",
			},
			mockFn:  func(f *frontend.MockClient, b *batch.MockClient, captured **types.StartBatchOperationRequest) {},
			wantErr: true,
		},
		"replicate missing source cluster": {
			action: StartBatchReplicate,
			flags: map[string]string{
				FlagListQuery:     "WorkflowType = 'buggy'",
				FlagReason:        "resend",
				FlagTargetCluster: "cluster-b",
			},
			mockFn:  func(f *frontend.MockClient, b *batch.MockClient, captured **types.StartBatchOperationRequest) {},
			wantErr: true,
		},
		"replicate missing target cluster": {
			action: StartBatchReplicate,
			flags: map[string]string{
				FlagListQuery:     "WorkflowType = 'buggy'",
				FlagReason:        "resend",
				FlagSourceCluster: "cluster-a",
			},
			mockFn:  func(f *frontend.MockClient, b *batch.MockClient, captured **types.StartBatchOperationRequest) {},
			wantErr: true,
		},
		"non-grpc transport is rejected": {
			action: StartBatchTerminate,
			flags: map[string]string{
				FlagListQuery: "q",
				FlagReason:    "r",
				FlagTransport: "tchannel",
			},
			mockFn:  func(f *frontend.MockClient, b *batch.MockClient, captured **types.StartBatchOperationRequest) {},
			wantErr: true,
		},
		"count workflows fails": {
			action: StartBatchTerminate,
			flags:  map[string]string{FlagListQuery: "q", FlagReason: "r"},
			mockFn: func(f *frontend.MockClient, b *batch.MockClient, captured **types.StartBatchOperationRequest) {
				f.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).
					Return(nil, assert.AnError)
			},
			wantErr: true,
		},
		"declining the confirmation prompt skips starting the batch": {
			action: StartBatchTerminate,
			flags: map[string]string{
				FlagListQuery: "q",
				FlagReason:    "r",
				FlagYes:       "false",
			},
			input: "no\n",
			mockFn: func(f *frontend.MockClient, b *batch.MockClient, captured **types.StartBatchOperationRequest) {
				f.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).
					Return(&types.CountWorkflowExecutionsResponse{Count: 3}, nil)
			},
			wantErr: false,
		},
		"start batch operation fails": {
			action: StartBatchTerminate,
			flags:  map[string]string{FlagListQuery: "q", FlagReason: "r"},
			mockFn: func(f *frontend.MockClient, b *batch.MockClient, captured **types.StartBatchOperationRequest) {
				f.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).
					Return(&types.CountWorkflowExecutionsResponse{Count: 3}, nil)
				b.EXPECT().StartBatchOperation(gomock.Any(), gomock.Any()).
					Return(nil, assert.AnError)
			},
			wantErr: true,
		},
		"terminate success": {
			action: StartBatchTerminate,
			flags: map[string]string{
				FlagListQuery: "TaskList = 'buggy'",
				FlagReason:    "cleanup",
			},
			mockFn: func(f *frontend.MockClient, b *batch.MockClient, captured **types.StartBatchOperationRequest) {
				f.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).
					Return(&types.CountWorkflowExecutionsResponse{Count: 3}, nil)
				b.EXPECT().StartBatchOperation(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ any, req *types.StartBatchOperationRequest, _ ...any) (*types.StartBatchOperationResponse, error) {
						*captured = req
						return &types.StartBatchOperationResponse{BatchID: req.BatchID}, nil
					})
			},
			wantErr: false,
			validate: func(t *testing.T, req *types.StartBatchOperationRequest) {
				assert.Equal(t, "test-domain", req.Domain)
				assert.Equal(t, "TaskList = 'buggy'", req.Query)
				assert.Equal(t, "cleanup", req.Reason)
				require.NotNil(t, req.Operation)
				assert.Equal(t, types.BatchOperationTypeTerminate, req.Operation.GetOperationType())
				assert.NotEmpty(t, req.BatchID)
			},
		},
		"terminate uses the provided batch ID": {
			action: StartBatchTerminate,
			flags: map[string]string{
				FlagListQuery: "TaskList = 'buggy'",
				FlagReason:    "cleanup",
				FlagBatchID:   "my-batch-id",
			},
			mockFn: func(f *frontend.MockClient, b *batch.MockClient, captured **types.StartBatchOperationRequest) {
				f.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).
					Return(&types.CountWorkflowExecutionsResponse{Count: 1}, nil)
				b.EXPECT().StartBatchOperation(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ any, req *types.StartBatchOperationRequest, _ ...any) (*types.StartBatchOperationResponse, error) {
						*captured = req
						return &types.StartBatchOperationResponse{BatchID: req.BatchID}, nil
					})
			},
			wantErr: false,
			validate: func(t *testing.T, req *types.StartBatchOperationRequest) {
				assert.Equal(t, "my-batch-id", req.BatchID)
			},
		},
		"cancel success": {
			action: StartBatchCancel,
			flags: map[string]string{
				FlagListQuery: "TaskList = 'buggy'",
				FlagReason:    "cleanup",
			},
			mockFn: func(f *frontend.MockClient, b *batch.MockClient, captured **types.StartBatchOperationRequest) {
				f.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).
					Return(&types.CountWorkflowExecutionsResponse{Count: 1}, nil)
				b.EXPECT().StartBatchOperation(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ any, req *types.StartBatchOperationRequest, _ ...any) (*types.StartBatchOperationResponse, error) {
						*captured = req
						return &types.StartBatchOperationResponse{BatchID: req.BatchID}, nil
					})
			},
			wantErr: false,
			validate: func(t *testing.T, req *types.StartBatchOperationRequest) {
				require.NotNil(t, req.Operation)
				assert.Equal(t, types.BatchOperationTypeCancel, req.Operation.GetOperationType())
			},
		},
		"signal success": {
			action: StartBatchSignal,
			flags: map[string]string{
				FlagListQuery:  "UserID = 5",
				FlagReason:     "approve",
				FlagSignalName: "approval",
				FlagInput:      "yes",
			},
			mockFn: func(f *frontend.MockClient, b *batch.MockClient, captured **types.StartBatchOperationRequest) {
				f.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).
					Return(&types.CountWorkflowExecutionsResponse{Count: 1}, nil)
				b.EXPECT().StartBatchOperation(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ any, req *types.StartBatchOperationRequest, _ ...any) (*types.StartBatchOperationResponse, error) {
						*captured = req
						return &types.StartBatchOperationResponse{BatchID: req.BatchID}, nil
					})
			},
			wantErr: false,
			validate: func(t *testing.T, req *types.StartBatchOperationRequest) {
				require.NotNil(t, req.Operation)
				assert.Equal(t, types.BatchOperationTypeSignal, req.Operation.GetOperationType())
				assert.Equal(t, "approval", req.Operation.GetSignalBatchOperation().GetSignalName())
				assert.Equal(t, "yes", string(req.Operation.GetSignalBatchOperation().GetInput()))
			},
		},
		"replicate success": {
			action: StartBatchReplicate,
			flags: map[string]string{
				FlagListQuery:     "WorkflowType = 'buggy'",
				FlagReason:        "resend",
				FlagSourceCluster: "cluster-a",
				FlagTargetCluster: "cluster-b",
			},
			mockFn: func(f *frontend.MockClient, b *batch.MockClient, captured **types.StartBatchOperationRequest) {
				f.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).
					Return(&types.CountWorkflowExecutionsResponse{Count: 2}, nil)
				b.EXPECT().StartBatchOperation(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ any, req *types.StartBatchOperationRequest, _ ...any) (*types.StartBatchOperationResponse, error) {
						*captured = req
						return &types.StartBatchOperationResponse{BatchID: req.BatchID}, nil
					})
			},
			wantErr: false,
			validate: func(t *testing.T, req *types.StartBatchOperationRequest) {
				require.NotNil(t, req.Operation)
				assert.Equal(t, types.BatchOperationTypeReplicate, req.Operation.GetOperationType())
				assert.Equal(t, "cluster-a", req.Operation.GetReplicateBatchOperation().GetSourceCluster())
				assert.Equal(t, "cluster-b", req.Operation.GetReplicateBatchOperation().GetTargetCluster())
			},
		},
		"refresh success": {
			action: StartBatchRefresh,
			flags: map[string]string{
				FlagListQuery: "TaskList = 'buggy'",
				FlagReason:    "refresh tasks",
			},
			mockFn: func(f *frontend.MockClient, b *batch.MockClient, captured **types.StartBatchOperationRequest) {
				f.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).
					Return(&types.CountWorkflowExecutionsResponse{Count: 1}, nil)
				b.EXPECT().StartBatchOperation(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ any, req *types.StartBatchOperationRequest, _ ...any) (*types.StartBatchOperationResponse, error) {
						*captured = req
						return &types.StartBatchOperationResponse{BatchID: req.BatchID}, nil
					})
			},
			wantErr: false,
			validate: func(t *testing.T, req *types.StartBatchOperationRequest) {
				require.NotNil(t, req.Operation)
				assert.Equal(t, types.BatchOperationTypeRefresh, req.Operation.GetOperationType())
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockFrontendClient := frontend.NewMockClient(mockCtrl)
			mockBatchClient := batch.NewMockClient(mockCtrl)
			var capturedReq *types.StartBatchOperationRequest
			tt.mockFn(mockFrontendClient, mockBatchClient, &capturedReq)

			app := NewCliApp(&clientFactoryMock{serverFrontendClient: mockFrontendClient, serverBatchClient: mockBatchClient})
			if tt.input != "" {
				app.Reader = strings.NewReader(tt.input)
			}

			c := newBatchOperationCLIContext(app, tt.flags)
			err := tt.action(c)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if tt.validate != nil {
				tt.validate(t, capturedReq)
			}
		})
	}
}
