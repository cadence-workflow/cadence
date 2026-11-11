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

// TaskListRouter finds the matching host that owns a task list. A task list is owned either by the
// matching hash ring or by the shard distributor, and the router asks whichever one owns it.
type TaskListRouter interface {
	// FromTaskList returns the host that owns the task list.
	FromTaskList(taskListName string) (membership.HostInfo, error)
}

type taskListRouter struct {
	resolver               membership.Resolver
	taskListDifferentiator *TaskListDifferentiator
	spectator              spectatorclient.Spectator
	logger                 log.Logger
	metricsScope           metrics.Scope
}

// NewTaskListRouter creates a TaskListRouter. The spectator is nil when the shard distributor isn't
// configured, and the router then looks up every task list on the hash ring.
func NewTaskListRouter(
	resolver membership.Resolver,
	taskListDifferentiator *TaskListDifferentiator,
	spectator spectatorclient.Spectator,
	logger log.Logger,
	metricsScope metrics.Scope,
) TaskListRouter {
	return &taskListRouter{
		resolver:               resolver,
		taskListDifferentiator: taskListDifferentiator,
		spectator:              spectator,
		logger:                 logger,
		metricsScope:           metricsScope,
	}
}

func (r *taskListRouter) FromTaskList(taskListName string) (membership.HostInfo, error) {
	if r.taskListDifferentiator.TaskListExcludedFromShardDistributor(taskListName) {
		r.emitRoutingPathMetric(routingPathHashRing)
		return r.resolver.Lookup(service.Matching, taskListName)
	}

	if r.spectator == nil {
		r.logger.Warn("No shard distributor client, defaulting to hash ring")
		r.emitRoutingPathMetric(routingPathHashRing)
		return r.resolver.Lookup(service.Matching, taskListName)
	}

	r.emitRoutingPathMetric(routingPathShardDistributor)

	shardID := r.taskListDifferentiator.ShardID(taskListName)
	return r.lookUpInShardDistributor(shardID)
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
