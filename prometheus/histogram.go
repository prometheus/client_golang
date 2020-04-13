// Copyright 2015 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheus

import (
	"fmt"
	"math"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/proto"

	dto "github.com/prometheus/client_model/go"
)

// A Histogram counts individual observations from an event or sample stream in
// configurable buckets. Similar to a summary, it also provides a sum of
// observations and an observation count.
//
// On the Prometheus server, quantiles can be calculated from a Histogram using
// the histogram_quantile function in the query language.
//
// Note that Histograms, in contrast to Summaries, can be aggregated with the
// Prometheus query language (see the documentation for detailed
// procedures). However, Histograms require the user to pre-define suitable
// buckets, and they are in general less accurate. The Observe method of a
// Histogram has a very low performance overhead in comparison with the Observe
// method of a Summary.
//
// To create Histogram instances, use NewHistogram.
type Histogram interface {
	Metric
	Collector

	// Observe adds a single observation to the histogram.
	Observe(float64)
}

// bucketLabel is used for the label that defines the upper bound of a
// bucket of a histogram ("le" -> "less or equal").
const bucketLabel = "le"

// DefBuckets are the default Histogram buckets. The default buckets are
// tailored to broadly measure the response time (in seconds) of a network
// service. Most likely, however, you will be required to define buckets
// customized to your use case.
var DefBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}

// DefSparseBucketsZeroThreshold is the default value for
// SparseBucketsZeroThreshold in the HistogramOpts.
var DefSparseBucketsZeroThreshold = 1e-128

var errBucketLabelNotAllowed = fmt.Errorf(
	"%q is not allowed as label name in histograms", bucketLabel,
)

// LinearBuckets creates 'count' buckets, each 'width' wide, where the lowest
// bucket has an upper bound of 'start'. The final +Inf bucket is not counted
// and not included in the returned slice. The returned slice is meant to be
// used for the Buckets field of HistogramOpts.
//
// The function panics if 'count' is zero or negative.
func LinearBuckets(start, width float64, count int) []float64 {
	if count < 1 {
		panic("LinearBuckets needs a positive count")
	}
	buckets := make([]float64, count)
	for i := range buckets {
		buckets[i] = start
		start += width
	}
	return buckets
}

// ExponentialBuckets creates 'count' buckets, where the lowest bucket has an
// upper bound of 'start' and each following bucket's upper bound is 'factor'
// times the previous bucket's upper bound. The final +Inf bucket is not counted
// and not included in the returned slice. The returned slice is meant to be
// used for the Buckets field of HistogramOpts.
//
// The function panics if 'count' is 0 or negative, if 'start' is 0 or negative,
// or if 'factor' is less than or equal 1.
func ExponentialBuckets(start, factor float64, count int) []float64 {
	if count < 1 {
		panic("ExponentialBuckets needs a positive count")
	}
	if start <= 0 {
		panic("ExponentialBuckets needs a positive start value")
	}
	if factor <= 1 {
		panic("ExponentialBuckets needs a factor greater than 1")
	}
	buckets := make([]float64, count)
	for i := range buckets {
		buckets[i] = start
		start *= factor
	}
	return buckets
}

// HistogramOpts bundles the options for creating a Histogram metric. It is
// mandatory to set Name to a non-empty string. All other fields are optional
// and can safely be left at their zero value, although it is strongly
// encouraged to set a Help string.
type HistogramOpts struct {
	// Namespace, Subsystem, and Name are components of the fully-qualified
	// name of the Histogram (created by joining these components with
	// "_"). Only Name is mandatory, the others merely help structuring the
	// name. Note that the fully-qualified name of the Histogram must be a
	// valid Prometheus metric name.
	Namespace string
	Subsystem string
	Name      string

	// Help provides information about this Histogram.
	//
	// Metrics with the same fully-qualified name must have the same Help
	// string.
	Help string

	// ConstLabels are used to attach fixed labels to this metric. Metrics
	// with the same fully-qualified name must have the same label names in
	// their ConstLabels.
	//
	// ConstLabels are only used rarely. In particular, do not use them to
	// attach the same labels to all your metrics. Those use cases are
	// better covered by target labels set by the scraping Prometheus
	// server, or by one specific metric (e.g. a build_info or a
	// machine_role metric). See also
	// https://prometheus.io/docs/instrumenting/writing_exporters/#target-labels-not-static-scraped-labels
	ConstLabels Labels

	// Buckets defines the buckets into which observations are counted. Each
	// element in the slice is the upper inclusive bound of a bucket. The
	// values must be sorted in strictly increasing order. There is no need
	// to add a highest bucket with +Inf bound, it will be added
	// implicitly. If Buckets is left as nil or set to a slice of length
	// zero, it is replaced by default buckets. The default buckets are
	// DefBuckets if no sparse buckets (see below) are used, otherwise the
	// default is no buckets. (In other words, if you want to use both
	// reguler buckets and sparse buckets, you have to define the regular
	// buckets here explicitly.)
	Buckets []float64

	// If SparseBucketsResolution is not zero, sparse buckets are used (in
	// addition to the regular buckets, if defined above). Every power of
	// ten is divided into the given number of exponential buckets. For
	// example, if set to 3, the bucket boundaries are approximately […,
	// 0.1, 0.215, 0.464, 1, 2.15, 4,64, 10, 21.5, 46.4, 100, …] Histograms
	// can only be properly aggregated if they use the same
	// resolution. Therefore, it is recommended to use 20 as a resolution,
	// which is generally expected to be a good tradeoff between resource
	// usage and accuracy (resulting in a maximum error of quantile values
	// of about 6%).
	SparseBucketsResolution uint8
	// All observations with an absolute value of less or equal
	// SparseBucketsZeroThreshold are accumulated into a “zero” bucket. For
	// best results, this should be close to a bucket boundary. This is
	// moste easily accomplished by picking a power of ten. If
	// SparseBucketsZeroThreshold is left at zero (or set to a negative
	// value), DefSparseBucketsZeroThreshold is used as the threshold.
	SparseBucketsZeroThreshold float64
}

// NewHistogram creates a new Histogram based on the provided HistogramOpts. It
// panics if the buckets in HistogramOpts are not in strictly increasing order.
//
// The returned implementation also implements ExemplarObserver. It is safe to
// perform the corresponding type assertion. Exemplars are tracked separately
// for each bucket.
func NewHistogram(opts HistogramOpts) Histogram {
	return newHistogram(
		NewDesc(
			BuildFQName(opts.Namespace, opts.Subsystem, opts.Name),
			opts.Help,
			nil,
			opts.ConstLabels,
		),
		opts,
	)
}

func newHistogram(desc *Desc, opts HistogramOpts, labelValues ...string) Histogram {
	if len(desc.variableLabels) != len(labelValues) {
		panic(makeInconsistentCardinalityError(desc.fqName, desc.variableLabels, labelValues))
	}

	for _, n := range desc.variableLabels {
		if n == bucketLabel {
			panic(errBucketLabelNotAllowed)
		}
	}
	for _, lp := range desc.constLabelPairs {
		if lp.GetName() == bucketLabel {
			panic(errBucketLabelNotAllowed)
		}
	}

	h := &histogram{
		desc:             desc,
		upperBounds:      opts.Buckets,
		sparseResolution: uint32(opts.SparseBucketsResolution),
		sparseThreshold:  opts.SparseBucketsZeroThreshold,
		labelPairs:       makeLabelPairs(desc, labelValues),
		counts:           [2]*histogramCounts{{}, {}},
		now:              time.Now,
	}
	if len(h.upperBounds) == 0 && opts.SparseBucketsResolution == 0 {
		h.upperBounds = DefBuckets
	}
	if h.sparseThreshold <= 0 {
		h.sparseThreshold = DefSparseBucketsZeroThreshold
	}
	for i, upperBound := range h.upperBounds {
		if i < len(h.upperBounds)-1 {
			if upperBound >= h.upperBounds[i+1] {
				panic(fmt.Errorf(
					"histogram buckets must be in increasing order: %f >= %f",
					upperBound, h.upperBounds[i+1],
				))
			}
		} else {
			if math.IsInf(upperBound, +1) {
				// The +Inf bucket is implicit. Remove it here.
				h.upperBounds = h.upperBounds[:i]
			}
		}
	}
	// Finally we know the final length of h.upperBounds and can make buckets
	// for both counts as well as exemplars:
	h.counts[0].buckets = make([]uint64, len(h.upperBounds))
	h.counts[1].buckets = make([]uint64, len(h.upperBounds))
	h.exemplars = make([]atomic.Value, len(h.upperBounds)+1)

	h.init(h) // Init self-collection.
	return h
}

type histogramCounts struct {
	// sumBits contains the bits of the float64 representing the sum of all
	// observations. sumBits and count have to go first in the struct to
	// guarantee alignment for atomic operations.
	// http://golang.org/pkg/sync/atomic/#pkg-note-BUG
	sumBits uint64
	count   uint64
	buckets []uint64
	// sparse buckets are implemented with a sync.Map for this PoC. A
	// dedicated data structure will likely be more efficient.
	// There are separate maps for negative and positive observations.
	// The map's value is a *uint64, counting observations in that bucket.
	// The map's key is the logarithmic index of the bucket. Index 0 is for an
	// upper bound of 1. Each increment/decrement by SparseBucketsResolution
	// multiplies/divides the upper bound by 10. Indices in between are
	// spaced exponentially as defined in spareBounds.
	sparseBucketsPositive, sparseBucketsNegative sync.Map
	// sparseZeroBucket counts all (positive and negative) observations in
	// the zero bucket (with an absolute value less or equal
	// SparseBucketsZeroThreshold).
	sparseZeroBucket uint64
}

// observe manages the parts of observe that only affects
// histogramCounts. doSparse is true if spare buckets should be done,
// too. whichSparse is 0 for the sparseZeroBucket and +1 or -1 for
// sparseBucketsPositive or sparseBucketsNegative, respectively. sparseKey is
// the key of the sparse bucket to use.
func (hc *histogramCounts) observe(v float64, bucket int, doSparse bool, whichSparse int, sparseKey int) {
	if bucket < len(hc.buckets) {
		atomic.AddUint64(&hc.buckets[bucket], 1)
	}
	for {
		oldBits := atomic.LoadUint64(&hc.sumBits)
		newBits := math.Float64bits(math.Float64frombits(oldBits) + v)
		if atomic.CompareAndSwapUint64(&hc.sumBits, oldBits, newBits) {
			break
		}
	}
	if doSparse {
		switch whichSparse {
		case 0:
			atomic.AddUint64(&hc.sparseZeroBucket, 1)
		case +1:
			addToSparseBucket(&hc.sparseBucketsPositive, sparseKey, 1)
		case -1:
			addToSparseBucket(&hc.sparseBucketsNegative, sparseKey, 1)
		default:
			panic(fmt.Errorf("invalid value for whichSparse: %d", whichSparse))
		}
	}
	// Increment count last as we take it as a signal that the observation
	// is complete.
	atomic.AddUint64(&hc.count, 1)
}

func addToSparseBucket(buckets *sync.Map, key int, increment uint64) {
	if existingBucket, ok := buckets.Load(key); ok {
		// Fast path without allocation.
		atomic.AddUint64(existingBucket.(*uint64), increment)
		return
	}
	// Bucket doesn't exist yet. Slow path allocating new counter.
	newBucket := increment // TODO(beorn7): Check if this is sufficient to not let increment escape.
	if actualBucket, loaded := buckets.LoadOrStore(key, &newBucket); loaded {
		// The bucket was created concurrently in another goroutine.
		// Have to increment after all.
		atomic.AddUint64(actualBucket.(*uint64), increment)
	}
}

type histogram struct {
	// countAndHotIdx enables lock-free writes with use of atomic updates.
	// The most significant bit is the hot index [0 or 1] of the count field
	// below. Observe calls update the hot one. All remaining bits count the
	// number of Observe calls. Observe starts by incrementing this counter,
	// and finish by incrementing the count field in the respective
	// histogramCounts, as a marker for completion.
	//
	// Calls of the Write method (which are non-mutating reads from the
	// perspective of the histogram) swap the hot–cold under the writeMtx
	// lock. A cooldown is awaited (while locked) by comparing the number of
	// observations with the initiation count. Once they match, then the
	// last observation on the now cool one has completed. All cool fields must
	// be merged into the new hot before releasing writeMtx.
	//
	// Fields with atomic access first! See alignment constraint:
	// http://golang.org/pkg/sync/atomic/#pkg-note-BUG
	countAndHotIdx uint64

	selfCollector
	desc     *Desc
	writeMtx sync.Mutex // Only used in the Write method.

	// Two counts, one is "hot" for lock-free observations, the other is
	// "cold" for writing out a dto.Metric. It has to be an array of
	// pointers to guarantee 64bit alignment of the histogramCounts, see
	// http://golang.org/pkg/sync/atomic/#pkg-note-BUG.
	counts [2]*histogramCounts

	upperBounds      []float64
	labelPairs       []*dto.LabelPair
	exemplars        []atomic.Value // One more than buckets (to include +Inf), each a *dto.Exemplar.
	sparseResolution uint32         // Instead of uint8 to be ready for protobuf encoding.
	sparseThreshold  float64

	now func() time.Time // To mock out time.Now() for testing.
}

func (h *histogram) Desc() *Desc {
	return h.desc
}

func (h *histogram) Observe(v float64) {
	h.observe(v, h.findBucket(v))
}

func (h *histogram) ObserveWithExemplar(v float64, e Labels) {
	i := h.findBucket(v)
	h.observe(v, i)
	h.updateExemplar(v, i, e)
}

func (h *histogram) Write(out *dto.Metric) error {
	// For simplicity, we protect this whole method by a mutex. It is not in
	// the hot path, i.e. Observe is called much more often than Write. The
	// complication of making Write lock-free isn't worth it, if possible at
	// all.
	h.writeMtx.Lock()
	defer h.writeMtx.Unlock()

	// Adding 1<<63 switches the hot index (from 0 to 1 or from 1 to 0)
	// without touching the count bits. See the struct comments for a full
	// description of the algorithm.
	n := atomic.AddUint64(&h.countAndHotIdx, 1<<63)
	// count is contained unchanged in the lower 63 bits.
	count := n & ((1 << 63) - 1)
	// The most significant bit tells us which counts is hot. The complement
	// is thus the cold one.
	hotCounts := h.counts[n>>63]
	coldCounts := h.counts[(^n)>>63]

	// Await cooldown.
	for count != atomic.LoadUint64(&coldCounts.count) {
		runtime.Gosched() // Let observations get work done.
	}

	his := &dto.Histogram{
		Bucket:          make([]*dto.Bucket, len(h.upperBounds)),
		SampleCount:     proto.Uint64(count),
		SampleSum:       proto.Float64(math.Float64frombits(atomic.LoadUint64(&coldCounts.sumBits))),
		SbResolution:    &h.sparseResolution,
		SbZeroThreshold: &h.sparseThreshold,
	}
	out.Histogram = his
	out.Label = h.labelPairs

	var cumCount uint64
	for i, upperBound := range h.upperBounds {
		cumCount += atomic.LoadUint64(&coldCounts.buckets[i])
		his.Bucket[i] = &dto.Bucket{
			CumulativeCount: proto.Uint64(cumCount),
			UpperBound:      proto.Float64(upperBound),
		}
		if e := h.exemplars[i].Load(); e != nil {
			his.Bucket[i].Exemplar = e.(*dto.Exemplar)
		}
	}
	// If there is an exemplar for the +Inf bucket, we have to add that bucket explicitly.
	if e := h.exemplars[len(h.upperBounds)].Load(); e != nil {
		b := &dto.Bucket{
			CumulativeCount: proto.Uint64(count),
			UpperBound:      proto.Float64(math.Inf(1)),
			Exemplar:        e.(*dto.Exemplar),
		}
		his.Bucket = append(his.Bucket, b)
	}
	// Add all the cold counts to the new hot counts and reset the cold counts.
	atomic.AddUint64(&hotCounts.count, count)
	atomic.StoreUint64(&coldCounts.count, 0)
	for {
		oldBits := atomic.LoadUint64(&hotCounts.sumBits)
		newBits := math.Float64bits(math.Float64frombits(oldBits) + his.GetSampleSum())
		if atomic.CompareAndSwapUint64(&hotCounts.sumBits, oldBits, newBits) {
			atomic.StoreUint64(&coldCounts.sumBits, 0)
			break
		}
	}
	for i := range h.upperBounds {
		atomic.AddUint64(&hotCounts.buckets[i], atomic.LoadUint64(&coldCounts.buckets[i]))
		atomic.StoreUint64(&coldCounts.buckets[i], 0)
	}
	if h.sparseResolution != 0 {
		zeroBucket := atomic.LoadUint64(&coldCounts.sparseZeroBucket)

		defer func() {
			atomic.AddUint64(&hotCounts.sparseZeroBucket, zeroBucket)
			atomic.StoreUint64(&coldCounts.sparseZeroBucket, 0)
			coldCounts.sparseBucketsPositive.Range(addAndReset(&hotCounts.sparseBucketsPositive))
			coldCounts.sparseBucketsNegative.Range(addAndReset(&hotCounts.sparseBucketsNegative))
		}()

		his.SbZeroCount = proto.Uint64(zeroBucket)
		his.SbNegative = makeSparseBuckets(&coldCounts.sparseBucketsNegative)
		his.SbPositive = makeSparseBuckets(&coldCounts.sparseBucketsPositive)
	}
	return nil
}

func makeSparseBuckets(buckets *sync.Map) *dto.SparseBuckets {
	var ii []int
	buckets.Range(func(k, v interface{}) bool {
		ii = append(ii, k.(int))
		return true
	})
	sort.Ints(ii)

	if len(ii) == 0 {
		return nil
	}

	sbs := dto.SparseBuckets{}
	var prevCount uint64
	var nextI int
	for n, i := range ii {
		v, _ := buckets.Load(i)
		count := atomic.LoadUint64(v.(*uint64))
		if n == 0 || i-nextI != 0 {
			sbs.Span = append(sbs.Span, &dto.SparseBuckets_Span{
				Offset: proto.Int(i - nextI),
				Length: proto.Uint32(1),
			})
		} else {
			*sbs.Span[len(sbs.Span)-1].Length++
		}
		sbs.Delta = append(sbs.Delta, int64(count)-int64(prevCount)) // TODO(beorn7): Do proper overflow handling.
		nextI, prevCount = i+1, count
	}
	return &sbs
}

// addAndReset returns a function to be used with sync.Map.Range of spare
// buckets in coldCounts. It increments the buckets in the provided hotBuckets
// according to the buckets ranged through. It then resets all buckets ranged
// through to 0 (but leaves them in place so that they don't need to get
// recreated on the next scrape).
func addAndReset(hotBuckets *sync.Map) func(k, v interface{}) bool {
	return func(k, v interface{}) bool {
		bucket := v.(*uint64)
		addToSparseBucket(hotBuckets, k.(int), atomic.LoadUint64(bucket))
		atomic.StoreUint64(bucket, 0)
		return true
	}
}

// findBucket returns the index of the bucket for the provided value, or
// len(h.upperBounds) for the +Inf bucket.
func (h *histogram) findBucket(v float64) int {
	// TODO(beorn7): For small numbers of buckets (<30), a linear search is
	// slightly faster than the binary search. If we really care, we could
	// switch from one search strategy to the other depending on the number
	// of buckets.
	//
	// Microbenchmarks (BenchmarkHistogramNoLabels):
	// 11 buckets: 38.3 ns/op linear - binary 48.7 ns/op
	// 100 buckets: 78.1 ns/op linear - binary 54.9 ns/op
	// 300 buckets: 154 ns/op linear - binary 61.6 ns/op
	return sort.SearchFloat64s(h.upperBounds, v)
}

// observe is the implementation for Observe without the findBucket part.
func (h *histogram) observe(v float64, bucket int) {
	doSparse := h.sparseResolution != 0
	var whichSparse, sparseKey int
	if doSparse {
		switch {
		case v > h.sparseThreshold:
			whichSparse = +1
		case v < -h.sparseThreshold:
			whichSparse = -1
		}
		// TODO(beorn7): This sometimes gives inaccurate results for
		// floats that are actual powers of 10, e.g. math.Log10(0.1) is
		// calculated as -0.9999999999999999 rather than -1 and thus
		// yields a key unexpectedly one off. Maybe special-case precise
		// powers of 10.
		sparseKey = int(math.Ceil(math.Log10(math.Abs(v)) * float64(h.sparseResolution)))
	}
	// We increment h.countAndHotIdx so that the counter in the lower
	// 63 bits gets incremented. At the same time, we get the new value
	// back, which we can use to find the currently-hot counts.
	n := atomic.AddUint64(&h.countAndHotIdx, 1)
	h.counts[n>>63].observe(v, bucket, doSparse, whichSparse, sparseKey)
}

// updateExemplar replaces the exemplar for the provided bucket. With empty
// labels, it's a no-op. It panics if any of the labels is invalid.
func (h *histogram) updateExemplar(v float64, bucket int, l Labels) {
	if l == nil {
		return
	}
	e, err := newExemplar(v, h.now(), l)
	if err != nil {
		panic(err)
	}
	h.exemplars[bucket].Store(e)
}

// HistogramVec is a Collector that bundles a set of Histograms that all share the
// same Desc, but have different values for their variable labels. This is used
// if you want to count the same thing partitioned by various dimensions
// (e.g. HTTP request latencies, partitioned by status code and method). Create
// instances with NewHistogramVec.
type HistogramVec struct {
	*metricVec
}

// NewHistogramVec creates a new HistogramVec based on the provided HistogramOpts and
// partitioned by the given label names.
func NewHistogramVec(opts HistogramOpts, labelNames []string) *HistogramVec {
	desc := NewDesc(
		BuildFQName(opts.Namespace, opts.Subsystem, opts.Name),
		opts.Help,
		labelNames,
		opts.ConstLabels,
	)
	return &HistogramVec{
		metricVec: newMetricVec(desc, func(lvs ...string) Metric {
			return newHistogram(desc, opts, lvs...)
		}),
	}
}

// GetMetricWithLabelValues returns the Histogram for the given slice of label
// values (same order as the VariableLabels in Desc). If that combination of
// label values is accessed for the first time, a new Histogram is created.
//
// It is possible to call this method without using the returned Histogram to only
// create the new Histogram but leave it at its starting value, a Histogram without
// any observations.
//
// Keeping the Histogram for later use is possible (and should be considered if
// performance is critical), but keep in mind that Reset, DeleteLabelValues and
// Delete can be used to delete the Histogram from the HistogramVec. In that case, the
// Histogram will still exist, but it will not be exported anymore, even if a
// Histogram with the same label values is created later. See also the CounterVec
// example.
//
// An error is returned if the number of label values is not the same as the
// number of VariableLabels in Desc (minus any curried labels).
//
// Note that for more than one label value, this method is prone to mistakes
// caused by an incorrect order of arguments. Consider GetMetricWith(Labels) as
// an alternative to avoid that type of mistake. For higher label numbers, the
// latter has a much more readable (albeit more verbose) syntax, but it comes
// with a performance overhead (for creating and processing the Labels map).
// See also the GaugeVec example.
func (v *HistogramVec) GetMetricWithLabelValues(lvs ...string) (Observer, error) {
	metric, err := v.metricVec.getMetricWithLabelValues(lvs...)
	if metric != nil {
		return metric.(Observer), err
	}
	return nil, err
}

// GetMetricWith returns the Histogram for the given Labels map (the label names
// must match those of the VariableLabels in Desc). If that label map is
// accessed for the first time, a new Histogram is created. Implications of
// creating a Histogram without using it and keeping the Histogram for later use
// are the same as for GetMetricWithLabelValues.
//
// An error is returned if the number and names of the Labels are inconsistent
// with those of the VariableLabels in Desc (minus any curried labels).
//
// This method is used for the same purpose as
// GetMetricWithLabelValues(...string). See there for pros and cons of the two
// methods.
func (v *HistogramVec) GetMetricWith(labels Labels) (Observer, error) {
	metric, err := v.metricVec.getMetricWith(labels)
	if metric != nil {
		return metric.(Observer), err
	}
	return nil, err
}

// WithLabelValues works as GetMetricWithLabelValues, but panics where
// GetMetricWithLabelValues would have returned an error. Not returning an
// error allows shortcuts like
//     myVec.WithLabelValues("404", "GET").Observe(42.21)
func (v *HistogramVec) WithLabelValues(lvs ...string) Observer {
	h, err := v.GetMetricWithLabelValues(lvs...)
	if err != nil {
		panic(err)
	}
	return h
}

// With works as GetMetricWith but panics where GetMetricWithLabels would have
// returned an error. Not returning an error allows shortcuts like
//     myVec.With(prometheus.Labels{"code": "404", "method": "GET"}).Observe(42.21)
func (v *HistogramVec) With(labels Labels) Observer {
	h, err := v.GetMetricWith(labels)
	if err != nil {
		panic(err)
	}
	return h
}

// CurryWith returns a vector curried with the provided labels, i.e. the
// returned vector has those labels pre-set for all labeled operations performed
// on it. The cardinality of the curried vector is reduced accordingly. The
// order of the remaining labels stays the same (just with the curried labels
// taken out of the sequence – which is relevant for the
// (GetMetric)WithLabelValues methods). It is possible to curry a curried
// vector, but only with labels not yet used for currying before.
//
// The metrics contained in the HistogramVec are shared between the curried and
// uncurried vectors. They are just accessed differently. Curried and uncurried
// vectors behave identically in terms of collection. Only one must be
// registered with a given registry (usually the uncurried version). The Reset
// method deletes all metrics, even if called on a curried vector.
func (v *HistogramVec) CurryWith(labels Labels) (ObserverVec, error) {
	vec, err := v.curryWith(labels)
	if vec != nil {
		return &HistogramVec{vec}, err
	}
	return nil, err
}

// MustCurryWith works as CurryWith but panics where CurryWith would have
// returned an error.
func (v *HistogramVec) MustCurryWith(labels Labels) ObserverVec {
	vec, err := v.CurryWith(labels)
	if err != nil {
		panic(err)
	}
	return vec
}

type constHistogram struct {
	desc       *Desc
	count      uint64
	sum        float64
	buckets    map[float64]uint64
	labelPairs []*dto.LabelPair
}

func (h *constHistogram) Desc() *Desc {
	return h.desc
}

func (h *constHistogram) Write(out *dto.Metric) error {
	his := &dto.Histogram{}
	buckets := make([]*dto.Bucket, 0, len(h.buckets))

	his.SampleCount = proto.Uint64(h.count)
	his.SampleSum = proto.Float64(h.sum)

	for upperBound, count := range h.buckets {
		buckets = append(buckets, &dto.Bucket{
			CumulativeCount: proto.Uint64(count),
			UpperBound:      proto.Float64(upperBound),
		})
	}

	if len(buckets) > 0 {
		sort.Sort(buckSort(buckets))
	}
	his.Bucket = buckets

	out.Histogram = his
	out.Label = h.labelPairs

	return nil
}

// NewConstHistogram returns a metric representing a Prometheus histogram with
// fixed values for the count, sum, and bucket counts. As those parameters
// cannot be changed, the returned value does not implement the Histogram
// interface (but only the Metric interface). Users of this package will not
// have much use for it in regular operations. However, when implementing custom
// Collectors, it is useful as a throw-away metric that is generated on the fly
// to send it to Prometheus in the Collect method.
//
// buckets is a map of upper bounds to cumulative counts, excluding the +Inf
// bucket.
//
// NewConstHistogram returns an error if the length of labelValues is not
// consistent with the variable labels in Desc or if Desc is invalid.
func NewConstHistogram(
	desc *Desc,
	count uint64,
	sum float64,
	buckets map[float64]uint64,
	labelValues ...string,
) (Metric, error) {
	if desc.err != nil {
		return nil, desc.err
	}
	if err := validateLabelValues(labelValues, len(desc.variableLabels)); err != nil {
		return nil, err
	}
	return &constHistogram{
		desc:       desc,
		count:      count,
		sum:        sum,
		buckets:    buckets,
		labelPairs: makeLabelPairs(desc, labelValues),
	}, nil
}

// MustNewConstHistogram is a version of NewConstHistogram that panics where
// NewConstMetric would have returned an error.
func MustNewConstHistogram(
	desc *Desc,
	count uint64,
	sum float64,
	buckets map[float64]uint64,
	labelValues ...string,
) Metric {
	m, err := NewConstHistogram(desc, count, sum, buckets, labelValues...)
	if err != nil {
		panic(err)
	}
	return m
}

type buckSort []*dto.Bucket

func (s buckSort) Len() int {
	return len(s)
}

func (s buckSort) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s buckSort) Less(i, j int) bool {
	return s[i].GetUpperBound() < s[j].GetUpperBound()
}
