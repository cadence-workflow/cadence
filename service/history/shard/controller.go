// Copyright (c) 2020 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination controller_mock.go -self_package github.com/uber/cadence/service/history/shard

package shard

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	workflow "github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/engine"
	"github.com/uber/cadence/service/history/lookup"
	"github.com/uber/cadence/service/history/resource"
)

const (
	shardControllerMembershipUpdateListenerName = "ShardController"
)

var (
	errShardIDOutOfBoundary = &workflow.BadRequestError{Message: "shard ID is out of boundary"}
)

type (
	// EngineFactory is used to create an instance of sharded history engine
	EngineFactory interface {
		CreateEngine(Context) engine.Engine
	}

	// Controller controls history service shards
	Controller interface {
		common.Daemon

		// PrepareToStop starts the graceful shutdown process for controller
		PrepareToStop()

		GetEngine(workflowID string) (engine.Engine, error)
		GetEngineForShard(shardID int) (engine.Engine, error)
		RemoveEngineForShard(shardID int)

		// Following methods describes the current status of the controller
		// TODO: consider converting to a unified describe method
		Status() int32
		NumShards() int
		ShardIDs() []int32
	}

	shardIDSnapshot struct {
		shardIDs  []int32
		numShards int
	}

	controller struct {
		resource.Resource

		membershipUpdateCh       chan *membership.ChangedEvent
		engineFactory            EngineFactory
		status                   int32
		shuttingDown             int32
		shutdownWG               sync.WaitGroup
		shutdownCh               chan struct{}
		logger                   log.Logger
		throttledLogger          log.Logger
		config                   *config.Config
		metricsScope             metrics.Scope
		replicationBudgetManager cache.Manager

		sync.RWMutex
		historyShards   map[int]*historyShardsItem
		shardIDSnapshot atomic.Pointer[shardIDSnapshot]
	}

	historyShardsItemStatus int

	historyShardsItem struct {
		resource.Resource

		shardID                  int
		config                   *config.Config
		logger                   log.Logger
		throttledLogger          log.Logger
		engineFactory            EngineFactory
		replicationBudgetManager cache.Manager

		sync.RWMutex
		status historyShardsItemStatus
		engine engine.Engine
	}
)

const (
	historyShardsItemStatusInitialized = iota
	historyShardsItemStatusStarted
	historyShardsItemStatusStopped
)

// NewShardController creates a new shard controller
func NewShardController(
	resource resource.Resource,
	factory EngineFactory,
	config *config.Config,
	replicationBudgetManager cache.Manager,
) Controller {
	hostAddress := resource.GetHostInfo().GetAddress()
	return &controller{
		Resource:                 resource,
		status:                   common.DaemonStatusInitialized,
		membershipUpdateCh:       make(chan *membership.ChangedEvent, 10),
		engineFactory:            factory,
		historyShards:            make(map[int]*historyShardsItem),
		shutdownCh:               make(chan struct{}),
		logger:                   resource.GetLogger().WithTags(tag.ComponentShardController, tag.Address(hostAddress)),
		throttledLogger:          resource.GetThrottledLogger().WithTags(tag.ComponentShardController, tag.Address(hostAddress)),
		config:                   config,
		metricsScope:             resource.GetMetricsClient().Scope(metrics.HistoryShardControllerScope),
		replicationBudgetManager: replicationBudgetManager,
	}
}

func newHistoryShardsItem(
	resource resource.Resource,
	shardID int,
	factory EngineFactory,
	config *config.Config,
	replicationBudgetManager cache.Manager,
) (*historyShardsItem, error) {

	hostAddress := resource.GetHostInfo().GetAddress()
	return &historyShardsItem{
		Resource:                 resource,
		shardID:                  shardID,
		status:                   historyShardsItemStatusInitialized,
		engineFactory:            factory,
		config:                   config,
		logger:                   resource.GetLogger().WithTags(tag.ShardID(shardID), tag.Address(hostAddress)),
		throttledLogger:          resource.GetThrottledLogger().WithTags(tag.ShardID(shardID), tag.Address(hostAddress)),
		replicationBudgetManager: replicationBudgetManager,
	}, nil
}

func (c *controller) Start() {
	if !atomic.CompareAndSwapInt32(&c.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	c.acquireShards()
	c.shutdownWG.Add(1)
	go c.shardManagementPump()

	err := c.GetMembershipResolver().Subscribe(service.History, shardControllerMembershipUpdateListenerName, c.membershipUpdateCh)
	if err != nil {
		c.logger.Error("subscribing to membership resolver", tag.Error(err))
	}

	c.logger.Info("Shard controller state changed", tag.LifeCycleStarted)
}

func (c *controller) Stop() {
	if !atomic.CompareAndSwapInt32(&c.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	c.logger.Info("Stopping shard controller", tag.ComponentShardController)
	defer c.logger.Info("Stopped shard controller", tag.ComponentShardController)

	c.PrepareToStop()

	if err := c.GetMembershipResolver().Unsubscribe(service.History, shardControllerMembershipUpdateListenerName); err != nil {
		c.logger.Error("unsubscribing from membership resolver", tag.Error(err), tag.OperationFailed)
	}
	close(c.shutdownCh)

	if success := common.AwaitWaitGroup(&c.shutdownWG, time.Minute); !success {
		c.logger.Warn("", tag.LifeCycleStopTimedout)
	}

	c.logger.Info("Shard controller state changed", tag.LifeCycleStopped)
}

func (c *controller) PrepareToStop() {
	atomic.StoreInt32(&c.shuttingDown, 1)
}

func (c *controller) GetEngine(workflowID string) (engine.Engine, error) {
	shardID := c.config.GetShardID(workflowID)
	return c.GetEngineForShard(shardID)
}

func (c *controller) GetEngineForShard(shardID int) (engine.Engine, error) {
	sw := c.metricsScope.StartTimer(metrics.GetEngineForShardLatency)
	defer sw.Stop()
	item, err := c.getOrCreateHistoryShardItem(shardID)
	if err != nil {
		return nil, err
	}
	return item.getOrCreateEngine(c.shardClosedCallback)
}

func (c *controller) RemoveEngineForShard(shardID int) {
	c.removeEngineForShard(shardID, nil)
}

func (c *controller) Status() int32 {
	return atomic.LoadInt32(&c.status)
}

func (c *controller) NumShards() int {
	s := c.shardIDSnapshot.Load()
	if s == nil {
		return 0
	}
	return s.numShards
}

func (c *controller) ShardIDs() []int32 {
	s := c.shardIDSnapshot.Load()
	if s == nil {
		return []int32{}
	}
	return s.shardIDs
}

func (c *controller) removeEngineForShard(shardID int, shardItem *historyShardsItem) {
	sw := c.metricsScope.StartTimer(metrics.RemoveEngineForShardLatency)
	defer sw.Stop()
	c.logger.Info("removeEngineForShard called", tag.ShardID(shardID))
	defer c.logger.Info("removeEngineForShard completed", tag.ShardID(shardID))

	currentShardItem, err := c.removeHistoryShardItem(shardID, shardItem)
	if err != nil {
		c.logger.Error("Failed to remove history shard item", tag.Error(err), tag.ShardID(shardID))
	}
	if shardItem != nil {
		// if shardItem is not nil, then currentShardItem either equals to shardItem or is nil
		// in both cases, we need to stop the engine in shardItem
		shardItem.stopEngine()
		return
	}

	// if shardItem is nil, then stop the engine for the current shardItem, if exists
	if currentShardItem != nil {
		currentShardItem.stopEngine()
	}
}

func (c *controller) shardClosedCallback(shardID int, shardItem *historyShardsItem) {
	c.metricsScope.IncCounter(metrics.ShardClosedCounter)
	c.logger.Info("Shard controller state changed", tag.LifeCycleStopping, tag.ComponentShard, tag.ShardID(shardID), tag.Reason("shardClosedCallback"))
	c.removeEngineForShard(shardID, shardItem)
}

func (c *controller) getOrCreateHistoryShardItem(shardID int) (*historyShardsItem, error) {
	if shardID >= c.config.NumberOfShards || shardID < 0 { // zero based shard ID
		c.logger.Error(fmt.Sprintf("Received shard ID: %v is larger than supported shard number %v",
			shardID,
			c.config.NumberOfShards,
		),
		)
		return nil, errShardIDOutOfBoundary
	}

	c.RLock()
	if item, ok := c.historyShards[shardID]; ok {
		if item.isValid() {
			c.RUnlock()
			return item, nil
		}
		// if item not valid then process to create a new one
	}
	c.RUnlock()

	c.logger.Info("Creating new history shard item", tag.ShardID(shardID))
	defer c.logger.Info("Created new history shard item", tag.ShardID(shardID))

	c.Lock()
	defer c.Unlock()

	if item, ok := c.historyShards[shardID]; ok {
		if item.isValid() {
			return item, nil
		}
		// if item not valid then process to create a new one
	}

	if c.isShuttingDown() || atomic.LoadInt32(&c.status) == common.DaemonStatusStopped {
		return nil, fmt.Errorf("controller for host '%v' shutting down", c.GetHostInfo().Identity())
	}
	info, err := lookup.HistoryServerByShardID(c.GetMembershipResolver(), shardID)
	if err != nil {
		return nil, err
	}

	shardBelongsToCurrentHost := info.Identity() == c.GetHostInfo().Identity()
	c.logger.Info("Shard belongs to current host?",
		tag.ShardID(shardID),
		tag.Value(shardBelongsToCurrentHost),
		tag.Dynamic("shard-owner", info.Identity()),
		tag.Dynamic("current-host", c.GetHostInfo().Identity()),
	)

	if shardBelongsToCurrentHost {
		shardItem, err := newHistoryShardsItem(
			c.Resource,
			shardID,
			c.engineFactory,
			c.config,
			c.replicationBudgetManager,
		)
		if err != nil {
			return nil, err
		}
		c.historyShards[shardID] = shardItem
		c.updateShardIDSnapshotLocked()
		c.metricsScope.IncCounter(metrics.ShardItemCreatedCounter)

		shardItem.logger.Info("Shard item state changed", tag.LifeCycleStarted, tag.ComponentShardItem)
		return shardItem, nil
	}

	// for backwards compatibility, always return tchannel port
	return nil, CreateShardOwnershipLostError(c.GetHostInfo(), info)
}

func (c *controller) updateShardIDSnapshotLocked() {
	shardIDs := make([]int32, 0, len(c.historyShards))
	for shardID := range c.historyShards {
		shardIDs = append(shardIDs, int32(shardID))
	}
	snapshot := &shardIDSnapshot{
		shardIDs:  shardIDs,
		numShards: len(shardIDs),
	}
	c.shardIDSnapshot.Store(snapshot)
}

func (c *controller) removeHistoryShardItem(shardID int, shardItem *historyShardsItem) (*historyShardsItem, error) {
	c.Lock()
	defer c.Unlock()

	currentShardItem, ok := c.historyShards[shardID]
	if !ok {
		return nil, fmt.Errorf("no item found to remove for shard: %v", shardID)
	}
	if shardItem != nil && currentShardItem != shardItem {
		// the shardItem comparison is a defensive check to make sure we are deleting
		// what we intend to delete.
		return nil, fmt.Errorf("current shardItem doesn't match the one we intend to delete for shard: %v", shardID)
	}

	delete(c.historyShards, shardID)
	c.updateShardIDSnapshotLocked()
	c.metricsScope.IncCounter(metrics.ShardItemRemovedCounter)
	currentShardItem.logger.Info("Shard item state changed", tag.LifeCycleStopped, tag.ComponentShardItem, tag.Number(int64(len(c.historyShards))))
	return currentShardItem, nil
}

// shardManagementPump is the main event loop for
// controller. It is responsible for acquiring /
// releasing shards in response to any event that can
// change the shard ownership. These events are
//
//	a. Ring membership change
//	b. Periodic ticker
//	c. ShardOwnershipLostError and subsequent ShardClosedEvents from engine
func (c *controller) shardManagementPump() {
	defer c.shutdownWG.Done()

	acquireTicker := time.NewTicker(c.config.AcquireShardInterval())
	defer acquireTicker.Stop()

	for {

		select {
		case <-c.shutdownCh:
			c.doShutdown()
			return
		case <-acquireTicker.C:
			c.acquireShards()
		case changedEvent := <-c.membershipUpdateCh:
			c.metricsScope.IncCounter(metrics.MembershipChangedCounter)

			c.logger.Info("Ring membership changed", tag.ValueRingMembershipChangedEvent,
				tag.NumberProcessed(len(changedEvent.HostsAdded)),
				tag.NumberDeleted(len(changedEvent.HostsRemoved)),
				tag.Number(int64(len(changedEvent.HostsUpdated))))
			c.acquireShards()
		}
	}
}

func (c *controller) acquireShards() {
	c.logger.Info("Acquiring shards", tag.ComponentShardController, tag.Number(int64(c.NumShards())))
	defer c.logger.Info("Acquired shards", tag.ComponentShardController, tag.Number(int64(c.NumShards())))

	c.metricsScope.IncCounter(metrics.AcquireShardsCounter)
	sw := c.metricsScope.StartTimer(metrics.AcquireShardsLatency)
	defer sw.Stop()

	numShards := c.config.NumberOfShards
	shardActionCh := make(chan int, numShards)
	// Submit all tasks to the channel.
	for shardID := 0; shardID < numShards; shardID++ {
		shardActionCh <- shardID // must be non-blocking as there is no other coordination with shutdown
	}
	close(shardActionCh)

	concurrency := max(c.config.AcquireShardConcurrency(), 1)
	var wg sync.WaitGroup
	wg.Add(concurrency)
	// Spawn workers that would do lookup and add/remove shards concurrently.
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			for shardID := range shardActionCh {
				if c.isShuttingDown() {
					return
				}
				info, err := lookup.HistoryServerByShardID(c.GetMembershipResolver(), shardID)
				if err != nil {
					c.logger.Error("Error looking up host for shardID", tag.Error(err), tag.OperationFailed, tag.ShardID(shardID))
				} else {
					if info.Identity() == c.GetHostInfo().Identity() {
						_, err1 := c.GetEngineForShard(shardID)
						if err1 != nil {
							c.metricsScope.IncCounter(metrics.GetEngineForShardErrorCounter)
							c.logger.Error("Unable to create history shard engine", tag.Error(err1), tag.OperationFailed, tag.ShardID(shardID))
						}
					}
				}
			}
		}()
	}
	// Wait until all shards are processed.
	wg.Wait()

	c.metricsScope.UpdateGauge(metrics.NumShardsGauge, float64(c.NumShards()))
}

func (c *controller) doShutdown() {
	c.logger.Info("Shard controller state changed", tag.LifeCycleStopping, tag.Reason("shutdown"))
	c.Lock()
	defer c.Unlock()
	for _, item := range c.historyShards {
		item.stopEngine()
	}
	c.historyShards = nil
	c.updateShardIDSnapshotLocked()
}

func (c *controller) isShuttingDown() bool {
	return atomic.LoadInt32(&c.shuttingDown) != 0
}

func (i *historyShardsItem) getOrCreateEngine(
	closeCallback func(int, *historyShardsItem),
) (engine.Engine, error) {
	i.RLock()
	if i.status == historyShardsItemStatusStarted {
		defer i.RUnlock()
		return i.engine, nil
	}
	i.RUnlock()

	i.Lock()
	defer i.Unlock()
	switch i.status {
	case historyShardsItemStatusInitialized:
		i.logger.Info("Shard engine state changed", tag.LifeCycleStarting, tag.ComponentShardEngine)
		context, err := acquireShard(i, closeCallback)
		if err != nil {
			// invalidate the shardItem so that the same shardItem won't be
			// used to create another shardContext
			i.logger.Info("Shard engine state changed", tag.LifeCycleStopped, tag.ComponentShardEngine)
			i.status = historyShardsItemStatusStopped
			return nil, err
		}
		if context.PreviousShardOwnerWasDifferent() {
			i.GetMetricsClient().RecordTimer(metrics.ShardInfoScope, metrics.ShardItemAcquisitionLatency,
				context.GetCurrentTime(i.GetClusterMetadata().GetCurrentClusterName()).Sub(context.GetLastUpdatedTime()))
		}
		i.engine = i.engineFactory.CreateEngine(context)
		i.engine.Start()
		i.logger.Info("Shard engine state changed", tag.LifeCycleStarted, tag.ComponentShardEngine)
		i.status = historyShardsItemStatusStarted
		return i.engine, nil
	case historyShardsItemStatusStarted:
		return i.engine, nil
	case historyShardsItemStatusStopped:
		return nil, fmt.Errorf("shard %v for host '%v' is shut down", i.shardID, i.GetHostInfo().Identity())
	default:
		panic(i.logInvalidStatus())
	}
}

func (i *historyShardsItem) stopEngine() {
	i.Lock()
	defer i.Unlock()

	i.logger.Info("Shard item stopEngine called", tag.ComponentShardEngine, tag.Dynamic("status", i.status))

	switch i.status {
	case historyShardsItemStatusInitialized:
		i.status = historyShardsItemStatusStopped
	case historyShardsItemStatusStarted:
		i.logger.Info("Shard engine state changed", tag.LifeCycleStopping, tag.ComponentShardEngine)
		i.engine.Stop()
		i.engine = nil
		i.logger.Info("Shard engine state changed", tag.LifeCycleStopped, tag.ComponentShardEngine)
		i.status = historyShardsItemStatusStopped
	case historyShardsItemStatusStopped:
		// no op
	default:
		panic(i.logInvalidStatus())
	}
}

func (i *historyShardsItem) isValid() bool {
	i.RLock()
	defer i.RUnlock()

	switch i.status {
	case historyShardsItemStatusInitialized, historyShardsItemStatusStarted:
		return true
	case historyShardsItemStatusStopped:
		return false
	default:
		panic(i.logInvalidStatus())
	}
}

func (i *historyShardsItem) logInvalidStatus() string {
	msg := fmt.Sprintf("Host '%v' encounter invalid status %v for shard item for shardID '%v'.",
		i.GetHostInfo().Identity(), i.status, i.shardID)
	i.logger.Error(msg)
	return msg
}

// IsShardOwnershiptLostError checks if a given error is shard ownership lost error
func IsShardOwnershiptLostError(err error) bool {
	switch err.(type) {
	case *persistence.ShardOwnershipLostError:
		return true
	}

	return false
}

// CreateShardOwnershipLostError creates a new shard ownership lost error
func CreateShardOwnershipLostError(
	currentHost membership.HostInfo,
	ownerHost membership.HostInfo,
) *types.ShardOwnershipLostError {
	address, err := ownerHost.GetNamedAddress(membership.PortTchannel)
	if err != nil {
		address = ownerHost.Identity()
	}
	return &types.ShardOwnershipLostError{
		Message: fmt.Sprintf("Shard is not owned by host: %v", currentHost.Identity()),
		Owner:   address,
	}
}
