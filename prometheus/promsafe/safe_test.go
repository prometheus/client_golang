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

package promsafe_test

import (
	"log"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promsafe"
)

func ExampleNewCounterVecT_single_label_manual() {
	// Manually registering with a single label

	c := promsafe.NewCounterVecT(prometheus.CounterOpts{
		Name: "items_counted_by_status",
	}, promsafe.SingleLabelProvider("status"))

	// Manually register the counter
	if err := prometheus.Register(c.Unsafe()); err != nil {
		log.Fatal("could not register: ", err.Error())
	}

	c.With("active").Inc()

	// Output:
}

func ExampleNewCounterVecT_multiple_labels_manual() {
	// Manually registering with multiple labels

	type MyCounterLabels struct {
		promsafe.StructLabelProvider
		EventType string
		Success   bool
		Position  uint8 // yes, it's a number, but be careful with high-cardinality labels

		ShouldNotBeUsed string `promsafe:"-"`
	}

	c := promsafe.NewCounterVecT(prometheus.CounterOpts{
		Name: "items_counted_detailed",
	}, new(MyCounterLabels))

	// Manually register the counter
	if err := prometheus.Register(c.Unsafe()); err != nil {
		log.Fatal("could not register: ", err.Error())
	}

	// and now, because of generics we can call Inc() with filled struct of labels:
	counter := c.With(&MyCounterLabels{
		EventType: "reservation", Success: true, Position: 1,
	})
	counter.Inc()

	// Output:
}

func ExampleNewCounterVecT_promauto_migrated() {
	// Examples on how to migrate from promauto to promsafe
	// When promauto was using a custom factory with custom registry

	myReg := prometheus.NewRegistry()

	counterOpts := prometheus.CounterOpts{
		Name: "items_counted_detailed_auto",
	}

	// Old unsafe code
	// promauto.With(myReg).NewCounterVec(counterOpts, []string{"event_type", "source"})
	// becomes:

	type TicketReservationAttemptsLabels struct {
		promsafe.StructLabelProvider
		EventType string
		Source    string
	}
	c := promsafe.WithAuto(myReg).NewCounterVecT(counterOpts, new(TicketReservationAttemptsLabels))

	c.With(&TicketReservationAttemptsLabels{
		EventType: "reservation", Source: "source1",
	}).Inc()

	// Output:
}

func ExampleNewCounterVecT_promauto_global_migrated() {
	// Examples on how to migrate from promauto to promsafe
	// when promauto public API was used (with default registry)

	// Setup so every NewCounter* call will use default registry
	// like promauto does
	// Note: it actually accepts other registry to become a default one
	promsafe.SetupGlobalPromauto()

	counterOpts := prometheus.CounterOpts{
		Name: "items_counted_detailed_auto_global",
	}

	// Old code:
	//c := promauto.NewCounterVec(counterOpts, []string{"status", "source"})
	//c.With(prometheus.Labels{
	//	"status": "active",
	//	"source": "source1",
	//}).Inc()
	// becomes:

	type TicketReservationAttemptsLabels struct {
		promsafe.StructLabelProvider
		Status string
		Source string
	}
	c := promsafe.NewCounterVecT(counterOpts, new(TicketReservationAttemptsLabels))

	c.With(&TicketReservationAttemptsLabels{
		Status: "active", Source: "source1",
	}).Inc()

	// Output:
}
