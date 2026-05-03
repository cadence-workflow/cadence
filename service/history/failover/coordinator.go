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
	cleanupMarkerInterval            = 30 * time.Minute
	invalidMarkerDuration            = 1 * time.Hour
	updateDomainRetryInitialInterval = 50 * time.Millisecond
	updateDomainRetryCoefficient     = 2.0
	updateDomainMaxRetry             = 2
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

		recorderLock sync.Mutex
		recorder     map[string]*failoverRecord

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
		recorder:         make(map[string]*failoverRecord),
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
	c.recorderLock.Lock()
	defer c.recorderLock.Unlock()

	record, ok := c.recorder[domainID]
	if !ok {
		return nil, errRecordNotFound
	}

	var pendingShards []int32
	for i := 0; i < c.config.NumberOfShards; i++ {
		if _, ok := record.shards[int32(i)]; !ok {
			pendingShards = append(pendingShards, int32(i))
		}
	}
	return &types.GetFailoverInfoResponse{
		CompletedShardCount: int32(len(record.shards)),
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
			c.handleFailoverMarkers(request)
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

	for {
		select {
		case <-c.shutdownChan:
			return
		case notificationReq := <-c.notificationChan:
			// if there is a shard movement happen, it is fine to have duplicate shard ID in the request
			// The receiver side will de-dup the shard IDs. See: handleFailoverMarkers
			aggregateNotificationRequests(notificationReq, requestByMarker)
		case <-timer.C:
			if err := c.notifyRemoteCoordinator(requestByMarker); err == nil {
				requestByMarker = make(map[types.FailoverMarkerAttributes]*receiveRequest)
			}
			timer.Reset(backoff.JitDuration(
				c.config.NotifyFailoverMarkerInterval(),
				c.config.NotifyFailoverMarkerTimerJitterCoefficient(),
			))
		}
	}
}

func (c *coordinatorImpl) handleFailoverMarkers(
	request *receiveRequest,
) {

	c.recorderLock.Lock()
	defer c.recorderLock.Unlock()

	marker := request.marker
	domainID := marker.GetDomainID()
	if record, ok := c.recorder[domainID]; ok {
		// if the local failover version is smaller than the new received marker,
		// it means there is another failover happened and the local one should be invalid.
		if record.failoverVersion < marker.GetFailoverVersion() {
			delete(c.recorder, domainID)
		}

		// if the local failover version is larger than the new received marker,
		// ignore the incoming marker
		if record.failoverVersion > marker.GetFailoverVersion() {
			return
		}
	}

	if _, ok := c.recorder[domainID]; !ok {
		// initialize the failover record
		c.recorder[marker.GetDomainID()] = &failoverRecord{
			failoverVersion: marker.GetFailoverVersion(),
			shards:          make(map[int32]struct{}),
		}
	}

	record := c.recorder[domainID]
	record.lastUpdatedTime = c.timeSource.Now()
	for _, shardID := range request.shardIDs {
		record.shards[shardID] = struct{}{}
	}

	domainName, err := c.domainCache.GetDomainName(domainID)
	if err != nil {
		c.logger.Error("Coordinator failed to get domain after receiving all failover markers",
			tag.WorkflowDomainID(domainID),
			tag.Error(err),
		)

		c.scope.Tagged(metrics.DomainTag(domainName)).IncCounter(metrics.CadenceFailures)
		return
	}

	if len(record.shards) == c.config.NumberOfShards {
		if err := domain.CleanPendingActiveState(
			c.domainManager,
			domainID,
			record.failoverVersion,
			c.retryPolicy,
		); err != nil {
			c.logger.Error("Coordinator failed to update domain after receiving all failover markers",
				tag.WorkflowDomainID(domainID),
				tag.Error(err),
			)
			c.scope.IncCounter(metrics.CadenceFailures)
			return
		}
		delete(c.recorder, domainID)
		now := c.timeSource.Now()
		c.scope.Tagged(
			metrics.DomainTag(domainName),
		).RecordTimer(
			metrics.GracefulFailoverLatency,
			now.Sub(time.Unix(0, marker.GetCreationTime())),
		)
		c.scope.Tagged(
			metrics.DomainTag(domainName),
		).RecordHistogramDuration(
			metrics.GracefulFailoverLatencyHistogram,
			now.Sub(time.Unix(0, marker.GetCreationTime())),
		)
		c.logger.Info("Updated domain from pending-active to active",
			tag.WorkflowDomainName(domainName),
			tag.FailoverVersion(marker.FailoverVersion),
		)
	} else {
		c.scope.Tagged(
			metrics.DomainTag(domainName),
		).RecordTimer(
			metrics.FailoverMarkerCount,
			time.Duration(len(record.shards)),
		)
	}
}

func (c *coordinatorImpl) cleanupInvalidMarkers() {
	c.recorderLock.Lock()
	defer c.recorderLock.Unlock()

	for domainID, record := range c.recorder {
		if c.timeSource.Now().Sub(record.lastUpdatedTime) > invalidMarkerDuration {
			delete(c.recorder, domainID)
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
