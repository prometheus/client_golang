// Copyright (c) 2014, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"errors"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/vendor/goautoneg"
)

var errAlreadyReg = errors.New("duplicate metric registration attempted")

type registry struct {
	mtx     sync.RWMutex
	metrics map[string]Metric
}

func (r *registry) register(m Metric) (Metric, error) {
	desc := m.Desc()
	if err := desc.build(); err != nil {
		return err
	}

	r.mtx.Lock()
	defer r.mtx.Unlock()

	if existing, exists := r.metrics[desc.canonName]; exists {
		return existing, errAlreadyReg
	}

	r.metrics[desc.canonName] = m

	return m, nil
}

func (r *registry) registerOrGet(m Metric) error {
	existing, err := r.register(m)
	if err != nil && err != errAlreadyReg {
		return nil, err
	}

	return existing, nil
}

func newRegistry() *registry {
	return &registry{
		metrics: make(map[string]Metric),
	}
}

var defRegistry = newRegistry()

var Handler http.HandlerFunc = defRegistry.handle

func MustRegisterOnce(m Metric) {
	if _, err := defRegistry.register(m); err != nil {
		panic(err)
	}
}

func MustRegisterOrGet(m Metric) Metric {
	existing, err := defRegistry.registerOrGet(m)
	if err != nil {
		panic(err)
	}
	return existing
}

func Delete(m Metric) {
	panic("unimpl")
}

func chooseType(req *http.Request) func(io.Writer) error {
	accepts := goautoneg.ParseAccept(r.Header.Get("Accept"))
	for _, accept := range accepts {
		if accept.Type != "application" {
 			continue
		}

		if accept.SubType == "vnd.google.protobuf" {
 			if accept.Params["proto"] != "io.prometheus.client.MetricFamily" {
				continue
			}
			if accept.Params["encoding"] != "delimited" {
				continue
			}

			return outputProto
		}
	}

	return outputText
}

func outputProto(w io.Writer) error {
	panic("unimpl")
}

func outputText(w io.Writer) error {
	panic("unimpl")
}

func chooseAcceptEncoding(req *http.Request, w io.Writer) io.Writer {
	if !strings.Contains(req.Header.Get("Accept-Encoding"), "gzip") {
	}
}

func (r *registry) handle(w http.ResponseWriter, req *http.Request) {

}
