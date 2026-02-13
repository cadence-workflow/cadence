package clientcommon

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination drain_observer_mock.go . DrainSignalObserver

// DrainSignalObserver observes infrastructure drain signals (e.g. removal
// from service discovery during a zone drain). Drain is reversible: if the
// instance reappears in discovery, Undrain() fires, allowing the consumer
// to resume operations.
//
// Implementations use close-to-broadcast semantics: the returned channel is
// closed when the event occurs, so all goroutines selecting on it wake up.
// After each close, a fresh channel is created for the next cycle. Callers
// should snapshot the channel once (e.g. at the start of a select loop) and
// select on that snapshot.
type DrainSignalObserver interface {
	// Drain returns a channel that is closed when the instance is removed
	// from service discovery (e.g. zone drain). The channel is replaced
	// with a fresh one after each drain/undrain cycle.
	Drain() <-chan struct{}

	// Undrain returns a channel that is closed when the instance is added
	// back to service discovery after a drain. The channel is replaced
	// with a fresh one after each drain/undrain cycle.
	Undrain() <-chan struct{}
}
