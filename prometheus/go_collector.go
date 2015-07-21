package prometheus

import (
	"runtime"
	"runtime/debug"
	"time"
)

type goCollector struct {
	goroutines   Gauge
	gcDesc       *Desc
	alloc        Gauge
	totalAlloc   Counter
	sys          Counter
	lookups      Counter
	mallocs      Counter
	frees        Counter
	heapAlloc    Gauge
	heapSys      Gauge
	heapIdle     Gauge
	heapInuse    Gauge
	heapReleased Gauge
	heapObjects  Gauge
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
		alloc: NewGauge(GaugeOpts{
			Namespace: "go",
			Subsystem: "memstats",
			Name:      "alloc_bytes",
			Help:      "Number of bytes allocated and still in use.",
		}),
		totalAlloc: NewCounter(CounterOpts{
			Namespace: "go",
			Subsystem: "memstats",
			Name:      "alloc_bytes_total",
			Help:      "Total number of bytes allocated, even if freed.",
		}),
		sys: NewGauge(GaugeOpts{
			Namespace: "go",
			Subsystem: "memstats",
			Name:      "sys_bytes",
			Help:      "Number of bytes obtained from system",
		}),
		lookups: NewCounter(CounterOpts{
			Namespace: "go",
			Subsystem: "memstats",
			Name:      "lookups_total",
			Help:      "Total number of pointer lookups.",
		}),
		mallocs: NewCounter(CounterOpts{
			Namespace: "go",
			Subsystem: "memstats",
			Name:      "mallocs_total",
			Help:      "Total number of mallocs.",
		}),
		frees: NewCounter(CounterOpts{
			Namespace: "go",
			Subsystem: "memstats",
			Name:      "frees_total",
			Help:      "Total number of frees.",
		}),
		heapAlloc: NewGauge(GaugeOpts{
			Namespace: "go",
			Subsystem: "memstats",
			Name:      "heap_alloc_bytes",
			Help:      "Number heap bytes allocated and still in use.",
		}),
		heapSys: NewGauge(GaugeOpts{
			Namespace: "go",
			Subsystem: "memstats",
			Name:      "heap_sys_bytes",
			Help:      "Total bytes in heap obtained from system.",
		}),
		heapIdle: NewGauge(GaugeOpts{
			Namespace: "go",
			Subsystem: "memstats",
			Name:      "heap_idle_bytes",
			Help:      "Number bytes in heap waiting to be used.",
		}),
		heapInuse: NewGauge(GaugeOpts{
			Namespace: "go",
			Subsystem: "memstats",
			Name:      "heap_inuse_bytes",
			Help:      "Number of bytes in heap that are in use.",
		}),
		heapReleased: NewGauge(GaugeOpts{
			Namespace: "go",
			Subsystem: "memstats",
			Name:      "heap_released_bytes",
			Help:      "Number of bytes in heap released to OS.",
		}),
		heapObjects: NewGauge(GaugeOpts{
			Namespace: "go",
			Subsystem: "memstats",
			Name:      "heap_objects",
			Help:      "Number of allocated objects.",
		}),
	}
}

// Describe returns all descriptions of the collector.
func (c *goCollector) Describe(ch chan<- *Desc) {
	ch <- c.goroutines.Desc()
	ch <- c.gcDesc
	ch <- c.alloc.Desc()
	ch <- c.totalAlloc.Desc()
	ch <- c.sys.Desc()
	ch <- c.lookups.Desc()
	ch <- c.mallocs.Desc()
	ch <- c.frees.Desc()
	ch <- c.heapAlloc.Desc()
	ch <- c.heapSys.Desc()
	ch <- c.heapIdle.Desc()
	ch <- c.heapInuse.Desc()
	ch <- c.heapReleased.Desc()
	ch <- c.heapObjects.Desc()
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

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	c.alloc.Set(float64(ms.Alloc))
	ch <- c.alloc
	c.totalAlloc.Set(float64(ms.TotalAlloc))
	ch <- c.totalAlloc
	c.sys.Set(float64(ms.Sys))
	ch <- c.sys
	c.lookups.Set(float64(ms.Lookups))
	ch <- c.lookups
	c.mallocs.Set(float64(ms.Mallocs))
	ch <- c.mallocs
	c.frees.Set(float64(ms.Frees))
	ch <- c.frees
	c.heapAlloc.Set(float64(ms.HeapAlloc))
	ch <- c.heapAlloc
	c.heapSys.Set(float64(ms.HeapSys))
	ch <- c.heapSys
	c.heapIdle.Set(float64(ms.HeapIdle))
	ch <- c.heapIdle
	c.heapInuse.Set(float64(ms.HeapInuse))
	ch <- c.heapInuse
	c.heapReleased.Set(float64(ms.HeapReleased))
	ch <- c.heapReleased
	c.heapObjects.Set(float64(ms.HeapObjects))
	ch <- c.heapObjects
}
