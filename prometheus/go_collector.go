package prometheus

import (
	"runtime"
	"runtime/debug"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type goCollector struct {
	*memStatsCollector
	goroutines Gauge
	gcDesc     *Desc
}

// NewGoCollector returns a collector which exports metrics about the current
// go process.
func NewGoCollector() *goCollector {
	return &goCollector{
		goroutines: NewGauge(GaugeOpts{
			Name: "go_goroutines",
			Help: "Number of goroutines that currently exist.",
		}),
		gcDesc: NewDesc(
			"go_gc_duration_seconds",
			"A summary of the GC invocation durations.",
			nil, nil),
		memStatsCollector: newMemStatsCollector(),
	}
}

// Describe returns all descriptions of the collector.
func (c *goCollector) Describe(ch chan<- *Desc) {
	ch <- c.goroutines.Desc()
	ch <- c.gcDesc
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
}

// memStatsCollector collects runtime.MemStats
type memStatsCollector struct {
	alloc        prometheus.Gauge
	totalAlloc   prometheus.Gauge
	sys          prometheus.Gauge
	lookups      prometheus.Gauge
	mallocs      prometheus.Gauge
	frees        prometheus.Gauge
	heapAlloc    prometheus.Gauge
	heapSys      prometheus.Gauge
	heapIdle     prometheus.Gauge
	heapInuse    prometheus.Gauge
	heapReleased prometheus.Gauge
	heapObjects  prometheus.Gauge
}

// newMemStatsCollector creates a new runtime.MemStats collector
func newMemStatsCollector() *memStatsCollector {
	return &MemStatsCollector{
		alloc: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "go",
			Subsystem: "memstats",
			Name:      "alloc_bytes",
			Help:      "bytes allocated and still in use",
		}),
		totalAlloc: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "go",
			Subsystem: "memstats",
			Name:      "alloc_bytes_total",
			Help:      "bytes allocated (even if freed)",
		}),
		sys: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "go",
			Subsystem: "memstats",
			Name:      "sys",
			Help:      "bytes obtained from system",
		}),
		lookups: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "go",
			Subsystem: "memstats",
			Name:      "lookups",
			Help:      "number of pointer lookups",
		}),
		mallocs: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "go",
			Subsystem: "memstats",
			Name:      "mallocs",
			Help:      "number of mallocs",
		}),
		frees: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "go",
			Subsystem: "memstats",
			Name:      "frees",
			Help:      "number of frees",
		}),
		heapAlloc: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "go",
			Subsystem: "memstats",
			Name:      "heap_alloc",
			Help:      "bytes allocated and still in use",
		}),
		heapSys: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "go",
			Subsystem: "memstats",
			Name:      "heap_sys",
			Help:      "bytes obtained from system",
		}),
		heapIdle: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "go",
			Subsystem: "memstats",
			Name:      "heap_idle",
			Help:      "bytes in idle spans",
		}),
		heapInuse: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "go",
			Subsystem: "memstats",
			Name:      "heap_inuse",
			Help:      "bytes in non-idle span",
		}),
		heapReleased: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "go",
			Subsystem: "memstats",
			Name:      "heap_released",
			Help:      "bytes released to the OS",
		}),
		heapObjects: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "go",
			Subsystem: "memstats",
			Name:      "heap_objects",
			Help:      "total number of allocated objects",
		}),
	}
}

// Describe sends Desc objects for each memstat we intend to collect.
func (m *MemStatsCollector) Describe(ch chan<- *prometheus.Desc) {
	if m == nil {
		m = NewMemStatsCollector()
	}

	ch <- m.alloc.Desc()
	ch <- m.totalAlloc.Desc()
	ch <- m.sys.Desc()
	ch <- m.lookups.Desc()
	ch <- m.mallocs.Desc()
	ch <- m.frees.Desc()
	ch <- m.heapAlloc.Desc()
	ch <- m.heapSys.Desc()
	ch <- m.heapIdle.Desc()
	ch <- m.heapInuse.Desc()
	ch <- m.heapReleased.Desc()
	ch <- m.heapObjects.Desc()
}

// Collect does the trick by calling ReadMemStats once and then constructing
// three different Metrics on the fly.
func (m *MemStatsCollector) Collect(ch chan<- prometheus.Metric) {
	if m == nil {
		m = NewMemStatsCollector()
	}

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	m.alloc.Set(float64(ms.Alloc))
	ch <- m.alloc
	m.totalAlloc.Set(float64(ms.TotalAlloc))
	ch <- m.totalAlloc
	m.sys.Set(float64(ms.Sys))
	ch <- m.sys
	m.lookups.Set(float64(ms.Lookups))
	ch <- m.lookups
	m.mallocs.Set(float64(ms.Mallocs))
	ch <- m.mallocs
	m.frees.Set(float64(ms.Frees))
	ch <- m.frees
	m.heapAlloc.Set(float64(ms.HeapAlloc))
	ch <- m.heapAlloc
	m.heapSys.Set(float64(ms.HeapSys))
	ch <- m.heapSys
	m.heapIdle.Set(float64(ms.HeapIdle))
	ch <- m.heapIdle
	m.heapInuse.Set(float64(ms.HeapInuse))
	ch <- m.heapInuse
	m.heapReleased.Set(float64(ms.HeapReleased))
	ch <- m.heapReleased
	m.heapObjects.Set(float64(ms.HeapObjects))
	ch <- m.heapObjects
}
