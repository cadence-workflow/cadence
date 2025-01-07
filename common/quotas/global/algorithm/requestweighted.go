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

/*
Package algorithm contains a running-weighted-average calculator for ratelimits,
and some associated types to prevent accidental confusion between the various
floats/ints/etc involved.

This package is intentionally unaware of RPC or any desired RPS limits, it just
tracks the observed rates of requests and returns proportions, so other code can
determine RPS values.

This is both to simplify testing and to keep its internals as fast as possible, as
it is relatively CPU heavy and needs to be quick to prevent a snowballing backlog
of concurrent updates and requests for aggregated data.

# How this fits in the global ratelimiting system

Going back to the [github.com/uber/cadence/common/quotas/global] package diagram:
each aggregating host will have one or more instances of this weight-calculator,
and it will receive a shard worth of request data.

Though this package has no direct connection to RPC structures, the request data is
expected to closely match the argument to RequestWeighted.Update, and the response
data like the return value of RequestWeighted.HostWeights.

This makes aggregating hosts intentionally very simple outside of this package:
they essentially just forward the request and response, multiplying by dynamicconfig
intended-ratelimit values (which occurs outside the mutex, to minimize contention).

# Expected use

Limiting hosts collect metrics and submit it to an aggregating host via
RequestWeighted.Update, through [some kind of RPC setup].

Once updated, the aggregating host can get the RequestWeighted.HostWeights for
that host's ratelimits (== the updated limits), multiply those 0..1 weights by
the dynamicconfig configured RPS, and return them to the limiting host that
triggered the request.

If there are unused RPS remaining, the aggregating host *may* increase the RPS
it returns by [some amount], to allow the limiting host to pre-emptively allow
more requests than is "fair" before the next update.  Even if it does not, an
increase in attempted usage will increase that limiter's weight on the next
update cycle, so this is mostly intended as a tool for reducing incorrectly-rejected
requests when a ratelimit's usage is well below its allowed limit.

# Dealing with expired data

As user calls change, or the aggregating-host ring changes, Limit keys may become
effectively unused in an instance.  During normal use, any accessed Limit will
clean up "expired" data if it is found, and there is essentially no ongoing
"upkeep" cost for an un-accessed Limit (aside from memory).

Hopefully this will be sufficient to keep memory use and performance reasonable.

If it is not, a trivial goroutine to periodically call RequestWeighted.GC will
clear *all* old data.  Every minute or so should be more than sufficient.
This method returns some simple metrics about how much data exists / was removed,
so it can be reported to help us estimate how necessary it is in practice.

# Dealing with excessive contention

In large clusters, there will be likely be many update-requests, many Limit keys,
and more data to process to return responses (more hosts per limit).  At some point
this could become a bottleneck, preventing timely updates.

On even our largest internal clusters, I do not believe we will run that risk.
At least, not with the planned frontend-only limits.  `pprof` should be able
to easily validate this and let us better estimate headroom for adding more in
the future.

If too much contention does occur, there are 3 relatively simple mitigations:
 1. turn it completely off, go back to host-local limits
 2. slow down the update frequency
 3. use N instances and shard keys across them

1 is pretty obvious and has clear behavior, though it means degrading behavior
for our imbalanced-load clusters.

2 is essentially the only short-term and dynamically-apply-able option that retains
any of the behavior we want.  This will impact how quickly the algorithm converges,
so you may also want to adjust the new-data weight to be higher, though "how much"
depends on what kind of behavior you want / can tolerate.

3 offers a linear contention improvement and should be basically trivial to build:
create N instances instead of 1, and shard the Limit keys to each instance.
Since this is mostly CPU bound and each one would be fully independent, making
`GOMAXPROCS` instances is an obvious first choice, and this does not need to be
dynamically reconfigured at runtime so there should be no need to build a "smart"
re-balancing / re-sharding system of any kind.

Tests contain a single-threaded benchmark and laptop-local results for estimating
contention, and for judging if changes will help or hinder, but real-world use and
different CPUs will of course be somewhat different.
Personally I expect "few mutexes, GOMAXPROCS instances" is roughly ideal for CPU
throughput with the current setup, but an entirely different internal structure
might exceed it.
*/
package algorithm

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"go.uber.org/multierr"
	"golang.org/x/exp/constraints"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/quotas/global/shared"
)

type (
	// requests holds the running per-second running average request data for a single key from a single host,
	// and the last time it was updated.
	requests struct {
		// TODO: add "first update" time, don't return to callers until it has passed 1(+?) update cycle as data is incomplete.

		lastUpdate         time.Time // only so we know if elapsed times are exceeded, not used to compute per-second rates
		accepted, rejected PerSecond // requests received, per second (conceptually divided by update rate)
	}

	// Identity is an arbitrary (stable) identifier for a "limiting" host.
	Identity string
	// Limit is the key being used to Limit requests.
	Limit string
	// PerSecond represents already-scaled-to-per-second RPS values
	PerSecond float64
	// HostWeight is a float between 0 and 1, showing how much of a ratelimit should be allocated to a host.
	HostWeight float64

	impl struct {
		// intentionally value-typed so caller cannot mutate the fields.
		// manually copy data if this changes.
		cfg    Config
		scope  metrics.Scope
		logger log.Logger

		// mut protects usage, as it is the only mutable data
		mut sync.Mutex
		// usage data for ratelimits, per host.
		//
		// data is first keyed on the limit and then on the host, as it's assumed that there will be
		// many times more limit keys than limiting hosts, and this will reduce cardinality more rapidly.
		usage map[Limit]map[Identity]requests

		clock clock.TimeSource
	}

	// Metrics reports overall counts discovered as part of garbage collection.
	//
	// This is not necessarily worth maintaining verbatim, but while it's easy to
	// collect it might give us some insight into overall behavior.
	Metrics struct {
		// HostLimits is the number of per-host limits that remain after cleanup
		HostLimits int
		// Limits is the number of cross-host limits that remain after cleanup
		Limits int

		// RemovedHostLimits is the number of per-host limits that were removed as part of this GC pass
		RemovedHostLimits int
		// RemovedLimits is the number of cross-host limits that were removed as part of this GC pass
		RemovedLimits int
	}

	// Requests contains accepted/rejected request counts for a ratelimit, as part of an update operation.
	// Only intended to be used in [RequestWeighted.Update].
	Requests struct {
		Accepted, Rejected int
	}

	// RequestWeighted returns aggregated RPS and distribution data for its Limit keys,
	// based on how many requests each Identity has received.
	RequestWeighted interface {
		// Update load-data for this host's requests, given a known elapsed time spent accumulating this load info.
		//
		// Elapsed time must be non-zero, but is not otherwise constrained.
		Update(params UpdateParams) error

		// HostUsage returns the per-[Limit] weights for all requested + known keys for this Identity,
		// as well as the Limit's overall used RPS (to decide RPS to allow for new hosts).
		HostUsage(host Identity, limits []Limit) (weights map[Limit]HostUsage, err error)

		// GC can be called periodically to pre-emptively prune old ratelimits.
		//
		// Limit keys that are accessed normally will automatically garbage-collect themselves and old host data,
		// and an unused limit only costs memory, so this may prove to be unnecessary.
		GC() (Metrics, error)
	}

	Config struct {
		// How much each update should be weighted vs prior data.
		// Must be between 0 and 1, recommend starting with 0.5 (4 updates until data has <10% influence)
		NewDataWeight dynamicconfig.FloatPropertyFn

		// Expected time between updates.  Should match the cluster's config for how often limiters check in,
		// i.e. this should probably be the same dynamic config value, updated at / near the same time.
		UpdateInterval dynamicconfig.DurationPropertyFn

		// How long to wait before considering a host-limit's RPS usage "probably inactive", rather than
		// simply delayed.
		//
		// Should always be larger than UpdateInterval, as less is meaningless.  Values are reduced based on
		// missed UpdateInterval multiples, not DecayAfter.
		// Unsure about a good default (try 2x UpdateInterval?), but larger numbers mean smoother behavior
		// but longer delays on adjusting to hosts that have disappeared or stopped receiving requests.
		DecayAfter dynamicconfig.DurationPropertyFn

		// How much time can pass without receiving any update before completely deleting data.
		//
		// Due to ever-reducing weight after DecayAfter, this is intended to reduce calculation costs,
		// not influence behavior / weights returned.  Even extremely-low-weighted hosts will still be retained
		// as long as they keep reporting "in use" (e.g. 1 rps used out of >100,000 is fine and will be tracked).
		//
		// "Good" values depend on a lot of details, but >=10*UpdateInterval seems reasonably safe for a
		// NewDataWeight of 0.5, as the latest data will be reduced to only 0.1% and may not be worth keeping.
		GcAfter dynamicconfig.DurationPropertyFn
	}

	// UpdateParams contains args for calling Update.
	UpdateParams struct {
		ID      Identity
		Load    map[Limit]Requests
		Elapsed time.Duration
	}

	HostUsage struct {
		Weight HostWeight
		Used   PerSecond
	}

	// configSnapshot holds a non-changing snapshot of the dynamic config values,
	// and also provides a validate() method to make sure they're sane.
	configSnapshot struct {
		now        time.Time
		weight     float64
		rate       time.Duration
		decayAfter time.Duration
		gcAfter    time.Duration
	}
)

// these could be configurable, but it's not expected to be a noticeable performance concern.
const (
	guessNumKeys   = 1024 // guesstimate at num of ratelimit keys in a cluster
	guessHostCount = 32   // guesstimate at num of frontend hosts in a cluster that receive traffic for each key

	// Some event-RPSes exceed 100k, but none get close to 1m.
	// So, to add some buffer, 10m per second is considered "impossible".
	// This is quite vague and can be changed, it essentially just serves as the logging threshold.
	guessImpossibleRps = 10_000_000
)

func (p UpdateParams) Validate() error {
	if len(p.ID) == 0 {
		return fmt.Errorf("empty caller ID")
	}
	if p.Elapsed <= 0 {
		return fmt.Errorf("elapsed time must be positive, got %v", p.Elapsed)
	}
	return nil
}

func (c configSnapshot) validate() error {
	// errors are untyped because they should not generally be "handled", only returned.
	// in principle, they're all "bad server config" / 5XX and if sustained will eventually lead to
	// the limiting hosts using their fallback limiters.

	var err error
	if c.weight < 0 {
		// nonsensical
		err = multierr.Append(err, fmt.Errorf("new data weight cannot be negative: %f", c.weight))
	}
	if c.rate <= 0 {
		// rate is used in division, absolutely must not be zero.
		// and negatives would just be weird enough to consider an error too.
		err = multierr.Append(err, fmt.Errorf("update rate must be positive: %v", c.rate))
	}

	// decayAfter and gcAfter should always be larger than rate, but currently no logic misbehaves
	// if this is not true, so it is not checked.  this allows e.g. temporary weird mid-rollout combinations
	// without breaking.
	// negative values cannot ever be correct though, so block them.
	if c.decayAfter < 0 {
		err = multierr.Append(err, fmt.Errorf("decay-after cannot be negative: %v", c.decayAfter))
	}
	if c.gcAfter < 0 {
		err = multierr.Append(err, fmt.Errorf("gc-after cannot be negative: %v", c.decayAfter))
	}

	if err != nil {
		return multierr.Append(errors.New("bad ratelimiter config"), err)
	}

	return nil
}

// shouldGC returns true if data should be garbage collected
func (c configSnapshot) shouldGC(dataAge time.Duration) bool {
	return dataAge >= c.gcAfter
}

// missedUpdateScalar returns an amount to multiply old RPS data by, to account for missed updates outside SLA.
func (c configSnapshot) missedUpdateScalar(dataAge time.Duration) PerSecond {
	if dataAge < c.decayAfter {
		// new enough to not decay old data
		return 1
	}
	// fast check as exponents are slow and computing past this is unnecessary.
	// normally, a `c.shouldGC(dataAge)` check should make this unused.
	if dataAge >= c.gcAfter {
		// old enough to treat old data as nonexistent
		return 0
	}

	// somewhere in the middle: account for missed updates by simulating 0-value updates.
	// this intentionally floors rather than allowing fractions because:
	// - precision isn't important
	// - tests are a bit easier (more stable / less crazy-looking values)
	// - as a bonus freebie: int division and int exponents are typically faster to compute
	missed := dataAge / c.rate
	if missed < 1 {
		// note: this path should never be used, it's effectively error handling.
		//
		// this can only be true if cfg.decayAfter is smaller than cfg.rate, which
		// should not occur in practice as that would be a nonsensical config combination.
		//
		// the types cannot *prevent* it though, so it must be handled, and there's a
		// reasonable response if it does occur:
		// don't reduce old data.
		return 1
	}

	// missed at least one full update period, calculate an exponential decay multiplier for the old values.
	return PerSecond(math.Pow(1-c.weight, math.Floor(float64(missed))))
}

// New returns a concurrency-safe host-weight aggregator.
//
// This instance is effectively single-threaded, but a small sharding wrapper should allow better concurrent
// throughput if needed (bound by CPU cores, as it's moderately CPU-costly).
func New(met metrics.Client, logger log.Logger, cfg Config) (RequestWeighted, error) {
	i := &impl{
		cfg:    cfg,
		scope:  met.Scope(metrics.GlobalRatelimiterAggregator),
		logger: logger.WithTags(tag.ComponentGlobalRatelimiter),
		usage:  make(map[Limit]map[Identity]requests, guessNumKeys), // start out relatively large

		clock: clock.NewRealTimeSource(),
	}
	_, err := i.snapshot() // validate config by just taking a snapshot
	if err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return i, nil
}

// Update performs a weighted update to the running RPS for this host's per-key request data
func (a *impl) Update(p UpdateParams) error {
	if err := p.Validate(); err != nil {
		return fmt.Errorf("bad args to update: %w", err)
	}
	a.mut.Lock()
	once := newOnce()
	defer once.Do(a.mut.Unlock)

	var initialized, reinitialized, updated, decayed int64

	snap, err := a.snapshot()
	if err != nil {
		return err
	}
	for key, req := range p.Load {
		ih := a.usage[key]
		if ih == nil {
			ih = make(map[Identity]requests, guessHostCount)
		}

		var next requests
		prev := ih[p.ID]

		// sanity check: elapsed time should not be less than 1s, so just cap it.
		// in practice this should always be safe with a >=1s configured rate, as
		// the caller should not send *more* frequently than every 1s (monotonic time).
		//
		// but this is rather easy to trigger in tests / fuzzing,
		// and extreme values lead to irrational math either way.
		elapsed := math.Max(float64(p.Elapsed), float64(time.Second))
		aps := shared.SanityLogFloat(0, PerSecond(float64(req.Accepted)/(elapsed/float64(time.Second))), guessImpossibleRps, "accepted rps", a.logger)
		rps := shared.SanityLogFloat(0, PerSecond(float64(req.Rejected)/(elapsed/float64(time.Second))), guessImpossibleRps, "rejected rps", a.logger)

		// zeros are not worth recording, and this also simplifies math elsewhere
		// for two major reasons:
		// - it prevents some divide-by-zero scenarios by simply not having actual zeros
		// - it prevents weights from perpetually lowering if zeros are repeatedly sent, where they may eventually reach zero
		//
		// these keys will eventually gc, just leave them alone until that happens.
		// currently this gc relies on the assumption that HostUsage will be called with the same set of keys "soon",
		// but that is fairly easy to fix if needed.
		if rps+aps == 0 {
			continue
		}

		if prev.lastUpdate.IsZero() {
			initialized++
			next = requests{
				lastUpdate: snap.now,
				accepted:   aps, // no requests == 100% weight
				rejected:   rps, // no requests == 100% weight
			}
		} else {
			age := snap.now.Sub(prev.lastUpdate)
			if snap.shouldGC(age) {
				reinitialized++
				// would have GC'd if we had seen it earlier, so it's the same as the zero state
				next = requests{
					lastUpdate: snap.now,
					accepted:   aps, // no requests == 100% weight
					rejected:   rps, // no requests == 100% weight
				}
			} else {
				updated++
				// compute the next rolling average step (`*reduce` simulates skipped updates)
				reduce := snap.missedUpdateScalar(age)
				if reduce < 1 {
					decayed++
				}
				next = requests{
					lastUpdate: snap.now,
					// TODO: max(1, actual) so this does not lead to <1 rps allowances?  or maybe just 1+actual and then reduce in used-responses?
					// otherwise currently this may lead to rare callers getting 0.0001 rps,
					// and never recovering, despite steady and fair usage.
					accepted: shared.SanityLogFloat(0, weighted(aps, prev.accepted*reduce, snap.weight), guessImpossibleRps, "weighted accepted rps", a.logger),
					rejected: shared.SanityLogFloat(0, weighted(rps, prev.rejected*reduce, snap.weight), guessImpossibleRps, "weighted rejected rps", a.logger),
				}
			}
		}

		ih[p.ID] = next
		a.usage[key] = ih
	}

	once.Do(a.mut.Unlock) // don't hold the lock while emitting metrics

	a.scope.RecordHistogramValue(metrics.GlobalRatelimiterInitialized, float64(initialized))
	a.scope.RecordHistogramValue(metrics.GlobalRatelimiterReinitialized, float64(reinitialized))
	a.scope.RecordHistogramValue(metrics.GlobalRatelimiterUpdated, float64(updated))
	a.scope.RecordHistogramValue(metrics.GlobalRatelimiterDecayed, float64(decayed))

	return nil
}

// getWeightsLocked returns the weights of observed hosts (based on ALL requests), and the total number of requests accepted per second.
func (a *impl) getWeightsLocked(key Limit, snap configSnapshot) (weights map[Identity]HostWeight, usedRPS PerSecond, met Metrics) {
	ir := a.usage[key]
	if len(ir) == 0 {
		return nil, 0, met
	}

	weights = make(map[Identity]HostWeight, len(ir))
	total := HostWeight(0.0)
	for id, reqs := range ir {
		// account for missed updates
		age := snap.now.Sub(reqs.lastUpdate)
		if snap.shouldGC(age) {
			// old, clean up
			delete(ir, id)
			met.RemovedHostLimits++
			continue
		}

		// should never be zero, `shouldGC` takes care of that.
		reduce := shared.SanityLogFloat(0, snap.missedUpdateScalar(age), 1, "missed update", a.logger)
		// similarly: should never be zero, accepted + rejected must be nonzero or they are not inserted.
		// this may be reduced to very low values, but still far from == 0.
		actual := HostWeight((reqs.accepted + reqs.rejected) * reduce)

		weights[id] = actual // populate with the reduced values so it doesn't have to be calculated again
		total += actual      // keep a running total to scale all values when done
		usedRPS += reqs.accepted * reduce
		met.HostLimits++
	}

	if len(ir) == 0 {
		// completely empty Limit, gc it as well
		delete(a.usage, key)
		met.RemovedLimits++
		return nil, 0, met
	}

	// zeros anywhere here should not be possible - they are prevented from being inserted,
	// and anything simply "losing weight" will only become "rather low", not zero,
	// before enough passes have occurred to garbage collect it.
	//
	// specifically, 1rps -> 0rps takes over 1,000 halving cycles, and a single 1rps event
	// during that will immediately re-set it above 0.5 and will need 1,000+ more cycles.
	//
	// if gc period / weight amount is set extreme enough this is "possible",
	// but we are unlikely to ever cause it.
	for id := range ir {
		// normalize by the total.
		// this also ensures all values are between 0 and 1 (inclusive),
		// though zero should be impossible.
		weights[id] = shared.SanityLogFloat(0, weights[id]/total, 1, "normalized weight", a.logger)
	}
	met.Limits = 1
	return weights, usedRPS, met
}

func (a *impl) HostUsage(host Identity, limits []Limit) (usage map[Limit]HostUsage, err error) {
	a.mut.Lock()
	once := newOnce()
	defer once.Do(a.mut.Unlock)

	snap, err := a.snapshot()
	if err != nil {
		return nil, err
	}

	var cumulative Metrics
	usage = make(map[Limit]HostUsage, len(limits))
	for _, lim := range limits {
		hosts, used, met := a.getWeightsLocked(lim, snap)

		cumulative.Limits += met.Limits // always 1 or 0
		cumulative.HostLimits += met.HostLimits
		cumulative.RemovedLimits += met.RemovedLimits // always 0 or 1 (opposite Limits)
		cumulative.RemovedHostLimits += met.RemovedHostLimits

		if len(hosts) > 0 {
			usage[lim] = HostUsage{
				// limit is known, has some usage on at least one host.
				// usage has an "upper limit" because it is only the accepted RPS, not all requests received.
				Used: shared.SanityLogFloat(0, used, guessImpossibleRps, "used rps", a.logger),
				// either known weight if there is info for this host, or zero if not.
				// zeros are interpreted as "unknown", the same as "not present".
				Weight: shared.SanityLogFloat(0, hosts[host], 1, "computed weight", a.logger),
			}
		}
	}

	once.Do(a.mut.Unlock) // don't hold the lock while emitting metrics

	a.scope.RecordHistogramValue(metrics.GlobalRatelimiterLimitsQueried, float64(cumulative.Limits))
	a.scope.RecordHistogramValue(metrics.GlobalRatelimiterHostLimitsQueried, float64(cumulative.HostLimits))
	a.scope.RecordHistogramValue(metrics.GlobalRatelimiterRemovedLimits, float64(cumulative.RemovedLimits))
	a.scope.RecordHistogramValue(metrics.GlobalRatelimiterRemovedHostLimits, float64(cumulative.RemovedHostLimits))

	return usage, nil
}

func (a *impl) GC() (Metrics, error) {
	a.mut.Lock()
	defer a.mut.Unlock()

	m := Metrics{}
	snap, err := a.snapshot()
	if err != nil {
		return Metrics{}, err
	}
	// TODO: too costly? can check the first-N% each time and it'll eventually visit all keys, demonstrated in tests.
	for lim, dat := range a.usage {
		for host, reqs := range dat {
			age := snap.now.Sub(reqs.lastUpdate)
			if snap.shouldGC(age) {
				// clean up stale host data within limits
				delete(dat, host)
				m.RemovedHostLimits++
			} else {
				m.HostLimits++
			}
		}

		// clean up stale limits
		if len(dat) == 0 {
			delete(a.usage, lim)
			m.RemovedLimits++
		} else {
			m.Limits++
		}
	}

	return m, nil
}

// returns a snapshot of config and "now" for easier chaining through calls, and reducing calls to
// non-trivial field types like dynamic config.
func (a *impl) snapshot() (configSnapshot, error) {
	snap := configSnapshot{
		now:        a.clock.Now(),
		weight:     a.cfg.NewDataWeight(),
		rate:       a.cfg.UpdateInterval(),
		decayAfter: a.cfg.DecayAfter(),
		gcAfter:    a.cfg.GcAfter(),
	}
	return snap, snap.validate()
}

func weighted[T numeric](newer, older T, weight float64) T {
	return T((float64(newer) * weight) +
		(float64(older) * (1 - weight)))
}

type numeric interface {
	constraints.Integer | constraints.Float
}

// non-sync version of sync.Once, for easier unlocking
type doOnce bool

func newOnce() *doOnce {
	value := doOnce(false)
	return &value
}

func (o *doOnce) Do(cb func()) {
	if *o == false {
		*o = true
		cb()
	}
}
