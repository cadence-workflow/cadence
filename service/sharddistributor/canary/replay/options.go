package replay

// Options configures CSV load replay for the canary.
type Options struct {
	CSVPath string

	// Speed is a multiplier for replay time (1.0 = real time).
	Speed float64

	// Namespace is the fixed namespace to replay into.
	Namespace string

	// NumFixedExecutors is how many fixed-namespace executors to run in-process.
	NumFixedExecutors int
}

func (o Options) Enabled() bool {
	return o.CSVPath != ""
}
