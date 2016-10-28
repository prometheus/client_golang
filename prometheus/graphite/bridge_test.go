package graphite

import (
	"bytes"
	"io"
	"net"
	"regexp"
	"testing"
	"time"

	"github.com/prometheus/common/model"

	"github.com/prometheus/client_golang/prometheus"
)

func TestSanitize(t *testing.T) {
	testCases := []struct {
		in, out string
	}{
		{in: "hello", out: "hello"},
		{in: "hE/l1o", out: "he_l1o"},
		{in: "he,*ll(.o", out: "he_ll_o"},
		{in: "hello_there%^&", out: "hello_there"},
	}

	for i, tc := range testCases {
		if want, got := tc.out, sanitize(tc.in); want != got {
			t.Fatalf("test case index %d: got sanitized string %s, want %s", i, got, want)
		}
	}
}

func TestWriteSummary(t *testing.T) {
	sumVec := prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:        "name",
			Help:        "docstring",
			ConstLabels: prometheus.Labels{"constname": "constvalue"},
		},
		[]string{"labelname"},
	)

	sumVec.WithLabelValues("val1").Observe(float64(10))
	sumVec.WithLabelValues("val1").Observe(float64(20))
	sumVec.WithLabelValues("val1").Observe(float64(30))
	sumVec.WithLabelValues("val2").Observe(float64(20))
	sumVec.WithLabelValues("val2").Observe(float64(30))
	sumVec.WithLabelValues("val2").Observe(float64(40))

	reg := prometheus.NewRegistry()
	reg.MustRegister(sumVec)

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	now := model.Time(1477043083)
	var buf bytes.Buffer
	_, err = writeMetrics(&buf, mfs, "prefix", now)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	want := `prefix.name.constname.constvalue.labelname.val1.quantile.0_5 20 1477043
prefix.name.constname.constvalue.labelname.val1.quantile.0_9 30 1477043
prefix.name.constname.constvalue.labelname.val1.quantile.0_99 30 1477043
prefix.name_sum.constname.constvalue.labelname.val1 60 1477043
prefix.name_count.constname.constvalue.labelname.val1 3 1477043
prefix.name.constname.constvalue.labelname.val2.quantile.0_5 30 1477043
prefix.name.constname.constvalue.labelname.val2.quantile.0_9 40 1477043
prefix.name.constname.constvalue.labelname.val2.quantile.0_99 40 1477043
prefix.name_sum.constname.constvalue.labelname.val2 90 1477043
prefix.name_count.constname.constvalue.labelname.val2 3 1477043
`

	if got := buf.String(); want != got {
		t.Fatalf("wanted \n%s\n, got \n%s\n", want, got)
	}
}

func TestWriteHistogram(t *testing.T) {
	histVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:        "name",
			Help:        "docstring",
			ConstLabels: prometheus.Labels{"constname": "constvalue"},
			Buckets:     []float64{0.01, 0.02, 0.05, 0.1},
		},
		[]string{"labelname"},
	)

	histVec.WithLabelValues("val1").Observe(float64(10))
	histVec.WithLabelValues("val1").Observe(float64(20))
	histVec.WithLabelValues("val1").Observe(float64(30))
	histVec.WithLabelValues("val2").Observe(float64(20))
	histVec.WithLabelValues("val2").Observe(float64(30))
	histVec.WithLabelValues("val2").Observe(float64(40))

	reg := prometheus.NewRegistry()
	reg.MustRegister(histVec)

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	now := model.Time(1477043083)
	var buf bytes.Buffer
	_, err = writeMetrics(&buf, mfs, "prefix", now)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	want := `prefix.name_bucket.constname.constvalue.labelname.val1.le.0_01 0 1477043
prefix.name_bucket.constname.constvalue.labelname.val1.le.0_02 0 1477043
prefix.name_bucket.constname.constvalue.labelname.val1.le.0_05 0 1477043
prefix.name_bucket.constname.constvalue.labelname.val1.le.0_1 0 1477043
prefix.name_sum.constname.constvalue.labelname.val1 60 1477043
prefix.name_count.constname.constvalue.labelname.val1 3 1477043
prefix.name_bucket.constname.constvalue.labelname.val1.le.inf 3 1477043
prefix.name_bucket.constname.constvalue.labelname.val2.le.0_01 0 1477043
prefix.name_bucket.constname.constvalue.labelname.val2.le.0_02 0 1477043
prefix.name_bucket.constname.constvalue.labelname.val2.le.0_05 0 1477043
prefix.name_bucket.constname.constvalue.labelname.val2.le.0_1 0 1477043
prefix.name_sum.constname.constvalue.labelname.val2 90 1477043
prefix.name_count.constname.constvalue.labelname.val2 3 1477043
prefix.name_bucket.constname.constvalue.labelname.val2.le.inf 3 1477043
`
	if got := buf.String(); want != got {
		t.Fatalf("wanted \n%s\n, got \n%s\n", want, got)
	}
}

func TestToReader(t *testing.T) {
	cntVec := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "name",
			Help:        "docstring",
			ConstLabels: prometheus.Labels{"constname": "constvalue"},
		},
		[]string{"labelname"},
	)
	cntVec.WithLabelValues("val1").Inc()
	cntVec.WithLabelValues("val2").Inc()

	reg := prometheus.NewRegistry()
	reg.MustRegister(cntVec)

	want := `prefix.name.constname.constvalue.labelname.val1 1 1477043
prefix.name.constname.constvalue.labelname.val2 1 1477043
`
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	now := model.Time(1477043083)
	var buf bytes.Buffer
	_, err = writeMetrics(&buf, mfs, "prefix", now)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if got := buf.String(); want != got {
		t.Fatalf("wanted \n%s\n, got \n%s\n", want, got)
	}
}

func TestPush(t *testing.T) {
	reg := prometheus.NewRegistry()
	cntVec := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "name",
			Help:        "docstring",
			ConstLabels: prometheus.Labels{"constname": "constvalue"},
		},
		[]string{"labelname"},
	)
	cntVec.WithLabelValues("val1").Inc()
	cntVec.WithLabelValues("val2").Inc()
	reg.MustRegister(cntVec)

	host := "localhost"
	port := ":56789"
	b, err := NewBridge(&Config{
		URL:      host + port,
		Gatherer: reg,
		Prefix:   "prefix",
	})
	if err != nil {
		t.Fatalf("error creating bridge: %v", err)
	}

	nmg, err := newMockGraphite(port)
	if err != nil {
		t.Fatalf("error creating mock graphite: %v", err)
	}
	defer nmg.Close()

	err = b.Push()
	if err != nil {
		t.Fatalf("error pushing: %v", err)
	}

	wants := []string{
		"prefix.name.constname.constvalue.labelname.val1 1",
		"prefix.name.constname.constvalue.labelname.val2 1",
	}

	select {
	case got := <-nmg.readc:
		for _, want := range wants {
			matched, err := regexp.MatchString(want, got)
			if err != nil {
				t.Fatalf("error pushing: %v", err)
			}
			if !matched {
				t.Fatalf("missing metric:\nno match for %s received by server:\n%s", want, got)
			}
		}
		return
	case err := <-nmg.errc:
		t.Fatalf("error reading push: %v", err)
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("no result from graphite server")
	}
}

func newMockGraphite(port string) (*mockGraphite, error) {
	readc := make(chan string)
	errc := make(chan error)
	ln, err := net.Listen("tcp", port)
	if err != nil {
		return nil, err
	}

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			errc <- err
		}
		var b bytes.Buffer
		io.Copy(&b, conn)
		readc <- b.String()
	}()

	return &mockGraphite{
		readc:    readc,
		errc:     errc,
		Listener: ln,
	}, nil
}

type mockGraphite struct {
	readc chan string
	errc  chan error

	net.Listener
}
