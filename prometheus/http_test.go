// Copyright 2014 The Prometheus Authors
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

package prometheus

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
)

type respBody string

func (b respBody) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusTeapot)
	w.Write([]byte(b))
}

func TestInstrumentHandler(t *testing.T) {
	defer func(n nower) {
		now = n.(nower)
	}(now)

	instant := time.Now()
	end := instant.Add(30 * time.Second)
	now = nowSeries(instant, end)
	respBody := respBody("Howdy there!")

	hndlr := InstrumentHandler("test-handler", respBody)

	opts := SummaryOpts{
		Subsystem:   "http",
		ConstLabels: Labels{"handler": "test-handler"},
	}

	reqCnt := MustRegisterOrGet(NewCounterVec(
		CounterOpts{
			Namespace:   opts.Namespace,
			Subsystem:   opts.Subsystem,
			Name:        "requests_total",
			Help:        "Total number of HTTP requests made.",
			ConstLabels: opts.ConstLabels,
		},
		instLabels,
	)).(*CounterVec)

	opts.Name = "request_duration_microseconds"
	opts.Help = "The HTTP request latencies in microseconds."
	reqDur := MustRegisterOrGet(NewSummary(opts)).(Summary)

	opts.Name = "request_size_bytes"
	opts.Help = "The HTTP request sizes in bytes."
	MustRegisterOrGet(NewSummary(opts))

	opts.Name = "response_size_bytes"
	opts.Help = "The HTTP response sizes in bytes."
	MustRegisterOrGet(NewSummary(opts))

	reqCnt.Reset()

	resp := httptest.NewRecorder()
	req := &http.Request{
		Method: "GET",
	}

	hndlr.ServeHTTP(resp, req)

	if resp.Code != http.StatusTeapot {
		t.Fatalf("expected status %d, got %d", http.StatusTeapot, resp.Code)
	}
	if string(resp.Body.Bytes()) != "Howdy there!" {
		t.Fatalf("expected body %s, got %s", "Howdy there!", string(resp.Body.Bytes()))
	}

	out := &dto.Metric{}
	reqDur.Write(out)
	if want, got := "test-handler", out.Label[0].GetValue(); want != got {
		t.Errorf("want label value %q in reqDur, got %q", want, got)
	}
	if want, got := uint64(1), out.Summary.GetSampleCount(); want != got {
		t.Errorf("want sample count %d in reqDur, got %d", want, got)
	}

	out.Reset()
	if want, got := 1, len(reqCnt.children); want != got {
		t.Errorf("want %d children in reqCnt, got %d", want, got)
	}
	cnt, err := reqCnt.GetMetricWithLabelValues("get", "418")
	if err != nil {
		t.Fatal(err)
	}
	cnt.Write(out)
	if want, got := "418", out.Label[0].GetValue(); want != got {
		t.Errorf("want label value %q in reqCnt, got %q", want, got)
	}
	if want, got := "test-handler", out.Label[1].GetValue(); want != got {
		t.Errorf("want label value %q in reqCnt, got %q", want, got)
	}
	if want, got := "get", out.Label[2].GetValue(); want != got {
		t.Errorf("want label value %q in reqCnt, got %q", want, got)
	}
	if out.Counter == nil {
		t.Fatal("expected non-nil counter in reqCnt")
	}
	if want, got := 1., out.Counter.GetValue(); want != got {
		t.Errorf("want reqCnt of %f, got %f", want, got)
	}
}

type errorCollector struct{}

func (e errorCollector) Describe(ch chan<- *Desc) {
	ch <- NewDesc("invalid_metric", "not helpful", nil, nil)
}

func (e errorCollector) Collect(ch chan<- Metric) {
	ch <- NewInvalidMetric(
		NewDesc("invalid_metric", "not helpful", nil, nil),
		errors.New("collect error"),
	)
}

func TestHandlerErrorHandling(t *testing.T) {

	// Create a registry that collects a MetricFamily with two elements,
	// another with one, and reports an error.
	reg := NewRegistry()

	cnt := NewCounter(CounterOpts{
		Name: "the_count",
		Help: "Ah-ah-ah! Thunder and lightning!",
	})
	MustRegisterWith(reg, cnt)

	cntVec := NewCounterVec(
		CounterOpts{
			Name:        "name",
			Help:        "docstring",
			ConstLabels: Labels{"constname": "constvalue"},
		},
		[]string{"labelname"},
	)
	cntVec.WithLabelValues("val1").Inc()
	cntVec.WithLabelValues("val2").Inc()
	MustRegisterWith(reg, cntVec)

	MustRegisterWith(reg, errorCollector{})

	logBuf := &bytes.Buffer{}
	logger := log.New(logBuf, "", 0)

	writer := httptest.NewRecorder()
	request, _ := http.NewRequest("GET", "/", nil)
	request.Header.Add("Accept", "test/plain")

	errorHandler := HandlerFor(reg, HandlerOpts{
		ErrorLog:      logger,
		ErrorHandling: HTTPErrorOnError,
	})
	continueHandler := HandlerFor(reg, HandlerOpts{
		ErrorLog:      logger,
		ErrorHandling: ContinueOnError,
	})
	panicHandler := HandlerFor(reg, HandlerOpts{
		ErrorLog:      logger,
		ErrorHandling: PanicOnError,
	})
	wantMsg := `error collecting metrics: 1 error(s) occurred:

* error collecting metric Desc{fqName: "invalid_metric", help: "not helpful", constLabels: {}, variableLabels: []}: collect error
`
	wantErrorBody := `An error has occurred during metrics collection:

1 error(s) occurred:

* error collecting metric Desc{fqName: "invalid_metric", help: "not helpful", constLabels: {}, variableLabels: []}: collect error
`
	wantOKBody := `# HELP name docstring
# TYPE name counter
name{constname="constvalue",labelname="val1"} 1
name{constname="constvalue",labelname="val2"} 1
# HELP the_count Ah-ah-ah! Thunder and lightning!
# TYPE the_count counter
the_count 0
`

	errorHandler.ServeHTTP(writer, request)
	if got, want := writer.Code, http.StatusInternalServerError; got != want {
		t.Errorf("got HTTP status code %d, want %d", got, want)
	}
	if got := logBuf.String(); got != wantMsg {
		t.Errorf("got log message %q, want %q", got, wantMsg)
	}
	if got := writer.Body.String(); got != wantErrorBody {
		t.Errorf("got body %q, want %q", got, wantErrorBody)
	}
	logBuf.Reset()
	writer.Body.Reset()
	writer.Code = http.StatusOK

	continueHandler.ServeHTTP(writer, request)
	if got, want := writer.Code, http.StatusOK; got != want {
		t.Errorf("got HTTP status code %d, want %d", got, want)
	}
	if got := logBuf.String(); got != wantMsg {
		t.Errorf("got log message %q, want %q", got, wantMsg)
	}
	if got := writer.Body.String(); got != wantOKBody {
		t.Errorf("got body %q, want %q", got, wantOKBody)
	}

	defer func() {
		if err := recover(); err == nil {
			t.Error("expected panic from panicHandler")
		}
	}()
	panicHandler.ServeHTTP(writer, request)
}
