// Copyright The Prometheus Authors
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

package push_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus/push"
)

func TestPushDotSegments(t *testing.T) {
	for _, test := range []struct {
		name     string
		job      string
		grouping string
		wantPath string
	}{
		{"dot job", ".", "", "/metrics/job@base64/Lg"},
		{"dot-dot job", "..", "", "/metrics/job@base64/Li4"},
		{"dot grouping", "testjob", ".", "/metrics/job/testjob/instance@base64/Lg"},
		{"dot-dot grouping", "testjob", "..", "/metrics/job/testjob/instance@base64/Li4"},
		{"embedded dot", "test.job", "", "/metrics/job/test.job"},
	} {
		t.Run(test.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/metrics/", func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					t.Errorf("Method = %q, want PUT", r.Method)
				}
				if r.URL.Path != test.wantPath {
					t.Errorf("Path = %q, want %q", r.URL.Path, test.wantPath)
				}
				w.WriteHeader(http.StatusOK)
			})
			server := httptest.NewServer(mux)
			defer server.Close()
			pusher := push.New(server.URL, test.job)
			if test.grouping != "" {
				pusher.Grouping("instance", test.grouping)
			}
			if err := pusher.Push(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
