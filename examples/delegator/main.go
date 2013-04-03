// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This skeletal example of the telemetry library is provided to demonstrate the
// use of boilerplate HTTP delegation telemetry methods.
package main

import (
	"flag"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/exp"
	"net/http"
)

// helloHandler demonstrates the DefaultCoarseMux's ability to sniff a
// http.ResponseWriter (specifically http.response) implicit setting of
// a response code.
func helloHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello, hello, hello..."))
}

// goodbyeHandler demonstrates the DefaultCoarseMux's ability to sniff an
// http.ResponseWriter (specifically http.response) explicit setting of
// a response code.
func goodbyeHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusGone)
	w.Write([]byte("... and now for the big goodbye!"))
}

// teapotHandler demonstrates the DefaultCoarseMux's ability to sniff an
// http.ResponseWriter (specifically http.response) explicit setting of
// a response code for pure comedic value.
func teapotHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusTeapot)
	w.Write([]byte("Short and stout..."))
}

var (
	listeningAddress = flag.String("listeningAddress", ":8080", "The address to listen to requests on.")
)

func main() {
	flag.Parse()

	exp.HandleFunc("/hello", helloHandler)
	exp.HandleFunc("/goodbye", goodbyeHandler)
	exp.HandleFunc("/teapot", teapotHandler)
	exp.Handle(prometheus.ExpositionResource, prometheus.DefaultHandler)

	http.ListenAndServe(*listeningAddress, exp.DefaultCoarseMux)
}
