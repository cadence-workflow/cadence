package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	sharddistributorv1 "github.com/uber/cadence/.gen/proto/sharddistributor/v1"
	"github.com/uber/cadence/service/sharddistributor/canary/processor"
	"github.com/uber/cadence/service/sharddistributor/canary/processorephemeral"
	"github.com/uber/cadence/service/sharddistributor/client/executorclient"
)

func TestPingHandler_Ping(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		shardKey  string
		setup     func(*gomock.Controller) ([]executorclient.Executor[*processor.ShardProcessor], []executorclient.Executor[*processorephemeral.ShardProcessor])
		wantID    string
		wantOwns  bool
	}{
		{
			name:      "fixed executor owns shard",
			namespace: "ns1",
			shardKey:  "shard-1",
			setup: func(ctrl *gomock.Controller) ([]executorclient.Executor[*processor.ShardProcessor], []executorclient.Executor[*processorephemeral.ShardProcessor]) {
				exec := executorclient.NewMockExecutor[*processor.ShardProcessor](ctrl)
				exec.EXPECT().GetNamespace().Return("ns1").AnyTimes()
				exec.EXPECT().GetShardProcess(gomock.Any(), "shard-1").Return(&processor.ShardProcessor{}, nil)
				exec.EXPECT().GetMetadata().Return(map[string]string{"grpc_address": "127.0.0.1:7953"})
				return []executorclient.Executor[*processor.ShardProcessor]{exec}, nil
			},
			wantID:   "127.0.0.1:7953",
			wantOwns: true,
		},
		{
			name:      "fixed executor does not own shard",
			namespace: "ns1",
			shardKey:  "shard-2",
			setup: func(ctrl *gomock.Controller) ([]executorclient.Executor[*processor.ShardProcessor], []executorclient.Executor[*processorephemeral.ShardProcessor]) {
				exec := executorclient.NewMockExecutor[*processor.ShardProcessor](ctrl)
				exec.EXPECT().GetNamespace().Return("ns1").AnyTimes()
				exec.EXPECT().GetShardProcess(gomock.Any(), "shard-2").Return(nil, errors.New("not found"))
				exec.EXPECT().GetMetadata().Return(map[string]string{"grpc_address": "127.0.0.1:7954"})
				return []executorclient.Executor[*processor.ShardProcessor]{exec}, nil
			},
			wantID:   "127.0.0.1:7954",
			wantOwns: false,
		},
		{
			name:      "ephemeral executor owns shard",
			namespace: "ns2",
			shardKey:  "shard-3",
			setup: func(ctrl *gomock.Controller) ([]executorclient.Executor[*processor.ShardProcessor], []executorclient.Executor[*processorephemeral.ShardProcessor]) {
				exec := executorclient.NewMockExecutor[*processorephemeral.ShardProcessor](ctrl)
				exec.EXPECT().GetNamespace().Return("ns2").AnyTimes()
				exec.EXPECT().GetShardProcess(gomock.Any(), "shard-3").Return(&processorephemeral.ShardProcessor{}, nil)
				exec.EXPECT().GetMetadata().Return(map[string]string{"grpc_address": "127.0.0.1:7955"})
				return nil, []executorclient.Executor[*processorephemeral.ShardProcessor]{exec}
			},
			wantID:   "127.0.0.1:7955",
			wantOwns: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			fixed, ephemeral := tt.setup(ctrl)
			handler := NewPingHandler(Params{
				Logger:             zap.NewNop(),
				ExecutorsFixed:     fixed,
				ExecutorsEphemeral: ephemeral,
			})

			resp, err := handler.Ping(context.Background(), &sharddistributorv1.PingRequest{
				Namespace: tt.namespace,
				ShardKey:  tt.shardKey,
			})

			require.NoError(t, err)
			assert.Equal(t, tt.wantID, resp.ExecutorId)
			assert.Equal(t, tt.wantOwns, resp.OwnsShard)
			assert.Equal(t, tt.shardKey, resp.ShardKey)
		})
	}
}
