// Copyright (c) 2014, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/_vendor/goautoneg"
)

var errAlreadyReg = errors.New("duplicate metric registration attempted")

const numBufs = 4

type registry struct {
	mtx     sync.RWMutex
	metrics map[string]Metric

	bufs chan *bytes.Buffer
}

func (r *registry) Register(m Metric) (Metric, error) {
	desc := m.Desc()
	if err := desc.build(); err != nil {
		return nil, err
	}

	r.mtx.Lock()
	defer r.mtx.Unlock()

	if existing, exists := r.metrics[desc.canonName]; exists {
		return existing, errAlreadyReg
	}

	r.metrics[desc.canonName] = m

	return m, nil
}

func (r *registry) RegisterOrGet(m Metric) (Metric, error) {
	existing, err := r.Register(m)
	if err != nil && err != errAlreadyReg {
		return nil, err
	}

	return existing, nil
}

func (r *registry) Unregister(m Metric) (bool, error) {
	desc := m.Desc()
	if err := desc.build(); err != nil {
		return false, err
	}

	r.mtx.Lock()
	defer r.mtx.Unlock()

	if _, ok := r.metrics[desc.canonName]; !ok {
		return false, nil
	}
	delete(r.metrics, desc.canonName)

	return true, nil
}

func (r *registry) getBuf() *bytes.Buffer {
	select {
	case buf := <-r.bufs:
		return buf
	default:
		return &bytes.Buffer{}
	}
}

func (r *registry) giveBuf(buf *bytes.Buffer) {
	select {
	case r.bufs <- buf:
		buf.Reset()
	default:
	}
}

func newRegistry() *registry {
	return &registry{
		metrics: make(map[string]Metric),
		bufs:    make(chan *bytes.Buffer, numBufs),
	}
}

var defRegistry = newRegistry()

// Handler is the default Prometheus http.HandlerFunc for the global metric
// registry.
var Handler = InstrumentHandler("prometheus", defRegistry)

// MustRegister enrolls a new metric.  It panics if the provided Desc is
// problematic or the metric is already registered.  It returns the enrolled
// metric.
func MustRegister(m Metric) Metric {
	m, err := defRegistry.Register(m)
	if err != nil {
		panic(err)
	}
	return m
}

// MustRegisterOrGet enrolls a new metric once and only once.  It panics if the
// provided Desc is invalid.  It returns the enrolled metric or the existing
// one.
func MustRegisterOrGet(m Metric) Metric {
	existing, err := defRegistry.RegisterOrGet(m)
	if err != nil {
		panic(err)
	}
	return existing
}

// Unregister unenrolls a metric returning whether the metric was unenrolled and
// whether an error existed.
func Unregister(m Metric) (bool, error) {
	return defRegistry.Unregister(m)
}

func chooseType(req *http.Request) func(io.Writer) error {
	accepts := goautoneg.ParseAccept(req.Header.Get("Accept"))
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

func (r *registry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	format := chooseType(req)
	buf := r.getBuf()
	defer r.giveBuf(buf)
	if err := format(buf); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}
