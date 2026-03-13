package task

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
)

// testPriorityTask is a simple test implementation of PriorityTask
type testPriorityTask struct {
	domain   string
	tasklist string
	tenant   string
	state    State
}

func (t *testPriorityTask) Execute() error            { return nil }
func (t *testPriorityTask) HandleErr(err error) error { return err }
func (t *testPriorityTask) RetryErr(err error) bool   { return false }
func (t *testPriorityTask) Ack()                      { t.state = TaskStateAcked }
func (t *testPriorityTask) Nack(err error)            { t.state = TaskStatePending }
func (t *testPriorityTask) Cancel()                   { t.state = TaskStateCanceled }
func (t *testPriorityTask) State() State              { return t.state }
func (t *testPriorityTask) Priority() int             { return 0 }
func (t *testPriorityTask) SetPriority(p int)         {}

func TestHierarchicalWRRTaskPool_SingleLevel_IWRROrdering(t *testing.T) {
	// Define domain to weight mapping
	domainWeights := map[string]int{
		"domain0": 5,
		"domain1": 3,
		"domain2": 1,
	}

	pool := newHierarchicalWeightedRoundRobinTaskPool[string, *testPriorityTask](
		testlogger.New(t),
		metrics.NoopClient,
		clock.NewMockedTimeSource(),
		&HierarchicalWeightedRoundRobinTaskPoolOptions[string, *testPriorityTask]{
			BufferSize: 20,
			TaskToWeightedKeysFn: func(task *testPriorityTask) []WeightedKey[string] {
				// Single-level hierarchy: tasks grouped by domain with different weights
				return []WeightedKey[string]{
					{Key: task.domain, Weight: domainWeights[task.domain]},
				}
			},
		},
	)
	pool.Start()
	defer pool.Stop()

	// Create tasks with weights [5, 3, 1]
	// Domain0 (weight 5): 5 tasks
	// Domain1 (weight 3): 3 tasks
	// Domain2 (weight 1): 1 task
	var tasks []*testPriorityTask
	for i := 0; i < 5; i++ {
		tasks = append(tasks, &testPriorityTask{domain: "domain0"})
	}
	for i := 0; i < 3; i++ {
		tasks = append(tasks, &testPriorityTask{domain: "domain1"})
	}
	tasks = append(tasks, &testPriorityTask{domain: "domain2"})

	// Enqueue tasks in parallel using randomly chosen Enqueue or TryEnqueue
	var wg sync.WaitGroup
	for _, task := range tasks {
		wg.Add(1)
		go func(t *testPriorityTask) {
			defer wg.Done()
			// Randomly choose between Enqueue and TryEnqueue
			if rand.Intn(2) == 0 {
				_ = pool.Enqueue(t)
			} else {
				_, _ = pool.TryEnqueue(t)
			}
		}(task)
	}
	wg.Wait()

	// IWRR pattern for weights [5, 3, 1]: domain0, domain0, domain0, domain1, domain0, domain1, domain0, domain1, domain2
	// Expected domain sequence: [0, 0, 0, 1, 0, 1, 0, 1, 2]
	expectedDomainPattern := []string{"domain0", "domain0", "domain0", "domain1", "domain0", "domain1", "domain0", "domain1", "domain2"}

	var actualDomainSequence []string
	for {
		task, ok := pool.TryDequeue()
		if !ok {
			break
		}
		actualDomainSequence = append(actualDomainSequence, task.domain)
	}

	assert.Equal(t, expectedDomainPattern, actualDomainSequence)
}

func TestHierarchicalWRRTaskPool_TwoLevel_IWRROrdering(t *testing.T) {
	// Define domain to weight mapping - different weights
	domainWeights := map[string]int{
		"domain1": 3,
		"domain2": 2,
		"domain3": 1,
	}

	// Define tasklist to weight mapping
	tasklistWeights := map[string]int{
		"tasklist1A": 2,
		"tasklist1B": 1,
		"tasklist2A": 3,
		"tasklist2B": 1,
		"tasklist3A": 2,
		"tasklist3B": 1,
	}

	pool := newHierarchicalWeightedRoundRobinTaskPool[string, *testPriorityTask](
		testlogger.New(t),
		metrics.NoopClient,
		clock.NewMockedTimeSource(),
		&HierarchicalWeightedRoundRobinTaskPoolOptions[string, *testPriorityTask]{
			BufferSize: 20,
			TaskToWeightedKeysFn: func(task *testPriorityTask) []WeightedKey[string] {
				// Two-level hierarchy: domain (level 1) -> tasklist (level 2)
				return []WeightedKey[string]{
					{Key: task.domain, Weight: domainWeights[task.domain]},
					{Key: task.tasklist, Weight: tasklistWeights[task.tasklist]},
				}
			},
		},
	)
	pool.Start()
	defer pool.Stop()

	// Create enough tasks for each domain to complete at least one full tasklist IWRR cycle
	// Domain1 tasklist IWRR [2, 1]: [tasklist1A, tasklist1A, tasklist1B] = 3 tasks
	// Domain2 tasklist IWRR [3, 1]: [tasklist2A, tasklist2A, tasklist2A, tasklist2B] = 4 tasks
	// Domain3 tasklist IWRR [2, 1]: [tasklist3A, tasklist3A, tasklist3B] = 3 tasks
	tasks := []*testPriorityTask{
		// Domain1 tasks (3 tasks for one complete tasklist IWRR)
		&testPriorityTask{domain: "domain1", tasklist: "tasklist1A"},
		&testPriorityTask{domain: "domain1", tasklist: "tasklist1A"},
		&testPriorityTask{domain: "domain1", tasklist: "tasklist1B"},
		// Domain2 tasks (4 tasks for one complete tasklist IWRR)
		&testPriorityTask{domain: "domain2", tasklist: "tasklist2A"},
		&testPriorityTask{domain: "domain2", tasklist: "tasklist2A"},
		&testPriorityTask{domain: "domain2", tasklist: "tasklist2A"},
		&testPriorityTask{domain: "domain2", tasklist: "tasklist2B"},
		// Domain3 tasks (3 tasks for one complete tasklist IWRR)
		&testPriorityTask{domain: "domain3", tasklist: "tasklist3A"},
		&testPriorityTask{domain: "domain3", tasklist: "tasklist3A"},
		&testPriorityTask{domain: "domain3", tasklist: "tasklist3B"},
	}

	// Enqueue all tasks in parallel using randomly chosen Enqueue or TryEnqueue
	var wg sync.WaitGroup
	for _, task := range tasks {
		wg.Add(1)
		go func(t *testPriorityTask) {
			defer wg.Done()
			// Randomly choose between Enqueue and TryEnqueue
			if rand.Intn(2) == 0 {
				_ = pool.Enqueue(t)
			} else {
				_, _ = pool.TryEnqueue(t)
			}
		}(task)
	}
	wg.Wait()

	// Domain IWRR pattern for weights [3, 2, 1]: [domain1, domain1, domain2, domain1, domain2, domain3]
	// When we dequeue, we follow this domain pattern, and within each domain we follow its tasklist IWRR
	//
	// Expected dequeue pattern:
	// 1. domain1 (1st visit) -> tasklist1A (position 0 in tasklist IWRR)
	// 2. domain1 (2nd visit) -> tasklist1A (position 1 in tasklist IWRR)
	// 3. domain2 (1st visit) -> tasklist2A (position 0 in tasklist IWRR)
	// 4. domain1 (3rd visit) -> tasklist1B (position 2 in tasklist IWRR)
	// 5. domain2 (2nd visit) -> tasklist2A (position 1 in tasklist IWRR)
	// 6. domain3 (1st visit) -> tasklist3A (position 0 in tasklist IWRR)
	// Now we loop back to domain1, but it's exhausted, so we skip it
	// 7. domain2 (3rd visit) -> tasklist2A (position 2 in tasklist IWRR)
	// 8. domain2 (4th visit) -> tasklist2B (position 3 in tasklist IWRR)
	// 9. domain3 (2nd visit) -> tasklist3A (position 1 in tasklist IWRR)
	// 10. domain3 (3rd visit) -> tasklist3B (position 2 in tasklist IWRR)
	type expectedTask struct {
		domain   string
		tasklist string
	}
	expectedPattern := []expectedTask{
		{"domain1", "tasklist1A"}, // domain IWRR pos 0, tasklist IWRR pos 0
		{"domain1", "tasklist1A"}, // domain IWRR pos 1, tasklist IWRR pos 1
		{"domain2", "tasklist2A"}, // domain IWRR pos 2, tasklist IWRR pos 0
		{"domain1", "tasklist1B"}, // domain IWRR pos 3, tasklist IWRR pos 2
		{"domain2", "tasklist2A"}, // domain IWRR pos 4, tasklist IWRR pos 1
		{"domain3", "tasklist3A"}, // domain IWRR pos 5, tasklist IWRR pos 0
		{"domain2", "tasklist2A"}, // domain1 exhausted, skip to domain2, tasklist IWRR pos 2
		{"domain2", "tasklist2B"}, // continue domain2, tasklist IWRR pos 3
		{"domain3", "tasklist3A"}, // domain3, tasklist IWRR pos 1
		{"domain3", "tasklist3B"}, // domain3, tasklist IWRR pos 2
	}

	// Dequeue all tasks and verify the order
	var actualPattern []expectedTask
	for {
		task, ok := pool.TryDequeue()
		if !ok {
			break
		}
		actualPattern = append(actualPattern, expectedTask{
			domain:   task.domain,
			tasklist: task.tasklist,
		})
	}

	// Verify exact ordering
	require.Equal(t, len(expectedPattern), len(actualPattern), "should dequeue all tasks")
	for i, expected := range expectedPattern {
		assert.Equal(t, expected.domain, actualPattern[i].domain, "task %d domain mismatch", i)
		assert.Equal(t, expected.tasklist, actualPattern[i].tasklist, "task %d tasklist mismatch", i)
	}
}

func TestHierarchicalWRRTaskPool_TwoLevel_LargeScale_IWRROrdering(t *testing.T) {
	// Define domain to weight mapping
	domainWeights := map[string]int{
		"domain1": 5,
		"domain2": 3,
		"domain3": 2,
	}

	// Define tasklist to weight mapping
	tasklistWeights := map[string]int{
		"tasklist1A": 3,
		"tasklist1B": 2,
		"tasklist1C": 1,
		"tasklist2A": 4,
		"tasklist2B": 2,
		"tasklist3A": 3,
		"tasklist3B": 1,
	}

	pool := newHierarchicalWeightedRoundRobinTaskPool[string, *testPriorityTask](
		testlogger.New(t),
		metrics.NoopClient,
		clock.NewMockedTimeSource(),
		&HierarchicalWeightedRoundRobinTaskPoolOptions[string, *testPriorityTask]{
			BufferSize: 100,
			TaskToWeightedKeysFn: func(task *testPriorityTask) []WeightedKey[string] {
				return []WeightedKey[string]{
					{Key: task.domain, Weight: domainWeights[task.domain]},
					{Key: task.tasklist, Weight: tasklistWeights[task.tasklist]},
				}
			},
		},
	)
	pool.Start()
	defer pool.Stop()

	// Create a large number of tasks
	// Domain1: 30 tasks (5 cycles of tasklist IWRR: [3,2,1] = 6 tasks per cycle)
	// Domain2: 30 tasks (5 cycles of tasklist IWRR: [4,2] = 6 tasks per cycle)
	// Domain3: 20 tasks (5 cycles of tasklist IWRR: [3,1] = 4 tasks per cycle)
	var tasks []*testPriorityTask

	// Domain1: 5 cycles of [tasklist1A(3), tasklist1B(2), tasklist1C(1)]
	for cycle := 0; cycle < 5; cycle++ {
		for i := 0; i < 3; i++ {
			tasks = append(tasks, &testPriorityTask{domain: "domain1", tasklist: "tasklist1A"})
		}
		for i := 0; i < 2; i++ {
			tasks = append(tasks, &testPriorityTask{domain: "domain1", tasklist: "tasklist1B"})
		}
		tasks = append(tasks, &testPriorityTask{domain: "domain1", tasklist: "tasklist1C"})
	}

	// Domain2: 5 cycles of [tasklist2A(4), tasklist2B(2)]
	for cycle := 0; cycle < 5; cycle++ {
		for i := 0; i < 4; i++ {
			tasks = append(tasks, &testPriorityTask{domain: "domain2", tasklist: "tasklist2A"})
		}
		for i := 0; i < 2; i++ {
			tasks = append(tasks, &testPriorityTask{domain: "domain2", tasklist: "tasklist2B"})
		}
	}

	// Domain3: 5 cycles of [tasklist3A(3), tasklist3B(1)]
	for cycle := 0; cycle < 5; cycle++ {
		for i := 0; i < 3; i++ {
			tasks = append(tasks, &testPriorityTask{domain: "domain3", tasklist: "tasklist3A"})
		}
		tasks = append(tasks, &testPriorityTask{domain: "domain3", tasklist: "tasklist3B"})
	}

	// Enqueue all tasks in parallel using randomly chosen Enqueue or TryEnqueue
	var wg sync.WaitGroup
	for _, task := range tasks {
		wg.Add(1)
		go func(t *testPriorityTask) {
			defer wg.Done()
			if rand.Intn(2) == 0 {
				_ = pool.Enqueue(t)
			} else {
				_, _ = pool.TryEnqueue(t)
			}
		}(task)
	}
	wg.Wait()

	// Generate expected patterns by level
	// Level 1 (domain): IWRR for weights [5, 3, 2]
	// Round 4: domain1 (weight 5 > 4)
	// Round 3: domain1 (weight 5 > 3)
	// Round 2: domain1, domain2 (weights 5,3 > 2)
	// Round 1: domain1, domain2, domain3 (weights 5,3,2 > 1)
	// Round 0: domain1, domain2, domain3 (weights 5,3,2 > 0)
	// Pattern: [domain1, domain1, domain1, domain2, domain1, domain2, domain3, domain1, domain2, domain3]
	expectedDomainPattern := []string{
		"domain1", "domain1", "domain1", "domain2", "domain1", "domain2", "domain3", "domain1", "domain2", "domain3",
	}

	// Level 2 (tasklist per domain):
	// domain1 with weights [3, 2, 1]: [tasklist1A, tasklist1A, tasklist1B, tasklist1A, tasklist1B, tasklist1C]
	// domain2 with weights [4, 2]: [tasklist2A, tasklist2A, tasklist2A, tasklist2B, tasklist2A, tasklist2B]
	// domain3 with weights [3, 1]: [tasklist3A, tasklist3A, tasklist3A, tasklist3B]
	expectedTasklistPatterns := map[string][]string{
		"domain1": {"tasklist1A", "tasklist1A", "tasklist1B", "tasklist1A", "tasklist1B", "tasklist1C"},
		"domain2": {"tasklist2A", "tasklist2A", "tasklist2A", "tasklist2B", "tasklist2A", "tasklist2B"},
		"domain3": {"tasklist3A", "tasklist3A", "tasklist3A", "tasklist3B"},
	}

	// Dequeue all tasks
	var dequeuedDomains []string
	dequeuedTasklistsByDomain := make(map[string][]string)
	for {
		task, ok := pool.TryDequeue()
		if !ok {
			break
		}
		dequeuedDomains = append(dequeuedDomains, task.domain)
		dequeuedTasklistsByDomain[task.domain] = append(
			dequeuedTasklistsByDomain[task.domain],
			task.tasklist,
		)
	}

	// Verify we got all 80 tasks
	require.Equal(t, 80, len(dequeuedDomains), "should dequeue all 80 tasks")

	// Verify level 1 (domain) pattern for the first cycle (first 10 tasks)
	// After that, domains may exhaust at different rates, so we only verify the first full cycle
	for i := 0; i < len(expectedDomainPattern) && i < len(dequeuedDomains); i++ {
		assert.Equal(t, expectedDomainPattern[i], dequeuedDomains[i],
			"domain mismatch at position %d in first cycle", i)
	}

	// Verify level 2 (tasklist) pattern for each domain
	// Each domain should follow its own tasklist IWRR pattern consistently
	for domain, dequeuedTasklists := range dequeuedTasklistsByDomain {
		expectedPattern := expectedTasklistPatterns[domain]
		for i := 0; i < len(dequeuedTasklists); i++ {
			expectedTasklist := expectedPattern[i%len(expectedPattern)]
			assert.Equal(t, expectedTasklist, dequeuedTasklists[i],
				"tasklist mismatch for domain %s at position %d", domain, i)
		}
	}
}

func TestHierarchicalWRRTaskPool_ThreeLevel_LargeScale_IWRROrdering(t *testing.T) {
	// Define domain to weight mapping
	domainWeights := map[string]int{
		"domain1": 3,
		"domain2": 2,
		"domain3": 1,
	}

	// Define tasklist to weight mapping
	tasklistWeights := map[string]int{
		"tasklist1A": 3,
		"tasklist1B": 1,
		"tasklist2A": 2,
		"tasklist2B": 1,
		"tasklist3A": 2,
		"tasklist3B": 1,
	}

	// Define tenant to weight mapping
	tenantWeights := map[string]int{
		"tenant1": 3,
		"tenant2": 2,
		"tenant3": 1,
	}

	pool := newHierarchicalWeightedRoundRobinTaskPool[string, *testPriorityTask](
		testlogger.New(t),
		metrics.NoopClient,
		clock.NewMockedTimeSource(),
		&HierarchicalWeightedRoundRobinTaskPoolOptions[string, *testPriorityTask]{
			BufferSize: 200,
			TaskToWeightedKeysFn: func(task *testPriorityTask) []WeightedKey[string] {
				return []WeightedKey[string]{
					{Key: task.domain, Weight: domainWeights[task.domain]},
					{Key: task.tasklist, Weight: tasklistWeights[task.tasklist]},
					{Key: task.tenant, Weight: tenantWeights[task.tenant]},
				}
			},
		},
	)
	pool.Start()
	defer pool.Stop()

	// Create a large number of tasks across 3 levels
	// Each (domain, tasklist) combination gets multiple tenant cycles
	// Tenant IWRR for weights [3, 2, 1]: 6 tasks per cycle
	var tasks []*testPriorityTask

	// Domain1 - tasklist1A (weight 3) - 3 tenant cycles = 18 tasks
	for cycle := 0; cycle < 3; cycle++ {
		for i := 0; i < 3; i++ {
			tasks = append(tasks, &testPriorityTask{domain: "domain1", tasklist: "tasklist1A", tenant: "tenant1"})
		}
		for i := 0; i < 2; i++ {
			tasks = append(tasks, &testPriorityTask{domain: "domain1", tasklist: "tasklist1A", tenant: "tenant2"})
		}
		tasks = append(tasks, &testPriorityTask{domain: "domain1", tasklist: "tasklist1A", tenant: "tenant3"})
	}

	// Domain1 - tasklist1B (weight 1) - 3 tenant cycles = 18 tasks
	for cycle := 0; cycle < 3; cycle++ {
		for i := 0; i < 3; i++ {
			tasks = append(tasks, &testPriorityTask{domain: "domain1", tasklist: "tasklist1B", tenant: "tenant1"})
		}
		for i := 0; i < 2; i++ {
			tasks = append(tasks, &testPriorityTask{domain: "domain1", tasklist: "tasklist1B", tenant: "tenant2"})
		}
		tasks = append(tasks, &testPriorityTask{domain: "domain1", tasklist: "tasklist1B", tenant: "tenant3"})
	}

	// Domain2 - tasklist2A (weight 2) - 2 tenant cycles = 12 tasks
	for cycle := 0; cycle < 2; cycle++ {
		for i := 0; i < 3; i++ {
			tasks = append(tasks, &testPriorityTask{domain: "domain2", tasklist: "tasklist2A", tenant: "tenant1"})
		}
		for i := 0; i < 2; i++ {
			tasks = append(tasks, &testPriorityTask{domain: "domain2", tasklist: "tasklist2A", tenant: "tenant2"})
		}
		tasks = append(tasks, &testPriorityTask{domain: "domain2", tasklist: "tasklist2A", tenant: "tenant3"})
	}

	// Domain2 - tasklist2B (weight 1) - 2 tenant cycles = 12 tasks
	for cycle := 0; cycle < 2; cycle++ {
		for i := 0; i < 3; i++ {
			tasks = append(tasks, &testPriorityTask{domain: "domain2", tasklist: "tasklist2B", tenant: "tenant1"})
		}
		for i := 0; i < 2; i++ {
			tasks = append(tasks, &testPriorityTask{domain: "domain2", tasklist: "tasklist2B", tenant: "tenant2"})
		}
		tasks = append(tasks, &testPriorityTask{domain: "domain2", tasklist: "tasklist2B", tenant: "tenant3"})
	}

	// Domain3 - tasklist3A (weight 2) - 2 tenant cycles = 12 tasks
	for cycle := 0; cycle < 2; cycle++ {
		for i := 0; i < 3; i++ {
			tasks = append(tasks, &testPriorityTask{domain: "domain3", tasklist: "tasklist3A", tenant: "tenant1"})
		}
		for i := 0; i < 2; i++ {
			tasks = append(tasks, &testPriorityTask{domain: "domain3", tasklist: "tasklist3A", tenant: "tenant2"})
		}
		tasks = append(tasks, &testPriorityTask{domain: "domain3", tasklist: "tasklist3A", tenant: "tenant3"})
	}

	// Domain3 - tasklist3B (weight 1) - 2 tenant cycles = 12 tasks
	for cycle := 0; cycle < 2; cycle++ {
		for i := 0; i < 3; i++ {
			tasks = append(tasks, &testPriorityTask{domain: "domain3", tasklist: "tasklist3B", tenant: "tenant1"})
		}
		for i := 0; i < 2; i++ {
			tasks = append(tasks, &testPriorityTask{domain: "domain3", tasklist: "tasklist3B", tenant: "tenant2"})
		}
		tasks = append(tasks, &testPriorityTask{domain: "domain3", tasklist: "tasklist3B", tenant: "tenant3"})
	}

	// Total: 18+18+12+12+12+12 = 84 tasks

	// Enqueue all tasks in parallel using randomly chosen Enqueue or TryEnqueue
	var wg sync.WaitGroup
	for _, task := range tasks {
		wg.Add(1)
		go func(t *testPriorityTask) {
			defer wg.Done()
			if rand.Intn(2) == 0 {
				_ = pool.Enqueue(t)
			} else {
				_, _ = pool.TryEnqueue(t)
			}
		}(task)
	}
	wg.Wait()

	// Dequeue all tasks
	type taskKey struct {
		domain   string
		tasklist string
	}
	var dequeuedDomains []string
	dequeuedTasklistsByDomain := make(map[string][]string)
	dequeuedTenantsByTasklist := make(map[taskKey][]string)

	for {
		task, ok := pool.TryDequeue()
		if !ok {
			break
		}
		dequeuedDomains = append(dequeuedDomains, task.domain)
		dequeuedTasklistsByDomain[task.domain] = append(
			dequeuedTasklistsByDomain[task.domain],
			task.tasklist,
		)
		key := taskKey{domain: task.domain, tasklist: task.tasklist}
		dequeuedTenantsByTasklist[key] = append(
			dequeuedTenantsByTasklist[key],
			task.tenant,
		)
	}

	// Verify we got all 84 tasks
	require.Equal(t, 84, len(dequeuedDomains), "should dequeue all 84 tasks")

	// Build expected domain sequence manually with exhaustion tracking
	// Domain task counts: domain1=36, domain2=24, domain3=24
	// Pattern: [domain1, domain1, domain2, domain1, domain2, domain3] repeats 12 times (72 tasks)
	// After 12 cycles: domain1=36 (exhausted), domain2=24 (exhausted), domain3=12
	// Then domain3 continues for 12 more tasks
	expectedDomainSequence := []string{
		// 12 full cycles
		"domain1", "domain1", "domain2", "domain1", "domain2", "domain3",
		"domain1", "domain1", "domain2", "domain1", "domain2", "domain3",
		"domain1", "domain1", "domain2", "domain1", "domain2", "domain3",
		"domain1", "domain1", "domain2", "domain1", "domain2", "domain3",
		"domain1", "domain1", "domain2", "domain1", "domain2", "domain3",
		"domain1", "domain1", "domain2", "domain1", "domain2", "domain3",
		"domain1", "domain1", "domain2", "domain1", "domain2", "domain3",
		"domain1", "domain1", "domain2", "domain1", "domain2", "domain3",
		"domain1", "domain1", "domain2", "domain1", "domain2", "domain3",
		"domain1", "domain1", "domain2", "domain1", "domain2", "domain3",
		"domain1", "domain1", "domain2", "domain1", "domain2", "domain3",
		"domain1", "domain1", "domain2", "domain1", "domain2", "domain3",
		// domain1 and domain2 exhausted, domain3 continues for 12 more
		"domain3", "domain3", "domain3", "domain3", "domain3", "domain3",
		"domain3", "domain3", "domain3", "domain3", "domain3", "domain3",
	}

	// Verify full domain sequence
	require.Equal(t, 84, len(expectedDomainSequence), "expected domain sequence length")
	require.Equal(t, len(expectedDomainSequence), len(dequeuedDomains),
		"domain sequence length mismatch")
	for i := 0; i < len(expectedDomainSequence); i++ {
		assert.Equal(t, expectedDomainSequence[i], dequeuedDomains[i],
			"domain mismatch at position %d", i)
	}

	// Build expected tasklist sequences manually for each domain
	expectedTasklistSequences := map[string][]string{
		// Domain1: tasklist1A=18, tasklist1B=18
		// Pattern: [tasklist1A, tasklist1A, tasklist1A, tasklist1B] repeats 6 times (24 tasks)
		// After 6 cycles: tasklist1A=18 (exhausted), tasklist1B=6
		// Then tasklist1B continues for 12 more tasks
		"domain1": {
			// 6 full cycles
			"tasklist1A", "tasklist1A", "tasklist1A", "tasklist1B",
			"tasklist1A", "tasklist1A", "tasklist1A", "tasklist1B",
			"tasklist1A", "tasklist1A", "tasklist1A", "tasklist1B",
			"tasklist1A", "tasklist1A", "tasklist1A", "tasklist1B",
			"tasklist1A", "tasklist1A", "tasklist1A", "tasklist1B",
			"tasklist1A", "tasklist1A", "tasklist1A", "tasklist1B",
			// tasklist1A exhausted, tasklist1B continues for 12 more
			"tasklist1B", "tasklist1B", "tasklist1B", "tasklist1B",
			"tasklist1B", "tasklist1B", "tasklist1B", "tasklist1B",
			"tasklist1B", "tasklist1B", "tasklist1B", "tasklist1B",
		},
		// Domain2: tasklist2A=12, tasklist2B=12
		// Pattern: [tasklist2A, tasklist2A, tasklist2B] repeats 6 times (18 tasks)
		// After 6 cycles: tasklist2A=12 (exhausted), tasklist2B=6
		// Then tasklist2B continues for 6 more tasks
		"domain2": {
			// 6 full cycles
			"tasklist2A", "tasklist2A", "tasklist2B",
			"tasklist2A", "tasklist2A", "tasklist2B",
			"tasklist2A", "tasklist2A", "tasklist2B",
			"tasklist2A", "tasklist2A", "tasklist2B",
			"tasklist2A", "tasklist2A", "tasklist2B",
			"tasklist2A", "tasklist2A", "tasklist2B",
			// tasklist2A exhausted, tasklist2B continues for 6 more
			"tasklist2B", "tasklist2B", "tasklist2B",
			"tasklist2B", "tasklist2B", "tasklist2B",
		},
		// Domain3: tasklist3A=12, tasklist3B=12
		// Pattern: [tasklist3A, tasklist3A, tasklist3B] repeats 6 times (18 tasks)
		// After 6 cycles: tasklist3A=12 (exhausted), tasklist3B=6
		// Then tasklist3B continues for 6 more tasks
		"domain3": {
			// 6 full cycles
			"tasklist3A", "tasklist3A", "tasklist3B",
			"tasklist3A", "tasklist3A", "tasklist3B",
			"tasklist3A", "tasklist3A", "tasklist3B",
			"tasklist3A", "tasklist3A", "tasklist3B",
			"tasklist3A", "tasklist3A", "tasklist3B",
			"tasklist3A", "tasklist3A", "tasklist3B",
			// tasklist3A exhausted, tasklist3B continues for 6 more
			"tasklist3B", "tasklist3B", "tasklist3B",
			"tasklist3B", "tasklist3B", "tasklist3B",
		},
	}

	// Verify full tasklist sequence for each domain
	for domain, expectedSequence := range expectedTasklistSequences {
		dequeuedTasklists := dequeuedTasklistsByDomain[domain]
		require.Equal(t, len(expectedSequence), len(dequeuedTasklists),
			"tasklist sequence length mismatch for domain %s", domain)
		for i := 0; i < len(expectedSequence); i++ {
			assert.Equal(t, expectedSequence[i], dequeuedTasklists[i],
				"tasklist mismatch for domain %s at position %d", domain, i)
		}
	}

	// Verify level 3 (tenant) pattern for each (domain, tasklist) combination
	// Build expected tenant sequences manually
	// Pattern: [tenant1, tenant1, tenant2, tenant1, tenant2, tenant3] - 6 tasks per cycle
	expectedTenantSequences := map[taskKey][]string{
		// Domain1 tasklists: 3 cycles each = 18 tasks
		{domain: "domain1", tasklist: "tasklist1A"}: {
			"tenant1", "tenant1", "tenant2", "tenant1", "tenant2", "tenant3",
			"tenant1", "tenant1", "tenant2", "tenant1", "tenant2", "tenant3",
			"tenant1", "tenant1", "tenant2", "tenant1", "tenant2", "tenant3",
		},
		{domain: "domain1", tasklist: "tasklist1B"}: {
			"tenant1", "tenant1", "tenant2", "tenant1", "tenant2", "tenant3",
			"tenant1", "tenant1", "tenant2", "tenant1", "tenant2", "tenant3",
			"tenant1", "tenant1", "tenant2", "tenant1", "tenant2", "tenant3",
		},
		// Domain2 tasklists: 2 cycles each = 12 tasks
		{domain: "domain2", tasklist: "tasklist2A"}: {
			"tenant1", "tenant1", "tenant2", "tenant1", "tenant2", "tenant3",
			"tenant1", "tenant1", "tenant2", "tenant1", "tenant2", "tenant3",
		},
		{domain: "domain2", tasklist: "tasklist2B"}: {
			"tenant1", "tenant1", "tenant2", "tenant1", "tenant2", "tenant3",
			"tenant1", "tenant1", "tenant2", "tenant1", "tenant2", "tenant3",
		},
		// Domain3 tasklists: 2 cycles each = 12 tasks
		{domain: "domain3", tasklist: "tasklist3A"}: {
			"tenant1", "tenant1", "tenant2", "tenant1", "tenant2", "tenant3",
			"tenant1", "tenant1", "tenant2", "tenant1", "tenant2", "tenant3",
		},
		{domain: "domain3", tasklist: "tasklist3B"}: {
			"tenant1", "tenant1", "tenant2", "tenant1", "tenant2", "tenant3",
			"tenant1", "tenant1", "tenant2", "tenant1", "tenant2", "tenant3",
		},
	}

	for key, expectedSequence := range expectedTenantSequences {
		dequeuedTenants := dequeuedTenantsByTasklist[key]
		require.Equal(t, len(expectedSequence), len(dequeuedTenants),
			"tenant sequence length mismatch for domain=%s, tasklist=%s",
			key.domain, key.tasklist)
		for i := 0; i < len(expectedSequence); i++ {
			assert.Equal(t, expectedSequence[i], dequeuedTenants[i],
				"tenant mismatch for domain=%s, tasklist=%s at position %d",
				key.domain, key.tasklist, i)
		}
	}
}

func TestHierarchicalWRRTaskPool_TryDequeue_Empty(t *testing.T) {
	domainWeights := map[string]int{
		"domain1": 3,
	}

	pool := newHierarchicalWeightedRoundRobinTaskPool[string](
		testlogger.New(t),
		metrics.NoopClient,
		clock.NewMockedTimeSource(),
		&HierarchicalWeightedRoundRobinTaskPoolOptions[string, *testPriorityTask]{
			BufferSize: 10,
			TaskToWeightedKeysFn: func(task *testPriorityTask) []WeightedKey[string] {
				return []WeightedKey[string]{
					{Key: task.domain, Weight: domainWeights[task.domain]},
				}
			},
		},
	)
	pool.Start()
	defer pool.Stop()

	// Try to dequeue from empty pool
	task, ok := pool.TryDequeue()
	assert.False(t, ok, "TryDequeue should return false on empty pool")
	assert.Nil(t, task, "task should be nil when dequeue fails")

	// Enqueue one task and dequeue it
	testTask := &testPriorityTask{domain: "domain1"}
	err := pool.Enqueue(testTask)
	require.NoError(t, err)

	task, ok = pool.TryDequeue()
	assert.True(t, ok, "TryDequeue should return true when task exists")
	assert.NotNil(t, task, "task should not be nil")

	// Try to dequeue again from now-empty pool
	task, ok = pool.TryDequeue()
	assert.False(t, ok, "TryDequeue should return false after pool is emptied")
	assert.Nil(t, task, "task should be nil when dequeue fails")
}

func TestHierarchicalWRRTaskPool_TryEnqueue_BufferFull(t *testing.T) {
	domainWeights := map[string]int{
		"domain1": 3,
	}

	// Create pool with small buffer size
	pool := newHierarchicalWeightedRoundRobinTaskPool[string, *testPriorityTask](
		testlogger.New(t),
		metrics.NoopClient,
		clock.NewMockedTimeSource(),
		&HierarchicalWeightedRoundRobinTaskPoolOptions[string, *testPriorityTask]{
			BufferSize: 2, // Small buffer to easily fill it
			TaskToWeightedKeysFn: func(task *testPriorityTask) []WeightedKey[string] {
				return []WeightedKey[string]{
					{Key: task.domain, Weight: domainWeights[task.domain]},
				}
			},
		},
	)
	pool.Start()
	defer pool.Stop()

	// Fill the buffer completely
	success1, err1 := pool.TryEnqueue(&testPriorityTask{domain: "domain1"})
	assert.True(t, success1, "first TryEnqueue should succeed")
	assert.NoError(t, err1)

	success2, err2 := pool.TryEnqueue(&testPriorityTask{domain: "domain1"})
	assert.True(t, success2, "second TryEnqueue should succeed")
	assert.NoError(t, err2)

	// Try to enqueue when buffer is full
	success3, err3 := pool.TryEnqueue(&testPriorityTask{domain: "domain1"})
	assert.False(t, success3, "TryEnqueue should return false when buffer is full")
	assert.NoError(t, err3, "TryEnqueue should not return error even when buffer is full")

	// Dequeue one task to free space
	task, ok := pool.TryDequeue()
	assert.True(t, ok, "TryDequeue should succeed")
	assert.NotNil(t, task)

	// Now TryEnqueue should succeed again
	success4, err4 := pool.TryEnqueue(&testPriorityTask{domain: "domain1"})
	assert.True(t, success4, "TryEnqueue should succeed after freeing space")
	assert.NoError(t, err4)
}

func TestHierarchicalWRRTaskPool_Enqueue_ContextCancellation(t *testing.T) {
	domainWeights := map[string]int{
		"domain1": 3,
	}

	// Create pool with small buffer size
	pool := newHierarchicalWeightedRoundRobinTaskPool[string, *testPriorityTask](
		testlogger.New(t),
		metrics.NoopClient,
		clock.NewMockedTimeSource(),
		&HierarchicalWeightedRoundRobinTaskPoolOptions[string, *testPriorityTask]{
			BufferSize: 1, // Very small buffer
			TaskToWeightedKeysFn: func(task *testPriorityTask) []WeightedKey[string] {
				return []WeightedKey[string]{
					{Key: task.domain, Weight: domainWeights[task.domain]},
				}
			},
		},
	)
	pool.Start()

	// Fill the buffer
	err := pool.Enqueue(&testPriorityTask{domain: "domain1"})
	require.NoError(t, err)

	// Start a goroutine that will block on Enqueue
	enqueueDone := make(chan error, 1)
	go func() {
		err := pool.Enqueue(&testPriorityTask{domain: "domain1"})
		enqueueDone <- err
	}()

	// Stop the pool, which cancels the context
	pool.Stop()

	// The blocked Enqueue should return with context error
	select {
	case err := <-enqueueDone:
		assert.Error(t, err, "Enqueue should return error when context is cancelled")
		assert.Equal(t, pool.ctx.Err(), err, "error should be context cancellation error")
	case <-time.After(1 * time.Second):
		t.Fatal("Enqueue did not unblock after context cancellation")
	}
}

func TestHierarchicalWRRTaskPool_CleanupLoop(t *testing.T) {
	domainWeights := map[string]int{
		"domain1": 3,
	}

	timeSource := clock.NewMockedTimeSource()
	pool := newHierarchicalWeightedRoundRobinTaskPool[string, *testPriorityTask](
		testlogger.New(t),
		metrics.NoopClient,
		timeSource,
		&HierarchicalWeightedRoundRobinTaskPoolOptions[string, *testPriorityTask]{
			BufferSize: 10,
			TaskToWeightedKeysFn: func(task *testPriorityTask) []WeightedKey[string] {
				return []WeightedKey[string]{
					{Key: task.domain, Weight: domainWeights[task.domain]},
				}
			},
		},
	)

	// Replace doCleanupFn with a fake that tracks calls
	cleanupCalls := make(chan struct {
		now time.Time
		ttl time.Duration
	}, 10)
	pool.doCleanupFn = func(now time.Time, ttl time.Duration) {
		cleanupCalls <- struct {
			now time.Time
			ttl time.Duration
		}{now: now, ttl: ttl}
	}

	pool.Start()

	// Advance time by cleanup interval (1800 seconds = TTL/2)
	timeSource.BlockUntil(1)
	timeSource.Advance(1800 * time.Second)

	// Wait for first cleanup call
	call1 := <-cleanupCalls
	expectedTTL := 3600 * time.Second
	require.Equal(t, expectedTTL, call1.ttl, "TTL should be 3600 seconds")
	require.NotZero(t, call1.now, "now should be set")

	// Advance time by another interval
	timeSource.BlockUntil(1)
	timeSource.Advance(1800 * time.Second)
	call2 := <-cleanupCalls
	require.Equal(t, expectedTTL, call2.ttl, "TTL should be 3600 seconds")
	require.True(t, call2.now.After(call1.now), "second call should have later timestamp")

	// Stop the pool
	pool.Stop()

	// Advance time - should NOT trigger cleanup because loop is stopped
	timeSource.Advance(1800 * time.Second)
	select {
	case <-cleanupCalls:
		t.Fatal("cleanup should not be called after Stop()")
	default:
		// Expected - no cleanup call
	}
}
