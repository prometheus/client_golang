// Copyright (c) 2014, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"http"
)

func ExampleInstrHandler() {
	var myHandler http.Handler

	http.Handle("/profile", InstrHandler("profile", myHandler))
	// ... and without further ado, you get
	// - request count
	// - request size
	// - response size
	// - total latency
	//
	// all partitioned by
	// - handler name
	// - status code
	// - HTTP method
}
