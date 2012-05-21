// Copyright (c) 2012, Matt T. Proud
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// main.go provides a simple skeletal example of how this instrumentation
// framework is registered and invoked.

package main

import (
	"github.com/matttproud/golang_instrumentation"
	"net/http"
)

func main() {
	exporter := registry.DefaultRegistry.YieldExporter()

	http.Handle("/metrics.json", exporter)
	http.ListenAndServe(":8080", nil)
}
