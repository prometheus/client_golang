// Copyright The Prometheus Authors
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

package prometheus_test

import (
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	dto "github.com/prometheus/client_model/go"
)

var ttlLookups = []struct {
	name string
	get  func(*prometheus.MetricVec) (prometheus.Metric, error)
}{
	{"label_values", func(v *prometheus.MetricVec) (prometheus.Metric, error) {
		return v.GetMetricWithLabelValues("200")
	}},
	{"labels", func(v *prometheus.MetricVec) (prometheus.Metric, error) {
		return v.GetMetricWith(prometheus.Labels{"code": "200"})
	}},
}

type customTTLCounter struct {
	prometheus.Counter
}

func (*customTTLCounter) CustomMethod() string { return "custom" }

func TestTTLCustomMetricRetainsInterface(t *testing.T) {
	for _, lookup := range ttlLookups {
		t.Run(lookup.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				const ttl = 100 * time.Millisecond

				var created *customTTLCounter
				vec := prometheus.V2.NewMetricVec(prometheus.MetricVecOpts{
					Desc: prometheus.NewDesc("custom_ttl_counter", "Custom TTL counter.", []string{"code"}, nil),
					NewMetric: func(lvs ...string) prometheus.Metric {
						created = &customTTLCounter{prometheus.NewCounter(prometheus.CounterOpts{
							Name:        "custom_ttl_counter",
							Help:        "Custom TTL counter.",
							ConstLabels: prometheus.Labels{"code": lvs[0]},
						})}
						return created
					},
					TTL: ttl,
				})

				metric, err := lookup.get(vec)
				if err != nil {
					t.Fatal(err)
				}

				child, ok := metric.(*customTTLCounter)
				if !ok || child != created {
					t.Fatalf("expected original custom counter, got %T", metric)
				}
				if child.CustomMethod() != "custom" {
					t.Fatal("custom method was not preserved")
				}

				child.Add(7)
				metrics := make(chan prometheus.Metric, 1)
				vec.Collect(metrics)
				if collected := <-metrics; collected != child {
					t.Fatalf("Collect returned %T instead of the original custom metric", collected)
				}

				if got := testutil.ToFloat64(vec); got != 7 {
					t.Fatalf("expected collected value 7, got %v", got)
				}

				time.Sleep(60 * time.Millisecond)
				again, err := lookup.get(vec)
				if err != nil || again != child {
					t.Fatalf("lookup did not preserve identity: %T, %v", again, err)
				}

				time.Sleep(60 * time.Millisecond)
				if n := vec.CleanupExpired(); n != 0 {
					t.Fatalf("lookup should refresh custom metric TTL, cleaned %d", n)
				}

				time.Sleep(ttl)
				child.Add(1)
				if n := vec.CleanupExpired(); n != 1 {
					t.Fatalf("cached custom mutations should not refresh TTL, cleaned %d", n)
				}

				fresh, err := lookup.get(vec)
				if err != nil || fresh == child || fresh != created {
					t.Fatalf("expected new original custom metric: %T, %v", fresh, err)
				}
				if got := testutil.ToFloat64(vec); got != 0 {
					t.Fatalf("expected fresh value 0, got %v", got)
				}
			})
		})
	}
}

func BenchmarkTTLLookup(b *testing.B) {
	for _, ttl := range []time.Duration{0, time.Hour} {
		for _, lookup := range ttlLookups {
			b.Run(ttl.String()+"/"+lookup.name, func(b *testing.B) {
				vec := prometheus.V2.NewCounterVec(prometheus.CounterVecOpts{
					CounterOpts:    prometheus.CounterOpts{Name: "ttl_lookup", Help: "TTL lookup benchmark."},
					VariableLabels: prometheus.UnconstrainedLabels([]string{"code"}),
					TTL:            ttl,
				})
				vec.WithLabelValues("200")

				b.ReportAllocs()
				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						if _, err := lookup.get(vec.MetricVec); err != nil {
							b.Error(err)
						}
					}
				})
			})
		}
	}
}

func BenchmarkRegistryCleanupExpired(b *testing.B) {
	for _, kind := range []string{"counter", "vector_without_ttl", "vector_with_ttl"} {
		b.Run(kind, func(b *testing.B) {
			inner := prometheus.NewRegistry()
			for i := range 100 {
				opts := prometheus.CounterOpts{Name: "cleanup_" + strconv.Itoa(i), Help: "Cleanup benchmark."}
				if kind == "counter" {
					inner.MustRegister(prometheus.NewCounter(opts))
					continue
				}

				var ttl time.Duration
				if kind == "vector_with_ttl" {
					ttl = time.Hour
				}
				vec := prometheus.V2.NewCounterVec(prometheus.CounterVecOpts{
					CounterOpts:    opts,
					VariableLabels: prometheus.UnconstrainedLabels([]string{"code"}),
					TTL:            ttl,
				})
				vec.WithLabelValues("200").Inc()
				inner.MustRegister(vec)
			}
			outer := prometheus.NewRegistry()
			outer.MustRegister(prometheus.WrapCollectorWithPrefix("nested_", inner))

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				outer.CleanupExpired()
			}
		})
	}
}

func TestTTLRegistryCleanupAllocations(t *testing.T) {
	for _, ttl := range []time.Duration{0, time.Hour} {
		t.Run(ttl.String(), func(t *testing.T) {
			inner := prometheus.NewRegistry()
			for i := range 100 {
				vec := prometheus.V2.NewCounterVec(prometheus.CounterVecOpts{
					CounterOpts:    prometheus.CounterOpts{Name: "cleanup_allocations_" + strconv.Itoa(i), Help: "Cleanup allocations."},
					VariableLabels: prometheus.UnconstrainedLabels([]string{"code"}),
					TTL:            ttl,
				})
				vec.WithLabelValues("200").Inc()
				inner.MustRegister(vec)
			}
			outer := prometheus.NewRegistry()
			outer.MustRegister(prometheus.WrapCollectorWithPrefix("nested_", inner))

			if allocs := testing.AllocsPerRun(100, func() { outer.CleanupExpired() }); allocs != 0 {
				t.Fatalf("expected allocation-free cleanup, got %v allocations", allocs)
			}
		})
	}
}

type trackingTTLCleaner struct {
	prometheus.Counter
	calls         atomic.Int64
	beforeCleanup func()
}

func (c *trackingTTLCleaner) CleanupExpired() int {
	if c.beforeCleanup != nil {
		c.beforeCleanup()
	}
	c.calls.Add(1)
	return 1
}

func TestTTLRegistryCleanupRegistration(t *testing.T) {
	for _, mode := range []string{"direct", "wrapped"} {
		t.Run(mode, func(t *testing.T) {
			reg := prometheus.NewRegistry()

			opts := prometheus.CounterOpts{Name: "cleanup_registration", Help: "Cleanup registration."}
			cleaner := &trackingTTLCleaner{Counter: prometheus.NewCounter(opts)}
			var (
				collector prometheus.Collector = cleaner
				alias     prometheus.Collector = prometheus.NewCounter(opts)
			)
			if mode == "wrapped" {
				collector = prometheus.WrapCollectorWithPrefix("wrapped_", collector)
				alias = prometheus.WrapCollectorWithPrefix("wrapped_", alias)
			}
			reg.MustRegister(collector)

			outer := prometheus.NewRegistry()
			outer.MustRegister(prometheus.WrapCollectorWithPrefix("nested_", reg))

			if err := reg.Register(collector); err == nil {
				t.Fatal("expected duplicate registration to fail")
			}
			if n := outer.CleanupExpired(); n != 1 {
				t.Fatalf("expected 1 cleanup call after registration, got %d", n)
			}

			if !reg.Unregister(alias) {
				t.Fatal("could not unregister using an equivalent collector")
			}
			if n := outer.CleanupExpired(); n != 0 || cleaner.calls.Load() != 1 {
				t.Fatal("cleanup called an unregistered collector")
			}

			reg.MustRegister(collector)
			if n := outer.CleanupExpired(); n != 1 || cleaner.calls.Load() != 2 {
				t.Fatal("cleanup did not track the re-registered collector")
			}
		})
	}
}

func TestTTLRegistryCleanupConcurrentRegistration(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var once sync.Once
		started := make(chan struct{})
		resume := make(chan struct{})
		first := &trackingTTLCleaner{
			Counter: prometheus.NewCounter(prometheus.CounterOpts{Name: "first_cleanup", Help: "First cleaner."}),
			beforeCleanup: func() {
				once.Do(func() {
					close(started)
					<-resume
				})
			},
		}
		second := &trackingTTLCleaner{
			Counter: prometheus.NewCounter(prometheus.CounterOpts{Name: "second_cleanup", Help: "Second cleaner."}),
		}
		replacement := &trackingTTLCleaner{
			Counter: prometheus.NewCounter(prometheus.CounterOpts{Name: "replacement_cleanup", Help: "Replacement cleaner."}),
		}
		reg := prometheus.NewRegistry()
		reg.MustRegister(first, second)

		done := make(chan int, 1)
		go func() { done <- reg.CleanupExpired() }()
		<-started

		if !reg.Unregister(second) {
			t.Fatal("could not unregister during cleanup")
		}
		reg.MustRegister(replacement)
		close(resume)

		if n := <-done; n != 2 || first.calls.Load() != 1 || second.calls.Load() != 1 || replacement.calls.Load() != 0 {
			t.Fatal("registration changed the in-progress cleanup snapshot")
		}
		if n := reg.CleanupExpired(); n != 2 || first.calls.Load() != 2 || second.calls.Load() != 1 || replacement.calls.Load() != 1 {
			t.Fatal("subsequent cleanup did not use the updated registrations")
		}
	})
}

func TestTTLExpiredLookupResetsState(t *testing.T) {
	for _, kind := range []string{"counter", "histogram"} {
		for _, lookup := range ttlLookups {
			t.Run(kind+"/"+lookup.name, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					const ttl = 100 * time.Millisecond

					var vec *prometheus.MetricVec
					if kind == "counter" {
						vec = prometheus.V2.NewCounterVec(prometheus.CounterVecOpts{
							CounterOpts:    prometheus.CounterOpts{Name: "ttl_reset_counter", Help: "TTL reset counter."},
							VariableLabels: prometheus.UnconstrainedLabels([]string{"code"}),
							TTL:            ttl,
						}).MetricVec
					} else {
						vec = prometheus.V2.NewHistogramVec(prometheus.HistogramVecOpts{
							HistogramOpts:  prometheus.HistogramOpts{Name: "ttl_reset_histogram", Help: "TTL reset histogram."},
							VariableLabels: prometheus.UnconstrainedLabels([]string{"code"}),
							TTL:            ttl,
						}).MetricVec
					}

					old, err := lookup.get(vec)
					if err != nil {
						t.Fatal(err)
					}
					if kind == "counter" {
						old.(prometheus.Counter).Add(7)
					} else {
						old.(prometheus.Histogram).Observe(7)
					}
					before := writeTTLMetric(t, old)

					time.Sleep(ttl + time.Millisecond)
					fresh, err := lookup.get(vec)
					if err != nil || fresh == old {
						t.Fatalf("expired lookup should replace child: %T, %v", fresh, err)
					}

					after := writeTTLMetric(t, fresh)
					if kind == "counter" {
						if after.GetCounter().GetValue() != 0 {
							t.Fatal("counter state survived expiration")
						}
						if !after.GetCounter().GetCreatedTimestamp().AsTime().After(before.GetCounter().GetCreatedTimestamp().AsTime()) {
							t.Fatal("counter creation timestamp did not reset")
						}
					} else {
						h := after.GetHistogram()
						if h.GetSampleCount() != 0 || h.GetSampleSum() != 0 {
							t.Fatal("histogram state survived expiration")
						}
						for _, bucket := range h.GetBucket() {
							if bucket.GetCumulativeCount() != 0 {
								t.Fatal("histogram bucket state survived expiration")
							}
						}
						if !h.GetCreatedTimestamp().AsTime().After(before.GetHistogram().GetCreatedTimestamp().AsTime()) {
							t.Fatal("histogram creation timestamp did not reset")
						}
					}

					if n := vec.CleanupExpired(); n != 0 {
						t.Fatalf("fresh replacement should remain registered, cleaned %d", n)
					}
				})
			})
		}
	}
}

func writeTTLMetric(t *testing.T, metric prometheus.Metric) *dto.Metric {
	t.Helper()

	out := &dto.Metric{}
	if err := metric.Write(out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestTTLConcurrentLookupAndCleanup(t *testing.T) {
	for _, lookup := range ttlLookups {
		t.Run(lookup.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				const ttl = 100 * time.Millisecond

				vec := prometheus.V2.NewCounterVec(prometheus.CounterVecOpts{
					CounterOpts:    prometheus.CounterOpts{Name: "ttl_concurrent", Help: "Concurrent TTL lookup."},
					VariableLabels: prometheus.UnconstrainedLabels([]string{"code"}),
					TTL:            ttl,
				})
				vec.WithLabelValues("200").Inc()

				for range 50 {
					time.Sleep(ttl + time.Millisecond)

					start := make(chan struct{})
					var (
						wg      sync.WaitGroup
						metrics [2]prometheus.Metric
						errs    [2]error
					)

					wg.Add(len(metrics) + 1)
					for i := range metrics {
						go func() {
							defer wg.Done()
							<-start
							metrics[i], errs[i] = lookup.get(vec.MetricVec)
						}()
					}

					go func() {
						defer wg.Done()
						<-start
						vec.CleanupExpired()
					}()

					close(start)
					wg.Wait()
					for i, err := range errs {
						if err != nil {
							t.Fatal(err)
						}
						if metrics[i] != metrics[0] {
							t.Fatal("concurrent lookups returned different metrics for the same labels")
						}
					}

					metrics[0].(prometheus.Counter).Add(1)
					if got := testutil.ToFloat64(vec); got != 1 {
						t.Fatalf("lookup returned a detached or stale child, exported value %v", got)
					}
				}
			})
		})
	}
}

func TestTTLWrappedRegistryCleanup(t *testing.T) {
	for _, cleanup := range []string{"gather", "manual"} {
		for _, mode := range []string{"direct", "registerer", "collector", "nested_registry"} {
			t.Run(cleanup+"/"+mode, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					const ttl = 100 * time.Millisecond

					vec := prometheus.V2.NewCounterVec(prometheus.CounterVecOpts{
						CounterOpts:    prometheus.CounterOpts{Name: "ttl_wrapped", Help: "Wrapped TTL counter."},
						VariableLabels: prometheus.UnconstrainedLabels([]string{"code"}),
						TTL:            ttl,
					})
					reg := prometheus.NewRegistry()

					var collector prometheus.Collector = vec
					switch mode {
					case "registerer":
						prometheus.WrapRegistererWithPrefix("wrapped_", prometheus.WrapRegistererWith(prometheus.Labels{"scope": "test"}, reg)).MustRegister(collector)
					case "collector":
						reg.MustRegister(prometheus.WrapCollectorWithPrefix("wrapped_", prometheus.WrapCollectorWith(prometheus.Labels{"scope": "test"}, collector)))
					case "nested_registry":
						inner := prometheus.NewRegistry()
						inner.MustRegister(collector)

						middle := prometheus.NewRegistry()
						middle.MustRegister(prometheus.WrapCollectorWithPrefix("inner_", inner))
						reg.MustRegister(prometheus.WrapCollectorWith(prometheus.Labels{"scope": "test"}, middle))
					default:
						reg.MustRegister(collector)
					}

					vec.WithLabelValues("200").Add(7)
					if cleanup == "manual" {
						if n := reg.CleanupExpired(); n != 0 {
							t.Fatalf("manual cleanup removed %d live metrics", n)
						}
					}

					mfs, err := reg.Gather()
					if err != nil || len(mfs) != 1 || mfs[0].GetMetric()[0].GetCounter().GetValue() != 7 {
						t.Fatalf("live wrapped metric was not collected: %v, %v", mfs, err)
					}

					time.Sleep(ttl + time.Millisecond)
					if cleanup == "manual" {
						if n := reg.CleanupExpired(); n != 1 {
							t.Fatalf("expected manual cleanup to remove one metric, got %d", n)
						}
					}

					mfs, err = reg.Gather()
					if err != nil || len(mfs) != 0 {
						t.Fatalf("expired wrapped metric was still collected: %v, %v", mfs, err)
					}

					if n := vec.CleanupExpired(); n != 0 {
						t.Fatalf("registry cleanup did not reclaim expired metrics, %d remained", n)
					}
				})
			})
		}
	}
}

type panicTTLCleaner struct {
	prometheus.Counter
	unchecked bool
}

func TestTTLRegistryCleanupCount(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const ttl = 100 * time.Millisecond

		outer := prometheus.NewRegistry()
		inner := prometheus.NewRegistry()
		for _, name := range []string{"first_ttl_counter", "second_ttl_counter"} {
			vec := prometheus.V2.NewCounterVec(prometheus.CounterVecOpts{
				CounterOpts:    prometheus.CounterOpts{Name: name, Help: "TTL cleanup count."},
				VariableLabels: prometheus.UnconstrainedLabels([]string{"code"}),
				TTL:            ttl,
			})
			vec.WithLabelValues("200").Inc()
			inner.MustRegister(vec)
		}
		outer.MustRegister(prometheus.WrapCollectorWithPrefix("nested_", inner))

		permanent := prometheus.NewCounter(prometheus.CounterOpts{Name: "permanent_counter", Help: "No TTL."})
		permanent.Add(3)
		outer.MustRegister(permanent)

		time.Sleep(ttl + time.Millisecond)

		var cleaner prometheus.ExpiredCleaner = outer
		if n := cleaner.CleanupExpired(); n != 2 {
			t.Fatalf("expected two metrics removed from the nested registry, got %d", n)
		}
		if n := cleaner.CleanupExpired(); n != 0 {
			t.Fatalf("expected nothing left to remove, got %d", n)
		}
		if got := testutil.ToFloat64(outer); got != 3 {
			t.Fatalf("cleanup changed the non-TTL metric, got %v", got)
		}
	})
}

type unregisteringTTLCleaner struct {
	prometheus.Counter
	registry *prometheus.Registry
}

func (c *unregisteringTTLCleaner) CleanupExpired() int {
	c.registry.Unregister(c)

	return 0
}

func TestTTLCleanupReleasesNestedRegistryLock(t *testing.T) {
	for _, cleanup := range []string{"gather", "manual"} {
		t.Run(cleanup, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				inner := prometheus.NewRegistry()
				cleaner := &unregisteringTTLCleaner{
					Counter:  prometheus.NewCounter(prometheus.CounterOpts{Name: "ttl_unregister", Help: "Self-unregistering cleaner."}),
					registry: inner,
				}
				inner.MustRegister(cleaner)

				outer := prometheus.NewRegistry()
				outer.MustRegister(prometheus.WrapCollectorWith(prometheus.Labels{"scope": "test"}, inner))

				done := make(chan error, 1)
				go func() {
					if cleanup == "manual" {
						outer.CleanupExpired()
						done <- nil
						return
					}
					_, err := outer.Gather()
					done <- err
				}()

				select {
				case err := <-done:
					if err != nil {
						t.Fatal(err)
					}
				case <-time.After(time.Second):
					t.Fatal("cleanup could not unregister from the nested registry")
				}

				if inner.Unregister(cleaner) {
					t.Fatal("cleanup did not unregister its collector")
				}
			})
		})
	}
}

func (c *panicTTLCleaner) Describe(ch chan<- *prometheus.Desc) {
	if !c.unchecked {
		c.Counter.Describe(ch)
	}
}

func (*panicTTLCleaner) CleanupExpired() int { panic("TTL cleanup panic") }

func TestTTLCleanupPanicDoesNotHangGather(t *testing.T) {
	for _, mode := range []string{"checked", "unchecked", "wrapped", "nested_registry"} {
		t.Run(mode, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				cleaner := &panicTTLCleaner{
					Counter:   prometheus.NewCounter(prometheus.CounterOpts{Name: "ttl_panic", Help: "Panicking TTL cleaner."}),
					unchecked: mode == "unchecked",
				}
				reg := prometheus.NewRegistry()

				var collector prometheus.Collector = cleaner
				if mode == "nested_registry" {
					inner := prometheus.NewRegistry()
					inner.MustRegister(cleaner)
					collector = inner
				}

				if mode == "wrapped" || mode == "nested_registry" {
					collector = prometheus.WrapCollectorWith(prometheus.Labels{"scope": "test"}, collector)
				}
				reg.MustRegister(collector)

				done := make(chan error, 1)
				go func() {
					_, err := reg.Gather()
					done <- err
				}()

				select {
				case err := <-done:
					if err == nil || !strings.Contains(err.Error(), "TTL cleanup panic") {
						t.Fatalf("expected cleanup panic error, got %v", err)
					}
				case <-time.After(time.Second):
					t.Fatal("Gather did not finish after a cleanup panic")
				}
			})
		})
	}
}

func TestTTLCachedHandleLifetime(t *testing.T) {
	for _, mode := range []string{"refresh_before_cleanup", "cleanup", "replacement"} {
		t.Run(mode, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				const ttl = 100 * time.Millisecond

				vec := prometheus.V2.NewCounterVec(prometheus.CounterVecOpts{
					CounterOpts:    prometheus.CounterOpts{Name: "ttl_cached_lifetime", Help: "Cached TTL handle lifetime."},
					VariableLabels: prometheus.UnconstrainedLabels([]string{"code"}),
					TTL:            ttl,
				})
				cached := vec.WithLabelValues("200")
				cached.Add(1)

				time.Sleep(ttl + time.Millisecond)

				// Collect directly: testutil helpers use Gather and would remove the child.
				metrics := make(chan prometheus.Metric, 1)
				vec.Collect(metrics)
				if n := len(metrics); n != 0 {
					t.Fatalf("expected expired child to be omitted, got %d", n)
				}

				if mode == "refresh_before_cleanup" {
					cached.Add(5)
					if n := vec.CleanupExpired(); n != 0 {
						t.Fatalf("cached mutation should refresh retained child, cleaned %d", n)
					}
					if vec.WithLabelValues("200") != cached || testutil.ToFloat64(vec) != 6 {
						t.Fatal("cached mutation did not retain original state")
					}
					return
				}

				if mode == "cleanup" {
					if n := vec.CleanupExpired(); n != 1 {
						t.Fatalf("expected one child removed, got %d", n)
					}
				}

				fresh := vec.WithLabelValues("200")
				if fresh == cached {
					t.Fatal("expected new child after removal or replacement")
				}
				fresh.Add(2)
				cached.Add(5)
				if got := testutil.ToFloat64(vec); got != 2 {
					t.Fatalf("old handle updates were exported after a fresh lookup, got %v", got)
				}
			})
		})
	}
}

func TestTTLWrapperMutatorsAndCurriedVec(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const ttl = 100 * time.Millisecond

		cvec := prometheus.V2.NewCounterVec(prometheus.CounterVecOpts{
			CounterOpts:    prometheus.CounterOpts{Name: "ttl_mutators_counter", Help: "Counter mutators."},
			VariableLabels: prometheus.UnconstrainedLabels([]string{"role", "code"}),
			TTL:            ttl,
		})
		curried := cvec.MustCurryWith(prometheus.Labels{"role": "api"})
		c := curried.WithLabelValues("200")
		time.Sleep(60 * time.Millisecond)
		c.Inc()
		time.Sleep(60 * time.Millisecond)
		c.(prometheus.ExemplarAdder).AddWithExemplar(2, prometheus.Labels{"trace": "1"})
		if n := cvec.CleanupExpired(); n != 0 {
			t.Fatalf("expected counter mutators to refresh TTL, cleaned %d", n)
		}
		if got := writeTTLMetric(t, c).GetCounter(); got.GetValue() != 3 || got.GetExemplar().GetValue() != 2 {
			t.Fatalf("unexpected counter state: %v", got)
		}

		gvec := prometheus.V2.NewGaugeVec(prometheus.GaugeVecOpts{
			GaugeOpts:      prometheus.GaugeOpts{Name: "ttl_mutators_gauge", Help: "Gauge mutators."},
			VariableLabels: prometheus.UnconstrainedLabels([]string{"code"}),
			TTL:            ttl,
		})
		g := gvec.WithLabelValues("200")
		for _, op := range []func(){
			g.Inc,
			g.Dec,
			func() { g.Add(5) },
			func() { g.Sub(2) },
			g.SetToCurrentTime,
		} {
			time.Sleep(60 * time.Millisecond)
			op()
			if n := gvec.CleanupExpired(); n != 0 {
				t.Fatalf("expected gauge mutator to refresh TTL, cleaned %d", n)
			}
		}

		hvec := prometheus.V2.NewHistogramVec(prometheus.HistogramVecOpts{
			HistogramOpts:  prometheus.HistogramOpts{Name: "ttl_mutators_histogram", Help: "Histogram mutators."},
			VariableLabels: prometheus.UnconstrainedLabels([]string{"code"}),
			TTL:            ttl,
		})
		h := hvec.WithLabelValues("200")
		time.Sleep(60 * time.Millisecond)
		h.(prometheus.ExemplarObserver).ObserveWithExemplar(0.5, prometheus.Labels{"trace": "2"})
		if n := hvec.CleanupExpired(); n != 0 {
			t.Fatalf("expected ObserveWithExemplar to refresh TTL, cleaned %d", n)
		}
		if got := writeTTLMetric(t, h.(prometheus.Metric)).GetHistogram(); got.GetSampleCount() != 1 {
			t.Fatalf("unexpected histogram count: %v", got.GetSampleCount())
		}
	})
}
