package metrics

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
	Shard int          `tag:"shard"`
	Type  TasklistType `tag:"tasklist_type" convert:".String()"`
	//                                      ^^^^^^^^^^^^^^^^^^
	//        has custom serialization  ---/
	//        doing it like this just simplifies using thrift enums
	//        or reusing convenient (and small/safe) types.
	//
	//        this is just called verbatim by the code generator, nothing fancy.

	Reserved struct{} `tag:"reserved"`
}
