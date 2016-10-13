package ranges

import (
	"net/url"
	"testing"
	"time"
)

func TestGetNodeCPU(t *testing.T) {
	// Range Vector Selectors
	queryString := `delta(
container_cpu_usage_seconds_total
    {
        instance="192.168.1.25",
        job="kubernetes-nodes",
        kubernetes_io_hostname="192.168.1.25",
        id="/"
    }[5m]
)
  `
	// url is proxied by kubernetes API Server
	addr := "https://192.168.1.24:6443/api/v1/proxy/namespaces/kube-system/services/prometheus-monitor:9090/"
	token := "z7jCgNcP4oNIqXJhA2IJPRD4DrTJ6jhN" // ignore if addr is http
	start := time.Now().Add(-time.Hour)
	end := time.Now()

	results, err := QueryRange(addr, token, queryString, start, end)
	if err != nil {
		if urlErr, ok := err.(*url.Error); ok {
			t.Fatalf("query node cpu/usage_rate fails, url error:%#v\n", urlErr.Err.Error())
		} else {
			t.Fatalf("query node cpu/usage_rate fails, error:%#v\n", err)
		}

	}
	t.Logf("type:%s\n data: %s\n", results.Type(), results.String())
}
