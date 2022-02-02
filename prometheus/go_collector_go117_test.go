// Copyright 2021 The Prometheus Authors
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

//go:build go1.17
// +build go1.17

package prometheus

import (
	"math"
	"reflect"
	"runtime"
	"runtime/metrics"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus/internal"
	dto "github.com/prometheus/client_model/go"
)

func TestGoCollectorRuntimeMetrics(t *testing.T) {
	metrics := collectGoMetrics(t)

	msChecklist := make(map[string]bool)
	for _, m := range goRuntimeMemStats() {
		msChecklist[m.desc.fqName] = false
	}

	if len(metrics) == 0 {
		t.Fatal("no metrics created by Collect")
	}

	// Check a few specific metrics.
	//
	// Checking them all is somewhat pointless because the runtime/metrics
	// metrics are going to shift underneath us. Also if we try to check
	// against the runtime/metrics package in an automated fashion we're kind
	// of missing the point, because we have to do all the same work the code
	// has to do to perform the translation. Same for supporting old metric
	// names (the best we can do here is make sure they're all accounted for).
	var sysBytes, allocs float64
	for _, m := range metrics {
		name := m.Desc().fqName
		switch name {
		case "go_memory_classes_total_bytes":
			checkMemoryMetric(t, m, &sysBytes)
		case "go_sys_bytes":
			checkMemoryMetric(t, m, &sysBytes)
		case "go_gc_heap_allocs_bytes_total":
			checkMemoryMetric(t, m, &allocs)
		case "go_alloc_bytes_total":
			checkMemoryMetric(t, m, &allocs)
		}
		if present, ok := msChecklist[name]; ok {
			if present {
				t.Errorf("memstats metric %s found more than once", name)
			}
			msChecklist[name] = true
		}
	}
	for name := range msChecklist {
		if present := msChecklist[name]; !present {
			t.Errorf("memstats metric %s not collected", name)
		}
	}
}

func checkMemoryMetric(t *testing.T, m Metric, expValue *float64) {
	t.Helper()

	pb := &dto.Metric{}
	m.Write(pb)
	var value float64
	if g := pb.GetGauge(); g != nil {
		value = g.GetValue()
	} else {
		value = pb.GetCounter().GetValue()
	}
	if value <= 0 {
		t.Error("bad value for total memory")
	}
	if *expValue == 0 {
		*expValue = value
	} else if value != *expValue {
		t.Errorf("legacy metric and runtime/metrics metric do not match: want %d, got %d", int64(*expValue), int64(value))
	}
}

var sink interface{}

func TestBatchHistogram(t *testing.T) {
	goMetrics := collectGoMetrics(t)

	var mhist Metric
	for _, m := range goMetrics {
		if m.Desc().fqName == "go_gc_heap_allocs_by_size_bytes_total" {
			mhist = m
			break
		}
	}
	if mhist == nil {
		t.Fatal("failed to find metric to test")
	}
	hist, ok := mhist.(*batchHistogram)
	if !ok {
		t.Fatal("found metric is not a runtime/metrics histogram")
	}

	// Make a bunch of allocations then do another collection.
	//
	// The runtime/metrics API tries to reuse memory where possible,
	// so make sure that we didn't hang on to any of that memory in
	// hist.
	countsCopy := make([]uint64, len(hist.counts))
	copy(countsCopy, hist.counts)
	for i := 0; i < 100; i++ {
		sink = make([]byte, 128)
	}
	collectGoMetrics(t)
	for i, v := range hist.counts {
		if v != countsCopy[i] {
			t.Error("counts changed during new collection")
			break
		}
	}

	// Get the runtime/metrics copy.
	s := []metrics.Sample{
		{Name: "/gc/heap/allocs-by-size:bytes"},
	}
	metrics.Read(s)
	rmHist := s[0].Value.Float64Histogram()
	wantBuckets := internal.RuntimeMetricsBucketsForUnit(rmHist.Buckets, "bytes")
	// runtime/metrics histograms always have a +Inf bucket and are lower
	// bound inclusive. In contrast, we have an implicit +Inf bucket and
	// are upper bound inclusive, so we can chop off the first bucket
	// (since the conversion to upper bound inclusive will shift all buckets
	// down one index) and the +Inf for the last bucket.
	wantBuckets = wantBuckets[1 : len(wantBuckets)-1]

	// Check to make sure the output proto makes sense.
	pb := &dto.Metric{}
	hist.Write(pb)

	if math.IsInf(pb.Histogram.Bucket[len(pb.Histogram.Bucket)-1].GetUpperBound(), +1) {
		t.Errorf("found +Inf bucket")
	}
	if got := len(pb.Histogram.Bucket); got != len(wantBuckets) {
		t.Errorf("got %d buckets in protobuf, want %d", got, len(wantBuckets))
	}
	for i, bucket := range pb.Histogram.Bucket {
		// runtime/metrics histograms are lower-bound inclusive, but we're
		// upper-bound inclusive. So just make sure the new inclusive upper
		// bound is somewhere close by (in some cases it's equal).
		wantBound := wantBuckets[i]
		if gotBound := *bucket.UpperBound; (wantBound-gotBound)/wantBound > 0.001 {
			t.Errorf("got bound %f, want within 0.1%% of %f", gotBound, wantBound)
		}
		// Make sure counts are cumulative. Because of the consistency guarantees
		// made by the runtime/metrics package, we're really not guaranteed to get
		// anything even remotely the same here.
		if i > 0 && *bucket.CumulativeCount < *pb.Histogram.Bucket[i-1].CumulativeCount {
			t.Error("cumulative counts are non-monotonic")
		}
	}
}

func collectGoMetrics(t *testing.T) []Metric {
	t.Helper()

	c := NewGoCollector().(*goCollector)

	// Collect all metrics.
	ch := make(chan Metric)
	var wg sync.WaitGroup
	var metrics []Metric
	wg.Add(1)
	go func() {
		defer wg.Done()
		for metric := range ch {
			metrics = append(metrics, metric)
		}
	}()
	c.Collect(ch)
	close(ch)

	wg.Wait()

	return metrics
}

func TestMemStatsEquivalence(t *testing.T) {
	var msReal, msFake runtime.MemStats
	descs := metrics.All()
	samples := make([]metrics.Sample, len(descs))
	samplesMap := make(map[string]*metrics.Sample)
	for i := range descs {
		samples[i].Name = descs[i].Name
		samplesMap[descs[i].Name] = &samples[i]
	}

	// Force a GC cycle to try to reach a clean slate.
	runtime.GC()

	// Populate msReal.
	runtime.ReadMemStats(&msReal)

	// Populate msFake.
	metrics.Read(samples)
	memStatsFromRM(&msFake, samplesMap)

	// Iterate over them and make sure they're somewhat close.
	msRealValue := reflect.ValueOf(msReal)
	msFakeValue := reflect.ValueOf(msFake)

	typ := msRealValue.Type()
	for i := 0; i < msRealValue.NumField(); i++ {
		fr := msRealValue.Field(i)
		ff := msFakeValue.Field(i)
		switch typ.Kind() {
		case reflect.Uint64:
			// N.B. Almost all fields of MemStats are uint64s.
			vr := fr.Interface().(uint64)
			vf := ff.Interface().(uint64)
			if float64(vr-vf)/float64(vf) > 0.05 {
				t.Errorf("wrong value for %s: got %d, want %d", typ.Field(i).Name, vf, vr)
			}
		}
	}
}

func TestExpectedRuntimeMetrics(t *testing.T) {
	goMetrics := collectGoMetrics(t)
	goMetricSet := make(map[string]Metric)
	for _, m := range goMetrics {
		goMetricSet[m.Desc().fqName] = m
	}

	descs := metrics.All()
	rmSet := make(map[string]struct{})
	// Iterate over runtime-reported descriptions to find new metrics.
	for i := range descs {
		rmName := descs[i].Name
		rmSet[rmName] = struct{}{}

		expFQName, ok := expectedRuntimeMetrics[rmName]
		if !ok {
			t.Errorf("found new runtime/metrics metric %s", rmName)
			_, _, _, ok := internal.RuntimeMetricsToProm(&descs[i])
			if !ok {
				t.Errorf("new metric has name that can't be converted, or has an unsupported Kind")
			}
			continue
		}
		_, ok = goMetricSet[expFQName]
		if !ok {
			t.Errorf("existing runtime/metrics metric %s (expected fq name %s) not collected", rmName, expFQName)
			continue
		}
	}
	// Now iterate over the expected metrics and look for removals.
	cardinality := 0
	for rmName, fqName := range expectedRuntimeMetrics {
		if _, ok := rmSet[rmName]; !ok {
			t.Errorf("runtime/metrics metric %s removed", rmName)
			continue
		}
		if _, ok := goMetricSet[fqName]; !ok {
			t.Errorf("runtime/metrics metric %s not appearing under expected name %s", rmName, fqName)
			continue
		}

		// While we're at it, check to make sure expected cardinality lines
		// up, but at the point of the protobuf write to get as close to the
		// real deal as possible.
		//
		// Note that we filter out non-runtime/metrics metrics here, because
		// those are manually managed.
		var m dto.Metric
		if err := goMetricSet[fqName].Write(&m); err != nil {
			t.Errorf("writing metric %s: %v", fqName, err)
			continue
		}
		// N.B. These are the only fields populated by runtime/metrics metrics specifically.
		// Other fields are populated by e.g. GCStats metrics.
		switch {
		case m.Counter != nil:
			fallthrough
		case m.Gauge != nil:
			cardinality++
		case m.Histogram != nil:
			cardinality += len(m.Histogram.Bucket) + 3 // + sum, count, and +inf
		default:
			t.Errorf("unexpected protobuf structure for metric %s", fqName)
		}
	}

	if t.Failed() {
		t.Log("a new Go version may have been detected, please run")
		t.Log("\tgo run gen_go_collector_metrics_set.go go1.X")
		t.Log("where X is the Go version you are currently using")
	}

	expectCardinality := expectedRuntimeMetricsCardinality
	if cardinality != expectCardinality {
		t.Errorf("unexpected cardinality for runtime/metrics metrics: got %d, want %d", cardinality, expectCardinality)
	}
}

func TestGoCollectorConcurrency(t *testing.T) {
	c := NewGoCollector().(*goCollector)

	// Set up multiple goroutines to Collect from the
	// same GoCollector. In race mode with GOMAXPROCS > 1,
	// this test should fail often if Collect is not
	// concurrent-safe.
	for i := 0; i < 4; i++ {
		go func() {
			ch := make(chan Metric)
			go func() {
				// Drain all metrics received until the
				// channel is closed.
				for range ch {
				}
			}()
			c.Collect(ch)
			close(ch)
		}()
	}
}
