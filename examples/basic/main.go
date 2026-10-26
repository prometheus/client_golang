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

// An example demonstrating idiomatic definition and usage of basic Prometheus metric
// types (Counter, CounterVec, Gauge, GaugeVec, HistogramVec) in a Go service.
package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var addr = flag.String("listen-address", ":8080", "The address to listen on for HTTP requests.")

// ServiceMetrics encapsulates all metric collectors for the service.
// Encapsulating metrics within a struct and accepting a prometheus.Registerer
// promotes testability and avoids relying on the global default registry.
type ServiceMetrics struct {
	// TaskCounter tracks total completed tasks, partitioned by task type and execution status.
	TaskCounter *prometheus.CounterVec

	// ActiveWorkers tracks the current number of workers actively running tasks.
	ActiveWorkers prometheus.Gauge

	// QueueDepth tracks current items pending in the worker queue per partition.
	QueueDepth *prometheus.GaugeVec

	// TaskDuration measures latency distribution for task executions.
	TaskDuration *prometheus.HistogramVec
}

// NewServiceMetrics initializes and registers all service metrics with the provided registerer.
func NewServiceMetrics(reg prometheus.Registerer) *ServiceMetrics {
	m := &ServiceMetrics{
		TaskCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "app_tasks_processed_total",
				Help: "Total number of tasks processed, partitioned by type and status.",
			},
			[]string{"type", "status"},
		),
		ActiveWorkers: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "app_active_workers",
				Help: "Current number of workers actively executing tasks.",
			},
		),
		QueueDepth: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "app_queue_depth",
				Help: "Current number of queued items per partition.",
			},
			[]string{"partition"},
		),
		TaskDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "app_task_duration_seconds",
				Help:    "Duration of task execution in seconds.",
				Buckets: prometheus.DefBuckets, // Standard default buckets: .005, .01, .025, ..., 10
			},
			[]string{"type"},
		),
	}

	// Register all collectors with the provided registry.
	reg.MustRegister(
		m.TaskCounter,
		m.ActiveWorkers,
		m.QueueDepth,
		m.TaskDuration,
	)

	return m
}

// WorkerPool simulates work execution and updates metrics accordingly.
type WorkerPool struct {
	metrics *ServiceMetrics
}

func NewWorkerPool(metrics *ServiceMetrics) *WorkerPool {
	return &WorkerPool{metrics: metrics}
}

// ProcessTask simulates running a single unit of work and recording metrics.
func (p *WorkerPool) ProcessTask(taskType string, duration time.Duration, succeed bool) {
	p.metrics.ActiveWorkers.Inc()
	defer p.metrics.ActiveWorkers.Dec()

	// Observe task duration
	p.metrics.TaskDuration.WithLabelValues(taskType).Observe(duration.Seconds())

	// Record task status counter
	status := "success"
	if !succeed {
		status = "failure"
	}
	p.metrics.TaskCounter.WithLabelValues(taskType, status).Inc()
}

func main() {
	flag.Parse()

	// Use an isolated registry rather than the global DefaultRegisterer.
	reg := prometheus.NewRegistry()

	// Register standard Go runtime and process collectors.
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	// Create and register application metrics.
	metrics := NewServiceMetrics(reg)
	pool := NewWorkerPool(metrics)

	// Simulate background work generating metric data.
	go func() {
		taskTypes := []string{"email", "image_processing", "database_backup"}
		r := rand.New(rand.NewSource(time.Now().UnixNano()))

		for {
			taskType := taskTypes[r.Intn(len(taskTypes))]
			simulatedDuration := time.Duration(10+r.Intn(100)) * time.Millisecond
			succeed := r.Float64() > 0.1 // 90% success rate

			// Update queue depth
			metrics.QueueDepth.WithLabelValues("default").Set(float64(r.Intn(25)))

			pool.ProcessTask(taskType, simulatedDuration, succeed)
			time.Sleep(500 * time.Millisecond)
		}
	}()

	// Expose the registered metrics at /metrics.
	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))

	fmt.Printf("Serving metrics on http://localhost%s/metrics\n", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
