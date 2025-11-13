// Copyright (c) 2019 Uber Technologies, Inc.
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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination bean_mock.go

package client

import (
	"sync"

	"github.com/uber/cadence/common/config"
	es "github.com/uber/cadence/common/elasticsearch"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/pinot"
	"github.com/uber/cadence/common/service"
)

type (
	// Bean in an collection of persistence manager
	Bean interface {
		Close()

		GetDomainManager() persistence.DomainManager
		SetDomainManager(persistence.DomainManager)

		GetDomainAuditManager() persistence.DomainAuditManager
		SetDomainAuditManager(persistence.DomainAuditManager)

		GetTaskManager() persistence.TaskManager
		SetTaskManager(persistence.TaskManager)

		GetVisibilityManager() persistence.VisibilityManager
		SetVisibilityManager(persistence.VisibilityManager)

		GetDomainReplicationQueueManager() persistence.QueueManager
		SetDomainReplicationQueueManager(persistence.QueueManager)

		GetShardManager() persistence.ShardManager
		SetShardManager(persistence.ShardManager)

		GetHistoryManager() persistence.HistoryManager
		SetHistoryManager(persistence.HistoryManager)

		GetExecutionManager(int) (persistence.ExecutionManager, error)
		SetExecutionManager(int, persistence.ExecutionManager)

		GetConfigStoreManager() persistence.ConfigStoreManager
		SetConfigStoreManager(persistence.ConfigStoreManager)
	}

	// BeanImpl stores persistence managers
	BeanImpl struct {
		domainManager                 persistence.DomainManager
		domainAuditManager            persistence.DomainAuditManager
		taskManager                   persistence.TaskManager
		visibilityManager             persistence.VisibilityManager
		domainReplicationQueueManager persistence.QueueManager
		shardManager                  persistence.ShardManager
		historyManager                persistence.HistoryManager
		configStoreManager            persistence.ConfigStoreManager
		executionManagerFactory       persistence.ExecutionManagerFactory

		sync.RWMutex
		shardIDToExecutionManager map[int]persistence.ExecutionManager
	}

	// Params contains dependencies for persistence
	Params struct {
		PersistenceConfig config.Persistence
		MetricsClient     metrics.Client
		MessagingClient   messaging.Client
		ESClient          es.GenericClient
		ESConfig          *config.ElasticSearchConfig
		PinotConfig       *config.PinotVisibilityConfig
		PinotClient       pinot.GenericClient
		OSClient          es.GenericClient
		OSConfig          *config.ElasticSearchConfig
	}
)

// NewBeanFromFactory crate a new store bean using factory
func NewBeanFromFactory(
	factory Factory,
	params *Params,
	serviceConfig *service.Config,
) (Bean, error) {

	metadataMgr, err := factory.NewDomainManager()
	if err != nil {
		return nil, err
	}

	domainAuditMgr, err := factory.NewDomainAuditManager()
	if err != nil {
		return nil, err
	}

	taskMgr, err := factory.NewTaskManager()
	if err != nil {
		return nil, err
	}

	visibilityMgr, err := factory.NewVisibilityManager(params, serviceConfig)
	if err != nil {
		return nil, err
	}

	domainReplicationQueue, err := factory.NewDomainReplicationQueueManager()
	if err != nil {
		return nil, err
	}

	shardMgr, err := factory.NewShardManager()
	if err != nil {
		return nil, err
	}

	historyMgr, err := factory.NewHistoryManager()
	if err != nil {
		return nil, err
	}

	configStoreMgr, err := factory.NewConfigStoreManager()
	if err != nil {
		return nil, err
	}

	return NewBean(
		metadataMgr,
		domainAuditMgr,
		taskMgr,
		visibilityMgr,
		domainReplicationQueue,
		shardMgr,
		historyMgr,
		configStoreMgr,
		factory,
	), nil
}

// NewBean create a new store bean
func NewBean(
	domainManager persistence.DomainManager,
	domainAuditManager persistence.DomainAuditManager,
	taskManager persistence.TaskManager,
	visibilityManager persistence.VisibilityManager,
	domainReplicationQueueManager persistence.QueueManager,
	shardManager persistence.ShardManager,
	historyManager persistence.HistoryManager,
	configStoreManager persistence.ConfigStoreManager,
	executionManagerFactory persistence.ExecutionManagerFactory,
) *BeanImpl {
	return &BeanImpl{
		domainManager:                 domainManager,
		domainAuditManager:            domainAuditManager,
		taskManager:                   taskManager,
		visibilityManager:             visibilityManager,
		domainReplicationQueueManager: domainReplicationQueueManager,
		shardManager:                  shardManager,
		historyManager:                historyManager,
		configStoreManager:            configStoreManager,
		executionManagerFactory:       executionManagerFactory,

		shardIDToExecutionManager: make(map[int]persistence.ExecutionManager),
	}
}

// GetDomainManager get DomainManager
func (s *BeanImpl) GetDomainManager() persistence.DomainManager {

	s.RLock()
	defer s.RUnlock()

	return s.domainManager
}

// SetMetadataManager set DomainManager
func (s *BeanImpl) SetDomainManager(
	domainManager persistence.DomainManager,
) {

	s.Lock()
	defer s.Unlock()

	s.domainManager = domainManager
}

// GetDomainAuditManager get DomainAuditManager
func (s *BeanImpl) GetDomainAuditManager() persistence.DomainAuditManager {

	s.RLock()
	defer s.RUnlock()

	return s.domainAuditManager
}

// SetDomainAuditManager set DomainAuditManager
func (s *BeanImpl) SetDomainAuditManager(
	domainAuditManager persistence.DomainAuditManager,
) {

	s.Lock()
	defer s.Unlock()

	s.domainAuditManager = domainAuditManager
}

// GetTaskManager get TaskManager
func (s *BeanImpl) GetTaskManager() persistence.TaskManager {

	s.RLock()
	defer s.RUnlock()

	return s.taskManager
}

// SetTaskManager set TaskManager
func (s *BeanImpl) SetTaskManager(
	taskManager persistence.TaskManager,
) {

	s.Lock()
	defer s.Unlock()

	s.taskManager = taskManager
}

// GetVisibilityManager get VisibilityManager
func (s *BeanImpl) GetVisibilityManager() persistence.VisibilityManager {

	s.RLock()
	defer s.RUnlock()

	return s.visibilityManager
}

// SetVisibilityManager set VisibilityManager
func (s *BeanImpl) SetVisibilityManager(
	visibilityManager persistence.VisibilityManager,
) {

	s.Lock()
	defer s.Unlock()

	s.visibilityManager = visibilityManager
}

// GetDomainReplicationQueueManager gets domain replication QueueManager
func (s *BeanImpl) GetDomainReplicationQueueManager() persistence.QueueManager {

	s.RLock()
	defer s.RUnlock()

	return s.domainReplicationQueueManager
}

// SetDomainReplicationQueueManager sets domain replication QueueManager
func (s *BeanImpl) SetDomainReplicationQueueManager(
	domainReplicationQueueManager persistence.QueueManager,
) {

	s.Lock()
	defer s.Unlock()

	s.domainReplicationQueueManager = domainReplicationQueueManager
}

// GetShardManager get ShardManager
func (s *BeanImpl) GetShardManager() persistence.ShardManager {

	s.RLock()
	defer s.RUnlock()

	return s.shardManager
}

// SetShardManager set ShardManager
func (s *BeanImpl) SetShardManager(
	shardManager persistence.ShardManager,
) {

	s.Lock()
	defer s.Unlock()

	s.shardManager = shardManager
}

// GetHistoryManager get HistoryManager
func (s *BeanImpl) GetHistoryManager() persistence.HistoryManager {

	s.RLock()
	defer s.RUnlock()

	return s.historyManager
}

// SetHistoryManager set HistoryManager
func (s *BeanImpl) SetHistoryManager(
	historyManager persistence.HistoryManager,
) {

	s.Lock()
	defer s.Unlock()

	s.historyManager = historyManager
}

// GetExecutionManager get ExecutionManager
func (s *BeanImpl) GetExecutionManager(
	shardID int,
) (persistence.ExecutionManager, error) {

	s.RLock()
	executionManager, ok := s.shardIDToExecutionManager[shardID]
	if ok {
		s.RUnlock()
		return executionManager, nil
	}
	s.RUnlock()

	s.Lock()
	defer s.Unlock()

	executionManager, ok = s.shardIDToExecutionManager[shardID]
	if ok {
		return executionManager, nil
	}

	executionManager, err := s.executionManagerFactory.NewExecutionManager(shardID)
	if err != nil {
		return nil, err
	}

	s.shardIDToExecutionManager[shardID] = executionManager
	return executionManager, nil
}

// SetExecutionManager set ExecutionManager
func (s *BeanImpl) SetExecutionManager(
	shardID int,
	executionManager persistence.ExecutionManager,
) {

	s.Lock()
	defer s.Unlock()

	s.shardIDToExecutionManager[shardID] = executionManager
}

// GetConfigStoreManager gets ConfigStoreManager
func (s *BeanImpl) GetConfigStoreManager() persistence.ConfigStoreManager {

	s.RLock()
	defer s.RUnlock()

	return s.configStoreManager
}

// GetConfigStoreManager gets ConfigStoreManager
func (s *BeanImpl) SetConfigStoreManager(
	configStoreManager persistence.ConfigStoreManager,
) {

	s.Lock()
	defer s.Unlock()

	s.configStoreManager = configStoreManager
}

// Close cleanup connections
func (s *BeanImpl) Close() {

	s.Lock()
	defer s.Unlock()

	s.domainManager.Close()
	if s.domainAuditManager != nil {
		s.domainAuditManager.Close()
	}
	s.taskManager.Close()
	if s.visibilityManager != nil {
		// visibilityManager can be nil
		s.visibilityManager.Close()
	}
	s.domainReplicationQueueManager.Close()
	s.shardManager.Close()
	s.historyManager.Close()
	s.executionManagerFactory.Close()
	s.configStoreManager.Close()
	for _, executionMgr := range s.shardIDToExecutionManager {
		executionMgr.Close()
	}
}
