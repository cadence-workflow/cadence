//go:build !race && cassandralwt
// +build !race,cassandralwt

/*
Measures how long a retried acquire waits while new acquires write to the same bucket. A retry comes
from an owner that already holds a token, e.g. History retrying after a timeout. It needs no write,
only a read to confirm the token, so it shows whether reads wait behind writes.

Each round fills half of a fresh bucket first, then sends a mix of new acquires and retries from the
owners holding that half, from concurrency goroutines at once.

Run it with the steps in semaphore_cassandra_lwt_test.go, which run both tests. To run only this
one, use -run ^TestSemaphoreCassandraLWTRetry$.

Results: semaphore_cassandra_lwt_results.md.
*/

package host

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
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
)

func TestSemaphoreCassandraLWTRetry(t *testing.T) {
	testflags.RequireCassandra(t)

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
		t.Logf("%v", runSemaphoreRetries(t, tokens, metadata, concurrency))
	}
}

// runSemaphoreRetries fills semaphoreLWTRounds fresh buckets. In each, the first half of the tokens
// is granted before timing starts. Then concurrency goroutines send, interleaved, a new acquire for
// each free token and a retried acquire for each owner holding one.
func runSemaphoreRetries(
	t *testing.T,
	tokens persistence.SemaphoreTokenManager,
	metadata persistence.SemaphoreMetadataManager,
	concurrency int,
) *retryStats {
	stats := &retryStats{concurrency: concurrency}
	counting := &readTimingTokenManager{inFlightTokenManager: &inFlightTokenManager{SemaphoreTokenManager: tokens}}
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

		// Grant the first half before timing starts, so those owners can retry.
		heldTokens := make(map[string]int)
		for i := range semaphoreLWTBucketSize / 2 {
			owner := fmt.Sprintf("held-%d:rid:1", i)
			res, err := mgr.Acquire(context.Background(), owner)
			if err != nil || res.Outcome != types.SemaphoreAcquireOutcomeAcquired {
				t.Fatalf("setup acquire for %v: %v, %v", owner, res.Outcome, err)
			}
			heldTokens[owner] = res.TokenID
		}
		counting.reset()

		jobs := make(chan retryJob, semaphoreLWTBucketSize)
		for i := range semaphoreLWTBucketSize / 2 {
			jobs <- retryJob{owner: fmt.Sprintf("new-%d:rid:1", i)}
			owner := fmt.Sprintf("held-%d:rid:1", i)
			jobs <- retryJob{owner: owner, retry: true, heldToken: heldTokens[owner]}
		}
		close(jobs)

		var wg sync.WaitGroup
		for range concurrency {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for job := range jobs {
					ctx, cancel := context.WithTimeout(context.Background(), semaphoreLWTTimeout)
					callStart := time.Now()
					res, err := mgr.Acquire(ctx, job.owner)
					cancel()
					stats.record(job, time.Since(callStart), res, err)
				}
			}()
		}
		wg.Wait()
		mgr.Stop()
		stats.addStoreCalls(counting)
	}
	return stats
}

type retryJob struct {
	owner     string
	retry     bool
	heldToken int
}

// readTimingTokenManager also records how long each token-row read took. A retry confirms its
// token with this read.
type readTimingTokenManager struct {
	*inFlightTokenManager

	readMu        sync.Mutex
	readLatencies []time.Duration
}

func (m *readTimingTokenManager) GetSemaphoreOwnershipByToken(ctx context.Context, request *persistence.GetSemaphoreOwnershipByTokenRequest) (*persistence.GetSemaphoreOwnershipByTokenResponse, error) {
	start := time.Now()
	resp, err := m.inFlightTokenManager.GetSemaphoreOwnershipByToken(ctx, request)
	m.readMu.Lock()
	m.readLatencies = append(m.readLatencies, time.Since(start))
	m.readMu.Unlock()
	return resp, err
}

// reset drops what the setup grants recorded.
func (m *readTimingTokenManager) reset() {
	m.mu.Lock()
	m.writeLatencies = nil
	m.mu.Unlock()
	m.readMu.Lock()
	m.readLatencies = nil
	m.readMu.Unlock()
	m.maxInFlight.Store(0)
}

// retryStats collects new acquires and retries separately, for one concurrency level.
type retryStats struct {
	concurrency int

	mu                sync.Mutex
	newLatencies      []time.Duration
	retryLatencies    []time.Duration
	failed            int
	timeouts          int
	deadlines         int
	wrongRetryToken   int
	writeLatencies    []time.Duration
	readLatencies     []time.Duration
	maxWritesInFlight int64
}

func (s *retryStats) record(job retryJob, latency time.Duration, res semaphore.AcquireResult, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job.retry {
		s.retryLatencies = append(s.retryLatencies, latency)
	} else {
		s.newLatencies = append(s.newLatencies, latency)
	}
	switch {
	case isStoreTimeout(err):
		s.failed++
		s.timeouts++
	case errors.Is(err, context.DeadlineExceeded):
		s.failed++
		s.deadlines++
	case err != nil || res.Outcome != types.SemaphoreAcquireOutcomeAcquired:
		s.failed++
	case job.retry && res.TokenID != job.heldToken:
		s.wrongRetryToken++
	}
}

func (s *retryStats) addStoreCalls(m *readTimingTokenManager) {
	m.mu.Lock()
	s.writeLatencies = append(s.writeLatencies, m.writeLatencies...)
	m.writeLatencies = nil
	m.mu.Unlock()
	m.readMu.Lock()
	s.readLatencies = append(s.readLatencies, m.readLatencies...)
	m.readLatencies = nil
	m.readMu.Unlock()
	s.maxWritesInFlight = max(s.maxWritesInFlight, m.maxInFlight.Load())
}

func (s *retryStats) String() string {
	out := fmt.Sprintf("concurrency: %d, new acquires: %d, retries: %d, failed: %d, new acquire p99: %v, retry p99: %v, write p99: %v, read p99: %v, max writes in flight: %d",
		s.concurrency, len(s.newLatencies), len(s.retryLatencies), s.failed,
		p99(s.newLatencies), p99(s.retryLatencies), p99(s.writeLatencies), p99(s.readLatencies), s.maxWritesInFlight)
	if s.timeouts > 0 {
		out += fmt.Sprintf(", timeout: %d", s.timeouts)
	}
	if s.deadlines > 0 {
		out += fmt.Sprintf(", deadline: %d", s.deadlines)
	}
	if s.wrongRetryToken > 0 {
		out += fmt.Sprintf(", retry got a different token: %d", s.wrongRetryToken)
	}
	return out
}
