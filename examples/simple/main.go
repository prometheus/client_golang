// Copyright (c) 2012, Matt T. Proud
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// main.go provides a simple skeletal example of how this instrumentation
// framework is registered and invoked.
package main

import (
	"flag"
	"github.com/prometheus/client_golang"
	"net/http"
)

var (
	listeningAddress string
)

func init() {
	flag.StringVar(&listeningAddress, "listeningAddress", ":8080", "The address to listen to requests on.")
}

func main() {
	flag.Parse()

	http.Handle(registry.ExpositionResource, registry.DefaultHandler)
	http.ListenAndServe(listeningAddress, nil)
}
