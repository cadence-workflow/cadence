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

package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/store"
)

const (
	_testNamespace = "test-matching"
)

func TestGetShardOwner(t *testing.T) {
	cfg := config.ShardDistribution{
		Enabled: true,
		Namespaces: []config.Namespace{
			{
				Name: _testNamespace,
			},
		},
	}

	tests := []struct {
		name           string
		request        *types.GetShardOwnerRequest
		setupMocks     func(mockStore *store.MockStore)
		expectedOwner  string
		expectedError  bool
		expectedErrMsg string
	}{
		{
			name: "Existing_Success",
			request: &types.GetShardOwnerRequest{
				Namespace: _testNamespace,
				ShardKey:  "123",
			},
			setupMocks: func(mockStore *store.MockStore) {
				mockStore.EXPECT().GetShardOwner(gomock.Any(), _testNamespace, "123").Return("owner1", nil)
			},
			expectedOwner: "owner1",
			expectedError: false,
		},

		{
			name: "InvalidNamespace",
			request: &types.GetShardOwnerRequest{
				Namespace: "namespace not found invalidNamespace",
				ShardKey:  "1",
			},
			expectedError:  true,
			expectedErrMsg: "namespace not found",
		},
		{
			name: "LookupError",
			request: &types.GetShardOwnerRequest{
				Namespace: _testNamespace,
				ShardKey:  "1",
			},
			setupMocks: func(mockStore *store.MockStore) {
				mockStore.EXPECT().GetShardOwner(gomock.Any(), _testNamespace, "1").Return("", errors.New("lookup error"))
			},
			expectedError:  true,
			expectedErrMsg: "lookup error",
		},
		{
			name: "ShardNotFound",
			request: &types.GetShardOwnerRequest{
				Namespace: _testNamespace,
				ShardKey:  "NON-EXISTING-SHARD",
			},
			setupMocks: func(mockStore *store.MockStore) {
				mockStore.EXPECT().GetShardOwner(gomock.Any(), _testNamespace, "NON-EXISTING-SHARD").Return("", store.ErrShardNotFound)
			},
			expectedError:  true,
			expectedErrMsg: "shard not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			logger := testlogger.New(t)
			mockStorage := store.NewMockStore(ctrl)

			handler := &handlerImpl{
				logger:               logger,
				shardDistributionCfg: cfg,
				storage:              mockStorage,
			}
			if tt.setupMocks != nil {
				tt.setupMocks(mockStorage)
			}
			resp, err := handler.GetShardOwner(context.Background(), tt.request)
			if tt.expectedError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedErrMsg)
				require.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedOwner, resp.Owner)
				require.Equal(t, tt.request.Namespace, resp.Namespace)
			}
		})
	}
}
