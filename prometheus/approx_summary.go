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
	// The default value is DefObjectives.
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

	if len(opts.Objectives) == 0 {
		opts.Objectives = DefObjectives
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
	*MetricVec
}

// NewApproxSummaryVec creates a new ApproxSummaryVec based on the provided SummaryOpts and
// partitioned by the given label names. At least one label name must be
// provided.
func NewApproxSummaryVec(opts ApproxSummaryOpts, labelNames []string) *ApproxSummaryVec {
	desc := NewDesc(
		BuildFQName(opts.Namespace, opts.Subsystem, opts.Name),
		opts.Help,
		labelNames,
		opts.ConstLabels,
	)
	return &ApproxSummaryVec{
		MetricVec: newMetricVec(desc, func(lvs ...string) Metric {
			return newApproxSummary(desc, opts, lvs...)
		}),
	}
}

// GetMetricWithLabelValues replaces the method of the same name in
// MetricVec. The difference is that this method returns a Summary and not a
// Metric so that no type conversion is required.
func (m *ApproxSummaryVec) GetMetricWithLabelValues(lvs ...string) (Summary, error) {
	metric, err := m.MetricVec.GetMetricWithLabelValues(lvs...)
	if metric != nil {
		return metric.(Summary), err
	}
	return nil, err
}

// GetMetricWith replaces the method of the same name in MetricVec. The
// difference is that this method returns a Summary and not a Metric so that no
// type conversion is required.
func (m *ApproxSummaryVec) GetMetricWith(labels Labels) (Summary, error) {
	metric, err := m.MetricVec.GetMetricWith(labels)
	if metric != nil {
		return metric.(Summary), err
	}
	return nil, err
}

// WithLabelValues works as GetMetricWithLabelValues, but panics where
// GetMetricWithLabelValues would have returned an error. By not returning an
// error, WithLabelValues allows shortcuts like
//     myVec.WithLabelValues("404", "GET").Observe(42.21)
func (m *ApproxSummaryVec) WithLabelValues(lvs ...string) Summary {
	return m.MetricVec.WithLabelValues(lvs...).(Summary)
}

// With works as GetMetricWith, but panics where GetMetricWithLabels would have
// returned an error. By not returning an error, With allows shortcuts like
//     myVec.With(Labels{"code": "404", "method": "GET"}).Observe(42.21)
func (m *ApproxSummaryVec) With(labels Labels) Summary {
	return m.MetricVec.With(labels).(Summary)
}
