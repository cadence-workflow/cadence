//go:build !race && cassandralwt
// +build !race,cassandralwt

/*
Measures how Cassandra handles concurrent semaphore acquires on one bucket, which is one partition
of semaphore_tokens: a bucket manager on the real token store, filled by concurrent acquires.
TestSemaphoreCassandraLWTRetry (semaphore_cassandra_lwt_retry_test.go) adds retried acquires; the
steps below run both.

The test runs whichever lock design the checked-out code has. To compare them, run the same steps
on each:
  - per-bucket lock: master.
  - per-token locks: the per-token lock PR (#8691) branch, with master merged in so it has this test.

Steps, on a two-node cluster with 2 replicas:

1. Start the cluster (once for both runs):

	docker compose -f docker/github_actions/docker-compose-cassandra-lwt.yml up -d --wait cass1 cass2

2. Check out the lock design to test, then run:

	docker compose -f docker/github_actions/docker-compose-cassandra-lwt.yml run --rm test-semaphore-lwt

   If the image build can't download modules, build the test on the host and run it in a container instead:

	GOOS=linux CGO_ENABLED=0 go test -c -tags cassandralwt -o /tmp/semlwt.test ./host/
	docker run --rm --network services-network -v "$PWD":/cadence -v /tmp/semlwt.test:/semlwt.test \
		-w /cadence/host -e CASSANDRA=1 -e CASSANDRA_SEEDS=cass1,cass2 -e SEMAPHORE_LWT_REPLICATION_FACTOR=2 \
		--entrypoint /semlwt.test cassandra:4.1.1 -test.run '^TestSemaphoreCassandraLWT(Retry)?$' -test.v -test.timeout 30m

3. Repeat step 2 on the other lock design, then compare the two runs, and the results in
   semaphore_cassandra_lwt_results.md.

4. Stop the cluster:

	docker compose -f docker/github_actions/docker-compose-cassandra-lwt.yml down

For a quick run against a single local node (1 replica, so conditional writes don't contend across replicas):

	docker run -d --name cass-lwt -p 9042:9042 cassandra:4.1.1
	CASSANDRA=1 go test -timeout 30m -count 1 -v -tags cassandralwt -run '^TestSemaphoreCassandraLWT(Retry)?$' ./host/

SEMAPHORE_LWT_REPLICATION_FACTOR sets replicas per row (default 1).

Results: semaphore_cassandra_lwt_results.md.
*/

package host

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pborman/uuid"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql/public"
	pt "github.com/uber/cadence/common/persistence/persistence-tests"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/matching/semaphore"
	"github.com/uber/cadence/testflags"

	_ "github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra" // register cassandra plugin
)

const (
	// semaphoreLWTRounds is how many fresh buckets each concurrency level fills.
	semaphoreLWTRounds = 4
	// semaphoreLWTBucketSize is the default bucket size (persistence.DefaultSemaphoreBucketSize).
	semaphoreLWTBucketSize = 100
	// semaphoreLWTTimeout is the deadline History gives each acquire.
	semaphoreLWTTimeout = 2 * time.Second
)

func TestSemaphoreCassandraLWT(t *testing.T) {
	testflags.RequireCassandra(t)

	// Replicas per row. With more than one, each conditional write runs Paxos across replicas.
	if replicationFactor := os.Getenv("SEMAPHORE_LWT_REPLICATION_FACTOR"); replicationFactor != "" {
		pt.DBSetupOptions["replication_factor"] = replicationFactor
	}
	t.Logf("Replication factor: %v", pt.DBSetupOptions["replication_factor"])

	dc := *persistence.NewDefaultDynamicConfiguration()
	testBase := pt.NewTestBase(t, pt.TestBaseParams{
		PersistenceConfig:    pt.SimplePersistenceConfig(t, public.NewTestConfigWithPublicCassandra),
		DynamicConfiguration: &dc,
	})
	testBase.Setup()
	defer testBase.TearDownWorkflowStore()

	tokens, err := testBase.PersistenceFactory.NewSemaphoreTokenManager()
	if err != nil {
		t.Fatalf("NewSemaphoreTokenManager: %v", err)
	}
	metadata, err := testBase.PersistenceFactory.NewSemaphoreMetadataManager()
	if err != nil {
		t.Fatalf("NewSemaphoreMetadataManager: %v", err)
	}

	for _, concurrency := range []int{1, 10, 100} {
		t.Logf("%v", runSemaphoreAcquires(t, tokens, metadata, concurrency))
	}
}

// runSemaphoreAcquires fills semaphoreLWTRounds fresh buckets of semaphoreLWTBucketSize tokens
// through a bucket manager on the real token store, with acquires from concurrency goroutines at once.
func runSemaphoreAcquires(
	t *testing.T,
	tokens persistence.SemaphoreTokenManager,
	metadata persistence.SemaphoreMetadataManager,
	concurrency int,
) *lwtStats {
	stats := &lwtStats{concurrency: concurrency}
	counting := &inFlightTokenManager{SemaphoreTokenManager: tokens}
	for range semaphoreLWTRounds {
		domainID, semaphoreName := uuid.New(), "sem-"+uuid.New()
		if _, err := metadata.CreateSemaphore(context.Background(), &persistence.CreateSemaphoreRequest{
			DomainID:      domainID,
			SemaphoreName: semaphoreName,
			Size:          semaphoreLWTBucketSize,
			BucketSize:    semaphoreLWTBucketSize,
		}); err != nil {
			t.Fatalf("CreateSemaphore: %v", err)
		}
		mgr, err := semaphore.NewManager(semaphore.ManagerParams{
			ID:       semaphore.Identifier{DomainID: domainID, SemaphoreName: semaphoreName, Bucket: 0},
			Tokens:   counting,
			Metadata: metadata,
			Logger:   log.NewNoop(),
			IdleTTL:  time.Hour,
			OnStopFn: func(semaphore.Manager) {},
			// The clock never moves, so the bucket never unloads mid-run.
			TimeSource: clock.NewMockedTimeSource(),
		})
		if err != nil {
			t.Fatalf("NewManager: %v", err)
		}
		if err := mgr.Start(context.Background()); err != nil {
			t.Fatalf("Start: %v", err)
		}

		owners := make(chan string, semaphoreLWTBucketSize)
		for i := range semaphoreLWTBucketSize {
			owners <- fmt.Sprintf("wid-%d:rid:1", i)
		}
		close(owners)

		start := time.Now()
		var wg sync.WaitGroup
		for range concurrency {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for owner := range owners {
					ctx, cancel := context.WithTimeout(context.Background(), semaphoreLWTTimeout)
					callStart := time.Now()
					res, err := mgr.Acquire(ctx, owner)
					cancel()
					stats.record(time.Since(callStart), acquireResult(res, err))
				}
			}()
		}
		wg.Wait()
		stats.elapsed += time.Since(start)
		mgr.Stop()
	}
	stats.maxInFlight = counting.maxInFlight.Load()
	stats.writeLatencies = counting.writeLatencies
	return stats
}

const (
	resultApplied  = "applied"
	resultNoSlot   = "no-slot"
	resultTimeout  = "timeout"
	resultDeadline = "deadline"
	resultOther    = "other-error"
)

func acquireResult(res semaphore.AcquireResult, err error) string {
	switch {
	case isStoreTimeout(err):
		return resultTimeout
	case errors.Is(err, context.DeadlineExceeded):
		return resultDeadline
	case err != nil:
		return resultOther
	case res.Outcome == types.SemaphoreAcquireOutcomeAcquired:
		return resultApplied
	case res.Outcome == types.SemaphoreAcquireOutcomeNoSlot:
		return resultNoSlot
	default:
		return resultOther
	}
}

// isStoreTimeout reports a write that timed out in the store, so its outcome is unknown. A call
// whose deadline passed before it sent a write is counted as "deadline" instead.
func isStoreTimeout(err error) bool {
	var timeoutErr *persistence.TimeoutError
	return errors.As(err, &timeoutErr)
}

// inFlightTokenManager counts grant writes sent to the store and not yet answered, and records how
// long each write took.
type inFlightTokenManager struct {
	persistence.SemaphoreTokenManager
	inFlight    atomic.Int64
	maxInFlight atomic.Int64

	mu             sync.Mutex
	writeLatencies []time.Duration
}

func (m *inFlightTokenManager) GrantSemaphoreToken(ctx context.Context, request *persistence.GrantSemaphoreTokenRequest) (*persistence.GrantSemaphoreTokenResponse, error) {
	n := m.inFlight.Add(1)
	defer m.inFlight.Add(-1)
	for {
		seen := m.maxInFlight.Load()
		if n <= seen || m.maxInFlight.CompareAndSwap(seen, n) {
			break
		}
	}
	start := time.Now()
	resp, err := m.SemaphoreTokenManager.GrantSemaphoreToken(ctx, request)
	m.mu.Lock()
	m.writeLatencies = append(m.writeLatencies, time.Since(start))
	m.mu.Unlock()
	return resp, err
}

// lwtStats collects the results and latencies of one concurrency level.
type lwtStats struct {
	concurrency int
	elapsed     time.Duration
	// The Cassandra writes inside the acquires.
	maxInFlight    int64
	writeLatencies []time.Duration

	mu        sync.Mutex
	results   map[string]int
	latencies []time.Duration
}

func (s *lwtStats) record(latency time.Duration, result string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.results == nil {
		s.results = make(map[string]int)
	}
	s.results[result]++
	s.latencies = append(s.latencies, latency)
}

func (s *lwtStats) String() string {
	n := len(s.latencies)
	success := s.results[resultApplied]
	out := fmt.Sprintf("Total grants: %d, concurrency: %d, success: %d, failed: %d, grants/s: %.0f, acquire p99: %v",
		n, s.concurrency, success, n-success, float64(success)/s.elapsed.Seconds(), p99(s.latencies))
	for _, result := range []string{resultNoSlot, resultTimeout, resultDeadline, resultOther} {
		if count := s.results[result]; count > 0 {
			out += fmt.Sprintf(", %s: %d", result, count)
		}
	}
	return out + fmt.Sprintf(", write p99: %v, max writes in flight: %d", p99(s.writeLatencies), s.maxInFlight)
}

// p99 returns the duration that 99% of latencies are within, or 0 if there are none.
func p99(latencies []time.Duration) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	slices.Sort(latencies)
	return latencies[min(len(latencies)*99/100, len(latencies)-1)].Round(10 * time.Microsecond)
}
