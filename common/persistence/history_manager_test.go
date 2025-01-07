// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

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

package persistence

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	workflow "github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/codec"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/types"
)

func setUpMocksForHistoryV2Manager(t *testing.T) (*historyV2ManagerImpl, *MockHistoryStore, *MockPayloadSerializer, *codec.MockBinaryEncoder) {
	ctrl := gomock.NewController(t)
	mockStore := NewMockHistoryStore(ctrl)
	mockSerializer := NewMockPayloadSerializer(ctrl)
	mockEncoder := codec.NewMockBinaryEncoder(ctrl)
	logger := log.NewNoop()

	mockStore.EXPECT().GetName().Return("mock history store")

	historyManager := NewHistoryV2ManagerImpl(
		mockStore,
		logger,
		mockSerializer,
		mockEncoder,
		dynamicconfig.GetIntPropertyFn(1024*10),
	)
	assert.Equal(t, "mock history store", historyManager.GetName())

	return historyManager.(*historyV2ManagerImpl), mockStore, mockSerializer, mockEncoder
}

func TestForkHistoryBranch(t *testing.T) {
	testCases := []struct {
		name          string
		setupMock     func(*MockHistoryStore, *codec.MockBinaryEncoder)
		request       *ForkHistoryBranchRequest
		expectError   bool
		expectedError string
		expected      *ForkHistoryBranchResponse
	}{
		{
			name: "success",
			setupMock: func(mockStore *MockHistoryStore, mockEncoder *codec.MockBinaryEncoder) {
				mockEncoder.EXPECT().
					Decode([]byte("fork-branch"), &workflow.HistoryBranch{}).DoAndReturn(func(data []byte, value *workflow.HistoryBranch) error {
					value.TreeID = common.Ptr("tree-id")
					value.BranchID = common.Ptr("branch-id")
					return nil
				}).Times(1)
				mockStore.EXPECT().
					ForkHistoryBranch(gomock.Any(), gomock.Any()).
					Return(&InternalForkHistoryBranchResponse{
						NewBranchInfo: types.HistoryBranch{
							TreeID:   "new-tree-id",
							BranchID: "new-branch-id",
						},
					}, nil).Times(1)
				mockEncoder.EXPECT().
					Encode(&workflow.HistoryBranch{
						TreeID:   common.StringPtr("new-tree-id"),
						BranchID: common.StringPtr("new-branch-id"),
					}).
					Return([]byte("new-branch-token"), nil).Times(1)
			},
			request: &ForkHistoryBranchRequest{
				ForkBranchToken: []byte("fork-branch"),
				ForkNodeID:      2,
				Info:            "fork info",
				ShardID:         common.Ptr(10),
			},
			expectError: false,
			expected: &ForkHistoryBranchResponse{
				NewBranchToken: []byte("new-branch-token"),
			},
		},
		{
			name: "nil Shard ID",
			setupMock: func(mockStore *MockHistoryStore, mockEncoder *codec.MockBinaryEncoder) {
				// No store or encoder interactions for invalid input
			},
			request: &ForkHistoryBranchRequest{
				ForkBranchToken: []byte("fork-branch"),
				ForkNodeID:      2,
				Info:            "fork info",
			},
			expectError:   true,
			expectedError: "shardID is not set for persistence operation",
		},
		{
			name: "invalid fork node ID",
			setupMock: func(mockStore *MockHistoryStore, mockEncoder *codec.MockBinaryEncoder) {
				// No store or encoder interactions for invalid input
			},
			request: &ForkHistoryBranchRequest{
				ForkBranchToken: []byte("fork-branch"),
				ForkNodeID:      1, // Invalid ID
				Info:            "fork info",
				ShardID:         common.Ptr(10),
			},
			expectError:   true,
			expectedError: "ForkNodeID must be > 1",
		},
		{
			name: "decode error",
			setupMock: func(mockStore *MockHistoryStore, mockEncoder *codec.MockBinaryEncoder) {
				mockEncoder.EXPECT().
					Decode(gomock.Any(), gomock.Any()).
					Return(errors.New("decode error")).Times(1)
			},
			request: &ForkHistoryBranchRequest{
				ForkBranchToken: []byte("fork-branch"),
				ForkNodeID:      2,
				Info:            "fork info",
				ShardID:         common.Ptr(10),
			},
			expectError:   true,
			expectedError: "decode error",
		},
		{
			name: "persistence error",
			setupMock: func(mockStore *MockHistoryStore, mockEncoder *codec.MockBinaryEncoder) {
				mockEncoder.EXPECT().
					Decode(gomock.Any(), gomock.Any()).DoAndReturn(func(data []byte, value *workflow.HistoryBranch) error {
					value.TreeID = common.Ptr("tree-id")
					value.BranchID = common.Ptr("branch-id")
					return nil
				}).Times(1)
				mockStore.EXPECT().
					ForkHistoryBranch(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("persistence error")).Times(1)
			},
			request: &ForkHistoryBranchRequest{
				ForkBranchToken: []byte("fork-branch"),
				ForkNodeID:      2,
				Info:            "fork info",
				ShardID:         common.Ptr(10),
			},
			expectError:   true,
			expectedError: "persistence error",
		},
		{
			name: "encode error",
			setupMock: func(mockStore *MockHistoryStore, mockEncoder *codec.MockBinaryEncoder) {
				mockEncoder.EXPECT().
					Decode(gomock.Any(), gomock.Any()).DoAndReturn(func(data []byte, value *workflow.HistoryBranch) error {
					value.TreeID = common.Ptr("tree-id")
					value.BranchID = common.Ptr("branch-id")
					return nil
				}).Times(1)
				mockStore.EXPECT().
					ForkHistoryBranch(gomock.Any(), gomock.Any()).
					Return(&InternalForkHistoryBranchResponse{
						NewBranchInfo: types.HistoryBranch{
							TreeID:   "new-tree-id",
							BranchID: "new-branch-id",
						},
					}, nil).Times(1)
				mockEncoder.EXPECT().
					Encode(gomock.Any()).Return(nil, errors.New("encode error"))
			},
			request: &ForkHistoryBranchRequest{
				ForkBranchToken: []byte("fork-branch"),
				ForkNodeID:      2,
				Info:            "fork info",
				ShardID:         common.Ptr(10),
			},
			expectError:   true,
			expectedError: "encode error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			historyManager, mockStore, _, mockEncoder := setUpMocksForHistoryV2Manager(t)

			tc.setupMock(mockStore, mockEncoder)

			// Call the method
			resp, err := historyManager.ForkHistoryBranch(context.Background(), tc.request)

			// Validate the result
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, resp)
			}
		})
	}
}

func TestDeleteHistoryBranch(t *testing.T) {
	testCases := []struct {
		name          string
		setupMock     func(*MockHistoryStore, *codec.MockBinaryEncoder)
		request       *DeleteHistoryBranchRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			setupMock: func(mockStore *MockHistoryStore, mockEncoder *codec.MockBinaryEncoder) {
				mockEncoder.EXPECT().
					Decode([]byte("branch-token"), &workflow.HistoryBranch{}).DoAndReturn(func(data []byte, value *workflow.HistoryBranch) error {
					value.TreeID = common.Ptr("tree-id")
					value.BranchID = common.Ptr("branch-id")
					return nil
				}).Times(1)
				mockStore.EXPECT().
					DeleteHistoryBranch(gomock.Any(), &InternalDeleteHistoryBranchRequest{
						ShardID: 10,
						BranchInfo: types.HistoryBranch{
							TreeID:   "tree-id",
							BranchID: "branch-id",
						},
					}).
					Return(nil).Times(1)
			},
			request: &DeleteHistoryBranchRequest{
				BranchToken: []byte("branch-token"),
				ShardID:     common.Ptr(10),
			},
			expectError: false,
		},
		{
			name: "nil shard ID error",
			setupMock: func(mockStore *MockHistoryStore, mockEncoder *codec.MockBinaryEncoder) {
			},
			request: &DeleteHistoryBranchRequest{
				BranchToken: []byte("branch-token"),
			},
			expectError:   true,
			expectedError: "shardID is not set for persistence operation",
		},
		{
			name: "decode error",
			setupMock: func(mockStore *MockHistoryStore, mockEncoder *codec.MockBinaryEncoder) {
				mockEncoder.EXPECT().
					Decode(gomock.Any(), gomock.Any()).
					Return(errors.New("decode error")).Times(1)
			},
			request: &DeleteHistoryBranchRequest{
				BranchToken: []byte("branch-token"),
				ShardID:     common.Ptr(10),
			},
			expectError:   true,
			expectedError: "decode error",
		},
		{
			name: "persistence error",
			setupMock: func(mockStore *MockHistoryStore, mockEncoder *codec.MockBinaryEncoder) {
				mockEncoder.EXPECT().
					Decode([]byte("branch-token"), &workflow.HistoryBranch{}).DoAndReturn(func(data []byte, value *workflow.HistoryBranch) error {
					value.TreeID = common.Ptr("tree-id")
					value.BranchID = common.Ptr("branch-id")
					return nil
				}).Times(1)
				mockStore.EXPECT().
					DeleteHistoryBranch(gomock.Any(), gomock.Any()).
					Return(errors.New("persistence error")).Times(1)
			},
			request: &DeleteHistoryBranchRequest{
				BranchToken: []byte("branch-token"),
				ShardID:     common.Ptr(10),
			},
			expectError:   true,
			expectedError: "persistence error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			historyManager, mockStore, _, mockEncoder := setUpMocksForHistoryV2Manager(t)

			tc.setupMock(mockStore, mockEncoder)

			// Call the method
			err := historyManager.DeleteHistoryBranch(context.Background(), tc.request)

			// Validate the result
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetHistoryTree(t *testing.T) {
	testCases := []struct {
		name          string
		setupMock     func(*MockHistoryStore, *codec.MockBinaryEncoder)
		request       *GetHistoryTreeRequest
		expectError   bool
		expectedError string
		expected      *GetHistoryTreeResponse
	}{
		{
			name: "success",
			setupMock: func(mockStore *MockHistoryStore, mockEncoder *codec.MockBinaryEncoder) {
				mockEncoder.EXPECT().
					Decode([]byte("branch-token"), &workflow.HistoryBranch{}).DoAndReturn(func(data []byte, value *workflow.HistoryBranch) error {
					value.TreeID = common.Ptr("tree-id")
					return nil
				}).Times(1)
				mockStore.EXPECT().
					GetHistoryTree(gomock.Any(), &InternalGetHistoryTreeRequest{
						TreeID:      "tree-id",
						ShardID:     common.Ptr(10),
						BranchToken: []byte("branch-token"),
					}).
					Return(&InternalGetHistoryTreeResponse{
						Branches: []*types.HistoryBranch{
							{
								TreeID:   "tree-id",
								BranchID: "branch-id",
							},
						},
					}, nil).Times(1)
			},
			request: &GetHistoryTreeRequest{
				BranchToken: []byte("branch-token"),
				ShardID:     common.Ptr(10),
			},
			expectError: false,
			expected: &GetHistoryTreeResponse{
				Branches: []*workflow.HistoryBranch{
					{
						TreeID:   common.Ptr("tree-id"),
						BranchID: common.Ptr("branch-id"),
					},
				},
			},
		},
		{
			name: "decode error",
			setupMock: func(mockStore *MockHistoryStore, mockEncoder *codec.MockBinaryEncoder) {
				mockEncoder.EXPECT().
					Decode(gomock.Any(), gomock.Any()).
					Return(errors.New("decode error")).Times(1)
			},
			request: &GetHistoryTreeRequest{
				BranchToken: []byte("branch-token"),
			},
			expectError:   true,
			expectedError: "decode error",
		},
		{
			name: "persistence error",
			setupMock: func(mockStore *MockHistoryStore, mockEncoder *codec.MockBinaryEncoder) {
				mockStore.EXPECT().
					GetHistoryTree(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("persistence error")).Times(1)
			},
			request: &GetHistoryTreeRequest{
				TreeID: "tree-id",
			},
			expectError:   true,
			expectedError: "persistence error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			historyManager, mockStore, _, mockEncoder := setUpMocksForHistoryV2Manager(t)

			tc.setupMock(mockStore, mockEncoder)

			// Call the method
			resp, err := historyManager.GetHistoryTree(context.Background(), tc.request)

			// Validate the result
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, resp)
			}
		})
	}
}

func TestAppendHistoryNodes(t *testing.T) {
	testCases := []struct {
		name          string
		setupMock     func(*MockHistoryStore, *codec.MockBinaryEncoder, *MockPayloadSerializer)
		request       *AppendHistoryNodesRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			setupMock: func(mockStore *MockHistoryStore, mockEncoder *codec.MockBinaryEncoder, mockSerializer *MockPayloadSerializer) {
				mockEncoder.EXPECT().
					Decode([]byte("branch-token"), &workflow.HistoryBranch{}).DoAndReturn(func(data []byte, value *workflow.HistoryBranch) error {
					value.TreeID = common.Ptr("tree-id")
					value.BranchID = common.Ptr("branch-id")
					return nil
				}).Times(1)
				mockSerializer.EXPECT().
					SerializeBatchEvents(gomock.Any(), gomock.Any()).
					Return(&DataBlob{Encoding: common.EncodingTypeThriftRW, Data: []byte("events")}, nil).Times(1)
				mockStore.EXPECT().
					AppendHistoryNodes(gomock.Any(), gomock.Any()).
					Return(nil).Times(1)
			},
			request: &AppendHistoryNodesRequest{
				BranchToken:   []byte("branch-token"),
				Events:        []*types.HistoryEvent{{ID: 1, Version: 1}},
				TransactionID: 1234,
				ShardID:       common.Ptr(10),
			},
			expectError: false,
		},
		{
			name: "shardID not set error",
			setupMock: func(mockStore *MockHistoryStore, mockEncoder *codec.MockBinaryEncoder, mockSerializer *MockPayloadSerializer) {
				// No mock interactions for this invalid input test
			},
			request: &AppendHistoryNodesRequest{
				BranchToken: []byte("branch-token"),
				Events:      []*types.HistoryEvent{}, // No events
			},
			expectError:   true,
			expectedError: "shardID is not set for persistence operation",
		},
		{
			name: "empty events error",
			setupMock: func(mockStore *MockHistoryStore, mockEncoder *codec.MockBinaryEncoder, mockSerializer *MockPayloadSerializer) {
				// No mock interactions for this invalid input test
			},
			request: &AppendHistoryNodesRequest{
				BranchToken: []byte("branch-token"),
				Events:      []*types.HistoryEvent{}, // No events
				ShardID:     common.Ptr(10),
			},
			expectError:   true,
			expectedError: "events to be appended cannot be empty",
		},
		{
			name: "event ID less than 1 error",
			setupMock: func(mockStore *MockHistoryStore, mockEncoder *codec.MockBinaryEncoder, mockSerializer *MockPayloadSerializer) {
				// No mock interactions for this invalid input test
			},
			request: &AppendHistoryNodesRequest{
				BranchToken: []byte("branch-token"),
				Events:      []*types.HistoryEvent{{ID: 0, Version: 1}},
				ShardID:     common.Ptr(10),
			},
			expectError:   true,
			expectedError: "eventID cannot be less than 1",
		},
		{
			name: "event version not the same error",
			setupMock: func(mockStore *MockHistoryStore, mockEncoder *codec.MockBinaryEncoder, mockSerializer *MockPayloadSerializer) {
				// No mock interactions for this invalid input test
			},
			request: &AppendHistoryNodesRequest{
				BranchToken: []byte("branch-token"),
				Events:      []*types.HistoryEvent{{ID: 1, Version: 1}, {ID: 2, Version: 2}},
				ShardID:     common.Ptr(10),
			},
			expectError:   true,
			expectedError: "event version must be the same inside a batch",
		},
		{
			name: "event ID not continuous",
			setupMock: func(mockStore *MockHistoryStore, mockEncoder *codec.MockBinaryEncoder, mockSerializer *MockPayloadSerializer) {
				// No mock interactions for this invalid input test
			},
			request: &AppendHistoryNodesRequest{
				BranchToken: []byte("branch-token"),
				Events:      []*types.HistoryEvent{{ID: 1, Version: 1}, {ID: 3, Version: 1}},
				ShardID:     common.Ptr(10),
			},
			expectError:   true,
			expectedError: "event ID must be continuous",
		},
		{
			name: "decode error",
			setupMock: func(mockStore *MockHistoryStore, mockEncoder *codec.MockBinaryEncoder, mockSerializer *MockPayloadSerializer) {
				mockEncoder.EXPECT().
					Decode(gomock.Any(), gomock.Any()).
					Return(errors.New("decode error")).Times(1)
			},
			request: &AppendHistoryNodesRequest{
				BranchToken:   []byte("branch-token"),
				Events:        []*types.HistoryEvent{{ID: 1, Version: 1}},
				TransactionID: 1234,
				ShardID:       common.Ptr(10),
			},
			expectError:   true,
			expectedError: "decode error",
		},
		{
			name: "serialization error",
			setupMock: func(mockStore *MockHistoryStore, mockEncoder *codec.MockBinaryEncoder, mockSerializer *MockPayloadSerializer) {
				mockEncoder.EXPECT().
					Decode(gomock.Any(), gomock.Any()).
					Return(nil).Times(1)
				mockSerializer.EXPECT().
					SerializeBatchEvents(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("serialization error")).Times(1)
			},
			request: &AppendHistoryNodesRequest{
				BranchToken:   []byte("branch-token"),
				Events:        []*types.HistoryEvent{{ID: 1, Version: 1}},
				TransactionID: 1234,
				ShardID:       common.Ptr(10),
			},
			expectError:   true,
			expectedError: "serialization error",
		},
		{
			name: "transaction size limit error",
			setupMock: func(mockStore *MockHistoryStore, mockEncoder *codec.MockBinaryEncoder, mockSerializer *MockPayloadSerializer) {
				mockEncoder.EXPECT().
					Decode([]byte("branch-token"), &workflow.HistoryBranch{}).DoAndReturn(func(data []byte, value *workflow.HistoryBranch) error {
					value.TreeID = common.Ptr("tree-id")
					value.BranchID = common.Ptr("branch-id")
					return nil
				}).Times(1)
				mockSerializer.EXPECT().
					SerializeBatchEvents(gomock.Any(), gomock.Any()).
					Return(&DataBlob{Encoding: common.EncodingTypeThriftRW, Data: make([]byte, 1024*10+1)}, nil).Times(1)
			},
			request: &AppendHistoryNodesRequest{
				BranchToken:   []byte("branch-token"),
				Events:        []*types.HistoryEvent{{ID: 1, Version: 1}},
				TransactionID: 1234,
				ShardID:       common.Ptr(10),
			},
			expectError:   true,
			expectedError: "transaction size of 10241 bytes exceeds limit of 10240 bytes",
		},
		{
			name: "persistence error",
			setupMock: func(mockStore *MockHistoryStore, mockEncoder *codec.MockBinaryEncoder, mockSerializer *MockPayloadSerializer) {
				mockEncoder.EXPECT().
					Decode([]byte("branch-token"), &workflow.HistoryBranch{}).DoAndReturn(func(data []byte, value *workflow.HistoryBranch) error {
					value.TreeID = common.Ptr("tree-id")
					value.BranchID = common.Ptr("branch-id")
					return nil
				}).Times(1)
				mockSerializer.EXPECT().
					SerializeBatchEvents(gomock.Any(), gomock.Any()).
					Return(&DataBlob{Encoding: common.EncodingTypeThriftRW, Data: []byte("events")}, nil).Times(1)
				mockStore.EXPECT().
					AppendHistoryNodes(gomock.Any(), gomock.Any()).
					Return(errors.New("persistence error")).Times(1)
			},
			request: &AppendHistoryNodesRequest{
				BranchToken:   []byte("branch-token"),
				Events:        []*types.HistoryEvent{{ID: 1, Version: 1}},
				TransactionID: 1234,
				ShardID:       common.Ptr(10),
			},
			expectError:   true,
			expectedError: "persistence error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			historyManager, mockStore, mockSerializer, mockEncoder := setUpMocksForHistoryV2Manager(t)
			tc.setupMock(mockStore, mockEncoder, mockSerializer)

			// Call the method
			_, err := historyManager.AppendHistoryNodes(context.Background(), tc.request)

			// Validate the result
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetAllHistoryTreeBranches(t *testing.T) {
	testCases := []struct {
		name          string
		setupMock     func(*MockHistoryStore)
		request       *GetAllHistoryTreeBranchesRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			setupMock: func(mockStore *MockHistoryStore) {
				mockStore.EXPECT().
					GetAllHistoryTreeBranches(gomock.Any(), gomock.Any()).
					Return(&GetAllHistoryTreeBranchesResponse{
						Branches: []HistoryBranchDetail{
							{TreeID: "tree-id-1", BranchID: "branch-id-1"},
						},
					}, nil).Times(1)
			},
			request: &GetAllHistoryTreeBranchesRequest{
				PageSize:      10,
				NextPageToken: []byte("next-token"),
			},
			expectError: false,
		},
		{
			name: "persistence error",
			setupMock: func(mockStore *MockHistoryStore) {
				mockStore.EXPECT().
					GetAllHistoryTreeBranches(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("persistence error")).Times(1)
			},
			request: &GetAllHistoryTreeBranchesRequest{
				PageSize:      10,
				NextPageToken: []byte("next-token"),
			},
			expectError:   true,
			expectedError: "persistence error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			historyManager, mockStore, _, _ := setUpMocksForHistoryV2Manager(t)

			tc.setupMock(mockStore)

			// Call the method
			resp, err := historyManager.GetAllHistoryTreeBranches(context.Background(), tc.request)

			// Validate the result
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, "tree-id-1", resp.Branches[0].TreeID)
			}
		})
	}
}

func TestReadRawHistoryBranch(t *testing.T) {
	testCases := []struct {
		name            string
		setupMock       func(*MockHistoryStore, *codec.MockBinaryEncoder)
		fakeSerialize   func(*historyV2PagingToken) ([]byte, error)
		fakeDeserialize func([]byte, int64) (*historyV2PagingToken, error)
		request         *ReadHistoryBranchRequest
		expectError     bool
		expectedError   string
		expectedBlobs   []*DataBlob
		expectedToken   *historyV2PagingToken
	}{
		{
			name: "success case",
			setupMock: func(mockStore *MockHistoryStore, mockEncoder *codec.MockBinaryEncoder) {
				mockEncoder.EXPECT().
					Decode([]byte("branch-token"), &workflow.HistoryBranch{}).DoAndReturn(func(data []byte, value *workflow.HistoryBranch) error {
					value.TreeID = common.Ptr("tree-id")
					value.BranchID = common.Ptr("branch-id")
					value.Ancestors = []*workflow.HistoryBranchRange{
						{
							BranchID:    common.StringPtr("ancestor-branch-id"),
							BeginNodeID: common.Int64Ptr(1),
							EndNodeID:   common.Int64Ptr(10),
						},
						{
							BranchID:    common.StringPtr("ancestor-branch-id-2"),
							BeginNodeID: common.Int64Ptr(10),
							EndNodeID:   common.Int64Ptr(20),
						},
						{
							BranchID:    common.StringPtr("ancestor-branch-id-3"),
							BeginNodeID: common.Int64Ptr(20),
							EndNodeID:   common.Int64Ptr(30),
						},
					}
					return nil
				}).Times(1)
				mockStore.EXPECT().
					ReadHistoryBranch(gomock.Any(), &InternalReadHistoryBranchRequest{
						TreeID:            "tree-id",
						BranchID:          "ancestor-branch-id-2",
						MinNodeID:         10,
						MaxNodeID:         19,
						NextPageToken:     []byte("store-token"),
						LastNodeID:        9,
						LastTransactionID: 10,
						ShardID:           1,
						PageSize:          10,
					}).
					Return(&InternalReadHistoryBranchResponse{
						History: []*DataBlob{
							{Data: []byte("history-event-data")},
						},
						NextPageToken:     []byte("next-token"),
						LastNodeID:        999,
						LastTransactionID: 1000,
					}, nil).Times(1)
			},
			fakeDeserialize: func(data []byte, lastEventID int64) (*historyV2PagingToken, error) {
				assert.Equal(t, []byte("token"), data)
				assert.Equal(t, int64(9), lastEventID)
				return &historyV2PagingToken{
					LastNodeID:        lastEventID,
					LastTransactionID: 10,
					CurrentRangeIndex: notStartedIndex,
					StoreToken:        []byte("store-token"),
				}, nil
			},
			request: &ReadHistoryBranchRequest{
				BranchToken:   []byte("branch-token"),
				ShardID:       common.IntPtr(1),
				PageSize:      10,
				MinEventID:    10,
				MaxEventID:    19,
				NextPageToken: []byte("token"),
			},
			expectError: false,
			expectedBlobs: []*DataBlob{
				{Data: []byte("history-event-data")},
			},
			expectedToken: &historyV2PagingToken{
				LastNodeID:        999,
				LastTransactionID: 1000,
				CurrentRangeIndex: 1,
				FinalRangeIndex:   1,
				StoreToken:        []byte("next-token"),
			},
		},
		{
			name: "invalid shard ID",
			setupMock: func(mockStore *MockHistoryStore, mockEncoder *codec.MockBinaryEncoder) {
				// No encoder or store interactions, invalid ShardID is set.
			},
			request: &ReadHistoryBranchRequest{
				BranchToken:   []byte("branch-token"),
				ShardID:       nil, // Invalid shardID
				PageSize:      10,
				MinEventID:    1,
				MaxEventID:    100,
				NextPageToken: []byte{},
			},
			expectError:   true,
			expectedError: "shardID is not set for persistence operation",
		},
		{
			name: "invalid pagination params",
			setupMock: func(mockStore *MockHistoryStore, mockEncoder *codec.MockBinaryEncoder) {
				// No encoder or store interactions, invalid pagination params.
			},
			request: &ReadHistoryBranchRequest{
				BranchToken:   []byte("branch-token"),
				ShardID:       common.IntPtr(1),
				PageSize:      0, // Invalid PageSize
				MinEventID:    100,
				MaxEventID:    100,
				NextPageToken: []byte{},
			},
			expectError:   true,
			expectedError: "no events can be found for pageSize 0, minEventID 100, maxEventID: 100",
		},
		{
			name: "deserialize error",
			setupMock: func(mockStore *MockHistoryStore, mockEncoder *codec.MockBinaryEncoder) {
			},
			fakeDeserialize: func(data []byte, lastEventID int64) (*historyV2PagingToken, error) {
				return nil, errors.New("deserialize error")
			},
			request: &ReadHistoryBranchRequest{
				BranchToken:   []byte("branch-token"),
				ShardID:       common.IntPtr(1),
				PageSize:      10,
				MinEventID:    1,
				MaxEventID:    100,
				NextPageToken: []byte{},
			},
			expectError:   true,
			expectedError: "deserialize error",
		},
		{
			name: "decode error",
			setupMock: func(mockStore *MockHistoryStore, mockEncoder *codec.MockBinaryEncoder) {
				mockEncoder.EXPECT().
					Decode(gomock.Any(), gomock.Any()).
					Return(errors.New("decode error")).Times(1)
			},
			request: &ReadHistoryBranchRequest{
				BranchToken:   []byte("branch-token"),
				ShardID:       common.IntPtr(1),
				PageSize:      10,
				MinEventID:    1,
				MaxEventID:    100,
				NextPageToken: []byte{},
			},
			expectError:   true,
			expectedError: "decode error",
		},
		{
			name: "persistence error",
			setupMock: func(mockStore *MockHistoryStore, mockEncoder *codec.MockBinaryEncoder) {
				mockEncoder.EXPECT().
					Decode([]byte("branch-token"), &workflow.HistoryBranch{}).DoAndReturn(func(data []byte, value *workflow.HistoryBranch) error {
					value.TreeID = common.Ptr("tree-id")
					value.BranchID = common.Ptr("branch-id")
					return nil
				}).Times(1)
				mockStore.EXPECT().
					ReadHistoryBranch(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("persistence error")).Times(1)
			},
			request: &ReadHistoryBranchRequest{
				BranchToken:   []byte("branch-token"),
				ShardID:       common.IntPtr(1),
				PageSize:      10,
				MinEventID:    1,
				MaxEventID:    100,
				NextPageToken: []byte{},
			},
			expectError:   true,
			expectedError: "persistence error",
		},
		{
			name: "empty history error",
			setupMock: func(mockStore *MockHistoryStore, mockEncoder *codec.MockBinaryEncoder) {
				mockEncoder.EXPECT().
					Decode([]byte("branch-token"), &workflow.HistoryBranch{}).DoAndReturn(func(data []byte, value *workflow.HistoryBranch) error {
					value.TreeID = common.Ptr("tree-id")
					value.BranchID = common.Ptr("branch-id")
					return nil
				}).Times(1)
				mockStore.EXPECT().
					ReadHistoryBranch(gomock.Any(), gomock.Any()).
					Return(&InternalReadHistoryBranchResponse{
						History:       []*DataBlob{},
						NextPageToken: []byte{},
					}, nil).Times(1)
			},
			request: &ReadHistoryBranchRequest{
				BranchToken:   []byte("branch-token"),
				ShardID:       common.IntPtr(1),
				PageSize:      10,
				MinEventID:    1,
				MaxEventID:    100,
				NextPageToken: []byte{},
			},
			expectError:   true,
			expectedError: "Workflow execution history not found.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			historyManager, mockStore, _, mockEncoder := setUpMocksForHistoryV2Manager(t)
			if tc.fakeSerialize != nil {
				historyManager.serializeTokenFn = tc.fakeSerialize
			}
			if tc.fakeDeserialize != nil {
				historyManager.deserializeTokenFn = tc.fakeDeserialize
			}

			tc.setupMock(mockStore, mockEncoder)

			// Call the method
			blobs, token, _, _, err := historyManager.readRawHistoryBranch(context.Background(), tc.request)

			// Validate the result
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedBlobs, blobs)
				assert.Equal(t, tc.expectedToken, token)
			}
		})
	}
}

func TestReadHistoryBranch(t *testing.T) {
	testCases := []struct {
		name               string
		setupMock          func(*MockPayloadSerializer)
		fakeReadRaw        func(ctx context.Context, request *ReadHistoryBranchRequest) ([]*DataBlob, *historyV2PagingToken, int, log.Logger, error)
		fakeSerializeToken func(pagingToken *historyV2PagingToken) ([]byte, error)
		byBatch            bool
		request            *ReadHistoryBranchRequest
		expectError        bool
		expectedError      string
		expectedEvents     []*types.HistoryEvent
		expectedBatch      []*types.History
	}{
		{
			name: "success by events",
			setupMock: func(mockSerializer *MockPayloadSerializer) {
				mockSerializer.EXPECT().
					DeserializeBatchEvents(&DataBlob{Data: []byte("history-event-data")}).
					Return([]*types.HistoryEvent{
						{ID: 1, Version: 1},
						{ID: 2, Version: 1},
					}, nil).Times(1)
				mockSerializer.EXPECT().
					DeserializeBatchEvents(&DataBlob{Data: []byte("history-event-data2")}).
					Return([]*types.HistoryEvent{
						{ID: 0, Version: 2},
					}, nil).Times(1)
				mockSerializer.EXPECT().
					DeserializeBatchEvents(&DataBlob{Data: []byte("history-event-data3")}).
					Return([]*types.HistoryEvent{
						{ID: 1, Version: 2},
					}, nil).Times(1)
			},
			fakeReadRaw: func(ctx context.Context, request *ReadHistoryBranchRequest) ([]*DataBlob, *historyV2PagingToken, int, log.Logger, error) {
				return []*DataBlob{
					{Data: []byte("history-event-data")},
					{Data: []byte("history-event-data2")},
					{Data: []byte("history-event-data3")},
				}, &historyV2PagingToken{LastEventVersion: 2, LastEventID: 0}, 100, log.NewNoop(), nil
			},
			fakeSerializeToken: func(pagingToken *historyV2PagingToken) ([]byte, error) {
				assert.Equal(t, &historyV2PagingToken{
					LastEventID:      1,
					LastEventVersion: 2,
				}, pagingToken)
				return []byte("next-page-token"), nil
			},
			byBatch: false,
			request: &ReadHistoryBranchRequest{
				BranchToken: []byte("branch-token"),
				PageSize:    10,
				MinEventID:  1,
				MaxEventID:  100,
			},
			expectError: false,
			expectedEvents: []*types.HistoryEvent{
				{ID: 1, Version: 2},
			},
		},
		{
			name: "success by batch",
			setupMock: func(mockSerializer *MockPayloadSerializer) {
				mockSerializer.EXPECT().
					DeserializeBatchEvents(&DataBlob{Data: []byte("history-event-data")}).
					Return([]*types.HistoryEvent{
						{ID: 1, Version: 1},
						{ID: 2, Version: 1},
					}, nil).Times(1)
				mockSerializer.EXPECT().
					DeserializeBatchEvents(&DataBlob{Data: []byte("history-event-data2")}).
					Return([]*types.HistoryEvent{
						{ID: 0, Version: 2},
					}, nil).Times(1)
				mockSerializer.EXPECT().
					DeserializeBatchEvents(&DataBlob{Data: []byte("history-event-data3")}).
					Return([]*types.HistoryEvent{
						{ID: 1, Version: 2},
					}, nil).Times(1)
			},
			fakeReadRaw: func(ctx context.Context, request *ReadHistoryBranchRequest) ([]*DataBlob, *historyV2PagingToken, int, log.Logger, error) {
				return []*DataBlob{
					{Data: []byte("history-event-data")},
					{Data: []byte("history-event-data2")},
					{Data: []byte("history-event-data3")},
				}, &historyV2PagingToken{LastEventVersion: 2, LastEventID: 0}, 100, log.NewNoop(), nil
			},
			fakeSerializeToken: func(pagingToken *historyV2PagingToken) ([]byte, error) {
				assert.Equal(t, &historyV2PagingToken{
					LastEventID:      1,
					LastEventVersion: 2,
				}, pagingToken)
				return []byte("next-page-token"), nil
			},
			byBatch: true,
			request: &ReadHistoryBranchRequest{
				BranchToken: []byte("branch-token"),
				PageSize:    10,
				MinEventID:  1,
				MaxEventID:  100,
			},
			expectError: false,
			expectedBatch: []*types.History{
				{Events: []*types.HistoryEvent{{ID: 1, Version: 2}}},
			},
		},
		{
			name: "empty batch error",
			setupMock: func(mockSerializer *MockPayloadSerializer) {
				mockSerializer.EXPECT().
					DeserializeBatchEvents(gomock.Any()).
					Return([]*types.HistoryEvent{}, nil).Times(1)
			},
			fakeReadRaw: func(ctx context.Context, request *ReadHistoryBranchRequest) ([]*DataBlob, *historyV2PagingToken, int, log.Logger, error) {
				return []*DataBlob{
					{Data: []byte("history-event-data")},
				}, &historyV2PagingToken{LastEventVersion: 1, LastEventID: 0}, 100, log.NewNoop(), nil
			},
			fakeSerializeToken: func(pagingToken *historyV2PagingToken) ([]byte, error) {
				return nil, nil // Doesn't matter in this case since it errors earlier
			},
			byBatch:       false,
			request:       &ReadHistoryBranchRequest{BranchToken: []byte("branch-token"), PageSize: 10, MinEventID: 1, MaxEventID: 100},
			expectError:   true,
			expectedError: "corrupted history event batch, empty events",
		},
		{
			name: "corrupted event IDs",
			setupMock: func(mockSerializer *MockPayloadSerializer) {
				mockSerializer.EXPECT().
					DeserializeBatchEvents(gomock.Any()).
					Return([]*types.HistoryEvent{
						{ID: 1, Version: 1},
						{ID: 3, Version: 1}, // Non-continuous ID
					}, nil).Times(1)
			},
			fakeReadRaw: func(ctx context.Context, request *ReadHistoryBranchRequest) ([]*DataBlob, *historyV2PagingToken, int, log.Logger, error) {
				return []*DataBlob{
					{Data: []byte("history-event-data")},
				}, &historyV2PagingToken{LastEventVersion: 1, LastEventID: 0}, 100, log.NewNoop(), nil
			},
			fakeSerializeToken: func(pagingToken *historyV2PagingToken) ([]byte, error) {
				return nil, nil // Doesn't matter in this case since it errors earlier
			},
			byBatch:       false,
			request:       &ReadHistoryBranchRequest{BranchToken: []byte("branch-token"), PageSize: 10, MinEventID: 1, MaxEventID: 100},
			expectError:   true,
			expectedError: "corrupted history event batch, wrong version and IDs",
		},
		{
			name: "corrupted event batch",
			setupMock: func(mockSerializer *MockPayloadSerializer) {
				mockSerializer.EXPECT().
					DeserializeBatchEvents(gomock.Any()).
					Return([]*types.HistoryEvent{
						{ID: 7, Version: 5},
						{ID: 8, Version: 5},
					}, nil).Times(1)
			},
			fakeReadRaw: func(ctx context.Context, request *ReadHistoryBranchRequest) ([]*DataBlob, *historyV2PagingToken, int, log.Logger, error) {
				return []*DataBlob{
					{Data: []byte("history-event-data")},
				}, &historyV2PagingToken{LastEventVersion: 2, LastEventID: 5}, 100, log.NewNoop(), nil // Higher version than event batch
			},
			fakeSerializeToken: func(pagingToken *historyV2PagingToken) ([]byte, error) {
				return []byte("next-page-token"), nil
			},
			byBatch:       false,
			request:       &ReadHistoryBranchRequest{BranchToken: []byte("branch-token"), PageSize: 10, MinEventID: 1, MaxEventID: 100},
			expectError:   true, // No error, but events are skipped
			expectedError: "corrupted history event batch, eventID is not continuous",
		},
		{
			name: "read raw error",
			setupMock: func(mockSerializer *MockPayloadSerializer) {
				// No DeserializeBatchEvents calls because readRawHistoryBranchFn fails before deserialization
			},
			fakeReadRaw: func(ctx context.Context, request *ReadHistoryBranchRequest) ([]*DataBlob, *historyV2PagingToken, int, log.Logger, error) {
				return nil, nil, 0, nil, errors.New("read raw error")
			},
			fakeSerializeToken: func(pagingToken *historyV2PagingToken) ([]byte, error) {
				return nil, nil // Doesn't matter in this case since it errors earlier
			},
			byBatch:       false,
			request:       &ReadHistoryBranchRequest{BranchToken: []byte("branch-token"), PageSize: 10, MinEventID: 1, MaxEventID: 100},
			expectError:   true,
			expectedError: "read raw error",
		},
		{
			name: "serialize token error",
			setupMock: func(mockSerializer *MockPayloadSerializer) {
				mockSerializer.EXPECT().
					DeserializeBatchEvents(gomock.Any()).
					Return([]*types.HistoryEvent{
						{ID: 1, Version: 1},
						{ID: 2, Version: 1},
					}, nil).Times(1)
			},
			fakeReadRaw: func(ctx context.Context, request *ReadHistoryBranchRequest) ([]*DataBlob, *historyV2PagingToken, int, log.Logger, error) {
				return []*DataBlob{
					{Data: []byte("history-event-data")},
				}, &historyV2PagingToken{LastEventVersion: 1, LastEventID: 0}, 100, log.NewNoop(), nil
			},
			fakeSerializeToken: func(pagingToken *historyV2PagingToken) ([]byte, error) {
				return nil, errors.New("serialize token error")
			},
			byBatch:       false,
			request:       &ReadHistoryBranchRequest{BranchToken: []byte("branch-token"), PageSize: 10, MinEventID: 1, MaxEventID: 100},
			expectError:   true,
			expectedError: "serialize token error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up mocks
			historyManager, _, mockSerializer, _ := setUpMocksForHistoryV2Manager(t)

			// Fake readRawHistoryBranchFn and serializeTokenFn
			historyManager.readRawHistoryBranchFn = tc.fakeReadRaw
			historyManager.serializeTokenFn = tc.fakeSerializeToken

			tc.setupMock(mockSerializer)

			// Call the method
			historyEvents, historyBatches, _, _, _, err := historyManager.readHistoryBranch(context.Background(), tc.byBatch, tc.request)

			// Validate the result
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
				if tc.byBatch {
					assert.Equal(t, tc.expectedBatch, historyBatches)
				} else {
					assert.Equal(t, tc.expectedEvents, historyEvents)
				}
			}
		})
	}
}

func TestReadRawHistoryBranchMethod(t *testing.T) {
	testCases := []struct {
		name               string
		setupMock          func()
		fakeReadRaw        func(ctx context.Context, request *ReadHistoryBranchRequest) ([]*DataBlob, *historyV2PagingToken, int, log.Logger, error)
		fakeSerializeToken func(pagingToken *historyV2PagingToken) ([]byte, error)
		request            *ReadHistoryBranchRequest
		expectError        bool
		expectedError      string
		expectedResponse   *ReadRawHistoryBranchResponse
	}{
		{
			name: "success",
			setupMock: func() {
				// No additional mock setup needed
			},
			fakeReadRaw: func(ctx context.Context, request *ReadHistoryBranchRequest) ([]*DataBlob, *historyV2PagingToken, int, log.Logger, error) {
				return []*DataBlob{
					{Data: []byte("history-event-blob")},
				}, &historyV2PagingToken{LastEventVersion: 1, LastEventID: 0}, 100, nil, nil
			},
			fakeSerializeToken: func(pagingToken *historyV2PagingToken) ([]byte, error) {
				return []byte("next-page-token"), nil
			},
			request: &ReadHistoryBranchRequest{
				BranchToken:   []byte("branch-token"),
				PageSize:      10,
				MinEventID:    1,
				MaxEventID:    100,
				NextPageToken: []byte{},
			},
			expectError: false,
			expectedResponse: &ReadRawHistoryBranchResponse{
				HistoryEventBlobs: []*DataBlob{
					{Data: []byte("history-event-blob")},
				},
				NextPageToken: []byte("next-page-token"),
				Size:          100,
			},
		},
		{
			name: "error in readRawHistoryBranchFn",
			setupMock: func() {
				// No additional mock setup needed
			},
			fakeReadRaw: func(ctx context.Context, request *ReadHistoryBranchRequest) ([]*DataBlob, *historyV2PagingToken, int, log.Logger, error) {
				return nil, nil, 0, nil, errors.New("read error")
			},
			fakeSerializeToken: func(pagingToken *historyV2PagingToken) ([]byte, error) {
				return nil, nil // Won't be called due to error in readRawHistoryBranchFn
			},
			request: &ReadHistoryBranchRequest{
				BranchToken:   []byte("branch-token"),
				PageSize:      10,
				MinEventID:    1,
				MaxEventID:    100,
				NextPageToken: []byte{},
			},
			expectError:   true,
			expectedError: "read error",
		},
		{
			name: "error in serializeTokenFn",
			setupMock: func() {
				// No additional mock setup needed
			},
			fakeReadRaw: func(ctx context.Context, request *ReadHistoryBranchRequest) ([]*DataBlob, *historyV2PagingToken, int, log.Logger, error) {
				return []*DataBlob{
					{Data: []byte("history-event-blob")},
				}, &historyV2PagingToken{LastEventVersion: 1, LastEventID: 0}, 100, nil, nil
			},
			fakeSerializeToken: func(pagingToken *historyV2PagingToken) ([]byte, error) {
				return nil, errors.New("serialization error")
			},
			request: &ReadHistoryBranchRequest{
				BranchToken:   []byte("branch-token"),
				PageSize:      10,
				MinEventID:    1,
				MaxEventID:    100,
				NextPageToken: []byte{},
			},
			expectError:   true,
			expectedError: "serialization error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up the mock dependencies and fakes
			historyManager, _, _, _ := setUpMocksForHistoryV2Manager(t)
			historyManager.readRawHistoryBranchFn = tc.fakeReadRaw
			historyManager.serializeTokenFn = tc.fakeSerializeToken

			// Call the method
			resp, err := historyManager.ReadRawHistoryBranch(context.Background(), tc.request)

			// Validate the result
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResponse, resp)
			}
		})
	}
}

func TestReadHistoryBranchByBatch(t *testing.T) {
	testCases := []struct {
		name             string
		fakeReadHistory  func(ctx context.Context, byBatch bool, request *ReadHistoryBranchRequest) ([]*types.HistoryEvent, []*types.History, []byte, int, int64, error)
		request          *ReadHistoryBranchRequest
		expectError      bool
		expectedError    string
		expectedResponse *ReadHistoryBranchByBatchResponse
	}{
		{
			name: "success",
			fakeReadHistory: func(ctx context.Context, byBatch bool, request *ReadHistoryBranchRequest) ([]*types.HistoryEvent, []*types.History, []byte, int, int64, error) {
				return nil, []*types.History{
					{Events: []*types.HistoryEvent{
						{ID: 1, Version: 1},
						{ID: 2, Version: 1},
					}},
				}, []byte("next-page-token"), 100, 1, nil
			},
			request: &ReadHistoryBranchRequest{
				BranchToken:   []byte("branch-token"),
				PageSize:      10,
				MinEventID:    1,
				MaxEventID:    100,
				NextPageToken: []byte{},
			},
			expectError: false,
			expectedResponse: &ReadHistoryBranchByBatchResponse{
				History: []*types.History{
					{Events: []*types.HistoryEvent{
						{ID: 1, Version: 1},
						{ID: 2, Version: 1},
					}},
				},
				NextPageToken:    []byte("next-page-token"),
				Size:             100,
				LastFirstEventID: 1,
			},
		},
		{
			name: "error in readHistoryBranchFn",
			fakeReadHistory: func(ctx context.Context, byBatch bool, request *ReadHistoryBranchRequest) ([]*types.HistoryEvent, []*types.History, []byte, int, int64, error) {
				return nil, nil, nil, 0, 0, errors.New("read error")
			},
			request: &ReadHistoryBranchRequest{
				BranchToken:   []byte("branch-token"),
				PageSize:      10,
				MinEventID:    1,
				MaxEventID:    100,
				NextPageToken: []byte{},
			},
			expectError:   true,
			expectedError: "read error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up faked readHistoryBranchFn
			historyManager, _, _, _ := setUpMocksForHistoryV2Manager(t)
			historyManager.readHistoryBranchFn = tc.fakeReadHistory

			// Call the method
			resp, err := historyManager.ReadHistoryBranchByBatch(context.Background(), tc.request)

			// Validate the result
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResponse, resp)
			}
		})
	}
}

func TestReadHistoryBranchMethod(t *testing.T) {
	testCases := []struct {
		name             string
		fakeReadHistory  func(ctx context.Context, byBatch bool, request *ReadHistoryBranchRequest) ([]*types.HistoryEvent, []*types.History, []byte, int, int64, error)
		request          *ReadHistoryBranchRequest
		expectError      bool
		expectedError    string
		expectedResponse *ReadHistoryBranchResponse
	}{
		{
			name: "success",
			fakeReadHistory: func(ctx context.Context, byBatch bool, request *ReadHistoryBranchRequest) ([]*types.HistoryEvent, []*types.History, []byte, int, int64, error) {
				return []*types.HistoryEvent{
					{ID: 1, Version: 1},
					{ID: 2, Version: 1},
				}, nil, []byte("next-page-token"), 100, 1, nil
			},
			request: &ReadHistoryBranchRequest{
				BranchToken:   []byte("branch-token"),
				PageSize:      10,
				MinEventID:    1,
				MaxEventID:    100,
				NextPageToken: []byte{},
			},
			expectError: false,
			expectedResponse: &ReadHistoryBranchResponse{
				HistoryEvents: []*types.HistoryEvent{
					{ID: 1, Version: 1},
					{ID: 2, Version: 1},
				},
				NextPageToken:    []byte("next-page-token"),
				Size:             100,
				LastFirstEventID: 1,
			},
		},
		{
			name: "error in readHistoryBranchFn",
			fakeReadHistory: func(ctx context.Context, byBatch bool, request *ReadHistoryBranchRequest) ([]*types.HistoryEvent, []*types.History, []byte, int, int64, error) {
				return nil, nil, nil, 0, 0, errors.New("read error")
			},
			request: &ReadHistoryBranchRequest{
				BranchToken:   []byte("branch-token"),
				PageSize:      10,
				MinEventID:    1,
				MaxEventID:    100,
				NextPageToken: []byte{},
			},
			expectError:   true,
			expectedError: "read error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up faked readHistoryBranchFn
			historyManager, _, _, _ := setUpMocksForHistoryV2Manager(t)
			historyManager.readHistoryBranchFn = tc.fakeReadHistory

			// Call the method
			resp, err := historyManager.ReadHistoryBranch(context.Background(), tc.request)

			// Validate the result
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResponse, resp)
			}
		})
	}
}

func TestDeserializeToken(t *testing.T) {
	testCases := []struct {
		name               string
		inputData          []byte
		defaultLastEventID int64
		expectedToken      *historyV2PagingToken
		expectError        bool
		expectedError      string
	}{
		{
			name:               "empty data",
			inputData:          []byte{},
			defaultLastEventID: 100,
			expectedToken: &historyV2PagingToken{
				LastEventID:       100,
				LastEventVersion:  common.EmptyVersion,
				CurrentRangeIndex: notStartedIndex,
				LastNodeID:        defaultLastNodeID,
				LastTransactionID: defaultLastTransactionID,
			},
			expectError: false,
		},
		{
			name:               "valid token data",
			inputData:          []byte(`{"LastEventID":200,"LastEventVersion":1,"CurrentRangeIndex":2,"FinalRangeIndex":3,"LastNodeID":100,"LastTransactionID":500}`),
			defaultLastEventID: 100,
			expectedToken: &historyV2PagingToken{
				LastEventID:       200,
				LastEventVersion:  1,
				CurrentRangeIndex: 2,
				FinalRangeIndex:   3,
				LastNodeID:        100,
				LastTransactionID: 500,
			},
			expectError: false,
		},
		{
			name:               "invalid json data",
			inputData:          []byte("invalid-json"),
			defaultLastEventID: 100,
			expectedToken:      nil,
			expectError:        true,
			expectedError:      "invalid character 'i' looking for beginning of value",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			token, err := deserializeToken(tc.inputData, tc.defaultLastEventID)

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedToken, token)
			}
		})
	}
}

func TestSerializeToken(t *testing.T) {
	testCases := []struct {
		name           string
		inputToken     *historyV2PagingToken
		expectedOutput []byte
		expectError    bool
		expectedError  string
	}{
		{
			name: "empty store token, final page reached",
			inputToken: &historyV2PagingToken{
				StoreToken:        []byte{},
				CurrentRangeIndex: 2,
				FinalRangeIndex:   2,
			},
			expectedOutput: nil,
			expectError:    false,
		},
		{
			name: "empty store token, increment current range index",
			inputToken: &historyV2PagingToken{
				StoreToken:        []byte{},
				CurrentRangeIndex: 1,
				FinalRangeIndex:   2,
			},
			expectedOutput: []byte(`{"LastEventVersion":0,"LastEventID":0,"StoreToken":"","CurrentRangeIndex":2,"FinalRangeIndex":2,"LastNodeID":0,"LastTransactionID":0}`),
			expectError:    false,
		},
		{
			name: "non-empty store token",
			inputToken: &historyV2PagingToken{
				StoreToken:        []byte("non-empty-token"),
				CurrentRangeIndex: 1,
				FinalRangeIndex:   2,
			},
			expectedOutput: []byte(`{"LastEventVersion":0,"LastEventID":0,"StoreToken":"bm9uLWVtcHR5LXRva2Vu","CurrentRangeIndex":1,"FinalRangeIndex":2,"LastNodeID":0,"LastTransactionID":0}`),
			expectError:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := serializeToken(tc.inputToken)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedOutput, output)
				t.Log(string(output))
			}
		})
	}
}

func TestTokenRoundtrip(t *testing.T) {
	testCases := []struct {
		name       string
		inputToken *historyV2PagingToken
	}{
		{
			name: "empty store token, basic token",
			inputToken: &historyV2PagingToken{
				LastEventID:       100,
				LastEventVersion:  1,
				CurrentRangeIndex: 1,
				FinalRangeIndex:   2,
				LastNodeID:        200,
				LastTransactionID: 300,
				StoreToken:        []byte{},
			},
		},
		{
			name: "non-empty store token",
			inputToken: &historyV2PagingToken{
				LastEventID:       100,
				LastEventVersion:  1,
				CurrentRangeIndex: 1,
				FinalRangeIndex:   2,
				LastNodeID:        200,
				LastTransactionID: 300,
				StoreToken:        []byte("non-empty-token"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Step 1: Serialize the token
			serializedToken, err := serializeToken(tc.inputToken)
			assert.NoError(t, err)

			// Step 2: Deserialize the token back
			deserializedToken, err := deserializeToken(serializedToken, tc.inputToken.LastEventID)
			assert.NoError(t, err)

			// Step 3: Compare the original token with the deserialized token
			assert.Equal(t, tc.inputToken, deserializedToken)
		})
	}
}
