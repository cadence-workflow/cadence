package structured

//go:generate metricsgen

// OperationTags is an embeddable struct that provides a single "operation" tag.
// It cooperates in tag emission, but does not contain an emitter.
//
// Per name, each instance should be unique and constructed only once, likely in
// the constructor of the thing performing an operation.
type OperationTags struct {
	Operation string `tag:"operation"` // our historical "the thing doing something" name, unchanged for consistency
}
