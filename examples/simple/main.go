// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// A simple skeletal example of how this instrumentation framework is registered
// and invoked.  Literally, this is the bare bones.
package main

import (
	"flag"
	"github.com/prometheus/client_golang/prometheus"
	"net/http"
)

func main() {
	flag.Parse()

	http.Handle(prometheus.ExpositionResource, prometheus.DefaultHandler)
	http.ListenAndServe(*listeningAddress, nil)
}

var (
	listeningAddress = flag.String("listeningAddress", ":8080", "The address to listen to requests on.")
)
