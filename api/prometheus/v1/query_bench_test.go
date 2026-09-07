// Copyright 2025 The Prometheus Authors
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

package v1

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/api"
)

// buildLargeMatrixResponse builds a full Prometheus API envelope
// ({"status":"success","data":{...}}) holding a range-query matrix result
// with numSeries series of numSamples samples each. This mirrors the real
// wire shape decoded by queryResult.UnmarshalJSON.
func buildLargeMatrixResponse(numSeries, numSamples int) string {
	var sb strings.Builder
	sb.WriteString(`{"status":"success","data":{"resultType":"matrix","result":[`)
	for s := 0; s < numSeries; s++ {
		if s > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, `{"metric":{"__name__":"http_requests_total","instance":"10.0.0.%d:9100"},"values":[`, s%255)
		for v := 0; v < numSamples; v++ {
			if v > 0 {
				sb.WriteString(",")
			}
			fmt.Fprintf(&sb, `[%d.000,"%d"]`, 1700000000+v, v*3+s)
		}
		sb.WriteString(`]}`)
	}
	sb.WriteString(`]}}`)
	return sb.String()
}

func BenchmarkQueryRange(b *testing.B) {
	body := buildLargeMatrixResponse(100, 350) // ~35k samples; realistic large range-query payload.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	client, err := api.NewClient(api.Config{Address: srv.URL})
	if err != nil {
		b.Fatal(err)
	}
	v1api := NewAPI(client)
	ctx := context.Background()
	r := Range{Start: time.Now().Add(-time.Hour), End: time.Now(), Step: time.Minute}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, _, err := v1api.QueryRange(ctx, "up", r); err != nil {
			b.Fatal(err)
		}
	}
}
