// Copyright (c) 2014, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
)

func ExampleInstrumentHandler() {
	var myHandler http.Handler

	http.Handle("/profile", InstrumentHandler("profile", myHandler))
	// ... and without further ado, you get
	// - request count
	// - request size
	// - response size
	// - total latency
	//
	// all partitioned by
	// - handler name
	// - status code
	// - HTTP method
}

type respBody string

func (b respBody) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusTeapot)
	w.Write([]byte(b))
}

func TestInstrumentHandler(t *testing.T) {
	defer func(n nower, c CounterVec, d, reqS, resS SummaryVec) {
		now = n.(nower)
		reqCnt = c
		reqDur = d
		reqSz = reqS
		resSz = resS
	}(now, reqCnt, reqDur, reqSz, resSz)

	instant := time.Now()
	end := instant.Add(30 * time.Second)
	now = nowSeries(instant, end)
	reqCnt = NewCounterVec(reqCnt.(*counterVec).desc)
	reqDur = NewSummaryVec(reqDur.(*summaryVec).desc)
	reqSz = NewSummaryVec(reqSz.(*summaryVec).desc)
	resSz = NewSummaryVec(resSz.(*summaryVec).desc)

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

	out := &dto.MetricFamily{}
	// reqDur.Write(out)
	// if out.GetType() != dto.MetricType_SUMMARY {
	// 	t.Fatalf("expected type %d, got %s", dto.MetricType_SUMMARY, out.GetType())
	// }
	// if len(out.Metric) != 1 {
	// 	t.Fatalf("expected single metric, got %d", len(out.Metric))
	// }
	// if len(out.Metric[0].Label) != 3 {
	// 	t.Fatalf("expected triple labels, got %d", len(out.Metric[0].Label))
	// }
	// if out.Metric[0].Label[0].GetName() != "code" {
	// 	t.Fatalf("expected label named code, got %s", out.Metric[0].Label[0].GetName())
	// }
	// if out.Metric[0].Label[0].GetValue() != "418" {
	// 	t.Fatalf("expected label valued 418, got %s", out.Metric[0].Label[0].GetValue())
	// }
	// if out.Metric[0].Label[1].GetName() != "handler" {
	// 	t.Fatalf("expected label named handler, got %s", out.Metric[0].Label[1].GetName())
	// }
	// if out.Metric[0].Label[1].GetValue() != "test-handler" {
	// 	t.Fatalf("expected label valued test-handler, got %s", out.Metric[0].Label[1].GetValue())
	// }
	// if out.Metric[0].Label[2].GetName() != "method" {
	// 	t.Fatalf("expected label named method, got %s", out.Metric[0].Label[2].GetName())
	// }
	// if out.Metric[0].Label[2].GetValue() != "get" {
	// 	t.Fatalf("expected label valued get, got %s", out.Metric[0].Label[2].GetValue())
	// }
	// if out.Metric[0].Counter == nil {
	// 	t.Fatal("expected non-nil counter")
	// }
	// if out.Metric[0].Counter.GetValue() != 1 {
	// 	t.Fatalf("expected count of 1, got %d", out.Metric[0].Counter.GetValue())
	// }

	out.Reset()
		reqCnt.Write(out)
	if out.GetType() != dto.MetricType_COUNTER {
		t.Fatalf("expected type %d, got %s", dto.MetricType_COUNTER, out.GetType())
	}
	if len(out.Metric) != 1 {
		t.Fatalf("expected single metric, got %d", len(out.Metric))
	}
	if len(out.Metric[0].Label) != 3 {
		t.Fatalf("expected triple labels, got %d", len(out.Metric[0].Label))
	}
	if out.Metric[0].Label[0].GetName() != "code" {
		t.Fatalf("expected label named code, got %s", out.Metric[0].Label[0].GetName())
	}
	if out.Metric[0].Label[0].GetValue() != "418" {
		t.Fatalf("expected label valued 418, got %s", out.Metric[0].Label[0].GetValue())
	}
	if out.Metric[0].Label[1].GetName() != "handler" {
		t.Fatalf("expected label named handler, got %s", out.Metric[0].Label[1].GetName())
	}
	if out.Metric[0].Label[1].GetValue() != "test-handler" {
		t.Fatalf("expected label valued test-handler, got %s", out.Metric[0].Label[1].GetValue())
	}
	if out.Metric[0].Label[2].GetName() != "method" {
		t.Fatalf("expected label named method, got %s", out.Metric[0].Label[2].GetName())
	}
	if out.Metric[0].Label[2].GetValue() != "get" {
		t.Fatalf("expected label valued get, got %s", out.Metric[0].Label[2].GetValue())
	}
	if out.Metric[0].Counter == nil {
		t.Fatal("expected non-nil counter")
	}
	if out.Metric[0].Counter.GetValue() != 1 {
		t.Fatalf("expected count of 1, got %f", out.Metric[0].Counter.GetValue())
	}
}
