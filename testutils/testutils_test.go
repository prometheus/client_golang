package testutils

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestGatherAndCompare(t *testing.T) {
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

	if err := GatherAndCompare(c, metadata+expected, "some_total"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
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

	if err := GatherAndCompare(c, metadata+expected); err != nil {
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

	expectedError := `
metric output does not match expectation; want:

# HELP some_other_metric A value that represents a counter.
# TYPE some_other_metric counter
some_other_metric{label1="value1"} 1


got:

# HELP some_total A value that represents a counter.
# TYPE some_total counter
some_total{label1="value1"} 1

`

	err := GatherAndCompare(c, metadata+expected)
	if err == nil {
		t.Error("Expected error, got no error.")
	}

	if err.Error() != expectedError {
		t.Errorf("Expected\n%#+v\nGot:\n%#+v\n", expectedError, err.Error())
	}
}
