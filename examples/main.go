package main

import (
	"expvar"
	"log"
	"net/http"
	"runtime"
	"time"

	dto "github.com/prometheus/client_model/go"

	"github.com/prometheus/client_golang/prometheus"
)

func main() {
	///////////////////////
	// A simple counter. //
	///////////////////////
	indexed := prometheus.MustNewCounter(&prometheus.Desc{
		Name: "documents_indexed",
		Help: "The number of documents indexed.",
	})
	prometheus.MustRegister(indexed)

	indexed.Set(42)
	indexed.Inc()
	indexed.Add(100)

	// For reference, this is how it looks like with the original proposal
	// (OP):
	// indexed := prometheus.NewCounter(prometheus.CounterDesc{
	//         Desc: prometheus.Desc{
	//                 Name: "documents_indexed",
	//                 Help: "The number of documents indexed.",
	//         },
	// })
	// prometheus.MustRegister(indexed)
	//
	// indexed.Set(42)
	// indexed.Dec()
	// indexed.Add(100)

	////////////////////////////////
	// A counter with dimensions. //
	////////////////////////////////
	searched := prometheus.MustNewCounterVec(&prometheus.Desc{
		Name:           "documents_searched",
		Help:           "The number of documents indexed.",
		VariableLabels: []string{"status_code", "version"},
	})
	prometheus.MustRegister(searched)

	// "Quick&easy" way: Pass in label values in order:
	searched.WithLabelValues("200", "prod").Set(2001)
	// "Safer" way (but more effort): Pass in labels as a map.
	searched.WithLabels(map[string]string{"status_code": "404", "version": "test"}).Set(4)
	// Alternative "safe" proposal: name->value pairs (less safe than the map above):
	// searched.WithLabels("status_code", "404", "version", "test").Set(4)

	// For even faster repeated access, a reference to the metric object can be kept.
	m := searched.WithLabelValues("200", "prod")
	m.Inc()

	// Same with the OP:
	// searched := prometheus.NewCounterVec(prometheus.CounterVecDesc{
	//         Desc: prometheus.Desc{
	//                 Name:           "documents_searched",
	//                 Help:           "The number of documents indexed.",
	//         },
	//         Labels: []string{"status_code", "version"},
	// })
	// prometheus.MustRegister(searched)
	//
	// searched.Set(2001, "200", "prod")
	// searched.Set(4, "404", "test")
	// searched.Inc("200", "prod")

	///////////////////////////////////
	// A summary with fancy options. //
	///////////////////////////////////
	summary := prometheus.MustNewSummary(
		&prometheus.Desc{
			Name: "fancy_summary",
			Help: "A summary to demonstrate the options.",
		},
		&prometheus.SummaryOptions{
			Objectives: map[float64]float64{
				0.3:   0.02,
				0.8:   0.01,
				0.999: 0.0001,
			},
			FlushInter: 5 * time.Minute,
		},
	)
	prometheus.MustRegister(summary)
	summary.Observe(123.345)

	// Same with the OP (a summary with labels would be again different):
	// summary := prometheus.NewSummary(prometheus.SummaryDesc{
	//         prometheus.Desc{
	//                 Name: "fancy_summary",
	//                 Help: "A summary to demonstrate the options.",
	//         },
	//         Objectives: map[float64]float64{
	//                 0.3:   0.02,
	//                 0.8:   0.01,
	//                 0.999: 0.0001,
	//         },
	//         FlushInter: 5 * time.Minute,
	// })
	// prometheus.MustRegister(summary)

	////////////////////
	// Expose expvar. //
	////////////////////
	expvarCollector := prometheus.MustNewExpvarCollector(map[string]*prometheus.Desc{
		"memstats": &prometheus.Desc{
			Name:           "expvar_memstats",
			Help:           "All numeric memstats as one metric family. Not a good role-model, actually... ;-)",
			Type:           dto.MetricType_UNTYPED, // To not imply this mash-up has any meaning...
			VariableLabels: []string{"type"},
		},
		"lone_int": &prometheus.Desc{
			Name: "expvar_lone_int",
			Help: "Just an expvar int as an example.",
			Type: dto.MetricType_GAUGE,
		},
		"complex_map": &prometheus.Desc{
			Name:           "expvar_complex_map",
			Help:           "A silly map to demonstrate depth two...",
			Type:           dto.MetricType_COUNTER,
			VariableLabels: []string{"en", "de"},
		},
	})
	prometheus.MustRegister(expvarCollector)

	// The Prometheus part is done here. But to show that this example is
	// doing anything, we have to manually export something via expvar.
	expvar.NewInt("lone_int").Set(42)
	expvarMap := expvar.NewMap("complex_map")
	var (
		expvarMap1, expvarMap2                             expvar.Map
		expvarInt11, expvarInt12, expvarInt21, expvarInt22 expvar.Int
	)
	expvarMap1.Init()
	expvarMap2.Init()
	expvarInt11.Set(11)
	expvarInt12.Set(12)
	expvarInt21.Set(21)
	expvarInt22.Set(22)
	expvarMap1.Set("dings", &expvarInt11)
	expvarMap1.Set("bums", &expvarInt12)
	expvarMap2.Set("dings", &expvarInt21)
	expvarMap2.Set("bums", &expvarInt22)
	expvarMap.Set("foo", &expvarMap1)
	expvarMap.Set("bar", &expvarMap2)

	//////////////////////
	// Expose memstats. //
	//////////////////////
	MemStatsDescriptors := []*prometheus.Desc{
		&prometheus.Desc{
			Subsystem: "memstats",
			Name:      "alloc",
			Help:      "bytes allocated and still in use",
			Type:      dto.MetricType_GAUGE,
		},
		&prometheus.Desc{
			Subsystem: "memstats",
			Name:      "total_alloc",
			Help:      "bytes allocated (even if freed)",
			Type:      dto.MetricType_GAUGE,
		},
		&prometheus.Desc{
			Subsystem: "memstats",
			Name:      "num_gc",
			Help:      "number of GCs run",
			Type:      dto.MetricType_COUNTER,
		},
	}
	prometheus.MustRegister(&MemStatsCollector{Descs: MemStatsDescriptors})

	//////////////////////////////////////
	// Multi-worker cluster management. //
	//////////////////////////////////////
	// Here each worker funnels into the
	// same set of multiple metrics.
	workerDB := NewClusterManager("db")
	workerCA := NewClusterManager("ca")
	prometheus.MustRegister(workerDB)
	prometheus.MustRegister(workerCA)

	http.Handle("/metrics", prometheus.Handler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

type MemStatsCollector struct {
	Descs []*prometheus.Desc
}

func (m *MemStatsCollector) DescribeMetrics() []*prometheus.Desc {
	return m.Descs
}

func (m *MemStatsCollector) CollectMetrics() []prometheus.Metric {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	metrics := prometheus.MustNewConstMetrics(
		m.Descs,
		[]float64{float64(ms.Alloc), float64(ms.TotalAlloc), float64(ms.NumGC)},
	)
	return metrics
	// If you don't like the ordering aspect of the above, you could do the
	// following, where order doesn't matter:
	// return []Metric{
	//         NewConstMetric(desc[0],float64(ms.Alloc)),
	//         NewConstMetric(desc[1],float64(ms.TotalAlloc)),
	//         NewConstMetric(desc[2],float64(ms.NumGC)),
	// }

	// To avoid new allocations each scrape, you could also keep metric
	// objects around and return the same objects each time, just with new
	// values set.
}

type ClusterManager struct {
	Zone         string
	OOMCountDesc *prometheus.Desc
	RAMUsageDesc *prometheus.Desc
	// ... many more fields
}

func (c *ClusterManager) ReallyExpensiveAssessmentOfTheSystemState() (
	oomCountByHost map[string]int, ramUsageByHost map[string]float64,
) {
	// Just example fake data.
	oomCountByHost = map[string]int{
		"foo.example.org": 42,
		"bar.example.org": 2001,
	}
	ramUsageByHost = map[string]float64{
		"foo.example.org": 6.023e23,
		"bar.example.org": 3.14,
	}
	return
}

func (c *ClusterManager) DescribeMetrics() []*prometheus.Desc {
	return []*prometheus.Desc{c.OOMCountDesc, c.RAMUsageDesc}
}

func (c *ClusterManager) CollectMetrics() []prometheus.Metric {
	// Create metrics from scratch each time because hosts that have gone
	// away since the last scrape must not stay around.  If that's too much
	// of a resource drain, keep the metrics around and reset them
	// properly.
	oomCountCounter := prometheus.MustNewCounterVec(c.OOMCountDesc)
	ramUsageGauge := prometheus.MustNewGaugeVec(c.RAMUsageDesc)
	oomCountByHost, ramUsageByHost := c.ReallyExpensiveAssessmentOfTheSystemState()
	for host, oomCount := range oomCountByHost {
		oomCountCounter.WithLabelValues(host).Set(float64(oomCount))
	}
	for host, ramUsage := range ramUsageByHost {
		ramUsageGauge.WithLabelValues(host).Set(ramUsage)
	}
	return append(oomCountCounter.CollectMetrics(), ramUsageGauge.CollectMetrics()...)
}

func NewClusterManager(zone string) *ClusterManager {
	return &ClusterManager{
		Zone: zone,
		OOMCountDesc: &prometheus.Desc{
			Subsystem:      "clustermanager",
			Name:           "oom_count",
			Help:           "number of OOM crashes",
			PresetLabels:   map[string]string{"zone": zone},
			VariableLabels: []string{"host"},
		},
		RAMUsageDesc: &prometheus.Desc{
			Subsystem:      "clustermanager",
			Name:           "ram_usage",
			Help:           "RAM usage in MiB as reported to the cluster manager",
			PresetLabels:   map[string]string{"zone": zone},
			VariableLabels: []string{"host"},
		},
	}
}
