// Copyright 2014 The Prometheus Authors
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
	"errors"
	"math"
	"sync/atomic"
	"time"

	dto "github.com/prometheus/client_model/go"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// 64-bit float mantissa: https://en.wikipedia.org/wiki/Double-precision_floating-point_format
var float64Mantissa uint64 = 9007199254740992

// Counter is a Metric that represents a single numerical value that only ever
// goes up. That implies that it cannot be used to count items whose number can
// also go down, e.g. the number of currently running goroutines. Those
// "counters" are represented by Gauges.
//
// A Counter is typically used to count requests served, tasks completed, errors
// occurred, etc.
//
// To create Counter instances, use NewCounter.
type Counter interface {
	Metric
	Collector

	// Inc increments the counter by 1. Use Add to increment it by arbitrary
	// non-negative values.
	Inc()
	// Add adds the given value to the counter. It panics if the value is <
	// 0.
	Add(float64)
}

// ExemplarAdder is implemented by Counters that offer the option of adding a
// value to the Counter together with an exemplar. Its AddWithExemplar method
// works like the Add method of the Counter interface but also replaces the
// currently saved exemplar (if any) with a new one, created from the provided
// value, the current time as timestamp, and the provided labels. Empty Labels
// will lead to a valid (label-less) exemplar. But if Labels is nil, the current
// exemplar is left in place. AddWithExemplar panics if the value is < 0, if any
// of the provided labels are invalid, or if the provided labels contain more
// than 128 runes in total.
type ExemplarAdder interface {
	AddWithExemplar(value float64, exemplar Labels)
}

// CounterOpts bundles the options for creating a Histogram metric. It is
// mandatory to set Name to a non-empty string. All other fields are optional
// and can safely be left at their zero value, although it is strongly
// encouraged to set a Help string.
type CounterOpts struct {
	// Namespace, Subsystem, and Name are components of the fully-qualified
	// name of the Metric (created by joining these components with
	// "_"). Only Name is mandatory, the others merely help structuring the
	// name. Note that the fully-qualified name of the metric must be a
	// valid Prometheus metric name.
	Namespace string
	Subsystem string
	Name      string

	// Help provides information about this metric.
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

	// now is for testing purposes, by default it's time.Now.
	now func() time.Time

	// Counters are double (float64) values. At values above 2^53, double loses
	// the ability to represent discrete integer values precisely. At 2^53 the
	// error is just +/-1 and is likely of little consequence. At 2^64 the error
	// is +/-1024 (the next smallest number that can be represented is 2^64-2048).
	// This may be significant error for long-running counters that reach the
	// upper range of uint64. To present large Counter values as integer-only,
	// set the IntegerExposition option. This will wrap the Counter twice, once at the
	// largest safe integer value, and again when the Counter's uint64 value
	// becomes 0. Prometheus will handle this rollover gracefully.
	IntegerExposition bool
}

// CounterVecOpts bundles the options to create a CounterVec metric.
// It is mandatory to set CounterOpts, see there for mandatory fields. VariableLabels
// is optional and can safely be left to its default value.
type CounterVecOpts struct {
	CounterOpts

	// VariableLabels are used to partition the metric vector by the given set
	// of labels. Each label value will be constrained with the optional Constraint
	// function, if provided.
	VariableLabels ConstrainableLabels
}

// NewCounter creates a new Counter based on the provided CounterOpts.
//
// The returned implementation also implements ExemplarAdder. It is safe to
// perform the corresponding type assertion.
//
// The returned implementation tracks the counter value in two separate
// variables, a float64 and a uint64. The latter is used to track calls of the
// Inc method and calls of the Add method with a value that can be represented
// as a uint64. This allows atomic increments of the counter with optimal
// performance. (It is common to have an Inc call in very hot execution paths.)
// Both internal tracking values are added up in the Write method. This has to
// be taken into account when it comes to precision and overflow behavior.
func NewCounter(opts CounterOpts) Counter {
	desc := NewDesc(
		BuildFQName(opts.Namespace, opts.Subsystem, opts.Name),
		opts.Help,
		nil,
		opts.ConstLabels,
	)
	if opts.now == nil {
		opts.now = time.Now
	}
	result := &counter{desc: desc, labelPairs: desc.constLabelPairs, now: opts.now, integerExposition: opts.IntegerExposition}
	result.init(result) // Init self-collection.
	result.createdTs = timestamppb.New(opts.now())
	return result
}

type counter struct {
	// valBits contains the bits of the represented float64 value, while
	// valInt stores values that are exact integers. Both have to go first
	// in the struct to guarantee alignment for atomic operations.
	// http://golang.org/pkg/sync/atomic/#pkg-note-BUG
	valBits uint64
	valInt  uint64

	selfCollector
	desc *Desc

	createdTs  *timestamppb.Timestamp
	labelPairs []*dto.LabelPair
	exemplar   atomic.Value // Containing nil or a *dto.Exemplar.

	integerExposition bool

	// now is for testing purposes, by default it's time.Now.
	now func() time.Time
}

func (c *counter) Desc() *Desc {
	return c.desc
}

func (c *counter) Add(v float64) {
	if v < 0 {
		panic(errors.New("counter cannot decrease in value"))
	}

	ival := uint64(v)
	if float64(ival) == v {
		atomic.AddUint64(&c.valInt, ival)
		return
	} else if c.integerExposition {
		// perhaps there should be an AddInt() method? this is a footgun for callers.
		panic(errors.New("cannot add large value with rounding error to integer counter"))
	}

	for {
		oldBits := atomic.LoadUint64(&c.valBits)
		newBits := math.Float64bits(math.Float64frombits(oldBits) + v)
		if atomic.CompareAndSwapUint64(&c.valBits, oldBits, newBits) {
			return
		}
	}
}

func (c *counter) AddWithExemplar(v float64, e Labels) {
	c.Add(v)
	c.updateExemplar(v, e)
}

func (c *counter) Inc() {
	atomic.AddUint64(&c.valInt, 1)
}

func (c *counter) get() float64 {
	ival := atomic.LoadUint64(&c.valInt)
	if c.integerExposition {
		return float64(ival % float64Mantissa)
	}
	fval := math.Float64frombits(atomic.LoadUint64(&c.valBits))
	return fval + float64(ival)
}

func (c *counter) Write(out *dto.Metric) error {
	// Read the Exemplar first and the value second. This is to avoid a race condition
	// where users see an exemplar for a not-yet-existing observation.
	var exemplar *dto.Exemplar
	if e := c.exemplar.Load(); e != nil {
		exemplar = e.(*dto.Exemplar)
	}
	val := c.get()
	return populateMetric(CounterValue, val, c.labelPairs, exemplar, out, c.createdTs)
}

func (c *counter) updateExemplar(v float64, l Labels) {
	if l == nil {
		return
	}
	e, err := newExemplar(v, c.now(), l)
	if err != nil {
		panic(err)
	}
	c.exemplar.Store(e)
}

// CounterVec is a Collector that bundles a set of Counters that all share the
// same Desc, but have different values for their variable labels. This is used
// if you want to count the same thing partitioned by various dimensions
// (e.g. number of HTTP requests, partitioned by response code and
// method). Create instances with NewCounterVec.
type CounterVec struct {
	*MetricVec
}

// NewCounterVec creates a new CounterVec based on the provided CounterOpts and
// partitioned by the given label names.
func NewCounterVec(opts CounterOpts, labelNames []string) *CounterVec {
	return V2.NewCounterVec(CounterVecOpts{
		CounterOpts:    opts,
		VariableLabels: UnconstrainedLabels(labelNames),
	})
}

// NewCounterVec creates a new CounterVec based on the provided CounterVecOpts.
func (v2) NewCounterVec(opts CounterVecOpts) *CounterVec {
	desc := V2.NewDesc(
		BuildFQName(opts.Namespace, opts.Subsystem, opts.Name),
		opts.Help,
		opts.VariableLabels,
		opts.ConstLabels,
	)
	if opts.now == nil {
		opts.now = time.Now
	}
	return &CounterVec{
		MetricVec: NewMetricVec(desc, func(lvs ...string) Metric {
			if len(lvs) != len(desc.variableLabels.names) {
				panic(makeInconsistentCardinalityError(desc.fqName, desc.variableLabels.names, lvs))
			}
			result := &counter{desc: desc, labelPairs: MakeLabelPairs(desc, lvs), now: opts.now, integerExposition: opts.IntegerExposition}
			result.init(result) // Init self-collection.
			result.createdTs = timestamppb.New(opts.now())
			return result
		}),
	}
}

// GetMetricWithLabelValues returns the Counter for the given slice of label
// values (same order as the variable labels in Desc). If that combination of
// label values is accessed for the first time, a new Counter is created.
//
// It is possible to call this method without using the returned Counter to only
// create the new Counter but leave it at its starting value 0. See also the
// SummaryVec example.
//
// Keeping the Counter for later use is possible (and should be considered if
// performance is critical), but keep in mind that Reset, DeleteLabelValues and
// Delete can be used to delete the Counter from the CounterVec. In that case,
// the Counter will still exist, but it will not be exported anymore, even if a
// Counter with the same label values is created later.
//
// An error is returned if the number of label values is not the same as the
// number of variable labels in Desc (minus any curried labels).
//
// Note that for more than one label value, this method is prone to mistakes
// caused by an incorrect order of arguments. Consider GetMetricWith(Labels) as
// an alternative to avoid that type of mistake. For higher label numbers, the
// latter has a much more readable (albeit more verbose) syntax, but it comes
// with a performance overhead (for creating and processing the Labels map).
// See also the GaugeVec example.
func (v *CounterVec) GetMetricWithLabelValues(lvs ...string) (Counter, error) {
	metric, err := v.MetricVec.GetMetricWithLabelValues(lvs...)
	if metric != nil {
		return metric.(Counter), err
	}
	return nil, err
}

// GetMetricWith returns the Counter for the given Labels map (the label names
// must match those of the variable labels in Desc). If that label map is
// accessed for the first time, a new Counter is created. Implications of
// creating a Counter without using it and keeping the Counter for later use are
// the same as for GetMetricWithLabelValues.
//
// An error is returned if the number and names of the Labels are inconsistent
// with those of the variable labels in Desc (minus any curried labels).
//
// This method is used for the same purpose as
// GetMetricWithLabelValues(...string). See there for pros and cons of the two
// methods.
func (v *CounterVec) GetMetricWith(labels Labels) (Counter, error) {
	metric, err := v.MetricVec.GetMetricWith(labels)
	if metric != nil {
		return metric.(Counter), err
	}
	return nil, err
}

// WithLabelValues works as GetMetricWithLabelValues, but panics where
// GetMetricWithLabelValues would have returned an error. Not returning an
// error allows shortcuts like
//
//	myVec.WithLabelValues("404", "GET").Add(42)
func (v *CounterVec) WithLabelValues(lvs ...string) Counter {
	c, err := v.GetMetricWithLabelValues(lvs...)
	if err != nil {
		panic(err)
	}
	return c
}

// With works as GetMetricWith, but panics where GetMetricWithLabels would have
// returned an error. Not returning an error allows shortcuts like
//
//	myVec.With(prometheus.Labels{"code": "404", "method": "GET"}).Add(42)
func (v *CounterVec) With(labels Labels) Counter {
	c, err := v.GetMetricWith(labels)
	if err != nil {
		panic(err)
	}
	return c
}

// CurryWith returns a vector curried with the provided labels, i.e. the
// returned vector has those labels pre-set for all labeled operations performed
// on it. The cardinality of the curried vector is reduced accordingly. The
// order of the remaining labels stays the same (just with the curried labels
// taken out of the sequence – which is relevant for the
// (GetMetric)WithLabelValues methods). It is possible to curry a curried
// vector, but only with labels not yet used for currying before.
//
// The metrics contained in the CounterVec are shared between the curried and
// uncurried vectors. They are just accessed differently. Curried and uncurried
// vectors behave identically in terms of collection. Only one must be
// registered with a given registry (usually the uncurried version). The Reset
// method deletes all metrics, even if called on a curried vector.
func (v *CounterVec) CurryWith(labels Labels) (*CounterVec, error) {
	vec, err := v.MetricVec.CurryWith(labels)
	if vec != nil {
		return &CounterVec{vec}, err
	}
	return nil, err
}

// MustCurryWith works as CurryWith but panics where CurryWith would have
// returned an error.
func (v *CounterVec) MustCurryWith(labels Labels) *CounterVec {
	vec, err := v.CurryWith(labels)
	if err != nil {
		panic(err)
	}
	return vec
}

// CounterFunc is a Counter whose value is determined at collect time by calling a
// provided function.
//
// To create CounterFunc instances, use NewCounterFunc.
type CounterFunc interface {
	Metric
	Collector
}

// NewCounterFunc creates a new CounterFunc based on the provided
// CounterOpts. The value reported is determined by calling the given function
// from within the Write method. Take into account that metric collection may
// happen concurrently. If that results in concurrent calls to Write, like in
// the case where a CounterFunc is directly registered with Prometheus, the
// provided function must be concurrency-safe. The function should also honor
// the contract for a Counter (values only go up, not down), but compliance will
// not be checked.
//
// Check out the ExampleGaugeFunc examples for the similar GaugeFunc.
func NewCounterFunc(opts CounterOpts, function func() float64) CounterFunc {
	return newValueFunc(NewDesc(
		BuildFQName(opts.Namespace, opts.Subsystem, opts.Name),
		opts.Help,
		nil,
		opts.ConstLabels,
	), CounterValue, function)
}
