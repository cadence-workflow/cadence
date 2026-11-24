package cli

import (
	"flag"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
	"go.uber.org/mock/gomock"

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

// newUpdateBatchOperationCLIContext mirrors newBatchOperationCLIContext, except
// FlagRPS/FlagConcurrency default to 0 (unset), matching the real "update" command's
// flags, which (unlike "start"'s) have no non-zero default.
func newUpdateBatchOperationCLIContext(app *cli.App, overrides map[string]string) *cli.Context {
	set := flag.NewFlagSet("test", 0)

	domain := "test-domain"
	if v, ok := overrides[FlagDomain]; ok {
		domain = v
	}
	set.String(FlagDomain, domain, "")
	set.String(FlagTransport, grpcTransport, "")

	set.Int(FlagRPS, 0, "")
	set.Int(FlagConcurrency, 0, "")

	handled := map[string]bool{FlagDomain: true, FlagTransport: true}
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
		mockFn   func(*frontend.MockClient, **types.StartBatchOperationRequest)
		wantErr  bool
		validate func(*testing.T, *types.StartBatchOperationRequest)
	}{
		"missing domain": {
			action:  StartBatchTerminate,
			flags:   map[string]string{FlagDomain: "", FlagListQuery: "q", FlagReason: "r"},
			mockFn:  func(f *frontend.MockClient, captured **types.StartBatchOperationRequest) {},
			wantErr: true,
		},
		"missing query": {
			action:  StartBatchTerminate,
			flags:   map[string]string{FlagListQuery: "", FlagReason: "r"},
			mockFn:  func(f *frontend.MockClient, captured **types.StartBatchOperationRequest) {},
			wantErr: true,
		},
		"missing reason": {
			action:  StartBatchTerminate,
			flags:   map[string]string{FlagListQuery: "q", FlagReason: ""},
			mockFn:  func(f *frontend.MockClient, captured **types.StartBatchOperationRequest) {},
			wantErr: true,
		},
		"signal missing signal name": {
			action: StartBatchSignal,
			flags: map[string]string{
				FlagListQuery: "UserID = 5",
				FlagReason:    "approve",
			},
			mockFn:  func(f *frontend.MockClient, captured **types.StartBatchOperationRequest) {},
			wantErr: true,
		},
		"replicate missing source cluster": {
			action: StartBatchReplicate,
			flags: map[string]string{
				FlagListQuery:     "WorkflowType = 'buggy'",
				FlagReason:        "resend",
				FlagTargetCluster: "cluster-b",
			},
			mockFn:  func(f *frontend.MockClient, captured **types.StartBatchOperationRequest) {},
			wantErr: true,
		},
		"replicate missing target cluster": {
			action: StartBatchReplicate,
			flags: map[string]string{
				FlagListQuery:     "WorkflowType = 'buggy'",
				FlagReason:        "resend",
				FlagSourceCluster: "cluster-a",
			},
			mockFn:  func(f *frontend.MockClient, captured **types.StartBatchOperationRequest) {},
			wantErr: true,
		},
		"non-grpc transport is rejected": {
			action: StartBatchTerminate,
			flags: map[string]string{
				FlagListQuery: "q",
				FlagReason:    "r",
				FlagTransport: "tchannel",
			},
			mockFn:  func(f *frontend.MockClient, captured **types.StartBatchOperationRequest) {},
			wantErr: true,
		},
		"count workflows fails": {
			action: StartBatchTerminate,
			flags:  map[string]string{FlagListQuery: "q", FlagReason: "r"},
			mockFn: func(f *frontend.MockClient, captured **types.StartBatchOperationRequest) {
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
			mockFn: func(f *frontend.MockClient, captured **types.StartBatchOperationRequest) {
				f.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).
					Return(&types.CountWorkflowExecutionsResponse{Count: 3}, nil)
			},
			wantErr: false,
		},
		"start batch operation fails": {
			action: StartBatchTerminate,
			flags:  map[string]string{FlagListQuery: "q", FlagReason: "r"},
			mockFn: func(f *frontend.MockClient, captured **types.StartBatchOperationRequest) {
				f.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).
					Return(&types.CountWorkflowExecutionsResponse{Count: 3}, nil)
				f.EXPECT().StartBatchOperation(gomock.Any(), gomock.Any()).
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
			mockFn: func(f *frontend.MockClient, captured **types.StartBatchOperationRequest) {
				f.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).
					Return(&types.CountWorkflowExecutionsResponse{Count: 3}, nil)
				f.EXPECT().StartBatchOperation(gomock.Any(), gomock.Any()).
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
			mockFn: func(f *frontend.MockClient, captured **types.StartBatchOperationRequest) {
				f.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).
					Return(&types.CountWorkflowExecutionsResponse{Count: 1}, nil)
				f.EXPECT().StartBatchOperation(gomock.Any(), gomock.Any()).
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
			mockFn: func(f *frontend.MockClient, captured **types.StartBatchOperationRequest) {
				f.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).
					Return(&types.CountWorkflowExecutionsResponse{Count: 1}, nil)
				f.EXPECT().StartBatchOperation(gomock.Any(), gomock.Any()).
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
			mockFn: func(f *frontend.MockClient, captured **types.StartBatchOperationRequest) {
				f.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).
					Return(&types.CountWorkflowExecutionsResponse{Count: 1}, nil)
				f.EXPECT().StartBatchOperation(gomock.Any(), gomock.Any()).
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
			mockFn: func(f *frontend.MockClient, captured **types.StartBatchOperationRequest) {
				f.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).
					Return(&types.CountWorkflowExecutionsResponse{Count: 2}, nil)
				f.EXPECT().StartBatchOperation(gomock.Any(), gomock.Any()).
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
			mockFn: func(f *frontend.MockClient, captured **types.StartBatchOperationRequest) {
				f.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).
					Return(&types.CountWorkflowExecutionsResponse{Count: 1}, nil)
				f.EXPECT().StartBatchOperation(gomock.Any(), gomock.Any()).
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
			var capturedReq *types.StartBatchOperationRequest
			tt.mockFn(mockFrontendClient, &capturedReq)

			app := NewCliApp(&clientFactoryMock{serverFrontendClient: mockFrontendClient})
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

func TestListBatchOperations(t *testing.T) {
	tests := map[string]struct {
		flags    map[string]string
		mockFn   func(*frontend.MockClient, **types.ListBatchOperationsRequest)
		wantErr  bool
		validate func(*testing.T, *types.ListBatchOperationsRequest)
	}{
		"missing domain": {
			flags:   map[string]string{FlagDomain: ""},
			mockFn:  func(f *frontend.MockClient, captured **types.ListBatchOperationsRequest) {},
			wantErr: true,
		},
		"invalid state": {
			flags:   map[string]string{FlagBatchOperationState: "bogus"},
			mockFn:  func(f *frontend.MockClient, captured **types.ListBatchOperationsRequest) {},
			wantErr: true,
		},
		"list fails": {
			flags: map[string]string{},
			mockFn: func(f *frontend.MockClient, captured **types.ListBatchOperationsRequest) {
				f.EXPECT().ListBatchOperations(gomock.Any(), gomock.Any()).
					Return(nil, assert.AnError)
			},
			wantErr: true,
		},
		"success, no state filter": {
			flags: map[string]string{},
			mockFn: func(f *frontend.MockClient, captured **types.ListBatchOperationsRequest) {
				f.EXPECT().ListBatchOperations(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ any, req *types.ListBatchOperationsRequest, _ ...any) (*types.ListBatchOperationsResponse, error) {
						*captured = req
						return &types.ListBatchOperationsResponse{
							BatchOperations: []*types.BatchOperationListEntry{
								{BatchID: "my-batch-id", State: types.BatchOperationStateRunning, StartedBy: "my-operator"},
							},
						}, nil
					})
			},
			wantErr: false,
			validate: func(t *testing.T, req *types.ListBatchOperationsRequest) {
				assert.Equal(t, "test-domain", req.Domain)
				assert.Equal(t, types.BatchOperationStateInvalid, req.State)
			},
		},
		"success, with state filter": {
			flags: map[string]string{FlagBatchOperationState: "Running"},
			mockFn: func(f *frontend.MockClient, captured **types.ListBatchOperationsRequest) {
				f.EXPECT().ListBatchOperations(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ any, req *types.ListBatchOperationsRequest, _ ...any) (*types.ListBatchOperationsResponse, error) {
						*captured = req
						return &types.ListBatchOperationsResponse{}, nil
					})
			},
			wantErr: false,
			validate: func(t *testing.T, req *types.ListBatchOperationsRequest) {
				assert.Equal(t, types.BatchOperationStateRunning, req.State)
			},
		},
		"success, no results": {
			flags: map[string]string{},
			mockFn: func(f *frontend.MockClient, captured **types.ListBatchOperationsRequest) {
				f.EXPECT().ListBatchOperations(gomock.Any(), gomock.Any()).
					Return(&types.ListBatchOperationsResponse{}, nil)
			},
			wantErr: false,
		},
		"success, follows next page token across multiple pages": {
			flags: map[string]string{},
			mockFn: func(f *frontend.MockClient, captured **types.ListBatchOperationsRequest) {
				first := f.EXPECT().ListBatchOperations(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ any, req *types.ListBatchOperationsRequest, _ ...any) (*types.ListBatchOperationsResponse, error) {
						assert.Empty(t, req.NextPageToken)
						return &types.ListBatchOperationsResponse{
							BatchOperations: []*types.BatchOperationListEntry{
								{BatchID: "batch-1", State: types.BatchOperationStateRunning, StartedBy: "op-1"},
							},
							NextPageToken: []byte("page-2"),
						}, nil
					})
				f.EXPECT().ListBatchOperations(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ any, req *types.ListBatchOperationsRequest, _ ...any) (*types.ListBatchOperationsResponse, error) {
						*captured = req
						assert.Equal(t, []byte("page-2"), req.NextPageToken)
						return &types.ListBatchOperationsResponse{
							BatchOperations: []*types.BatchOperationListEntry{
								{BatchID: "batch-2", State: types.BatchOperationStateCompleted, StartedBy: "op-2"},
							},
						}, nil
					}).After(first)
			},
			wantErr: false,
			validate: func(t *testing.T, req *types.ListBatchOperationsRequest) {
				assert.Equal(t, []byte("page-2"), req.NextPageToken)
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockFrontendClient := frontend.NewMockClient(mockCtrl)
			var capturedReq *types.ListBatchOperationsRequest
			tt.mockFn(mockFrontendClient, &capturedReq)

			app := NewCliApp(&clientFactoryMock{serverFrontendClient: mockFrontendClient})
			c := newBatchOperationCLIContext(app, tt.flags)
			err := ListBatchOperations(c)

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

func TestDescribeBatchOperation(t *testing.T) {
	startTime := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)

	tests := map[string]struct {
		flags    map[string]string
		mockFn   func(*frontend.MockClient, **types.DescribeBatchOperationRequest)
		wantErr  bool
		validate func(*testing.T, *types.DescribeBatchOperationRequest)
	}{
		"missing domain": {
			flags:   map[string]string{FlagDomain: "", FlagBatchID: "my-batch-id"},
			mockFn:  func(f *frontend.MockClient, captured **types.DescribeBatchOperationRequest) {},
			wantErr: true,
		},
		"missing batch id": {
			flags:   map[string]string{FlagBatchID: ""},
			mockFn:  func(f *frontend.MockClient, captured **types.DescribeBatchOperationRequest) {},
			wantErr: true,
		},
		"describe fails": {
			flags: map[string]string{FlagBatchID: "my-batch-id"},
			mockFn: func(f *frontend.MockClient, captured **types.DescribeBatchOperationRequest) {
				f.EXPECT().DescribeBatchOperation(gomock.Any(), gomock.Any()).
					Return(nil, assert.AnError)
			},
			wantErr: true,
		},
		"success": {
			flags: map[string]string{FlagBatchID: "my-batch-id"},
			mockFn: func(f *frontend.MockClient, captured **types.DescribeBatchOperationRequest) {
				f.EXPECT().DescribeBatchOperation(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ any, req *types.DescribeBatchOperationRequest, _ ...any) (*types.DescribeBatchOperationResponse, error) {
						*captured = req
						return &types.DescribeBatchOperationResponse{
							Operation: &types.BatchOperation{
								OperationType: types.BatchOperationTypeTerminate.Ptr(),
							},
							Query:     "TaskList = 'buggy'",
							Reason:    "cleanup",
							State:     types.BatchOperationStateRunning,
							StartedBy: "my-operator",
							StartTime: &startTime,
						}, nil
					})
			},
			wantErr: false,
			validate: func(t *testing.T, req *types.DescribeBatchOperationRequest) {
				assert.Equal(t, "test-domain", req.Domain)
				assert.Equal(t, "my-batch-id", req.BatchID)
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockFrontendClient := frontend.NewMockClient(mockCtrl)
			var capturedReq *types.DescribeBatchOperationRequest
			tt.mockFn(mockFrontendClient, &capturedReq)

			app := NewCliApp(&clientFactoryMock{serverFrontendClient: mockFrontendClient})
			c := newBatchOperationCLIContext(app, tt.flags)
			err := DescribeBatchOperation(c)

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

func TestStopBatchOperation(t *testing.T) {
	tests := map[string]struct {
		flags    map[string]string
		mockFn   func(*frontend.MockClient, **types.StopBatchOperationRequest)
		wantErr  bool
		validate func(*testing.T, *types.StopBatchOperationRequest)
	}{
		"missing domain": {
			flags:   map[string]string{FlagDomain: "", FlagBatchID: "my-batch-id"},
			mockFn:  func(f *frontend.MockClient, captured **types.StopBatchOperationRequest) {},
			wantErr: true,
		},
		"missing batch id": {
			flags:   map[string]string{FlagBatchID: ""},
			mockFn:  func(f *frontend.MockClient, captured **types.StopBatchOperationRequest) {},
			wantErr: true,
		},
		"stop fails": {
			flags: map[string]string{FlagBatchID: "my-batch-id"},
			mockFn: func(f *frontend.MockClient, captured **types.StopBatchOperationRequest) {
				f.EXPECT().StopBatchOperation(gomock.Any(), gomock.Any()).
					Return(nil, assert.AnError)
			},
			wantErr: true,
		},
		"success": {
			flags: map[string]string{FlagBatchID: "my-batch-id", FlagReason: "cleanup"},
			mockFn: func(f *frontend.MockClient, captured **types.StopBatchOperationRequest) {
				f.EXPECT().StopBatchOperation(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ any, req *types.StopBatchOperationRequest, _ ...any) (*types.StopBatchOperationResponse, error) {
						*captured = req
						return &types.StopBatchOperationResponse{}, nil
					})
			},
			wantErr: false,
			validate: func(t *testing.T, req *types.StopBatchOperationRequest) {
				assert.Equal(t, "test-domain", req.Domain)
				assert.Equal(t, "my-batch-id", req.BatchID)
				assert.Equal(t, "cleanup", req.Reason)
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockFrontendClient := frontend.NewMockClient(mockCtrl)
			var capturedReq *types.StopBatchOperationRequest
			tt.mockFn(mockFrontendClient, &capturedReq)

			app := NewCliApp(&clientFactoryMock{serverFrontendClient: mockFrontendClient})
			c := newBatchOperationCLIContext(app, tt.flags)
			err := StopBatchOperation(c)

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

func TestUpdateBatchOperation(t *testing.T) {
	tests := map[string]struct {
		flags    map[string]string
		setup    func(*testing.T, *cli.Context)
		mockFn   func(*frontend.MockClient, **types.UpdateBatchOperationRequest)
		wantErr  bool
		validate func(*testing.T, *types.UpdateBatchOperationRequest)
	}{
		"missing domain": {
			flags:   map[string]string{FlagDomain: "", FlagBatchID: "my-batch-id"},
			mockFn:  func(f *frontend.MockClient, captured **types.UpdateBatchOperationRequest) {},
			wantErr: true,
		},
		"missing batch id": {
			flags:   map[string]string{FlagBatchID: ""},
			mockFn:  func(f *frontend.MockClient, captured **types.UpdateBatchOperationRequest) {},
			wantErr: true,
		},
		"neither rps nor concurrency set": {
			flags:   map[string]string{FlagBatchID: "my-batch-id"},
			mockFn:  func(f *frontend.MockClient, captured **types.UpdateBatchOperationRequest) {},
			wantErr: true,
		},
		"negative rps rejected": {
			flags: map[string]string{FlagBatchID: "my-batch-id"},
			setup: func(t *testing.T, c *cli.Context) {
				require.NoError(t, c.Set(FlagRPS, "-1"))
			},
			mockFn:  func(f *frontend.MockClient, captured **types.UpdateBatchOperationRequest) {},
			wantErr: true,
		},
		"negative concurrency rejected": {
			flags: map[string]string{FlagBatchID: "my-batch-id"},
			setup: func(t *testing.T, c *cli.Context) {
				require.NoError(t, c.Set(FlagConcurrency, "-5"))
			},
			mockFn:  func(f *frontend.MockClient, captured **types.UpdateBatchOperationRequest) {},
			wantErr: true,
		},
		"update fails": {
			flags: map[string]string{FlagBatchID: "my-batch-id"},
			setup: func(t *testing.T, c *cli.Context) {
				require.NoError(t, c.Set(FlagRPS, "100"))
			},
			mockFn: func(f *frontend.MockClient, captured **types.UpdateBatchOperationRequest) {
				f.EXPECT().UpdateBatchOperation(gomock.Any(), gomock.Any()).
					Return(nil, assert.AnError)
			},
			wantErr: true,
		},
		"success, rps only": {
			flags: map[string]string{FlagBatchID: "my-batch-id"},
			setup: func(t *testing.T, c *cli.Context) {
				require.NoError(t, c.Set(FlagRPS, "100"))
			},
			mockFn: func(f *frontend.MockClient, captured **types.UpdateBatchOperationRequest) {
				f.EXPECT().UpdateBatchOperation(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ any, req *types.UpdateBatchOperationRequest, _ ...any) (*types.UpdateBatchOperationResponse, error) {
						*captured = req
						return &types.UpdateBatchOperationResponse{}, nil
					})
			},
			wantErr: false,
			validate: func(t *testing.T, req *types.UpdateBatchOperationRequest) {
				assert.Equal(t, "test-domain", req.Domain)
				assert.Equal(t, "my-batch-id", req.BatchID)
				assert.EqualValues(t, 100, req.RPS)
				assert.EqualValues(t, 0, req.Concurrency)
			},
		},
		"success, concurrency only": {
			flags: map[string]string{FlagBatchID: "my-batch-id"},
			setup: func(t *testing.T, c *cli.Context) {
				require.NoError(t, c.Set(FlagConcurrency, "20"))
			},
			mockFn: func(f *frontend.MockClient, captured **types.UpdateBatchOperationRequest) {
				f.EXPECT().UpdateBatchOperation(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ any, req *types.UpdateBatchOperationRequest, _ ...any) (*types.UpdateBatchOperationResponse, error) {
						*captured = req
						return &types.UpdateBatchOperationResponse{}, nil
					})
			},
			wantErr: false,
			validate: func(t *testing.T, req *types.UpdateBatchOperationRequest) {
				assert.EqualValues(t, 0, req.RPS)
				assert.EqualValues(t, 20, req.Concurrency)
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockFrontendClient := frontend.NewMockClient(mockCtrl)
			var capturedReq *types.UpdateBatchOperationRequest
			tt.mockFn(mockFrontendClient, &capturedReq)

			app := NewCliApp(&clientFactoryMock{serverFrontendClient: mockFrontendClient})
			c := newUpdateBatchOperationCLIContext(app, tt.flags)
			if tt.setup != nil {
				tt.setup(t, c)
			}
			err := UpdateBatchOperation(c)

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
