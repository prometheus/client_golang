// Copyright 2014 Prometheus Team
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
	reqCnt.Reset()
	reqDur.Reset()
	reqSz.Reset()
	resSz.Reset()

	respBody := respBody("Howdy there!")

	hndlr := InstrumentHandler("test-handler", respBody)

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

	if want, got := 1, len(reqDur.children); want != got {
		t.Errorf("want %d children in reqDur, got %d", want, got)
	}
	sum, err := reqDur.GetMetricWithLabelValues("test-handler", "get", "418")
	if err != nil {
		t.Fatal(err)
	}
	out := &dto.Metric{}
	sum.Write(out)
	if want, got := "418", out.Label[0].GetValue(); want != got {
		t.Errorf("want label value %q in reqDur, got %q", want, got)
	}
	if want, got := "test-handler", out.Label[1].GetValue(); want != got {
		t.Errorf("want label value %q in reqDur, got %q", want, got)
	}
	if want, got := "get", out.Label[2].GetValue(); want != got {
		t.Errorf("want label value %q in reqDur, got %q", want, got)
	}
	if want, got := uint64(1), out.Summary.GetSampleCount(); want != got {
		t.Errorf("want sample count %d in reqDur, got %d", want, got)
	}

	out.Reset()
	if want, got := 1, len(reqCnt.children); want != got {
		t.Errorf("want %d children in reqCnt, got %d", want, got)
	}
	cnt, err := reqCnt.GetMetricWithLabelValues("test-handler", "get", "418")
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
