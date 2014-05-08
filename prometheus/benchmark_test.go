package prometheus

import (
	"testing"
)

func BenchmarkPrometheusCounter(b *testing.B) {
	m := MustNewCounterVec(&Desc{
		Name:           "benchmark_counter",
		Help:           "A counter to benchmark it.",
		VariableLabels: []string{"one", "two", "three"},
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.WithLabelValues("zwei", "eins", "drei").Inc()
	}
}

func BenchmarkPrometheusCounterNoLabels(b *testing.B) {
	m := MustNewCounter(&Desc{
		Name: "benchmark_counter",
		Help: "A counter to benchmark it.",
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Inc()
	}
}

func BenchmarkPrometheusGauge(b *testing.B) {
	m := MustNewGaugeVec(&Desc{
		Name:           "benchmark_gauge",
		Help:           "A gauge to benchmark it.",
		VariableLabels: []string{"one", "two", "three"},
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.WithLabelValues("eins", "zwei", "drei").Set(3.1415)
	}
}

func BenchmarkPrometheusGaugeNoLabels(b *testing.B) {
	m := MustNewGauge(&Desc{
		Name: "benchmark_gauge",
		Help: "A gauge to benchmark it.",
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Set(3.1415)
	}
}

func BenchmarkPrometheusSummary(b *testing.B) {
	m := MustNewSummaryVec(
		&Desc{
			Name:           "benchmark_summary",
			Help:           "A summary to benchmark it.",
			VariableLabels: []string{"one", "two", "three"},
		},
		&SummaryOptions{},
	)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.WithLabelValues("eins", "zwei", "drei").Observe(3.1415)
	}
}

func BenchmarkPrometheusSummaryNoLabels(b *testing.B) {
	m := MustNewSummary(
		&Desc{
			Name: "benchmark_summary",
			Help: "A summary to benchmark it.",
		},
		&SummaryOptions{},
	)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Observe(3.1415)
	}
}
