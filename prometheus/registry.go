// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package prometheus

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"sync"

	dto "github.com/prometheus/client_model/go"

	"github.com/prometheus/client_golang/_vendor/goautoneg"
	"github.com/prometheus/client_golang/text"
)

var errAlreadyReg = errors.New("duplicate metric registration attempted")

const (
	numBufs = 4

	contentTypeHeader = "Content-Type"

	// APIVersion is the version of the format of the exported data.  This
	// will match this library's version, which subscribes to the Semantic
	// Versioning scheme.
	APIVersion = "0.0.4"

	// DelimitedTelemetryContentType is the content type set on telemetry
	// data responses in delimited protobuf format.
	DelimitedTelemetryContentType = `application/vnd.google.protobuf; proto="io.prometheus.client.MetricFamily"; encoding="delimited"`
	// TextTelemetryContentType is the content type set on telemetry data
	// responses in text format.
	TextTelemetryContentType = `text/plain; version=` + APIVersion
	// ProtoTextTelemetryContentType is the content type set on telemetry
	// data responses in protobuf text format.  (Only used for debugging.)
	ProtoTextTelemetryContentType = `application/vnd.google.protobuf; proto="io.prometheus.client.MetricFamily"; encoding="text"`
	// ProtoCompactTextTelemetryContentType is the content type set on
	// telemetry data responses in protobuf compact text format.  (Only used
	// for debugging.)
	ProtoCompactTextTelemetryContentType = `application/vnd.google.protobuf; proto="io.prometheus.client.MetricFamily"; encoding="compact-text"`
)

// encoder is a function that writes a dto.MetricFamily to an io.Writer in a
// certain encoding. It returns the number of bytes written and any error
// encountered.  Note that ext.WriteDelimited and text.MetricFamilyToText are
// encoders.
type encoder func(io.Writer, *dto.MetricFamily) (int, error)

type registry struct {
	mtx                       sync.RWMutex
	metrics                   map[string]Metric
	bufs                      chan *bytes.Buffer
	metricFamilyInjectionHook func() []*dto.MetricFamily
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

func SetMetricFamilyInjectionHook(hook func() []*dto.MetricFamily) {
	defRegistry.metricFamilyInjectionHook = hook
}

func chooseEncoder(req *http.Request) (encoder, string) {
	accepts := goautoneg.ParseAccept(req.Header.Get("Accept"))
	for _, accept := range accepts {
		switch {
		case accept.Type == "application" &&
			accept.SubType == "vnd.google.protobuf" &&
			accept.Params["proto"] == "io.prometheus.client.MetricFamily":
			switch accept.Params["encoding"] {
			case "delimited":
				return text.WriteProtoDelimited, DelimitedTelemetryContentType
			case "text":
				return text.WriteProtoText, ProtoTextTelemetryContentType
			case "compact-text":
				return text.WriteProtoCompactText, ProtoCompactTextTelemetryContentType
			default:
				continue
			}
		case accept.Type == "text" &&
			accept.SubType == "plain" &&
			(accept.Params["version"] == "0.0.4" || accept.Params["version"] == ""):
			return text.MetricFamilyToText, TextTelemetryContentType
		default:
			continue
		}
	}
	return text.MetricFamilyToText, TextTelemetryContentType
}

func (r *registry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	enc, contentType := chooseEncoder(req)
	buf := r.getBuf()
	defer r.giveBuf(buf)
	header := w.Header()
	header.Set(contentTypeHeader, contentType)
	if _, err := r.writePB(buf, enc); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	if _, err := r.writeExternalPB(buf, enc); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	w.Write(buf.Bytes())
}

func (r *registry) writePB(w io.Writer, writeEncoded encoder) (int, error) {
	// TODO implement
	return 0, nil
}

func (r *registry) writeExternalPB(w io.Writer, writeEncoded encoder) (int, error) {
	var written int
	if r.metricFamilyInjectionHook == nil {
		return 0, nil
	}
	for _, f := range r.metricFamilyInjectionHook() {
		i, err := writeEncoded(w, f)
		written += i
		if err != nil {
			return i, err
		}
	}
	return written, nil
}
