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
	"fmt"
	"sort"
	"sync"

	"github.com/golang/protobuf/proto"

	dto "github.com/prometheus/client_model/go"
)

// ApproxSummaryOpts bundles the options for creating a Summary metric. It is
// mandatory to set Name and Help to a non-empty string. All other fields are
// optional and can safely be left at their zero value.
type ApproxSummaryOpts struct {
	// Namespace, Subsystem, and Name are components of the fully-qualified
	// name of the Summary (created by joining these components with
	// "_"). Only Name is mandatory, the others merely help structuring the
	// name. Note that the fully-qualified name of the Summary must be a
	// valid Prometheus metric name.
	Namespace string
	Subsystem string
	Name      string

	// Help provides information about this Summary. Mandatory!
	//
	// Metrics with the same fully-qualified name must have the same Help
	// string.
	Help string

	// ConstLabels are used to attach fixed labels to this
	// Summary. Summaries with the same fully-qualified name must have the
	// same label names in their ConstLabels.
	//
	// Note that in most cases, labels have a value that varies during the
	// lifetime of a process. Those labels are usually managed with a
	// ApproxSummaryVec. ConstLabels serve only special purposes. One is for the
	// special case where the value of a label does not change during the
	// lifetime of a process, e.g. if the revision of the running binary is
	// put into a label. Another, more advanced purpose is if more than one
	// Collector needs to collect Summaries with the same fully-qualified
	// name. In that case, those Summaries must differ in the values of
	// their ConstLabels. See the Collector examples.
	//
	// If the value of a label never changes (not even between binaries),
	// that label most likely should not be a label at all (but part of the
	// metric name).
	ConstLabels Labels

	// Objectives defines the quantile rank estimates with their respective
	// absolute error. If Objectives[q] = e, then the value reported
	// for q will be the φ-quantile value for some φ between q-e and q+e.
	// The default value is an empty map, resulting in a summary without
	// quantiles.
	Objectives map[float64]float64

	// Lam is the position convergence rate parameter, which must be in
	// [0-1], although practical values are less than 0.1 or thereabouts.
	Lam float64
	// Gam is the shape convergence rate parameter, which must be in
	// [0-1], although practical values are less than 0.1 or thereabouts.
	Gam float64
	// Rho is the majorisation parameter, which must be in [0-1], although
	// practical values are less than 1e-3. Should be significantly smaller
	// than either of the other two parameters, although this is not
	// checked.
	Rho float64
}

// NewApproxSummary creates a new Summary based on the provided SummaryOpts.
func NewApproxSummary(opts ApproxSummaryOpts) Summary {
	return newApproxSummary(
		NewDesc(
			BuildFQName(opts.Namespace, opts.Subsystem, opts.Name),
			opts.Help,
			nil,
			opts.ConstLabels,
		),
		opts,
	)
}

func newApproxSummary(desc *Desc, opts ApproxSummaryOpts, labelValues ...string) Summary {
	if len(desc.variableLabels) != len(labelValues) {
		panic(errInconsistentCardinality)
	}

	for _, n := range desc.variableLabels {
		if n == quantileLabel {
			panic(errQuantileLabelNotAllowed)
		}
	}
	for _, lp := range desc.constLabelPairs {
		if lp.GetName() == quantileLabel {
			panic(errQuantileLabelNotAllowed)
		}
	}

	if opts.Lam < 0 || opts.Lam > 1 {
		panic(fmt.Errorf("illegal Lambda=%v", opts.Lam))
	}

	if opts.Gam < 0 || opts.Gam > 1 {
		panic(fmt.Errorf("illegal Gamma=%v", opts.Gam))
	}

	if opts.Rho < 0 || opts.Rho > 1 {
		panic(fmt.Errorf("illegal Rho=%v", opts.Rho))
	}

	s := &approxSummary{
		desc: desc,

		objectives:       opts.Objectives,
		sortedObjectives: make([]float64, 0, len(opts.Objectives)),

		labelPairs: makeLabelPairs(desc, labelValues),
	}

	for qu := range s.objectives {
		s.sortedObjectives = append(s.sortedObjectives, qu)
	}
	sort.Float64s(s.sortedObjectives)

	s.tracker = NewMMSPITracker(
		s.sortedObjectives,
		opts.Lam,
		opts.Gam,
		opts.Rho)

	s.init(s) // Init self-collection.
	return s
}

type approxSummary struct {
	selfCollector

	mtx sync.Mutex

	desc *Desc

	objectives       map[float64]float64
	sortedObjectives []float64

	labelPairs []*dto.LabelPair

	sum float64
	cnt uint64

	tracker *MQTracker
}

func (s *approxSummary) Desc() *Desc {
	return s.desc
}

func (s *approxSummary) Observe(v float64) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.tracker.Observe(v)
	s.sum += v
	s.cnt++
}

func (s *approxSummary) Write(out *dto.Metric) error {
	sum := &dto.Summary{}
	qs := make([]*dto.Quantile, 0, len(s.objectives))

	s.mtx.Lock()

	sum.SampleCount = proto.Uint64(s.cnt)
	sum.SampleSum = proto.Float64(s.sum)

	est := s.tracker.Estimate()
	for i, rank := range s.sortedObjectives {
		qs = append(qs, &dto.Quantile{
			Quantile: proto.Float64(rank),
			Value:    proto.Float64(est[i]),
		})
	}

	s.mtx.Unlock()

	if len(qs) > 0 {
		sort.Sort(quantSort(qs))
	}
	sum.Quantile = qs

	out.Summary = sum
	out.Label = s.labelPairs
	return nil
}

// ApproxSummaryVec is a Collector that bundles a set of Summaries that all share the
// same Desc, but have different values for their variable labels. This is used
// if you want to count the same thing partitioned by various dimensions
// (e.g. HTTP request latencies, partitioned by status code and method). Create
// instances with NewApproxSummaryVec.
type ApproxSummaryVec struct {
	*metricVec
}

// NewApproxSummaryVec creates a new ApproxSummaryVec based on the provided ApproxSummaryOpts and
// partitioned by the given label names.
//
// Due to the way a ApproxSummary is represented in the Prometheus text format and how
// it is handled by the Prometheus server internally, “quantile” is an illegal
// label name. NewApproxSummaryVec will panic if this label name is used.
func NewApproxSummaryVec(opts ApproxSummaryOpts, labelNames []string) *ApproxSummaryVec {
	for _, ln := range labelNames {
		if ln == quantileLabel {
			panic(errQuantileLabelNotAllowed)
		}
	}
	desc := NewDesc(
		BuildFQName(opts.Namespace, opts.Subsystem, opts.Name),
		opts.Help,
		labelNames,
		opts.ConstLabels,
	)
	return &ApproxSummaryVec{
		metricVec: newMetricVec(desc, func(lvs ...string) Metric {
			return newApproxSummary(desc, opts, lvs...)
		}),
	}
}

// GetMetricWithLabelValues returns the ApproxSummary for the given slice of label
// values (same order as the VariableLabels in Desc). If that combination of
// label values is accessed for the first time, a new ApproxSummary is created.
//
// It is possible to call this method without using the returned ApproxSummary to only
// create the new ApproxSummary but leave it at its starting value, a ApproxSummary without
// any observations.
//
// Keeping the ApproxSummary for later use is possible (and should be considered if
// performance is critical), but keep in mind that Reset, DeleteLabelValues and
// Delete can be used to delete the ApproxSummary from the ApproxSummaryVec. In that case,
// the ApproxSummary will still exist, but it will not be exported anymore, even if a
// ApproxSummary with the same label values is created later. See also the CounterVec
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
func (v *ApproxSummaryVec) GetMetricWithLabelValues(lvs ...string) (Observer, error) {
	metric, err := v.metricVec.getMetricWithLabelValues(lvs...)
	if metric != nil {
		return metric.(Observer), err
	}
	return nil, err
}

// GetMetricWith returns the ApproxSummary for the given Labels map (the label names
// must match those of the VariableLabels in Desc). If that label map is
// accessed for the first time, a new ApproxSummary is created. Implications of
// creating a ApproxSummary without using it and keeping the ApproxSummary for later use are
// the same as for GetMetricWithLabelValues.
//
// An error is returned if the number and names of the Labels are inconsistent
// with those of the VariableLabels in Desc (minus any curried labels).
//
// This method is used for the same purpose as
// GetMetricWithLabelValues(...string). See there for pros and cons of the two
// methods.
func (v *ApproxSummaryVec) GetMetricWith(labels Labels) (Observer, error) {
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
func (v *ApproxSummaryVec) WithLabelValues(lvs ...string) Observer {
	s, err := v.GetMetricWithLabelValues(lvs...)
	if err != nil {
		panic(err)
	}
	return s
}

// With works as GetMetricWith, but panics where GetMetricWithLabels would have
// returned an error. Not returning an error allows shortcuts like
//     myVec.With(prometheus.Labels{"code": "404", "method": "GET"}).Observe(42.21)
func (v *ApproxSummaryVec) With(labels Labels) Observer {
	s, err := v.GetMetricWith(labels)
	if err != nil {
		panic(err)
	}
	return s
}

// CurryWith returns a vector curried with the provided labels, i.e. the
// returned vector has those labels pre-set for all labeled operations performed
// on it. The cardinality of the curried vector is reduced accordingly. The
// order of the remaining labels stays the same (just with the curried labels
// taken out of the sequence – which is relevant for the
// (GetMetric)WithLabelValues methods). It is possible to curry a curried
// vector, but only with labels not yet used for currying before.
//
// The metrics contained in the ApproxSummaryVec are shared between the curried and
// uncurried vectors. They are just accessed differently. Curried and uncurried
// vectors behave identically in terms of collection. Only one must be
// registered with a given registry (usually the uncurried version). The Reset
// method deletes all metrics, even if called on a curried vector.
func (v *ApproxSummaryVec) CurryWith(labels Labels) (ObserverVec, error) {
	vec, err := v.curryWith(labels)
	if vec != nil {
		return &ApproxSummaryVec{vec}, err
	}
	return nil, err
}

// MustCurryWith works as CurryWith but panics where CurryWith would have
// returned an error.
func (v *ApproxSummaryVec) MustCurryWith(labels Labels) ObserverVec {
	vec, err := v.CurryWith(labels)
	if err != nil {
		panic(err)
	}
	return vec
}
