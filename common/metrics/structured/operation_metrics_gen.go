package structured

// Code generated ./internal/tools/metricsgen; DO NOT EDIT

// NewOperationTags constructs a new metric-tag-holding OperationTags,
// and it must be used, instead of custom initialization to ensure newly added
// tags can be detected at compile time instead of missing them at run time.
func NewOperationTags(
	Operation string,
) OperationTags {
	o := OperationTags{
		Operation: Operation,
	}
	return o
}

// NumTags returns the number of tags that are intended to be written in all
// cases.  This will include all embedded parent tags and all reserved tags,
// and is intended to be used to pre-allocate maps of tags.
func (o OperationTags) NumTags() int {
	num := 1 // num of self fields
	num += 0 // num of reserved fields
	return num
}

// PutTags writes this set of tags (and its embedded parents) to the passed map.
func (o OperationTags) PutTags(into DynamicTags) {
	into["operation"] = o.Operation
}

// GetTags is a minor helper to get a pre-allocated-and-filled map with room
// for reserved fields (i.e. 'struct{}' type fields, which only declare intent).
func (o OperationTags) GetTags() DynamicTags {
	tags := make(DynamicTags, o.NumTags())
	o.PutTags(tags)
	return tags
}

// NumTags returns the number of tags that are intended to be written in all
// cases.  This will include all embedded parent tags and all reserved tags,
// and is intended to be used to pre-allocate maps of tags.
func (d DynamicOperationTags) NumTags() int {
	num := 0 // num of self fields
	num += 1 // num of reserved fields
	return num
}
