// The MIT License (MIT)
//
// Copyright (c) 2017-2020 Uber Technologies Inc.
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

package fetcher

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/codec"
	"github.com/uber/cadence/common/pagination"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/entity"
)

func TestConcreteExecutionIterator(t *testing.T) {
	ctrl := gomock.NewController(t)
	retryer := persistence.NewMockRetryer(ctrl)
	retryer.EXPECT().ListConcreteExecutions(gomock.Any(), gomock.Any()).
		Return(&persistence.ListConcreteExecutionsResponse{}, nil).
		Times(1)

	iterator := ConcreteExecutionIterator(
		context.Background(),
		retryer,
		10,
	)
	require.NotNil(t, iterator)
}

func TestConcreteExecution(t *testing.T) {
	encoder := codec.NewThriftRWEncoder()
	tests := []struct {
		desc       string
		req        ExecutionRequest
		mockFn     func(retryer *persistence.MockRetryer)
		wantEntity entity.Entity
		wantErr    bool
	}{
		{
			desc: "success",
			req: ExecutionRequest{
				DomainID:   "test-domain-id",
				WorkflowID: "test-workflow-id",
				RunID:      "test-run-id",
				DomainName: "test-domain-name",
			},
			mockFn: func(retryer *persistence.MockRetryer) {
				retryer.EXPECT().GetWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(
						&persistence.GetWorkflowExecutionResponse{
							State: &persistence.WorkflowMutableState{
								ExecutionInfo: &persistence.WorkflowExecutionInfo{
									BranchToken: mustGetValidBranchToken(t, encoder, "test-tree-id", "test-branch-id"),
									State:       persistence.WorkflowStateRunning,
									DomainID:    "test-domain-id",
									WorkflowID:  "test-workflow-id",
									RunID:       "test-run-id",
								},
							},
						},
						nil,
					).Times(1)

				retryer.EXPECT().GetShardID().Return(355).Times(1)
			},
			wantEntity: &entity.ConcreteExecution{
				BranchToken: mustGetValidBranchToken(t, encoder, "test-tree-id", "test-branch-id"),
				TreeID:      "test-tree-id",
				BranchID:    "test-branch-id",
				Execution: entity.Execution{
					ShardID:    355,
					DomainID:   "test-domain-id",
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
					State:      persistence.WorkflowStateRunning,
				},
			},
		},
		{
			desc: "GetWorkflowExecution failed",
			req: ExecutionRequest{
				DomainID:   "test-domain-id",
				WorkflowID: "test-workflow-id",
				RunID:      "test-run-id",
				DomainName: "test-domain-name",
			},
			mockFn: func(retryer *persistence.MockRetryer) {
				retryer.EXPECT().GetWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(nil, fmt.Errorf("failed")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			retryer := persistence.NewMockRetryer(ctrl)

			tc.mockFn(retryer)

			gotEntity, err := ConcreteExecution(context.Background(), retryer, tc.req)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ConcreteExecution() err: %v, wantErr %v", err, tc.wantErr)
			}

			if diff := cmp.Diff(tc.wantEntity, gotEntity); diff != "" {
				t.Errorf("ConcreteExecution() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetConcreteExecutions(t *testing.T) {
	encoder := codec.NewThriftRWEncoder()
	testExecutions := []*persistence.ListConcreteExecutionsEntity{
		{
			ExecutionInfo: &persistence.WorkflowExecutionInfo{
				BranchToken: mustGetValidBranchToken(t, encoder, "test-tree-id-1", "test-branch-id-1"),
				State:       persistence.WorkflowStateRunning,
				DomainID:    "test-domain-id-1",
				WorkflowID:  "test-workflow-id-1",
				RunID:       "test-run-id-1",
			},
		},
		{
			ExecutionInfo: &persistence.WorkflowExecutionInfo{
				BranchToken: mustGetValidBranchToken(t, encoder, "test-tree-id-2", "test-branch-id-2"),
				State:       persistence.WorkflowStateCompleted,
				DomainID:    "test-domain-id-2",
				WorkflowID:  "test-workflow-id-2",
				RunID:       "test-run-id-2",
			},
		},
	}

	tests := []struct {
		desc      string
		pageSize  int
		pageToken pagination.PageToken
		mockFn    func(*testing.T, *persistence.MockRetryer)
		wantPage  pagination.Page
		wantErr   bool
	}{
		{
			desc:      "success",
			pageSize:  2,
			pageToken: []byte("test-page-token"),
			mockFn: func(t *testing.T, retryer *persistence.MockRetryer) {
				retryer.EXPECT().ListConcreteExecutions(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, req *persistence.ListConcreteExecutionsRequest) (*persistence.ListConcreteExecutionsResponse, error) {
						wantReq := &persistence.ListConcreteExecutionsRequest{
							PageSize:  2,
							PageToken: []byte("test-page-token"),
						}
						if diff := cmp.Diff(wantReq, req); diff != "" {
							t.Errorf("Request mismatch (-want +got):\n%s", diff)
						}
						return &persistence.ListConcreteExecutionsResponse{
							PageToken:  []byte("test-next-page-token"),
							Executions: testExecutions,
						}, nil
					}).Times(1)

				// will be called for each execution in the response
				retryer.EXPECT().GetShardID().Return(355).Times(2)
			},
			wantPage: pagination.Page{
				CurrentToken: []byte("test-page-token"),
				NextToken:    []byte("test-next-page-token"),
				Entities:     concreteExecutionsToEntities(testExecutions, 355, encoder),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			retryer := persistence.NewMockRetryer(ctrl)

			tc.mockFn(t, retryer)

			fetchFn := getConcreteExecutions(retryer, tc.pageSize, encoder)
			gotPage, err := fetchFn(context.Background(), tc.pageToken)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ConcreteExecution() err: %v, wantErr %v", err, tc.wantErr)
			}

			if diff := cmp.Diff(tc.wantPage, gotPage); diff != "" {
				t.Errorf("ConcreteExecution() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetBranchToken(t *testing.T) {
	encoder := codec.NewThriftRWEncoder()
	testCases := []struct {
		name              string
		entity            *persistence.ListConcreteExecutionsEntity
		wantErr           bool
		wantBranchToken   []byte
		wantHistoryBranch shared.HistoryBranch
	}{
		{
			name: "valid branch token - no version histories",
			entity: &persistence.ListConcreteExecutionsEntity{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					BranchToken: mustGetValidBranchToken(t, encoder, "test-tree-id", "test-branch-id"),
				},
			},
			wantBranchToken: mustGetValidBranchToken(t, encoder, "test-tree-id", "test-branch-id"),
			wantHistoryBranch: shared.HistoryBranch{
				TreeID:   common.StringPtr("test-tree-id"),
				BranchID: common.StringPtr("test-branch-id"),
			},
		},
		{
			name: "valid branch token - with version histories",
			entity: &persistence.ListConcreteExecutionsEntity{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					BranchToken: mustGetValidBranchToken(t, encoder, "test-tree-id", "test-branch-id"),
				},
				VersionHistories: &persistence.VersionHistories{
					CurrentVersionHistoryIndex: 1,
					Histories: []*persistence.VersionHistory{
						{}, // this will be ignored because index is 1
						{
							BranchToken: mustGetValidBranchToken(t, encoder, "test-tree-id-from-versionhistory", "test-branch-id-from-versionhistory"),
						},
					},
				},
			},
			wantBranchToken: mustGetValidBranchToken(t, encoder, "test-tree-id-from-versionhistory", "test-branch-id-from-versionhistory"),
			wantHistoryBranch: shared.HistoryBranch{
				TreeID:   common.StringPtr("test-tree-id-from-versionhistory"),
				BranchID: common.StringPtr("test-branch-id-from-versionhistory"),
			},
		},
		{
			name: "version history index out of bound",
			entity: &persistence.ListConcreteExecutionsEntity{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					BranchToken: mustGetValidBranchToken(t, encoder, "test-tree-id", "test-branch-id"),
				},
				VersionHistories: &persistence.VersionHistories{
					CurrentVersionHistoryIndex: 2,
					Histories: []*persistence.VersionHistory{
						{},
						{
							BranchToken: mustGetValidBranchToken(t, encoder, "test-tree-id-from-versionhistory", "test-branch-id-from-versionhistory"),
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid branch token",
			entity: &persistence.ListConcreteExecutionsEntity{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					BranchToken: []byte("invalid"),
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			branchToken, branch, err := getBranchToken(
				tc.entity.ExecutionInfo.BranchToken,
				tc.entity.VersionHistories,
				encoder,
			)

			if tc.wantErr {
				require.Error(t, err)
				require.Nil(t, branchToken)
				require.Empty(t, branch.GetTreeID())
				require.Empty(t, branch.GetBranchID())
			} else {
				if diff := cmp.Diff(tc.wantHistoryBranch, branch); diff != "" {
					t.Fatalf("HistoryBranch mismatch (-want +got):\n%s", diff)
				}
				require.Equal(t, tc.wantBranchToken, branchToken)
			}
		})
	}
}

func mustGetValidBranchToken(t *testing.T, encoder *codec.ThriftRWEncoder, treeID, branchID string) []byte {
	hb := &shared.HistoryBranch{
		TreeID:   common.StringPtr(treeID),
		BranchID: common.StringPtr(branchID),
	}
	bytes, err := encoder.Encode(hb)
	if err != nil {
		t.Fatalf("failed to encode branch token: %v", err)
	}

	return bytes
}

func concreteExecutionsToEntities(execs []*persistence.ListConcreteExecutionsEntity, shardID int, encoder *codec.ThriftRWEncoder) []pagination.Entity {
	entities := make([]pagination.Entity, len(execs))
	for i, e := range execs {
		branchToken, branch, err := getBranchToken(e.ExecutionInfo.BranchToken, e.VersionHistories, encoder)
		if err != nil {
			return nil
		}
		concreteExec := &entity.ConcreteExecution{
			BranchToken: branchToken,
			TreeID:      branch.GetTreeID(),
			BranchID:    branch.GetBranchID(),
			Execution: entity.Execution{
				ShardID:    shardID,
				DomainID:   e.ExecutionInfo.DomainID,
				WorkflowID: e.ExecutionInfo.WorkflowID,
				RunID:      e.ExecutionInfo.RunID,
				State:      e.ExecutionInfo.State,
			},
		}
		entities[i] = concreteExec
	}
	return entities
}
