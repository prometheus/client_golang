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

package prometheus

import (
	"runtime"
	"runtime/debug"
	"time"
)

// goRuntimeMemStats provides the metrics initially provided by runtime.ReadMemStats.
// From Go 1.17 those similar (and better) statistics are provided by runtime/metrics, so
// while eval closure works on runtime.MemStats, the struct from Go 1.17+ is
// populated using runtime/metrics. Those are the defaults we can't alter.
func goRuntimeMemStats() memStatsMetrics {
	const ns = "go_memstats" // see memstatNamespace(string)
	return memStatsMetrics{
		{
			eval:   func(ms *runtime.MemStats) float64 { return float64(ms.Alloc) },
			metric: NewGauge(GaugeOpts{Namespace: ns, Name: "alloc_bytes", Help: "Number of bytes allocated in heap and currently in use. Equals to /memory/classes/heap/objects:bytes."}),
		}, {
			eval:   func(ms *runtime.MemStats) float64 { return float64(ms.TotalAlloc) },
			metric: NewCounter(CounterOpts{Namespace: ns, Name: "alloc_bytes_total", Help: "Total number of bytes allocated in heap until now, even if released already. Equals to /gc/heap/allocs:bytes."}),
		}, {
			eval:   func(ms *runtime.MemStats) float64 { return float64(ms.Sys) },
			metric: NewGauge(GaugeOpts{Namespace: ns, Name: "sys_bytes", Help: "Number of bytes obtained from system. Equals to /memory/classes/total:byte."}),
		}, {
			eval: func(ms *runtime.MemStats) float64 { return float64(ms.Mallocs) },
			// TODO(bwplotka): We could add go_memstats_heap_objects, probably useful for discovery. Let's gather more feedback, kind of a waste of bytes for everybody for compatibility reasons to keep both, and we can't really rename/remove useful metric.
			metric: NewCounter(CounterOpts{Namespace: ns, Name: "mallocs_total", Help: "Total number of heap objects allocated, both live and gc-ed. Semantically a counter version for go_memstats_heap_objects gauge. Equals to /gc/heap/allocs:objects + /gc/heap/tiny/allocs:objects."}),
		}, {
			eval:   func(ms *runtime.MemStats) float64 { return float64(ms.Frees) },
			metric: NewCounter(CounterOpts{Namespace: ns, Name: "frees_total", Help: "Total number of heap objects frees. Equals to /gc/heap/frees:objects + /gc/heap/tiny/allocs:objects."}),
		}, {
			eval:   func(ms *runtime.MemStats) float64 { return float64(ms.HeapAlloc) },
			metric: NewGauge(GaugeOpts{Namespace: ns, Name: "heap_alloc_bytes", Help: "Number of heap bytes allocated and currently in use, same as go_memstats_alloc_bytes. Equals to /memory/classes/heap/objects:bytes."}),
		}, {
			eval:   func(ms *runtime.MemStats) float64 { return float64(ms.HeapSys) },
			metric: NewGauge(GaugeOpts{Namespace: ns, Name: "heap_sys_bytes", Help: "Number of heap bytes obtained from system. Equals to /memory/classes/heap/objects:bytes + /memory/classes/heap/unused:bytes + /memory/classes/heap/released:bytes + /memory/classes/heap/free:bytes."}),
		}, {
			eval:   func(ms *runtime.MemStats) float64 { return float64(ms.HeapIdle) },
			metric: NewGauge(GaugeOpts{Namespace: ns, Name: "heap_idle_bytes", Help: "Number of heap bytes waiting to be used. Equals to /memory/classes/heap/released:bytes + /memory/classes/heap/free:bytes."}),
		}, {
			eval:   func(ms *runtime.MemStats) float64 { return float64(ms.HeapInuse) },
			metric: NewGauge(GaugeOpts{Namespace: ns, Name: "heap_inuse_bytes", Help: "Number of heap bytes that are in use. Equals to /memory/classes/heap/objects:bytes + /memory/classes/heap/unused:bytes"}),
		}, {
			eval:   func(ms *runtime.MemStats) float64 { return float64(ms.HeapReleased) },
			metric: NewGauge(GaugeOpts{Namespace: ns, Name: "heap_released_bytes", Help: "Number of heap bytes released to OS. Equals to /memory/classes/heap/released:bytes."}),
		}, {
			eval:   func(ms *runtime.MemStats) float64 { return float64(ms.HeapObjects) },
			metric: NewGauge(GaugeOpts{Namespace: ns, Name: "heap_objects", Help: "Number of currently allocated objects. Equals to /gc/heap/objects:objects."}),
		}, {
			eval:   func(ms *runtime.MemStats) float64 { return float64(ms.StackInuse) },
			metric: NewGauge(GaugeOpts{Namespace: ns, Name: "stack_inuse_bytes", Help: "Number of bytes obtained from system for stack allocator in non-CGO environments. Equals to /memory/classes/heap/stacks:bytes."}),
		}, {
			eval:   func(ms *runtime.MemStats) float64 { return float64(ms.StackSys) },
			metric: NewGauge(GaugeOpts{Namespace: ns, Name: "stack_sys_bytes", Help: "Number of bytes obtained from system for stack allocator. Equals to /memory/classes/heap/stacks:bytes + /memory/classes/os-stacks:bytes."}),
		}, {
			eval:   func(ms *runtime.MemStats) float64 { return float64(ms.MSpanInuse) },
			metric: NewGauge(GaugeOpts{Namespace: ns, Name: "mspan_inuse_bytes", Help: "Number of bytes in use by mspan structures. Equals to /memory/classes/metadata/mspan/inuse:bytes."}),
		}, {
			eval:   func(ms *runtime.MemStats) float64 { return float64(ms.MSpanSys) },
			metric: NewGauge(GaugeOpts{Namespace: ns, Name: "mspan_sys_bytes", Help: "Number of bytes used for mspan structures obtained from system. Equals to /memory/classes/metadata/mspan/inuse:bytes + /memory/classes/metadata/mspan/free:bytes."}),
		}, {
			eval:   func(ms *runtime.MemStats) float64 { return float64(ms.MCacheInuse) },
			metric: NewGauge(GaugeOpts{Namespace: ns, Name: "mcache_inuse_bytes", Help: "Number of bytes in use by mcache structures. Equals to /memory/classes/metadata/mcache/inuse:bytes."}),
		}, {
			eval:   func(ms *runtime.MemStats) float64 { return float64(ms.MCacheSys) },
			metric: NewGauge(GaugeOpts{Namespace: ns, Name: "mcache_sys_bytes", Help: "Number of bytes used for mcache structures obtained from system. Equals to /memory/classes/metadata/mcache/inuse:bytes + /memory/classes/metadata/mcache/free:bytes."}),
		}, {
			eval:   func(ms *runtime.MemStats) float64 { return float64(ms.BuckHashSys) },
			metric: NewGauge(GaugeOpts{Namespace: ns, Name: "buck_hash_sys_bytes", Help: "Number of bytes used by the profiling bucket hash table. Equals to /memory/classes/profiling/buckets:bytes."}),
		}, {
			eval:   func(ms *runtime.MemStats) float64 { return float64(ms.GCSys) },
			metric: NewGauge(GaugeOpts{Namespace: ns, Name: "gc_sys_bytes", Help: "Number of bytes used for garbage collection system metadata. Equals to /memory/classes/metadata/other:bytes."}),
		}, {
			eval:   func(ms *runtime.MemStats) float64 { return float64(ms.OtherSys) },
			metric: NewGauge(GaugeOpts{Namespace: ns, Name: "other_sys_bytes", Help: "Number of bytes used for other system allocations. Equals to /memory/classes/other:bytes."}),
		}, {
			eval:   func(ms *runtime.MemStats) float64 { return float64(ms.NextGC) },
			metric: NewGauge(GaugeOpts{Namespace: ns, Name: "next_gc_bytes", Help: "Number of heap bytes when next garbage collection will take place. Equals to /gc/heap/goal:bytes."}),
		},
	}
}

type baseGoCollector struct {
	goroutinesDesc *Desc
	threadsDesc    *Desc
	gcDesc         *Desc
	gcLastTimeDesc *Desc
	goInfoDesc     *Desc
}

func newBaseGoCollector() baseGoCollector {
	return baseGoCollector{
		goroutinesDesc: NewDesc(
			"go_goroutines",
			"Number of goroutines that currently exist.",
			nil, nil),
		threadsDesc: NewDesc(
			"go_threads",
			"Number of OS threads created.",
			nil, nil),
		gcDesc: NewDesc(
			"go_gc_duration_seconds",
			"A summary of the wall-time pause (stop-the-world) duration in garbage collection cycles.",
			nil, nil),
		gcLastTimeDesc: NewDesc(
			"go_memstats_last_gc_time_seconds",
			"Number of seconds since 1970 of last garbage collection.",
			nil, nil),
		goInfoDesc: NewDesc(
			"go_info",
			"Information about the Go environment.",
			nil, Labels{"version": runtime.Version()}),
	}
}

// Describe returns all descriptions of the collector.
func (c *baseGoCollector) Describe(ch chan<- *Desc) {
	ch <- c.goroutinesDesc
	ch <- c.threadsDesc
	ch <- c.gcDesc
	ch <- c.gcLastTimeDesc
	ch <- c.goInfoDesc
}

// Collect returns the current state of all metrics of the collector.
func (c *baseGoCollector) Collect(ch chan<- Metric) {
	ch <- MustNewConstMetric(c.goroutinesDesc, GaugeValue, float64(runtime.NumGoroutine()))

	n := getRuntimeNumThreads()
	ch <- MustNewConstMetric(c.threadsDesc, GaugeValue, n)

	var stats debug.GCStats
	stats.PauseQuantiles = make([]time.Duration, 5)
	debug.ReadGCStats(&stats)

	quantiles := make(map[float64]float64)
	for idx, pq := range stats.PauseQuantiles[1:] {
		quantiles[float64(idx+1)/float64(len(stats.PauseQuantiles)-1)] = pq.Seconds()
	}
	quantiles[0.0] = stats.PauseQuantiles[0].Seconds()
	ch <- MustNewConstSummary(c.gcDesc, uint64(stats.NumGC), stats.PauseTotal.Seconds(), quantiles)
	ch <- MustNewConstMetric(c.gcLastTimeDesc, GaugeValue, float64(stats.LastGC.UnixNano())/1e9)
	ch <- MustNewConstMetric(c.goInfoDesc, GaugeValue, 1)
}

func memstatNamespace(s string) string {
	return "go_memstats_" + s
}

// memStatsMetric provide description, evaluator, runtime/metrics name, and
// value type for memstat metric.
type memStatsMetric struct {
	last   float64
	eval   func(*runtime.MemStats) float64
	metric Metric
}

func (m *memStatsMetric) update(memStats *runtime.MemStats) Metric {
	last := m.eval(memStats)
	switch actual := m.metric.(type) {
	case Counter:
		actual.Add(last - m.last)
	case Gauge:
		actual.Set(last)
	default:
		panic("unexpected metric type")
	}
	m.last = last
	return m.metric
}

// memStatsMetrics provide description, evaluator, runtime/metrics name, and
// value type for memstat metrics.
type memStatsMetrics []memStatsMetric
