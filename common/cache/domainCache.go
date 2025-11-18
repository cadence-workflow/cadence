// Copyright (c) 2017 Uber Technologies, Inc.
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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination domainCache_mock.go -self_package github.com/uber/cadence/common/cache

package cache

import (
	"context"
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

// ReplicationPolicy is the domain's replication policy,
// derived from domain's replication config
type ReplicationPolicy int

const (
	// ReplicationPolicyOneCluster indicate that workflows does not need to be replicated
	// applicable to local domain & global domain with one cluster
	ReplicationPolicyOneCluster ReplicationPolicy = 0
	// ReplicationPolicyMultiCluster indicate that workflows need to be replicated
	ReplicationPolicyMultiCluster ReplicationPolicy = 1
)

const (
	domainCacheInitialSize        = 10 * 1024
	domainCacheMinRefreshInterval = 1 * time.Second
	// DomainCacheRefreshInterval domain cache refresh interval
	DomainCacheRefreshInterval = 10 * time.Second
	domainCacheRefreshPageSize = 200

	domainCachePersistenceTimeout = 3 * time.Second

	domainCacheInitialized int32 = 0
	domainCacheStarted     int32 = 1
	domainCacheStopped     int32 = 2
)

type (
	// PrepareCallbackFn is function to be called before CallbackFn is called,
	// it is guaranteed that PrepareCallbackFn and CallbackFn pair will be both called or non will be called
	PrepareCallbackFn func()

	// CallbackFn is function to be called when the domain cache entries are changed
	// it is guaranteed that  CallbackFn pair will be both called or non will be called
	CallbackFn func(updatedDomains []*DomainCacheEntry)

	// CatchUpFn is a function to execute PrepareCallbackFn and/or CallbackFn during domain callback registration,
	// allowing the caller to catch up with the latest domain changes
	CatchUpFn func(domainCache DomainCache, prepareCallback PrepareCallbackFn, callback CallbackFn)

	// DomainCache is used the cache domain information and configuration to avoid making too many calls to cassandra.
	// This cache is mainly used by frontend for resolving domain names to domain uuids which are used throughout the
	// system.  Each domain entry is kept in the cache for one hour but also has an expiry of 10 seconds.  This results
	// in updating the domain entry every 10 seconds but in the case of a cassandra failure we can still keep on serving
	// requests using the stale entry from cache upto an hour
	DomainCache interface {
		common.Daemon
		RegisterDomainChangeCallback(id string, catchUpFn CatchUpFn, prepareCallback PrepareCallbackFn, callback CallbackFn)
		UnregisterDomainChangeCallback(id string)
		GetDomain(name string) (*DomainCacheEntry, error)
		GetDomainByID(id string) (*DomainCacheEntry, error)
		GetDomainID(name string) (string, error)
		GetDomainName(id string) (string, error)
		GetAllDomain() map[string]*DomainCacheEntry
		GetCacheSize() (sizeOfCacheByName int64, sizeOfCacheByID int64)
	}

	DefaultDomainCache struct {
		status          int32
		shutdownChan    chan struct{}
		clusterGroup    string
		clusterMetadata cluster.Metadata
		cacheNameToID   *atomic.Value
		cacheByID       *atomic.Value
		domainManager   persistence.DomainManager
		timeSource      clock.TimeSource
		scope           metrics.Scope
		logger          log.Logger

		// refresh lock is used to guarantee at most one
		// coroutine is doing domain refreshment
		refreshLock     sync.Mutex
		lastRefreshTime time.Time
		// This is debug field to emit callback count
		lastCallbackEmitTime time.Time

		callbackLock     sync.Mutex
		prepareCallbacks map[string]PrepareCallbackFn
		callbacks        map[string]CallbackFn

		throttleRetry *backoff.ThrottleRetry

		// ctx and cancel are used to control the lifecycle of background operations
		ctx    context.Context
		cancel context.CancelFunc
	}

	// DomainCacheEntries is DomainCacheEntry slice
	DomainCacheEntries []*DomainCacheEntry

	// DomainCacheEntry contains the info and config for a domain
	DomainCacheEntry struct {
		mu                          sync.RWMutex
		info                        *persistence.DomainInfo
		config                      *persistence.DomainConfig
		replicationConfig           *persistence.DomainReplicationConfig
		configVersion               int64
		failoverVersion             int64
		isGlobalDomain              bool
		failoverNotificationVersion int64
		previousFailoverVersion     int64
		failoverEndTime             *int64
		notificationVersion         int64
		initialized                 bool

		// list of active clusters for active-active domains. initialized from replication config.
		activeClusters []string
	}
)

type DomainCacheOption func(*DefaultDomainCache)

func WithTimeSource(timeSource clock.TimeSource) DomainCacheOption {
	return func(cache *DefaultDomainCache) {
		if timeSource != nil {
			cache.timeSource = timeSource
		}
	}
}

// NewDomainCache creates a new instance of cache for holding onto domain information to reduce the load on persistence
func NewDomainCache(
	domainManager persistence.DomainManager,
	metadata cluster.Metadata,
	metricsClient metrics.Client,
	logger log.Logger,
	opts ...DomainCacheOption,
) *DefaultDomainCache {

	throttleRetry := backoff.NewThrottleRetry(
		backoff.WithRetryPolicy(common.CreateDomainCacheRetryPolicy()),
		backoff.WithRetryableError(common.IsServiceTransientError),
	)

	ctx, cancel := context.WithCancel(context.Background())
	cache := &DefaultDomainCache{
		status:           domainCacheInitialized,
		shutdownChan:     make(chan struct{}),
		clusterGroup:     getClusterGroupIdentifier(metadata),
		clusterMetadata:  metadata,
		cacheNameToID:    &atomic.Value{},
		cacheByID:        &atomic.Value{},
		domainManager:    domainManager,
		timeSource:       clock.NewRealTimeSource(),
		scope:            metricsClient.Scope(metrics.DomainCacheScope),
		logger:           logger,
		prepareCallbacks: make(map[string]PrepareCallbackFn),
		callbacks:        make(map[string]CallbackFn),
		throttleRetry:    throttleRetry,
		ctx:              ctx,
		cancel:           cancel,
	}
	cache.cacheNameToID.Store(newDomainCache())
	cache.cacheByID.Store(newDomainCache())

	for _, opt := range opts {
		opt(cache)
	}

	return cache
}

func getClusterGroupIdentifier(metadata cluster.Metadata) string {
	var clusters []string
	for cluster := range metadata.GetEnabledClusterInfo() {
		clusters = append(clusters, cluster)
	}
	sort.Strings(clusters)
	return strings.Join(clusters, "_")
}

func newDomainCache() Cache {
	return NewSimple(&SimpleOptions{
		InitialCapacity: domainCacheInitialSize,
	})
}

// NewGlobalDomainCacheEntryForTest returns an entry with test data
func NewGlobalDomainCacheEntryForTest(
	info *persistence.DomainInfo,
	config *persistence.DomainConfig,
	repConfig *persistence.DomainReplicationConfig,
	failoverVersion int64,
) *DomainCacheEntry {

	return &DomainCacheEntry{
		info:              info,
		config:            config,
		isGlobalDomain:    true,
		replicationConfig: repConfig,
		failoverVersion:   failoverVersion,
	}
}

// NewLocalDomainCacheEntryForTest returns an entry with test data
func NewLocalDomainCacheEntryForTest(
	info *persistence.DomainInfo,
	config *persistence.DomainConfig,
	targetCluster string,
) *DomainCacheEntry {

	return &DomainCacheEntry{
		info:           info,
		config:         config,
		isGlobalDomain: false,
		replicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: targetCluster,
			Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: targetCluster}},
		},
		failoverVersion: constants.EmptyVersion,
	}
}

// NewDomainCacheEntryForTest returns an entry with test data
func NewDomainCacheEntryForTest(
	info *persistence.DomainInfo,
	config *persistence.DomainConfig,
	isGlobalDomain bool,
	repConfig *persistence.DomainReplicationConfig,
	failoverVersion int64,
	failoverEndtime *int64,
	failoverNotificationVersion int64,
	previousFailoverVersion int64,
	notificationVersion int64,
) *DomainCacheEntry {
	return &DomainCacheEntry{
		info:                        info,
		config:                      config,
		isGlobalDomain:              isGlobalDomain,
		replicationConfig:           repConfig,
		failoverVersion:             failoverVersion,
		failoverEndTime:             failoverEndtime,
		failoverNotificationVersion: failoverNotificationVersion,
		previousFailoverVersion:     previousFailoverVersion,
		notificationVersion:         notificationVersion,
		activeClusters:              getActiveClusters(repConfig),
	}
}

func (c *DefaultDomainCache) GetCacheSize() (sizeOfCacheByName int64, sizeOfCacheByID int64) {
	return int64(c.cacheByID.Load().(Cache).Size()), int64(c.cacheNameToID.Load().(Cache).Size())
}

// Start starts the background refresh of domain
func (c *DefaultDomainCache) Start() {
	if !atomic.CompareAndSwapInt32(&c.status, domainCacheInitialized, domainCacheStarted) {
		return
	}

	// initialize the cache by initial scan
	err := c.refreshDomains()
	if err != nil {
		c.logger.Fatal("Unable to initialize domain cache", tag.Error(err))
	}
	go c.refreshLoop()
}

// Stop stops background refresh of domain
func (c *DefaultDomainCache) Stop() {
	if !atomic.CompareAndSwapInt32(&c.status, domainCacheStarted, domainCacheStopped) {
		return
	}
	c.cancel() // Cancel the context first
	close(c.shutdownChan)
}

func (c *DefaultDomainCache) GetAllDomain() map[string]*DomainCacheEntry {
	result := make(map[string]*DomainCacheEntry)
	ite := c.cacheByID.Load().(Cache).Iterator()
	defer ite.Close()

	for ite.HasNext() {
		entry := ite.Next()
		id := entry.Key().(string)
		domainCacheEntry := entry.Value().(*DomainCacheEntry)
		domainCacheEntry.mu.RLock()
		dup := domainCacheEntry.duplicate()
		domainCacheEntry.mu.RUnlock()
		result[id] = dup
	}
	return result
}

// RegisterDomainChangeCallback set a domain change callback
// WARN: the beforeCallback function will be triggered by domain cache when holding the domain cache lock,
// make sure the callback function will not call domain cache again in case of dead lock
// afterCallback will be invoked when NOT holding the domain cache lock.
func (c *DefaultDomainCache) RegisterDomainChangeCallback(
	id string,
	catchUpFn CatchUpFn,
	prepareCallback PrepareCallbackFn,
	callback CallbackFn,
) {

	c.callbackLock.Lock()
	c.prepareCallbacks[id] = prepareCallback
	c.callbacks[id] = callback
	c.callbackLock.Unlock()

	catchUpFn(c, prepareCallback, callback)

}

// UnregisterDomainChangeCallback delete a domain failover callback
func (c *DefaultDomainCache) UnregisterDomainChangeCallback(
	id string,
) {

	c.callbackLock.Lock()
	defer c.callbackLock.Unlock()

	delete(c.prepareCallbacks, id)
	delete(c.callbacks, id)
}

// GetDomain retrieves the information from the cache if it exists, otherwise retrieves the information from metadata
// store and writes it to the cache with an expiry before returning back
func (c *DefaultDomainCache) GetDomain(
	name string,
) (*DomainCacheEntry, error) {

	if name == "" {
		return nil, &types.BadRequestError{Message: "Domain name is empty"}
	}

	return c.getDomain(name)
}

// GetDomainByID retrieves the information from the cache if it exists, otherwise retrieves the information from metadata
// store and writes it to the cache with an expiry before returning back
func (c *DefaultDomainCache) GetDomainByID(
	id string,
) (*DomainCacheEntry, error) {

	if id == "" {
		return nil, &types.BadRequestError{Message: "DomainID is empty."}
	}
	return c.getDomainByID(id, true)
}

// GetDomainID retrieves domainID by using GetDomain
func (c *DefaultDomainCache) GetDomainID(
	name string,
) (string, error) {

	entry, err := c.GetDomain(name)
	if err != nil {
		return "", err
	}
	return entry.info.ID, nil
}

// GetDomainName returns domain name given the domain id
func (c *DefaultDomainCache) GetDomainName(
	id string,
) (string, error) {

	entry, err := c.getDomainByID(id, false)
	if err != nil {
		return "", err
	}
	return entry.info.Name, nil
}

func (c *DefaultDomainCache) refreshLoop() {
	timer := c.timeSource.NewTicker(DomainCacheRefreshInterval)
	defer timer.Stop()

	for {
		select {
		case <-c.shutdownChan:
			return
		case <-timer.Chan():
			err := c.refreshDomains()
			if err != nil {
				c.logger.Error("Error refreshing domain cache", tag.Error(err))
				continue
			}

			c.logger.Debug("Domain cache refreshed")
		}
	}
}

func (c *DefaultDomainCache) refreshDomains() error {
	c.refreshLock.Lock()
	defer c.refreshLock.Unlock()
	return c.throttleRetry.Do(c.ctx, c.refreshDomainsLocked)
}

// this function only refresh the domains in the v2 table
// the domains in the v1 table will be refreshed if cache is stale
func (c *DefaultDomainCache) refreshDomainsLocked(ctx context.Context) error {
	now := c.timeSource.Now()
	if now.Sub(c.lastRefreshTime) < domainCacheMinRefreshInterval {
		return nil
	}

	// first load the metadata record, then load domains
	// this can guarantee that domains in the cache are not updated more than metadata record
	ctx, cancel := context.WithTimeout(ctx, domainCachePersistenceTimeout)
	defer cancel()
	metadata, err := c.domainManager.GetMetadata(ctx)
	if err != nil {
		return err
	}

	var token []byte
	request := &persistence.ListDomainsRequest{PageSize: domainCacheRefreshPageSize}
	var domains DomainCacheEntries
	continuePage := true

	for continuePage {
		ctx, cancel := context.WithTimeout(ctx, domainCachePersistenceTimeout)
		request.NextPageToken = token
		response, err := c.domainManager.ListDomains(ctx, request)
		cancel()
		if err != nil {
			return err
		}
		token = response.NextPageToken
		for _, domain := range response.Domains {
			domains = append(domains, c.buildEntryFromRecord(domain))
		}
		continuePage = len(token) != 0
	}

	// we mush apply the domain change by order
	// since history shard have to update the shard info
	// with domain change version.
	sort.Sort(domains)
	var updatedEntries []*DomainCacheEntry

	// make a copy of the existing domain cache, so we can calculate diff and do compare and swap
	newCacheNameToID := newDomainCache()
	newCacheByID := newDomainCache()
	for _, domain := range c.GetAllDomain() {
		newCacheNameToID.Put(domain.info.Name, domain.info.ID)
		newCacheByID.Put(domain.info.ID, domain)
	}

UpdateLoop:
	for _, domain := range domains {
		if domain.notificationVersion >= metadata.NotificationVersion {
			// this guarantee that domain change events before the
			// domainNotificationVersion is loaded into the cache.

			// the domain change events after the domainNotificationVersion
			// will be loaded into cache in the next refresh
			c.logger.Info("Domain notification is not less than than metadata notification version", tag.WorkflowDomainName(domain.GetInfo().Name))
			break UpdateLoop
		}
		triggerCallback, nextEntry, err := c.updateIDToDomainCache(newCacheByID, domain.info.ID, domain)
		if err != nil {
			return err
		}

		c.scope.Tagged(
			metrics.DomainTag(nextEntry.info.Name),
			metrics.DomainTypeTag(nextEntry.isGlobalDomain),
			metrics.ClusterGroupTag(c.clusterGroup),
			metrics.ActiveClusterTag(nextEntry.replicationConfig.ActiveClusterName),
			metrics.IsActiveActiveDomainTag(nextEntry.replicationConfig.IsActiveActive()),
		).UpdateGauge(metrics.ActiveClusterGauge, 1)

		c.updateNameToIDCache(newCacheNameToID, nextEntry.info.Name, nextEntry.info.ID)

		if triggerCallback {
			updatedEntries = append(updatedEntries, nextEntry)
		}
	}

	// NOTE: READ REF BEFORE MODIFICATION
	// ref: historyEngine.go registerDomainFailoverCallback function
	c.callbackLock.Lock()
	defer c.callbackLock.Unlock()
	c.triggerDomainChangePrepareCallbackLocked()
	c.cacheByID.Store(newCacheByID)
	c.cacheNameToID.Store(newCacheNameToID)
	c.triggerDomainChangeCallbackLocked(updatedEntries)

	// only update last refresh time when refresh succeeded
	c.lastRefreshTime = now
	if now.Sub(c.lastCallbackEmitTime) > 30*time.Minute {
		c.lastCallbackEmitTime = now
		c.scope.AddCounter(metrics.DomainCacheCallbacksCount, int64(len(c.callbacks)))
		c.scope.RecordHistogramDuration(metrics.DomainCacheUpdateLatency, c.timeSource.Now().Sub(now))
	}

	return nil
}

func (c *DefaultDomainCache) checkDomainExists(
	name string,
	id string,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), domainCachePersistenceTimeout)
	defer cancel()

	_, err := c.domainManager.GetDomain(ctx, &persistence.GetDomainRequest{Name: name, ID: id})
	return err
}

func (c *DefaultDomainCache) updateNameToIDCache(
	cacheNameToID Cache,
	name string,
	id string,
) {

	cacheNameToID.Put(name, id)
}

func (c *DefaultDomainCache) updateIDToDomainCache(
	cacheByID Cache,
	id string,
	record *DomainCacheEntry,
) (bool, *DomainCacheEntry, error) {
	elem, err := cacheByID.PutIfNotExist(id, &DomainCacheEntry{})
	if err != nil {
		return false, nil, err
	}
	entry := elem.(*DomainCacheEntry)

	entry.mu.Lock()
	defer entry.mu.Unlock()

	// initialized will be true when the entry contains valid data
	triggerCallback := entry.initialized && record.notificationVersion > entry.notificationVersion

	entry.info = record.info
	entry.config = record.config
	entry.replicationConfig = record.replicationConfig
	entry.configVersion = record.configVersion
	entry.failoverVersion = record.failoverVersion
	entry.isGlobalDomain = record.isGlobalDomain
	entry.failoverNotificationVersion = record.failoverNotificationVersion
	entry.previousFailoverVersion = record.previousFailoverVersion
	entry.failoverEndTime = record.failoverEndTime
	entry.notificationVersion = record.notificationVersion
	entry.initialized = record.initialized
	entry.activeClusters = record.activeClusters
	return triggerCallback, entry.duplicate(), nil
}

// getDomain retrieves the information from the cache if it exists, otherwise retrieves the information from metadata
// store and writes it to the cache with an expiry before returning back
func (c *DefaultDomainCache) getDomain(
	name string,
) (*DomainCacheEntry, error) {

	id, cacheHit := c.cacheNameToID.Load().(Cache).Get(name).(string)
	if cacheHit {
		return c.getDomainByID(id, true)
	}

	if err := c.checkDomainExists(name, ""); err != nil {
		return nil, err
	}

	c.refreshLock.Lock()
	defer c.refreshLock.Unlock()
	id, cacheHit = c.cacheNameToID.Load().(Cache).Get(name).(string)
	if cacheHit {
		return c.getDomainByID(id, true)
	}
	if err := c.refreshDomainsLocked(context.Background()); err != nil {
		return nil, err
	}
	id, cacheHit = c.cacheNameToID.Load().(Cache).Get(name).(string)
	if cacheHit {
		return c.getDomainByID(id, true)
	}
	// impossible case
	return nil, &types.InternalServiceError{Message: "DefaultDomainCache encounter case where domain exists but cannot be loaded"}
}

// getDomainByID retrieves the information from the cache if it exists, otherwise retrieves the information from metadata
// store and writes it to the cache with an expiry before returning back
func (c *DefaultDomainCache) getDomainByID(
	id string,
	deepCopy bool,
) (*DomainCacheEntry, error) {

	var result *DomainCacheEntry
	defer func() {
		if result != nil {
			c.logger.Debugf("GetDomainByID returning domain %s, failoverVersion: %d", result.info.Name, result.failoverVersion)
		}
	}()
	entry, cacheHit := c.cacheByID.Load().(Cache).Get(id).(*DomainCacheEntry)
	if cacheHit {
		entry.mu.RLock()
		result = entry
		if deepCopy {
			result = entry.duplicate()
		}
		entry.mu.RUnlock()
		return result, nil
	}

	if err := c.checkDomainExists("", id); err != nil {
		return nil, err
	}

	c.refreshLock.Lock()
	defer c.refreshLock.Unlock()
	entry, cacheHit = c.cacheByID.Load().(Cache).Get(id).(*DomainCacheEntry)
	if cacheHit {
		entry.mu.RLock()
		result = entry
		if deepCopy {
			result = entry.duplicate()
		}
		entry.mu.RUnlock()
		return result, nil
	}
	if err := c.refreshDomainsLocked(context.Background()); err != nil {
		return nil, err
	}
	entry, cacheHit = c.cacheByID.Load().(Cache).Get(id).(*DomainCacheEntry)
	if cacheHit {
		entry.mu.RLock()
		result = entry
		if deepCopy {
			result = entry.duplicate()
		}
		entry.mu.RUnlock()
		return result, nil
	}
	// impossible case
	return nil, &types.InternalServiceError{Message: "DefaultDomainCache encounter case where domain exists but cannot be loaded"}
}

func (c *DefaultDomainCache) triggerDomainChangePrepareCallbackLocked() {
	sw := c.scope.StartTimer(metrics.DomainCachePrepareCallbacksLatency)
	defer sw.Stop()

	for _, prepareCallback := range c.prepareCallbacks {
		prepareCallback()
	}
}

func (c *DefaultDomainCache) triggerDomainChangeCallbackLocked(nextDomains []*DomainCacheEntry) {
	sw := c.scope.StartTimer(metrics.DomainCacheCallbacksLatency)
	defer sw.Stop()

	c.logger.Debug("Domain change callbacks are going to triggered", tag.Number(int64(len(nextDomains))))
	for _, callback := range c.callbacks {
		callback(nextDomains)
	}
	c.logger.Debug("Domain change callbacks are completed", tag.Number(int64(len(nextDomains))))
}

func (c *DefaultDomainCache) buildEntryFromRecord(
	record *persistence.GetDomainResponse,
) *DomainCacheEntry {

	// this is a shallow copy, but since the record is generated by persistence
	// and only accessible here, it would be fine
	return &DomainCacheEntry{
		info:                        record.Info,
		config:                      record.Config,
		replicationConfig:           record.ReplicationConfig,
		configVersion:               record.ConfigVersion,
		failoverVersion:             record.FailoverVersion,
		isGlobalDomain:              record.IsGlobalDomain,
		failoverNotificationVersion: record.FailoverNotificationVersion,
		previousFailoverVersion:     record.PreviousFailoverVersion,
		failoverEndTime:             record.FailoverEndTime,
		notificationVersion:         record.NotificationVersion,
		initialized:                 true,
		activeClusters:              getActiveClusters(record.ReplicationConfig),
	}
}

func copyResetBinary(bins types.BadBinaries) types.BadBinaries {
	newbins := make(map[string]*types.BadBinaryInfo, len(bins.Binaries))
	for k, v := range bins.Binaries {
		newbins[k] = v
	}
	return types.BadBinaries{
		Binaries: newbins,
	}
}

func (entry *DomainCacheEntry) duplicate() *DomainCacheEntry {
	// this is a deep copy
	result := &DomainCacheEntry{}
	result.info = &persistence.DomainInfo{
		ID:          entry.info.ID,
		Name:        entry.info.Name,
		Status:      entry.info.Status,
		Description: entry.info.Description,
		OwnerEmail:  entry.info.OwnerEmail,
	}
	result.info.Data = map[string]string{}
	for k, v := range entry.info.Data {
		result.info.Data[k] = v
	}
	result.config = &persistence.DomainConfig{
		Retention:                entry.config.Retention,
		EmitMetric:               entry.config.EmitMetric,
		HistoryArchivalStatus:    entry.config.HistoryArchivalStatus,
		HistoryArchivalURI:       entry.config.HistoryArchivalURI,
		VisibilityArchivalStatus: entry.config.VisibilityArchivalStatus,
		VisibilityArchivalURI:    entry.config.VisibilityArchivalURI,
		BadBinaries:              copyResetBinary(entry.config.BadBinaries),
		AsyncWorkflowConfig:      entry.config.AsyncWorkflowConfig.DeepCopy(),
		IsolationGroups:          entry.config.IsolationGroups.DeepCopy(),
	}
	result.replicationConfig = &persistence.DomainReplicationConfig{
		ActiveClusterName: entry.replicationConfig.ActiveClusterName,
	}
	for _, clusterCfg := range entry.replicationConfig.Clusters {
		c := *clusterCfg
		result.replicationConfig.Clusters = append(result.replicationConfig.Clusters, &c)
	}
	result.replicationConfig.ActiveClusters = entry.replicationConfig.ActiveClusters.DeepCopy()
	result.configVersion = entry.configVersion
	result.failoverVersion = entry.failoverVersion
	result.isGlobalDomain = entry.isGlobalDomain
	result.failoverNotificationVersion = entry.failoverNotificationVersion
	result.previousFailoverVersion = entry.previousFailoverVersion
	result.failoverEndTime = entry.failoverEndTime
	result.notificationVersion = entry.notificationVersion
	result.initialized = entry.initialized
	result.activeClusters = entry.activeClusters
	return result
}

// GetInfo return the domain info
func (entry *DomainCacheEntry) GetInfo() *persistence.DomainInfo {
	return entry.info
}

// GetConfig return the domain config
func (entry *DomainCacheEntry) GetConfig() *persistence.DomainConfig {
	return entry.config
}

// GetReplicationConfig return the domain replication config
func (entry *DomainCacheEntry) GetReplicationConfig() *persistence.DomainReplicationConfig {
	return entry.replicationConfig
}

// GetConfigVersion return the domain config version
func (entry *DomainCacheEntry) GetConfigVersion() int64 {
	return entry.configVersion
}

// GetFailoverVersion return the domain failover version
func (entry *DomainCacheEntry) GetFailoverVersion() int64 {
	return entry.failoverVersion
}

// IsGlobalDomain return whether the domain is a global domain
func (entry *DomainCacheEntry) IsGlobalDomain() bool {
	return entry.isGlobalDomain
}

// GetFailoverNotificationVersion return the global notification version of when failover happened
func (entry *DomainCacheEntry) GetFailoverNotificationVersion() int64 {
	return entry.failoverNotificationVersion
}

// GetNotificationVersion return the global notification version of when domain changed
func (entry *DomainCacheEntry) GetNotificationVersion() int64 {
	return entry.notificationVersion
}

// GetPreviousFailoverVersion return the last domain failover version
func (entry *DomainCacheEntry) GetPreviousFailoverVersion() int64 {
	return entry.previousFailoverVersion
}

// GetFailoverEndTime return the failover end time
func (entry *DomainCacheEntry) GetFailoverEndTime() *int64 {
	return entry.failoverEndTime
}

// GetActiveClusterInfoByClusterAttribute return the active cluster info for a given cluster attribute
// if clusterAttribute is nil, return the domain-level active cluster info and true
// if the clusterAttribute exists, return the active cluster info of the clusterAttribute and true
// if the clusterAttribute is not found, return false
func (entry *DomainCacheEntry) GetActiveClusterInfoByClusterAttribute(clusterAttribute *types.ClusterAttribute) (*types.ActiveClusterInfo, bool) {
	if clusterAttribute == nil {
		return &types.ActiveClusterInfo{
			ActiveClusterName: entry.GetReplicationConfig().ActiveClusterName,
			FailoverVersion:   entry.GetFailoverVersion(),
		}, true
	}
	if entry.replicationConfig.ActiveClusters == nil {
		return nil, false
	}
	if entry.replicationConfig.ActiveClusters.AttributeScopes == nil {
		return nil, false
	}
	scope, ok := entry.replicationConfig.ActiveClusters.AttributeScopes[clusterAttribute.Scope]
	if !ok {
		return nil, false
	}
	info, ok := scope.ClusterAttributes[clusterAttribute.Name]
	if !ok {
		return nil, false
	}
	return &types.ActiveClusterInfo{
		ActiveClusterName: info.ActiveClusterName,
		FailoverVersion:   info.FailoverVersion,
	}, true
}

// NewDomainNotActiveError return a domain not active error
// currentCluster is the current cluster
// activeCluster is the active cluster which is either domain's active cluster or it's inferred from workflow task version
func (entry *DomainCacheEntry) NewDomainNotActiveError(currentCluster, activeCluster string) *types.DomainNotActiveError {
	if entry.GetReplicationConfig().IsActiveActive() {
		return &types.DomainNotActiveError{
			Message: fmt.Sprintf(
				"Domain: %s is active in cluster(s): %v, while current cluster %s is a standby cluster. Operation active cluster: %s",
				entry.GetInfo().Name,
				entry.activeClusters,
				currentCluster,
				activeCluster,
			),
			DomainName:     entry.GetInfo().Name,
			CurrentCluster: currentCluster,
			ActiveCluster:  activeCluster,
			ActiveClusters: entry.activeClusters,
		}
	}

	return &types.DomainNotActiveError{
		Message: fmt.Sprintf(
			"Domain: %s is active in cluster: %s, while current cluster %s is a standby cluster.",
			entry.GetInfo().Name,
			activeCluster,
			currentCluster,
		),
		DomainName:     entry.GetInfo().Name,
		CurrentCluster: currentCluster,
		ActiveCluster:  activeCluster,
	}
}

// IsActive return whether the domain is active in the current cluster,
// - for local domain, it is always active
// - for global domain, it is active if it is not pending active and the domain's active cluster is the current cluster or if the domain is active-active and the active cluster of one of the cluster attributes is the current cluster
// TODO(active-active): for active-active domains, we should review this logic because now workflows can be active in different clusters based on the cluster attribute.
// We should also revisit its usage in history service.
func (entry *DomainCacheEntry) IsActiveIn(currentCluster string) bool {
	if !entry.IsGlobalDomain() {
		// domain is not a global domain, meaning domain is always "active" within each cluster
		return true
	}

	if entry.IsDomainPendingActive() {
		return false
	}

	activeCluster := entry.GetReplicationConfig().ActiveClusterName
	if currentCluster == activeCluster {
		return true
	}

	if activeClusters := entry.GetReplicationConfig().ActiveClusters; activeClusters != nil {
		for _, scope := range activeClusters.AttributeScopes {
			for _, cl := range scope.ClusterAttributes {
				if cl.ActiveClusterName == currentCluster {
					return true
				}
			}
		}
	}

	return false
}

// IsDomainPendingActive returns whether the domain is in pending active state
func (entry *DomainCacheEntry) IsDomainPendingActive() bool {
	if !entry.isGlobalDomain {
		// domain is not a global domain, meaning domain can never be in pending active state
		return false
	}
	return entry.failoverEndTime != nil
}

// GetReplicationPolicy return the derived workflow replication policy
func (entry *DomainCacheEntry) GetReplicationPolicy() ReplicationPolicy {
	// frontend guarantee that the clusters always contains the active domain, so if the # of clusters is 1
	// then we do not need to send out any events for replication
	if entry.isGlobalDomain && len(entry.replicationConfig.Clusters) > 1 {
		return ReplicationPolicyMultiCluster
	}
	return ReplicationPolicyOneCluster
}

// HasReplicationCluster returns true if the domain has replication in the cluster
func (entry *DomainCacheEntry) HasReplicationCluster(clusterName string) bool {
	for _, cluster := range entry.GetReplicationConfig().Clusters {
		if cluster.ClusterName == clusterName {
			return true
		}
	}
	return false
}

// Len return length
func (t DomainCacheEntries) Len() int {
	return len(t)
}

// Swap implements sort.Interface.
func (t DomainCacheEntries) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

// Less implements sort.Interface
func (t DomainCacheEntries) Less(i, j int) bool {
	return t[i].notificationVersion < t[j].notificationVersion
}

// CreateDomainCacheEntry create a cache entry with domainName
func CreateDomainCacheEntry(
	domainName string,
) *DomainCacheEntry {

	return &DomainCacheEntry{info: &persistence.DomainInfo{Name: domainName}}
}

// SampleRetentionKey is key to specify sample retention
var SampleRetentionKey = "sample_retention_days"

// SampleRateKey is key to specify sample rate
var SampleRateKey = "sample_retention_rate"

// GetRetentionDays returns retention in days for given workflow
func (entry *DomainCacheEntry) GetRetentionDays(
	workflowID string,
) int32 {

	if entry.IsSampledForLongerRetention(workflowID) {
		if sampledRetentionValue, ok := entry.info.Data[SampleRetentionKey]; ok {
			sampledRetentionDays, err := strconv.Atoi(sampledRetentionValue)
			if err != nil || sampledRetentionDays < int(entry.config.Retention) {
				return entry.config.Retention
			}
			return int32(sampledRetentionDays)
		}
	}
	return entry.config.Retention
}

// IsSampledForLongerRetentionEnabled return whether sample for longer retention is enabled or not
func (entry *DomainCacheEntry) IsSampledForLongerRetentionEnabled(
	workflowID string,
) bool {

	_, ok := entry.info.Data[SampleRateKey]
	return ok
}

// IsSampledForLongerRetention return should given workflow been sampled or not
func (entry *DomainCacheEntry) IsSampledForLongerRetention(
	workflowID string,
) bool {

	if sampledRateValue, ok := entry.info.Data[SampleRateKey]; ok {
		sampledRate, err := strconv.ParseFloat(sampledRateValue, 64)
		if err != nil {
			return false
		}

		h := fnv.New32a()
		_, err = h.Write([]byte(workflowID))
		if err != nil {
			return false
		}
		hash := h.Sum32()

		r := float64(hash%1000) / float64(1000) // use 1000 so we support one decimal rate like 1.5%.
		if r < sampledRate {                    // sampled
			return true
		}
	}
	return false
}

// IsDeprecatedOrDeleted This function checks the domain status to see if the domain has been deprecated or deleted.
func (entry *DomainCacheEntry) IsDeprecatedOrDeleted() bool {
	if entry.info.Status == persistence.DomainStatusDeprecated || entry.info.Status == persistence.DomainStatusDeleted {
		return true
	}
	return false
}

func getActiveClusters(replicationConfig *persistence.DomainReplicationConfig) []string {
	if !replicationConfig.IsActiveActive() {
		return nil
	}
	// TODO(active-active): Replace with `GetAllClusters` once we remove regions
	activeClusters := make([]string, 0, len(replicationConfig.ActiveClusters.AttributeScopes))
	for _, scope := range replicationConfig.ActiveClusters.AttributeScopes {
		for _, cl := range scope.ClusterAttributes {
			activeClusters = append(activeClusters, cl.ActiveClusterName)
		}
	}
	sort.Strings(activeClusters)
	return activeClusters
}
