package structured

import "github.com/uber/cadence/common/metrics"

// OperationTags is an embeddable struct that provides a single "operation" tag,
// as we need it to be included almost everywhere.
type OperationTags struct {
	// Operation is our historical "the thing doing something" name, unchanged for consistency
	Operation string `tag:"operation"`
}

// DynamicOperationTags is like OperationTags, but it's intended for places
// where we're dynamically choosing the operation per metric.
//
// This is intentionally an "incomplete" metric struct, and using it requires
// some manual work to populate the tags when emitting metrics.
type DynamicOperationTags struct {
	// DynamicOperation has the same intent as OperationTags.Operation,
	// but is provided at runtime via WithOperation or WithOperationInt.
	DynamicOperation struct{} `tag:"operation"`
}

// WithOperation adds the "operation" tag
func (d DynamicOperationTags) WithOperation(operation string, addTo Tags) Tags {
	return addTo.With("operation", operation)
}

// WithOperationInt adds the "operation" tag via the given scopeDefinition's operation string.
func (d DynamicOperationTags) WithOperationInt(operation int, addTo Tags) Tags {
	return d.WithOperation(getOperationString(operation), addTo)
}

func getOperationString(op int) string {
	for _, serviceOps := range metrics.ScopeDefs {
		if serviceOp, ok := serviceOps[op]; ok {
			return serviceOp.GetOperationString() // unit test ensures this is unique
		}
	}
	return "unknown_operation_int" // should never happen
}
