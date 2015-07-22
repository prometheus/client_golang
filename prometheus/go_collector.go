package prometheus

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"time"
)

type goCollector struct {
	goroutines Gauge
	gcDesc     *Desc

	memstats *memStatCollector
}

// NewGoCollector returns a collector which exports metrics about the current
// go process.
func NewGoCollector() *goCollector {
	return &goCollector{
		goroutines: NewGauge(GaugeOpts{
			Namespace: "go",
			Name:      "goroutines",
			Help:      "Number of goroutines that currently exist.",
		}),
		gcDesc: NewDesc(
			"go_gc_duration_seconds",
			"A summary of the GC invocation durations.",
			nil, nil),
		memstats: &memStatCollector{
			ms: new(runtime.MemStats),
			metrics: memStatsMetrics{
				{
					desc: NewDesc(
						memstatNamespace("alloc_bytes"),
						"Number of bytes allocated and still in use.",
						nil, nil,
					),
					eval:    func(ms *runtime.MemStats) float64 { return float64(ms.Alloc) },
					valType: GaugeValue,
				}, {
					desc: NewDesc(
						memstatNamespace("alloc_bytes_total"),
						"Total number of bytes allocated, even if freed.",
						nil, nil,
					),
					eval:    func(ms *runtime.MemStats) float64 { return float64(ms.TotalAlloc) },
					valType: CounterValue,
				}, {
					desc: NewDesc(
						memstatNamespace("sys_bytes"),
						"Number of bytes obtained from system",
						nil, nil,
					),
					eval:    func(ms *runtime.MemStats) float64 { return float64(ms.Sys) },
					valType: CounterValue,
				}, {
					desc: NewDesc(
						memstatNamespace("lookups_total"),
						"Total number of pointer lookups.",
						nil, nil,
					),
					eval:    func(ms *runtime.MemStats) float64 { return float64(ms.Lookups) },
					valType: CounterValue,
				}, {
					desc: NewDesc(
						memstatNamespace("mallocs_total"),
						"Total number of mallocs.",
						nil, nil,
					),
					eval:    func(ms *runtime.MemStats) float64 { return float64(ms.Mallocs) },
					valType: CounterValue,
				}, {
					desc: NewDesc(
						memstatNamespace("frees_total"),
						"Total number of frees.",
						nil, nil,
					),
					eval:    func(ms *runtime.MemStats) float64 { return float64(ms.Frees) },
					valType: CounterValue,
				}, {
					desc: NewDesc(
						memstatNamespace("heap_alloc_bytes"),
						"Number heap bytes allocated and still in use.",
						nil, nil,
					),
					eval:    func(ms *runtime.MemStats) float64 { return float64(ms.HeapAlloc) },
					valType: GaugeValue,
				}, {
					desc: NewDesc(
						memstatNamespace("heap_sys_bytes"),
						"Total bytes in heap obtained from system.",
						nil, nil,
					),
					eval:    func(ms *runtime.MemStats) float64 { return float64(ms.HeapSys) },
					valType: GaugeValue,
				}, {
					desc: NewDesc(
						memstatNamespace("heap_idle_bytes"),
						"Number bytes in heap waiting to be used.",
						nil, nil,
					),
					eval:    func(ms *runtime.MemStats) float64 { return float64(ms.HeapIdle) },
					valType: GaugeValue,
				}, {
					desc: NewDesc(
						memstatNamespace("heap_inuse_bytes"),
						"Number of bytes in heap that are in use.",
						nil, nil,
					),
					eval:    func(ms *runtime.MemStats) float64 { return float64(ms.HeapInuse) },
					valType: GaugeValue,
				}, {
					desc: NewDesc(
						memstatNamespace("heap_released_bytes"),
						"Number of bytes in heap released to OS.",
						nil, nil,
					),
					eval:    func(ms *runtime.MemStats) float64 { return float64(ms.HeapReleased) },
					valType: GaugeValue,
				}, {
					desc: NewDesc(
						memstatNamespace("heap_objects"),
						"Number of allocated objects.",
						nil, nil,
					),
					eval:    func(ms *runtime.MemStats) float64 { return float64(ms.HeapObjects) },
					valType: GaugeValue,
				}, {
					desc: NewDesc(
						memstatNamespace("stack_bytes_inuse"),
						"Number of bytes in use by the stack allocator.",
						nil, nil,
					),
					eval:    func(ms *runtime.MemStats) float64 { return float64(ms.StackInuse) },
					valType: GaugeValue,
				}, {
					desc: NewDesc(
						memstatNamespace("stack_sys_bytes"),
						"Number of bytes in obtained from system for stack allocator.",
						nil, nil,
					),
					eval:    func(ms *runtime.MemStats) float64 { return float64(ms.StackSys) },
					valType: GaugeValue,
				}, {
					desc: NewDesc(
						memstatNamespace("mspan_inuse"),
						"Number of mspan structures in use.",
						nil, nil,
					),
					eval:    func(ms *runtime.MemStats) float64 { return float64(ms.MSpanInuse) },
					valType: GaugeValue,
				}, {
					desc: NewDesc(
						memstatNamespace("mspan_sys"),
						"Number of mspan structures obtained from system.",
						nil, nil,
					),
					eval:    func(ms *runtime.MemStats) float64 { return float64(ms.MSpanSys) },
					valType: GaugeValue,
				}, {
					desc: NewDesc(
						memstatNamespace("mcache_inuse"),
						"Number of mcache structures in use.",
						nil, nil,
					),
					eval:    func(ms *runtime.MemStats) float64 { return float64(ms.MCacheInuse) },
					valType: GaugeValue,
				}, {
					desc: NewDesc(
						memstatNamespace("mcache_sys"),
						"Number of mcache structures obtained from system.",
						nil, nil,
					),
					eval:    func(ms *runtime.MemStats) float64 { return float64(ms.MCacheSys) },
					valType: GaugeValue,
				}, {
					desc: NewDesc(
						memstatNamespace("buck_hash_sys"),
						"Profiling bucket hash table.",
						nil, nil,
					),
					eval:    func(ms *runtime.MemStats) float64 { return float64(ms.BuckHashSys) },
					valType: GaugeValue,
				}, {
					desc: NewDesc(
						memstatNamespace("gc_metadata"),
						"GC metadata.",
						nil, nil,
					),
					eval:    func(ms *runtime.MemStats) float64 { return float64(ms.GCSys) },
					valType: GaugeValue,
				}, {
					desc: NewDesc(
						memstatNamespace("other_sys"),
						"Other system allocations.",
						nil, nil,
					),
					eval:    func(ms *runtime.MemStats) float64 { return float64(ms.OtherSys) },
					valType: GaugeValue,
				}, {
					desc: NewDesc(
						memstatNamespace("next_gc"),
						"Next collection will happen when HeapAlloc ≥ this amount.",
						nil, nil,
					),
					eval:    func(ms *runtime.MemStats) float64 { return float64(ms.NextGC) },
					valType: GaugeValue,
				}, {
					desc: NewDesc(
						memstatNamespace("last_gc"),
						"End time of last garbage collection (nanoseconds since 1970).",
						nil, nil,
					),
					eval:    func(ms *runtime.MemStats) float64 { return float64(ms.LastGC) },
					valType: CounterValue,
				}, {
					desc: NewDesc(
						memstatNamespace("pause_total"),
						"Total garbage collection pauses for all collections.",
						nil, nil,
					),
					eval:    func(ms *runtime.MemStats) float64 { return float64(ms.PauseTotalNs) },
					valType: CounterValue,
				}, {
					desc: NewDesc(
						memstatNamespace("gc_total"),
						"Number of garbage collection.",
						nil, nil,
					),
					eval:    func(ms *runtime.MemStats) float64 { return float64(ms.NumGC) },
					valType: CounterValue,
				},
			},
		},
	}
}

func memstatNamespace(s string) string {
	return fmt.Sprintf("go_memstats_%s", s)
}

// Describe returns all descriptions of the collector.
func (c *goCollector) Describe(ch chan<- *Desc) {
	ch <- c.goroutines.Desc()
	ch <- c.gcDesc

	c.memstats.Describe(ch)
}

// Collect returns the current state of all metrics of the collector.
func (c *goCollector) Collect(ch chan<- Metric) {
	c.goroutines.Set(float64(runtime.NumGoroutine()))
	ch <- c.goroutines

	var stats debug.GCStats
	stats.PauseQuantiles = make([]time.Duration, 5)
	debug.ReadGCStats(&stats)

	quantiles := make(map[float64]float64)
	for idx, pq := range stats.PauseQuantiles[1:] {
		quantiles[float64(idx+1)/float64(len(stats.PauseQuantiles)-1)] = pq.Seconds()
	}
	quantiles[0.0] = stats.PauseQuantiles[0].Seconds()
	ch <- MustNewConstSummary(c.gcDesc, uint64(stats.NumGC), float64(stats.PauseTotal.Seconds()), quantiles)

	c.memstats.Collect(ch)
}

// metrics that provide description, value, and value type for memstat metrics
type memStatsMetrics []struct {
	desc    *Desc
	eval    func(*runtime.MemStats) float64
	valType ValueType
}

type memStatCollector struct {
	// memstats object to reuse
	ms *runtime.MemStats
	// metrics to describe and collect
	metrics memStatsMetrics
}

func (c *memStatCollector) Describe(ch chan<- *Desc) {
	for _, i := range c.metrics {
		ch <- i.desc
	}
}

func (c *memStatCollector) Collect(ch chan<- Metric) {
	runtime.ReadMemStats(c.ms)
	for _, i := range c.metrics {
		ch <- MustNewConstMetric(i.desc, i.valType, i.eval(c.ms))
	}
}
