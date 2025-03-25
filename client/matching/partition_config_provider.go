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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination partition_config_provider_mock.go -package matching github.com/uber/cadence/client/matching PartitionConfigProvider

package matching

import (
	"sync"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

type (
	// PartitionConfigProvider is the interface for implementers of
	// component that provides partition configuration for task list
	// partitions
	PartitionConfigProvider interface {
		// GetNumberOfReadPartitions returns the number of read partitions
		GetNumberOfReadPartitions(domainID string, taskList types.TaskList, taskListType int) int
		// GetNumberOfWritePartitions returns the number of write partitions
		GetNumberOfWritePartitions(domainID string, taskList types.TaskList, taskListType int) int
		// GetPartitionConfig returns the cached partition configuration
		GetPartitionConfig(domainID string, taskList types.TaskList, taskListType int) *types.TaskListPartitionConfig
		// UpdatePartitionConfig updates the partition configuration for a task list
		UpdatePartitionConfig(domainID string, taskList types.TaskList, taskListType int, config *types.TaskListPartitionConfig)
	}

	syncedTaskListPartitionConfig struct {
		sync.RWMutex
		types.TaskListPartitionConfig
	}

	partitionConfigProviderImpl struct {
		configCache         cache.Cache
		logger              log.Logger
		metricsClient       metrics.Client
		domainIDToName      func(string) (string, error)
		enableReadFromCache dynamicconfig.BoolPropertyFnWithTaskListInfoFilters
		nReadPartitions     dynamicconfig.IntPropertyFnWithTaskListInfoFilters
		nWritePartitions    dynamicconfig.IntPropertyFnWithTaskListInfoFilters
	}
)

var singlePartitionConfig = createDefaultConfig(1, 1)

func (c *syncedTaskListPartitionConfig) updateConfig(newConfig types.TaskListPartitionConfig) bool {
	c.Lock()
	defer c.Unlock()
	if c.Version < newConfig.Version {
		c.TaskListPartitionConfig = newConfig
		return true
	}
	return false
}

func NewPartitionConfigProvider(
	logger log.Logger,
	metricsClient metrics.Client,
	domainIDToName func(string) (string, error),
	dc *dynamicconfig.Collection,
) PartitionConfigProvider {
	return &partitionConfigProviderImpl{
		logger:              logger,
		metricsClient:       metricsClient,
		domainIDToName:      domainIDToName,
		enableReadFromCache: dc.GetBoolPropertyFilteredByTaskListInfo(dynamicconfig.MatchingEnableGetNumberOfPartitionsFromCache),
		nReadPartitions:     dc.GetIntPropertyFilteredByTaskListInfo(dynamicconfig.MatchingNumTasklistReadPartitions),
		nWritePartitions:    dc.GetIntPropertyFilteredByTaskListInfo(dynamicconfig.MatchingNumTasklistWritePartitions),
		configCache: cache.New(&cache.Options{
			TTL:             0,
			InitialCapacity: 100,
			Pin:             false,
			MaxCount:        3000,
			ActivelyEvict:   false,
		}),
	}
}

func (p *partitionConfigProviderImpl) GetNumberOfReadPartitions(domainID string, taskList types.TaskList, taskListType int) int {
	config := p.GetPartitionConfig(domainID, taskList, taskListType)
	return len(config.ReadPartitions)
}

func (p *partitionConfigProviderImpl) GetNumberOfWritePartitions(domainID string, taskList types.TaskList, taskListType int) int {
	config := p.GetPartitionConfig(domainID, taskList, taskListType)
	v := config.Version
	w := len(config.WritePartitions)
	r := len(config.ReadPartitions)
	if w > r {
		p.logger.Warn("Number of write partitions exceeds number of read partitions, using number of read partitions", tag.WorkflowDomainID(domainID), tag.WorkflowTaskListName(taskList.GetName()), tag.WorkflowTaskListType(taskListType), tag.Dynamic("read-partition", r), tag.Dynamic("write-partition", w), tag.Dynamic("config-version", v))
		return r
	}
	return w
}

func (p *partitionConfigProviderImpl) GetPartitionConfig(domainID string, taskList types.TaskList, taskListType int) *types.TaskListPartitionConfig {
	domainName, err := p.domainIDToName(domainID)
	if err != nil {
		return createDefaultConfig(1, 1)
	}
	if !p.enableReadFromCache(domainName, taskList.GetName(), taskListType) {
		nWrite := p.nWritePartitions(domainName, taskList.GetName(), taskListType)
		nRead := p.nReadPartitions(domainName, taskList.GetName(), taskListType)
		// checks to make sure number of writes never exceeds number of reads
		if nWrite > nRead {
			p.logger.Warn("Number of write partitions exceeds number of read partitions, using number of read partitions", tag.WorkflowDomainID(domainID), tag.WorkflowTaskListName(taskList.GetName()), tag.WorkflowTaskListType(taskListType), tag.Dynamic("read-partition", nRead), tag.Dynamic("write-partition", nWrite))
			nWrite = nRead
		}
		return createDefaultConfig(nRead, nWrite)
	}
	c := p.getCachedPartitionConfig(domainID, taskList, taskListType)
	if c == nil {
		return singlePartitionConfig
	}
	c.RLock()
	config := c.TaskListPartitionConfig
	c.RUnlock()
	v := config.Version
	w := len(config.WritePartitions)
	r := len(config.ReadPartitions)
	scope := p.metricsClient.Scope(metrics.PartitionConfigProviderScope, metrics.DomainTag(domainName), metrics.TaskListRootPartitionTag(taskList.GetName()), getTaskListTypeTag(taskListType))
	scope.UpdateGauge(metrics.TaskListPartitionConfigNumReadGauge, float64(r))
	scope.UpdateGauge(metrics.TaskListPartitionConfigNumWriteGauge, float64(w))
	scope.UpdateGauge(metrics.TaskListPartitionConfigVersionGauge, float64(v))

	return &config
}

func (p *partitionConfigProviderImpl) UpdatePartitionConfig(domainID string, taskList types.TaskList, taskListType int, config *types.TaskListPartitionConfig) {
	if config == nil || taskList.GetKind() != types.TaskListKindNormal {
		return
	}
	taskListKey := key{
		domainID:     domainID,
		taskListName: taskList.Name,
		taskListType: taskListType,
	}
	var err error
	cI := p.configCache.Get(taskListKey)
	if cI == nil {
		cI, err = p.configCache.PutIfNotExist(taskListKey, &syncedTaskListPartitionConfig{TaskListPartitionConfig: *config})
		if err != nil {
			p.logger.Error("Failed put partition config into cache", tag.Error(err))
			return
		}
	}
	c, ok := cI.(*syncedTaskListPartitionConfig)
	if !ok {
		return
	}
	updated := c.updateConfig(*config)
	if updated {
		w := len(c.WritePartitions)
		r := len(c.ReadPartitions)
		p.logger.Info("tasklist partition config updated", tag.WorkflowDomainID(domainID), tag.WorkflowTaskListName(taskList.Name), tag.WorkflowTaskListType(taskListType), tag.Dynamic("read-partition", r), tag.Dynamic("write-partition", w), tag.Dynamic("config-version", config.Version))
	}
}

func (p *partitionConfigProviderImpl) getCachedPartitionConfig(domainID string, taskList types.TaskList, taskListType int) *syncedTaskListPartitionConfig {
	if taskList.GetKind() != types.TaskListKindNormal {
		return nil
	}
	taskListKey := key{
		domainID:     domainID,
		taskListName: taskList.Name,
		taskListType: taskListType,
	}
	cI := p.configCache.Get(taskListKey)
	if cI == nil {
		p.logger.Debug("Partition config not found in cache", tag.WorkflowDomainID(domainID), tag.WorkflowTaskListName(taskList.Name), tag.WorkflowTaskListType(taskListType))
		return nil
	}
	c, ok := cI.(*syncedTaskListPartitionConfig)
	if !ok {
		return nil
	}
	return c
}

func getTaskListTypeTag(taskListType int) metrics.Tag {
	switch taskListType {
	case persistence.TaskListTypeActivity:
		return metrics.TaskListTypeTag("activity")
	case persistence.TaskListTypeDecision:
		return metrics.TaskListTypeTag("decision")
	default:
		return metrics.TaskListTypeTag("")
	}
}

func createDefaultConfig(nRead, nWrite int) *types.TaskListPartitionConfig {
	read := make(map[int]*types.TaskListPartition, nRead)
	for i := 0; i < nRead; i++ {
		read[i] = &types.TaskListPartition{}
	}
	write := make(map[int]*types.TaskListPartition, nWrite)
	for i := 0; i < nWrite; i++ {
		write[i] = &types.TaskListPartition{}
	}
	return &types.TaskListPartitionConfig{
		Version:         0,
		ReadPartitions:  read,
		WritePartitions: write,
	}
}
