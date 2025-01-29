package prometheus

import "testing"

func TestCollectorFunc(t *testing.T) {
	testDesc := NewDesc(
		"test_metric",
		"A test metric",
		nil, nil,
	)

	cf := CollectorFunc(func(ch chan<- Metric) {
		ch <- MustNewConstMetric(
			testDesc,
			GaugeValue,
			42.0,
		)
	})

	ch := make(chan Metric, 1)
	cf.Collect(ch)
	close(ch)

	metric := <-ch
	if metric == nil {
		t.Fatal("Expected metric, got nil")
	}

	descCh := make(chan *Desc, 1)
	cf.Describe(descCh)
	close(descCh)

	desc := <-descCh
	if desc == nil {
		t.Fatal("Expected desc, got nil")
	}

	if desc.String() != testDesc.String() {
		t.Fatalf("Expected %s, got %s", testDesc.String(), desc.String())
	}
}

func TestCollectorFuncWithRegistry(t *testing.T) {
	reg := NewPedanticRegistry()

	cf := CollectorFunc(func(ch chan<- Metric) {
		ch <- MustNewConstMetric(
			NewDesc(
				"test_metric",
				"A test metric",
				nil, nil,
			),
			GaugeValue,
			42.0,
		)
	})

	err := reg.Register(cf)
	if err != nil {
		t.Errorf("Failed to register CollectorFunc: %v", err)
	}

	collectedMetrics, err := reg.Gather()
	if err != nil {
		t.Errorf("Failed to gather metrics: %v", err)
	}

	if len(collectedMetrics) != 1 {
		t.Errorf("Expected 1 metric family, got %d", len(collectedMetrics))
	}
}
