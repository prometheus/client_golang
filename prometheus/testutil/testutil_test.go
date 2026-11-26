// Copyright 2018 The Prometheus Authors
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

package testutil

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/prometheus/client_golang/prometheus"
)

type untypedCollector struct{}

func (u untypedCollector) Describe(c chan<- *prometheus.Desc) {
	c <- prometheus.NewDesc("name", "help", nil, nil)
}

func (u untypedCollector) Collect(c chan<- prometheus.Metric) {
	c <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("name", "help", nil, nil),
		prometheus.UntypedValue,
		2001,
	)
}

func TestToFloat64(t *testing.T) {
	gaugeWithAValueSet := prometheus.NewGauge(prometheus.GaugeOpts{})
	gaugeWithAValueSet.Set(3.14)

	counterVecWithOneElement := prometheus.NewCounterVec(prometheus.CounterOpts{}, []string{"foo"})
	counterVecWithOneElement.WithLabelValues("bar").Inc()

	counterVecWithTwoElements := prometheus.NewCounterVec(prometheus.CounterOpts{}, []string{"foo"})
	counterVecWithTwoElements.WithLabelValues("bar").Add(42)
	counterVecWithTwoElements.WithLabelValues("baz").Inc()

	histogramVecWithOneElement := prometheus.NewHistogramVec(prometheus.HistogramOpts{}, []string{"foo"})
	histogramVecWithOneElement.WithLabelValues("bar").Observe(2.7)

	scenarios := map[string]struct {
		collector prometheus.Collector
		panics    bool
		want      float64
	}{
		"simple counter": {
			collector: prometheus.NewCounter(prometheus.CounterOpts{}),
			panics:    false,
			want:      0,
		},
		"simple gauge": {
			collector: prometheus.NewGauge(prometheus.GaugeOpts{}),
			panics:    false,
			want:      0,
		},
		"simple untyped": {
			collector: untypedCollector{},
			panics:    false,
			want:      2001,
		},
		"simple histogram": {
			collector: prometheus.NewHistogram(prometheus.HistogramOpts{}),
			panics:    true,
		},
		"simple summary": {
			collector: prometheus.NewSummary(prometheus.SummaryOpts{}),
			panics:    true,
		},
		"simple gauge with an actual value set": {
			collector: gaugeWithAValueSet,
			panics:    false,
			want:      3.14,
		},
		"counter vec with zero elements": {
			collector: prometheus.NewCounterVec(prometheus.CounterOpts{}, nil),
			panics:    true,
		},
		"counter vec with one element": {
			collector: counterVecWithOneElement,
			panics:    false,
			want:      1,
		},
		"counter vec with two elements": {
			collector: counterVecWithTwoElements,
			panics:    true,
		},
		"histogram vec with one element": {
			collector: histogramVecWithOneElement,
			panics:    true,
		},
	}

	for n, s := range scenarios {
		t.Run(n, func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil && s.panics {
					t.Error("expected panic")
				} else if r != nil && !s.panics {
					t.Error("unexpected panic: ", r)
				}
				// Any other combination is the expected outcome.
			}()
			if got := ToFloat64(s.collector); got != s.want {
				t.Errorf("want %f, got %f", s.want, got)
			}
		})
	}
}

func TestCollectAndCompare(t *testing.T) {
	const metadata = `
		# HELP some_total A value that represents a counter.
		# TYPE some_total counter
	`

	c := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "some_total",
		Help: "A value that represents a counter.",
		ConstLabels: prometheus.Labels{
			"label1": "value1",
		},
	})
	c.Inc()

	expected := `

		some_total{ label1 = "value1" } 1
	`

	if err := CollectAndCompare(c, strings.NewReader(metadata+expected), "some_total"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestCollectAndCompareNoLabel(t *testing.T) {
	const metadata = `
		# HELP some_total A value that represents a counter.
		# TYPE some_total counter
	`

	c := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "some_total",
		Help: "A value that represents a counter.",
	})
	c.Inc()

	expected := `

		some_total 1
	`

	if err := CollectAndCompare(c, strings.NewReader(metadata+expected), "some_total"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestCollectAndCompareNoHelp(t *testing.T) {
	const metadata = `
		# TYPE some_total counter
	`

	c := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "some_total",
	})
	c.Inc()

	expected := `

		some_total 1
	`

	if err := CollectAndCompare(c, strings.NewReader(metadata+expected), "some_total"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestCollectAndCompareHistogram(t *testing.T) {
	inputs := []struct {
		name        string
		c           prometheus.Collector
		metadata    string
		expect      string
		observation float64
	}{
		{
			name: "Testing Histogram Collector",
			c: prometheus.NewHistogram(prometheus.HistogramOpts{
				Name:    "some_histogram",
				Help:    "An example of a histogram",
				Buckets: []float64{1, 2, 3},
			}),
			metadata: `
				# HELP some_histogram An example of a histogram
				# TYPE some_histogram histogram
			`,
			expect: `
				some_histogram{le="1"} 0
				some_histogram{le="2"} 0
				some_histogram{le="3"} 1
        			some_histogram_bucket{le="+Inf"} 1
        			some_histogram_sum 2.5
        			some_histogram_count 1

			`,
			observation: 2.5,
		},
		{
			name: "Testing HistogramVec Collector",
			c: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Name:    "some_histogram",
				Help:    "An example of a histogram",
				Buckets: []float64{1, 2, 3},
			}, []string{"test"}),

			metadata: `
				# HELP some_histogram An example of a histogram
				# TYPE some_histogram histogram
			`,
			expect: `
            			some_histogram_bucket{test="test",le="1"} 0
            			some_histogram_bucket{test="test",le="2"} 0
            			some_histogram_bucket{test="test",le="3"} 1
            			some_histogram_bucket{test="test",le="+Inf"} 1
            			some_histogram_sum{test="test"} 2.5
           		 	some_histogram_count{test="test"} 1
		
			`,
			observation: 2.5,
		},
	}

	for _, input := range inputs {
		switch collector := input.c.(type) {
		case prometheus.Histogram:
			collector.Observe(input.observation)
		case *prometheus.HistogramVec:
			collector.WithLabelValues("test").Observe(input.observation)
		default:
			t.Fatalf("unsupported collector tested")

		}

		t.Run(input.name, func(t *testing.T) {
			if err := CollectAndCompare(input.c, strings.NewReader(input.metadata+input.expect)); err != nil {
				t.Errorf("unexpected collecting result:\n%s", err)
			}
		})

	}
}

func TestNoMetricFilter(t *testing.T) {
	const metadata = `
		# HELP some_total A value that represents a counter.
		# TYPE some_total counter
	`

	c := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "some_total",
		Help: "A value that represents a counter.",
		ConstLabels: prometheus.Labels{
			"label1": "value1",
		},
	})
	c.Inc()

	expected := `
		some_total{label1="value1"} 1
	`

	if err := CollectAndCompare(c, strings.NewReader(metadata+expected)); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestMetricNotFound(t *testing.T) {
	const metadata = `
		# HELP some_other_metric A value that represents a counter.
		# TYPE some_other_metric counter
	`

	c := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "some_total",
		Help: "A value that represents a counter.",
		ConstLabels: prometheus.Labels{
			"label1": "value1",
		},
	})

	c.Inc()

	expected := `
		some_other_metric{label1="value1"} 1
	`

	expectedError := `-# HELP some_total A value that represents a counter.
-# TYPE some_total counter
-some_total{label1="value1"} 1
+# HELP some_other_metric A value that represents a counter.
+# TYPE some_other_metric counter
+some_other_metric{label1="value1"} 1
 `

	err := CollectAndCompare(c, strings.NewReader(metadata+expected))
	if err == nil {
		t.Error("Expected error, got no error.")
	}

	if err.Error() != expectedError {
		t.Errorf("Expected\n%#+v\nGot:\n%#+v", expectedError, err.Error())
	}
}

func TestScrapeAndCompare(t *testing.T) {
	const expected = `
		# HELP some_total A value that represents a counter.
		# TYPE some_total counter

		some_total{ label1 = "value1" } 1
	`

	expectedReader := strings.NewReader(expected)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, expected)
	}))
	defer ts.Close()

	if err := ScrapeAndCompare(ts.URL, expectedReader, "some_total"); err != nil {
		t.Errorf("unexpected scraping result:\n%s", err)
	}
}

func TestScrapeAndCompareWithMultipleExpected(t *testing.T) {
	const expected = `
		# HELP some_total A value that represents a counter.
		# TYPE some_total counter

		some_total{ label1 = "value1" } 1

		# HELP some_total2 A value that represents a counter.
		# TYPE some_total2 counter

		some_total2{ label2 = "value2" } 1
	`

	expectedReader := strings.NewReader(expected)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, expected)
	}))
	defer ts.Close()

	if err := ScrapeAndCompare(ts.URL, expectedReader, "some_total2"); err != nil {
		t.Errorf("unexpected scraping result:\n%s", err)
	}
}

func TestScrapeAndCompareFetchingFail(t *testing.T) {
	err := ScrapeAndCompare("some_url", strings.NewReader("some expectation"), "some_total")
	if err == nil {
		t.Errorf("expected an error but got nil")
	}
	if !strings.HasPrefix(err.Error(), "scraping metrics failed") {
		t.Errorf("unexpected error happened: %s", err)
	}
}

func TestScrapeAndCompareBadStatusCode(t *testing.T) {
	const expected = `
		# HELP some_total A value that represents a counter.
		# TYPE some_total counter

		some_total{ label1 = "value1" } 1
	`

	expectedReader := strings.NewReader(expected)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintln(w, expected)
	}))
	defer ts.Close()

	err := ScrapeAndCompare(ts.URL, expectedReader, "some_total")
	if err == nil {
		t.Errorf("expected an error but got nil")
	}
	if !strings.HasPrefix(err.Error(), "the scraping target returned a status code other than 200") {
		t.Errorf("unexpected error happened: %s", err)
	}
}

func TestCollectAndCount(t *testing.T) {
	c := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "some_total",
			Help: "A value that represents a counter.",
		},
		[]string{"foo"},
	)
	if got, want := CollectAndCount(c), 0; got != want {
		t.Errorf("unexpected metric count, got %d, want %d", got, want)
	}
	c.WithLabelValues("bar")
	if got, want := CollectAndCount(c), 1; got != want {
		t.Errorf("unexpected metric count, got %d, want %d", got, want)
	}
	c.WithLabelValues("baz")
	if got, want := CollectAndCount(c), 2; got != want {
		t.Errorf("unexpected metric count, got %d, want %d", got, want)
	}
	if got, want := CollectAndCount(c, "some_total"), 2; got != want {
		t.Errorf("unexpected metric count, got %d, want %d", got, want)
	}
	if got, want := CollectAndCount(c, "some_other_total"), 0; got != want {
		t.Errorf("unexpected metric count, got %d, want %d", got, want)
	}
}

func TestCollectAndFormat(t *testing.T) {
	const text = `# HELP foo_bar A value that represents the number of bars in foo.
# TYPE foo_bar counter
foo_bar{fizz="bang"} 1
`
	const openMetrics = `# HELP foo_bar A value that represents the number of bars in foo.
# TYPE foo_bar unknown
foo_bar{fizz="bang"} 1.0
# EOF
`
	c := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "foo_bar",
			Help: "A value that represents the number of bars in foo.",
		},
		[]string{"fizz"},
	)
	c.WithLabelValues("bang").Inc()

	for _, test := range []struct {
		name       string
		format     expfmt.FormatType
		metricName string
		expected   string
	}{
		{"Text", expfmt.TypeTextPlain, "foo_bar", text},
		{"OpenMetrics", expfmt.TypeOpenMetrics, "foo_bar", openMetrics},
		{"TextNoMatch", expfmt.TypeTextPlain, "missing", ""},
		{"OpenMetricsNoMatch", expfmt.TypeOpenMetrics, "missing", "# EOF\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := CollectAndFormat(c, test.format, test.metricName)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if string(got) != test.expected {
				t.Errorf("unexpected metric output, got %q, expected %q", got, test.expected)
			}
		})
	}
}

func TestGatherAndFormat(t *testing.T) {
	const text = `# HELP foo_bar A value that represents the number of bars in foo.
# TYPE foo_bar counter
foo_bar{fizz="bang"} 1
`
	const openMetrics = `# HELP foo_bar A value that represents the number of bars in foo.
# TYPE foo_bar unknown
foo_bar{fizz="bang"} 1.0
`
	const otherOpenMetrics = `# HELP other A metric that should be filtered out.
# TYPE other counter
other_total 1.0
`
	reg := prometheus.NewPedanticRegistry()
	want := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "foo_bar",
			Help: "A value that represents the number of bars in foo.",
		},
		[]string{"fizz"},
	)
	other := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "other_total",
		Help: "A metric that should be filtered out.",
	})
	if err := reg.Register(want); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(other); err != nil {
		t.Fatal(err)
	}
	want.WithLabelValues("bang").Inc()
	other.Inc()

	for _, test := range []struct {
		name        string
		gatherer    prometheus.Gatherer
		format      expfmt.FormatType
		metricNames []string
		expected    string
	}{
		{"Text", reg, expfmt.TypeTextPlain, []string{"foo_bar"}, text},
		{"OpenMetrics", reg, expfmt.TypeOpenMetrics, []string{"foo_bar"}, openMetrics + "# EOF\n"},
		{"OpenMetricsMultipleFamilies", reg, expfmt.TypeOpenMetrics, []string{"foo_bar", "other_total"}, openMetrics + otherOpenMetrics + "# EOF\n"},
		{"OpenMetricsNoNames", reg, expfmt.TypeOpenMetrics, nil, openMetrics + otherOpenMetrics + "# EOF\n"},
		{"TextNoMatch", reg, expfmt.TypeTextPlain, []string{"missing"}, ""},
		{"OpenMetricsNoMatch", reg, expfmt.TypeOpenMetrics, []string{"missing"}, "# EOF\n"},
		{"TextEmptyRegistry", prometheus.NewPedanticRegistry(), expfmt.TypeTextPlain, []string{"foo_bar"}, ""},
		{"OpenMetricsEmptyRegistry", prometheus.NewPedanticRegistry(), expfmt.TypeOpenMetrics, []string{"foo_bar"}, "# EOF\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := GatherAndFormat(test.gatherer, test.format, test.metricNames...)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if string(got) != test.expected {
				t.Errorf("unexpected metric output, got %q, expected %q", got, test.expected)
			}
		})
	}
}

func TestGatherAndCompareWithOptions(t *testing.T) {
	omFmt, err := expfmt.NewOpenMetricsFormat("1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	reg := prometheus.NewPedanticRegistry()
	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "my_counter_total",
		Help: "Some counter with exemplar",
	})
	if err := reg.Register(counter); err != nil {
		t.Fatal(err)
	}
	counter.(prometheus.ExemplarAdder).AddWithExemplar(1, prometheus.Labels{"trace_id": "abc"})

	expected := `# HELP my_counter Some counter with exemplar
# TYPE my_counter counter
my_counter_total 1.0 # {trace_id="abc"} 1.0
# EOF
`

	// Compare with OpenMetrics format and ignore exemplar timestamp.
	if err := GatherAndCompareWithOptions(reg, strings.NewReader(expected), WithFormat(omFmt), WithIgnoreExemplarTimestamps()); err != nil {
		t.Fatalf("unexpected GatherAndCompareWithOptions failure: %v", err)
	}

	// Compare with mismatched exemplar label should fail.
	mismatched := `# HELP my_counter Some counter with exemplar
# TYPE my_counter counter
my_counter_total 1.0 # {trace_id="wrong"} 1.0
# EOF
`
	if err := GatherAndCompareWithOptions(reg, strings.NewReader(mismatched), WithFormat(omFmt), WithIgnoreExemplarTimestamps()); err == nil {
		t.Fatal("expected failure due to mismatched exemplar, got nil")
	}

	// Test CollectAndCompareWithOptions with exemplar.
	c := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "collected_total",
		Help: "A collected counter with exemplar",
	})
	c.(prometheus.ExemplarAdder).AddWithExemplar(42, prometheus.Labels{"span_id": "xyz"})

	expectedCollected := `# HELP collected A collected counter with exemplar
# TYPE collected counter
collected_total 42.0 # {span_id="xyz"} 42.0
# EOF
`
	if err := CollectAndCompareWithOptions(c, strings.NewReader(expectedCollected), WithFormat(omFmt), WithIgnoreExemplarTimestamps()); err != nil {
		t.Fatalf("unexpected CollectAndCompareWithOptions failure: %v", err)
	}

	// Test WithMetricNames filtering with format option.
	other := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "other_counter_total",
		Help: "Other counter",
	})
	other.Inc()
	if err := reg.Register(other); err != nil {
		t.Fatal(err)
	}

	if err := GatherAndCompareWithOptions(reg, strings.NewReader(expected), WithFormat(omFmt), WithIgnoreExemplarTimestamps(), WithMetricNames("my_counter_total")); err != nil {
		t.Fatalf("unexpected GatherAndCompareWithOptions with metric filter failure: %v", err)
	}

	// Test Histogram with exemplar and WithIgnoreExemplarTimestamps.
	hist := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "my_hist",
		Help:    "A histogram",
		Buckets: []float64{1, 5, 10},
	})
	hist.(prometheus.ExemplarObserver).ObserveWithExemplar(2.5, prometheus.Labels{"trace_id": "xyz"})

	expectedHist := `# HELP my_hist A histogram
# TYPE my_hist histogram
my_hist_bucket{le="1.0"} 0
my_hist_bucket{le="5.0"} 1 # {trace_id="xyz"} 2.5
my_hist_bucket{le="10.0"} 1
my_hist_bucket{le="+Inf"} 1
my_hist_sum 2.5
my_hist_count 1
# EOF
`
	if err := CollectAndCompareWithOptions(hist, strings.NewReader(expectedHist), WithFormat(omFmt), WithIgnoreExemplarTimestamps()); err != nil {
		t.Fatalf("unexpected CollectAndCompareWithOptions histogram failure: %v", err)
	}

	// Test exemplar with explicit timestamp matching.
	collectorWithTimestamp := &fixedExemplarCollector{
		desc: prometheus.NewDesc("ts_metric_total", "A counter with timestamp", nil, nil),
	}
	expectedWithTimestamp := `# HELP ts_metric A counter with timestamp
# TYPE ts_metric counter
ts_metric_total 1.0 # {trace_id="fixed"} 1.0 1600000000.000
# EOF
`
	if err := CollectAndCompareWithOptions(collectorWithTimestamp, strings.NewReader(expectedWithTimestamp), WithFormat(omFmt)); err != nil {
		t.Fatalf("unexpected CollectAndCompareWithOptions with timestamp failure: %v", err)
	}

	expectedWrongTimestamp := `# HELP ts_metric A counter with timestamp
# TYPE ts_metric counter
ts_metric_total 1.0 # {trace_id="fixed"} 1.0 1700000000.000
# EOF
`
	if err := CollectAndCompareWithOptions(collectorWithTimestamp, strings.NewReader(expectedWrongTimestamp), WithFormat(omFmt)); err == nil {
		t.Fatal("expected CollectAndCompareWithOptions to fail due to timestamp mismatch, got nil")
	}

	// Test TransactionalGatherAndCompareWithOptions.
	txGatherer := prometheus.ToTransactionalGatherer(reg)
	if err := TransactionalGatherAndCompareWithOptions(txGatherer, strings.NewReader(expected), WithFormat(omFmt), WithIgnoreExemplarTimestamps(), WithMetricNames("my_counter_total")); err != nil {
		t.Fatalf("unexpected TransactionalGatherAndCompareWithOptions failure: %v", err)
	}
}

type fixedExemplarCollector struct {
	desc *prometheus.Desc
}

func (c *fixedExemplarCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

func (c *fixedExemplarCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- &fixedExemplarMetric{desc: c.desc}
}

type fixedExemplarMetric struct {
	desc *prometheus.Desc
}

func (m *fixedExemplarMetric) Desc() *prometheus.Desc { return m.desc }

func (m *fixedExemplarMetric) Write(out *dto.Metric) error {
	val := 1.0
	out.Counter = &dto.Counter{
		Value: &val,
		Exemplar: &dto.Exemplar{
			Label: []*dto.LabelPair{
				{Name: new("trace_id"), Value: new("fixed")},
			},
			Value:     new(1.0),
			Timestamp: timestamppb.New(time.Unix(1600000000, 0)),
		},
	}
	return nil
}
