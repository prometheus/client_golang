// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package prometheus

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"sort"
	"sync"
	"code.google.com/p/goprotobuf/proto"

	dto "github.com/prometheus/client_model/go"

	"github.com/prometheus/client_golang/_vendor/goautoneg"
	"github.com/prometheus/client_golang/text"
)

var errAlreadyReg = errors.New("duplicate metrics collector registration attempted")

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
	metricsCollectorsByID     map[uint64]MetricsCollector // ID is a hash of the descIDs.
	descIDs                   map[uint64]struct{}
	dimHashesByName           map[string]uint64
	bufs                      chan *bytes.Buffer
	metricFamilyInjectionHook func() []*dto.MetricFamily
}

func (r *registry) Register(m MetricsCollector) (MetricsCollector, error) {
	descs := m.DescribeMetrics()
	collectorID, err := buildDescsAndCalculateCollectorID(descs)
	if err != nil {
		return m, err
	}

	r.mtx.Lock()
	defer r.mtx.Unlock()

	if existing, exists := r.metricsCollectorsByID[collectorID]; exists {
		return existing, errAlreadyReg
	}

	// Test consistency and uniqueness.
	newDescIDs := map[uint64]struct{}{}
	newDimHashesByName := map[string]uint64{}
	for _, desc := range descs {
		// descID uniqueness, i.e. canonName and preset label values.
		if _, exists := r.descIDs[desc.id]; exists {
			return nil, fmt.Errorf("descriptor %+v already exists with the same fully-qualified name and preset label values", desc)
		}
		if _, exists := newDescIDs[desc.id]; exists {
			return nil, fmt.Errorf("metrics collector has two descriptors with the same fully-qualified name and preset label values, offender is %+v", desc)
		}
		newDescIDs[desc.id] = struct{}{}
		// Dimension consistency, i.e. label names, type, help.
		if dimHash, exists := r.dimHashesByName[desc.canonName]; exists {
			if dimHash != desc.dimHash {
				return nil, fmt.Errorf("previously registered descriptors with the same fully qualified name as %+v have different label dimensions, help string, or type", desc)
			}
		} else {
			if dimHash, exists := newDimHashesByName[desc.canonName]; exists {
				if dimHash != desc.dimHash {
					return nil, fmt.Errorf("metrics collector has inconsistent label dimensions, help string, or type for the same fully-qualified name, offender is %+v", desc)
				}
			}
			newDimHashesByName[desc.canonName] = desc.dimHash
		}
	}
	// Only after all tests have passed, actually register.
	r.metricsCollectorsByID[collectorID] = m
	for hash := range newDescIDs {
		r.descIDs[hash] = struct{}{}
	}
	for name, dimHash := range newDimHashesByName {
		r.dimHashesByName[name] = dimHash
	}
	return m, nil
}

func (r *registry) RegisterOrGet(m MetricsCollector) (MetricsCollector, error) {
	existing, err := r.Register(m)
	if err != nil && err != errAlreadyReg {
		return nil, err
	}
	return existing, nil
}

func (r *registry) Unregister(m MetricsCollector) (bool, error) {
	descs := m.DescribeMetrics()
	collectorID, err := buildDescsAndCalculateCollectorID(descs)
	if err != nil {
		return false, err
	}

	r.mtx.RLock()
	if _, ok := r.metricsCollectorsByID[collectorID]; !ok {
		r.mtx.RUnlock()
		return false, nil
	}
	r.mtx.RUnlock()

	r.mtx.Lock()
	defer r.mtx.Unlock()

	delete(r.metricsCollectorsByID, collectorID)
	for _, desc := range descs {
		delete(r.descIDs, desc.id)
	}
	// dimHashesByName is left untouched as those must be consistent
	// throughout the lifetime of a program.
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

func buildDescsAndCalculateCollectorID(descs []*Desc) (uint64, error) {
	if len(descs) == 0 {
		return 0, errNoDesc
	}
	h := fnv.New64a()
	for _, desc := range descs {
		if err := desc.build(); err != nil {
			return 0, err
		}
		binary.Write(h, binary.BigEndian, desc.id)
	}
	return h.Sum64(), nil
}

func newRegistry() *registry {
	return &registry{
		metricsCollectorsByID: map[uint64]MetricsCollector{},
		descIDs:               map[uint64]struct{}{},
		dimHashesByName:       map[string]uint64{},
		bufs:                  make(chan *bytes.Buffer, numBufs),
	}
}

var defRegistry = newRegistry()

// Handler is the default Prometheus http.HandlerFunc for the global metric
// registry.
var Handler = InstrumentHandler("prometheus", defRegistry)

// MustRegister enrolls a new metrics collector.  It panics if the provided
// descriptors are problematic or at least one of them shares the same name and
// preset labels with one that is already registered.  It returns the enrolled
// metrics collector. Do not register the same MetricsCollector multiple times
// concurrently.
func MustRegister(m MetricsCollector) MetricsCollector {
	m, err := defRegistry.Register(m)
	if err != nil {
		panic(err)
	}
	return m
}

// MustRegisterOrGet enrolls a new metrics collector once and only once.  It panics if the
// provided Desc is invalid.  It returns the enrolled metric or the existing
// one. Do not register the same MetricsCollector multiple times
// concurrently.
func MustRegisterOrGet(m MetricsCollector) MetricsCollector {
	existing, err := defRegistry.RegisterOrGet(m)
	if err != nil {
		panic(err)
	}
	return existing
}

// Unregister unenrolls a metric returning whether the metric was unenrolled and
// whether an error existed.
func Unregister(m MetricsCollector) (bool, error) {
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
	// - Write resulting merged MetricFamilies with encoder (sorted by name).

	metricFamiliesByName := make(map[string]*dto.MetricFamily, len(r.dimHashesByName))
	collectorIDs := make([]uint64, 0, len(r.metricsCollectorsByID))
	collectors := make([]MetricsCollector, 0, len(r.metricsCollectorsByID))

	r.mtx.RLock()
	// For reproducible order, sort MetricsCollectors by their ID.
	for collectorID := range r.metricsCollectorsByID {
		collectorIDs = append(collectorIDs, collectorID)
	}
	sort.Sort(hashSorter(collectorIDs))
	for _, collectorID := range collectorIDs {
		collectors = append(collectors, r.metricsCollectorsByID[collectorID])
	}
	defer r.mtx.RUnlock()

	for _, collector := range collectors {
		// TODO: Vet concurrent collection of metrics.
		for _, metric := range collector.CollectMetrics() {
			desc := metric.Desc()
			// TODO: Optional check if desc is an element of collector.DescribeMetrics().
			metricFamily, ok := metricFamiliesByName[desc.canonName]
			if !ok {
				// TODO: Vet getting MetricFamily object from pool.
				metricFamily = &dto.MetricFamily{
					Name: proto.String(desc.canonName),
					Help: proto.String(desc.Help),
					Type: desc.Type.Enum(),
				}
				metricFamiliesByName[desc.canonName] = metricFamily
			}
			// TODO: Vet getting Metric object from pool.
			dtoMetric := &dto.Metric{}
			metric.Write(dtoMetric)
			// TODO: Optional check if dtoMetric is consistent with desc.
			metricFamily.Metric = append(metricFamily.Metric, dtoMetric)
		}
	}
	names := make([]string, 0, len(metricFamiliesByName))
	for name := range metricFamiliesByName {
		names = append(names, name)
	}
	sort.Strings(names)

	var written int
	for _, name := range names {
		w, err := writeEncoded(w, metricFamiliesByName[name])
		written += w
		if err != nil {
			return written, err
		}
	}
	return written, nil
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
