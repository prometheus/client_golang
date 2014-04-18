// Copyright (c) 2014, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"net/http"
)

func ExampleMustRegister() {
	var gauge = NewGauge(GaugeDesc{
		Desc{
			Name: "my_spiffy_metric",
			Help: "it's spiffy description",
		},
	})

	MustRegister(gauge)
}

func ExampleMustRegisterOrGet() {
	// I may have already registered this.
	var gauge = MustRegisterOrGet(NewGauge(GaugeDesc{
		Desc{
			Name: "my_spiffy_metric",
			Help: "it's spiffy description",
		},
	})).(Gauge)

	gauge.Set(42)
}

func ExampleUnregister() {
	var oldAndBusted Gauge // I no longer need this!
	Unregister(oldAndBusted)
}

func ExampleHandler() {
	http.Handle("/metrics", Handler) // Easy!
}
