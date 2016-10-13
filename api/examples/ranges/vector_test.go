// Copyright 2015 The Prometheus Authors
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

// A test case exposing how to send a Range Selector query (with beaer token)

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
	// url is proxied by kubernetes API Server, and verified by API Server
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
