package metrics

import (
	"fmt"
	"runtime"
	"time"
)

type TasklistType int

// types do not have to be simple, custom for-metrics serialization is pretty easy
func (t TasklistType) String() string {
	if t == 0 {
		return "decision"
	} else {
		return "activity"
	}
}

//go:generate metricsgen

// ShardTasklistTags contains the stable per-instance tags for task processors.
// It is not used directly to emit anything, just helps build up data incrementally.
type ShardTasklistTags struct {
	// embeds parent fields that it expects, because it's simple.
	// these embedded fields are likely always unique just because our common tags are like that,
	// but it's pretty easy to notice it if not (and easy to reliably lint).
	ServiceTags

	// other fields it needs are added too
	Shard      int          `tag:"shard"`
	OtherShard int          `tag:"other_shard" convert:"fmt.Sprintf(\"other-%v\", {{.}})"`
	Type       TasklistType `tag:"tasklist_type" convert:".String()"`
	//                                           ^^^^^^^^^^^^^^^^^^
	//                                       has custom serialization
	//
	// doing it like this just simplifies using thrift enums,
	// or reusing convenient (and small/safe) types.
	//
	// `.`-prefixed is just appended verbatim by the code generator, nothing fancy.

	// "reserve" a tag, providing an IDE hint that it's in use,
	// and saving space in the default tags-population for it.
	//
	// note that this is a waste of space unless any emitters of this struct's
	// tags also include this reserved tag, because the default generated code
	// is not capable of knowing how to do so.
	Reserved struct{} `tag:"reserved"`
}

func (s ShardTasklistTags) ProcessingLatency(emit Emitter, dur time.Duration) {
	s.Inc("name")
	tags := s.GetTags()
	// ^ tags is fully populated, and has room for the reserved tag,
	// but it must be populated separately because it's custom.
	//
	// note that anything embedding ServiceTags must do this by hand as well,
	// so these should generally be reserved for "leaf" metrics, not intermediates,
	// or you should customize `Tags` to take care of that.
	tags["reserved"] = fmt.Sprintf("custom-%d", runtime.GOMAXPROCS(0))

	// emit the [whatever] for this ProcessingLatency event.
	//
	// in this case it's a histogram, with the name inline because:
	// 1) it's searchable and readable, and 2) it's used in a single location, no need to share a const.
	emit.Histogram(DynamicTags(tags), "processing_latency_per_shard_and_tasklist", dur)
	// and this also dual-emits a rollup with less info.
	// for convenience this just uses some parent tags directly, because they have everything needed,
	// and I expect that to be relatively common (since that's how most of our rollups work).
	emit.Histogram(s.ServiceTags, "processing_latency", dur)
}
