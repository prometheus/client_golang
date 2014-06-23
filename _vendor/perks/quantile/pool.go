//+build !go1.3

package quantile

type samplePool struct {
	pool chan *Sample
}

func newSamplePool(capacity int) *samplePool {
	return &samplePool{pool: make(chan *Sample, capacity)}
}

func (sp *samplePool) Get(value, width, delta float64) *Sample {
	select {
	case sample := <-sp.pool:
		sample.Value, sample.Width, sample.Delta = value, width, delta
		return sample
	default:
		return &Sample{value, width, delta}
	}
}

func (sp *samplePool) Put(sample *Sample) {
	select {
	case sp.pool <- sample:
	default:
	}
}
