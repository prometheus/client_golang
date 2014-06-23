// Copyright 2014 Prometheus Team
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
	"hash/fnv"
	"sort"
	"sync"
	"time"

	"code.google.com/p/goprotobuf/proto"

	dto "github.com/prometheus/client_model/go"

	"github.com/prometheus/client_golang/_vendor/perks/quantile"
)

// A Summary captures individual observations from an event or sample stream and
// summarizes them in a manner similar to traditional summary statistics: 1. sum
// of observations, 2. observation count, 3. rank estimations.
//
// A typical use-case is the observation of request latencies. By default, a
// Summary provides the median, the 90th and the 99th percentile of the latency
// as rank estimations.
//
// To create Summary instances, use NewSummary.
type Summary interface {
	Metric
	Collector

	// Observe adds a single observation to the summary.
	Observe(float64)
}

// DefObjectives are the default Summary quantile values.
var (
	DefObjectives = []float64{0.5, 0.9, 0.99}
)

// Default values for SummaryOpts.
const (
	// DefMaxAge is the default duration for which observations stay
	// relevant.
	DefMaxAge time.Duration = 10 * time.Minute
	// DefAgeBuckets is the default number of buckets used to calculate the
	// age of observations.
	DefAgeBuckets = 10
	// DefBufCap is the standard buffer size for collecting Summary observations.
	DefBufCap = 500
	// DefEpsilon is the default error epsilon for the quantile rank estimates.
	DefEpsilon = 0.001
)

// SummaryOpts bundles the options for creating a Summary metric. It is
// mandatory to set Name and Help to a non-empty string. All other fields are
// optional and can safely be left at their zero value.
type SummaryOpts struct {
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
	// SummaryVec. ConstLabels serve only special purposes. One is for the
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

	// Objectives defines the quantile rank estimates. The default value is
	// DefObjectives.
	Objectives []float64

	// MaxAge defines the duration for which an observation stays relevant
	// for the summary. Must be positive. The default value is DefMaxAge.
	MaxAge time.Duration

	// AgeBuckets is the number of buckets used to exclude observations that
	// are older than MaxAge from the summary. A higher number has a
	// resource penalty, so only increase it if the higher resolution is
	// really required. The default value is DefAgeBuckets.
	AgeBuckets uint32

	// BufCap defines the default sample stream buffer size.  The default
	// value of DefBufCap should suffice for most uses. If there is a need
	// to increase the value, a multiple of 500 is recommended (because that
	// is the internal buffer size of the underlying package
	// "github.com/bmizerany/perks/quantile").
	BufCap uint32

	// Epsilon is the error epsilon for the quantile rank estimate. Must be
	// positive. The default is DefEpsilon.
	Epsilon float64
}

// NewSummary creates a new Summary based on the provided SummaryOpts.
func NewSummary(opts SummaryOpts) Summary {
	return newSummary(
		NewDesc(
			BuildFQName(opts.Namespace, opts.Subsystem, opts.Name),
			opts.Help,
			nil,
			opts.ConstLabels,
		),
		opts,
	)
}

func newSummary(desc *Desc, opts SummaryOpts, labelValues ...string) Summary {
	if len(desc.variableLabels) != len(labelValues) {
		panic(errInconsistentCardinality)
	}

	if len(opts.Objectives) == 0 {
		opts.Objectives = DefObjectives
	}

	if opts.MaxAge < 0 {
		panic(fmt.Errorf("illegal max age MaxAge=%v", opts.MaxAge))
	}
	if opts.MaxAge == 0 {
		opts.MaxAge = DefMaxAge
	}

	if opts.AgeBuckets == 0 {
		opts.AgeBuckets = DefAgeBuckets
	}

	if opts.BufCap == 0 {
		opts.BufCap = DefBufCap
	}

	if opts.Epsilon < 0 {
		panic(fmt.Errorf("illegal value for Epsilon=%f", opts.Epsilon))
	}
	if opts.Epsilon == 0. {
		opts.Epsilon = DefEpsilon
	}

	s := &summary{
		desc: desc,

		objectives: opts.Objectives,
		epsilon:    opts.Epsilon,

		labelPairs: makeLabelPairs(desc, labelValues),

		hotBuf:         make([]float64, 0, opts.BufCap),
		coldBuf:        make([]float64, 0, opts.BufCap),
		streamDuration: opts.MaxAge / time.Duration(opts.AgeBuckets),
	}
	s.mergedTailStreams = s.newStream()
	s.mergedAllStreams = s.newStream()
	s.headStreamExpTime = time.Now().Add(s.streamDuration)
	s.hotBufExpTime = s.headStreamExpTime

	for i := uint32(0); i < opts.AgeBuckets; i++ {
		s.streams = append(s.streams, s.newStream())
	}
	s.headStream = s.streams[0]

	s.Init(s) // Init self-collection.
	return s
}

type summary struct {
	SelfCollector

	bufMtx sync.Mutex // Protects hotBuf and hotBufExpTime.
	mtx    sync.Mutex // Protects every other moving part.
	// Lock bufMtx before mtx if both are needed.

	desc *Desc

	objectives []float64
	epsilon    float64

	labelPairs []*dto.LabelPair

	sum float64
	cnt uint64

	hotBuf, coldBuf []float64

	streams                          []*quantile.Stream
	streamDuration                   time.Duration
	headStreamIdx                    int
	headStreamExpTime, hotBufExpTime time.Time

	headStream, mergedTailStreams, mergedAllStreams *quantile.Stream
}

func (s *summary) Desc() *Desc {
	return s.desc
}

func (s *summary) Observe(v float64) {
	s.bufMtx.Lock()
	defer s.bufMtx.Unlock()

	now := time.Now()
	if now.After(s.hotBufExpTime) {
		s.asyncFlush(now)
	}
	s.hotBuf = append(s.hotBuf, v)
	if len(s.hotBuf) == cap(s.hotBuf) {
		s.asyncFlush(now)
	}
}

func (s *summary) Write(out *dto.Metric) {
	sum := &dto.Summary{}
	qs := make([]*dto.Quantile, 0, len(s.objectives))

	s.bufMtx.Lock()
	s.mtx.Lock()

	if len(s.hotBuf) != 0 {
		s.swapBufs(time.Now())
	}
	s.bufMtx.Unlock()

	s.flushColdBuf()
	s.mergedAllStreams.Merge(s.mergedTailStreams.Samples())
	s.mergedAllStreams.Merge(s.headStream.Samples())
	sum.SampleCount = proto.Uint64(s.cnt)
	sum.SampleSum = proto.Float64(s.sum)

	for _, rank := range s.objectives {
		qs = append(qs, &dto.Quantile{
			Quantile: proto.Float64(rank),
			Value:    proto.Float64(s.mergedAllStreams.Query(rank)),
		})
	}
	s.mergedAllStreams.Reset()

	s.mtx.Unlock()

	if len(qs) > 0 {
		sort.Sort(quantSort(qs))
	}
	sum.Quantile = qs

	out.Summary = sum
	out.Label = s.labelPairs
}

func (s *summary) newStream() *quantile.Stream {
	stream := quantile.NewTargeted(s.objectives...)
	stream.SetEpsilon(s.epsilon)
	return stream
}

// asyncFlush needs bufMtx locked.
func (s *summary) asyncFlush(now time.Time) {
	s.mtx.Lock()
	s.swapBufs(now)

	// Unblock the original goroutine that was responsible for the mutation
	// that triggered the compaction.  But hold onto the global non-buffer
	// state mutex until the operation finishes.
	go func() {
		s.flushColdBuf()
		s.mtx.Unlock()
	}()
}

// rotateStreams needs mtx AND bufMtx locked.
func (s *summary) maybeRotateStreams() {
	if s.hotBufExpTime.Equal(s.headStreamExpTime) {
		// Fast return to avoid re-merging s.mergedTailStreams.
		return
	}
	for !s.hotBufExpTime.Equal(s.headStreamExpTime) {
		s.headStreamIdx++
		if s.headStreamIdx >= len(s.streams) {
			s.headStreamIdx = 0
		}
		s.headStream = s.streams[s.headStreamIdx]
		s.headStream.Reset()
		s.headStreamExpTime = s.headStreamExpTime.Add(s.streamDuration)
	}
	s.mergedTailStreams.Reset()
	for _, stream := range s.streams {
		if stream != s.headStream {
			s.mergedTailStreams.Merge(stream.Samples())
		}
	}

}

// flushColdBuf needs mtx locked.
func (s *summary) flushColdBuf() {
	for _, v := range s.coldBuf {
		s.headStream.Insert(v)
		s.cnt++
		s.sum += v
	}
	s.coldBuf = s.coldBuf[0:0]
	s.maybeRotateStreams()
}

// swapBufs needs mtx AND bufMtx locked, coldBuf must be empty.
func (s *summary) swapBufs(now time.Time) {
	s.hotBuf, s.coldBuf = s.coldBuf, s.hotBuf
	// hotBuf is now empty and gets new expiration set.
	for now.After(s.hotBufExpTime) {
		s.hotBufExpTime = s.hotBufExpTime.Add(s.streamDuration)
	}
}

type quantSort []*dto.Quantile

func (s quantSort) Len() int {
	return len(s)
}

func (s quantSort) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s quantSort) Less(i, j int) bool {
	return s[i].GetQuantile() < s[j].GetQuantile()
}

// SummaryVec is a Collector that bundles a set of Summaries that all share the
// same Desc, but have different values for their variable labels. This is used
// if you want to count the same thing partitioned by various dimensions
// (e.g. http request latencies, partitioned by status code and method). Create
// instances with NewSummaryVec.
type SummaryVec struct {
	MetricVec
}

// NewSummaryVec creates a new SummaryVec based on the provided SummaryOpts and
// partitioned by the given label names. At least one label name must be
// provided.
func NewSummaryVec(opts SummaryOpts, labelNames []string) *SummaryVec {
	desc := NewDesc(
		BuildFQName(opts.Namespace, opts.Subsystem, opts.Name),
		opts.Help,
		labelNames,
		opts.ConstLabels,
	)
	return &SummaryVec{
		MetricVec: MetricVec{
			children: map[uint64]Metric{},
			desc:     desc,
			hash:     fnv.New64a(),
			newMetric: func(lvs ...string) Metric {
				return newSummary(desc, opts, lvs...)
			},
		},
	}
}

// GetMetricWithLabelValues replaces the method of the same name in
// MetricVec. The difference is that this method returns a Summary and not a
// Metric so that no type conversion is required.
func (m *SummaryVec) GetMetricWithLabelValues(lvs ...string) (Summary, error) {
	metric, err := m.MetricVec.GetMetricWithLabelValues(lvs...)
	if metric != nil {
		return metric.(Summary), err
	}
	return nil, err
}

// GetMetricWith replaces the method of the same name in MetricVec. The
// difference is that this method returns a Summary and not a Metric so that no
// type conversion is required.
func (m *SummaryVec) GetMetricWith(labels Labels) (Summary, error) {
	metric, err := m.MetricVec.GetMetricWith(labels)
	if metric != nil {
		return metric.(Summary), err
	}
	return nil, err
}

// WithLabelValues works as GetMetricWithLabelValues, but panics where
// GetMetricWithLabelValues would have returned an error. By not returning an
// error, WithLabelValues allows shortcuts like
//     myVec.WithLabelValues("404", "GET").Add(42)
func (m *SummaryVec) WithLabelValues(lvs ...string) Summary {
	return m.MetricVec.WithLabelValues(lvs...).(Summary)
}

// With works as GetMetricWith, but panics where GetMetricWithLabels would have
// returned an error. By not returning an error, With allows shortcuts like
//     myVec.With(Labels{"code": "404", "method": "GET"}).Add(42)
func (m *SummaryVec) With(labels Labels) Summary {
	return m.MetricVec.With(labels).(Summary)
}
