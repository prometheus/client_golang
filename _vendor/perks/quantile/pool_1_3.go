//+build go1.3

package quantile

import "sync"

// With the Go1.3 sync Pool, there is no max capacity, and a globally shared
// pool is more efficient.
var globalSamplePool = sync.Pool{New: func() interface{} { return &Sample{} }}

type samplePool struct{}

func newSamplePool(capacity int) *samplePool {
	// capacity ignored for Go1.3 sync.Pool.
	return &samplePool{}
}

func (_ samplePool) Get(value, width, delta float64) *Sample {
	sample := globalSamplePool.Get().(*Sample)
	sample.Value, sample.Width, sample.Delta = value, width, delta
	return sample
}

func (_ samplePool) Put(sample *Sample) {
	globalSamplePool.Put(sample)
}
