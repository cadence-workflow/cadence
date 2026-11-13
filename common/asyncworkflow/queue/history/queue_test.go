package history

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	historyclient "github.com/uber/cadence/client/history"
	"github.com/uber/cadence/common/asyncworkflow/queue/provider"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

func jsonConfig(data string) provider.Decoder {
	return newDecoder(&types.DataBlob{EncodingType: types.EncodingTypeJSON.Ptr(), Data: []byte(data)})
}

func TestNewQueue(t *testing.T) {
	tests := []struct {
		name    string
		decoder provider.Decoder
		wantErr bool
	}{
		{name: "empty json object", decoder: jsonConfig("{}")},
		{name: "unsupported encoding", decoder: newDecoder(&types.DataBlob{EncodingType: types.EncodingTypeThriftRW.Ptr(), Data: []byte("{}")}), wantErr: true},
		{name: "malformed json", decoder: jsonConfig("{"), wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q, err := newQueue(tc.decoder)
			if tc.wantErr {
				require.Error(t, err)
				assert.Nil(t, q)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "history", q.ID())
		})
	}
}

func TestCreateProducer(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := historyclient.NewMockClient(ctrl)
	q, err := newQueue(jsonConfig("{}"))
	require.NoError(t, err)

	tests := []struct {
		name    string
		params  *provider.Params
		wantErr bool
	}{
		{
			name:    "missing history client",
			params:  &provider.Params{Logger: testlogger.New(t), MetricsClient: metrics.NewNoopMetricsClient(), NumHistoryShards: 4},
			wantErr: true,
		},
		{
			name:    "non-positive shard count",
			params:  &provider.Params{Logger: testlogger.New(t), MetricsClient: metrics.NewNoopMetricsClient(), HistoryClient: client, NumHistoryShards: 0},
			wantErr: true,
		},
		{
			name:   "history client and shard count set",
			params: &provider.Params{Logger: testlogger.New(t), MetricsClient: metrics.NewNoopMetricsClient(), HistoryClient: client, NumHistoryShards: 4},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, err := q.CreateProducer(tc.params)
			if tc.wantErr {
				require.Error(t, err)
				assert.Nil(t, p)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, p)
		})
	}
}

func TestCreateConsumerIsNoop(t *testing.T) {
	q, err := newQueue(jsonConfig("{}"))
	require.NoError(t, err)
	c, err := q.CreateConsumer(&provider.Params{Logger: testlogger.New(t), MetricsClient: metrics.NewNoopMetricsClient()})
	require.NoError(t, err)
	assert.NoError(t, c.Start())
	assert.NotPanics(t, c.Stop)
}

func TestRegistration(t *testing.T) {
	_, ok := provider.GetQueueProvider("history")
	assert.True(t, ok)
	_, ok = provider.GetDecoder("history")
	assert.True(t, ok)
}
