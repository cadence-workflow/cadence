# Semaphore Cassandra LWT results

Results of the tests in `semaphore_cassandra_lwt_test.go` and `semaphore_cassandra_lwt_retry_test.go`, run 2026-10-29.

Setup for every run: two Cassandra nodes, 2 replicas, on a laptop. One bucket of 100 tokens, which is one partition of `semaphore_tokens`. Each concurrency level fills 4 fresh buckets, one after another.

- **concurrency:** callers asking for a token at the same time.
- **timeout:** Cassandra timed out the write, so its outcome is unknown. (`deadline`, a call whose 2 s deadline passed before it sent a write, never happened in these runs.)
- **Lock designs compared:**
  - Bucket lock: master at 292f8c73e. Callers wait for the bucket lock in Matching, so Cassandra sees one write at a time.
  - Per-token locks: #8691 at e28bd40d9. Each caller locks only its token, so all callers' writes go to Cassandra at once.

## Acquires (`TestSemaphoreCassandraLWT`)

400 acquires per line, all from new owners. `acquire p99` is a whole Acquire call; `write p99` is only the Cassandra write inside it.

### Bucket lock, two runs

```
Total grants: 400, concurrency: 1, success: 400, failed: 0, grants/s: 345, acquire p99: 4.82ms, write p99: 4.81ms, max writes in flight: 1
Total grants: 400, concurrency: 10, success: 400, failed: 0, grants/s: 358, acquire p99: 35.99ms, write p99: 3.86ms, max writes in flight: 1
Total grants: 400, concurrency: 100, success: 400, failed: 0, grants/s: 363, acquire p99: 273.76ms, write p99: 3.24ms, max writes in flight: 1

Total grants: 400, concurrency: 1, success: 400, failed: 0, grants/s: 351, acquire p99: 3.77ms, write p99: 3.77ms, max writes in flight: 1
Total grants: 400, concurrency: 10, success: 400, failed: 0, grants/s: 356, acquire p99: 38.85ms, write p99: 3.88ms, max writes in flight: 1
Total grants: 400, concurrency: 100, success: 400, failed: 0, grants/s: 361, acquire p99: 278.7ms, write p99: 3.35ms, max writes in flight: 1
```

### Per-token locks, two runs

```
Total grants: 400, concurrency: 1, success: 400, failed: 0, grants/s: 355, acquire p99: 3.5ms, write p99: 3.49ms, max writes in flight: 1
Total grants: 400, concurrency: 10, success: 400, failed: 0, grants/s: 406, acquire p99: 167.83ms, write p99: 167.83ms, max writes in flight: 10
Total grants: 400, concurrency: 100, success: 399, failed: 1, grants/s: 317, acquire p99: 293.35ms, timeout: 1, write p99: 293.34ms, max writes in flight: 100

Total grants: 400, concurrency: 1, success: 400, failed: 0, grants/s: 362, acquire p99: 3.26ms, write p99: 3.26ms, max writes in flight: 1
Total grants: 400, concurrency: 10, success: 398, failed: 2, grants/s: 408, acquire p99: 182.56ms, timeout: 2, write p99: 177.37ms, max writes in flight: 10
Total grants: 400, concurrency: 100, success: 397, failed: 3, grants/s: 321, acquire p99: 286.37ms, timeout: 3, write p99: 286.37ms, max writes in flight: 100
```

## Retries while new acquires write (`TestSemaphoreCassandraLWTRetry`)

200 new acquires and 200 retries per line, interleaved. A retry comes from an owner that already holds a token and needs only a read to confirm it (`read p99`), so `retry p99` minus `read p99` is roughly time spent waiting for a lock. Each round has 50 new acquires, so at most 50 writes can be in flight.

### Bucket lock, two runs

Retries wait in the same line as new acquires.

```
concurrency: 1, new acquires: 200, retries: 200, failed: 0, new acquire p99: 4.86ms, retry p99: 4.84ms, write p99: 4.85ms, read p99: 4.84ms, max writes in flight: 1
concurrency: 10, new acquires: 200, retries: 200, failed: 0, new acquire p99: 42.11ms, retry p99: 21.71ms, write p99: 4.7ms, read p99: 2.24ms, max writes in flight: 1
concurrency: 100, new acquires: 200, retries: 200, failed: 0, new acquire p99: 247.52ms, retry p99: 92.92ms, write p99: 3.84ms, read p99: 2.03ms, max writes in flight: 1

concurrency: 1, new acquires: 200, retries: 200, failed: 0, new acquire p99: 3.23ms, retry p99: 2.2ms, write p99: 3.23ms, read p99: 2.2ms, max writes in flight: 1
concurrency: 10, new acquires: 200, retries: 200, failed: 0, new acquire p99: 33.49ms, retry p99: 18.1ms, write p99: 3.22ms, read p99: 2.12ms, max writes in flight: 1
concurrency: 100, new acquires: 200, retries: 200, failed: 0, new acquire p99: 230.32ms, retry p99: 90.19ms, write p99: 4.18ms, read p99: 2.19ms, max writes in flight: 1
```

### Per-token locks, two runs

A retry locks only its own token.

```
concurrency: 1, new acquires: 200, retries: 200, failed: 0, new acquire p99: 3.51ms, retry p99: 4.58ms, write p99: 3.45ms, read p99: 4.58ms, max writes in flight: 1
concurrency: 10, new acquires: 200, retries: 200, failed: 1, new acquire p99: 167.74ms, retry p99: 3.47ms, write p99: 166.58ms, read p99: 3.44ms, max writes in flight: 10, timeout: 1
concurrency: 100, new acquires: 200, retries: 200, failed: 1, new acquire p99: 227.02ms, retry p99: 13.92ms, write p99: 227.01ms, read p99: 13.91ms, max writes in flight: 50, timeout: 1

concurrency: 1, new acquires: 200, retries: 200, failed: 0, new acquire p99: 3.59ms, retry p99: 2.39ms, write p99: 3.58ms, read p99: 2.39ms, max writes in flight: 1
concurrency: 10, new acquires: 200, retries: 200, failed: 0, new acquire p99: 163.45ms, retry p99: 2.57ms, write p99: 163.44ms, read p99: 2.56ms, max writes in flight: 10
concurrency: 100, new acquires: 200, retries: 200, failed: 1, new acquire p99: 193.76ms, retry p99: 3.15ms, write p99: 193.75ms, read p99: 3.15ms, max writes in flight: 50, timeout: 1
```

## Analysis

- **Parallel writes on one partition barely change throughput.** Bucket lock: 345-363 grants/s at every concurrency. Per-token locks: 406-408 at concurrency 10 (about 13% more) but 317-321 at concurrency 100 (about 12% less).
- **New acquires wait about as long on both designs, in different places.**
  - Bucket lock: in Matching. At concurrency 100, acquire p99 is 274-279 ms while each write stays at 3-4 ms.
  - Per-token locks: in Cassandra. At concurrency 100, acquire p99 and write p99 are both 286-293 ms.
- **Only per-token locks cause failures.** 0-3 timeouts per line at concurrency 10 and 100, in both tests. A timeout leaves the write's outcome unknown, so the token may or may not be granted. The bucket lock had no failures in any run.
- **With the bucket lock, retries wait behind writes.** A retry needs only a read (about 2 ms), but its p99 is 18-22 ms at concurrency 10 and 90-93 ms at 100, because master's single bucket lock is held across both the grant write and the retry's confirming read (`findHeldToken`). With per-token locks a retry locks only its own token, so its p99 stays at 3-14 ms.
- **Absolute numbers vary between sessions on the same laptop.** An earlier session the same day got 430-460 grants/s on both designs. The comparison between designs held in every run.

## Recommendation

- **Use a bucket lock for writes.** Same throughput as per-token locks within about 13%, no timeouts, and the waiting stays in Matching instead of in Cassandra. History does the same: the shard lock is held across the conditional write to the executions table. It is also simpler than per-token locks: one lock order, and no per-token lock handles.
- **Don't make retries wait for the bucket lock.** Split the lock in two: the bucket lock held across every write, and a cache lock that guards only the in-memory cache and is never held across a Cassandra call. A retry looks up its token under the cache lock, lets go, reads the token's row with no lock held, and answers from it without writing the cache. Details and every case: `matching_bucket_locking_details.md`, section 5.
- **Run the retry test again once this is built.** Retry p99 should drop close to read p99, as it does with per-token locks.

## Limits

- A laptop with 2 nodes and 2 replicas, not production hardware or replica count. Absolute numbers will differ.
- One bucket only. A semaphore with several buckets spreads writes across partitions, so total throughput grows with the number of buckets. Not measured here.
- With a bucket lock, one bucket's queue sets the latency. At about 360 grants/s, roughly 700 callers waiting on one bucket would reach the 2 s acquire deadline. If one bucket gets that busy, use more buckets (a smaller bucket size).
- Release isn't compared. It isn't built on master yet.
