package graphite

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"regexp"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestSanitize(t *testing.T) {
	testCases := []struct {
		in, out string
	}{
		{in: "hello", out: "hello"},
		{in: "hE/l1o", out: "hE_l1o"},
		{in: "he,*ll(.o", out: "he__ll__o"},
		{in: "hello_there%^&", out: "hello_there___"},
	}

	for i, tc := range testCases {
		if want, got := tc.out, sanitize(tc.in); want != got {
			t.Errorf("test case index %d: got sanitized string %s, want %s", i, got, want)
		}
	}
}

func TestToReader(t *testing.T) {
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

	want := `prefix.nameconstname.constvalue.labelname.val1 1 1477043083
prefix.nameconstname.constvalue.labelname.val2 1 1477043083
`
	mfs, err := reg.Gather()
	if err != nil {
		t.Errorf("error: %v", err)
	}

	now := int64(1477043083)
	buf, err := toReader(mfs, "prefix.", now)
	if err != nil {
		t.Errorf("error: %v", err)
	}

	if got := buf.String(); want != got {
		t.Errorf("wanted \n%s\n, got \n%s\n", want, got)
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
		t.Errorf("error creating bridge: %v", err)
	}
	defer b.Stop()

	nmg, err := newMockGraphite(port)
	if err != nil {
		t.Errorf("error creating mock graphite: %v", err)
	}
	defer nmg.Close()

	err = b.Push()
	if err != nil {
		t.Errorf("error pushing: %v", err)
	}

	wants := []string{
		"prefix.nameconstname.constvalue.labelname.val1 1",
		"prefix.nameconstname.constvalue.labelname.val2 1",
	}

	select {
	case got := <-nmg.readc:
		for _, want := range wants {
			matched, err := regexp.MatchString(want, got)
			if err != nil {
				t.Errorf("error pushing: %v", err)
			}
			if !matched {
				t.Errorf("missing metric:\nno match for %s received by server:\n%s", want, got)
			}
		}
		fmt.Println(b)
		return
	case err := <-nmg.errc:
		t.Errorf("error reading push: %v", err)
	}

	t.Fatal()
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
