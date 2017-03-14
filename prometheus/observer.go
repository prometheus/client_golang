package prometheus

// Observer is the interface that wraps the Observe method, which is used by
// Histogram and Summary to add observations.
type Observer interface {
	Observe(float64)
}

// The ObserverFunc type is an adapter to allow the use of ordinary
// functions as Observers. If f is a function with the appropriate
// signature, ObserverFunc(f) is an Observer that calls f.
//
// This adapter is usually used in connection with the Timer type, and there are
// two general use cases:
//
// The most common one is to use a Gauge as the Observer for a Timer.
// See the "Gauge" Timer example.
//
// The more advanced use case is to create a function that dynamically decides
// which Observer to use for observing the duration. See the "Complex" Timer
// example.
type ObserverFunc func(float64)

// Observe calls f(value). It implements Observer.
func (f ObserverFunc) Observe(value float64) {
	f(value)
}

// ObserverVec implements MetricVec for the purpose of using instance labels
// for an Observer.
type ObserverVec interface {
	GetMetricWith(Labels) (Observer, error)
	GetMetricWithLabelValues(lvs ...string) (Observer, error)
	With(Labels) Observer
	WithLabelValues(...string) Observer
}
