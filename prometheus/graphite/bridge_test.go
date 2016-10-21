package graphite

import (
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
