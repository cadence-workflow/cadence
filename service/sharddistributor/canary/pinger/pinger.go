package pinger

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"

	sharddistributorv1 "github.com/uber/cadence/.gen/proto/sharddistributor/v1"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/clock"
)

//go:generate mockgen -package $GOPACKAGE -destination canary_client_mock.go github.com/uber/cadence/.gen/proto/sharddistributor/v1 ShardDistributorExecutorCanaryAPIYARPCClient

const (
	pingInterval    = 1 * time.Second
	pingJitterCoeff = 0.1 // 10% jitter
)

// Pinger periodically pings shard owners in the fixed namespace
type Pinger struct {
	logger       *zap.Logger
	timeSource   clock.TimeSource
	canaryClient sharddistributorv1.ShardDistributorExecutorCanaryAPIYARPCClient
	namespace    string
	numShards    int
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// Params are the parameters for creating a Pinger
type Params struct {
	fx.In

	Logger       *zap.Logger
	TimeSource   clock.TimeSource
	CanaryClient sharddistributorv1.ShardDistributorExecutorCanaryAPIYARPCClient
}

// NewPinger creates a new Pinger for the fixed namespace
func NewPinger(params Params, namespace string, numShards int) *Pinger {
	return &Pinger{
		logger:       params.Logger,
		timeSource:   params.TimeSource,
		canaryClient: params.CanaryClient,
		namespace:    namespace,
		numShards:    numShards,
	}
}

// Start begins the periodic ping loop
func (p *Pinger) Start(ctx context.Context) {
	p.logger.Info("Starting canary pinger", zap.String("namespace", p.namespace), zap.Int("num_shards", p.numShards))
	p.ctx, p.cancel = context.WithCancel(context.WithoutCancel(ctx))
	p.wg.Add(1)
	go p.pingLoop()
}

// Stop stops the ping loop
func (p *Pinger) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
	p.wg.Wait()
}

func (p *Pinger) pingLoop() {
	defer p.wg.Done()

	ticker := p.timeSource.NewTicker(backoff.JitDuration(pingInterval, pingJitterCoeff))
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			p.logger.Info("Pinger context done, stopping")
			return
		case <-ticker.Chan():
			p.pingRandomShard()
			ticker.Reset(backoff.JitDuration(pingInterval, pingJitterCoeff))
		}
	}
}

// Pings a random shard in the namespace and logs the results
func (p *Pinger) pingRandomShard() {
	shardNum := rand.Intn(p.numShards)
	shardKey := fmt.Sprintf("%d", shardNum)

	PingShard(p.ctx, p.canaryClient, p.logger, p.namespace, shardKey)
}
