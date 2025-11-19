package tasklist

import (
	"sync/atomic"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/client/executorclient"
)

type shardProcessorImpl struct {
	Manager
	Status atomic.Int32
}

func NewShardProcessor(params ManagerParams) (ShardProcessor, error) {
	mgr, err := NewManager(params)
	if err != nil {
		return nil, err
	}
	return &shardProcessorImpl{Manager: mgr}, nil

}

func (sp *shardProcessorImpl) GetShardReport() executorclient.ShardReport {
	loadBalancerHints := sp.Manager.LoadBalancerHints()
	return executorclient.ShardReport{
		// For now reporting the load as queries per second (QPS) value.
		ShardLoad: loadBalancerHints.RatePerSecond,
		Status:    types.ShardStatus(sp.Status.Load()),
	}
}

func (sp *shardProcessorImpl) SetShardStatus(status types.ShardStatus) {
	sp.Status.Store(int32(status))
}
