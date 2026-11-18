package history

import (
	"errors"
	"fmt"

	"github.com/uber/cadence/common/asyncworkflow/queue/provider"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
)

type queueImpl struct {
	config *queueConfig
}

type noopConsumer struct{}

func newQueue(decoder provider.Decoder) (provider.Queue, error) {
	var out queueConfig
	if err := decoder.Decode(&out); err != nil {
		return nil, fmt.Errorf("bad config: %w", err)
	}
	return &queueImpl{
		config: &out,
	}, nil
}

func (q *queueImpl) ID() string {
	return q.config.ID()
}

func (q *queueImpl) CreateConsumer(p *provider.Params) (provider.Consumer, error) {
	p.Logger.Info("History-backed async workflow queues are consumed inside the history service")
	return &noopConsumer{}, nil
}

func (q *queueImpl) CreateProducer(p *provider.Params) (messaging.Producer, error) {
	if p.HistoryClient == nil {
		return nil, errors.New("history client is required")
	}
	if p.NumHistoryShards <= 0 {
		return nil, errors.New("number of history shards must be positive")
	}

	producer := newProducer(p.HistoryClient, p.NumHistoryShards, p.Logger)
	return messaging.NewMetricProducer(
		producer,
		p.MetricsClient,
		messaging.WithMetricTags(metrics.TopicTag("history")),
	), nil
}

func (c *noopConsumer) Start() error {
	return nil
}

func (c *noopConsumer) Stop() {}
