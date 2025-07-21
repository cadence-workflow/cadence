// Copyright (c) 2017-2020 Uber Technologies, Inc.
// Portions of the Software are attributed to Copyright (c) 2020 Temporal Technologies Inc.
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

package persistence

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

// workflowExecCacheEntry represents a cached workflow execution with expiration
type workflowExecCacheEntry struct {
	Response  *GetWorkflowExecutionResponse
	ExpiresAt time.Time
}

// WorkflowExecCache is an LRU cache with TTL for deserialized workflow executions
type WorkflowExecCache struct {
	mu       sync.RWMutex
	lookup   map[string]*listNode
	lruList  *listNode
	lruTail  *listNode
	capacity int
	ttl      time.Duration
	timeSource clock.TimeSource
	
	// Metrics
	hits   uint64
	misses uint64
	total  uint64
}

type listNode struct {
	key   string
	val   *workflowExecCacheEntry
	prev  *listNode
	next  *listNode
}

// NewWorkflowExecCache creates a new workflow execution cache with the given capacity and TTL
func NewWorkflowExecCache(capacity int, ttl time.Duration, timeSource clock.TimeSource) *WorkflowExecCache {
	if capacity <= 0 {
		capacity = 1024 // Default capacity
	}
	if ttl <= 0 {
		ttl = 5 * time.Second // Default TTL
	}
	return &WorkflowExecCache{
		lookup:     make(map[string]*listNode, capacity),
		capacity:   capacity,
		ttl:        ttl,
		timeSource: timeSource,
	}
}

// Get returns the cached workflow execution if present and not expired
func (c *WorkflowExecCache) Get(key string) (*GetWorkflowExecutionResponse, bool) {
	atomic.AddUint64(&c.total, 1)
	
	c.mu.RLock()
	node, ok := c.lookup[key]
	c.mu.RUnlock()
	
	if !ok {
		atomic.AddUint64(&c.misses, 1)
		return nil, false
	}
	
	// Check if expired (without write lock)
	if c.timeSource.Now().After(node.val.ExpiresAt) {
		c.mu.Lock()
		// Double-check expiration with write lock
		if c.timeSource.Now().After(node.val.ExpiresAt) {
			c.remove(node)
			delete(c.lookup, key)
			c.mu.Unlock()
			atomic.AddUint64(&c.misses, 1)
			return nil, false
		}
		c.mu.Unlock()
	}
	
	// Move to front of LRU list (requires write lock)
	c.mu.Lock()
	c.moveToFront(node)
	c.mu.Unlock()
	
	atomic.AddUint64(&c.hits, 1)
	return node.val.Response, true
}

// Set adds or updates a workflow execution in the cache
func (c *WorkflowExecCache) Set(key string, resp *GetWorkflowExecutionResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if node, ok := c.lookup[key]; ok {
		// Update existing entry
		node.val.Response = resp
		node.val.ExpiresAt = c.timeSource.Now().Add(c.ttl)
		c.moveToFront(node)
		return
	}
	
	// Create new entry
	entry := &workflowExecCacheEntry{
		Response:  resp,
		ExpiresAt: c.timeSource.Now().Add(c.ttl),
	}
	node := &listNode{
		key: key,
		val: entry,
	}
	c.insertFront(node)
	c.lookup[key] = node
	
	// Evict if over capacity
	if len(c.lookup) > c.capacity {
		evict := c.lruTail
		if evict != nil {
			c.remove(evict)
			delete(c.lookup, evict.key)
		}
	}
}

// Invalidate removes an entry from cache
func (c *WorkflowExecCache) Invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	node, ok := c.lookup[key]
	if !ok {
		return
	}
	c.remove(node)
	delete(c.lookup, key)
}

// Insert to front of LRU list
func (c *WorkflowExecCache) insertFront(node *listNode) {
	node.prev = nil
	node.next = c.lruList
	if c.lruList != nil {
		c.lruList.prev = node
	}
	c.lruList = node
	if c.lruTail == nil {
		c.lruTail = node
	}
}

// Remove node from list
func (c *WorkflowExecCache) remove(node *listNode) {
	if node.prev != nil {
		node.prev.next = node.next
	} else {
		c.lruList = node.next
	}
	if node.next != nil {
		node.next.prev = node.prev
	} else {
		c.lruTail = node.prev
	}
}

// Move node to front of list
func (c *WorkflowExecCache) moveToFront(node *listNode) {
	if c.lruList == node {
		return
	}
	c.remove(node)
	c.insertFront(node)
}

// GetStats returns cache statistics
func (c *WorkflowExecCache) GetStats() (hits, misses, total uint64) {
	return atomic.LoadUint64(&c.hits), 
	       atomic.LoadUint64(&c.misses), 
	       atomic.LoadUint64(&c.total)
}

// Helper function to build a cache key
func buildWorkflowExecutionCacheKey(domainID, workflowID, runID string) string {
	return fmt.Sprintf("%s:%s:%s", domainID, workflowID, runID)
}

type (
	// executionManagerImpl implements ExecutionManager based on ExecutionStore, statsComputer and PayloadSerializer
	executionManagerImpl struct {
		serializer    PayloadSerializer
		persistence   ExecutionStore
		statsComputer statsComputer
		logger        log.Logger
		timeSrc       clock.TimeSource
		metricsClient metrics.Client

		// Cache configuration
		cache        *WorkflowExecCache
		cacheSize    int
		cacheTTL     time.Duration
		cacheEnabled bool
	}
)

var _ ExecutionManager = (*executionManagerImpl)(nil)

// NewExecutionManagerImpl returns new ExecutionManager with configurable caching
func NewExecutionManagerImpl(
	persistence ExecutionStore,
	logger log.Logger,
	serializer PayloadSerializer,
	metricsClient metrics.Client,
	config *config.Persistence,
) ExecutionManager {
	timeSource := clock.NewRealTimeSource()
	
	// Default cache settings
	cacheSize := 1024
	cacheTTL := 5 * time.Second
	cacheEnabled := true
	
	// Override with config if provided
	if config != nil && config.ExecutionCache != nil {
		cacheSize = config.ExecutionCache.Size
		cacheTTL = config.ExecutionCache.TTL
		cacheEnabled = config.ExecutionCache.Enabled
	}
	
	cache := NewWorkflowExecCache(cacheSize, cacheTTL, timeSource)
	
	return &executionManagerImpl{
		serializer:    serializer,
		persistence:   persistence,
		statsComputer: statsComputer{},
		logger:        logger,
		timeSrc:       timeSource,
		metricsClient: metricsClient,
		cache:         cache,
		cacheSize:     cacheSize,
		cacheTTL:      cacheTTL,
		cacheEnabled:  cacheEnabled,
	}
}

func (m *executionManagerImpl) GetName() string {
	return m.persistence.GetName()
}

func (m *executionManagerImpl) GetShardID() int {
	return m.persistence.GetShardID()
}

// GetWorkflowExecution fetches and deserializes workflow execution with caching support
func (m *executionManagerImpl) GetWorkflowExecution(
	ctx context.Context,
	request *GetWorkflowExecutionRequest,
) (*GetWorkflowExecutionResponse, error) {
	sw := m.metricsClient.StartTimer(metrics.PersistenceGetWorkflowExecutionScope, metrics.PersistenceLatency)
	defer sw.Stop()

	cacheKey := buildWorkflowExecutionCacheKey(
		request.DomainID,
		request.Execution.GetWorkflowID(),
		request.Execution.GetRunID(),
	)

	// Try cache if enabled
	if m.cacheEnabled {
		if cached, hit := m.cache.Get(cacheKey); hit {
			m.metricsClient.IncCounter(metrics.PersistenceGetWorkflowExecutionScope, metrics.PersistenceCacheHits)
			return cached, nil
		}
		m.metricsClient.IncCounter(metrics.PersistenceGetWorkflowExecutionScope, metrics.PersistenceCacheMisses)
	}

	// Cache miss or disabled - fetch from persistence
	internalRequest := &InternalGetWorkflowExecutionRequest{
		DomainID:  request.DomainID,
		Execution: request.Execution,
		RangeID:   request.RangeID,
	}
	
	response, err := m.persistence.GetWorkflowExecution(ctx, internalRequest)
	if err != nil {
		return nil, err
	}
	
	// Prepare response with empty state
	newResponse := &GetWorkflowExecutionResponse{
		State: &WorkflowMutableState{
			TimerInfos:         response.State.TimerInfos,
			RequestCancelInfos: response.State.RequestCancelInfos,
			SignalInfos:        response.State.SignalInfos,
			SignalRequestedIDs: response.State.SignalRequestedIDs,
			ReplicationState:   response.State.ReplicationState,
			Checksum:           response.State.Checksum,
		},
	}

	// Deserialize components in parallel for performance
	var errs []error
	var wg sync.WaitGroup
	var errMutex sync.Mutex
	
	// Helper to record errors from goroutines
	recordError := func(err error) {
		if err != nil {
			errMutex.Lock()
			errs = append(errs, err)
			errMutex.Unlock()
		}
	}
	
	wg.Add(5)
	
	// Activity infos
	go func() {
		defer wg.Done()
		var err error
		newResponse.State.ActivityInfos, err = m.DeserializeActivityInfos(response.State.ActivityInfos)
		recordError(err)
	}()
	
	// Child execution infos
	go func() {
		defer wg.Done()
		var err error
		newResponse.State.ChildExecutionInfos, err = m.DeserializeChildExecutionInfos(response.State.ChildExecutionInfos)
		recordError(err)
	}()
	
	// Buffered events
	go func() {
		defer wg.Done()
		var err error
		newResponse.State.BufferedEvents, err = m.DeserializeBufferedEvents(response.State.BufferedEvents)
		recordError(err)
	}()
	
	// Execution info
	go func() {
		defer wg.Done()
		var err error
		newResponse.State.ExecutionInfo, newResponse.State.ExecutionStats, err = m.DeserializeExecutionInfo(response.State.ExecutionInfo)
		recordError(err)
	}()
	
	// Version histories
	go func() {
		defer wg.Done()
		var err error
		newResponse.State.VersionHistories, err = m.DeserializeVersionHistories(response.State.VersionHistories)
		recordError(err)
	}()
	
	// Wait for all deserializations to complete
	wg.Wait()
	
	// Check for errors
	if len(errs) > 0 {
		return nil, errs[0] // Return first error
	}

	newResponse.MutableStateStats = m.statsComputer.computeMutableStateStats(response)

	// Deserialize checksum if needed
	if len(newResponse.State.Checksum.Value) == 0 {
		var err error
		newResponse.State.Checksum, err = m.serializer.DeserializeChecksum(response.State.ChecksumData)
		if err != nil {
			return nil, err
		}
	}

	// Cache result if enabled
	if m.cacheEnabled {
		m.cache.Set(cacheKey, newResponse)
	}

	return newResponse, nil
}

// UpdateWorkflowExecution persists a workflow mutation with cache invalidation
func (m *executionManagerImpl) UpdateWorkflowExecution(
	ctx context.Context,
	request *UpdateWorkflowExecutionRequest,
) (*UpdateWorkflowExecutionResponse, error) {
	sw := m.metricsClient.StartTimer(metrics.PersistenceUpdateWorkflowExecutionScope, metrics.PersistenceLatency)
	defer sw.Stop()

	serializedWorkflowMutation, err := m.SerializeWorkflowMutation(&request.UpdateWorkflowMutation, request.Encoding)
	if err != nil {
		return nil, err
	}
	var serializedNewWorkflowSnapshot *InternalWorkflowSnapshot
	if request.NewWorkflowSnapshot != nil {
		serializedNewWorkflowSnapshot, err = m.SerializeWorkflowSnapshot(request.NewWorkflowSnapshot, request.Encoding)
		if err != nil {
			return nil, err
		}
	}

	newRequest := &InternalUpdateWorkflowExecutionRequest{
		RangeID:               request.RangeID,
		Mode:                  request.Mode,
		UpdateWorkflowMutation: *serializedWorkflowMutation,
		NewWorkflowSnapshot:   serializedNewWorkflowSnapshot,
		WorkflowRequestMode:   request.WorkflowRequestMode,
		CurrentTimeStamp:      m.timeSrc.Now(),
	}
	msuss := m.statsComputer.computeMutableStateUpdateStats(newRequest)
	
	// Perform update in persistence
	err = m.persistence.UpdateWorkflowExecution(ctx, newRequest)
	if err != nil {
		return nil, err
	}

	// Invalidate cache for the updated workflow
	if m.cacheEnabled {
		// Invalidate main workflow
		if serializedWorkflowMutation.ExecutionInfo != nil {
			key := buildWorkflowExecutionCacheKey(
				serializedWorkflowMutation.ExecutionInfo.DomainID,
				serializedWorkflowMutation.ExecutionInfo.WorkflowID,
				serializedWorkflowMutation.ExecutionInfo.RunID,
			)
			m.cache.Invalidate(key)
		}
		
		// Invalidate new workflow if any
		if serializedNewWorkflowSnapshot != nil && serializedNewWorkflowSnapshot.ExecutionInfo != nil {
			key := buildWorkflowExecutionCacheKey(
				serializedNewWorkflowSnapshot.ExecutionInfo.DomainID,
				serializedNewWorkflowSnapshot.ExecutionInfo.WorkflowID,
				serializedNewWorkflowSnapshot.ExecutionInfo.RunID,
			)
			m.cache.Invalidate(key)
		}
	}

	return &UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: msuss}, nil
}

// ConflictResolveWorkflowExecution resolves conflicts with cache invalidation
func (m *executionManagerImpl) ConflictResolveWorkflowExecution(
	ctx context.Context,
	request *ConflictResolveWorkflowExecutionRequest,
) (*ConflictResolveWorkflowExecutionResponse, error) {
	sw := m.metricsClient.StartTimer(metrics.PersistenceConflictResolveWorkflowExecutionScope, metrics.PersistenceLatency)
	defer sw.Stop()

	serializedResetWorkflowSnapshot, err := m.SerializeWorkflowSnapshot(&request.ResetWorkflowSnapshot, request.Encoding)
	if err != nil {
		return nil, err
	}
	var serializedCurrentWorkflowMutation *InternalWorkflowMutation
	if request.CurrentWorkflowMutation != nil {
		serializedCurrentWorkflowMutation, err = m.SerializeWorkflowMutation(request.CurrentWorkflowMutation, request.Encoding)
		if err != nil {
			return nil, err
		}
	}
	var serializedNewWorkflowSnapshot *InternalWorkflowSnapshot
	if request.NewWorkflowSnapshot != nil {
		serializedNewWorkflowSnapshot, err = m.SerializeWorkflowSnapshot(request.NewWorkflowSnapshot, request.Encoding)
		if err != nil {
			return nil, err
		}
	}

	newRequest := &InternalConflictResolveWorkflowExecutionRequest{
		RangeID:                  request.RangeID,
		Mode:                     request.Mode,
		ResetWorkflowSnapshot:    *serializedResetWorkflowSnapshot,
		NewWorkflowSnapshot:      serializedNewWorkflowSnapshot,
		CurrentWorkflowMutation:  serializedCurrentWorkflowMutation,
		WorkflowRequestMode:      request.WorkflowRequestMode,
		CurrentTimeStamp:         m.timeSrc.Now(),
	}
	msuss := m.statsComputer.computeMutableStateConflictResolveStats(newRequest)
	
	err = m.persistence.ConflictResolveWorkflowExecution(ctx, newRequest)
	if err != nil {
		return nil, err
	}

	// Invalidate cache entries for all affected workflows
	if m.cacheEnabled {
		// Reset workflow
		if serializedResetWorkflowSnapshot != nil && serializedResetWorkflowSnapshot.ExecutionInfo != nil {
			key := buildWorkflowExecutionCacheKey(
				serializedResetWorkflowSnapshot.ExecutionInfo.DomainID,
				serializedResetWorkflowSnapshot.ExecutionInfo.WorkflowID,
				serializedResetWorkflowSnapshot.ExecutionInfo.RunID,
			)
			m.cache.Invalidate(key)
		}
		
		// Current workflow
		if serializedCurrentWorkflowMutation != nil && serializedCurrentWorkflowMutation.ExecutionInfo != nil {
			key := buildWorkflowExecutionCacheKey(
				serializedCurrentWorkflowMutation.ExecutionInfo.DomainID,
				serializedCurrentWorkflowMutation.ExecutionInfo.WorkflowID,
				serializedCurrentWorkflowMutation.ExecutionInfo.RunID,
			)
			m.cache.Invalidate(key)
		}
		
		// New workflow
		if serializedNewWorkflowSnapshot != nil && serializedNewWorkflowSnapshot.ExecutionInfo != nil {
			key := buildWorkflowExecutionCacheKey(
				serializedNewWorkflowSnapshot.ExecutionInfo.DomainID,
				serializedNewWorkflowSnapshot.ExecutionInfo.WorkflowID,
				serializedNewWorkflowSnapshot.ExecutionInfo.RunID,
			)
			m.cache.Invalidate(key)
		}
	}

	return &ConflictResolveWorkflowExecutionResponse{MutableStateUpdateSessionStats: msuss}, nil
}

// CreateWorkflowExecution creates a workflow with cache invalidation
func (m *executionManagerImpl) CreateWorkflowExecution(
	ctx context.Context,
	request *CreateWorkflowExecutionRequest,
) (*CreateWorkflowExecutionResponse, error) {
	sw := m.metricsClient.StartTimer(metrics.PersistenceCreateWorkflowExecutionScope, metrics.PersistenceLatency)
	defer sw.Stop()

	encoding := constants.EncodingTypeThriftRW
	serializedNewWorkflowSnapshot, err := m.SerializeWorkflowSnapshot(&request.NewWorkflowSnapshot, encoding)
	if err != nil {
		return nil, err
	}

	newRequest := &InternalCreateWorkflowExecutionRequest{
		RangeID:                  request.RangeID,
		Mode:                     request.Mode,
		PreviousRunID:            request.PreviousRunID,
		PreviousLastWriteVersion: request.PreviousLastWriteVersion,
		NewWorkflowSnapshot:      *serializedNewWorkflowSnapshot,
		WorkflowRequestMode:      request.WorkflowRequestMode,
		CurrentTimeStamp:         m.timeSrc.Now(),
	}

	msuss := m.statsComputer.computeMutableStateCreateStats(newRequest)
	response, err := m.persistence.CreateWorkflowExecution(ctx, newRequest)
	if err != nil {
		return nil, err
	}

	// Invalidate cache for the created workflow
	if m.cacheEnabled && serializedNewWorkflowSnapshot != nil && serializedNewWorkflowSnapshot.ExecutionInfo != nil {
		key := buildWorkflowExecutionCacheKey(
			serializedNewWorkflowSnapshot.ExecutionInfo.DomainID,
			serializedNewWorkflowSnapshot.ExecutionInfo.WorkflowID,
			serializedNewWorkflowSnapshot.ExecutionInfo.RunID,
		)
		m.cache.Invalidate(key)
	}

	return &CreateWorkflowExecutionResponse{
		MutableStateUpdateSessionStats: msuss,
	}, nil
}

// Helper function to run tasks in parallel and return the first error encountered
func runParallel(fns ...func() error) error {
	if len(fns) == 0 {
		return nil
	}
	
	errorCh := make(chan error, len(fns))
	wg := sync.WaitGroup{}
	wg.Add(len(fns))
	
	for _, fn := range fns {
		go func(f func() error) {
			defer wg.Done()
			if err := f(); err != nil {
				select {
				case errorCh <- err:
				default:
					// Channel already has an error
				}
			}
		}(fn)
	}
	
	// Wait for all functions to complete
	wg.Wait()
	
	// Check for errors
	select {
	case err := <-errorCh:
		return err
	default:
		return nil
	}
}

// DeserializeExecutionInfo deserializes workflow execution info
func (m *executionManagerImpl) DeserializeExecutionInfo(
	info *InternalWorkflowExecutionInfo,
) (*WorkflowExecutionInfo, *ExecutionStats, error) {
	if info == nil {
		return nil, nil, nil
	}
	
	var completionEvent *types.HistoryEvent
	var autoResetPoints *types.ResetPoints
	var activeClusterSelectionPolicy *types.ActiveClusterSelectionPolicy
	var err error
	
	// Deserialize complex objects in parallel
	err = runParallel(
		func() error {
			var err error
			completionEvent, err = m.serializer.DeserializeEvent(info.CompletionEvent)
			return err
		},
		func() error {
			var err error
			autoResetPoints, err = m.serializer.DeserializeResetPoints(info.AutoResetPoints)
			return err
		},
		func() error {
			var err error
			activeClusterSelectionPolicy, err = m.serializer.DeserializeActiveClusterSelectionPolicy(info.ActiveClusterSelectionPolicy)
			return err
		},
	)
	if err != nil {
		return nil, nil, err
	}

	// Prepare execution info with all fields
	newInfo := &WorkflowExecutionInfo{
		CompletionEvent:                    completionEvent,
		DomainID:                           info.DomainID,
		WorkflowID:                         info.WorkflowID,
		RunID:                              info.RunID,
		FirstExecutionRunID:                info.FirstExecutionRunID,
		ParentDomainID:                     info.ParentDomainID,
		ParentWorkflowID:                   info.ParentWorkflowID,
		ParentRunID:                        info.ParentRunID,
		InitiatedID:                        info.InitiatedID,
		CompletionEventBatchID:             info.CompletionEventBatchID,
		TaskList:                           info.TaskList,
		IsCron:                             len(info.CronSchedule) > 0,
		WorkflowTypeName:                   info.WorkflowTypeName,
		WorkflowTimeout:                    int32(info.WorkflowTimeout.Seconds()),
		DecisionStartToCloseTimeout:        int32(info.DecisionStartToCloseTimeout.Seconds()),
		ExecutionContext:                   info.ExecutionContext,
		State:                              info.State,
		CloseStatus:                        info.CloseStatus,
		LastFirstEventID:                   info.LastFirstEventID,
		LastEventTaskID:                    info.LastEventTaskID,
		NextEventID:                        info.NextEventID,
		LastProcessedEvent:                 info.LastProcessedEvent,
		StartTimestamp:                     info.StartTimestamp,
		LastUpdatedTimestamp:               info.LastUpdatedTimestamp,
		CreateRequestID:                    info.CreateRequestID,
		SignalCount:                        info.SignalCount,
		DecisionVersion:                    info.DecisionVersion,
		DecisionScheduleID:                 info.DecisionScheduleID,
		DecisionStartedID:                  info.DecisionStartedID,
		DecisionRequestID:                  info.DecisionRequestID,
		DecisionTimeout:                    int32(info.DecisionTimeout.Seconds()),
		DecisionAttempt:                    info.DecisionAttempt,
		DecisionStartedTimestamp:           info.DecisionStartedTimestamp.UnixNano(),
		DecisionScheduledTimestamp:         info.DecisionScheduledTimestamp.UnixNano(),
		DecisionOriginalScheduledTimestamp: info.DecisionOriginalScheduledTimestamp.UnixNano(),
		CancelRequested:                    info.CancelRequested,
		CancelRequestID:                    info.CancelRequestID,
		StickyTaskList:                     info.StickyTaskList,
		StickyScheduleToStartTimeout:       int32(info.StickyScheduleToStartTimeout.Seconds()),
		ClientLibraryVersion:               info.ClientLibraryVersion,
		ClientFeatureVersion:               info.ClientFeatureVersion,
		ClientImpl:                         info.ClientImpl,
		Attempt:                            info.Attempt,
		HasRetryPolicy:                     info.HasRetryPolicy,
		InitialInterval:                    int32(info.InitialInterval.Seconds()),
		BackoffCoefficient:                 info.BackoffCoefficient,
		MaximumInterval:                    int32(info.MaximumInterval.Seconds()),
		ExpirationTime:                     info.ExpirationTime,
		MaximumAttempts:                    info.MaximumAttempts,
		NonRetriableErrors:                 info.NonRetriableErrors,
		BranchToken:                        info.BranchToken,
		CronSchedule:                       info.CronSchedule,
		CronOverlapPolicy:                  info.CronOverlapPolicy,
		ExpirationSeconds:                  int32(info.ExpirationInterval.Seconds()),
		AutoResetPoints:                    autoResetPoints,
		SearchAttributes:                   info.SearchAttributes,
		Memo:                               info.Memo,
		PartitionConfig:                    info.PartitionConfig,
		ActiveClusterSelectionPolicy:       activeClusterSelectionPolicy,
	}
	
	newStats := &ExecutionStats{
		HistorySize: info.HistorySize,
	}
	
	return newInfo, newStats, nil
}

// DeserializeBufferedEvents deserializes buffered events efficiently
func (m *executionManagerImpl) DeserializeBufferedEvents(
	blobs []*DataBlob,
) ([]*types.HistoryEvent, error) {
	if len(blobs) == 0 {
		return nil, nil
	}
	
	// Pre-allocate result slice with estimated capacity
	events := make([]*types.HistoryEvent, 0, len(blobs)*10) // Assume ~10 events per blob
	
	for _, b := range blobs {
		history, err := m.serializer.DeserializeBatchEvents(b)
		if err != nil {
			return nil, err
		}
		events = append(events, history...)
	}
	
	return events, nil
}

// DeserializeChildExecutionInfos deserializes child execution infos
func (m *executionManagerImpl) DeserializeChildExecutionInfos(
	infos map[int64]*InternalChildExecutionInfo,
) (map[int64]*ChildExecutionInfo, error) {
	if len(infos) == 0 {
		return make(map[int64]*ChildExecutionInfo), nil
	}
	
	newInfos := make(map[int64]*ChildExecutionInfo, len(infos))
	
	// Process each child execution info
	for k, v := range infos {
		// Deserialize events
		initiatedEvent, err := m.serializer.DeserializeEvent(v.InitiatedEvent)
		if err != nil {
			return nil, err
		}
		startedEvent, err := m.serializer.DeserializeEvent(v.StartedEvent)
		if err != nil {
			return nil, err
		}
		
		c := &ChildExecutionInfo{
			InitiatedEvent:         initiatedEvent,
			StartedEvent:           startedEvent,
			Version:                v.Version,
			InitiatedID:            v.InitiatedID,
			InitiatedEventBatchID:  v.InitiatedEventBatchID,
			StartedID:              v.StartedID,
			StartedWorkflowID:      v.StartedWorkflowID,
			StartedRunID:           v.StartedRunID,
			CreateRequestID:        v.CreateRequestID,
			DomainID:               v.DomainID,
			DomainNameDEPRECATED:   v.DomainNameDEPRECATED,
			WorkflowTypeName:       v.WorkflowTypeName,
			ParentClosePolicy:      v.ParentClosePolicy,
		}

		// Backward compatibility
		if startedEvent != nil && 
		   startedEvent.ChildWorkflowExecutionStartedEventAttributes != nil &&
		   startedEvent.ChildWorkflowExecutionStartedEventAttributes.WorkflowExecution != nil {
			execution := startedEvent.ChildWorkflowExecutionStartedEventAttributes.WorkflowExecution
			c.StartedWorkflowID = execution.GetWorkflowID()
			c.StartedRunID = execution.GetRunID()
		}
		
		newInfos[k] = c
	}
	
	return newInfos, nil
}

// DeserializeActivityInfos deserializes activity infos
func (m *executionManagerImpl) DeserializeActivityInfos(
	infos map[int64]*InternalActivityInfo,
) (map[int64]*ActivityInfo, error) {
	if len(infos) == 0 {
		return make(map[int64]*ActivityInfo), nil
	}
	
	newInfos := make(map[int64]*ActivityInfo, len(infos))
	
	// Process each activity info
	for k, v := range infos {
		// Deserialize events
		scheduledEvent, err := m.serializer.DeserializeEvent(v.ScheduledEvent)
		if err != nil {
			return nil, err
		}
		startedEvent, err := m.serializer.DeserializeEvent(v.StartedEvent)
		if err != nil {
			return nil, err
		}
		
		a := &ActivityInfo{
			ScheduledEvent:                        scheduledEvent,
			StartedEvent:                          startedEvent,
			Version:                               v.Version,
			ScheduleID:                            v.ScheduleID,
			ScheduledEventBatchID:                 v.ScheduledEventBatchID,
			ScheduledTime:                         v.ScheduledTime,
			StartedID:                             v.StartedID,
			StartedTime:                           v.StartedTime,
			ActivityID:                            v.ActivityID,
			RequestID:                             v.RequestID,
			Details:                               v.Details,
			ScheduleToStartTimeout:                int32(v.ScheduleToStartTimeout.Seconds()),
			ScheduleToCloseTimeout:                int32(v.ScheduleToCloseTimeout.Seconds()),
			StartToCloseTimeout:                   int32(v.StartToCloseTimeout.Seconds()),
			HeartbeatTimeout:                      int32(v.HeartbeatTimeout.Seconds()),
			CancelRequested:                       v.CancelRequested,
			CancelRequestID:                       v.CancelRequestID,
			LastHeartBeatUpdatedTime:              v.LastHeartBeatUpdatedTime,
			TimerTaskStatus:                       v.TimerTaskStatus,
			Attempt:                               v.Attempt,
			DomainID:                              v.DomainID,
			StartedIdentity:                       v.StartedIdentity,
			TaskList:                              v.TaskList,
			HasRetryPolicy:                        v.HasRetryPolicy,
			InitialInterval:                       int32(v.InitialInterval.Seconds()),
			BackoffCoefficient:                    v.BackoffCoefficient,
			MaximumInterval:                       int32(v.MaximumInterval.Seconds()),
			ExpirationTime:                        v.ExpirationTime,
			MaximumAttempts:                       v.MaximumAttempts,
			NonRetriableErrors:                    v.NonRetriableErrors,
			LastFailureReason:                     v.LastFailureReason,
			LastWorkerIdentity:                    v.LastWorkerIdentity,
			LastFailureDetails:                    v.LastFailureDetails,
			LastHeartbeatTimeoutVisibilityInSeconds: v.LastHeartbeatTimeoutVisibilityInSeconds,
		}
		
		newInfos[k] = a
	}
	
	return newInfos, nil
}

// SerializeUpsertActivityInfos serializes activity infos efficiently
func (m *executionManagerImpl) SerializeUpsertActivityInfos(
	infos []*ActivityInfo,
	encoding constants.EncodingType,
) ([]*InternalActivityInfo, error) {
	if len(infos) == 0 {
		return nil, nil
	}
	
	newInfos := make([]*InternalActivityInfo, 0, len(infos))
	
	for _, v := range infos {
		// Serialize events
		scheduledEvent, err := m.serializer.SerializeEvent(v.ScheduledEvent, encoding)
		if err != nil {
			return nil, err
		}
		startedEvent, err := m.serializer.SerializeEvent(v.StartedEvent, encoding)
		if err != nil {
			return nil, err
		}
		
		i := &InternalActivityInfo{
			Version:                                 v.Version,
			ScheduleID:                              v.ScheduleID,
			ScheduledEventBatchID:                   v.ScheduledEventBatchID,
			ScheduledEvent:                          scheduledEvent,
			ScheduledTime:                           v.ScheduledTime,
			StartedID:                               v.StartedID,
			StartedEvent:                            startedEvent,
			StartedTime:                             v.StartedTime,
			ActivityID:                              v.ActivityID,
			RequestID:                               v.RequestID,
			Details:                                 v.Details,
			ScheduleToStartTimeout:                  common.SecondsToDuration(int64(v.ScheduleToStartTimeout)),
			ScheduleToCloseTimeout:                  common.SecondsToDuration(int64(v.ScheduleToCloseTimeout)),
			StartToCloseTimeout:                     common.SecondsToDuration(int64(v.StartToCloseTimeout)),
			HeartbeatTimeout:                        common.SecondsToDuration(int64(v.HeartbeatTimeout)),
			CancelRequested:                         v.CancelRequested,
			CancelRequestID:                         v.CancelRequestID,
			LastHeartBeatUpdatedTime:                v.LastHeartBeatUpdatedTime,
			TimerTaskStatus:                         v.TimerTaskStatus,
			Attempt:                                 v.Attempt,
			DomainID:                                v.DomainID,
			StartedIdentity:                         v.StartedIdentity,
			TaskList:                                v.TaskList,
			HasRetryPolicy:                          v.HasRetryPolicy,
			InitialInterval:                         common.SecondsToDuration(int64(v.InitialInterval)),
			BackoffCoefficient:                      v.BackoffCoefficient,
			MaximumInterval:                         common.SecondsToDuration(int64(v.MaximumInterval)),
			ExpirationTime:                          v.ExpirationTime,
			MaximumAttempts:                         v.MaximumAttempts,
			NonRetriableErrors:                      v.NonRetriableErrors,
			LastFailureReason:                       v.LastFailureReason,
			LastWorkerIdentity:                      v.LastWorkerIdentity,
			LastFailureDetails:                      v.LastFailureDetails,
			LastHeartbeatTimeoutVisibilityInSeconds: v.LastHeartbeatTimeoutVisibilityInSeconds,
		}
		
		newInfos = append(newInfos, i)
	}
	
	return newInfos, nil
}

// SerializeUpsertChildExecutionInfos serializes child execution infos
func (m *executionManagerImpl) SerializeUpsertChildExecutionInfos(
	infos []*ChildExecutionInfo,
	encoding constants.EncodingType,
) ([]*InternalChildExecutionInfo, error) {
	if len(infos) == 0 {
		return nil, nil
	}
	
	newInfos := make([]*InternalChildExecutionInfo, 0, len(infos))
	
	for _, v := range infos {
		// Serialize events
		initiatedEvent, err := m.serializer.SerializeEvent(v.InitiatedEvent, encoding)
		if err != nil {
			return nil, err
		}
		startedEvent, err := m.serializer.SerializeEvent(v.StartedEvent, encoding)
		if err != nil {
			return nil, err
		}
		
		i := &InternalChildExecutionInfo{
			InitiatedEvent:         initiatedEvent,
			StartedEvent:           startedEvent,
			Version:                v.Version,
			InitiatedID:            v.InitiatedID,
			InitiatedEventBatchID:  v.InitiatedEventBatchID,
			CreateRequestID:        v.CreateRequestID,
			StartedID:              v.StartedID,
			StartedWorkflowID:      v.StartedWorkflowID,
			StartedRunID:           v.StartedRunID,
			DomainID:               v.DomainID,
			DomainNameDEPRECATED:   v.DomainNameDEPRECATED,
			WorkflowTypeName:       v.WorkflowTypeName,
			ParentClosePolicy:      v.ParentClosePolicy,
		}
		
		newInfos = append(newInfos, i)
	}
	
	return newInfos, nil
}

// SerializeExecutionInfo serializes workflow execution info
func (m *executionManagerImpl) SerializeExecutionInfo(
	info *WorkflowExecutionInfo,
	stats *ExecutionStats,
	encoding constants.EncodingType,
) (*InternalWorkflowExecutionInfo, error) {
	if info == nil {
		return &InternalWorkflowExecutionInfo{}, nil
	}
	
	var completionEvent *DataBlob
	var resetPoints *DataBlob
	var activeClusterSelectionPolicy *DataBlob
	var err error
	
	// Serialize complex fields in parallel
	err = runParallel(
		func() error {
			var err error
			completionEvent, err = m.serializer.SerializeEvent(info.CompletionEvent, encoding)
			return err
		},
		func() error {
			var err error
			resetPoints, err = m.serializer.SerializeResetPoints(info.AutoResetPoints, encoding)
			return err
		},
		func() error {
			var err error
			activeClusterSelectionPolicy, err = m.serializer.SerializeActiveClusterSelectionPolicy(info.ActiveClusterSelectionPolicy, encoding)
			return err
		},
	)
	if err != nil {
		return nil, err
	}

	return &InternalWorkflowExecutionInfo{
		DomainID:                           info.DomainID,
		WorkflowID:                         info.WorkflowID,
		RunID:                              info.RunID,
		FirstExecutionRunID:                info.FirstExecutionRunID,
		ParentDomainID:                     info.ParentDomainID,
		ParentWorkflowID:                   info.ParentWorkflowID,
		ParentRunID:                        info.ParentRunID,
		InitiatedID:                        info.InitiatedID,
		CompletionEventBatchID:             info.CompletionEventBatchID,
		CompletionEvent:                    completionEvent,
		TaskList:                           info.TaskList,
		WorkflowTypeName:                   info.WorkflowTypeName,
		WorkflowTimeout:                    common.SecondsToDuration(int64(info.WorkflowTimeout)),
		DecisionStartToCloseTimeout:        common.SecondsToDuration(int64(info.DecisionStartToCloseTimeout)),
		ExecutionContext:                   info.ExecutionContext,
		State:                              info.State,
		CloseStatus:                        info.CloseStatus,
		LastFirstEventID:                   info.LastFirstEventID,
		LastEventTaskID:                    info.LastEventTaskID,
		NextEventID:                        info.NextEventID,
		LastProcessedEvent:                 info.LastProcessedEvent,
		StartTimestamp:                     info.StartTimestamp,
		LastUpdatedTimestamp:               info.LastUpdatedTimestamp,
		CreateRequestID:                    info.CreateRequestID,
		SignalCount:                        info.SignalCount,
		DecisionVersion:                    info.DecisionVersion,
		DecisionScheduleID:                 info.DecisionScheduleID,
		DecisionStartedID:                  info.DecisionStartedID,
		DecisionRequestID:                  info.DecisionRequestID,
		DecisionTimeout:                    common.SecondsToDuration(int64(info.DecisionTimeout)),
		DecisionAttempt:                    info.DecisionAttempt,
		DecisionStartedTimestamp:           time.Unix(0, info.DecisionStartedTimestamp).UTC(),
		DecisionScheduledTimestamp:         time.Unix(0, info.DecisionScheduledTimestamp).UTC(),
		DecisionOriginalScheduledTimestamp: time.Unix(0, info.DecisionOriginalScheduledTimestamp).UTC(),
		CancelRequested:                    info.CancelRequested,
		CancelRequestID:                    info.CancelRequestID,
		StickyTaskList:                     info.StickyTaskList,
		StickyScheduleToStartTimeout:       common.SecondsToDuration(int64(info.StickyScheduleToStartTimeout)),
		ClientLibraryVersion:               info.ClientLibraryVersion,
		ClientFeatureVersion:               info.ClientFeatureVersion,
		ClientImpl:                         info.ClientImpl,
		AutoResetPoints:                    resetPoints,
		Attempt:                            info.Attempt,
		HasRetryPolicy:                     info.HasRetryPolicy,
		InitialInterval:                    common.SecondsToDuration(int64(info.InitialInterval)),
		BackoffCoefficient:                 info.BackoffCoefficient,
		MaximumInterval:                    common.SecondsToDuration(int64(info.MaximumInterval)),
		ExpirationTime:                     info.ExpirationTime,
		MaximumAttempts:                    info.MaximumAttempts,
		NonRetriableErrors:                 info.NonRetriableErrors,
		BranchToken:                        info.BranchToken,
		CronSchedule:                       info.CronSchedule,
		ExpirationInterval:                 common.SecondsToDuration(int64(info.ExpirationSeconds)),
		Memo:                               info.Memo,
		SearchAttributes:                   info.SearchAttributes,
		PartitionConfig:                    info.PartitionConfig,
		CronOverlapPolicy:                  info.CronOverlapPolicy,
		ActiveClusterSelectionPolicy:       activeClusterSelectionPolicy,

		// Stats
		HistorySize: stats.HistorySize,
	}, nil
}

// SerializeWorkflowMutation optimized for empty/trivial mutations
func (m *executionManagerImpl) SerializeWorkflowMutation(
	input *WorkflowMutation,
	encoding constants.EncodingType,
) (*InternalWorkflowMutation, error) {
	if input == nil {
		return &InternalWorkflowMutation{}, nil
	}
	
	// Fast path for minimal mutation (only deletions)
	if input.ExecutionInfo == nil &&
	   input.VersionHistories == nil &&
	   len(input.UpsertActivityInfos) == 0 &&
	   len(input.UpsertChildExecutionInfos) == 0 &&
	   input.NewBufferedEvents == nil &&
	   input.Checksum == nil &&
	   len(input.UpsertRequestCancelInfos) == 0 &&
	   len(input.UpsertSignalInfos) == 0 &&
	   len(input.UpsertSignalRequestedIDs) == 0 &&
	   len(input.TasksByCategory) == 0 {
		return &InternalWorkflowMutation{
			DeleteActivityInfos:       input.DeleteActivityInfos,
			DeleteTimerInfos:          input.DeleteTimerInfos,
			DeleteChildExecutionInfos: input.DeleteChildExecutionInfos,
			DeleteRequestCancelInfos:  input.DeleteRequestCancelInfos,
			DeleteSignalInfos:         input.DeleteSignalInfos,
			DeleteSignalRequestedIDs:  input.DeleteSignalRequestedIDs,
			ClearBufferedEvents:       input.ClearBufferedEvents,
			Condition:                 input.Condition,
		}, nil
	}

	// Serialize components in parallel
	var serializedExecutionInfo *InternalWorkflowExecutionInfo
	var serializedVersionHistories *DataBlob
	var serializedUpsertActivityInfos []*InternalActivityInfo
	var serializedUpsertChildExecutionInfos []*InternalChildExecutionInfo
	var serializedNewBufferedEvents *DataBlob
	var startVersion, lastWriteVersion int64
	var checksumData *DataBlob
	var err error

	// Use parallel serialization only for mutations with many components
	if len(input.UpsertActivityInfos) > 5 || len(input.UpsertChildExecutionInfos) > 5 {
		err = runParallel(
			// Execution info
			func() error {
				var err error
				serializedExecutionInfo, err = m.SerializeExecutionInfo(input.ExecutionInfo, input.ExecutionStats, encoding)
				return err
			},
			// Version histories
			func() error {
				var err error
				serializedVersionHistories, err = m.SerializeVersionHistories(input.VersionHistories, encoding)
				if err != nil {
					return err
				}
				
				// Version fields
				startVersion, err = getStartVersion(input.VersionHistories)
				if err != nil {
					return err
				}
				lastWriteVersion, err = getLastWriteVersion(input.VersionHistories)
				return err
			},
			// Activity infos
			func() error {
				var err error
				serializedUpsertActivityInfos, err = m.SerializeUpsertActivityInfos(input.UpsertActivityInfos, encoding)
				return err
			},
			// Child execution infos
			func() error {
				var err error
				serializedUpsertChildExecutionInfos, err = m.SerializeUpsertChildExecutionInfos(input.UpsertChildExecutionInfos, encoding)
				return err
			},
			// Buffered events
			func() error {
				if input.NewBufferedEvents != nil {
					var err error
					serializedNewBufferedEvents, err = m.serializer.SerializeBatchEvents(input.NewBufferedEvents, encoding)
					return err
				}
				return nil
			},
			// Checksum
			func() error {
				var err error
				checksumData, err = m.serializer.SerializeChecksum(input.Checksum, constants.EncodingTypeJSON)
				return err
			},
		)
	} else {
		// Serialize sequentially for smaller mutations
		serializedExecutionInfo, err = m.SerializeExecutionInfo(input.ExecutionInfo, input.ExecutionStats, encoding)
		if err != nil {
			return nil, err
		}
		
		serializedVersionHistories, err = m.SerializeVersionHistories(input.VersionHistories, encoding)
		if err != nil {
			return nil, err
		}
		
		startVersion, err = getStartVersion(input.VersionHistories)
		if err != nil {
			return nil, err
		}
		
		lastWriteVersion, err = getLastWriteVersion(input.VersionHistories)
		if err != nil {
			return nil, err
		}
		
		serializedUpsertActivityInfos, err = m.SerializeUpsertActivityInfos(input.UpsertActivityInfos, encoding)
		if err != nil {
			return nil, err
		}
		
		serializedUpsertChildExecutionInfos, err = m.SerializeUpsertChildExecutionInfos(input.UpsertChildExecutionInfos, encoding)
		if err != nil {
			return nil, err
		}
		
		if input.NewBufferedEvents != nil {
			serializedNewBufferedEvents, err = m.serializer.SerializeBatchEvents(input.NewBufferedEvents, encoding)
			if err != nil {
				return nil, err
			}
		}
		
		checksumData, err = m.serializer.SerializeChecksum(input.Checksum, constants.EncodingTypeJSON)
		if err != nil {
			return nil, err
		}
	}
	
	if err != nil {
		return nil, err
	}

	return &InternalWorkflowMutation{
		ExecutionInfo:             serializedExecutionInfo,
		VersionHistories:          serializedVersionHistories,
		StartVersion:              startVersion,
		LastWriteVersion:          lastWriteVersion,
		UpsertActivityInfos:       serializedUpsertActivityInfos,
		DeleteActivityInfos:       input.DeleteActivityInfos,
		UpsertTimerInfos:          input.UpsertTimerInfos,
		DeleteTimerInfos:          input.DeleteTimerInfos,
		UpsertChildExecutionInfos: serializedUpsertChildExecutionInfos,
		DeleteChildExecutionInfos: input.DeleteChildExecutionInfos,
		UpsertRequestCancelInfos:  input.UpsertRequestCancelInfos,
		DeleteRequestCancelInfos:  input.DeleteRequestCancelInfos,
		UpsertSignalInfos:         input.UpsertSignalInfos,
		DeleteSignalInfos:         input.DeleteSignalInfos,
		UpsertSignalRequestedIDs:  input.UpsertSignalRequestedIDs,
		DeleteSignalRequestedIDs:  input.DeleteSignalRequestedIDs,
		NewBufferedEvents:         serializedNewBufferedEvents,
		ClearBufferedEvents:       input.ClearBufferedEvents,
		TasksByCategory:           input.TasksByCategory,
		WorkflowRequests:          input.WorkflowRequests,
		Condition:                 input.Condition,
		Checksum:                  input.Checksum,
		ChecksumData:              checksumData,
	}, nil
}

// SerializeWorkflowSnapshot serializes workflow snapshot
func (m *executionManagerImpl) SerializeWorkflowSnapshot(
	input *WorkflowSnapshot,
	encoding constants.EncodingType,
) (*InternalWorkflowSnapshot, error) {
	if input == nil {
		return nil, nil
	}
	
	// Serialize components in parallel for large snapshots
	var serializedExecutionInfo *InternalWorkflowExecutionInfo
	var serializedVersionHistories *DataBlob
	var serializedActivityInfos []*InternalActivityInfo
	var serializedChildExecutionInfos []*InternalChildExecutionInfo
	var startVersion, lastWriteVersion int64
	var checksumData *DataBlob
	var err error
	
	// Use parallel serialization if snapshot has significant data
	if len(input.ActivityInfos) > 5 || len(input.ChildExecutionInfos) > 5 {
		var wg sync.WaitGroup
		type result[T any] struct {
			value T
			err   error
		}
		execInfoCh := make(chan result[*InternalWorkflowExecutionInfo], 1)
		verHistCh := make(chan result[*DataBlob], 1)
		actCh := make(chan result[[]*InternalActivityInfo], 1)
		childCh := make(chan result[[]*InternalChildExecutionInfo], 1)
		checksumCh := make(chan result[*DataBlob], 1)
		versionCh := make(chan result[struct{ start, last int64 }], 1)

		// Launch parallel serializations
		wg.Add(6)
		go func() {
			defer wg.Done()
			val, err := m.SerializeExecutionInfo(input.ExecutionInfo, input.ExecutionStats, encoding)
			execInfoCh <- result[*InternalWorkflowExecutionInfo]{val, err}
		}()
		go func() {
			defer wg.Done()
			val, err := m.SerializeVersionHistories(input.VersionHistories, encoding)
			verHistCh <- result[*DataBlob]{val, err}
		}()
		go func() {
			defer wg.Done()
			val, err := m.SerializeUpsertActivityInfos(input.ActivityInfos, encoding)
			actCh <- result[[]*InternalActivityInfo]{val, err}
		}()
		go func() {
			defer wg.Done()
			val, err := m.SerializeUpsertChildExecutionInfos(input.ChildExecutionInfos, encoding)
			childCh <- result[[]*InternalChildExecutionInfo]{val, err}
		}()
		go func() {
			defer wg.Done()
			val, err := m.serializer.SerializeChecksum(input.Checksum, constants.EncodingTypeJSON)
			checksumCh <- result[*DataBlob]{val, err}
		}()
		go func() {
			defer wg.Done()
			start, err := getStartVersion(input.VersionHistories)
			if err != nil {
				versionCh <- result[struct{ start, last int64 }]{err: err}
				return
			}
			last, err := getLastWriteVersion(input.VersionHistories)
			versionCh <- result[struct{ start, last int64 }]{
				value: struct{ start, last int64 }{start, last},
				err:   err,
			}
		}()

		// Collect results and check for errors
		for i := 0; i < 6; i++ {
			select {
			case r := <-execInfoCh:
				if r.err != nil {
					err = r.err
					break
				}
				serializedExecutionInfo = r.value
			case r := <-verHistCh:
				if r.err != nil {
					err = r.err
					break
				}
				serializedVersionHistories = r.value
			case r := <-actCh:
				if r.err != nil {
					err = r.err
					break
				}
				serializedActivityInfos = r.value
			case r := <-childCh:
				if r.err != nil {
					err = r.err
					break
				}
				serializedChildExecutionInfos = r.value
			case r := <-checksumCh:
				if r.err != nil {
					err = r.err
					break
				}
				checksumData = r.value
			case r := <-versionCh:
				if r.err != nil {
					err = r.err
					break
				}
				startVersion = r.value.start
				lastWriteVersion = r.value.last
			}
			if err != nil {
				break
			}
		}
		
		// Drain channels to avoid goroutine leaks in case of error
		if err != nil {
			go func() {
				for range execInfoCh {}
				for range verHistCh {}
				for range actCh {}
				for range childCh {}
				for range checksumCh {}
				for range versionCh {}
			}()
		}
		
		wg.Wait()
	} else {
		// Sequential serialization for smaller snapshots
		serializedExecutionInfo, err = m.SerializeExecutionInfo(input.ExecutionInfo, input.ExecutionStats, encoding)
		if err != nil {
			return nil, err
		}
		
		serializedVersionHistories, err = m.SerializeVersionHistories(input.VersionHistories, encoding)
		if err != nil {
			return nil, err
		}
		
		startVersion, err = getStartVersion(input.VersionHistories)
		if err != nil {
			return nil, err
		}
		
		lastWriteVersion, err = getLastWriteVersion(input.VersionHistories)
		if err != nil {
			return nil, err
		}
		
		serializedActivityInfos, err = m.SerializeUpsertActivityInfos(input.ActivityInfos, encoding)
		if err != nil {
			return nil, err
		}
		
		serializedChildExecutionInfos, err = m.SerializeUpsertChildExecutionInfos(input.ChildExecutionInfos, encoding)
		if err != nil {
			return nil, err
		}
		
		checksumData, err = m.serializer.SerializeChecksum(input.Checksum, constants.EncodingTypeJSON)
		if err != nil {
			return nil, err
		}
	}
	
	if err != nil {
		return nil, err
	}

	return &InternalWorkflowSnapshot{
		ExecutionInfo:       serializedExecutionInfo,
		VersionHistories:    serializedVersionHistories,
		StartVersion:        startVersion,
		LastWriteVersion:    lastWriteVersion,
		ActivityInfos:       serializedActivityInfos,
		TimerInfos:          input.TimerInfos,
		ChildExecutionInfos: serializedChildExecutionInfos,
		RequestCancelInfos:  input.RequestCancelInfos,
		SignalInfos:         input.SignalInfos,
		SignalRequestedIDs:  input.SignalRequestedIDs,
		TasksByCategory:     input.TasksByCategory,
		WorkflowRequests:    input.WorkflowRequests,
		Condition:           input.Condition,
		Checksum:            input.Checksum,
		ChecksumData:        checksumData,
	}, nil
}

// Helper function to drain a channel for proper cleanup
func drainChannel[T any](ch <-chan T) {
	for range ch { /* drain channel */ }
}

// SerializeVersionHistories serializes version histories
func (m *executionManagerImpl) SerializeVersionHistories(
	versionHistories *VersionHistories,
	encoding constants.EncodingType,
) (*DataBlob, error) {
	if versionHistories == nil {
		return nil, nil
	}
	return m.serializer.SerializeVersionHistories(versionHistories.ToInternalType(), encoding)
}

// DeserializeVersionHistories deserializes version histories
func (m *executionManagerImpl) DeserializeVersionHistories(
	blob *DataBlob,
) (*VersionHistories, error) {
	if blob == nil {
		return nil, nil
	}
	versionHistories, err := m.serializer.DeserializeVersionHistories(blob)
	if err != nil {
		return nil, err
	}
	return NewVersionHistoriesFromInternalType(versionHistories), nil
}

// --- Pass-through methods to underlying persistence layer ---

func (m *executionManagerImpl) DeleteWorkflowExecution(
	ctx context.Context,
	request *DeleteWorkflowExecutionRequest,
) error {
	sw := m.metricsClient.StartTimer(metrics.PersistenceDeleteWorkflowExecutionScope, metrics.PersistenceLatency)
	defer sw.Stop()
	
	// Invalidate cache before deletion
	if m.cacheEnabled {
		key := buildWorkflowExecutionCacheKey(request.DomainID, request.Execution.GetWorkflowID(), request.Execution.GetRunID())
		m.cache.Invalidate(key)
	}
	
	return m.persistence.DeleteWorkflowExecution(ctx, request)
}

func (m *executionManagerImpl) DeleteCurrentWorkflowExecution(
	ctx context.Context,
	request *DeleteCurrentWorkflowExecutionRequest,
) error {
	sw := m.metricsClient.StartTimer(metrics.PersistenceDeleteCurrentWorkflowExecutionScope, metrics.PersistenceLatency)
	defer sw.Stop()
	
	return m.persistence.DeleteCurrentWorkflowExecution(ctx, request)
}

func (m *executionManagerImpl) GetCurrentExecution(
	ctx context.Context,
	request *GetCurrentExecutionRequest,
) (*GetCurrentExecutionResponse, error) {
	sw := m.metricsClient.StartTimer(metrics.PersistenceGetCurrentExecutionScope, metrics.PersistenceLatency)
	defer sw.Stop()
	
	return m.persistence.GetCurrentExecution(ctx, request)
}

func (m *executionManagerImpl) ListCurrentExecutions(
	ctx context.Context,
	request *ListCurrentExecutionsRequest,
) (*ListCurrentExecutionsResponse, error) {
	sw := m.metricsClient.StartTimer(metrics.PersistenceListCurrentExecutionsScope, metrics.PersistenceLatency)
	defer sw.Stop()
	
	return m.persistence.ListCurrentExecutions(ctx, request)
}

func (m *executionManagerImpl) IsWorkflowExecutionExists(
	ctx context.Context,
	request *IsWorkflowExecutionExistsRequest,
) (*IsWorkflowExecutionExistsResponse, error) {
	sw := m.metricsClient.StartTimer(metrics.PersistenceIsWorkflowExecutionExistsScope, metrics.PersistenceLatency)
	defer sw.Stop()
	
	return m.persistence.IsWorkflowExecutionExists(ctx, request)
}

func (m *executionManagerImpl) ListConcreteExecutions(
	ctx context.Context,
	request *ListConcreteExecutionsRequest,
) (*ListConcreteExecutionsResponse, error) {
	sw := m.metricsClient.StartTimer(metrics.PersistenceListConcreteExecutionsScope, metrics.PersistenceLatency)
	defer sw.Stop()
	
	response, err := m.persistence.ListConcreteExecutions(ctx, request)
	if err != nil {
		return nil, err
	}
	
	// Deserialize executions in parallel
	newResponse := &ListConcreteExecutionsResponse{
		Executions: make([]*ListConcreteExecutionsEntity, len(response.Executions)),
		PageToken:  response.NextPageToken,
	}
	
	var wg sync.WaitGroup
	errs := make([]error, len(response.Executions))
	
	wg.Add(len(response.Executions))
	for i, e := range response.Executions {
		go func(i int, e *InternalListConcreteExecutionsEntity) {
			defer wg.Done()
			
			info, _, err := m.DeserializeExecutionInfo(e.ExecutionInfo)
			if err != nil {
				errs[i] = err
				return
			}
			vh, err := m.DeserializeVersionHistories(e.VersionHistories)
			if err != nil {
				errs[i] = err
				return
			}
			newResponse.Executions[i] = &ListConcreteExecutionsEntity{
				ExecutionInfo:    info,
				VersionHistories: vh,
			}
		}(i, e)
	}
	
	wg.Wait()
	
	// Check for errors
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	
	return newResponse, nil
}

func (m *executionManagerImpl) PutReplicationTaskToDLQ(
	ctx context.Context,
	request *PutReplicationTaskToDLQRequest,
) error {
	internalRequest := &InternalPutReplicationTaskToDLQRequest{
		SourceClusterName: request.SourceClusterName,
		TaskInfo:          m.toInternalReplicationTaskInfo(request.TaskInfo),
	}
	return m.persistence.PutReplicationTaskToDLQ(ctx, internalRequest)
}

func (m *executionManagerImpl) GetReplicationTasksFromDLQ(
	ctx context.Context,
	request *GetReplicationTasksFromDLQRequest,
) (*GetHistoryTasksResponse, error) {
	return m.persistence.GetReplicationTasksFromDLQ(ctx, request)
}

func (m *executionManagerImpl) GetReplicationDLQSize(
	ctx context.Context,
	request *GetReplicationDLQSizeRequest,
) (*GetReplicationDLQSizeResponse, error) {
	return m.persistence.GetReplicationDLQSize(ctx, request)
}

func (m *executionManagerImpl) DeleteReplicationTaskFromDLQ(
	ctx context.Context,
	request *DeleteReplicationTaskFromDLQRequest,
) error {
	return m.persistence.DeleteReplicationTaskFromDLQ(ctx, request)
}

func (m *executionManagerImpl) RangeDeleteReplicationTaskFromDLQ(
	ctx context.Context,
	request *RangeDeleteReplicationTaskFromDLQRequest,
) (*RangeDeleteReplicationTaskFromDLQResponse, error) {
	return m.persistence.RangeDeleteReplicationTaskFromDLQ(ctx, request)
}

func (m *executionManagerImpl) CreateFailoverMarkerTasks(
	ctx context.Context,
	request *CreateFailoverMarkersRequest,
) error {
	request.CurrentTimeStamp = m.timeSrc.Now()
	return m.persistence.CreateFailoverMarkerTasks(ctx, request)
}

func (m *executionManagerImpl) GetActiveClusterSelectionPolicy(
	ctx context.Context,
	domainID, wfID, rID string,
) (*types.ActiveClusterSelectionPolicy, error) {
	blob, err := m.persistence.GetActiveClusterSelectionPolicy(ctx, domainID, wfID, rID)
	if err != nil {
		return nil, err
	}
	if blob == nil {
		return nil, &types.EntityNotExistsError{
			Message: "active cluster selection policy not found",
		}
	}
	policy, err := m.serializer.DeserializeActiveClusterSelectionPolicy(blob)
	if err != nil {
		return nil, err
	}
	return policy, nil
}

func (m *executionManagerImpl) Close() {
	if m.cacheEnabled {
		// Log cache stats before closing
		hits, misses, total := m.cache.GetStats()
		if total > 0 {
			m.logger.Info("Workflow execution cache stats",
				tag.Metric("hits", hits),
				tag.Metric("misses", misses),
				tag.Metric("total", total),
				tag.Metric("hitRate", float64(hits)/float64(total)*100),
			)
		}
	}
	m.persistence.Close()
}

func (m *executionManagerImpl) fromInternalReplicationTaskInfos(internalInfos []*InternalReplicationTaskInfo) []*ReplicationTaskInfo {
	if internalInfos == nil {
		return nil
	}
	infos := make([]*ReplicationTaskInfo, len(internalInfos))
	for i := 0; i < len(internalInfos); i++ {
		infos[i] = m.fromInternalReplicationTaskInfo(internalInfos[i])
	}
	return infos
}

func (m *executionManagerImpl) fromInternalReplicationTaskInfo(internalInfo *InternalReplicationTaskInfo) *ReplicationTaskInfo {
	if internalInfo == nil {
		return nil
	}
	return &ReplicationTaskInfo{
		DomainID:          internalInfo.DomainID,
		WorkflowID:        internalInfo.WorkflowID,
		RunID:             internalInfo.RunID,
		TaskID:            internalInfo.TaskID,
		TaskType:          internalInfo.TaskType,
		FirstEventID:      internalInfo.FirstEventID,
		NextEventID:       internalInfo.NextEventID,
		Version:           internalInfo.Version,
		ScheduledID:       internalInfo.ScheduledID,
		BranchToken:       internalInfo.BranchToken,
		NewRunBranchToken: internalInfo.NewRunBranchToken,
		CreationTime:      internalInfo.CreationTime.UnixNano(),
	}
}

func (m *executionManagerImpl) toInternalReplicationTaskInfo(info *ReplicationTaskInfo) *InternalReplicationTaskInfo {
	if info == nil {
		return nil
	}
	return &InternalReplicationTaskInfo{
		DomainID:          info.DomainID,
		WorkflowID:        info.WorkflowID,
		RunID:             info.RunID,
		TaskID:            info.TaskID,
		TaskType:          info.TaskType,
		FirstEventID:      info.FirstEventID,
		NextEventID:       info.NextEventID,
		Version:           info.Version,
		ScheduledID:       info.ScheduledID,
		BranchToken:       info.BranchToken,
		NewRunBranchToken: info.NewRunBranchToken,
		CreationTime:      time.Unix(0, info.CreationTime).UTC(),
		CurrentTimeStamp:  m.timeSrc.Now(),
	}
}

func (m *executionManagerImpl) GetHistoryTasks(
	ctx context.Context,
	request *GetHistoryTasksRequest,
) (*GetHistoryTasksResponse, error) {
	sw := m.metricsClient.StartTimer(metrics.PersistenceGetHistoryTasksScope, metrics.PersistenceLatency)
	defer sw.Stop()
	
	return m.persistence.GetHistoryTasks(ctx, request)
}

func (m *executionManagerImpl) CompleteHistoryTask(
	ctx context.Context,
	request *CompleteHistoryTaskRequest,
) error {
	sw := m.metricsClient.StartTimer(metrics.PersistenceCompleteHistoryTaskScope, metrics.PersistenceLatency)
	defer sw.Stop()
	
	return m.persistence.CompleteHistoryTask(ctx, request)
}

func (m *executionManagerImpl) RangeCompleteHistoryTask(
	ctx context.Context,
	request *RangeCompleteHistoryTaskRequest,
) (*RangeCompleteHistoryTaskResponse, error) {
	sw := m.metricsClient.StartTimer(metrics.PersistenceRangeCompleteHistoryTaskScope, metrics.PersistenceLatency)
	defer sw.Stop()
	
	return m.persistence.RangeCompleteHistoryTask(ctx, request)
}

// Helper functions for version histories

func getStartVersion(
	versionHistories *VersionHistories,
) (int64, error) {
	if versionHistories == nil {
		return constants.EmptyVersion, nil
	}

	versionHistory, err := versionHistories.GetCurrentVersionHistory()
	if err != nil {
		return 0, err
	}
	versionHistoryItem, err := versionHistory.GetFirstItem()
	if err != nil {
		return 0, err
	}
	return versionHistoryItem.Version, nil
}

func getLastWriteVersion(
	versionHistories *VersionHistories,
) (int64, error) {
	if versionHistories == nil {
		return constants.EmptyVersion, nil
	}

	versionHistory, err := versionHistories.GetCurrentVersionHistory()
	if err != nil {
		return 0, err
	}
	versionHistoryItem, err := versionHistory.GetLastItem()
	if err != nil {
		return 0, err
	}
	return versionHistoryItem.Version, nil
}