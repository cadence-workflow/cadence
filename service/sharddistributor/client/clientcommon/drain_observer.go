package clientcommon

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination drain_observer_mock.go . DrainSignalObserver

// DrainSignalObserver observes infrastructure drain signals.
// Drain is reversible: if the instance reappears in discovery,
// Undrain() fires, allowing the consumer to resume operations.
//
// Implementations use close-to-broadcast semantics: the returned channel is
// closed when the event occurs, so all goroutines selecting on it wake up.
// After each close, a fresh channel is created for the next cycle.
type DrainSignalObserver interface {
	// Drain returns a channel closed when the instance is
	// removed from service discovery.
	Drain() <-chan struct{}

	// Undrain returns a channel closed when the instance is
	// added back to service discovery after a drain.
	Undrain() <-chan struct{}
}
