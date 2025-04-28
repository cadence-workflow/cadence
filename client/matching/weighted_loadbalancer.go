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

import (
	"math"
	"math/rand"
	"path"
	"sort"
	"strconv"
	"sync"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

const _backlogThreshold = 100

type (
	weightSelector struct {
		sync.RWMutex
		weights     []int64
		initialized bool
		threshold   int64
	}

	weightedLoadBalancer struct {
		fallbackLoadBalancer LoadBalancer
		provider             PartitionConfigProvider
		weightCache          cache.Cache
		logger               log.Logger
	}
)

func newWeightSelector(n int, threshold int64) *weightSelector {
	pw := &weightSelector{weights: make([]int64, n), threshold: threshold}
	for i := range pw.weights {
		pw.weights[i] = -1
	}
	return pw
}

func (pw *weightSelector) pickBetween(partitions []int) int {
	totalWeight := int64(0)
	cumulativeWeights := make([]int64, len(partitions))
	pw.RLock()
	defer pw.RUnlock()

	if !pw.initialized {
		return -1
	}
	shouldEnable := false
	for i, partitionID := range partitions {
		// Invalid partition specified, bail out
		if partitionID >= len(pw.weights) {
			return -1
		}
		weight := pw.weights[partitionID]
		totalWeight += weight
		cumulativeWeights[i] = totalWeight
		if weight > pw.threshold {
			shouldEnable = true // only enable weight selection if backlog size is larger than the threshold
		}
	}
	if totalWeight <= 0 || !shouldEnable {
		return -1
	}
	r := rand.Int63n(totalWeight)
	index := sort.Search(len(cumulativeWeights), func(i int) bool {
		return cumulativeWeights[i] > r
	})
	return partitions[index]
}

func (pw *weightSelector) pick() (int, []int64) {
	totalWeight := int64(0)
	pw.RLock()
	defer pw.RUnlock()
	cumulativeWeights := make([]int64, len(pw.weights))
	if !pw.initialized {
		return -1, cumulativeWeights
	}
	shouldDrain := false
	for i, w := range pw.weights {
		totalWeight += w
		cumulativeWeights[i] = totalWeight
		if w > pw.threshold {
			shouldDrain = true // only enable weight selection if backlog size is larger than the threshold
		}
	}
	if totalWeight <= 0 || !shouldDrain {
		return -1, cumulativeWeights
	}
	r := rand.Int63n(totalWeight)
	index := sort.Search(len(cumulativeWeights), func(i int) bool {
		return cumulativeWeights[i] > r
	})
	return index, cumulativeWeights
}

func (pw *weightSelector) update(n, p int, weight int64) {
	pw.Lock()
	defer pw.Unlock()
	if n > len(pw.weights) {
		newWeights := make([]int64, n)
		copy(newWeights, pw.weights)
		for i := len(pw.weights); i < n; i++ {
			newWeights[i] = -1
		}
		pw.weights = newWeights
		pw.initialized = false
	} else if n < len(pw.weights) {
		pw.weights = pw.weights[:n]
	}
	if p < n { // the opposite condition can happen when the task list is scaled down and we get an update from a drained partition
		pw.weights[p] = weight
	}
	for _, w := range pw.weights {
		if w == -1 {
			return
		}
	}
	pw.initialized = true
}

func NewWeightedLoadBalancer(
	lb LoadBalancer,
	provider PartitionConfigProvider,
	logger log.Logger,
) WeightedLoadBalancer {
	return &weightedLoadBalancer{
		fallbackLoadBalancer: lb,
		provider:             provider,
		weightCache: cache.New(&cache.Options{
			TTL:             0,
			InitialCapacity: 100,
			Pin:             false,
			MaxCount:        3000,
			ActivelyEvict:   false,
			MetricsScope:    provider.GetMetricsClient().Scope(metrics.LoadBalancerScope),
			Logger:          logger,
		}),
		logger: logger,
	}
}

func (lb *weightedLoadBalancer) PickWritePartition(
	taskListType int,
	req WriteRequest,
) string {
	return lb.fallbackLoadBalancer.PickWritePartition(taskListType, req)
}

func (lb *weightedLoadBalancer) PickReadPartition(
	taskListType int,
	req ReadRequest,
	isolationGroup string,
) string {
	taskListKey := key{
		domainID:     req.GetDomainUUID(),
		taskListName: req.GetTaskList().GetName(),
		taskListType: taskListType,
	}
	wI := lb.weightCache.Get(taskListKey)
	if wI == nil {
		return lb.fallbackLoadBalancer.PickReadPartition(taskListType, req, isolationGroup)
	}
	w, ok := wI.(*weightSelector)
	if !ok {
		return lb.fallbackLoadBalancer.PickReadPartition(taskListType, req, isolationGroup)
	}
	p, cumulativeWeights := w.pick()
	lb.logger.Debug("pick read partition", tag.WorkflowDomainID(req.GetDomainUUID()), tag.WorkflowTaskListName(req.GetTaskList().Name), tag.WorkflowTaskListType(taskListType), tag.Dynamic("cumulative-weights", cumulativeWeights), tag.Dynamic("task-list-partition", p))
	if p < 0 {
		return lb.fallbackLoadBalancer.PickReadPartition(taskListType, req, isolationGroup)
	}
	return getPartitionTaskListName(req.GetTaskList().GetName(), p)
}

func (lb *weightedLoadBalancer) UpdateWeight(
	taskListType int,
	req ReadRequest,
	partition string,
	info *types.LoadBalancerHints,
) {
	taskList := *req.GetTaskList()
	if info == nil {
		return
	}
	p := 0
	if partition != taskList.GetName() {
		var err error
		p, err = strconv.Atoi(path.Base(partition))
		if err != nil {
			return
		}
	}
	taskListKey := key{
		domainID:     req.GetDomainUUID(),
		taskListName: taskList.GetName(),
		taskListType: taskListType,
	}
	n := lb.provider.GetNumberOfReadPartitions(req.GetDomainUUID(), taskList, taskListType)
	if n <= 1 {
		lb.weightCache.Delete(taskListKey)
		return
	}
	wI := lb.weightCache.Get(taskListKey)
	if wI == nil {
		var err error
		w := newWeightSelector(n, _backlogThreshold)
		wI, err = lb.weightCache.PutIfNotExist(taskListKey, w)
		if err != nil {
			return
		}
	}
	w, ok := wI.(*weightSelector)
	if !ok {
		return
	}
	weight := calcWeightFromLoadBalancerHints(info)
	lb.logger.Debug("update task list partition weight", tag.WorkflowDomainID(req.GetDomainUUID()), tag.WorkflowTaskListName(taskList.GetName()), tag.WorkflowTaskListType(taskListType), tag.Dynamic("task-list-partition", p), tag.Dynamic("weight", weight), tag.Dynamic("load-balancer-hints", info))
	w.update(n, p, weight)
}

func (lb *weightedLoadBalancer) PickBetween(domainID, taskListName string, taskListType int, partitions []int) int {
	if len(partitions) == 0 {
		return -1
	}
	if len(partitions) == 1 {
		return partitions[0]
	}
	taskListKey := key{
		domainID:     domainID,
		taskListName: taskListName,
		taskListType: taskListType,
	}
	wI := lb.weightCache.Get(taskListKey)
	if wI == nil {
		return -1
	}
	w, ok := wI.(*weightSelector)
	if !ok {
		return -1
	}
	return w.pickBetween(partitions)
}

func calcWeightFromLoadBalancerHints(info *types.LoadBalancerHints) int64 {
	// according to Little's Law, the average number of tasks in the queue L = λW
	// where λ is the average arrival rate and W is the average wait time a task spends in the queue
	// here λ is the QPS and W is the average match latency which is 10ms
	// so the backlog hint should be backlog count + L.
	smoothingNumber := int64(0)
	qps := info.RatePerSecond
	if qps > 0.01 {
		smoothingNumber = int64(math.Ceil(qps * 0.01))
	}
	return info.BacklogCount + smoothingNumber
}
