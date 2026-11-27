// Copyright 2017 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package promhttp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"

	"github.com/prometheus/client_golang/prometheus"
)

func TestInstrumentHandlerReadFromContentLength(t *testing.T) {
	type copyResult struct {
		n   int64
		err error
	}
	// The handler declares 768 bytes but copies from a 1024-byte source.
	payload := bytes.Repeat([]byte("0123456789abcdef"), 64)
	copied := make(chan copyResult, 1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "768")
		// Hide WriteTo so io.Copy calls the response writer's ReadFrom.
		n, err := io.Copy(w, struct{ io.Reader }{bytes.NewReader(payload)})
		copied <- copyResult{n, err}
	})
	run := func(t *testing.T, h http.Handler) copyResult {
		t.Helper()
		srv := httptest.NewServer(h)
		defer srv.Close()
		resp, err := srv.Client().Get(srv.URL)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Error(err)
		}
		if !bytes.Equal(body, payload[:768]) {
			t.Errorf("body (%d bytes) differs from the first 768 source bytes", len(body))
		}
		return <-copied
	}
	// Go 1.27 added a length check to the part of ReadFrom that copies
	// directly to TCP (https://go.dev/cl/755701). Use the plain handler's
	// copy result as the expectation for the running Go version.
	want := run(t, handler)
	counter := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "requests_total"}, []string{"code"})
	timer := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "header_seconds"}, nil)
	for name, h := range map[string]http.Handler{
		"counter":              InstrumentHandlerCounter(counter, handler),
		"time to write header": InstrumentHandlerTimeToWriteHeader(timer, handler),
	} {
		t.Run(name, func(t *testing.T) {
			got := run(t, h)
			if got.n != want.n || !errors.Is(got.err, want.err) {
				t.Errorf("io.Copy returned (%d, %v), plain handler returned (%d, %v)", got.n, got.err, want.n, want.err)
			}
		})
	}
}

func TestTimeToWriteHeaderReadFromAfterRejectedWrite(t *testing.T) {
	run := func(timed bool) int64 {
		copied := make(chan int64, 1)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Length", "1")
			if _, err := w.Write([]byte("abcd")); !errors.Is(err, http.ErrContentLength) {
				t.Errorf("initial Write returned %v, want ErrContentLength", err)
			}
			w.(http.Flusher).Flush()
			if timed {
				w = newDelegator(w, func(int) {})
			}
			src := struct{ io.Reader }{strings.NewReader("abcd")}
			n, err := w.(io.ReaderFrom).ReadFrom(src)
			if err != nil {
				t.Error(err)
			}
			copied <- n
		}))
		defer srv.Close()
		resp, err := srv.Client().Get(srv.URL)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		return <-copied
	}
	// Go 1.27 enforces the declared length on the copy handed to TCP.
	if want, got := run(false), run(true); got != want {
		t.Errorf("timed ReadFrom copied %d bytes, plain handler copied %d", got, want)
	}
}

func TestTimeToWriteHeaderReadFromFlushesBeforeNextRead(t *testing.T) {
	first := strings.Repeat("x", 512)
	tail, release := io.Pipe()
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// After 512 bytes, the source blocks until the test closes the pipe.
		_, _ = w.(io.ReaderFrom).ReadFrom(io.MultiReader(strings.NewReader(first), tail))
	})
	timer := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "header_seconds"}, nil)
	srv := httptest.NewServer(InstrumentHandlerTimeToWriteHeader(timer, handler))
	defer srv.Close()
	defer release.Close()

	client := srv.Client()
	client.Timeout = 5 * time.Second
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	got := make([]byte, len(first))
	if _, err := io.ReadFull(resp.Body, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != first {
		t.Error("client received different bytes from the source")
	}
}

func TestTimeToWriteHeaderTLSReadFromUDP(t *testing.T) {
	src, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	packet := bytes.Repeat([]byte("x"), 1024)
	if _, err := src.WriteToUDP(packet, src.LocalAddr().(*net.UDPAddr)); err != nil {
		t.Fatal(err)
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// A short first read discards the rest of the only datagram.
		if err := src.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Error(err)
			return
		}
		if _, err := io.CopyN(w, src, int64(len(packet))); err != nil {
			t.Error(err)
		}
	})
	timer := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "header_seconds"}, nil)
	// Over TLS, net/http can read the whole datagram at once. On plain
	// TCP, net/http itself limits the first read of a new response to 512 bytes.
	srv := httptest.NewTLSServer(InstrumentHandlerTimeToWriteHeader(timer, handler))
	defer srv.Close()
	resp, err := srv.Client().Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body, packet) {
		t.Errorf("response body (%d bytes) differs from the UDP datagram (%d bytes)", len(body), len(packet))
	}
}

func TestTimeToWriteHeaderReadFromWriterTo(t *testing.T) {
	for _, network := range []string{"TLS", "Unix"} {
		t.Run(network, func(t *testing.T) {
			testTimeToWriteHeaderReadFromWriterTo(t, network)
		})
	}
}

func testTimeToWriteHeaderReadFromWriterTo(t *testing.T, network string) {
	timer := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "header_seconds"}, nil)
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "40960")
		// On both connections net/http uses strings.Reader.WriteTo. Its 64 KiB
		// write exceeds the declared 40 KiB, so net/http rejects it entirely.
		src := strings.NewReader(strings.Repeat("x", 64<<10))
		n, err := w.(io.ReaderFrom).ReadFrom(src)
		if n != 0 || !errors.Is(err, http.ErrContentLength) {
			t.Errorf("ReadFrom returned (%d, %v), want (0, ErrContentLength)", n, err)
		}
	})
	timed := InstrumentHandlerTimeToWriteHeader(timer, handler)
	var srv *httptest.Server
	if network == "TLS" {
		srv = httptest.NewTLSServer(timed)
	} else {
		// A relative name keeps the socket path within the platform limit.
		t.Chdir(t.TempDir())
		listener, err := net.Listen("unix", "http.sock")
		if err != nil {
			t.Skipf("Unix socket unavailable: %v", err)
		}
		srv = &httptest.Server{Listener: listener, Config: &http.Server{Handler: timed}}
		srv.Start()
		srv.Client().Transport.(*http.Transport).DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", listener.Addr().String())
		}
	}
	defer srv.Close()
	resp, err := srv.Client().Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if len(body) != 0 || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("client read %d bytes, err=%v, want 0 bytes and unexpected EOF", len(body), err)
	}
	var metric dto.Metric
	if err := timer.WithLabelValues().(prometheus.Metric).Write(&metric); err != nil {
		t.Fatal(err)
	}
	if got := metric.GetHistogram().GetSampleCount(); got != 1 {
		t.Errorf("got %d header observations, want 1", got)
	}
}

func TestLabelCheck(t *testing.T) {
	scenarios := map[string]struct {
		metricName    string // Defaults to "c".
		varLabels     []string
		constLabels   []string
		curriedLabels []string
		dynamicLabels []string
		ok            bool
	}{
		"empty": {
			varLabels:     []string{},
			constLabels:   []string{},
			curriedLabels: []string{},
			ok:            true,
		},
		"code as single var label": {
			varLabels:     []string{"code"},
			constLabels:   []string{},
			curriedLabels: []string{},
			ok:            true,
		},
		"method as single var label": {
			varLabels:     []string{"method"},
			constLabels:   []string{},
			curriedLabels: []string{},
			ok:            true,
		},
		"code and method as var labels": {
			varLabels:     []string{"method", "code"},
			constLabels:   []string{},
			curriedLabels: []string{},
			ok:            true,
		},
		"valid case with all labels used": {
			varLabels:     []string{"code", "method"},
			constLabels:   []string{"foo", "bar"},
			curriedLabels: []string{"dings", "bums"},
			dynamicLabels: []string{"dyn", "amics"},
			ok:            true,
		},
		"all labels used with an invalid const label name": {
			varLabels:     []string{"code", "method"},
			constLabels:   []string{"in\x80valid", "bar"},
			curriedLabels: []string{"dings", "bums"},
			dynamicLabels: []string{"dyn", "amics"},
			ok:            false,
		},
		"unsupported var label": {
			varLabels:     []string{"foo"},
			constLabels:   []string{},
			curriedLabels: []string{},
			ok:            false,
		},
		"mixed var labels": {
			varLabels:     []string{"method", "foo", "code"},
			constLabels:   []string{},
			curriedLabels: []string{},
			ok:            false,
		},
		"unsupported var label but curried": {
			varLabels:     []string{},
			constLabels:   []string{},
			curriedLabels: []string{"foo"},
			ok:            true,
		},
		"mixed var labels but unsupported curried": {
			varLabels:     []string{"code", "method"},
			constLabels:   []string{},
			curriedLabels: []string{"foo"},
			ok:            true,
		},
		"supported label as const and curry": {
			varLabels:     []string{},
			constLabels:   []string{"code"},
			curriedLabels: []string{"method"},
			ok:            true,
		},
		"supported label as const and dynamic": {
			varLabels:     []string{},
			constLabels:   []string{"code"},
			dynamicLabels: []string{"method"},
			ok:            true,
		},
		"supported label as curried and dynamic": {
			varLabels:     []string{},
			curriedLabels: []string{"code"},
			dynamicLabels: []string{"method"},
			ok:            true,
		},
		"supported label as const and curry with unsupported as var": {
			varLabels:     []string{"foo"},
			constLabels:   []string{"code"},
			curriedLabels: []string{"method"},
			ok:            false,
		},
		"invalid name and otherwise empty": {
			metricName:    "in\x80valid",
			varLabels:     []string{},
			constLabels:   []string{},
			curriedLabels: []string{},
			ok:            false,
		},
		"invalid name with all the otherwise valid labels": {
			metricName:    "in\x80valid",
			varLabels:     []string{"code", "method"},
			constLabels:   []string{"foo", "bar"},
			curriedLabels: []string{"dings", "bums"},
			dynamicLabels: []string{"dyn", "amics"},
			ok:            false,
		},
	}

	for name, sc := range scenarios {
		t.Run(name, func(t *testing.T) {
			metricName := sc.metricName
			if metricName == "" {
				metricName = "c"
			}
			constLabels := prometheus.Labels{}
			for _, l := range sc.constLabels {
				constLabels[l] = "dummy"
			}
			labelNames := append(append(sc.varLabels, sc.curriedLabels...), sc.dynamicLabels...)
			c := prometheus.V2.NewCounterVec(
				prometheus.CounterVecOpts{
					CounterOpts: prometheus.CounterOpts{
						Name:        metricName,
						Help:        "c help",
						ConstLabels: constLabels,
					},
					VariableLabels: prometheus.UnconstrainedLabels(labelNames),
				},
			)
			o := prometheus.ObserverVec(prometheus.V2.NewHistogramVec(
				prometheus.HistogramVecOpts{
					HistogramOpts: prometheus.HistogramOpts{
						Name:        metricName,
						Help:        "c help",
						ConstLabels: constLabels,
					},
					VariableLabels: prometheus.UnconstrainedLabels(labelNames),
				},
			))
			for _, l := range sc.curriedLabels {
				c = c.MustCurryWith(prometheus.Labels{l: "dummy"})
				o = o.MustCurryWith(prometheus.Labels{l: "dummy"})
			}
			opts := []Option{}
			for _, l := range sc.dynamicLabels {
				opts = append(opts, WithLabelFromCtx(l,
					func(_ context.Context) string {
						return "foo"
					},
				))
			}

			func() {
				defer func() {
					if err := recover(); err != nil {
						if sc.ok {
							t.Error("unexpected panic:", err)
						}
					} else if !sc.ok {
						t.Error("expected panic")
					}
				}()
				InstrumentHandlerCounter(c, nil, opts...)
			}()
			func() {
				defer func() {
					if err := recover(); err != nil {
						if sc.ok {
							t.Error("unexpected panic:", err)
						}
					} else if !sc.ok {
						t.Error("expected panic")
					}
				}()
				InstrumentHandlerDuration(o, nil, opts...)
			}()
			if sc.ok {
				// Test if wantCode and wantMethod were detected correctly.
				var wantCode, wantMethod bool
				for _, l := range sc.varLabels {
					if l == "code" {
						wantCode = true
					}
					if l == "method" {
						wantMethod = true
					}
				}
				// Curry the dynamic labels since this is done normally behind the scenes for the check
				for _, l := range sc.dynamicLabels {
					c = c.MustCurryWith(prometheus.Labels{l: "dummy"})
					o = o.MustCurryWith(prometheus.Labels{l: "dummy"})
				}
				gotCode, gotMethod := checkLabels(c)
				if gotCode != wantCode {
					t.Errorf("wanted code=%t for counter, got code=%t", wantCode, gotCode)
				}
				if gotMethod != wantMethod {
					t.Errorf("wanted method=%t for counter, got method=%t", wantMethod, gotMethod)
				}
				gotCode, gotMethod = checkLabels(o)
				if gotCode != wantCode {
					t.Errorf("wanted code=%t for observer, got code=%t", wantCode, gotCode)
				}
				if gotMethod != wantMethod {
					t.Errorf("wanted method=%t for observer, got method=%t", wantMethod, gotMethod)
				}
			}
		})
	}
}

func TestLabels(t *testing.T) {
	scenarios := map[string]struct {
		varLabels    []string
		reqMethod    string
		respStatus   int
		extraMethods []string
		wantLabels   prometheus.Labels
		ok           bool
	}{
		"empty": {
			varLabels:  []string{},
			wantLabels: prometheus.Labels{},
			reqMethod:  "GET",
			respStatus: 200,
			ok:         true,
		},
		"code as single var label": {
			varLabels:  []string{"code"},
			reqMethod:  "GET",
			respStatus: 200,
			wantLabels: prometheus.Labels{"code": "200"},
			ok:         true,
		},
		"code as single var label and out-of-range code": {
			varLabels:  []string{"code"},
			reqMethod:  "GET",
			respStatus: 99,
			wantLabels: prometheus.Labels{"code": "unknown"},
			ok:         true,
		},
		"code as single var label and in-range but unrecognized code": {
			varLabels:  []string{"code"},
			reqMethod:  "GET",
			respStatus: 308,
			wantLabels: prometheus.Labels{"code": "308"},
			ok:         true,
		},
		"method as single var label": {
			varLabels:  []string{"method"},
			reqMethod:  "GET",
			respStatus: 200,
			wantLabels: prometheus.Labels{"method": "get"},
			ok:         true,
		},
		"method as single var label and unknown method": {
			varLabels:  []string{"method"},
			reqMethod:  "CUSTOM_METHOD",
			respStatus: 200,
			wantLabels: prometheus.Labels{"method": "unknown"},
			ok:         true,
		},
		"code and method as var labels": {
			varLabels:  []string{"method", "code"},
			reqMethod:  "GET",
			respStatus: 200,
			wantLabels: prometheus.Labels{"method": "get", "code": "200"},
			ok:         true,
		},
		"method as single var label with extra methods specified": {
			varLabels:    []string{"method"},
			reqMethod:    "CUSTOM_METHOD",
			respStatus:   200,
			extraMethods: []string{"CUSTOM_METHOD", "CUSTOM_METHOD_1"},
			wantLabels:   prometheus.Labels{"method": "custom_method"},
			ok:           true,
		},
		"all labels used with an unknown method and out-of-range code": {
			varLabels:  []string{"code", "method"},
			reqMethod:  "CUSTOM_METHOD",
			respStatus: 99,
			wantLabels: prometheus.Labels{"method": "unknown", "code": "unknown"},
			ok:         false,
		},
	}
	checkLabels := func(labels []string) (gotCode, gotMethod bool) {
		for _, label := range labels {
			switch label {
			case "code":
				gotCode = true
			case "method":
				gotMethod = true
			default:
				panic("metric partitioned with non-supported labels for this test")
			}
		}
		return gotCode, gotMethod
	}
	equalLabels := func(gotLabels, wantLabels prometheus.Labels) bool {
		if len(gotLabels) != len(wantLabels) {
			return false
		}
		for ln, lv := range gotLabels {
			olv, ok := wantLabels[ln]
			if !ok {
				return false
			}
			if olv != lv {
				return false
			}
		}
		return true
	}

	for name, sc := range scenarios {
		t.Run(name, func(t *testing.T) {
			if sc.ok {
				gotCode, gotMethod := checkLabels(sc.varLabels)
				gotLabels := labels(gotCode, gotMethod, sc.reqMethod, sc.respStatus, sc.extraMethods...)
				if !equalLabels(gotLabels, sc.wantLabels) {
					t.Errorf("wanted labels=%v for counter, got code=%v", sc.wantLabels, gotLabels)
				}
			}
		})
	}
}

func makeInstrumentedHandler(handler http.HandlerFunc, opts ...Option) (http.Handler, *prometheus.Registry) {
	reg := prometheus.NewRegistry()

	inFlightGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "in_flight_requests",
		Help: "A gauge of requests currently being served by the wrapped handler.",
	})

	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_requests_total",
			Help: "A counter for requests to the wrapped handler.",
		},
		[]string{"code", "method"},
	)

	histVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:        "response_duration_seconds",
			Help:        "A histogram of request latencies.",
			Buckets:     prometheus.DefBuckets,
			ConstLabels: prometheus.Labels{"handler": "api"},
		},
		[]string{"method"},
	)

	writeHeaderVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:        "write_header_duration_seconds",
			Help:        "A histogram of time to first write latencies.",
			Buckets:     prometheus.DefBuckets,
			ConstLabels: prometheus.Labels{"handler": "api"},
		},
		[]string{},
	)

	responseSize := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "push_request_size_bytes",
			Help:    "A histogram of request sizes for requests.",
			Buckets: []float64{200, 500, 900, 1500},
		},
		[]string{},
	)

	reg.MustRegister(inFlightGauge, counter, histVec, responseSize, writeHeaderVec)

	return InstrumentHandlerInFlight(inFlightGauge,
		InstrumentHandlerCounter(counter,
			InstrumentHandlerDuration(histVec,
				InstrumentHandlerTimeToWriteHeader(writeHeaderVec,
					InstrumentHandlerResponseSize(responseSize, handler, opts...),
					opts...),
				opts...),
			opts...),
	), reg
}

func TestMiddlewareAPI(t *testing.T) {
	chain, reg := makeInstrumentedHandler(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("OK"))
	})

	r, _ := http.NewRequest(http.MethodGet, "www.example.com", nil)
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, r)

	assetMetricAndExemplars(t, reg, 5, nil)
}

func TestMiddlewareAPI_WithExemplars(t *testing.T) {
	exemplar := prometheus.Labels{"traceID": "example situation observed by this metric"}

	chain, reg := makeInstrumentedHandler(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("OK"))
	}, WithExemplarFromContext(func(_ context.Context) prometheus.Labels { return exemplar }))

	r, _ := http.NewRequest(http.MethodGet, "www.example.com", nil)
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, r)

	assetMetricAndExemplars(t, reg, 5, labelsToLabelPair(exemplar))
}

// TestMiddlewareAPI_SummaryWithExemplars is a regression test for
// https://github.com/prometheus/client_golang/issues/1258. SummaryVec is a
// valid prometheus.ObserverVec but the underlying summary does not implement
// prometheus.ExemplarObserver — only histograms can carry exemplars in the
// Prometheus exposition format. The instrumentation helpers must therefore
// fall back to a plain Observe when given a non-ExemplarObserver, instead of
// panicking with a failed type assertion at request time.
func TestMiddlewareAPI_SummaryWithExemplars(t *testing.T) {
	reg := prometheus.NewRegistry()

	durationVec := prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "request_duration_seconds",
			Help:       "A summary of request durations.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"code", "method"},
	)
	reg.MustRegister(durationVec)

	exemplar := prometheus.Labels{"traceID": "abc123"}
	handler := InstrumentHandlerDuration(
		durationVec,
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("OK"))
		}),
		WithExemplarFromContext(func(_ context.Context) prometheus.Labels { return exemplar }),
	)

	r, _ := http.NewRequest(http.MethodGet, "www.example.com", nil)
	w := httptest.NewRecorder()

	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("InstrumentHandlerDuration panicked with a SummaryVec observer: %v", rec)
		}
	}()
	handler.ServeHTTP(w, r)
}

// nonExemplarObserver implements prometheus.Observer but deliberately omits
// ObserveWithExemplar, so it does not satisfy prometheus.ExemplarObserver.
type nonExemplarObserver struct {
	last float64
}

func (o *nonExemplarObserver) Observe(v float64) { o.last = v }

// nonExemplarCounter implements prometheus.Counter but deliberately omits
// AddWithExemplar, so it does not satisfy prometheus.ExemplarAdder.
type nonExemplarCounter struct {
	last float64
}

func (c *nonExemplarCounter) Desc() *prometheus.Desc           { return nil }
func (c *nonExemplarCounter) Write(*dto.Metric) error          { return nil }
func (c *nonExemplarCounter) Describe(chan<- *prometheus.Desc) {}
func (c *nonExemplarCounter) Collect(chan<- prometheus.Metric) {}
func (c *nonExemplarCounter) Inc()                             { c.last = 1 }
func (c *nonExemplarCounter) Add(v float64)                    { c.last = v }

func TestObserveWithExemplar_NonExemplarObserverFallsBack(t *testing.T) {
	obs := &nonExemplarObserver{}

	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("observeWithExemplar panicked for non-ExemplarObserver: %v", rec)
		}
	}()
	observeWithExemplar(obs, 1.5, prometheus.Labels{"traceID": "abc"})

	if obs.last != 1.5 {
		t.Fatalf("expected fallback Observe(1.5), got Observe(%v)", obs.last)
	}
}

func TestAddWithExemplar_NonExemplarAdderFallsBack(t *testing.T) {
	c := &nonExemplarCounter{}

	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("addWithExemplar panicked for non-ExemplarAdder: %v", rec)
		}
	}()
	addWithExemplar(c, 2.5, prometheus.Labels{"traceID": "abc"})

	if c.last != 2.5 {
		t.Fatalf("expected fallback Add(2.5), got Add(%v)", c.last)
	}
}

func TestInstrumentTimeToFirstWrite(t *testing.T) {
	var i int
	dobs := &responseWriterDelegator{
		ResponseWriter: httptest.NewRecorder(),
		observeWriteHeader: func(status int) {
			i = status
		},
	}
	d := newDelegator(dobs, nil)

	d.WriteHeader(http.StatusOK)

	if i != http.StatusOK {
		t.Fatalf("failed to execute observeWriteHeader")
	}
}

// testResponseWriter is an http.ResponseWriter that also implements
// http.CloseNotifier, http.Flusher, and io.ReaderFrom.
type testResponseWriter struct {
	closeNotifyCalled, flushCalled, readFromCalled bool
}

func (t *testResponseWriter) Header() http.Header       { return nil }
func (t *testResponseWriter) Write([]byte) (int, error) { return 0, nil }
func (t *testResponseWriter) WriteHeader(int)           {}
func (t *testResponseWriter) CloseNotify() <-chan bool {
	t.closeNotifyCalled = true
	return nil
}
func (t *testResponseWriter) Flush() { t.flushCalled = true }
func (t *testResponseWriter) ReadFrom(io.Reader) (int64, error) {
	t.readFromCalled = true
	return 0, nil
}

// testFlusher is an http.ResponseWriter that also implements http.Flusher.
type testFlusher struct {
	flushCalled bool
}

func (t *testFlusher) Header() http.Header       { return nil }
func (t *testFlusher) Write([]byte) (int, error) { return 0, nil }
func (t *testFlusher) WriteHeader(int)           {}
func (t *testFlusher) Flush()                    { t.flushCalled = true }

func TestInterfaceUpgrade(t *testing.T) {
	w := &testResponseWriter{}
	d := newDelegator(w, nil)
	//nolint:staticcheck // Ignore SA1019. http.CloseNotifier is deprecated but we keep it here to not break existing users.
	d.(http.CloseNotifier).CloseNotify()
	if !w.closeNotifyCalled {
		t.Error("CloseNotify not called")
	}
	d.(http.Flusher).Flush()
	if !w.flushCalled {
		t.Error("Flush not called")
	}
	d.(io.ReaderFrom).ReadFrom(nil)
	if !w.readFromCalled {
		t.Error("ReadFrom not called")
	}
	if _, ok := d.(http.Hijacker); ok {
		t.Error("delegator unexpectedly implements http.Hijacker")
	}

	f := &testFlusher{}
	d = newDelegator(f, nil)
	//nolint:staticcheck // Ignore SA1019. http.CloseNotifier is deprecated but we keep it here to not break existing users.
	if _, ok := d.(http.CloseNotifier); ok {
		t.Error("delegator unexpectedly implements http.CloseNotifier")
	}
	d.(http.Flusher).Flush()
	if !w.flushCalled {
		t.Error("Flush not called")
	}
	if _, ok := d.(io.ReaderFrom); ok {
		t.Error("delegator unexpectedly implements io.ReaderFrom")
	}
	if _, ok := d.(http.Hijacker); ok {
		t.Error("delegator unexpectedly implements http.Hijacker")
	}
}

// Regression test against https://github.com/prometheus/client_golang/pull/1318
func TestInstrumentHandlerLabelFromCtxConcurrent(_ *testing.T) {
	const (
		workers           = 32
		requestsPerWorker = 200
	)

	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_requests_total",
			Help: "A counter for requests to the wrapped handler.",
		},
		[]string{"dyn"},
	)
	histogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "test_request_duration_seconds",
			Help: "A histogram of latencies for requests to the wrapped handler.",
		},
		[]string{"dyn"},
	)

	reg := prometheus.NewRegistry()
	reg.MustRegister(counter, histogram)

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	dyn := WithLabelFromCtx("dyn", func(_ context.Context) string {
		return "v"
	})

	chain := InstrumentHandlerCounter(
		counter,
		InstrumentHandlerDuration(histogram, next, dyn),
		dyn,
	)

	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			for range requestsPerWorker {
				req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
				rec := httptest.NewRecorder()
				chain.ServeHTTP(rec, req)
			}
		}()
	}
	wg.Wait()
}

func ExampleInstrumentHandlerDuration() {
	inFlightGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "in_flight_requests",
		Help: "A gauge of requests currently being served by the wrapped handler.",
	})

	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_requests_total",
			Help: "A counter for requests to the wrapped handler.",
		},
		[]string{"code", "method"},
	)

	// duration is partitioned by the HTTP method and handler. It uses custom
	// buckets based on the expected request duration.
	duration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "request_duration_seconds",
			Help:    "A histogram of latencies for requests.",
			Buckets: []float64{.25, .5, 1, 2.5, 5, 10},
		},
		[]string{"handler", "method"},
	)

	// responseSize has no labels, making it a zero-dimensional
	// ObserverVec.
	responseSize := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "response_size_bytes",
			Help:    "A histogram of response sizes for requests.",
			Buckets: []float64{200, 500, 900, 1500},
		},
		[]string{},
	)

	// Create the handlers that will be wrapped by the middleware.
	pushHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("Push"))
	})
	pullHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("Pull"))
	})

	// Register all of the metrics in the standard registry.
	prometheus.MustRegister(inFlightGauge, counter, duration, responseSize)

	// Instrument the handlers with all the metrics, injecting the "handler"
	// label by currying.
	pushChain := InstrumentHandlerInFlight(inFlightGauge,
		InstrumentHandlerDuration(duration.MustCurryWith(prometheus.Labels{"handler": "push"}),
			InstrumentHandlerCounter(counter,
				InstrumentHandlerResponseSize(responseSize, pushHandler),
			),
		),
	)
	pullChain := InstrumentHandlerInFlight(inFlightGauge,
		InstrumentHandlerDuration(duration.MustCurryWith(prometheus.Labels{"handler": "pull"}),
			InstrumentHandlerCounter(counter,
				InstrumentHandlerResponseSize(responseSize, pullHandler),
			),
		),
	)

	http.Handle("/metrics", Handler())
	http.Handle("/push", pushChain)
	http.Handle("/pull", pullChain)

	if err := http.ListenAndServe(":3000", nil); err != nil {
		log.Fatal(err)
	}
}
