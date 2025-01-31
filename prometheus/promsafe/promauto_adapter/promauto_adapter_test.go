// Copyright 2024 The Prometheus Authors
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

package promauto_adapter_test

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promsafe"

	promauto "github.com/prometheus/client_golang/prometheus/promsafe/promauto_adapter"
)

type EventType int64

const (
	EventTypeUndefined EventType = iota
	EventTypeUser
	EventTypeSystem
)

// String returns stringified value of EventType enum
func (et EventType) String() string {
	switch et {
	case EventTypeUser:
		return "user"
	case EventTypeSystem:
		return "system"
	default:
		return "undefined"
	}
}

func ExampleNewCounterVec_promauto_adapted() {
	// Examples on how to migrate from promauto to promsafe gently
	//
	// Before:
	//   import "github.com/prometheus/client_golang/prometheus/promauto"
	// func main() {
	//   myReg := prometheus.NewRegistry()
	//   counterOpts := prometheus.CounterOpts{Name:"..."}
	//   promauto.With(myReg).NewCounterVec(counterOpts, []string{"event_type", "source"})
	// }
	//
	// After:
	//
	//   import (
	//     promauto "github.com/prometheus/client_golang/prometheus/promsafe/promauto_adapter"
	//   )
	//   ...

	myReg := prometheus.NewRegistry()
	counterOpts := prometheus.CounterOpts{
		Name: "items_counted_detailed_auto",
	}

	type MyLabels struct {
		promsafe.StructLabelProvider
		EventType EventType
		Source    string
	}
	// if declare Mylabels as global type you can add .ToPrometheusLabels() method
	// that will use fast labels convertion instead of automatic reflect-based

	c := promauto.With[MyLabels](myReg).NewCounterVec(counterOpts)

	c.With(MyLabels{
		EventType: EventTypeUser, Source: "source1",
	}).Inc()

	// Output:
}
