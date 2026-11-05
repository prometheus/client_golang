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

package main

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// TestWorkerPool_Metrics demonstrates verifying metric values using testutil.ToFloat64.
func TestWorkerPool_Metrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewServiceMetrics(reg)
	pool := NewWorkerPool(metrics)

	// Verify initial state: active workers should be 0.
	if val := testutil.ToFloat64(metrics.ActiveWorkers); val != 0 {
		t.Fatalf("expected 0 active workers initially, got %f", val)
	}

	// Process simulated tasks
	pool.ProcessTask("email", 50*time.Millisecond, true)
	pool.ProcessTask("email", 40*time.Millisecond, true)
	pool.ProcessTask("email", 120*time.Millisecond, false)

	// Verify task counters using testutil.ToFloat64
	successCounter := metrics.TaskCounter.WithLabelValues("email", "success")
	if val := testutil.ToFloat64(successCounter); val != 2 {
		t.Errorf("expected 2 successful email tasks, got %f", val)
	}

	failureCounter := metrics.TaskCounter.WithLabelValues("email", "failure")
	if val := testutil.ToFloat64(failureCounter); val != 1 {
		t.Errorf("expected 1 failed email task, got %f", val)
	}

	// Active workers should be back to 0 after processing tasks
	if val := testutil.ToFloat64(metrics.ActiveWorkers); val != 0 {
		t.Errorf("expected 0 active workers after completion, got %f", val)
	}
}

// TestServiceMetrics_Exposition demonstrates verifying the full formatted text output
// using testutil.CollectAndCompare.
func TestServiceMetrics_Exposition(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewServiceMetrics(reg)
	pool := NewWorkerPool(metrics)

	pool.ProcessTask("image_processing", 100*time.Millisecond, true)
	metrics.QueueDepth.WithLabelValues("default").Set(7)

	expectedQueue := `
# HELP app_queue_depth Current number of queued items per partition.
# TYPE app_queue_depth gauge
app_queue_depth{partition="default"} 7
`
	if err := testutil.CollectAndCompare(metrics.QueueDepth, strings.NewReader(expectedQueue), "app_queue_depth"); err != nil {
		t.Fatalf("unexpected metric output:\n%s", err)
	}

	expectedCounter := `
# HELP app_tasks_processed_total Total number of tasks processed, partitioned by type and status.
# TYPE app_tasks_processed_total counter
app_tasks_processed_total{status="success",type="image_processing"} 1
`
	if err := testutil.CollectAndCompare(metrics.TaskCounter, strings.NewReader(expectedCounter), "app_tasks_processed_total"); err != nil {
		t.Fatalf("unexpected metric output:\n%s", err)
	}
}

// TestServiceMetrics_Lint demonstrates using testutil.CollectAndLint to verify that
// metrics adhere to standard Prometheus naming, suffixes, and best practices.
func TestServiceMetrics_Lint(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewServiceMetrics(reg)

	// Lint all collectors registered with the metric set
	collectors := []prometheus.Collector{
		metrics.TaskCounter,
		metrics.ActiveWorkers,
		metrics.QueueDepth,
		metrics.TaskDuration,
	}

	for _, c := range collectors {
		problems, err := testutil.CollectAndLint(c)
		if err != nil {
			t.Fatalf("linting collector failed: %v", err)
		}
		if len(problems) > 0 {
			t.Errorf("found %d lint problem(s) in collector: %v", len(problems), problems)
		}
	}
}
