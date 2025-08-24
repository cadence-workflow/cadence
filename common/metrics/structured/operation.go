package structured

import "github.com/uber/cadence/common/metrics"

//go:generate metricsgen

// OperationTags is an embeddable struct that provides a single "operation" tag.
// It cooperates in tag emission, but does not contain an emitter.
type OperationTags struct {
	Operation string `tag:"operation"` // our historical "the thing doing something" name, unchanged for consistency
}

// DynamicOperationTags is like OperationTags, but it's intended for places
// where we're dynamically choosing the operation per metric.
//
// This is intentionally an "incomplete" metric struct, and using it requires
// some manual work to populate the tags when emitting metrics.
//
// When this type is embedded, the generated code will require passing an
// operation int to GetTags and PutTags, so it cannot be forgotten.
// You are free to ignore this and use PutOperation instead, if you have the
// string already.
//
// skip:New skip:Convenience skip:PutTags skip:GetTags
type DynamicOperationTags struct {
	DynamicOperation struct{} `tag:"operation"` // same intent as OperationTags
}

// PutOperation sets the "operation" tag to the given string.
func (d DynamicOperationTags) PutOperation(op string, into DynamicTags) {
	into["operation"] = op
}

// PutTags is a custom implementation that requires an operation-metric int to
// be passed in, for safety.
func (d DynamicOperationTags) PutTags(operation int, into DynamicTags) {
	into["operation"] = getOperationString(operation)
}

// GetTags is a custom implementation that requires an operation-metric int to
// be passed in, for safety.
func (d DynamicOperationTags) GetTags(operation int) DynamicTags {
	tags := make(DynamicTags, d.NumTags())
	d.PutTags(operation, tags)
	return tags
}

func getOperationString(op int) string {
	for _, serviceOps := range metrics.ScopeDefs {
		if serviceOp, ok := serviceOps[op]; ok {
			return serviceOp.GetOperationString() // unit test ensures this is unique
		}
	}
	return "unknown_operation_int" // should never happen
}
