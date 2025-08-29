package metrics

func ignoredInTestFiles() {
	var scope Scope

	// test code is allowed to break the rules because it does not run in production

	scope.IncCounter(123)
	scope.IncCounter(123)
	scope.ExponentialHistogram(123, 0)
}
