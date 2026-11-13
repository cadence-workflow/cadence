package matching

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination tasklist_router_mock.go -self_package github.com/uber/cadence/client/matching

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/cadence-workflow/shard-manager/service/sharddistributor/client/spectatorclient"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/service"
)

const (
	routingPathHashRing         = "hash_ring"
	routingPathShardDistributor = "shard_distributor"
)

// TaskListRouter finds the matching host that owns a task list.
type TaskListRouter interface {
	FromTaskList(taskListName string) (membership.HostInfo, error)
}

type taskListRouter struct {
	resolver               membership.Resolver
	taskListDifferentiator *TaskListDifferentiator
	spectator              spectatorclient.Spectator
	logger                 log.Logger
	metricsScope           metrics.Scope
}

func NewTaskListRouter(
	resolver membership.Resolver,
	taskListDifferentiator *TaskListDifferentiator,
	spectator spectatorclient.Spectator,
	logger log.Logger,
	metricsScope metrics.Scope,
) TaskListRouter {
	if spectator == nil {
		logger.Warn("No shard distributor client, routing every task list to the hash ring")
	}
	return &taskListRouter{
		resolver:               resolver,
		taskListDifferentiator: taskListDifferentiator,
		spectator:              spectator,
		logger:                 logger,
		metricsScope:           metricsScope,
	}
}

func (r *taskListRouter) FromTaskList(taskListName string) (membership.HostInfo, error) {
	shard := r.taskListDifferentiator.TaskListShard(taskListName)
	if !shard.Onboarded || r.spectator == nil {
		r.emitRoutingPathMetric(routingPathHashRing)
		return r.resolver.Lookup(service.Matching, taskListName)
	}

	r.emitRoutingPathMetric(routingPathShardDistributor)
	return r.lookUpInShardDistributor(shard.ID)
}

// TODO: cache the hostinfos, creating them on every request is relatively expensive (we need to do string parsing etc.)
func (r *taskListRouter) lookUpInShardDistributor(key string) (membership.HostInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	owner, err := r.spectator.GetShardOwner(ctx, key)
	if err != nil {
		return membership.HostInfo{}, err
	}

	portMap := make(membership.PortMap)
	for _, portType := range []string{membership.PortTchannel, membership.PortGRPC} {
		portString, ok := owner.Metadata[portType]
		if !ok {
			r.logger.Warn(fmt.Sprintf("Port %s not found in metadata", portType), tag.Value(owner))
			continue
		}

		port, err := strconv.ParseUint(portString, 10, 16)
		if err != nil {
			r.logger.Warn(fmt.Sprintf("Failed to convert %s port to int", portType), tag.Error(err), tag.Value(owner))
			continue
		}
		// This is safe because the conversion above ensures the port is within the uint16 range
		portMap[portType] = uint16(port)
	}

	hostaddress := owner.Metadata["hostIP"]
	if hostaddress == "" {
		return membership.HostInfo{}, fmt.Errorf("hostIP not found in metadata")
	}

	address := net.JoinHostPort(hostaddress, strconv.Itoa(int(portMap[membership.PortTchannel])))

	hostInfo := membership.NewDetailedHostInfo(address, owner.ExecutorID, portMap)
	return hostInfo, nil
}

func (r *taskListRouter) emitRoutingPathMetric(path string) {
	r.metricsScope.Tagged(metrics.RoutingPathTag(path)).IncCounter(metrics.ShardDistributorResolverLookups)
}
