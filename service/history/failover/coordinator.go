// The MIT License (MIT)
//
// Copyright (c) 2017-2020 Uber Technologies Inc.
//
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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination coordinator_mock.go -self_package github.com/uber/cadence/service/history/failover

package failover

import (
	ctx "context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/domain"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
)

const (
	notificationChanBufferSize       = 1000
	receiveChanBufferSize            = 1000
	workerChanBufferSize             = 512
	cleanupMarkerInterval            = 30 * time.Minute
	invalidMarkerDuration            = 1 * time.Hour
	updateDomainRetryInitialInterval = 50 * time.Millisecond
	updateDomainRetryCoefficient     = 2.0
	updateDomainMaxRetry             = 2
	notifyFailoverMarkerMinInterval  = 500 * time.Millisecond
)

var (
	errRecordNotFound = &types.EntityNotExistsError{Message: "Graceful failover record not found in shard coordinator"}
)

type (
	// Coordinator manages the failover markers on sending and receiving
	Coordinator interface {
		common.Daemon

		NotifyFailoverMarkers(shardID int32, markers []*types.FailoverMarkerAttributes)
		ReceiveFailoverMarkers(shardIDs []int32, marker *types.FailoverMarkerAttributes)
		GetFailoverInfo(domainID string) (*types.GetFailoverInfoResponse, error)
	}

	coordinatorImpl struct {
		status           int32
		notificationChan chan *notificationRequest
		receiveChan      chan *receiveRequest
		shutdownChan     chan struct{}
		retryPolicy      backoff.RetryPolicy

		workersLock sync.RWMutex
		workers     map[string]*domainWorker

		domainManager persistence.DomainManager
		historyClient history.Client
		config        *config.Config
		timeSource    clock.TimeSource
		domainCache   cache.DomainCache
		scope         metrics.Scope
		logger        log.Logger
	}

	notificationRequest struct {
		shardID int32
		markers []*types.FailoverMarkerAttributes
	}

	receiveRequest struct {
		shardIDs []int32
		marker   *types.FailoverMarkerAttributes
	}

	failoverRecord struct {
		failoverVersion int64
		shards          map[int32]struct{}
		lastUpdatedTime time.Time
		firstSeenTime   time.Time
	}

	workerMsg struct {
		receive *receiveRequest
		cleanup bool
	}

	domainWorker struct {
		domainID    string
		receiveChan chan workerMsg

		mu     sync.Mutex
		record *failoverRecord
	}
)

// NewCoordinator initialize a failover coordinator
func NewCoordinator(
	domainManager persistence.DomainManager,
	historyClient history.Client,
	timeSource clock.TimeSource,
	domainCache cache.DomainCache,
	config *config.Config,
	metricsClient metrics.Client,
	logger log.Logger,
) Coordinator {

	retryPolicy := backoff.NewExponentialRetryPolicy(updateDomainRetryInitialInterval)
	retryPolicy.SetBackoffCoefficient(updateDomainRetryCoefficient)
	retryPolicy.SetMaximumAttempts(updateDomainMaxRetry)

	return &coordinatorImpl{
		status:           common.DaemonStatusInitialized,
		workers:          make(map[string]*domainWorker),
		notificationChan: make(chan *notificationRequest, notificationChanBufferSize),
		receiveChan:      make(chan *receiveRequest, receiveChanBufferSize),
		shutdownChan:     make(chan struct{}),
		retryPolicy:      retryPolicy,
		domainManager:    domainManager,
		historyClient:    historyClient,
		timeSource:       timeSource,
		domainCache:      domainCache,
		config:           config,
		scope:            metricsClient.Scope(metrics.FailoverMarkerScope),
		logger:           logger.WithTags(tag.ComponentFailoverCoordinator),
	}
}

func (c *coordinatorImpl) Start() {

	if !atomic.CompareAndSwapInt32(
		&c.status,
		common.DaemonStatusInitialized,
		common.DaemonStatusStarted,
	) {
		return
	}

	go c.receiveFailoverMarkersLoop()
	go c.notifyFailoverMarkerLoop()

	c.logger.Info("Coordinator state changed", tag.LifeCycleStarted)
}

func (c *coordinatorImpl) Stop() {

	if !atomic.CompareAndSwapInt32(
		&c.status,
		common.DaemonStatusStarted,
		common.DaemonStatusStopped,
	) {
		return
	}

	close(c.shutdownChan)
	c.logger.Info("Coordinator state changed", tag.LifeCycleStopped)
}

func (c *coordinatorImpl) NotifyFailoverMarkers(
	shardID int32,
	markers []*types.FailoverMarkerAttributes,
) {

	c.notificationChan <- &notificationRequest{
		shardID: shardID,
		markers: markers,
	}
}

func (c *coordinatorImpl) ReceiveFailoverMarkers(
	shardIDs []int32,
	marker *types.FailoverMarkerAttributes,
) {

	c.receiveChan <- &receiveRequest{
		shardIDs: shardIDs,
		marker:   marker,
	}
}

func (c *coordinatorImpl) GetFailoverInfo(
	domainID string,
) (*types.GetFailoverInfoResponse, error) {
	c.workersLock.RLock()
	w, ok := c.workers[domainID]
	c.workersLock.RUnlock()
	if !ok {
		return nil, errRecordNotFound
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.record == nil {
		return nil, errRecordNotFound
	}

	var pendingShards []int32
	for i := 0; i < c.config.NumberOfShards; i++ {
		if _, ok := w.record.shards[int32(i)]; !ok {
			pendingShards = append(pendingShards, int32(i))
		}
	}
	return &types.GetFailoverInfoResponse{
		CompletedShardCount: int32(len(w.record.shards)),
		PendingShards:       pendingShards,
	}, nil
}

func (c *coordinatorImpl) receiveFailoverMarkersLoop() {

	ticker := time.NewTicker(cleanupMarkerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.shutdownChan:
			return
		case <-ticker.C:
			c.cleanupInvalidMarkers()
		case request := <-c.receiveChan:
			c.dispatchToWorker(request)
		}
	}
}

func (c *coordinatorImpl) dispatchToWorker(request *receiveRequest) {
	domainID := request.marker.GetDomainID()
	for {
		w := c.getOrCreateWorker(domainID)

		// Re-validate under RLock that w is still the current worker for this
		// domain before sending. Without this, the worker goroutine could call
		// tryRemoveWorker (deleting itself from the map and exiting) between
		// getOrCreateWorker returning and the channel send, leaving the message
		// sitting in an orphaned buffered channel that no one drains.
		c.workersLock.RLock()
		cur, ok := c.workers[domainID]
		if !ok || cur != w {
			c.workersLock.RUnlock()
			continue
		}
		select {
		case w.receiveChan <- workerMsg{receive: request}:
			c.workersLock.RUnlock()
			return
		default:
			// Worker's channel is saturated. Don't hold workersLock across the
			// inline call — handleFailoverMarkers may invoke CleanPendingActiveState,
			// which would block all worker exits and new-worker creates for every
			// other domain. A saturated channel also means tryRemoveWorker cannot
			// succeed for this w (len(receiveChan) > 0), so w is guaranteed to
			// still be the live worker when we call handleFailoverMarkers below.
			c.workersLock.RUnlock()
			c.handleFailoverMarkers(w, request)
			return
		}
	}
}

func (c *coordinatorImpl) getOrCreateWorker(domainID string) *domainWorker {
	c.workersLock.RLock()
	w, ok := c.workers[domainID]
	c.workersLock.RUnlock()
	if ok {
		return w
	}

	c.workersLock.Lock()
	defer c.workersLock.Unlock()
	if w, ok := c.workers[domainID]; ok {
		return w
	}
	w = &domainWorker{
		domainID:    domainID,
		receiveChan: make(chan workerMsg, workerChanBufferSize),
	}
	c.workers[domainID] = w
	go c.runDomainWorker(w)
	return w
}

func (c *coordinatorImpl) runDomainWorker(w *domainWorker) {
	for {
		select {
		case <-c.shutdownChan:
			return
		case msg := <-w.receiveChan:
			done := false
			if msg.cleanup {
				done = true
			} else {
				done = c.handleFailoverMarkers(w, msg.receive)
			}
			if done && c.tryRemoveWorker(w) {
				return
			}
		}
	}
}

// tryRemoveWorker removes the worker from the outer map iff it has no pending
// messages and no live failover record. Returns true if the worker was removed
// and the goroutine should exit. Returns false if work arrived after the worker
// decided to exit (queued message, or a record re-created by an inline dispatch
// from dispatchToWorker), in which case the worker keeps running.
func (c *coordinatorImpl) tryRemoveWorker(w *domainWorker) bool {
	c.workersLock.Lock()
	defer c.workersLock.Unlock()
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.receiveChan) > 0 || w.record != nil {
		return false
	}
	delete(c.workers, w.domainID)
	return true
}

// handleFailoverMarkers processes a single receiveRequest for one domain.
// Returns true if the failover record was deleted (worker should attempt exit).
func (c *coordinatorImpl) handleFailoverMarkers(
	w *domainWorker,
	request *receiveRequest,
) bool {

	w.mu.Lock()
	defer w.mu.Unlock()

	marker := request.marker
	domainID := marker.GetDomainID()
	if w.record != nil {
		// if the local failover version is less than the failover version in the marker,
		// it means that another failover happened for this domain and the local one should be invalidated
		if w.record.failoverVersion < marker.GetFailoverVersion() {
			w.record = nil
		}

		// if the local failover version is larger than the failover version in the marker,
		// ignore the incoming marker
		if w.record != nil && w.record.failoverVersion > marker.GetFailoverVersion() {
			return false
		}
	}

	now := c.timeSource.Now()
	if w.record == nil {
		w.record = &failoverRecord{
			failoverVersion: marker.GetFailoverVersion(),
			shards:          make(map[int32]struct{}),
			firstSeenTime:   now,
		}
	}

	w.record.lastUpdatedTime = now
	for _, shardID := range request.shardIDs {
		w.record.shards[shardID] = struct{}{}
	}

	domainName, err := c.domainCache.GetDomainName(domainID)
	if err != nil {
		c.logger.Error("Coordinator failed to get domain name while recording failover markers from request",
			tag.WorkflowDomainID(domainID),
			tag.Error(err),
		)

		c.scope.Tagged(metrics.DomainTag(domainName)).IncCounter(metrics.CadenceFailures)
		return false
	}

	if len(w.record.shards) == c.config.NumberOfShards {
		cleanStart := c.timeSource.Now()
		updated, err := domain.CleanPendingActiveState(
			c.domainManager,
			domainID,
			w.record.failoverVersion,
			c.retryPolicy,
		)
		cleanDuration := c.timeSource.Now().Sub(cleanStart)
		if err != nil {
			c.logger.Error("Coordinator failed to update domain after receiving failover markers from all shards",
				tag.WorkflowDomainID(domainID),
				tag.Error(err),
			)
			c.scope.IncCounter(metrics.CadenceFailures)
			return false
		}
		firstSeenTime := w.record.firstSeenTime
		w.record = nil
		// reset the gauge so it reflects the current (empty) pending state for this domain
		// rather than the last partial-count value, which would otherwise linger forever
		c.scope.Tagged(
			metrics.DomainTag(domainName),
		).UpdateGauge(
			metrics.FailoverMarkerCount,
			0,
		)

		if !updated {
			// another path already cleared the pending-active state — avoid the
			// misleading "Updated domain from pending-active to active" log and
			// the bogus GracefulFailoverLatency (which would otherwise report
			// now - marker.CreationTime on a long-stale marker)
			return true
		}

		now := c.timeSource.Now()
		// use the last marker to calculate the failover duration
		failoverDuration := now.Sub(time.Unix(0, marker.GetCreationTime()))
		markerPipelineDuration := now.Sub(firstSeenTime)
		c.scope.Tagged(
			metrics.DomainTag(domainName),
		).RecordTimer(
			metrics.GracefulFailoverLatency,
			failoverDuration,
		)
		c.scope.Tagged(
			metrics.DomainTag(domainName),
		).RecordHistogramDuration(
			metrics.GracefulFailoverLatencyHistogram,
			failoverDuration,
		)
		c.logger.Info("Updated domain from pending-active to active",
			tag.WorkflowDomainName(domainName),
			tag.FailoverVersion(marker.FailoverVersion),
			tag.Duration(failoverDuration),
			tag.Dynamic("marker-pipeline-duration", markerPipelineDuration),
			tag.Dynamic("clean-pending-active-duration", cleanDuration),
		)
		return true
	}

	c.scope.Tagged(
		metrics.DomainTag(domainName),
	).UpdateGauge(
		metrics.FailoverMarkerCount,
		float64(len(w.record.shards)),
	)
	return false
}

func (c *coordinatorImpl) cleanupInvalidMarkers() {
	c.workersLock.RLock()
	expired := make([]*domainWorker, 0)
	for _, w := range c.workers {
		w.mu.Lock()
		stale := w.record != nil && c.timeSource.Now().Sub(w.record.lastUpdatedTime) > invalidMarkerDuration
		if stale {
			w.record = nil
		}
		empty := w.record == nil
		w.mu.Unlock()
		if empty {
			expired = append(expired, w)
		}
	}
	c.workersLock.RUnlock()

	for _, w := range expired {
		select {
		case w.receiveChan <- workerMsg{cleanup: true}:
		case <-c.shutdownChan:
			return
		default:
			// Worker's channel is full; skip this round. The record is already
			// cleared, so the worker will reach tryRemoveWorker and exit on
			// its own once it drains.
		}
	}
}

func (c *coordinatorImpl) notifyFailoverMarkerLoop() {

	timer := time.NewTimer(backoff.JitDuration(
		c.config.NotifyFailoverMarkerInterval(),
		c.config.NotifyFailoverMarkerTimerJitterCoefficient(),
	))
	defer timer.Stop()
	requestByMarker := make(map[types.FailoverMarkerAttributes]*receiveRequest)
	var lastFlush time.Time

	flush := func() {
		if len(requestByMarker) == 0 {
			return
		}
		if err := c.notifyRemoteCoordinator(requestByMarker); err == nil {
			requestByMarker = make(map[types.FailoverMarkerAttributes]*receiveRequest)
			lastFlush = c.timeSource.Now()
		}
	}

	resetTimer := func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(backoff.JitDuration(
			c.config.NotifyFailoverMarkerInterval(),
			c.config.NotifyFailoverMarkerTimerJitterCoefficient(),
		))
	}

	for {
		select {
		case <-c.shutdownChan:
			return
		case notificationReq := <-c.notificationChan:
			// if a shard movement happens, it is fine to have duplicated shard IDs in the request
			// The receiver side will de-dup the shard IDs. See: handleFailoverMarkers
			aggregateNotificationRequests(notificationReq, requestByMarker)
			if c.timeSource.Now().Sub(lastFlush) >= notifyFailoverMarkerMinInterval {
				flush()
				resetTimer()
			} else {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(notifyFailoverMarkerMinInterval - c.timeSource.Now().Sub(lastFlush))
			}
		case <-timer.C:
			flush()
			timer.Reset(backoff.JitDuration(
				c.config.NotifyFailoverMarkerInterval(),
				c.config.NotifyFailoverMarkerTimerJitterCoefficient(),
			))
		}
	}
}

func (c *coordinatorImpl) notifyRemoteCoordinator(
	requestByMarker map[types.FailoverMarkerAttributes]*receiveRequest,
) error {

	if len(requestByMarker) > 0 {
		var tokens []*types.FailoverMarkerToken
		for _, request := range requestByMarker {
			tokens = append(tokens, &types.FailoverMarkerToken{
				ShardIDs:       request.shardIDs,
				FailoverMarker: request.marker,
			})
		}

		err := c.historyClient.NotifyFailoverMarkers(
			ctx.Background(),
			&types.NotifyFailoverMarkersRequest{
				FailoverMarkerTokens: tokens,
			},
		)
		if err != nil {
			c.scope.IncCounter(metrics.FailoverMarkerNotificationFailure)
			c.logger.Error("Failed to notify failover markers", tag.Error(err))
			return err
		}
	}
	return nil
}

func aggregateNotificationRequests(
	request *notificationRequest,
	requestByMarker map[types.FailoverMarkerAttributes]*receiveRequest,
) {

	for _, marker := range request.markers {
		markerMask := types.FailoverMarkerAttributes{
			DomainID:        marker.DomainID,
			FailoverVersion: marker.FailoverVersion,
		}
		if _, ok := requestByMarker[markerMask]; !ok {
			requestByMarker[markerMask] = &receiveRequest{
				shardIDs: []int32{},
				marker:   marker,
			}
		}
		req := requestByMarker[markerMask]
		req.shardIDs = append(req.shardIDs, request.shardID)
	}
}
